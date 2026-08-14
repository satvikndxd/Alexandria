-- Alexandria · Migration 0006
-- Scholar system: verified contributor notes with peer review, citations,
-- and full revision history. Notes attach to a Work plus a chapter reference
-- so the reader can surface them contextually.

CREATE TYPE scholar_status AS ENUM ('pending', 'verified', 'rejected', 'revoked');
CREATE TYPE note_status AS ENUM ('draft', 'in_review', 'published', 'retracted');

CREATE TABLE scholar_profiles (
  user_id       uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  field         text NOT NULL,                       -- "Classical Literature"
  affiliation   text,                                 -- university / institution
  orcid         text UNIQUE,                          -- 0000-0002-XXXX-XXXX
  status        scholar_status NOT NULL DEFAULT 'pending',
  verified_by   uuid REFERENCES users(id) ON DELETE SET NULL,
  verified_at   timestamptz,
  coi_statement text NOT NULL DEFAULT '',             -- conflict-of-interest declaration
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER scholar_profiles_updated_at BEFORE UPDATE ON scholar_profiles
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE scholar_notes (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  work_id      uuid NOT NULL REFERENCES works(id) ON DELETE CASCADE,
  author_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  chapter_ref  text NOT NULL DEFAULT '',              -- "Inferno, Canto I"
  anchor_quote text NOT NULL DEFAULT '',              -- passage the note explains
  title        text NOT NULL CHECK (length(title) BETWEEN 3 AND 200),
  body         text NOT NULL CHECK (length(body) >= 200),  -- friction: no drive-by notes
  kind         text NOT NULL DEFAULT 'context'
               CHECK (kind IN ('context', 'linguistic', 'historical', 'interpretive', 'textual')),
  status       note_status NOT NULL DEFAULT 'draft',
  is_community boolean NOT NULL DEFAULT false,        -- community contributor vs verified scholar
  version      integer NOT NULL DEFAULT 1,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER scholar_notes_updated_at BEFORE UPDATE ON scholar_notes
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX scholar_notes_work_idx ON scholar_notes (work_id, status);

-- Citations are REQUIRED for published notes (enforced in the service layer:
-- publish transition checks count >= 1).
CREATE TABLE note_citations (
  id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  note_id   uuid NOT NULL REFERENCES scholar_notes(id) ON DELETE CASCADE,
  citation  text NOT NULL CHECK (length(citation) >= 10),  -- Chicago/MLA formatted
  url       text,
  position  integer NOT NULL DEFAULT 0
);
CREATE INDEX note_citations_note_idx ON note_citations (note_id);

-- Peer review: two approvals from distinct verified scholars publish a note.
CREATE TABLE note_reviews (
  note_id     uuid NOT NULL REFERENCES scholar_notes(id) ON DELETE CASCADE,
  reviewer_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  approved    boolean NOT NULL,
  comments    text NOT NULL DEFAULT '',
  created_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (note_id, reviewer_id)
);

-- Immutable revision history.
CREATE TABLE note_revisions (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  note_id    uuid NOT NULL REFERENCES scholar_notes(id) ON DELETE CASCADE,
  version    integer NOT NULL,
  title      text NOT NULL,
  body       text NOT NULL,
  edited_by  uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (note_id, version)
);
