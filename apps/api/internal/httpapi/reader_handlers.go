package httpapi

import (
	"net/http"
	"strconv"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/alexandria-reads/alexandria/apps/api/internal/db"
	"github.com/alexandria-reads/alexandria/apps/api/internal/reader"
)

// The reader surface. Texts are fetched once into an immutable cache and
// chapterized once per process; the API serves rune-slices, and annotations
// ride along when the caller is signed in (RLS-scoped, as always).

var (
	editionCacheMu sync.Mutex
	editionCache   = map[int32]*reader.Edition{}
)

func (s *Server) cachedEdition(id int32, language string) (*reader.Edition, error) {
	editionCacheMu.Lock()
	defer editionCacheMu.Unlock()
	if e, ok := editionCache[id]; ok {
		return e, nil
	}
	raw, err := s.textCache.Text(s.bootCtx, id)
	if err != nil {
		return nil, err
	}
	e := reader.Build(id, language, raw)
	editionCache[id] = e
	return e, nil
}

type readerMetaChapter struct {
	Index int    `json:"index"`
	Title string `json:"title"`
}

func (s *Server) handleReaderMeta(w http.ResponseWriter, r *http.Request) {
	edition, ed, ok := s.readerEdition(w, r)
	if !ok {
		return
	}
	chapters := make([]readerMetaChapter, 0, len(ed.Chapters))
	for _, c := range ed.Chapters {
		chapters = append(chapters, readerMetaChapter{Index: c.Index, Title: c.Title})
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"edition_id":     edition.ID,
		"work_slug":      edition.WorkSlug,
		"work_title":     edition.WorkTitle,
		"title":          ed.Title,
		"language":       ed.Language,
		"license_note":   ed.LicenseNote,
		"trademark_note": ed.TrademarkNote,
		"chapters":       chapters,
	})
}

func (s *Server) handleReaderChapter(w http.ResponseWriter, r *http.Request) {
	edition, ed, ok := s.readerEdition(w, r)
	if !ok {
		return
	}
	idx, err := strconv.Atoi(chi.URLParam(r, "idx"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_chapter", "Malformed chapter index.")
		return
	}
	text, ok2 := ed.ChapterText(idx)
	if !ok2 {
		respondError(w, http.StatusNotFound, "no_such_chapter", "This edition has no such chapter.")
		return
	}
	resp := map[string]any{
		"index": idx,
		"text":  text,
	}
	// The reader's own margin for this chapter, when signed in. RLS makes this
	// incapable of returning anyone else's notes.
	if sess, ok3 := CurrentUser(r); ok3 {
		anns, err := s.store.Queries().ListAnnotationsForEdition(r.Context(), db.ListAnnotationsForEditionParams{
			UserID: sess.UserID, EditionID: edition.ID,
			ChapterIdx: int32Ptr(idx),
		})
		if err == nil {
			resp["annotations"] = anns
		}
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) readerEdition(w http.ResponseWriter, r *http.Request) (*db.GetEditionByIDRow, *reader.Edition, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_id", "Malformed edition id.")
		return nil, nil, false
	}
	edition, err := s.store.GetEditionByID(r.Context(), id)
	if err != nil {
		respondStoreError(w, err)
		return nil, nil, false
	}
	if edition.GutenbergID == nil {
		respondError(w, http.StatusNotFound, "not_readable",
			"This edition has no public-domain text in the cache yet.")
		return nil, nil, false
	}
	ed, err := s.cachedEdition(*edition.GutenbergID, edition.Language)
	if err != nil {
		respondError(w, http.StatusBadGateway, "text_unavailable",
			"The text cache could not reach Project Gutenberg. Try again shortly.")
		return nil, nil, false
	}
	return edition, ed, true
}

func int32Ptr(i int) *int32 { v := int32(i); return &v }
