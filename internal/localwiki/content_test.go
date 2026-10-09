package localwiki

import (
	"errors"
	"io"
	"math/rand/v2"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/zim"
)

// fakeContentArchive is an in-memory contentArchive; entry indexes are slice positions.
type fakeContentArchive struct {
	ns       byte
	uuid     [16]byte
	entries  []zim.Entry
	blobs    map[uint32]string
	articles []uint32
	mainIdx  int
}

func (f *fakeContentArchive) ContentNamespace() byte { return f.ns }
func (f *fakeContentArchive) UUID() [16]byte         { return f.uuid }
func (f *fakeContentArchive) ArticleCount() int      { return len(f.articles) }

func (f *fakeContentArchive) EntryByPath(ns byte, path string) (zim.Entry, error) {
	for _, entry := range f.entries {
		if entry.Namespace == ns && entry.Path == path {
			return entry, nil
		}
	}
	return zim.Entry{}, zim.ErrNotFound
}

func (f *fakeContentArchive) Resolve(entry zim.Entry) (zim.Entry, error) {
	for depth := 0; entry.IsRedirect; depth++ {
		if depth == 8 {
			return zim.Entry{}, zim.ErrRedirectLoop
		}
		if int(entry.RedirectTo) >= len(f.entries) {
			return zim.Entry{}, zim.ErrCorrupt
		}
		entry = f.entries[entry.RedirectTo]
	}
	return entry, nil
}

func (f *fakeContentArchive) Open(entry zim.Entry) (*io.SectionReader, error) {
	blob, ok := f.blobs[entry.Index]
	if !ok || entry.IsRedirect {
		return nil, zim.ErrNotFound
	}
	return io.NewSectionReader(strings.NewReader(blob), 0, int64(len(blob))), nil
}

func (f *fakeContentArchive) ArticleAt(i int) (zim.Entry, error) {
	if i < 0 || i >= len(f.articles) {
		return zim.Entry{}, zim.ErrNotFound
	}
	return f.entries[f.articles[i]], nil
}

func (f *fakeContentArchive) MainEntry() (zim.Entry, error) {
	if f.mainIdx < 0 {
		return zim.Entry{}, zim.ErrNotFound
	}
	return f.Resolve(f.entries[f.mainIdx])
}

func newFakeContentArchive() *fakeContentArchive {
	f := &fakeContentArchive{ns: 'C', uuid: [16]byte{0xde, 0xad, 0xbe, 0xef}, blobs: map[uint32]string{}}
	add := func(entry zim.Entry, blob string) {
		entry.Index = uint32(len(f.entries))
		f.entries = append(f.entries, entry)
		if !entry.IsRedirect {
			f.blobs[entry.Index] = blob
		}
	}
	add(zim.Entry{Namespace: 'C', Path: "Hauptseite", Title: "Hauptseite", MimeType: "text/html"}, "<html><title>Hauptseite</title></html>")      // 0
	add(zim.Entry{Namespace: 'C', Path: "Berlin", Title: "Berlin", MimeType: "text/html"}, "<html><title>Berlin</title><p>Hauptstadt</p></html>") // 1
	add(zim.Entry{Namespace: 'C', Path: "Berlin_(Stadt)", Title: "Berlin (Stadt)", IsRedirect: true, RedirectTo: 1}, "")                          // 2
	add(zim.Entry{Namespace: 'C', Path: "_assets_/logo.png", Title: "_assets_/logo.png", MimeType: "image/png"}, "\x89PNG\r\n\x1a\nfixture")      // 3
	add(zim.Entry{Namespace: 'M', Path: "Title", Title: "Title", MimeType: "text/plain"}, "Wikipedia")                                            // 4
	add(zim.Entry{Namespace: 'C', Path: "Loop_A", Title: "Loop_A", IsRedirect: true, RedirectTo: 6}, "")                                          // 5
	add(zim.Entry{Namespace: 'C', Path: "Loop_B", Title: "Loop_B", IsRedirect: true, RedirectTo: 5}, "")                                          // 6
	add(zim.Entry{Namespace: 'C', Path: "Ausbruch", Title: "Ausbruch", IsRedirect: true, RedirectTo: 4}, "")                                      // 7
	f.articles = []uint32{0, 1, 3, 2}
	return f
}

func TestArchiveContentServesTheContentNamespace(t *testing.T) {
	archive := newFakeContentArchive()
	item, err := archiveContent(archive, "Berlin")
	if err != nil {
		t.Fatalf("content: %v", err)
	}
	data, err := io.ReadAll(item.Reader)
	if err != nil || string(data) != "<html><title>Berlin</title><p>Hauptstadt</p></html>" {
		t.Fatalf("body = %q, %v", data, err)
	}
	if item.MimeType != "text/html; charset=utf-8" || item.Size != int64(len(data)) || item.Path != "Berlin" {
		t.Fatalf("item = %+v", item)
	}
	if item.ETag != contentETag(archive.uuid, "Berlin") || !strings.HasPrefix(item.ETag, `"deadbeef`) || !strings.HasSuffix(item.ETag, `"`) {
		t.Fatalf("etag = %q", item.ETag)
	}
	main, err := archiveContent(archive, "Hauptseite")
	if err != nil || main.ETag == item.ETag {
		t.Fatalf("etags must differ per path: %q %q %v", main.ETag, item.ETag, err)
	}
	logo, err := archiveContent(archive, "_assets_/logo.png")
	if err != nil || logo.MimeType != "image/png" {
		t.Fatalf("logo = %+v, %v", logo, err)
	}
}

func TestArchiveContentResolvesRedirects(t *testing.T) {
	archive := newFakeContentArchive()
	item, err := archiveContent(archive, "Berlin_(Stadt)")
	if err != nil {
		t.Fatalf("redirect: %v", err)
	}
	if item.Path != "Berlin" || item.ETag != contentETag(archive.uuid, "Berlin") {
		t.Fatalf("redirect item = %+v", item)
	}
	if _, err := archiveContent(archive, "Loop_A"); !errors.Is(err, zim.ErrRedirectLoop) {
		t.Fatalf("loop error = %v", err)
	}
}

func TestArchiveContentStaysInTheContentNamespace(t *testing.T) {
	archive := newFakeContentArchive()
	for _, path := range []string{"Title", "Ausbruch", "Gibt_es_nicht"} {
		if _, err := archiveContent(archive, path); !errors.Is(err, zim.ErrNotFound) {
			t.Fatalf("%s error = %v, want ErrNotFound", path, err)
		}
	}
}

func TestArchiveContentRejectsInvalidPaths(t *testing.T) {
	archive := newFakeContentArchive()
	for _, path := range []string{"", strings.Repeat("a", maxContentPathBytes+1), "bad\x00path", "\xff\xfe"} {
		if _, err := archiveContent(archive, path); !errors.Is(err, ErrInvalidPath) {
			t.Fatalf("%q error = %v, want ErrInvalidPath", path, err)
		}
	}
}

func TestContentMimeType(t *testing.T) {
	for raw, want := range map[string]string{
		"text/html":                     "text/html; charset=utf-8",
		"TEXT/CSS":                      "text/css; charset=utf-8",
		"text/html; charset=ISO-8859-1": "text/html; charset=ISO-8859-1",
		"image/png":                     "image/png",
		"image/svg+xml":                 "image/svg+xml",
		"application/javascript":        "application/javascript",
		"":                              "application/octet-stream",
		"garbage":                       "application/octet-stream",
		"*/*":                           "application/octet-stream",
	} {
		if got := contentMimeType(raw); got != want {
			t.Errorf("contentMimeType(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestArchiveRandomSkipsEntriesThatAreNotArticles(t *testing.T) {
	archive := newFakeContentArchive()
	picks := []int{2, 3} // the PNG first, then the redirect to Berlin
	calls := 0
	ref, err := archiveRandom(archive, func(n int) int {
		if n != 4 {
			t.Fatalf("pick range = %d, want 4", n)
		}
		pick := picks[calls]
		calls++
		return pick
	})
	if err != nil || ref != (contentRef{Title: "Berlin", Path: "Berlin"}) || calls != 2 {
		t.Fatalf("random = %+v, %v after %d picks", ref, err, calls)
	}
}

func TestArchiveRandomGivesUpWithoutArticles(t *testing.T) {
	archive := newFakeContentArchive()
	archive.articles = []uint32{3}
	calls := 0
	if _, err := archiveRandom(archive, func(int) int { calls++; return 0 }); !errors.Is(err, zim.ErrNotFound) || calls != randomAttempts {
		t.Fatalf("random without articles = %v after %d picks", err, calls)
	}
	archive.articles = nil
	if _, err := archiveRandom(archive, func(int) int { t.Fatal("must not pick from an empty list"); return 0 }); !errors.Is(err, zim.ErrNotFound) {
		t.Fatalf("empty list = %v", err)
	}
}

func TestArchiveMain(t *testing.T) {
	archive := newFakeContentArchive()
	if ref, err := archiveMain(archive); err != nil || ref != (contentRef{Title: "Hauptseite", Path: "Hauptseite"}) {
		t.Fatalf("main = %+v, %v", ref, err)
	}
	archive.mainIdx = 4
	if _, err := archiveMain(archive); !errors.Is(err, zim.ErrNotFound) {
		t.Fatalf("main outside the content namespace = %v", err)
	}
	archive.mainIdx = -1
	if _, err := archiveMain(archive); !errors.Is(err, zim.ErrNotFound) {
		t.Fatalf("missing main = %v", err)
	}
}

func TestLibraryContentWithoutArchive(t *testing.T) {
	var missing *Library
	if _, err := missing.Content("Berlin"); !errors.Is(err, zim.ErrNotFound) {
		t.Fatalf("nil library content = %v", err)
	}
	if _, err := (&Library{}).Random(); !errors.Is(err, zim.ErrNotFound) {
		t.Fatalf("empty library random = %v", err)
	}
	if _, err := (&Library{}).Main(); !errors.Is(err, zim.ErrNotFound) {
		t.Fatalf("empty library main = %v", err)
	}
}

// The generated fixture (slice 2) is a real libzim archive with articles in the title list.
func TestArchiveContentOnGeneratedFixture(t *testing.T) {
	path := filepath.Join("..", "zim", "testdata", "fixture_en.zim")
	archive, err := zim.Open(path, zim.Options{})
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer archive.Close()
	ref, err := archiveRandom(archive, rand.IntN)
	if err != nil {
		t.Fatalf("random: %v", err)
	}
	item, err := archiveContent(archive, ref.Path)
	if err != nil {
		t.Fatalf("content %q: %v", ref.Path, err)
	}
	data, err := io.ReadAll(item.Reader)
	if err != nil || len(data) == 0 || !strings.HasPrefix(item.MimeType, "text/html") || item.Path != ref.Path || item.Size != int64(len(data)) {
		t.Fatalf("content = %q, %d bytes, path %q, %v", item.MimeType, len(data), item.Path, err)
	}
	again, err := archiveContent(archive, ref.Path)
	if err != nil || again.ETag != item.ETag {
		t.Fatalf("etag must be stable: %q %q %v", item.ETag, again.ETag, err)
	}
	if main, err := archiveMain(archive); err == nil {
		if _, err := archiveContent(archive, main.Path); err != nil {
			t.Fatalf("main page content: %v", err)
		}
	} else if !errors.Is(err, zim.ErrNotFound) {
		t.Fatalf("main: %v", err)
	}
}
