package xapian

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"testing"
)

const testBlockSize = 2048

// tItem describes one synthetic B-tree item.
type tItem struct {
	key        string
	comp       int // component number (1 = first)
	last       bool
	compressed bool
	chunk      []byte
	child      uint32 // branch items only
}

// buildBlock lays out a glass block: 11-byte header, directory of 2-byte
// offsets, items packed at the end of the block.
func buildBlock(level int, items []tItem) []byte {
	return buildBlockSize(testBlockSize, level, items)
}

func buildBlockSize(size, level int, items []tItem) []byte {
	b := make([]byte, size)
	b[4] = byte(level)
	dirEnd := blockHeaderSize + 2*len(items)
	binary.BigEndian.PutUint16(b[9:], uint16(dirEnd))
	end := size
	for i, it := range items {
		var raw []byte
		if level == 0 {
			size := 3 + len(it.key) + len(it.chunk)
			if it.comp > 1 {
				size += 2
			}
			flags := byte(0)
			if it.comp == 1 {
				flags |= itemFirst
			}
			if it.last {
				flags |= itemLast
			}
			if it.compressed {
				flags |= itemCompressed
			}
			raw = binary.BigEndian.AppendUint16(nil, uint16(size-3))
			raw[0] |= flags
			raw = append(raw, byte(len(it.key)))
			raw = append(raw, it.key...)
			if it.comp > 1 {
				raw = binary.BigEndian.AppendUint16(raw, uint16(it.comp))
			}
			raw = append(raw, it.chunk...)
		} else {
			raw = binary.BigEndian.AppendUint32(nil, it.child)
			raw = append(raw, byte(len(it.key)))
			raw = append(raw, it.key...)
			raw = binary.BigEndian.AppendUint16(raw, uint16(it.comp))
		}
		end -= len(raw)
		copy(b[end:], raw)
		binary.BigEndian.PutUint16(b[blockHeaderSize+2*i:], uint16(end))
	}
	return b
}

// tableOf lays out blocks 1..n (all of one size) after an unused block 0.
func tableOf(root uint32, level int, blocks ...[]byte) *table {
	size := len(blocks[0])
	file := make([]byte, size)
	for _, b := range blocks {
		file = append(file, b...)
	}
	return &table{name: "test", r: bytes.NewReader(file), blockSize: size, nblocks: uint32(len(blocks) + 1),
		root: root, level: level, cache: newBlockCache(16)}
}

func deflateRaw(t *testing.T, s string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, _ := flate.NewWriter(&buf, flate.DefaultCompression)
	w.Write([]byte(s))
	w.Close()
	return buf.Bytes()
}

// testTable: block 0 unused, leaves 1 and 2, root branch 3. The tag of
// "berry" spans both leaves; "fig" is compressed.
func testTable(t *testing.T) *table {
	t.Helper()
	leaf1 := buildBlock(0, []tItem{
		{key: "", comp: 1, last: true},
		{key: "apple", comp: 1, last: true, chunk: []byte("A")},
		{key: "berry", comp: 1, chunk: []byte("B1")},
	})
	leaf2 := buildBlock(0, []tItem{
		{key: "berry", comp: 2, last: true, chunk: []byte("B2")},
		{key: "cherry", comp: 1, last: true, chunk: []byte("C")},
		{key: "fig", comp: 1, last: true, compressed: true, chunk: deflateRaw(t, "fig fig fig fig")},
	})
	root := buildBlock(1, []tItem{
		{key: "", comp: 0, child: 1},
		{key: "berry", comp: 2, child: 2},
	})
	file := append(append(append(make([]byte, testBlockSize), leaf1...), leaf2...), root...)
	return &table{name: "test", r: bytes.NewReader(file), blockSize: testBlockSize, nblocks: 4,
		root: 3, level: 1, cache: newBlockCache(16)}
}

func TestCursorSeekLE(t *testing.T) {
	tb := testTable(t)
	cases := []struct {
		key, want    string
		found, exact bool
	}{
		{"", "", true, true},
		{"apple", "apple", true, true},
		{"b", "apple", true, false},
		{"berry", "berry", true, true},
		{"berry\x00", "berry", true, false},
		{"c", "berry", true, false},
		{"cherry", "cherry", true, true},
		{"zzz", "fig", true, false},
	}
	for _, c := range cases {
		cur := tb.cursor()
		found, exact, err := cur.seekLE([]byte(c.key))
		if err != nil || found != c.found || exact != c.exact {
			t.Fatalf("seekLE(%q) = %v,%v,%v", c.key, found, exact, err)
		}
		k, _ := cur.key()
		if string(k) != c.want {
			t.Errorf("seekLE(%q) at %q, want %q", c.key, k, c.want)
		}
	}
}

func TestCursorTagsAndIteration(t *testing.T) {
	tb := testTable(t)
	tag, ok, err := tb.get([]byte("berry"))
	if err != nil || !ok || string(tag) != "B1B2" {
		t.Fatalf("berry tag = %q,%v,%v", tag, ok, err)
	}
	tag, ok, err = tb.get([]byte("fig"))
	if err != nil || !ok || string(tag) != "fig fig fig fig" {
		t.Fatalf("compressed tag = %q,%v,%v", tag, ok, err)
	}
	if _, ok, _ := tb.get([]byte("banana")); ok {
		t.Fatal("banana found")
	}
	cur := tb.cursor()
	var keys []string
	ok, err = cur.first()
	for ; ok && err == nil; ok, err = cur.next() {
		k, _ := cur.key()
		keys = append(keys, string(k))
	}
	if err != nil || len(keys) != 5 || keys[2] != "berry" || keys[3] != "cherry" {
		t.Fatalf("iteration = %q, %v", keys, err)
	}
	for _, c := range []struct{ from, want string }{{"b", "berry"}, {"cherry", "cherry"}, {"d", "fig"}} {
		cur := tb.cursor()
		ok, err := cur.seekGE([]byte(c.from))
		k, _ := cur.key()
		if err != nil || !ok || string(k) != c.want {
			t.Errorf("seekGE(%q) = %q,%v,%v", c.from, k, ok, err)
		}
	}
	if ok, err := tb.cursor().seekGE([]byte("zzz")); ok || err != nil {
		t.Errorf("seekGE past end = %v,%v", ok, err)
	}
}

func TestCursorRejectsCorruptBlocks(t *testing.T) {
	tb := testTable(t)
	file := tb.r.(*bytes.Reader)
	raw := make([]byte, file.Size())
	file.ReadAt(raw, 0)
	mutate := func(f func([]byte)) *table {
		cp := append([]byte(nil), raw...)
		f(cp)
		return &table{name: "test", r: bytes.NewReader(cp), blockSize: testBlockSize, nblocks: 4,
			root: 3, level: 1, cache: newBlockCache(16)}
	}
	bad := []*table{
		mutate(func(b []byte) { binary.BigEndian.PutUint16(b[3*testBlockSize+9:], 0xffff) }), // dir_end
		mutate(func(b []byte) { b[2*testBlockSize+4] = 1 }),                                  // leaf claims level 1
		mutate(func(b []byte) { b[2*testBlockSize+4] = levelFreelist }),                      // freelist block in tree
		mutate(func(b []byte) { // second branch item points outside the file
			o := int(binary.BigEndian.Uint16(b[3*testBlockSize+13:]))
			binary.BigEndian.PutUint32(b[3*testBlockSize+o:], 99)
		}),
	}
	for i, tb := range bad {
		_, _, err := tb.cursor().seekLE([]byte("cherry"))
		if !errors.Is(err, ErrCorrupt) {
			t.Errorf("case %d: err = %v, want ErrCorrupt", i, err)
		}
	}
}

// itemAt returns the offset of item i of the block starting at base in file.
func itemAt(file []byte, base, i int) int {
	return base + int(binary.BigEndian.Uint16(file[base+blockHeaderSize+2*i:]))
}

func TestCursorRejectsMalformedItems(t *testing.T) {
	tb := testTable(t)
	file := tb.r.(*bytes.Reader)
	raw := make([]byte, file.Size())
	file.ReadAt(raw, 0)
	leaf2, root := 2*testBlockSize, 3*testBlockSize
	cases := map[string]func([]byte){
		"empty leaf": func(b []byte) { binary.BigEndian.PutUint16(b[leaf2+9:], blockHeaderSize) },
		"leaf item overruns block": func(b []byte) { // "cherry": size field 0x1fff
			o := itemAt(b, leaf2, 1)
			b[o] |= 0x1f
			b[o+1] = 0xff
		},
		"leaf key overruns item": func(b []byte) { // "cherry": key length 255
			b[itemAt(b, leaf2, 1)+2] = 0xff
		},
		"branch item overruns block": func(b []byte) { // item 0 sits at the block end
			b[itemAt(b, root, 0)+4] = 0xff
		},
		"item offset inside directory": func(b []byte) {
			binary.BigEndian.PutUint16(b[leaf2+blockHeaderSize+2:], blockHeaderSize)
		},
	}
	for name, f := range cases {
		cp := append([]byte(nil), raw...)
		f(cp)
		bad := &table{name: "test", r: bytes.NewReader(cp), blockSize: testBlockSize, nblocks: 4,
			root: 3, level: 1, cache: newBlockCache(16)}
		_, _, err := bad.cursor().seekLE([]byte("cherry"))
		if err == nil {
			_, err = bad.cursor().first()
		}
		if !errors.Is(err, ErrCorrupt) {
			t.Errorf("%s: err = %v, want ErrCorrupt", name, err)
		}
	}
}

// TestCursorRejectsSharedChildren: branch items that share a child turn the
// tree into a DAG. Walking it must fail at the first repeated item instead of
// visiting the leaf once per path (here 100 x 100 times).
func TestCursorRejectsSharedChildren(t *testing.T) {
	leaf := buildBlock(0, []tItem{
		{key: "", comp: 1, last: true},
		{key: "a", comp: 1, last: true},
		{key: "b", comp: 1, last: true},
		{key: "c", comp: 1, last: true},
	})
	fan := func(level int, child uint32) []byte {
		items := []tItem{{key: "", comp: 0, child: child}}
		for i := 1; i < 100; i++ {
			items = append(items, tItem{key: fmt.Sprintf("k%03d", i), comp: 1, child: child})
		}
		return buildBlock(level, items)
	}
	tb := tableOf(3, 2, leaf, fan(1, 1), fan(2, 2))
	cur := tb.cursor()
	n := 0
	ok, err := cur.first()
	for ; ok && err == nil && n < 1000; ok, err = cur.next() {
		n++
	}
	if !errors.Is(err, ErrCorrupt) || n > 4 {
		t.Fatalf("walk over shared children: %d entries, err = %v, want ErrCorrupt after at most 4", n, err)
	}
}

// TestCursorSeekRejectsSharedChildrenBackward: seekLE steps back over
// continuation components; through shared children that loop must fail too.
func TestCursorSeekRejectsSharedChildrenBackward(t *testing.T) {
	leaf1 := buildBlock(0, []tItem{
		{key: "", comp: 1, last: true},
		{key: "k", comp: 1, chunk: []byte("1")},
	})
	leaf2 := buildBlock(0, []tItem{
		{key: "k", comp: 2, chunk: []byte("2")},
		{key: "k", comp: 3, chunk: []byte("3")},
	})
	items := []tItem{{key: "", comp: 0, child: 1}}
	for i := 1; i < 100; i++ {
		items = append(items, tItem{key: "k", comp: 2 * i, child: 2})
	}
	tb := tableOf(3, 1, leaf1, leaf2, buildBlock(1, items))
	found, _, err := tb.cursor().seekLE([]byte("z"))
	if !errors.Is(err, ErrCorrupt) {
		t.Fatalf("seekLE over shared children = %v,%v, want ErrCorrupt", found, err)
	}
}

func TestCursorKeepsMultiComponentOrder(t *testing.T) {
	// One tag in four components spread over three leaves, with separators
	// that carry component numbers: forward and backward steps stay valid.
	l1 := buildBlock(0, []tItem{{key: "", comp: 1, last: true}, {key: "k", comp: 1, chunk: []byte("a")}})
	l2 := buildBlock(0, []tItem{{key: "k", comp: 2, chunk: []byte("b")}, {key: "k", comp: 3, chunk: []byte("c")}})
	l3 := buildBlock(0, []tItem{{key: "k", comp: 4, last: true, chunk: []byte("d")}, {key: "l", comp: 1, last: true}})
	root := buildBlock(1, []tItem{{key: "", comp: 0, child: 1}, {key: "k", comp: 2, child: 2}, {key: "k", comp: 4, child: 3}})
	tb := tableOf(4, 1, l1, l2, l3, root)
	tag, ok, err := tb.get([]byte("k"))
	if err != nil || !ok || string(tag) != "abcd" {
		t.Fatalf("tag = %q,%v,%v", tag, ok, err)
	}
	for _, key := range []string{"k\x00", "kz"} {
		cur := tb.cursor()
		found, exact, err := cur.seekLE([]byte(key))
		k, _ := cur.key()
		if err != nil || !found || exact || string(k) != "k" {
			t.Errorf("seekLE(%q) = %q,%v,%v,%v", key, k, found, exact, err)
		}
	}
	var keys []string
	cur := tb.cursor()
	for ok, err = cur.first(); ok && err == nil; ok, err = cur.next() {
		k, _ := cur.key()
		keys = append(keys, string(k))
	}
	if err != nil || fmt.Sprint(keys) != "[ k l]" {
		t.Fatalf("walk = %q, %v", keys, err)
	}
}

func TestCursorRejectsBadTags(t *testing.T) {
	cases := map[string][]tItem{
		"component skipped": {{key: "k", comp: 1, chunk: []byte("a")}, {key: "k", comp: 3, last: true, chunk: []byte("c")}},
		"other key follows": {{key: "k", comp: 1, chunk: []byte("a")}, {key: "l", comp: 1, last: true}},
		"tag ends early":    {{key: "k", comp: 1, chunk: []byte("a")}},
		"invalid deflate":   {{key: "k", comp: 1, last: true, compressed: true, chunk: []byte{0xff, 0xff, 0xff}}},
		"truncated deflate": {{key: "k", comp: 1, last: true, compressed: true, chunk: deflateRaw(t, "hello hello")[:3]}},
	}
	for name, items := range cases {
		items = append([]tItem{{key: "", comp: 1, last: true}}, items...)
		tb := tableOf(1, 0, buildBlock(0, items))
		if _, _, err := tb.get([]byte("k")); !errors.Is(err, ErrCorrupt) {
			t.Errorf("%s: err = %v, want ErrCorrupt", name, err)
		}
	}
}

// TestCursorBoundsInflatedTags: a small compressed tag that inflates past
// maxTagSize fails without materialising more than maxTagSize+1 bytes.
func TestCursorBoundsInflatedTags(t *testing.T) {
	var buf bytes.Buffer
	w, _ := flate.NewWriter(&buf, flate.BestCompression)
	w.Write(make([]byte, maxTagSize+1024))
	w.Close()
	bomb := buf.Bytes()
	items := []tItem{{key: "", comp: 1, last: true}}
	for comp := 1; len(bomb) > 0; comp++ {
		n := min(len(bomb), 8000)
		items = append(items, tItem{key: "bomb", comp: comp, last: n == len(bomb), compressed: comp == 1, chunk: bomb[:n]})
		bomb = bomb[n:]
	}
	tb := tableOf(1, 0, buildBlockSize(maxBlockSize, 0, items))
	if _, _, err := tb.get([]byte("bomb")); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("inflate bomb: err = %v, want ErrCorrupt", err)
	}
}

func TestCursorEmptyTable(t *testing.T) {
	tb := &table{name: "test", r: bytes.NewReader(nil), blockSize: testBlockSize, nblocks: 1, empty: true,
		cache: newBlockCache(16)}
	if ok, err := tb.cursor().first(); ok || err != nil {
		t.Errorf("first() on empty table = %v,%v, want false,nil", ok, err)
	}
	if ok, err := tb.cursor().seekGE([]byte("a")); ok || err != nil {
		t.Errorf("seekGE on empty table = %v,%v, want false,nil", ok, err)
	}
}

// eofAtEnd reports io.EOF together with a complete read that ends the input,
// which io.ReaderAt permits.
type eofAtEnd struct{ *bytes.Reader }

func (r eofAtEnd) ReadAt(p []byte, off int64) (int, error) {
	n, err := r.Reader.ReadAt(p, off)
	if err == nil && off+int64(n) == r.Size() {
		err = io.EOF
	}
	return n, err
}

func TestReadBlockAcceptsFullReadWithEOF(t *testing.T) {
	tb := testTable(t)
	tb.r = eofAtEnd{tb.r.(*bytes.Reader)} // the root is the last block
	if tag, ok, err := tb.get([]byte("cherry")); err != nil || !ok || string(tag) != "C" {
		t.Fatalf("get with EOF on a full read = %q,%v,%v", tag, ok, err)
	}
	tb = testTable(t)
	tb.nblocks, tb.root = 5, 4 // block 4 lies past the end of the input
	if _, _, err := tb.cursor().seekLE([]byte("a")); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("short read: err = %v, want ErrCorrupt", err)
	}
}
