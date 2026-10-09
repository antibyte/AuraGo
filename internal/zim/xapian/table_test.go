package xapian

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
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
	b := make([]byte, testBlockSize)
	b[4] = byte(level)
	dirEnd := blockHeaderSize + 2*len(items)
	binary.BigEndian.PutUint16(b[9:], uint16(dirEnd))
	end := testBlockSize
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
