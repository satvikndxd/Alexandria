# ADR 0005 — Anti-AI-slop: structural friction, not detection

**Status:** Accepted · 2026-08-14

## Context
AI-text detectors are unreliable and adversarially fragile. Pretending
otherwise would launder false accusations against real readers.

## Decision
Make low-effort mass posting structurally unattractive, at three layers:

| Layer | Rule | Where enforced |
|---|---|---|
| Schema | Review ≥ 150 chars, note ≥ 200 chars, 1 review/user/work | `CHECK` constraints, unique indexes |
| Domain | Rating bounds, filler heuristics (repeated glyphs, <12 distinct words), rune-based length | `internal/domain` (unit-tested) |
| Policy | 2 reviews/day for new accounts; 10 for Trusted Readers (reputation ≥ 100); citation required to publish a scholar note; peer review by 2 verified scholars | `contribution_ledger` window counts + service layer |

Plus human moderation: report queues (`reports`), justified actions
(`moderation_actions.rationale` is NOT NULL), audit trail, appeals.

## Consequences
- A determined human can still post slop slowly — moderation handles that.
- No reader is ever auto-accused of being a machine.
- The limits are transparent, documented, and shown in the UI.
