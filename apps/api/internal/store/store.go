// Package store is the data-access layer of the modular monolith.
//
// Every query in the system is written once, as SQL, in packages/db/queries and
// compiled to type-safe Go by sqlc into internal/db. This package adds exactly
// three things on top:
//
//  1. connection/pool lifecycle with production-shaped defaults
//  2. transaction helpers that set the Row-Level-Security context
//     (app.user_id / app.service) with SET LOCAL so it cannot leak between
//     pooled connections
//  3. multi-statement domain operations (review + ledger, registration +
//     shelves, ingest job + outbox) that must be atomic
//
// There is no ORM and no query builder: if a query is not visible in SQL, it
// does not ship.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alexandria-reads/alexandria/apps/api/internal/db"
)

// ErrNotFound is returned when a lookup misses; handlers map it to 404.
var ErrNotFound = errors.New("not found")

// ErrSelfAction rejects operations that target the actor's own account
// (following yourself, blocking yourself): meaningless, and a classic source
// of graph cycles that feed logic then has to defend against.
var ErrSelfAction = errors.New("you cannot target your own account with this action")

// ErrBlocked rejects social operations across a block edge in either
// direction. A block is not an invitation to be replied to.
var ErrBlocked = errors.New("this interaction is not permitted")

type Store struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, q: db.New(pool)}
}

// Queries exposes the generated query set for single-statement operations.
func (s *Store) Queries() *db.Queries { return s.q }

func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// Connect opens a pool with conservative, production-shaped settings.
func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.MaxConns = 16
	cfg.MinConns = 1
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second
	return pgxpool.NewWithConfig(ctx, cfg)
}

// Tx runs fn inside a transaction as the service context. Use it for writes
// that are not scoped to a single reader (ingestion, moderation, public reads).
func (s *Store) Tx(ctx context.Context, fn func(q *db.Queries) error) error {
	return s.tx(ctx, nil, false, fn)
}

// TxUser runs fn inside a transaction with Row-Level Security scoped to
// userID. This is the ONLY way request-path code should touch RLS-covered
// tables (shelves, shelf_items, reading_sessions, annotations, notifications).
func (s *Store) TxUser(ctx context.Context, userID uuid.UUID, fn func(q *db.Queries) error) error {
	return s.tx(ctx, &userID, false, fn)
}

// ReadUser scopes a read-only transaction to a reader. Same guarantees as
// TxUser without a write intent.
func (s *Store) ReadUser(ctx context.Context, userID uuid.UUID, fn func(q *db.Queries) error) error {
	return s.tx(ctx, &userID, true, fn)
}

func (s *Store) tx(ctx context.Context, userID *uuid.UUID, readOnly bool, fn func(q *db.Queries) error) error {
	opts := pgx.TxOptions{}
	if readOnly {
		opts.AccessMode = pgx.ReadOnly
	}
	tx, err := s.pool.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after a successful commit

	qtx := s.q.WithTx(tx)

	// SET LOCAL dies with the transaction: a pooled connection returned to the
	// pool can never carry the previous tenant's identity.
	if userID != nil {
		if _, err := tx.Exec(ctx, "SELECT set_config('app.user_id', $1, true)", userID.String()); err != nil {
			return fmt.Errorf("set rls user context: %w", err)
		}
	} else {
		// Explicitly clear any inherited value; paranoia is cheap here.
		if _, err := tx.Exec(ctx, "SELECT set_config('app.user_id', '', true), set_config('app.service', 'on', true)"); err != nil {
			return fmt.Errorf("set service context: %w", err)
		}
	}

	if err := fn(qtx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// ---- pgtype helpers ---------------------------------------------------------
// Nullable Postgres types are pgtype wrappers in the generated code. These
// conversions keep the rest of the codebase in plain Go types.

func TS(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t.UTC(), Valid: !t.IsZero()}
}

func TSPtr(t *time.Time) pgtype.Timestamptz {
	if t == nil || t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return TS(*t)
}

func TSGet(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time.UTC()
	return &t
}

func UUIDPtr(u *uuid.UUID) pgtype.UUID {
	if u == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: [16]byte(*u), Valid: true}
}

func UUIDGet(v pgtype.UUID) *uuid.UUID {
	if !v.Valid {
		return nil
	}
	u := uuid.UUID(v.Bytes)
	return &u
}

func I32Ptr(v *int32) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *v, Valid: true}
}

func I32Narg(v *int32) pgtype.Int4 { return I32Ptr(v) }

func DatePtr(d *time.Time) pgtype.Date {
	if d == nil || d.IsZero() {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: d.UTC(), Valid: true}
}

func DateGet(v pgtype.Date) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time.UTC()
	return &t
}

// Narg converts an optional string to the nullable shape sqlc generated for
// sqlc.narg parameters.
func Str(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func StrGet(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
