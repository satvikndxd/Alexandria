package push

import (
	"context"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func clientKeys(t *testing.T) (*ecdh.PrivateKey, Subscription) {
	t.Helper()
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	if _, err := rand.Read(auth); err != nil {
		t.Fatal(err)
	}
	return priv, Subscription{
		Endpoint: "https://push.example/endpoint/1",
		P256DH:   priv.PublicKey().Bytes(),
		Auth:     auth,
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	priv, sub := clientKeys(t)
	payload := []byte(`{"title":"A reply arrived"}`)
	record, err := Encrypt(payload, sub)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	// Wire format: salt(16) rs(4) idlen(1)=65 ephPub(65) ct…
	if len(record) < 21+65 {
		t.Fatalf("record too short: %d bytes", len(record))
	}
	if record[20] != 65 {
		t.Fatalf("idlen = %d, want 65", record[20])
	}
	got, err := Decrypt(record, sub.Auth, priv, nil)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("round trip = %q, want %q", got, payload)
	}
}

func TestCiphertextDoesNotLeakPayload(t *testing.T) {
	_, sub := clientKeys(t)
	payload := []byte("someone replied to your review of Middlemarch")
	record, err := Encrypt(payload, sub)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(record), "Middlemarch") {
		t.Fatal("plaintext leaked into the record")
	}
}

func TestVapidTokenVerifies(t *testing.T) {
	pub, priv, err := GenerateVAPID()
	if err != nil {
		t.Fatal(err)
	}
	c := Config{PublicKey: pub, PrivateKey: priv, Subject: "mailto:ops@alexandria.example"}
	token, err := c.vapidToken("https://push.example/endpoint/1")
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token parts = %d", len(parts))
	}
	// Claims carry the push service origin as audience and our contact.
	cb, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var claims vapidClaims
	if err := json.Unmarshal(cb, &claims); err != nil {
		t.Fatal(err)
	}
	if claims.Audience != "https://push.example" || !strings.HasPrefix(claims.Subject, "mailto:") {
		t.Fatalf("claims wrong: %+v", claims)
	}
	// ES256 raw r||s must verify against the configured public point.
	pb, _ := base64.RawURLEncoding.DecodeString(pub)
	x, y := elliptic.Unmarshal(elliptic.P256(), pb)
	key := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}
	sig, _ := base64.RawURLEncoding.DecodeString(parts[2])
	if len(sig) != 64 {
		t.Fatalf("signature length = %d, want 64 (raw r||s)", len(sig))
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if !ecdsa.Verify(key, digest[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])) {
		t.Fatal("VAPID signature does not verify")
	}
}

func TestSendDeliversEncryptedAndSigned(t *testing.T) {
	pub, priv, _ := GenerateVAPID()
	cfg := Config{PublicKey: pub, PrivateKey: priv, Subject: "mailto:ops@alexandria.example"}
	clientPriv, sub := clientKeys(t)

	var gotAuth, gotEncoding string
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotEncoding = r.Header.Get("Content-Encoding")
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	sub.Endpoint = srv.URL

	payload := []byte(`{"title":"Verdict on your note"}`)
	if err := Send(context.Background(), cfg, sub, payload); err != nil {
		t.Fatalf("send: %v", err)
	}
	if !strings.HasPrefix(gotAuth, "vapid t=") || !strings.Contains(gotAuth, "k="+pub) {
		t.Fatalf("authorization header wrong: %q", gotAuth)
	}
	if gotEncoding != "aes128gcm" {
		t.Fatalf("content-encoding = %q", gotEncoding)
	}
	got, err := Decrypt(body, sub.Auth, clientPriv, nil)
	if err != nil {
		t.Fatalf("push service body undecryptable by the client: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("delivered %q, want %q", got, payload)
	}
}

func TestSendReportsGone(t *testing.T) {
	pub, priv, _ := GenerateVAPID()
	cfg := Config{PublicKey: pub, PrivateKey: priv, Subject: "mailto:ops@alexandria.example"}
	_, sub := clientKeys(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusGone)
	}))
	defer srv.Close()
	sub.Endpoint = srv.URL
	if err := Send(context.Background(), cfg, sub, []byte("x")); err != ErrGone {
		t.Fatalf("err = %v, want ErrGone", err)
	}
}
