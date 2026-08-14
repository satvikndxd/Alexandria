# @alexandria/db

Canonical PostgreSQL 16 schema, migrations, and `sqlc` query definitions.

## Layout

```
migrations/   Ordered SQL migrations (run with `make migrate` or tern/goose)
queries/      Raw SQL consumed by sqlc → generates apps/api/internal/store
seed/         Development seed data (public-domain classics)
sqlc.yaml     Codegen config (pgx/v5, typed UUIDs)
```

## Design decisions

- **FRBR**: `works` (abstract) → `editions` (manifestations). Reviews and
  scholar notes attach to works; covers and reading progress to editions.
- **Friction in the schema**: minimum body lengths, one-review-per-work, and
  the `contribution_ledger` posting-cap window are `CHECK`s and constraints —
  they cannot be bypassed by a buggy handler.
- **Rights-aware covers**: `cover_assets` separates *source* from *license*
  from *cache policy*. Nothing is cached to MinIO unless `cacheable = true`.
- **Transactional outbox**: domain writes and event publication are atomic;
  a relay drains `outbox` into NATS JetStream.
- **Soft deletes** (`deleted_at`) on user-visible content; hard purges happen
  in scheduled jobs to honour data-deletion requests.

## Regenerating the store

```sh
cd packages/db
sqlc generate   # writes apps/api/internal/store
```
