package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/alexandria-reads/alexandria/apps/api/internal/db"
)

// ReviewInput is the validated review a handler wants to persist. Validation
// itself lives in internal/domain; the store trusts it and enforces only
// invariants the database owns (uniqueness, rating range, lengths).
type ReviewInput struct {
	UserID      uuid.UUID
	WorkID      uuid.UUID
	EditionID   *uuid.UUID
	Rating      int32 // half-stars, 1..10
	Title       string
	Body        string
	HasSpoilers bool
	PromptWhy   string
}

// CreateReview writes the review and its contribution-ledger row in one
// transaction. The ledger row is what daily posting caps count, so caps can
// never drift from the content that actually exists.
func (s *Store) CreateReview(ctx context.Context, in ReviewInput) (*db.Review, error) {
	var review db.Review
	err := s.Tx(ctx, func(q *db.Queries) error {
		r, err := q.CreateReview(ctx, db.CreateReviewParams{
			UserID: in.UserID, WorkID: in.WorkID, EditionID: UUIDPtr(in.EditionID),
			Rating: in.Rating, Title: in.Title, Body: in.Body,
			HasSpoilers: in.HasSpoilers, PromptWhy: in.PromptWhy,
		})
		if err != nil {
			return err
		}
		review = r
		return q.RecordContribution(ctx, db.RecordContributionParams{
			UserID: in.UserID, Kind: "review",
		})
	})
	if err != nil {
		return nil, err
	}
	return &review, nil
}

// UpdateReview edits in place. The unique(user_id, work_id) constraint means a
// reader has exactly one review per work; rereading changes the review rather
// than duplicating it, which is what keeps per-work rating aggregates honest.
func (s *Store) UpdateReview(ctx context.Context, reviewID, userID uuid.UUID, in ReviewInput) (*db.Review, error) {
	r, err := s.q.UpdateReview(ctx, db.UpdateReviewParams{
		ID: reviewID, UserID: userID,
		Rating: in.Rating, Title: in.Title, Body: in.Body,
		HasSpoilers: in.HasSpoilers, PromptWhy: in.PromptWhy,
		EditionID: UUIDPtr(in.EditionID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) GetReview(ctx context.Context, id uuid.UUID) (*db.GetReviewByIDRow, error) {
	r, err := s.q.GetReviewByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) ListReviewsForWork(ctx context.Context, workID uuid.UUID, sort string, limit, offset int32) ([]db.ListReviewsForWorkRow, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return s.q.ListReviewsForWork(ctx, db.ListReviewsForWorkParams{
		WorkID: workID, Sort: sort, Lim: limit, Off: offset,
	})
}

func (s *Store) ListReviewsByUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.ListReviewsByUserRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 24
	}
	return s.q.ListReviewsByUser(ctx, db.ListReviewsByUserParams{UserID: userID, Lim: limit, Off: offset})
}

func (s *Store) DeleteReview(ctx context.Context, reviewID, userID uuid.UUID) error {
	return s.Tx(ctx, func(q *db.Queries) error {
		n, err := q.SoftDeleteReview(ctx, db.SoftDeleteReviewParams{ID: reviewID, UserID: userID})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// DailyReviewCount is the anti-slop cap input: reviews posted by this account
// in the trailing 24h, counted from the ledger rather than the reviews table so
// deleted spam still counts against its author.
func (s *Store) DailyReviewCount(ctx context.Context, userID uuid.UUID) (int, error) {
	n, err := s.q.CountContributionsSince(ctx, db.CountContributionsSinceParams{
		UserID: userID, Kind: "review", Since: TS(time.Now().Add(-24 * time.Hour)),
	})
	return int(n), err
}

// ---- Likes -----------------------------------------------------------------------
// Likes exist to surface good criticism, not to rank people. They adjust the
// review's own counter and never a user's reputation or feed weight.

func (s *Store) SetReviewLike(ctx context.Context, reviewID, userID uuid.UUID, liked bool) error {
	return s.Tx(ctx, func(q *db.Queries) error {
		if liked {
			if _, err := q.LikeReview(ctx, db.LikeReviewParams{ReviewID: reviewID, UserID: userID}); err != nil {
				return err
			}
		} else if _, err := q.UnlikeReview(ctx, db.UnlikeReviewParams{ReviewID: reviewID, UserID: userID}); err != nil {
			return err
		}
		return q.SyncReviewLikeCount(ctx, reviewID)
	})
}

func (s *Store) HasLikedReview(ctx context.Context, reviewID, userID uuid.UUID) (bool, error) {
	return s.q.HasUserLikedReview(ctx, db.HasUserLikedReviewParams{ReviewID: reviewID, UserID: userID})
}

// ---- Comments ----------------------------------------------------------------------

func (s *Store) CreateReviewComment(ctx context.Context, reviewID, userID uuid.UUID, body string) (*db.ReviewComment, error) {
	var out db.ReviewComment
	err := s.Tx(ctx, func(q *db.Queries) error {
		c, err := q.CreateReviewComment(ctx, db.CreateReviewCommentParams{
			ReviewID: reviewID, UserID: userID, Body: body,
		})
		if err != nil {
			return err
		}
		out = c
		return q.RecordContribution(ctx, db.RecordContributionParams{UserID: userID, Kind: "comment"})
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Store) ListReviewComments(ctx context.Context, reviewID uuid.UUID, limit, offset int32) ([]db.ListCommentsForReviewRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.q.ListCommentsForReview(ctx, db.ListCommentsForReviewParams{
		ReviewID: reviewID, Lim: limit, Off: offset,
	})
}

func (s *Store) DeleteReviewComment(ctx context.Context, commentID, userID uuid.UUID) error {
	n, err := s.q.SoftDeleteReviewComment(ctx, db.SoftDeleteReviewCommentParams{ID: commentID, UserID: userID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
