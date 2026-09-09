# 24 — CI/CD

**Status:** implemented · `infrastructure/ci/github-actions-ci.yml`

## Pipeline (every push and PR)
1. **api**: gofmt (no diffs) → `go vet` → `go build` → unit tests (`-race`).
2. **integration**: Postgres 16 service → `cmd/migrate up` + `status` →
   provision `alexandria_app` → integration suite with `-race -count=1`.
3. **sqlc**: regenerate and `git diff --exit-code` on `apps/api/internal/db`
   — drift between SQL and shipped Go fails the build.
4. **web**: `npm ci` → `tsc --noEmit` → `next build`.
5. **mobile**: `flutter analyze` + `flutter test`.

## Deploy
- Staging: build Docker images (`infrastructure/api.Dockerfile`,
  `web.Dockerfile`), push to GHCR, roll to the staging K3s namespace;
  `cmd/migrate up` runs as an init container (gate: `status` must exit 0).
- Production: same images, tagged by SHA; manual promotion; migrations forward
  only, so rollback is a code rollback, never a schema rollback
  ([ADR 0008](../adr/0008-embedded-forward-only-migrations.md)).
- Mobile: Fastlane beta tracks ⏳ Phase 7.

## Rules we enforce in review, not in tooling
No hand-edits under `internal/db/`; no SQL strings in Go; no weakening of
`packages/core` friction constants without an RFC; no new third-party
analytics imports (a grep in review).
