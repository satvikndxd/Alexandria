# 09 — Database Architecture

**Status:** implemented · 12 migrations in `packages/db/migrations/`

## FRBR core ([ADR 0002](../adr/0002-frbr-data-model.md))
`works` (the abstract creation) → `editions` (a specific ISBN/manifestation,
or a Gutenberg text). Reviews and scholar notes attach to **works**; progress,
formats and covers attach to **editions**. Duplicates are prevented by stable
external keys: `works.openlibrary_id` unique, `editions.isbn13` unique where
present, `editions.gutenberg_id` unique where present — NULL-safe partial
indexes so edition-less and ISBN-less rows coexist safely.

## Entity map (abridged; full DDL is the migrations)
identity: users · profiles · webauthn_credentials · sessions · auth_challenges ·
auth_attempts · email_outbox · follows · user_blocks
bibliography: authors · works · work_authors · subjects · work_subjects ·
editions · cover_assets · external_identifiers · gutenberg_texts · affiliate_links
library: shelves · shelf_items · reading_sessions · annotations
discussion: reviews · review_likes · review_comments · scholar_profiles ·
scholar_notes · note_citations · note_reviews · note_revisions
clubs: clubs · club_memberships · club_reads · channels · messages
trust: reports · moderation_actions · contribution_ledger · notifications
ops: ingest_jobs · outbox · schema_migrations

## Invariants enforced in the schema (not the app)
- reviews: rating 1..10, body 150..20 000, unique(user_id, work_id); rating
  aggregates maintained by trigger.
- reading_sessions: progress 0..10000; at most one open session per
  (user, work) via partial unique index.
- shelves: one system shelf per kind per user; custom names unique per user.
- scholar_notes: body ≥200; citations ≥10 chars; publish gate in service layer.
- moderation_actions: rationale ≥10 chars — every human act is justifiable.
- contribution_ledger: append-only; daily caps count ledger rows, so deleting
  spam cannot refund its author's quota.

## Row-Level Security ([ADR 0007](../adr/0007-rls-privacy-backstop.md))
shelves, shelf_items, reading_sessions, annotations, notifications carry
owner-or-service policies with `FORCE`; public shelves get an explicit
public-read policy. Request path sets `app.user_id` via `SET LOCAL`; workers
set `app.service='on'`. Superusers bypass RLS by definition, so production
connects as `alexandria_app`.

## The RLS context trap (review checklist)
A query against an RLS-covered table executed on a bare pool connection sees
*nothing* — not even the caller's own rows — because `app.user_id` is unset.
This has bitten three handler paths already (club message gates, room gates,
reader annotations), each time producing a plausible 403/404/empty instead of
an error. Review rule: **any read of shelves, shelf_items, reading_sessions,
annotations or notifications must run through `scopedRead` / `TxUser` /
`ReadUser`; any raw `Queries()` call touching those tables is a bug.**

## Indexing policy
Every hot query names its index in review: trigram GIN on titles/names for the
degraded search path, partial indexes for open sessions and unread
notifications, keyset cursors (`id > @after`) for reindex and feeds — OFFSET
is banned from unbounded streams.

## Migrations ([ADR 0008](../adr/0008-embedded-forward-only-migrations.md))
Embedded at build, applied by `cmd/migrate`, checksummed, forward-only,
advisory-locked, one transaction per file; `status` exits non-zero when
pending so it is a deploy gate.
