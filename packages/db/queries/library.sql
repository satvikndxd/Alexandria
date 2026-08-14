-- name: EnsureSystemShelves :exec
INSERT INTO shelves (user_id, kind, name)
VALUES ($1, 'want_to_read', 'Want to Read'),
       ($1, 'reading', 'Currently Reading'),
       ($1, 'read', 'Read'),
       ($1, 'dnf', 'Did Not Finish'),
       ($1, 'favorites', 'Favorites')
ON CONFLICT DO NOTHING;

-- name: AddToShelf :one
INSERT INTO shelf_items (shelf_id, work_id, edition_id, format)
VALUES ($1, $2, $3, $4)
ON CONFLICT (shelf_id, work_id) DO UPDATE SET edition_id = EXCLUDED.edition_id
RETURNING *;

-- name: ListShelfItems :many
SELECT si.*, w.title, w.slug, w.rating_sum, w.rating_count
  FROM shelf_items si
  JOIN works w ON w.id = si.work_id
 WHERE si.shelf_id = $1
 ORDER BY si.added_at DESC
 LIMIT $2 OFFSET $3;

-- name: UpsertReadingProgress :one
INSERT INTO reading_sessions (user_id, work_id, edition_id, format, started_on, progress_bp)
VALUES ($1, $2, $3, $4, CURRENT_DATE, $5)
ON CONFLICT (user_id, work_id) WHERE finished_on IS NULL AND NOT dnf
DO UPDATE SET progress_bp = EXCLUDED.progress_bp,
              finished_on = CASE WHEN EXCLUDED.progress_bp >= 10000 THEN CURRENT_DATE ELSE NULL END
RETURNING *;

-- name: GetOpenSession :one
SELECT * FROM reading_sessions
 WHERE user_id = $1 AND work_id = $2 AND finished_on IS NULL AND NOT dnf;
