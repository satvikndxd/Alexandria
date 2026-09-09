package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/alexandria-reads/alexandria/apps/api/internal/db"
	"github.com/alexandria-reads/alexandria/apps/api/internal/realtime"
	"github.com/alexandria-reads/alexandria/apps/api/internal/store"
)

// handleRoomToken mints a LiveKit join token — but only after Alexandria's own
// rules have said yes: the channel must be a room, the caller must be a club
// member, and the channel's spoiler gate must be passed. LiveKit is the SFU,
// not the authority; a leaked room name grants nothing.
func (s *Server) handleRoomToken(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	channelID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_id", "Malformed channel id.")
		return
	}
	channel, err := s.store.Queries().GetChannelByID(r.Context(), channelID)
	if errors.Is(err, pgx.ErrNoRows) {
		respondStoreError(w, store.ErrNotFound)
		return
	}
	if err != nil {
		respondStoreError(w, err)
		return
	}
	if channel.Kind != db.ChannelKindVoice && channel.Kind != db.ChannelKindVideo {
		respondError(w, http.StatusBadRequest, "not_a_room",
			"Only voice and video channels have rooms; text channels are read, not joined.")
		return
	}
	if _, err := s.store.Queries().GetClubMembership(r.Context(), db.GetClubMembershipParams{
		ClubID: channel.ClubID, UserID: sess.UserID,
	}); errors.Is(err, pgx.ErrNoRows) {
		respondError(w, http.StatusForbidden, "not_a_member",
			"Rooms are for club members. Join the club first.")
		return
	} else if err != nil {
		respondStoreError(w, err)
		return
	}

	// The spoiler gate applies to rooms too: a gated voice channel stays
	// closed below the threshold, so endings are not overheard either. The
	// gate reads the caller's private reading progress, so it must run inside
	// their RLS context — outside it, Postgres hides the progress row and
	// every reader would look like 0%.
	var gate db.ChannelIsSpoilerGatedForRow
	if err := s.scopedRead(r, func(q *db.Queries) error {
		g, err := q.ChannelIsSpoilerGatedFor(r.Context(), db.ChannelIsSpoilerGatedForParams{
			ChannelID: channelID, UserID: sess.UserID,
		})
		gate = g
		return err
	}); err != nil {
		respondStoreError(w, err)
		return
	}
	if spoilerGated(gate) {
		respondError(w, http.StatusForbidden, "spoiler_gated",
			"This room opens further into the book. Alexandria protects endings.")
		return
	}

	club, err := s.store.Queries().GetClubByID(r.Context(), channel.ClubID)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	token, err := s.lk.MintRoomToken(
		realtime.RoomName(club.Slug, channel.ID.String()),
		sess.UserID.String(), sess.Username, true, 2*time.Hour)
	if errors.Is(err, realtime.ErrDisabled) {
		respondError(w, http.StatusServiceUnavailable, "rooms_disabled",
			"This instance has no voice/video server configured.")
		return
	}
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"room":  realtime.RoomName(club.Slug, channel.ID.String()),
		"url":   s.lk.URL,
	})
}
