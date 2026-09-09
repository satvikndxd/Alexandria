package reader

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"

	"github.com/alexandria-reads/alexandria/apps/api/internal/epub"
)

// EPUB support: editions may carry an uploaded EPUB container (moderator-
// approved, rights-checked at upload) in preference to the Gutenberg text.
// Containers are immutable once stored, which is what keeps annotation
// offsets meaningful across sessions.

var (
	epubMu    sync.Mutex
	epubCache = map[uuid.UUID]*epub.Book{}
)

func (c *Cache) epubPath(editionID uuid.UUID) string {
	return filepath.Join(c.Dir, "epub", editionID.String()+".epub")
}

// HasEpub reports whether a container is stored for this edition.
func (c *Cache) HasEpub(editionID uuid.UUID) bool {
	if c.Dir == "" {
		return false
	}
	_, err := os.Stat(c.epubPath(editionID))
	return err == nil
}

// StoreEpub validates then writes a container. Parse-before-write means an
// unreadable file can never land in the cache and 404 every reader later.
func (c *Cache) StoreEpub(editionID uuid.UUID, data []byte) (*epub.Book, error) {
	book, err := epub.Parse(data)
	if err != nil {
		return nil, err
	}
	if c.Dir == "" {
		return book, nil
	}
	p := c.epubPath(editionID)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		return nil, err
	}
	epubMu.Lock()
	epubCache[editionID] = book
	epubMu.Unlock()
	return book, nil
}

// EpubBook returns the parsed container, from memory when warm.
func (c *Cache) EpubBook(ctx context.Context, editionID uuid.UUID) (*epub.Book, error) {
	epubMu.Lock()
	if b, ok := epubCache[editionID]; ok {
		epubMu.Unlock()
		return b, nil
	}
	epubMu.Unlock()
	if c.Dir == "" {
		return nil, fmt.Errorf("no epub cache configured")
	}
	data, err := os.ReadFile(c.epubPath(editionID))
	if err != nil {
		return nil, err
	}
	book, err := epub.Parse(data)
	if err != nil {
		return nil, err
	}
	epubMu.Lock()
	epubCache[editionID] = book
	epubMu.Unlock()
	return book, nil
}

// EditionFromEpub maps a container onto the reader's Edition shape: chapters
// in spine order, with rune offsets accumulated over a single Body so the
// existing anchor model (chapter_idx + offsets into the chapter string) is
// unchanged for EPUB readers.
func EditionFromEpub(editionID uuid.UUID, language string, book *epub.Book) *Edition {
	ed := &Edition{
		GutenbergID: 0,
		Title:       book.Title,
		Language:    language,
		LicenseNote: "This edition was supplied as an EPUB container. Rights are the uploader's responsibility and are recorded at upload; public-domain status is not implied by the format.",
		TrademarkNote: "",
		Source:      "epub",
	}
	runes := 0
	for i, ch := range book.Chapters {
		text := ch.Text()
		n := len([]rune(text))
		ed.Chapters = append(ed.Chapters, Chapter{
			Index: i, Title: ch.Title, Start: runes, End: runes + n,
		})
		if i > 0 {
			ed.Body += "\n\n"
			runes += 2
		}
		ed.Body += text
		runes += n
	}
	// ChapterText slices Body by these offsets; keep them consistent by
	// re-deriving per-chapter text from Body in ChapterText (unchanged).
	_ = editionID
	return ed
}
