-- Project Gutenberg ingestion and public-domain text access.
--
-- Compliance shape, mirrored in internal/ingest/gutenberg.go:
--   * we consume Gutenberg's OFFICIAL offline catalog feeds, never the website
--   * each text is downloaded once and cached to our own object storage; we do
--     not hotlink or proxy gutenberg.org at read time
--   * the Gutenberg trademark notice and license header are preserved in
--     metadata, and the reading text is stripped of boilerplate separately
--     (stored apart from the notice, so the notice is never lost)

-- name: UpsertGutenbergText :one
INSERT INTO gutenberg_texts (gutenberg_id, title, authors, language, subjects, formats, edition_id)
VALUES (@gutenberg_id, @title, @authors, @language, @subjects, @formats,
        sqlc.narg('edition_id'))
ON CONFLICT (gutenberg_id) DO UPDATE SET
  title      = EXCLUDED.title,
  authors    = EXCLUDED.authors,
  language   = EXCLUDED.language,
  subjects   = EXCLUDED.subjects,
  formats    = gutenberg_texts.formats || EXCLUDED.formats,
  edition_id = COALESCE(EXCLUDED.edition_id, gutenberg_texts.edition_id),
  ingested_at = now()
RETURNING *;

-- name: GetGutenbergText :one
SELECT * FROM gutenberg_texts WHERE gutenberg_id = @gutenberg_id;

-- name: ListGutenbergTexts :many
SELECT g.*, w.slug AS work_slug, w.id AS work_id
  FROM gutenberg_texts g
  LEFT JOIN editions e ON e.gutenberg_id = g.gutenberg_id AND e.deleted_at IS NULL
  LEFT JOIN works w ON w.id = e.work_id AND w.deleted_at IS NULL
 WHERE (@language::text IS NULL OR g.language = @language)
   AND (sqlc.narg('title_prefix')::text IS NULL OR g.title ILIKE sqlc.narg('title_prefix') || '%')
   AND (@after_id::integer IS NULL OR g.gutenberg_id > @after_id)
 ORDER BY g.gutenberg_id
 LIMIT @lim;

-- name: CountGutenbergTexts :one
SELECT count(*) FROM gutenberg_texts;

-- name: LinkGutenbergEdition :execrows
UPDATE gutenberg_texts SET edition_id = @edition_id WHERE gutenberg_id = @gutenberg_id;

-- ---- Ingestion job bookkeeping ---------------------------------------------

-- name: EnqueueIngestJob :one
INSERT INTO ingest_jobs (kind, idem_key, payload)
VALUES (@kind, @idem_key, @payload)
ON CONFLICT (idem_key) DO UPDATE SET idem_key = ingest_jobs.idem_key  -- no-op; returns the existing row
RETURNING *;

-- name: ClaimIngestJob :one
-- Atomic claim: two workers cannot take the same job, and a job stuck in
-- `running` past its lease is reclaimable.
UPDATE ingest_jobs
   SET status = 'running', attempts = attempts + 1, started_at = now()
 WHERE id = (
   SELECT j.id FROM ingest_jobs j
    WHERE j.status = 'queued'
       OR (j.status = 'running' AND j.started_at < @lease_expired_before)
    ORDER BY j.scheduled_at
    FOR UPDATE SKIP LOCKED
    LIMIT 1)
RETURNING *;

-- name: FinishIngestJob :execrows
UPDATE ingest_jobs SET status = @status, last_error = sqlc.narg('last_error'), finished_at = now()
 WHERE id = @id;

-- name: RequeueIngestJob :execrows
-- The retry-at timestamp is computed by the caller so the backoff policy lives
-- in one testable place (internal/ingest) instead of in SQL arithmetic.
UPDATE ingest_jobs
   SET status = 'queued',
       scheduled_at = @scheduled_at,
       last_error = sqlc.narg('last_error')
 WHERE id = @id AND attempts < @max_attempts;

-- name: ListIngestJobsByStatus :many
SELECT * FROM ingest_jobs WHERE status = ANY(@statuses::ingest_job_status[])
 ORDER BY scheduled_at LIMIT @lim;

-- name: IngestJobStats :one
SELECT status, count(*)::bigint AS n FROM ingest_jobs GROUP BY status ORDER BY status;

-- ---- Transactional outbox ---------------------------------------------------

-- name: OutboxAppend :exec
INSERT INTO outbox (subject, payload) VALUES (@subject, @payload);

-- name: OutboxPeek :many
SELECT * FROM outbox ORDER BY id LIMIT @lim;

-- name: OutboxDelete :exec
DELETE FROM outbox WHERE id = ANY(@ids::bigint[]);

-- name: OutboxDepth :one
SELECT count(*) FROM outbox;
