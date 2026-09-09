-- Alexandria · Migration 0011
-- Row-Level Security for private reading data.
--
-- Alexandria's privacy promise is that a reader's reading history is nobody's
-- business but their own. Application-level `WHERE user_id = ?` filters are one
-- buggy handler away from leaking a private shelf, so the database enforces it
-- as well: these policies are the LAST line of defense, not the first.
--
-- Two actors exist:
--   * the request path — sets `app.user_id` per transaction (`SET LOCAL`, so it
--     can never leak across pooled connections)
--   * the service path — ingestion workers, reindexers, moderators; sets
--     `app.service = 'on'` and is not user-scoped
--
-- FORCE ROW LEVEL SECURITY is set so the table OWNER obeys the policies too.
-- That is what makes RLS trustworthy here: a developer connecting with the
-- migration role in local dev still cannot read another reader's private rows.
--
-- Public content (reviews, published scholar notes, club messages) is
-- deliberately NOT covered: policies there would add per-row cost and hide
-- nothing. Public shelves are readable by everyone — that is the feature — so
-- they get an explicit public-read policy OR'd with the owner policy.

CREATE OR REPLACE FUNCTION current_app_user() RETURNS uuid
LANGUAGE sql STABLE AS $$
  SELECT nullif(current_setting('app.user_id', true), '')::uuid
$$;

CREATE OR REPLACE FUNCTION is_service_context() RETURNS boolean
LANGUAGE sql STABLE AS $$
  SELECT coalesce(current_setting('app.service', true), 'off') = 'on'
$$;

ALTER TABLE annotations      ENABLE ROW LEVEL SECURITY;
ALTER TABLE shelves          ENABLE ROW LEVEL SECURITY;
ALTER TABLE shelf_items      ENABLE ROW LEVEL SECURITY;
ALTER TABLE reading_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE notifications    ENABLE ROW LEVEL SECURITY;

ALTER TABLE annotations      FORCE ROW LEVEL SECURITY;
ALTER TABLE shelves          FORCE ROW LEVEL SECURITY;
ALTER TABLE shelf_items      FORCE ROW LEVEL SECURITY;
ALTER TABLE reading_sessions FORCE ROW LEVEL SECURITY;
ALTER TABLE notifications    FORCE ROW LEVEL SECURITY;

-- ---- owner-only tables ------------------------------------------------------
-- Annotations (highlights/notes) and notifications are private without
-- exception: there is no public projection of them.

CREATE POLICY annotations_owner ON annotations FOR ALL
  USING      (is_service_context() OR user_id = current_app_user())
  WITH CHECK (is_service_context() OR user_id = current_app_user());

CREATE POLICY notifications_owner ON notifications FOR ALL
  USING      (is_service_context() OR user_id = current_app_user())
  WITH CHECK (is_service_context() OR user_id = current_app_user());

CREATE POLICY reading_sessions_owner ON reading_sessions FOR ALL
  USING      (is_service_context() OR user_id = current_app_user())
  WITH CHECK (is_service_context() OR user_id = current_app_user());

-- ---- shelves: private by default, readable when explicitly public -----------
-- Permissive policies are OR'd, so the owner always sees their own shelves and
-- everyone sees shelves the owner chose not to mark private.

CREATE POLICY shelves_owner ON shelves FOR ALL
  USING      (is_service_context() OR user_id = current_app_user())
  WITH CHECK (is_service_context() OR user_id = current_app_user());

CREATE POLICY shelves_public_read ON shelves FOR SELECT
  USING (is_private = false);

CREATE POLICY shelf_items_owner ON shelf_items FOR ALL
  USING (
    is_service_context()
    OR EXISTS (SELECT 1 FROM shelves s
                WHERE s.id = shelf_items.shelf_id
                  AND s.user_id = current_app_user())
  )
  WITH CHECK (
    is_service_context()
    OR EXISTS (SELECT 1 FROM shelves s
                WHERE s.id = shelf_items.shelf_id
                  AND s.user_id = current_app_user())
  );

CREATE POLICY shelf_items_public_read ON shelf_items FOR SELECT
  USING (EXISTS (SELECT 1 FROM shelves s
                  WHERE s.id = shelf_items.shelf_id
                    AND s.is_private = false));

-- ---- grants -----------------------------------------------------------------
-- The request path SHOULD connect as a least-privilege, non-owner role in
-- production. FORCE above means correctness does not depend on it, but the
-- role still limits blast radius if a connection string leaks.

DO $$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'alexandria_app') THEN
    CREATE ROLE alexandria_app LOGIN PASSWORD 'alexandria_app';
  END IF;
END $$;

GRANT USAGE ON SCHEMA public TO alexandria_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO alexandria_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO alexandria_app;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO alexandria_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO alexandria_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT USAGE, SELECT ON SEQUENCES TO alexandria_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT EXECUTE ON FUNCTIONS TO alexandria_app;
