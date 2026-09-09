package httpapi_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// TestScholarPeerReviewFlow walks the entire scholarship lifecycle over HTTP:
// application → moderator verification → submission → two distinct verified
// approvals → publication, with the gates (citations, own-note, unverified
// reviewer, draft visibility) asserted at each step.
func TestScholarPeerReviewFlow(t *testing.T) {
	e := newEnv(t)
	w := e.seedWork("The Divine Comedy")

	register := func(username string) uuid.UUID {
		u, err := e.st.RegisterUser(e.ctx, username, username+"@example.com", nil)
		if err != nil {
			t.Fatalf("register %s: %v", username, err)
		}
		return u.ID
	}
	authorID := register("dante_reader")
	s1ID := register("scholar_one")
	s2ID := register("scholar_two")
	modID := register("the_dean")
	layID := register("just_reading")

	if _, err := e.ownerPool.Exec(e.ctx,
		`UPDATE users SET role='admin' WHERE id=$1`, modID); err != nil {
		t.Fatalf("grant moderator: %v", err)
	}

	modC, modCSRF := e.signInExisting("the_dean")
	s1C, s1CSRF := e.signInExisting("scholar_one")
	s2C, s2CSRF := e.signInExisting("scholar_two")
	auC, auCSRF := e.signInExisting("dante_reader")
	layC, layCSRF := e.signInExisting("just_reading")

	// Applications, then the moderator's verification act.
	for i, c := range []struct {
		cl   clientCSRF
		id   uuid.UUID
		name string
	}{{clientCSRF{s1C, s1CSRF}, s1ID, "one"}, {clientCSRF{s2C, s2CSRF}, s2ID, "two"}} {
		// ORCID iDs are unique per person; two applicants sharing one is a
		// conflict, and the API says so (asserted below).
		resp := e.send(c.cl.c, http.MethodPost, "/v1/me/scholar-profile", map[string]any{
			"field": "Medieval Literature", "coi_statement": "none",
			"orcid": []string{"0000-0002-1825-0097", "0000-0001-5109-3700"}[i],
		}, c.cl.csrf)
		mustStatus(t, resp, http.StatusCreated)
		resp = e.send(modC, http.MethodPost, "/v1/moderation/scholars/"+c.id.String(),
			map[string]any{"status": "verified"}, modCSRF)
		mustStatus(t, resp, http.StatusOK)
	}

	// A third account claiming scholar one's ORCID is refused, not 500'd.
	impC, impCSRF := e.signInExisting("just_reading")
	resp := e.send(impC, http.MethodPost, "/v1/me/scholar-profile", map[string]any{
		"field": "Something", "coi_statement": "none", "orcid": "0000-0002-1825-0097",
	}, impCSRF)
	mustStatus(t, resp, http.StatusConflict)

	noteBody := map[string]any{
		"title": "Virgil as shade and as guide",
		"body": strings.Repeat(
			"The choice of Virgil to lead the pilgrim is not merely literary homage; it positions imperial Rome as the necessary prelude to the Christian journey. ", 2),
		"kind":        "context",
		"chapter_ref": "Inferno, Canto I",
		"citations": []map[string]any{
			{"citation": "Ziolkowski, John. The Medieval Virgil. 1991.", "url": "https://example.org/z"},
		},
		"submit": true,
	}

	// A note without citations never enters review.
	noCite := map[string]any{}
	for k, v := range noteBody {
		noCite[k] = v
	}
	delete(noCite, "citations")
	resp = e.send(auC, http.MethodPost, "/v1/works/"+w.Slug+"/notes", noCite, auCSRF)
	mustStatus(t, resp, http.StatusUnprocessableEntity)

	// The real submission.
	resp = e.send(auC, http.MethodPost, "/v1/works/"+w.Slug+"/notes", noteBody, auCSRF)
	created := mustStatus(t, resp, http.StatusCreated)
	noteID := uuid.MustParse(nestedString(t, created, "note", "id"))
	if got := nestedString(t, created, "note", "author_id"); got != authorID.String() {
		t.Fatalf("note author = %s, want %s", got, authorID)
	}
	// The author is not a verified scholar, so the note is stamped community
	// provenance at creation — a fact, not a judgement.
	if !strings.Contains(mustJSON(t, created), `"is_community":true`) {
		t.Fatalf("expected community provenance for an unverified author: %v", created)
	}

	// Drafts in review are invisible to strangers.
	resp = e.get(e.client(), "/v1/notes/"+noteID.String())
	mustStatus(t, resp, http.StatusNotFound)

	// The author cannot review their own note.
	resp = e.send(auC, http.MethodPost, "/v1/notes/"+noteID.String()+"/review",
		map[string]any{"approved": true, "comments": "fine work"}, auCSRF)
	mustStatus(t, resp, http.StatusForbidden)

	// Nor can an unverified reader (who never even applied).
	resp = e.get(layC, "/v1/me/scholar-profile")
	mustStatus(t, resp, http.StatusNotFound)
	if _, err := e.ownerPool.Exec(e.ctx, `SELECT 1 WHERE $1 <> $2`, layID, s1ID); err != nil {
		t.Fatalf("sanity: %v", err)
	}
	resp = e.send(layC, http.MethodPost, "/v1/notes/"+noteID.String()+"/review",
		map[string]any{"approved": true, "comments": "looks right to me"}, layCSRF)
	mustStatus(t, resp, http.StatusForbidden)

	// First approval: still in review.
	resp = e.send(s1C, http.MethodPost, "/v1/notes/"+noteID.String()+"/review",
		map[string]any{"approved": true, "comments": "Defensible reading; sources check out."}, s1CSRF)
	body := mustStatus(t, resp, http.StatusOK)
	if nestedString(t, body, "status") != "in_review" {
		t.Fatalf("after one approval status = %v, want in_review", body["status"])
	}

	// Second distinct approval: published.
	resp = e.send(s2C, http.MethodPost, "/v1/notes/"+noteID.String()+"/review",
		map[string]any{"approved": true, "comments": "Concur; the citation carries the claim."}, s2CSRF)
	body = mustStatus(t, resp, http.StatusOK)
	if nestedString(t, body, "status") != "published" {
		t.Fatalf("after two approvals status = %v, want published", body["status"])
	}

	// Published notes are public record, citations and all.
	resp = e.get(e.client(), "/v1/notes/"+noteID.String())
	pub := mustStatus(t, resp, http.StatusOK)
	if !strings.Contains(mustJSON(t, pub), "Ziolkowski") {
		t.Fatal("published note lost its citation")
	}
	if !strings.Contains(mustJSON(t, pub), "revisions") {
		t.Fatal("published note lost its revision history")
	}

	// The queue is empty again, and only scholars/moderators may look.
	resp = e.send(layC, http.MethodGet, "/v1/scholar/queue", nil, layCSRF)
	mustStatus(t, resp, http.StatusForbidden)
	resp = e.send(s1C, http.MethodGet, "/v1/scholar/queue", nil, s1CSRF)
	mustStatus(t, resp, http.StatusOK)

	// Retraction keeps the record.
	resp = e.send(auC, http.MethodPost, "/v1/notes/"+noteID.String()+"/retract", map[string]any{}, auCSRF)
	mustStatus(t, resp, http.StatusOK)
	resp = e.get(e.client(), "/v1/notes/"+noteID.String())
	retr := mustStatus(t, resp, http.StatusOK)
	if nestedString(t, retr, "note", "status") != "retracted" {
		t.Fatalf("retracted note status = %v", retr)
	}
}

type clientCSRF struct {
	c    *http.Client
	csrf string
}
