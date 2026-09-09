# 17 — Moderation & Trust System

**Status:** reports + moderator actions implemented; queues UI ⏳; human
moderator network ⏳ community phase

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

## What moderation must never become
Automated truth arbitration, shadow-banning without a record, or a reputation
market. If an action cannot be explained in prose to the person it affects,
the schema refuses it.
