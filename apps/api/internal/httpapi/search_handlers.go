package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/alexandria-reads/alexandria/apps/api/internal/db"
	"github.com/alexandria-reads/alexandria/apps/api/internal/search"
	"github.com/alexandria-reads/alexandria/apps/api/internal/store"
)

// handleSearch is the discovery endpoint. Meilisearch serves it when healthy;
// when the index is unavailable the API degrades to a Postgres title-prefix
// scan so search never hard-fails. The fallback is slower and dumber, and the
// response says so, rather than returning an error page to a reader.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit, offset := paginate(r, 20, 50)

	query := search.Query{
		Text:         q,
		Index:        orIndex(r.URL.Query().Get("type")),
		Subjects:     splitCSV(r.URL.Query().Get("subjects")),
		Language:     r.URL.Query().Get("language"),
		PublicDomain: queryBool(r, "public_domain"),
		AuthorSlug:   r.URL.Query().Get("author"),
		Sort:         r.URL.Query().Get("sort"),
		Limit:        int(limit),
		Offset:       int(offset),
	}

	if s.search != nil && s.search.Enabled() {
		res, err := s.search.Search(r.Context(), query)
		if err == nil {
			respondJSON(w, http.StatusOK, map[string]any{
				"source":  "meilisearch",
				"query":   q,
				"hits":    res.Hits,
				"total":   res.Total,
				"took_ms": res.ProcessingTimeMs,
			})
			return
		}
		// Fall through to the degraded path; log for observability.
	}

	works, err := s.store.ListWorks(r.Context(), store.WorkFilters{
		TitlePrefix:  q,
		SubjectSlug:  firstOrEmpty(query.Subjects),
		Language:     query.Language,
		PublicDomain: query.PublicDomain,
		Sort:         "title",
		Limit:        limit,
		Offset:       offset,
	})
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"source":   "postgres_fallback",
		"query":    q,
		"hits":     works,
		"total":    len(works),
		"degraded": true,
	})
}

func orIndex(v string) string {
	switch v {
	case search.IndexAuthors, search.IndexClubs:
		return v
	}
	return search.IndexWorks
}

func splitCSV(v string) []string {
	if v == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i <= len(v); i++ {
		if i == len(v) || v[i] == ',' {
			if part := trimSpace(v[start:i]); part != "" {
				out = append(out, part)
			}
			start = i + 1
		}
	}
	return out
}

func firstOrEmpty(v []string) string {
	if len(v) == 0 {
		return ""
	}
	return v[0]
}

// ---- trust & safety -----------------------------------------------------------------------

type reportRequest struct {
	SubjectType string    `json:"subject_type"`
	SubjectID   uuid.UUID `json:"subject_id"`
	Reason      string    `json:"reason"`
	Details     string    `json:"details"`
}

func (s *Server) handleCreateReport(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	var req reportRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !validReportTarget(req.SubjectType) || !validReportReason(req.Reason) {
		respondError(w, http.StatusUnprocessableEntity, "invalid_report",
			"Unknown subject type or reason.")
		return
	}
	report, err := s.store.Queries().CreateReport(r.Context(), db.CreateReportParams{
		ReporterID:  store.UUIDPtr(&sess.UserID),
		SubjectType: req.SubjectType,
		SubjectID:   req.SubjectID,
		Reason:      db.ReportReason(req.Reason),
		Details:     req.Details,
	})
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{"report_id": report.ID})
}

// suspendExpiry converts a suspension length in hours to an absolute
// timestamp: suspensions expire on their own, so "temporary" cannot become a
// permanent ban through moderator forgetfulness.
func suspendExpiry(hours *int32) pgtype.Timestamptz {
	if hours == nil || *hours <= 0 {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: time.Now().Add(time.Duration(*hours) * time.Hour), Valid: true}
}

func validReportTarget(t string) bool {
	switch t {
	case "user", "review", "comment", "message", "note", "club", "cover":
		return true
	}
	return false
}

func validReportReason(v string) bool {
	switch db.ReportReason(v) {
	case db.ReportReasonSpam, db.ReportReasonAiSlop, db.ReportReasonHarassment,
		db.ReportReasonHate, db.ReportReasonThreat, db.ReportReasonSexualContent,
		db.ReportReasonMisinformation, db.ReportReasonCopyright,
		db.ReportReasonImpersonation, db.ReportReasonFakeReview, db.ReportReasonOther:
		return true
	}
	return false
}

func (s *Server) handleListReports(w http.ResponseWriter, r *http.Request) {
	limit, offset := paginate(r, 50, 200)
	rows, err := s.store.Queries().ListOpenReports(r.Context(), db.ListOpenReportsParams{
		Statuses: []db.ReportStatus{db.ReportStatusOpen, db.ReportStatusInReview},
		Lim:      limit,
		Off:      offset,
	})
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"reports": rows})
}

type moderationActionRequest struct {
	Kind         string `json:"kind"`
	Rationale    string `json:"rationale"`
	SuspendHours *int32 `json:"suspend_hours"`
}

func (s *Server) handleModerationAction(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	reportID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_id", "Malformed report id.")
		return
	}
	var req moderationActionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	// The schema requires a written rationale of >= 10 chars: every human
	// action is justifiable in prose, or it does not happen.
	if len([]rune(req.Rationale)) < 10 {
		respondError(w, http.StatusUnprocessableEntity, "rationale_required",
			"Moderation actions require a written rationale of at least 10 characters.")
		return
	}
	report, err := s.store.Queries().GetReport(r.Context(), reportID)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	action, err := s.store.Queries().RecordModerationAction(r.Context(), db.RecordModerationActionParams{
		ModeratorID: store.UUIDPtr(&sess.UserID),
		TargetUser:  store.UUIDPtr(&report.SubjectID),
		ReportID:    store.UUIDPtr(&reportID),
		Kind:        db.ModerationActionKind(req.Kind), Rationale: req.Rationale,
		ExpiresAt: suspendExpiry(req.SuspendHours),
	})
	if err != nil {
		respondStoreError(w, err)
		return
	}
	if req.Kind == string(db.ModerationActionKindTempSuspension) && req.SuspendHours != nil {
		if _, err := s.store.Queries().SuspendUser(r.Context(), db.SuspendUserParams{
			UserID: report.SubjectID, Until: suspendExpiry(req.SuspendHours),
		}); err != nil {
			respondStoreError(w, err)
			return
		}
	}
	if _, err := s.store.Queries().ResolveReport(r.Context(), db.ResolveReportParams{
		ID: reportID, Status: db.ReportStatusActioned,
	}); err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"action": action})
}
