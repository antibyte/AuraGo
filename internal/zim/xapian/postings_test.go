package xapian

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"testing"
)

func TestDatabaseGoldenPostings(t *testing.T) {
	for _, name := range fixtureNames {
		for _, kind := range []string{"fulltext", "title"} {
			g := loadGoldenDB(t, name, kind)
			db := openFixture(t, name, kind)
			for _, gt := range g.Terms {
				tf, cf, err := db.termStats(gt.Term)
				if err != nil || tf != gt.TF || cf != gt.CF {
					t.Errorf("%s/%s %q stats = (%d,%d,%v) want (%d,%d)", name, kind, gt.Term, tf, cf, err, gt.TF, gt.CF)
				}
				if tf, err := db.TermFreq(gt.Term); err != nil || tf != gt.TF {
					t.Errorf("%s/%s TermFreq(%q) = %d,%v want %d", name, kind, gt.Term, tf, err, gt.TF)
				}
				it, err := db.Postings(gt.Term)
				if err != nil {
					t.Fatalf("%s/%s Postings(%q): %v", name, kind, gt.Term, err)
				}
				var got [][2]uint32
				for it.Next() {
					got = append(got, [2]uint32{it.DocID(), it.WDF()})
				}
				if it.Err() != nil {
					t.Fatalf("%s/%s %q: %v", name, kind, gt.Term, it.Err())
				}
				if len(got) != len(gt.Postings) {
					t.Errorf("%s/%s %q: %d postings want %d", name, kind, gt.Term, len(got), len(gt.Postings))
					continue
				}
				for i := range got {
					if got[i] != gt.Postings[i] {
						t.Errorf("%s/%s %q posting %d = %v want %v", name, kind, gt.Term, i, got[i], gt.Postings[i])
						break
					}
				}
			}
			if g.TermsComplete {
				all, err := db.TermsWithPrefix("", 0)
				if err != nil || len(all) != g.TermCount {
					t.Errorf("%s/%s TermsWithPrefix(\"\") = %d terms, %v; want %d", name, kind, len(all), err, g.TermCount)
				}
			}
		}
	}
}

func TestDatabaseGoldenDocLengths(t *testing.T) {
	for _, name := range fixtureNames {
		for _, kind := range []string{"fulltext", "title"} {
			g := loadGoldenDB(t, name, kind)
			db := openFixture(t, name, kind)
			for _, dl := range g.DocLens {
				got, err := db.DocLength(dl[0])
				if err != nil || got != dl[1] {
					t.Errorf("%s/%s DocLength(%d) = %d, %v; want %d", name, kind, dl[0], got, err, dl[1])
				}
			}
			if _, err := db.DocLength(db.LastDocID() + 1); !errors.Is(err, ErrDocNotFound) {
				t.Errorf("%s/%s DocLength past end: %v", name, kind, err)
			}
		}
	}
}

// firstChunk encodes the first posting chunk of a term: tf, cf and first
// docid - 1, then the chunk itself.
func firstChunk(tf, cf uint64, last bool, ps ...[2]uint32) []byte {
	b := appendUint(appendUint(nil, tf), cf)
	b = appendUint(b, uint64(ps[0][0]-1))
	return append(b, laterChunk(last, ps...)...)
}

// laterChunk encodes a chunk: '1' (last) or '0', last docid - first docid,
// the first wdf, then (docid gap - 1, wdf) pairs.
func laterChunk(last bool, ps ...[2]uint32) []byte {
	b := []byte{'0'}
	if last {
		b[0] = '1'
	}
	b = appendUint(b, uint64(ps[len(ps)-1][0]-ps[0][0]))
	b = appendUint(b, uint64(ps[0][1]))
	for i := 1; i < len(ps); i++ {
		b = appendUint(b, uint64(ps[i][0]-ps[i-1][0]-1))
		b = appendUint(b, uint64(ps[i][1]))
	}
	return b
}

// postlistDB is a database whose postlist table is one leaf holding the null
// item and the given key -> tag entries; the docdata table is empty.
func postlistDB(lastDocID uint32, entries map[string][]byte) *Database {
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	items := []tItem{{key: "", comp: 1, last: true}}
	for _, k := range keys {
		items = append(items, tItem{key: k, comp: 1, last: true, chunk: entries[k]})
	}
	return &Database{
		postlist: tableOf(1, 0, buildBlock(0, items)),
		docdata:  &table{name: "docdata", empty: true},
		info:     versionInfo{docCount: lastDocID, lastDocID: lastDocID},
	}
}

// tPostings is split into chunks starting at docids 1, 10 and 20.
var tPostings = [][2]uint32{{1, 1}, {3, 2}, {5, 1}, {7, 3}, {10, 1}, {12, 2}, {14, 1}, {20, 4}, {25, 1}, {30, 2}}

// chunkedDB holds 30 documents (length docid+100, in two chunks), "t" in
// three chunks, "u" in one and "v" in two ({3 8 14} and {15 25 29 30}).
func chunkedDB() *Database {
	p := tPostings
	var dl [][2]uint32
	for did := uint32(1); did <= 30; did++ {
		dl = append(dl, [2]uint32{did, did + 100})
	}
	db := postlistDB(30, map[string][]byte{
		string(postlistKey("")):           firstChunk(0, 0, false, dl[:15]...),
		string(postlistChunkKey("", 16)):  laterChunk(true, dl[15:]...),
		string(postlistKey("t")):          firstChunk(10, 18, false, p[:4]...),
		string(postlistChunkKey("t", 10)): laterChunk(false, p[4:7]...),
		string(postlistChunkKey("t", 20)): laterChunk(true, p[7:]...),
		string(postlistKey("u")):          firstChunk(1, 1, true, [2]uint32{2, 1}),
		string(postlistKey("v")):          firstChunk(7, 7, false, [2]uint32{3, 1}, [2]uint32{8, 1}, [2]uint32{14, 1}),
		string(postlistChunkKey("v", 15)): laterChunk(true, [2]uint32{15, 1}, [2]uint32{25, 1}, [2]uint32{29, 1}, [2]uint32{30, 1}),
	})
	db.info.totalLength = 30*100 + 30*31/2
	db.avgLen = float64(db.info.totalLength) / 30
	return db
}

func TestPostingsAcrossChunks(t *testing.T) {
	db := chunkedDB()
	it, err := db.Postings("t")
	if err != nil {
		t.Fatal(err)
	}
	var got [][2]uint32
	for it.Next() {
		got = append(got, [2]uint32{it.DocID(), it.WDF()})
	}
	if it.Err() != nil || !slices.Equal(got, tPostings) {
		t.Fatalf("postings of t = %v, %v", got, it.Err())
	}
	for term, want := range map[string]uint32{"t": 10, "u": 1, "v": 7, "s": 0, "tt": 0, "w": 0, "": 0} {
		if tf, err := db.TermFreq(term); err != nil || tf != want {
			t.Errorf("TermFreq(%q) = %d,%v want %d", term, tf, err, want)
		}
	}
	for _, term := range []string{"", "s", "tt", "w", "\x00"} {
		it, err := db.Postings(term)
		if err != nil || it.Next() || it.Err() != nil {
			t.Errorf("Postings(%q) of a missing term: err %v", term, err)
		}
	}
	for did, want := range map[uint32]uint32{1: 101, 15: 115, 16: 116, 20: 120, 30: 130} {
		if got, err := db.DocLength(did); err != nil || got != want {
			t.Errorf("DocLength(%d) = %d,%v want %d", did, got, err, want)
		}
	}
	for _, did := range []uint32{0, 31} {
		if _, err := db.DocLength(did); !errors.Is(err, ErrDocNotFound) {
			t.Errorf("DocLength(%d): %v", did, err)
		}
	}
}

func TestPostingsSkipTo(t *testing.T) {
	db := chunkedDB()
	wdf := map[uint32]uint32{}
	for _, p := range tPostings {
		wdf[p[0]] = p[1]
	}
	for _, c := range []struct {
		target, want uint32
		ok           bool
	}{
		{1, 1, true}, {2, 3, true}, {7, 7, true},
		{8, 10, true}, // the seek lands on the first chunk, which ends at 7: take the next one
		{11, 12, true}, {14, 14, true},
		{15, 20, true}, // the chunk from 10 ends at 14: take the next one
		{20, 20, true}, {21, 25, true}, {30, 30, true}, {31, 0, false},
	} {
		pl, err := db.openPostings("t")
		if err != nil {
			t.Fatal(err)
		}
		ok, err := pl.skipTo(c.target)
		if err != nil || ok != c.ok || (ok && (pl.DocID() != c.want || pl.WDF() != wdf[c.want])) {
			t.Errorf("skipTo(%d) = %v,%v at %d/%d, want %v at %d", c.target, ok, err, pl.DocID(), pl.WDF(), c.ok, c.want)
		}
	}
	// One list: skips mixed with Next, including a skip to the current docid.
	pl, err := db.openPostings("t")
	if err != nil {
		t.Fatal(err)
	}
	var trace []string
	skip := func(target uint32) {
		ok, err := pl.skipTo(target)
		trace = append(trace, fmt.Sprintf("s%d:%v:%d:%v", target, ok, pl.DocID(), err))
	}
	next := func() {
		ok := pl.Next()
		trace = append(trace, fmt.Sprintf("n:%v:%d:%v", ok, pl.DocID(), pl.Err()))
	}
	skip(2)
	next()
	skip(8)
	next()
	skip(15)
	skip(15)
	next()
	skip(31)
	next()
	want := "[s2:true:3:<nil> n:true:5:<nil> s8:true:10:<nil> n:true:12:<nil> s15:true:20:<nil> " +
		"s15:true:20:<nil> n:true:25:<nil> s31:false:30:<nil> n:false:30:<nil>]"
	if fmt.Sprint(trace) != want {
		t.Errorf("trace = %v\nwant    %s", trace, want)
	}
	// AND search leapfrogs both lists across their chunk boundaries.
	hits, total, err := Search(context.Background(), db, []string{"t", "v"}, OpAnd, 0, 10)
	var dids []uint32
	for _, h := range hits {
		dids = append(dids, h.DocID)
	}
	slices.Sort(dids)
	if err != nil || total != 4 || !slices.Equal(dids, []uint32{3, 14, 25, 30}) {
		t.Errorf("t AND v = %v (total %d), %v; want [3 14 25 30]", dids, total, err)
	}
}

func TestPostingsRejectCorruptChunks(t *testing.T) {
	p := tPostings
	u := func(v uint64) []byte { return appendUint(nil, v) }
	cat := func(parts ...[]byte) []byte { return slices.Concat(parts...) }
	k1 := string(postlistKey("t"))
	head := firstChunk(10, 18, false, p[:4]...) // ends at 7, not last
	cases := map[string]map[string][]byte{
		"truncated term frequency":  {k1: {0x80}},
		"missing chunk header":      {k1: cat(u(1), u(1), u(0))},
		"bad chunk header":          {k1: cat(u(1), u(1), u(0), []byte("x"), u(0), u(1))},
		"first docid overflows":     {k1: cat(u(1), u(1), u(0xffffffff), []byte("1"), u(0), u(1))},
		"chunk end overflows":       {k1: cat(u(1), u(1), u(0xffffffef), []byte("1"), u(0x20), u(1))},
		"docid beyond chunk end":    {k1: cat(u(2), u(2), u(0), []byte("1"), u(2), u(1), u(4), u(1))},
		"chunk ends before its end": {k1: cat(u(1), u(1), u(0), []byte("1"), u(5), u(1))},
		"truncated posting":         {k1: cat(u(2), u(2), u(0), []byte("1"), u(2), u(1), []byte{0x80})},
		"continuation missing":      {k1: head},
		"continuation of another term": {k1: head,
			string(postlistKey("u")): firstChunk(1, 1, true, [2]uint32{2, 1})},
		"chunk docids not increasing": {k1: head,
			string(postlistChunkKey("t", 5)): laterChunk(true, [2]uint32{5, 1}, [2]uint32{6, 1})},
		"chunk key with trailing bytes": {k1: head,
			string(postlistChunkKey("t", 10)) + "\x01": laterChunk(true, p[4:7]...)},
		"chunk key for docid 0": {k1: head,
			string(postlistChunkKey("t", 0)): laterChunk(true, p[4:7]...)},
	}
	for name, entries := range cases {
		db := postlistDB(30, entries)
		it, err := db.Postings("t")
		if err == nil {
			for it.Next() {
			}
			err = it.Err()
		}
		if !errors.Is(err, ErrCorrupt) {
			t.Errorf("%s: err = %v, want ErrCorrupt", name, err)
		}
	}
	// skipTo's jump: the chunk found ends before the target and no chunk follows.
	db := postlistDB(30, map[string][]byte{k1: head, string(postlistChunkKey("t", 10)): laterChunk(false, p[4:7]...)})
	pl, err := db.openPostings("t")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pl.skipTo(15); !errors.Is(err, ErrCorrupt) {
		t.Errorf("skipTo past a missing continuation: err = %v, want ErrCorrupt", err)
	}
	if pl.Next() || !errors.Is(pl.Err(), ErrCorrupt) {
		t.Errorf("list after error: Next true or Err %v", pl.Err())
	}
}
