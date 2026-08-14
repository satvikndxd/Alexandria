# ADR 0004 — Event-driven ingestion via NATS JetStream + transactional outbox

**Status:** Accepted · 2026-08-14

## Context
Fetching Open Library metadata or Gutenberg texts inside an HTTP handler
couples our latency to a third party's, and hammering those services violates
their usage guidelines. Open Library asks for identifying User-Agents and
modest request rates; Project Gutenberg forbids scraping and provides official
offline catalogs/feeds instead.

## Decision
1. HTTP handlers write an `ingest_jobs` row (idempotency key) **and** an
   `outbox` row in the same transaction, then return immediately.
2. An outbox relay drains rows into JetStream (`ingest.>`, work-queue
   retention). NATS downtime delays events; it never loses them.
3. Workers consume durable pull subscriptions with explicit acks and a
   5-step backoff (5s → 10m). Handlers are idempotent (upsert on external
   IDs), so at-least-once delivery is safe.
4. External clients are paced (1 req/s ticker for Open Library) and identify
   themselves. Gutenberg ingestion parses the official `feeds/` catalogs and
   downloads texts to MinIO once, never hotlinking or re-crawling.

## Consequences
- API latency is independent of third-party health.
- Backfills are a stream replay, not a script marathon.
- One more moving part (NATS) in compose — acceptable; it is a single
  container with a data volume.
