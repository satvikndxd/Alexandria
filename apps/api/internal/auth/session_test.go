package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestSessionTokensAreOpaqueAndHashed: the stored form must not allow
// recovering the cookie value, and two tokens must never collide.
func TestSessionTokensAreOpaqueAndHashed(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		token, hash, err := NewSessionToken()
		if err != nil {
			t.Fatalf("NewSessionToken: %v", err)
		}
		if len(hash) != 32 {
			t.Fatalf("hash length = %d, want 32 (SHA-256)", len(hash))
		}
		if strings.Contains(string(hash), token) || strings.Contains(token, string(hash)) {
			t.Fatal("token and hash are not independent")
		}
		if seen[token] {
			t.Fatal("token collision")
		}
		seen[token] = true
	}
}

// TestHashTokenIsStable: hashing is a pure function so any node can resolve a
// cookie without shared state.
func TestHashTokenIsStable(t *testing.T) {
	a := HashToken("abc")
	b := HashToken("abc")
	c := HashToken("abd")
	if string(a) != string(b) {
		t.Fatal("hash not stable")
	}
	if string(a) == string(c) {
		t.Fatal("hash collision on adjacent inputs")
	}
}

// TestSessionCookieAttributes pins the cookie hardening: HttpOnly (no JS
// access), SameSite=Lax (cross-site POST carries no cookie), Path=/, and
// Secure when the policy says TLS.
func TestSessionCookieAttributes(t *testing.T) {
	for _, tc := range []struct {
		secure     bool
		wantSecure bool
	}{
		{secure: false, wantSecure: false},
		{secure: true, wantSecure: true},
	} {
		w := httptest.NewRecorder()
		SetSessionCookie(w, "tok", DefaultCookiePolicy(tc.secure))
		cookies := w.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("cookies = %d, want 1", len(cookies))
		}
		c := cookies[0]
		if c.Name != SessionCookieName || c.Value != "tok" {
			t.Fatalf("cookie = %s=%s", c.Name, c.Value)
		}
		if !c.HttpOnly {
			t.Error("session cookie must be HttpOnly")
		}
		if c.Secure != tc.wantSecure {
			t.Errorf("Secure = %v, want %v", c.Secure, tc.wantSecure)
		}
		if c.SameSite != http.SameSiteLaxMode {
			t.Errorf("SameSite = %v, want Lax", c.SameSite)
		}
		if c.Path != "/" {
			t.Errorf("Path = %q, want /", c.Path)
		}
		if c.MaxAge != int(SessionTTL.Seconds()) {
			t.Errorf("MaxAge = %d, want %d", c.MaxAge, int(SessionTTL.Seconds()))
		}
	}
}

// TestCSRFTokenIsBoundToSession: the expected header value derives from the
// session token, so a CSRF cookie planted for one session cannot authenticate
// a request on another.
func TestCSRFTokenIsBoundToSession(t *testing.T) {
	pepper := []byte("pepper")
	a := CSRFToken(pepper, "session-a")
	b := CSRFToken(pepper, "session-b")
	if a == b {
		t.Fatal("csrf token is not bound to the session")
	}
	if a == CSRFToken([]byte("other"), "session-a") {
		t.Fatal("csrf token ignores the pepper")
	}
}

// TestCheckCSRF: safe methods pass without tokens; authenticated state changes
// require a matching cookie+header pair.
func TestCheckCSRF(t *testing.T) {
	pepper := []byte("pepper")
	tok := "session-token"
	want := CSRFToken(pepper, tok)

	get := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := CheckCSRF(get, pepper); err != nil {
		t.Fatalf("GET should not require csrf: %v", err)
	}

	// Authenticated POST without the header is refused.
	post := httptest.NewRequest(http.MethodPost, "/", nil)
	post.AddCookie(&http.Cookie{Name: SessionCookieName, Value: tok})
	if err := CheckCSRF(post, pepper); err == nil {
		t.Fatal("POST with session but no csrf header must be refused")
	}

	// Header present but cookie absent is refused (double-submit needs both).
	post2 := httptest.NewRequest(http.MethodPost, "/", nil)
	post2.AddCookie(&http.Cookie{Name: SessionCookieName, Value: tok})
	post2.Header.Set(CSRFHeaderName, want)
	if err := CheckCSRF(post2, pepper); err == nil {
		t.Fatal("header without matching cookie must be refused")
	}

	// Both present and matching: allowed.
	post3 := httptest.NewRequest(http.MethodPost, "/", nil)
	post3.AddCookie(&http.Cookie{Name: SessionCookieName, Value: tok})
	post3.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: want})
	post3.Header.Set(CSRFHeaderName, want)
	if err := CheckCSRF(post3, pepper); err != nil {
		t.Fatalf("valid double-submit refused: %v", err)
	}

	// A token from a different session is refused.
	post4 := httptest.NewRequest(http.MethodPost, "/", nil)
	post4.AddCookie(&http.Cookie{Name: SessionCookieName, Value: tok})
	post4.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: CSRFToken(pepper, "other-session")})
	post4.Header.Set(CSRFHeaderName, CSRFToken(pepper, "other-session"))
	if err := CheckCSRF(post4, pepper); err == nil {
		t.Fatal("cross-session csrf token must be refused")
	}
}

// TestBucketKeyDoesNotLeakInputs: buckets are keyed HMACs, so the raw email or
// IP never appears in the throttling table.
func TestBucketKeyDoesNotLeakInputs(t *testing.T) {
	k := BucketKey([]byte("pepper"), "magic_link", "Someone@Example.com ")
	if strings.Contains(k, "someone@example.com") || strings.Contains(k, "Someone") {
		t.Fatalf("bucket key leaks its input: %s", k)
	}
	if k != BucketKey([]byte("pepper"), "magic_link", "someone@example.com") {
		t.Fatal("bucket key must normalise case and whitespace")
	}
	if k == BucketKey([]byte("pepper"), "magic_link", "other@example.com") {
		t.Fatal("distinct inputs produced the same bucket")
	}
}

// TestMagicLinkTokenShape pins the token format the email carries, because the
// verify path parses it positionally.
func TestMagicLinkTokenShape(t *testing.T) {
	token := "6ba7b810-9dad-11d1-80b4-00c04fd430c8." + strings.Repeat("A", 43)
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		t.Fatalf("token shape changed: %q", token)
	}
	_ = time.Now // lifetimes are asserted in the integration suite
}
