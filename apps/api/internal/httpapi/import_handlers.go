package httpapi

import (
	"io"
	"net/http"

	"github.com/alexandria-reads/alexandria/apps/api/internal/auth"
	"github.com/alexandria-reads/alexandria/apps/api/internal/importx"
	"github.com/alexandria-reads/alexandria/apps/api/internal/store"
)

// Import handlers. Bodies are the reader's own export files, posted raw; the
// parse is pure, the apply is transactional, and the report tells the truth
// about every row.

const maxImportBytes = 32 << 20

func (s *Server) readImportBody(w http.ResponseWriter, r *http.Request) (string, bool) {
	defer r.Body.Close() //nolint:errcheck
	b, err := io.ReadAll(io.LimitReader(r.Body, maxImportBytes))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_body", "Could not read the upload.")
		return "", false
	}
	if len(b) == 0 {
		respondError(w, http.StatusBadRequest, "empty_body", "Paste or upload your export first.")
		return "", false
	}
	return string(b), true
}

// importGuard applies the throttle and returns the report-writing closure's
// precondition: a slot in this hour's window.
func (s *Server) importGuard(w http.ResponseWriter, r *http.Request) bool {
	sess, _ := CurrentUser(r)
	bucket := auth.BucketKey([]byte(s.cfg.SessionPepper), "import", sess.UserID.String())
	ok, err := s.store.TakeImportSlot(r.Context(), bucket)
	if err != nil {
		respondStoreError(w, err)
		return false
	}
	if !ok {
		respondError(w, http.StatusTooManyRequests, "import_limit",
			"Three imports per hour keep history imports deliberate. Try again shortly.")
		return false
	}
	return true
}

func (s *Server) handleImportGoodreads(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	body, ok := s.readImportBody(w, r)
	if !ok {
		return
	}
	plan, err := importx.ParseGoodreads(stringReader(body))
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, "bad_csv", err.Error())
		return
	}
	if !s.importGuard(w, r) {
		return
	}
	rep, err := s.store.ApplyImport(r.Context(), sess.UserID, plan)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	rep.Skips = append(rep.Skips, plan.Skips...)
	respondJSON(w, http.StatusOK, map[string]any{"report": rep, "plan_summary": summary(plan)})
}

func (s *Server) handleImportStoryGraph(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	body, ok := s.readImportBody(w, r)
	if !ok {
		return
	}
	plan, err := importx.ParseStoryGraph(stringReader(body))
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, "bad_csv", err.Error())
		return
	}
	if !s.importGuard(w, r) {
		return
	}
	rep, err := s.store.ApplyImport(r.Context(), sess.UserID, plan)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	rep.Skips = append(rep.Skips, plan.Skips...)
	respondJSON(w, http.StatusOK, map[string]any{"report": rep, "plan_summary": summary(plan)})
}

// handleImportKindle aligns clippings against cached public-domain texts
// before writing: a highlight without a real anchor is reported, not guessed.
func (s *Server) handleImportKindle(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	body, ok := s.readImportBody(w, r)
	if !ok {
		return
	}
	plan := importx.ParseKindleClippings(body)
	if !s.importGuard(w, r) {
		return
	}
	rep := &store.ImportReport{}
	var aligned []store.AlignedHighlight
	for i, h := range plan.Highlights {
		work, err := s.store.MatchWork(r.Context(), importx.MatchKey{Title: h.BookTitle, Author: h.Author})
		if err != nil {
			rep.Skips = append(rep.Skips, importx.Skip{Row: i, Reason: "no_matching_work", Detail: h.BookTitle})
			continue
		}
		editions, err := s.store.Queries().ListEditionsForWork(r.Context(), work.ID)
		if err != nil || len(editions) == 0 || editions[0].GutenbergID == nil {
			rep.Skips = append(rep.Skips, importx.Skip{Row: i, Reason: "no_cached_text", Detail: h.BookTitle})
			continue
		}
		ed, err := s.cachedEdition(*editions[0].GutenbergID, editions[0].Language)
		if err != nil {
			rep.Skips = append(rep.Skips, importx.Skip{Row: i, Reason: "text_unavailable", Detail: h.BookTitle})
			continue
		}
		chapters := make([]importx.Chapter, 0, len(ed.Chapters))
		for _, c := range ed.Chapters {
			text, ok := ed.ChapterText(c.Index)
			if !ok {
				continue
			}
			chapters = append(chapters, importx.Chapter{Index: c.Index, Text: text})
		}
		a, ok := importx.Align(chapters, h.Passage)
		if !ok {
			rep.Skips = append(rep.Skips, importx.Skip{Row: i, Reason: "not_aligned", Detail: firstN(h.Passage, 48)})
			continue
		}
		aligned = append(aligned, store.AlignedHighlight{
			EditionID: editions[0].ID, ChapterIdx: int32(a.ChapterIndex),
			Start: int32(a.Start), End: int32(a.End), Passage: h.Passage, Kind: h.Kind,
		})
	}
	n, err := s.store.ApplyHighlights(r.Context(), sess.UserID, aligned, "kindle")
	if err != nil {
		respondStoreError(w, err)
		return
	}
	rep.HighlightsAligned = n
	rep.Skips = append(rep.Skips, plan.Skips...)
	respondJSON(w, http.StatusOK, map[string]any{"report": rep})
}

// ---- export & account ---------------------------------------------------------------

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	bundle, err := s.store.Export(r.Context(), sess.UserID)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="alexandria-export.json"`)
	respondJSON(w, http.StatusOK, bundle)
}

func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	if err := s.store.DeleteAccount(r.Context(), sess.UserID); err != nil {
		respondStoreError(w, err)
		return
	}
	auth.ClearSessionCookie(w, s.authSvc.Cookies())
	auth.ClearCSRFCookie(w, s.authSvc.Cookies())
	respondJSON(w, http.StatusOK, map[string]any{
		"status": "deleted",
		"note":   "Your account is gone from the library; a scheduled purge removes the rows after the appeal window.",
	})
}

func summary(p *importx.Plan) map[string]int {
	return map[string]int{
		"shelves":    len(p.Shelves),
		"reviews":    len(p.Reviews),
		"highlights": len(p.Highlights),
		"skipped":    len(p.Skips),
	}
}

func firstN(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
