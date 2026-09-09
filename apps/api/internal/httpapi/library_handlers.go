package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/alexandria-reads/alexandria/apps/api/internal/db"
	"github.com/alexandria-reads/alexandria/apps/api/internal/store"
)

// The library handlers are the RLS surface. Every store call passes the
// session's user id into TxUser/ReadUser, which sets app.user_id with SET
// LOCAL; Postgres then refuses any row that is not the caller's, no matter
// what the handler asked for.

func (s *Server) handleListShelves(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	shelves, err := s.store.Shelves(r.Context(), sess.UserID)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"shelves": shelves})
}

type createShelfRequest struct {
	Name      string `json:"name"`
	IsPrivate bool   `json:"is_private"`
}

func (s *Server) handleCreateShelf(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	var req createShelfRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.Name) < 1 || len(req.Name) > 64 {
		respondError(w, http.StatusUnprocessableEntity, "invalid_shelf", "Shelf names are 1–64 characters.")
		return
	}
	shelf, err := s.store.CreateCustomShelf(r.Context(), sess.UserID, req.Name, req.IsPrivate)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{"shelf": shelf})
}

func (s *Server) handleDeleteShelf(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_id", "Malformed shelf id.")
		return
	}
	if err := s.store.DeleteCustomShelf(r.Context(), sess.UserID, id); err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}

func (s *Server) handleLibrary(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	limit, offset := paginate(r, 24, 100)
	items, err := s.store.LibraryView(r.Context(), sess.UserID,
		r.URL.Query().Get("shelf"), limit, offset)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"items": items, "limit": limit, "offset": offset})
}

type addToLibraryRequest struct {
	WorkSlug  string     `json:"work_slug"`
	WorkID    *uuid.UUID `json:"work_id"`
	ShelfKind string     `json:"shelf_kind"`
	EditionID *uuid.UUID `json:"edition_id"`
	Format    string     `json:"format"`
}

func (s *Server) handleAddToLibrary(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	var req addToLibraryRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	workID := req.WorkID
	if workID == nil {
		work, err := s.store.GetWorkBySlug(r.Context(), req.WorkSlug)
		if err != nil {
			respondStoreError(w, err)
			return
		}
		workID = &work.ID
	}
	kind := db.ShelfKind(req.ShelfKind)
	switch kind {
	case db.ShelfKindWantToRead, db.ShelfKindReading, db.ShelfKindRead,
		db.ShelfKindDnf, db.ShelfKindFavorites:
	default:
		respondError(w, http.StatusUnprocessableEntity, "invalid_shelf_kind",
			"shelf_kind must be one of: want_to_read, reading, read, dnf, favorites")
		return
	}
	if err := s.store.MoveToShelf(r.Context(), sess.UserID, store.ShelfMove{
		WorkID: *workID, EditionID: req.EditionID, Format: req.Format, To: kind,
	}); err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "shelved", "shelf_kind": kind})
}

func (s *Server) handleRemoveFromLibrary(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	work, err := s.store.GetWorkBySlug(r.Context(), urlSlug(r, "slug"))
	if err != nil {
		respondStoreError(w, err)
		return
	}
	if err := s.store.RemoveFromLibrary(r.Context(), sess.UserID, work.ID); err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "removed"})
}

type progressRequest struct {
	WorkSlug   string     `json:"work_slug"`
	EditionID  *uuid.UUID `json:"edition_id"`
	Format     string     `json:"format"`
	ProgressBP int32      `json:"progress_bp"` // 0..10000
}

func (s *Server) handleSetProgress(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	var req progressRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	work, err := s.store.GetWorkBySlug(r.Context(), req.WorkSlug)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	session, err := s.store.SetProgress(r.Context(), sess.UserID, work.ID,
		req.EditionID, req.Format, req.ProgressBP)
	if err != nil {
		if err.Error() == "progress must be 0..10000 basis points, got "+itoa(int(req.ProgressBP)) {
			respondError(w, http.StatusUnprocessableEntity, "invalid_progress", err.Error())
			return
		}
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"session": session})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

type workOnlyRequest struct {
	WorkSlug string `json:"work_slug"`
}

func (s *Server) handleFinishBook(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	var req workOnlyRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	work, err := s.store.GetWorkBySlug(r.Context(), req.WorkSlug)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	if err := s.store.MarkFinished(r.Context(), sess.UserID, work.ID, nil); err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "finished"})
}

func (s *Server) handleDNF(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	var req workOnlyRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	work, err := s.store.GetWorkBySlug(r.Context(), req.WorkSlug)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	if err := s.store.MarkDNF(r.Context(), sess.UserID, work.ID, nil); err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "dnf"})
}

func (s *Server) handleCurrentlyReading(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	rows, err := s.store.CurrentlyReading(r.Context(), sess.UserID, 8)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"currently_reading": rows})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	stats, err := s.store.ReadingStats(r.Context(), sess.UserID)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	counts, err := s.store.LibraryCounts(r.Context(), sess.UserID)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	life, err := s.store.ReadingLife(r.Context(), sess.UserID)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"stats": stats, "shelves": counts,
		"year": life.Year, "streak_days": life.StreakDays, "streak_cells": life.StreakCells,
	})
}

// ---- annotations (reader highlights & notes) -------------------------------------------------

type annotationRequest struct {
	EditionID  uuid.UUID `json:"edition_id"`
	ChapterIdx int32     `json:"chapter_idx"`
	StartOff   int32     `json:"start_off"`
	EndOff     int32     `json:"end_off"`
	Kind       string    `json:"kind"` // highlight | note | bookmark
	Body       string    `json:"body"`
}

func (s *Server) handleCreateAnnotation(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	var req annotationRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.EndOff <= req.StartOff || req.StartOff < 0 {
		respondError(w, http.StatusUnprocessableEntity, "invalid_range", "Annotation offsets must satisfy 0 <= start < end.")
		return
	}
	kind := req.Kind
	switch kind {
	case "highlight", "note", "bookmark":
	default:
		kind = "highlight"
	}
	ann, err := s.store.CreateAnnotation(r.Context(), db.CreateAnnotationParams{
		UserID: sess.UserID, EditionID: req.EditionID, ChapterIdx: req.ChapterIdx,
		StartOff: req.StartOff, EndOff: req.EndOff, Kind: kind, Body: req.Body,
		IsPrivate: true, // annotations are private by default; sharing is opt-in later
	})
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{"annotation": ann})
}

func (s *Server) handleListAnnotations(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	limit, offset := paginate(r, 50, 200)
	// Without an edition filter the page shows the reader's whole margin:
	// every note across every edition, RLS-scoped to them alone.
	if raw := r.URL.Query().Get("edition_id"); raw == "" {
		anns, err := s.store.Queries().ListAnnotationsForUser(r.Context(), db.ListAnnotationsForUserParams{
			UserID: sess.UserID, Lim: limit, Off: offset,
		})
		if err != nil {
			respondStoreError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]any{"annotations": anns})
		return
	}
	edition, err := uuid.Parse(r.URL.Query().Get("edition_id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_id", "Malformed edition_id.")
		return
	}
	anns, err := s.store.ListAnnotations(r.Context(), sess.UserID, edition, queryInt32(r, "chapter_idx"))
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"annotations": anns})
}

func (s *Server) handleDeleteAnnotation(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_id", "Malformed annotation id.")
		return
	}
	if err := s.store.DeleteAnnotation(r.Context(), sess.UserID, id); err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
}
