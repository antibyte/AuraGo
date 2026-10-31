package atlaspdf

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/infra"

	"github.com/ledongthuc/pdf"
)

var testNow = time.Date(2026, 10, 31, 14, 30, 0, 0, time.UTC)

func testDoc(t *testing.T, s infra.Snapshot, paper Paper) *doc {
	t.Helper()
	m, err := newModel(s)
	if err != nil {
		t.Fatal(err)
	}
	d, err := newDoc(Options{Paper: paper, Map: true, Hosts: true, Inventory: true, Now: testNow}, m)
	if err != nil {
		t.Fatal(err)
	}
	d.sc = buildScene(m, "")
	return d
}

func pdfText(t *testing.T, data []byte) (string, int) {
	t.Helper()
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("parse pdf: %v", err)
	}
	var b strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		text, err := r.Page(i).GetPlainText(nil)
		if err != nil {
			t.Fatalf("page %d text: %v", i, err)
		}
		b.WriteString(text)
	}
	return b.String(), r.NumPage()
}

func TestSceneRegionUnionGuestsAndUnplacedBlock(t *testing.T) {
	f := &fixture{}
	f.place(infra.NodeSite, "s", "Site", "", 0, 0, 100, 100)
	f.place(infra.NodeRoom, "r", "Room", "s", 0, 0, 0, 0)
	f.asset(infra.NodeDevice, "h", "Host", "", "r", "", "", 500, 500, true)
	f.guest(infra.NodeVM, "v", "Vm", "", "h", "", "")
	f.add(infra.Node{ID: "k", Kind: infra.NodeContainer, Name: "Box"})
	f.link("k", "v", infra.EdgeRunsOn, "", "", "")
	f.guest(infra.NodeVM, "a", "A", "", "b", "", "")
	f.guest(infra.NodeVM, "b", "B", "", "a", "", "")
	f.asset(infra.NodeDevice, "lone", "Lone", "", "", "", "", 0, 0, false)
	f.add(infra.Node{ID: "orphan", Kind: infra.NodeContainer, Name: "Orphan"})
	m, err := newModel(f.snapshot())
	if err != nil {
		t.Fatal(err)
	}
	sc := buildScene(m, "")

	room := sc.regions[sc.regionIdx["r"]].Rect
	if want := (rect{0, 0, 656, 580}); room != want {
		t.Fatalf("room rect = %+v, want %+v (default size grown to the padded card)", room, want)
	}
	if site := sc.regions[sc.regionIdx["s"]].Rect; site != room.pad(regionPad) || sc.regions[sc.regionIdx["s"]].Depth != 0 || sc.regions[sc.regionIdx["r"]].Depth != 1 {
		t.Fatalf("site rect %+v depth %d, want the room rect plus the frame padding at depth 0", site, sc.regions[sc.regionIdx["s"]].Depth)
	}
	host := sc.cards[sc.cardIdx["h"]]
	if len(host.Guests) != 2 || host.Guests[0] != "v" || host.Guests[1] != "k" {
		t.Fatalf("host guests = %v, want [v k] (nested guests included)", host.Guests)
	}
	if _, ok := sc.cardIdx["v"]; ok {
		t.Fatal("a guest must not get a card of its own")
	}
	if m.hostOf["k"] != "v" || sc.endpoint("k") != "h" {
		t.Fatalf("container k should ride on h through v, got endpoint %q", sc.endpoint("k"))
	}
	if _, aGuest := m.hostOf["a"]; !aGuest {
		t.Fatal("a should stay a guest of b")
	}
	if _, bGuest := m.hostOf["b"]; bGuest {
		t.Fatal("mutual hosting must drop the larger id's link")
	}
	lone, ok := sc.cardIdx["lone"]
	if !ok || !sc.cards[lone].Unplaced {
		t.Fatal("an unplaced asset belongs to the unplaced block")
	}
	if _, ok := sc.cardIdx["orphan"]; ok {
		t.Fatal("a hostless container only appears in the tables")
	}
	frame := sc.regions[sc.regionIdx[unplacedID]]
	if !frame.Pseudo || frame.Rect.Y < room.bottom()+unplacedGap-1 {
		t.Fatalf("unplaced frame %+v should sit %v below the map", frame.Rect, unplacedGap)
	}
}

func TestPlanSmallLabIsOneMapPage(t *testing.T) {
	d := testDoc(t, smallLab(), PaperA4)
	pages := d.planMap()
	if len(pages) != 1 || !pages[0].Overview || pages[0].Scale < readableScale {
		t.Fatalf("small lab pages = %d (scale %.3f), want one readable overview", len(pages), pages[0].Scale)
	}
}

func TestPlanLargeLabPacksIslandsAtOneReadableScale(t *testing.T) {
	for _, paper := range []Paper{PaperA4, PaperA3} {
		d := testDoc(t, largeLab(), paper)
		pages := d.planMap()
		if len(pages) < 3 || !pages[0].Overview || pages[0].Scale >= readableScale {
			t.Fatalf("%s: got %d pages, overview scale %.3f; want an unreadable overview plus detail pages", paper, len(pages), pages[0].Scale)
		}
		detail := pages[1:]
		onPage := map[int]bool{}
		for i, p := range detail {
			if math.Abs(p.Scale-detail[0].Scale) > 1e-12 || p.Scale < readableScale-1e-9 || p.Scale > 1.3*readableScale+1e-9 {
				t.Errorf("%s: detail page %d scale %.4f; want one shared scale in [%.4f, %.4f]", paper, i+1, p.Scale, readableScale, 1.3*readableScale)
			}
			var boxes []rect
			for _, pl := range p.Places {
				is := &d.islands[pl.Island]
				r := pl.boxRect(is, p.Scale)
				r.Y -= islandCaptionH
				r.H += islandCaptionH
				if r.X < d.content.X-1e-6 || r.Y < d.content.Y-1e-6 || r.right() > d.content.right()+1e-6 || r.bottom() > d.content.bottom()+1e-6 {
					t.Errorf("%s: page %d island %q at %+v leaves the content area", paper, i+1, is.Caption, r)
				}
				if r.intersects(p.Locator) {
					t.Errorf("%s: page %d island %q overlaps the locator inset", paper, i+1, is.Caption)
				}
				for _, o := range boxes {
					if r.intersects(o) {
						t.Errorf("%s: page %d islands overlap", paper, i+1)
					}
				}
				boxes = append(boxes, r)
				for _, ci := range is.Cards {
					onPage[ci] = true
				}
				if len(p.Places) == 1 && len(d.islands) > 1 && len(is.Regions) == 0 && len(is.Cards) == 1 {
					t.Errorf("%s: page %d holds a single loose card alone", paper, i+1)
				}
			}
		}
		for ci, c := range d.sc.cards {
			if !onPage[ci] {
				t.Errorf("%s: card %s appears on no detail page", paper, c.ID)
			}
		}
		// the Serverschrank rack stays together in one island
		found := false
		for _, is := range d.islands {
			if is.regions["rack1"] && is.Parts == 0 {
				found = true
				for ci, c := range d.sc.cards {
					if d.m.parent[c.ID] == "rack1" && !is.cards[c.ID] {
						t.Errorf("%s: rack card %s (index %d) is not in the rack's island", paper, c.ID, ci)
					}
				}
			}
		}
		if !found {
			t.Errorf("%s: no untiled island holds the Serverschrank rack", paper)
		}
	}
}

func TestRenderLargeLabProducesAMultiPagePDF(t *testing.T) {
	data, err := Render(largeLab(), Options{Paper: PaperA4, Map: true, Hosts: true, Inventory: true, Now: testNow})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Fatal("output is not a PDF")
	}
	text, pages := pdfText(t, data)
	if pages < 8 {
		t.Fatalf("pages = %d, want overview, detail, hosts and table pages", pages)
	}
	for _, want := range []string{"Homelab map", "Fritz!Box 7590", "Hosts & guests", "docker01", "Inventory", "Connections", "Büro"} {
		if !strings.Contains(text, want) {
			t.Errorf("PDF text lacks %q", want)
		}
	}
	if !strings.Contains(text, "Page 1 / "+itoa(pages)) {
		t.Errorf("footer lacks the page count %d", pages)
	}
}

func TestRenderSectionsAndPapers(t *testing.T) {
	for _, paper := range []Paper{PaperA3, PaperLetter} {
		data, err := Render(smallLab(), Options{Paper: paper, Hosts: true, Now: testNow})
		if err != nil {
			t.Fatal(err)
		}
		text, pages := pdfText(t, data)
		if pages != 1 || !strings.Contains(text, "Hosts & guests") || strings.Contains(text, "Legend") {
			t.Fatalf("%s hosts-only render: pages=%d", paper, pages)
		}
	}
}

func TestRenderEmptySnapshotShowsTheEmptyMessage(t *testing.T) {
	data, err := Render(infra.Snapshot{}, Options{Now: testNow})
	if err != nil {
		t.Fatal(err)
	}
	text, pages := pdfText(t, data)
	if pages != 1 || !strings.Contains(text, "No assets yet") {
		t.Fatalf("empty render: pages=%d text=%q", pages, text)
	}
}

func TestRenderSurvivesBadData(t *testing.T) {
	nan := math.NaN()
	f := &fixture{}
	f.place(infra.NodeSite, "a", "Cycle A", "b", 0, 0, 300, 200)
	f.place(infra.NodeSite, "b", "Cycle B", "a", 400, 0, math.Inf(1), nan)
	f.place(infra.NodeRoom, "self", "Self", "self", 1e9, 0, 0, 0)
	f.asset(infra.NodeDevice, "n", "NaN card", "", "a", "", "", nan, 10, true)
	f.asset(infra.NodeDevice, "e", "Emoji \U0001F642 हिन्दी", "no/such/icon", "a", "", "weird", 100, 100, true)
	f.asset(infra.NodeDevice, "e", "Duplicate id", "", "", "", "", 0, 0, true)
	f.add(infra.Node{ID: "", Kind: "alien", Name: ""})
	f.guest(infra.NodeVM, "g1", "Loop 1", "", "g2", "", "")
	f.guest(infra.NodeVM, "g2", "Loop 2", "", "g3", "", "")
	f.guest(infra.NodeVM, "g3", "Loop 3", "", "g1", "", "")
	f.link("e", "missing", infra.EdgeEthernet, "", "", "")
	f.link("e", "e", infra.EdgeEthernet, "", "", "")
	f.link("g1", "g2", infra.EdgeRunsOn, "", "", "")
	f.link("a", "b", infra.EdgeMemberOf, "", "", "")
	data, err := Render(f.snapshot(), Options{Paper: "bogus", Now: testNow})
	if err != nil {
		t.Fatal(err)
	}
	if text, _ := pdfText(t, data); !strings.Contains(text, "Cycle A") {
		t.Fatal("bad data should still render the valid parts")
	}
}

func TestRenderRejectsOversizedSnapshots(t *testing.T) {
	_, err := Render(infra.Snapshot{Nodes: make([]infra.Node, maxNodes+1)}, Options{})
	if err != ErrTooLarge {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
}

func TestUntranslatableLabelsFallBackToEnglish(t *testing.T) {
	hindi := func(key string) string {
		if key == "desktop.infra_pdf_title" || key == "desktop.infra_pdf_hosts_title" {
			return "हिन्दी"
		}
		return ""
	}
	data, err := Render(smallLab(), Options{Text: hindi, Now: testNow})
	if err != nil {
		t.Fatal(err)
	}
	if text, _ := pdfText(t, data); !strings.Contains(text, "Homelab map") {
		t.Fatal("labels the fonts cannot print must fall back to English")
	}
	german := func(key string) string {
		if key == "desktop.infra_pdf_title" {
			return "Heimnetz-Karte"
		}
		return key
	}
	data, err = Render(smallLab(), Options{Text: german, Now: testNow})
	if err != nil {
		t.Fatal(err)
	}
	if text, _ := pdfText(t, data); !strings.Contains(text, "Heimnetz-Karte") || !strings.Contains(text, "Inventory") {
		t.Fatal("translated keys are used and missing keys use the English default")
	}
}

// TestWritePreviewPDFs writes the fixture documents to INFRA_PDF_PREVIEW for visual review.
func TestWritePreviewPDFs(t *testing.T) {
	dir := os.Getenv("INFRA_PDF_PREVIEW")
	if dir == "" {
		t.Skip("INFRA_PDF_PREVIEW is not set")
	}
	icons := os.DirFS(filepath.Join("..", "..", "..", "ui"))
	for name, c := range map[string]struct {
		snap  infra.Snapshot
		paper Paper
	}{
		"small-a4.pdf": {smallLab(), PaperA4}, "large-a4.pdf": {largeLab(), PaperA4}, "large-a3.pdf": {largeLab(), PaperA3},
	} {
		data, err := Render(c.snap, Options{Paper: c.paper, Map: true, Hosts: true, Inventory: true, Icons: icons, Now: testNow})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
		_, pages := pdfText(t, data)
		t.Logf("%s: %d pages, %d bytes", name, pages, len(data))
	}
}

func TestCJKNamesUseTheFallbackFont(t *testing.T) {
	f := &fixture{}
	f.asset(infra.NodeDevice, "d", "\u670d\u52a1\u5668 Server", "", "", "10.0.0.1", "running", 100, 100, true)
	opts := Options{Map: true, Now: testNow, CJKFonts: os.DirFS(filepath.Join("..", "..", "newspaper", "fonts"))}
	data, err := Render(f.snapshot(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if dir := os.Getenv("INFRA_PDF_PREVIEW"); dir != "" {
		_ = os.WriteFile(filepath.Join(dir, "cjk.pdf"), data, 0o644)
	}
	// gofpdf embeds the font as CIDs that the text extractor cannot map back, so only the fallback is asserted.
	opts.CJKFonts = nil
	plain, err := Render(f.snapshot(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if text, _ := pdfText(t, plain); !strings.Contains(text, "??? Server") {
		t.Fatalf("without the fallback font the characters become '?', got %q", text)
	}
}

// TestLogDetailPlan prints the detail pages and their islands when INFRA_PDF_PREVIEW is set.
func TestLogDetailPlan(t *testing.T) {
	if os.Getenv("INFRA_PDF_PREVIEW") == "" {
		t.Skip("INFRA_PDF_PREVIEW is not set")
	}
	for _, paper := range []Paper{PaperA4, PaperA3} {
		d := testDoc(t, largeLab(), paper)
		for i, p := range d.planMap() {
			var parts []string
			for _, pl := range p.Places {
				is := d.islands[pl.Island]
				parts = append(parts, fmt.Sprintf("%q(%d cards, part %d/%d)", is.Caption, len(is.Cards), is.Part, is.Parts))
			}
			t.Logf("%s page %d scale %.3f %q: %s", paper, i+1, p.Scale, p.Title, strings.Join(parts, "; "))
		}
	}
}
