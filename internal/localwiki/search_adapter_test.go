package localwiki

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"aurago/internal/zim"
	"aurago/internal/zim/zimtest"
)

// These tests run the real zim/xapian adapters behind searchView; the
// archive-backed suite of Task 9 covers every fixture article.

func openTestFixture(t *testing.T, name string) *Library {
	t.Helper()
	lib, err := OpenLibrary(filepath.Join("..", "zim", "testdata", name), Edition{Name: name})
	if err != nil {
		t.Fatalf("OpenLibrary(%s): %v", name, err)
	}
	t.Cleanup(func() { _ = lib.Close() })
	return lib
}

// assertEntryTitles checks that every hit shows the title of its resolved ZIM
// entry (the full-text index stores folded titles) and appears once.
func assertEntryTitles(t *testing.T, lib *Library, refs []Ref) {
	t.Helper()
	seen := map[string]bool{}
	for _, r := range refs {
		if seen[r.Path] {
			t.Fatalf("path %q listed twice in %+v", r.Path, refs)
		}
		seen[r.Path] = true
		e, err := lib.archive.EntryByPath(lib.archive.ContentNamespace(), r.Path)
		if err != nil {
			t.Fatalf("hit %q: %v", r.Path, err)
		}
		if e.IsRedirect {
			t.Fatalf("hit %q is a redirect, want the resolved article", r.Path)
		}
		if r.Title != e.Title {
			t.Fatalf("hit %q title %q, want the entry title %q", r.Path, r.Title, e.Title)
		}
	}
}

func searchRefs(hits []SearchHit) []Ref {
	out := make([]Ref, len(hits))
	for i, h := range hits {
		out[i] = h.Ref
	}
	return out
}

func TestSearchViewFulltextUsesEntryTitlesAndLeads(t *testing.T) {
	lib := openTestFixture(t, "fixture_de.zim")
	if !lib.Fulltext() {
		t.Fatalf("fixture_de has no usable full-text index: %s", lib.fulltextNote)
	}
	res, err := lib.Search(context.Background(), "Hafen", 5, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Fulltext || res.Edition.Name != "fixture_de.zim" || len(res.Results) == 0 {
		t.Fatalf("result = %+v", res)
	}
	top := res.Results[0]
	if top.Ref != (Ref{Title: "Hamburg", Path: "Hamburg"}) {
		t.Fatalf("top hit = %+v, want Hamburg (the only article about a Hafen)", top.Ref)
	}
	if !strings.Contains(top.Snippet, "Hafen") || utf8.RuneCountInString(top.Snippet) > snippetMaxRunes {
		t.Fatalf("snippet = %q", top.Snippet)
	}
	if top.Lead == "" || utf8.RuneCountInString(top.Lead) > leadMaxRunes {
		t.Fatalf("lead = %q", top.Lead)
	}
	assertEntryTitles(t, lib, searchRefs(res.Results))
}

func TestSearchViewRedirectTitlesResolveToTheirTarget(t *testing.T) {
	for _, tc := range []struct{ file, query, want string }{
		{"fixture_de.zim", "Hauptstadt Deutschlands", "Berlin"},
		{"fixture_de.zim", "hauptstadt deutschlands", "Berlin"},
		{"fixture_de.zim", "Muenchen", "München"},
		{"fixture_en.zim", "Cafe", "Café"},
		{"fixture_ja.zim", "Tokyo", "東京"},
		{"fixture_ja.zim", "東京", "東京"},
	} {
		lib := openTestFixture(t, tc.file)
		res, err := lib.Search(context.Background(), tc.query, 5, 0)
		if err != nil {
			t.Fatalf("%s %q: %v", tc.file, tc.query, err)
		}
		if len(res.Results) == 0 || res.Results[0].Path != tc.want {
			t.Fatalf("%s %q: results %+v, want %q first", tc.file, tc.query, searchRefs(res.Results), tc.want)
		}
		assertEntryTitles(t, lib, searchRefs(res.Results))
	}
}

func TestSuggestViewResolvesRedirectHits(t *testing.T) {
	lib := openTestFixture(t, "fixture_de.zim")
	for _, query := range []string{"Spree", "Haupt", "Ham", "B"} {
		refs, err := lib.Suggest(context.Background(), query, 10)
		if err != nil {
			t.Fatalf("Suggest(%q): %v", query, err)
		}
		if len(refs) == 0 || len(refs) > maxSuggestions {
			t.Fatalf("Suggest(%q) = %+v", query, refs)
		}
		assertEntryTitles(t, lib, refs)
	}
	// The title index answers "Spree" with C/Spree and the redirect
	// C/Spree-Athen; the redirect shows up as its target.
	refs, err := lib.Suggest(context.Background(), "Spree", 10)
	if err != nil {
		t.Fatal(err)
	}
	if want := []Ref{{Title: "Spree (Fluss)", Path: "Spree"}, {Title: "Berlin", Path: "Berlin"}}; !reflect.DeepEqual(refs, want) {
		t.Fatalf("Suggest(Spree) = %+v, want %+v", refs, want)
	}
	ja := openTestFixture(t, "fixture_ja.zim")
	refs, err = ja.Suggest(context.Background(), "東", 10)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range refs {
		found = found || r.Path == "東京"
	}
	if !found {
		t.Fatalf("Suggest(東) = %+v, want 東京", refs)
	}
	assertEntryTitles(t, ja, refs)
}

// deprecateTestDirent turns the content directory entry at off into a
// well-formed deprecated ("deleted", MIME 0xFFFD) entry as old archives keep
// them in the v0 title list (same layout as internal/zim's test helper).
func deprecateTestDirent(d []byte, off int64) {
	slot := d[off:]
	ns := slot[3]
	strs := slot[16:]
	pathEnd := bytes.IndexByte(strs, 0)
	titleEnd := pathEnd + 1 + bytes.IndexByte(strs[pathEnd+1:], 0)
	names := append([]byte(nil), strs[:titleEnd+1]...)
	clear(slot[:16+len(names)])
	binary.LittleEndian.PutUint16(slot, 0xFFFD)
	slot[3] = ns
	copy(slot[8:], names)
}

func TestZimStoreSkipsDeprecatedEntriesAndOversizedArticles(t *testing.T) {
	b := zimtest.New()
	b.TitleListV1, b.TitleListV0 = false, true
	z := b.AddCluster(zimtest.CompressionZstd, false)
	b.AddArticle(z, 'C', "Berlin", "Berlin", htmlPage("Berlin", "<p>Berlin ist eine Stadt.</p>"))
	b.AddArticle(z, 'C', "Bern", "Bern", htmlPage("Bern", "<p>Bern ist eine Stadt.</p>"))
	big := b.AddCluster(zimtest.CompressionZstd, false)
	b.AddArticle(big, 'C', "Riese", "Riese", htmlPage("Riese", "<p>Riese "+strings.Repeat("x", maxArticleBytes)+"</p>"))
	data, layout, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	deprecateTestDirent(data, layout.DirentOffset["C/Bern"])
	a, err := zim.Open(zimtest.WriteBytes(t, data), zim.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	store := zimStore{a: a}

	entries, err := store.titlePrefix("Ber", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Path != "Berlin" {
		t.Fatalf("titlePrefix(Ber) = %+v, want Berlin without the deprecated Bern", entries)
	}

	riese, err := store.lookup("Riese")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.readHTML(riese); !errors.Is(err, ErrArticleTooLarge) {
		t.Fatalf("readHTML(oversized) err = %v, want ErrArticleTooLarge", err)
	}
	ix := searchIndex{store: store}
	hits, err := ix.search(context.Background(), "Riese", 5, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Ref != (Ref{Title: "Riese", Path: "Riese"}) || hits[0].Snippet != "" || hits[0].Lead != "" {
		t.Fatalf("hits = %+v, want the oversized article by title without snippet or lead", hits)
	}

	_ = a.Close()
	if _, err := ix.search(context.Background(), "Berlin", 5, 0); !errors.Is(err, zim.ErrClosed) {
		t.Fatalf("search after Close err = %v, want zim.ErrClosed", err)
	}
	if _, err := ix.suggest(context.Background(), "Be", 5); !errors.Is(err, zim.ErrClosed) {
		t.Fatalf("suggest after Close err = %v, want zim.ErrClosed", err)
	}
}
