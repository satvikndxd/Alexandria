package httpapi

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/alexandria-reads/alexandria/apps/api/internal/db"
	"github.com/alexandria-reads/alexandria/apps/api/internal/domain"
	"github.com/alexandria-reads/alexandria/apps/api/internal/store"
)

// ---- catalogue -----------------------------------------------------------------------

func (s *Server) handleListWorks(w http.ResponseWriter, r *http.Request) {
	limit, offset := paginate(r, 24, 100)
	f := store.WorkFilters{
		SubjectSlug:     r.URL.Query().Get("subject"),
		Language:        r.URL.Query().Get("language"),
		PublicDomain:    queryBool(r, "public_domain"),
		PublishedAfter:  queryInt32(r, "published_after"),
		PublishedBefore: queryInt32(r, "published_before"),
		TitlePrefix:     r.URL.Query().Get("title_prefix"),
		Sort:            orSort(r.URL.Query().Get("sort"), "rating"),
		Limit:           limit,
		Offset:          offset,
	}
	if a := r.URL.Query().Get("author"); a != "" {
		if id, err := uuid.Parse(a); err == nil {
			f.AuthorID = &id
		}
	}
	works, err := s.store.ListWorks(r.Context(), f)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"works": works, "limit": limit, "offset": offset,
	})
}

func orSort(v, def string) string {
	switch v {
	case "rating", "recent", "title", "highest", "lowest", "liked":
		return v
	}
	return def
}

func (s *Server) handleGetWork(w http.ResponseWriter, r *http.Request) {
	slug := urlSlug(r, "slug")
	detail, err := s.store.WorkDetail(r.Context(), slug)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	resp := map[string]any{
		"work":                detail.Work,
		"average_rating":      averageRating(detail.Work.RatingSum, detail.Work.RatingCount),
		"authors":             detail.Authors,
		"editions":            detail.Editions,
		"subjects":            detail.Subjects,
		"rating_distribution": detail.Ratings,
		"review_count":        detail.ReviewCount,
	}
	// The caller's own shelf state, when signed in, so the book page can render
	// "Continue reading" / "On your Read shelf" without a second request.
	if sess, ok := CurrentUser(r); ok {
		entry, err := s.store.LibraryEntry(r.Context(), sess.UserID, detail.Work.ID)
		if err != nil {
			respondStoreError(w, err)
			return
		}
		resp["my_library"] = entry
	}
	respondJSON(w, http.StatusOK, resp)
}

func averageRating(sum, count int64) float64 {
	if count == 0 {
		return 0
	}
	return float64(sum) / float64(count) / 2.0 // half-stars → stars
}

func (s *Server) handleListEditions(w http.ResponseWriter, r *http.Request) {
	work, err := s.store.GetWorkBySlug(r.Context(), urlSlug(r, "slug"))
	if err != nil {
		respondStoreError(w, err)
		return
	}
	editions, err := s.store.Queries().ListEditionsForWork(r.Context(), work.ID)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"editions": editions})
}

func (s *Server) handleGetAuthor(w http.ResponseWriter, r *http.Request) {
	author, err := s.store.GetAuthorBySlug(r.Context(), urlSlug(r, "slug"))
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"author": author})
}

func (s *Server) handleAuthorWorks(w http.ResponseWriter, r *http.Request) {
	author, err := s.store.GetAuthorBySlug(r.Context(), urlSlug(r, "slug"))
	if err != nil {
		respondStoreError(w, err)
		return
	}
	limit, offset := paginate(r, 24, 100)
	works, err := s.store.Queries().ListWorksForAuthor(r.Context(), db.ListWorksForAuthorParams{
		AuthorID: author.ID, Lim: limit, Off: offset,
	})
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"works": works, "author": author})
}

func (s *Server) handleListSubjects(w http.ResponseWriter, r *http.Request) {
	limit, _ := paginate(r, 40, 200)
	subjects, err := s.store.Queries().ListPopularSubjects(r.Context(), limit)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"subjects": subjects})
}

// ---- reviews -----------------------------------------------------------------------------

func (s *Server) handleListReviews(w http.ResponseWriter, r *http.Request) {
	work, err := s.store.GetWorkBySlug(r.Context(), urlSlug(r, "slug"))
	if err != nil {
		respondStoreError(w, err)
		return
	}
	limit, offset := paginate(r, 20, 50)
	reviews, err := s.store.ListReviewsForWork(r.Context(), work.ID,
		orSort(r.URL.Query().Get("sort"), "recent"), limit, offset)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	spoilerSafe := maskSpoilers(r, reviews)
	respondJSON(w, http.StatusOK, map[string]any{"reviews": spoilerSafe})
}

// maskSpoilers enforces the platform's spoiler contract: a spoiler-tagged
// review's body is withheld until the client explicitly asks for it with
// ?reveal_spoilers=true, and the reader's own progress is checked when they
// are signed in. Blur-and-tap is a UI affordance; withholding the bytes is the
// actual protection.
func maskSpoilers(r *http.Request, reviews []db.ListReviewsForWorkRow) []db.ListReviewsForWorkRow {
	reveal := r.URL.Query().Get("reveal_spoilers") == "true"
	if reveal {
		return reviews
	}
	out := make([]db.ListReviewsForWorkRow, len(reviews))
	copy(out, reviews)
	for i := range out {
		if out[i].HasSpoilers {
			out[i].Body = ""
			out[i].Title = "[spoilers hidden]"
		}
	}
	return out
}

type reviewRequest struct {
	Rating      float64    `json:"rating"` // stars, half-star increments
	Title       string     `json:"title"`
	Body        string     `json:"body"`
	PromptWhy   string     `json:"prompt_why"`
	HasSpoilers bool       `json:"has_spoilers"`
	EditionID   *uuid.UUID `json:"edition_id"`
}

func (s *Server) handleCreateReview(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	var req reviewRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	draft := domain.ReviewDraft{
		Rating: domain.Rating(req.Rating * 2), Title: req.Title, Body: req.Body,
		PromptWhy: req.PromptWhy, HasSpoilers: req.HasSpoilers,
	}
	if err := draft.Validate(s.friction); err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_review", err.Error())
		return
	}

	posted, err := s.store.DailyReviewCount(r.Context(), sess.UserID)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	if err := s.friction.CheckDailyLimit(int(sess.Reputation), posted); err != nil {
		respondError(w, http.StatusTooManyRequests, "daily_limit", err.Error())
		return
	}

	work, err := s.store.GetWorkBySlug(r.Context(), urlSlug(r, "slug"))
	if err != nil {
		respondStoreError(w, err)
		return
	}
	review, err := s.store.CreateReview(r.Context(), store.ReviewInput{
		UserID: sess.UserID, WorkID: work.ID, EditionID: req.EditionID,
		Rating: int32(draft.Rating), Title: req.Title, Body: req.Body,
		HasSpoilers: req.HasSpoilers, PromptWhy: req.PromptWhy,
	})
	if err != nil {
		// unique(user_id, work_id): one considered opinion per reader per work.
		respondError(w, http.StatusConflict, "already_reviewed",
			"You have already reviewed this work — edit your existing review instead.")
		return
	}
	s.enqueueSearchIndex(r, work.ID)
	respondJSON(w, http.StatusCreated, map[string]any{"review": review})
}

func (s *Server) handleGetReview(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_id", "Malformed review id.")
		return
	}
	review, err := s.store.GetReview(r.Context(), id)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"review": review})
}

func (s *Server) handleUpdateReview(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_id", "Malformed review id.")
		return
	}
	var req reviewRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	draft := domain.ReviewDraft{
		Rating: domain.Rating(req.Rating * 2), Title: req.Title, Body: req.Body,
		PromptWhy: req.PromptWhy, HasSpoilers: req.HasSpoilers,
	}
	if err := draft.Validate(s.friction); err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_review", err.Error())
		return
	}
	review, err := s.store.UpdateReview(r.Context(), id, sess.UserID, store.ReviewInput{
		UserID: sess.UserID, Rating: int32(draft.Rating), Title: req.Title, Body: req.Body,
		HasSpoilers: req.HasSpoilers, PromptWhy: req.PromptWhy, EditionID: req.EditionID,
	})
	if err != nil {
		respondStoreError(w, err)
		return
	}
	s.enqueueSearchIndex(r, review.WorkID)
	respondJSON(w, http.StatusOK, map[string]any{"review": review})
}

func (s *Server) handleDeleteReview(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_id", "Malformed review id.")
		return
	}
	review, err := s.store.GetReview(r.Context(), id)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	if err := s.store.DeleteReview(r.Context(), id, sess.UserID); err != nil {
		respondStoreError(w, err)
		return
	}
	s.enqueueSearchIndex(r, review.WorkID)
	respondJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

type likeRequest struct {
	Liked bool `json:"liked"`
}

func (s *Server) handleLikeReview(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_id", "Malformed review id.")
		return
	}
	var req likeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.store.SetReviewLike(r.Context(), id, sess.UserID, req.Liked); err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"liked": req.Liked})
}

type commentRequest struct {
	Body string `json:"body"`
}

func (s *Server) handleCreateComment(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_id", "Malformed review id.")
		return
	}
	var req commentRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := domain.ValidateComment(req.Body); err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_comment", err.Error())
		return
	}
	comment, err := s.store.CreateReviewComment(r.Context(), id, sess.UserID, req.Body)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{"comment": comment})
}

func (s *Server) handleListComments(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_id", "Malformed review id.")
		return
	}
	limit, offset := paginate(r, 50, 100)
	comments, err := s.store.ListReviewComments(r.Context(), id, limit, offset)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"comments": comments})
}

func (s *Server) handleDeleteComment(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_id", "Malformed comment id.")
		return
	}
	if err := s.store.DeleteReviewComment(r.Context(), id, sess.UserID); err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

// ---- scholar notes ---------------------------------------------------------------------------

func (s *Server) handleListScholarNotes(w http.ResponseWriter, r *http.Request) {
	work, err := s.store.GetWorkBySlug(r.Context(), urlSlug(r, "slug"))
	if err != nil {
		respondStoreError(w, err)
		return
	}
	limit, offset := paginate(r, 20, 50)
	var viewer *uuid.UUID
	if sess, ok := CurrentUser(r); ok {
		viewer = &sess.UserID
	}
	notes, err := s.store.Queries().ListNotesForWork(r.Context(), db.ListNotesForWorkParams{
		WorkID:     work.ID,
		ViewerID:   store.UUIDPtr(viewer),
		ChapterRef: strNil(r.URL.Query().Get("chapter")),
		Lim:        limit,
		Off:        offset,
	})
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"notes": notes})
}

func strNil(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// ---- related works ------------------------------------------------------------------------------

func (s *Server) handleRelatedWorks(w http.ResponseWriter, r *http.Request) {
	work, err := s.store.GetWorkBySlug(r.Context(), urlSlug(r, "slug"))
	if err != nil {
		respondStoreError(w, err)
		return
	}
	limit, _ := paginate(r, 6, 24)
	related, err := s.store.RelatedWorks(r.Context(), work.ID, 1, limit)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"related": related})
}

// enqueueSearchIndex publishes a derived-state event; the worker keeps the
// Meilisearch projection in step. Failures here must not fail the request: the
// index is rebuildable, the review is not.
func (s *Server) enqueueSearchIndex(r *http.Request, workID uuid.UUID) {
	// Detached from the request: the index update outlives the response, but a
	// cancelled client must not cancel the publication.
	ctx := context.WithoutCancel(r.Context())
	go func() {
		_ = s.store.PublishEvent(ctx, store.SubjectSearchIndex, map[string]any{
			"work_id": workID,
		})
	}()
}
