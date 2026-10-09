package localwiki

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The testdata HTML files are trimmed real mwoffliner pages (text CC BY-SA
// 4.0, Wikipedia contributors; sources in the file comments): the English
// one from a mwoffliner 1.13 mini ZIM, the German one in mwoffliner 2.x
// (Parsoid read view) structure. The .md files are the expected output.
func TestRenderGoldenMwofflinerPages(t *testing.T) {
	for _, tc := range []struct{ name, title string }{
		{"render_en_okjokull", "Okjökull"},
		{"render_de_bielefeld", "Bielefeld"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("testdata", tc.name+".html"))
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join("testdata", tc.name+".md"))
			if err != nil {
				t.Fatal(err)
			}
			art, err := renderArticle(raw, tc.title)
			if err != nil {
				t.Fatal(err)
			}
			if got := art.fullMarkdown(); got != strings.TrimSpace(strings.ReplaceAll(string(want), "\r\n", "\n")) {
				t.Fatalf("markdown mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}
		})
	}
}

func TestRenderGoldenBielefeldSections(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "render_de_bielefeld.html"))
	if err != nil {
		t.Fatal(err)
	}
	art, err := renderArticle(raw, "Bielefeld")
	if err != nil {
		t.Fatal(err)
	}
	var headings []string
	for _, s := range art.sectionList() {
		headings = append(headings, s.Heading)
	}
	if got := strings.Join(headings, "|"); got != "|Geographie|Ausdehnung und Nutzung des Stadtgebiets|Religion" {
		t.Fatalf("headings = %q (Weblinks and Einzelnachweise must be dropped)", got)
	}
}
