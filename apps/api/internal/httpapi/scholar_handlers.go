package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/alexandria-reads/alexandria/apps/api/internal/db"
	"github.com/alexandria-reads/alexandria/apps/api/internal/domain"
	"github.com/alexandria-reads/alexandria/apps/api/internal/store"
)

// The scholar surface: application, submission, peer review, retraction, and
// the moderator's verification act. Provenance is always visible: verified
// scholars and community contributors are labelled differently, and revision
// history travels with every note.

type noteRequest struct {
	Title       string `json:"title"`
	Body        string `json:"body"`
	Kind        string `json:"kind"`
	ChapterRef  string `json:"chapter_ref"`
	AnchorQuote string `json:"anchor_quote"`
	Citations   []struct {
		Citation string `json:"citation"`
		URL      string `json:"url"`
	} `json:"citations"`
	Submit bool `json:"submit"`
}

func (r noteRequest) draft() domain.NoteDraft {
	d := domain.NoteDraft{
		Title: r.Title, Body: r.Body, Kind: r.Kind,
		ChapterRef: r.ChapterRef, AnchorQuote: r.AnchorQuote,
	}
	for _, c := range r.Citations {
		d.Citations = append(d.Citations, domain.CitationDraft{Citation: c.Citation, URL: c.URL})
	}
	return d
}

func (s *Server) handleCreateNote(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	var req noteRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	draft := req.draft()
	if err := draft.Validate(); err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_note", err.Error())
		return
	}
	work, err := s.store.GetWorkBySlug(r.Context(), urlSlug(r, "slug"))
	if err != nil {
		respondStoreError(w, err)
		return
	}
	note, err := s.store.CreateNote(r.Context(), sess.UserID, work.ID, draft, req.Submit)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{"note": note})
}

func (s *Server) handleGetNote(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_id", "Malformed note id.")
		return
	}
	note, err := s.store.GetNote(r.Context(), id)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	// Drafts and notes in review are visible to their author and to reviewers;
	// published and retracted notes are public record.
	if note.Status == db.NoteStatusDraft || note.Status == db.NoteStatusInReview {
		allowed := false
		if sess, ok := CurrentUser(r); ok {
			allowed = sess.UserID == note.AuthorID || sess.IsModerator()
			if !allowed && note.Status == db.NoteStatusInReview {
				if v, verr := s.store.IsVerifiedScholar(r.Context(), sess.UserID); verr == nil && v {
					allowed = true
				}
			}
		}
		if !allowed {
			respondStoreError(w, store.ErrNotFound)
			return
		}
	}
	citations, _ := s.store.NoteCitations(r.Context(), id)
	revisions, _ := s.store.NoteRevisions(r.Context(), id)
	reviews, _ := s.store.NoteReviews(r.Context(), id)
	respondJSON(w, http.StatusOK, map[string]any{
		"note": note, "citations": citations, "revisions": revisions, "reviews": reviews,
	})
}

func (s *Server) handleUpdateNote(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_id", "Malformed note id.")
		return
	}
	var req noteRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	draft := req.draft()
	if err := draft.Validate(); err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_note", err.Error())
		return
	}
	note, err := s.store.ReviseNote(r.Context(), id, sess.UserID, draft)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"note": note})
}

func (s *Server) handleSubmitNote(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_id", "Malformed note id.")
		return
	}
	if err := s.store.SubmitNote(r.Context(), id, sess.UserID); err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": string(db.NoteStatusInReview)})
}

// noteReviewRequest is a peer-review judgement; distinct from the reader
// reviewRequest in content_handlers.go, which is a rating of a book.
type noteReviewRequest struct {
	Approved bool   `json:"approved"`
	Comments string `json:"comments"`
}

func (s *Server) handleReviewNote(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_id", "Malformed note id.")
		return
	}
	var req noteReviewRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	note, err := s.store.ReviewNote(r.Context(), id, sess.UserID, req.Approved, req.Comments)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	// The author hears the verdict, addressed to them alone.
	_ = s.store.Notify(r.Context(), note.AuthorID, "note_reviewed", map[string]any{
		"approved": req.Approved, "status": string(note.Status),
		"note_id": id.String(), "reviewer": sess.Username,
	})
	respondJSON(w, http.StatusOK, map[string]any{"note": note, "status": note.Status})
}

func (s *Server) handleRetractNote(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_id", "Malformed note id.")
		return
	}
	if err := s.store.RetractNote(r.Context(), id, sess.UserID, sess.IsModerator()); err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": string(db.NoteStatusRetracted)})
}

// ---- profiles & queues -----------------------------------------------------------------

type scholarApplyRequest struct {
	Field        string `json:"field"`
	Affiliation  string `json:"affiliation"`
	Orcid        string `json:"orcid"`
	CoiStatement string `json:"coi_statement"`
}

func (s *Server) handleScholarApply(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	var req scholarApplyRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	draft := domain.ProfileDraft{
		Field: req.Field, Affiliation: req.Affiliation,
		Orcid: req.Orcid, CoiStatement: req.CoiStatement,
	}
	if err := draft.Validate(); err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_profile", err.Error())
		return
	}
	prof, err := s.store.ApplyScholarProfile(r.Context(), sess.UserID, draft)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{"profile": prof})
}

func (s *Server) handleScholarProfile(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	prof, err := s.store.ScholarProfile(r.Context(), sess.UserID)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"profile": prof})
}

func (s *Server) handleScholarQueue(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	verified, err := s.store.IsVerifiedScholar(r.Context(), sess.UserID)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	if !verified && !sess.IsModerator() {
		respondError(w, http.StatusForbidden, "forbidden", "The review queue is for verified scholars.")
		return
	}
	limit, offset := paginate(r, 20, 50)
	rows, err := s.store.NotesAwaitingReview(r.Context(), limit, offset)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"queue": rows})
}

func (s *Server) handlePendingScholars(w http.ResponseWriter, r *http.Request) {
	limit, offset := paginate(r, 20, 50)
	rows, err := s.store.PendingScholarProfiles(r.Context(), limit, offset)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"pending": rows})
}

type scholarStatusRequest struct {
	Status string `json:"status"`
}

func (s *Server) handleSetScholarStatus(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_id", "Malformed user id.")
		return
	}
	var req scholarStatusRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	status := db.ScholarStatus(req.Status)
	switch status {
	case db.ScholarStatusVerified, db.ScholarStatusRejected, db.ScholarStatusRevoked:
	default:
		respondError(w, http.StatusUnprocessableEntity, "invalid_status",
			"status must be verified, rejected or revoked")
		return
	}
	if err := s.store.SetScholarStatus(r.Context(), sess.UserID, userID, status); err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": string(status)})
}
