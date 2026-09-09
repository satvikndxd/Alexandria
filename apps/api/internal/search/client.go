// Package search is Alexandria's read path for discovery: a deliberately thin,
// typed client over Meilisearch's HTTP API.
//
// Why hand-rolled instead of the official SDK: the surface we need is four
// endpoints, and a 200-line client we can read in review beats a dependency
// whose release cadence we do not control. Indexing is derived state — Postgres
// remains the source of truth and the index can always be rebuilt from it
// (cmd/reindex).
package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	IndexWorks   = "works"
	IndexAuthors = "authors"
	IndexClubs   = "clubs"
)

type Client struct {
	base string
	key  string
	hc   *http.Client
}

func New(base, key string) *Client {
	return &Client{
		base: base,
		key:  key,
		hc:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Enabled reports whether search is configured. The API degrades to the
// Postgres trigram path when it is not, so a local dev box without Meilisearch
// still works.
func (c *Client) Enabled() bool { return c != nil && c.base != "" }

// ---- documents ----------------------------------------------------------------

type AuthorRef struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
	Role string `json:"role,omitempty"`
}

type WorkDoc struct {
	ID             string      `json:"id"`
	Kind           string      `json:"kind"` // "work"
	Slug           string      `json:"slug"`
	Title          string      `json:"title"`
	Subtitle       string      `json:"subtitle,omitempty"`
	Description    string      `json:"description,omitempty"`
	Authors        []AuthorRef `json:"authors"`
	Subjects       []string    `json:"subjects"`
	Language       string      `json:"language"`
	FirstPublished *int32      `json:"first_published,omitempty"`
	PublicDomain   bool        `json:"public_domain"`
	Rating         float64     `json:"rating"`
	RatingCount    int64       `json:"rating_count"`
	ReviewCount    int64       `json:"review_count"`
	GutenbergID    *int32      `json:"gutenberg_id,omitempty"`
	CoverKey       string      `json:"cover_key,omitempty"`
	CoverLicense   string      `json:"cover_license,omitempty"`
	EditionID      string      `json:"edition_id,omitempty"`
}

type AuthorDoc struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"` // "author"
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	SortName  string `json:"sort_name"`
	Bio       string `json:"bio,omitempty"`
	BirthYear *int32 `json:"birth_year,omitempty"`
	DeathYear *int32 `json:"death_year,omitempty"`
	Claimed   bool   `json:"claimed"`
	WorkCount int64  `json:"work_count"`
}

type ClubDoc struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"` // "club"
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MemberCount int64  `json:"member_count"`
}

// ---- index management -------------------------------------------------------------

type indexSettings struct {
	SearchableAttributes []string `json:"searchableAttributes"`
	FilterableAttributes []string `json:"filterableAttributes"`
	SortableAttributes   []string `json:"sortableAttributes"`
	TypoTolerance        *typo    `json:"typoTolerance,omitempty"`
}

type typo struct {
	Enabled bool `json:"enabled"`
}

func (c *Client) EnsureIndexes(ctx context.Context) error {
	for _, idx := range []struct {
		uid      string
		settings indexSettings
	}{
		{IndexWorks, indexSettings{
			SearchableAttributes: []string{"title", "authors.name", "subtitle", "subjects", "description"},
			FilterableAttributes: []string{"subjects", "language", "public_domain", "authors.slug", "first_published"},
			SortableAttributes:   []string{"rating", "rating_count", "review_count", "first_published", "title"},
		}},
		{IndexAuthors, indexSettings{
			SearchableAttributes: []string{"name", "sort_name", "bio"},
			FilterableAttributes: []string{"claimed"},
			SortableAttributes:   []string{"work_count", "name"},
		}},
		{IndexClubs, indexSettings{
			SearchableAttributes: []string{"name", "description"},
			SortableAttributes:   []string{"member_count", "name"},
		}},
	} {
		if err := c.createIndex(ctx, idx.uid); err != nil {
			return err
		}
		if err := c.put(ctx, "/indexes/"+idx.uid+"/settings", idx.settings); err != nil {
			return fmt.Errorf("settings for %s: %w", idx.uid, err)
		}
	}
	return nil
}

func (c *Client) createIndex(ctx context.Context, uid string) error {
	err := c.post(ctx, "/indexes", map[string]string{"uid": uid, "primaryKey": "id"})
	if err != nil && isIndexExists(err) {
		return nil // idempotent by design
	}
	return err
}

// ---- writes -------------------------------------------------------------------------

func (c *Client) IndexWorks(ctx context.Context, docs []WorkDoc) error {
	return c.post(ctx, "/indexes/"+IndexWorks+"/documents?primaryKey=id", docs)
}

func (c *Client) IndexAuthors(ctx context.Context, docs []AuthorDoc) error {
	return c.post(ctx, "/indexes/"+IndexAuthors+"/documents?primaryKey=id", docs)
}

func (c *Client) IndexClubs(ctx context.Context, docs []ClubDoc) error {
	return c.post(ctx, "/indexes/"+IndexClubs+"/documents?primaryKey=id", docs)
}

func (c *Client) DeleteDocuments(ctx context.Context, index string, ids []string) error {
	return c.post(ctx, "/indexes/"+index+"/documents/delete-batch", ids)
}

// ---- reads ---------------------------------------------------------------------------

// Query is the user-facing search request. Filters compose with AND; facets
// within a filter are OR.
type Query struct {
	Text         string
	Index        string // works | authors | clubs | "" (works+authors)
	Subjects     []string
	Language     string
	PublicDomain *bool
	AuthorSlug   string
	Sort         string // e.g. "rating:desc"
	Limit        int
	Offset       int
}

type Hit struct {
	Raw json.RawMessage `json:"-"`
	// Common fields across indexes, for rendering without a second fetch.
	ID          string      `json:"id"`
	Kind        string      `json:"kind"`
	Slug        string      `json:"slug"`
	Title       string      `json:"title,omitempty"`
	Name        string      `json:"name,omitempty"`
	Authors     []AuthorRef `json:"authors,omitempty"`
	Rating      float64     `json:"rating,omitempty"`
	RatingCount int64       `json:"rating_count,omitempty"`
	CoverKey    string      `json:"cover_key,omitempty"`
}

type Results struct {
	Hits             []Hit `json:"hits"`
	Total            int64 `json:"estimatedTotalHits"`
	ProcessingTimeMs int64 `json:"processingTimeMs"`
}

func (c *Client) Search(ctx context.Context, q Query) (*Results, error) {
	if q.Limit <= 0 || q.Limit > 50 {
		q.Limit = 20
	}
	index := q.Index
	if index == "" {
		index = IndexWorks
	}
	body := map[string]any{
		"q":      q.Text,
		"limit":  q.Limit,
		"offset": q.Offset,
		"attributesToRetrieve": []string{
			"id", "kind", "slug", "title", "name", "authors", "rating", "rating_count", "cover_key",
		},
	}
	if f := filterExpr(q); f != "" {
		body["filter"] = f
	}
	if q.Sort != "" {
		body["sort"] = []string{q.Sort}
	}
	var res Results
	if err := c.postInto(ctx, "/indexes/"+index+"/search", body, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// filterExpr builds Meilisearch's filter syntax from the typed query. Values
// are quoted and escaped so user input can never inject filter operators.
func filterExpr(q Query) string {
	var parts []string
	for _, s := range q.Subjects {
		parts = append(parts, `subjects = "`+escape(s)+`"`)
	}
	if q.Language != "" {
		parts = append(parts, `language = "`+escape(q.Language)+`"`)
	}
	if q.PublicDomain != nil {
		parts = append(parts, "public_domain = "+strconv.FormatBool(*q.PublicDomain))
	}
	if q.AuthorSlug != "" {
		parts = append(parts, `authors.slug = "`+escape(q.AuthorSlug)+`"`)
	}
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " AND "
		}
		out += p
	}
	return out
}

func escape(s string) string {
	r := url.QueryEscape(s)
	dec, err := url.QueryUnescape(r)
	if err != nil {
		return s
	}
	_ = dec
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			out = append(out, '\\', '"')
		case '\\':
			out = append(out, '\\', '\\')
		default:
			out = append(out, s[i])
		}
	}
	return string(out)
}

// ---- transport --------------------------------------------------------------------------

type apiErr struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

func (e apiErr) Error() string { return e.Code + ": " + e.Message }

func isIndexExists(err error) bool {
	var ae apiErr
	if ok := asAPIErr(err, &ae); ok {
		return ae.Code == "index_already_exists"
	}
	return false
}

func asAPIErr(err error, out *apiErr) bool {
	type wrapped interface{ APIError() *apiErr }
	if w, ok := err.(wrapped); ok {
		*out = *w.APIError()
		return true
	}
	return false
}

type httpErr struct {
	status int
	api    apiErr
}

func (e *httpErr) Error() string     { return fmt.Sprintf("meilisearch %d: %s", e.status, e.api.Message) }
func (e *httpErr) APIError() *apiErr { return &e.api }

func (c *Client) do(ctx context.Context, method, path string, body any, into any) error {
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.key != "" {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("meilisearch request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		he := &httpErr{status: resp.StatusCode}
		_ = json.Unmarshal(raw, &he.api)
		return he
	}
	if into != nil && len(raw) > 0 {
		return json.Unmarshal(raw, into)
	}
	return nil
}

func (c *Client) post(ctx context.Context, path string, body any) error {
	return c.do(ctx, http.MethodPost, path, body, nil)
}

func (c *Client) put(ctx context.Context, path string, body any) error {
	return c.do(ctx, http.MethodPut, path, body, nil)
}

func (c *Client) postInto(ctx context.Context, path string, body, into any) error {
	return c.do(ctx, http.MethodPost, path, body, into)
}

// Health pings the instance; used by /healthz to report search availability
// without ever failing the API because search is down.
func (c *Client) Health(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/health", nil, nil)
}
