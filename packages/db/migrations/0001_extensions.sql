-- Alexandria · Migration 0001
-- Extensions and shared helpers.

CREATE EXTENSION IF NOT EXISTS pgcrypto;      -- gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS citext;        -- case-insensitive usernames/emails
CREATE EXTENSION IF NOT EXISTS pg_trgm;       -- trigram fallback search

-- updated_at trigger shared by all tables.
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
