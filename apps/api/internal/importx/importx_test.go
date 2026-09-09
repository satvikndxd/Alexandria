package importx

import (
	"strings"
	"testing"
)

const goodreadsCSV = `Book Id,Title,Author,ISBN13,My Rating,Exclusive Shelf,Date Read,My Review,Spoiler
1,Pride and Prejudice,"Austen, Jane","=""9780141439518""",5,read,2023/05/12,"Austen's irony is a scalpel disguised as embroidery: every sentence that appears to praise a character is quietly measuring them against the society that produced them, and the measurement is never flattering to the measurer.",false
2,Moby-Dick,"Melville, Herman","=""9780142437247""",4,currently-reading,,
3,The Odyssey,Homer,,3,to-read,,
4,Emma,"Austen, Jane",,2,read,,"too slow for me",false
`

func TestParseGoodreads(t *testing.T) {
	p, err := ParseGoodreads(strings.NewReader(goodreadsCSV))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(p.Shelves) != 4 {
		t.Fatalf("shelves = %d, want 4", len(p.Shelves))
	}
	first := p.Shelves[0]
	if first.Work.ISBN13 != "9780141439518" {
		t.Errorf("ISBN13 not cleaned from spreadsheet quoting: %q", first.Work.ISBN13)
	}
	if first.ShelfKind != "read" || first.RatingHalf != 10 {
		t.Errorf("first shelf = %+v", first)
	}
	if first.FinishedOn == nil || first.FinishedOn.Year() != 2023 {
		t.Errorf("finished date not parsed: %v", first.FinishedOn)
	}
	if p.Shelves[1].ShelfKind != "reading" || p.Shelves[2].ShelfKind != "want_to_read" {
		t.Errorf("status mapping wrong: %+v", p.Shelves)
	}
	// One qualifying review; the four-word one stays a rating, and says so.
	if len(p.Reviews) != 1 {
		t.Fatalf("reviews = %d, want 1", len(p.Reviews))
	}
	if len(p.Skips) != 1 || p.Skips[0].Reason != "review_below_minimum" {
		t.Fatalf("skips = %+v, want one review_below_minimum", p.Skips)
	}
}

const storygraphCSV = `Title,Author,Read status,Dates read,Rating,Review
The Brother Karamazov,Fyodor Dostoevsky,Read,2024/01/02→2024/03/01,4.5,
Crime and Punishment,Fyodor Dostoevsky,Abandoned,2024/04/01,,
Notes from Underground,Fyodor Dostoevsky,Currently reading,,,
`

func TestParseStoryGraph(t *testing.T) {
	p, err := ParseStoryGraph(strings.NewReader(storygraphCSV))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(p.Shelves) != 3 {
		t.Fatalf("shelves = %d, want 3", len(p.Shelves))
	}
	if p.Shelves[0].ShelfKind != "read" || p.Shelves[0].RatingHalf != 9 {
		t.Errorf("read+4.5 mapped wrong: %+v", p.Shelves[0])
	}
	if p.Shelves[0].StartedOn == nil || p.Shelves[0].FinishedOn == nil ||
		p.Shelves[0].StartedOn.Month() != 1 || p.Shelves[0].FinishedOn.Month() != 3 {
		t.Errorf("date range not split: %+v", p.Shelves[0])
	}
	if p.Shelves[1].ShelfKind != "dnf" || p.Shelves[2].ShelfKind != "reading" {
		t.Errorf("status vocabulary wrong: %+v", p.Shelves)
	}
}

const clippings = `Meditations (Marcus Aurelius)
- Your Highlight at location 100-120 | Added on Monday, 1 January 2024 12:00:00

You have power over your mind — not
outside events. Realize this, and you will find strength.
==========
Meditations (Marcus Aurelius)
- Your Note at location 200 | Added on Tuesday, 2 January 2024 12:00:00

Compare with the Epictetus passage on the dichotomy of control.
==========
`

func TestParseKindleClippings(t *testing.T) {
	p := ParseKindleClippings(clippings)
	if len(p.Highlights) != 2 {
		t.Fatalf("highlights = %d, want 2", len(p.Highlights))
	}
	h := p.Highlights[0]
	if h.BookTitle != "Meditations" || h.Author != "Marcus Aurelius" {
		t.Errorf("title/author split wrong: %+v", h)
	}
	if h.Kind != "highlight" || p.Highlights[1].Kind != "note" {
		t.Errorf("kinds wrong: %+v", p.Highlights)
	}
	if !strings.Contains(h.Passage, "power over your mind") {
		t.Errorf("passage lost: %q", h.Passage)
	}
}

func TestAlignFindsExactOffsetsAcrossWhitespace(t *testing.T) {
	chapter := "Chapter I.\n\nYou have power over your mind — not outside events.   Realize this, and you will find strength.\n"
	chapters := []Chapter{{Index: 0, Text: chapter}}
	a, ok := Align(chapters, "You have power over your mind — not\noutside events.")
	if !ok {
		t.Fatal("alignment not found across a line break")
	}
	runes := []rune(chapter)
	if string(runes[a.Start:a.End]) != "You have power over your mind — not outside events." {
		t.Fatalf("aligned slice wrong: %q", string(runes[a.Start:a.End]))
	}
	if _, ok := Align(chapters, "a passage that is not in the text"); ok {
		t.Error("Align must not fuzzy-match")
	}
}

func TestNormalizeAndISBN(t *testing.T) {
	if NormalizeTitle("The Picture of Dorian Gray!") != "picture of dorian gray" {
		t.Errorf("normalize: %q", NormalizeTitle("The Picture of Dorian Gray!"))
	}
	if CleanISBN13(`="978-0141439518"`) != "9780141439518" {
		t.Errorf("isbn: %q", CleanISBN13(`="978-0141439518"`))
	}
	if CleanISBN13("014044114X") != "" {
		t.Error("ISBN-10 must not be guessed into an ISBN-13 match")
	}
}
