-- Alexandria · Migration 0009
-- Event-driven ingestion bookkeeping. HTTP handlers publish intents to NATS
-- JetStream; workers record job state here (idempotency + observability).
-- The outbox table guarantees at-least-once publication even if NATS is
-- briefly unavailable during a transaction.

CREATE TYPE ingest_job_status AS ENUM ('queued', 'running', 'succeeded', 'failed', 'dead');

CREATE TABLE ingest_jobs (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  kind         text NOT NULL CHECK (kind IN (
                 'openlibrary_work', 'openlibrary_edition', 'cover_fetch',
                 'gutenberg_catalog', 'gutenberg_text', 'search_index')),
  -- Natural idempotency key, e.g. "openlibrary_work:OL66554W"
  idem_key     text NOT NULL UNIQUE,
  payload      jsonb NOT NULL DEFAULT '{}',
  status       ingest_job_status NOT NULL DEFAULT 'queued',
  attempts     integer NOT NULL DEFAULT 0,
  last_error   text,
  scheduled_at timestamptz NOT NULL DEFAULT now(),
  started_at   timestamptz,
  finished_at  timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ingest_jobs_status_idx ON ingest_jobs (status, scheduled_at);

-- Transactional outbox: rows are written in the same tx as domain changes,
-- then relayed to JetStream by the outbox publisher and deleted on ack.
CREATE TABLE outbox (
  id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  subject    text NOT NULL,             -- NATS subject, e.g. ingest.openlibrary.work
  payload    jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Gutenberg catalog snapshot (parsed from the official offline feeds).
CREATE TABLE gutenberg_texts (
  gutenberg_id integer PRIMARY KEY,
  title        text NOT NULL,
  authors      text[] NOT NULL DEFAULT '{}',
  language     text NOT NULL DEFAULT 'en',
  subjects     text[] NOT NULL DEFAULT '{}',
  formats      jsonb NOT NULL DEFAULT '{}',   -- format -> canonical file key in MinIO
  edition_id   uuid REFERENCES editions(id) ON DELETE SET NULL,
  ingested_at  timestamptz NOT NULL DEFAULT now()
);

-- Affiliate links (Book -> Purchase Options -> Amazon; never inline ads).
CREATE TABLE affiliate_links (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  edition_id uuid NOT NULL REFERENCES editions(id) ON DELETE CASCADE,
  vendor     text NOT NULL CHECK (vendor IN ('amazon', 'bookshop_org')),
  url        text NOT NULL,
  region     text NOT NULL DEFAULT 'us',
  disclosure text NOT NULL DEFAULT 'As an Amazon Associate, Alexandria earns from qualifying purchases.',
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (edition_id, vendor, region)
);
