-- Alexandria · Migration 0003
-- Bibliography: FRBR model. A Work is the abstract literary creation;
-- an Edition is a concrete manifestation (a specific ISBN, printing, or
-- Gutenberg text). Reviews and scholar notes attach to Works; reading
-- progress and covers attach to Editions.

CREATE TABLE authors (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name         text NOT NULL,
  sort_name    text NOT NULL,                     -- "Austen, Jane"
  bio          text NOT NULL DEFAULT '',
  birth_year   integer,
  death_year   integer,
  openlibrary_id text UNIQUE,                     -- e.g. OL21594A
  wikidata_id  text,
  is_claimed   boolean NOT NULL DEFAULT false,    -- indie author claimed profile
  claimed_by   uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  deleted_at   timestamptz
);
CREATE TRIGGER authors_updated_at BEFORE UPDATE ON authors
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX authors_name_trgm_idx ON authors USING gin (name gin_trgm_ops);

CREATE TABLE works (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug           text NOT NULL UNIQUE,            -- "pride-and-prejudice"
  title          text NOT NULL,
  subtitle       text,
  original_language text NOT NULL DEFAULT 'en',   -- BCP-47
  first_published integer,                        -- year; negative = BCE
  description    text NOT NULL DEFAULT '',
  is_public_domain boolean NOT NULL DEFAULT false,
  openlibrary_id text UNIQUE,                     -- e.g. OL66554W
  rating_sum     bigint NOT NULL DEFAULT 0,       -- denormalized; rating stored in half-stars (1..10)
  rating_count   bigint NOT NULL DEFAULT 0,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz
);
CREATE TRIGGER works_updated_at BEFORE UPDATE ON works
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX works_title_trgm_idx ON works USING gin (title gin_trgm_ops);

CREATE TABLE work_authors (
  work_id   uuid NOT NULL REFERENCES works(id) ON DELETE CASCADE,
  author_id uuid NOT NULL REFERENCES authors(id) ON DELETE CASCADE,
  role      text NOT NULL DEFAULT 'author',       -- author | translator | editor | illustrator
  position  integer NOT NULL DEFAULT 0,
  PRIMARY KEY (work_id, author_id, role)
);
CREATE INDEX work_authors_author_idx ON work_authors (author_id);

CREATE TABLE subjects (
  id   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug text NOT NULL UNIQUE,
  name text NOT NULL,
  kind text NOT NULL DEFAULT 'subject'            -- subject | genre | movement | period
);

CREATE TABLE work_subjects (
  work_id    uuid NOT NULL REFERENCES works(id) ON DELETE CASCADE,
  subject_id uuid NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
  PRIMARY KEY (work_id, subject_id)
);
CREATE INDEX work_subjects_subject_idx ON work_subjects (subject_id);

CREATE TYPE edition_format AS ENUM ('hardcover', 'paperback', 'ebook', 'audiobook', 'gutenberg', 'other');

CREATE TABLE editions (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  work_id        uuid NOT NULL REFERENCES works(id) ON DELETE CASCADE,
  title          text NOT NULL,                   -- edition title may differ (translations)
  isbn10         text,
  isbn13         text,
  format         edition_format NOT NULL DEFAULT 'other',
  publisher      text,
  published_on   date,
  language       text NOT NULL DEFAULT 'en',
  page_count     integer,
  gutenberg_id   integer,                         -- PG etext number, if a public-domain text
  openlibrary_id text,                            -- e.g. OL12345M
  metadata       jsonb NOT NULL DEFAULT '{}',     -- translator notes, binding, series...
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz
);
CREATE TRIGGER editions_updated_at BEFORE UPDATE ON editions
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE UNIQUE INDEX editions_isbn13_idx ON editions (isbn13) WHERE isbn13 IS NOT NULL;
CREATE UNIQUE INDEX editions_gutenberg_idx ON editions (gutenberg_id) WHERE gutenberg_id IS NOT NULL;
CREATE INDEX editions_work_idx ON editions (work_id);

-- Rights-aware cover pipeline. Source ≠ license ≠ cache policy.
CREATE TYPE cover_license AS ENUM ('public_domain', 'cc0', 'cc_by', 'cc_by_sa', 'fair_use_thumbnail', 'user_upload', 'unknown');

CREATE TABLE cover_assets (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  edition_id       uuid NOT NULL REFERENCES editions(id) ON DELETE CASCADE,
  source           text NOT NULL,                 -- openlibrary | wikimedia | user | generated
  source_url       text,
  license          cover_license NOT NULL DEFAULT 'unknown',
  license_url      text,
  attribution_text text,
  object_key       text,                          -- MinIO key when cached locally
  cacheable        boolean NOT NULL DEFAULT false,
  cache_expires_at timestamptz,
  uploaded_by      uuid REFERENCES users(id) ON DELETE SET NULL,
  is_primary       boolean NOT NULL DEFAULT false,
  created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX cover_assets_edition_idx ON cover_assets (edition_id);
CREATE UNIQUE INDEX cover_assets_primary_idx ON cover_assets (edition_id) WHERE is_primary;

-- External identifiers beyond ISBN/OL (Wikidata, LCCN, OCLC, Goodreads legacy IDs…)
CREATE TABLE external_identifiers (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  edition_id uuid NOT NULL REFERENCES editions(id) ON DELETE CASCADE,
  namespace  text NOT NULL,                       -- 'lccn' | 'oclc' | 'wikidata' | ...
  value      text NOT NULL,
  UNIQUE (namespace, value)
);
