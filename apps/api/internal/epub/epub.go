// Package epub parses EPUB 3 (and well-formed EPUB 2) containers into the
// block structure Alexandria's reader typesets.
//
// What we keep and what we drop: the spine order, the chapter documents, and
// their block-level text (paragraphs, headings, list items, blockquotes).
// What we drop: styling, scripts, navigation documents, and inline markup —
// the reader has its own typography system, and a typeset edition is a
// reading surface, not a browser.
//
// Anchors stay reflow-safe because blocks are joined with "\n\n" into the
// chapter string the API serves; annotation offsets are runes over exactly
// that string, so font, leading, theme and screen size cannot move them.
package epub

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
)

var (
	ErrNotEpub    = errors.New("not an EPUB: missing META-INF/container.xml")
	ErrNoRootfile = errors.New("EPUB container declares no rootfile (OPF)")
	ErrEmptySpine = errors.New("EPUB spine has no readable documents")
)

// ChapterDoc is one spine document, reduced to ordered text blocks.
type ChapterDoc struct {
	ID     string
	Title  string
	Blocks []string
}

// Text joins blocks the way the reader splits them, so served offsets and
// rendered paragraphs are the same string.
func (c ChapterDoc) Text() string { return strings.Join(c.Blocks, "\n\n") }

// Book is the parsed container.
type Book struct {
	Title    string
	Language string
	Authors  []string
	Chapters []ChapterDoc
}

// ———— OPF / container structures ————

type containerXML struct {
	XMLName   xml.Name `xml:"container"`
	Rootfiles struct {
		Rootfile []struct {
			FullPath  string `xml:"full-path,attr"`
			MediaType string `xml:"media-type,attr"`
		} `xml:"rootfile"`
	} `xml:"rootfiles"`
}

type opfXML struct {
	XMLName  xml.Name `xml:"package"`
	Metadata struct {
		Titles    []string `xml:"title"`
		Languages []string `xml:"language"`
		Creators  []struct {
			Value string `xml:",chardata"`
		} `xml:"creator"`
	} `xml:"metadata"`
	Manifest struct {
		Items []struct {
			ID         string `xml:"id,attr"`
			Href       string `xml:"href,attr"`
			MediaType  string `xml:"media-type,attr"`
			Properties string `xml:"properties,attr"`
		} `xml:"item"`
	} `xml:"manifest"`
	Spine struct {
		ItemRefs []struct {
			IDRef string `xml:"idref,attr"`
		} `xml:"itemref"`
	} `xml:"spine"`
}

// Parse reads an EPUB container from memory.
func Parse(data []byte) (*Book, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[path.Clean(f.Name)] = f
	}

	cf, ok := files["META-INF/container.xml"]
	if !ok {
		return nil, ErrNotEpub
	}
	var container containerXML
	if err := readXML(cf, &container); err != nil {
		return nil, fmt.Errorf("container.xml: %w", err)
	}
	opfPath := ""
	for _, rf := range container.Rootfiles.Rootfile {
		if rf.FullPath != "" {
			opfPath = path.Clean(rf.FullPath)
			break
		}
	}
	if opfPath == "" {
		return nil, ErrNoRootfile
	}
	of, ok := files[opfPath]
	if !ok {
		return nil, fmt.Errorf("OPF declared at %s but absent from container", opfPath)
	}
	var opf opfXML
	if err := readXML(of, &opf); err != nil {
		return nil, fmt.Errorf("OPF: %w", err)
	}

	book := &Book{}
	if len(opf.Metadata.Titles) > 0 {
		book.Title = strings.TrimSpace(opf.Metadata.Titles[0])
	}
	if len(opf.Metadata.Languages) > 0 {
		book.Language = strings.TrimSpace(opf.Metadata.Languages[0])
	}
	for _, c := range opf.Metadata.Creators {
		if v := strings.TrimSpace(c.Value); v != "" {
			book.Authors = append(book.Authors, v)
		}
	}

	manifest := map[string]struct {
		href      string
		mediaType string
		isNav     bool
	}{}
	for _, it := range opf.Manifest.Items {
		manifest[it.ID] = struct {
			href      string
			mediaType string
			isNav     bool
		}{
			href:      it.Href,
			mediaType: strings.ToLower(it.MediaType),
			// The navigation document is a table of contents, not prose: it
			// must never become "Chapter 1".
			isNav: strings.Contains(strings.ToLower(it.Properties), "nav"),
		}
	}

	opfDir := path.Dir(opfPath)
	for i, ref := range opf.Spine.ItemRefs {
		item, ok := manifest[ref.IDRef]
		if !ok || item.isNav {
			continue
		}
		if item.mediaType != "application/xhtml+xml" && item.mediaType != "text/html" {
			continue
		}
		// Hrefs are relative to the OPF document's directory.
		docPath := path.Clean(path.Join(opfDir, unescapeHref(item.href)))
		df, ok := files[docPath]
		if !ok {
			continue
		}
		blocks, title, err := extractBlocks(df)
		if err != nil {
			return nil, fmt.Errorf("chapter %s: %w", docPath, err)
		}
		if len(blocks) == 0 {
			continue // navigation pages and front matter with no prose
		}
		if title == "" {
			title = fmt.Sprintf("Chapter %d", i+1)
		}
		book.Chapters = append(book.Chapters, ChapterDoc{ID: ref.IDRef, Title: title, Blocks: blocks})
	}
	if len(book.Chapters) == 0 {
		return nil, ErrEmptySpine
	}
	if book.Title == "" {
		book.Title = "Untitled EPUB"
	}
	return book, nil
}

func unescapeHref(h string) string {
	// Hrefs are URL-escaped within the OPF; container paths are not.
	if u, err := url.PathUnescape(h); err == nil {
		return u
	}
	return h
}

func readXML(f *zip.File, into any) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close() //nolint:errcheck
	dec := xml.NewDecoder(rc)
	// EPUB XHTML is XML; strict parsing is correct and catches corrupt
	// containers early rather than mid-chapter.
	return dec.Decode(into)
}

// blockElements are the block-level tags whose text becomes a paragraph in
// the reader. Everything else (spans, emphasis, images) is inline and folds
// into the enclosing block.
var blockElements = map[string]bool{
	"p": true, "div": true, "h1": true, "h2": true, "h3": true, "h4": true,
	"h5": true, "h6": true, "blockquote": true, "li": true, "section": false,
	"pre": true,
}

var headingElements = map[string]bool{
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
}

// extractBlocks walks the XHTML token stream with a stack of open blocks, so
// a <div> wrapping three <p>s yields three paragraphs, not one merged blob,
// and inline markup (em, span, a) folds into its enclosing block. A token
// stream — not a DOM — keeps memory bounded on 700 KB chapters.
func extractBlocks(f *zip.File) (blocks []string, firstHeading string, err error) {
	rc, err := f.Open()
	if err != nil {
		return nil, "", err
	}
	defer rc.Close() //nolint:errcheck

	dec := xml.NewDecoder(rc)
	dec.Strict = false // tolerate the occasional malformed entity in the wild
	dec.Entity = xml.HTMLEntity

	type frame struct {
		sb  strings.Builder
		tag string
	}
	// Frames live behind pointers: slice growth would otherwise copy a
	// written strings.Builder by value, which the type forbids (panic).
	var stack []*frame
	for {
		tok, terr := dec.Token()
		if terr == io.EOF {
			break
		}
		if terr != nil {
			return nil, "", terr
		}
		switch t := tok.(type) {
		case xml.StartElement:
			name := strings.ToLower(t.Name.Local)
			if blockElements[name] {
				stack = append(stack, &frame{tag: name})
			}
		case xml.EndElement:
			name := strings.ToLower(t.Name.Local)
			if !blockElements[name] || len(stack) == 0 {
				continue
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if top.tag != name {
				continue // mismatched markup: drop the frame, keep going
			}
			text := collapseSpace(top.sb.String())
			if text == "" {
				continue // wrappers and navigation shells produce no block
			}
			blocks = append(blocks, text)
			if firstHeading == "" && headingElements[name] {
				firstHeading = text
			}
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].sb.Write(t)
			}
		}
	}
	return blocks, firstHeading, nil
}

func collapseSpace(s string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(strings.ReplaceAll(s, "\u00a0", " ")), " "))
}
