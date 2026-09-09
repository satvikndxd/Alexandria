package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/alexandria-reads/alexandria/apps/api/internal/realtime"
)

// TestRoomAuthorization proves LiveKit is never the authority: membership,
// channel kind and the spoiler gate are all decided by the monolith before a
// token is signed, and the signed token carries exactly the grant we meant.
func TestRoomAuthorization(t *testing.T) {
	e := newEnv(t)
	w := e.seedWork("Crime and Punishment")

	ownerC, ownerCSRF := e.signIn("room_host", "host@example.com")
	strangerC, strangerCSRF := e.signIn("room_stranger", "stranger@example.com")

	resp := e.send(ownerC, http.MethodPost, "/v1/clubs", map[string]any{
		"name": "Russian Literature", "slug": "russian-literature-rooms",
	}, ownerCSRF)
	mustStatus(t, resp, http.StatusCreated)

	var clubID uuid.UUID
	if err := e.ownerPool.QueryRow(e.ctx,
		`SELECT id FROM clubs WHERE slug='russian-literature-rooms'`).Scan(&clubID); err != nil {
		t.Fatal(err)
	}
	var voiceID, textID, gatedID uuid.UUID
	if err := e.ownerPool.QueryRow(e.ctx, `
		INSERT INTO channels (club_id, kind, name) VALUES ($1,'voice','Reading Room') RETURNING id`, clubID).Scan(&voiceID); err != nil {
		t.Fatal(err)
	}
	if err := e.ownerPool.QueryRow(e.ctx, `
		INSERT INTO channels (club_id, kind, name) VALUES ($1,'text','general') RETURNING id`, clubID).Scan(&textID); err != nil {
		t.Fatal(err)
	}
	if err := e.ownerPool.QueryRow(e.ctx, `
		INSERT INTO channels (club_id, kind, name, work_id, spoiler_threshold_bp)
		VALUES ($1,'voice','part-five', $2, 6600) RETURNING id`, clubID, w.ID).Scan(&gatedID); err != nil {
		t.Fatal(err)
	}

	path := func(id uuid.UUID) string { return "/v1/channels/" + id.String() + "/room/token" }

	// A member gets a signed token with exactly the intended grant.
	resp = e.send(ownerC, http.MethodPost, path(voiceID), map[string]any{}, ownerCSRF)
	body := mustStatus(t, resp, http.StatusOK)
	token, _ := body["token"].(string)
	lk := realtime.Config{URL: "wss://lk.test", APIKey: "test-key", APISecret: "test-secret-test-secret"}
	cl, err := lk.VerifyRoomToken(token)
	if err != nil {
		t.Fatalf("issued token does not verify: %v", err)
	}
	if !cl.Video.RoomJoin || !cl.Video.CanPublish || cl.Video.CanRecord {
		t.Fatalf("grant wrong: %+v", cl.Video)
	}
	if cl.Video.Room != body["room"] {
		t.Fatalf("room mismatch: %q vs %v", cl.Video.Room, body["room"])
	}

	// A stranger gets nothing, and is told why.
	resp = e.send(strangerC, http.MethodPost, path(voiceID), map[string]any{}, strangerCSRF)
	mustStatus(t, resp, http.StatusForbidden)

	// Text channels are read, not joined.
	resp = e.send(ownerC, http.MethodPost, path(textID), map[string]any{}, ownerCSRF)
	mustStatus(t, resp, http.StatusBadRequest)

	// The spoiler gate closes rooms too: 10% progress cannot hear part five.
	resp = e.send(ownerC, http.MethodPost, "/v1/me/progress", map[string]any{
		"work_slug": w.Slug, "progress_bp": 1000,
	}, ownerCSRF)
	mustStatus(t, resp, http.StatusOK)
	resp = e.send(ownerC, http.MethodPost, path(gatedID), map[string]any{}, ownerCSRF)
	mustStatus(t, resp, http.StatusForbidden)

	// Past the threshold, the door opens.
	resp = e.send(ownerC, http.MethodPost, "/v1/me/progress", map[string]any{
		"work_slug": w.Slug, "progress_bp": 7000,
	}, ownerCSRF)
	mustStatus(t, resp, http.StatusOK)
	resp = e.send(ownerC, http.MethodPost, path(gatedID), map[string]any{}, ownerCSRF)
	mustStatus(t, resp, http.StatusOK)
}
