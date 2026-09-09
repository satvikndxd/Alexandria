-- Social graph and the chronological feed.
--
-- The feed is a UNION over activity types ordered by time. There is no score
-- column, no engagement weighting, and no ranking function — that absence is
-- the product decision. "Community Picks" elsewhere is collaborative filtering
-- over co-occurrence (readers of X also read Y), which is computed in the
-- worker and stored, never blended into this stream.

-- name: FollowUser :execrows
INSERT INTO follows (follower_id, followee_id) VALUES (@follower_id, @followee_id)
ON CONFLICT DO NOTHING;

-- name: UnfollowUser :execrows
DELETE FROM follows WHERE follower_id = @follower_id AND followee_id = @followee_id;

-- name: IsFollowing :one
SELECT EXISTS (SELECT 1 FROM follows
                WHERE follower_id = @follower_id AND followee_id = @followee_id) AS following;

-- name: ListFollowing :many
SELECT u.id, u.username, p.display_name, p.avatar_key, f.created_at AS followed_at
  FROM follows f
  JOIN users u ON u.id = f.followee_id AND u.deleted_at IS NULL
  LEFT JOIN profiles p ON p.user_id = u.id
 WHERE f.follower_id = @user_id
 ORDER BY f.created_at DESC
 LIMIT @lim OFFSET @off;

-- name: ListFollowers :many
SELECT u.id, u.username, p.display_name, p.avatar_key, f.created_at AS followed_at
  FROM follows f
  JOIN users u ON u.id = f.follower_id AND u.deleted_at IS NULL
  LEFT JOIN profiles p ON p.user_id = u.id
 WHERE f.followee_id = @user_id
 ORDER BY f.created_at DESC
 LIMIT @lim OFFSET @off;

-- name: CountFollows :one
SELECT
  (SELECT count(*)::bigint FROM follows f1 WHERE f1.follower_id = @user_id) AS following,
  (SELECT count(*)::bigint FROM follows f2 WHERE f2.followee_id = @user_id) AS followers;

-- name: BlockUser :execrows
INSERT INTO user_blocks (blocker_id, blocked_id) VALUES (@blocker_id, @blocked_id)
ON CONFLICT DO NOTHING;

-- name: UnblockUser :execrows
DELETE FROM user_blocks WHERE blocker_id = @blocker_id AND blocked_id = @blocked_id;

-- name: IsBlocked :one
-- Checked in BOTH directions: a block is not an invitation to be replied to.
SELECT EXISTS (SELECT 1 FROM user_blocks
                WHERE (blocker_id = @a AND blocked_id = @b)
                   OR (blocker_id = @b AND blocked_id = @a)) AS blocked;

-- name: ListBlockedUsers :many
SELECT u.id, u.username, b.created_at
  FROM user_blocks b JOIN users u ON u.id = b.blocked_id
 WHERE b.blocker_id = @user_id
 ORDER BY b.created_at DESC;

-- ---- Feed -------------------------------------------------------------------

-- name: ListFollowingFeed :many
-- Chronological activity from accounts the reader follows. Blocked accounts are
-- excluded in both directions; private shelves never appear.
WITH activity AS (
  SELECT r.id           AS entity_id,
         'review'::text AS kind,
         r.user_id      AS actor_id,
         r.work_id      AS work_id,
         r.created_at   AS created_at,
         r.rating       AS rating,
         left(r.body, 320) AS excerpt,
         r.has_spoilers AS has_spoilers
    FROM reviews r
   WHERE r.deleted_at IS NULL
  UNION ALL
  SELECT si.id, 'shelf_add'::text, s.user_id, si.work_id, si.added_at,
         NULL::integer, NULL::text, false
    FROM shelf_items si
    JOIN shelves s ON s.id = si.shelf_id
   WHERE NOT s.is_private
)
SELECT a.entity_id, a.kind, a.actor_id, a.work_id, a.created_at, a.rating,
       a.excerpt, a.has_spoilers,
       u.username, p.display_name, p.avatar_key,
       w.slug AS work_slug, w.title AS work_title,
       (SELECT au.name FROM work_authors wa JOIN authors au ON au.id = wa.author_id
         WHERE wa.work_id = w.id ORDER BY wa.position LIMIT 1) AS primary_author,
       (SELECT ca.object_key FROM editions e
          JOIN cover_assets ca ON ca.edition_id = e.id AND ca.is_primary
         WHERE e.work_id = w.id AND e.deleted_at IS NULL LIMIT 1) AS cover_key
  FROM activity a
  JOIN follows f  ON f.followee_id = a.actor_id
  JOIN users u    ON u.id = a.actor_id AND u.deleted_at IS NULL
  LEFT JOIN profiles p ON p.user_id = u.id
  JOIN works w    ON w.id = a.work_id AND w.deleted_at IS NULL
 WHERE f.follower_id = @user_id
   AND NOT EXISTS (SELECT 1 FROM user_blocks b
                    WHERE (b.blocker_id = @user_id AND b.blocked_id = a.actor_id)
                       OR (b.blocker_id = a.actor_id AND b.blocked_id = @user_id))
   AND (@since::timestamptz IS NULL OR a.created_at < @since)
 ORDER BY a.created_at DESC
 LIMIT @lim;

-- name: ListRecentReviewsAcrossCommunity :many
-- The logged-out / empty-following home page: newest substantive reviews
-- platform-wide. Reverse-chronological, capped, and it never pretends to be
-- personalized.
SELECT r.*, u.username, p.display_name,
       w.slug AS work_slug, w.title AS work_title,
       (SELECT au.name FROM work_authors wa JOIN authors au ON au.id = wa.author_id
         WHERE wa.work_id = w.id ORDER BY wa.position LIMIT 1) AS primary_author,
       (SELECT ca.object_key FROM editions e
          JOIN cover_assets ca ON ca.edition_id = e.id AND ca.is_primary
         WHERE e.work_id = w.id AND e.deleted_at IS NULL LIMIT 1) AS cover_key
  FROM reviews r
  JOIN users u ON u.id = r.user_id AND u.deleted_at IS NULL
  LEFT JOIN profiles p ON p.user_id = u.id
  JOIN works w ON w.id = r.work_id AND w.deleted_at IS NULL
 WHERE r.deleted_at IS NULL AND NOT r.has_spoilers
 ORDER BY r.created_at DESC
 LIMIT @lim OFFSET @off;

-- name: ListCoOccurringWorks :many
-- "Readers of this also read", computed from shared shelves. Collaborative
-- filtering over co-occurrence, not engagement: a book ranks here because
-- people who shelved X also shelved Y, weighted by how selective Y is
-- (inverse popularity), which is what keeps blockbusters from swallowing every
-- recommendation slot.
WITH co_shelves AS (
  -- Shelves containing the source work. Private shelves are excluded so a
  -- reader's private library never influences anybody's recommendations.
  SELECT DISTINCT si.shelf_id
    FROM shelf_items si
    JOIN shelves s ON s.id = si.shelf_id AND NOT s.is_private
   WHERE si.work_id = @work_id
),
candidates AS (
  -- The minimum-shared threshold is a HAVING, not an outer WHERE: sqlc cannot
  -- resolve CTE aliases in the outer predicate, and filtering here keeps the
  -- join set small either way.
  SELECT si.work_id, count(*)::float AS shared
    FROM shelf_items si
    JOIN shelves s ON s.id = si.shelf_id AND NOT s.is_private
   WHERE si.shelf_id IN (SELECT shelf_id FROM co_shelves)
     AND si.work_id <> @work_id
   GROUP BY si.work_id
  HAVING count(*) >= @min_shared
),
popularity AS (
  SELECT si.work_id, count(*)::float AS total
    FROM shelf_items si
    JOIN shelves s ON s.id = si.shelf_id AND NOT s.is_private
   GROUP BY si.work_id
)
SELECT w.id, w.slug, w.title, w.first_published, w.rating_sum, w.rating_count,
       (c.shared / (1.0 + p.total))::float AS affinity,
       (SELECT a.name FROM work_authors wa JOIN authors a ON a.id = wa.author_id
         WHERE wa.work_id = w.id ORDER BY wa.position LIMIT 1) AS primary_author,
       (SELECT ca.object_key FROM editions e
          JOIN cover_assets ca ON ca.edition_id = e.id AND ca.is_primary
         WHERE e.work_id = w.id AND e.deleted_at IS NULL LIMIT 1) AS cover_key
  FROM candidates c
  JOIN popularity p ON p.work_id = c.work_id
  JOIN works w ON w.id = c.work_id AND w.deleted_at IS NULL
 ORDER BY affinity DESC
 LIMIT @lim;

-- ---- Notifications ----------------------------------------------------------

-- name: CreateNotification :one
INSERT INTO notifications (user_id, kind, payload) VALUES (@user_id, @kind, @payload)
RETURNING *;

-- name: ListNotifications :many
SELECT * FROM notifications
 WHERE user_id = @user_id
   AND (sqlc.narg('unread_only')::boolean IS NULL OR NOT sqlc.narg('unread_only')::boolean OR read_at IS NULL)
 ORDER BY created_at DESC
 LIMIT @lim OFFSET @off;

-- name: MarkNotificationRead :execrows
UPDATE notifications SET read_at = now() WHERE id = @id AND user_id = @user_id AND read_at IS NULL;

-- name: MarkAllNotificationsRead :execrows
UPDATE notifications SET read_at = now() WHERE user_id = @user_id AND read_at IS NULL;

-- name: CountUnreadNotifications :one
SELECT count(*) FROM notifications WHERE user_id = @user_id AND read_at IS NULL;
