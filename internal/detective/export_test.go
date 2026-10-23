package detective

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"github.com/ledongthuc/pdf"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAutomaticUpdateExportsPreserveEverySection(t *testing.T) {
	s := newTestService(t)
	c, err := s.Create(Request{Topic: "export"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		x := updateSession(t, s, c.ID, "append")
		r := updateReport(updateEvidence(t, x, i))
		if i == 0 {
			r.Summary, r.Limitations = "BASE-SUMMARY", "BASE-LIMITATION"
		} else {
			r.Title, r.Summary, r.Limitations = "UPDATE-TITLE", "UPDATE-SUMMARY", "UPDATE-LIMITATION"
		}
		if err = x.Submit(r); err != nil {
			t.Fatal(err)
		}
	}
	for _, format := range []string{"md", "pdf", "docx"} {
		a, err := s.ExportRevision(context.Background(), c.ID, 2, format)
		if err != nil {
			t.Fatal(format, err)
		}
		text := string(a.Data)
		if format == "pdf" {
			doc, err := pdf.NewReader(bytes.NewReader(a.Data), int64(len(a.Data)))
			if err != nil {
				t.Fatal(err)
			}
			text = ""
			for i := 1; i <= doc.NumPage(); i++ {
				part, err := doc.Page(i).GetPlainText(nil)
				if err != nil {
					t.Fatal(err)
				}
				text += part
			}
		} else if format == "docx" {
			z, err := zip.NewReader(bytes.NewReader(a.Data), int64(len(a.Data)))
			if err != nil {
				t.Fatal(err)
			}
			r, err := z.Open("word/document.xml")
			if err != nil {
				t.Fatal(err)
			}
			b, err := io.ReadAll(r)
			r.Close()
			if err != nil {
				t.Fatal(err)
			}
			text = string(b)
		}
		for _, want := range []string{"BASE-SUMMARY", "BASE-LIMITATION", "UPDATE-TITLE", "UPDATE-SUMMARY", "UPDATE-LIMITATION", "measurement is 0", "measurement is 1"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s lost %s", format, want)
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, format := range []string{"md", "pdf", "docx"} {
		if _, err := ExportContext(ctx, Report{}, format); !errors.Is(err, context.Canceled) {
			t.Fatal("export ignored cancellation", format, err)
		}
	}
	w := exportWriter{ctx: context.Background(), w: io.Discard, bytes: 32 << 20}
	if _, err := w.Write([]byte("x")); err == nil {
		t.Fatal("output limit ignored")
	}
}

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

// An export refusal names its reason through a sentinel, so the API can answer with a
// code the app translates (audit 4.14).
func TestExportRefusalsCarryStableSentinels(t *testing.T) {
	if _, err := Export(Report{Revision: 1, Title: "Detective", CreatedAt: time.Now()}, "xls"); !errors.Is(err, ErrExportFormat) {
		t.Fatalf("unknown format: %v", err)
	}
	devanagari := Report{Revision: 1, Title: "रिपोर्ट", Summary: "सारांश", CreatedAt: time.Now()}
	if _, err := Export(devanagari, "pdf"); !errors.Is(err, ErrPDFFont) {
		t.Fatalf("non-Latin PDF: %v", err)
	}
	if artifact, err := Export(devanagari, "docx"); err != nil || len(artifact.Data) == 0 {
		t.Fatalf("non-Latin docx: %v", err)
	}
}