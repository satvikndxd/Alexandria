# 13 — Kindle / External Reading Integrations

**Status:** implemented for the user-export classes · `internal/importx`
(pure parsers, unit-tested), `internal/store/imports.go` (transactional apply
with provenance), `/v1/imports/*`, `/v1/export`, `/v1/account/delete`, web
`/settings` · the "not realistically possible" classes remain unbuilt by
decision, not by omission

## Classification
| Integration | Class | Mechanism | Notes |
|---|---|---|---|
| Amazon Kindle progress | **Not realistically possible** | — | No public API for reading progress or annotations. We do not scrape the Kindle app or the cloud reader. |
| Kindle annotations | User-controlled export | Clipboard/HTML parser for the "My Notebook" export (`Clippings.txt` / notebook export) | User pastes or uploads their own export; parser maps highlights to `(edition, locator)` best-effort. |
| Goodreads | User-controlled export | CSV import (`reviews.csv`) | Map shelves→shelves, ratings→half-stars (round), reviews→reviews **only if ≥150 chars**, else import as rating-only (friction is not waived for imports that publish). |
| StoryGraph | User-controlled export | CSV import | Same policy; note its ISBN column feeds edition matching. |
| Google Books | Official API (metadata only) | Books API v1 | Metadata/shelves only; thumbnails are **not** redistribution rights ([11](11-book-data-cover-strategy.md)). |
| Apple Books / Kobo | Not realistically possible | — | No user-exportable progress path that does not involve scraping proprietary clients. |
| Library systems (Alma/Primo) | Future research | SIP2/NCIP or vendor APIs per institution | Phase 6+; per-institution configuration, never credentials in our DB. |

## What shipped (Phase 5)
- **Goodreads `reviews.csv`** and **StoryGraph CSV**: header-driven parsing,
  status-vocabulary mapping per platform, ISBN-13 cleaned from spreadsheet
  quoting, conservative matching (ISBN13 → exact normalized title with an
  author-surname check), ratings without qualifying prose stored as private
  shelf ratings (`shelf_items.rating`, migration 0013) and never as stub
  reviews, imported prose published only if it clears the 150-char bar,
  provenance stamped (`imported_from`).
- **Kindle `My Clippings.txt`**: parsed blocks (highlight/note/bookmark), then
  aligned against the cached public-domain text with exact whitespace-collapsed
  matching and rune-true offsets (`importx.Align`); unanchorable clippings are
  reported, never guessed. Device "locations" are ignored as meaningless.
- **Reports**: every import returns counts plus per-row skips with reasons
  (`no_matching_work`, `review_exists`, `review_below_minimum`, `not_aligned`),
  surfaced in the UI.
- **Throttle**: three imports per hour per hashed bucket (migration 0013 adds
  the `import` kind to the attempt bucket).
- **Export & deletion**: `GET /v1/export` returns the whole life as JSON;
  `POST /v1/account/delete` soft-deletes and revokes sessions immediately,
  hard purge after the appeal window.

## Principles
1. **User-controlled export only.** If a platform offers no export, the
   integration does not exist. Scraping proprietary clients is a ToS violation
   and a security liability for our users' credentials.
2. **Imports obey our friction where they publish.** An imported 12-character
   "review" becomes a rating, not a review.
3. **Matching is conservative.** Imports match editions by ISBN13, then by
   (title, author) with a confirmation UI for ambiguity; unmatched rows land in
   a review queue the user resolves, never as duplicate works.
4. **No credential passthrough.** We never store Amazon/Goodreads passwords or
   OAuth tokens for platforms without a real OAuth scope for the data.
