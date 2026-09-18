package detective

import (
	"bytes"
	"github.com/ledongthuc/pdf"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResearchExportsLongTableAndScripts(t *testing.T) {
	rows := [][]string{{"Bereich", "Beobachtung"}}
	for i := 0; i < 65; i++ {
		rows = append(rows, []string{"Überprüfung", strings.Repeat("Ein überprüfbarer Befund. ", 12)})
	}
	rows = append(rows, []string{"Langer Abschnitt", strings.Repeat("Langzeittest mit überprüfbaren Angaben. ", 100) + " END-OF-TABLE"})
	r := Report{Revision: 1, Title: "Detective – Bericht", Summary: "Prüfung von Tabellen, Quellen und Seitenumbrüchen.", CreatedAt: time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC), Blocks: []Block{{Type: "table", Rows: rows, Evidence: []string{"ev"}}}, Findings: []Finding{{ID: "ev", SourceID: "src"}}, Sources: []Source{{ID: "src", URL: "https://example.org/study", Title: "Studie", RetrievedAt: time.Now()}}}
	for _, format := range []string{"md", "pdf", "docx"} {
		a, err := Export(r, format)
		if err != nil {
			t.Fatal(format, err)
		}
		if format == "pdf" {
			doc, err := pdf.NewReader(bytes.NewReader(a.Data), int64(len(a.Data)))
			if err != nil {
				t.Fatal(err)
			}
			if doc.NumPage() < 3 {
				t.Fatal("long table was not paginated")
			}
			found := false
			for i := 1; i <= doc.NumPage(); i++ {
				text, err := doc.Page(i).GetPlainText(nil)
				if err != nil {
					t.Fatal(err)
				}
				found = found || strings.Contains(text, "END-OF-TABLE")
			}
			if !found {
				t.Fatal("last table cell was lost")
			}
		}
		if dir := os.Getenv("AURAGO_DETECTIVE_ARTIFACT_DIR"); dir != "" {
			if err = os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(dir, "export-fixture."+format), a.Data, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, text := range []string{"汉字", "日本語", "हिन्दी", "😀"} {
		r.Title = text
		if _, err := Export(r, "pdf"); err == nil {
			t.Fatalf("silently rendered unsupported script: %s", text)
		}
		if !strings.Contains(ReportHTML(r), text) {
			t.Fatal("renderer HTML lost Unicode")
		}
		if _, err := Export(r, "docx"); err != nil {
			t.Fatal(err)
		}
	}
	r.Title = "<script>alert(1)</script>"
	if strings.Contains(ReportHTML(r), "<script>") {
		t.Fatal("report HTML executes source markup")
	}
}
