package xapian

import (
	"bytes"
	"fmt"
	"sort"
	"testing"
)

// synthChunkBytes is the posting and value chunk size Xapian's glass backend
// aims for.
const synthChunkBytes = 2000

// synthIndex describes an in-memory glass database for tests and benchmarks.
type synthIndex struct {
	blockSize int
	docLens   []uint32               // length of docid i+1; 0 = no such document
	terms     map[string][][2]uint32 // term -> (docid, wdf) in ascending docid order
	values    map[uint32][]string    // slot -> value of docid i+1 ("" = unset)
	data      []string               // document data of docid i+1 ("" = none)
	// separator, when set, rewrites the key of a branch item (the first key
	// of its child) before the block is written, to craft damaged trees.
	separator func(level int, key string) string
}

// synthDB builds a database whose postlist table holds the document length
// list and the given posting lists; docdata and values are empty, so hits come
// back without path and title.
func synthDB(tb testing.TB, blockSize int, docLens []uint32, terms map[string][][2]uint32) *Database {
	tb.Helper()
	return synthIndex{blockSize: blockSize, docLens: docLens, terms: terms}.build(tb)
}

// build lays the index out like Xapian: posting and value chunks of about
// synthChunkBytes, blockSize leaves under as many branch levels as needed,
// block 0 left for the version block.
func (s synthIndex) build(tb testing.TB) *Database {
	tb.Helper()
	type entry struct{ key, tag []byte }
	var post []entry
	addList := func(term string, ps [][2]uint32, tf, cf uint64) {
		for start := 0; start < len(ps); {
			end, size := start+1, 0
			for end < len(ps) && size < synthChunkBytes {
				size += len(appendUint(appendUint(nil, uint64(ps[end][0]-ps[end-1][0]-1)), uint64(ps[end][1])))
				end++
			}
			last := end == len(ps)
			if start == 0 {
				post = append(post, entry{postlistKey(term), firstChunk(tf, cf, last, ps[:end]...)})
			} else {
				post = append(post, entry{postlistChunkKey(term, ps[start][0]), laterChunk(last, ps[start:end]...)})
			}
			start = end
		}
	}
	var dl [][2]uint32
	var total uint64
	for i, l := range s.docLens {
		if l > 0 {
			dl = append(dl, [2]uint32{uint32(i + 1), l})
			total += uint64(l)
		}
	}
	addList("", dl, 0, 0)
	for term, ps := range s.terms {
		var cf uint64
		for _, p := range ps {
			cf += uint64(p[1])
		}
		addList(term, ps, uint64(len(ps)), cf)
	}
	for slot, vals := range s.values {
		var tag []byte
		var first, prev uint32
		flush := func() {
			if tag != nil {
				post = append(post, entry{valueChunkKey(slot, first), tag})
				tag = nil
			}
		}
		for i, v := range vals {
			if v == "" {
				continue
			}
			did := uint32(i + 1)
			if tag == nil {
				first = did
			} else {
				tag = appendUint(tag, uint64(did-prev-1))
			}
			tag = append(appendUint(tag, uint64(len(v))), v...)
			prev = did
			if len(tag) >= synthChunkBytes {
				flush()
			}
		}
		flush()
	}
	var docs []entry
	for i, d := range s.data {
		if d != "" {
			docs = append(docs, entry{docdataKey(uint32(i + 1)), []byte(d)})
		}
	}

	file := make([]byte, s.blockSize)
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
			for end < len(items) && used+2+itemSize(items[end]) <= s.blockSize {
				used += 2 + itemSize(items[end])
				end++
			}
			if end == start {
				tb.Fatalf("synthIndex: item too large for %d-byte blocks", s.blockSize)
			}
			file = append(file, buildBlockSize(s.blockSize, level, items[start:end])...)
			refs = append(refs, ref{items[start].key, items[start].comp, next})
			next++
			start = end
		}
		return refs
	}
	// buildTable writes one B-tree and returns its root and level (nil when
	// the table is empty).
	buildTable := func(name string, entries []entry) *table {
		if len(entries) == 0 {
			return &table{name: name, empty: true}
		}
		sort.Slice(entries, func(i, j int) bool { return bytes.Compare(entries[i].key, entries[j].key) < 0 })
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
				key := r.key
				if s.separator != nil && i > 0 {
					key = s.separator(level, key)
				}
				branch[i] = tItem{key: key, comp: r.comp, child: r.block}
			}
			refs = pack(level, branch, func(it tItem) int { return 4 + 1 + len(it.key) + 2 })
		}
		return &table{name: name, blockSize: s.blockSize, root: refs[0].block, level: level}
	}
	postlist := buildTable("postlist", post)
	docdata := buildTable("docdata", docs)
	cache := newBlockCache(defaultCacheBytes / s.blockSize)
	r := bytes.NewReader(file)
	for _, t := range []*table{postlist, docdata} {
		t.r, t.nblocks, t.cache, t.blockSize = r, next, cache, s.blockSize
	}
	db := &Database{
		postlist: postlist,
		docdata:  docdata,
		info:     versionInfo{docCount: uint32(len(dl)), lastDocID: uint32(len(s.docLens)), totalLength: total},
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

// Values and document data survive chunking, and the docdata table gets its
// own tree in the same file.
func TestSynthIndexValuesAndData(t *testing.T) {
	const n = 3000
	lens := make([]uint32, n)
	titles := make([]string, n)
	targets := make([]string, n)
	data := make([]string, n)
	for i := range lens {
		lens[i] = 5
		titles[i] = fmt.Sprintf("Title %d", i+1)
		if i%3 != 1 {
			targets[i] = fmt.Sprintf("Target_%d", i/3)
		}
		data[i] = fmt.Sprintf("C/Page_%d", i+1)
	}
	db := synthIndex{blockSize: minBlockSize, docLens: lens, data: data,
		values: map[uint32][]string{0: titles, 1: targets}}.build(t)
	if db.docdata.empty || db.docdata.level == 0 {
		t.Fatalf("docdata table: empty %v, level %d", db.docdata.empty, db.docdata.level)
	}
	for _, did := range []uint32{1, 2, 3, 150, 1999, n} {
		i := did - 1
		if v, err := db.Value(did, 0); err != nil || v != titles[i] {
			t.Errorf("Value(%d, 0) = %q, %v", did, v, err)
		}
		if v, err := db.Value(did, 1); err != nil || v != targets[i] {
			t.Errorf("Value(%d, 1) = %q, %v; want %q", did, v, err, targets[i])
		}
		if d, err := db.Data(did); err != nil || d != data[i] {
			t.Errorf("Data(%d) = %q, %v", did, d, err)
		}
	}
}
