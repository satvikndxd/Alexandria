-- Import support: matching and provenance-stamped writes.
-- Matching is conservative — exact normalized title, or ISBN13 — and unmatched
-- rows are reported to the reader, never invented as new works.

-- name: FindWorksByNormalizedTitle :many
SELECT w.* FROM works w
 WHERE w.deleted_at IS NULL
   AND lower(regexp_replace(w.title, '^(the|a|an)\s+', '', 'i')) = @needle
 ORDER BY w.rating_count DESC
 LIMIT 5;

-- name: FindEditionByISBN13 :one
SELECT e.*, w.id AS work_id FROM editions e
  JOIN works w ON w.id = e.work_id
 WHERE e.isbn13 = @isbn13 AND e.deleted_at IS NULL;

-- name: SetShelfItemRating :execrows
UPDATE shelf_items SET rating = @rating, imported_from = sqlc.narg('imported_from')
 WHERE id = @id;

-- name: MarkAnnotationImported :execrows
UPDATE annotations SET imported_from = @imported_from WHERE id = @id;

-- name: ImportClosedSession :one
-- A finished read from another platform: closed on arrival, with the reader's
-- own dates. The partial unique index only constrains OPEN sessions, so
-- imported history never collides with a current read.
INSERT INTO reading_sessions (user_id, work_id, edition_id, format, started_on, finished_on, progress_bp)
VALUES (@user_id, @work_id, sqlc.narg('edition_id'), sqlc.narg('format'),
        sqlc.narg('started_on'), @finished_on, 10000)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: CustomShelfByName :one
SELECT * FROM shelves WHERE user_id = @user_id AND kind = 'custom' AND name = @name;

-- name: CountImportsSince :one
SELECT count(*) FROM auth_attempts
 WHERE bucket_key = @bucket_key AND kind = 'import' AND created_at > @since;

-- name: RecordImportAttempt :exec
INSERT INTO auth_attempts (kind, bucket_key, allowed) VALUES ('import', @bucket_key, @allowed);
