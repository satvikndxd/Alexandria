// Package migrations embeds the Alexandria SQL schema.
//
// packages/db is the single source of truth for the schema: sqlc reads
// migrations/ to generate type-safe Go, and apps/api/cmd/migrate embeds the
// very same files to apply them. Embedding (rather than reading a path at
// runtime) means a released API binary carries its own schema and cannot drift
// from the code it was built with.
//
// This is a separate Go module so `go:embed` can reach the SQL: embed patterns
// may not reference paths outside the embedding module. apps/api depends on it
// through a `replace` directive — the standard monorepo pattern.
package migrations

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed *.sql
var files embed.FS

// Migration is one numbered, ordered schema change.
type Migration struct {
	// Version is the numeric prefix, e.g. 10 for "0010_auth.sql".
	Version int
	// Name is the file name, used as the idempotency key.
	Name string
	// SQL is the full migration body, applied in a single transaction.
	SQL string
}

// All returns every embedded migration in version order.
func All() ([]Migration, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}
	out := make([]Migration, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		version, err := parseVersion(e.Name())
		if err != nil {
			return nil, err
		}
		body, err := files.ReadFile(e.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", e.Name(), err)
		}
		out = append(out, Migration{Version: version, Name: e.Name(), SQL: string(body)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// parseVersion extracts the leading zero-padded number from "0010_auth.sql".
// A migration that does not follow the convention is a build-time mistake, so
// it fails loudly rather than being silently skipped.
func parseVersion(name string) (int, error) {
	base := strings.TrimSuffix(name, ".sql")
	idx := strings.Index(base, "_")
	if idx <= 0 {
		return 0, fmt.Errorf("migration %q must be named NNNN_description.sql", name)
	}
	var v int
	if _, err := fmt.Sscanf(base[:idx], "%d", &v); err != nil {
		return 0, fmt.Errorf("migration %q has a non-numeric version prefix: %w", name, err)
	}
	return v, nil
}
