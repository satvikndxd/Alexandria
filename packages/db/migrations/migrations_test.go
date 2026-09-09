package migrations

import (
	"strings"
	"testing"
)

func TestAllIsOrderedAndNonEmpty(t *testing.T) {
	ms, err := All()
	if err != nil {
		t.Fatalf("All() error: %v", err)
	}
	if len(ms) < 11 {
		t.Fatalf("expected the full schema to be embedded, got %d migrations", len(ms))
	}
	for i := 1; i < len(ms); i++ {
		if ms[i].Version <= ms[i-1].Version {
			t.Errorf("migrations out of order: %s (v%d) follows %s (v%d)",
				ms[i].Name, ms[i].Version, ms[i-1].Name, ms[i-1].Version)
		}
		if ms[i].Version == ms[i-1].Version {
			t.Errorf("duplicate migration version %d", ms[i].Version)
		}
	}
	for _, m := range ms {
		if strings.TrimSpace(m.SQL) == "" {
			t.Errorf("migration %s is empty", m.Name)
		}
	}
}

// TestVersionsAreUnique guards the property the runner relies on: version is
// the primary key of schema_migrations, so a collision would silently skip a
// migration and leave environments diverged.
func TestVersionsAreUnique(t *testing.T) {
	ms, err := All()
	if err != nil {
		t.Fatalf("All() error: %v", err)
	}
	seen := map[int]string{}
	for _, m := range ms {
		if prev, dup := seen[m.Version]; dup {
			t.Fatalf("version %d used by both %s and %s", m.Version, prev, m.Name)
		}
		seen[m.Version] = m.Name
	}
}

func TestParseVersionRejectsBadNames(t *testing.T) {
	for _, name := range []string{"auth.sql", "_auth.sql", "ten_auth.sql", "0010auth.sql"} {
		if _, err := parseVersion(name); err == nil {
			t.Errorf("parseVersion(%q) should have failed", name)
		}
	}
	for _, tc := range []struct {
		name string
		want int
	}{{"0001_extensions.sql", 1}, {"0010_auth.sql", 10}, {"0123_x_y.sql", 123}} {
		got, err := parseVersion(tc.name)
		if err != nil {
			t.Fatalf("parseVersion(%q): %v", tc.name, err)
		}
		if got != tc.want {
			t.Errorf("parseVersion(%q) = %d, want %d", tc.name, got, tc.want)
		}
	}
}
