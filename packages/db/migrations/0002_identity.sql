-- Alexandria · Migration 0002
-- Identity: users, profiles, credentials, follows, blocks.
-- Auth is passkey-first (WebAuthn); password hash column exists only for the
-- magic-link/password fallback and stores Argon2id digests.

CREATE TYPE user_role AS ENUM ('reader', 'trusted_reader', 'scholar', 'moderator', 'admin');

CREATE TABLE users (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  username       citext NOT NULL UNIQUE CHECK (length(username) BETWEEN 3 AND 32),
  email          citext NOT NULL UNIQUE,
  email_verified boolean NOT NULL DEFAULT false,
  password_hash  text,                          -- Argon2id; NULL for passkey-only accounts
  role           user_role NOT NULL DEFAULT 'reader',
  reputation     integer NOT NULL DEFAULT 0,    -- gates posting limits (anti-slop)
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  deleted_at     timestamptz                    -- soft delete; hard purge via async job
);
CREATE TRIGGER users_updated_at BEFORE UPDATE ON users
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE profiles (
  user_id      uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  display_name text NOT NULL DEFAULT '',
  bio          text NOT NULL DEFAULT '' CHECK (length(bio) <= 2000),
  avatar_key   text,                            -- MinIO object key, never a hotlink
  pronouns     text,
  location     text,
  is_private   boolean NOT NULL DEFAULT false,
  updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER profiles_updated_at BEFORE UPDATE ON profiles
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- WebAuthn credentials (go-webauthn shapes).
CREATE TABLE webauthn_credentials (
  id              bytea PRIMARY KEY,            -- credential ID from authenticator
  user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  public_key      bytea NOT NULL,
  attestation_aaguid uuid,
  sign_count      bigint NOT NULL DEFAULT 0,
  transports      text[] NOT NULL DEFAULT '{}',
  nickname        text NOT NULL DEFAULT 'Passkey',
  created_at      timestamptz NOT NULL DEFAULT now(),
  last_used_at    timestamptz
);
CREATE INDEX webauthn_credentials_user_idx ON webauthn_credentials (user_id);

CREATE TABLE sessions (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash   bytea NOT NULL UNIQUE,           -- SHA-256 of opaque session token
  user_agent   text,
  ip_hash      bytea,                           -- salted hash; raw IPs are never stored
  expires_at   timestamptz NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  revoked_at   timestamptz
);
CREATE INDEX sessions_user_idx ON sessions (user_id) WHERE revoked_at IS NULL;

CREATE TABLE follows (
  follower_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  followee_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (follower_id, followee_id),
  CHECK (follower_id <> followee_id)
);
CREATE INDEX follows_followee_idx ON follows (followee_id);

CREATE TABLE user_blocks (
  blocker_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  blocked_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (blocker_id, blocked_id),
  CHECK (blocker_id <> blocked_id)
);
