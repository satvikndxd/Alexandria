// Package httpapi_test holds the integration suite: real PostgreSQL, real
// migrations, real HTTP through httptest. These tests are the reason the
// project runs a Postgres cluster in CI — the invariants they protect (RLS,
// friction caps, spoiler gating, CSRF) are exactly the ones a unit test with a
// mocked store cannot see.
//
// Set ALEXANDRIA_TEST_DATABASE_URL to run them; without it they skip, so
// `go test ./...` stays green on a contributor laptop with no services up.
package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/alexandria-reads/alexandria/apps/api/internal/auth"
	"github.com/alexandria-reads/alexandria/apps/api/internal/config"
	"github.com/alexandria-reads/alexandria/apps/api/internal/db"
	"github.com/alexandria-reads/alexandria/apps/api/internal/httpapi"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alexandria-reads/alexandria/apps/api/internal/migrate"
	"github.com/alexandria-reads/alexandria/apps/api/internal/store"
)

var testDSN = os.Getenv("ALEXANDRIA_TEST_DATABASE_URL")

func TestMain(m *testing.M) {
	if testDSN == "" {
		fmt.Println("httpapi_test: ALEXANDRIA_TEST_DATABASE_URL unset; skipping integration suite")
		os.Exit(0)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := migrate.ConnectPool(ctx, testDSN)
	if err != nil {
		fmt.Println("connect:", err)
		os.Exit(1)
	}
	defer pool.Close()
	applied, err := migrate.New(pool).Up(ctx)
	if err != nil {
		fmt.Println("migrate:", err)
		os.Exit(1)
	}
	if len(applied) > 0 {
		fmt.Println("applied migrations:", applied)
	}
	os.Exit(m.Run())
}

// appDSN rewrites the owner DSN to the least-privilege application role.
func appDSN(owner string) string {
	if i := strings.Index(owner, "@"); i > 0 {
		return "postgres://alexandria_app:alexandria_app" + owner[i:]
	}
	return owner
}

// ---- fixtures -----------------------------------------------------------------------

type env struct {
	t         *testing.T
	ctx       context.Context
	st        *store.Store
	ownerPool *pgxpool.Pool // test-only probes; never used by the API under test
	srv       *httpapi.Server
	url       string
	client    func() *http.Client
	cfg       config.Config
}

func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()

	// Owner-role pool: migrations, truncation, and test-only probes.
	ownerPool, err := store.Connect(ctx, testDSN)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(ownerPool.Close)
	truncateAll(t, ownerPool)

	// The API itself runs as the least-privilege application role. A superuser
	// bypasses Row-Level Security by definition (even with FORCE), so testing
	// RLS through the owner role would prove nothing. This is the same shape
	// production must use.
	pool, err := store.Connect(ctx, appDSN(testDSN))
	if err != nil {
		t.Fatalf("connect as app role: %v", err)
	}
	t.Cleanup(pool.Close)

	st := store.New(pool)
	cfg := config.Config{
		Addr:                  ":0",
		ShutdownTimeout:       time.Second,
		DatabaseURL:           testDSN,
		WebAuthnRPID:          "localhost",
		WebAuthnRPDisplayName: "Alexandria Test",
		WebAuthnOrigins:       []string{"http://localhost:3000"},
		WebOrigin:             "http://localhost:3000",
		SessionPepper:         "test-pepper",
		ReviewMinChars:        150,
		NewAccountDaily:       2,
		TrustedDaily:          10,
		TrustedReputation:     100,
	}
	authSvc, err := auth.NewService(st, cfg.WebAuthnRPID, cfg.WebAuthnRPDisplayName, cfg.WebAuthnOrigins, false)
	if err != nil {
		t.Fatalf("auth service: %v", err)
	}
	magic := auth.NewMagicLinks(st, cfg.WebOrigin, false)
	srv := httpapi.New(cfg, st, authSvc, magic, nil) // nil search ⇒ degraded path

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return &env{t: t, ctx: ctx, st: st, ownerPool: ownerPool, srv: srv, url: ts.URL, cfg: cfg,
		client: func() *http.Client { return jarClient(t) }}
}

// truncateAll resets every content table between tests while preserving
// schema_migrations: each test starts from an empty library but the schema
// itself is applied once per run.
func truncateAll(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	tables := []string{
		"messages", "channels", "club_reads", "club_memberships", "clubs",
		"note_revisions", "note_reviews", "note_citations", "scholar_notes", "scholar_profiles",
		"review_comments", "review_likes", "reviews",
		"annotations", "reading_sessions", "shelf_items", "shelves",
		"external_identifiers", "cover_assets", "editions",
		"work_subjects", "subjects", "work_authors", "works", "authors",
		"affiliate_links", "gutenberg_texts",
		"notifications", "contribution_ledger", "moderation_actions", "reports",
		"follows", "user_blocks",
		"email_outbox", "auth_attempts", "auth_challenges", "sessions",
		"profiles", "webauthn_credentials", "users",
		"outbox", "ingest_jobs",
	}
	_, err := pool.Exec(context.Background(),
		"TRUNCATE TABLE "+strings.Join(tables, ", ")+" RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func jarClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	return &http.Client{Jar: jar, Timeout: 15 * time.Second}
}

// ---- helpers ------------------------------------------------------------------------

func (e *env) post(path string, body any, csrf string) *http.Response {
	e.t.Helper()
	return e.send(e.client(), http.MethodPost, path, body, csrf)
}

func (e *env) send(c *http.Client, method, path string, body any, csrf string) *http.Response {
	e.t.Helper()
	var rdr *strings.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			e.t.Fatalf("marshal: %v", err)
		}
		rdr = strings.NewReader(string(b))
	} else {
		rdr = strings.NewReader("")
	}
	req, err := http.NewRequestWithContext(e.ctx, method, e.url+path, rdr)
	if err != nil {
		e.t.Fatalf("request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if csrf != "" {
		req.Header.Set(auth.CSRFHeaderName, csrf)
	}
	resp, err := c.Do(req)
	if err != nil {
		e.t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

func (e *env) get(c *http.Client, path string) *http.Response {
	e.t.Helper()
	req, err := http.NewRequestWithContext(e.ctx, http.MethodGet, e.url+path, nil)
	if err != nil {
		e.t.Fatalf("request: %v", err)
	}
	resp, err := c.Do(req)
	if err != nil {
		e.t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func decodeBody(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close() //nolint:errcheck
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

func mustStatus(t *testing.T, resp *http.Response, want int) map[string]any {
	t.Helper()
	body := decodeBody(t, resp)
	if resp.StatusCode != want {
		t.Fatalf("status = %d, want %d (body: %v)", resp.StatusCode, want, body)
	}
	return body
}

// signIn creates an account and authenticates it through the magic-link flow,
// returning a cookie-jar client plus the CSRF token bound to that session.
func (e *env) signIn(username, email string) (*http.Client, string) {
	e.t.Helper()
	if _, err := e.st.RegisterUser(e.ctx, username, email, nil); err != nil {
		e.t.Fatalf("register %s: %v", username, err)
	}
	c := e.client()
	resp := e.send(c, http.MethodPost, "/v1/auth/magic-link", map[string]string{"email": email}, "")
	mustStatus(e.t, resp, http.StatusAccepted)

	token := e.latestMagicToken(email)
	resp = e.send(c, http.MethodPost, "/v1/auth/magic-link/verify", map[string]string{"token": token}, "")
	mustStatus(e.t, resp, http.StatusOK)

	resp = e.get(c, "/v1/me")
	body := mustStatus(e.t, resp, http.StatusOK)
	csrf, _ := body["csrf_token"].(string)
	if csrf == "" {
		e.t.Fatalf("no csrf token in /v1/me: %v", body)
	}
	return c, csrf
}

// latestMagicToken reads the queued email straight out of the outbox, exactly
// as a reader would read it from their inbox.
func (e *env) latestMagicToken(email string) string {
	e.t.Helper()
	var payload []byte
	err := e.ownerPool.QueryRow(e.ctx,
		`SELECT payload FROM email_outbox WHERE to_email = $1 AND template = 'magic_link'
		  ORDER BY id DESC LIMIT 1`, email).Scan(&payload)
	if err != nil {
		e.t.Fatalf("no magic link queued for %s: %v", email, err)
	}
	var p struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		e.t.Fatalf("payload: %v", err)
	}
	u, err := url.Parse(p.URL)
	if err != nil {
		e.t.Fatalf("url: %v", err)
	}
	tok := u.Query().Get("token")
	if tok == "" {
		e.t.Fatalf("no token in link %q", p.URL)
	}
	return tok
}

// seedWork creates a work (and edition) directly through the ingestion path,
// the same way a worker would.
func (e *env) seedWork(title string) *db.Work {
	e.t.Helper()
	w, err := e.st.UpsertWorkMaterialized(e.ctx, store.WorkUpsert{
		Title: title, Description: "A seeded work for integration tests.",
		Language: "en", Subjects: []string{"test fixtures"},
		Authors: []store.AuthorUpsert{{Name: "Fixture Author", Role: "author"}},
	})
	if err != nil {
		e.t.Fatalf("seed work: %v", err)
	}
	return w
}

func goodReviewBody(seed string) string {
	// >150 runes, >12 distinct words, no dominant glyph: passes every friction
	// rule by construction, and reads like a person wrote it.
	return strings.TrimSpace(fmt.Sprintf(
		"%s — This edition rewarded a slow second reading: the marginalia echo the "+
			"translation choices discussed in the introduction, and the pacing of the "+
			"final chapter lands differently once you know where the narrator is "+
			"standing. I docked half a star for the footnotes, which repeat rather than "+
			"clarify.", seed))
}

// signInExisting authenticates an already-registered account through the
// magic-link flow (the same path a reader without a passkey takes).
func (e *env) signInExisting(username string) (*http.Client, string) {
	e.t.Helper()
	var email string
	if err := e.st.Pool().QueryRow(e.ctx,
		`SELECT email FROM users WHERE username = $1`, username).Scan(&email); err != nil {
		e.t.Fatalf("lookup %s: %v", username, err)
	}
	c := e.client()
	resp := e.send(c, http.MethodPost, "/v1/auth/magic-link", map[string]string{"email": email}, "")
	mustStatus(e.t, resp, http.StatusAccepted)
	resp = e.send(c, http.MethodPost, "/v1/auth/magic-link/verify",
		map[string]string{"token": e.latestMagicToken(email)}, "")
	mustStatus(e.t, resp, http.StatusOK)
	resp = e.get(c, "/v1/me")
	body := mustStatus(e.t, resp, http.StatusOK)
	csrf, _ := body["csrf_token"].(string)
	return c, csrf
}

func (e *env) signInReuse(username string) (*http.Client, string) {
	return e.signInExisting(username)
}

// ---- small JSON helpers ---------------------------------------------------------------

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

// nestedString walks a decoded JSON body: nestedString(t, m, "shelf", "id").
func nestedString(t *testing.T, m map[string]any, path ...string) string {
	t.Helper()
	var cur any = m
	for _, k := range path {
		obj, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("path %v: not an object at %q (%T)", path, k, cur)
		}
		cur, ok = obj[k]
		if !ok {
			t.Fatalf("path %v: missing key %q", path, k)
		}
	}
	str, ok := cur.(string)
	if !ok {
		t.Fatalf("path %v: value is %T, not string", path, cur)
	}
	return str
}

func migrateConnect(pool *pgxpool.Pool) (*migrate.Migrator, error) {
	return migrate.New(pool), nil
}

var _ = uuid.Nil
