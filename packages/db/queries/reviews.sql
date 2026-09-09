-- Reviews, likes, and comments — the human-written core of Alexandria.
--
-- The rating aggregate (works.rating_sum / rating_count) is maintained by the
-- apply_review_rating() trigger from migration 0005, so no query here recomputes
-- it and none can drift from it.

-- name: CreateReview :one
INSERT INTO reviews (user_id, work_id, edition_id, rating, title, body, has_spoilers, prompt_why)
VALUES (@user_id, @work_id, sqlc.narg('edition_id'), @rating, @title, @body,
        @has_spoilers, @prompt_why)
RETURNING *;

-- name: GetReviewByID :one
SELECT r.*, u.username, p.display_name, p.avatar_key,
       w.slug AS work_slug, w.title AS work_title
  FROM reviews r
  JOIN users u ON u.id = r.user_id
  LEFT JOIN profiles p ON p.user_id = u.id
  JOIN works w ON w.id = r.work_id
 WHERE r.id = @id AND r.deleted_at IS NULL;

-- name: GetReviewByUserAndWork :one
SELECT * FROM reviews
 WHERE user_id = @user_id AND work_id = @work_id AND deleted_at IS NULL;

-- name: UpdateReview :one
UPDATE reviews
   SET rating       = @rating,
       title        = @title,
       body         = @body,
       has_spoilers = @has_spoilers,
       prompt_why   = @prompt_why,
       edition_id   = COALESCE(sqlc.narg('edition_id'), edition_id)
 WHERE id = @id AND user_id = @user_id AND deleted_at IS NULL
RETURNING *;

-- name: ListReviewsForWork :many
SELECT r.*, u.username, p.display_name, p.avatar_key, u.role AS author_role
  FROM reviews r
  JOIN users u ON u.id = r.user_id
  LEFT JOIN profiles p ON p.user_id = u.id
 WHERE r.work_id = @work_id AND r.deleted_at IS NULL
 ORDER BY
   CASE @sort::text
     WHEN 'highest' THEN r.rating
     WHEN 'lowest'  THEN -r.rating
     WHEN 'liked'   THEN r.like_count
     ELSE 0
   END DESC,
   r.created_at DESC
 LIMIT @lim OFFSET @off;

-- name: ListReviewsByUser :many
SELECT r.*, w.slug AS work_slug, w.title AS work_title, w.first_published,
       (SELECT a.name FROM work_authors wa JOIN authors a ON a.id = wa.author_id
         WHERE wa.work_id = w.id ORDER BY wa.position LIMIT 1) AS primary_author,
       (SELECT ca.object_key FROM editions e
          JOIN cover_assets ca ON ca.edition_id = e.id AND ca.is_primary
         WHERE e.work_id = w.id AND e.deleted_at IS NULL LIMIT 1) AS cover_key
  FROM reviews r
  JOIN works w ON w.id = r.work_id AND w.deleted_at IS NULL
 WHERE r.user_id = @user_id AND r.deleted_at IS NULL
 ORDER BY r.created_at DESC
 LIMIT @lim OFFSET @off;

-- name: SoftDeleteReview :execrows
UPDATE reviews SET deleted_at = now() WHERE id = @id AND user_id = @user_id AND deleted_at IS NULL;

-- name: RatingDistributionForWork :many
-- Powers the histogram on the book page. GROUP BY over the half-star scale so
-- the UI never has to bucket in application code.
SELECT rating, count(*)::bigint AS n
  FROM reviews
 WHERE work_id = @work_id AND deleted_at IS NULL
 GROUP BY rating
 ORDER BY rating;

-- name: CountReviewsForWork :one
SELECT count(*) FROM reviews WHERE work_id = @work_id AND deleted_at IS NULL;

-- name: RecordContribution :exec
INSERT INTO contribution_ledger (user_id, kind) VALUES (@user_id, @kind);

-- name: CountContributionsSince :one
-- Posting caps are computed by counting ledger rows in a trailing window:
-- cheap, transparent, and impossible to game without actually posting.
SELECT count(*) FROM contribution_ledger
 WHERE user_id = @user_id AND kind = @kind AND created_at > @since;

-- ---- Likes ------------------------------------------------------------------

-- name: LikeReview :execrows
INSERT INTO review_likes (review_id, user_id) VALUES (@review_id, @user_id)
ON CONFLICT DO NOTHING;

-- name: UnlikeReview :execrows
DELETE FROM review_likes WHERE review_id = @review_id AND user_id = @user_id;

-- name: SyncReviewLikeCount :exec
-- Denormalized count refreshed on change; keeps list queries free of a
-- correlated aggregate over review_likes.
UPDATE reviews r
   SET like_count = (SELECT count(*)::integer FROM review_likes rl WHERE rl.review_id = r.id)
 WHERE r.id = @review_id;

-- name: HasUserLikedReview :one
SELECT EXISTS (SELECT 1 FROM review_likes
                WHERE review_id = @review_id AND user_id = @user_id) AS liked;

-- name: ListLikedReviewIDsForUser :many
SELECT review_id FROM review_likes WHERE user_id = @user_id;

-- ---- Comments ---------------------------------------------------------------

-- name: CreateReviewComment :one
INSERT INTO review_comments (review_id, user_id, body) VALUES (@review_id, @user_id, @body)
RETURNING *;

-- name: ListCommentsForReview :many
SELECT c.*, u.username, p.display_name
  FROM review_comments c
  JOIN users u ON u.id = c.user_id
  LEFT JOIN profiles p ON p.user_id = u.id
 WHERE c.review_id = @review_id AND c.deleted_at IS NULL
 ORDER BY c.created_at ASC
 LIMIT @lim OFFSET @off;

-- name: SoftDeleteReviewComment :execrows
UPDATE review_comments SET deleted_at = now()
 WHERE id = @id AND user_id = @user_id AND deleted_at IS NULL;

-- name: GetReviewComment :one
SELECT * FROM review_comments WHERE id = @id AND deleted_at IS NULL;
