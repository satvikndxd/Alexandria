# 21 — Licensing & Legal Considerations

**Status:** research document · engineering recommendations clearly separated
from legal advice · **professional legal review is required before scale**

## Engineering recommendations (our beliefs, not counsel)
- **Metadata** (titles, authors, ISBNs, dates): facts; low risk; attribute
  sources (Open Library) in docs and UI where displayed.
- **Covers**: the real risk surface. Rights-aware pipeline
  ([11](11-book-data-cover-strategy.md)); default to generated woodcut plates;
  cache only cleared licenses; immediate takedown path.
- **Project Gutenberg texts**: PD in the US; trademark obligations travel with
  the text ([12](12-gutenberg-integration.md)); jurisdictional status
  (life+50 vs life+70) must be surfaced per work, not assumed.
- **User content**: license grant in ToS limited to operating the platform;
  authors keep copyright; reviews are the reviewer's text.
- **Quotations in reviews/notes**: fair-use-shaped by design (short,
  transformative, attributed) — but fair use is a doctrine, not a rule.
- **Affiliate links**: Amazon Associates disclosure on every purchase page
  ("As an Amazon Associate…"), geographic tag constraints respected, no
  price scraping beyond the PA-API terms; Bookshop.org as the ethical second
  vendor and the fallback if Amazon terms change.

## Open legal questions (need counsel)
1. International copyright variance for PD texts served cross-border.
2. Google Books thumbnail caching: our current stance is *never cache, never
   redistribute*; confirm fair-dealing exposure for on-screen display.
3. DMCA designated agent registration and takedown SLAs for a self-hosted
   federation (each instance operator is their own agent — document this).
4. Voice-room recording consent regimes (two-party consent jurisdictions) —
   default is no recording ([16](16-realtime.md)).
5. GDPR/CCPA data-subject request tooling: export/delete commands
   (⏳ Phase 5) and the retention schedule in [19](19-privacy-model.md).
6. ORCID/institutional verification: what we may store and display about a
   scholar's affiliation.

## Trademarks
"Alexandria" name/logo clearance ⏳; Project Gutenberg™ notice stored and
displayed with every cached text; no implication of endorsement anywhere.

## The rule for contributors
When in doubt: do not cache, do not redistribute, label the uncertainty in
the record (`cover_assets.license = unknown`), and open an issue tagged
`legal-question`.
