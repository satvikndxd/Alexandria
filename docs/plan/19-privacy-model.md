# 19 — Privacy Model

**Status:** implemented · the short version: we collect little, store less,
and let Postgres enforce the boundary

## Collection
- No third-party analytics, no trackers, no ad pixels. Self-hosted Plausible
  or Umami if we ever want traffic shape (decision pending; default: none).
- No raw IP at rest: hashed buckets for throttling, salted hash on sessions.
- No reading-history telemetry of any kind; progress is the reader's own row,
  RLS-private.
- Push tokens ⏳: stored only when the reader opts in, deleted with the
  account, never shared.

## Storage boundaries
- Private by database law: shelves, shelf_items, reading_sessions,
  annotations, notifications carry RLS policies with FORCE; a leaked read
  query cannot cross readers ([ADR 0007](../adr/0007-rls-privacy-backstop.md)).
- Public by choice, not by default: a shelf is public only when its owner
  says so (`shelves.is_private`), and the public-read policy is explicit.
- Emails: stored for account function only; the outbox keeps payloads until
  sent then retains minimal rows; magic-link secrets are HMACs, never
  plaintext.

## Retention & deletion
- Account deletion: soft delete for audit windows, hard purge by async job
  (⏳ scheduler; currently operator-run), cascading shelves/sessions/notes.
- Sessions and ceremonies expire on their own; stale rows pruned hourly.
- Moderation records outlive deletion where law or appeal requires, and say
  so in the privacy notice.

## Honesty rules
- No dark patterns: private means private in the API, not just in the CSS.
- No "engagement" profiles: there is no behavioural model of you anywhere in
  the schema.
- Privacy notice is a page, not a PDF: what we store, why, for how long, and
  the exact command to export or delete it (export ⏳ Phase 5).
