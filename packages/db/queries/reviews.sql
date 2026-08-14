-- name: CreateReview :one
INSERT INTO reviews (user_id, work_id, edition_id, rating, title, body, has_spoilers, prompt_why)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListReviewsForWork :many
SELECT r.*, u.username, p.display_name
  FROM reviews r
  JOIN users u ON u.id = r.user_id
  LEFT JOIN profiles p ON p.user_id = u.id
 WHERE r.work_id = $1 AND r.deleted_at IS NULL
 ORDER BY r.created_at DESC
 LIMIT $2 OFFSET $3;

-- name: SoftDeleteReview :exec
UPDATE reviews SET deleted_at = now() WHERE id = $1 AND user_id = $2;

-- name: CountRecentContributions :one
SELECT count(*) FROM contribution_ledger
 WHERE user_id = $1 AND kind = $2 AND created_at > now() - $3::interval;

-- name: RecordContribution :exec
INSERT INTO contribution_ledger (user_id, kind) VALUES ($1, $2);
