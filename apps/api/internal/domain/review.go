// Package domain holds pure business logic — no I/O, no SQL, fully unit-testable.
package domain

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Rating is stored in half-stars: 1 == ½★, 10 == 5★.
// Half-star granularity matches how readers actually think ("a 3.5 book")
// while integer storage keeps aggregation exact.
type Rating int

const (
	RatingMin Rating = 1
	RatingMax Rating = 10
)

func (r Rating) Valid() bool { return r >= RatingMin && r <= RatingMax }

// Stars renders "3.5" for display.
func (r Rating) Stars() string {
	if r%2 == 0 {
		return fmt.Sprintf("%d", r/2)
	}
	return fmt.Sprintf("%d.5", r/2)
}

// ReviewDraft is what a user submits; Validate applies the anti-slop rules.
type ReviewDraft struct {
	Rating      Rating
	Title       string
	Body        string
	PromptWhy   string
	HasSpoilers bool
}

// FrictionPolicy carries the structural anti-slop knobs. These are friction,
// not detection: we do not pretend to detect AI text — we make low-effort
// mass posting structurally unattractive.
type FrictionPolicy struct {
	MinBodyChars      int
	MaxBodyChars      int
	NewAccountDaily   int
	TrustedDaily      int
	TrustedReputation int
}

func DefaultFriction() FrictionPolicy {
	return FrictionPolicy{
		MinBodyChars:      150,
		MaxBodyChars:      20000,
		NewAccountDaily:   2,
		TrustedDaily:      10,
		TrustedReputation: 100,
	}
}

var (
	ErrRatingOutOfRange = errors.New("rating must be between 0.5 and 5 stars")
	ErrBodyTooShort     = errors.New("review body is below the minimum length — tell us why, not just what")
	ErrBodyTooLong      = errors.New("review body exceeds the maximum length")
	ErrTitleTooLong     = errors.New("review title exceeds 200 characters")
	ErrLowEffortBody    = errors.New("review looks like filler — repeated characters do not count as prose")
	ErrDailyLimit       = errors.New("daily review limit reached — Trusted Readers unlock higher limits")
)

// Validate enforces the content-shape rules. Length is measured in runes so
// non-Latin scripts are not penalized.
func (d ReviewDraft) Validate(p FrictionPolicy) error {
	if !d.Rating.Valid() {
		return ErrRatingOutOfRange
	}
	if utf8.RuneCountInString(d.Title) > 200 {
		return ErrTitleTooLong
	}
	body := strings.TrimSpace(d.Body)
	n := utf8.RuneCountInString(body)
	if n < p.MinBodyChars {
		return ErrBodyTooShort
	}
	if n > p.MaxBodyChars {
		return ErrBodyTooLong
	}
	if isFiller(body) {
		return ErrLowEffortBody
	}
	return nil
}

// DailyReviewAllowance returns how many reviews this account may post per day.
func (p FrictionPolicy) DailyReviewAllowance(reputation int) int {
	if reputation >= p.TrustedReputation {
		return p.TrustedDaily
	}
	return p.NewAccountDaily
}

// CheckDailyLimit compares an account's posts in the trailing 24h window
// against its allowance.
func (p FrictionPolicy) CheckDailyLimit(reputation int, postedLast24h int) error {
	if postedLast24h >= p.DailyReviewAllowance(reputation) {
		return ErrDailyLimit
	}
	return nil
}

// isFiller catches the cheapest padding strategies: a body dominated by a
// single repeated character or containing almost no distinct words.
func isFiller(body string) bool {
	counts := map[rune]int{}
	total := 0
	for _, r := range body {
		if r == ' ' || r == '\n' || r == '\t' {
			continue
		}
		counts[r]++
		total++
	}
	if total == 0 {
		return true
	}
	for _, c := range counts {
		if float64(c)/float64(total) > 0.4 { // one glyph is >40% of the text
			return true
		}
	}
	words := map[string]struct{}{}
	for _, w := range strings.Fields(strings.ToLower(body)) {
		words[w] = struct{}{}
	}
	return len(words) < 12 // 150+ chars but fewer than 12 distinct words
}
