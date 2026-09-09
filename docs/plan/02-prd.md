# 02 — Product Requirements Document

**Status:** living document · derived from implemented behaviour where marked ✅

## Core loop
Discover → Track → Read → Annotate → Discuss.

| Stage | Requirement | State |
|---|---|---|
| Discover | Catalogue browse by subject/era/public-domain; global search over works, authors, clubs; explainable related-works | ✅ `GET /v1/works`, `/v1/search`, `/v1/works/{slug}/related` |
| Track | Five system shelves + custom shelves; edition & format tracking; progress in basis points; rereads as new sessions; DNF as honest data | ✅ `packages/db/migrations/0004_library.sql` |
| Read | Public-domain reader with typography controls, bookmarks, highlights | ⏳ Phase 3 (text pipeline specified in [12](12-gutenberg-integration.md)) |
| Annotate | Private highlights/notes anchored by chapter + offsets; RLS-private | ✅ schema + API; reader UI ⏳ |
| Discuss | Reviews with friction; comments; clubs with chapter-gated channels | ✅ reviews/clubs; voice/video ⏳ |

## Functional requirements (condensed)
- FR-1 Accounts: passkey-first registration/login, magic-link fallback, no
  passwords. ✅ ([ADR 0006](../adr/0006-passkey-first-auth.md))
- FR-2 Library: shelf moves are atomic and exclusive across reading states;
  finishing a book moves it to Read and closes the session. ✅
- FR-3 Reviews: half-star ratings (1..10 stored), ≥150 characters, one per
  reader per work, spoiler flag withholds bytes server-side. ✅
- FR-4 Friction: new accounts 2 reviews/day; Trusted Readers (reputation ≥100)
  10/day; caps computed from `contribution_ledger`. ✅
- FR-5 Spoiler protection: progress-aware gating for club channels
  (`channels.spoiler_threshold_bp`), 403 `spoiler_gated`. ✅
- FR-6 Scholarship: notes ≥200 chars, citations required to publish, two
  approvals from distinct verified scholars. Schema ✅, workflow UI ⏳
  ([15](15-scholar-system.md)).
- FR-7 Search: Meilisearch primary, Postgres trigram/title-prefix fallback
  labelled `degraded` in the response. ✅
- FR-8 Reader: EPUB/TXT, parchment/sepia/dark themes, font/size/leading/margin
  controls, TOC, bookmarks, citations. ⏳ Phase 3.
- FR-9 Clubs: text channels ✅; voice/video rooms ⏳ ([16](16-realtime.md)).
- FR-10 Import: Goodreads/StoryGraph CSV, Kindle clipboard parser. ⏳
  ([13](13-external-integrations.md)).

## Non-goals (explicit, reviewed each phase)
E-commerce beyond affiliate links · audiobook hosting · AI summarization ·
pay-to-win discovery · public follower leaderboards · third-party analytics.

## Acceptance criteria style
Every requirement ships with a test: integration tests for server invariants
(`apps/api/internal/httpapi/integration_*_test.go`), the browserless smoke
script for the whole loop (`infrastructure/smoke/e2e_smoke.py`), and a named
route for UI. "Implemented" without a test is "planned".
