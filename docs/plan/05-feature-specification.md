# 05 — Feature Specification

**Status:** implemented unless marked ⏳ · cites code, not intentions

## Library
- Five system shelves per account, created atomically at registration
  (`store.RegisterUser`); custom shelves user-scoped unique by name.
- A work occupies exactly one reading-state shelf; moves are transactional
  (`store.MoveToShelf`), favorites orthogonal.
- Progress: 0..10000 basis points (`reading_sessions.progress_bp`, CHECKed).
  Reaching 10000 closes the session, stamps `finished_on`, moves the shelf to
  Read. Rereads open a new session; history is never mutated.
- Formats: physical, ebook, audiobook, public_domain, library_copy, other.

## Reviews
- Rating stored as integer half-stars 1..10; aggregates denormalized on
  `works.rating_sum/rating_count` by trigger (`0005_reviews.sql`).
- Body 150..20 000 characters (schema CHECK), rune-counted in the domain layer
  so non-Latin scripts are not penalized; filler heuristics reject a dominant
  glyph or <12 distinct words.
- One review per reader per work (unique index); edits revise, rereads update.
- Spoilers: `has_spoilers` withholds `body`/`title` server-side unless
  `?reveal_spoilers=true`; UI veils as affordance only.
- Likes adjust the review's counter only — never reputation, never feed rank.

## Scholar notes ⏳ workflow
Schema complete (0006 migration): status lifecycle `draft → in_review →
published → retracted`, citations required for publish, two distinct verified
approvals, immutable revisions. HTTP submission/review endpoints are the next
backend increment; see [15](15-scholar-system.md).

## Clubs
- Channels: text ✅; voice/video ⏳ LiveKit ([16](16-realtime.md)).
- Spoiler gating is data: `channels.spoiler_threshold_bp` vs the member's
  progress on `channels.work_id`; enforced in the API inside the caller's RLS
  context (`scopedRead`), because the gate reads private progress.
- Membership roles: member/moderator/organizer; founder seeded as organizer.

## Reader ⏳
TXT (Gutenberg `-0.txt`) first, EPUB second; typography controls, themes,
TOC, bookmarks, highlights anchored `(chapter_idx, start_off, end_off)` —
the annotation schema already exists and is RLS-private.

## Discovery
- Catalogue sorts: Bayesian rating `(sum + 20·3.5)/(count + 20)`, recency,
  title. No engagement sort exists.
- Related works: co-shelving counts weighted by inverse popularity
  (`ListCoOccurringWorks`), private shelves excluded from the statistic.
