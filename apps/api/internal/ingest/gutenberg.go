// Package ingest contains the background workers that hydrate Alexandria's
// bibliography and derived state. Nothing here runs inside an HTTP request:
// handlers enqueue intents, workers do the slow, polite fetching.
package ingest

import (
	"compress/gzip"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/alexandria-reads/alexandria/apps/api/internal/search"
	"github.com/alexandria-reads/alexandria/apps/api/internal/store"
)

// CatalogURL is Project Gutenberg's official machine-readable catalog. Using
// the offline feed — rather than crawling the website — is what Gutenberg's
// own automated-access guidance asks for: one document, one request, no
// hammering of HTML pages.
const CatalogURL = "https://www.gutenberg.org/cache/epub/feeds/pg_catalog.csv.gz"

// TextURLPattern yields the canonical UTF-8 text file for an etext number.
// Texts are downloaded ONCE into our own object storage at ingest time; the
// reader never hotlinks gutenberg.org, which keeps their bandwidth costs flat
// no matter how many people read on Alexandria.
const TextURLPattern = "https://www.gutenberg.org/files/%d/%d-0.txt"

// CatalogRow is one line of pg_catalog.csv.
//
// Columns, per the published header:
//
//	Text#,Type,Issued,Title,Language,Authors,Subjects,LoCC,Bookshelves
type CatalogRow struct {
	TextID      int32
	Type        string
	Issued      string
	Title       string
	Language    string
	Authors     []string
	Subjects    []string
	LoCC        []string
	Bookshelves []string
}

// FetchCatalog downloads and parses the official catalog. It is a single
// streaming GET: the whole catalog is ~5 MB gzipped and parsing it is far
// cheaper than 90k per-book requests would be.
func FetchCatalog(ctx context.Context, url string) ([]CatalogRow, error) {
	if url == "" {
		url = CatalogURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", politeUserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch gutenberg catalog: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gutenberg catalog returned %d", resp.StatusCode)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gunzip catalog: %w", err)
	}
	defer gz.Close() //nolint:errcheck
	return parseCatalog(gz)
}

func parseCatalog(r io.Reader) ([]CatalogRow, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1 // tolerate rows with trailing columns missing
	cr.ReuseRecord = true

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("catalog header: %w", err)
	}
	col := map[string]int{}
	for i, name := range header {
		col[strings.TrimSpace(name)] = i
	}
	for _, required := range []string{"Text#", "Type", "Title", "Language", "Authors", "Subjects"} {
		if _, ok := col[required]; !ok {
			return nil, fmt.Errorf("catalog is missing expected column %q", required)
		}
	}

	get := func(rec []string, name string) string {
		i, ok := col[name]
		if !ok || i >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[i])
	}

	var out []CatalogRow
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			// A malformed line in a 90k-row public feed is noise, not failure:
			// skip it and keep the ingest moving, but say so.
			slog.Warn("skipping malformed catalog row", "err", err)
			continue
		}
		rowType := get(rec, "Type")
		if rowType != "Text" {
			continue // audiobooks/images arrive through other pipelines
		}
		id, err := strconv.Atoi(get(rec, "Text#"))
		if err != nil || id <= 0 {
			continue
		}
		title := get(rec, "Title")
		if title == "" {
			continue
		}
		out = append(out, CatalogRow{
			TextID:      int32(id),
			Type:        rowType,
			Issued:      get(rec, "Issued"),
			Title:       title,
			Language:    orLang(get(rec, "Language")),
			Authors:     splitList(get(rec, "Authors"), ";"),
			Subjects:    splitList(get(rec, "Subjects"), ";"),
			LoCC:        splitList(get(rec, "LoCC"), ";"),
			Bookshelves: splitList(get(rec, "Bookshelves"), ";"),
		})
	}
	return out, nil
}

// splitList splits on a separator while trimming; catalog fields embed commas
// inside quoted values ("Jefferson, Thomas, 1743-1826"), so only ';' is safe.
func splitList(v, sep string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func orLang(v string) string {
	if v == "" {
		return "en"
	}
	return v
}

// IngestCatalog upserts catalog rows into the FRBR model. Work and Edition are
// created in one transaction per row (store.UpsertGutenbergRecord), and the
// upserts are keyed on gutenberg_id, so re-running the job on a fresh catalog
// snapshot converges instead of duplicating.
//
// limit caps a single run so a worker restart cannot monopolize the database;
// the job is re-queued until the catalog is fully applied.
func IngestCatalog(ctx context.Context, st *store.Store, rows []CatalogRow, limit int) (applied int, err error) {
	for i, row := range rows {
		if limit > 0 && i >= limit {
			break
		}
		select {
		case <-ctx.Done():
			return applied, ctx.Err()
		default:
		}

		authors := make([]store.AuthorUpsert, 0, len(row.Authors))
		for _, a := range row.Authors {
			authors = append(authors, store.AuthorUpsert{Name: a, Role: "author"})
		}
		subjects := row.Subjects
		if len(row.Bookshelves) > 0 {
			subjects = append(append([]string{}, subjects...), row.Bookshelves...)
		}

		// License and trademark notices travel WITH the record. Gutenberg's
		// terms require the trademark statement to stay attached to their
		// texts, so it is stored as data rather than hoped-for in the UI.
		metadata := map[string]any{
			"gutenberg_id":   row.TextID,
			"issued":         row.Issued,
			"lcc":            row.LoCC,
			"text_url":       fmt.Sprintf(TextURLPattern, row.TextID, row.TextID),
			"license_note":   "Project Gutenberg™ eBook. Released under the Project Gutenberg License; public domain in the United States. Copyright status varies by jurisdiction.",
			"trademark_note": "Project Gutenberg is a trademark of the Project Gutenberg Literary Archive Foundation. This record links to and caches the official text; it does not imply endorsement.",
		}
		formats := map[string]string{"text/utf-8": fmt.Sprintf(TextURLPattern, row.TextID, row.TextID)}

		metaBytes, err := json.Marshal(metadata)
		if err != nil {
			return applied, err
		}
		if _, err := st.UpsertGutenbergRecord(ctx, store.GutenbergRecord{
			ID: row.TextID, Title: row.Title, Authors: row.Authors,
			Language: row.Language, Subjects: subjects, Formats: formats,
			Metadata: metaBytes,
		}, store.WorkUpsert{
			Title: row.Title, Language: row.Language,
			Description: "", IsPublicDomain: true,
			Subjects: subjects, Authors: authors,
		}); err != nil {
			return applied, fmt.Errorf("gutenberg etext %d: %w", row.TextID, err)
		}
		applied++
	}
	return applied, nil
}

// HandleSearchIndex consumes ingest.search_index and refreshes the Meilisearch
// projection for the affected works. The index is derived state: a failure here
// is retried by JetStream and, worst case, healed by cmd/reindex.
func HandleSearchIndex(st *store.Store, sc Indexer) func(ctx context.Context, data []byte) error {
	return func(ctx context.Context, data []byte) error {
		var p struct {
			WorkID string `json:"work_id"`
		}
		if err := json.Unmarshal(data, &p); err != nil {
			slog.Error("malformed search payload dropped", "err", err)
			return nil
		}
		docs, err := st.SearchDocs(ctx, []uuid.UUID{parseUUID(p.WorkID)})
		if err != nil {
			return err
		}
		if len(docs) == 0 {
			return nil
		}
		return sc.IndexWorks(ctx, docs)
	}
}

// Indexer is the slice of the search client the worker needs.
type Indexer interface {
	IndexWorks(ctx context.Context, docs []search.WorkDoc) error
}

func parseUUID(s string) uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil
	}
	return id
}

// RunCatalogJob is the worker entrypoint for a gutenberg_catalog job: fetch the
// official feed once, apply up to `limit` rows, and report how many landed so
// the caller can decide whether to re-queue.
func RunCatalogJob(ctx context.Context, st *store.Store, catalogURL string, limit int) (int, error) {
	rows, err := FetchCatalog(ctx, catalogURL)
	if err != nil {
		return 0, err
	}
	slog.Info("gutenberg catalog fetched", "rows", len(rows))
	return IngestCatalog(ctx, st, rows, limit)
}

// backoffSchedule is the retry ladder for failed ingest jobs. It lives here,
// next to the workers that fail, rather than in SQL.
func backoffSchedule(attempts int) time.Duration {
	switch {
	case attempts <= 1:
		return 5 * time.Second
	case attempts == 2:
		return 30 * time.Second
	case attempts == 3:
		return 2 * time.Minute
	default:
		return 10 * time.Minute
	}
}
