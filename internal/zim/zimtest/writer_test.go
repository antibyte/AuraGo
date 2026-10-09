package zimtest

import (
	"bytes"
	"crypto/md5" //nolint:gosec // ZIM checksum format
	"encoding/binary"
	"testing"
)

func TestBuildWritesHeaderLayoutAndChecksum(t *testing.T) {
	b := New()
	z := b.AddCluster(CompressionZstd, false)
	u := b.AddCluster(CompressionNone, true)
	b.AddArticle(z, 'C', "Zebra", "Zebra", "<p>z</p>")
	b.AddArticle(z, 'C', "Apfel", "Apfel", "<p>a</p>")
	b.Add(Entry{Namespace: 'X', Path: "fulltext/xapian", MimeType: "application/octet-stream+xapian", Data: []byte("idx"), Cluster: u})
	b.AddRedirect('W', "mainPage", "", "C/Zebra")
	data, layout, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	le := binary.LittleEndian
	if le.Uint32(data[0:]) != 0x044D495A || le.Uint16(data[4:]) != 6 || le.Uint16(data[6:]) != 2 {
		t.Fatalf("bad magic/version: % x", data[:8])
	}
	// Apfel, Zebra, W/mainPage, X/fulltext/xapian, X/listing/titleOrdered/v1
	if got := le.Uint32(data[24:]); got != 5 || len(layout.EntryIndex) != 5 {
		t.Fatalf("entry count = %d (layout %d), want 5", got, len(layout.EntryIndex))
	}
	if layout.EntryIndex["C/Apfel"] != 0 || layout.EntryIndex["C/Zebra"] != 1 || layout.EntryIndex["W/mainPage"] != 2 {
		t.Fatalf("entries not in namespace/path order: %v", layout.EntryIndex)
	}
	if got := le.Uint32(data[64:]); got != layout.EntryIndex["W/mainPage"] {
		t.Fatalf("header mainPage = %d", got)
	}
	if got := le.Uint32(data[28:]); got != 3 {
		t.Fatalf("cluster count = %d, want 3 (two added + title listing)", got)
	}
	if data[layout.ClusterOffsets[0]] != 5 || data[layout.ClusterOffsets[1]] != 0x11 || data[layout.ClusterOffsets[2]] != 1 {
		t.Fatalf("cluster info bytes = %#x %#x %#x", data[layout.ClusterOffsets[0]], data[layout.ClusterOffsets[1]], data[layout.ClusterOffsets[2]])
	}
	if le.Uint64(data[72:]) != uint64(layout.ChecksumPos) || int64(len(data)) != layout.ChecksumPos+16 {
		t.Fatalf("checksum position %d, file size %d", le.Uint64(data[72:]), len(data))
	}
	sum := md5.Sum(data[:layout.ChecksumPos]) //nolint:gosec // ZIM checksum format
	if !bytes.Equal(sum[:], data[layout.ChecksumPos:]) {
		t.Fatal("stored MD5 does not match")
	}
	// The v1 listing (uncompressed, last cluster) lists front articles by title.
	listing := data[layout.ClusterOffsets[2]+1:]
	first := le.Uint32(listing[0:])
	got := []uint32{le.Uint32(listing[first:]), le.Uint32(listing[first+4:])}
	if got[0] != layout.EntryIndex["C/Apfel"] || got[1] != layout.EntryIndex["C/Zebra"] {
		t.Fatalf("title listing = %v", got)
	}
}

func TestBuildRejectsBrokenInput(t *testing.T) {
	b := New()
	b.AddRedirect('C', "A", "", "C/Missing")
	if _, _, err := b.Build(); err == nil {
		t.Fatal("Build accepted a redirect to a missing entry")
	}
	b = New()
	c := b.AddCluster(CompressionNone, false)
	b.AddMetadata(c, "Title", "x")
	b.AddMetadata(c, "Title", "y")
	if _, _, err := b.Build(); err == nil {
		t.Fatal("Build accepted duplicate paths")
	}
}

func TestClusterBodyOffsets(t *testing.T) {
	body := ClusterBody([][]byte{[]byte("ab"), []byte("c")}, false)
	want := []byte{12, 0, 0, 0, 14, 0, 0, 0, 15, 0, 0, 0, 'a', 'b', 'c'}
	if !bytes.Equal(body, want) {
		t.Fatalf("ClusterBody = % x, want % x", body, want)
	}
}
