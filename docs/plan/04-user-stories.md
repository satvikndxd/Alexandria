# 04 — User Stories

**Status:** living backlog · ✅ = implemented with a test or route, ⏳ = scheduled

- ✅ *As a reader, I want my feed to be only what followed accounts actually
  did, chronologically,* so that no algorithm can farm me.
  `GET /v1/feed` — UNION over reviews and public shelf-adds, `ORDER BY created_at DESC`.
- ✅ *As a scholar, I want to attach a note to a specific passage,* so readers
  meet context exactly where confusion lives. `scholar_notes.chapter_ref` +
  `anchor_quote`.
- ✅ *As a reader three chapters in, I want discussions of the ending kept from
  me,* so endings stay mine. `403 spoiler_gated` until progress passes the
  channel threshold; review bytes withheld without `reveal_spoilers=true`.
- ✅ *As a new account, I want to be slowed down rather than suspected,* so
  quality stays high without AI "detection". 2 reviews/day, 150-char minimum,
  filler heuristics ([ADR 0005](../adr/0005-friction-not-detection.md)).
- ✅ *As a privacy-conscious reader, I want private shelves to be private even
  if the server has a bug,* so my reading history is mine. Row-Level Security
  ([ADR 0007](../adr/0007-rls-privacy-backstop.md)); integration test proves
  cross-reader invisibility.
- ✅ *As a reader without a password manager, I want to sign in with my face,*
  so credential theft is structurally hard. Passkeys ([ADR 0006](../adr/0006-passkey-first-auth.md)).
- ✅ *As a maintainer, I want schema changes to be impossible to half-apply,*
  so environments never drift. Checksummed forward-only migrations
  ([ADR 0008](../adr/0008-embedded-forward-only-migrations.md)).
- ⏳ *As a reader of Project Gutenberg texts, I want a typeset reader with my
  own typography,* so public-domain reading feels like a good edition.
  Phase 3 ([12](12-gutenberg-integration.md)).
- ⏳ *As a Goodreads refugee, I want my shelves and reviews imported,* so
  switching costs nothing. Phase 5 ([13](13-external-integrations.md)).
- ⏳ *As a club member, I want a weekly voice room,* so discussion can be
  spoken. Phase 4 ([16](16-realtime.md)).
- ⏳ *As a scholar, I want my note's revision history public,* so corrections
  are visible rather than silent. Schema ✅, UI ⏳ ([15](15-scholar-system.md)).
