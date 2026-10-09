package zim

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"sync"
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
	var xapian *io.SectionReader // an uncompressed-cluster section, used after Close below
	for path, size := range map[string]int64{"fulltext/xapian": 1646592, "title/xapian": 827392} {
		r, err := a.Open(mustEntry(t, a, 'X', path))
		if err != nil {
			t.Fatalf("Open(X/%s) error = %v", path, err)
		}
		if r.Size() != size {
			t.Fatalf("X/%s = size %d; want %d", path, r.Size(), size)
		}
		head := make([]byte, 14)
		if _, err := r.ReadAt(head, 0); err != nil || !bytes.Equal(head, []byte("\x0f\x0dXapian Glass")) {
			t.Fatalf("X/%s head = %q, %v", path, head, err)
		}
		xapian = r
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

	// "climate" is byte-wise and case-sensitive: only the lower-case title of
	// the article stored as C/Climateprediction.net matches.
	got, err = a.TitlePrefix("climate", 5)
	if err != nil || !equalStrings(titlesOf(got), []string{"climateprediction.net"}) || got[0].Path != "Climateprediction.net" {
		t.Fatalf("TitlePrefix(climate) = %q, %v", titlesOf(got), err)
	}
	if main, err := a.MainEntry(); err != nil || main.Namespace != 'C' || main.IsRedirect {
		t.Fatalf("MainEntry = %+v, %v; want a resolved C entry", main, err)
	}

	walkRealFixture(t, a)
	concurrentWalkThenClose(t, a, xapian, r)
}

// walkRealFixture reads every entry of the archive.
func walkRealFixture(t *testing.T, a *Archive) {
	t.Helper()
	if len(a.mimeTypes) != 9 {
		t.Fatalf("MIME types = %d (%q), want 9", len(a.mimeTypes), a.mimeTypes)
	}
	namespaces := map[byte]int{}
	redirects, blobs := 0, 0
	var prev Entry
	for i := uint32(0); i < a.EntryCount(); i++ {
		e, err := a.EntryAt(i)
		if err != nil {
			t.Fatalf("EntryAt(%d) error = %v", i, err)
		}
		if i > 0 && compareKey(prev.Namespace, prev.Path, e.Namespace, e.Path) >= 0 {
			t.Fatalf("entries %d and %d are not in path order: %c/%s, %c/%s", i-1, i, prev.Namespace, prev.Path, e.Namespace, e.Path)
		}
		prev = e
		namespaces[e.Namespace]++
		if e.IsRedirect {
			redirects++
			if res, err := a.Resolve(e); err != nil || res.IsRedirect {
				t.Fatalf("Resolve(%c/%s) = %+v, %v", e.Namespace, e.Path, res, err)
			}
			continue
		}
		blobs++
		r, err := a.Open(e)
		if err != nil {
			t.Fatalf("Open(%c/%s) error = %v", e.Namespace, e.Path, err)
		}
		if n, err := io.Copy(io.Discard, r); err != nil || n != r.Size() {
			t.Fatalf("reading %c/%s: %d of %d bytes, %v", e.Namespace, e.Path, n, r.Size(), err)
		}
	}
	if want := map[byte]int{'C': 20551, 'M': 12, 'W': 1, 'X': 4}; !equalCounts(namespaces, want) {
		t.Fatalf("namespace counts = %v, want %v", namespaces, want)
	}
	if redirects != 16567 || blobs != 4001 {
		t.Fatalf("redirects = %d, content blobs = %d; want 16567 and 4001", redirects, blobs)
	}

	// The v1 listing is sorted byte-wise by title and lists no redirects.
	prevTitle := ""
	for i := 0; i < a.ArticleCount(); i++ {
		e, err := a.ArticleAt(i)
		if err != nil || e.IsRedirect || e.Namespace != 'C' || e.Title < prevTitle {
			t.Fatalf("ArticleAt(%d) = %+v, %v (previous title %q)", i, e, err, prevTitle)
		}
		prevTitle = e.Title
	}
}

func equalCounts(a, b map[byte]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// concurrentWalkThenClose reads articles from several goroutines at once, then
// closes the archive and expects ErrClosed everywhere, including on sections
// that were opened before Close (uncompressed file section and cached cluster).
func concurrentWalkThenClose(t *testing.T, a *Archive, fileSection, clusterSection *io.SectionReader) {
	t.Helper()
	const workers = 8
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := w; i < a.ArticleCount(); i += workers {
				e, err := a.ArticleAt(i)
				if err == nil {
					e, err = a.Resolve(e)
				}
				var r *io.SectionReader
				if err == nil {
					r, err = a.Open(e)
				}
				if err == nil {
					_, err = io.Copy(io.Discard, r)
				}
				if err != nil {
					t.Errorf("worker %d, article %d: %v", w, i, err)
					return
				}
			}
		}()
	}
	wg.Wait()
	if t.Failed() {
		t.FailNow()
	}

	if err := a.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	_, err := a.EntryAt(0)
	wantErr(t, err, ErrClosed)
	_, err = a.ArticleAt(0)
	wantErr(t, err, ErrClosed)
	_, err = a.TitlePrefix("Kyoto", 5)
	wantErr(t, err, ErrClosed)
	_, err = a.Metadata("Language")
	wantErr(t, err, ErrClosed)
	wantErr(t, a.VerifyChecksum(context.Background()), ErrClosed)
	for name, r := range map[string]*io.SectionReader{"uncompressed section": fileSection, "cluster section": clusterSection} {
		if _, err := r.ReadAt(make([]byte, 8), 0); !errors.Is(err, ErrClosed) {
			t.Fatalf("%s read after Close: error = %v, want ErrClosed", name, err)
		}
	}
}
