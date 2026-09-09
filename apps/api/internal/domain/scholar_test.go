package domain

import (
	"errors"
	"strings"
	"testing"
)

func goodNote() NoteDraft {
	return NoteDraft{
		Title: "Virgil as shade and as guide",
		Body: strings.Repeat("The choice of Virgil to lead the pilgrim is not merely literary homage; it positions imperial Rome as the necessary prelude to the Christian journey. ", 2),
		Kind:  "context",
		Citations: []CitationDraft{
			{Citation: "Ziolkowski, John. The Medieval Virgil. 1991.", URL: "https://example.org/z"},
		},
	}
}

func TestNoteDraftValidate(t *testing.T) {
	if err := goodNote().Validate(); err != nil {
		t.Fatalf("good note rejected: %v", err)
	}
	short := goodNote()
	short.Body = "Too brief to be scholarship."
	if err := short.Validate(); !errors.Is(err, ErrNoteBody) {
		t.Errorf("short body = %v, want ErrNoteBody", err)
	}
	noCite := goodNote()
	noCite.Citations = nil
	if err := noCite.Validate(); !errors.Is(err, ErrNoteCitations) {
		t.Errorf("no citations = %v, want ErrNoteCitations", err)
	}
	thinCite := goodNote()
	thinCite.Citations = []CitationDraft{{Citation: "ibid."}}
	if err := thinCite.Validate(); !errors.Is(err, ErrCitation) {
		t.Errorf("thin citation = %v, want ErrCitation", err)
	}
	badURL := goodNote()
	badURL.Citations = []CitationDraft{{Citation: "A perfectly good citation line.", URL: "javascript:alert(1)"}}
	if err := badURL.Validate(); !errors.Is(err, ErrCitationURL) {
		t.Errorf("bad url = %v, want ErrCitationURL", err)
	}
	badKind := goodNote()
	badKind.Kind = "vibes"
	if err := badKind.Validate(); !errors.Is(err, ErrNoteKind) {
		t.Errorf("bad kind = %v, want ErrNoteKind", err)
	}
	filler := goodNote()
	filler.Body = strings.Repeat("x ", 200)
	if err := filler.Validate(); !errors.Is(err, ErrLowEffortBody) {
		t.Errorf("filler body = %v, want ErrLowEffortBody", err)
	}
}

func TestPublishGate(t *testing.T) {
	if err := PublishGate(2, 1); err != nil {
		t.Errorf("2 approvals + 1 citation should publish, got %v", err)
	}
	if err := PublishGate(1, 1); !errors.Is(err, ErrGateApprovals) {
		t.Errorf("1 approval = %v, want ErrGateApprovals", err)
	}
	if err := PublishGate(2, 0); !errors.Is(err, ErrGateCitations) {
		t.Errorf("0 citations = %v, want ErrGateCitations", err)
	}
	if err := PublishGate(3, 0); !errors.Is(err, ErrGateCitations) {
		t.Error("citations must gate before approvals")
	}
}

func TestProfileDraftValidate(t *testing.T) {
	ok := ProfileDraft{Field: "Classical Literature", CoiStatement: "none", Orcid: "0000-0002-1825-0097"}
	if err := ok.Validate(); err != nil {
		t.Fatalf("good profile rejected: %v", err)
	}
	bad := ProfileDraft{Field: "Classical Literature", CoiStatement: "none", Orcid: "1234"}
	if err := bad.Validate(); !errors.Is(err, ErrOrcid) {
		t.Errorf("bad orcid = %v, want ErrOrcid", err)
	}
	noCOI := ProfileDraft{Field: "Classical Literature", CoiStatement: ""}
	if err := noCOI.Validate(); !errors.Is(err, ErrScholarCOI) {
		t.Errorf("missing COI = %v, want ErrScholarCOI", err)
	}
}
