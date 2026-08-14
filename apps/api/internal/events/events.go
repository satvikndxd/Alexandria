// Package events wires the transactional outbox to NATS JetStream.
//
// Why an outbox instead of publishing directly from handlers:
//   - a domain write and its event are atomic (same Postgres tx)
//   - NATS downtime never loses events, it only delays them
//   - workers get at-least-once delivery with idempotency keys to dedupe
package events

import (
	"context"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	StreamIngest = "INGEST"

	SubjectOpenLibraryWork = "ingest.openlibrary_work"
	SubjectCoverFetch      = "ingest.cover_fetch"
	SubjectGutenbergText   = "ingest.gutenberg_text"
	SubjectSearchIndex     = "ingest.search_index"
)

type Bus struct {
	nc *nats.Conn
	js jetstream.JetStream
}

func Connect(url string) (*Bus, error) {
	nc, err := nats.Connect(url,
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.Name("alexandria-api"))
	if err != nil {
		return nil, err
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, err
	}
	return &Bus{nc: nc, js: js}, nil
}

// EnsureStreams declares the ingestion stream idempotently at boot.
func (b *Bus) EnsureStreams(ctx context.Context) error {
	_, err := b.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      StreamIngest,
		Subjects:  []string{"ingest.>"},
		Retention: jetstream.WorkQueuePolicy,
		Storage:   jetstream.FileStorage,
		MaxAge:    7 * 24 * time.Hour,
	})
	return err
}

func (b *Bus) Publish(ctx context.Context, subject string, payload []byte) error {
	_, err := b.js.Publish(ctx, subject, payload)
	return err
}

// Consume attaches a durable pull consumer for one worker kind.
func (b *Bus) Consume(ctx context.Context, durable, filter string, handle func(ctx context.Context, data []byte) error) error {
	cons, err := b.js.CreateOrUpdateConsumer(ctx, StreamIngest, jetstream.ConsumerConfig{
		Durable:       durable,
		FilterSubject: filter,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    5,
		BackOff:       []time.Duration{5 * time.Second, 30 * time.Second, 2 * time.Minute, 10 * time.Minute},
	})
	if err != nil {
		return err
	}
	cc, err := cons.Consume(func(msg jetstream.Msg) {
		if err := handle(ctx, msg.Data()); err != nil {
			slog.Warn("ingest handler failed; will redeliver", "subject", msg.Subject(), "err", err)
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
	})
	if err != nil {
		return err
	}
	<-ctx.Done()
	cc.Stop()
	return nil
}

func (b *Bus) Close() { b.nc.Drain() } //nolint:errcheck

// Outbox relay ---------------------------------------------------------------

type peeker interface {
	OutboxPeek(ctx context.Context, limit int32) ([]RelayRow, error)
	OutboxDelete(ctx context.Context, ids []int64) error
}

type RelayRow struct {
	ID      int64
	Subject string
	Payload []byte
}

func RunOutboxRelay(ctx context.Context, src peeker, bus *Bus, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			rows, err := src.OutboxPeek(ctx, 100)
			if err != nil {
				slog.Error("outbox peek failed", "err", err)
				continue
			}
			var done []int64
			for _, r := range rows {
				if err := bus.Publish(ctx, r.Subject, r.Payload); err != nil {
					slog.Warn("outbox publish failed; retry next tick", "subject", r.Subject, "err", err)
					break // preserve ordering
				}
				done = append(done, r.ID)
			}
			if len(done) > 0 {
				if err := src.OutboxDelete(ctx, done); err != nil {
					slog.Error("outbox delete failed", "err", err)
				}
			}
		}
	}
}
