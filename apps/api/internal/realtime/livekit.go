// Package realtime mints LiveKit room tokens from Alexandria's own
// authorization decisions.
//
// LiveKit is the SFU; it is not an authority. Every token here is issued only
// after the monolith has checked club membership and the channel's spoiler
// gate, so a forged or leaked room name grants nothing: LiveKit accepts only
// tokens signed with our API secret, and we sign only what our own rules
// allow.
//
// The JWT is HS256 with the LiveKit video grant, built on the standard
// library — a room token is three claims and a signature, and adding a JWT
// dependency for that would be weight without judgement.
package realtime

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Config carries the LiveKit deployment's coordinates. Empty URL means rooms
// are disabled and the API says so rather than minting tokens for a server
// that is not there.
type Config struct {
	URL       string // wss://livekit.example.com
	APIKey    string
	APISecret string
}

func (c Config) Enabled() bool {
	return c.URL != "" && c.APIKey != "" && c.APISecret != ""
}

// VideoGrant is the subset of LiveKit's grant we issue. Rooms are joined,
// published to and subscribed from; recording is never granted (recording is
// off by default and requires all-party consent — docs/plan/16).
type VideoGrant struct {
	RoomJoin     bool   `json:"roomJoin"`
	Room         string `json:"room"`
	CanPublish   bool   `json:"canPublish"`
	CanSubscribe bool   `json:"canSubscribe"`
	CanRecord    bool   `json:"canRecord"`
}

type claims struct {
	Issuer    string     `json:"iss"`
	Subject   string     `json:"sub"`
	Name      string     `json:"name,omitempty"`
	IssuedAt  int64      `json:"iat"`
	NotBefore int64      `json:"nbf"`
	Expires   int64      `json:"exp"`
	Video     VideoGrant `json:"video"`
}

// ErrDisabled is returned when no LiveKit deployment is configured.
var ErrDisabled = errors.New("voice and video rooms are not configured on this instance")

// MintRoomToken signs a short-lived join token for one reader in one room.
// Identity is the user id (opaque to LiveKit, meaningful to us in logs and
// moderation); name is the display handle shown in the room.
func (c Config) MintRoomToken(room, identity, displayName string, canPublish bool, ttl time.Duration) (string, error) {
	if !c.Enabled() {
		return "", ErrDisabled
	}
	if room == "" || identity == "" {
		return "", errors.New("room and identity are required")
	}
	if ttl <= 0 || ttl > 12*time.Hour {
		ttl = 2 * time.Hour
	}
	now := time.Now()
	cl := claims{
		Issuer:    c.APIKey,
		Subject:   identity,
		Name:      displayName,
		IssuedAt:  now.Unix(),
		NotBefore: now.Unix(),
		Expires:   now.Add(ttl).Unix(),
		Video: VideoGrant{
			RoomJoin: true, Room: room,
			CanPublish: canPublish, CanSubscribe: true, CanRecord: false,
		},
	}
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	hb, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	cb, err := json.Marshal(cl)
	if err != nil {
		return "", err
	}
	signing := base64url(hb) + "." + base64url(cb)
	mac := hmac.New(sha256.New, []byte(c.APISecret))
	mac.Write([]byte(signing))
	return signing + "." + base64url(mac.Sum(nil)), nil
}

// VerifyRoomToken checks signature and expiry, returning the claims. It exists
// so tests (and any future server-side check) validate exactly what clients
// present to LiveKit.
func (c Config) VerifyRoomToken(token string) (*claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("malformed token")
	}
	mac := hmac.New(sha256.New, []byte(c.APISecret))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(mac.Sum(nil), mustBase64url(parts[2])) {
		return nil, errors.New("bad signature")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var cl claims
	if err := json.Unmarshal(raw, &cl); err != nil {
		return nil, err
	}
	if time.Now().Unix() > cl.Expires {
		return nil, errors.New("token expired")
	}
	return &cl, nil
}

// RoomName is the single place channel identity becomes a LiveKit room name,
// so rooms and channels can never drift apart: club slug + channel id, both
// stable, both ours.
func RoomName(clubSlug, channelID string) string {
	return "alexandria:" + clubSlug + ":" + channelID
}

// Claims is the exported view tests and handlers use.
type Claims = claims

func base64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func mustBase64url(s string) []byte {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil
	}
	return b
}

