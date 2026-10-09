package xapian

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

// versionBlock builds block 0 of a single-file glass database. roots holds
// (root, level, fake) per table; all tables use 8 KiB blocks.
func versionBlock(magic string, format uint16, roots [tableCount][3]int, stats [8]uint64) []byte {
	b := []byte(magic)
	b = binary.BigEndian.AppendUint16(b, format)
	b = append(b, bytes.Repeat([]byte{0xab}, 16)...) // UUID
	b = appendUint(b, 1)                             // revision
	for _, r := range roots {
		flags := uint64(r[1]) << 2
		if r[2] != 0 {
			flags |= 1
		}
		b = appendUint(b, uint64(r[0]))
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

var liveRoots = [tableCount][3]int{{1, 0, 0}, {2, 0, 0}, {0, 0, 1}, {0, 0, 1}, {0, 0, 1}, {0, 0, 1}}

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
}

func TestParseVersionRejects(t *testing.T) {
	stats := [8]uint64{1, 0, 1, 1, 0, 0, 1, 0}
	deep := liveRoots
	deep[tablePostlist] = [3]int{1, 10, 0}
	badBS := versionBlock(glassMagic, glassFormatVersion, liveRoots, stats)
	cases := []struct {
		name string
		b    []byte
		want error
	}{
		{"honey", versionBlock("\x0f\x0dXapian Honey", glassFormatVersion, liveRoots, stats), ErrUnsupportedFormat},
		{"not xapian", bytes.Repeat([]byte("x"), 64), ErrUnsupportedFormat},
		{"future glass", versionBlock(glassMagic, glassFormatVersion+1, liveRoots, stats), ErrUnsupportedFormat},
		{"level", versionBlock(glassMagic, glassFormatVersion, deep, stats), ErrCorrupt},
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
