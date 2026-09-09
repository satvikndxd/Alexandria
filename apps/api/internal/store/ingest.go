package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/alexandria-reads/alexandria/apps/api/internal/db"
)

// Ingestion bookkeeping: idempotent job queue + transactional outbox.
//
// Handlers never fetch remote metadata. They enqueue an intent; the worker
// consumes it from JetStream with at-least-once semantics and deduplicates via
// the job's idempotency key. If NATS is down, the outbox row still exists and
// the relay publishes it when the bus returns.

// Job kinds. They mirror ingest_jobs.kind's CHECK constraint.
const (
	JobOpenLibraryWork    = "openlibrary_work"
	JobOpenLibraryEdition = "openlibrary_edition"
	JobCoverFetch         = "cover_fetch"
	JobGutenbergCatalog   = "gutenberg_catalog"
	JobGutenbergText      = "gutenberg_text"
	JobSearchIndex        = "search_index"
)

// NATS subjects, kept next to the job kinds they carry so the two cannot drift.
const (
	SubjectOpenLibraryWork = "ingest.openlibrary_work"
	SubjectCoverFetch      = "ingest.cover_fetch"
	SubjectGutenbergText   = "ingest.gutenberg_text"
	SubjectSearchIndex     = "ingest.search_index"
)

func SubjectForKind(kind string) string {
	switch kind {
	case JobOpenLibraryWork, JobOpenLibraryEdition:
		return SubjectOpenLibraryWork
	case JobCoverFetch:
		return SubjectCoverFetch
	case JobGutenbergText:
		return SubjectGutenbergText
	case JobSearchIndex:
		return SubjectSearchIndex
	}
	return "ingest." + kind
}

// EnqueueIngest writes the job and its outbox event in one transaction. The
// idempotency key makes a duplicated request a no-op instead of a duplicate
// fetch of a rate-limited upstream.
func (s *Store) EnqueueIngest(ctx context.Context, kind, idemKey string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal ingest payload: %w", err)
	}
	return s.Tx(ctx, func(q *db.Queries) error {
		if _, err := q.EnqueueIngestJob(ctx, db.EnqueueIngestJobParams{
			Kind: kind, IdemKey: idemKey, Payload: body,
		}); err != nil {
			return err
		}
		return q.OutboxAppend(ctx, db.OutboxAppendParams{
			Subject: SubjectForKind(kind), Payload: body,
		})
	})
}

// PublishEvent appends a bare outbox event (no job row) — used for projections
// like the search index that are derived state, not work to be done.
func (s *Store) PublishEvent(ctx context.Context, subject string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return s.Tx(ctx, func(q *db.Queries) error {
		return q.OutboxAppend(ctx, db.OutboxAppendParams{Subject: subject, Payload: body})
	})
}

func (s *Store) OutboxPeek(ctx context.Context, limit int32) ([]db.Outbox, error) {
	return s.q.OutboxPeek(ctx, limit)
}

func (s *Store) OutboxDelete(ctx context.Context, ids []int64) error {
	return s.q.OutboxDelete(ctx, ids)
}

// ClaimNextJob lets a worker take the oldest queued (or lease-expired) job
// atomically. Zero rows means the queue is empty right now.
func (s *Store) ClaimNextJob(ctx context.Context, lease time.Duration) (*db.IngestJob, error) {
	job, err := s.q.ClaimIngestJob(ctx, TS(time.Now().Add(-lease)))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *Store) FinishJob(ctx context.Context, id uuid.UUID, status db.IngestJobStatus, lastErr string) error {
	_, err := s.q.FinishIngestJob(ctx, db.FinishIngestJobParams{
		ID: id, Status: status, LastError: Str(lastErr),
	})
	return err
}

func (s *Store) RequeueJob(ctx context.Context, id uuid.UUID, backoff time.Duration, maxAttempts int32, lastErr string) error {
	_, err := s.q.RequeueIngestJob(ctx, db.RequeueIngestJobParams{
		ID: id, ScheduledAt: TS(time.Now().Add(backoff)), MaxAttempts: maxAttempts,
		LastError: Str(lastErr),
	})
	return err
}

// ---- Gutenberg catalog -------------------------------------------------------------

type GutenbergRecord struct {
	ID       int32
	Title    string
	Authors  []string
	Language string
	Subjects []string
	Formats  map[string]string
	// Metadata is the edition's jsonb payload: license and trademark notices,
	// LCC classes, and the cached text URL. It travels with the record so the
	// reader can always show where a text came from and under what terms.
	Metadata []byte
}

// UpsertGutenbergRecord stores the catalog row and links it to its edition and
// work in one transaction, so a partially-ingested text is never visible.
func (s *Store) UpsertGutenbergRecord(ctx context.Context, rec GutenbergRecord, work WorkUpsert) (*db.GutenbergText, error) {
	var text db.GutenbergText
	err := s.Tx(ctx, func(q *db.Queries) error {
		w, err := s.UpsertWorkMaterialized(ctx, work)
		if err != nil {
			return err
		}
		metadata := rec.Metadata
		if len(metadata) == 0 {
			metadata = MetadataJSON(map[string]any{
				"gutenberg_id": rec.ID,
				"source":       "project_gutenberg",
				"formats":      rec.Formats,
			})
		}
		edition, err := s.UpsertGutenbergEdition(ctx, w.ID, rec.Title, rec.Language, rec.ID, metadata)
		if err != nil {
			return err
		}
		formats, err := json.Marshal(rec.Formats)
		if err != nil {
			return err
		}
		t, err := q.UpsertGutenbergText(ctx, db.UpsertGutenbergTextParams{
			GutenbergID: rec.ID,
			Title:       rec.Title,
			Authors:     rec.Authors,
			Language:    rec.Language,
			Subjects:    rec.Subjects,
			Formats:     formats,
			EditionID:   UUIDPtr(&edition.ID),
		})
		if err != nil {
			return err
		}
		text = t
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &text, nil
}

func (s *Store) GetGutenbergText(ctx context.Context, id int32) (*db.GutenbergText, error) {
	t, err := s.q.GetGutenbergText(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// PruneStaleAuth is housekeeping for the ceremony tables, run by a cron-ish
// loop in cmd/api. Challenges and sessions are short-lived by design.
func (s *Store) PruneStaleAuth(ctx context.Context) error {
	now := TS(time.Now())
	if _, err := s.q.DeleteExpiredChallenges(ctx, now); err != nil {
		return err
	}
	if _, err := s.q.DeleteStaleSessions(ctx, TS(time.Now().Add(-30*24*time.Hour))); err != nil {
		return err
	}
	return nil
}
