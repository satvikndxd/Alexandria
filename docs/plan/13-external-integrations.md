# 13 — Kindle / External Reading Integrations

**Status:** research document · nothing here is implemented · no API is
assumed to exist unless named with its mechanism

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
