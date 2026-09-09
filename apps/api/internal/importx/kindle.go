package importx

import (
	"strings"
)

// ParseKindleClippings reads the "My Clippings.txt" shape Amazon's devices and
// the "Read on Kindle" notebook export produce:
//
//	Title (Author)
//	- Your Highlight at location 123-145 | Added on Monday, 1 January 2024 12:00:00
//
//	The clipped passage text, possibly across lines.
//	==========
//
// This is a user-controlled export: we never touch Amazon's clients or cloud.
// Locations are device-specific and meaningless to us, so alignment happens
// later against the cached public-domain text (Align), and clippings for books
// we cannot align are reported, not guessed.
func ParseKindleClippings(text string) *Plan {
	p := &Plan{Source: "kindle"}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	blocks := strings.Split(text, "==========")
	for i, b := range blocks {
		b = strings.TrimSpace(b)
		if b == "" {
			continue
		}
		lines := strings.SplitN(b, "\n", 3)
		if len(lines) < 2 {
			p.skip(i+1, "malformed_block", firstN(b, 40))
			continue
		}
		titleLine := strings.TrimSpace(lines[0])
		title, author := splitTitleAuthor(titleLine)
		meta := strings.TrimSpace(lines[1])
		kind := "highlight"
		switch {
		case strings.Contains(meta, "Your Note"):
			kind = "note"
		case strings.Contains(meta, "Bookmark"):
			kind = "bookmark"
		}
		passage := ""
		if len(lines) > 2 {
			passage = strings.TrimSpace(lines[2])
		}
		if title == "" || passage == "" {
			p.skip(i+1, "incomplete_clipping", firstN(titleLine, 40))
			continue
		}
		added := ""
		if idx := strings.Index(meta, "Added on"); idx >= 0 {
			added = strings.TrimSpace(meta[idx+len("Added on"):])
		}
		p.Highlights = append(p.Highlights, HighlightPlan{
			BookTitle: title, Author: author, Passage: passage, Kind: kind, AddedOn: added,
		})
	}
	return p
}

// splitTitleAuthor splits "The Odyssey (Homer)" → ("The Odyssey", "Homer").
func splitTitleAuthor(line string) (title, author string) {
	open := strings.LastIndex(line, "(")
	if open > 0 && strings.HasSuffix(strings.TrimSpace(line), ")") {
		return strings.TrimSpace(line[:open]), strings.TrimSpace(line[open+1 : len(strings.TrimSpace(line))-1])
	}
	return strings.TrimSpace(line), ""
}

func firstN(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
