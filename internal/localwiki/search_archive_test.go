package localwiki

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"aurago/internal/zim"
	"aurago/internal/zim/zimtest"
)

// syntheticArchive builds a small new-namespace ZIM without Xapian indexes:
// the degraded title-only mode with the titleOrdered prefix fallback.
func syntheticArchive(t *testing.T) *zim.Archive {
	t.Helper()
	b := zimtest.New()
	c := b.AddCluster(zimtest.CompressionZstd, false)
	b.AddMetadata(c, "Language", "deu")
	b.AddArticle(c, 'C', "Berlin", "Berlin", htmlPage("Berlin",
		`<section data-mw-section-id="0"><p><b>Berlin</b> ist die Hauptstadt Deutschlands.</p></section>`+
			`<section data-mw-section-id="1"><div class="mw-heading mw-heading2"><h2 id="Geschichte">Geschichte</h2></div><p>Erste Erwähnung 1237.</p></section>`))
	b.AddArticle(c, 'C', "Bern", "Bern", htmlPage("Bern", `<p>Bern ist die Bundesstadt der Schweiz.</p>`))
	b.AddArticle(c, 'C', "Hamburg", "Hamburg", htmlPage("Hamburg", `<p>Hamburg hat einen Hafen.</p>`))
	b.Add(zimtest.Entry{Namespace: 'C', Path: "Hauptstadt_Deutschlands", Title: "Hauptstadt Deutschlands", Redirect: "C/Berlin", Front: true})
	path, _ := b.WriteFile(t)
	a, err := zim.Open(path, zim.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

func TestSearchIndexOverSyntheticArchive(t *testing.T) {
	useFreshRenderCache(t)
	ix := searchIndex{store: zimStore{a: syntheticArchive(t)}}
	ctx := context.Background()

	hits, err := ix.search(ctx, "Hauptstadt Deutschlands", 5, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Path != "Berlin" || hits[0].Snippet != "Berlin ist die Hauptstadt Deutschlands." || hits[0].Lead != "**Berlin** ist die Hauptstadt Deutschlands." {
		t.Fatalf("hits = %+v", hits)
	}

	refs, err := ix.suggest(ctx, "Be", 10)
	if err != nil {
		t.Fatal(err)
	}
	if want := []Ref{{Title: "Berlin", Path: "Berlin"}, {Title: "Bern", Path: "Bern"}}; !reflect.DeepEqual(refs, want) {
		t.Fatalf("suggest = %+v", refs)
	}

	art, err := ix.read(ctx, ReadRequest{Title: "Hauptstadt Deutschlands", Section: "Geschichte"})
	if err != nil {
		t.Fatal(err)
	}
	if art.Path != "Berlin" || art.RedirectedFrom != "Hauptstadt Deutschlands" || art.Content != "## Geschichte\n\nErste Erwähnung 1237." {
		t.Fatalf("article = %+v", art)
	}

	if _, err := ix.read(ctx, ReadRequest{Path: "Paris"}); !errors.Is(err, ErrArticleNotFound) {
		t.Fatalf("missing article: %v", err)
	}
}

// A closed archive (the edition is being swapped) is reported as zim.ErrClosed
// by every entry point, never as a missing article.
func TestSearchIndexReportsClosedSyntheticArchive(t *testing.T) {
	useFreshRenderCache(t)
	a := syntheticArchive(t)
	ix := searchIndex{store: zimStore{a: a}}
	ctx := context.Background()
	if _, err := ix.read(ctx, ReadRequest{Path: "Berlin"}); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := ix.read(ctx, ReadRequest{Path: "Berlin"}); !errors.Is(err, zim.ErrClosed) {
		t.Fatalf("read by path: err = %v, want zim.ErrClosed", err)
	}
	if _, err := ix.read(ctx, ReadRequest{Title: "Hauptstadt Deutschlands"}); !errors.Is(err, zim.ErrClosed) {
		t.Fatalf("read by title: err = %v, want zim.ErrClosed", err)
	}
	if _, err := ix.lead(ctx, "Berlin"); !errors.Is(err, zim.ErrClosed) {
		t.Fatalf("lead: err = %v, want zim.ErrClosed", err)
	}
}
