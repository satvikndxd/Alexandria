# ADR 0008 — Forward-only, checksummed, embedded migrations

**Status:** Accepted · 2026-09-09

## Context
Schema changes that differ between environments are the most expensive class
of bug: they surface in production, under load, as "works on my machine".
golang-migrate et al. would add a dependency and a second source of truth next
to the SQL we already review.

## Decision
- Migrations are plain numbered SQL in `packages/db/migrations`, embedded at
  build time (`packages/db/migrations/migrations.go`) and applied by
  `apps/api/cmd/migrate` — ~120 lines of stdlib + pgx, no dependency.
- **Forward-only.** There is no `Down()`. Rollback in production is a restore
  or a compensating forward migration; generated down-migrations give false
  safety and are routinely wrong about data.
- **One transaction per file** (Postgres DDL is transactional; we use it).
- **Checksummed.** Editing an already-applied migration is a hard error
  (`ErrHistoryEdited`), so environments cannot silently diverge from git
  history. The dev workflow for a mistaken migration is to reset the
  throwaway database, not to edit history.
- **Advisory-locked**, so two pods booting at once serialize instead of racing.
- `migrate status` exits non-zero when migrations are pending, making it a
  deploy gate.

## Consequences
- A released binary carries the exact schema it was compiled against.
- The integration suite applies migrations through this same runner, so the
  tool is tested by every CI run rather than only in production.
- Contributors cannot "fix" an applied migration quietly; the refusal is loud
  and explains itself.
