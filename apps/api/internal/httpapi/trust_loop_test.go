package httpapi_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// TestNotificationAndModerationLoop closes the trust loop over HTTP:
// addressed notifications on replies and follows, a report that routes to the
// moderator queue, an action that demands a rationale, and a metrics surface
// that shows the friction system working.
func TestNotificationAndModerationLoop(t *testing.T) {
	e := newEnv(t)
	w := e.seedWork("Middlemarch")

	// A moderator, registered and appointed explicitly.
	if _, err := e.st.RegisterUser(e.ctx, "queue_mod", "queuemod@example.com", nil); err != nil {
		t.Fatalf("register moderator: %v", err)
	}
	if _, err := e.ownerPool.Exec(e.ctx,
		`UPDATE users SET role='admin' WHERE username='queue_mod'`); err != nil {
		t.Fatalf("grant moderator: %v", err)
	}
	modC, modCSRF := e.signInExisting("queue_mod")

	if _, err := e.st.RegisterUser(e.ctx, "note_author", "noteauthor@example.com", nil); err != nil {
		t.Fatalf("register author: %v", err)
	}
	if _, err := e.st.RegisterUser(e.ctx, "kind_reader", "kindreader@example.com", nil); err != nil {
		t.Fatalf("register commenter: %v", err)
	}
	authorC, authorCSRF := e.signInExisting("note_author")
	commentC, commentCSRF := e.signInExisting("kind_reader")

	// The author publishes a review; a reader replies and follows.
	resp := e.send(authorC, http.MethodPost, "/v1/works/"+w.Slug+"/reviews", map[string]any{
		"rating": 5, "title": "A provincial canvas",
		"body": strings.Repeat("Middlemarch measures its characters against the society that made them, and the measurement is tender rather than cruel; that tenderness is why the book outlives its plot. ", 1),
	}, authorCSRF)
	review := mustStatus(t, resp, http.StatusCreated)
	reviewID := uuid.MustParse(nestedString(t, review, "review", "id"))

	resp = e.send(commentC, http.MethodPost, "/v1/reviews/"+reviewID.String()+"/comments", map[string]any{
		"body": "This changed how I'll reread the Casaubon chapters.",
	}, commentCSRF)
	mustStatus(t, resp, http.StatusCreated)

	resp = e.send(commentC, http.MethodPost, "/v1/users/note_author/follow", map[string]any{}, commentCSRF)
	mustStatus(t, resp, http.StatusOK)

	// The author's bell: two addressed notes, nothing broadcast.
	resp = e.get(authorC, "/v1/me/notifications")
	notes := mustStatus(t, resp, http.StatusOK)
	raw := mustJSON(t, notes)
	if !strings.Contains(raw, "review_comment") || !strings.Contains(raw, "new_follower") {
		t.Fatalf("notifications missing kinds: %s", raw)
	}
	resp = e.get(authorC, "/v1/me")
	if !strings.Contains(mustJSON(t, mustStatus(t, resp, http.StatusOK)), `"unread_notifications":2`) {
		t.Fatalf("unread count wrong, body: %s", raw)
	}
	resp = e.send(authorC, http.MethodPost, "/v1/me/notifications/read", map[string]any{}, authorCSRF)
	mustStatus(t, resp, http.StatusOK)
	resp = e.get(authorC, "/v1/me")
	if !strings.Contains(mustJSON(t, mustStatus(t, resp, http.StatusOK)), `"unread_notifications":0`) {
		t.Fatal("mark-all-read did not clear the bell")
	}

	// A reader reports the review; it routes to the queue.
	resp = e.send(commentC, http.MethodPost, "/v1/reports", map[string]any{
		"subject_type": "review", "subject_id": reviewID.String(),
		"reason": "ai_slop", "details": "reads machine-smooth to me",
	}, commentCSRF)
	mustStatus(t, resp, http.StatusCreated)

	resp = e.send(modC, http.MethodGet, "/v1/moderation/reports", nil, modCSRF)
	queue := mustStatus(t, resp, http.StatusOK)
	queueRaw := mustJSON(t, queue)
	if !strings.Contains(queueRaw, "ai_slop") {
		t.Fatalf("report not in queue: %s", queueRaw)
	}
	var parsed struct {
		Reports []struct {
			ID string `json:"id"`
		} `json:"reports"`
	}
	if err := json.Unmarshal([]byte(queueRaw), &parsed); err != nil {
		t.Fatalf("queue json: %v", err)
	}
	if len(parsed.Reports) == 0 {
		t.Fatal("queue empty after report")
	}
	reportID := parsed.Reports[0].ID

	// An action without a real rationale is refused (schema CHECK ≥10 chars).
	resp = e.send(modC, http.MethodPost, "/v1/moderation/reports/"+reportID+"/action", map[string]any{
		"kind": "warning", "rationale": "meh",
	}, modCSRF)
	mustStatus(t, resp, http.StatusUnprocessableEntity)

	// With prose, the action records and the report leaves the open queue.
	resp = e.send(modC, http.MethodPost, "/v1/moderation/reports/"+reportID+"/action", map[string]any{
		"kind": "note", "rationale": "Read closely; the prose is the reader's own and cites the novel. No action beyond a note.",
	}, modCSRF)
	mustStatus(t, resp, http.StatusOK)
	resp = e.send(modC, http.MethodGet, "/v1/moderation/reports", nil, modCSRF)
	if strings.Contains(mustJSON(t, mustStatus(t, resp, http.StatusOK)), reportID) {
		t.Fatal("resolved report still in the open queue")
	}

	// Metrics expose request flow and the friction system's effect.
	resp = e.get(e.client(), "/metrics")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("metrics status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close() //nolint:errcheck
	m := string(body)
	if !strings.Contains(m, "alexandria_http_requests_total") ||
		!strings.Contains(m, "alexandria_friction_refusals_total") {
		t.Fatalf("metrics missing series: %s", m[:min(400, len(m))])
	}
	if !strings.Contains(m, `code="rationale_required"`) && !strings.Contains(m, `code="invalid_review"`) {
		t.Fatalf("friction refusals not counted by code: %s", m[:min(600, len(m))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
