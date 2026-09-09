package importx

import (
	"strings"
	"unicode/utf8"
)

const minReviewRunes = 150

func runeLen(s string) int { return utf8.RuneCountInString(strings.TrimSpace(s)) }

// Chapter is the slice of normalized text a highlight may align to.
type Chapter struct {
	Index int
	Text  string
}

// CollapseInfo maps a whitespace-collapsed string back to rune offsets in the
// original, so an alignment found in the collapsed space is exact in the
// served text.
type CollapseInfo struct {
	// OrigStart[i] is the rune offset in the original of collapsed rune i.
	OrigStart []int
}

// Collapse whitespace runs to single spaces, recording the origin of each
// surviving rune.
func Collapse(s string) (string, CollapseInfo) {
	runes := []rune(s)
	var sb strings.Builder
	info := CollapseInfo{OrigStart: make([]int, 0, len(runes))}
	prevSpace := false
	for i, r := range runes {
		if r == '\n' || r == '\t' || r == '\r' || r == ' ' {
			if !prevSpace && sb.Len() > 0 {
				sb.WriteRune(' ')
				info.OrigStart = append(info.OrigStart, i)
			}
			prevSpace = true
			continue
		}
		sb.WriteRune(r)
		info.OrigStart = append(info.OrigStart, i)
		prevSpace = false
	}
	return sb.String(), info
}

// Alignment locates a clipped passage inside one chapter, in rune offsets of
// the chapter's served text.
type Alignment struct {
	ChapterIndex int
	Start        int
	End          int
}

// Align searches each chapter's collapsed text for the collapsed passage.
// It returns the first exact match; fuzzy alignment is deliberately absent,
// because a mis-anchored highlight is worse than an unaligned one.
func Align(chapters []Chapter, passage string) (Alignment, bool) {
	needle, _ := Collapse(passage)
	needle = strings.TrimSpace(needle)
	if needle == "" {
		return Alignment{}, false
	}
	for _, ch := range chapters {
		collapsed, info := Collapse(ch.Text)
		idx := strings.Index(collapsed, needle)
		if idx < 0 {
			continue
		}
		collapsedRunes := []rune(collapsed)
		startCollapsed := len([]rune(collapsed[:idx]))
		endCollapsed := startCollapsed + len([]rune(needle))
		if startCollapsed >= len(info.OrigStart) || endCollapsed > len(info.OrigStart) {
			continue
		}
		start := info.OrigStart[startCollapsed]
		end := len([]rune(ch.Text))
		if endCollapsed < len(info.OrigStart) {
			end = info.OrigStart[endCollapsed]
		}
		_ = collapsedRunes
		return Alignment{ChapterIndex: ch.Index, Start: start, End: end}, true
	}
	return Alignment{}, false
}
