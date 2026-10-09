package xapian

import (
	"context"
	"testing"
)

// libzim's own Searcher order (python-libzim) must be reproduced exactly;
// scores must match the python3-xapian replica of libzim's query.
func TestSearchGoldenLibzimOrder(t *testing.T) {
	for _, name := range fixtureNames {
		var g goldenQueries
		loadJSON(t, "queries_"+name+".json", &g)
		db := openFixture(t, name, "fulltext")
		a := NewAnalyzer(g.Language)
		for _, q := range g.Fulltext {
			terms := a.QueryTerms(q.Query)
			hits, total, err := Search(context.Background(), db, terms, OpAnd, 0, 50)
			if err != nil {
				t.Fatalf("%s %q: %v", name, q.Query, err)
			}
			if total != q.Libzim.Estimated {
				t.Errorf("%s %q: estimatedTotal %d, libzim %d", name, q.Query, total, q.Libzim.Estimated)
			}
			if len(hits) != len(q.Libzim.Paths) {
				t.Errorf("%s %q: %d hits, libzim %d", name, q.Query, len(hits), len(q.Libzim.Paths))
				continue
			}
			for i, h := range hits {
				if h.Path != q.Libzim.Paths[i] {
					t.Errorf("%s %q: hit %d = %q, libzim %q", name, q.Query, i, shortPath(h.Path), shortPath(q.Libzim.Paths[i]))
					break
				}
			}
			for i, want := range weighted(t, q.And) {
				if i >= len(hits) {
					break
				}
				if hits[i].DocID != want.DocID || !closeEnough(hits[i].Score, want.Weight) {
					t.Errorf("%s %q AND hit %d = (%d, %.12g), xapian (%d, %.12g)", name, q.Query, i, hits[i].DocID, hits[i].Score, want.DocID, want.Weight)
					break
				}
			}
		}
	}
}

func TestSearchGoldenOr(t *testing.T) {
	for _, name := range fixtureNames {
		var g goldenQueries
		loadJSON(t, "queries_"+name+".json", &g)
		db := openFixture(t, name, "fulltext")
		a := NewAnalyzer(g.Language)
		for _, q := range g.Fulltext {
			want := weighted(t, q.Or)
			hits, _, err := Search(context.Background(), db, a.QueryTerms(q.Query), OpOr, 0, 50)
			if err != nil {
				t.Fatalf("%s %q: %v", name, q.Query, err)
			}
			if len(hits) != len(want) {
				t.Errorf("%s %q OR: %d hits, xapian %d", name, q.Query, len(hits), len(want))
				continue
			}
			for i := range want {
				if hits[i].DocID != want[i].DocID || !closeEnough(hits[i].Score, want[i].Weight) || "C/"+hits[i].Path != want[i].Data {
					t.Errorf("%s %q OR hit %d = (%d, %.12g, %q), xapian (%d, %.12g, %q)", name, q.Query, i,
						hits[i].DocID, hits[i].Score, shortPath(hits[i].Path), want[i].DocID, want[i].Weight, shortPath(want[i].Data))
					break
				}
			}
		}
	}
}

func TestSearchOffsetLimitAndTitle(t *testing.T) {
	db := openFixture(t, "bulk", "fulltext")
	all, total, err := Search(context.Background(), db, []string{"zebra"}, OpAnd, 0, 30)
	if err != nil || total != 1101 || len(all) != 30 {
		t.Fatalf("zebra: %d hits of %d, %v", len(all), total, err)
	}
	page, _, err := Search(context.Background(), db, []string{"zebra"}, OpAnd, 10, 10)
	if err != nil || len(page) != 10 {
		t.Fatalf("page: %d, %v", len(page), err)
	}
	for i := range page {
		if page[i].DocID != all[10+i].DocID {
			t.Fatalf("offset page differs at %d", i)
		}
	}
	if all[0].Title == "" || all[0].Path == "" {
		t.Fatalf("hit lacks title/path: %+v", all[0])
	}
	none, total, err := Search(context.Background(), db, []string{"zebra", "nosuchterm"}, OpAnd, 0, 10)
	if err != nil || total != 0 || none != nil {
		t.Fatalf("AND with absent term: %v %d %v", none, total, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := Search(ctx, db, []string{"zebra"}, OpOr, 0, 10); err == nil {
		t.Fatal("cancelled context not reported")
	}
}

func TestExistingTerms(t *testing.T) {
	db := openFixture(t, "de", "fulltext")
	got, err := ExistingTerms(db, []string{"spree", "ath", "berlin", "spree"})
	if err != nil || len(got) != 3 || got[0] != "spree" || got[1] != "berlin" || got[2] != "spree" {
		t.Fatalf("ExistingTerms = %q, %v", got, err)
	}
	hits, _, err := Search(context.Background(), db, got, OpAnd, 0, 10)
	if err != nil || len(hits) == 0 || hits[0].Path != "Spree" {
		t.Fatalf("search after dropping absent terms = %+v, %v", hits, err)
	}
}

func TestOrTermFreqEstimate(t *testing.T) {
	cases := []struct {
		tfs  []uint32
		n    uint32
		want uint32
	}{
		{[]uint32{3}, 5, 3},
		{[]uint32{1, 1}, 20, 2},       // 1+1-1/20 = 1.95 -> 2
		{[]uint32{2, 2}, 4, 3},        // 2+2-1 = 3
		{[]uint32{1, 1, 1, 1}, 4, 3},  // (1,1)->2 (1.75), (1,1)->2, (2,2)->3
		{[]uint32{10, 1, 1}, 100, 12}, // (1,1)->2, (2,10)->11.8->12
	}
	for _, c := range cases {
		if got := orTermFreqEstimate(c.tfs, c.n); got != c.want {
			t.Errorf("orTermFreqEstimate(%v, %d) = %d, want %d", c.tfs, c.n, got, c.want)
		}
	}
}
