package auth

import (
	"context"
	"crypto/hmac"
	crand "crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/alexandria-reads/alexandria/apps/api/internal/db"
	"github.com/alexandria-reads/alexandria/apps/api/internal/store"
)

// MagicLinks is the fallback factor for readers without a passkey: a
// single-use, 15-minute link delivered over the transactional email outbox.
//
// Anti-oracle rule: requesting a link for an unknown address returns the exact
// same response as a known one and queues nothing. An attacker therefore
// cannot use the login flow to enumerate the user table, and a harassed mailbox
// receives no mail it did not ask for.
type MagicLinks struct {
	st      *store.Store
	origin  string // e.g. https://alexandria.example — links are absolute
	pepper  []byte
	cookies CookiePolicy
}

func NewMagicLinks(st *store.Store, origin string, secure bool) *MagicLinks {
	return &MagicLinks{
		st:      st,
		origin:  strings.TrimSuffix(origin, "/"),
		pepper:  []byte("alexandria-magiclink-pepper"),
		cookies: DefaultCookiePolicy(secure),
	}
}

// Request queues a magic link if — and only if — the address has an account.
// The response is identical either way.
func (m *MagicLinks) Request(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil // indistinguishable from "queued"
	}
	bucket := BucketKey(m.pepper, "magic_link", email)
	allowed, err := m.st.ThrottleAuth(ctx, db.AuthChallengeKindMagicLink, bucket,
		MagicLinkMaxPerHour, time.Hour)
	if err != nil {
		return err
	}
	if !allowed {
		// Still pretend success: throttling must not be observable either.
		return nil
	}

	user, err := m.st.Queries().GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if user.DeletedAt.Valid || (user.SuspendedUntil.Valid && user.SuspendedUntil.Time.After(time.Now())) {
		return nil // suspended or deleted accounts get no links, silently
	}

	secret := make([]byte, 32)
	if _, err := crand.Read(secret); err != nil {
		return fmt.Errorf("magic link entropy: %w", err)
	}
	ch, err := m.st.CreateChallenge(ctx, db.AuthChallengeKindMagicLink, &user.ID, &email,
		hmacSum(m.pepper, secret), nil, MagicLinkTTL)
	if err != nil {
		return err
	}
	token := ch.ID.String() + "." + base64.RawURLEncoding.EncodeToString(secret)
	link := m.origin + "/signin/verify?token=" + token

	payload, err := json.Marshal(map[string]any{
		"user_id":  user.ID,
		"username": user.Username,
		"url":      link,
		"ttl_min":  int(MagicLinkTTL.Minutes()),
	})
	if err != nil {
		return err
	}
	return m.st.Queries().QueueEmail(ctx, db.QueueEmailParams{
		ToEmail:  email,
		Template: "magic_link",
		Payload:  payload,
	})
}

// Verify consumes a magic-link token and returns the account it authenticates.
func (m *MagicLinks) Verify(ctx context.Context, token string) (uuid.UUID, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return uuid.Nil, ErrInvalidToken
	}
	id, err := uuid.Parse(parts[0])
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}
	secret, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(secret) != 32 {
		return uuid.Nil, ErrInvalidToken
	}

	ch, err := m.st.ConsumeChallenge(ctx, id, db.AuthChallengeKindMagicLink)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}
	want := hmacSum(m.pepper, secret)
	if subtle.ConstantTimeCompare(ch.Challenge, want) != 1 {
		return uuid.Nil, ErrInvalidToken
	}
	uid := store.UUIDGet(ch.UserID)
	if uid == nil {
		return uuid.Nil, ErrInvalidToken
	}
	return *uid, nil
}

// MintSessionForUser creates a session after a successful fallback factor and
// returns the opaque token for the cookie layer.
func (m *MagicLinks) MintSessionForUser(ctx context.Context, userID uuid.UUID, userAgent string, ip string) (string, error) {
	token, hash, err := NewSessionToken()
	if err != nil {
		return "", err
	}
	if _, err := m.st.CreateSession(ctx, userID, hash, strPtr(userAgent), hashIP(ip), SessionTTL); err != nil {
		return "", err
	}
	if err := m.st.Queries().RecordLogin(ctx, userID); err != nil {
		return "", err
	}
	return token, nil
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// hmacSum derives the stored form of a magic-link secret: the raw secret
// travels in the email, only its keyed hash is persisted, so a database
// disclosure cannot be replayed as a login.
func hmacSum(key, msg []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(msg)
	return mac.Sum(nil)
}
