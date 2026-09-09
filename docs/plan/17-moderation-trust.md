# 17 — Moderation & Trust System

**Status:** implemented end-to-end · report filing on reviews/notes/etc.,
moderator queue + scholar verification at `/moderation`, notification centre
at `/notifications` with addressed notes only, `/metrics` exposing friction
refusals by code · human moderator recruitment remains a community-phase act

## Structural friction first ([ADR 0005](../adr/0005-friction-not-detection.md))
We do not pretend to detect AI text. We make mass low-effort posting
structurally unattractive: 150-char minimum, filler heuristics, daily caps
from an append-only ledger, one review per reader per work, citation gates on
scholarship, rationale gates on moderation itself.

## Human workflow
- Reports (`reports`, reason enum incl. `ai_slop`, `copyright`,
  `impersonation`) route to a queue ordered by severity (threat > hate >
  harassment > …), never by volume.
- Actions (`moderation_actions`): warning, content_removal, temp_suspension,
  permanent_ban, restore, note — each with a **written rationale ≥10 chars**
  (schema CHECK) and optional expiry; suspensions are windows that expire on
  their own (`users.suspended_until`).
- Appeals: a report resolved against you can be re-opened by a different
  moderator; the rationale chain is the audit trail.
- Audit: every action names moderator, target, report; nothing is anonymous to
  the system, everything is minimal to the public.

## Reputation
Moves only through recorded events (moderator action, published scholar note,
sustained un-reported history). Never from likes or follows — those are the
gamification vectors we refuse.

## Copyright
DMCA path: `TakedownCover` for covers; content removal action + report
resolution for text; designated agent process documented in
[21](21-licensing-legal.md) (legal review pending).

## Surfaces (implemented)
- `ReportButton` on reviews (and the same vocabulary everywhere): the reason
  select IS the moderation enum, so a report routes without translation.
- `/moderation`: queue ordered by severity (threat > hate > harassment > …),
  action form whose centre is the written rationale (schema CHECK ≥10 chars,
  surfaced as 422 when absent); scholar applications with verify/reject.
- Moderation actions resolve the accountable user from the report subject
  (review author, commenter, message sender, note author) — a report id is
  not a user id, and the FK rightly refuses the confusion.
- `/notifications`: addressed notes only (reply to your review, verdict on
  your note, new follower); mark-one and mark-all; unread count on `/v1/me`
  computed inside the reader's RLS scope.
- `/metrics`: Prometheus text format, dependency-free; friction refusals by
  code so the anti-slop system's effect is observable, not anecdotal.

## What moderation must never become
Automated truth arbitration, shadow-banning without a record, or a reputation
market. If an action cannot be explained in prose to the person it affects,
the schema refuses it.
