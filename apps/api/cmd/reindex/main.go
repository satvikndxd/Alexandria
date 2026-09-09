// Command reindex rebuilds the Meilisearch projection from Postgres.
//
// The index is derived state, so this command is the recovery path for every
// search inconsistency: schema change, Meilisearch restore, or a worker that
// was down for a week. It pages the catalogue with keyset cursors so a
// million-book library reindexes in bounded memory.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/alexandria-reads/alexandria/apps/api/internal/config"
	"github.com/alexandria-reads/alexandria/apps/api/internal/search"
	"github.com/alexandria-reads/alexandria/apps/api/internal/store"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	batch := flag.Int("batch", 500, "documents per indexing request")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	cfg := config.Load()
	pool, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("postgres connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	st := store.New(pool)

	sc := search.New(cfg.MeiliURL, cfg.MeiliKey)
	if err := sc.EnsureIndexes(ctx); err != nil {
		slog.Error("index setup failed", "err", err)
		os.Exit(1)
	}

	var (
		after  uuid.UUID
		worksN int
	)
	for {
		docs, err := st.WorkDocsPage(ctx, after, int32(*batch))
		if err != nil {
			slog.Error("work page failed", "err", err)
			os.Exit(1)
		}
		if len(docs) == 0 {
			break
		}
		if err := sc.IndexWorks(ctx, docs); err != nil {
			slog.Error("index works failed", "err", err)
			os.Exit(1)
		}
		worksN += len(docs)
		after = uuid.MustParse(docs[len(docs)-1].ID)
		slog.Info("indexed works", "total", worksN)
	}

	for offset := int32(0); ; offset += int32(*batch) {
		docs, err := st.AuthorDocsPage(ctx, int32(*batch), offset)
		if err != nil {
			slog.Error("author page failed", "err", err)
			os.Exit(1)
		}
		if len(docs) == 0 {
			break
		}
		if err := sc.IndexAuthors(ctx, docs); err != nil {
			slog.Error("index authors failed", "err", err)
			os.Exit(1)
		}
	}
	for offset := int32(0); ; offset += int32(*batch) {
		docs, err := st.ClubDocsPage(ctx, int32(*batch), offset)
		if err != nil {
			slog.Error("club page failed", "err", err)
			os.Exit(1)
		}
		if len(docs) == 0 {
			break
		}
		if err := sc.IndexClubs(ctx, docs); err != nil {
			slog.Error("index clubs failed", "err", err)
			os.Exit(1)
		}
	}

	slog.Info("reindex complete", "works", worksN)
}
