package importx

import (
	"encoding/csv"
	"io"
	"strconv"
	"strings"
)

// ParseStoryGraph reads a The StoryGraph CSV export. Status vocabulary differs
// from Goodreads ("Abandoned", "Did not finish"), and ratings arrive as floats
// ("4.5") or star words; both are handled.
func ParseStoryGraph(r io.Reader) (*Plan, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true

	header, err := cr.Read()
	if err != nil {
		return nil, err
	}
	col := map[string]int{}
	for i, name := range header {
		col[strings.ToLower(strings.TrimSpace(name))] = i
	}
	get := func(rec []string, name string) string {
		i, ok := col[strings.ToLower(name)]
		if !ok || i >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[i])
	}

	p := &Plan{Source: "storygraph"}
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
		key := MatchKey{Title: title, Author: get(rec, "Author")}

		rating := 0
		if f, err := strconv.ParseFloat(get(rec, "Rating"), 64); err == nil {
			rating = halfStars(f)
		}

		// "Dates read" may hold a range or a list; the last date is the finish.
		var started, finished *timeTime
		if dates := splitDates(get(rec, "Dates read")); len(dates) > 0 {
			started = dates[0]
			finished = dates[len(dates)-1]
		}

		switch strings.ToLower(get(rec, "Read status")) {
		case "read", "finished":
			p.Shelves = append(p.Shelves, ShelfPlan{Work: key, ShelfKind: "read", RatingHalf: rating, StartedOn: started, FinishedOn: finished})
		case "currently reading", "reading":
			p.Shelves = append(p.Shelves, ShelfPlan{Work: key, ShelfKind: "reading", RatingHalf: rating, StartedOn: started})
		case "to read", "want to read":
			p.Shelves = append(p.Shelves, ShelfPlan{Work: key, ShelfKind: "want_to_read", RatingHalf: rating})
		case "did not finish", "abandoned", "dnf":
			p.Shelves = append(p.Shelves, ShelfPlan{Work: key, ShelfKind: "dnf", RatingHalf: rating, FinishedOn: finished})
		default:
			p.skip(row, "unknown_status", get(rec, "Read status"))
		}

		body := get(rec, "Review")
		if runeLen(body) >= minReviewRunes {
			p.Reviews = append(p.Reviews, ReviewPlan{Work: key, RatingHalf: rating, Body: body})
		} else if body != "" {
			p.skip(row, "review_below_minimum", "kept as rating only; Alexandria reviews are ≥150 characters")
		}
	}
	return p, nil
}

// splitDates parses "2023/04/12" or "2023/04/12→2023/05/01" or comma lists.
func splitDates(s string) []*timeTime {
	if s == "" {
		return nil
	}
	var out []*timeTime
	for _, part := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '→' || r == '-' && false || r == ';'
	}) {
		if t, ok := parseDate(part); ok {
			out = append(out, &t)
		}
	}
	return out
}
