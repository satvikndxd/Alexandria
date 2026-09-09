// Package importx parses reading history exported from other platforms into
// an import *plan*: a pure, inspectable description of what would change.
// Applying a plan is the store's job (internal/store/imports.go); deciding
// what a row means is done here, where it can be unit-tested without a
// database.
//
// Principles:
//   - Nothing is invented. A rating without qualifying prose becomes a private
//     shelf rating, never a stub review with machine-written body.
//   - Nothing is duplicated. Matching is exact (ISBN13) or exact-normalized
//     title; unmatched rows are reported, not created as new works.
//   - Nothing is waived. Imported review prose still must clear the 150-char
//     bar to publish; shorter imported text stays a rating.
package importx

import (
	"regexp"
	"strings"
	"time"
)

// timeTime keeps call sites readable where *time.Time would double the noise.
type timeTime = time.Time

// MatchKey identifies the target work without assuming it exists.
type MatchKey struct {
	ISBN13 string
	Title  string
	Author string
}

// ShelfPlan moves a work onto a shelf, with the reader's own rating and dates.
type ShelfPlan struct {
	Work       MatchKey
	ShelfKind  string // want_to_read | reading | read | dnf | favorites
	CustomName string // set when ShelfKind == "custom"
	RatingHalf int    // 1..10, 0 = none
	Format     string
	StartedOn  *time.Time
	FinishedOn *time.Time
}

// ReviewPlan publishes the reader's own imported prose, unchanged.
type ReviewPlan struct {
	Work        MatchKey
	RatingHalf  int
	Title       string
	Body        string
	HasSpoilers bool
}

// HighlightPlan is a Kindle clipping awaiting alignment against a cached
// public-domain text.
type HighlightPlan struct {
	BookTitle string
	Author    string
	Passage   string
	Kind      string // highlight | note | bookmark
	AddedOn   string
}

// Skip records a row we deliberately did not import, with the reason, so the
// reader can see exactly what happened to their history.
type Skip struct {
	Row    int    `json:"row"`
	Reason string `json:"reason"`
	Detail string `json:"detail"`
}

// Plan is the whole import, ready to apply or to preview.
type Plan struct {
	Source     string // goodreads | storygraph | kindle
	Shelves    []ShelfPlan
	Reviews    []ReviewPlan
	Highlights []HighlightPlan
	Skips      []Skip
}

func (p *Plan) skip(row int, reason, detail string) {
	p.Skips = append(p.Skips, Skip{Row: row, Reason: reason, Detail: detail})
}

// ———— normalization helpers ————

var (
	articleRe = regexp.MustCompile(`^(the|a|an)\s+`)
	punctRe   = regexp.MustCompile(`[^\p{L}\p{N} ]+`)
	spaceRe   = regexp.MustCompile(`\s+`)
	isbnNoise = regexp.MustCompile(`[^0-9Xx]`)
	nonDigits = regexp.MustCompile(`[^0-9]`)
)

// NormalizeTitle lower-cases, strips a leading article and punctuation, and
// collapses whitespace, so "The Picture of Dorian Gray!" matches
// "picture of dorian gray".
func NormalizeTitle(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	t = punctRe.ReplaceAllString(t, " ")
	t = spaceRe.ReplaceAllString(t, " ")
	t = articleRe.ReplaceAllString(t, "")
	return strings.TrimSpace(t)
}

// CleanISBN13 strips spreadsheet quoting ("=“978…”") and hyphens, and returns
// "" for anything that is not a 13-digit ISBN.
func CleanISBN13(raw string) string {
	s := isbnNoise.ReplaceAllString(raw, "")
	if len(s) == 10 {
		return "" // we match on ISBN-13 only; 10-digit conversions are guesswork
	}
	if len(s) != 13 {
		return ""
	}
	return s
}

// parseDate accepts the date shapes the three exporters actually emit.
func parseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{"2006/01/02", "2006-01-02", "01/02/2006", "Jan 2, 2006", "2006-01-02T15:04:05Z07:00"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// halfStars converts a 0..5 float rating to Alexandria's 1..10 half-stars.
func halfStars(f float64) int {
	if f <= 0 {
		return 0
	}
	h := int(round(f * 2))
	if h < 1 {
		h = 1
	}
	if h > 10 {
		h = 10
	}
	return h
}

func round(f float64) int {
	if f < 0 {
		return int(f - 0.5)
	}
	return int(f + 0.5)
}
