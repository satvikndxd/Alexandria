-- Alexandria · Migration 0010
-- Authentication ceremonies and account lifecycle.
--
-- Auth is passkey-first (WebAuthn) with an email magic-link fallback. There is
-- deliberately NO password-based primary flow: passwords are the single largest
-- source of credential-stuffing and reuse breaches, and a reading platform has
-- no business holding that liability. `users.password_hash` (migration 0002)
-- remains only as an operator-set break-glass digest.
--
-- Every ceremony is a short-lived, single-use row in auth_challenges. Storing
-- ceremonies in Postgres (rather than process memory) keeps the API stateless
-- and horizontally scalable: the node that begins a ceremony need not be the
-- node that finishes it.

CREATE TYPE auth_challenge_kind AS ENUM (
  'webauthn_registration',
  'webauthn_login',
  'magic_link',
  'email_verification'
);

CREATE TABLE auth_challenges (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  kind         auth_challenge_kind NOT NULL,
  -- NULL for login-begin: the user is only known once the authenticator
  -- asserts, so the ceremony cannot be bound to an account up front.
  user_id      uuid REFERENCES users(id) ON DELETE CASCADE,
  email        citext,
  -- WebAuthn challenge bytes, or the HMAC secret of a magic-link token.
  challenge    bytea NOT NULL,
  -- Serialized webauthn.SessionData (public-key ceremony state).
  session_data jsonb NOT NULL DEFAULT '{}',
  consumed_at  timestamptz,
  expires_at   timestamptz NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  -- A challenge is single-use: consuming it is an atomic conditional UPDATE.
  CONSTRAINT auth_challenges_not_expired CHECK (expires_at > created_at)
);
CREATE INDEX auth_challenges_email_idx
  ON auth_challenges (email, kind, created_at DESC)
  WHERE consumed_at IS NULL;
CREATE INDEX auth_challenges_expiry_idx ON auth_challenges (expires_at);

-- Auth throttling. Structural friction applied to the login path itself:
-- magic links and failed assertions are counted per (email|ip, kind) so a
-- flood cannot be used to harass a mailbox or brute-force an account.
CREATE TABLE auth_attempts (
  id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  kind       auth_challenge_kind NOT NULL,
  bucket_key text NOT NULL,              -- hashed email or IP; never raw
  allowed    boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX auth_attempts_window_idx ON auth_attempts (bucket_key, kind, created_at DESC);

-- Transactional email outbox. Magic links are queued in the same tx that
-- creates the challenge and delivered by a worker, so a slow/blocked SMTP
-- relay can never stall an auth request or lose a token.
CREATE TABLE email_outbox (
  id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  to_email   citext NOT NULL,
  template   text NOT NULL CHECK (template IN ('magic_link', 'email_verification', 'welcome', 'moderation_notice')),
  payload    jsonb NOT NULL DEFAULT '{}',
  attempts   integer NOT NULL DEFAULT 0,
  sent_at    timestamptz,
  last_error text,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX email_outbox_pending_idx ON email_outbox (created_at) WHERE sent_at IS NULL;

-- Account lifecycle. Suspension is a window, not a boolean, so "temporary
-- suspension" expires without a moderator having to remember to undo it.
ALTER TABLE users
  ADD COLUMN suspended_until timestamptz,
  ADD COLUMN last_login_at   timestamptz;

-- Authors get canonical slugs so /authors/jane-austen is addressable and
-- stable even if a display name is corrected later.
ALTER TABLE authors ADD COLUMN slug text;
-- Backfill before tightening the constraint so the migration is safe on a
-- populated database. The id suffix guarantees uniqueness for names that
-- collide ("William James" vs "William James"); the ingest worker prefers a
-- clean slug for rows it creates itself.
UPDATE authors
   SET slug = trim(BOTH '-' FROM regexp_replace(lower(name), '[^a-z0-9]+', '-', 'g'))
              || '-' || left(replace(id::text, '-', ''), 6)
 WHERE slug IS NULL;
ALTER TABLE authors ALTER COLUMN slug SET NOT NULL;
CREATE UNIQUE INDEX authors_slug_idx ON authors (slug);
