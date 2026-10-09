package localwiki

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/zim"
)

// openFixtureLibrary opens a slice-2 fixture ZIM (self-authored articles with
// real libzim full-text and title indexes) through OpenLibrary, the path
// production takes, so the analyzer language is chosen as the manager does.
func openFixtureLibrary(t *testing.T, name, language string) *Library {
	t.Helper()
	lib, err := OpenLibrary(filepath.Join("..", "zim", "testdata", name),
		Edition{Language: language, Variant: VariantNoPic, Date: "2026-10", Name: name})
	if err != nil {
		t.Fatalf("open fixture %s: %v", name, err)
	}
	t.Cleanup(func() { _ = lib.Close() })
	return lib
}

// fixtureArticles returns the non-redirect HTML front articles of a fixture.
func fixtureArticles(t *testing.T, lib *Library) []zim.Entry {
	t.Helper()
	var out []zim.Entry
	for i := 0; i < lib.archive.ArticleCount(); i++ {
		e, err := lib.archive.ArticleAt(i)
		if err != nil {
			t.Fatal(err)
		}
		if !e.IsRedirect && isHTMLEntry(e) {
			out = append(out, e)
		}
	}
	if len(out) == 0 {
		t.Fatal("fixture has no HTML articles")
	}
	return out
}

func TestFixtureSearchFindsEveryArticleByItsTitle(t *testing.T) {
	for _, fx := range []struct{ file, lang string }{{"fixture_de.zim", "de"}, {"fixture_en.zim", "en"}} {
		t.Run(fx.file, func(t *testing.T) {
			lib := openFixtureLibrary(t, fx.file, fx.lang)
			if !lib.Fulltext() {
				t.Fatal("fixture must have a supported full-text index")
			}
			for _, e := range fixtureArticles(t, lib) {
				res, err := lib.Search(context.Background(), e.Title, 5, 1)
				if err != nil {
					t.Fatalf("search %q: %v", e.Title, err)
				}
				if res.Edition != lib.Edition() || !res.Fulltext {
					t.Fatalf("result edition %+v fulltext %v", res.Edition, res.Fulltext)
				}
				if len(res.Results) == 0 || res.Results[0].Path != e.Path {
					t.Fatalf("search %q: results %+v, want %q first", e.Title, res.Results, e.Path)
				}
				if res.Results[0].Snippet == "" || res.Results[0].Lead == "" {
					t.Fatalf("search %q: first result lacks snippet or lead: %+v", e.Title, res.Results[0])
				}
				seen := map[string]bool{}
				for _, hit := range res.Results {
					if seen[hit.Path] {
						t.Fatalf("search %q: duplicate %q in %+v", e.Title, hit.Path, res.Results)
					}
					seen[hit.Path] = true
				}
			}
		})
	}
}

func TestFixtureReadAndSuggestEveryArticle(t *testing.T) {
	useFreshRenderCache(t)
	lib := openFixtureLibrary(t, "fixture_de.zim", "de")
	for _, e := range fixtureArticles(t, lib) {
		art, err := lib.Read(context.Background(), ReadRequest{Path: e.Path})
		if err != nil {
			t.Fatalf("read %q: %v", e.Path, err)
		}
		if art.Path != e.Path || !strings.HasPrefix(art.Content, "# "+e.Title) || len(art.Sections) == 0 {
			t.Fatalf("read %q: %+v", e.Path, art)
		}
		prefix := string([]rune(e.Title)[:min(3, len([]rune(e.Title)))])
		refs, err := lib.Suggest(context.Background(), prefix, 10)
		if err != nil {
			t.Fatalf("suggest %q: %v", prefix, err)
		}
		found := false
		for _, r := range refs {
			found = found || r.Path == e.Path
		}
		if !found {
			t.Fatalf("suggest %q: %+v lacks %q", prefix, refs, e.Path)
		}
	}
}

// The following tests use the German fixture articles of slice 2's corpus
// (scripts/localwiki/fixtures/corpus.py, listed in the slice-4 plan's contract
// notes): Berlin ("… Hauptstadt …"), Hamburg (the only body with "Hafen")
// and the front redirect C/Hauptstadt_Deutschlands "Hauptstadt Deutschlands" → Berlin.

func TestFixtureFulltextFindsBodyTermsAndDropsUnknownTerms(t *testing.T) {
	lib := openFixtureLibrary(t, "fixture_de.zim", "de")
	for _, query := range []string{"Hafen", "Hafen qqxyzzy"} {
		res, err := lib.Search(context.Background(), query, 5, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Results) == 0 || res.Results[0].Path != "Hamburg" || !strings.Contains(res.Results[0].Snippet, "Hafen") {
			t.Fatalf("search %q: %+v", query, res.Results)
		}
	}
}

func TestFixtureRedirectTitleResolvesToTarget(t *testing.T) {
	useFreshRenderCache(t)
	lib := openFixtureLibrary(t, "fixture_de.zim", "de")
	res, err := lib.Search(context.Background(), "Hauptstadt Deutschlands", 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Results) == 0 || res.Results[0].Path != "Berlin" || res.Results[0].Title != "Berlin" {
		t.Fatalf("results = %+v, want Berlin first", res.Results)
	}
	// Lower case misses every path candidate, so the title index (a redirect
	// hit) and the redirect resolution are exercised.
	art, err := lib.Read(context.Background(), ReadRequest{Title: "hauptstadt deutschlands"})
	if err != nil {
		t.Fatal(err)
	}
	if art.Path != "Berlin" || art.RedirectedFrom != "Hauptstadt Deutschlands" {
		t.Fatalf("article = %+v", art.Ref)
	}
}

func TestFixtureTitleOnlyModeWithoutIndexes(t *testing.T) {
	lib := openFixtureLibrary(t, "fixture_de.zim", "de")
	lib.fulltext, lib.title = nil, nil
	res, err := lib.Search(context.Background(), "Berlin", 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Fulltext || len(res.Results) == 0 || res.Results[0].Path != "Berlin" {
		t.Fatalf("degraded result = %+v", res)
	}
}

func TestFixtureReadReturnsTheArticleTextAndPages(t *testing.T) {
	useFreshRenderCache(t)
	lib := openFixtureLibrary(t, "fixture_de.zim", "de")
	art, err := lib.Read(context.Background(), ReadRequest{Path: "Berlin"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# Berlin\n\n",
		"Berlin ist die Hauptstadt Deutschlands und liegt an der Spree.",
		"Viele Brücken verbinden die Stadtteile.",
	} {
		if !strings.Contains(art.Content, want) {
			t.Fatalf("content lacks %q:\n%s", want, art.Content)
		}
	}
	if art.NextOffset != nil || art.RedirectedFrom != "" || art.Title != "Berlin" {
		t.Fatalf("article = %+v", art)
	}
	// A short article has a lead section only.
	if len(art.Sections) != 1 || art.Sections[0].Index != 0 || art.Sections[0].Level != 0 {
		t.Fatalf("sections = %+v", art.Sections)
	}
	lead, err := lib.Read(context.Background(), ReadRequest{Path: "Berlin", Section: "0"})
	if err != nil || lead.Content == "" || strings.HasPrefix(lead.Content, "# ") {
		t.Fatalf("lead section = %+v err %v", lead, err)
	}
	if _, err := lib.Read(context.Background(), ReadRequest{Path: "Berlin", Section: "Geschichte"}); !errors.Is(err, ErrSectionNotFound) {
		t.Fatalf("unknown section: err = %v", err)
	}
	if _, err := lib.Read(context.Background(), ReadRequest{Path: "Berlin", Offset: 1 << 20}); !errors.Is(err, ErrOffsetOutOfRange) {
		t.Fatalf("offset past the end: err = %v", err)
	}
}

func TestFixtureReadResolvesRedirectsAndAccents(t *testing.T) {
	useFreshRenderCache(t)
	lib := openFixtureLibrary(t, "fixture_de.zim", "de")
	for _, tc := range []struct {
		req                   ReadRequest
		path, title, redirect string
	}{
		{ReadRequest{Path: "Hauptstadt_Deutschlands"}, "Berlin", "Berlin", "Hauptstadt Deutschlands"},
		{ReadRequest{Title: "Spree-Athen"}, "Berlin", "Berlin", "Spree-Athen"},
		{ReadRequest{Title: "Muenchen"}, "München", "München", "Muenchen"},
		{ReadRequest{Title: "Strasse"}, "Straße", "Straße", "Strasse"},
		{ReadRequest{Title: "münchen"}, "München", "München", ""},
		{ReadRequest{Title: "Spree (Fluss)"}, "Spree", "Spree (Fluss)", ""},
		{ReadRequest{Title: "spree"}, "Spree", "Spree (Fluss)", ""},
	} {
		art, err := lib.Read(context.Background(), tc.req)
		if err != nil {
			t.Fatalf("read %+v: %v", tc.req, err)
		}
		if art.Path != tc.path || art.Title != tc.title || art.RedirectedFrom != tc.redirect {
			t.Fatalf("read %+v = %+v (redirected from %q), want %q %q from %q", tc.req, art.Ref, art.RedirectedFrom, tc.path, tc.title, tc.redirect)
		}
	}
	if _, err := lib.Read(context.Background(), ReadRequest{Title: "Paris"}); !errors.Is(err, ErrArticleNotFound) {
		t.Fatalf("missing article: err = %v", err)
	}
}

func TestFixtureLeadFollowsRedirectsAndIsBounded(t *testing.T) {
	lib := openFixtureLibrary(t, "fixture_de.zim", "de")
	lead, err := lib.Lead(context.Background(), "Berlin")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(lead, "Berlin ist die Hauptstadt Deutschlands") || len([]rune(lead)) > leadMaxRunes {
		t.Fatalf("lead = %q", lead)
	}
	viaRedirect, err := lib.Lead(context.Background(), "Hauptstadt_Deutschlands")
	if err != nil || viaRedirect != lead {
		t.Fatalf("lead via redirect = %q err %v, want %q", viaRedirect, err, lead)
	}
	if _, err := lib.Lead(context.Background(), "Paris"); !errors.Is(err, ErrArticleNotFound) {
		t.Fatalf("missing article: err = %v", err)
	}
}

// A library whose archive was closed by an edition swap reports zim.ErrClosed
// from every entry point: callers map that to "being updated", not "not found".
func TestFixtureClosedLibraryReportsErrClosed(t *testing.T) {
	useFreshRenderCache(t)
	lib := openFixtureLibrary(t, "fixture_de.zim", "de")
	ctx := context.Background()
	if _, err := lib.Read(ctx, ReadRequest{Path: "Berlin"}); err != nil {
		t.Fatal(err)
	}
	if err := lib.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := lib.Read(ctx, ReadRequest{Path: "Berlin"}); !errors.Is(err, zim.ErrClosed) {
		t.Fatalf("read: err = %v, want zim.ErrClosed", err)
	}
	if _, err := lib.Read(ctx, ReadRequest{Title: "Hauptstadt Deutschlands"}); !errors.Is(err, zim.ErrClosed) {
		t.Fatalf("read by title: err = %v, want zim.ErrClosed", err)
	}
	if _, err := lib.Lead(ctx, "Berlin"); !errors.Is(err, zim.ErrClosed) {
		t.Fatalf("lead: err = %v, want zim.ErrClosed", err)
	}
	if _, err := lib.Search(ctx, "Berlin", 5, 1); !errors.Is(err, zim.ErrClosed) {
		t.Fatalf("search: err = %v, want zim.ErrClosed", err)
	}
	if _, err := lib.Suggest(ctx, "Ber", 5); !errors.Is(err, zim.ErrClosed) {
		t.Fatalf("suggest: err = %v, want zim.ErrClosed", err)
	}
}
