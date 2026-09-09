package epub

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

// buildEpub assembles a minimal but conformant EPUB 3 in memory: mimetype
// entry, container, OPF with a nav document and two prose chapters, one of
// which wraps paragraphs in a div and uses both numeric and named entities.
func buildEpub(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	write := func(name, body string) {
		f, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	write("mimetype", "application/epub+zip")
	write("META-INF/container.xml", `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`)
	write("OEBPS/content.opf", `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" xml:lang="en">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Synthetic Classics</dc:title>
    <dc:language>en</dc:language>
    <dc:creator>Someone, Some</dc:creator>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="c1" href="text/c1.xhtml" media-type="application/xhtml+xml"/>
    <item id="c2" href="text/c2.xhtml" media-type="application/xhtml+xml"/>
    <item id="css" href="style.css" media-type="text/css"/>
  </manifest>
  <spine>
    <itemref idref="nav"/>
    <itemref idref="c1"/>
    <itemref idref="c2"/>
  </spine>
</package>`)
	write("OEBPS/nav.xhtml", `<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml"><body><nav><ol><li><a href="text/c1.xhtml">I</a></li></ol></nav></body></html>`)
	write("OEBPS/text/c1.xhtml", `<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml"><body>
<h2>I. The Beginning</h2>
<div>
  <p>First paragraph &#8212; with an em dash entity.</p>
  <p>Second paragraph, <em>emphasised</em> &amp; folded inline.</p>
</div>
</body></html>`)
	write("OEBPS/text/c2.xhtml", `<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml"><body>
<h2>II. The Middle</h2>
<p>A single block with &mdash; a named entity.</p>
</body></html>`)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestParseSyntheticEpub(t *testing.T) {
	book, err := Parse(buildEpub(t))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if book.Title != "Synthetic Classics" || book.Language != "en" {
		t.Fatalf("metadata wrong: %+v", book)
	}
	if len(book.Authors) != 1 || book.Authors[0] != "Someone, Some" {
		t.Fatalf("authors wrong: %+v", book.Authors)
	}
	// The nav document must not become a chapter.
	if len(book.Chapters) != 2 {
		t.Fatalf("chapters = %d, want 2 (nav excluded): %+v", len(book.Chapters), book.Chapters)
	}
	c1 := book.Chapters[0]
	if c1.Title != "I. The Beginning" {
		t.Fatalf("chapter title = %q", c1.Title)
	}
	// Heading + a div wrapping two paragraphs = three blocks; the div must
	// not merge its paragraphs into one blob.
	if len(c1.Blocks) != 3 {
		t.Fatalf("blocks = %d, want 3: %q", len(c1.Blocks), c1.Blocks)
	}
	if !strings.Contains(c1.Blocks[1], "— with an em dash entity") {
		t.Fatalf("numeric entity not decoded: %q", c1.Blocks[1])
	}
	if !strings.Contains(c1.Blocks[2], "emphasised & folded inline") {
		t.Fatalf("inline markup not folded: %q", c1.Blocks[2])
	}
	if len(book.Chapters[1].Blocks) != 2 || !strings.Contains(book.Chapters[1].Blocks[1], "— a named entity") {
		t.Fatalf("named entity not decoded: %q", book.Chapters[1].Blocks)
	}
	// The served text is exactly what the reader splits on.
	if strings.Count(c1.Text(), "\n\n") != len(c1.Blocks)-1 {
		t.Fatalf("Text() join wrong: %q", c1.Text())
	}
}

func TestParseRejectsNonEpub(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, _ := zw.Create("hello.txt")
	_, _ = f.Write([]byte("not an epub"))
	_ = zw.Close()
	if _, err := Parse(buf.Bytes()); err != ErrNotEpub {
		t.Fatalf("err = %v, want ErrNotEpub", err)
	}
	if _, err := Parse([]byte("garbage")); err == nil {
		t.Fatal("garbage parsed as an EPUB")
	}
}
