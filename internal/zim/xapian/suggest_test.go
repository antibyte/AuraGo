package xapian

import (
	"context"
	"testing"
)

// libzim's SuggestionSearcher order must be reproduced exactly; scores must
// match the python3-xapian replica of libzim's suggestion query.
func TestSuggestGoldenLibzimOrder(t *testing.T) {
	for _, name := range fixtureNames {
		var g goldenQueries
		loadJSON(t, "queries_"+name+".json", &g)
		db := openFixture(t, name, "title")
		a := NewAnalyzer(g.Language)
		for _, q := range g.Suggest {
			hits, err := Suggest(context.Background(), db, a, q.Query, 50)
			if err != nil {
				t.Fatalf("%s %q: %v", name, q.Query, err)
			}
			if len(hits) != len(q.Libzim.Paths) {
				t.Errorf("%s %q: %d hits %v, libzim %v", name, q.Query, len(hits), hitPaths(hits), q.Libzim.Paths)
				continue
			}
			for i, h := range hits {
				if h.Path != q.Libzim.Paths[i] {
					t.Errorf("%s %q: hit %d = %q, libzim %q (all %v)", name, q.Query, i, shortPath(h.Path), shortPath(q.Libzim.Paths[i]), hitPaths(hits))
					break
				}
			}
			for i, want := range weighted4(t, q.Replica) {
				if i >= len(hits) {
					break
				}
				if hits[i].DocID != want.DocID || !closeEnough(hits[i].Score, want.Weight) {
					t.Errorf("%s %q hit %d = (%d, %.12g), xapian (%d, %.12g) [%s]", name, q.Query, i, hits[i].DocID, hits[i].Score, want.DocID, want.Weight, q.XapianQuery)
					break
				}
			}
		}
	}
}

func TestSuggestEdgeCases(t *testing.T) {
	db := openFixture(t, "de", "title")
	a := NewAnalyzer("deu")
	for _, q := range []string{"", "   ", "!!!"} {
		hits, err := Suggest(context.Background(), db, a, q, 10)
		if err != nil || len(hits) != 0 {
			t.Errorf("Suggest(%q) = %v, %v", q, hitPaths(hits), err)
		}
	}
	if hits, err := Suggest(context.Background(), db, a, "berlin", 0); err != nil || hits != nil {
		t.Errorf("limit 0 = %v, %v", hits, err)
	}
	hits, err := Suggest(context.Background(), db, a, "b", 2)
	if err != nil || len(hits) != 2 || hits[0].Title == "" {
		t.Errorf("limit 2 = %+v, %v", hits, err)
	}
}

func hitPaths(h []Hit) []string {
	out := make([]string, len(h))
	for i := range h {
		out[i] = shortPath(h[i].Path)
	}
	return out
}
