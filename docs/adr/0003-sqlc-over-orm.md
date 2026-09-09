# ADR 0003 — sqlc + pgx, no ORM

**Status:** Accepted · 2026-08-14 · *Amended 2026-09-09*

## Context
GORM/Ent hide the SQL that determines whether Alexandria survives its first
popular book page. We want every query visible, reviewable, and matched to an
index documented in the migrations.

## Decision
- Raw SQL lives in `packages/db/queries/*.sql` and is the single source of
  truth. `sqlc generate` compiles it into `apps/api/internal/db` (package
  `db`), which is **committed** so builds never require the sqlc binary and
  code review can read exactly what ships. CI fails on regeneration drift.
- `internal/store` adds exactly three things on top of the generated queries:
  pool lifecycle, transaction helpers that set the Row-Level-Security context
  (`TxUser` / `ReadUser` / `Tx`), and multi-statement domain operations that
  must be atomic (review + contribution ledger, registration + shelves,
  ingest job + outbox event). No SQL is hand-written in Go any more; the
  earlier duplication of queries between `store.go` and `queries/*.sql` was
  removed in the amendment.
- `packages/db` is its own Go module so `go:embed` can carry the migrations
  into `apps/api/cmd/migrate`: a released binary always applies the exact
  schema it was compiled against.
- Aggregates that would need `COUNT(*)`/`AVG()` scans on hot paths (work
  ratings, review like counts) are denormalized and maintained by triggers or
  on-write refreshes.

## Consequences
- Query review happens in code review, in SQL, next to the schema.
- Generated files are never edited by hand; a PR touching `internal/db`
  without touching `queries/` or `migrations/` is a review red flag.
- Postgres-only. Accepted: Alexandria will never target MySQL.
