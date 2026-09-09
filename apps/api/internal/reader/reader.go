// Package reader turns a Project Gutenberg etext into a typeset edition:
// fetch-once caching, boilerplate separation, chapterization, and stable
// anchors for annotations.
//
// Anchors are (chapter_idx, start_off, end_off) measured in RUNES over the
// normalized chapter text the API serves. Because the server — not the DOM —
// defines the string, a highlight survives reflow, theme changes, and font
// changes; it breaks only if the text itself is re-ingested differently, which
// the cache's immutability prevents.
package reader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// politeUserAgent identifies every automated fetch, per Gutenberg's
// automated-access guidance.
const politeUserAgent = "AlexandriaReader/0.3 (+https://github.com/alexandria-reads/alexandria; open-source reading platform; contact: infra@alexandria.example)"

// TextURLPattern is the canonical UTF-8 text location, verified 2026-09-09.
const TextURLPattern = "https://www.gutenberg.org/files/%d/%d-0.txt"

// Chapter is a slice of the normalized body.
type Chapter struct {
	Index int    `json:"index"`
	Title string `json:"title"`
	Start int    `json:"start"` // rune offset into Body
	End   int    `json:"end"`   // rune offset into Body, exclusive
}

// Edition is a normalized, chapterized text plus the notices that must travel
// with it.
type Edition struct {
	GutenbergID  int32      `json:"gutenberg_id"`
	Title        string     `json:"title"`
	Language     string     `json:"language"`
	Body         string     `json:"body"` // normalized reading text
	Boilerplate  string     `json:"-"`    // PG header/license: stored, never typeset
	LicenseNote  string     `json:"license_note"`
	TrademarkNote string    `json:"trademark_note"`
	Chapters     []Chapter  `json:"chapters"`
	// Source records where the reading text came from: gutenberg or epub.
	Source string `json:"source,omitempty"`
}

// ---- fetching: cache-once, never hotlink -----------------------------------------

// Cache resolves a text from local object storage, downloading it once from
// Gutenberg if absent. Production swaps the directory for MinIO; the interface
// is the same because the semantics are: immutable object, keyed by etext id.
type Cache struct {
	Dir    string
	Client *http.Client
}

func NewCache(dir string) *Cache {
	return &Cache{Dir: dir, Client: &http.Client{Timeout: 60 * time.Second}}
}

func (c *Cache) path(id int32) string {
	return filepath.Join(c.Dir, fmt.Sprintf("%d.txt", id))
}

// Text returns the raw bytes for an etext, downloading at most once per cache.
func (c *Cache) Text(ctx context.Context, id int32) ([]byte, error) {
	if c.Dir != "" {
		if b, err := os.ReadFile(c.path(id)); err == nil {
			return b, nil
		}
	}
	url := fmt.Sprintf(TextURLPattern, id, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", politeUserAgent)
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch etext %d: %w", id, err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("etext %d: upstream status %d", id, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	if c.Dir != "" {
		if err := os.MkdirAll(c.Dir, 0o755); err == nil {
			_ = os.WriteFile(c.path(id), b, 0o644)
		}
	}
	return b, nil
}

// ---- normalization: the reading text is not the license text ----------------------

var (
	boilerStart = regexp.MustCompile(`(?is)\*\s*\*\s*\*\s*START OF (THIS|THE) PROJECT GUTENBERG[^\n]*`)
	boilerEnd   = regexp.MustCompile(`(?is)\*\s*\*\s*\*\s*END OF (THIS|THE) PROJECT GUTENBERG[^\n]*`)
	crlf        = regexp.MustCompile(`\r\n?`)
	blankRuns   = regexp.MustCompile(`\n{3,}`)
)

// Normalize separates Gutenberg's boilerplate (header, license) from the
// reading text. The boilerplate is preserved on the Edition — the trademark
// notice must never be lost — but is never typeset into the reader.
func Normalize(raw string) (body, boilerplate, title string) {
	raw = crlf.ReplaceAllString(raw, "\n")
	boilerplate = ""
	if m := boilerStart.FindStringIndex(raw); m != nil {
		head := raw[:m[0]]
		rest := raw[m[1]:]
		if e := boilerEnd.FindStringIndex(rest); e != nil {
			body = rest[:e[0]]
			boilerplate = head + raw[m[0]:m[1]] + rest[e[1]:]
		} else {
			body = rest
			boilerplate = head
		}
	} else {
		body = raw
	}
	title = firstLine(body)
	body = blankRuns.ReplaceAllString(strings.TrimSpace(body), "\n\n")
	return body, boilerplate, title
}

func firstLine(s string) string {
	s = strings.TrimLeft(s, "\n")
	if i := strings.IndexByte(s, '\n'); i > 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

// ---- chapterization ------------------------------------------------------------------

var chapterLine = regexp.MustCompile(`(?m)^(?:CHAPTER|Chapter|BOOK|Book|PART|Part|CANTO|Canto|SECTION|Section)\b[^\n]{0,80}$`)

// Chapterize splits the body on conventional chapter headings. Texts without
// recognisable headings become a single chapter titled after the work, so the
// reader and the annotation anchors always have somewhere to live.
func Chapterize(body string) []Chapter {
	runes := []rune(body)
	locs := chapterLine.FindAllStringIndex(body, -1)
	var chapters []Chapter
	if len(locs) == 0 || len(locs) > 2000 {
		return []Chapter{{Index: 0, Title: "Complete text", Start: 0, End: len(runes)}}
	}
	// Preamble before the first heading, if any.
	if locs[0][0] > 0 && strings.TrimSpace(body[:locs[0][0]]) != "" {
		chapters = append(chapters, Chapter{Index: 0, Title: "Preamble", Start: 0, End: runeIndexOf(body, locs[0][0])})
	}
	for i, l := range locs {
		start := runeIndexOf(body, l[0])
		end := len(runes)
		if i+1 < len(locs) {
			end = runeIndexOf(body, locs[i+1][0])
		}
		chapters = append(chapters, Chapter{
			Index: len(chapters),
			Title: strings.TrimSpace(body[l[0]:l[1]]),
			Start: start,
			End:   end,
		})
	}
	return chapters
}

// runeIndexOf converts a byte offset to a rune offset. Anchors are runes so
// non-ASCII texts annotate correctly.
func runeIndexOf(s string, byteOff int) int {
	return len([]rune(s[:byteOff]))
}

// Build assembles an Edition from raw etext bytes.
func Build(gutenbergID int32, language string, raw []byte) *Edition {
	body, boiler, title := Normalize(string(raw))
	return &Edition{
		GutenbergID: gutenbergID,
		Title:       title,
		Language:    language,
		Body:        body,
		Boilerplate: boiler,
		LicenseNote: "Project Gutenberg™ eBook. Released under the Project Gutenberg License; public domain in the United States. Copyright status varies by jurisdiction.",
		TrademarkNote: "Project Gutenberg is a trademark of the Project Gutenberg Literary Archive Foundation. This edition caches the official text; it does not imply endorsement.",
		Chapters:    Chapterize(body),
	}
}

// ChapterText returns the rune-slice for a chapter index.
func (e *Edition) ChapterText(idx int) (string, bool) {
	for _, c := range e.Chapters {
		if c.Index == idx {
			runes := []rune(e.Body)
			if c.Start < 0 || c.End > len(runes) || c.Start >= c.End {
				return "", false
			}
			return string(runes[c.Start:c.End]), true
		}
	}
	return "", false
}
