package xapian

import (
	"bytes"
	"sort"
	"testing"
)

// synthChunkBytes is the posting chunk size Xapian's glass backend aims for.
const synthChunkBytes = 2000

// synthDB builds an in-memory glass database whose postlist table holds the
// document length list (docLens[i] is the length of docid i+1; 0 = no such
// document) and the given term posting lists ((docid, wdf) pairs in
// ascending docid order), chunked like Xapian and laid out in blockSize
// leaves under as many branch levels as needed. docdata and values are
// empty: hits come back without path and title.
func synthDB(tb testing.TB, blockSize int, docLens []uint32, terms map[string][][2]uint32) *Database {
	tb.Helper()
	type entry struct{ key, tag []byte }
	var entries []entry
	addList := func(term string, ps [][2]uint32, tf, cf uint64) {
		for start := 0; start < len(ps); {
			end, size := start+1, 0
			for end < len(ps) && size < synthChunkBytes {
				size += len(appendUint(appendUint(nil, uint64(ps[end][0]-ps[end-1][0]-1)), uint64(ps[end][1])))
				end++
			}
			last := end == len(ps)
			if start == 0 {
				entries = append(entries, entry{postlistKey(term), firstChunk(tf, cf, last, ps[:end]...)})
			} else {
				entries = append(entries, entry{postlistChunkKey(term, ps[start][0]), laterChunk(last, ps[start:end]...)})
			}
			start = end
		}
	}
	var dl [][2]uint32
	var total uint64
	for i, l := range docLens {
		if l > 0 {
			dl = append(dl, [2]uint32{uint32(i + 1), l})
			total += uint64(l)
		}
	}
	addList("", dl, 0, 0)
	for term, ps := range terms {
		var cf uint64
		for _, p := range ps {
			cf += uint64(p[1])
		}
		addList(term, ps, uint64(len(ps)), cf)
	}
	sort.Slice(entries, func(i, j int) bool { return bytes.Compare(entries[i].key, entries[j].key) < 0 })

	// Pack items into blocks level by level; block 0 is the version block.
	file := make([]byte, blockSize)
	next := uint32(1)
	type ref struct {
		key   string
		comp  int
		block uint32
	}
	pack := func(level int, items []tItem, itemSize func(tItem) int) []ref {
		var refs []ref
		for start := 0; start < len(items); {
			end, used := start, blockHeaderSize
			for end < len(items) && used+2+itemSize(items[end]) <= blockSize {
				used += 2 + itemSize(items[end])
				end++
			}
			if end == start {
				tb.Fatalf("synthDB: item too large for %d-byte blocks", blockSize)
			}
			file = append(file, buildBlockSize(blockSize, level, items[start:end])...)
			refs = append(refs, ref{items[start].key, items[start].comp, next})
			next++
			start = end
		}
		return refs
	}
	leaves := []tItem{{key: "", comp: 1, last: true}}
	for _, e := range entries {
		leaves = append(leaves, tItem{key: string(e.key), comp: 1, last: true, chunk: e.tag})
	}
	refs := pack(0, leaves, func(it tItem) int { return 3 + len(it.key) + len(it.chunk) })
	level := 0
	for len(refs) > 1 {
		level++
		branch := make([]tItem, len(refs))
		for i, r := range refs {
			branch[i] = tItem{key: r.key, comp: r.comp, child: r.block}
		}
		refs = pack(level, branch, func(it tItem) int { return 4 + 1 + len(it.key) + 2 })
	}
	db := &Database{
		postlist: &table{name: "postlist", r: bytes.NewReader(file), blockSize: blockSize, nblocks: next,
			root: refs[0].block, level: level, cache: newBlockCache(defaultCacheBytes / blockSize)},
		docdata: &table{name: "docdata", empty: true},
		info:    versionInfo{docCount: uint32(len(dl)), lastDocID: uint32(len(docLens)), totalLength: total},
	}
	if len(dl) > 0 {
		db.avgLen = float64(total) / float64(len(dl))
	}
	return db
}

// TestSynthDB checks the builder against the readers: every list survives
// chunking and a three-level tree.
func TestSynthDB(t *testing.T) {
	const n = 100000
	lens := make([]uint32, n)
	var odd, every7 [][2]uint32
	for i := range lens {
		did := uint32(i + 1)
		lens[i] = 10 + did%97
		if did%2 == 1 {
			odd = append(odd, [2]uint32{did, 1 + did%5})
		}
		if did%7 == 0 {
			every7 = append(every7, [2]uint32{did, 2})
		}
	}
	lens[99] = 0 // docid 100 does not exist
	db := synthDB(t, minBlockSize, lens, map[string][][2]uint32{"odd": odd, "seven": every7})
	if db.postlist.level < 2 {
		t.Fatalf("tree has %d levels; want at least 3 to cover branch walks", db.postlist.level+1)
	}
	if db.DocCount() != n-1 || db.LastDocID() != n {
		t.Fatalf("DocCount %d, LastDocID %d", db.DocCount(), db.LastDocID())
	}
	var dl [][2]uint32
	for i, l := range lens {
		if l > 0 {
			dl = append(dl, [2]uint32{uint32(i + 1), l})
		}
	}
	for term, want := range map[string][][2]uint32{"": dl, "odd": odd, "seven": every7} {
		it, err := db.openPostings(term)
		if err != nil {
			t.Fatal(err)
		}
		var got [][2]uint32
		for it.Next() {
			got = append(got, [2]uint32{it.DocID(), it.WDF()})
		}
		if it.Err() != nil || len(got) != len(want) {
			t.Fatalf("%q: %d postings (%v), want %d", term, len(got), it.Err(), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%q posting %d = %v, want %v", term, i, got[i], want[i])
			}
		}
		if tf, err := db.TermFreq(term); term != "" && (err != nil || tf != uint32(len(want))) {
			t.Fatalf("%q: TermFreq %d, %v", term, tf, err)
		}
	}
	for _, did := range []uint32{1, 2, 99, 101, 4095, 4096, 50001, n} {
		if got, err := db.DocLength(did); err != nil || got != 10+did%97 {
			t.Fatalf("DocLength(%d) = %d, %v", did, got, err)
		}
	}
	if _, err := db.DocLength(100); err != ErrDocNotFound {
		t.Fatalf("DocLength(100): %v, want ErrDocNotFound", err)
	}
}
