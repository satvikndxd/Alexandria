// Command api is the modular monolith's HTTP entrypoint.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexandria-reads/alexandria/apps/api/internal/auth"
	"github.com/alexandria-reads/alexandria/apps/api/internal/config"
	"github.com/alexandria-reads/alexandria/apps/api/internal/events"
	"github.com/alexandria-reads/alexandria/apps/api/internal/httpapi"
	"github.com/alexandria-reads/alexandria/apps/api/internal/metrics"
	"github.com/alexandria-reads/alexandria/apps/api/internal/search"
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

	authSvc, err := auth.NewService(st, cfg.WebAuthnRPID, cfg.WebAuthnRPDisplayName,
		cfg.WebAuthnOrigins, cfg.SessionSecureCookie)
	if err != nil {
		slog.Error("webauthn setup failed", "err", err)
		os.Exit(1)
	}
	magic := auth.NewMagicLinks(st, cfg.WebOrigin, cfg.SessionSecureCookie)

	var sc *search.Client
	if cfg.MeiliURL != "" {
		sc = search.New(cfg.MeiliURL, cfg.MeiliKey)
		if err := sc.EnsureIndexes(ctx); err != nil {
			// Search is derived state: boot without it rather than refuse to
			// serve. The degraded path answers from Postgres.
			slog.Warn("meilisearch unavailable; search degrades to postgres", "err", err)
		}
	}

	// NATS is optional at boot: the outbox buffers events until the bus
	// returns, which is precisely why ingestion is event-driven.
	if bus, err := events.Connect(cfg.NATSURL); err != nil {
		slog.Warn("nats unavailable; outbox will buffer events", "err", err)
	} else {
		defer bus.Close()
		if err := bus.EnsureStreams(ctx); err != nil {
			slog.Warn("jetstream stream setup failed", "err", err)
		}
		go events.RunOutboxRelay(ctx, outboxAdapter{st}, bus, 2*time.Second)
	}

	// Housekeeping: ceremonies and dead sessions expire on their own, but the
	// rows need pruning to keep the lookup indexes tight; the same tick
	// publishes queue-depth gauges.
	go pruneLoop(ctx, st, time.Hour)

	if err := httpapi.New(cfg, st, authSvc, magic, sc).Run(ctx); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}

func pruneLoop(ctx context.Context, st *store.Store, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := st.PruneStaleAuth(ctx); err != nil {
				slog.Warn("auth pruning failed", "err", err)
			}
			if depth, err := st.Queries().OutboxDepth(ctx); err == nil {
				metrics.SetGauge("outbox_depth", depth)
			}
		}
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
