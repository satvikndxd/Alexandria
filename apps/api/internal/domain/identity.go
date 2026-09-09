package domain

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Identity validation lives in the domain layer so the same rules guard the
// HTTP edge, the Flutter client (mirrored in packages/core), and any future
// importer. The database CHECK constraints remain the final backstop.

var (
	ErrUsernameFormat = errors.New("usernames are 3–32 characters of letters, digits, dot, dash or underscore")
	ErrEmailFormat    = errors.New("that does not look like an email address")
	ErrDisplayNameLen = errors.New("display names are limited to 80 characters")
	ErrBioLen         = errors.New("biographies are limited to 2000 characters")
)

var (
	usernameRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9._-]{1,30}[a-z0-9])?$`)
	// Deliberately simple: a full RFC 5322 parser rejects real addresses and
	// accepts useless ones. "one @, dot after it, no spaces" catches the mistakes
	// that matter; delivery failure is the real validator.
	emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s.]+(\.[^@\s.]+)+$`)
)

// NormalizeUsername lower-cases and trims; usernames are case-insensitive
// (citext) but displayed as typed at registration, so we store the canonical
// form and let profiles carry display preference.
func NormalizeUsername(u string) string {
	return strings.ToLower(strings.TrimSpace(u))
}

func ValidateUsername(u string) error {
	u = NormalizeUsername(u)
	n := utf8.RuneCountInString(u)
	if n < 3 || n > 32 {
		return ErrUsernameFormat
	}
	if !usernameRe.MatchString(u) {
		return ErrUsernameFormat
	}
	return nil
}

func NormalizeEmail(e string) string {
	return strings.ToLower(strings.TrimSpace(e))
}

func ValidateEmail(e string) error {
	e = NormalizeEmail(e)
	if utf8.RuneCountInString(e) > 254 || !emailRe.MatchString(e) {
		return ErrEmailFormat
	}
	return nil
}

func ValidateDisplayName(s string) error {
	if utf8.RuneCountInString(s) > 80 {
		return ErrDisplayNameLen
	}
	return nil
}

func ValidateBio(s string) error {
	if utf8.RuneCountInString(s) > 2000 {
		return ErrBioLen
	}
	return nil
}

// ValidateComment guards review comments: short-form but not empty, and with
// enough distinct content that "nice" x200 does not pass.
func ValidateComment(body string) error {
	body = strings.TrimSpace(body)
	n := utf8.RuneCountInString(body)
	if n < 2 || n > 5000 {
		return errors.New("comments are 2–5000 characters")
	}
	if isFiller(body) {
		return ErrLowEffortBody
	}
	return nil
}
