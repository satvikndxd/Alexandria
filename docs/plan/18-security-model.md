# 18 — Security Model

**Status:** implemented unless marked ⏳ · threat-modelled, not checklist-driven

## Identity
- Passkeys (WebAuthn, user-verification required) as primary factor;
  ceremonies stateless across nodes (`auth_challenges`, single-use by atomic
  conditional update, self-expiring). Cloned-authenticator detection: a
  non-increasing sign count fails closed and flags the credential.
- Magic links: single-use, 15-minute TTL, HMAC-stored secret (the raw secret
  lives only in the email), throttled per hashed mailbox; the endpoint oracles
  nothing (identical 202, no mail for unknown addresses).
- No passwords in the primary flow; `users.password_hash` exists only as an
  operator-set Argon2id break-glass digest.
- Sessions: 256-bit opaque tokens, SHA-256 at rest, `HttpOnly` + `SameSite=Lax`
  + `Secure` behind TLS, sliding renewal capped by absolute TTL; logout
  revokes; suspension/deletion checked at session resolution, not per-handler.

## CSRF & request integrity
Double-submit token HMAC-bound to the session token, verified on every
mutating method for authenticated sessions; integration-tested (403 without).

## Authorization
- RLS as the privacy backstop ([ADR 0007](../adr/0007-rls-privacy-backstop.md));
  request path runs as least-privilege `alexandria_app` because superusers
  bypass RLS by definition.
- Spoiler gating is authorization: bytes never leave the server below a
  channel threshold.
- Moderator surfaces require role; every action requires prose.

## Injection & input classes
- SQL: sqlc-parameterized everywhere; no string-built SQL in Go (banned by
  review convention, CI-drift-checked).
- XSS: server-rendered React escapes by default; review bodies are plain text
  by design (no HTML in reviews, ever) so there is no sanitizer to bypass;
  CSP `default-src 'self'`, `object-src 'none'`.
- SSRF: upstream hosts are allowlisted constants (openlibrary.org,
  gutenberg.org); no user-supplied URL is ever fetched server-side.
- Uploads ⏳ (avatars, club banners): type-sniffed, re-encoded, stored in
  MinIO under random keys, served from a cookie-less origin.
- Deserialization: JSON only, `MaxBytesReader` caps, `DisallowUnknownFields`
  on client payloads so typos fail loudly.

## Secrets & ops
- 12-factor env only; DSNs redacted in logs (`config.redactDSN`); peppers and
  SMTP creds never logged; session pepper per environment (documented
  generation command in `.env.example`).
- IP addresses: salted-hash at rest (`auth_attempts.bucket_key`,
  `sessions.ip_hash`); raw IPs never persisted.
- Backups: pg_dump + WAL archive ⏳ production runbook ([22](22-infrastructure.md));
  restore drills quarterly.

## Abuse resistance
Token-bucket rate limit per IP; auth throttling per hashed bucket; outbox
bounded; ingest pacing; keyset cursors so deep pagination cannot be used as
DoS; `middleware.Timeout(30s)` on every request.
