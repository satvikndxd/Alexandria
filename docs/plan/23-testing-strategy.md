# 23 — Testing Strategy

**Status:** implemented · the pyramid we actually maintain

## Layers
1. **Domain unit tests** (`internal/domain/*_test.go`, `internal/auth/*_test.go`):
   friction rules (rune-based lengths, filler heuristics), identity validation,
   session/CSRF hardening, token properties. No I/O.
2. **Integration tests** (`internal/httpapi/integration_*_test.go`): real
   PostgreSQL 16, migrations applied by the production runner, API exercised
   as the least-privilege role. They cover the invariants a mock cannot see:
   RLS cross-reader invisibility, CSRF enforcement, spoiler byte-withholding,
   daily caps, single-use ceremonies, mailbox non-oracle, shelf/progress
   agreement, club spoiler gating, migration idempotency.
   Skipped politely without `ALEXANDRIA_TEST_DATABASE_URL`.
3. **End-to-end smoke** (`infrastructure/smoke/e2e_smoke.py`): browserless,
   drives web edge → monolith → Postgres: magic-link ceremony, CSRF 403,
   shelving, progress, SSR pages, review friction accept/refuse.
4. **Web**: `tsc --noEmit` + `next build` as type/route gates; **Playwright E2E**
   (`apps/web/e2e`) against a live stack: folio chrome, lost-folio page, search
   surface, and the reader's typography persistence (size/theme survive reload;
   the ink theme's surface is asserted after its transition settles). Specs
   needing seeded data skip honestly without it; sandbox/CI browsers select the
   headless shell and jitless VAPID-free flags via PW_CHANNEL/PW_JITLESS.
5. **Mobile**: `flutter analyze` + widget tests for the drop-cap painter.
6. **Schema**: CI applies every migration to a fresh Postgres; sqlc drift
   check fails the build.

## Coverage philosophy
Percentages are a smell test, not a target. We demand coverage where the cost
of being wrong is a privacy leak or a lie to the reader: RLS, auth, friction,
spoilers, money (affiliate disclosure), and migrations. A green suite that
misses those is worse than none — it buys false confidence.

## Fixtures honesty
Web fixtures are public-domain works and clearly-labelled demonstration
seeds; the UI prints "Demonstration seeds" so the platform never presents
invented activity as live.
