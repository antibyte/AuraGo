package xapian

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestDatabaseGoldenValues(t *testing.T) {
	for _, name := range fixtureNames {
		for _, kind := range []string{"fulltext", "title"} {
			g := loadGoldenDB(t, name, kind)
			db := openFixture(t, name, kind)
			for slot, rows := range g.Values {
				s := uint32(slot[0] - '0')
				for did, want := range pairs(t, rows) {
					got, err := db.Value(did, s)
					if err != nil || got != want {
						t.Errorf("%s/%s Value(%d,%d) = %q, %v; want %q", name, kind, did, s, shortPath(got), err, shortPath(want))
					}
				}
			}
			// A forward reader returns the same values as random lookups.
			r := db.newValueReader(0)
			for did := uint32(1); did <= db.LastDocID(); did++ {
				fwd, err := r.get(did)
				if err != nil {
					t.Fatal(err)
				}
				if rnd, _ := db.Value(did, 0); fwd != rnd {
					t.Fatalf("%s/%s slot 0 doc %d: forward %q, random %q", name, kind, did, shortPath(fwd), shortPath(rnd))
				}
			}
		}
	}
}

// valueChunkDB builds a slot-0 value stream of n documents, each holding a
// 2000-byte value (none for the docids in unset). The builder cuts a chunk
// once it holds 2000 bytes, so every document gets a value chunk and a leaf
// of its own and a test can steer the seek of any single docid. The values
// are returned by docid (index 0 is unused).
func valueChunkDB(t *testing.T, n int, sep func(level int, key string) string, unset ...uint32) (*Database, []string) {
	t.Helper()
	vals := make([]string, n)
	want := make([]string, n+1)
	for i := range vals {
		vals[i] = fmt.Sprintf("v%d-", i+1)
		vals[i] += strings.Repeat("x", 2000-len(vals[i]))
		want[i+1] = vals[i]
	}
	for _, did := range unset {
		vals[did-1], want[did] = "", ""
	}
	db := synthIndex{blockSize: minBlockSize, docLens: make([]uint32, n),
		values: map[uint32][]string{0: vals}, separator: sep}.build(t)
	if db.postlist.level < 1 {
		t.Fatalf("tree has no branch level to steer (level %d)", db.postlist.level)
	}
	return db, want
}

// slot0Chunk returns the first docid of a slot-0 value chunk key.
func slot0Chunk(key string) (uint32, bool) {
	prefix := valueChunkKey(0, 0)
	prefix = prefix[:len(prefix)-2] // the docid is a two-byte sortable uint below 0x8000
	rest, ok := strings.CutPrefix(key, string(prefix))
	if !ok {
		return 0, false
	}
	first, n, err := unpackSortableUint([]byte(rest))
	if err != nil || n != len(rest) {
		return 0, false
	}
	return uint32(first), true
}

// Lookups in ascending docid order, gaps and the tail past the last value
// included, return the stored values on a valid tree.
func TestValueReaderAscendingLookups(t *testing.T) {
	const n = 40
	db, want := valueChunkDB(t, n, nil, 22, 23, 36, 37, 38, 39, n)
	r := db.newValueReader(0)
	for did := uint32(1); did <= n+5; did++ {
		exp := ""
		if did <= n {
			exp = want[did]
		}
		if got, err := r.get(did); err != nil || got != exp {
			t.Fatalf("get(%d) = %q, %v; want %q", did, shortPath(got), err, shortPath(exp))
		}
	}
	// Out-of-order lookups reload the chunk and agree.
	for _, did := range []uint32{30, 23, 22, 40, 3, 21, 24} {
		if got, err := db.Value(did, 0); err != nil || got != want[did] {
			t.Fatalf("Value(%d) = %q, %v; want %q", did, shortPath(got), err, shortPath(want[did]))
		}
	}
}

// A forward load, after the reader ran off the end of its chunk, must not
// land on an earlier chunk. Raising the separator of chunk 21 to 23 keeps
// the tree in order (docids 22 and 23 hold no value) but sends seeks for
// docid 22 to chunk 20: a reader that was just at chunk 21 would walk back.
func TestValueReaderForwardLoadRejectsEarlierChunk(t *testing.T) {
	const n = 40
	unset := []uint32{22, 23}
	honest, _ := valueChunkDB(t, n, nil, unset...)
	crafted, _ := valueChunkDB(t, n, func(_ int, key string) string {
		if first, ok := slot0Chunk(key); ok && first == 21 {
			return string(valueChunkKey(0, 23))
		}
		return key
	}, unset...)
	for _, db := range []*Database{honest, crafted} {
		r := db.newValueReader(0)
		if got, err := r.get(23); err != nil || got != "" { // lands on chunk 21 and runs off its end
			t.Fatalf("get(23) = %q, %v; want unset", shortPath(got), err)
		}
		got, err := r.get(22) // unset too; the forward load must not leave chunk 21 backwards
		if db == honest {
			if err != nil || got != "" {
				t.Fatalf("honest tree: get(22) = %q, %v; want unset", shortPath(got), err)
			}
			continue
		}
		if !errors.Is(err, ErrCorrupt) || !strings.Contains(err.Error(), "before the current chunk") {
			t.Fatalf("crafted tree: get(22) = %q, %v; want ErrCorrupt (landed before the current chunk)", shortPath(got), err)
		}
	}
	// A fresh reader is steered to chunk 20 for docid 22 as well; the chunk
	// that follows it starts at 21, which a valid tree would have chosen.
	if _, err := crafted.Value(22, 0); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("Value(22) on the crafted tree: %v; want ErrCorrupt", err)
	}
	if got, err := crafted.Value(23, 0); err != nil || got != "" {
		t.Fatalf("Value(23) = %q, %v; want unset", shortPath(got), err)
	}
}

// Separators that send every seek past chunk 20 back to it keep each walk in
// order, and a reader moving forward lands on the chunk it just left: no
// "earlier" chunk, yet no progress. The first lookup beyond chunk 20 must fail
// instead of answering "unset" and re-walking the chunk on every later one.
func TestValueReaderAscendingRejectsSeparatorsSteeringLeft(t *testing.T) {
	const n = 40
	high := string(append(valueChunkKey(0, 0xffffffff), 0xff)) // above every slot-0 key
	steered := 0
	crafted, want := valueChunkDB(t, n, func(_ int, key string) string {
		if first, ok := slot0Chunk(key); ok && first > 20 {
			steered++
			return high
		}
		return key
	})
	if steered == 0 {
		t.Fatal("no branch separator was rewritten")
	}
	r := crafted.newValueReader(0)
	for did := uint32(1); did <= 20; did++ {
		if got, err := r.get(did); err != nil || got != want[did] {
			t.Fatalf("get(%d) = %q, %v", did, shortPath(got), err)
		}
	}
	got, err := r.get(21)
	if !errors.Is(err, ErrCorrupt) {
		t.Fatalf("get(21) = %q, %v; want ErrCorrupt", shortPath(got), err)
	}
	if got, err := crafted.Value(30, 0); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("Value(30) = %q, %v; want ErrCorrupt", shortPath(got), err)
	}
	if got, err := crafted.Value(20, 0); err != nil || got != want[20] {
		t.Fatalf("Value(20) = %q, %v", shortPath(got), err)
	}
}
