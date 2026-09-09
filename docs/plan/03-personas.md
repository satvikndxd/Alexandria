# 03 — User Personas

**Status:** living document · each persona maps to features that exist or are
scheduled; we do not design for personas we refuse to serve (advertisers,
engagement analysts).

## 1 · The Archivist (deep reader)
Tracks every edition; cares about translators, bindings, typography; writes
long-form reviews.
- Served by: FRBR `works → editions` ([ADR 0002](../adr/0002-frbr-data-model.md)),
  edition metadata JSONB, cover license lines on the book page, half-star
  ratings, 20 000-character review ceiling.
- Tension we accept: edition-level discussion is thin until the reader lands
  (Phase 3 annotations are edition-anchored).

## 2 · The Scholar (contributor)
Academic or subject expert; writes contextual notes tied to passages; needs
provenance and peer review, not likes.
- Served by: `scholar_profiles` (ORCID, affiliation, COI statement),
  `note_reviews` (two distinct verified approvals), `note_revisions`
  (immutable history), `note_citations` (required to publish).
- Anti-goal: never invent credentials; verification is manual + ORCID
  ([15](15-scholar-system.md)).

## 3 · The Club Organizer
Runs a monthly Dostoevsky group; needs spoiler-free chapter discussion and,
later, voice rooms.
- Served by: clubs, channels, `spoiler_threshold_bp`, current-read scheduling
  (`club_reads`); voice/video scheduled Phase 4 ([16](16-realtime.md)).

## 4 · The Indie Author
Wants presence without buying visibility.
- Served by: claimable author records (`authors.is_claimed`), free
  self-promotion on author pages; **no** paid ranking, ever
  ([14](14-social-community.md)).

## 5 · The Returner (added from research)
Someone who read widely, stopped for a decade, and wants a library that waits
without nagging.
- Served by: quiet defaults, no streak-shaming (the streak is a private square
  calendar, never a public badge), DNF as legitimate data, magic-link sign-in
  with no password to remember.
