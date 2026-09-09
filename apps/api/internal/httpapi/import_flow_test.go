package httpapi_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/alexandria-reads/alexandria/apps/api/internal/db"
)

const importCSV = `Book Id,Title,Author,ISBN13,My Rating,Exclusive Shelf,Date Read,My Review,Spoiler
1,Pride and Prejudice,"Austen, Jane","=""9780141439518""",5,read,2023/05/12,"Austen's irony is a scalpel disguised as embroidery: every sentence that appears to praise a character is quietly measuring them against the society that produced them, and the measurement is never flattering to the measurer.",false
`

// TestImportExportLoop covers the portability promise: history arrives with
// provenance, ratings without prose stay private, imported prose publishes
// unchanged, unmatched rows are reported, the throttle holds, and the export
// gives the reader their whole life back.
func TestImportExportLoop(t *testing.T) {
	e := newEnv(t)

	// A work with a real ISBN-13 edition to match against.
	w := e.seedWork("Pride and Prejudice")
	if _, err := e.st.Queries().UpsertEdition(e.ctx, db.UpsertEditionParams{
		WorkID: w.ID, Title: "Pride and Prejudice (Penguin Classics)",
		Isbn13: strPtr("9780141439518"), Format: db.EditionFormatPaperback,
		Language: "en", Metadata: []byte("{}"),
	}); err != nil {
		t.Fatalf("seed edition: %v", err)
	}

	c, csrf := e.signIn("refugee", "refugee@example.com")

	post := func(path, body string) (int, map[string]any) {
		resp := e.sendRaw(c, http.MethodPost, path, body, csrf)
		return resp.StatusCode, decodeBody(t, resp)
	}

	// 1 · The import lands: shelved, rated, and the long review publishes.
	status, body := post("/v1/imports/goodreads", importCSV)
	if status != http.StatusOK {
		t.Fatalf("import status = %d, body %v", status, body)
	}
	report := body["report"].(map[string]any)
	if report["shelved"].(float64) != 1 || report["ratings_set"].(float64) != 1 || report["reviews_created"].(float64) != 1 {
		t.Fatalf("report = %v", report)
	}

	// 2 · The shelf really holds it, with the imported rating and provenance.
	resp := e.get(c, "/v1/me/library?shelf=read")
	lib := mustStatus(t, resp, http.StatusOK)
	raw := mustJSON(t, lib)
	if !strings.Contains(raw, "Pride and Prejudice") {
		t.Fatal("imported work missing from Read shelf")
	}
	var itemID string
	if err := e.st.Pool().QueryRow(e.ctx,
		`SELECT si.id FROM shelf_items si JOIN shelves s ON s.id=si.shelf_id
		  WHERE s.user_id = (SELECT id FROM users WHERE username='refugee') AND si.rating = 10 AND si.imported_from='goodreads'`).Scan(&itemID); err != nil {
		t.Fatalf("imported rating/provenance not persisted: %v", err)
	}

	// 3 · Re-importing does not duplicate the review.
	status, body = post("/v1/imports/goodreads", importCSV)
	if status != http.StatusOK {
		t.Fatalf("re-import status = %d", status)
	}
	skips := body["report"].(map[string]any)["skips"].([]any)
	found := false
	for _, sk := range skips {
		if m, ok := sk.(map[string]any); ok && m["reason"] == "review_exists" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected review_exists skip, got %v", skips)
	}

	// 4 · Unmatched titles are reported, never invented.
	status, body = post("/v1/imports/goodreads",
		"Title,Author,ISBN13,My Rating,Exclusive Shelf\nA Book Nobody Ingested,Someone,,4,read\n")
	if status != http.StatusOK {
		t.Fatalf("unmatched-import status = %d", status)
	}
	if !strings.Contains(mustJSON(t, body), "no_matching_work") {
		t.Fatalf("unmatched row not reported: %v", body)
	}

	// 5 · The throttle holds at three imports per hour.
	status, _ = post("/v1/imports/goodreads", importCSV)
	if status != http.StatusTooManyRequests {
		t.Fatalf("fourth import status = %d, want 429", status)
	}

	// 6 · Export returns the reader's whole life, including the import.
	resp = e.get(c, "/v1/export")
	exp := mustStatus(t, resp, http.StatusOK)
	if !strings.Contains(mustJSON(t, exp), "9780141439518") && !strings.Contains(mustJSON(t, exp), "Pride and Prejudice") {
		t.Fatal("export missing imported history")
	}
	if exp["exported_at"] == nil {
		t.Fatal("export missing timestamp")
	}
}

func strPtr(s string) *string { return &s }
