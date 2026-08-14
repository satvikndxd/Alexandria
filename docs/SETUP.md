# Developer Setup

## Prerequisites
- Go 1.22+
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

## 2 · API (host)

```sh
cd apps/api
go run ./cmd/api      # http://localhost:8080/healthz
go run ./cmd/worker   # ingestion consumer (needs NATS)
go test ./...
```

All configuration is env vars with compose-matching defaults — see
`internal/config/config.go`.

## 3 · Web

```sh
npm install
npm run dev --workspace @alexandria/web   # http://localhost:3000
```

Without `ALEXANDRIA_API_URL` the web app renders seeded public-domain
fixtures, so UI work needs no backend.

## 4 · Mobile

```sh
cd apps/mobile
flutter pub get
flutter run
flutter test
```

## Changing the schema
1. Add a new `packages/db/migrations/NNNN_*.sql` (never edit applied ones).
2. Update/add queries in `packages/db/queries/`.
3. `cd packages/db && sqlc generate`, reconcile `apps/api/internal/store`.
4. CI applies every migration to a fresh Postgres 16 to catch ordering bugs.

## Conventions
- Sharp corners, hard offset shadows, no emojis as icons — see
  `apps/web/tailwind.config.ts` for the enforced tokens.
- Friction constants live in `packages/core/src/index.ts` and
  `apps/api/internal/domain` — change both or CI (planned check) fails.
