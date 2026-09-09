-- Book clubs: memberships, roles, channels, messages, and spoiler gating.
--
-- Spoiler gating is data, not UI: a channel carries spoiler_threshold_bp, and
-- the read path joins the member's progress on the club's current work so the
-- API can refuse to return content the reader has not reached yet. Hiding
-- spoilers only in the client would leak them through the network response.

-- name: CreateClub :one
INSERT INTO clubs (slug, name, description, is_private, created_by)
VALUES (@slug, @name, @description, @is_private, @created_by)
RETURNING *;

-- name: GetClubBySlug :one
SELECT c.*, u.username AS organizer_username,
       (SELECT count(*)::bigint FROM club_memberships cm WHERE cm.club_id = c.id) AS member_count
  FROM clubs c
  LEFT JOIN users u ON u.id = c.created_by
 WHERE c.slug = @slug AND c.deleted_at IS NULL;

-- name: GetClubByID :one
SELECT * FROM clubs WHERE id = @id AND deleted_at IS NULL;

-- name: ListClubs :many
SELECT c.*,
       (SELECT count(*)::bigint FROM club_memberships cm WHERE cm.club_id = c.id) AS member_count,
       (SELECT w.title FROM club_reads cr JOIN works w ON w.id = cr.work_id
         WHERE cr.club_id = c.id AND cr.is_current LIMIT 1) AS current_read_title,
       (SELECT w.slug FROM club_reads cr JOIN works w ON w.id = cr.work_id
         WHERE cr.club_id = c.id AND cr.is_current LIMIT 1) AS current_read_slug
  FROM clubs c
 WHERE c.deleted_at IS NULL
   AND (NOT c.is_private OR EXISTS (SELECT 1 FROM club_memberships cm
                                     WHERE cm.club_id = c.id AND cm.user_id = sqlc.narg('viewer_id')))
   AND (sqlc.narg('query')::text IS NULL OR c.name ILIKE '%' || sqlc.narg('query') || '%')
 ORDER BY
   CASE WHEN sqlc.narg('viewer_id')::uuid IS NOT NULL AND EXISTS (
          SELECT 1 FROM club_memberships cm WHERE cm.club_id = c.id
            AND cm.user_id = sqlc.narg('viewer_id')) THEN 0 ELSE 1 END,
   member_count DESC, c.created_at DESC
 LIMIT @lim OFFSET @off;

-- name: JoinClub :one
INSERT INTO club_memberships (club_id, user_id, role) VALUES (@club_id, @user_id, 'member')
ON CONFLICT (club_id, user_id) DO UPDATE SET role = club_memberships.role
RETURNING *;

-- name: LeaveClub :execrows
DELETE FROM club_memberships WHERE club_id = @club_id AND user_id = @user_id;

-- name: GetClubMembership :one
SELECT * FROM club_memberships WHERE club_id = @club_id AND user_id = @user_id;

-- name: ListClubMemberships :many
SELECT cm.*, u.username, p.display_name
  FROM club_memberships cm
  JOIN users u ON u.id = cm.user_id
  LEFT JOIN profiles p ON p.user_id = u.id
 WHERE cm.club_id = @club_id
 ORDER BY cm.role DESC, cm.joined_at
 LIMIT @lim OFFSET @off;

-- name: ListClubsForUser :many
SELECT c.*, cm.role,
       (SELECT count(*)::bigint FROM club_memberships x WHERE x.club_id = c.id) AS member_count
  FROM club_memberships cm JOIN clubs c ON c.id = cm.club_id
 WHERE cm.user_id = @user_id AND c.deleted_at IS NULL
 ORDER BY cm.joined_at DESC;

-- name: SetClubMemberRole :execrows
UPDATE club_memberships SET role = @role
 WHERE club_id = @club_id AND user_id = @user_id;

-- name: SetCurrentClubRead :one
INSERT INTO club_reads (club_id, work_id, starts_on, ends_on, is_current)
VALUES (@club_id, @work_id, @starts_on, sqlc.narg('ends_on'), true)
RETURNING *;

-- name: ClearCurrentClubRead :execrows
UPDATE club_reads SET is_current = false WHERE club_id = @club_id AND is_current;

-- name: GetCurrentClubRead :one
SELECT cr.*, w.slug AS work_slug, w.title AS work_title
  FROM club_reads cr JOIN works w ON w.id = cr.work_id
 WHERE cr.club_id = @club_id AND cr.is_current;

-- name: ListChannelsForClub :many
SELECT ch.*,
       -- The reader's own progress on this channel's work, so the API can
       -- decide whether the channel is spoiler-gated for them.
       coalesce((SELECT r.progress_bp FROM reading_sessions r
                  WHERE r.user_id = sqlc.narg('viewer_id') AND r.work_id = ch.work_id
                  ORDER BY r.updated_at DESC LIMIT 1), 0)::integer AS viewer_progress_bp
  FROM channels ch
 WHERE ch.club_id = @club_id
 ORDER BY ch.position, ch.name;

-- name: CreateChannel :one
INSERT INTO channels (club_id, kind, name, topic, work_id, spoiler_threshold_bp, position)
VALUES (@club_id, @kind, @name, @topic, sqlc.narg('work_id'),
        sqlc.narg('spoiler_threshold_bp'), @position)
RETURNING *;

-- name: GetChannelByID :one
SELECT * FROM channels WHERE id = @id;

-- name: CreateMessage :one
INSERT INTO messages (channel_id, user_id, body, reply_to, has_spoilers)
VALUES (@channel_id, @user_id, @body, sqlc.narg('reply_to'), @has_spoilers)
RETURNING *;

-- name: ListMessages :many
-- Keyset pagination on (created_at, id): chat history is unbounded and OFFSET
-- would get slower the deeper a reader scrolls.
SELECT m.*, u.username, p.display_name
  FROM messages m
  JOIN users u ON u.id = m.user_id
  LEFT JOIN profiles p ON p.user_id = u.id
 WHERE m.channel_id = @channel_id
   AND m.deleted_at IS NULL
   AND (@before_created_at::timestamptz IS NULL OR m.created_at < @before_created_at)
   AND (@before_id::uuid IS NULL OR m.id <> @before_id)
 ORDER BY m.created_at DESC
 LIMIT @lim;

-- name: SoftDeleteMessage :execrows
UPDATE messages SET deleted_at = now()
 WHERE id = @id AND (user_id = @user_id OR sqlc.narg('is_moderator')::boolean)
   AND deleted_at IS NULL;

-- name: ChannelIsSpoilerGatedFor :one
-- The authorization question the message and channel endpoints must ask before
-- returning content: has this reader passed the channel's spoiler threshold?
SELECT
  ch.spoiler_threshold_bp IS NULL AS ungated,
  coalesce((SELECT r.progress_bp FROM reading_sessions r
             WHERE r.user_id = @user_id AND r.work_id = ch.work_id
             ORDER BY r.updated_at DESC LIMIT 1), 0)::integer AS progress_bp,
  ch.spoiler_threshold_bp
  FROM channels ch
 WHERE ch.id = @channel_id;
