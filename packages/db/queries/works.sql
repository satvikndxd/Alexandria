-- Bibliography: the FRBR core (Work → Edition) plus authors and subjects.
--
-- Duplicate books are the failure mode that kills every community-maintained
-- catalogue, so every write path here is an idempotent upsert keyed on a
-- stable external identifier, never on a title string.

-- name: GetWorkBySlug :one
SELECT * FROM works WHERE slug = $1 AND deleted_at IS NULL;

-- name: GetWorkByID :one
SELECT * FROM works WHERE id = $1 AND deleted_at IS NULL;

-- name: GetWorkByOpenLibraryID :one
SELECT * FROM works WHERE openlibrary_id = $1 AND deleted_at IS NULL;

-- name: WorkSlugIsTaken :one
SELECT EXISTS (SELECT 1 FROM works WHERE slug = @slug) AS taken;

-- name: UpsertWork :one
INSERT INTO works (slug, title, subtitle, original_language, first_published,
                   description, is_public_domain, openlibrary_id)
VALUES (@slug, @title, sqlc.narg('subtitle'), @original_language,
        sqlc.narg('first_published'), @description, @is_public_domain,
        sqlc.narg('openlibrary_id'))
ON CONFLICT (openlibrary_id) DO UPDATE SET
  title       = EXCLUDED.title,
  subtitle    = COALESCE(EXCLUDED.subtitle, works.subtitle),
  description = CASE WHEN EXCLUDED.description <> '' THEN EXCLUDED.description ELSE works.description END,
  is_public_domain = works.is_public_domain OR EXCLUDED.is_public_domain
RETURNING *;

-- name: InsertWork :one
-- For works with no external identifier yet (indie authors, Gutenberg texts
-- whose Open Library work record is missing). Slug uniqueness is enforced by
-- the schema; callers resolve collisions before inserting.
INSERT INTO works (slug, title, subtitle, original_language, first_published,
                   description, is_public_domain)
VALUES (@slug, @title, sqlc.narg('subtitle'), @original_language,
        sqlc.narg('first_published'), @description, @is_public_domain)
RETURNING *;

-- name: ListWorks :many
-- The catalogue listing. Every filter is optional and expressed as a
-- NULL-or-match predicate so sqlc generates one plan and the caller never
-- builds SQL strings.
--
-- Ordering is explicit and non-algorithmic: `rating` is a Bayesian-weighted
-- average (never a raw mean, which puts a single 5★ above 400 4.8★ ratings),
-- `recent` is publication year, `added` is ingest order. There is no
-- engagement-weighted sort, by design.
SELECT w.*,
       (SELECT count(*) FROM editions e
         WHERE e.work_id = w.id AND e.deleted_at IS NULL) AS edition_count,
       (SELECT e.gutenberg_id FROM editions e
         WHERE e.work_id = w.id AND e.gutenberg_id IS NOT NULL
         ORDER BY e.gutenberg_id LIMIT 1) AS gutenberg_id
  FROM works w
 WHERE w.deleted_at IS NULL
   AND (sqlc.narg('subject_slug')::text IS NULL OR EXISTS (
         SELECT 1 FROM work_subjects ws JOIN subjects s ON s.id = ws.subject_id
          WHERE ws.work_id = w.id AND s.slug = sqlc.narg('subject_slug')))
   AND (sqlc.narg('author_id')::uuid IS NULL OR EXISTS (
         SELECT 1 FROM work_authors wa
          WHERE wa.work_id = w.id AND wa.author_id = sqlc.narg('author_id')))
   AND (sqlc.narg('language')::text IS NULL OR w.original_language = sqlc.narg('language'))
   AND (sqlc.narg('public_domain_only')::boolean IS NULL
        OR NOT sqlc.narg('public_domain_only')::boolean
        OR w.is_public_domain)
   AND (sqlc.narg('published_after')::integer IS NULL OR w.first_published >= sqlc.narg('published_after'))
   AND (sqlc.narg('published_before')::integer IS NULL OR w.first_published <= sqlc.narg('published_before'))
   AND (sqlc.narg('title_prefix')::text IS NULL OR w.title ILIKE sqlc.narg('title_prefix') || '%')
 ORDER BY
   CASE @sort::text
     WHEN 'rating'  THEN (w.rating_sum + 20.0 * 3.5) / (w.rating_count + 20.0)
     WHEN 'recent'  THEN coalesce(w.first_published, -9999)
     WHEN 'title'   THEN 0
     ELSE 0
   END DESC,
   CASE WHEN @sort = 'title' THEN w.title END ASC,
   w.rating_count DESC,
   w.created_at DESC
 LIMIT @lim OFFSET @off;

-- name: ListWorksBySubject :many
SELECT w.* FROM works w
  JOIN work_subjects ws ON ws.work_id = w.id
  JOIN subjects s ON s.id = ws.subject_id
 WHERE s.slug = @slug AND w.deleted_at IS NULL
 ORDER BY (w.rating_sum + 20.0 * 3.5) / (w.rating_count + 20.0) DESC, w.rating_count DESC
 LIMIT @lim OFFSET @off;

-- name: ListWorkIDsForIndex :many
-- Paging cursor for a full Meilisearch reindex: keyset pagination on id, which
-- stays O(log n) where OFFSET would degrade badly on a large catalogue.
SELECT id FROM works
 WHERE deleted_at IS NULL AND id > @after_id
 ORDER BY id
 LIMIT @lim;

-- name: CountWorks :one
SELECT count(*) FROM works WHERE deleted_at IS NULL;

-- ---- Authors ----------------------------------------------------------------

-- name: UpsertAuthor :one
INSERT INTO authors (name, sort_name, bio, birth_year, death_year, slug, openlibrary_id, wikidata_id)
VALUES (@name, @sort_name, @bio, sqlc.narg('birth_year'), sqlc.narg('death_year'),
        @slug, sqlc.narg('openlibrary_id'), sqlc.narg('wikidata_id'))
ON CONFLICT (openlibrary_id) DO UPDATE SET
  name      = EXCLUDED.name,
  sort_name = EXCLUDED.sort_name,
  bio       = CASE WHEN EXCLUDED.bio <> '' THEN EXCLUDED.bio ELSE authors.bio END,
  birth_year = COALESCE(authors.birth_year, EXCLUDED.birth_year),
  death_year = COALESCE(authors.death_year, EXCLUDED.death_year)
RETURNING *;

-- name: GetAuthorBySlug :one
SELECT * FROM authors WHERE slug = @slug AND deleted_at IS NULL;

-- name: GetAuthorByName :one
-- Dedup key for authors that arrive without a stable external identifier
-- (Gutenberg catalog names, indie authors). Exact match, case-insensitive.
SELECT * FROM authors WHERE name = @name AND deleted_at IS NULL LIMIT 1;

-- name: GetAuthorByID :one
SELECT * FROM authors WHERE id = @id AND deleted_at IS NULL;

-- name: ListAuthorsForWork :many
SELECT a.*, wa.role, wa.position
  FROM work_authors wa
  JOIN authors a ON a.id = wa.author_id
 WHERE wa.work_id = @work_id AND a.deleted_at IS NULL
 ORDER BY wa.position, wa.role;

-- name: LinkWorkAuthor :exec
INSERT INTO work_authors (work_id, author_id, role, position)
VALUES (@work_id, @author_id, @role, @position)
ON CONFLICT (work_id, author_id, role) DO UPDATE SET position = EXCLUDED.position;

-- name: ListWorksForAuthor :many
SELECT w.*, wa.role
  FROM work_authors wa
  JOIN works w ON w.id = wa.work_id
 WHERE wa.author_id = @author_id AND w.deleted_at IS NULL
 ORDER BY coalesce(w.first_published, -9999) ASC
 LIMIT @lim OFFSET @off;

-- name: ClaimAuthorProfile :execrows
-- Indie authors claim an author record; claims are audited, never paid.
UPDATE authors SET is_claimed = true, claimed_by = @user_id, bio = @bio
 WHERE id = @author_id AND NOT is_claimed;

-- ---- Subjects ---------------------------------------------------------------

-- name: UpsertSubject :one
INSERT INTO subjects (slug, name, kind) VALUES (@slug, @name, @kind)
ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name, kind = EXCLUDED.kind
RETURNING *;

-- name: LinkWorkSubject :exec
INSERT INTO work_subjects (work_id, subject_id) VALUES (@work_id, @subject_id)
ON CONFLICT DO NOTHING;

-- name: ListSubjectsForWork :many
SELECT s.* FROM work_subjects ws JOIN subjects s ON s.id = ws.subject_id
 WHERE ws.work_id = @work_id
 ORDER BY s.name;

-- name: ListPopularSubjects :many
SELECT s.*, count(ws.work_id) AS work_count
  FROM subjects s
  JOIN work_subjects ws ON ws.subject_id = s.id
 GROUP BY s.id
 ORDER BY work_count DESC
 LIMIT @lim;

-- ---- Editions ---------------------------------------------------------------

-- name: ListEditionsForWork :many
SELECT e.*,
       ca.object_key       AS cover_key,
       ca.license          AS cover_license,
       ca.attribution_text AS cover_attribution
  FROM editions e
  LEFT JOIN cover_assets ca ON ca.edition_id = e.id AND ca.is_primary
 WHERE e.work_id = @work_id AND e.deleted_at IS NULL
 ORDER BY e.published_on DESC NULLS LAST;

-- name: GetEditionByID :one
SELECT e.*, w.slug AS work_slug, w.title AS work_title
  FROM editions e JOIN works w ON w.id = e.work_id
 WHERE e.id = @id AND e.deleted_at IS NULL;

-- name: GetEditionByGutenbergID :one
SELECT * FROM editions WHERE gutenberg_id = @gutenberg_id AND deleted_at IS NULL;

-- name: UpsertEdition :one
INSERT INTO editions (work_id, title, isbn10, isbn13, format, publisher, published_on,
                      language, page_count, gutenberg_id, openlibrary_id, metadata)
VALUES (@work_id, @title, sqlc.narg('isbn10'), sqlc.narg('isbn13'), @format,
        sqlc.narg('publisher'), sqlc.narg('published_on'), @language,
        sqlc.narg('page_count'), sqlc.narg('gutenberg_id'),
        sqlc.narg('openlibrary_id'), @metadata)
ON CONFLICT (isbn13) WHERE isbn13 IS NOT NULL DO UPDATE SET
  publisher  = COALESCE(EXCLUDED.publisher, editions.publisher),
  page_count = COALESCE(EXCLUDED.page_count, editions.page_count),
  metadata   = editions.metadata || EXCLUDED.metadata
RETURNING *;

-- name: UpsertGutenbergEdition :one
-- Gutenberg texts have no ISBN, so they need their own conflict target.
-- Reusing UpsertEdition here would insert a new row on every catalog refresh,
-- because a NULL isbn13 never conflicts.
INSERT INTO editions (work_id, title, format, publisher, language, page_count, gutenberg_id, metadata)
VALUES (@work_id, @title, 'gutenberg', @publisher, @language,
        sqlc.narg('page_count'), @gutenberg_id, @metadata)
ON CONFLICT (gutenberg_id) WHERE gutenberg_id IS NOT NULL DO UPDATE SET
  title    = EXCLUDED.title,
  metadata = editions.metadata || EXCLUDED.metadata
RETURNING *;

-- ---- Covers (rights-aware) --------------------------------------------------

-- name: InsertCoverAsset :one
INSERT INTO cover_assets (edition_id, source, source_url, license, license_url,
                          attribution_text, object_key, cacheable, cache_expires_at,
                          uploaded_by, is_primary)
VALUES (@edition_id, @source, sqlc.narg('source_url'), @license, sqlc.narg('license_url'),
        sqlc.narg('attribution_text'), sqlc.narg('object_key'), @cacheable,
        sqlc.narg('cache_expires_at'), sqlc.narg('uploaded_by'), @is_primary)
RETURNING *;

-- name: ClearPrimaryCover :exec
UPDATE cover_assets SET is_primary = false WHERE edition_id = @edition_id AND is_primary;

-- name: GetPrimaryCoverForEdition :one
SELECT * FROM cover_assets WHERE edition_id = @edition_id AND is_primary;

-- name: ListCoversForWork :many
SELECT ca.*, e.id AS edition_id, e.title AS edition_title
  FROM cover_assets ca JOIN editions e ON e.id = ca.edition_id
 WHERE e.work_id = @work_id AND e.deleted_at IS NULL
 ORDER BY ca.is_primary DESC, ca.created_at DESC;

-- name: TakedownCover :execrows
-- DMCA path: immediate, auditable, and it removes the cached object reference
-- so the CDN can no longer serve it.
UPDATE cover_assets
   SET is_primary = false, cacheable = false, object_key = NULL,
       license = 'unknown', attribution_text = 'Removed following a rights request.'
 WHERE id = @id;

-- name: UpsertExternalIdentifier :exec
INSERT INTO external_identifiers (edition_id, namespace, value)
VALUES (@edition_id, @namespace, @value)
ON CONFLICT (namespace, value) DO NOTHING;

-- name: ListExternalIdentifiers :many
SELECT * FROM external_identifiers WHERE edition_id = @edition_id ORDER BY namespace;
