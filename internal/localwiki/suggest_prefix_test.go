package localwiki

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	"aurago/internal/zim/xapian"
)

func refTitles(refs []Ref) []string {
	var out []string
	for _, r := range refs {
		out = append(out, r.Title)
	}
	return out
}

// berlArticles mirrors the live acceptance: the title index ranks titles that
// contain "Berl" as a whole word first, so "Berlin" fell out of the top 10.
func berlArticles() *fakeArticleStore {
	page := func(title string) string { return htmlPage(title, "<p>"+title+".</p>") }
	return newFakeArticleStore(
		fakeEntry{path: "Berl", title: "Berl", html: page("Berl")},
		fakeEntry{path: "Berl_Broder", title: "Berl Broder", html: page("Berl Broder")},
		fakeEntry{path: "Berla", title: "Berla", html: page("Berla")},
		fakeEntry{path: "Berlin", title: "Berlin", html: page("Berlin")},
		fakeEntry{path: "Berlin_(Stadt)", title: "Berlin (Stadt)", redirectTo: "Berlin"},
		fakeEntry{path: "Berliner", title: "Berliner", html: page("Berliner")},
		fakeEntry{path: "Antonie_Berl", title: "Antonie Berl", html: page("Antonie Berl")},
		fakeEntry{path: "Beit_Berl", title: "Beit Berl", html: page("Beit Berl")},
		fakeEntry{path: "Bild", title: "Bild", html: page("Bild")},
	)
}

func berlTitleIndex() fakeTitleIndex {
	libzimOrder := []xapian.Hit{{Path: "Berl"}, {Path: "Berl_Broder"}, {Path: "Antonie_Berl"}, {Path: "Beit_Berl"}}
	return fakeTitleIndex{
		hits: map[string][]xapian.Hit{"Berl": libzimOrder, "berl": libzimOrder, "Berl Bro": {{Path: "Berl_Broder"}}},
		// The most frequent completions; "bild" is not one of "berl" and is
		// never asked for.
		terms: map[string][]string{"berl": {"berlin", "berliner", "berlinale"}},
	}
}

// A single typed word lists the titles that start with it first: the exact
// title, the titles of its most frequent completions, the other titles in
// title order (a redirect counts as its target), then the title index's hits.
func TestSuggestPutsTitlesStartingWithTheWordFirst(t *testing.T) {
	ix := searchIndex{store: berlArticles(), titles: berlTitleIndex()}
	want := []string{"Berl", "Berlin", "Berliner", "Berl Broder", "Berla", "Antonie Berl", "Beit Berl"}
	for _, query := range []string{"Berl", "berl", "  Berl "} {
		refs, err := ix.suggest(context.Background(), query, 10)
		if err != nil {
			t.Fatal(err)
		}
		if got := refTitles(refs); !reflect.DeepEqual(got, want) {
			t.Fatalf("suggest(%q) = %v, want %v", query, got, want)
		}
	}
	refs, err := ix.suggest(context.Background(), "Berl", 3)
	if err != nil {
		t.Fatal(err)
	}
	if got := refTitles(refs); !reflect.DeepEqual(got, want[:3]) {
		t.Fatalf("limit 3 = %v, want %v", got, want[:3])
	}
	// Several words keep the title index's ranking.
	refs, err = ix.suggest(context.Background(), "Berl Bro", 10)
	if err != nil {
		t.Fatal(err)
	}
	if got := refTitles(refs); !reflect.DeepEqual(got, []string{"Berl Broder"}) {
		t.Fatalf("two words = %v", got)
	}
}

// Without a title index, or when it fails, the title-ordered titles still come
// first; one character never asks the index for completions.
func TestSuggestPrefixTitlesWithoutCompletions(t *testing.T) {
	broken := berlTitleIndex()
	broken.err = errors.New("damaged index")
	for name, titles := range map[string]titleIndex{"no index": nil, "failing index": broken} {
		ix := searchIndex{store: berlArticles(), titles: titles}
		refs, err := ix.suggest(context.Background(), "Berl", 10)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got, want := refTitles(refs), []string{"Berl", "Berl Broder", "Berla", "Berlin", "Berliner"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: %v, want %v", name, got, want)
		}
	}
	index := berlTitleIndex()
	index.terms = map[string][]string{"b": {"berlin"}}
	ix := searchIndex{store: berlArticles(), titles: index}
	refs, err := ix.suggest(context.Background(), "b", 10)
	if err != nil {
		t.Fatal(err)
	}
	if got := refTitles(refs); got[0] != "Beit Berl" || slices.Index(got, "Berlin") < slices.Index(got, "Berl Broder") {
		t.Fatalf("one character must stay in title order: %v", got)
	}
}

func TestCompletionPathsKeepTheTypedSpelling(t *testing.T) {
	for _, tc := range []struct {
		word, term string
		typed      []string
		folded     string
	}{
		{"Berl", "berlin", []string{"Berlin"}, ""},
		{"berl", "berlin", []string{"berlin", "Berlin"}, ""},
		{"Mün", "munchen", []string{"München"}, "Munchen"},
		{"BERL", "berlin", []string{"BERLin"}, "Berlin"},
		{"Mün", "other", nil, "Other"},
	} {
		typed, folded := completionPaths(tc.word, tc.term)
		if !reflect.DeepEqual(typed, tc.typed) || folded != tc.folded {
			t.Errorf("completionPaths(%q, %q) = %v, %q, want %v, %q", tc.word, tc.term, typed, folded, tc.typed, tc.folded)
		}
	}
}

// The folded spelling of a completion is only looked up when the typed one
// names no article: "Mün" lists München, not the redirect "Munchen" (to a
// person called Munchen) ahead of real prefix titles; "BERL" still reaches
// Berlin through it.
func TestSuggestTriesTheFoldedCompletionOnlyAsAFallback(t *testing.T) {
	page := func(title string) string { return htmlPage(title, "<p>"+title+".</p>") }
	store := newFakeArticleStore(
		fakeEntry{path: "München", title: "München", html: page("München")},
		fakeEntry{path: "Münster", title: "Münster", html: page("Münster")},
		fakeEntry{path: "Munchen", title: "Munchen", redirectTo: "Charles_Munchen"},
		fakeEntry{path: "Charles_Munchen", title: "Charles Munchen", html: page("Charles Munchen")},
		fakeEntry{path: "Berlin", title: "Berlin", html: page("Berlin")},
	)
	index := fakeTitleIndex{terms: map[string][]string{"mun": {"munchen", "munster"}, "berl": {"berlin"}}}
	ix := searchIndex{store: store, titles: index}
	refs, err := ix.suggest(context.Background(), "Mün", 10)
	if err != nil {
		t.Fatal(err)
	}
	if got := refTitles(refs); !reflect.DeepEqual(got, []string{"München", "Münster"}) {
		t.Fatalf("suggest(Mün) = %v, want München and Münster without the folded redirect", got)
	}
	refs, err = ix.suggest(context.Background(), "BERL", 10)
	if err != nil {
		t.Fatal(err)
	}
	if got := refTitles(refs); !reflect.DeepEqual(got, []string{"Berlin"}) {
		t.Fatalf("suggest(BERL) = %v, want Berlin through the folded completion", got)
	}
}

// When the prefix stage fills the list, the title index is not asked.
func TestSuggestSkipsTheTitleIndexWhenThePrefixStageIsFull(t *testing.T) {
	var calls int
	index := berlTitleIndex()
	index.suggests = &calls
	ix := searchIndex{store: berlArticles(), titles: index}
	refs, err := ix.suggest(context.Background(), "Berl", 3)
	if err != nil {
		t.Fatal(err)
	}
	if got := refTitles(refs); !reflect.DeepEqual(got, []string{"Berl", "Berlin", "Berliner"}) || calls != 0 {
		t.Fatalf("limit 3 = %v with %d index searches, want the prefix titles and none", got, calls)
	}
	if _, err := ix.suggest(context.Background(), "Berl", 10); err != nil || calls != 1 {
		t.Fatalf("limit 10: %d index searches, err %v, want one", calls, err)
	}
}

// On the real-index fixtures: completions reach titles with diacritics and
// redirects, exact and prefix titles come first, and every answer stays within
// the limit with display titles from the resolved entries.
func TestFixtureSuggestPrefersTitlesStartingWithTheWord(t *testing.T) {
	for _, tc := range []struct {
		file, language, query string
		first                 []string
	}{
		{"fixture_de.zim", "de", "Mün", []string{"München"}},
		{"fixture_de.zim", "de", "Mue", []string{"München"}},
		{"fixture_de.zim", "de", "Zug", []string{"Zug", "Zugspitze"}},
		{"fixture_de.zim", "de", "Ham", []string{"Hamburg"}},
		{"fixture_de.zim", "de", "Spree", []string{"Spree (Fluss)", "Berlin"}},
		{"fixture_en.zim", "en", "App", []string{"Apple", "Apple pie"}},
		{"fixture_en.zim", "en", "thames", []string{"River Thames"}},
		{"fixture_en.zim", "en", "Bri", []string{"Bridge"}},
	} {
		lib := openFixtureLibrary(t, tc.file, tc.language)
		refs, err := lib.Suggest(context.Background(), tc.query, 10)
		if err != nil {
			t.Fatalf("%s %q: %v", tc.file, tc.query, err)
		}
		got := refTitles(refs)
		if len(got) < len(tc.first) || !reflect.DeepEqual(got[:len(tc.first)], tc.first) || len(got) > maxSuggestions {
			t.Fatalf("%s %q = %v, want %v first", tc.file, tc.query, got, tc.first)
		}
		assertEntryTitles(t, lib, refs)
	}
}
