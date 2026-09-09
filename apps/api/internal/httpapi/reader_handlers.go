package httpapi

import (
	"io"
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
		"source":         orDefault(ed.Source, "gutenberg"),
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
	// The reader's own margin for this chapter, when signed in. Annotations
	// are RLS-private, so this MUST run inside the caller's scoped
	// transaction — outside it, Postgres hides the reader's own notes too.
	if sess, ok3 := CurrentUser(r); ok3 {
		var anns []db.Annotation
		if err := s.scopedRead(r, func(q *db.Queries) error {
			rows, err := q.ListAnnotationsForEdition(r.Context(), db.ListAnnotationsForEditionParams{
				UserID: sess.UserID, EditionID: edition.ID,
				ChapterIdx: int32Ptr(idx),
			})
			anns = rows
			return err
		}); err == nil {
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
	// An uploaded, rights-recorded EPUB takes precedence over the Gutenberg
	// text: it is the edition a moderator actually approved.
	if s.textCache.HasEpub(edition.ID) {
		book, err := s.textCache.EpubBook(r.Context(), edition.ID)
		if err == nil {
			return edition, reader.EditionFromEpub(edition.ID, edition.Language, book), true
		}
		// Fall through to Gutenberg rather than stranding the reader.
	}
	if edition.GutenbergID == nil {
		respondError(w, http.StatusNotFound, "not_readable",
			"This edition has no readable text in the cache yet.")
		return nil, nil, false
	}
	ed, err := s.cachedEdition(*edition.GutenbergID, edition.Language)
	if err != nil {
		respondError(w, http.StatusBadGateway, "text_unavailable",
			"The text cache could not reach Project Gutenberg. Try again shortly.")
		return nil, nil, false
	}
	ed.Source = "gutenberg"
	return edition, ed, true
}

// handleUploadEpub stores a rights-checked EPUB container for an edition.
// Moderator-only: any signed-in reader uploading arbitrary containers is a
// copyright pipeline, not a library.
func (s *Server) handleUploadEpub(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_id", "Malformed edition id.")
		return
	}
	defer r.Body.Close() //nolint:errcheck
	data, err := io.ReadAll(io.LimitReader(r.Body, 64<<20))
	if err != nil || len(data) == 0 {
		respondError(w, http.StatusBadRequest, "bad_body", "Upload the EPUB file itself.")
		return
	}
	book, err := s.textCache.StoreEpub(id, data)
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, "bad_epub", err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{
		"status": "stored", "title": book.Title, "chapters": len(book.Chapters),
	})
}

func int32Ptr(i int) *int32 { v := int32(i); return &v }

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
