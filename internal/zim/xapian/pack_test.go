package xapian

import (
	"bytes"
	"errors"
	"math"
	"testing"
)

func TestUnpackUint(t *testing.T) {
	cases := []struct {
		in   []byte
		want uint64
		n    int
	}{
		{[]byte{0x00}, 0, 1},
		{[]byte{0x7f, 0xaa}, 127, 1},
		{[]byte{0x80, 0x01}, 128, 2},
		{[]byte{0xac, 0x02}, 300, 2},
		{[]byte{0xff, 0xff, 0xff, 0xff, 0x0f}, math.MaxUint32, 5},
		{[]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01}, math.MaxUint64, 10},
	}
	for _, c := range cases {
		got, n, err := unpackUint(c.in)
		if err != nil || got != c.want || n != c.n {
			t.Errorf("unpackUint(% x) = %d,%d,%v want %d,%d", c.in, got, n, err, c.want, c.n)
		}
		if enc := appendUint(nil, c.want); !bytes.Equal(enc, c.in[:c.n]) {
			t.Errorf("appendUint(%d) = % x want % x", c.want, enc, c.in[:c.n])
		}
	}
	for _, bad := range [][]byte{nil, {0x80}, {0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x02},
		{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x81, 0x00}} { // tenth byte continues
		if _, _, err := unpackUint(bad); !errors.Is(err, ErrCorrupt) {
			t.Errorf("unpackUint(% x) err = %v, want ErrCorrupt", bad, err)
		}
	}
	if _, _, err := unpackUint32([]byte{0x80, 0x80, 0x80, 0x80, 0x10}); !errors.Is(err, ErrCorrupt) {
		t.Errorf("unpackUint32 overflow err = %v", err)
	}
}

func TestSortableUint(t *testing.T) {
	cases := []struct {
		v   uint64
		enc []byte
	}{
		{0, []byte{0x00, 0x00}},
		{1, []byte{0x00, 0x01}},
		{0x7fff, []byte{0x7f, 0xff}},
		{0x8000, []byte{0x80, 0x80, 0x00}},
		{0x3fffff, []byte{0xbf, 0xff, 0xff}},
		{0x400000, []byte{0xc0, 0x40, 0x00, 0x00}},
		{math.MaxUint32, []byte{0xe0, 0xff, 0xff, 0xff, 0xff}},
		{math.MaxUint64, []byte{0xfe, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
	}
	for _, c := range cases {
		if got := appendSortableUint(nil, c.v); !bytes.Equal(got, c.enc) {
			t.Errorf("appendSortableUint(%#x) = % x want % x", c.v, got, c.enc)
		}
		got, n, err := unpackSortableUint(append(append([]byte(nil), c.enc...), 0x99))
		if err != nil || got != c.v || n != len(c.enc) {
			t.Errorf("unpackSortableUint(% x) = %#x,%d,%v", c.enc, got, n, err)
		}
	}
	// Encodings sort like the numbers.
	prev := appendSortableUint(nil, 0)
	for _, v := range []uint64{1, 255, 0x7fff, 0x8000, 0x10000, 0x3fffff, 0x400000, 1 << 31, math.MaxUint32} {
		cur := appendSortableUint(nil, v)
		if bytes.Compare(prev, cur) >= 0 {
			t.Errorf("encoding of %#x does not sort after its predecessor", v)
		}
		prev = cur
	}
	for _, bad := range [][]byte{nil, {0x12}, {0xff, 0, 0}, {0x80, 0x01}} {
		if _, _, err := unpackSortableUint(bad); !errors.Is(err, ErrCorrupt) {
			t.Errorf("unpackSortableUint(% x) err = %v", bad, err)
		}
	}
}

func TestSortPreservingString(t *testing.T) {
	for _, s := range []string{"", "berlin", "a\x00b", "\x00", "zz\x00"} {
		enc := appendSortPreservingString(nil, s, false)
		got, n, term := unpackSortPreservingString(append(enc, 0x01, 0x02))
		if got != s || n != len(enc) || !term {
			t.Errorf("round trip %q: got %q n=%d term=%v (enc % x)", s, got, n, term, enc)
		}
		last := appendSortPreservingString(nil, s, true)
		got, n, term = unpackSortPreservingString(last)
		if got != s || n != len(last) || term {
			t.Errorf("last=true %q: got %q n=%d term=%v", s, got, n, term)
		}
	}
	if k := postlistKey(""); !bytes.Equal(k, []byte{0x00, 0xe0}) {
		t.Errorf("doclen key = % x", k)
	}
	if k := postlistChunkKey("ab", 0x8000); !bytes.Equal(k, []byte{'a', 'b', 0x00, 0x80, 0x80, 0x00}) {
		t.Errorf("chunk key = % x", k)
	}
	if k := valueChunkKey(1, 3); !bytes.Equal(k, []byte{0x00, 0xd8, 0x01, 0x00, 0x03}) {
		t.Errorf("value chunk key = % x", k)
	}
	if k := metadataKey("language"); !bytes.Equal(k, append([]byte{0x00, 0xc0}, "language"...)) {
		t.Errorf("metadata key = % x", k)
	}
}
