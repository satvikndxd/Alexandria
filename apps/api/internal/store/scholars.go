package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/alexandria-reads/alexandria/apps/api/internal/db"
	"github.com/alexandria-reads/alexandria/apps/api/internal/domain"
)

// Scholar operations. Every multi-statement transition (create+citations,
// review+maybe-publish) is one transaction, so a note can never exist in a
// state its own rules forbid.

var (
	// ErrOrcidTaken: an ORCID iD identifies one person; two accounts sharing
	// one is either a mistake or an impersonation attempt, and neither gets a
	// 500.
	ErrOrcidTaken     = errors.New("that ORCID iD is already attached to another account")
	ErrNotNoteAuthor  = errors.New("only the note's author may do that")
	ErrNotVerified    = errors.New("peer review is reserved for verified scholars")
	ErrReviewOwnNote  = errors.New("you cannot review your own note")
	ErrNoteState      = errors.New("the note is not in a state that allows that")
)

// ApplyScholarProfile records an application for verification. Status stays
// pending: verification is a moderator's act, never the applicant's.
func (s *Store) ApplyScholarProfile(ctx context.Context, userID uuid.UUID, p domain.ProfileDraft) (*db.ScholarProfile, error) {
	prof, err := s.q.UpsertScholarProfile(ctx, db.UpsertScholarProfileParams{
		UserID: userID, Field: p.Field,
		Affiliation: Str(p.Affiliation), Orcid: Str(p.Orcid),
		CoiStatement: p.CoiStatement,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "scholar_profiles_orcid_key" {
			return nil, ErrOrcidTaken
		}
		return nil, err
	}
	return &prof, nil
}

func (s *Store) ScholarProfile(ctx context.Context, userID uuid.UUID) (*db.ScholarProfile, error) {
	p, err := s.q.GetScholarProfile(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) IsVerifiedScholar(ctx context.Context, userID uuid.UUID) (bool, error) {
	return s.q.IsVerifiedScholar(ctx, userID)
}

// CreateNote writes a note, its citations and its first revision atomically.
// Community provenance is stamped at creation: it is a fact about the author's
// status then, not a label that follows them forever.
func (s *Store) CreateNote(ctx context.Context, authorID, workID uuid.UUID, d domain.NoteDraft, submit bool) (*db.ScholarNote, error) {
	verified, err := s.IsVerifiedScholar(ctx, authorID)
	if err != nil {
		return nil, err
	}
	status := db.NoteStatusDraft
	if submit {
		status = db.NoteStatusInReview
	}
	var note db.ScholarNote
	err = s.Tx(ctx, func(q *db.Queries) error {
		n, err := q.CreateScholarNote(ctx, db.CreateScholarNoteParams{
			WorkID: workID, AuthorID: authorID,
			ChapterRef: d.ChapterRef, AnchorQuote: d.AnchorQuote,
			Title: d.Title, Body: d.Body, Kind: d.Kind,
			Status: status, IsCommunity: !verified,
		})
		if err != nil {
			return err
		}
		note = n
		for i, c := range d.Citations {
			if err := q.AddNoteCitation(ctx, db.AddNoteCitationParams{
				NoteID: n.ID, Citation: c.Citation, Url: Str(c.URL), Position: int32(i),
			}); err != nil {
				return err
			}
		}
		return q.RecordNoteRevision(ctx, db.RecordNoteRevisionParams{
			NoteID: n.ID, Version: n.Version, Title: n.Title, Body: n.Body,
			EditedBy: UUIDPtr(&authorID),
		})
	})
	if err != nil {
		return nil, err
	}
	return &note, nil
}

// ReviseNote bumps the version, replaces nothing silently (the revision row is
// the memory), and re-opens review if the note had published: a substantial
// change to published scholarship must be re-defended.
func (s *Store) ReviseNote(ctx context.Context, noteID, authorID uuid.UUID, d domain.NoteDraft) (*db.ScholarNote, error) {
	var note db.ScholarNote
	err := s.Tx(ctx, func(q *db.Queries) error {
		current, err := q.GetScholarNote(ctx, noteID)
		if err != nil {
			return err
		}
		if current.AuthorID != authorID {
			return ErrNotNoteAuthor
		}
		n, err := q.UpdateScholarNote(ctx, db.UpdateScholarNoteParams{
			ID: noteID, AuthorID: authorID,
			Title: d.Title, Body: d.Body, ChapterRef: d.ChapterRef,
			AnchorQuote: d.AnchorQuote, Kind: d.Kind,
		})
		if err != nil {
			return err
		}
		note = n
		if current.Status == db.NoteStatusPublished {
			if _, err := q.SetNoteStatus(ctx, db.SetNoteStatusParams{
				ID: noteID, Status: db.NoteStatusInReview,
			}); err != nil {
				return err
			}
			note.Status = db.NoteStatusInReview
		}
		for i, c := range d.Citations {
			if err := q.AddNoteCitation(ctx, db.AddNoteCitationParams{
				NoteID: noteID, Citation: c.Citation, Url: Str(c.URL), Position: int32(i),
			}); err != nil {
				return err
			}
		}
		return q.RecordNoteRevision(ctx, db.RecordNoteRevisionParams{
			NoteID: noteID, Version: n.Version, Title: n.Title, Body: n.Body,
			EditedBy: UUIDPtr(&authorID),
		})
	})
	if err != nil {
		return nil, err
	}
	return &note, nil
}

// SubmitNote moves a draft into review.
func (s *Store) SubmitNote(ctx context.Context, noteID, authorID uuid.UUID) error {
	return s.Tx(ctx, func(q *db.Queries) error {
		n, err := q.GetScholarNote(ctx, noteID)
		if err != nil {
			return err
		}
		if n.AuthorID != authorID {
			return ErrNotNoteAuthor
		}
		if n.Status != db.NoteStatusDraft {
			return ErrNoteState
		}
		cites, err := q.CountNoteCitations(ctx, noteID)
		if err != nil {
			return err
		}
		if cites < 1 {
			return domain.ErrNoteCitations
		}
		_, err = q.SetNoteStatus(ctx, db.SetNoteStatusParams{ID: noteID, Status: db.NoteStatusInReview})
		return err
	})
}

// ReviewNote records a verified scholar's judgement and publishes when the
// gate is met: two distinct approvals (author excluded, primary key prevents
// double-voting) and at least one citation.
func (s *Store) ReviewNote(ctx context.Context, noteID, reviewerID uuid.UUID, approved bool, comments string) (*db.GetScholarNoteRow, error) {
	verified, err := s.IsVerifiedScholar(ctx, reviewerID)
	if err != nil {
		return nil, err
	}
	if !verified {
		return nil, ErrNotVerified
	}
	var note db.GetScholarNoteRow
	err = s.Tx(ctx, func(q *db.Queries) error {
		n, err := q.GetScholarNote(ctx, noteID)
		if err != nil {
			return err
		}
		if n.AuthorID == reviewerID {
			return ErrReviewOwnNote
		}
		if n.Status != db.NoteStatusInReview {
			return ErrNoteState
		}
		if err := q.ReviewNote(ctx, db.ReviewNoteParams{
			NoteID: noteID, ReviewerID: reviewerID, Approved: approved, Comments: comments,
		}); err != nil {
			return err
		}
		approvals, err := q.CountNoteApprovals(ctx, db.CountNoteApprovalsParams{
			NoteID: noteID, AuthorID: n.AuthorID,
		})
		if err != nil {
			return err
		}
		cites, err := q.CountNoteCitations(ctx, noteID)
		if err != nil {
			return err
		}
		if approved && domain.PublishGate(approvals, cites) == nil {
			if _, err := q.SetNoteStatus(ctx, db.SetNoteStatusParams{
				ID: noteID, Status: db.NoteStatusPublished,
			}); err != nil {
				return err
			}
			n.Status = db.NoteStatusPublished
		}
		note = n
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &note, nil
}

// RetractNote keeps the record and its history; retraction is public correction,
// not deletion.
func (s *Store) RetractNote(ctx context.Context, noteID, actorID uuid.UUID, isModerator bool) error {
	return s.Tx(ctx, func(q *db.Queries) error {
		n, err := q.GetScholarNote(ctx, noteID)
		if err != nil {
			return err
		}
		if n.AuthorID != actorID && !isModerator {
			return ErrNotNoteAuthor
		}
		_, err = q.SetNoteStatus(ctx, db.SetNoteStatusParams{ID: noteID, Status: db.NoteStatusRetracted})
		return err
	})
}

func (s *Store) GetNote(ctx context.Context, noteID uuid.UUID) (*db.GetScholarNoteRow, error) {
	n, err := s.Queries().GetScholarNote(ctx, noteID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (s *Store) NoteCitations(ctx context.Context, noteID uuid.UUID) ([]db.NoteCitation, error) {
	return s.Queries().ListNoteCitations(ctx, noteID)
}

func (s *Store) NoteRevisions(ctx context.Context, noteID uuid.UUID) ([]db.NoteRevision, error) {
	return s.Queries().ListNoteRevisions(ctx, noteID)
}

func (s *Store) NoteReviews(ctx context.Context, noteID uuid.UUID) ([]db.ListNoteReviewsRow, error) {
	return s.Queries().ListNoteReviews(ctx, noteID)
}

func (s *Store) NotesAwaitingReview(ctx context.Context, limit, offset int32) ([]db.ListNotesAwaitingReviewRow, error) {
	return s.Queries().ListNotesAwaitingReview(ctx, db.ListNotesAwaitingReviewParams{Lim: limit, Off: offset})
}

func (s *Store) PendingScholarProfiles(ctx context.Context, limit, offset int32) ([]db.ListPendingScholarProfilesRow, error) {
	return s.Queries().ListPendingScholarProfiles(ctx, db.ListPendingScholarProfilesParams{Lim: limit, Off: offset})
}

// SetScholarStatus is the moderator's verification act, recorded with the
// moderator's own id on the profile.
func (s *Store) SetScholarStatus(ctx context.Context, moderatorID, userID uuid.UUID, status db.ScholarStatus) error {
	n, err := s.q.SetScholarStatus(ctx, db.SetScholarStatusParams{
		UserID: userID, Status: status, VerifiedBy: UUIDPtr(&moderatorID),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
