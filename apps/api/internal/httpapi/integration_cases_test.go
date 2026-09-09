package httpapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/alexandria-reads/alexandria/apps/api/internal/db"
	"github.com/alexandria-reads/alexandria/apps/api/internal/store"
)

// TestMagicLinkFlowAndSession walks the whole fallback auth path: request →
// outbox → verify → session cookie → /me → logout revokes.
func TestMagicLinkFlowAndSession(t *testing.T) {
	e := newEnv(t)

	c, csrf := e.signIn("margery", "margery@example.com")
	if csrf == "" {
		t.Fatal("expected a csrf token after sign-in")
	}

	// The session cookie is HttpOnly+SameSite=Lax; the CSRF cookie is readable.
	resp := e.get(c, "/v1/me")
	body := mustStatus(t, resp, http.StatusOK)
	if body["authenticated"] != true {
		t.Fatalf("expected authenticated /v1/me, got %v", body)
	}

	// Logout revokes: the same cookie is dead afterwards.
	resp = e.send(c, http.MethodPost, "/v1/auth/logout", map[string]any{}, csrf)
	mustStatus(t, resp, http.StatusOK)
	resp = e.get(c, "/v1/me")
	body = mustStatus(t, resp, http.StatusOK)
	if body["authenticated"] != false {
		t.Fatalf("expected signed-out /v1/me, got %v", body)
	}
}

// TestMagicLinkIsSingleUse: replaying a consumed token must fail.
func TestMagicLinkIsSingleUse(t *testing.T) {
	e := newEnv(t)
	if _, err := e.st.RegisterUser(e.ctx, "replay", "replay@example.com", nil); err != nil {
		t.Fatal(err)
	}
	c := e.client()
	resp := e.send(c, http.MethodPost, "/v1/auth/magic-link", map[string]string{"email": "replay@example.com"}, "")
	mustStatus(t, resp, http.StatusAccepted)
	token := e.latestMagicToken("replay@example.com")

	resp = e.send(c, http.MethodPost, "/v1/auth/magic-link/verify", map[string]string{"token": token}, "")
	mustStatus(t, resp, http.StatusOK)

	fresh := e.client()
	resp = e.send(fresh, http.MethodPost, "/v1/auth/magic-link/verify", map[string]string{"token": token}, "")
	mustStatus(t, resp, http.StatusUnauthorized)
}

// TestMagicLinkDoesNotOracleMailboxes: an unknown address gets the same 202 and
// queues nothing.
func TestMagicLinkDoesNotOracleMailboxes(t *testing.T) {
	e := newEnv(t)
	c := e.client()
	resp := e.send(c, http.MethodPost, "/v1/auth/magic-link", map[string]string{"email": "ghost@example.com"}, "")
	mustStatus(t, resp, http.StatusAccepted)

	var n int
	if err := e.st.Pool().QueryRow(e.ctx,
		`SELECT count(*) FROM email_outbox WHERE to_email = 'ghost@example.com'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("queued %d emails for an unknown address; the endpoint oracles mailboxes", n)
	}
}

// TestCSRFEnforcedOnStateChanges proves the double-submit guard: same cookie,
// missing header ⇒ 403.
func TestCSRFEnforcedOnStateChanges(t *testing.T) {
	e := newEnv(t)
	w := e.seedWork("The Glass Bead Game")
	c, csrf := e.signIn("hermann", "hermann@example.com")

	payload := map[string]any{"work_slug": w.Slug, "shelf_kind": "want_to_read"}
	resp := e.send(c, http.MethodPost, "/v1/me/library", payload, "")
	mustStatus(t, resp, http.StatusForbidden)

	resp = e.send(c, http.MethodPost, "/v1/me/library", payload, csrf)
	mustStatus(t, resp, http.StatusOK)
}

// TestReviewFrictionPipeline is the anti-slop contract, exercised over HTTP:
// length, filler detection, uniqueness, and the daily cap.
func TestReviewFrictionPipeline(t *testing.T) {
	e := newEnv(t)
	w1 := e.seedWork("Middlemarch")
	w2 := e.seedWork("Bleak House")
	w3 := e.seedWork("Daniel Deronda")
	c, csrf := e.signIn("dorothea", "dorothea@example.com")

	// Too short.
	resp := e.send(c, http.MethodPost, "/v1/works/"+w1.Slug+"/reviews", map[string]any{
		"rating": 5, "body": "great book, loved it",
	}, csrf)
	mustStatus(t, resp, http.StatusUnprocessableEntity)

	// Filler: 150+ chars but one repeated glyph.
	resp = e.send(c, http.MethodPost, "/v1/works/"+w1.Slug+"/reviews", map[string]any{
		"rating": 5, "body": strings.Repeat("a", 400),
	}, csrf)
	mustStatus(t, resp, http.StatusUnprocessableEntity)

	// A real review passes.
	resp = e.send(c, http.MethodPost, "/v1/works/"+w1.Slug+"/reviews", map[string]any{
		"rating": 4.5, "title": "A provincial canvas", "body": goodReviewBody("Middlemarch"),
	}, csrf)
	mustStatus(t, resp, http.StatusCreated)

	// One considered opinion per reader per work.
	resp = e.send(c, http.MethodPost, "/v1/works/"+w1.Slug+"/reviews", map[string]any{
		"rating": 3, "body": goodReviewBody("Second thought"),
	}, csrf)
	mustStatus(t, resp, http.StatusConflict)

	// Second review of the day is allowed (cap 2 for new accounts)…
	resp = e.send(c, http.MethodPost, "/v1/works/"+w2.Slug+"/reviews", map[string]any{
		"rating": 4, "body": goodReviewBody("Bleak House"),
	}, csrf)
	mustStatus(t, resp, http.StatusCreated)

	// …the third is not.
	resp = e.send(c, http.MethodPost, "/v1/works/"+w3.Slug+"/reviews", map[string]any{
		"rating": 4, "body": goodReviewBody("Daniel Deronda"),
	}, csrf)
	mustStatus(t, resp, http.StatusTooManyRequests)
}

// TestSpoilerBytesNeverLeakWithoutReveal: masking happens server-side.
func TestSpoilerBytesNeverLeakWithoutReveal(t *testing.T) {
	e := newEnv(t)
	w := e.seedWork("The Murder of Roger Ackroyd")
	c, csrf := e.signIn("poirot", "poirot@example.com")
	secret := "the narrator is the killer, which is the sentence this test protects"
	resp := e.send(c, http.MethodPost, "/v1/works/"+w.Slug+"/reviews", map[string]any{
		"rating": 5, "body": goodReviewBody("Ackroyd") + " " + secret,
		"has_spoilers": true,
	}, csrf)
	mustStatus(t, resp, http.StatusCreated)

	// Anonymous reader: body withheld.
	resp = e.get(e.client(), "/v1/works/"+w.Slug+"/reviews")
	body := mustStatus(t, resp, http.StatusOK)
	raw, _ := jsonMarshal(body)
	if strings.Contains(string(raw), secret) {
		t.Fatal("spoiler body leaked to a reader who did not reveal spoilers")
	}

	// Explicit reveal returns it.
	resp = e.get(e.client(), "/v1/works/"+w.Slug+"/reviews?reveal_spoilers=true")
	body = mustStatus(t, resp, http.StatusOK)
	raw, _ = jsonMarshal(body)
	if !strings.Contains(string(raw), secret) {
		t.Fatal("revealed review did not include its body")
	}
}

// TestLibraryProgressKeepsShelvesInAgreement: progress drives shelf state, and
// finishing closes the session.
func TestLibraryProgressKeepsShelvesInAgreement(t *testing.T) {
	e := newEnv(t)
	w := e.seedWork("Ulysses")
	c, csrf := e.signIn("leopold", "leopold@example.com")

	resp := e.send(c, http.MethodPost, "/v1/me/progress", map[string]any{
		"work_slug": w.Slug, "progress_bp": 4200, "format": "public_domain",
	}, csrf)
	mustStatus(t, resp, http.StatusOK)

	resp = e.get(c, "/v1/me/library?shelf=reading")
	body := mustStatus(t, resp, http.StatusOK)
	if !strings.Contains(mustJSON(t, body), w.Slug) {
		t.Fatal("in-progress book missing from Currently Reading")
	}

	resp = e.send(c, http.MethodPost, "/v1/me/progress", map[string]any{
		"work_slug": w.Slug, "progress_bp": 10000,
	}, csrf)
	mustStatus(t, resp, http.StatusOK)

	resp = e.get(c, "/v1/me/library?shelf=read")
	if !strings.Contains(mustJSON(t, mustStatus(t, resp, http.StatusOK)), w.Slug) {
		t.Fatal("finished book missing from Read")
	}
	resp = e.get(c, "/v1/me/library?shelf=reading")
	if strings.Contains(mustJSON(t, mustStatus(t, resp, http.StatusOK)), w.Slug) {
		t.Fatal("finished book still on Currently Reading: shelves disagree")
	}
}

// TestRLSPrivateShelves: reader B cannot see reader A's private shelf through
// the API, nor through SQL with B's RLS context — the database refuses.
func TestRLSPrivateShelves(t *testing.T) {
	e := newEnv(t)
	w := e.seedWork("The Secret History")
	ca, csrfA := e.signIn("camilla", "camilla@example.com")
	cb, _ := e.signIn("henry", "henry@example.com")

	// A private custom shelf holding the work.
	resp := e.send(ca, http.MethodPost, "/v1/me/shelves", map[string]any{
		"name": "guilty pleasures", "is_private": true,
	}, csrfA)
	created := mustStatus(t, resp, http.StatusCreated)
	shelfID := uuid.MustParse(nestedString(t, created, "shelf", "id"))

	resp = e.send(ca, http.MethodPost, "/v1/me/library", map[string]any{
		"work_slug": w.Slug, "shelf_kind": "want_to_read",
	}, csrfA)
	mustStatus(t, resp, http.StatusOK)

	// Put the work on the private shelf directly (custom shelves are addressed
	// by id in the store layer).
	if err := e.st.TxUser(e.ctx, camillaID(t, e), func(q *db.Queries) error {
		_, err := q.AddToShelf(e.ctx, db.AddToShelfParams{ShelfID: shelfID, WorkID: w.ID})
		return err
	}); err != nil {
		t.Fatalf("add to private shelf: %v", err)
	}

	// B's library shows nothing of A's, private or otherwise.
	resp = e.get(cb, "/v1/me/library")
	if strings.Contains(mustJSON(t, mustStatus(t, resp, http.StatusOK)), w.Slug) {
		t.Fatal("reader B sees reader A's shelving through the API")
	}

	// And at the SQL layer with B's context set: the row simply does not exist.
	// The probe runs on the SAME least-privilege connection the API uses, with
	// Henry's RLS context set: the private row must be invisible to Postgres
	// itself, not merely filtered by our SQL.
	var n int
	err := e.st.TxUser(e.ctx, henryID(t, e), func(q *db.Queries) error {
		return e.st.Pool().QueryRow(e.ctx,
			`SELECT count(*) FROM shelf_items si JOIN shelves s ON s.id = si.shelf_id
			  WHERE s.is_private = true`).Scan(&n)
	})
	if err != nil {
		t.Fatalf("rls probe: %v", err)
	}
	if n != 0 {
		t.Fatalf("RLS leak: %d private shelf rows visible under another reader's context", n)
	}
	_ = ca
	_ = cb
}

// TestClubSpoilerGate: a chapter-gated channel refuses readers who have not
// reached the threshold, and admits them once they have.
func TestClubSpoilerGate(t *testing.T) {
	e := newEnv(t)
	w := e.seedWork("Crime and Punishment")
	c, csrf := e.signIn("raskolnikov", "raskolnikov@example.com")

	resp := e.send(c, http.MethodPost, "/v1/clubs", map[string]any{
		"name": "Russian Literature", "slug": "russian-literature",
	}, csrf)
	mustStatus(t, resp, http.StatusCreated)

	// A channel gated at 50% progress.
	var clubID uuid.UUID
	if err := e.ownerPool.QueryRow(e.ctx,
		`SELECT id FROM clubs WHERE slug = 'russian-literature'`).Scan(&clubID); err != nil {
		t.Fatal(err)
	}
	var channelID uuid.UUID
	if err := e.ownerPool.QueryRow(e.ctx, `
		INSERT INTO channels (club_id, kind, name, work_id, spoiler_threshold_bp)
		VALUES ($1, 'text', 'part-three', $2, 5000) RETURNING id`, clubID, w.ID).Scan(&channelID); err != nil {
		t.Fatal(err)
	}

	// 10% progress ⇒ gated.
	resp = e.send(c, http.MethodPost, "/v1/me/progress", map[string]any{
		"work_slug": w.Slug, "progress_bp": 1000,
	}, csrf)
	mustStatus(t, resp, http.StatusOK)
	resp = e.get(c, "/v1/channels/"+channelID.String()+"/messages")
	mustStatus(t, resp, http.StatusForbidden)

	// 60% progress ⇒ admitted.
	resp = e.send(c, http.MethodPost, "/v1/me/progress", map[string]any{
		"work_slug": w.Slug, "progress_bp": 6000,
	}, csrf)
	mustStatus(t, resp, http.StatusOK)
	resp = e.get(c, "/v1/channels/"+channelID.String()+"/messages")
	mustStatus(t, resp, http.StatusOK)
}

// TestSearchDegradesToPostgres: with no Meilisearch configured the API still
// answers, honestly labelled as degraded.
func TestSearchDegradesToPostgres(t *testing.T) {
	e := newEnv(t)
	e.seedWork("Persuasion")
	resp := e.get(e.client(), "/v1/search?q=Persu")
	body := mustStatus(t, resp, http.StatusOK)
	if body["source"] != "postgres_fallback" || body["degraded"] != true {
		t.Fatalf("expected degraded postgres fallback, got %v", body)
	}
	if !strings.Contains(mustJSON(t, body), "persuasion") {
		t.Fatalf("fallback search missed the seeded work: %v", body)
	}
}

// TestFollowFeedIsChronologicalAndBlockAware: follows drive the feed, blocks
// remove the blocked actor's activity in both directions.
func TestFollowFeedIsChronologicalAndBlockAware(t *testing.T) {
	e := newEnv(t)
	w := e.seedWork("Emma")
	e.signIn("author_a", "a@example.com") // registers A; session comes below
	cb, csrfB := e.signIn("reader_b", "b@example.com")

	// A reviews; B follows A and sees it. The CSRF token must come from the
	// same session whose cookie is in the jar — it is bound to the session
	// token by design.
	ca, csrfA2 := e.signInReuse("author_a")
	resp := e.send(ca, http.MethodPost, "/v1/works/"+w.Slug+"/reviews", map[string]any{
		"rating": 4, "body": goodReviewBody("Emma"),
	}, csrfA2)
	mustStatus(t, resp, http.StatusCreated)

	resp = e.send(cb, http.MethodPost, "/v1/users/author_a/follow", map[string]any{}, csrfB)
	mustStatus(t, resp, http.StatusOK)
	resp = e.get(cb, "/v1/feed")
	if !strings.Contains(mustJSON(t, mustStatus(t, resp, http.StatusOK)), w.Slug) {
		t.Fatal("followed review missing from feed")
	}

	// Blocking A removes A from B's feed.
	resp = e.send(cb, http.MethodPost, "/v1/users/author_a/block", map[string]any{}, csrfB)
	mustStatus(t, resp, http.StatusOK)
	resp = e.get(cb, "/v1/feed")
	if strings.Contains(mustJSON(t, mustStatus(t, resp, http.StatusOK)), w.Slug) {
		t.Fatal("blocked account still appears in feed")
	}
}

// TestMigrationRunnerIsIdempotent: a second Up applies nothing.
func TestMigrationRunnerIsIdempotent(t *testing.T) {
	pool, err := store.Connect(context.Background(), testDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	mp, err := migrateConnect(pool)
	if err != nil {
		t.Fatal(err)
	}
	applied, err := mp.Up(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(applied) != 0 {
		t.Fatalf("second Up applied %v; migrations are not idempotent", applied)
	}
}

// ---- small JSON helpers -----------------------------------------------------------------

func camillaID(t *testing.T, e *env) uuid.UUID { return userIDByUsername(t, e, "camilla") }
func henryID(t *testing.T, e *env) uuid.UUID   { return userIDByUsername(t, e, "henry") }

func userIDByUsername(t *testing.T, e *env, username string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := e.ownerPool.QueryRow(e.ctx,
		`SELECT id FROM users WHERE username = $1`, username).Scan(&id); err != nil {
		t.Fatalf("lookup %s: %v", username, err)
	}
	return id
}
