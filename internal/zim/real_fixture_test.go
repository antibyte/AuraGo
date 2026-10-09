package zim

import (
	"bytes"
	"context"
	"encoding/hex"
	"io"
	"strings"
	"testing"

	"aurago/internal/zim/zimtest"
)

// TestRealFixture checks the reader against a libzim-written archive. It runs
// only with AURAGO_ZIM_REAL_FIXTURE=1 (downloads 8.6 MiB once into the user cache).
func TestRealFixture(t *testing.T) {
	a := openPath(t, zimtest.RealFixture(t))
	uuid := a.UUID()
	if a.Size() != zimtest.RealFixtureSize || a.EntryCount() != 20568 || a.ArticleCount() != 3821 ||
		a.ContentNamespace() != 'C' || hex.EncodeToString(uuid[:]) != "53293c1bced35244564ab40e435e2f0a" || !a.titles.v1 {
		t.Fatalf("archive = size %d, entries %d, articles %d, ns %c, uuid %x, v1 %v",
			a.Size(), a.EntryCount(), a.ArticleCount(), a.ContentNamespace(), uuid, a.titles.v1)
	}
	for name, want := range map[string]string{
		"Language": "eng", "Title": "Climate change by Wikipedia", "Date": "2024-06-18",
		"Name": "wikipedia_en_climate_change", "Flavour": "mini", "Creator": "Wikipedia", "Publisher": "openZIM",
	} {
		if got, err := a.Metadata(name); err != nil || got != want {
			t.Fatalf("Metadata(%q) = %q, %v; want %q", name, got, err, want)
		}
	}
	if main, err := a.MainEntry(); err != nil || main.Path != "index" || main.Title != "Main Page" {
		t.Fatalf("MainEntry = %+v, %v", main, err)
	}

	loadsBefore := a.loads.Load()
	for path, size := range map[string]int64{"fulltext/xapian": 1646592, "title/xapian": 827392} {
		r, err := a.Open(mustEntry(t, a, 'X', path))
		if err != nil || r.Size() != size {
			t.Fatalf("X/%s = size %d, %v; want %d", path, r.Size(), err, size)
		}
		head := make([]byte, 14)
		if _, err := r.ReadAt(head, 0); err != nil || !bytes.Equal(head, []byte("\x0f\x0dXapian Glass")) {
			t.Fatalf("X/%s head = %q, %v", path, head, err)
		}
	}
	if a.loads.Load() != loadsBefore {
		t.Fatal("Xapian blobs must be read from uncompressed clusters without decompression")
	}

	if first, err := a.ArticleAt(0); err != nil || first.Title != "100% renewable energy" {
		t.Fatalf("ArticleAt(0) = %+v, %v", first, err)
	}
	if last, err := a.ArticleAt(3820); err != nil || last.Path != "Δ18O" || last.Title != "δ18O" {
		t.Fatalf("ArticleAt(3820) = %+v, %v", last, err)
	}
	got, err := a.TitlePrefix("Kyoto", 5)
	if err != nil || !equalStrings(titlesOf(got), []string{"Kyoto Protocol", "Kyoto Protocol and government action"}) {
		t.Fatalf("TitlePrefix(Kyoto) = %q, %v", titlesOf(got), err)
	}
	target, err := a.Resolve(mustEntry(t, a, 'C', "10-year_flood"))
	if err != nil || target.Path != "100-year_flood" {
		t.Fatalf("Resolve(10-year_flood) = %+v, %v", target, err)
	}
	r, err := a.Open(mustEntry(t, a, 'C', "Climate_change"))
	if err != nil {
		t.Fatal(err)
	}
	html, err := io.ReadAll(r)
	if err != nil || len(html) != 14386 || !strings.Contains(string(html), "<title>Climate change</title>") {
		t.Fatalf("C/Climate_change = %d bytes, %v", len(html), err)
	}
	if err := a.VerifyChecksum(context.Background()); err != nil {
		t.Fatalf("VerifyChecksum = %v", err)
	}
}
