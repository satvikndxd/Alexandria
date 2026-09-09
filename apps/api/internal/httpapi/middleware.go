package httpapi

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/alexandria-reads/alexandria/apps/api/internal/auth"
	"github.com/alexandria-reads/alexandria/apps/api/internal/db"
	"github.com/alexandria-reads/alexandria/apps/api/internal/store"
)

type ctxKey int

const (
	ctxKeySession ctxKey = iota
)

// Session is the authenticated principal resolved from the cookie. Handlers
// read it via CurrentUser; nothing else in the system trusts a client-supplied
// identity, which is what allows the old "X-Alexandria-Dev-User" header shim to
// be deleted entirely.
type Session struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	Username      string
	Email         string
	Role          db.UserRole
	Reputation    int32
	EmailVerified bool
	Token         string
	TokenHash     []byte
	ExpiresAt     time.Time
}

func (s *Session) IsModerator() bool {
	return s.Role == db.UserRoleModerator || s.Role == db.UserRoleAdmin
}

func (s *Session) IsAdmin() bool { return s.Role == db.UserRoleAdmin }

// resolveSession hydrates the principal on every request. It is deliberately
// cheap: one indexed lookup by token hash. Sessions slide forward at most once
// per SessionSlideAfter so an active reader is never logged out, while an
// abandoned cookie still expires on schedule.
func (s *Server) resolveSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := auth.SessionCookie(r)
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		hash := auth.HashToken(token)
		row, err := s.store.ActiveSession(r.Context(), hash)
		if err != nil {
			// Expired, revoked, or unknown: proceed unauthenticated. The cookie
			// is cleared so clients stop sending a dead token.
			auth.ClearSessionCookie(w, s.authSvc.Cookies())
			next.ServeHTTP(w, r)
			return
		}
		if store.SessionBlocked(row) {
			auth.ClearSessionCookie(w, s.authSvc.Cookies())
			next.ServeHTTP(w, r)
			return
		}
		sess := &Session{
			ID: row.ID, UserID: row.UserID, Username: row.Username, Email: row.Email,
			Role: row.Role, Reputation: row.Reputation, EmailVerified: row.EmailVerified,
			Token: token, TokenHash: hash, ExpiresAt: row.ExpiresAt.Time,
		}
		if time.Until(sess.ExpiresAt) < auth.SessionTTL-auth.SessionSlideAfter {
			// Slide. Fire-and-forget: a failure here must not fail the request.
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				_ = s.store.ExtendSession(ctx, sess.ID, auth.SessionTTL)
			}()
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKeySession, sess)))
	})
}

// CurrentUser returns the authenticated principal, if any.
func CurrentUser(r *http.Request) (*Session, bool) {
	sess, ok := r.Context().Value(ctxKeySession).(*Session)
	return sess, ok && sess != nil
}

// requireAuth wraps handlers that need a principal.
func requireAuth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := CurrentUser(r); !ok {
			respondError(w, http.StatusUnauthorized, "unauthenticated",
				"Sign in to do that — Alexandria keeps its shelves personal.")
			return
		}
		h(w, r)
	}
}

// requireModeration gates the human-moderation surfaces.
func requireModeration(h http.HandlerFunc) http.HandlerFunc {
	return requireAuth(func(w http.ResponseWriter, r *http.Request) {
		sess, _ := CurrentUser(r)
		if !sess.IsModerator() {
			respondError(w, http.StatusForbidden, "forbidden", "Moderator access required.")
			return
		}
		h(w, r)
	})
}

// csrfProtect validates the double-submit token on state-changing methods for
// authenticated sessions. Safe methods are exempt by definition.
func (s *Server) csrfProtect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete:
			if _, ok := CurrentUser(r); ok {
				if err := auth.CheckCSRF(r, []byte(s.cfg.SessionPepper)); err != nil {
					respondStoreError(w, err)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// scopedRead runs a read-only transaction in the caller's Row-Level-Security
// context when authenticated, and in the service context otherwise. Queries
// that touch RLS-covered tables (reading progress, shelves) MUST go through
// here: outside a scoped transaction the reader's own progress is invisible to
// Postgres, which would silently gate every reader out of every gated channel.
func (s *Server) scopedRead(r *http.Request, fn func(q *db.Queries) error) error {
	if sess, ok := CurrentUser(r); ok {
		return s.store.ReadUser(r.Context(), sess.UserID, fn)
	}
	return s.store.Tx(r.Context(), fn)
}

// clientIP extracts the peer address for throttling buckets and salted IP
// hashes. X-Forwarded-For is only trusted when a proxy is explicitly
// configured, because otherwise any client can spoof its bucket and defeat
// rate limiting.
func (s *Server) clientIP(r *http.Request) string {
	if s.cfg.TrustProxy && r.Header.Get("X-Forwarded-For") != "" {
		// left-most address added by our own edge
		host := r.Header.Get("X-Forwarded-For")
		for i := 0; i < len(host); i++ {
			if host[i] == ',' {
				host = host[:i]
				break
			}
		}
		return net.ParseIP(trimSpace(host)).String()
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
