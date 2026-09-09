-- Moderation, trust, and account lifecycle enforcement.
--
-- Human-first by construction: every action row requires a written rationale
-- (CHECK constraint in migration 0008), reports are routed to a queue rather
-- than auto-resolved, and there is no automated content classifier anywhere in
-- this file.

-- name: CreateReport :one
INSERT INTO reports (reporter_id, subject_type, subject_id, reason, details)
VALUES (sqlc.narg('reporter_id'), @subject_type, @subject_id, @reason, @details)
RETURNING *;

-- name: ListOpenReports :many
SELECT r.*, u.username AS reporter_username,
       m.username AS assigned_username
  FROM reports r
  LEFT JOIN users u ON u.id = r.reporter_id
  LEFT JOIN users m ON m.id = r.assigned_to
 -- pgx cannot encode a slice of a custom enum; compare as text instead.
 WHERE r.status::text = ANY(@statuses::text[])
 ORDER BY
   CASE r.reason
     WHEN 'threat' THEN 0 WHEN 'hate' THEN 1 WHEN 'harassment' THEN 2
     WHEN 'sexual_content' THEN 3 WHEN 'copyright' THEN 4 ELSE 5
   END,
   r.created_at ASC
 LIMIT @lim OFFSET @off;

-- name: GetReport :one
SELECT * FROM reports WHERE id = @id;

-- name: AssignReport :execrows
UPDATE reports SET assigned_to = @moderator_id, status = 'in_review'
 WHERE id = @id AND status IN ('open', 'in_review');

-- name: ResolveReport :execrows
UPDATE reports SET status = @status, resolved_at = now()
 WHERE id = @id AND status <> 'actioned';

-- name: CountReportsAgainstSubject :one
SELECT count(*) FROM reports
 WHERE subject_type = @subject_type AND subject_id = @subject_id
   AND status IN ('open', 'in_review');

-- name: RecordModerationAction :one
INSERT INTO moderation_actions (moderator_id, target_user, report_id, kind, rationale, expires_at)
VALUES (@moderator_id, @target_user, sqlc.narg('report_id'), @kind, @rationale,
        sqlc.narg('expires_at'))
RETURNING *;

-- name: ListModerationActionsForUser :many
SELECT ma.*, u.username AS moderator_username
  FROM moderation_actions ma
  LEFT JOIN users u ON u.id = ma.moderator_id
 WHERE ma.target_user = @user_id
 ORDER BY ma.created_at DESC
 LIMIT @lim;

-- name: SuspendUser :execrows
UPDATE users SET suspended_until = @until WHERE id = @user_id;

-- name: LiftSuspension :execrows
UPDATE users SET suspended_until = NULL WHERE id = @user_id;

-- name: ListActiveSuspensions :many
SELECT id, username, suspended_until FROM users
 WHERE suspended_until IS NOT NULL AND suspended_until > now() AND deleted_at IS NULL;

-- name: AdjustReputation :execrows
-- Reputation gates posting limits. It moves only through explicit, recorded
-- events (a moderator action, a published scholar note, a sustained history of
-- un-reported reviews) — never from likes or follower counts, which are the
-- gamification vectors Alexandria refuses to build.
UPDATE users SET reputation = GREATEST(0, reputation + @delta) WHERE id = @user_id;

-- name: ListTrustedReaders :many
SELECT id, username, reputation FROM users
 WHERE reputation >= @threshold AND deleted_at IS NULL
 ORDER BY reputation DESC
 LIMIT @lim;

-- name: DeleteStaleAuthChallenges :execrows
DELETE FROM auth_challenges WHERE expires_at < @now;

-- name: DeleteStaleContributions :execrows
-- The ledger only ever needs its trailing window for cap checks; pruning keeps
-- the index tight without losing auditability (reviews themselves remain).
DELETE FROM contribution_ledger WHERE created_at < @before;

-- name: GetMessage :one
SELECT * FROM messages WHERE id = $1 AND deleted_at IS NULL;
