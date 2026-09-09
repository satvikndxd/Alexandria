// Command worker consumes JetStream subjects and hydrates the bibliography,
// the search projection, and the email outbox. Run as many replicas as needed:
// JetStream work-queue semantics guarantee each message is processed once.
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
	"github.com/alexandria-reads/alexandria/apps/api/internal/ingest"
	"github.com/alexandria-reads/alexandria/apps/api/internal/mailer"
	"github.com/alexandria-reads/alexandria/apps/api/internal/search"
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

	var sc *search.Client
	if cfg.MeiliURL != "" {
		sc = search.New(cfg.MeiliURL, cfg.MeiliKey)
	}

	ol := ingest.NewOpenLibraryClient()

	// Metadata ingestion: paced, identified, retried by JetStream.
	go consume(ctx, bus, "ol-work-worker", events.SubjectOpenLibraryWork,
		ingest.HandleWorkMessage(ol, st))

	// Search projection refresh.
	if sc != nil {
		go consume(ctx, bus, "search-index-worker", events.SubjectSearchIndex,
			ingest.HandleSearchIndex(st, searchIndexer{sc}))
	}

	// Transactional email.
	var sender mailer.Sender = mailer.LogSender{}
	if cfg.SMTPHost != "" {
		sender = mailer.SMTPSender{
			Host: cfg.SMTPHost, Port: cfg.SMTPPort,
			Username: cfg.SMTPUsername, Password: cfg.SMTPPassword, From: cfg.SMTPFrom,
		}
	}
	go func() {
		if err := mailer.New(st, sender).Run(ctx); err != nil {
			slog.Error("mailer stopped", "err", err)
		}
	}()

	// Optional one-shot Gutenberg catalog ingest at boot. The catalog is ~5 MB
	// gzipped and 90k rows, so it runs as a bounded batch and re-queues itself
	// until converged rather than blocking worker startup.
	if os.Getenv("RUN_GUTENBERG_CATALOG") == "true" {
		go runCatalog(ctx, st, cfg)
	}

	<-ctx.Done()
	slog.Info("worker shutting down")
	time.Sleep(500 * time.Millisecond) // let in-flight handlers ack
}

func consume(ctx context.Context, bus *events.Bus, durable, subject string, h func(ctx context.Context, data []byte) error) {
	if err := bus.Consume(ctx, durable, subject, h); err != nil && ctx.Err() == nil {
		slog.Error("consumer failed", "durable", durable, "err", err)
		os.Exit(1)
	}
}

func runCatalog(ctx context.Context, st *store.Store, cfg config.Config) {
	const batch = 500
	for {
		applied, err := ingest.RunCatalogJob(ctx, st, cfg.GutenbergCatalogURL, batch)
		if err != nil {
			slog.Error("gutenberg catalog run failed", "err", err)
			return
		}
		slog.Info("gutenberg catalog batch applied", "rows", applied)
		if applied < batch {
			slog.Info("gutenberg catalog converged")
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

// searchIndexer adapts the search client to the worker's interface.
type searchIndexer struct{ sc *search.Client }

func (s searchIndexer) IndexWorks(ctx context.Context, docs []search.WorkDoc) error {
	if len(docs) == 0 {
		return nil
	}
	return s.sc.IndexWorks(ctx, docs)
}
