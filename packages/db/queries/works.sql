-- name: GetWorkBySlug :one
SELECT w.*,
       (SELECT coalesce(json_agg(json_build_object('id', a.id, 'name', a.name, 'role', wa.role) ORDER BY wa.position), '[]')
          FROM work_authors wa JOIN authors a ON a.id = wa.author_id
         WHERE wa.work_id = w.id)::text AS authors_json
  FROM works w
 WHERE w.slug = $1 AND w.deleted_at IS NULL;

-- name: GetWorkByOpenLibraryID :one
SELECT * FROM works WHERE openlibrary_id = $1 AND deleted_at IS NULL;

-- name: UpsertWork :one
INSERT INTO works (slug, title, subtitle, original_language, first_published, description, is_public_domain, openlibrary_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (openlibrary_id) DO UPDATE SET
  title = EXCLUDED.title,
  subtitle = EXCLUDED.subtitle,
  description = CASE WHEN EXCLUDED.description <> '' THEN EXCLUDED.description ELSE works.description END,
  is_public_domain = EXCLUDED.is_public_domain
RETURNING *;

-- name: ListWorksBySubject :many
SELECT w.* FROM works w
  JOIN work_subjects ws ON ws.work_id = w.id
  JOIN subjects s ON s.id = ws.subject_id
 WHERE s.slug = $1 AND w.deleted_at IS NULL
 ORDER BY w.rating_sum::float / GREATEST(w.rating_count, 1) DESC, w.rating_count DESC
 LIMIT $2 OFFSET $3;

-- name: ListEditionsForWork :many
SELECT e.*,
       ca.object_key AS cover_key,
       ca.license    AS cover_license,
       ca.attribution_text AS cover_attribution
  FROM editions e
  LEFT JOIN cover_assets ca ON ca.edition_id = e.id AND ca.is_primary
 WHERE e.work_id = $1 AND e.deleted_at IS NULL
 ORDER BY e.published_on DESC NULLS LAST;

-- name: UpsertEdition :one
INSERT INTO editions (work_id, title, isbn10, isbn13, format, publisher, published_on, language, page_count, gutenberg_id, openlibrary_id, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (isbn13) WHERE isbn13 IS NOT NULL DO UPDATE SET
  publisher = EXCLUDED.publisher,
  page_count = COALESCE(EXCLUDED.page_count, editions.page_count),
  metadata = editions.metadata || EXCLUDED.metadata
RETURNING *;
