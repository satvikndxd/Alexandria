-- Alexandria · Migration 0014
-- Web Push subscriptions.
--
-- Push is opt-in per reader and per device: the subscription object (endpoint
-- plus the client's p256dh and auth secrets) is personal data, stored only
-- while the reader keeps it, and deleted with the account (ON DELETE CASCADE)
-- or on a 404/410 from the push service.
--
-- We store the raw endpoint but never log it; payloads are encrypted per RFC
-- 8291 before they leave the server, so the push service learns that a
-- reader was notified, never what about.

CREATE TABLE push_subscriptions (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id        uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  endpoint       text NOT NULL,
  p256dh         bytea NOT NULL,
  auth           bytea NOT NULL,
  user_agent     text,
  created_at     timestamptz NOT NULL DEFAULT now(),
  last_pushed_at timestamptz,
  UNIQUE (user_id, endpoint)
);
CREATE INDEX push_subscriptions_user_idx ON push_subscriptions (user_id);
CREATE INDEX push_subscriptions_due_idx ON push_subscriptions (last_pushed_at);
