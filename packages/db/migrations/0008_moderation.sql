-- Alexandria · Migration 0008
-- Moderation, trust, and the anti-slop ledger.
-- Structural friction lives in three places:
--   1. CHECK constraints (minimum lengths) — migrations 0005/0006
--   2. contribution_ledger — append-only record used for daily posting caps
--   3. reputation on users — unlocks higher limits ("Trusted Reader")

CREATE TYPE report_reason AS ENUM (
  'spam', 'ai_slop', 'harassment', 'hate', 'threat', 'sexual_content',
  'misinformation', 'copyright', 'impersonation', 'fake_review', 'other'
);
CREATE TYPE report_status AS ENUM ('open', 'in_review', 'actioned', 'dismissed');

CREATE TABLE reports (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  reporter_id  uuid REFERENCES users(id) ON DELETE SET NULL,
  subject_type text NOT NULL CHECK (subject_type IN ('user','review','comment','message','note','club','cover')),
  subject_id   uuid NOT NULL,
  reason       report_reason NOT NULL,
  details      text NOT NULL DEFAULT '',
  status       report_status NOT NULL DEFAULT 'open',
  assigned_to  uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  resolved_at  timestamptz
);
CREATE INDEX reports_queue_idx ON reports (status, created_at) WHERE status IN ('open', 'in_review');

CREATE TYPE moderation_action_kind AS ENUM (
  'warning', 'content_removal', 'temp_suspension', 'permanent_ban', 'restore', 'note'
);

CREATE TABLE moderation_actions (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  moderator_id uuid REFERENCES users(id) ON DELETE SET NULL,
  target_user  uuid REFERENCES users(id) ON DELETE CASCADE,
  report_id    uuid REFERENCES reports(id) ON DELETE SET NULL,
  kind         moderation_action_kind NOT NULL,
  rationale    text NOT NULL CHECK (length(rationale) >= 10),  -- every action is justified
  expires_at   timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX moderation_actions_target_idx ON moderation_actions (target_user, created_at DESC);

-- Append-only ledger of user contributions; posting caps are computed by
-- counting rows in the trailing window. Cheap, transparent, un-gameable.
CREATE TABLE contribution_ledger (
  id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind       text NOT NULL CHECK (kind IN ('review','comment','message','note','shelf_add')),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX contribution_ledger_window_idx ON contribution_ledger (user_id, kind, created_at DESC);

CREATE TABLE notifications (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind       text NOT NULL,
  payload    jsonb NOT NULL DEFAULT '{}',
  read_at    timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notifications_user_idx ON notifications (user_id, created_at DESC) WHERE read_at IS NULL;
