package xapian

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
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

// prefixIndex holds 250 terms st000..st249 whose term frequency grows with
// the number in steps of eight (so the most frequent ones sort last and the
// 100th place is a tie), each document indexing one of them, plus "stz" in
// 6,000 documents (several posting chunks).
func prefixIndex(t *testing.T, separator func(int, string) string) (*Database, map[string]uint32) {
	t.Helper()
	var spec []expansion
	for k := 0; k < 250; k++ {
		spec = append(spec, expansion{fmt.Sprintf("st%03d", k), uint32(1 + (k+1)/8)})
	}
	return termIndex(t, append(spec, expansion{"stz", 6000}), separator)
}

// termIndex indexes each term of spec in tf documents of its own (title =
// the capitalised term, no collapse key).
func termIndex(t *testing.T, spec []expansion, separator func(int, string) string) (*Database, map[string]uint32) {
	t.Helper()
	terms := map[string][][2]uint32{}
	tfs := map[string]uint32{}
	var lens []uint32
	var titles, data []string
	add := func(term string) {
		did := uint32(len(lens) + 1)
		terms[term] = append(terms[term], [2]uint32{did, 1})
		terms[anchorTerm] = append(terms[anchorTerm], [2]uint32{did, 1})
		lens = append(lens, 2)
		titles = append(titles, strings.ToUpper(term[:1])+term[1:])
		data = append(data, fmt.Sprintf("C/%s_%d", term, did))
		tfs[term]++
	}
	for _, e := range spec {
		for i := uint32(0); i < e.tf; i++ {
			add(e.term)
		}
	}
	db := synthIndex{blockSize: minBlockSize, docLens: lens, terms: terms, data: data,
		values: map[uint32][]string{0: titles}, separator: separator}.build(t)
	return db, tfs
}

// mostFrequent is the expansion oracle: the 100 best of terms (read in byte
// order) by term frequency, ties to the term that sorts first.
func mostFrequent(terms []string, tfs map[string]uint32) map[string]bool {
	var all []expansion
	for _, term := range terms {
		all = append(all, expansion{term, tfs[term]})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].better(all[j]) })
	want := map[string]bool{}
	for _, e := range all[:min(len(all), maxPartialExpansion)] {
		want[e.term] = true
	}
	return want
}

func checkMembers(t *testing.T, g *partialGroup, want map[string]bool, tfs map[string]uint32) {
	t.Helper()
	if len(g.members) != len(want) {
		t.Fatalf("%d members, want %d", len(g.members), len(want))
	}
	for _, l := range g.members {
		if !want[l.term] {
			t.Errorf("expansion %q (tf %d) is not one of the most frequent", l.term, tfs[l.term])
		}
	}
}

// The partial word expands to the 100 most frequent of all terms with the
// prefix, like Xapian's WILDCARD_LIMIT_MOST_FREQUENT, not to the most
// frequent of the first terms in byte order.
func TestSuggestPartialUsesMostFrequentOfAllTerms(t *testing.T) {
	db, tfs := prefixIndex(t, nil)
	if pl, err := db.openPostings("stz"); err != nil || pl.isLastChunk {
		t.Fatalf("stz must span several chunks (%v)", err)
	}
	terms, err := db.TermsWithPrefix("st", 0)
	if err != nil || len(terms) != 251 || terms[249] != "st249" || terms[250] != "stz" {
		t.Fatalf("TermsWithPrefix(st) = %d terms, %v", len(terms), err)
	}
	want := mostFrequent(terms, tfs)
	s := &suggester{p: poller{ctx: context.Background()}, db: db, a: NewAnalyzer("eng"), w: newBM25Params(db, 0.001, 1)}
	g, err := s.partial("st", "")
	if err != nil {
		t.Fatal(err)
	}
	checkMembers(t, g, want, tfs)
	if !want["stz"] || !want["st249"] || want["st000"] {
		t.Fatalf("oracle: stz %v, st249 %v, st000 %v", want["stz"], want["st249"], want["st000"])
	}
	// Suggestions come from the most frequent terms, including the ones that
	// sort last; the walk lists "stz" once although it has several chunks.
	hits, err := Suggest(context.Background(), db, NewAnalyzer("eng"), "st", 10000)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, h := range hits {
		seen[strings.ToLower(h.Title)] = true
	}
	if !seen["st249"] || !seen["stz"] || seen["st000"] {
		t.Fatalf("Suggest(st) titles: st249 %v, stz %v, st000 %v", seen["st249"], seen["stz"], seen["st000"])
	}

	// More than 10,000 terms with the prefix, the most frequent sorting last
	// (the old scan stopped after 10,000 terms and missed them).
	var spec []expansion
	for k := 0; k < 10050; k++ {
		tf := uint32(1)
		if k >= 10000 {
			tf = 3
		}
		spec = append(spec, expansion{fmt.Sprintf("sx%05d", k), tf})
	}
	big, bigTfs := termIndex(t, spec, nil)
	terms, err = big.TermsWithPrefix("sx", 0)
	if err != nil || len(terms) != len(spec) {
		t.Fatalf("TermsWithPrefix(sx) = %d terms, %v", len(terms), err)
	}
	want = mostFrequent(terms, bigTfs)
	if !want["sx10049"] || !want["sx00049"] || want["sx00050"] {
		t.Fatalf("oracle: %d terms", len(want))
	}
	s = &suggester{p: poller{ctx: context.Background()}, db: big, a: NewAnalyzer("eng"), w: newBM25Params(big, 0.001, 1)}
	if g, err = s.partial("sx", ""); err != nil {
		t.Fatal(err)
	}
	checkMembers(t, g, want, bigTfs)
}

// maxPartialScan bounds the terms one expansion reads; past it the
// expansion keeps the most frequent of the terms read.
func TestSuggestPartialScanCap(t *testing.T) {
	db, tfs := prefixIndex(t, nil)
	defer func(n int) { maxPartialScan = n }(maxPartialScan)
	maxPartialScan = 120
	s := &suggester{p: poller{ctx: context.Background()}, db: db, a: NewAnalyzer("eng"), w: newBM25Params(db, 0.001, 1)}
	g, err := s.partial("st", "")
	if err != nil {
		t.Fatal(err)
	}
	read, err := db.TermsWithPrefix("st", 120) // st000..st119
	if err != nil {
		t.Fatal(err)
	}
	want := mostFrequent(read, tfs)
	// st015..st022 tie at the cut-off; the first three of them make it.
	if !want["st119"] || !want["st023"] || !want["st015"] || !want["st017"] || want["st018"] || want["st022"] {
		t.Fatalf("oracle: cut-off tie not where expected: %v", want)
	}
	checkMembers(t, g, want, tfs)
	// The scan polls ctx (the poller is one call short of a check).
	done, cancel := context.WithCancel(context.Background())
	cancel()
	cancelled := &suggester{p: poller{ctx: done, n: ctxCheckEvery - 1}, db: db, a: NewAnalyzer("eng"), w: s.w}
	if _, err := cancelled.partial("st", ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("prefix scan ignored a cancellation: %v", err)
	}
}

// Skipping a term's later chunks is a seek, which must move forward: crafted
// separators that send it back must fail instead of looping.
func TestWalkTermsRejectsBackwardSkip(t *testing.T) {
	firstLater := string(appendSortPreservingString(nil, "stz", false))
	db, _ := prefixIndex(t, func(_ int, key string) string {
		if key > firstLater {
			return "\xff" // above every key: seeks past stz's chunks land too far left
		}
		return key
	})
	if _, err := db.TermsWithPrefix("st", 0); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("TermsWithPrefix over a backward skip: %v; want ErrCorrupt", err)
	}
}

// The bounded top-k must give exactly what sorting every candidate and
// collapsing on value slot 1 gives, for any limit, with many score and title
// ties and keys shared by documents on either side of the cut-off.
func TestSuggestTopMatchesFullSort(t *testing.T) {
	const n = 4000
	rng := uint64(3)
	next := func(m uint64) uint64 {
		rng = rng*6364136223846793005 + 1442695040888963407
		return (rng >> 33) % m
	}
	lens := make([]uint32, n)
	titles := make([]string, n)
	keys := make([]string, n)
	data := make([]string, n)
	for i := range lens {
		lens[i] = 1
		titles[i] = fmt.Sprintf("T%02d", next(40)) // many equal titles
		if next(3) > 0 {
			keys[i] = fmt.Sprintf("k%03d", next(500)) // keys shared by ~5 documents
		}
		data[i] = fmt.Sprintf("C/p%d", i+1)
	}
	db := synthIndex{blockSize: minBlockSize, docLens: lens, data: data,
		values: map[uint32][]string{0: titles, 1: keys}}.build(t)
	var cands []scored
	for did := uint32(1); did <= n; did++ {
		if next(4) > 0 {
			cands = append(cands, scored{did, float64(next(25))}) // many equal scores
		}
	}
	// Reference: the pre-top-k ranking (sort all, keep the first per key).
	ref := make([]suggestion, len(cands))
	for i, c := range cands {
		ref[i] = suggestion{Hit: Hit{DocID: c.did, Score: c.score, Title: titles[c.did-1]}, key: keys[c.did-1]}
	}
	sort.Slice(ref, func(i, j int) bool { return ref[i].better(ref[j]) })
	var collapsed []Hit
	seen := map[string]bool{}
	for _, r := range ref {
		if r.key != "" {
			if seen[r.key] {
				continue
			}
			seen[r.key] = true
		}
		r.Path = data[r.DocID-1][2:]
		collapsed = append(collapsed, r.Hit)
	}
	for _, limit := range []int{1, 2, 7, 50, 333, len(collapsed) - 1, len(collapsed), n} {
		top := newSuggestTop(db, limit)
		for _, c := range cands {
			if err := top.offer(c); err != nil {
				t.Fatal(err)
			}
		}
		if len(top.h.byKey) > limit {
			t.Fatalf("limit %d: %d collapse keys kept", limit, len(top.h.byKey))
		}
		p := poller{ctx: context.Background()}
		got, err := top.hits(&p)
		if err != nil {
			t.Fatal(err)
		}
		want := collapsed[:min(limit, len(collapsed))]
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("limit %d: top-k differs from the full sort\n got %v\nwant %v", limit, got[:min(5, len(got))], want[:min(5, len(want))])
		}
	}
}

// A query without word characters expands to the terms starting with it
// (libzim's OP_WILDCARD); the stream polls ctx like every other loop.
func TestSuggestWildcardOnly(t *testing.T) {
	db, _ := termIndex(t, []expansion{{"!a", 300}, {"!b", 400}, {"a", 50}}, nil)
	en := NewAnalyzer("eng")
	hits, err := Suggest(context.Background(), db, en, "!", 1000)
	if err != nil || len(hits) != 700 {
		t.Fatalf("Suggest(!) = %d hits, %v; want 700", len(hits), err)
	}
	for _, h := range hits {
		if !strings.HasPrefix(h.Title, "!") {
			t.Fatalf("Suggest(!) returned %q", h.Title)
		}
	}
	if _, err := Suggest(&lateCancel{Context: context.Background()}, db, en, "!", 10); !errors.Is(err, context.Canceled) {
		t.Fatalf("wildcard stream over 700 documents ignored a cancellation: %v", err)
	}
}

// MostFrequentTerms lists the same expansions Suggest weighs, most frequent
// first, ties in byte order.
func TestMostFrequentTerms(t *testing.T) {
	for _, name := range fixtureNames {
		db := openFixture(t, name, "title")
		for _, prefix := range []string{"b", "ha", "s", "z"} {
			all, err := db.TermsWithPrefix(prefix, 0)
			if err != nil {
				t.Fatal(err)
			}
			type termFreq struct {
				term string
				tf   uint32
			}
			var want []termFreq
			for _, term := range all {
				tf, err := db.TermFreq(term)
				if err != nil {
					t.Fatal(err)
				}
				want = append(want, termFreq{term, tf})
			}
			sort.Slice(want, func(i, j int) bool {
				if want[i].tf != want[j].tf {
					return want[i].tf > want[j].tf
				}
				return want[i].term < want[j].term
			})
			for _, n := range []int{1, 3, 200} {
				got, err := MostFrequentTerms(context.Background(), db, prefix, n)
				if err != nil {
					t.Fatal(err)
				}
				if len(got) != min(n, len(want), maxPartialExpansion) {
					t.Fatalf("%s %q n=%d: %d terms, want %d", name, prefix, n, len(got), min(n, len(want)))
				}
				for i, term := range got {
					if term != want[i].term {
						t.Fatalf("%s %q n=%d: term %d = %q, want %q (%v)", name, prefix, n, i, term, want[i].term, got)
					}
				}
			}
		}
	}
	db := openFixture(t, "de", "title")
	if got, err := MostFrequentTerms(context.Background(), db, "", 5); err != nil || got != nil {
		t.Fatalf("empty prefix = %v, %v", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := MostFrequentTerms(ctx, db, "b", 5); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled ctx: %v", err)
	}
}
