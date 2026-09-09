package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/alexandria-reads/alexandria/apps/api/internal/auth"
	"github.com/alexandria-reads/alexandria/apps/api/internal/store"
)

// ---- responses --------------------------------------------------------------------
// Every error in the API has one shape, so clients (web, Flutter) can render
// humane messages without string-matching:
//
//	{"error": {"code": "invalid_review", "message": "review body is below …"}}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func respondError(w http.ResponseWriter, status int, code, msg string) {
	respondJSON(w, status, map[string]apiError{"error": {Code: code, Message: msg}})
}

// respondStoreError maps storage failures to honest HTTP statuses. A missed
// lookup is a 404; a friction or policy violation is a 4xx with its own code;
// anything else is a 500 that never leaks internals.
func respondStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		respondError(w, http.StatusNotFound, "not_found", "No such record in the library.")
	case errors.Is(err, store.ErrSelfAction):
		respondError(w, http.StatusUnprocessableEntity, "self_action", "You cannot target your own account with this action.")
	case errors.Is(err, store.ErrBlocked):
		respondError(w, http.StatusForbidden, "blocked", "This interaction is not permitted.")
	case errors.Is(err, auth.ErrThrottled):
		respondError(w, http.StatusTooManyRequests, "throttled", "Too many attempts — Alexandria favors patience.")
	case errors.Is(err, auth.ErrInvalidToken):
		respondError(w, http.StatusUnauthorized, "invalid_ceremony", "That sign-in attempt is no longer valid. Please try again.")
	case errors.Is(err, auth.ErrNoCredentials):
		respondError(w, http.StatusUnprocessableEntity, "no_passkeys", "This account has no passkey yet — use the email link, then add one.")
	case errors.Is(err, auth.ErrCSRF):
		respondError(w, http.StatusForbidden, "csrf", "Your session could not be verified for this change. Refresh and try again.")
	default:
		respondError(w, http.StatusInternalServerError, "internal", "The archive is momentarily unreachable.")
	}
}

// ---- request parsing ------------------------------------------------------------------

func decodeJSON(w http.ResponseWriter, r *http.Request, into any) bool {
	defer r.Body.Close() //nolint:errcheck
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields() // typos in client payloads fail loudly, not silently
	if err := dec.Decode(into); err != nil {
		respondError(w, http.StatusBadRequest, "bad_json", "Malformed request body.")
		return false
	}
	return true
}

// paginate reads limit/offset with hard caps. Offset pagination is fine for
// bounded catalogues; unbounded streams (feed, chat) use keyset cursors.
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

func queryBool(r *http.Request, key string) *bool {
	v := r.URL.Query().Get(key)
	if v == "" {
		return nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return nil
	}
	return &b
}

func queryInt32(r *http.Request, key string) *int32 {
	v := r.URL.Query().Get(key)
	if v == "" {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < -100000 || n > 100000 {
		return nil
	}
	i := int32(n)
	return &i
}

// tsParam reads an RFC3339 cursor into the nullable timestamp shape sqlc
// generated for keyset pagination.
func tsParam(r *http.Request, key string) pgtype.Timestamptz {
	return store.TSPtr(queryTime(r, key))
}

func queryTime(r *http.Request, key string) *time.Time {
	v := r.URL.Query().Get(key)
	if v == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return nil
	}
	return &t
}

// urlSlug is the single place URL parameters are read, so slug handling
// (lower-casing, trimming) cannot drift between handlers.
func urlSlug(r *http.Request, param string) string {
	return chi.URLParam(r, param)
}
