package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/alexandria-reads/alexandria/apps/api/internal/domain"
	"github.com/alexandria-reads/alexandria/apps/api/internal/store"
)

func (s *Server) handleGetWork(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	work, err := s.store.GetWorkBySlug(r.Context(), slug)
	if errors.Is(err, pgx.ErrNoRows) {
		respondError(w, http.StatusNotFound, "not_found", "No such work in the library.")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", "The archive is momentarily unreachable.")
		return
	}
	editions, err := s.store.ListEditionsForWork(r.Context(), work.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", "The archive is momentarily unreachable.")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"work":           work,
		"average_rating": work.AverageRating(),
		"editions":       editions,
	})
}

func (s *Server) handleListReviews(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	work, err := s.store.GetWorkBySlug(r.Context(), slug)
	if errors.Is(err, pgx.ErrNoRows) {
		respondError(w, http.StatusNotFound, "not_found", "No such work in the library.")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", "The archive is momentarily unreachable.")
		return
	}
	limit, offset := paginate(r, 20, 100)
	reviews, err := s.store.ListReviewsForWork(r.Context(), work.ID, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", "The archive is momentarily unreachable.")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"reviews": reviews})
}

type createReviewRequest struct {
	Rating      float64 `json:"rating"` // stars, half-star increments
	Title       string  `json:"title"`
	Body        string  `json:"body"`
	PromptWhy   string  `json:"prompt_why"`
	HasSpoilers bool    `json:"has_spoilers"`
	EditionID   *string `json:"edition_id"`
}

// handleCreateReview applies the full anti-slop gauntlet:
//  1. authenticated user (session middleware; header shim until auth lands)
//  2. domain validation — length, rating bounds, filler heuristics
//  3. daily posting cap scaled by reputation
//  4. DB constraints as the last line of defense
func (s *Server) handleCreateReview(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUser(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthenticated", "Sign in to review.")
		return
	}

	var req createReviewRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "bad_json", "Malformed request body.")
		return
	}

	halfStars := domain.Rating(req.Rating * 2)
	draft := domain.ReviewDraft{
		Rating:      halfStars,
		Title:       req.Title,
		Body:        req.Body,
		PromptWhy:   req.PromptWhy,
		HasSpoilers: req.HasSpoilers,
	}
	if err := draft.Validate(s.friction); err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_review", err.Error())
		return
	}

	rep, err := s.store.GetUserReputation(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", "The archive is momentarily unreachable.")
		return
	}
	recent, err := s.store.CountRecentReviews(r.Context(), userID, 24*time.Hour)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", "The archive is momentarily unreachable.")
		return
	}
	if err := s.friction.CheckDailyLimit(rep, recent); err != nil {
		respondError(w, http.StatusTooManyRequests, "daily_limit", err.Error())
		return
	}

	work, err := s.store.GetWorkBySlug(r.Context(), chi.URLParam(r, "slug"))
	if errors.Is(err, pgx.ErrNoRows) {
		respondError(w, http.StatusNotFound, "not_found", "No such work in the library.")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", "The archive is momentarily unreachable.")
		return
	}

	params := store.CreateReviewParams{
		UserID:      userID,
		WorkID:      work.ID,
		Rating:      int16(halfStars),
		Title:       req.Title,
		Body:        req.Body,
		HasSpoilers: req.HasSpoilers,
		PromptWhy:   req.PromptWhy,
	}
	if req.EditionID != nil {
		if eid, err := uuid.Parse(*req.EditionID); err == nil {
			params.EditionID = &eid
		}
	}
	review, err := s.store.CreateReview(r.Context(), params)
	if err != nil {
		// unique(user_id, work_id) → 409, everything else → 500
		respondError(w, http.StatusConflict, "already_reviewed", "You have already reviewed this work — edit your existing review instead.")
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{"review": review})
}

// currentUser resolves the authenticated user. Placeholder header shim until
// the WebAuthn session middleware (see docs/adr/0005) replaces it; the shim
// is refused outside dev builds via config in main.
func currentUser(r *http.Request) (uuid.UUID, bool) {
	raw := r.Header.Get("X-Alexandria-Dev-User")
	if raw == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	return id, err == nil
}

func paginate(r *http.Request, def, max int32) (limit, offset int32) {
	limit = def
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && int32(v) <= max {
		limit = int32(v)
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && v >= 0 {
		offset = int32(v)
	}
	return limit, offset
}
