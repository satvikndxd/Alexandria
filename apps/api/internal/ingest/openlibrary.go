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
	"time"
)

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
		ua:      "AlexandriaBot/0.1 (+https://github.com/alexandria-reads/alexandria; open-source reading platform)",
	}
}

// WorkPayload is the message body on ingest.openlibrary_work.
type WorkPayload struct {
	OpenLibraryID string `json:"openlibrary_id"` // e.g. OL66554W
}

// OLWork is the subset of Open Library's work document we consume.
type OLWork struct {
	Title       string   `json:"title"`
	Description any      `json:"description"` // string OR {type,value}
	Subjects    []string `json:"subjects"`
	Covers      []int64  `json:"covers"`
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

// WorkUpserter is the slice of the store this worker needs.
type WorkUpserter interface {
	UpsertWorkFromOpenLibrary(ctx context.Context, olid, title, description string, subjects []string) error
}

// HandleWorkMessage is the JetStream handler for ingest.openlibrary_work.
// Idempotent: upserts keyed on openlibrary_id.
func HandleWorkMessage(client *OpenLibraryClient, db WorkUpserter) func(ctx context.Context, data []byte) error {
	return func(ctx context.Context, data []byte) error {
		var p WorkPayload
		if err := json.Unmarshal(data, &p); err != nil {
			slog.Error("malformed ingest payload dropped", "err", err)
			return nil // poison message: ack, do not redeliver forever
		}
		w, err := client.FetchWork(ctx, p.OpenLibraryID)
		if err != nil {
			return err // nak → JetStream backoff schedule
		}
		return db.UpsertWorkFromOpenLibrary(ctx, p.OpenLibraryID, w.Title, w.DescriptionText(), w.Subjects)
	}
}
