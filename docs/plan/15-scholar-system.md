# 15 — Scholar System

**Status:** schema + queries implemented (migration 0006, `queries/scholars.sql`);
submission/peer-review HTTP endpoints and UI ⏳ next backend increment

## Roles and provenance
- **Verified Scholar**: manual admin review + ORCID and/or institutional email;
  `scholar_profiles.status ∈ pending|verified|rejected|revoked`, with
  `verified_by/at` recorded.
- **Community Contributor**: anyone; notes are labelled
  "Community contributor" (gold) vs "Verified scholar" (botanical). Different
  provenance, not lesser worth — the anti-elitism guarantee.
- Conflict-of-interest statement is a required field on the profile and is
  displayed with published notes.

## Note lifecycle
`draft → in_review → published → retracted`, with:
- body ≥200 chars (schema CHECK) — no drive-by notes;
- ≥1 citation (`note_citations`, ≥10 chars each) required to publish;
- **two approvals from distinct verified scholars**, author excluded
  (`CountNoteApprovals`);
- immutable revision history (`note_revisions`); every edit appends a version;
- retraction keeps the record and its history (scholarship corrects in
  public).

## Why peer review and not reputation points
A note asserts expertise about a text; the check must be epistemic (can two
qualified readers defend it?), not behavioural (did it get likes?). Reputation
(`users.reputation`) gates only posting volume, never truth claims.

## Editorial workflow (to build)
1. Author submits draft with citations → `in_review`, reviewers drawn from
   verified scholars in the field (fallback: any verified scholar).
2. Reviewers approve/reject with comments (`note_reviews`); two approvals
   publish; author sees comments either way.
3. Post-publication corrections append revisions; substantial change re-opens
   review; retraction is moderator- or author-initiated with rationale.

## Never, ever
Invent scholars, invent credentials, generate notes with AI, or accept
"verified" from an email domain alone. Verification is a human act with a
recorded human (`verified_by`).
