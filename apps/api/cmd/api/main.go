// Alexandria API — the modular monolith's HTTP entrypoint.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexandria-reads/alexandria/apps/api/internal/config"
	"github.com/alexandria-reads/alexandria/apps/api/internal/events"
	"github.com/alexandria-reads/alexandria/apps/api/internal/httpapi"
	"github.com/alexandria-reads/alexandria/apps/api/internal/store"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	cfg := config.Load()
	slog.Info("booting alexandria api", "config", cfg.String())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("postgres connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	st := store.New(pool)

	// NATS is optional at boot (the outbox buffers events), mandatory for ingestion.
	if bus, err := events.Connect(cfg.NATSURL); err != nil {
		slog.Warn("nats unavailable; outbox will buffer events", "err", err)
	} else {
		defer bus.Close()
		if err := bus.EnsureStreams(ctx); err != nil {
			slog.Warn("jetstream stream setup failed", "err", err)
		}
		go events.RunOutboxRelay(ctx, outboxAdapter{st}, bus, 2*time.Second)
	}

	if err := httpapi.New(cfg, st).Run(ctx); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}

// outboxAdapter narrows *store.Store to the relay's interface.
type outboxAdapter struct{ st *store.Store }

func (a outboxAdapter) OutboxPeek(ctx context.Context, limit int32) ([]events.RelayRow, error) {
	rows, err := a.st.OutboxPeek(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]events.RelayRow, len(rows))
	for i, r := range rows {
		out[i] = events.RelayRow{ID: r.ID, Subject: r.Subject, Payload: r.Payload}
	}
	return out, nil
}

func (a outboxAdapter) OutboxDelete(ctx context.Context, ids []int64) error {
	return a.st.OutboxDelete(ctx, ids)
}
