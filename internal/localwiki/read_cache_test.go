package localwiki

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"aurago/internal/zim"
	"aurago/internal/zim/xapian"
	"aurago/internal/zim/zimtest"
)

func TestReadTitleMatchesExactTitleBeforeTitleKey(t *testing.T) {
	useFreshRenderCache(t)
	store := newFakeArticleStore(
		fakeEntry{path: "Berlin", title: "Berlin", html: htmlPage("Berlin", `<p>Die Stadt.</p>`)},
		fakeEntry{path: "Berlin_(Band)", title: "Berlin (Band)", html: htmlPage("Berlin (Band)", `<p>Die Band.</p>`)},
		fakeEntry{path: "Munster", title: "Munster", html: htmlPage("Munster", `<p>Im Elsass.</p>`)},
		fakeEntry{path: "Münster", title: "Münster", html: htmlPage("Münster", `<p>In Westfalen.</p>`)},
	)
	cityFirst := []xapian.Hit{{Path: "Berlin", Title: "Berlin"}, {Path: "Berlin_(Band)", Title: "Berlin (Band)"}}
	munsterFirst := []xapian.Hit{{Path: "Munster", Title: "Munster"}, {Path: "Münster", Title: "Münster"}}
	titles := fakeTitleIndex{hits: map[string][]xapian.Hit{
		"berlin (band)":  cityFirst, // exact, case-insensitive
		"berlin_(band)":  cityFirst, // underscores read as spaces
		"Berlin (bänd)":  cityFirst, // equal after folding, qualifier kept
		"MÜNSTER":        munsterFirst,
		"Berlin (Stadt)": {{Path: "Berlin", Title: "Berlin"}}, // only the titleKey matches
	}}
	ix := searchIndex{store: store, titles: titles}
	for title, want := range map[string]string{
		"berlin (band)":  "Berlin_(Band)",
		"berlin_(band)":  "Berlin_(Band)",
		"Berlin (bänd)":  "Berlin_(Band)",
		"MÜNSTER":        "Münster",
		"munster":        "Munster", // a path candidate
		"Berlin (Stadt)": "Berlin",
	} {
		art, err := ix.read(context.Background(), ReadRequest{Title: title})
		if err != nil || art.Path != want {
			t.Fatalf("read title %q: %+v, %v; want %q", title, art.Ref, err, want)
		}
	}
}

func TestReadPathWithSpacesTriesUnderscores(t *testing.T) {
	useFreshRenderCache(t)
	store := newFakeArticleStore(fakeEntry{path: "Albert_Einstein", title: "Albert Einstein", html: htmlPage("Albert Einstein", `<p>Physiker.</p>`)})
	ix := searchIndex{store: store}
	for _, path := range []string{"Albert Einstein", "C/Albert  Einstein", "Albert_Einstein"} {
		art, err := ix.read(context.Background(), ReadRequest{Path: path})
		if err != nil || art.Path != "Albert_Einstein" {
			t.Fatalf("path %q: %+v, %v", path, art.Ref, err)
		}
	}
}

func TestReadFindsNumericHeadings(t *testing.T) {
	useFreshRenderCache(t)
	store := newFakeArticleStore(fakeEntry{path: "Wende", title: "Wende", html: htmlPage("Wende", `<p>Lead.</p><h2>1989</h2><p>Mauerfall.</p>`)})
	art, err := (searchIndex{store: store}).read(context.Background(), ReadRequest{Path: "Wende", Section: "1989"})
	if err != nil || art.Content != "## 1989\n\nMauerfall." {
		t.Fatalf("section 1989: %q, %v", art.Content, err)
	}
}

func TestReadStopsWhenTheContextEnds(t *testing.T) {
	useFreshRenderCache(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	store := cityArticles()
	if _, err := (searchIndex{store: store}).read(ctx, ReadRequest{Path: "Berlin"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if store.reads != 0 {
		t.Fatalf("article read %d times after the deadline", store.reads)
	}
}

func cacheTestArticle(bodyBytes int) *renderedArticle {
	return &renderedArticle{title: "a", sections: []renderedSection{{body: strings.Repeat("x", bodyBytes)}}}
}

func TestArticleCacheKeepsItsByteBudget(t *testing.T) {
	c := newArticleCache(8, 1000)
	size := cacheTestArticle(250).memSize()
	if size <= 250 || size > 500 {
		t.Fatalf("test needs an article between budget/4 and budget/2, got %d bytes", size)
	}
	c.put("a", cacheTestArticle(250))
	c.put("b", cacheTestArticle(250))
	if c.bytes != 2*size || len(c.keys) != 2 {
		t.Fatalf("bytes %d keys %v", c.bytes, c.keys)
	}
	c.put("c", cacheTestArticle(250))
	if _, ok := c.get("a"); ok || c.bytes > 1000 || len(c.keys) != 2 {
		t.Fatalf("budget exceeded: bytes %d keys %v", c.bytes, c.keys)
	}
	c.put("big", cacheTestArticle(500))
	if _, ok := c.get("big"); ok {
		t.Fatal("an article above half the budget must not be cached")
	}
	c.put("b", cacheTestArticle(250)) // replacing an entry keeps the count right
	if c.bytes != 2*size {
		t.Fatalf("bytes after replace = %d, want %d", c.bytes, 2*size)
	}
	c.purge("b")
	if _, ok := c.get("b"); ok || c.bytes != size {
		t.Fatalf("purge left bytes %d keys %v", c.bytes, c.keys)
	}
}

// writeLibraryZIM builds an edition with a main page, the article Ulm (body
// HTML), an image and a redirect to the image, under the given UUID.
func writeLibraryZIM(t *testing.T, uuid, body string) string {
	t.Helper()
	b := zimtest.New()
	copy(b.UUID[:], uuid)
	c := b.AddCluster(zimtest.CompressionZstd, false)
	b.AddMetadata(c, "Language", "deu")
	b.AddArticle(c, 'C', "Ulm", "Ulm", htmlPage("Ulm", body))
	b.Add(zimtest.Entry{Namespace: 'C', Path: "Logo.png", Title: "Logo.png", MimeType: "image/png", Data: []byte("\x89PNG"), Cluster: c})
	b.AddRedirect('C', "Wappen", "Wappen", "C/Logo.png")
	b.AddRedirect('W', "mainPage", "", "C/Ulm")
	path, _ := b.WriteFile(t)
	return path
}

func TestReadCacheSeparatesEditionsAndLibraries(t *testing.T) {
	useFreshRenderCache(t)
	ctx := context.Background()
	open := func(uuid, body string) *zim.Archive {
		a, err := zim.Open(writeLibraryZIM(t, uuid, body), zim.Options{})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = a.Close() })
		return a
	}
	first := open("aurago-uuid-0001", "<p>Erste Ausgabe.</p>")
	second := open("aurago-uuid-0002", "<p>Zweite Ausgabe.</p>")
	for _, tc := range []struct {
		a    *zim.Archive
		want string
	}{{first, "Erste Ausgabe."}, {second, "Zweite Ausgabe."}, {first, "Erste Ausgabe."}} {
		art, err := (searchIndex{store: zimStore{a: tc.a}}).read(ctx, ReadRequest{Path: "Ulm"})
		if err != nil || art.Content != "# Ulm\n\n"+tc.want {
			t.Fatalf("read = %q, %v; want %q (another UUID must miss the cache)", art.Content, err, tc.want)
		}
	}
	if len(renderCache.keys) != 2 {
		t.Fatalf("cache keys = %v, want one per edition", renderCache.keys)
	}
	if a, b := (zimStore{a: first, owner: 1}).cacheKey("Ulm"), (zimStore{a: first, owner: 2}).cacheKey("Ulm"); a == b {
		t.Fatalf("two libraries share the cache key %q", a)
	}
}

func TestLibraryCloseDropsItsRenderedArticles(t *testing.T) {
	useFreshRenderCache(t)
	path := writeLibraryZIM(t, "aurago-uuid-0003", "<p>Text.</p>")
	lib, err := OpenLibrary(path, Edition{Name: "ulm"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := OpenLibrary(path, Edition{Name: "ulm"})
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	for _, l := range []*Library{lib, other} {
		if _, err := l.Read(context.Background(), ReadRequest{Path: "Ulm"}); err != nil {
			t.Fatal(err)
		}
	}
	if len(renderCache.keys) != 2 {
		t.Fatalf("cache keys = %v, want one per library", renderCache.keys)
	}
	if err := lib.Close(); err != nil {
		t.Fatal(err)
	}
	otherPrefix := zimStore{a: other.archive, owner: other.cacheID}.cachePrefix()
	if len(renderCache.keys) != 1 || !strings.HasPrefix(renderCache.keys[0], otherPrefix) {
		t.Fatalf("cache keys after Close = %v, want only the other library's", renderCache.keys)
	}
}

func TestReadRejectsNonArticles(t *testing.T) {
	useFreshRenderCache(t)
	a, err := zim.Open(writeLibraryZIM(t, "aurago-uuid-0004", "<p>Text.</p>"), zim.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	ix := searchIndex{store: zimStore{a: a}}
	for _, req := range []ReadRequest{{Path: "Logo.png"}, {Path: "Wappen"}, {Title: "Wappen"}} {
		if _, err := ix.read(context.Background(), req); !errors.Is(err, ErrNotArticle) {
			t.Fatalf("read %+v: err = %v, want ErrNotArticle", req, err)
		}
	}
	if _, err := ix.lead(context.Background(), "Logo.png"); !errors.Is(err, ErrNotArticle) {
		t.Fatalf("lead of an image: err = %v", err)
	}
}

func TestReadPagesThroughRealArchives(t *testing.T) {
	useFreshRenderCache(t)
	ctx := context.Background()

	// A fixture article fits one page; an offset into it returns the rest.
	lib := openFixtureLibrary(t, "fixture_de.zim", "de")
	whole, err := lib.Read(ctx, ReadRequest{Path: "Berlin"})
	if err != nil || whole.NextOffset != nil || !strings.HasPrefix(whole.Content, "# Berlin\n\n") {
		t.Fatalf("read Berlin: %+v, %v", whole, err)
	}
	runes := []rune(whole.Content)
	tail, err := lib.Read(ctx, ReadRequest{Path: "Berlin", Offset: 10})
	if err != nil || tail.NextOffset != nil || tail.Content != strings.TrimLeft(string(runes[10:]), "\n") {
		t.Fatalf("read Berlin at 10: %q, %v", tail.Content, err)
	}
	if _, err := lib.Read(ctx, ReadRequest{Path: "Berlin", Offset: len(runes)}); !errors.Is(err, ErrOffsetOutOfRange) {
		t.Fatalf("offset at the end: err = %v", err)
	}

	// A long article in a real archive pages with NextOffset until the end.
	var body strings.Builder
	body.WriteString("<p>Ulm liegt an der Donau.</p><h2>Geschichte</h2>")
	for i := range 80 {
		fmt.Fprintf(&body, "<p>Absatz %d: %s Ende.</p>", i, strings.Repeat("Münster und Donau ", 12))
	}
	long, err := OpenLibrary(writeLibraryZIM(t, "aurago-uuid-0005", body.String()), Edition{Name: "ulm"})
	if err != nil {
		t.Fatal(err)
	}
	defer long.Close()
	art, err := searchIndex{store: zimStore{a: long.archive, owner: long.cacheID}}.rendered(ctx, mustEntry(t, long.archive, "Ulm"))
	if err != nil {
		t.Fatal(err)
	}
	for section, full := range map[string]string{"": art.fullMarkdown(), "Geschichte": art.sectionMarkdown(1)} {
		var pages []string
		offset := 0
		for {
			page, err := long.Read(ctx, ReadRequest{Path: "Ulm", Section: section, Offset: offset})
			if err != nil {
				t.Fatalf("section %q offset %d: %v", section, offset, err)
			}
			if n := utf8.RuneCountInString(page.Content); n > readPageRunes || n == 0 {
				t.Fatalf("section %q offset %d: page of %d runes", section, offset, n)
			}
			pages = append(pages, page.Content)
			if page.NextOffset == nil {
				break
			}
			if *page.NextOffset <= offset {
				t.Fatalf("next offset %d does not advance from %d", *page.NextOffset, offset)
			}
			offset = *page.NextOffset
		}
		if len(pages) < 2 {
			t.Fatalf("section %q: %d pages, want several", section, len(pages))
		}
		if got := strings.Join(pages, "\n\n"); got != full {
			t.Fatalf("section %q: the pages joined differ from the article (%d vs %d bytes)", section, len(got), len(full))
		}
	}
}

func mustEntry(t *testing.T, a *zim.Archive, path string) zim.Entry {
	t.Helper()
	e, err := a.EntryByPath(a.ContentNamespace(), path)
	if err != nil {
		t.Fatal(err)
	}
	return e
}
