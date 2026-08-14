// Package store is the explicit-SQL data layer (pgx/v5, no ORM).
//
// The SQL here mirrors packages/db/queries/*.sql — the source of truth for
// contributors regenerating via sqlc (`cd packages/db && sqlc generate`).
// Methods are kept hand-tuned and reviewed: every query is visible, every
// index it relies on is documented in the migrations.
package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Connect opens a pool with conservative, production-shaped settings.
func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 16
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second
	return pgxpool.NewWithConfig(ctx, cfg)
}

// ---- Bibliography -----------------------------------------------------------

type Work struct {
	ID             uuid.UUID       `json:"id"`
	Slug           string          `json:"slug"`
	Title          string          `json:"title"`
	Subtitle       *string         `json:"subtitle"`
	FirstPublished *int32          `json:"first_published"`
	Description    string          `json:"description"`
	IsPublicDomain bool            `json:"is_public_domain"`
	RatingSum      int64           `json:"-"`
	RatingCount    int64           `json:"rating_count"`
	Authors        json.RawMessage `json:"authors"`
}

// AverageRating returns half-star average (e.g. 4.5) or 0 when unrated.
func (w Work) AverageRating() float64 {
	if w.RatingCount == 0 {
		return 0
	}
	return float64(w.RatingSum) / float64(w.RatingCount) / 2.0
}

func (s *Store) GetWorkBySlug(ctx context.Context, slug string) (*Work, error) {
	const q = `
SELECT w.id, w.slug, w.title, w.subtitle, w.first_published, w.description,
       w.is_public_domain, w.rating_sum, w.rating_count,
       (SELECT coalesce(json_agg(json_build_object('id', a.id, 'name', a.name, 'role', wa.role) ORDER BY wa.position), '[]')
          FROM work_authors wa JOIN authors a ON a.id = wa.author_id
         WHERE wa.work_id = w.id) AS authors
  FROM works w
 WHERE w.slug = $1 AND w.deleted_at IS NULL`
	var w Work
	err := s.pool.QueryRow(ctx, q, slug).Scan(
		&w.ID, &w.Slug, &w.Title, &w.Subtitle, &w.FirstPublished, &w.Description,
		&w.IsPublicDomain, &w.RatingSum, &w.RatingCount, &w.Authors)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

type Edition struct {
	ID          uuid.UUID       `json:"id"`
	WorkID      uuid.UUID       `json:"work_id"`
	Title       string          `json:"title"`
	ISBN13      *string         `json:"isbn13"`
	Format      string          `json:"format"`
	Publisher   *string         `json:"publisher"`
	Language    string          `json:"language"`
	PageCount   *int32          `json:"page_count"`
	GutenbergID *int32          `json:"gutenberg_id"`
	Metadata    json.RawMessage `json:"metadata"`
}

func (s *Store) ListEditionsForWork(ctx context.Context, workID uuid.UUID) ([]Edition, error) {
	const q = `
SELECT id, work_id, title, isbn13, format, publisher, language, page_count, gutenberg_id, metadata
  FROM editions
 WHERE work_id = $1 AND deleted_at IS NULL
 ORDER BY published_on DESC NULLS LAST`
	rows, err := s.pool.Query(ctx, q, workID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Edition
	for rows.Next() {
		var e Edition
		if err := rows.Scan(&e.ID, &e.WorkID, &e.Title, &e.ISBN13, &e.Format,
			&e.Publisher, &e.Language, &e.PageCount, &e.GutenbergID, &e.Metadata); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ---- Reviews ----------------------------------------------------------------

type Review struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	Username    string    `json:"username"`
	WorkID      uuid.UUID `json:"work_id"`
	Rating      int16     `json:"rating"` // half-stars 1..10
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	HasSpoilers bool      `json:"has_spoilers"`
	LikeCount   int32     `json:"like_count"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateReviewParams struct {
	UserID      uuid.UUID
	WorkID      uuid.UUID
	EditionID   *uuid.UUID
	Rating      int16
	Title       string
	Body        string
	HasSpoilers bool
	PromptWhy   string
}

// CreateReview inserts the review and its contribution-ledger row atomically,
// so posting caps can never drift from actual content.
func (s *Store) CreateReview(ctx context.Context, p CreateReviewParams) (*Review, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	const q = `
INSERT INTO reviews (user_id, work_id, edition_id, rating, title, body, has_spoilers, prompt_why)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, created_at`
	var r Review
	r.UserID, r.WorkID, r.Rating, r.Title, r.Body, r.HasSpoilers =
		p.UserID, p.WorkID, p.Rating, p.Title, p.Body, p.HasSpoilers
	if err := tx.QueryRow(ctx, q, p.UserID, p.WorkID, p.EditionID, p.Rating,
		p.Title, p.Body, p.HasSpoilers, p.PromptWhy).Scan(&r.ID, &r.CreatedAt); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO contribution_ledger (user_id, kind) VALUES ($1, 'review')`, p.UserID); err != nil {
		return nil, err
	}
	return &r, tx.Commit(ctx)
}

func (s *Store) ListReviewsForWork(ctx context.Context, workID uuid.UUID, limit, offset int32) ([]Review, error) {
	const q = `
SELECT r.id, r.user_id, u.username, r.work_id, r.rating, r.title, r.body,
       r.has_spoilers, r.like_count, r.created_at
  FROM reviews r JOIN users u ON u.id = r.user_id
 WHERE r.work_id = $1 AND r.deleted_at IS NULL
 ORDER BY r.created_at DESC
 LIMIT $2 OFFSET $3`
	rows, err := s.pool.Query(ctx, q, workID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Review
	for rows.Next() {
		var r Review
		if err := rows.Scan(&r.ID, &r.UserID, &r.Username, &r.WorkID, &r.Rating,
			&r.Title, &r.Body, &r.HasSpoilers, &r.LikeCount, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CountRecentReviews counts a user's reviews in the trailing window (posting cap).
func (s *Store) CountRecentReviews(ctx context.Context, userID uuid.UUID, window time.Duration) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM contribution_ledger
		  WHERE user_id = $1 AND kind = 'review' AND created_at > now() - $2::interval`,
		userID, window.String()).Scan(&n)
	return n, err
}

func (s *Store) GetUserReputation(ctx context.Context, userID uuid.UUID) (int, error) {
	var rep int
	err := s.pool.QueryRow(ctx, `SELECT reputation FROM users WHERE id = $1 AND deleted_at IS NULL`, userID).Scan(&rep)
	return rep, err
}

// ---- Outbox (event-driven ingestion) ---------------------------------------

type OutboxRow struct {
	ID      int64
	Subject string
	Payload json.RawMessage
}

// EnqueueIngest writes an ingest job and its event to the outbox in one tx.
func (s *Store) EnqueueIngest(ctx context.Context, kind, idemKey string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
INSERT INTO ingest_jobs (kind, idem_key, payload) VALUES ($1, $2, $3)
ON CONFLICT (idem_key) DO NOTHING`, kind, idemKey, body); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO outbox (subject, payload) VALUES ($1, $2)`,
		"ingest."+kind, body); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) OutboxPeek(ctx context.Context, limit int32) ([]OutboxRow, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, subject, payload FROM outbox ORDER BY id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OutboxRow
	for rows.Next() {
		var r OutboxRow
		if err := rows.Scan(&r.ID, &r.Subject, &r.Payload); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) OutboxDelete(ctx context.Context, ids []int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM outbox WHERE id = ANY($1)`, ids)
	return err
}
