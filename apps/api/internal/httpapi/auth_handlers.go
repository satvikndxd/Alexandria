package httpapi

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/alexandria-reads/alexandria/apps/api/internal/auth"
	"github.com/alexandria-reads/alexandria/apps/api/internal/domain"
)

// ---- registration -----------------------------------------------------------------
// Registration is a two-step passkey ceremony: create the account, then let the
// authenticator mint the credential. The account exists between the steps (so
// a dropped tab does not strand a username) but has no session until a factor
// succeeds.

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := domain.ValidateUsername(req.Username); err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_username", err.Error())
		return
	}
	if err := domain.ValidateEmail(req.Email); err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_email", err.Error())
		return
	}
	username := domain.NormalizeUsername(req.Username)
	email := domain.NormalizeEmail(req.Email)

	taken, err := s.store.Queries().UsernameIsTaken(r.Context(), username)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	if taken {
		respondError(w, http.StatusConflict, "username_taken", "That username is already on the shelf.")
		return
	}
	taken, err = s.store.Queries().EmailIsTaken(r.Context(), email)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	if taken {
		// Deliberately vague: registration must not confirm which of the two
		// fields collided with an existing account.
		respondError(w, http.StatusConflict, "account_exists", "An account with those details already exists. Try signing in.")
		return
	}

	user, err := s.store.RegisterUser(r.Context(), username, email, nil)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	ceremony, err := s.authSvc.BeginRegistration(r.Context(), user.ID)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{
		"user": map[string]any{"id": user.ID, "username": user.Username},
		// The web client hands `ceremony.options` to navigator.credentials.create
		// and posts the result back with the ceremony id.
		"ceremony": ceremony,
		"next":     "POST /v1/auth/passkey/finish-registration with X-Ceremony-ID",
	})
}

const ceremonyHeader = "X-Ceremony-ID"

func (s *Server) handleFinishRegistration(w http.ResponseWriter, r *http.Request) {
	sess, ok := CurrentUser(r)
	var userID uuid.UUID
	if ok {
		userID = sess.UserID
	} else {
		// The registration ceremony is bound to the account created in the
		// previous step; the ceremony row carries that user id.
		ceremonyID, err := uuid.Parse(r.Header.Get(ceremonyHeader))
		if err != nil {
			respondError(w, http.StatusBadRequest, "bad_ceremony", "Missing or malformed ceremony id.")
			return
		}
		_ = ceremonyID
		respondError(w, http.StatusUnauthorized, "unauthenticated",
			"Restart registration — this ceremony is no longer attached to a session.")
		return
	}

	ceremonyID, err := uuid.Parse(r.Header.Get(ceremonyHeader))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_ceremony", "Missing or malformed ceremony id.")
		return
	}
	rr := &auth.RequestReader{HTTP: r, UserAgent: r.UserAgent(), IP: s.clientIP(r)}
	if err := s.authSvc.FinishRegistration(r.Context(), userID, ceremonyID, rr); err != nil {
		respondStoreError(w, err)
		return
	}
	s.startSession(w, r, userID)
	respondJSON(w, http.StatusOK, map[string]any{"status": "registered", "username": sessOrUsername(r, userID, s)})
}

// ---- login ---------------------------------------------------------------------------

type beginLoginRequest struct {
	Username string `json:"username"`
}

func (s *Server) handleBeginLogin(w http.ResponseWriter, r *http.Request) {
	var req beginLoginRequest
	// An empty body is legal: it starts a discoverable ("choose a passkey")
	// ceremony. decodeJSON would reject EOF, so parse leniently here.
	if r.ContentLength > 0 {
		if !decodeJSON(w, r, &req) {
			return
		}
	}
	ceremony, err := s.authSvc.BeginLogin(r.Context(), req.Username, s.clientIP(r))
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ceremony": ceremony})
}

func (s *Server) handleFinishLogin(w http.ResponseWriter, r *http.Request) {
	ceremonyID, err := uuid.Parse(r.Header.Get(ceremonyHeader))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_ceremony", "Missing or malformed ceremony id.")
		return
	}
	rr := &auth.RequestReader{HTTP: r, UserAgent: r.UserAgent(), IP: s.clientIP(r)}
	userID, _, err := s.authSvc.FinishLogin(r.Context(), ceremonyID, rr)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	s.startSession(w, r, userID)
	respondJSON(w, http.StatusOK, map[string]any{"status": "signed_in"})
}

// ---- magic links ------------------------------------------------------------------------

type magicLinkRequest struct {
	Email string `json:"email"`
}

func (s *Server) handleRequestMagicLink(w http.ResponseWriter, r *http.Request) {
	var req magicLinkRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	// Always 202, always the same body: the endpoint must not oracle mailbox
	// existence (see internal/auth.MagicLinks.Request).
	if err := s.magic.Request(r.Context(), req.Email); err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusAccepted, map[string]any{
		"status": "If that address has an account, a sign-in link is on its way.",
	})
}

type magicLinkVerifyRequest struct {
	Token string `json:"token"`
}

func (s *Server) handleVerifyMagicLink(w http.ResponseWriter, r *http.Request) {
	var req magicLinkVerifyRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	userID, err := s.magic.Verify(r.Context(), req.Token)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	s.startSession(w, r, userID)
	respondJSON(w, http.StatusOK, map[string]any{"status": "signed_in"})
}

// ---- session lifecycle ---------------------------------------------------------------------

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	sess, ok := CurrentUser(r)
	if ok {
		_ = s.store.RevokeSession(r.Context(), sess.TokenHash)
	}
	auth.ClearSessionCookie(w, s.authSvc.Cookies())
	auth.ClearCSRFCookie(w, s.authSvc.Cookies())
	respondJSON(w, http.StatusOK, map[string]any{"status": "signed_out"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	sess, ok := CurrentUser(r)
	if !ok {
		respondJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	profile, err := s.store.Queries().GetProfileByUserID(r.Context(), sess.UserID)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	counts, err := s.store.LibraryCounts(r.Context(), sess.UserID)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	unread, err := s.store.UnreadNotificationCount(r.Context(), sess.UserID)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"user": map[string]any{
			"id": sess.UserID, "username": sess.Username, "email": sess.Email,
			"role": sess.Role, "reputation": sess.Reputation,
			"email_verified": sess.EmailVerified,
			"display_name":   profile.DisplayName, "bio": profile.Bio,
			"is_private": profile.IsPrivate,
		},
		"library_counts":         counts,
		"unread_notifications":   unread,
		"csrf_token":             auth.CSRFToken([]byte(s.cfg.SessionPepper), sess.Token),
		"daily_review_allowance": s.friction.DailyReviewAllowance(int(sess.Reputation)),
	})
}

type updateProfileRequest struct {
	DisplayName string  `json:"display_name"`
	Bio         string  `json:"bio"`
	Pronouns    *string `json:"pronouns"`
	Location    *string `json:"location"`
	IsPrivate   bool    `json:"is_private"`
}

func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	sess, _ := CurrentUser(r)
	var req updateProfileRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := domain.ValidateDisplayName(req.DisplayName); err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_display_name", err.Error())
		return
	}
	if err := domain.ValidateBio(req.Bio); err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_bio", err.Error())
		return
	}
	profile, err := s.store.UpdateProfile(r.Context(), sess.UserID, req.DisplayName, req.Bio,
		req.Pronouns, req.Location, req.IsPrivate)
	if err != nil {
		respondStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"profile": profile})
}

// startSession mints a cookie session after any successful factor and pairs it
// with the CSRF double-submit cookie bound to it.
func (s *Server) startSession(w http.ResponseWriter, r *http.Request, userID uuid.UUID) {
	token, hash, err := auth.NewSessionToken()
	if err != nil {
		respondStoreError(w, err)
		return
	}
	ua := r.UserAgent()
	if _, err := s.store.CreateSession(r.Context(), userID, hash, &ua,
		[]byte(s.clientIP(r)), auth.SessionTTL); err != nil {
		respondStoreError(w, err)
		return
	}
	policy := s.authSvc.Cookies()
	auth.SetSessionCookie(w, token, policy)
	auth.IssueCSRFCookie(w, auth.CSRFToken([]byte(s.cfg.SessionPepper), token), policy)
}

func sessOrUsername(r *http.Request, id uuid.UUID, s *Server) string {
	if sess, ok := CurrentUser(r); ok {
		return sess.Username
	}
	return id.String()
}
