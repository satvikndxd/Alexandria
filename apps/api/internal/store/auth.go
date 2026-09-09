package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/alexandria-reads/alexandria/apps/api/internal/db"
)

// Registration is one atomic onboarding transaction: the account, its empty
// profile, its five system shelves, and its welcome email all land together or
// not at all. A half-registered user is a support ticket waiting to happen.
func (s *Store) RegisterUser(ctx context.Context, username, email string, passwordHash *string) (*db.User, error) {
	var user db.User
	err := s.Tx(ctx, func(q *db.Queries) error {
		u, err := q.CreateUser(ctx, db.CreateUserParams{
			Username:     strings.ToLower(strings.TrimSpace(username)),
			Email:        strings.ToLower(strings.TrimSpace(email)),
			PasswordHash: passwordHash,
		})
		if err != nil {
			return err
		}
		user = u
		if _, err := q.UpsertProfile(ctx, db.UpsertProfileParams{UserID: u.ID}); err != nil {
			return fmt.Errorf("seed profile: %w", err)
		}
		if err := q.EnsureSystemShelves(ctx, u.ID); err != nil {
			return fmt.Errorf("seed shelves: %w", err)
		}
		if err := q.QueueEmail(ctx, db.QueueEmailParams{
			ToEmail:  u.Email,
			Template: "welcome",
			Payload:  []byte(fmt.Sprintf(`{"user_id":%q,"username":%q}`, u.ID, u.Username)),
		}); err != nil {
			return fmt.Errorf("queue welcome email: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// ---- Sessions ----------------------------------------------------------------
// Session tokens are 256-bit random values. Only their SHA-256 hash is stored,
// so a database dump never yields live sessions.

// CreateSession inserts a session row and returns the opaque token to set as a
// cookie. The caller (internal/auth) owns cookie attributes and hashing.
func (s *Store) CreateSession(ctx context.Context, userID uuid.UUID, tokenHash []byte, userAgent *string, ipHash []byte, ttl time.Duration) (*db.Session, error) {
	sess, err := s.q.CreateSession(ctx, db.CreateSessionParams{
		UserID:    userID,
		TokenHash: tokenHash,
		UserAgent: userAgent,
		IpHash:    ipHash,
		ExpiresAt: TS(time.Now().Add(ttl)),
	})
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

// ActiveSession resolves a session token hash to its account state.
// Suspension and deletion are resolved here so no handler can forget them.
func (s *Store) ActiveSession(ctx context.Context, tokenHash []byte) (*db.GetActiveSessionRow, error) {
	row, err := s.q.GetActiveSession(ctx, tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// SessionBlocked turns a resolved session into an authorization verdict. It is
// a function rather than a method because db.GetActiveSessionRow is generated
// (non-local) type: the verdict logic lives here, with the policy, instead of
// being re-derived (or forgotten) in each handler.
func SessionBlocked(row *db.GetActiveSessionRow) bool {
	if row.UserDeletedAt.Valid {
		return true
	}
	return row.SuspendedUntil.Valid && row.SuspendedUntil.Time.After(time.Now())
}

func (s *Store) ExtendSession(ctx context.Context, id uuid.UUID, ttl time.Duration) error {
	_, err := s.q.ExtendSession(ctx, db.ExtendSessionParams{
		ID:        id,
		ExpiresAt: TS(time.Now().Add(ttl)),
	})
	return err
}

func (s *Store) RevokeSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.q.RevokeSessionByToken(ctx, tokenHash)
	return err
}

func (s *Store) RevokeAllSessions(ctx context.Context, userID uuid.UUID) error {
	_, err := s.q.RevokeSessionsForUser(ctx, userID)
	return err
}

// ---- Auth ceremonies ----------------------------------------------------------

func (s *Store) CreateChallenge(ctx context.Context, kind db.AuthChallengeKind, userID *uuid.UUID, email *string, challenge, sessionData []byte, ttl time.Duration) (*db.AuthChallenge, error) {
	// WebAuthn ceremonies carry serialized SessionData; magic links do not.
	// The column is NOT NULL DEFAULT '{}', and passing SQL NULL would violate
	// it, so the absence is expressed as an empty object instead.
	if sessionData == nil {
		sessionData = []byte("{}")
	}
	c, err := s.q.CreateAuthChallenge(ctx, db.CreateAuthChallengeParams{
		Kind:        kind,
		UserID:      UUIDPtr(userID),
		Email:       email,
		Challenge:   challenge,
		SessionData: sessionData,
		ExpiresAt:   TS(time.Now().Add(ttl)),
	})
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ConsumeChallenge atomically marks a ceremony single-use. A replayed token
// loses the race and returns ErrNotFound.
func (s *Store) ConsumeChallenge(ctx context.Context, id uuid.UUID, kind db.AuthChallengeKind) (*db.AuthChallenge, error) {
	c, err := s.q.ConsumeAuthChallenge(ctx, db.ConsumeAuthChallengeParams{ID: id, Kind: kind})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ThrottleAuth counts attempts in a trailing window. Buckets are hashed keys
// (email or IP) computed by internal/auth; raw values are never persisted.
func (s *Store) ThrottleAuth(ctx context.Context, kind db.AuthChallengeKind, bucket string, limit int, window time.Duration) (allowed bool, err error) {
	n, err := s.q.CountAuthAttemptsSince(ctx, db.CountAuthAttemptsSinceParams{
		BucketKey: bucket,
		Kind:      kind,
		Since:     TS(time.Now().Add(-window)),
	})
	if err != nil {
		return false, err
	}
	if err := s.q.RecordAuthAttempt(ctx, db.RecordAuthAttemptParams{
		Kind: kind, BucketKey: bucket, Allowed: n < int64(limit),
	}); err != nil {
		return false, err
	}
	return n < int64(limit), nil
}

// ---- Profiles ------------------------------------------------------------------

func (s *Store) PublicProfile(ctx context.Context, username string) (*db.GetPublicProfileByUsernameRow, error) {
	row, err := s.q.GetPublicProfileByUsername(ctx, strings.ToLower(username))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Store) UpdateProfile(ctx context.Context, userID uuid.UUID, displayName, bio string, pronouns, location *string, isPrivate bool) (*db.Profile, error) {
	p, err := s.q.UpsertProfile(ctx, db.UpsertProfileParams{
		UserID:      userID,
		DisplayName: displayName,
		Bio:         bio,
		Pronouns:    pronouns,
		Location:    location,
		IsPrivate:   isPrivate,
	})
	if err != nil {
		return nil, err
	}
	return &p, nil
}
