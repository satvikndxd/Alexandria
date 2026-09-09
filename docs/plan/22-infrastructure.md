# 22 — Infrastructure & Deployment

**Status:** compose + Dockerfiles + CI implemented; K3s production runbook ⏳

## Local / single-node (the supported self-host path)
`infrastructure/docker-compose.yml`: Postgres 16 (migrations on first boot),
NATS JetStream, Meilisearch, MinIO. Add the API/worker containers or run them
on the host — both are documented. One VPS (Hetzner/DigitalOcean, 4 GB) runs
the whole platform at hobby scale.

## Production shape (K3s)
- API: ≥2 replicas behind the edge; stateless (sessions in Postgres,
  ceremonies in Postgres) so HPA is trivial.
- Worker: ≥1 replica per consumer kind; JetStream work-queue semantics make
  replicas safe.
- Postgres: primary + streaming replica; WAL archive to object storage;
  restore drills quarterly.
- Meilisearch: single node + `cmd/reindex` as the recovery path (index is
  derived state; never back up what you can rebuild).
- MinIO: erasure-coded pool or managed S3; lifecycle rules expire
  `fair_use_thumbnail` caches per `cache_expires_at`.
- coturn + LiveKit SFU: Phase 4, bandwidth modelled in [29](29-cost-model.md).

## Edge & the BFF rule
Next.js sits at the edge and owns `/api/v1/*` rewrites to the monolith, so
cookies stay first-party. **Rewrites are baked at build time**: build with
`ALEXANDRIA_API_URL` set. TLS terminates at the edge; `SESSION_SECURE_COOKIE=true`
and `TRUST_PROXY=true` only there.

## Observability (self-hosted only)
- Logs: slog JSON → Loki. Metrics: Prometheus (pgx pool, outbox depth, ingest
  job states, friction refusals by code). Traces: OpenTelemetry ⏳.
- Uptime: external probe of `/healthz` (which reports search degradation
  honestly). Error tracking: self-hosted Sentry ⏳ or log-based alerts.
- The dashboard we care about: outbox depth, ingest dead-letters, RLS
  denials (should be zero outside tests), friction refusal mix.

## Environments
dev (compose) → staging (K3s small, real data anonymized or seeded) →
production. Migrations run by `cmd/migrate` as a deploy gate
(`status` exits non-zero when pending).
