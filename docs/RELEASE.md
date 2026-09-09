# Release Runbook

**Status:** living · Fastlane lanes live in `apps/mobile/{android,ios}/fastlane`

## Principles
- **Tags are the source of truth:** `v<major>.<minor>.<patch>` on `main`; every
  tag builds the same artifacts CI already built for the commit.
- **Secrets only in CI:** Play service-account JSON, Apple IDs, and signing
  certificates are CI secrets. Fastlane files reference environment variables
  and nothing else; a cloned repo can build but never publish.
- **Migrations precede deploys:** `cmd/migrate up` runs as an init container
  and `cmd/migrate status` is a gate (non-zero exit blocks the rollout).
  Migrations are forward-only, so rollback is a code rollback, never schema.
- **Mobile ships behind the same API contract:** the /v1 surface is versioned;
  a breaking change means a new prefix and a deprecation window, because
  installed apps cannot be recalled.

## Android internal beta
```sh
cd apps/mobile/android && bundle exec fastlane beta   # PLAY_JSON_KEY from CI
```
Produces `app-release.aab` on the Play internal track.

## iOS TestFlight beta
```sh
cd apps/mobile/ios && bundle exec fastlane beta       # APPLE_ID/TEAM_ID from CI
```

## Desktop (Linux/macOS/Windows)
`flutter build linux|macos|windows --release`; artifacts attach to the GitHub
Release for the tag. No store involved; signatures per platform toolchain.

## Server
1. Tag → CI builds `api`/`web` images → GHCR.
2. Staging rollout; smoke script (`infrastructure/smoke/e2e_smoke.py`) green.
3. Production rollout; `cmd/migrate up` init container; watch
   `/metrics` (friction refusals, outbox depth) for one hour.

## Rollback
Re-deploy the previous tag. Schema is forward-only: compensating forward
migrations only, written as new numbered files, never edits to applied ones.
