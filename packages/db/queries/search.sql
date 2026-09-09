-- Search projections.
--
-- Meilisearch is fed from Postgres, never written to directly by request
-- handlers: the database stays the source of truth and the index is a derived,
-- rebuildable projection. These queries shape exactly one index document each,
-- so reindexing is a pure function of the catalogue.
--
-- Nested arrays are returned as JSON and unmarshalled in Go (internal/search),
-- which keeps sqlc's types honest instead of smuggling strings through SQL.

-- name: WorkSearchDocuments :many
SELECT w.id, w.slug, w.title, w.subtitle, w.description, w.first_published,
       w.original_language, w.is_public_domain, w.rating_sum, w.rating_count,
       coalesce((
         SELECT json_agg(json_build_object('name', a.name, 'slug', a.slug, 'role', wa.role)
                         ORDER BY wa.position)
           FROM work_authors wa JOIN authors a ON a.id = wa.author_id
          WHERE wa.work_id = w.id), '[]'::json) AS authors_json,
       coalesce((
         SELECT json_agg(DISTINCT s.name)
           FROM work_subjects ws JOIN subjects s ON s.id = ws.subject_id
          WHERE ws.work_id = w.id), '[]'::json) AS subjects_json,
       (SELECT e.gutenberg_id FROM editions e
         WHERE e.work_id = w.id AND e.gutenberg_id IS NOT NULL AND e.deleted_at IS NULL
         ORDER BY e.gutenberg_id LIMIT 1) AS gutenberg_id,
       -- Edition/cover facts arrive as ONE nullable json object. Scalar
       -- subqueries are NULL when a work has no edition yet, and sqlc cannot
       -- see that through casts; a json column is nullable by construction and
       -- is parsed defensively in Go, so the scan can never fail on an
       -- edition-less work.
       (SELECT json_build_object(
                  'edition_id', e.id::text,
                  'cover_key',  ca.object_key,
                  'license',    ca.license::text)
          FROM editions e
          LEFT JOIN cover_assets ca ON ca.edition_id = e.id AND ca.is_primary
         WHERE e.work_id = w.id AND e.deleted_at IS NULL
         ORDER BY e.published_on DESC NULLS LAST
         LIMIT 1) AS edition_json,
       (SELECT count(*)::bigint FROM reviews r
         WHERE r.work_id = w.id AND r.deleted_at IS NULL) AS review_count
  FROM works w
 WHERE w.deleted_at IS NULL
   AND (sqlc.narg('ids')::uuid[] IS NULL OR w.id = ANY(sqlc.narg('ids')::uuid[]))
 ORDER BY w.id
 LIMIT @lim OFFSET @off;

-- name: AuthorSearchDocuments :many
SELECT a.id, a.slug, a.name, a.sort_name, a.bio, a.birth_year, a.death_year,
       a.is_claimed,
       (SELECT count(*)::bigint FROM work_authors wa JOIN works w ON w.id = wa.work_id
         WHERE wa.author_id = a.id AND w.deleted_at IS NULL) AS work_count
  FROM authors a
 WHERE a.deleted_at IS NULL
   AND (sqlc.narg('ids')::uuid[] IS NULL OR a.id = ANY(sqlc.narg('ids')::uuid[]))
 ORDER BY a.id
 LIMIT @lim OFFSET @off;

-- name: ClubSearchDocuments :many
SELECT c.id, c.slug, c.name, c.description, c.is_private,
       (SELECT count(*)::bigint FROM club_memberships cm WHERE cm.club_id = c.id) AS member_count
  FROM clubs c
 WHERE c.deleted_at IS NULL AND NOT c.is_private
 ORDER BY c.id
 LIMIT @lim OFFSET @off;

-- name: ListIndexedWorkIDs :many
SELECT id FROM works WHERE deleted_at IS NULL ORDER BY id LIMIT @lim OFFSET @off;
