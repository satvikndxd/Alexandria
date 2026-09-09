# 26 — MVP Definition

**Status:** achieved (2026-09-09) · the smallest Alexandria that still feels
like Alexandria

## In the box
- Accounts: passkey registration/login, magic-link fallback, sessions, CSRF,
  profiles, privacy toggle.
- Catalogue: FRBR works/editions/authors/subjects; Open Library ingestion;
  Gutenberg catalog ingestion; search (Meilisearch + honest fallback).
- Library: system + custom shelves, formats, progress, rereads, DNF; all
  RLS-private.
- Reviews: half-stars, friction pipeline, spoiler byte-withholding, likes,
  comments; one per reader per work.
- Social: follows/blocks, chronological feed, community feed, public profiles.
- Clubs: text channels with chapter-gated spoilers.
- Trust: reports, moderator actions with mandated rationale, reputation gates.
- The folio: reference-sheet UI on web, fixtures-honest when the API sleeps.
- Proof: integration suite on real Postgres + browserless smoke script.

## Out of the box (and why)
- Voice/video: a realtime ops burden that adds nothing to the reading loop.
- Scholar peer-review UI: the schema is ready; the workflow deserves its own
  phase with reviewers recruited, not stubbed.
- Reader: the MVP tracks books; Phase 3 reads them. (A tracking-only MVP was
  chosen so the friction and privacy invariants could harden first.)
- Push, i18n, imports: portability features, not identity features.

## The test we applied
Remove any item above and Alexandria stops being Alexandria; add any item
below and it stops being an MVP.
