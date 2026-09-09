-- Personal library: shelves, shelf items, reading sessions, annotations.
--
-- Every query here runs under Row-Level Security (migration 0011) AND filters
-- on user_id explicitly. That duplication is intentional: RLS is the last line
-- of defense, and an explicit predicate keeps the plan index-driven and the
-- intent readable in review.

-- name: EnsureSystemShelves :exec
-- Called once at registration. Idempotent, so it is safe to retry.
INSERT INTO shelves (user_id, kind, name)
VALUES (@user_id, 'want_to_read', 'Want to Read'),
       (@user_id, 'reading',      'Currently Reading'),
       (@user_id, 'read',         'Read'),
       (@user_id, 'dnf',          'Did Not Finish'),
       (@user_id, 'favorites',    'Favorites')
ON CONFLICT DO NOTHING;

-- name: ListShelvesForUser :many
SELECT s.*,
       (SELECT count(*) FROM shelf_items si WHERE si.shelf_id = s.id) AS item_count
  FROM shelves s
 WHERE s.user_id = @user_id
 ORDER BY
   CASE s.kind
     WHEN 'reading'      THEN 0
     WHEN 'want_to_read' THEN 1
     WHEN 'read'         THEN 2
     WHEN 'favorites'    THEN 3
     WHEN 'dnf'          THEN 4
     ELSE 5
   END,
   s.name;

-- name: GetShelfByID :one
SELECT * FROM shelves WHERE id = @id AND user_id = @user_id;

-- name: GetSystemShelf :one
SELECT * FROM shelves WHERE user_id = @user_id AND kind = @kind;

-- name: CreateCustomShelf :one
INSERT INTO shelves (user_id, kind, name, is_private)
VALUES (@user_id, 'custom', @name, @is_private)
RETURNING *;

-- name: UpdateShelfPrivacy :execrows
UPDATE shelves SET is_private = @is_private
 WHERE id = @id AND user_id = @user_id AND kind = 'custom';

-- name: DeleteCustomShelf :execrows
-- System shelves are permanent (the library's shape depends on them); only
-- custom shelves can be removed. Items cascade with the shelf.
DELETE FROM shelves WHERE id = @id AND user_id = @user_id AND kind = 'custom';

-- name: AddToShelf :one
INSERT INTO shelf_items (shelf_id, work_id, edition_id, format)
VALUES (@shelf_id, @work_id, sqlc.narg('edition_id'), sqlc.narg('format'))
ON CONFLICT (shelf_id, work_id) DO UPDATE SET
  edition_id = COALESCE(EXCLUDED.edition_id, shelf_items.edition_id),
  format     = COALESCE(EXCLUDED.format, shelf_items.format)
RETURNING *;

-- name: RemoveFromShelf :execrows
DELETE FROM shelf_items WHERE shelf_id = @shelf_id AND work_id = @work_id;

-- name: RemoveWorkFromUserShelves :execrows
-- Moving a book between reading states must not leave it on two shelves at
-- once ("Currently Reading" and "Read" simultaneously is a data bug, not a
-- user preference). Favorites is exempt: it is orthogonal to reading state.
DELETE FROM shelf_items
 WHERE work_id = @work_id
   AND shelf_id IN (SELECT s.id FROM shelves s
                     WHERE s.user_id = @user_id
                       AND s.kind IN ('want_to_read', 'reading', 'read', 'dnf')
                       AND s.kind <> @keep_kind);

-- name: GetShelfItem :one
SELECT * FROM shelf_items WHERE id = @id;

-- name: ListShelfItemsWithWork :many
-- The library view. One row per shelved work, with the most recent reading
-- session joined laterally so rereads never multiply rows.
SELECT si.id, si.shelf_id, si.work_id, si.edition_id, si.format, si.added_at,
       s.kind AS shelf_kind, s.name AS shelf_name, s.is_private,
       w.slug AS work_slug, w.title AS work_title, w.first_published,
       w.rating_sum, w.rating_count, w.is_public_domain,
       -- The lateral join yields NULL for shelved works with no session yet;
       -- coalesce explicitly or the scan fails on the first such row.
       rs.id AS session_id,
       coalesce(rs.progress_bp, 0)::integer AS progress_bp,
       rs.started_on, rs.finished_on, coalesce(rs.dnf, false)::boolean AS dnf,
       (SELECT a.name FROM work_authors wa JOIN authors a ON a.id = wa.author_id
         WHERE wa.work_id = w.id ORDER BY wa.position LIMIT 1) AS primary_author,
       (SELECT ca.object_key FROM editions e
          JOIN cover_assets ca ON ca.edition_id = e.id AND ca.is_primary
         WHERE e.work_id = w.id AND e.deleted_at IS NULL LIMIT 1) AS cover_key
  FROM shelf_items si
  JOIN shelves s ON s.id = si.shelf_id
  JOIN works w   ON w.id = si.work_id AND w.deleted_at IS NULL
  LEFT JOIN LATERAL (
        SELECT r.id, r.progress_bp, r.started_on, r.finished_on, r.dnf
          FROM reading_sessions r
         WHERE r.user_id = s.user_id AND r.work_id = si.work_id
         ORDER BY r.updated_at DESC
         LIMIT 1
       ) rs ON true
 WHERE s.user_id = @user_id
   AND (sqlc.narg('shelf_kind')::shelf_kind IS NULL OR s.kind = sqlc.narg('shelf_kind'))
 ORDER BY si.added_at DESC
 LIMIT @lim OFFSET @off;

-- name: GetLibraryEntry :one
-- What the book page needs to render the caller's own state.
SELECT si.id AS shelf_item_id, s.kind AS shelf_kind, si.format, si.added_at,
       rs.id AS session_id,
       coalesce(rs.progress_bp, 0)::integer AS progress_bp,
       rs.started_on, rs.finished_on, coalesce(rs.dnf, false)::boolean AS dnf
  FROM shelves s
  JOIN shelf_items si ON si.shelf_id = s.id AND si.work_id = @work_id
  LEFT JOIN LATERAL (
        SELECT r.id, r.progress_bp, r.started_on, r.finished_on, r.dnf
          FROM reading_sessions r
         WHERE r.user_id = s.user_id AND r.work_id = @work_id
         ORDER BY r.updated_at DESC LIMIT 1
       ) rs ON true
 WHERE s.user_id = @user_id
 ORDER BY CASE s.kind WHEN 'reading' THEN 0 WHEN 'read' THEN 1 ELSE 2 END
 LIMIT 1;

-- name: LibraryCountsForUser :many
SELECT s.kind, count(si.id)::bigint AS n
  FROM shelves s LEFT JOIN shelf_items si ON si.shelf_id = s.id
 WHERE s.user_id = @user_id
 GROUP BY s.kind;

-- ---- Reading sessions -------------------------------------------------------

-- name: UpsertReadingProgress :one
-- Upserts the single open session for (user, work). Reaching 100% closes it and
-- stamps the finish date; the partial unique index guarantees one open session.
INSERT INTO reading_sessions (user_id, work_id, edition_id, format, started_on, progress_bp)
VALUES (@user_id, @work_id, sqlc.narg('edition_id'), sqlc.narg('format'),
        CURRENT_DATE, @progress_bp)
ON CONFLICT (user_id, work_id) WHERE finished_on IS NULL AND NOT dnf
DO UPDATE SET
  progress_bp = EXCLUDED.progress_bp,
  edition_id  = COALESCE(EXCLUDED.edition_id, reading_sessions.edition_id),
  format      = COALESCE(EXCLUDED.format, reading_sessions.format),
  finished_on = CASE WHEN EXCLUDED.progress_bp >= 10000 THEN CURRENT_DATE ELSE NULL END
RETURNING *;

-- name: GetOpenSession :one
SELECT * FROM reading_sessions
 WHERE user_id = @user_id AND work_id = @work_id AND finished_on IS NULL AND NOT dnf;

-- name: FinishSession :execrows
UPDATE reading_sessions
   SET progress_bp = 10000, finished_on = coalesce(@finished_on, CURRENT_DATE)
 WHERE user_id = @user_id AND work_id = @work_id AND finished_on IS NULL AND NOT dnf;

-- name: MarkSessionDNF :execrows
UPDATE reading_sessions
   SET dnf = true, finished_on = coalesce(@finished_on, CURRENT_DATE)
 WHERE user_id = @user_id AND work_id = @work_id AND finished_on IS NULL;

-- name: StartReread :one
-- Closes any open session, then opens a fresh one: a reread is a new row, never
-- a mutation of history.
INSERT INTO reading_sessions (user_id, work_id, edition_id, format, started_on, progress_bp)
VALUES (@user_id, @work_id, sqlc.narg('edition_id'), sqlc.narg('format'), CURRENT_DATE, 0)
RETURNING *;

-- name: ListReadingSessions :many
SELECT * FROM reading_sessions
 WHERE user_id = @user_id AND (sqlc.narg('work_id')::uuid IS NULL OR work_id = sqlc.narg('work_id'))
 ORDER BY updated_at DESC
 LIMIT @lim OFFSET @off;

-- name: ListCurrentlyReading :many
-- "Continue reading" on the home page: open sessions, most recently touched
-- first. Chronological by activity — never by engagement.
SELECT rs.*, w.slug AS work_slug, w.title AS work_title,
       e.gutenberg_id,
       (SELECT a.name FROM work_authors wa JOIN authors a ON a.id = wa.author_id
         WHERE wa.work_id = w.id ORDER BY wa.position LIMIT 1) AS primary_author,
       (SELECT ca.object_key FROM editions e2
          JOIN cover_assets ca ON ca.edition_id = e2.id AND ca.is_primary
         WHERE e2.work_id = w.id AND e2.deleted_at IS NULL LIMIT 1) AS cover_key
  FROM reading_sessions rs
  JOIN works w ON w.id = rs.work_id AND w.deleted_at IS NULL
  LEFT JOIN editions e ON e.id = rs.edition_id
 WHERE rs.user_id = @user_id AND rs.finished_on IS NULL AND NOT rs.dnf
 ORDER BY rs.updated_at DESC
 LIMIT @lim;

-- name: ReadingStatsForUser :one
SELECT
  count(*) FILTER (WHERE finished_on IS NOT NULL AND NOT dnf)::bigint AS books_finished,
  count(*) FILTER (WHERE dnf)::bigint                                 AS books_dnf,
  count(*) FILTER (WHERE finished_on IS NULL AND NOT dnf)::bigint     AS books_in_progress,
  coalesce(sum(progress_bp) FILTER (WHERE finished_on IS NULL AND NOT dnf), 0)::bigint AS open_progress_bp
FROM reading_sessions
WHERE user_id = @user_id;

-- ---- Annotations ------------------------------------------------------------

-- name: CreateAnnotation :one
INSERT INTO annotations (user_id, edition_id, chapter_idx, start_off, end_off, kind, body, is_private)
VALUES (@user_id, @edition_id, @chapter_idx, @start_off, @end_off, @kind, @body, @is_private)
RETURNING *;

-- name: ListAnnotationsForEdition :many
SELECT * FROM annotations
 WHERE user_id = @user_id AND edition_id = @edition_id
   AND (sqlc.narg('chapter_idx')::integer IS NULL OR chapter_idx = sqlc.narg('chapter_idx'))
 ORDER BY chapter_idx, start_off;

-- name: UpdateAnnotationBody :execrows
UPDATE annotations SET body = @body, kind = @kind
 WHERE id = @id AND user_id = @user_id;

-- name: DeleteAnnotation :execrows
DELETE FROM annotations WHERE id = @id AND user_id = @user_id;

-- ---- reading-life projections (right rail) -----------------------------------
-- These exist so the UI never has to invent a number: every figure on the
-- reader's right rail is derived from their own sessions, annotations and
-- reviews, and shows zero when there is genuinely nothing yet.

-- name: YearStatsForUser :one
SELECT
  count(*) FILTER (WHERE rs.finished_on >= make_date(@year, 1, 1))::bigint AS books_finished,
  coalesce(sum(CASE WHEN e.page_count IS NOT NULL AND e.page_count > 0
                    THEN round(e.page_count * rs.progress_bp / 10000.0)
                    ELSE 0 END), 0)::bigint AS pages_read,
  (SELECT count(*)::bigint FROM annotations a WHERE a.user_id = @user_id)::bigint AS notes
FROM reading_sessions rs
LEFT JOIN editions e ON e.id = rs.edition_id
WHERE rs.user_id = @user_id;

-- name: ListActivityDays :many
-- Distinct days on which the reader touched a session, note, or review.
-- The streak is computed in Go so the rule ("consecutive, today or yesterday
-- anchored") lives in one testable place instead of in SQL date arithmetic.
SELECT DISTINCT day FROM (
  SELECT rs2.updated_at::date AS day FROM reading_sessions rs2 WHERE rs2.user_id = @user_id
  UNION
  SELECT a2.created_at::date  FROM annotations a2      WHERE a2.user_id = @user_id
  UNION
  SELECT r2.created_at::date  FROM reviews r2          WHERE r2.user_id = @user_id AND r2.deleted_at IS NULL
) activity
ORDER BY day DESC
LIMIT @lim;

-- name: ListAnnotationsForUser :many
-- The reader's margin notes across every edition, for the Notes page.
-- RLS-scoped like all annotation reads: only the caller's rows can appear.
SELECT a.*, e.title AS edition_title, w.slug AS work_slug, w.title AS work_title
  FROM annotations a
  JOIN editions e ON e.id = a.edition_id
  JOIN works w ON w.id = e.work_id
 WHERE a.user_id = @user_id
 ORDER BY a.updated_at DESC
 LIMIT @lim OFFSET @off;
