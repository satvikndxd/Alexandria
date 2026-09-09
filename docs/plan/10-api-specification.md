# 10 — API Specification

**Status:** implemented · routes in `apps/api/internal/httpapi/server.go`

REST + JSON (GraphQL was evaluated and rejected for MVP: the monolith's query
set is small, typed by sqlc, and REST's cacheability suits the BFF; a GraphQL
layer may return when Flutter's field-level needs prove it).

## Conventions
- Prefix `/v1`; errors are always `{"error":{"code","message"}}` with honest
  codes (`invalid_review`, `daily_limit`, `spoiler_gated`, `csrf`, …).
- Auth: cookie session (`HttpOnly`, `SameSite=Lax`) + `X-CSRF-Token`
  double-submit bound to the session token; required on all mutating methods.
- Pagination: `limit`/`offset` with hard caps for bounded lists; keyset cursors
  (`before`, `after`) for unbounded streams.
- Rate limit: token bucket 120 req/min, burst 30, per client IP; auth
  ceremonies throttled per hashed bucket.
- Spoilers: withheld bytes by default; `?reveal_spoilers=true` opts in.
- Degradation is announced: search responds `source: postgres_fallback`.

## Route table (abridged; server.go is authoritative)
```
POST /v1/auth/register                      POST /v1/auth/passkey/finish-registration
POST /v1/auth/passkey/begin-login           POST /v1/auth/passkey/finish-login
POST /v1/auth/magic-link                    POST /v1/auth/magic-link/verify
POST /v1/auth/logout                        GET  /v1/me            PATCH /v1/me
GET  /v1/works[?subject&language&public_domain&sort]      GET /v1/works/{slug}
GET  /v1/works/{slug}/editions|reviews|notes|related      POST /v1/works/{slug}/reviews
GET/PATCH/DELETE /v1/reviews/{id}           POST /v1/reviews/{id}/like|comments
GET  /v1/me/shelves|library|stats|currently-reading|notifications|annotations
POST /v1/me/library|progress|progress/finish|progress/dnf|shelves|annotations
GET  /v1/feed /v1/community-feed            GET/POST/DELETE /v1/users/{username}/follow|block
GET  /v1/clubs /v1/clubs/{slug} /v1/clubs/{slug}/channels
GET  /v1/channels/{id}/messages             POST /v1/channels/{id}/messages
GET  /v1/search?q=&type=&subjects=&language=&public_domain=
POST /v1/reports                            GET  /v1/moderation/reports
POST /v1/moderation/reports/{id}/action     GET  /healthz
```

## Example: create review
```http
POST /v1/works/meditations/reviews
X-CSRF-Token: <bound to session>
{"rating":4.5,"title":"A stoic companion","body":"…≥150 chars…",
 "prompt_why":"It changed my mornings.","has_spoilers":false}
→ 201 {"review":{…}}            → 422 invalid_review (friction)
→ 409 already_reviewed          → 429 daily_limit
```

## Idempotency & retries
Ingest jobs carry `idem_key` (e.g. `openlibrary_work:OL66554W`); redelivery
converges. Client mutations are not idempotent by design (a review is a human
act) except shelf moves, which are put-semantics.
