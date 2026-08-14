# ADR 0003 — sqlc + pgx, no ORM

**Status:** Accepted · 2026-08-14

## Context
GORM/Ent hide the SQL that determines whether Alexandria survives its first
popular book page. We want every query visible, reviewable, and matched to an
index documented in the migrations.

## Decision
- Raw SQL lives in `packages/db/queries/*.sql` (the source of truth) and is
  compiled to type-safe Go with `sqlc generate` into `apps/api/internal/store`.
- The checked-in store package is hand-tuned to the same SQL where sqlc's
  output needs shaping (json aggregation, tx composition); the queries files
  keep it honest and regenerable.
- Aggregates that would need `COUNT(*)`/`AVG()` scans on hot paths
  (work ratings) are denormalized and maintained by triggers.

## Consequences
- Query review happens in code review, in SQL, next to the schema.
- Postgres-only. Accepted: Alexandria will never target MySQL.
