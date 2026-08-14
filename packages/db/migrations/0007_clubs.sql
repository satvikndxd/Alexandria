-- Alexandria · Migration 0007
-- Book clubs: channel structure with chapter-gated spoiler thresholds.
-- A channel with spoiler_threshold_bp = 5000 stays blurred/muted for members
-- whose reading progress on the club's current work is below 50%.

CREATE TYPE channel_kind AS ENUM ('text', 'voice', 'video', 'announcements');
CREATE TYPE club_role AS ENUM ('member', 'moderator', 'organizer');

CREATE TABLE clubs (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug        text NOT NULL UNIQUE,
  name        text NOT NULL CHECK (length(name) BETWEEN 3 AND 100),
  description text NOT NULL DEFAULT '',
  is_private  boolean NOT NULL DEFAULT false,
  created_by  uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz
);
CREATE TRIGGER clubs_updated_at BEFORE UPDATE ON clubs
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE club_memberships (
  club_id   uuid NOT NULL REFERENCES clubs(id) ON DELETE CASCADE,
  user_id   uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role      club_role NOT NULL DEFAULT 'member',
  joined_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (club_id, user_id)
);
CREATE INDEX club_memberships_user_idx ON club_memberships (user_id);

-- A club may schedule works ("currently reading Crime and Punishment").
CREATE TABLE club_reads (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  club_id    uuid NOT NULL REFERENCES clubs(id) ON DELETE CASCADE,
  work_id    uuid NOT NULL REFERENCES works(id) ON DELETE CASCADE,
  starts_on  date NOT NULL,
  ends_on    date,
  is_current boolean NOT NULL DEFAULT false
);
CREATE UNIQUE INDEX club_reads_current_idx ON club_reads (club_id) WHERE is_current;

CREATE TABLE channels (
  id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  club_id              uuid NOT NULL REFERENCES clubs(id) ON DELETE CASCADE,
  kind                 channel_kind NOT NULL DEFAULT 'text',
  name                 text NOT NULL CHECK (length(name) BETWEEN 2 AND 64),
  topic                text NOT NULL DEFAULT '',
  work_id              uuid REFERENCES works(id) ON DELETE SET NULL,
  -- Spoiler gate: NULL = ungated, else required progress in basis points.
  spoiler_threshold_bp integer CHECK (spoiler_threshold_bp BETWEEN 0 AND 10000),
  position             integer NOT NULL DEFAULT 0,
  created_at           timestamptz NOT NULL DEFAULT now(),
  UNIQUE (club_id, name)
);
CREATE INDEX channels_club_idx ON channels (club_id, position);

CREATE TABLE messages (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  channel_id uuid NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  body       text NOT NULL CHECK (length(body) BETWEEN 1 AND 4000),
  reply_to   uuid REFERENCES messages(id) ON DELETE SET NULL,
  has_spoilers boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  edited_at  timestamptz,
  deleted_at timestamptz
);
CREATE INDEX messages_channel_idx ON messages (channel_id, created_at DESC);
