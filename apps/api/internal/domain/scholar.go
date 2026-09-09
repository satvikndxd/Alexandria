package domain

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Scholar-note validation and the publication gate, as pure logic so the
// rules are unit-testable and identical whether a note arrives from the web
// form, an importer, or a future API client.
//
// The gate is epistemic, not behavioural: a note publishes when two distinct
// verified scholars defend it and it cites its sources — never when it is
// popular.

var (
	ErrNoteTitle     = errors.New("note titles are 3–200 characters")
	ErrNoteBody      = errors.New("scholar notes are at least 200 characters — a margin gloss is not scholarship")
	ErrNoteKind      = errors.New("kind must be one of: context, linguistic, historical, interpretive, textual")
	ErrNoteCitations = errors.New("a note must cite at least one source")
	ErrCitation      = errors.New("each citation needs at least 10 characters of recoverable provenance")
	ErrCitationURL   = errors.New("citation URLs must be http(s)")
	ErrGateApprovals = errors.New("publication requires approvals from two distinct verified scholars")
	ErrGateCitations = errors.New("publication requires at least one citation")
)

var noteKinds = map[string]bool{
	"context": true, "linguistic": true, "historical": true,
	"interpretive": true, "textual": true,
}

type CitationDraft struct {
	Citation string
	URL      string
}

type NoteDraft struct {
	Title       string
	Body        string
	Kind        string
	ChapterRef  string
	AnchorQuote string
	Citations   []CitationDraft
}

func (d NoteDraft) Validate() error {
	t := strings.TrimSpace(d.Title)
	if n := utf8.RuneCountInString(t); n < 3 || n > 200 {
		return ErrNoteTitle
	}
	b := strings.TrimSpace(d.Body)
	if utf8.RuneCountInString(b) < 200 {
		return ErrNoteBody
	}
	if isFiller(b) {
		return ErrLowEffortBody
	}
	if !noteKinds[d.Kind] {
		return ErrNoteKind
	}
	if len(d.Citations) < 1 {
		return ErrNoteCitations
	}
	for _, c := range d.Citations {
		if utf8.RuneCountInString(strings.TrimSpace(c.Citation)) < 10 {
			return ErrCitation
		}
		if u := strings.TrimSpace(c.URL); u != "" {
			p, err := url.Parse(u)
			if err != nil || (p.Scheme != "http" && p.Scheme != "https") || p.Host == "" {
				return ErrCitationURL
			}
		}
	}
	return nil
}

// PublishGate is the peer-review rule, stated once: two distinct verified
// approvals and at least one citation. The distinctness is enforced by the
// primary key on note_reviews and by excluding the author in the count query;
// this function states the thresholds so both the store and the tests agree.
func PublishGate(approvals, citations int64) error {
	if citations < 1 {
		return ErrGateCitations
	}
	if approvals < 2 {
		return ErrGateApprovals
	}
	return nil
}

// ProfileDraft validates a scholar-profile application. Verification itself is
// a human act (moderator + ORCID/institutional check); this only keeps the
// application legible.
type ProfileDraft struct {
	Field        string
	Affiliation  string
	Orcid        string
	CoiStatement string
}

var (
	ErrScholarField = errors.New("state the field you work in (3–120 characters)")
	ErrScholarCOI   = errors.New("a conflict-of-interest statement is required — including 'none'")
	ErrOrcid        = errors.New("ORCID iDs look like 0000-0002-1825-0097")
)

func (p ProfileDraft) Validate() error {
	if n := utf8.RuneCountInString(strings.TrimSpace(p.Field)); n < 3 || n > 120 {
		return ErrScholarField
	}
	if utf8.RuneCountInString(strings.TrimSpace(p.CoiStatement)) < 3 {
		return ErrScholarCOI
	}
	if o := strings.TrimSpace(p.Orcid); o != "" && !orcidRe.MatchString(o) {
		return ErrOrcid
	}
	return nil
}

// orcidRe accepts the 16-digit hyphenated form with a digit or X check digit.
var orcidRe = regexp.MustCompile(`^\d{4}-\d{4}-\d{4}-\d{3}[\dX]$`)
