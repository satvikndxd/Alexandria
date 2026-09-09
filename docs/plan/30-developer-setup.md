# 30 — Developer Setup Guide

**Status:** implemented · the whole platform on one laptop

## Prerequisites
Go 1.24+, Node 22+, Docker + Compose, Flutter 3.x (mobile only),
sqlc 1.27 (only when editing `packages/db/queries`),
`psycopg2-binary` (only for the smoke script).

## 1 · Services
```sh
docker compose -f infrastructure/docker-compose.yml up -d   # PG16, NATS, Meili, MinIO
```

## 2 · Schema
```sh
cd apps/api
go run ./cmd/migrate up      # embedded, checksummed, forward-only
go run ./cmd/migrate status  # exit 1 when pending (deploy gate)
```

## 3 · Server
```sh
go run ./cmd/api      # :8080  /healthz reports search degradation honestly
go run ./cmd/worker   # ingestion · mail · search indexing
```

## 4 · Web (the BFF edge)
```sh
npm install
ALEXANDRIA_API_URL=http://localhost:8080 npm run dev --workspace @alexandria/web
```
Next bakes `/api/v1/*` rewrites at **build** time — set the env for
`next build` too. Without it, pages render labelled public-domain fixtures.

## 5 · Tests
```sh
cd apps/api && go test ./...                     # unit; skips integration politely
ALEXANDRIA_TEST_DATABASE_URL=postgres://alexandria:alexandria@localhost:5432/alexandria?sslmode=disable \
  go test ./internal/httpapi/                    # real PG, RLS, CSRF, friction
python3 infrastructure/smoke/e2e_smoke.py        # whole loop, no browser
```

## Changing the schema
1. New `packages/db/migrations/NNNN_*.sql` (never edit applied files — the
   runner refuses edited history; reset throwaway DBs instead).
2. Edit/add `packages/db/queries/*.sql`.
3. `cd packages/db && sqlc generate` (output is committed; CI drift-checks).
4. Store wrappers only for multi-statement or RLS-scoped operations.

## Conventions that get PRs merged
Sharp corners, hard offsets, no emoji icons; SQL in sqlc files only; friction
constants changed only via RFC; every server invariant arrives with an
integration test; every UI state arrives with its empty and degraded forms.
