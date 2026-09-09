// Package push implements Web Push as specified: VAPID authorisation
// (RFC 8292) and aes128gcm payload encryption (RFC 8291), on the standard
// library plus x/crypto/hkdf.
//
// Why hand-roll: the protocol is three cryptographic steps with exact wire
// formats, and a dependency here would own our key handling. What matters is
// the property, and it is testable end to end: the push service receives an
// opaque ciphertext and a signature it can verify, and only the reader's
// device — holding the private half of p256dh — can read the body.
package push

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/crypto/hkdf"
)

// Config carries the VAPID identity. Private is the raw P-256 scalar,
// base64url; Public is the uncompressed point, base64url, exactly as the
// browser's PushManager expects applicationServerKey.
type Config struct {
	PublicKey  string
	PrivateKey string
	Subject    string // mailto:ops@example.org
}

func (c Config) Enabled() bool {
	return c.PublicKey != "" && c.PrivateKey != "" && c.Subject != ""
}

// ErrGone signals a dead subscription (404/410): the caller must delete it.
var ErrGone = errors.New("push subscription is gone")

// Subscription is the stored client material.
type Subscription struct {
	Endpoint string
	P256DH   []byte
	Auth     []byte
}

// ———— aes128gcm payload encryption (RFC 8291) ————

// Encrypt produces the aes128gcm record for one payload:
//
//	salt(16) || rs(4) || idlen(1)=65 || ephemeral public key(65) || ciphertext
//
// The plaintext carries the 0x02 final-record delimiter before encryption.
func Encrypt(payload []byte, sub Subscription) ([]byte, error) {
	clientPub, err := ecdh.P256().NewPublicKey(sub.P256DH)
	if err != nil {
		return nil, fmt.Errorf("client p256dh: %w", err)
	}
	eph, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	shared, err := eph.ECDH(clientPub)
	if err != nil {
		return nil, fmt.Errorf("ecdh: %w", err)
	}

	// IKM per RFC 8291: HKDF(auth, shared, "WebPush: info\0" || clientPub || ephPub)
	info := bytes.NewBuffer(nil)
	info.WriteString("WebPush: info\x00")
	info.Write(sub.P256DH)
	ephPub := eph.PublicKey().Bytes()
	info.Write(ephPub)
	ikm := hkdfExpand(sub.Auth, shared, info.Bytes(), 32)

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	cek := hkdfExpand(salt, ikm, []byte("Content-Encoding: aes128gcm\x00"), 16)
	nonce := hkdfExpand(salt, ikm, []byte("Content-Encoding: nonce\x00"), 12)

	block, err := aesGCM(cek)
	if err != nil {
		return nil, err
	}
	// Single record, padded delimiter 0x02.
	plain := append(append([]byte{}, payload...), 0x02)
	ct := block.Seal(nil, nonce, plain, nil)

	out := bytes.NewBuffer(nil)
	out.Write(salt)
	rs := make([]byte, 4)
	binary.BigEndian.PutUint32(rs, 4096)
	out.Write(rs)
	out.WriteByte(65)
	out.Write(ephPub)
	out.Write(ct)
	return out.Bytes(), nil
}

// Decrypt is the client side of RFC 8291, used by tests to prove the record
// we emit is the record the spec describes.
func Decrypt(record, payload []byte, privKey *ecdh.PrivateKey, senderPub []byte) ([]byte, error) {
	if len(record) < 16+4+1 {
		return nil, errors.New("record too short")
	}
	salt := record[:16]
	idlen := int(record[20])
	ephPub := record[21 : 21+idlen]
	ct := record[21+idlen:]

	eph, err := ecdh.P256().NewPublicKey(ephPub)
	if err != nil {
		return nil, err
	}
	shared, err := privKey.ECDH(eph)
	if err != nil {
		return nil, err
	}
	info := bytes.NewBuffer(nil)
	info.WriteString("WebPush: info\x00")
	info.Write(privKey.PublicKey().Bytes())
	info.Write(ephPub)
	ikm := hkdfExpand(payload, shared, info.Bytes(), 32)

	cek := hkdfExpand(salt, ikm, []byte("Content-Encoding: aes128gcm\x00"), 16)
	nonce := hkdfExpand(salt, ikm, []byte("Content-Encoding: nonce\x00"), 12)
	block, err := aesGCM(cek)
	if err != nil {
		return nil, err
	}
	plain, err := block.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, err
	}
	if len(plain) == 0 || plain[len(plain)-1] != 0x02 {
		return nil, errors.New("missing padding delimiter")
	}
	_ = senderPub
	return plain[:len(plain)-1], nil
}

func aesGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func hkdfExpand(salt, secret, info []byte, n int) []byte {
	r := hkdf.New(sha256.New, secret, salt, info)
	out := make([]byte, n)
	if _, err := io.ReadFull(r, out); err != nil {
		panic(err) // HKDF-SHA256 cannot fail for these lengths
	}
	return out
}

// ———— VAPID (RFC 8292) ————

type vapidClaims struct {
	Audience string `json:"aud"`
	Expires  int64  `json:"exp"`
	Subject  string `json:"sub"`
}

// vapidToken signs the JWT a push service verifies. ES256 with raw r||s, as
// the spec requires; ASN.1 would be rejected.
func (c Config) vapidToken(pushEndpoint string) (string, error) {
	u, err := url.Parse(pushEndpoint)
	if err != nil {
		return "", err
	}
	claims := vapidClaims{
		Audience: u.Scheme + "://" + u.Host,
		Expires:  time.Now().Add(12 * time.Hour).Unix(),
		Subject:  c.Subject,
	}
	header := map[string]string{"alg": "ES256", "typ": "JWT"}
	hb, _ := json.Marshal(header)
	cb, _ := json.Marshal(claims)
	signing := b64(hb) + "." + b64(cb)

	d, err := base64.RawURLEncoding.DecodeString(c.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("vapid private key: %w", err)
	}
	key, err := privateFromScalar(d)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(signing))
	r, s, err := ecdsa.Sign(rand.Reader, key, digest[:])
	if err != nil {
		return "", err
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return signing + "." + b64(sig), nil
}

func privateFromScalar(d []byte) (*ecdsa.PrivateKey, error) {
	curve := elliptic.P256()
	k := new(big.Int).SetBytes(d)
	if k.Sign() == 0 || k.Cmp(curve.Params().N) >= 0 {
		return nil, errors.New("vapid scalar out of range")
	}
	x, y := curve.ScalarBaseMult(d)
	return &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{Curve: curve, X: x, Y: y},
		D:         k,
	}, nil
}

// PublicPoint derives the uncompressed point from the private scalar, for
// operators who only configured the secret.
func PublicPoint(privateB64url string) (string, error) {
	d, err := base64.RawURLEncoding.DecodeString(privateB64url)
	if err != nil {
		return "", err
	}
	key, err := privateFromScalar(d)
	if err != nil {
		return "", err
	}
	return b64(elliptic.Marshal(key.Curve, key.X, key.Y)), nil
}

// ———— send ————

// Send delivers one encrypted payload to one subscription.
func Send(ctx context.Context, c Config, sub Subscription, payload []byte) error {
	record, err := Encrypt(payload, sub)
	if err != nil {
		return err
	}
	token, err := c.vapidToken(sub.Endpoint)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Endpoint, bytes.NewReader(record))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("TTL", "86400")
	req.Header.Set("Urgency", "normal")
	req.Header.Set("Authorization", "vapid t="+token+", k="+c.PublicKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10)) //nolint:errcheck
	switch {
	case resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK:
		return nil
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return ErrGone
	default:
		return fmt.Errorf("push service answered %d", resp.StatusCode)
	}
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// GenerateVAPID returns a fresh keypair as the config strings expect.
func GenerateVAPID() (public, private string, err error) {
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	// ecdh private bytes are the raw scalar, which is what ES256 signing uses.
	return b64(key.PublicKey().Bytes()), b64(key.Bytes()), nil
}
