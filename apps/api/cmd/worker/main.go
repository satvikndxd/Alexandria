// Alexandria ingestion worker — consumes JetStream subjects and hydrates
// the bibliography. Run as many replicas as needed; JetStream work-queue
// semantics guarantee each message is processed once.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/alexandria-reads/alexandria/apps/api/internal/config"
	"github.com/alexandria-reads/alexandria/apps/api/internal/events"
	"github.com/alexandria-reads/alexandria/apps/api/internal/ingest"
	"github.com/alexandria-reads/alexandria/apps/api/internal/store"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	cfg := config.Load()
	slog.Info("booting alexandria worker", "config", cfg.String())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("postgres connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	st := store.New(pool)

	bus, err := events.Connect(cfg.NATSURL)
	if err != nil {
		slog.Error("nats connect failed", "err", err)
		os.Exit(1)
	}
	defer bus.Close()
	if err := bus.EnsureStreams(ctx); err != nil {
		slog.Error("jetstream stream setup failed", "err", err)
		os.Exit(1)
	}

	ol := ingest.NewOpenLibraryClient()
	slog.Info("worker consuming", "subject", events.SubjectOpenLibraryWork)
	if err := bus.Consume(ctx, "ol-work-worker", events.SubjectOpenLibraryWork,
		ingest.HandleWorkMessage(ol, st)); err != nil && ctx.Err() == nil {
		slog.Error("consumer failed", "err", err)
		os.Exit(1)
	}
}
