package domain

import (
	"strings"
	"testing"
)

func TestValidateUsername(t *testing.T) {
	for _, ok := range []string{"margery", "a-b_c.d", "ABC", "  padded  "} {
		if err := ValidateUsername(ok); err != nil {
			t.Errorf("ValidateUsername(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", "ab", strings.Repeat("x", 33), "-lead", "trail-", "spa ce", "emoji📚"} {
		if err := ValidateUsername(bad); err == nil {
			t.Errorf("ValidateUsername(%q) should fail", bad)
		}
	}
	if NormalizeUsername("  Margery  ") != "margery" {
		t.Error("normalisation must lower-case and trim")
	}
}

func TestValidateEmail(t *testing.T) {
	for _, ok := range []string{"reader@example.com", "first.last+tag@sub.domain.org"} {
		if err := ValidateEmail(ok); err != nil {
			t.Errorf("ValidateEmail(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", "no-at.example.com", "a@b", "a b@c.com", "a@b@c.com", strings.Repeat("x", 250) + "@e.com"} {
		if err := ValidateEmail(bad); err == nil {
			t.Errorf("ValidateEmail(%q) should fail", bad)
		}
	}
}

func TestReviewLengthIsRuneBased(t *testing.T) {
	// Length is measured in runes so non-Latin scripts are not penalised: 149
	// Japanese runes fail exactly like 149 ASCII runes, and 151 pass the length
	// rule exactly like 151 ASCII runes.
	short := ReviewDraft{Rating: 8, Body: strings.Repeat("読", 149)}
	if err := short.Validate(DefaultFriction()); err != ErrBodyTooShort {
		t.Errorf("149-rune body = %v, want ErrBodyTooShort", err)
	}

	// 16 distinct 10-rune tokens (rotations of a 20-rune alphabet): 160 runes
	// of body, 16 distinct words, no dominant glyph. Must validate clean.
	const alphabet = "あいうえおかきくけこさしすせそたちつてとな"
	var b strings.Builder
	for i := 0; i < 16; i++ {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(alphabet[i%20:] + alphabet[:i%20])
	}
	body := b.String()
	if n := len([]rune(body)); n < 150 {
		t.Fatalf("fixture body is %d runes, need >= 150", n)
	}
	ok := ReviewDraft{Rating: 8, Body: body}
	if err := ok.Validate(DefaultFriction()); err != nil {
		t.Errorf("151+-rune, 16-distinct-word Japanese body = %v, want nil", err)
	}
}

func TestValidateCommentCalibration(t *testing.T) {
	if err := ValidateComment("This changed how I'll reread it."); err != nil {
		t.Errorf("short legitimate comment rejected: %v", err)
	}
	if err := ValidateComment("thank you"); err != nil {
		t.Errorf("two-word comment rejected: %v", err)
	}
	if err := ValidateComment("ok ok ok ok"); err != ErrLowEffortBody {
		t.Errorf("one-vocabulary loop accepted: %v", err)
	}
	if err := ValidateComment(strings.Repeat("x", 50)); err != ErrLowEffortBody {
		t.Errorf("glyph spam accepted: %v", err)
	}
	if err := ValidateComment("a a a a a a a a"); err != ErrLowEffortBody {
		t.Errorf("three-word loop accepted: %v", err)
	}
}
