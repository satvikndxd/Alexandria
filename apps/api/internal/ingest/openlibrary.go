// Package ingest contains the background workers that hydrate Alexandria's
// bibliography from external sources. Nothing here runs inside an HTTP
// request: handlers enqueue intents, workers do the slow, polite fetching.
package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/alexandria-reads/alexandria/apps/api/internal/store"
)

// politeUserAgent identifies every automated request, as Open Library's and
// Project Gutenberg's automated-access guidelines both require. An anonymous
// scraper is indistinguishable from an attack; a named bot with a contact
// address is a neighbour.
const politeUserAgent = "AlexandriaBot/0.2 (+https://github.com/alexandria-reads/alexandria; open-source reading platform; contact: infra@alexandria.example)"

// OpenLibraryClient is a deliberately polite client:
//   - single-flight pacing via a time.Ticker (default 1 req/sec)
//   - identifying User-Agent per Open Library's API guidelines
//   - exponential backoff on 429/5xx handled by JetStream redelivery
type OpenLibraryClient struct {
	http    *http.Client
	baseURL string
	pace    *time.Ticker
	ua      string
}

func NewOpenLibraryClient() *OpenLibraryClient {
	return &OpenLibraryClient{
		http:    &http.Client{Timeout: 20 * time.Second},
		baseURL: "https://openlibrary.org",
		pace:    time.NewTicker(time.Second),
		ua:      politeUserAgent,
	}
}

// WorkPayload is the message body on ingest.openlibrary_work.
type WorkPayload struct {
	OpenLibraryID string `json:"openlibrary_id"` // e.g. OL66554W
}

// OLWork is the subset of Open Library's work document we consume.
type OLWork struct {
	Title       string        `json:"title"`
	Description any           `json:"description"` // string OR {type,value}
	Subjects    []string      `json:"subjects"`
	Covers      []int64       `json:"covers"`
	Authors     []OLAuthorRef `json:"authors"`
}

func (w OLWork) DescriptionText() string {
	switch d := w.Description.(type) {
	case string:
		return d
	case map[string]any:
		if v, ok := d["value"].(string); ok {
			return v
		}
	}
	return ""
}

// FetchWork retrieves one work document, respecting the pace ticker.
func (c *OpenLibraryClient) FetchWork(ctx context.Context, olid string) (*OLWork, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.pace.C:
	}

	url := fmt.Sprintf("%s/works/%s.json", c.baseURL, olid)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.ua)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, fmt.Errorf("openlibrary rate limited (429): backing off")
	case resp.StatusCode >= 500:
		return nil, fmt.Errorf("openlibrary server error: %d", resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("openlibrary unexpected status %d: %s", resp.StatusCode, body)
	}

	var w OLWork
	if err := json.NewDecoder(resp.Body).Decode(&w); err != nil {
		return nil, fmt.Errorf("decode work %s: %w", olid, err)
	}
	return &w, nil
}

// OLAuthorRef is how Open Library embeds authorship on a work document.
type OLAuthorRef struct {
	Author struct {
		Key string `json:"key"` // "/authors/OL21594A"
	} `json:"author"`
	Role *string `json:"role"`
}

// OLAuthor is the subset of an author document we consume.
type OLAuthor struct {
	Name      string `json:"name"`
	BirthYear *int32 `json:"birth_date_year"`
	DeathYear *int32 `json:"death_date_year"`
	Bio       any    `json:"bio"`
}

// FetchAuthor retrieves one author document, at the same polite pace.
func (c *OpenLibraryClient) FetchAuthor(ctx context.Context, olid string) (*OLAuthor, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.pace.C:
	}
	url := fmt.Sprintf("%s/authors/%s.json", c.baseURL, olid)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.ua)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openlibrary author %s: status %d", olid, resp.StatusCode)
	}
	var a OLAuthor
	if err := json.NewDecoder(resp.Body).Decode(&a); err != nil {
		return nil, fmt.Errorf("decode author %s: %w", olid, err)
	}
	return &a, nil
}

// workAuthors caps how many author documents one work ingest may fetch. A
// runaway contributor list must not turn one message into fifty upstream
// requests; the remainder simply arrive on the next refresh.
const maxAuthorsPerWork = 5

// HandleWorkMessage is the JetStream handler for ingest.openlibrary_work.
// Idempotent: every write is an upsert keyed on openlibrary_id, so a redelivered
// message converges instead of duplicating.
func HandleWorkMessage(client *OpenLibraryClient, st *store.Store) func(ctx context.Context, data []byte) error {
	return func(ctx context.Context, data []byte) error {
		var p WorkPayload
		if err := json.Unmarshal(data, &p); err != nil {
			slog.Error("malformed ingest payload dropped", "err", err)
			return nil // poison message: ack, never redeliver forever
		}
		w, err := client.FetchWork(ctx, p.OpenLibraryID)
		if err != nil {
			return err // nak → JetStream backoff schedule
		}

		authors := make([]store.AuthorUpsert, 0, len(w.Authors))
		for i, ref := range w.Authors {
			if i >= maxAuthorsPerWork {
				break
			}
			olid := strings.TrimPrefix(ref.Author.Key, "/authors/")
			a, err := client.FetchAuthor(ctx, olid)
			if err != nil {
				slog.Warn("author fetch failed; continuing without it", "olid", olid, "err", err)
				continue
			}
			role := "author"
			if ref.Role != nil && *ref.Role != "" {
				role = *ref.Role
			}
			authors = append(authors, store.AuthorUpsert{
				Name: a.Name, OpenLibraryID: olid, Role: role,
				BirthYear: a.BirthYear, DeathYear: a.DeathYear,
			})
		}

		work, err := st.UpsertWorkMaterialized(ctx, store.WorkUpsert{
			Title: w.Title, Description: w.DescriptionText(),
			Language: "en", Subjects: w.Subjects,
			OpenLibraryID: p.OpenLibraryID, Authors: authors,
		})
		if err != nil {
			return err
		}
		// Keep the search projection in step. Derived state: safe to lose,
		// healed by cmd/reindex, so a failure must not fail the ingest.
		ctx = context.WithoutCancel(ctx)
		return st.PublishEvent(ctx, store.SubjectSearchIndex, map[string]any{"work_id": work.ID})
	}
}
