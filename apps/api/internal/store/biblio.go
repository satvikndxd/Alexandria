package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/alexandria-reads/alexandria/apps/api/internal/db"
)

var slugScrub = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify produces stable, URL-safe slugs ("Pride and Prejudice" →
// "pride-and-prejudice"). Callers must resolve collisions via UniqueSlug.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugScrub.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// UniqueSlug appends a numeric suffix until the slug is free. Slugs are stable
// once minted: an existing work keeps its slug even if its title is corrected.
func (s *Store) UniqueSlug(ctx context.Context, base string) (string, error) {
	candidate := base
	for i := 2; ; i++ {
		taken, err := s.q.WorkSlugIsTaken(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !taken {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
		if i > 1000 {
			return "", fmt.Errorf("could not mint a unique slug for %q", base)
		}
	}
}

func (s *Store) UniqueAuthorSlug(ctx context.Context, base string) (string, error) {
	candidate := base
	for i := 2; ; i++ {
		_, err := s.q.GetAuthorBySlug(ctx, candidate)
		if errors.Is(err, pgx.ErrNoRows) {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
		if i > 1000 {
			return "", fmt.Errorf("could not mint a unique author slug for %q", base)
		}
	}
}

// ---- Reads --------------------------------------------------------------------

func (s *Store) GetWorkBySlug(ctx context.Context, slug string) (*db.Work, error) {
	w, err := s.q.GetWorkBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (s *Store) GetWorkByID(ctx context.Context, id uuid.UUID) (*db.Work, error) {
	w, err := s.q.GetWorkByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &w, nil
}

// WorkDetail is everything the book page renders in one round trip.
type WorkDetail struct {
	Work        db.Work                           `json:"work"`
	Authors     []db.ListAuthorsForWorkRow        `json:"authors"`
	Editions    []db.ListEditionsForWorkRow       `json:"editions"`
	Subjects    []db.Subject                      `json:"subjects"`
	Ratings     []db.RatingDistributionForWorkRow `json:"rating_distribution"`
	ReviewCount int64                             `json:"review_count"`
}

func (s *Store) WorkDetail(ctx context.Context, slug string) (*WorkDetail, error) {
	w, err := s.GetWorkBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	authors, err := s.q.ListAuthorsForWork(ctx, w.ID)
	if err != nil {
		return nil, err
	}
	editions, err := s.q.ListEditionsForWork(ctx, w.ID)
	if err != nil {
		return nil, err
	}
	subjects, err := s.q.ListSubjectsForWork(ctx, w.ID)
	if err != nil {
		return nil, err
	}
	dist, err := s.q.RatingDistributionForWork(ctx, w.ID)
	if err != nil {
		return nil, err
	}
	n, err := s.q.CountReviewsForWork(ctx, w.ID)
	if err != nil {
		return nil, err
	}
	return &WorkDetail{
		Work: *w, Authors: authors, Editions: editions, Subjects: subjects,
		Ratings: dist, ReviewCount: n,
	}, nil
}

// WorkFilters drives the catalogue listing and the search fallback path.
type WorkFilters struct {
	SubjectSlug     string
	AuthorID        *uuid.UUID
	Language        string
	PublicDomain    *bool
	PublishedAfter  *int32
	PublishedBefore *int32
	TitlePrefix     string
	Sort            string // rating | recent | title
	Limit, Offset   int32
}

func (f WorkFilters) params() db.ListWorksParams {
	p := db.ListWorksParams{
		AuthorID:         UUIDPtr(f.AuthorID),
		PublicDomainOnly: f.PublicDomain,
		PublishedAfter:   f.PublishedAfter,
		PublishedBefore:  f.PublishedBefore,
		Sort:             f.Sort,
		Off:              f.Offset,
		Lim:              f.Limit,
	}
	if f.SubjectSlug != "" {
		p.SubjectSlug = &f.SubjectSlug
	}
	if f.Language != "" {
		p.Language = &f.Language
	}
	if f.TitlePrefix != "" {
		p.TitlePrefix = &f.TitlePrefix
	}
	return p
}

func (s *Store) ListWorks(ctx context.Context, f WorkFilters) ([]db.ListWorksRow, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 24
	}
	return s.q.ListWorks(ctx, f.params())
}

func (s *Store) GetAuthorBySlug(ctx context.Context, slug string) (*db.Author, error) {
	a, err := s.q.GetAuthorBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Store) GetEditionByID(ctx context.Context, id uuid.UUID) (*db.GetEditionByIDRow, error) {
	e, err := s.q.GetEditionByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// ---- Ingestion writes ----------------------------------------------------------
// Every write path is an idempotent upsert keyed on a stable external
// identifier, so a retried ingest message can never create a duplicate book.

type WorkUpsert struct {
	Title          string
	Subtitle       string
	Description    string
	Language       string
	FirstPublished *int32
	IsPublicDomain bool
	OpenLibraryID  string
	Subjects       []string
	Authors        []AuthorUpsert
}

type AuthorUpsert struct {
	Name          string
	OpenLibraryID string
	Role          string
	BirthYear     *int32
	DeathYear     *int32
}

// UpsertWorkMaterialized upserts a work, its author links and its subjects in
// one transaction. Slug is minted only for new works.
func (s *Store) UpsertWorkMaterialized(ctx context.Context, u WorkUpsert) (*db.Work, error) {
	var work db.Work
	err := s.Tx(ctx, func(q *db.Queries) error {
		var (
			w   db.Work
			err error
		)
		if u.OpenLibraryID != "" {
			existing, err := q.GetWorkByOpenLibraryID(ctx, &u.OpenLibraryID)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if err == nil {
				w = existing
			}
		}
		if w.ID == uuid.Nil {
			slug := Slugify(u.Title)
			if slug == "" {
				slug = "untitled"
			}
			slug, err = s.UniqueSlug(ctx, slug)
			if err != nil {
				return err
			}
			w, err = q.UpsertWork(ctx, db.UpsertWorkParams{
				Slug:             slug,
				Title:            u.Title,
				Subtitle:         Str(u.Subtitle),
				OriginalLanguage: orDefault(u.Language, "en"),
				FirstPublished:   u.FirstPublished,
				Description:      u.Description,
				IsPublicDomain:   u.IsPublicDomain,
				OpenlibraryID:    Str(u.OpenLibraryID),
			})
			if err != nil {
				return fmt.Errorf("upsert work: %w", err)
			}
		}
		work = w

		for i, au := range u.Authors {
			a, err := s.upsertAuthor(ctx, q, au)
			if err != nil {
				return err
			}
			if err := q.LinkWorkAuthor(ctx, db.LinkWorkAuthorParams{
				WorkID: w.ID, AuthorID: a.ID,
				Role: orDefault(au.Role, "author"), Position: int32(i),
			}); err != nil {
				return fmt.Errorf("link author: %w", err)
			}
		}
		for _, subj := range u.Subjects {
			if len(subj) > 80 || strings.TrimSpace(subj) == "" {
				continue // Open Library subjects are noisy; keep the taxonomy clean
			}
			sg, err := q.UpsertSubject(ctx, db.UpsertSubjectParams{
				Slug: Slugify(subj), Name: subj, Kind: "subject",
			})
			if err != nil {
				return err
			}
			if err := q.LinkWorkSubject(ctx, db.LinkWorkSubjectParams{
				WorkID: w.ID, SubjectID: sg.ID,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &work, nil
}

func (s *Store) upsertAuthor(ctx context.Context, q *db.Queries, au AuthorUpsert) (*db.Author, error) {
	// Authors without a stable external identifier (Gutenberg names, indie
	// authors) are deduplicated by exact name, otherwise every catalog refresh
	// would mint a new "Jane Austen-2".
	if au.OpenLibraryID == "" {
		if existing, err := q.GetAuthorByName(ctx, au.Name); err == nil {
			return &existing, nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	sortName := au.Name
	if parts := strings.Fields(au.Name); len(parts) > 1 {
		sortName = strings.Join(parts[1:], " ") + ", " + parts[0]
	}
	slugBase := Slugify(au.Name)
	if slugBase == "" {
		slugBase = "unknown"
	}
	slug, err := s.UniqueAuthorSlug(ctx, slugBase)
	if err != nil {
		return nil, err
	}
	a, err := q.UpsertAuthor(ctx, db.UpsertAuthorParams{
		Name: au.Name, SortName: sortName, Bio: "", Slug: slug,
		OpenlibraryID: Str(au.OpenLibraryID),
		BirthYear:     au.BirthYear, DeathYear: au.DeathYear,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert author: %w", err)
	}
	return &a, nil
}

// UpsertGutenbergEdition creates or refreshes the Gutenberg manifestation of a
// work, using its own conflict target (Gutenberg texts have no ISBN).
func (s *Store) UpsertGutenbergEdition(ctx context.Context, workID uuid.UUID, title, language string, gutenbergID int32, metadata []byte) (*db.Edition, error) {
	e, err := s.q.UpsertGutenbergEdition(ctx, db.UpsertGutenbergEditionParams{
		WorkID: workID, Title: title, Language: orDefault(language, "en"),
		GutenbergID: &gutenbergID, Metadata: metadata,
	})
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// MetadataJSON renders an edition's flexible metadata column.
func MetadataJSON(pairs map[string]any) []byte {
	if len(pairs) == 0 {
		return []byte("{}")
	}
	b, err := json.Marshal(pairs)
	if err != nil {
		return []byte("{}")
	}
	return b
}

// ParseDate accepts the ISO-8601 date shapes ingestion sources provide
// ("2006-01-02", "2006-01", "2006") and yields an invalid pgtype.Date for
// anything else, so malformed upstream dates degrade to NULL, never to a
// wrong date.
func ParseDate(t string) pgtype.Date {
	for _, layout := range []string{"2006-01-02", "2006-01", "2006"} {
		if d, err := time.Parse(layout, t); err == nil {
			return pgtype.Date{Time: d.UTC(), Valid: true}
		}
	}
	return pgtype.Date{}
}
