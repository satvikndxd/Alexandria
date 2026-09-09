# 15 — Scholar System

**Status:** implemented end-to-end · migration 0006, `queries/scholars.sql`,
`internal/domain/scholar.go`, `internal/store/scholars.go`,
`internal/httpapi/scholar_handlers.go`, web `/scholar/*` and
`/books/[slug]/note`

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

## Editorial workflow (implemented)
1. Author submits draft with citations → `in_review`
   (`POST /v1/works/{slug}/notes` with `submit:true`, or `/submit` later).
2. Verified scholars approve/reject with comments
   (`POST /v1/notes/{id}/review`); the store publishes when
   `domain.PublishGate(approvals, citations)` passes — two distinct approvals,
   author excluded, ≥1 citation. Own-note and unverified reviews are 403.
3. Revising a published note bumps the version, appends a revision row, and
   re-opens review: substantial change to published scholarship must be
   re-defended. Retraction keeps the record and its history.
4. Drafts and notes in review are invisible to strangers (404), visible to
   author, reviewers and moderators; the queue is
   `GET /v1/scholar/queue` (verified scholars + moderators only).
5. Verification is a moderator act with a recorded moderator id
   (`POST /v1/moderation/scholars/{userID}`); ORCID uniqueness is enforced and
   reported as 409 `orcid_taken`, never a 500.

The integration suite walks the whole lifecycle over HTTP
(`scholar_flow_test.go`): application → verification → submission → gates →
two approvals → publication → public record with citations and revisions →
retraction.

## Never, ever
Invent scholars, invent credentials, generate notes with AI, or accept
"verified" from an email domain alone. Verification is a human act with a
recorded human (`verified_by`).
