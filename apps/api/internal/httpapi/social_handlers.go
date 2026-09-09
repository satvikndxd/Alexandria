package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/alexandria-reads/alexandria/apps/api/internal/db"
	"github.com/alexandria-reads/alexandria/apps/api/internal/store"
)

// ---- feed --------------------------------------------------------------------------------
// Chronological, followed-accounts-only. There is no ranking function to tune
// and therefore nothing to manipulate: the feed cannot be farmed.

func (s *Server) handleFeed(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	limit, _ := paginate(r, 20, 50)
	rows, err := s.store.Feed(r.Context(), sess.UserID, queryTime(r, "since"), limit)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"activity": rows})
}

func (s *Server) handleCommunityFeed(w http.ResponseWriter, r *http.Request) {
	limit, offset := paginate(r, 12, 50)
	rows, err := s.store.CommunityFeed(r.Context(), limit, offset)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"activity": rows})
}

// ---- profiles ------------------------------------------------------------------------------

func (s *Server) handlePublicProfile(w http.ResponseWriter, r *http.Request) {
	username := urlSlug(r, "username")
	profile, err := s.store.PublicProfile(r.Context(), username)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	if isPrivateProfile(profile.IsPrivate) {
		// A private account is visible as an existence-only card: no bio, no
		// shelves, no stats. Blocking scrapers from enumerating content while
		// keeping URLs stable.
		respondJSON(w, http.StatusOK, map[string]any{
			"user": map[string]any{
				"username": profile.Username, "is_private": true,
			},
		})
		return
	}
	counts, err := s.store.FollowCounts(r.Context(), profile.ID)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	reviews, err := s.store.ListReviewsByUser(r.Context(), profile.ID, 10, 0)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	resp := map[string]any{
		"user": map[string]any{
			"id": profile.ID, "username": profile.Username,
			"display_name": profile.DisplayName, "bio": profile.Bio,
			"pronouns": profile.Pronouns, "location": profile.Location,
			"avatar_key": profile.AvatarKey, "member_since": profile.CreatedAt,
		},
		"follows": counts,
		"reviews": reviews,
	}
	// The reader's own follow state toward this account.
	if sess, ok := CurrentUser(r); ok && sess.UserID != profile.ID {
		following, err := s.store.Queries().IsFollowing(r.Context(), db.IsFollowingParams{
			FollowerID: sess.UserID, FolloweeID: profile.ID,
		})
		if err != nil {
			respondStoreError(w, err)
			return
		}
		resp["following"] = following
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleUserReviews(w http.ResponseWriter, r *http.Request) {
	profile, err := s.store.PublicProfile(r.Context(), urlSlug(r, "username"))
	if err != nil {
		respondStoreError(w, err)
		return
	}
	if isPrivateProfile(profile.IsPrivate) {
		respondJSON(w, http.StatusOK, map[string]any{"reviews": []any{}})
		return
	}
	limit, offset := paginate(r, 24, 100)
	reviews, err := s.store.ListReviewsByUser(r.Context(), profile.ID, limit, offset)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"reviews": reviews})
}

func (s *Server) handleFollowers(w http.ResponseWriter, r *http.Request) {
	profile, err := s.store.PublicProfile(r.Context(), urlSlug(r, "username"))
	if err != nil {
		respondStoreError(w, err)
		return
	}
	limit, offset := paginate(r, 50, 100)
	rows, err := s.store.Followers(r.Context(), profile.ID, limit, offset)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"followers": rows})
}

func (s *Server) handleFollowing(w http.ResponseWriter, r *http.Request) {
	profile, err := s.store.PublicProfile(r.Context(), urlSlug(r, "username"))
	if err != nil {
		respondStoreError(w, err)
		return
	}
	limit, offset := paginate(r, 50, 100)
	rows, err := s.store.Following(r.Context(), profile.ID, limit, offset)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"following": rows})
}

func (s *Server) resolveProfileUser(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	profile, err := s.store.PublicProfile(r.Context(), urlSlug(r, "username"))
	if err != nil {
		respondStoreError(w, err)
		return uuid.Nil, false
	}
	return profile.ID, true
}

func (s *Server) handleFollow(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	target, ok := s.resolveProfileUser(w, r)
	if !ok {
		return
	}
	if err := s.store.Follow(r.Context(), sess.UserID, target); err != nil {
		respondStoreError(w, err)
		return
	}
	_ = s.store.Notify(r.Context(), target, "new_follower", map[string]any{
		"actor": sess.Username, "actor_id": sess.UserID.String(),
	})
	respondJSON(w, http.StatusOK, map[string]any{"following": true})
}

func (s *Server) handleUnfollow(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	target, ok := s.resolveProfileUser(w, r)
	if !ok {
		return
	}
	if err := s.store.Unfollow(r.Context(), sess.UserID, target); err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"following": false})
}

func (s *Server) handleBlock(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	target, ok := s.resolveProfileUser(w, r)
	if !ok {
		return
	}
	if err := s.store.Block(r.Context(), sess.UserID, target); err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"blocked": true})
}

func (s *Server) handleUnblock(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	target, ok := s.resolveProfileUser(w, r)
	if !ok {
		return
	}
	if err := s.store.Unblock(r.Context(), sess.UserID, target); err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"blocked": false})
}

// ---- notifications ---------------------------------------------------------------------------

func (s *Server) handleNotifications(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	limit, offset := paginate(r, 30, 100)
	unread := r.URL.Query().Get("unread") == "true"
	// Notifications are RLS-private: bare-pool reads see nothing, not even
	// the caller's own bell.
	var rows []db.Notification
	if err := s.scopedRead(r, func(q *db.Queries) error {
		got, err := q.ListNotifications(r.Context(), db.ListNotificationsParams{
			UserID: sess.UserID, UnreadOnly: &unread, Lim: limit, Off: offset,
		})
		rows = got
		return err
	}); err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"notifications": rows})
}

type markReadRequest struct {
	ID *uuid.UUID `json:"id"`
}

func (s *Server) handleMarkNotificationsRead(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	var req markReadRequest
	if r.ContentLength > 0 {
		if !decodeJSON(w, r, &req) {
			return
		}
	}
	if err := s.store.TxUser(r.Context(), sess.UserID, func(q *db.Queries) error {
		if req.ID != nil {
			_, err := q.MarkNotificationRead(r.Context(), db.MarkNotificationReadParams{ID: *req.ID, UserID: sess.UserID})
			return err
		}
		_, err := q.MarkAllNotificationsRead(r.Context(), sess.UserID)
		return err
	}); err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "read"})
}

// ---- clubs --------------------------------------------------------------------------------------

func (s *Server) handleListClubs(w http.ResponseWriter, r *http.Request) {
	limit, offset := paginate(r, 24, 100)
	var viewer *uuid.UUID
	if sess, ok := CurrentUser(r); ok {
		viewer = &sess.UserID
	}
	rows, err := s.store.Queries().ListClubs(r.Context(), db.ListClubsParams{
		ViewerID: store.UUIDPtr(viewer),
		Query:    strNil(r.URL.Query().Get("q")),
		Lim:      limit,
		Off:      offset,
	})
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"clubs": rows})
}

type createClubRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	IsPrivate   bool   `json:"is_private"`
}

func (s *Server) handleCreateClub(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	var req createClubRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.Name) < 3 || len(req.Name) > 100 {
		respondError(w, http.StatusUnprocessableEntity, "invalid_club", "Club names are 3–100 characters.")
		return
	}
	slug := req.Slug
	if slug == "" {
		slug = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(req.Name), " ", "-"))
	}
	club, err := s.store.Queries().CreateClub(r.Context(), db.CreateClubParams{
		Slug: slug, Name: req.Name, Description: req.Description,
		IsPrivate: req.IsPrivate, CreatedBy: store.UUIDPtr(&sess.UserID),
	})
	if err != nil {
		respondStoreError(w, err)
		return
	}
	// The founder is its organizer; a club with no organizer is unmoderatable.
	if _, err := s.store.Queries().JoinClub(r.Context(), db.JoinClubParams{
		ClubID: club.ID, UserID: sess.UserID,
	}); err != nil {
		respondStoreError(w, err)
		return
	}
	if _, err := s.store.Queries().SetClubMemberRole(r.Context(), db.SetClubMemberRoleParams{
		ClubID: club.ID, UserID: sess.UserID, Role: db.ClubRoleOrganizer,
	}); err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{"club": club})
}

func (s *Server) handleGetClub(w http.ResponseWriter, r *http.Request) {
	club, err := s.store.Queries().GetClubBySlug(r.Context(), urlSlug(r, "slug"))
	if err != nil {
		respondStoreError(w, err)
		return
	}
	if club.IsPrivate {
		sess, ok := CurrentUser(r)
		member := false
		if ok {
			if _, merr := s.store.Queries().GetClubMembership(r.Context(), db.GetClubMembershipParams{
				ClubID: club.ID, UserID: sess.UserID,
			}); merr == nil {
				member = true
			}
		}
		if !member {
			respondError(w, http.StatusNotFound, "not_found", "No such club.")
			return
		}
	}
	read, _ := s.store.Queries().GetCurrentClubRead(r.Context(), club.ID)
	respondJSON(w, http.StatusOK, map[string]any{"club": club, "current_read": read})
}

func (s *Server) handleJoinClub(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	club, err := s.store.Queries().GetClubBySlug(r.Context(), urlSlug(r, "slug"))
	if err != nil {
		respondStoreError(w, err)
		return
	}
	m, err := s.store.Queries().JoinClub(r.Context(), db.JoinClubParams{ClubID: club.ID, UserID: sess.UserID})
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"membership": m})
}

func (s *Server) handleLeaveClub(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	club, err := s.store.Queries().GetClubBySlug(r.Context(), urlSlug(r, "slug"))
	if err != nil {
		respondStoreError(w, err)
		return
	}
	if _, err := s.store.Queries().LeaveClub(r.Context(), db.LeaveClubParams{
		ClubID: club.ID, UserID: sess.UserID,
	}); err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "left"})
}

func (s *Server) clubForRequest(w http.ResponseWriter, r *http.Request) (db.GetClubBySlugRow, bool) {
	club, err := s.store.Queries().GetClubBySlug(r.Context(), urlSlug(r, "slug"))
	if err != nil {
		respondStoreError(w, err)
		return db.GetClubBySlugRow{}, false
	}
	return club, true
}

// isPrivateProfile interprets the nullable is_private that results from the
// LEFT JOIN onto profiles: a user with no profile row (should not happen, but
// the schema permits it) is treated as public-by-default-empty, i.e. private
// content is simply absent rather than hidden behind a lie.
func isPrivateProfile(v *bool) bool { return v != nil && *v }

func (s *Server) handleListChannels(w http.ResponseWriter, r *http.Request) {
	club, ok := s.clubForRequest(w, r)
	if !ok {
		return
	}
	var viewer *uuid.UUID
	if sess, ok := CurrentUser(r); ok {
		viewer = &sess.UserID
	}
	rows, err := s.store.Queries().ListChannelsForClub(r.Context(), db.ListChannelsForClubParams{
		ClubID: club.ID, ViewerID: store.UUIDPtr(viewer),
	})
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"channels": rows})
}

// handleListMessages enforces chapter-aware spoiler gating server-side. A
// channel whose spoiler_threshold_bp exceeds the reader's progress on that
// channel's work returns 403 spoiler_gated — the bytes never leave the server,
// so no client bug or devtools session can leak an ending.
func (s *Server) handleListMessages(w http.ResponseWriter, r *http.Request) {
	channelID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_id", "Malformed channel id.")
		return
	}
	// Both the gate check and the message read run inside the caller's RLS
	// context: the gate consults the reader's own reading progress, which is
	// private data and therefore invisible outside a scoped transaction.
	limit, _ := paginate(r, 50, 100)
	var rows []db.ListMessagesRow
	err = s.scopedRead(r, func(q *db.Queries) error {
		g, err := q.ChannelIsSpoilerGatedFor(r.Context(), db.ChannelIsSpoilerGatedForParams{
			ChannelID: channelID,
			UserID:    viewerIDOrZero(r),
		})
		if err != nil {
			return err
		}
		if spoilerGated(g) {
			return errSpoilerGated
		}
		rows, err = q.ListMessages(r.Context(), db.ListMessagesParams{
			ChannelID:       channelID,
			Lim:             limit,
			BeforeCreatedAt: tsParam(r, "before"),
			BeforeID:        uuid.Nil,
		})
		return err
	})
	if errors.Is(err, errSpoilerGated) {
		respondError(w, http.StatusForbidden, "spoiler_gated",
			"This discussion is gated until you have read further. Alexandria protects endings.")
		return
	}
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"messages": rows})
}

type postMessageRequest struct {
	Body        string     `json:"body"`
	ReplyTo     *uuid.UUID `json:"reply_to"`
	HasSpoilers bool       `json:"has_spoilers"`
}

func (s *Server) handlePostMessage(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	channelID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_id", "Malformed channel id.")
		return
	}
	var req postMessageRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	body := strings.TrimSpace(req.Body)
	if len(body) < 1 || len(body) > 4000 {
		respondError(w, http.StatusUnprocessableEntity, "invalid_message", "Messages are 1–4000 characters.")
		return
	}
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
			"This channel is gated until you have read further.")
		return
	}
	msg, err := s.store.Queries().CreateMessage(r.Context(), db.CreateMessageParams{
		ChannelID: channelID, UserID: sess.UserID, Body: body,
		ReplyTo: store.UUIDPtr(req.ReplyTo), HasSpoilers: req.HasSpoilers,
	})
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{"message": msg})
}

// errSpoilerGated lets the scoped transaction abort the message read without
// conflating "gated" with a storage error.
var errSpoilerGated = errors.New("spoiler gated")

// spoilerGated interprets the gating projection: a channel is gated when it
// carries a threshold the reader has not reached. sqlc types the IS NULL
// predicate as interface{}, so the conversion is confined to this one helper.
func spoilerGated(gate db.ChannelIsSpoilerGatedForRow) bool {
	ungated, _ := gate.Ungated.(bool)
	if ungated || gate.SpoilerThresholdBp == nil {
		return false
	}
	return gate.ProgressBp < *gate.SpoilerThresholdBp
}

func viewerIDOrZero(r *http.Request) uuid.UUID {
	if sess, ok := CurrentUser(r); ok {
		return sess.UserID
	}
	return uuid.Nil
}
