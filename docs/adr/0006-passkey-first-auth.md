# ADR 0006 — Passkey-first identity, magic links as fallback, no passwords

**Status:** Accepted · 2026-09-09

## Context
A reading platform holds nothing worth a password breach: no payments, no
government IDs. Passwords would still be our largest credential-stuffing and
reuse-breach surface, and "add 2FA later" never arrives. Zitadel (OIDC) was
evaluated and rejected for MVP: it adds a stateful identity service to every
contributor's `docker compose up`, and outsources the one thing a social
platform must own — its account model.

## Decision
- **Primary factor: WebAuthn passkeys** via `go-webauthn`. Registration and
  login are ceremonies; user verification (biometric/PIN) is required, so a
  passkey is a real factor rather than a possession check.
- **Ceremony state lives in Postgres** (`auth_challenges`), not process
  memory: any API node can finish a ceremony another node began, and every
  ceremony is single-use by atomic conditional update and expires on its own.
- **Fallback factor: email magic links**, single-use, 15-minute TTL, delivered
  through the transactional `email_outbox` so a slow SMTP relay can never stall
  an auth request. The endpoint returns an identical 202 for unknown addresses
  and queues nothing: login must not oracle mailbox existence.
- **Sessions are opaque 256-bit tokens**; only their SHA-256 is stored. Cookie
  is `HttpOnly` + `SameSite=Lax`, sliding renewal capped by an absolute TTL.
- **No password column in the primary flow.** `users.password_hash` remains
  solely as an operator-set break-glass digest (Argon2id when used).
- **CSRF:** double-submit token HMAC-bound to the session token, checked on
  every state-changing method for authenticated sessions. Lax cookies already
  block cross-site POSTs; this covers the residual cases.
- Auth is throttled structurally: hashed buckets (email/IP) in
  `auth_attempts`, never raw values.

## Consequences
- Phishing-resistant login with zero password storage, reset flows, or breach
  liability.
- Clients must implement the WebAuthn ceremony (web: `@simplewebauthn/browser`;
  Flutter: a platform channel — tracked for Phase 2.1). Until then every
  account can sign in by magic link.
- Sign-in cannot confirm whether an email exists. Support tooling must look
  accounts up by username (public by design) instead.
