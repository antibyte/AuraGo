package zim

import (
	"encoding/binary"
	"testing"
)

func TestParseHeaderReadsBuiltArchive(t *testing.T) {
	data, layout, err := sampleBuilder().Build()
	if err != nil {
		t.Fatal(err)
	}
	h, err := parseHeader(data[:headerSize], int64(len(data)))
	if err != nil {
		t.Fatalf("parseHeader() error = %v", err)
	}
	if h.major != 6 || h.minor != 2 {
		t.Fatalf("version = %d.%d, want 6.2", h.major, h.minor)
	}
	if string(h.uuid[:]) != "aurago-zimtest-1" {
		t.Fatalf("uuid = %q", h.uuid[:])
	}
	if int(h.entryCount) != len(layout.EntryIndex) {
		t.Fatalf("entryCount = %d, want %d", h.entryCount, len(layout.EntryIndex))
	}
	if h.pathPtrPos != uint64(layout.PathPtrPos) || h.clusterPtrPos != uint64(layout.ClusterPtrPos) {
		t.Fatalf("pointer positions = %d/%d, want %d/%d", h.pathPtrPos, h.clusterPtrPos, layout.PathPtrPos, layout.ClusterPtrPos)
	}
	if !h.hasChecksum() || h.hasTitleListV0() {
		t.Fatalf("hasChecksum=%v hasTitleListV0=%v, want true/false", h.hasChecksum(), h.hasTitleListV0())
	}
	if got := h.dataEnd(int64(len(data))); got != layout.ChecksumPos {
		t.Fatalf("dataEnd = %d, want %d", got, layout.ChecksumPos)
	}
}

func TestParseHeaderRejectsInvalidHeaders(t *testing.T) {
	data, _, err := sampleBuilder().Build()
	if err != nil {
		t.Fatal(err)
	}
	le := binary.LittleEndian
	cases := []struct {
		name   string
		mutate func(h []byte)
		size   int64
		want   error
	}{
		{"bad magic", func(h []byte) { le.PutUint32(h[0:], 0x12345678) }, 0, ErrNotZIM},
		{"major 7", func(h []byte) { le.PutUint16(h[4:], 7) }, 0, ErrUnsupported},
		{"mime list position", func(h []byte) { le.PutUint64(h[56:], 100) }, 0, ErrCorrupt},
		{"truncated file", func([]byte) {}, int64(len(data)) - 1, ErrCorrupt},
		{"entries without clusters", func(h []byte) { le.PutUint32(h[28:], 0) }, 0, ErrCorrupt},
		{"more clusters than entries", func(h []byte) { le.PutUint32(h[28:], le.Uint32(h[24:])+1) }, 0, ErrCorrupt},
		{"path pointers beyond end", func(h []byte) { le.PutUint64(h[32:], uint64(len(data))) }, 0, ErrCorrupt},
		{"cluster pointers beyond end", func(h []byte) { le.PutUint64(h[48:], ^uint64(0)-4) }, 0, ErrCorrupt},
		{"title pointers inside header", func(h []byte) { le.PutUint64(h[40:], 8) }, 0, ErrCorrupt},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := append([]byte(nil), data[:headerSize]...)
			tc.mutate(h)
			size := tc.size
			if size == 0 {
				size = int64(len(data))
			}
			_, err := parseHeader(h, size)
			wantErr(t, err, tc.want)
		})
	}
}

func TestParseHeaderRejectsShortInput(t *testing.T) {
	_, err := parseHeader(make([]byte, 10), 10)
	wantErr(t, err, ErrNotZIM)
}

func TestTableFitsHandlesOverflow(t *testing.T) {
	if tableFits(^uint64(0)-8, 4, 8, 80, ^uint64(0)) {
		t.Fatal("tableFits accepted an overflowing table")
	}
	if !tableFits(80, 2, 8, 80, 96) {
		t.Fatal("tableFits rejected an exact fit")
	}
	if tableFits(80, 3, 8, 80, 96) {
		t.Fatal("tableFits accepted a table past the end")
	}
}

func TestParseMimeList(t *testing.T) {
	got, err := parseMimeList([]byte("text/html\x00image/png\x00\x00garbage"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "text/html" || got[1] != "image/png" {
		t.Fatalf("parseMimeList() = %q", got)
	}
	if _, err := parseMimeList([]byte("text/html\x00image/png")); err == nil {
		t.Fatal("parseMimeList accepted an unterminated list")
	}
}
