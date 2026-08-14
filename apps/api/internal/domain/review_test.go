package domain

import (
	"errors"
	"strings"
	"testing"
)

func validBody() string {
	return "The Garnett translation flattens Dostoevsky's humor, but the architecture of guilt " +
		"underneath survives intact. Raskolnikov's fever dreams read like woodcut engravings: " +
		"stark, overexposed, unforgettable. I docked half a star only for the pacing of the epilogue."
}

func TestRatingStars(t *testing.T) {
	cases := map[Rating]string{1: "0.5", 7: "3.5", 10: "5"}
	for r, want := range cases {
		if got := r.Stars(); got != want {
			t.Errorf("Rating(%d).Stars() = %q, want %q", r, got, want)
		}
	}
}

func TestValidateRejectsShortBody(t *testing.T) {
	d := ReviewDraft{Rating: 8, Body: "great book!!"}
	if err := d.Validate(DefaultFriction()); !errors.Is(err, ErrBodyTooShort) {
		t.Fatalf("want ErrBodyTooShort, got %v", err)
	}
}

func TestValidateRejectsFiller(t *testing.T) {
	d := ReviewDraft{Rating: 8, Body: strings.Repeat("a", 200)}
	if err := d.Validate(DefaultFriction()); !errors.Is(err, ErrLowEffortBody) {
		t.Fatalf("want ErrLowEffortBody, got %v", err)
	}
	d.Body = strings.Repeat("good book very good book ", 10)
	if err := d.Validate(DefaultFriction()); !errors.Is(err, ErrLowEffortBody) {
		t.Fatalf("want ErrLowEffortBody for low-vocabulary padding, got %v", err)
	}
}

func TestValidateAcceptsRealProse(t *testing.T) {
	d := ReviewDraft{Rating: 7, Body: validBody(), PromptWhy: "The Garnett translation flattens the humor."}
	if err := d.Validate(DefaultFriction()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateCountsRunesNotBytes(t *testing.T) {
	// 150 CJK characters is a substantial review even though byte length differs.
	body := strings.Repeat("этот роман изменил меня ", 8) // cyrillic, ~190 runes, varied words
	d := ReviewDraft{Rating: 9, Body: body}
	if err := d.Validate(DefaultFriction()); errors.Is(err, ErrBodyTooShort) {
		t.Fatalf("rune counting failed: %v", err)
	}
}

func TestValidateRatingBounds(t *testing.T) {
	for _, r := range []Rating{0, 11, -3} {
		d := ReviewDraft{Rating: r, Body: validBody()}
		if err := d.Validate(DefaultFriction()); !errors.Is(err, ErrRatingOutOfRange) {
			t.Errorf("Rating %d: want ErrRatingOutOfRange, got %v", r, err)
		}
	}
}

func TestDailyLimits(t *testing.T) {
	p := DefaultFriction()
	if got := p.DailyReviewAllowance(0); got != 2 {
		t.Errorf("new account allowance = %d, want 2", got)
	}
	if got := p.DailyReviewAllowance(150); got != 10 {
		t.Errorf("trusted allowance = %d, want 10", got)
	}
	if err := p.CheckDailyLimit(0, 2); !errors.Is(err, ErrDailyLimit) {
		t.Errorf("want ErrDailyLimit at cap, got %v", err)
	}
	if err := p.CheckDailyLimit(150, 2); err != nil {
		t.Errorf("trusted reader under cap should pass, got %v", err)
	}
}
