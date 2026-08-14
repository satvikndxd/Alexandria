-- Alexandria · Migration 0004
-- Personal library: shelves, shelf items, reading sessions & progress.

CREATE TYPE shelf_kind AS ENUM ('want_to_read', 'reading', 'read', 'dnf', 'favorites', 'custom');

CREATE TABLE shelves (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind       shelf_kind NOT NULL DEFAULT 'custom',
  name       text NOT NULL,
  is_private boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER shelves_updated_at BEFORE UPDATE ON shelves
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- One system shelf of each built-in kind per user.
CREATE UNIQUE INDEX shelves_system_kind_idx ON shelves (user_id, kind) WHERE kind <> 'custom';
CREATE UNIQUE INDEX shelves_custom_name_idx ON shelves (user_id, name) WHERE kind = 'custom';

CREATE TYPE read_format AS ENUM ('physical', 'ebook', 'audiobook', 'public_domain', 'library_copy', 'other');

CREATE TABLE shelf_items (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  shelf_id   uuid NOT NULL REFERENCES shelves(id) ON DELETE CASCADE,
  work_id    uuid NOT NULL REFERENCES works(id) ON DELETE CASCADE,
  edition_id uuid REFERENCES editions(id) ON DELETE SET NULL,
  format     read_format,
  added_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (shelf_id, work_id)
);
CREATE INDEX shelf_items_work_idx ON shelf_items (work_id);

-- One row per read-through; supports rereads.
CREATE TABLE reading_sessions (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  work_id     uuid NOT NULL REFERENCES works(id) ON DELETE CASCADE,
  edition_id  uuid REFERENCES editions(id) ON DELETE SET NULL,
  format      read_format,
  started_on  date,
  finished_on date,
  dnf         boolean NOT NULL DEFAULT false,
  -- progress: 0..10000 basis points so integer math stays exact
  progress_bp integer NOT NULL DEFAULT 0 CHECK (progress_bp BETWEEN 0 AND 10000),
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER reading_sessions_updated_at BEFORE UPDATE ON reading_sessions
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX reading_sessions_user_idx ON reading_sessions (user_id, updated_at DESC);
-- At most one open (unfinished, non-DNF) session per user+work.
CREATE UNIQUE INDEX reading_sessions_open_idx ON reading_sessions (user_id, work_id)
  WHERE finished_on IS NULL AND NOT dnf;

-- Highlights & private notes on public-domain texts.
CREATE TABLE annotations (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  edition_id  uuid NOT NULL REFERENCES editions(id) ON DELETE CASCADE,
  -- CFI-like locator: chapter index + character offsets into normalized text
  chapter_idx integer NOT NULL,
  start_off   integer NOT NULL,
  end_off     integer NOT NULL,
  kind        text NOT NULL DEFAULT 'highlight' CHECK (kind IN ('highlight', 'note', 'bookmark')),
  body        text NOT NULL DEFAULT '',
  is_private  boolean NOT NULL DEFAULT true,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER annotations_updated_at BEFORE UPDATE ON annotations
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX annotations_user_edition_idx ON annotations (user_id, edition_id, chapter_idx);
