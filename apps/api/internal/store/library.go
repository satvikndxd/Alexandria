package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/alexandria-reads/alexandria/apps/api/internal/db"
)

// The library is the RLS-covered surface of the product. Every function here
// runs inside TxUser/ReadUser, so even a logic bug cannot read another
// reader's shelves: Postgres refuses the row before the query sees it.

func (s *Store) Shelves(ctx context.Context, userID uuid.UUID) ([]db.ListShelvesForUserRow, error) {
	var out []db.ListShelvesForUserRow
	err := s.ReadUser(ctx, userID, func(q *db.Queries) error {
		rows, err := q.ListShelvesForUser(ctx, userID)
		if err != nil {
			return err
		}
		out = rows
		return nil
	})
	return out, err
}

func (s *Store) LibraryCounts(ctx context.Context, userID uuid.UUID) ([]db.LibraryCountsForUserRow, error) {
	var out []db.LibraryCountsForUserRow
	err := s.ReadUser(ctx, userID, func(q *db.Queries) error {
		rows, err := q.LibraryCountsForUser(ctx, userID)
		if err != nil {
			return err
		}
		out = rows
		return nil
	})
	return out, err
}

func (s *Store) LibraryView(ctx context.Context, userID uuid.UUID, shelfKind string, limit, offset int32) ([]db.ListShelfItemsWithWorkRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 24
	}
	var out []db.ListShelfItemsWithWorkRow
	err := s.ReadUser(ctx, userID, func(q *db.Queries) error {
		rows, err := q.ListShelfItemsWithWork(ctx, db.ListShelfItemsWithWorkParams{
			UserID:    userID,
			ShelfKind: shelfKindOr(shelfKind),
			Lim:       limit,
			Off:       offset,
		})
		if err != nil {
			return err
		}
		out = rows
		return nil
	})
	return out, err
}

func shelfKindOr(kind string) db.NullShelfKind {
	switch db.ShelfKind(kind) {
	case db.ShelfKindWantToRead, db.ShelfKindReading, db.ShelfKindRead,
		db.ShelfKindDnf, db.ShelfKindFavorites, db.ShelfKindCustom:
		return db.NullShelfKind{ShelfKind: db.ShelfKind(kind), Valid: true}
	}
	return db.NullShelfKind{}
}

// LibraryEntry is what the book page needs to render the caller's own state.
func (s *Store) LibraryEntry(ctx context.Context, userID, workID uuid.UUID) (*db.GetLibraryEntryRow, error) {
	var out *db.GetLibraryEntryRow
	err := s.ReadUser(ctx, userID, func(q *db.Queries) error {
		row, err := q.GetLibraryEntry(ctx, db.GetLibraryEntryParams{UserID: userID, WorkID: workID})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // not on any shelf: a valid state, not an error
		}
		if err != nil {
			return err
		}
		out = &row
		return nil
	})
	return out, err
}

// ShelfMove describes a change of reading state.
type ShelfMove struct {
	WorkID    uuid.UUID
	EditionID *uuid.UUID
	Format    string
	To        db.ShelfKind
}

// MoveToShelf moves a work to a reading-state shelf. Moving to reading/read/
// dnf/want_to_read first removes it from the other three, so a book is never
// simultaneously "currently reading" and "read". Favorites is orthogonal and
// untouched.
func (s *Store) MoveToShelf(ctx context.Context, userID uuid.UUID, mv ShelfMove) error {
	return s.TxUser(ctx, userID, func(q *db.Queries) error {
		shelf, err := q.GetSystemShelf(ctx, db.GetSystemShelfParams{UserID: userID, Kind: mv.To})
		if errors.Is(err, pgx.ErrNoRows) {
			// A fresh account whose system shelves were never seeded (e.g. the
			// account predates seeding). Heal it rather than fail the request.
			if err := q.EnsureSystemShelves(ctx, userID); err != nil {
				return err
			}
			shelf, err = q.GetSystemShelf(ctx, db.GetSystemShelfParams{UserID: userID, Kind: mv.To})
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		keep := db.ShelfKindCustom // "remove everything" for favorites moves
		if mv.To != db.ShelfKindFavorites && mv.To != db.ShelfKindCustom {
			keep = mv.To
		}
		if _, err := q.RemoveWorkFromUserShelves(ctx, db.RemoveWorkFromUserShelvesParams{
			WorkID: mv.WorkID, UserID: userID, KeepKind: keep,
		}); err != nil {
			return fmt.Errorf("clear prior state: %w", err)
		}
		if _, err := q.AddToShelf(ctx, db.AddToShelfParams{
			ShelfID: shelf.ID, WorkID: mv.WorkID,
			EditionID: UUIDPtr(mv.EditionID),
			Format:    readFormatOr(mv.Format),
		}); err != nil {
			return fmt.Errorf("add to shelf: %w", err)
		}
		return nil
	})
}

func readFormatOr(f string) db.NullReadFormat {
	switch db.ReadFormat(f) {
	case db.ReadFormatPhysical, db.ReadFormatEbook, db.ReadFormatAudiobook,
		db.ReadFormatPublicDomain, db.ReadFormatLibraryCopy, db.ReadFormatOther:
		return db.NullReadFormat{ReadFormat: db.ReadFormat(f), Valid: true}
	}
	return db.NullReadFormat{}
}

func (s *Store) RemoveFromLibrary(ctx context.Context, userID, workID uuid.UUID) error {
	return s.TxUser(ctx, userID, func(q *db.Queries) error {
		shelves, err := q.ListShelvesForUser(ctx, userID)
		if err != nil {
			return err
		}
		for _, sh := range shelves {
			if _, err := q.RemoveFromShelf(ctx, db.RemoveFromShelfParams{
				ShelfID: sh.ID, WorkID: workID,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// SetProgress records reading progress in basis points (0..10000). Reaching
// 100% closes the session and stamps finished_on; the shelf follows to "read".
// Falling back from 100% reopens the session.
func (s *Store) SetProgress(ctx context.Context, userID, workID uuid.UUID, editionID *uuid.UUID, format string, progressBP int32) (*db.ReadingSession, error) {
	if progressBP < 0 || progressBP > 10000 {
		return nil, fmt.Errorf("progress must be 0..10000 basis points, got %d", progressBP)
	}
	var session db.ReadingSession
	err := s.TxUser(ctx, userID, func(q *db.Queries) error {
		sess, err := q.UpsertReadingProgress(ctx, db.UpsertReadingProgressParams{
			UserID: userID, WorkID: workID,
			EditionID:  UUIDPtr(editionID),
			Format:     readFormatOr(format),
			ProgressBp: progressBP,
		})
		if err != nil {
			return err
		}
		session = sess

		// Keep the shelf in agreement with the session: finishing a book moves
		// it to Read; being in progress moves it to Currently Reading.
		target := db.ShelfKindReading
		if progressBP >= 10000 {
			target = db.ShelfKindRead
		}
		shelf, err := q.GetSystemShelf(ctx, db.GetSystemShelfParams{UserID: userID, Kind: target})
		if err != nil {
			return err
		}
		if _, err := q.AddToShelf(ctx, db.AddToShelfParams{
			ShelfID: shelf.ID, WorkID: workID,
			EditionID: UUIDPtr(editionID), Format: readFormatOr(format),
		}); err != nil {
			return err
		}
		if _, err := q.RemoveWorkFromUserShelves(ctx, db.RemoveWorkFromUserShelvesParams{
			WorkID: workID, UserID: userID, KeepKind: target,
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// MarkFinished closes the open session with an optional explicit finish date.
func (s *Store) MarkFinished(ctx context.Context, userID, workID uuid.UUID, finishedOn *time.Time) error {
	return s.TxUser(ctx, userID, func(q *db.Queries) error {
		n, err := q.FinishSession(ctx, db.FinishSessionParams{
			UserID: userID, WorkID: workID, FinishedOn: DatePtr(finishedOn),
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		shelf, err := q.GetSystemShelf(ctx, db.GetSystemShelfParams{UserID: userID, Kind: db.ShelfKindRead})
		if err != nil {
			return err
		}
		if _, err := q.AddToShelf(ctx, db.AddToShelfParams{ShelfID: shelf.ID, WorkID: workID}); err != nil {
			return err
		}
		_, err = q.RemoveWorkFromUserShelves(ctx, db.RemoveWorkFromUserShelvesParams{
			WorkID: workID, UserID: userID, KeepKind: db.ShelfKindRead,
		})
		return err
	})
}

// MarkDNF records a did-not-finish. DNF is honest data: it closes the session
// but never pretends the book was completed.
func (s *Store) MarkDNF(ctx context.Context, userID, workID uuid.UUID, finishedOn *time.Time) error {
	return s.TxUser(ctx, userID, func(q *db.Queries) error {
		n, err := q.MarkSessionDNF(ctx, db.MarkSessionDNFParams{
			UserID: userID, WorkID: workID, FinishedOn: DatePtr(finishedOn),
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		shelf, err := q.GetSystemShelf(ctx, db.GetSystemShelfParams{UserID: userID, Kind: db.ShelfKindDnf})
		if err != nil {
			return err
		}
		if _, err := q.AddToShelf(ctx, db.AddToShelfParams{ShelfID: shelf.ID, WorkID: workID}); err != nil {
			return err
		}
		_, err = q.RemoveWorkFromUserShelves(ctx, db.RemoveWorkFromUserShelvesParams{
			WorkID: workID, UserID: userID, KeepKind: db.ShelfKindDnf,
		})
		return err
	})
}

func (s *Store) CurrentlyReading(ctx context.Context, userID uuid.UUID, limit int32) ([]db.ListCurrentlyReadingRow, error) {
	if limit <= 0 || limit > 24 {
		limit = 8
	}
	var out []db.ListCurrentlyReadingRow
	err := s.ReadUser(ctx, userID, func(q *db.Queries) error {
		rows, err := q.ListCurrentlyReading(ctx, db.ListCurrentlyReadingParams{UserID: userID, Lim: limit})
		if err != nil {
			return err
		}
		out = rows
		return nil
	})
	return out, err
}

func (s *Store) ReadingStats(ctx context.Context, userID uuid.UUID) (*db.ReadingStatsForUserRow, error) {
	var out *db.ReadingStatsForUserRow
	err := s.ReadUser(ctx, userID, func(q *db.Queries) error {
		row, err := q.ReadingStatsForUser(ctx, userID)
		if err != nil {
			return err
		}
		out = &row
		return nil
	})
	return out, err
}

// ---- Custom shelves ------------------------------------------------------------

func (s *Store) CreateCustomShelf(ctx context.Context, userID uuid.UUID, name string, isPrivate bool) (*db.Shelf, error) {
	var shelf db.Shelf
	err := s.TxUser(ctx, userID, func(q *db.Queries) error {
		s2, err := q.CreateCustomShelf(ctx, db.CreateCustomShelfParams{
			UserID: userID, Name: name, IsPrivate: isPrivate,
		})
		if err != nil {
			return err
		}
		shelf = s2
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &shelf, nil
}

func (s *Store) DeleteCustomShelf(ctx context.Context, userID, shelfID uuid.UUID) error {
	return s.TxUser(ctx, userID, func(q *db.Queries) error {
		_, err := q.DeleteCustomShelf(ctx, db.DeleteCustomShelfParams{ID: shelfID, UserID: userID})
		return err
	})
}

// ---- Annotations -----------------------------------------------------------------

func (s *Store) CreateAnnotation(ctx context.Context, a db.CreateAnnotationParams) (*db.Annotation, error) {
	var out db.Annotation
	err := s.TxUser(ctx, a.UserID, func(q *db.Queries) error {
		ann, err := q.CreateAnnotation(ctx, a)
		if err != nil {
			return err
		}
		out = ann
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Store) ListAnnotations(ctx context.Context, userID, editionID uuid.UUID, chapter *int32) ([]db.Annotation, error) {
	var out []db.Annotation
	err := s.ReadUser(ctx, userID, func(q *db.Queries) error {
		rows, err := q.ListAnnotationsForEdition(ctx, db.ListAnnotationsForEditionParams{
			UserID: userID, EditionID: editionID, ChapterIdx: chapter,
		})
		if err != nil {
			return err
		}
		out = rows
		return nil
	})
	return out, err
}

func (s *Store) DeleteAnnotation(ctx context.Context, userID, id uuid.UUID) error {
	return s.TxUser(ctx, userID, func(q *db.Queries) error {
		n, err := q.DeleteAnnotation(ctx, db.DeleteAnnotationParams{ID: id, UserID: userID})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// ---- reading-life projection (the right rail) ---------------------------------
// Every figure the rail shows is derived from the reader's own sessions,
// annotations and reviews. When there is genuinely nothing yet, the numbers
// are zero — the UI never invents a streak or a page count.

type ReadingLife struct {
	Year       db.YearStatsForUserRow `json:"year"`
	StreakDays int                    `json:"streak_days"`
	// StreakCells renders the last 14 days for the rail's square calendar:
	// "filled" | "empty" | "today".
	StreakCells []string `json:"streak_cells"`
}

// ReadingLife computes year totals and the current streak. The streak rule —
// consecutive days, anchored at today or yesterday so a reader who slept in
// isn't punished at breakfast — lives here, in Go, where it can be unit
// tested, rather than in SQL date arithmetic.
func (s *Store) ReadingLife(ctx context.Context, userID uuid.UUID) (*ReadingLife, error) {
	year, err := s.q.YearStatsForUser(ctx, db.YearStatsForUserParams{
		UserID: userID, Year: int32(time.Now().Year()),
	})
	if err != nil {
		return nil, err
	}
	days, err := s.q.ListActivityDays(ctx, db.ListActivityDaysParams{UserID: userID, Lim: 400})
	if err != nil {
		return nil, err
	}
	active := make(map[time.Time]bool, len(days))
	for _, d := range days {
		if d.Valid {
			active[dayKey(d.Time)] = true
		}
	}
	life := &ReadingLife{Year: year, StreakDays: streakFrom(active, time.Now())}
	life.StreakCells = streakCells(active, time.Now(), 14)
	return life, nil
}

func dayKey(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func streakFrom(active map[time.Time]bool, now time.Time) int {
	today := dayKey(now)
	cursor := today
	if !active[cursor] {
		cursor = cursor.AddDate(0, 0, -1) // grace: yesterday still counts this morning
		if !active[cursor] {
			return 0
		}
	}
	n := 0
	for active[cursor] {
		n++
		cursor = cursor.AddDate(0, 0, -1)
	}
	return n
}

func streakCells(active map[time.Time]bool, now time.Time, n int) []string {
	today := dayKey(now)
	cells := make([]string, 0, n)
	for i := n - 1; i >= 0; i-- {
		d := today.AddDate(0, 0, -i)
		switch {
		case i == 0:
			if active[d] {
				cells = append(cells, "today")
			} else {
				cells = append(cells, "empty")
			}
		case active[d]:
			cells = append(cells, "filled")
		default:
			cells = append(cells, "empty")
		}
	}
	return cells
}
