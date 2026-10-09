package xapian

import (
	"context"
	"reflect"
	"sync"
	"testing"

	"aurago/internal/zim"
	"aurago/internal/zim/zimtest"
)

// realSuggestComparable is how many leading libzim suggestions are compared.
// "climate c" expands the prefix "c" to more than 100 terms; 53 of them share
// the cut-off term frequency and libzim picks among them with std::nth_element,
// so positions after 8 depend on C++ library internals.
var realSuggestComparable = map[string]int{"climate c": 8}

// TestRealFixtureIndexes checks a real mwoffliner-built Wikipedia ZIM
// (slice 1's pinned zim-testing-suite file). Runs only with
// AURAGO_ZIM_REAL_FIXTURE=1.
func TestRealFixtureIndexes(t *testing.T) {
	a, err := zim.Open(zimtest.RealFixture(t), zim.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	var golden struct {
		Fulltext map[string]struct {
			Paths []string `json:"paths"`
		} `json:"fulltext"`
		Suggest map[string]struct {
			Paths []string `json:"paths"`
		} `json:"suggest"`
	}
	loadJSON(t, "real_fixture_libzim.json", &golden)
	dbs := map[string]*Database{}
	for _, kind := range []string{"fulltext", "title"} {
		e, err := a.EntryByPath('X', kind+"/xapian")
		if err != nil {
			t.Fatal(err)
		}
		sr, err := a.Open(e)
		if err != nil {
			t.Fatal(err)
		}
		db, err := Open(sr, sr.Size())
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		dbs[kind] = db
		if k, _ := db.Metadata("kind"); k != kind {
			t.Errorf("%s: metadata kind %q", kind, k)
		}
		if l, _ := db.Metadata("language"); l != "eng" {
			t.Errorf("%s: metadata language %q", kind, l)
		}
		// Document lengths are complete and add up to the stored total.
		var n uint32
		var sum uint64
		lens, err := db.openPostings("")
		if err != nil {
			t.Fatal(err)
		}
		for lens.Next() {
			n++
			sum += uint64(lens.WDF())
		}
		if lens.Err() != nil || n != db.DocCount() || sum != db.TotalLength() {
			t.Fatalf("%s: %d lengths summing to %d (%v); want %d / %d", kind, n, sum, lens.Err(), db.DocCount(), db.TotalLength())
		}
		// Every posting list agrees with its term and collection frequency.
		terms := 0
		err = db.walkTerms("", func(term string, _ *cursor) (bool, error) {
			terms++
			tf, cf, err := db.termStats(term)
			if err != nil {
				return false, err
			}
			it, err := db.Postings(term)
			if err != nil {
				return false, err
			}
			var cnt uint32
			var wdf uint64
			for it.Next() {
				cnt++
				wdf += uint64(it.WDF())
			}
			if it.Err() != nil || cnt != tf || wdf != cf {
				t.Errorf("%s %q: %d postings / wdf %d (%v), want %d / %d", kind, term, cnt, wdf, it.Err(), tf, cf)
			}
			return true, nil
		})
		if err != nil || terms == 0 {
			t.Fatalf("%s: walked %d terms: %v", kind, terms, err)
		}
	}
	an := NewAnalyzer("eng")
	for q, want := range golden.Fulltext {
		hits, _, err := Search(context.Background(), dbs["fulltext"], an.QueryTerms(q), OpAnd, 0, len(want.Paths))
		if err != nil || !reflect.DeepEqual(pathsOf(hits), want.Paths) {
			t.Errorf("Search(%q) = %v, %v\nlibzim %v", q, pathsOf(hits), err, want.Paths)
		}
	}
	for q, want := range golden.Suggest {
		hits, err := Suggest(context.Background(), dbs["title"], an, q, len(want.Paths))
		got, exp := pathsOf(hits), want.Paths
		if n, ok := realSuggestComparable[q]; ok && len(got) >= n && len(exp) >= n {
			got, exp = got[:n], exp[:n]
		}
		if err != nil || !reflect.DeepEqual(got, exp) {
			t.Errorf("Suggest(%q) = %v, %v\nlibzim %v", q, got, err, exp)
		}
	}
}

// Database is shared by concurrent searches (max 4 in localwiki); a tiny
// block cache forces evictions while goroutines read.
func TestConcurrentReaders(t *testing.T) {
	ft := openFixture(t, "bulk", "fulltext")
	ti := openFixture(t, "bulk", "title")
	for _, db := range []*Database{ft, ti} {
		small := newBlockCache(16)
		db.postlist.cache, db.docdata.cache = small, small
	}
	a := NewAnalyzer("eng")
	ctx := context.Background()
	wantSearch, _, err := Search(ctx, ft, []string{"zebra", "group"}, OpOr, 0, 25)
	if err != nil {
		t.Fatal(err)
	}
	wantSuggest, err := Suggest(ctx, ti, a, "bulk item 05", 10)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan string, 64)
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 15; i++ {
				got, _, err := Search(ctx, ft, []string{"zebra", "group"}, OpOr, 0, 25)
				if err != nil || !reflect.DeepEqual(got, wantSearch) {
					errs <- "search differs under concurrency"
					return
				}
				sg, err := Suggest(ctx, ti, a, "bulk item 05", 10)
				if err != nil || !reflect.DeepEqual(sg, wantSuggest) {
					errs <- "suggest differs under concurrency"
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
}

func pathsOf(hits []Hit) []string {
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.Path
	}
	return out
}
