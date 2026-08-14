-- name: EnqueueIngestJob :one
INSERT INTO ingest_jobs (kind, idem_key, payload)
VALUES ($1, $2, $3)
ON CONFLICT (idem_key) DO UPDATE SET scheduled_at = ingest_jobs.scheduled_at  -- no-op, returns row
RETURNING *;

-- name: MarkIngestJobRunning :exec
UPDATE ingest_jobs SET status = 'running', attempts = attempts + 1, started_at = now()
 WHERE id = $1;

-- name: MarkIngestJobDone :exec
UPDATE ingest_jobs SET status = $2, last_error = $3, finished_at = now() WHERE id = $1;

-- name: OutboxAppend :exec
INSERT INTO outbox (subject, payload) VALUES ($1, $2);

-- name: OutboxPeek :many
SELECT * FROM outbox ORDER BY id LIMIT $1;

-- name: OutboxDelete :exec
DELETE FROM outbox WHERE id = ANY($1::bigint[]);
