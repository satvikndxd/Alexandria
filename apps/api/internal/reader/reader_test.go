package reader

import (
	"strings"
	"testing"
)

const sample = "Project Gutenberg's Test, by Anonymous\r\n" +
	"\r\n" +
	"Produced by Nobody\r\n" +
	"\r\n" +
	"*** START OF THIS PROJECT GUTENBERG EBOOK TEST ***\r\n" +
	"\r\n" +
	"\r\n" +
	"\r\n" +
	"\r\n" +
	"TEST\r\n" +
	"\r\n" +
	"CHAPTER I\r\n" +
	"\r\n" +
	"The beginning, with café and naïve non-ASCII words.\r\n" +
	"\r\n" +
	"\r\n" +
	"\r\n" +
	"CHAPTER II\r\n" +
	"\r\n" +
	"The middle.\r\n" +
	"\r\n" +
	"*** END OF THIS PROJECT GUTENBERG EBOOK TEST ***\r\n" +
	"License terms live here and must never be lost.\r\n"

func TestNormalizeSeparatesBoilerplate(t *testing.T) {
	body, boiler, title := Normalize(sample)
	if strings.Contains(body, "START OF THIS PROJECT GUTENBERG") {
		t.Error("reading text still contains the PG header")
	}
	if !strings.Contains(boiler, "License terms live here") {
		t.Error("license boilerplate was lost instead of preserved")
	}
	if strings.Contains(body, "\r") {
		t.Error("CRLF not normalized")
	}
	if strings.Contains(body, "\n\n\n") {
		t.Error("blank runs not collapsed")
	}
	if title != "TEST" {
		t.Errorf("title = %q, want TEST", title)
	}
	if !strings.Contains(body, "CHAPTER I") {
		t.Error("body lost its chapters")
	}
}

func TestChapterizeFindsHeadingsAndRuneOffsets(t *testing.T) {
	body, _, _ := Normalize(sample)
	chapters := Chapterize(body)
	// The title page before the first heading becomes a "Preamble" chapter:
	// that is intended, so anchors always have somewhere to live.
	if len(chapters) != 3 {
		t.Fatalf("chapters = %d, want 3 (%+v)", len(chapters), chapters)
	}
	if chapters[0].Title != "Preamble" || chapters[1].Title != "CHAPTER I" || chapters[2].Title != "CHAPTER II" {
		t.Fatalf("titles = %q, %q, %q", chapters[0].Title, chapters[1].Title, chapters[2].Title)
	}
	ed := &Edition{Body: body, Chapters: chapters}
	first, ok := ed.ChapterText(1)
	if !ok {
		t.Fatal("chapter 1 missing")
	}
	if !strings.Contains(first, "café") || strings.Contains(first, "The middle.") {
		t.Errorf("chapter slice wrong: %q", first)
	}
	// Rune offsets must address runes, not bytes: the first chapter contains
	// multi-byte characters, so a byte-based slice would drift.
	runes := []rune(body)
	if string(runes[chapters[1].Start:chapters[1].End]) != first {
		t.Error("rune offsets do not reproduce the chapter slice")
	}
}

func TestChapterizeFallsBackToSingleChapter(t *testing.T) {
	ch := Chapterize("A text with no headings at all.\n\nJust prose.")
	if len(ch) != 1 || ch[0].Start != 0 || ch[0].End != len([]rune("A text with no headings at all.\n\nJust prose.")) {
		t.Fatalf("fallback chapter wrong: %+v", ch)
	}
}

func TestBuildCarriesNotices(t *testing.T) {
	ed := Build(42, "en", []byte(sample))
	if ed.GutenbergID != 42 || ed.TrademarkNote == "" || ed.LicenseNote == "" {
		t.Fatal("edition must carry its license and trademark notices")
	}
	if ed.Boilerplate == "" {
		t.Fatal("boilerplate must be preserved on the edition")
	}
}
