package newspaper

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"aurago/internal/webassets"

	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

func TestPDFFontCoverage(t *testing.T) {
	for _, tc := range []struct {
		text string
		cjk  bool
	}{
		{"Grüße aus Pforzheim — Ω Ж", false},
		{"Grüße aus Pforzheim — 此", true},
		{"每日新闻 · 東京ニュース · 臺灣", true},
	} {
		t.Run(tc.text, func(t *testing.T) {
			regular, bold, err := pdfFonts(tc.text, os.DirFS("fonts"))
			if err != nil {
				t.Fatal(err)
			}
			wantRegular, wantBold := goregular.TTF, gobold.TTF
			if tc.cjk {
				wantRegular, err = os.ReadFile("fonts/NotoSansSC-Regular.ttf")
				if err != nil {
					t.Fatal(err)
				}
				wantBold, err = os.ReadFile("fonts/NotoSansSC-Bold.ttf")
				if err != nil {
					t.Fatal(err)
				}
			}
			if !bytes.Equal(regular, wantRegular) || !bytes.Equal(bold, wantBold) {
				t.Fatal("incorrect regular/bold font pair")
			}
		})
	}
	for _, text := range []string{"此 😀", "此 أخبار", "此 समाचार", "此 חדשות"} {
		if _, _, err := pdfFonts(text, os.DirFS("fonts")); err == nil {
			t.Errorf("unsupported text accepted: %q", text)
		}
	}
}

func TestPDFSourceTitleUsesVerifiedFontAssets(t *testing.T) {
	root := t.TempDir()
	manifest := webassets.Manifest{Version: 1}
	files := map[string][]byte{}
	for _, name := range []string{"NotoSansSC-Regular.ttf", "NotoSansSC-Bold.ttf"} {
		data, err := os.ReadFile(filepath.Join("fonts", name))
		if err != nil {
			t.Fatal(err)
		}
		path := "newspaper/fonts/" + name
		files[path] = data
		manifest.Files = append(manifest.Files, webassets.Entry{Path: path, Size: int64(len(data)), SHA256: webassets.Digest(data)})
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	id := webassets.Digest(raw)
	files[webassets.ManifestName] = raw
	for name, data := range files {
		path := filepath.Join(root, id, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	store := webassets.Open(root, webassets.Pin{ID: id})
	if !store.Ready() {
		t.Fatal(store.Error)
	}
	previous := webassets.Default
	webassets.Default = store
	t.Cleanup(func() { webassets.Default = previous; store.Close() })
	draft := testDraft(time.Now().UTC())
	e := Edition{Title: "Der Morgen", Language: "de", LocalDate: "2026-10-18", Stories: draft.Stories, Sources: draft.Sources}
	e.Sources[0].Title = "此消息来自当地"
	if _, err := PDF(e); err != nil {
		t.Fatal(err)
	}
	if _, err := renderPDF(e, fstest.MapFS{}); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing CJK resources must fail without losing text: %v", err)
	}
	e.Sources[0].Title = "Local news"
	if _, err := renderPDF(e, fstest.MapFS{}); err != nil {
		t.Fatalf("Latin PDF should not need external fonts: %v", err)
	}
}

func TestPDFMixedCJKPreservesEdition(t *testing.T) {
	draft := testDraft(time.Now().UTC())
	e := Edition{Title: "Der Morgen · 每日新闻", Language: "de", LocalDate: "2026-10-18", Place: "Pforzheim", Revision: 1, Stories: draft.Stories, Sources: draft.Sources, Corrections: []string{"Korrektur 此"}}
	e.Sources[0].Title = "此 · 東京ニュース · 臺灣"
	e.Stories[0].Paragraphs[0].Text += " Grüße aus Pforzheim 此。"
	before := Text(e)
	pdf, err := renderPDF(e, os.DirFS("fonts"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) || !bytes.Contains(pdf, []byte("/FontFile2")) {
		t.Fatal("missing PDF with embedded TrueType fonts")
	}
	if len(pdf) > 20<<20 {
		t.Fatal("PDF exceeds Telegram delivery limit")
	}
	if Text(e) != before || !strings.Contains(HTML(e), e.Sources[0].Title) {
		t.Fatal("PDF rendering modified the saved edition")
	}
}
