package store

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/alexandria-reads/alexandria/apps/api/internal/db"
)

func unreadPtr(v bool) *bool { return &v }

// ---- Social graph ------------------------------------------------------------------
// Follows are the whole ranking signal Alexandria has: the feed is the
// chronological union of what followed accounts did. There is no score column
// anywhere in this file, by design.

func (s *Store) Follow(ctx context.Context, followerID, followeeID uuid.UUID) error {
	if followerID == followeeID {
		return ErrSelfAction
	}
	blocked, err := s.q.IsBlocked(ctx, db.IsBlockedParams{A: followerID, B: followeeID})
	if err != nil {
		return err
	}
	if blocked {
		return ErrBlocked
	}
	_, err = s.q.FollowUser(ctx, db.FollowUserParams{FollowerID: followerID, FolloweeID: followeeID})
	return err
}

func (s *Store) Unfollow(ctx context.Context, followerID, followeeID uuid.UUID) error {
	_, err := s.q.UnfollowUser(ctx, db.UnfollowUserParams{FollowerID: followerID, FolloweeID: followeeID})
	return err
}

func (s *Store) Block(ctx context.Context, blockerID, blockedID uuid.UUID) error {
	if blockerID == blockedID {
		return ErrSelfAction
	}
	return s.Tx(ctx, func(q *db.Queries) error {
		// Blocking also dissolves the follow edge in both directions: a block
		// is not a request to keep seeing someone in your feed.
		if _, err := q.UnfollowUser(ctx, db.UnfollowUserParams{FollowerID: blockerID, FolloweeID: blockedID}); err != nil {
			return err
		}
		if _, err := q.UnfollowUser(ctx, db.UnfollowUserParams{FollowerID: blockedID, FolloweeID: blockerID}); err != nil {
			return err
		}
		_, err := q.BlockUser(ctx, db.BlockUserParams{BlockerID: blockerID, BlockedID: blockedID})
		return err
	})
}

func (s *Store) Unblock(ctx context.Context, blockerID, blockedID uuid.UUID) error {
	_, err := s.q.UnblockUser(ctx, db.UnblockUserParams{BlockerID: blockerID, BlockedID: blockedID})
	return err
}

// Feed returns chronological activity from followed accounts. `since` is an
// optional keyset cursor (created_at) for infinite scroll without OFFSET.
func (s *Store) Feed(ctx context.Context, userID uuid.UUID, since *time.Time, limit int32) ([]db.ListFollowingFeedRow, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return s.q.ListFollowingFeed(ctx, db.ListFollowingFeedParams{
		UserID: userID, Since: TSPtr(since), Lim: limit,
	})
}

// CommunityFeed backs the logged-out and empty-following home page: newest
// substantive, non-spoiler reviews platform-wide. Reverse chronological, never
// personalized, never weighted.
func (s *Store) CommunityFeed(ctx context.Context, limit, offset int32) ([]db.ListRecentReviewsAcrossCommunityRow, error) {
	if limit <= 0 || limit > 50 {
		limit = 12
	}
	return s.q.ListRecentReviewsAcrossCommunity(ctx, db.ListRecentReviewsAcrossCommunityParams{Lim: limit, Off: offset})
}

// RelatedWorks is collaborative filtering over co-shelving, weighted by inverse
// popularity. It is the only "recommendation" surface and it is explainable:
// every row means "N readers who shelved this also shelved that".
func (s *Store) RelatedWorks(ctx context.Context, workID uuid.UUID, minShared int64, limit int32) ([]db.ListCoOccurringWorksRow, error) {
	if limit <= 0 || limit > 24 {
		limit = 6
	}
	if minShared < 1 {
		minShared = 1
	}
	return s.q.ListCoOccurringWorks(ctx, db.ListCoOccurringWorksParams{
		WorkID: workID, MinShared: float64(minShared), Lim: limit,
	})
}

func (s *Store) FollowCounts(ctx context.Context, userID uuid.UUID) (*db.CountFollowsRow, error) {
	row, err := s.q.CountFollows(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Store) Following(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.ListFollowingRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.q.ListFollowing(ctx, db.ListFollowingParams{UserID: userID, Lim: limit, Off: offset})
}

func (s *Store) Followers(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.ListFollowersRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.q.ListFollowers(ctx, db.ListFollowersParams{UserID: userID, Lim: limit, Off: offset})
}

// ---- Notifications -------------------------------------------------------------

func (s *Store) Notify(ctx context.Context, userID uuid.UUID, kind string, payload []byte) error {
	_, err := s.q.CreateNotification(ctx, db.CreateNotificationParams{
		UserID: userID, Kind: kind, Payload: payload,
	})
	return err
}

func (s *Store) Notifications(ctx context.Context, userID uuid.UUID, unreadOnly bool, limit, offset int32) ([]db.Notification, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	return s.q.ListNotifications(ctx, db.ListNotificationsParams{
		UserID: userID, UnreadOnly: unreadPtr(unreadOnly),
		Lim: limit, Off: offset,
	})
}

func (s *Store) UnreadNotificationCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.q.CountUnreadNotifications(ctx, userID)
}

func (s *Store) MarkNotificationsRead(ctx context.Context, userID uuid.UUID, id *uuid.UUID) error {
	if id != nil {
		_, err := s.q.MarkNotificationRead(ctx, db.MarkNotificationReadParams{ID: *id, UserID: userID})
		return err
	}
	_, err := s.q.MarkAllNotificationsRead(ctx, userID)
	return err
}
