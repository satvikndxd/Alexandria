# Developer Setup

## Prerequisites
- Go 1.24+
- Node 22+ (npm workspaces)
- Docker + Docker Compose
- Flutter 3.x (only for `apps/mobile`)
- `sqlc` (only when changing `packages/db/queries`)

## 1 · Backing services

```sh
docker compose -f infrastructure/docker-compose.yml up -d
```

Brings up Postgres 16 (migrations auto-applied on first boot), NATS JetStream,
Meilisearch, and MinIO. Add `--profile full` to also build and run the Go API
and worker in containers.

## 2 · Schema

```sh
cd apps/api
go run ./cmd/migrate up       # applies the embedded schema (checksummed)
go run ./cmd/migrate status   # exit 1 when migrations are pending
```

Compose applies migrations on first boot for convenience; `cmd/migrate` is
what staging/production and the integration suite use. Migrations are
forward-only: a mistaken migration means resetting a throwaway database,
never editing an applied file (the runner refuses edited history).

## 3 · API (host)

```sh
cd apps/api
go run ./cmd/api      # http://localhost:8080/healthz
go run ./cmd/worker   # ingestion · mail · search indexing (needs NATS)
go run ./cmd/reindex  # rebuild Meilisearch from Postgres
go test ./...         # unit tests; no services required
```

Integration tests need a live Postgres and are skipped without it:

```sh
ALEXANDRIA_TEST_DATABASE_URL=postgres://alexandria:alexandria@localhost:5432/alexandria?sslmode=disable \
  go test ./internal/httpapi/
```

They connect as the least-privilege `alexandria_app` role on purpose:
superusers bypass Row-Level Security, so testing RLS as the owner would prove
nothing.

All configuration is env vars with compose-matching defaults — see
`internal/config/config.go` and `.env.example`.

## 4 · Web

```sh
npm install
npm run dev --workspace @alexandria/web   # http://localhost:3000
```

Without `ALEXANDRIA_API_URL` the web app renders seeded public-domain
fixtures, so UI work needs no backend.

The web app is the edge (BFF): browser traffic reaches the monolith through
same-origin rewrites (`/api/v1/*` → `ALEXANDRIA_API_URL/v1/*`), so session and
CSRF cookies stay first-party and there is no CORS anywhere. **Next bakes
rewrites into the routes manifest at build time**, so set `ALEXANDRIA_API_URL`
for `next build` too, not only at runtime.

End-to-end smoke (auth → CSRF → library → SSR), no browser needed:

```sh
python3 infrastructure/smoke/e2e_smoke.py   # needs psycopg2-binary + running stack
```

## 5 · Mobile

```sh
cd apps/mobile
flutter pub get
flutter run
flutter test
```

## Changing the schema
1. Add a new `packages/db/migrations/NNNN_*.sql` (never edit applied ones).
2. Update/add queries in `packages/db/queries/`.
3. `cd packages/db && sqlc generate` — output is committed at
   `apps/api/internal/db`; CI fails on regeneration drift.
4. Add store wrappers only for multi-statement/RLS-scoped operations
   (`internal/store`); never hand-write SQL in Go.
5. CI applies every migration to a fresh Postgres 16 and runs the integration
   suite against it.

## Conventions
- Sharp corners, hard offset shadows, no emojis as icons — see
  `apps/web/tailwind.config.ts` for the enforced tokens.
- Friction constants live in `packages/core/src/index.ts` and
  `apps/api/internal/domain` — change both or CI (planned check) fails.
