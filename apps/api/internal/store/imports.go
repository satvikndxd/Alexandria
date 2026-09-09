package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/alexandria-reads/alexandria/apps/api/internal/db"
	"github.com/alexandria-reads/alexandria/apps/api/internal/domain"
	"github.com/alexandria-reads/alexandria/apps/api/internal/importx"
)

// Import application. The plan is pure (internal/importx); everything here is
// a transactional write with provenance stamped on the row, so a reader or a
// moderator can always tell imported history from Alexandria history.

// ImportReport is what the reader sees after an import: exactly what landed,
// and exactly what did not, with reasons.
type ImportReport struct {
	Shelved           int             `json:"shelved"`
	RatingsSet        int             `json:"ratings_set"`
	ReviewsCreated    int             `json:"reviews_created"`
	HighlightsAligned int             `json:"highlights_aligned"`
	Skips             []importx.Skip  `json:"skips"`
}

// AlignedHighlight is a clipping whose offsets were resolved against a cached
// public-domain text before reaching the store.
type AlignedHighlight struct {
	EditionID  uuid.UUID
	ChapterIdx int32
	Start      int32
	End        int32
	Passage    string
	Kind       string
}

const importWindow = time.Hour
const importLimitPerWindow = 3

// TakeImportSlot enforces the import throttle: three imports per hour per
// hashed bucket. A flood of history is the same shape of abuse as a flood of
// logins and gets the same structural answer.
func (s *Store) TakeImportSlot(ctx context.Context, bucket string) (bool, error) {
	n, err := s.q.CountImportsSince(ctx, db.CountImportsSinceParams{
		BucketKey: bucket, Since: TS(time.Now().Add(-importWindow)),
	})
	if err != nil {
		return false, err
	}
	allowed := n < importLimitPerWindow
	if err := s.q.RecordImportAttempt(ctx, db.RecordImportAttemptParams{
		BucketKey: bucket, Allowed: allowed,
	}); err != nil {
		return false, err
	}
	return allowed, nil
}

// MatchWork resolves an import key conservatively: ISBN-13 first, then exact
// normalized title with an author-surname sanity check. It never creates.
func (s *Store) MatchWork(ctx context.Context, key importx.MatchKey) (*db.Work, error) {
	if key.ISBN13 != "" {
		ed, err := s.q.FindEditionByISBN13(ctx, &key.ISBN13)
		if err == nil {
			return s.GetWorkByID(ctx, ed.WorkID)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	needle := importx.NormalizeTitle(key.Title)
	if needle == "" {
		return nil, ErrNotFound
	}
	cands, err := s.q.FindWorksByNormalizedTitle(ctx, needle)
	if err != nil {
		return nil, err
	}
	for _, w := range cands {
		if key.Author == "" {
			return &w, nil
		}
		authors, err := s.q.ListAuthorsForWork(ctx, w.ID)
		if err != nil {
			return nil, err
		}
		surname := lastWord(key.Author)
		for _, a := range authors {
			if strings.Contains(strings.ToLower(a.Name), strings.ToLower(surname)) {
				return &w, nil
			}
		}
	}
	return nil, ErrNotFound
}

func lastWord(s string) string {
	f := strings.Fields(s)
	if len(f) == 0 {
		return s
	}
	return f[len(f)-1]
}

// ApplyImport writes the plan. Each row is independent: one unmatchable title
// cannot abort a thousand that match, and every refusal is reported.
func (s *Store) ApplyImport(ctx context.Context, userID uuid.UUID, plan *importx.Plan) (*ImportReport, error) {
	rep := &ImportReport{}
	for i, sp := range plan.Shelves {
		work, err := s.MatchWork(ctx, sp.Work)
		if err != nil {
			rep.Skips = append(rep.Skips, importx.Skip{Row: i, Reason: "no_matching_work", Detail: sp.Work.Title})
			continue
		}
		if err := s.shelfImported(ctx, userID, *work, sp, plan.Source); err != nil {
			rep.Skips = append(rep.Skips, importx.Skip{Row: i, Reason: "write_failed", Detail: err.Error()})
			continue
		}
		rep.Shelved++
		if sp.RatingHalf > 0 {
			rep.RatingsSet++
		}
	}
	for i, rp := range plan.Reviews {
		work, err := s.MatchWork(ctx, rp.Work)
		if err != nil {
			rep.Skips = append(rep.Skips, importx.Skip{Row: i, Reason: "no_matching_work", Detail: rp.Work.Title})
			continue
		}
		if _, err := s.q.GetReviewByUserAndWork(ctx, db.GetReviewByUserAndWorkParams{
			UserID: userID, WorkID: work.ID,
		}); err == nil {
			rep.Skips = append(rep.Skips, importx.Skip{Row: i, Reason: "review_exists", Detail: work.Title})
			continue
		}
		draft := domain.ReviewDraft{Rating: domain.Rating(rp.RatingHalf), Body: rp.Body, Title: rp.Title, HasSpoilers: rp.HasSpoilers}
		if err := draft.Validate(domain.DefaultFriction()); err != nil {
			rep.Skips = append(rep.Skips, importx.Skip{Row: i, Reason: "friction", Detail: err.Error()})
			continue
		}
		if _, err := s.CreateReview(ctx, ReviewInput{
			UserID: userID, WorkID: work.ID, Rating: int32(rp.RatingHalf),
			Title: rp.Title, Body: rp.Body, HasSpoilers: rp.HasSpoilers,
			PromptWhy: "Imported from " + plan.Source + "; the reader's own words, unchanged.",
		}); err != nil {
			rep.Skips = append(rep.Skips, importx.Skip{Row: i, Reason: "write_failed", Detail: err.Error()})
			continue
		}
		rep.ReviewsCreated++
	}
	return rep, nil
}

func (s *Store) shelfImported(ctx context.Context, userID uuid.UUID, work db.Work, sp importx.ShelfPlan, source string) error {
	return s.TxUser(ctx, userID, func(q *db.Queries) error {
		var shelf db.Shelf
		var err error
		if sp.ShelfKind == "custom" {
			name := sp.CustomName
			shelf, err = q.CustomShelfByName(ctx, db.CustomShelfByNameParams{UserID: userID, Name: name})
			if errors.Is(err, pgx.ErrNoRows) {
				shelf, err = q.CreateCustomShelf(ctx, db.CreateCustomShelfParams{
					UserID: userID, Name: name, IsPrivate: false,
				})
			}
			if err != nil {
				return err
			}
		} else {
			shelf, err = q.GetSystemShelf(ctx, db.GetSystemShelfParams{
				UserID: userID, Kind: db.ShelfKind(sp.ShelfKind),
			})
			if errors.Is(err, pgx.ErrNoRows) {
				if err := q.EnsureSystemShelves(ctx, userID); err != nil {
					return err
				}
				shelf, err = q.GetSystemShelf(ctx, db.GetSystemShelfParams{
					UserID: userID, Kind: db.ShelfKind(sp.ShelfKind),
				})
			}
			if err != nil {
				return err
			}
			// Reading-state exclusivity applies to imports too: a book is not
			// simultaneously currently-reading and read.
			if _, err := q.RemoveWorkFromUserShelves(ctx, db.RemoveWorkFromUserShelvesParams{
				WorkID: work.ID, UserID: userID, KeepKind: db.ShelfKind(sp.ShelfKind),
			}); err != nil {
				return err
			}
		}
		item, err := q.AddToShelf(ctx, db.AddToShelfParams{
			ShelfID: shelf.ID, WorkID: work.ID,
			Format: readFormatOr(sp.Format),
		})
		if err != nil {
			return err
		}
		if sp.RatingHalf > 0 {
			rating := int32(sp.RatingHalf)
			if _, err := q.SetShelfItemRating(ctx, db.SetShelfItemRatingParams{
				ID: item.ID, Rating: &rating, ImportedFrom: &source,
			}); err != nil {
				return err
			}
		}
		if sp.FinishedOn != nil {
			if _, err := q.ImportClosedSession(ctx, db.ImportClosedSessionParams{
				UserID: userID, WorkID: work.ID,
				StartedOn: DatePtr(sp.StartedOn), FinishedOn: DatePtr(sp.FinishedOn),
				Format: readFormatOr(sp.Format),
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// ApplyHighlights writes aligned clippings as private annotations with
// provenance.
func (s *Store) ApplyHighlights(ctx context.Context, userID uuid.UUID, items []AlignedHighlight, source string) (int, error) {
	n := 0
	for _, h := range items {
		ann, err := s.CreateAnnotation(ctx, db.CreateAnnotationParams{
			UserID: userID, EditionID: h.EditionID, ChapterIdx: h.ChapterIdx,
			StartOff: h.Start, EndOff: h.End, Kind: h.Kind, Body: h.Passage,
			IsPrivate: true,
		})
		if err != nil {
			return n, err
		}
		if _, err := s.q.MarkAnnotationImported(ctx, db.MarkAnnotationImportedParams{
			ID: ann.ID, ImportedFrom: &source,
		}); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// ---- export & deletion -----------------------------------------------------------

// ExportBundle is the reader's whole Alexandria life, in one document: the
// privacy promise completed is a command, not a support ticket.
type ExportBundle struct {
	ExportedAt  time.Time           `json:"exported_at"`
	Profile     any                 `json:"profile"`
	Shelves     any                 `json:"shelves"`
	ShelfItems  any                 `json:"shelf_items"`
	Sessions    any                 `json:"reading_sessions"`
	Reviews     any                 `json:"reviews"`
	Annotations any                 `json:"annotations"`
	Notes       any                 `json:"scholar_notes"`
}

func (s *Store) Export(ctx context.Context, userID uuid.UUID) (*ExportBundle, error) {
	b := &ExportBundle{ExportedAt: time.Now()}
	var err error
	if b.Profile, err = s.q.GetProfileByUserID(ctx, userID); err != nil {
		return nil, err
	}
	if b.Shelves, err = s.q.ListShelvesForUser(ctx, userID); err != nil {
		return nil, err
	}
	if b.ShelfItems, err = s.q.ListShelfItemsWithWork(ctx, db.ListShelfItemsWithWorkParams{
		UserID: userID, Lim: 100000,
	}); err != nil {
		return nil, err
	}
	if b.Sessions, err = s.q.ListReadingSessions(ctx, db.ListReadingSessionsParams{
		UserID: userID, Lim: 100000,
	}); err != nil {
		return nil, err
	}
	if b.Reviews, err = s.q.ListReviewsByUser(ctx, db.ListReviewsByUserParams{UserID: userID, Lim: 100000}); err != nil {
		return nil, err
	}
	if b.Annotations, err = s.q.ListAnnotationsForUser(ctx, db.ListAnnotationsForUserParams{UserID: userID, Lim: 100000}); err != nil {
		return nil, err
	}
	return b, nil
}

// DeleteAccount soft-deletes and revokes every session. The hard purge runs as
// a scheduled job after the appeal window (documented in docs/plan/19); the
// soft delete is immediate and total from the reader's point of view.
func (s *Store) DeleteAccount(ctx context.Context, userID uuid.UUID) error {
	return s.Tx(ctx, func(q *db.Queries) error {
		if _, err := q.RevokeSessionsForUser(ctx, userID); err != nil {
			return err
		}
		return q.SoftDeleteUser(ctx, userID)
	})
}
