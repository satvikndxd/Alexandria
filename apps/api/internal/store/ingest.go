package store

import (
	"context"
	"regexp"
	"strings"
)

var slugScrub = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify produces stable, URL-safe slugs ("Pride and Prejudice" → "pride-and-prejudice").
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugScrub.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// UpsertWorkFromOpenLibrary is the idempotent write path used by the
// ingestion worker. Subjects are upserted and linked in the same tx.
func (s *Store) UpsertWorkFromOpenLibrary(ctx context.Context, olid, title, description string, subjects []string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var workID string
	err = tx.QueryRow(ctx, `
INSERT INTO works (slug, title, description, openlibrary_id)
VALUES ($1, $2, $3, $4)
ON CONFLICT (openlibrary_id) DO UPDATE SET
  title = EXCLUDED.title,
  description = CASE WHEN EXCLUDED.description <> '' THEN EXCLUDED.description ELSE works.description END
RETURNING id`, Slugify(title)+"-"+strings.ToLower(olid), title, description, olid).Scan(&workID)
	if err != nil {
		return err
	}

	for _, subj := range subjects {
		if len(subj) > 80 || subj == "" {
			continue // OL subjects can be noisy; keep the taxonomy clean
		}
		var subjID string
		if err := tx.QueryRow(ctx, `
INSERT INTO subjects (slug, name) VALUES ($1, $2)
ON CONFLICT (slug) DO UPDATE SET name = subjects.name
RETURNING id`, Slugify(subj), subj).Scan(&subjID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO work_subjects (work_id, subject_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING`, workID, subjID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
