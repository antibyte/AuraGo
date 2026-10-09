package xapian

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

// versionBlock builds block 0 of a single-file glass database. roots holds
// (root, level, fake) per table; all tables use 8 KiB blocks.
func versionBlock(magic string, format uint16, roots [tableCount][3]uint64, stats [8]uint64) []byte {
	b := []byte(magic)
	b = binary.BigEndian.AppendUint16(b, format)
	b = append(b, bytes.Repeat([]byte{0xab}, 16)...) // UUID
	b = appendUint(b, 1)                             // revision
	for _, r := range roots {
		flags := r[1] << 2
		if r[2] != 0 {
			flags |= 1
		}
		b = appendUint(b, r[0])
		b = appendUint(b, flags)
		b = appendUint(b, 7)        // entries
		b = appendUint(b, 8192>>11) // block size
		b = appendUint(b, 0)        // compress_min
		b = appendUint(b, 0)        // empty freelist
	}
	for _, s := range stats {
		b = appendUint(b, s)
	}
	out := make([]byte, 8192)
	copy(out, b)
	return out
}

var liveRoots = [tableCount][3]uint64{{1, 0, 0}, {2, 0, 0}, {0, 0, 1}, {0, 0, 1}, {0, 0, 1}, {0, 0, 1}}

var fakeRoots = [tableCount][3]uint64{{0, 0, 1}, {0, 0, 1}, {0, 0, 1}, {0, 0, 1}, {0, 0, 1}, {0, 0, 1}}

// With liveRoots every root info varint is one byte: the block size field of
// table t sits after the revision byte, t earlier root infos of 6 bytes and
// the root, flags and entries fields.
func blockSizeOffset(t int) int { return versionHeaderSize + 1 + 6*t + 3 }

func TestParseVersion(t *testing.T) {
	// doccount 4, lastdocid 4, doclen bounds 12..16, wdf ub 4, total 56.
	v, err := parseVersion(versionBlock(glassMagic, glassFormatVersion, liveRoots, [8]uint64{4, 0, 12, 4, 12, 0, 56, 0}))
	if err != nil {
		t.Fatal(err)
	}
	if v.docCount != 4 || v.lastDocID != 4 || v.totalLength != 56 || v.doclenLower != 12 || v.doclenUpper != 16 || v.wdfUpper != 4 {
		t.Fatalf("stats = %+v", v)
	}
	pl := v.roots[tablePostlist]
	if pl.root != 1 || pl.level != 0 || pl.rootIsFake || pl.blockSize != 8192 || pl.numEntries != 7 {
		t.Fatalf("postlist root info = %+v", pl)
	}
	if !v.roots[tableTermlist].rootIsFake {
		t.Fatal("termlist should be fake (libzim uses DB_NO_TERMLIST)")
	}
	// The total length is the one 64-bit statistic.
	if v, err := parseVersion(versionBlock(glassMagic, glassFormatVersion, liveRoots, [8]uint64{4, 0, 12, 4, 12, 0, 1 << 40, 0})); err != nil || v.totalLength != 1<<40 {
		t.Fatalf("total length 2^40: %d, %v", v.totalLength, err)
	}
}

func TestParseVersionRejects(t *testing.T) {
	stats := [8]uint64{1, 0, 1, 1, 0, 0, 1, 0}
	deep := liveRoots
	deep[tablePostlist] = [3]uint64{1, 10, 0}
	// flags 0x200000001: level 0x80000000 wraps to a negative int on 32-bit
	// platforms if it is narrowed before the range check.
	huge := fakeRoots
	huge[tablePostlist] = [3]uint64{0, 0x80000000, 1}
	badBS := versionBlock(glassMagic, glassFormatVersion, liveRoots, stats)
	bigBS := versionBlock(glassMagic, glassFormatVersion, liveRoots, stats)
	bigBS[blockSizeOffset(tablePostlist)] = 64 // 128 KiB
	vb := versionBlock(glassMagic, glassFormatVersion, liveRoots, stats)
	bigRev := append(append(append([]byte(nil), vb[:versionHeaderSize]...), appendUint(nil, 1<<32)...), vb[versionHeaderSize+1:]...)
	withStats := func(st [8]uint64) []byte { return versionBlock(glassMagic, glassFormatVersion, liveRoots, st) }
	cases := []struct {
		name string
		b    []byte
		want error
	}{
		{"honey", versionBlock("\x0f\x0dXapian Honey", glassFormatVersion, liveRoots, stats), ErrUnsupportedFormat},
		{"not xapian", bytes.Repeat([]byte("x"), 64), ErrUnsupportedFormat},
		{"future glass", versionBlock(glassMagic, glassFormatVersion+1, liveRoots, stats), ErrUnsupportedFormat},
		{"level", versionBlock(glassMagic, glassFormatVersion, deep, stats), ErrCorrupt},
		{"level overflows int", versionBlock(glassMagic, glassFormatVersion, huge, [8]uint64{}), ErrCorrupt},
		{"block size field too large", bigBS, ErrUnsupportedFormat},
		{"revision beyond 32 bits", bigRev, ErrCorrupt},
		{"last docid wraps", withStats([8]uint64{1, 1<<64 - 1, 1, 1, 0, 0, 1, 0}), ErrCorrupt},
		{"last docid beyond 32 bits", withStats([8]uint64{1, 0xffffffff, 1, 1, 0, 0, 1, 0}), ErrCorrupt},
		{"doclen lower bound beyond 32 bits", withStats([8]uint64{1, 0, 1 << 32, 1, 0, 0, 1, 0}), ErrCorrupt},
		{"wdf upper bound beyond 32 bits", withStats([8]uint64{1, 0, 1, 1 << 32, 0, 0, 1, 0}), ErrCorrupt},
		{"doclen upper bound overflows", withStats([8]uint64{1, 0, 1, 1, 0xffffffff, 0, 1, 0}), ErrCorrupt},
		{"oldest changeset beyond 32 bits", withStats([8]uint64{1, 0, 1, 1, 0, 1 << 32, 1, 0}), ErrCorrupt},
		{"spelling bound beyond 32 bits", withStats([8]uint64{1, 0, 1, 1, 0, 0, 1, 1 << 32}), ErrCorrupt},
		{"truncated", versionBlock(glassMagic, glassFormatVersion, liveRoots, stats)[:34], ErrCorrupt},
		{"block size", func() []byte { badBS[versionHeaderSize+1+3] = 3; return badBS }(), ErrUnsupportedFormat},
	}
	for _, c := range cases {
		if _, err := parseVersion(c.b); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
}

func TestOpenRejectsNonGlass(t *testing.T) {
	if _, err := Open(bytes.NewReader(make([]byte, 100)), 100); !errors.Is(err, ErrUnsupportedFormat) {
		t.Errorf("tiny input: %v", err)
	}
	blob := make([]byte, 4*8192)
	if _, err := Open(bytes.NewReader(blob), int64(len(blob))); !errors.Is(err, ErrUnsupportedFormat) {
		t.Errorf("zero blocks: %v", err)
	}
	// Valid version block whose postlist root points at an all-zero block.
	copy(blob, versionBlock(glassMagic, glassFormatVersion, liveRoots, [8]uint64{1, 0, 1, 1, 0, 0, 1, 0}))
	if _, err := Open(bytes.NewReader(blob), int64(len(blob))); !errors.Is(err, ErrCorrupt) {
		t.Errorf("empty root block: %v", err)
	}
}

func TestOpenRejectsBadRoots(t *testing.T) {
	open := func(vb []byte) error {
		blob := make([]byte, 4*8192)
		copy(blob, vb)
		_, err := Open(bytes.NewReader(blob), int64(len(blob)))
		return err
	}
	oneDoc := [8]uint64{1, 0, 1, 1, 0, 0, 1, 0}
	if err := open(versionBlock(glassMagic, glassFormatVersion, fakeRoots, [8]uint64{})); err != nil {
		t.Fatalf("empty database: %v", err)
	}
	mixed := versionBlock(glassMagic, glassFormatVersion, liveRoots, oneDoc)
	mixed[blockSizeOffset(tableDocdata)] = 4096 >> 11
	outside, zero := liveRoots, liveRoots
	outside[tablePostlist] = [3]uint64{9, 0, 0}
	zero[tablePostlist] = [3]uint64{0, 0, 0}
	huge := fakeRoots
	huge[tablePostlist] = [3]uint64{0, 0x80000000, 1}
	cases := []struct {
		name string
		b    []byte
		want error
	}{
		{"mixed block sizes", mixed, ErrUnsupportedFormat},
		{"documents without postlist", versionBlock(glassMagic, glassFormatVersion, fakeRoots, oneDoc), ErrCorrupt},
		{"root past the end", versionBlock(glassMagic, glassFormatVersion, outside, oneDoc), ErrCorrupt},
		{"root block 0", versionBlock(glassMagic, glassFormatVersion, zero, oneDoc), ErrCorrupt},
		{"level overflows int", versionBlock(glassMagic, glassFormatVersion, huge, [8]uint64{}), ErrCorrupt},
	}
	for _, c := range cases {
		if err := open(c.b); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
}
