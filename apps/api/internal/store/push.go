package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/alexandria-reads/alexandria/apps/api/internal/db"
	"github.com/alexandria-reads/alexandria/apps/api/internal/push"
)

// Push subscription management and the delivery tick.

func (s *Store) AddPushSubscription(ctx context.Context, userID uuid.UUID, endpoint string, p256dh, auth []byte, userAgent string) (*db.PushSubscription, error) {
	sub, err := s.q.AddPushSubscription(ctx, db.AddPushSubscriptionParams{
		UserID: userID, Endpoint: endpoint, P256dh: p256dh, Auth: auth,
		UserAgent: Str(userAgent),
	})
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *Store) DeletePushSubscription(ctx context.Context, userID uuid.UUID, endpoint string) error {
	_, err := s.q.DeletePushSubscription(ctx, db.DeletePushSubscriptionParams{
		UserID: userID, Endpoint: endpoint,
	})
	return err
}

func (s *Store) ListPushSubscriptions(ctx context.Context, userID uuid.UUID) ([]db.PushSubscription, error) {
	return s.q.ListPushSubscriptionsForUser(ctx, userID)
}

// PushTick delivers every due notification to every due subscription, then
// advances the subscription's cursor. Dead subscriptions (404/410) are removed
// rather than retried: a push endpoint that forgot us will never remember us.
func (s *Store) PushTick(ctx context.Context, cfg push.Config) (sent int, err error) {
	// Phase 1 — gather inside a SERVICE-context transaction: notifications
	// are RLS-private, and a delivery sweep reads across users, which only
	// the service role may do. (A bare connection here sees nothing.)
	type job struct {
		subID    uuid.UUID
		userID   uuid.UUID
		endpoint string
		p256dh   []byte
		auth     []byte
		payload  []byte
	}
	var jobs []job
	err = s.Tx(ctx, func(q *db.Queries) error {
		due, err := q.ListPushDue(ctx, 100)
		if err != nil {
			return err
		}
		for _, sub := range due {
			since := time.Unix(0, 0)
			if sub.LastPushedAt.Valid {
				since = sub.LastPushedAt.Time
			}
			notes, err := q.ListNotificationsSince(ctx, db.ListNotificationsSinceParams{
				UserID: sub.UserID, Since: TS(since), Lim: 5,
			})
			if err != nil {
				return err
			}
			if len(notes) == 0 {
				continue
			}
			n := notes[0]
			payload, _ := json.Marshal(map[string]any{
				"title": "Alexandria",
				"body":  describeNotification(n.Kind, n.Payload),
				"url":   "/notifications",
			})
			jobs = append(jobs, job{
				subID: sub.ID, userID: sub.UserID, endpoint: sub.Endpoint,
				p256dh: sub.P256dh, auth: sub.Auth, payload: payload,
			})
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	// Phase 2 — send OUTSIDE any transaction: network I/O must never hold a
	// transaction open.
	type outcome struct {
		subID    uuid.UUID
		userID   uuid.UUID
		endpoint string
		gone     bool
	}
	var outcomes []outcome
	for _, jb := range jobs {
		err := push.Send(ctx, cfg, push.Subscription{
			Endpoint: jb.endpoint, P256DH: jb.p256dh, Auth: jb.auth,
		}, jb.payload)
		if errors.Is(err, push.ErrGone) {
			outcomes = append(outcomes, outcome{subID: jb.subID, userID: jb.userID, endpoint: jb.endpoint, gone: true})
			continue
		}
		if err != nil {
			return sent, err
		}
		outcomes = append(outcomes, outcome{subID: jb.subID, userID: jb.userID, endpoint: jb.endpoint})
		sent++
	}

	// Phase 3 — advance cursors and forget dead subscriptions.
	err = s.Tx(ctx, func(q *db.Queries) error {
		for _, oc := range outcomes {
			if oc.gone {
				if _, err := q.DeletePushSubscription(ctx, db.DeletePushSubscriptionParams{
					UserID: oc.userID, Endpoint: oc.endpoint,
				}); err != nil {
					return err
				}
				continue
			}
			if _, err := q.MarkPushed(ctx, db.MarkPushedParams{
				ID: oc.subID, PushedAt: TS(time.Now()),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	return sent, err
}

// describeNotification renders the push body from the stored note. Push
// payloads are minimal by design: the detail lives in the notification
// centre, not on a lock screen.
func describeNotification(kind string, payload json.RawMessage) string {
	var p map[string]any
	_ = json.Unmarshal(payload, &p)
	actor, _ := p["actor"].(string)
	if actor == "" {
		actor, _ = p["reviewer"].(string)
	}
	switch kind {
	case "review_comment":
		return actor + " replied to your review."
	case "new_follower":
		return actor + " now follows your shelves."
	case "note_reviewed":
		if approved, ok := p["approved"].(bool); ok && approved {
			return actor + " approved your note."
		}
		return actor + " reviewed your note."
	default:
		return "You have a new notification."
	}
}
