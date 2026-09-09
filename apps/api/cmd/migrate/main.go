// Command migrate applies the embedded Alexandria schema.
//
//	go run ./cmd/migrate up        # apply pending migrations
//	go run ./cmd/migrate status    # show applied vs embedded
//
// Migrations are embedded at build time (packages/db/migrations.go), so a
// released binary always carries the exact schema it was compiled against.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/alexandria-reads/alexandria/apps/api/internal/config"
	"github.com/alexandria-reads/alexandria/apps/api/internal/migrate"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cfg := config.Load()
	pool, err := migrate.ConnectPool(ctx, cfg.DatabaseURL)
	if err != nil {
		fail("connect", err)
	}
	defer pool.Close()

	m := migrate.New(pool)
	switch cmd {
	case "up":
		applied, err := m.Up(ctx)
		if err != nil {
			fail("migrate", err)
		}
		if len(applied) == 0 {
			fmt.Println("schema is up to date")
			return
		}
		for _, name := range applied {
			fmt.Println("applied", name)
		}
	case "status":
		st, err := m.Status(ctx)
		if err != nil {
			fail("status", err)
		}
		migrate.SortStatuses(st)
		fmt.Printf("%-8s %-34s %s\n", "VERSION", "NAME", "STATE")
		pending := 0
		for _, s := range st {
			state := "pending"
			if s.Applied != nil {
				state = "applied " + s.Applied.AppliedAt.Format(time.RFC3339)
			} else {
				pending++
			}
			fmt.Printf("%-8d %-34s %s\n", s.Migration.Version, s.Migration.Name, state)
		}
		if pending > 0 {
			os.Exit(1) // useful as a CI/deploy gate
		}
	default:
		fmt.Fprintf(os.Stderr, "usage: migrate {up|status}\n")
		os.Exit(2)
	}
}

func fail(stage string, err error) {
	if errors.Is(err, migrate.ErrHistoryEdited) {
		slog.Error("migration history was edited; refusing to proceed", "stage", stage, "err", err)
	} else {
		slog.Error("migration failed", "stage", stage, "err", err)
	}
	os.Exit(1)
}
