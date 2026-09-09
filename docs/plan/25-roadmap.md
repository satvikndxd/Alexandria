# 25 — Development Roadmap

**Status:** living · phases gate on acceptance criteria, not dates

| Phase | Scope | State |
|---|---|---|
| 0 Research | platform comparisons, legal landscape, catalog verification | ✅ (this package, [13](13-external-integrations.md), [16](16-realtime.md)) |
| 1 Foundation | monorepo, FRBR schema, monolith, outbox ingestion, design system, web shell | ✅ |
| 2 MVP | auth (passkeys+magic links), RLS library, reviews+friction, clubs text, search, web wired to API, integration+smoke suites | ✅ |
| 3 Reader | Gutenberg text cache, TXT/EPUB renderer, typography engine, annotations UI, citations | ⏳ current |
| 4 Community | scholar submission + peer-review endpoints/UI, LiveKit voice/video, notifications push | ⏳ |
| 5 Portability | Goodreads/StoryGraph CSV import, Kindle clipboard parser, data export/delete tooling | ⏳ |
| 6 Institutions | library-card integrations (Alma/Primo) research, university partnerships | ⏳ |
| 7 Releases | Flutter app hardening, Fastlane betas, desktop builds | ⏳ |
| 8 Scaling | K3s multi-node, read replicas, CDN for covers/texts, observability depth | ⏳ |

## Phase 3 acceptance criteria (next)
- A Gutenberg etext renders as a typeset edition: TOC, chapter paging,
  parchment/sepia/dark themes, font/size/leading/margin controls persisted per
  reader.
- Highlights/notes create `annotations` rows anchored
  `(chapter_idx, start_off, end_off)` and reappear at the right offsets after
  a reflow (offsets are over normalized text, not DOM).
- Text is served from our cache, never hotlinked; boilerplate stripped and
  preserved separately; license/trademark notice visible in the reader footer.
- Integration tests cover anchor stability and RLS privacy of annotations.

## What we deliberately do not build yet
Voice/video, EPUB DRM anything, social DMs, mobile push, i18n UI, audiobooks.
Each has a home in a later phase and a reason it is not in this one.
