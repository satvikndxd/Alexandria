# 11 — Book Data & Cover Strategy

**Status:** metadata pipeline implemented (Open Library worker); cover
caching to MinIO ⏳; rights model implemented in schema and UI

## Metadata: Open Library, politely
- Worker (`internal/ingest/openlibrary.go`): 1 req/s pacing ticker,
  identifying User-Agent with contact address, 429/5xx → JetStream backoff.
- Work upserts keyed on `openlibrary_id`; authors resolved (≤5 per work) and
  linked with roles; subjects filtered (≤80 chars) into a clean taxonomy.
- Handlers never fetch upstream: they enqueue `ingest.openlibrary_work`
  intents; the outbox guarantees publication.

## FRBR discipline
Titles never identify anything. Slugs are minted once and stay stable;
collisions get numeric suffixes; corrections do not rewrite URLs.

## Cover rights model (the point of this document)
`cover_assets` separates **source** from **license** from **cache policy**:
`source`, `source_url`, `license` (enum: public_domain, cc0, cc_by, cc_by_sa,
fair_use_thumbnail, user_upload, unknown), `license_url`, `attribution_text`,
`object_key`, `cacheable`, `cache_expires_at`.
Rules:
1. Never hotlink an unverified cover; never redistribute a copyrighted
   thumbnail from our CDN.
2. Until a license row clears, the UI renders the house **woodcut plate**
   (`WoodcutCover.tsx`) — a generated typographic cover in the manuscript
   style. An absent cover is a design decision, not a hole.
3. Public-domain and CC covers are cached to MinIO with attribution stored and
   printed on the book page ("Cover: …").
4. DMCA: `TakedownCover` clears primary/cache in one statement; the CDN can no
   longer serve it.
- Hierarchy when multiple sources exist: user upload (verified ownership) →
  Wikimedia Commons (PD classics) → Open Library covers (license-checked) →
  Google Books thumbnails **only** as fair-use, non-cached, non-redistributed
  references (legal review pending, [21](21-licensing-legal.md)).

## Identifiers beyond ISBN
`external_identifiers(namespace, value)` for LCCN, OCLC, Wikidata, legacy
Goodreads ids — importers write here rather than inventing columns.
