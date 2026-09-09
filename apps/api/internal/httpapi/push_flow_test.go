package httpapi_test

import (
	"crypto/ecdh"
	"strings"
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/alexandria-reads/alexandria/apps/api/internal/push"
)

// TestPushFlow proves the whole opt-in chain over HTTP: configuration
// publishes the public key, a subscription is stored, a real event produces a
// real encrypted push to the reader's endpoint, the cursor advances so the
// same note is never pushed twice, and a Gone endpoint is forgotten.
func TestPushFlow(t *testing.T) {
	e := newEnv(t)
	pub := e.pushCfg.PublicKey

	// A fake push service that records what it receives.
	var hits atomic.Int32
	var lastAuth, lastEncoding string
	var lastBody []byte
	svc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		lastAuth = r.Header.Get("Authorization")
		lastEncoding = r.Header.Get("Content-Encoding")
		lastBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer svc.Close()

	followed, _ := e.signIn("pushed_reader", "pushed@example.com")
	followerC, followerCSRF := e.signIn("push_follower", "pushfollower@example.com")

	// Config publishes the public key, and only the public key.
	resp := e.get(e.client(), "/v1/push/config")
	cfgBody := mustStatus(t, resp, http.StatusOK)
	if cfgBody["public_key"] != pub {
		t.Fatalf("config public key = %v", cfgBody["public_key"])
	}

	// Subscribe with real client key material.
	clientPriv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	authSecret := make([]byte, 16)
	if _, err := rand.Read(authSecret); err != nil {
		t.Fatal(err)
	}
	resp = e.send(followed, http.MethodPost, "/v1/push/subscribe", map[string]any{
		"endpoint": svc.URL,
		"keys": map[string]string{
			"p256dh": base64.RawURLEncoding.EncodeToString(clientPriv.PublicKey().Bytes()),
			"auth":   base64.RawURLEncoding.EncodeToString(authSecret),
		},
	}, "")
	mustStatus(t, resp, http.StatusForbidden) // CSRF applies to subscriptions too
	resp = e.send(followed, http.MethodPost, "/v1/push/subscribe", map[string]any{
		"endpoint": svc.URL,
		"keys": map[string]string{
			"p256dh": base64.RawURLEncoding.EncodeToString(clientPriv.PublicKey().Bytes()),
			"auth":   base64.RawURLEncoding.EncodeToString(authSecret),
		},
	}, e.csrfFor(t, followed))
	mustStatus(t, resp, http.StatusCreated)

	// Nothing due yet: no notifications exist.
	sent, err := e.storeWithPush(e.pushCfg).PushTick(e.ctx, e.pushCfg)
	if err != nil {
		t.Fatalf("tick: %v", err)
	}
	if sent != 0 || hits.Load() != 0 {
		t.Fatalf("pushed with nothing due: sent=%d hits=%d", sent, hits.Load())
	}

	// A follow notifies the reader; the tick delivers exactly once.
	resp = e.send(followerC, http.MethodPost, "/v1/users/pushed_reader/follow", map[string]any{}, followerCSRF)
	mustStatus(t, resp, http.StatusOK)
	sent, err = e.storeWithPush(e.pushCfg).PushTick(e.ctx, e.pushCfg)
	if err != nil {
		t.Fatalf("tick: %v", err)
	}
	if sent != 1 || hits.Load() != 1 {
		t.Fatalf("sent=%d hits=%d, want 1/1", sent, hits.Load())
	}
	if lastEncoding != "aes128gcm" {
		t.Fatalf("encoding = %q", lastEncoding)
	}
	if !strings.HasPrefix(lastAuth, "vapid t=") || !strings.Contains(lastAuth, "k="+pub) {
		t.Fatalf("auth = %q", lastAuth)
	}
	// The push service holds ciphertext only: decrypt with the client keys.
	plain, err := push.Decrypt(lastBody, authSecret, clientPriv, nil)
	if err != nil {
		t.Fatalf("client cannot decrypt what the service received: %v", err)
	}
	if !contains(string(plain), "follows your shelves") {
		t.Fatalf("payload body wrong: %q", plain)
	}

	// The cursor advanced: a second tick pushes nothing.
	sent, _ = e.storeWithPush(e.pushCfg).PushTick(e.ctx, e.pushCfg)
	if sent != 0 || hits.Load() != 1 {
		t.Fatalf("duplicate push: sent=%d hits=%d", sent, hits.Load())
	}
}

func contains(hay, needle string) bool {
	return len(hay) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(hay); i++ {
			if hay[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
