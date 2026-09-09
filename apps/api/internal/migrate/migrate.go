// Package migrate applies the embedded Alexandria schema.
//
// Design decisions, all deliberate:
//
//   - Forward-only. There is no Down(). A rollback in production is a restore
//     or a compensating forward migration; auto-generated down-migrations give
//     a false sense of safety and are routinely wrong about data.
//   - Checksummed. Editing an already-applied migration is a hard error, so
//     environments cannot silently diverge from git history.
//   - Advisory-locked. Two API pods booting at once must not race; the second
//     waits for the first and then finds nothing to do.
//   - One transaction per file. A migration either lands completely or not at
//     all — Postgres supports transactional DDL, so we use it.
package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alexandria-reads/alexandria/packages/db/migrations"
)

// lockKey is an arbitrary but stable 64-bit key namespacing Alexandria
// migrations inside pg_advisory_lock. Changing it would let two versions of the
// tool run concurrently, so it is a constant, not configuration.
const lockKey int64 = 0x41_4c_45_58_4d_47_52_31 // "ALEXMGR1"

// ErrHistoryEdited means an applied migration's contents no longer match what
// was recorded. Deploying anyway would leave the schema undefined.
var ErrHistoryEdited = errors.New("applied migration has been modified after the fact")

// Record is one row of schema_migrations.
type Record struct {
	Version   int
	Name      string
	Checksum  string
	AppliedAt time.Time
}

// Status pairs an embedded migration with whether it has been applied.
type Status struct {
	Migration migrations.Migration
	Applied   *Record
}

// Migrator runs migrations over a pool. The pool SHOULD be configured with
// pgx.QueryExecModeSimpleProtocol: migration files contain multiple statements
// and dollar-quoted function bodies, which the extended protocol rejects.
type Migrator struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Migrator { return &Migrator{pool: pool} }

// ConnectPool builds a pool suitable for migrations (simple protocol, single
// connection — migrations are serial by nature).
func ConnectPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.MaxConns = 1
	cfg.MinConns = 0
	cfg.MaxConnLifetime = 5 * time.Minute
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	return pgxpool.NewWithConfig(ctx, cfg)
}

const ddl = `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version     integer PRIMARY KEY,
  name        text NOT NULL UNIQUE,
  checksum    text NOT NULL,
  applied_at  timestamptz NOT NULL DEFAULT now(),
  duration_ms bigint NOT NULL DEFAULT 0
)`

// Up applies every pending migration in version order and returns the names of
// the migrations it applied.
func (m *Migrator) Up(ctx context.Context) ([]string, error) {
	all, err := migrations.All()
	if err != nil {
		return nil, err
	}

	conn, err := m.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Release()

	// Held for the whole run; released by session end as a backstop.
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", lockKey); err != nil {
		return nil, fmt.Errorf("acquire advisory lock: %w", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", lockKey)
	}()

	if _, err := conn.Exec(ctx, ddl); err != nil {
		return nil, fmt.Errorf("create schema_migrations: %w", err)
	}

	applied, err := m.records(ctx, conn)
	if err != nil {
		return nil, err
	}

	var done []string
	for _, mig := range all {
		prev, seen := applied[mig.Version]
		sum := Checksum(mig.SQL)
		if seen {
			if prev.Checksum != sum {
				return done, fmt.Errorf("%w: %s (recorded %s, file %s)",
					ErrHistoryEdited, mig.Name, prev.Checksum, sum)
			}
			continue
		}
		if err := m.apply(ctx, conn, mig, sum); err != nil {
			return done, err
		}
		done = append(done, mig.Name)
	}
	return done, nil
}

func (m *Migrator) apply(ctx context.Context, conn *pgxpool.Conn, mig migrations.Migration, sum string) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin %s: %w", mig.Name, err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	start := time.Now()
	if _, err := tx.Exec(ctx, mig.SQL); err != nil {
		return fmt.Errorf("apply %s: %w", mig.Name, err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO schema_migrations (version, name, checksum, duration_ms) VALUES ($1, $2, $3, $4)`,
		mig.Version, mig.Name, sum, time.Since(start).Milliseconds()); err != nil {
		return fmt.Errorf("record %s: %w", mig.Name, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit %s: %w", mig.Name, err)
	}
	return nil
}

// Status reports every embedded migration and whether it has been applied.
func (m *Migrator) Status(ctx context.Context) ([]Status, error) {
	all, err := migrations.All()
	if err != nil {
		return nil, err
	}
	conn, err := m.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, ddl); err != nil {
		return nil, err
	}
	applied, err := m.records(ctx, conn)
	if err != nil {
		return nil, err
	}
	out := make([]Status, 0, len(all))
	for _, mig := range all {
		rec := applied[mig.Version]
		out = append(out, Status{Migration: mig, Applied: rec})
	}
	return out, nil
}

func (m *Migrator) records(ctx context.Context, conn *pgxpool.Conn) (map[int]*Record, error) {
	rows, err := conn.Query(ctx,
		`SELECT version, name, checksum, applied_at FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	defer rows.Close()
	out := map[int]*Record{}
	for rows.Next() {
		var r Record
		if err := rows.Scan(&r.Version, &r.Name, &r.Checksum, &r.AppliedAt); err != nil {
			return nil, err
		}
		out[r.Version] = &r
	}
	return out, rows.Err()
}

// Checksum is the content hash recorded for a migration.
func Checksum(sql string) string {
	sum := sha256.Sum256([]byte(sql))
	return hex.EncodeToString(sum[:])
}

// SortStatuses is exported for the CLI so output is deterministic.
func SortStatuses(s []Status) {
	sort.Slice(s, func(i, j int) bool { return s[i].Migration.Version < s[j].Migration.Version })
}
