// Package auth implements Alexandria's identity layer: passkey (WebAuthn)
// ceremonies as the primary factor, email magic links as the fallback, opaque
// hashed session tokens as the session mechanism, and double-submit CSRF
// protection for cookie-authenticated state changes.
//
// Threat model in one paragraph: credentials are phishing-resistant by
// construction (the authenticator binds the ceremony to the RP origin, so a
// lookalike domain cannot harvest them); session tokens are 256-bit random and
// stored only as SHA-256 hashes, so a database disclosure yields no live
// sessions; magic links are single-use, short-lived, throttled per hashed
// mailbox, and never confirm whether an address exists; and every ceremony
// state row expires on its own so nothing accumulates.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Cookie and ceremony lifetimes. Thirty days of session with sliding renewal
// balances "a library you stay signed into" against a stolen cookie's window;
// ceremonies are minutes because they are one-round-trip secrets.
const (
	SessionCookieName = "alexandria_session"
	CSRFCookieName    = "alexandria_csrf"
	CSRFHeaderName    = "X-CSRF-Token"

	SessionTTL           = 30 * 24 * time.Hour
	SessionSlideAfter    = time.Hour
	RegistrationTTL      = 10 * time.Minute
	LoginTTL             = 5 * time.Minute
	MagicLinkTTL         = 15 * time.Minute
	EmailVerificationTTL = 24 * time.Hour

	MagicLinkMaxPerHour           = 5
	LoginAttemptMaxPerQuarterHour = 10
)

var (
	ErrInvalidToken  = errors.New("invalid or expired authentication token")
	ErrThrottled     = errors.New("too many attempts — please wait a while")
	ErrCSRF          = errors.New("cross-site request rejected")
	ErrNoCredentials = errors.New("this account has no passkeys yet — use the email link, then add a passkey")
)

// ---- session tokens ------------------------------------------------------------

// NewSessionToken returns a fresh opaque token and the hash to store. The raw
// token never touches the database; only its SHA-256 does.
func NewSessionToken() (token string, hash []byte, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("session entropy: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, HashToken(token), nil
}

func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// BucketKey hashes a throttling dimension (email address or client IP) with a
// server-side pepper. Buckets must be stable per server but useless if the
// table leaks, and raw emails/IPs are personal data we decline to store.
func BucketKey(pepper []byte, kind, value string) string {
	mac := hmac.New(sha256.New, pepper)
	mac.Write([]byte(kind))
	mac.Write([]byte{0})
	mac.Write([]byte(strings.ToLower(strings.TrimSpace(value))))
	return hex.EncodeToString(mac.Sum(nil))
}

// ---- cookies ----------------------------------------------------------------------

// CookiePolicy carries the environment-dependent cookie attributes.
type CookiePolicy struct {
	Secure   bool   // true behind TLS (always in production)
	Domain   string // usually empty: host-only cookies
	SameSite http.SameSite
}

func DefaultCookiePolicy(secure bool) CookiePolicy {
	return CookiePolicy{Secure: secure, SameSite: http.SameSiteLaxMode}
}

func SetSessionCookie(w http.ResponseWriter, token string, p CookiePolicy) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		Domain:   p.Domain,
		HttpOnly: true,
		Secure:   p.Secure,
		SameSite: p.SameSite,
		MaxAge:   int(SessionTTL.Seconds()),
	})
}

func ClearSessionCookie(w http.ResponseWriter, p CookiePolicy) {
	http.SetCookie(w, &http.Cookie{
		Name: SessionCookieName, Value: "", Path: "/", Domain: p.Domain,
		HttpOnly: true, Secure: p.Secure, SameSite: p.SameSite, MaxAge: -1,
	})
}

func SessionCookie(r *http.Request) string {
	c, err := r.Cookie(SessionCookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

// ---- CSRF -------------------------------------------------------------------------
// Cookies are SameSite=Lax, which already blocks cross-site POSTs from other
// sites' top-level navigations. The double-submit token below is defense in
// depth for the cases Lax does not cover (same-site subdomains, older clients,
// and any future move to SameSite=None for cross-platform clients).

func IssueCSRFCookie(w http.ResponseWriter, token string, p CookiePolicy) {
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    token,
		Path:     "/",
		Domain:   p.Domain,
		HttpOnly: false, // the web client must read it to echo it in a header
		Secure:   p.Secure,
		SameSite: p.SameSite,
		MaxAge:   int(SessionTTL.Seconds()),
	})
}

func ClearCSRFCookie(w http.ResponseWriter, p CookiePolicy) {
	http.SetCookie(w, &http.Cookie{
		Name: CSRFCookieName, Value: "", Path: "/", Domain: p.Domain,
		Secure: p.Secure, SameSite: p.SameSite, MaxAge: -1,
	})
}

// CSRFToken derives the expected header value from the session token. Binding
// it to the session means an attacker who can set a CSRF cookie still cannot
// forge a token for someone else's session.
func CSRFToken(pepper []byte, sessionToken string) string {
	mac := hmac.New(sha256.New, pepper)
	mac.Write([]byte("csrf"))
	mac.Write([]byte{0})
	mac.Write([]byte(sessionToken))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// CheckCSRF validates the double-submit pair for a state-changing request.
func CheckCSRF(r *http.Request, pepper []byte) error {
	session := SessionCookie(r)
	if session == "" {
		// Unauthenticated state changes carry no cookie and are rejected by the
		// auth middleware anyway; nothing to validate here.
		return nil
	}
	c, err := r.Cookie(CSRFCookieName)
	if err != nil {
		return ErrCSRF
	}
	header := r.Header.Get(CSRFHeaderName)
	if header == "" {
		return ErrCSRF
	}
	want := CSRFToken(pepper, session)
	if subtle.ConstantTimeCompare([]byte(header), []byte(want)) != 1 ||
		subtle.ConstantTimeCompare([]byte(c.Value), []byte(want)) != 1 {
		return ErrCSRF
	}
	return nil
}

// RandomHex returns n bytes of CSPRNG entropy as hex; used for magic-link
// secrets and the CSRF pepper bootstrap.
func RandomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
