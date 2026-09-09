-- Identity, authentication ceremonies, and sessions.
--
-- Conventions used throughout packages/db/queries:
--   * time windows are passed as timestamptz boundaries computed in Go
--     (`created_at > @since`), never as interval strings. That keeps the
--     predicate sargable and avoids coupling SQL to Go's duration formatting.
--   * every destructive ceremony is a single atomic conditional UPDATE, so
--     "single use" cannot be violated by two concurrent requests.

-- name: CreateUser :one
INSERT INTO users (username, email, password_hash)
VALUES (@username, @email, sqlc.narg('password_hash'))
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1 AND deleted_at IS NULL;

-- name: EmailIsTaken :one
SELECT EXISTS (SELECT 1 FROM users WHERE email = @email) AS taken;

-- name: UsernameIsTaken :one
SELECT EXISTS (SELECT 1 FROM users WHERE username = @username) AS taken;

-- name: RecordLogin :exec
UPDATE users SET last_login_at = now() WHERE id = $1;

-- name: SetEmailVerified :exec
UPDATE users SET email_verified = true WHERE id = $1;

-- name: SoftDeleteUser :exec
UPDATE users SET deleted_at = now() WHERE id = $1;

-- name: UpsertProfile :one
INSERT INTO profiles (user_id, display_name, bio, pronouns, location, is_private)
VALUES (@user_id, @display_name, @bio, sqlc.narg('pronouns'), sqlc.narg('location'), @is_private)
ON CONFLICT (user_id) DO UPDATE SET
  display_name = EXCLUDED.display_name,
  bio          = EXCLUDED.bio,
  pronouns     = EXCLUDED.pronouns,
  location     = EXCLUDED.location,
  is_private   = EXCLUDED.is_private
RETURNING *;

-- name: GetProfileByUserID :one
SELECT * FROM profiles WHERE user_id = $1;

-- name: GetPublicProfileByUsername :one
-- The public projection: never exposes email, reputation internals, or a
-- private account's details beyond what the profile page needs.
SELECT u.id, u.username, u.created_at, u.role,
       p.display_name, p.bio, p.pronouns, p.location, p.avatar_key, p.is_private
  FROM users u
  LEFT JOIN profiles p ON p.user_id = u.id
 WHERE u.username = @username AND u.deleted_at IS NULL;

-- ---- WebAuthn credentials ---------------------------------------------------

-- name: InsertWebAuthnCredential :exec
INSERT INTO webauthn_credentials
  (id, user_id, public_key, attestation_aaguid, sign_count, transports, nickname, flags)
VALUES
  (@id, @user_id, @public_key, sqlc.narg('attestation_aaguid'),
   @sign_count, @transports, @nickname, @flags);

-- name: GetWebAuthnCredential :one
SELECT * FROM webauthn_credentials WHERE id = $1;

-- name: ListWebAuthnCredentialsForUser :many
SELECT * FROM webauthn_credentials WHERE user_id = $1 ORDER BY created_at DESC;

-- name: CredentialIDExists :one
SELECT EXISTS (SELECT 1 FROM webauthn_credentials WHERE id = @id) AS exists;

-- name: UpdateWebAuthnCredentialAfterAssertion :exec
UPDATE webauthn_credentials
   SET sign_count = @sign_count, last_used_at = now()
 WHERE id = @id;

-- name: DeleteWebAuthnCredential :execrows
DELETE FROM webauthn_credentials WHERE id = @id AND user_id = @user_id;

-- name: FindUserByCredentialID :one
-- Login resolves the account from the asserted credential, because a passkey
-- ceremony begins before we know who is signing in.
SELECT u.* FROM users u
  JOIN webauthn_credentials c ON c.user_id = u.id
 WHERE c.id = @credential_id AND u.deleted_at IS NULL;

-- ---- Sessions ---------------------------------------------------------------

-- name: CreateSession :one
INSERT INTO sessions (user_id, token_hash, user_agent, ip_hash, expires_at)
VALUES (@user_id, @token_hash, sqlc.narg('user_agent'), sqlc.narg('ip_hash'), @expires_at)
RETURNING *;

-- name: GetActiveSession :one
-- The single query behind session middleware. Suspension and soft-deletion are
-- resolved here so no handler can forget to check them.
SELECT s.id, s.user_id, s.expires_at, s.created_at,
       u.username, u.email, u.role, u.reputation, u.email_verified,
       u.suspended_until, u.deleted_at AS user_deleted_at
  FROM sessions s
  JOIN users u ON u.id = s.user_id
 WHERE s.token_hash = @token_hash
   AND s.revoked_at IS NULL
   AND s.expires_at > now();

-- name: ExtendSession :execrows
UPDATE sessions SET expires_at = @expires_at
 WHERE id = @id AND revoked_at IS NULL;

-- name: RevokeSessionByToken :execrows
UPDATE sessions SET revoked_at = now()
 WHERE token_hash = @token_hash AND revoked_at IS NULL;

-- name: RevokeSessionsForUser :execrows
UPDATE sessions SET revoked_at = now() WHERE user_id = @user_id AND revoked_at IS NULL;

-- name: RevokeOtherSessions :execrows
UPDATE sessions SET revoked_at = now()
 WHERE user_id = @user_id AND token_hash <> @keep_token_hash AND revoked_at IS NULL;

-- name: ListSessionsForUser :many
SELECT id, user_agent, ip_hash, created_at, expires_at, revoked_at
  FROM sessions
 WHERE user_id = @user_id
 ORDER BY created_at DESC
 LIMIT @lim;

-- name: DeleteStaleSessions :execrows
-- Sessions revoked or expired more than @since are garbage; removing them keeps
-- the lookup index small without ever touching a live session.
DELETE FROM sessions
 WHERE (revoked_at IS NOT NULL AND revoked_at < @since)
    OR (revoked_at IS NULL AND expires_at < @since);

-- ---- Auth ceremonies --------------------------------------------------------

-- name: CreateAuthChallenge :one
INSERT INTO auth_challenges (kind, user_id, email, challenge, session_data, expires_at)
VALUES (@kind, sqlc.narg('user_id'), sqlc.narg('email'), @challenge, @session_data, @expires_at)
RETURNING *;

-- name: ConsumeAuthChallenge :one
-- Single-use by construction: the `consumed_at IS NULL` predicate makes a
-- concurrent replay lose the race and return zero rows.
UPDATE auth_challenges
   SET consumed_at = now()
 WHERE id = @id AND kind = @kind AND consumed_at IS NULL AND expires_at > now()
RETURNING *;

-- name: DeleteExpiredChallenges :execrows
DELETE FROM auth_challenges WHERE expires_at < @now;

-- name: RecordAuthAttempt :exec
INSERT INTO auth_attempts (kind, bucket_key, allowed) VALUES (@kind, @bucket_key, @allowed);

-- name: CountAuthAttemptsSince :one
SELECT count(*) FROM auth_attempts
 WHERE bucket_key = @bucket_key AND kind = @kind AND created_at > @since;

-- ---- Transactional email outbox --------------------------------------------

-- name: QueueEmail :exec
INSERT INTO email_outbox (to_email, template, payload) VALUES (@to_email, @template, @payload);

-- name: FetchPendingEmails :many
SELECT * FROM email_outbox
 WHERE sent_at IS NULL AND attempts < @max_attempts
 ORDER BY id
 LIMIT @lim;

-- name: MarkEmailSent :execrows
UPDATE email_outbox SET sent_at = now(), attempts = attempts + 1 WHERE id = @id;

-- name: MarkEmailFailed :execrows
UPDATE email_outbox SET attempts = attempts + 1, last_error = @last_error WHERE id = @id;
