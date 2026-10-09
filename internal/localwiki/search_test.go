package localwiki

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"aurago/internal/zim/xapian"
)

func cityArticles() *fakeArticleStore {
	return newFakeArticleStore(
		fakeEntry{path: "Berlin", title: "Berlin", html: htmlPage("Berlin",
			`<div id="mf-section-0"><p><b>Berlin</b> ist die Hauptstadt der Bundesrepublik Deutschland. Die Stadt liegt an der Spree.</p></div>`+
				`<h2 class="section-heading"><span class="mw-headline">Geschichte</span></h2><div id="mf-section-1"><p>Berlin wurde 1237 erstmals erwähnt.</p></div>`)},
		fakeEntry{path: "Bern", title: "Bern", html: htmlPage("Bern", `<p>Bern ist die Bundesstadt der Schweiz.</p>`)},
		fakeEntry{path: "Hamburg", title: "Hamburg", html: htmlPage("Hamburg", `<p>Hamburg hat einen großen Hafen. Die Hauptstadt der Hanse war Lübeck.</p>`)},
		fakeEntry{path: "Hauptstadt", title: "Hauptstadt Deutschlands", redirectTo: "Berlin"},
	)
}

func hitPaths(hits []SearchHit) []string {
	var out []string
	for _, h := range hits {
		out = append(out, h.Path)
	}
	return out
}

func TestSearchPutsExactTitleBeforeFulltextHits(t *testing.T) {
	ft := &fakeFulltextIndex{freq: map[string]uint32{"berlin": 2}, and: []xapian.Hit{{DocID: 3, Path: "Hamburg"}, {DocID: 1, Path: "Berlin"}}}
	ix := searchIndex{store: cityArticles(), fulltext: ft}
	hits, err := ix.search(context.Background(), "berlin", 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := hitPaths(hits); !reflect.DeepEqual(got, []string{"Berlin", "Hamburg"}) {
		t.Fatalf("paths = %v, want exact title first then BM25 order without duplicates", got)
	}
	if hits[0].Title != "Berlin" {
		t.Fatalf("title = %q", hits[0].Title)
	}
}

func TestSearchTreatsRedirectTitleAsExactMatch(t *testing.T) {
	titles := fakeTitleIndex{hits: map[string][]xapian.Hit{
		"Hauptstadt Deutschlands": {{Path: "Hamburg", Title: "Hamburg"}, {Path: "Hauptstadt", Title: "Hauptstadt Deutschlands"}},
	}}
	ix := searchIndex{store: cityArticles(), titles: titles}
	hits, err := ix.search(context.Background(), "Hauptstadt  Deutschlands", 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := hitPaths(hits); !reflect.DeepEqual(got, []string{"Berlin", "Hamburg"}) {
		t.Fatalf("paths = %v, want the redirect target first and each article once", got)
	}
}

func TestSearchDropsZeroPostingTermsAndFillsWithOr(t *testing.T) {
	ft := &fakeFulltextIndex{
		freq: map[string]uint32{"die": 0, "hauptstadt": 2, "hafen": 1},
		and:  []xapian.Hit{{DocID: 1, Path: "Berlin"}},
		or:   []xapian.Hit{{DocID: 1, Path: "Berlin"}, {DocID: 3, Path: "Hamburg"}, {DocID: 2, Path: "Bern"}},
	}
	ix := searchIndex{store: cityArticles(), fulltext: ft}
	hits, err := ix.search(context.Background(), "die Hauptstadt Hafen", 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := hitPaths(hits); !reflect.DeepEqual(got, []string{"Berlin", "Hamburg", "Bern"}) {
		t.Fatalf("paths = %v, want AND hit then OR fill", got)
	}
	if len(ft.searches) != 2 || ft.searches[0].op != xapian.OpAnd || ft.searches[1].op != xapian.OpOr {
		t.Fatalf("searches = %+v, want AND then OR", ft.searches)
	}
	if want := []string{"hauptstadt", "hafen"}; !reflect.DeepEqual(ft.searches[0].terms, want) {
		t.Fatalf("terms = %v, want %v (zero-posting term dropped)", ft.searches[0].terms, want)
	}
}

func TestSearchSkipsFulltextWhenNoTermHasPostings(t *testing.T) {
	ft := &fakeFulltextIndex{freq: map[string]uint32{}}
	ix := searchIndex{store: cityArticles(), fulltext: ft}
	hits, err := ix.search(context.Background(), "der die das", 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(ft.searches) != 0 || len(hits) != 0 {
		t.Fatalf("searches = %+v hits = %v, want none", ft.searches, hits)
	}
}

func TestSearchTitleOnlyModeUsesPrefixFallback(t *testing.T) {
	ix := searchIndex{store: cityArticles()}
	hits, err := ix.search(context.Background(), "ber", 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := hitPaths(hits); !reflect.DeepEqual(got, []string{"Berlin", "Bern"}) {
		t.Fatalf("paths = %v, want the capitalised prefix matches", got)
	}
}

func TestSearchFallsBackToPrefixWhenTitleIndexFails(t *testing.T) {
	ix := searchIndex{store: cityArticles(), titles: fakeTitleIndex{err: xapian.ErrUnsupportedFormat}}
	hits, err := ix.search(context.Background(), "Ham", 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := hitPaths(hits); !reflect.DeepEqual(got, []string{"Hamburg"}) {
		t.Fatalf("paths = %v", got)
	}
}

func TestSearchAddsSnippetsAndLeadsOnlyForTopResults(t *testing.T) {
	ft := &fakeFulltextIndex{freq: map[string]uint32{"hauptstadt": 2}, and: []xapian.Hit{{DocID: 1, Path: "Berlin"}, {DocID: 3, Path: "Hamburg"}}}
	ix := searchIndex{store: cityArticles(), fulltext: ft}
	hits, err := ix.search(context.Background(), "Hauptstadt", 5, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Fatalf("hits = %+v", hits)
	}
	if hits[0].Snippet != "Berlin ist die Hauptstadt der Bundesrepublik Deutschland." {
		t.Fatalf("snippet = %q", hits[0].Snippet)
	}
	if hits[1].Snippet != "Die Hauptstadt der Hanse war Lübeck." {
		t.Fatalf("second snippet = %q", hits[1].Snippet)
	}
	if !strings.HasPrefix(hits[0].Lead, "**Berlin** ist die Hauptstadt") || strings.Contains(hits[0].Lead, "Geschichte") {
		t.Fatalf("lead = %q, want only the lead section", hits[0].Lead)
	}
	if hits[1].Lead != "" {
		t.Fatalf("second result has a lead: %q", hits[1].Lead)
	}
}

func TestSearchValidatesQueryLimits(t *testing.T) {
	ix := searchIndex{store: cityArticles()}
	for _, tc := range []struct {
		query string
		want  error
	}{
		{"   ", ErrQueryEmpty},
		{strings.Repeat("wort ", 17), ErrQueryTooLong},
		{strings.Repeat("x", 201), ErrQueryTooLong},
	} {
		if _, err := ix.search(context.Background(), tc.query, 5, 0); !errors.Is(err, tc.want) {
			t.Fatalf("query %q: err = %v, want %v", tc.query, err, tc.want)
		}
	}
	if _, err := ix.search(context.Background(), strings.Repeat("x", 200), 5, 0); err != nil {
		t.Fatalf("200-character query rejected: %v", err)
	}
}

func TestClampLimit(t *testing.T) {
	for _, tc := range []struct{ in, max, want int }{{0, 10, 5}, {-3, 10, 5}, {3, 10, 3}, {50, 10, 10}} {
		if got := clampLimit(tc.in, tc.max); got != tc.want {
			t.Fatalf("clampLimit(%d, %d) = %d, want %d", tc.in, tc.max, got, tc.want)
		}
	}
}

func TestBoundedCallRejectsWhenAllSlotsAreBusy(t *testing.T) {
	for range maxConcurrentSearches {
		searchSlots <- struct{}{}
	}
	defer func() {
		for range maxConcurrentSearches {
			<-searchSlots
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := boundedCall(ctx, func(context.Context) (int, error) { return 1, nil })
	if !errors.Is(err, ErrSearchBusy) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want ErrSearchBusy wrapping the deadline", err)
	}
}

func TestBoundedCallAppliesSearchTimeout(t *testing.T) {
	left, err := boundedCall(context.Background(), func(ctx context.Context) (time.Duration, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			return 0, errors.New("no deadline")
		}
		return time.Until(deadline), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if left <= 0 || left > searchTimeout {
		t.Fatalf("deadline in %v, want within %v", left, searchTimeout)
	}
}

func TestSuggestResolvesAndDedupesTitles(t *testing.T) {
	titles := fakeTitleIndex{hits: map[string][]xapian.Hit{
		"Haupt": {{Path: "Hauptstadt", Title: "Hauptstadt Deutschlands"}, {Path: "Berlin", Title: "Berlin"}},
	}}
	ix := searchIndex{store: cityArticles(), titles: titles}
	refs, err := ix.suggest(context.Background(), "Haupt", 10)
	if err != nil {
		t.Fatal(err)
	}
	if want := []Ref{{Title: "Berlin", Path: "Berlin"}}; !reflect.DeepEqual(refs, want) {
		t.Fatalf("refs = %+v, want %+v", refs, want)
	}
}

// A one-character query goes to the cheap titleOrdered prefix scan, not to the
// title index (slice 2 contract note 7: a one-letter prefix expands to the 100
// most frequent terms there).
func TestSuggestSingleCharacterUsesTitlePrefix(t *testing.T) {
	titles := fakeTitleIndex{hits: map[string][]xapian.Hit{"h": {{Path: "Bern", Title: "Bern"}}}}
	ix := searchIndex{store: cityArticles(), titles: titles}
	refs, err := ix.suggest(context.Background(), "h", 10)
	if err != nil {
		t.Fatal(err)
	}
	if want := []Ref{{Title: "Hamburg", Path: "Hamburg"}, {Title: "Berlin", Path: "Berlin"}}; !reflect.DeepEqual(refs, want) {
		t.Fatalf("refs = %+v, want %+v (prefix scan as typed and capitalised)", refs, want)
	}
}

func TestSearchCapsLeadsAtThree(t *testing.T) {
	var articles []fakeEntry
	var and []xapian.Hit
	for i, name := range []string{"Aachen", "Bonn", "Celle", "Dessau", "Essen"} {
		articles = append(articles, fakeEntry{path: name, title: name, html: htmlPage(name, `<p>`+name+` ist eine Stadt.</p><h2>Geschichte</h2><p>Alt.</p>`)})
		and = append(and, xapian.Hit{DocID: uint32(i + 1), Path: name})
	}
	ft := &fakeFulltextIndex{freq: map[string]uint32{"stadt": 5}, and: and}
	ix := searchIndex{store: newFakeArticleStore(articles...), fulltext: ft}
	hits, err := ix.search(context.Background(), "Stadt", 5, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 5 {
		t.Fatalf("hits = %+v", hits)
	}
	for i, h := range hits {
		if wantLead := i < maxSearchLeads; (h.Lead != "") != wantLead || h.Snippet == "" {
			t.Fatalf("hit %d (%s): lead %q snippet %q, want a lead only for the first %d", i, h.Path, h.Lead, h.Snippet, maxSearchLeads)
		}
	}
	if hits[0].Lead != "Aachen ist eine Stadt." {
		t.Fatalf("lead = %q", hits[0].Lead)
	}
}

func TestSearchKeepsHitsWhenTimeRunsOutDuringSnippets(t *testing.T) {
	store := cityArticles()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store.afterRead = cancel // the deadline passes while the first snippet is extracted
	ft := &fakeFulltextIndex{freq: map[string]uint32{"hauptstadt": 2}, and: []xapian.Hit{{DocID: 1, Path: "Berlin"}, {DocID: 3, Path: "Hamburg"}}}
	ix := searchIndex{store: store, fulltext: ft}
	hits, err := ix.search(ctx, "Hauptstadt", 5, 3)
	if err != nil {
		t.Fatalf("err = %v, want the hits found so far", err)
	}
	if got := hitPaths(hits); !reflect.DeepEqual(got, []string{"Berlin", "Hamburg"}) {
		t.Fatalf("paths = %v", got)
	}
	if hits[0].Snippet == "" || hits[0].Lead == "" {
		t.Fatalf("first hit lost its snippet or lead: %+v", hits[0])
	}
	if hits[1].Ref != (Ref{Title: "Hamburg", Path: "Hamburg"}) || hits[1].Snippet != "" || hits[1].Lead != "" {
		t.Fatalf("second hit = %+v, want title and path without snippet", hits[1])
	}
	if store.reads != 1 {
		t.Fatalf("reads = %d, want no reads after the deadline", store.reads)
	}
}

func TestTitlePrefixFallbackScansACapitalisedQueryOnce(t *testing.T) {
	for _, tc := range []struct {
		query string
		want  []string
	}{
		{"Ham", []string{"Ham"}},
		{"ham", []string{"ham", "Ham"}},
		{"東京", []string{"東京"}},
	} {
		store := cityArticles()
		ix := searchIndex{store: store}
		if _, err := ix.titleHits(context.Background(), tc.query, 5); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(store.prefixes, tc.want) {
			t.Fatalf("titleHits(%q) scanned %q, want %q", tc.query, store.prefixes, tc.want)
		}
	}
}
