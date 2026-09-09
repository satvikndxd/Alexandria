package importx

import (
	"encoding/csv"
	"io"
	"strconv"
	"strings"
)

// ParseGoodreads reads a Goodreads `reviews.csv` export. Column names are
// looked up from the header rather than assumed by position, because
// Goodreads has reordered them over the years.
//
// Mapping decisions (documented, not accidental):
//   - exclusive shelf drives the reading state; unknown names become custom
//     shelves rather than being forced into a state they are not
//   - "My Review" publishes only if it clears the 150-character bar; shorter
//     imported text becomes a private shelf rating instead
//   - additional (non-exclusive) bookshelves are ignored: Alexandria's custom
//     shelves are the reader's to create, and silently mass-creating them from
//     another platform's tags would clutter rather than serve
func ParseGoodreads(r io.Reader) (*Plan, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true

	header, err := cr.Read()
	if err != nil {
		return nil, err
	}
	col := map[string]int{}
	for i, name := range header {
		col[strings.TrimSpace(name)] = i
	}
	get := func(rec []string, name string) string {
		i, ok := col[name]
		if !ok || i >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[i])
	}

	p := &Plan{Source: "goodreads"}
	row := 1
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		row++
		if err != nil {
			p.skip(row, "malformed_row", err.Error())
			continue
		}
		title := get(rec, "Title")
		if title == "" {
			p.skip(row, "no_title", "")
			continue
		}
		key := MatchKey{
			ISBN13: CleanISBN13(get(rec, "ISBN13")),
			Title:  title,
			Author: get(rec, "Author"),
		}

		rating := 0
		if n, err := strconv.Atoi(get(rec, "My Rating")); err == nil && n > 0 {
			rating = halfStars(float64(n))
		}
		var finished, started *timeTime
		if t, ok := parseDate(get(rec, "Date Read")); ok {
			finished = &t
		}
		if t, ok := parseDate(get(rec, "Date Added")); ok {
			started = &t
		}

		switch shelf := get(rec, "Exclusive Shelf"); shelf {
		case "read":
			p.Shelves = append(p.Shelves, ShelfPlan{Work: key, ShelfKind: "read", RatingHalf: rating, FinishedOn: finished, StartedOn: started})
		case "currently-reading":
			p.Shelves = append(p.Shelves, ShelfPlan{Work: key, ShelfKind: "reading", RatingHalf: rating, StartedOn: started})
		case "to-read":
			p.Shelves = append(p.Shelves, ShelfPlan{Work: key, ShelfKind: "want_to_read", RatingHalf: rating})
		case "":
			p.skip(row, "no_shelf", title)
		default:
			p.Shelves = append(p.Shelves, ShelfPlan{Work: key, ShelfKind: "custom", CustomName: shelf, RatingHalf: rating})
		}

		body := get(rec, "My Review")
		if runeLen(body) >= minReviewRunes {
			p.Reviews = append(p.Reviews, ReviewPlan{
				Work: key, RatingHalf: rating, Body: body,
				HasSpoilers: strings.EqualFold(get(rec, "Spoiler"), "true"),
			})
		} else if body != "" {
			p.skip(row, "review_below_minimum", "kept as rating only; Alexandria reviews are ≥150 characters")
		}
	}
	return p, nil
}
