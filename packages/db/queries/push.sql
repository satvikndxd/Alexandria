-- name: AddPushSubscription :one
INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth, user_agent)
VALUES (@user_id, @endpoint, @p256dh, @auth, sqlc.narg('user_agent'))
ON CONFLICT (user_id, endpoint) DO UPDATE SET
  p256dh = EXCLUDED.p256dh,
  auth   = EXCLUDED.auth
RETURNING *;

-- name: ListPushSubscriptionsForUser :many
SELECT * FROM push_subscriptions WHERE user_id = @user_id ORDER BY created_at DESC;

-- name: DeletePushSubscription :execrows
DELETE FROM push_subscriptions WHERE user_id = @user_id AND endpoint = @endpoint;

-- name: ListPushDue :many
-- Subscriptions with anything notified since their last push. 'epoch' as the
-- floor means a fresh subscription is due immediately, which is what a
-- reader expects when they flip the switch.
SELECT s.* FROM push_subscriptions s
 WHERE EXISTS (
   SELECT 1 FROM notifications n
    WHERE n.user_id = s.user_id
      AND n.created_at > coalesce(s.last_pushed_at, 'epoch'::timestamptz))
 ORDER BY s.created_at
 LIMIT @lim;

-- name: MarkPushed :execrows
UPDATE push_subscriptions SET last_pushed_at = @pushed_at WHERE id = @id;

-- name: ListNotificationsSince :many
SELECT * FROM notifications
 WHERE user_id = @user_id AND created_at > @since
 ORDER BY created_at DESC
 LIMIT @lim;
