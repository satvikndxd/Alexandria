package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/alexandria-reads/alexandria/apps/api/internal/db"
	"github.com/alexandria-reads/alexandria/apps/api/internal/store"
)

// Service owns the WebAuthn ceremonies. Ceremonies are stateless across API
// nodes: the in-flight SessionData is persisted in auth_challenges, so the node
// that finishes a ceremony need not be the node that began it.
type Service struct {
	wa      *webauthn.WebAuthn
	st      *store.Store
	cookies CookiePolicy
}

func NewService(st *store.Store, rpID, rpDisplayName string, rpOrigins []string, secure bool) (*Service, error) {
	wa, err := webauthn.New(&webauthn.Config{
		RPID:          rpID,
		RPDisplayName: rpDisplayName,
		RPOrigins:     rpOrigins,
		// Platform and roaming authenticators both welcome; user verification
		// (biometric/PIN) required, which is what makes a passkey a real factor
		// rather than a possession check.
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			UserVerification: protocol.VerificationRequired,
			ResidentKey:      protocol.ResidentKeyRequirementPreferred,
		},
		AttestationPreference: protocol.PreferNoAttestation,
	})
	if err != nil {
		return nil, fmt.Errorf("webauthn config: %w", err)
	}
	return &Service{wa: wa, st: st, cookies: DefaultCookiePolicy(secure)}, nil
}

func (s *Service) Cookies() CookiePolicy { return s.cookies }

// ---- webauthn.User adapter --------------------------------------------------------

type passkeyUser struct {
	id    uuid.UUID
	name  string
	creds []webauthn.Credential
}

func (u passkeyUser) WebAuthnID() []byte          { return u.id[:] }
func (u passkeyUser) WebAuthnName() string        { return u.name }
func (u passkeyUser) WebAuthnDisplayName() string { return u.name }
func (u passkeyUser) WebAuthnCredentials() []webauthn.Credential {
	return u.creds
}

// loadUser hydrates the adapter, including every stored credential. go-webauthn
// builds exclusion lists (registration) and allowed-credential lists (login)
// from this slice, so it must be complete.
func (s *Service) loadUser(ctx context.Context, userID uuid.UUID) (passkeyUser, error) {
	user, err := s.st.Queries().GetUserByID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return passkeyUser{}, ErrInvalidToken
	}
	if err != nil {
		return passkeyUser{}, err
	}
	rows, err := s.st.Queries().ListWebAuthnCredentialsForUser(ctx, userID)
	if err != nil {
		return passkeyUser{}, err
	}
	creds := make([]webauthn.Credential, 0, len(rows))
	for _, r := range rows {
		creds = append(creds, credentialFromRow(r))
	}
	return passkeyUser{id: user.ID, name: user.Username, creds: creds}, nil
}

func aaguidBytes(v pgtype.UUID) []byte {
	if !v.Valid {
		return nil
	}
	return v.Bytes[:]
}

func credentialFromRow(r db.WebauthnCredential) webauthn.Credential {
	flags := webauthn.CredentialFlags{UserPresent: true, UserVerified: true}
	if len(r.Flags) > 0 {
		var f struct {
			UserPresent  *bool `json:"user_present"`
			UserVerified *bool `json:"user_verified"`
		}
		if err := json.Unmarshal(r.Flags, &f); err == nil {
			if f.UserPresent != nil {
				flags.UserPresent = *f.UserPresent
			}
			if f.UserVerified != nil {
				flags.UserVerified = *f.UserVerified
			}
		}
	}
	transports := make([]protocol.AuthenticatorTransport, 0, len(r.Transports))
	for _, t := range r.Transports {
		transports = append(transports, protocol.AuthenticatorTransport(t))
	}
	signCount := uint32(r.SignCount) //nolint:gosec // sign counts are non-negative by spec
	return webauthn.Credential{
		ID:        r.ID,
		PublicKey: r.PublicKey,
		Transport: transports,
		Flags:     flags,
		Authenticator: webauthn.Authenticator{
			AAGUID:    aaguidBytes(r.AttestationAaguid),
			SignCount: signCount,
		},
	}
}

// ---- Registration -------------------------------------------------------------------

// Ceremony is what the client receives when it begins a ceremony: an opaque id
// to present at finish time, plus the protocol payload to hand to the browser.
type Ceremony struct {
	CeremonyID uuid.UUID `json:"ceremony_id"`
	// Options is the CredentialCreation / CredentialAssertion document.
	Options any `json:"options"`
}

func (s *Service) BeginRegistration(ctx context.Context, userID uuid.UUID) (*Ceremony, error) {
	u, err := s.loadUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	creation, session, err := s.wa.BeginRegistration(u)
	if err != nil {
		return nil, fmt.Errorf("begin registration: %w", err)
	}
	id, err := s.persistSession(ctx, db.AuthChallengeKindWebauthnRegistration, &userID, nil, session, RegistrationTTL)
	if err != nil {
		return nil, err
	}
	return &Ceremony{CeremonyID: id, Options: creation}, nil
}

// FinishRegistration validates the authenticator's response and stores the new
// credential. The ceremony row is consumed atomically: a replayed response
// fails, and two concurrent finishes cannot both succeed.
func (s *Service) FinishRegistration(ctx context.Context, userID, ceremonyID uuid.UUID, r *RequestReader) error {
	ch, err := s.st.ConsumeChallenge(ctx, ceremonyID, db.AuthChallengeKindWebauthnRegistration)
	if err != nil {
		return ErrInvalidToken
	}
	if store.UUIDGet(ch.UserID) == nil || *store.UUIDGet(ch.UserID) != userID {
		return ErrInvalidToken
	}
	session, err := unmarshalSession(ch.SessionData)
	if err != nil {
		return err
	}
	u, err := s.loadUser(ctx, userID)
	if err != nil {
		return err
	}
	cred, err := s.wa.FinishRegistration(u, *session, r.HTTP)
	if err != nil {
		return fmt.Errorf("finish registration: %w", err)
	}
	return s.storeCredential(ctx, userID, cred, "Passkey")
}

func (s *Service) storeCredential(ctx context.Context, userID uuid.UUID, cred *webauthn.Credential, nickname string) error {
	var aaguid *uuid.UUID
	if len(cred.Authenticator.AAGUID) == 16 {
		u := uuid.UUID(cred.Authenticator.AAGUID)
		aaguid = &u
	}
	transports := make([]string, 0, len(cred.Transport))
	for _, t := range cred.Transport {
		transports = append(transports, string(t))
	}
	flags, err := json.Marshal(map[string]bool{
		"user_present":  cred.Flags.UserPresent,
		"user_verified": cred.Flags.UserVerified,
	})
	if err != nil {
		return err
	}
	return s.st.Queries().InsertWebAuthnCredential(ctx, db.InsertWebAuthnCredentialParams{
		ID: cred.ID, UserID: userID, PublicKey: cred.PublicKey,
		AttestationAaguid: store.UUIDPtr(aaguid),
		SignCount:         int64(cred.Authenticator.SignCount),
		Transports:        transports,
		Nickname:          nickname,
		Flags:             flags,
	})
}

// ---- Login ----------------------------------------------------------------------------

// BeginLogin starts an assertion ceremony. identifier is a USERNAME (public by
// design, so confirming it reveals nothing) or empty for a discoverable
// ("which passkey?") ceremony. Email addresses are deliberately not accepted:
// login must never oracle mailbox existence.
func (s *Service) BeginLogin(ctx context.Context, identifier, peerIP string) (*Ceremony, error) {
	bucket := BucketKey([]byte("auth-pepper"), "login", orAnonymous(identifier, peerIP))
	allowed, err := s.st.ThrottleAuth(ctx, db.AuthChallengeKindWebauthnLogin, bucket,
		LoginAttemptMaxPerQuarterHour, 15*time.Minute)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrThrottled
	}

	if strings.TrimSpace(identifier) == "" {
		assertion, session, err := s.wa.BeginDiscoverableLogin()
		if err != nil {
			return nil, fmt.Errorf("begin discoverable login: %w", err)
		}
		id, err := s.persistSession(ctx, db.AuthChallengeKindWebauthnLogin, nil, nil, session, LoginTTL)
		if err != nil {
			return nil, err
		}
		return &Ceremony{CeremonyID: id, Options: assertion}, nil
	}

	user, err := s.st.Queries().GetUserByUsername(ctx, strings.ToLower(identifier))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidToken
	}
	if err != nil {
		return nil, err
	}
	u, err := s.loadUser(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	if len(u.creds) == 0 {
		return nil, ErrNoCredentials
	}
	assertion, session, err := s.wa.BeginLogin(u)
	if err != nil {
		return nil, fmt.Errorf("begin login: %w", err)
	}
	id, err := s.persistSession(ctx, db.AuthChallengeKindWebauthnLogin, &user.ID, nil, session, LoginTTL)
	if err != nil {
		return nil, err
	}
	return &Ceremony{CeremonyID: id, Options: assertion}, nil
}

// FinishLogin completes the assertion and mints a session. It returns the
// account and the opaque session token to set as a cookie.
func (s *Service) FinishLogin(ctx context.Context, ceremonyID uuid.UUID, r *RequestReader) (userID uuid.UUID, token string, err error) {
	ch, err := s.st.ConsumeChallenge(ctx, ceremonyID, db.AuthChallengeKindWebauthnLogin)
	if err != nil {
		return uuid.Nil, "", ErrInvalidToken
	}
	session, err := unmarshalSession(ch.SessionData)
	if err != nil {
		return uuid.Nil, "", err
	}

	var cred *webauthn.Credential
	if uid := store.UUIDGet(ch.UserID); uid != nil {
		u, loadErr := s.loadUser(ctx, *uid)
		if loadErr != nil {
			return uuid.Nil, "", loadErr
		}
		cred, err = s.wa.FinishLogin(u, *session, r.HTTP)
		userID = *uid
	} else {
		cred, err = s.wa.FinishDiscoverableLogin(s.discoverableHandler(ctx), *session, r.HTTP)
		if err == nil && cred != nil {
			uid2, uerr := uuid.FromBytes(session.UserID)
			userID = uid2
			if uerr != nil || userID == uuid.Nil {
				// Discoverable ceremonies carry the user handle in the response;
				// resolve it through the credential we just validated.
				row, lookupErr := s.st.Queries().GetWebAuthnCredential(ctx, cred.ID)
				if lookupErr != nil {
					return uuid.Nil, "", lookupErr
				}
				userID = row.UserID
			}
		}
	}
	if err != nil {
		slog.WarnContext(ctx, "passkey assertion rejected", "err", err)
		return uuid.Nil, "", ErrInvalidToken
	}
	if cred.Authenticator.CloneWarning {
		// A non-increasing sign count means the private key exists in two
		// places. Alexandria fails closed: a cloned authenticator is a
		// compromised one.
		slog.ErrorContext(ctx, "cloned authenticator detected; refusing login and flagging credential",
			"credential_id", fmt.Sprintf("%x", cred.ID))
		return uuid.Nil, "", ErrInvalidToken
	}

	if err := s.st.Queries().UpdateWebAuthnCredentialAfterAssertion(ctx, db.UpdateWebAuthnCredentialAfterAssertionParams{
		ID: cred.ID, SignCount: int64(cred.Authenticator.SignCount),
	}); err != nil {
		return uuid.Nil, "", err
	}

	token, hash, err := NewSessionToken()
	if err != nil {
		return uuid.Nil, "", err
	}
	ua := r.UserAgent
	ipHash := hashIP(r.IP)
	if _, err := s.st.CreateSession(ctx, userID, hash, strPtr(ua), ipHash, SessionTTL); err != nil {
		return uuid.Nil, "", err
	}
	if err := s.st.Queries().RecordLogin(ctx, userID); err != nil {
		return uuid.Nil, "", err
	}
	return userID, token, nil
}

// discoverableHandler resolves the user handle embedded in a discoverable
// assertion back to an account.
func (s *Service) discoverableHandler(ctx context.Context) webauthn.DiscoverableUserHandler {
	return func(rawID, userHandle []byte) (webauthn.User, error) {
		uid, err := uuid.FromBytes(userHandle)
		if err != nil || uid == uuid.Nil {
			return nil, ErrInvalidToken
		}
		return s.loadUser(ctx, uid)
	}
}

// ---- ceremony persistence --------------------------------------------------------------

func (s *Service) persistSession(ctx context.Context, kind db.AuthChallengeKind, userID *uuid.UUID, email *string, session *webauthn.SessionData, ttl time.Duration) (uuid.UUID, error) {
	body, err := json.Marshal(session)
	if err != nil {
		return uuid.Nil, err
	}
	ch, err := s.st.CreateChallenge(ctx, kind, userID, email, []byte(session.Challenge), body, ttl)
	if err != nil {
		return uuid.Nil, err
	}
	return ch.ID, nil
}

func unmarshalSession(raw []byte) (*webauthn.SessionData, error) {
	var sd webauthn.SessionData
	if err := json.Unmarshal(raw, &sd); err != nil {
		return nil, fmt.Errorf("corrupt ceremony state: %w", err)
	}
	return &sd, nil
}

func orAnonymous(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return "anonymous:" + fallback
	}
	return v
}

// RequestReader pairs the HTTP request (go-webauthn parses the ceremony
// response straight from its body) with the transport facts the session layer
// records: client IP and user agent.
type RequestReader struct {
	HTTP      *http.Request
	UserAgent string
	IP        string
}

// hashIP salts the client IP before it is stored. Raw IP addresses are personal
// data and a magnet for regulation; a salted hash still lets us detect
// credential stuffing from one address without retaining the address itself.
func hashIP(ip string) []byte {
	if ip == "" {
		return nil
	}
	mac := hmac.New(sha256.New, []byte("alexandria-ip-pepper"))
	mac.Write([]byte(ip))
	return mac.Sum(nil)
}
