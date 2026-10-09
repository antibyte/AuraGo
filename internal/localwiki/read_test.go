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
)

// useFreshRenderCache swaps the process-wide render cache for an empty one for
// the duration of the test: fakes and zimtest archives reuse cache keys.
func useFreshRenderCache(t *testing.T) {
	t.Helper()
	saved := renderCache
	renderCache = newArticleCache(renderCacheSize, renderCacheBytes)
	t.Cleanup(func() { renderCache = saved })
}

func TestReadByPathReturnsMarkdownAndSections(t *testing.T) {
	useFreshRenderCache(t)
	ix := searchIndex{store: cityArticles()}
	art, err := ix.read(context.Background(), ReadRequest{Path: "Berlin"})
	if err != nil {
		t.Fatal(err)
	}
	want := "# Berlin\n\n**Berlin** ist die Hauptstadt der Bundesrepublik Deutschland. Die Stadt liegt an der Spree.\n\n## Geschichte\n\nBerlin wurde 1237 erstmals erwähnt."
	if art.Content != want {
		t.Fatalf("content = %q\nwant      %q", art.Content, want)
	}
	if art.Title != "Berlin" || art.Path != "Berlin" || art.RedirectedFrom != "" || art.NextOffset != nil {
		t.Fatalf("article = %+v", art)
	}
	if len(art.Sections) != 2 || art.Sections[1] != (Section{Index: 1, Heading: "Geschichte", Level: 2, Chars: utf8.RuneCountInString("## Geschichte\n\nBerlin wurde 1237 erstmals erwähnt.")}) {
		t.Fatalf("sections = %+v", art.Sections)
	}
}

func TestReadByTitleFollowsRedirect(t *testing.T) {
	useFreshRenderCache(t)
	titles := fakeTitleIndex{hits: map[string][]xapian.Hit{
		"hauptstadt deutschlands": {{Path: "Hauptstadt", Title: "Hauptstadt Deutschlands"}},
	}}
	ix := searchIndex{store: cityArticles(), titles: titles}
	art, err := ix.read(context.Background(), ReadRequest{Title: "hauptstadt deutschlands"})
	if err != nil {
		t.Fatal(err)
	}
	if art.Path != "Berlin" || art.RedirectedFrom != "Hauptstadt Deutschlands" {
		t.Fatalf("article = %+v, want Berlin redirected from the redirect title", art.Ref)
	}
}

func TestReadAcceptsIsolatedAndPrefixedReferences(t *testing.T) {
	useFreshRenderCache(t)
	ix := searchIndex{store: cityArticles()}
	for _, path := range []string{"<external_data>\nBerlin\n</external_data>", "C/Berlin", "/Berlin", " Berlin "} {
		art, err := ix.read(context.Background(), ReadRequest{Path: path})
		if err != nil || art.Path != "Berlin" {
			t.Fatalf("path %q: article %+v err %v", path, art.Ref, err)
		}
	}
}

func TestReadSelectsSectionByHeadingOrIndex(t *testing.T) {
	useFreshRenderCache(t)
	ix := searchIndex{store: cityArticles()}
	for _, spec := range []string{"Geschichte", "geschichte", "1", "Gesch"} {
		art, err := ix.read(context.Background(), ReadRequest{Path: "Berlin", Section: spec})
		if err != nil {
			t.Fatalf("section %q: %v", spec, err)
		}
		if art.Content != "## Geschichte\n\nBerlin wurde 1237 erstmals erwähnt." {
			t.Fatalf("section %q: content = %q", spec, art.Content)
		}
	}
	lead, err := ix.read(context.Background(), ReadRequest{Path: "Berlin", Section: "0"})
	if err != nil || !strings.HasPrefix(lead.Content, "**Berlin** ist") || strings.Contains(lead.Content, "Geschichte") {
		t.Fatalf("lead section = %q err %v", lead.Content, err)
	}
}

func TestReadUnknownSectionListsAvailableSections(t *testing.T) {
	useFreshRenderCache(t)
	ix := searchIndex{store: cityArticles()}
	_, err := ix.read(context.Background(), ReadRequest{Path: "Berlin", Section: "Wirtschaft"})
	var notFound *SectionNotFoundError
	if !errors.As(err, &notFound) || !errors.Is(err, ErrSectionNotFound) {
		t.Fatalf("err = %v, want SectionNotFoundError", err)
	}
	if len(notFound.Sections) != 2 || notFound.Sections[1].Heading != "Geschichte" {
		t.Fatalf("sections = %+v", notFound.Sections)
	}
	if _, err := ix.read(context.Background(), ReadRequest{Path: "Berlin", Section: "7"}); !errors.Is(err, ErrSectionNotFound) {
		t.Fatalf("index out of range: err = %v", err)
	}
}

func TestReadPagesLongArticles(t *testing.T) {
	useFreshRenderCache(t)
	var body strings.Builder
	for i := 0; i < 60; i++ {
		body.WriteString("<p>" + strings.Repeat("Wort ", 40) + "Ende.</p>")
	}
	store := newFakeArticleStore(fakeEntry{path: "Lang", title: "Lang", html: htmlPage("Lang", body.String())})
	ix := searchIndex{store: store}
	var pages []string
	offset := 0
	for {
		art, err := ix.read(context.Background(), ReadRequest{Path: "Lang", Offset: offset})
		if err != nil {
			t.Fatal(err)
		}
		if n := utf8.RuneCountInString(art.Content); n > readPageRunes {
			t.Fatalf("page has %d runes", n)
		}
		pages = append(pages, art.Content)
		if art.NextOffset == nil {
			break
		}
		if *art.NextOffset <= offset {
			t.Fatalf("next offset %d does not advance from %d", *art.NextOffset, offset)
		}
		offset = *art.NextOffset
	}
	if len(pages) < 2 {
		t.Fatalf("pages = %d, want several", len(pages))
	}
	for i, p := range pages[:len(pages)-1] {
		if !strings.HasSuffix(p, "Ende.") {
			t.Fatalf("page %d is not cut at a paragraph end: %q", i, p[len(p)-20:])
		}
	}
	if _, err := ix.read(context.Background(), ReadRequest{Path: "Lang", Offset: 1 << 20}); !errors.Is(err, ErrOffsetOutOfRange) {
		t.Fatalf("offset past the end: err = %v", err)
	}
	if _, err := ix.read(context.Background(), ReadRequest{Path: "Lang", Offset: -1}); !errors.Is(err, ErrOffsetOutOfRange) {
		t.Fatalf("negative offset: err = %v", err)
	}
}

// A smaller page size (the agent tool's inline budget) pages the same text:
// pages of any size chain through rune offsets, also in a multi-byte script.
func TestReadHonoursPageRunes(t *testing.T) {
	useFreshRenderCache(t)
	var body strings.Builder
	for i := 0; i < 60; i++ {
		body.WriteString("<p>" + strings.Repeat("भारत गणराज्य ", 20) + "अंत।</p>")
	}
	store := newFakeArticleStore(fakeEntry{path: "भारत", title: "भारत", html: htmlPage("भारत", body.String())})
	ix := searchIndex{store: store}
	full, err := ix.read(context.Background(), ReadRequest{Path: "भारत", PageRunes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	if n := utf8.RuneCountInString(full.Content); n > readPageRunes || full.NextOffset == nil {
		t.Fatalf("an oversized page size must stay at %d runes: got %d, next %v", readPageRunes, n, full.NextOffset)
	}
	var pages []string
	offset := 0
	for size := 300; ; size = 300 + (size+700)%1500 {
		art, err := ix.read(context.Background(), ReadRequest{Path: "भारत", Offset: offset, PageRunes: size})
		if err != nil {
			t.Fatal(err)
		}
		if n := utf8.RuneCountInString(art.Content); n > size || n == 0 {
			t.Fatalf("page at %d has %d runes, asked for %d", offset, n, size)
		}
		pages = append(pages, art.Content)
		if art.NextOffset == nil {
			break
		}
		if *art.NextOffset <= offset {
			t.Fatalf("next offset %d does not advance from %d", *art.NextOffset, offset)
		}
		offset = *art.NextOffset
	}
	if len(pages) < 10 {
		t.Fatalf("pages = %d, want many small ones", len(pages))
	}
	joined := strings.Join(pages, "\n\n")
	if got, want := strings.Count(joined, "अंत।"), 60; got != want {
		t.Fatalf("small pages hold %d paragraph ends, want %d (nothing lost or repeated)", got, want)
	}
	tiny, err := ix.read(context.Background(), ReadRequest{Path: "भारत", PageRunes: 1})
	if err != nil || utf8.RuneCountInString(tiny.Content) > minPageRunes || utf8.RuneCountInString(tiny.Content) < minPageRunes/2 {
		t.Fatalf("a tiny page size must be raised to %d runes: %d runes, err %v", minPageRunes, utf8.RuneCountInString(tiny.Content), err)
	}
}

func TestReadReportsMissingArticles(t *testing.T) {
	useFreshRenderCache(t)
	ix := searchIndex{store: cityArticles()}
	for _, req := range []ReadRequest{{Path: "Paris"}, {Title: "Paris"}, {}} {
		if _, err := ix.read(context.Background(), req); !errors.Is(err, ErrArticleNotFound) {
			t.Fatalf("request %+v: err = %v", req, err)
		}
	}
	broken := newFakeArticleStore(fakeEntry{path: "Loop", title: "Loop", redirectTo: "Loop"})
	if _, err := (searchIndex{store: broken}).read(context.Background(), ReadRequest{Path: "Loop"}); !errors.Is(err, ErrArticleNotFound) {
		t.Fatalf("redirect loop: err = %v", err)
	}
}

func TestReadCachesRenderedArticles(t *testing.T) {
	useFreshRenderCache(t)
	store := cityArticles()
	ix := searchIndex{store: store}
	for range 3 {
		if _, err := ix.read(context.Background(), ReadRequest{Path: "Bern", Section: "0"}); err != nil {
			t.Fatal(err)
		}
	}
	if store.reads != 1 {
		t.Fatalf("article read %d times, want 1 (cached render)", store.reads)
	}
}

func TestArticleCacheEvictsLeastRecentlyUsed(t *testing.T) {
	c := newArticleCache(2, renderCacheBytes)
	a, b, d := &renderedArticle{title: "a"}, &renderedArticle{title: "b"}, &renderedArticle{title: "d"}
	c.put("a", a)
	c.put("b", b)
	c.get("a")
	c.put("d", d)
	if _, ok := c.get("b"); ok {
		t.Fatal("b should be evicted")
	}
	if got, ok := c.get("a"); !ok || got != a {
		t.Fatal("a should stay cached")
	}
}

// closedStore is an articleStore whose archive was closed by an edition swap:
// lookups and reads fail with zim.ErrClosed.
type closedStore struct{ *fakeArticleStore }

func (closedStore) lookup(string) (zim.Entry, error)             { return zim.Entry{}, zim.ErrClosed }
func (closedStore) titlePrefix(string, int) ([]zim.Entry, error) { return nil, zim.ErrClosed }

// closedAfterLookupStore finds entries but fails when it reads their content:
// the archive is closed between the two steps.
type closedAfterLookupStore struct{ *fakeArticleStore }

func (closedAfterLookupStore) readHTML(zim.Entry) ([]byte, error) {
	return nil, fmt.Errorf("read article: %w", zim.ErrClosed)
}

func TestReadPassesOnClosedArchive(t *testing.T) {
	useFreshRenderCache(t)
	ix := searchIndex{store: closedStore{cityArticles()}}
	for _, req := range []ReadRequest{{Path: "Berlin"}, {Title: "Berlin"}} {
		if _, err := ix.read(context.Background(), req); !errors.Is(err, zim.ErrClosed) {
			t.Fatalf("request %+v: err = %v, want zim.ErrClosed (not an ordinary miss)", req, err)
		}
	}
	late := searchIndex{store: closedAfterLookupStore{cityArticles()}}
	if _, err := late.read(context.Background(), ReadRequest{Path: "Berlin"}); !errors.Is(err, zim.ErrClosed) {
		t.Fatalf("read after close: err = %v, want zim.ErrClosed", err)
	}
	if _, err := late.lead(context.Background(), "Berlin"); !errors.Is(err, zim.ErrClosed) {
		t.Fatalf("lead after close: err = %v, want zim.ErrClosed", err)
	}
}

func TestLeadFollowsRedirectsAndStopsBeforeTheFirstHeading(t *testing.T) {
	ix := searchIndex{store: cityArticles()}
	for _, path := range []string{"Berlin", "Hauptstadt", "C/Berlin"} {
		lead, err := ix.lead(context.Background(), path)
		if err != nil {
			t.Fatalf("lead %q: %v", path, err)
		}
		if lead != "**Berlin** ist die Hauptstadt der Bundesrepublik Deutschland. Die Stadt liegt an der Spree." {
			t.Fatalf("lead %q = %q", path, lead)
		}
	}
	if _, err := ix.lead(context.Background(), "Paris"); !errors.Is(err, ErrArticleNotFound) {
		t.Fatalf("missing article: err = %v", err)
	}
}
