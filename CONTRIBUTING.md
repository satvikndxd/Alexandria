# Contributing to Alexandria

Thank you for helping build a human-first library. A few house rules keep the
project coherent:

## Ground rules
1. **No generative-AI content in the product.** Code assistance is your
   business; AI-written reviews, summaries, scholar notes, or "sample data"
   posing as human activity will be rejected. Seed/demo content must be
   labelled as such (see `isSeed` in fixtures).
2. **The aesthetic is law.** Parchment/ink/botanical/vermilion/gold, sharp
   corners, hard print-block shadows, EB Garamond + UnifrakturMaguntia,
   engraved SVG icons. If a screen could belong to a generic SaaS app,
   redesign it. Tokens: `apps/web/tailwind.config.ts`,
   `apps/mobile/lib/theme/tokens.dart`, `packages/core/src/index.ts`.
3. **Explicit SQL only.** No ORMs. Queries live in `packages/db/queries` and
   ship with the indexes they rely on.
4. **Slow work goes through NATS.** Never call Open Library, Gutenberg, or
   any third party from an HTTP handler.
5. **Respect the sources.** Identifying User-Agents, paced requests, official
   catalogs only, and every cover asset carries its license row.

## Workflow
- Architectural changes start with an ADR in `docs/adr/`.
- `gofmt`, `go vet`, `go test -race`, `tsc --noEmit`, and the migration check
  must pass (see `.github/workflows/ci.yml`).
- Commits: imperative subject, body explains *why*.

## Licensing
- Backend, web, infrastructure: **AGPL-3.0-only** (root `LICENSE`).
- Flutter client + design tokens: **MIT** (`apps/mobile/LICENSE`) to
  encourage reuse of the design system.
- By contributing you agree your contribution is licensed accordingly.
