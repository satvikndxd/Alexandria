# ADR 0001 — Modular monolith over microservices

**Status:** Accepted · 2026-08-14

## Context
Alexandria is open source and must be trivially self-hostable. Contributors
should run the whole platform with one `docker compose up`, not orchestrate
twenty services.

## Decision
A single Go binary (`apps/api`) contains every module (identity, bibliography,
library, reviews, scholars, clubs, moderation) behind internal package
boundaries. A second binary (`cmd/worker`) runs the ingestion consumers from
the same module tree. Modules communicate in-process; asynchronous work flows
through NATS JetStream via a transactional outbox.

## Consequences
- One deployable, one schema, one tracing context → radically simpler ops.
- Module boundaries are enforced by Go package visibility (`internal/`), so a
  future extraction (e.g. realtime chat) is a refactor, not a rewrite.
- Scaling is horizontal (N replicas of api, M replicas of worker) until a
  measured bottleneck justifies extraction.
