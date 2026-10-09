package xapian

import (
	"context"
	"errors"
	"math"
	"sort"
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
}

// lateCancel passes the entry check (its first Err call) and reports
// cancellation from then on, so only the checks inside the loops can see it.
type lateCancel struct {
	context.Context
	calls int
}

func (c *lateCancel) Err() error {
	if c.calls++; c.calls > 1 {
		return context.Canceled
	}
	return nil
}

func TestSearchHonoursCancellation(t *testing.T) {
	db := openFixture(t, "bulk", "fulltext")
	done, cancel := context.WithCancel(context.Background())
	cancel()
	for _, c := range []struct {
		terms []string
		op    Op
	}{
		{[]string{"0550"}, OpAnd}, // one match: no loop runs long enough to poll
		{[]string{"zebra"}, OpAnd},
		{[]string{"0550", "g3"}, OpOr},
		{[]string{"nosuchterm"}, OpAnd},
	} {
		if _, _, err := Search(done, db, c.terms, c.op, 0, 10); !errors.Is(err, context.Canceled) {
			t.Errorf("Search(%q, %v) with a cancelled ctx: %v", c.terms, c.op, err)
		}
	}
	ti := openFixture(t, "bulk", "title")
	en := NewAnalyzer("eng")
	if _, err := Suggest(done, ti, en, "bulk item 0550", 10); !errors.Is(err, context.Canceled) {
		t.Errorf("Suggest with a cancelled ctx: %v", err)
	}
	if _, err := Suggest(&lateCancel{Context: context.Background()}, ti, en, "bulk", 10); !errors.Is(err, context.Canceled) {
		t.Errorf("Suggest over 1,100 titles ignored a cancellation: %v", err)
	}

	// Two lists that never agree: an AND without a single match must still
	// notice the cancellation while it skips.
	lens := make([]uint32, 4000)
	var odd, even [][2]uint32
	for i := range lens {
		did := uint32(i + 1)
		lens[i] = 10
		if did%2 == 1 {
			odd = append(odd, [2]uint32{did, 1})
		} else {
			even = append(even, [2]uint32{did, 1})
		}
	}
	disjoint := synthDB(t, 8192, lens, map[string][][2]uint32{"odd": odd, "even": even})
	terms := []string{"odd", "even"}
	if hits, total, err := Search(context.Background(), disjoint, terms, OpAnd, 0, 10); err != nil || hits != nil || total != 0 {
		t.Fatalf("odd AND even = %v, %d, %v", hits, total, err)
	}
	for _, op := range []Op{OpAnd, OpOr} {
		if _, _, err := Search(&lateCancel{Context: context.Background()}, disjoint, terms, op, 0, 10); !errors.Is(err, context.Canceled) {
			t.Errorf("op %v over 4,000 documents ignored a cancellation: %v", op, err)
		}
	}
}

func TestSearchWindow(t *testing.T) {
	db := openFixture(t, "bulk", "fulltext")
	ctx := context.Background()
	for _, w := range [][2]int{{-1, 10}, {0, -1}, {math.MinInt, math.MaxInt}} {
		if _, _, err := Search(ctx, db, []string{"zebra"}, OpAnd, w[0], w[1]); err == nil {
			t.Errorf("offset %d, limit %d accepted", w[0], w[1])
		}
	}
	// offset+limit must not overflow, on 32-bit ints either.
	hits, total, err := Search(ctx, db, []string{"zebra"}, OpAnd, 5, math.MaxInt)
	if err != nil || total != 1101 || len(hits) != 1096 {
		t.Fatalf("offset 5, limit MaxInt: %d hits of %d, %v", len(hits), total, err)
	}

	// More matches than the window: hits stop at MaxSearchWindow, the total
	// does not.
	n := MaxSearchWindow + 500
	lens := make([]uint32, n)
	var all [][2]uint32
	for i := range lens {
		lens[i] = uint32(10 + i%50)
		all = append(all, [2]uint32{uint32(i + 1), 1})
	}
	big := synthDB(t, 8192, lens, map[string][][2]uint32{"all": all})
	ref, total, err := Search(ctx, big, []string{"all"}, OpAnd, 0, n)
	if err != nil || total != n || len(ref) != MaxSearchWindow {
		t.Fatalf("limit %d: %d hits of %d, %v", n, len(ref), total, err)
	}
	tail, total, err := Search(ctx, big, []string{"all"}, OpAnd, MaxSearchWindow-3, 10)
	if err != nil || total != n || len(tail) != 3 || tail[0] != ref[MaxSearchWindow-3] || tail[2] != ref[MaxSearchWindow-1] {
		t.Fatalf("last page: %v of %d, %v", tail, total, err)
	}
	beyond, total, err := Search(ctx, big, []string{"all"}, OpAnd, MaxSearchWindow, 10)
	if err != nil || total != n || beyond != nil {
		t.Fatalf("offset at the cap: %v of %d, %v", beyond, total, err)
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
		{[]uint32{0xffffffff, 0xffffffff}, 0xffffffff, 0xffffffff},
		// Damaged term frequencies: the estimate stays inside [0, N].
		{[]uint32{30, 30}, 10, 0},                // 60-90 = -30
		{[]uint32{20, 1}, 10, 10},                // 21-2 = 19
		{[]uint32{0xffffffff, 0xffffffff}, 0, 0}, // no documents: 2^33
	}
	for _, c := range cases {
		if got := orTermFreqEstimate(c.tfs, c.n); got != c.want {
			t.Errorf("orTermFreqEstimate(%v, %d) = %d, want %d", c.tfs, c.n, got, c.want)
		}
	}
}

// Scoring streams the document length list with skipTo, which only works if
// both merges hand out docids in ascending order. Check that, and that the
// streamed lengths give exactly the scores of per-document DocLength lookups.
func TestSearchStreamsDocLengthsInDocidOrder(t *testing.T) {
	const n = 30000
	lens := make([]uint32, n)
	lists := map[string][][2]uint32{}
	for i := range lens {
		did := uint32(i + 1)
		lens[i] = 20 + did*7919%400
		for _, m := range []struct {
			term string
			mod  uint32
		}{{"three", 3}, {"five", 5}, {"seven", 7}} {
			if did%m.mod == 0 || did%1009 == 1 {
				lists[m.term] = append(lists[m.term], [2]uint32{did, 1 + did%m.mod})
			}
		}
	}
	lens[41] = 0 // a deleted document inside a chunk
	for term, ps := range lists {
		var kept [][2]uint32
		for _, p := range ps {
			if p[0] != 42 {
				kept = append(kept, p)
			}
		}
		lists[term] = kept
	}
	db := synthDB(t, minBlockSize, lens, lists)
	query := []string{"seven", "three", "five"}
	w := newBM25(db)
	in := func(term string, did uint32) (uint32, bool) {
		ps := lists[term]
		i := sort.Search(len(ps), func(i int) bool { return ps[i][0] >= did })
		if i < len(ps) && ps[i][0] == did {
			return ps[i][1], true
		}
		return 0, false
	}
	for _, op := range []Op{OpAnd, OpOr} {
		// The order Search adds term weights in: rarest first for AND, query
		// order for OR.
		order := query
		if op == OpAnd {
			order = []string{"seven", "five", "three"}
		}
		var want []Hit
		for did := uint32(1); did <= n; did++ {
			s, matched := 0.0, 0
			for _, term := range order {
				wdf, ok := in(term, did)
				if !ok {
					continue
				}
				dl, err := db.DocLength(did)
				if err != nil {
					t.Fatalf("DocLength(%d): %v", did, err)
				}
				s += w.sumPart(w.termWeight(uint32(len(lists[term]))), wdf, dl)
				matched++
			}
			if matched == len(order) || (op == OpOr && matched > 0) {
				want = append(want, Hit{DocID: did, Score: s})
			}
		}
		sortHits(want)

		var srcs []*searchTerm
		for _, term := range order {
			pl, err := db.openPostings(term)
			if err != nil {
				t.Fatal(err)
			}
			srcs = append(srcs, &searchTerm{pl: pl})
		}
		var visited []uint32
		visit := func(did uint32, _ []*searchTerm) error {
			if len(visited) > 0 && did <= visited[len(visited)-1] {
				t.Fatalf("op %v visits docid %d after %d", op, did, visited[len(visited)-1])
			}
			visited = append(visited, did)
			return nil
		}
		var err error
		if op == OpAnd {
			err = andMatches(context.Background(), srcs, visit)
		} else {
			err = orMatches(context.Background(), srcs, visit)
		}
		if err != nil || len(visited) != len(want) {
			t.Fatalf("op %v visited %d documents (%v), want %d", op, len(visited), err, len(want))
		}

		limit := min(len(want), MaxSearchWindow)
		hits, total, err := Search(context.Background(), db, query, op, 0, limit)
		if err != nil || total != len(want) || len(hits) != limit {
			t.Fatalf("op %v: %d hits of %d (%v), want %d of %d", op, len(hits), total, err, limit, len(want))
		}
		for i := range hits {
			if hits[i].DocID != want[i].DocID || hits[i].Score != want[i].Score {
				t.Fatalf("op %v hit %d = (%d, %v), per-document lookup gives (%d, %v)", op, i, hits[i].DocID, hits[i].Score, want[i].DocID, want[i].Score)
			}
		}
	}
}
