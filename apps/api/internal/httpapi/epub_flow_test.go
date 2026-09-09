package httpapi_test

import (
	"archive/zip"
	"bytes"
	"net/http"
	"strings"
	"testing"
)

// miniEpub builds a two-chapter EPUB in memory for the upload flow.
func miniEpub(t *testing.T) []byte {
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
    <dc:title>Uploaded Edition</dc:title><dc:language>en</dc:language>
  </metadata>
  <manifest>
    <item id="c1" href="c1.xhtml" media-type="application/xhtml+xml"/>
    <item id="c2" href="c2.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="c1"/><itemref idref="c2"/></spine>
</package>`)
	write("OEBPS/c1.xhtml", `<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml"><body><h2>One</h2><p>It was a bright cold day in April, and the clocks were striking thirteen.</p></body></html>`)
	write("OEBPS/c2.xhtml", `<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml"><body><h2>Two</h2><p>The corridor smelt of boiled cabbage and old rag mats.</p></body></html>`)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestEpubUploadAndRead proves the container path: moderator-only upload,
// parse-before-store, EPUB preferred over Gutenberg text, and anchors still
// rune-true over the served chapter string.
func TestEpubUploadAndRead(t *testing.T) {
	e := newEnv(t)
	w := e.seedWork("Nineteen Eighty-Four")

	// An edition with no Gutenberg id: only the EPUB can make it readable.
	var editionID string
	if err := e.ownerPool.QueryRow(e.ctx, `
		INSERT INTO editions (work_id, title, format, language)
		VALUES ($1, 'Nineteen Eighty-Four (uploaded EPUB)', 'ebook', 'en')
		RETURNING id`, w.ID).Scan(&editionID); err != nil {
		t.Fatal(err)
	}

	readerC, readerCSRF := e.signIn("epub_reader", "epubreader@example.com")

	// Not readable yet.
	resp := e.get(readerC, "/v1/editions/"+editionID+"/reader")
	mustStatus(t, resp, http.StatusNotFound)

	// A non-moderator cannot upload containers.
	resp = e.sendRaw(readerC, http.MethodPost, "/v1/editions/"+editionID+"/epub",
		string(miniEpub(t)), readerCSRF)
	mustStatus(t, resp, http.StatusForbidden)

	// A moderator can; garbage is refused before it reaches the cache.
	if _, err := e.ownerPool.Exec(e.ctx,
		`UPDATE users SET role='admin' WHERE username='epub_reader'`); err != nil {
		t.Fatal(err)
	}
	resp = e.sendRaw(readerC, http.MethodPost, "/v1/editions/"+editionID+"/epub",
		"not an epub at all", readerCSRF)
	mustStatus(t, resp, http.StatusUnprocessableEntity)

	resp = e.sendRawBytes(readerC, http.MethodPost, "/v1/editions/"+editionID+"/epub",
		miniEpub(t), readerCSRF)
	mustStatus(t, resp, http.StatusCreated)

	// The reader now serves the EPUB, in spine order, labelled as such.
	resp = e.get(readerC, "/v1/editions/"+editionID+"/reader")
	meta := mustStatus(t, resp, http.StatusOK)
	if meta["source"] != "epub" {
		t.Fatalf("source = %v, want epub", meta["source"])
	}
	chapters := meta["chapters"].([]any)
	if len(chapters) != 2 {
		t.Fatalf("chapters = %d, want 2", len(chapters))
	}
	resp = e.get(readerC, "/v1/editions/"+editionID+"/reader/chapter/0")
	ch := mustStatus(t, resp, http.StatusOK)
	text, _ := ch["text"].(string)
	if !strings.Contains(text, "clocks were striking thirteen") {
		t.Fatalf("chapter text wrong: %q", text)
	}

	// Anchors remain rune-true over the served string.
	resp = e.send(readerC, http.MethodPost, "/v1/me/annotations", map[string]any{
		"edition_id": editionID, "chapter_idx": 0,
		"start_off": 0, "end_off": len([]rune(strings.Split(text, "\n\n")[1])),
		"kind": "highlight", "body": "the famous opening",
	}, readerCSRF)
	mustStatus(t, resp, http.StatusCreated)
	resp = e.get(readerC, "/v1/editions/"+editionID+"/reader/chapter/0")
	if !strings.Contains(mustJSON(t, mustStatus(t, resp, http.StatusOK)), "the famous opening") {
		t.Fatal("annotation did not ride with the EPUB chapter")
	}
}
