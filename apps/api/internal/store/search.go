package store

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/alexandria-reads/alexandria/apps/api/internal/db"
	"github.com/alexandria-reads/alexandria/apps/api/internal/search"
)

// Search projections: Postgres → Meilisearch documents.
//
// These are the only place the index document shape is assembled, so the
// worker, the reindex command and the event-driven refresh can never disagree
// about what a "work document" is.

// SearchDocs builds index documents for the given work ids. Unknown ids yield
// no document (a deletion is propagated by DeleteDocuments, not by indexing an
// empty shell).
func (s *Store) SearchDocs(ctx context.Context, ids []uuid.UUID) ([]search.WorkDoc, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.q.WorkSearchDocuments(ctx, db.WorkSearchDocumentsParams{
		Ids: ids, Lim: int32(len(ids)), Off: 0,
	})
	if err != nil {
		return nil, err
	}
	out := make([]search.WorkDoc, 0, len(rows))
	for _, r := range rows {
		out = append(out, workDocFromRow(r))
	}
	return out, nil
}

// WorkDocsPage pages the whole catalogue for a full reindex, using keyset
// pagination on id so the cursor stays cheap at any depth.
func (s *Store) WorkDocsPage(ctx context.Context, after uuid.UUID, limit int32) ([]search.WorkDoc, error) {
	ids, err := s.q.ListWorkIDsForIndex(ctx, db.ListWorkIDsForIndexParams{
		AfterID: after, Lim: limit,
	})
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return s.SearchDocs(ctx, ids)
}

func (s *Store) AuthorDocsPage(ctx context.Context, limit, offset int32) ([]search.AuthorDoc, error) {
	rows, err := s.q.AuthorSearchDocuments(ctx, db.AuthorSearchDocumentsParams{Lim: limit, Off: offset})
	if err != nil {
		return nil, err
	}
	out := make([]search.AuthorDoc, 0, len(rows))
	for _, r := range rows {
		out = append(out, search.AuthorDoc{
			ID: r.ID.String(), Kind: "author", Slug: r.Slug, Name: r.Name,
			SortName: r.SortName, Bio: r.Bio,
			BirthYear: r.BirthYear, DeathYear: r.DeathYear,
			Claimed: r.IsClaimed, WorkCount: r.WorkCount,
		})
	}
	return out, nil
}

func (s *Store) ClubDocsPage(ctx context.Context, limit, offset int32) ([]search.ClubDoc, error) {
	rows, err := s.q.ClubSearchDocuments(ctx, db.ClubSearchDocumentsParams{Lim: limit, Off: offset})
	if err != nil {
		return nil, err
	}
	out := make([]search.ClubDoc, 0, len(rows))
	for _, r := range rows {
		out = append(out, search.ClubDoc{
			ID: r.ID.String(), Kind: "club", Slug: r.Slug, Name: r.Name,
			Description: r.Description, MemberCount: r.MemberCount,
		})
	}
	return out, nil
}

func workDocFromRow(r db.WorkSearchDocumentsRow) search.WorkDoc {
	doc := search.WorkDoc{
		ID: r.ID.String(), Kind: "work", Slug: r.Slug, Title: r.Title,
		Subtitle: StrGet(r.Subtitle), Description: r.Description,
		Language: r.OriginalLanguage, FirstPublished: r.FirstPublished,
		PublicDomain: r.IsPublicDomain,
		Rating:       averageHalfStars(r.RatingSum, r.RatingCount),
		RatingCount:  r.RatingCount,
		ReviewCount:  r.ReviewCount,
		GutenbergID:  r.GutenbergID,
	}
	var extras struct {
		EditionID *string `json:"edition_id"`
		CoverKey  *string `json:"cover_key"`
		License   *string `json:"license"`
	}
	_ = json.Unmarshal(jsonField(r.EditionJson), &extras)
	doc.EditionID = StrGet(extras.EditionID)
	doc.CoverKey = StrGet(extras.CoverKey)
	doc.CoverLicense = StrGet(extras.License)

	_ = json.Unmarshal(jsonField(r.AuthorsJson), &doc.Authors)
	if doc.Authors == nil {
		doc.Authors = []search.AuthorRef{}
	}
	_ = json.Unmarshal(jsonField(r.SubjectsJson), &doc.Subjects)
	if doc.Subjects == nil {
		doc.Subjects = []string{}
	}
	return doc
}

func averageHalfStars(sum, count int64) float64 {
	if count == 0 {
		return 0
	}
	return float64(sum) / float64(count) / 2.0
}

// jsonField normalizes sqlc's interface{} typing for json columns: pgx hands
// back []byte, but a defensive branch keeps the projection robust if the
// driver ever returns a string.
func jsonField(v any) []byte {
	switch t := v.(type) {
	case nil:
		return []byte("null")
	case []byte:
		return t
	case string:
		return []byte(t)
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return []byte("null")
		}
		return b
	}
}
