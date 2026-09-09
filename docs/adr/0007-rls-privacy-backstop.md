# ADR 0007 — Row-Level Security as the privacy backstop

**Status:** Accepted · 2026-09-09

## Context
Alexandria's promise is that reading history is nobody's business but the
reader's. Application-level `WHERE user_id = ?` is one buggy handler away from
leaking a private shelf, and "we filter in the query" is unfalsifiable in
review.

## Decision
- Private-by-nature tables (`shelves`, `shelf_items`, `reading_sessions`,
  `annotations`, `notifications`) carry RLS policies (migration 0011):
  owner-or-service for writes, plus an explicit public-read policy for shelves
  the owner did **not** mark private.
- The request path sets `app.user_id` with `SET LOCAL` inside a transaction
  (`store.TxUser` / `ReadUser`); workers set `app.service = 'on'`. `SET LOCAL`
  dies with the transaction, so a pooled connection can never carry the
  previous tenant's identity.
- `FORCE ROW LEVEL SECURITY` is set so even the table owner obeys policies.
- **The request path must connect as a non-superuser role.** Superusers bypass
  RLS by definition — `FORCE` included — which the integration suite
  demonstrates deliberately by testing through `alexandria_app`.
- Queries that read a reader's own progress for authorization (club spoiler
  gates) run inside the scoped transaction; outside it the progress is simply
  invisible, which fails closed.

## Consequences
- A leaked read query cannot cross readers; the database refuses the row
  before the query sees it.
- Every RLS-covered access pays one `set_config` per transaction. Accepted.
- Local dev connecting as the owner role is still protected (FORCE), but CI
  and production use the least-privilege role; `.env.example` says so.
- Public content (reviews, published notes, club messages) is deliberately
  **not** RLS-covered: policies there would cost per-row and hide nothing.
