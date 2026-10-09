package zimtest

import (
	"bytes"
	"crypto/md5" //nolint:gosec // ZIM checksum format
	"encoding/binary"
	"strings"
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

	// An empty MIME type would sort first in the MIME list and end it early;
	// NUL bytes would end the MIME type, path or title early.
	for _, tc := range []struct {
		name   string
		e      Entry
		target bool // add the article C/A that a redirect entry points to
		want   string
	}{
		{name: "empty MIME type", e: Entry{Namespace: 'C', Path: "A", MimeType: "", Data: []byte("x")}, want: "empty MIME type"},
		{name: "NUL in MIME", e: Entry{Namespace: 'C', Path: "A", MimeType: "text/\x00html", Data: []byte("x")}, want: "NUL byte in its MIME type"},
		{name: "NUL in path", e: Entry{Namespace: 'C', Path: "A\x00B", MimeType: "text/html", Data: []byte("x")}, want: "NUL byte in its path"},
		{name: "NUL in title", e: Entry{Namespace: 'C', Path: "A", Title: "A\x00B", MimeType: "text/html", Data: []byte("x")}, want: "NUL byte in its title"},
		// The redirect target exists, so only the NUL check can fail the build.
		{name: "NUL in redirect path", e: Entry{Namespace: 'C', Path: "R\x00", Redirect: "C/A"}, target: true, want: "NUL byte in its path"},
	} {
		b = New()
		tc.e.Cluster = b.AddCluster(CompressionNone, false)
		if tc.target {
			b.AddArticle(tc.e.Cluster, 'C', "A", "A", "<p>a</p>")
		}
		b.Add(tc.e)
		_, _, err := b.Build()
		if err == nil {
			t.Fatalf("Build accepted an entry with %s", tc.name)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("Build error for %s = %v, want it to mention %q", tc.name, err, tc.want)
		}
	}

	// Redirects have no MIME type, and the raw-redirect hook must keep working.
	b = New()
	c = b.AddCluster(CompressionNone, false)
	b.AddArticle(c, 'C', "A", "A", "<p>a</p>")
	bad := uint32(9999)
	b.Add(Entry{Namespace: 'C', Path: "Bad", RawRedirectIndex: &bad})
	b.AddRedirect('C', "R", "", "C/A")
	if _, _, err := b.Build(); err != nil {
		t.Fatalf("Build rejected redirects without a MIME type: %v", err)
	}
}

func TestBuildWritesTitleListV0InTitleOrder(t *testing.T) {
	b := New()
	b.TitleListV0 = true
	c := b.AddCluster(CompressionNone, false)
	b.AddArticle(c, 'C', "A", "Zeta", "<p>a</p>")
	b.AddArticle(c, 'C', "B", "Alpha", "<p>b</p>")
	b.AddRedirect('W', "mainPage", "", "C/A")
	data, layout, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	le := binary.LittleEndian
	if layout.TitlePtrPos < 0 || le.Uint64(data[40:]) != uint64(layout.TitlePtrPos) {
		t.Fatalf("header titlePtrPos = %d, layout %d", le.Uint64(data[40:]), layout.TitlePtrPos)
	}
	// Sorted by namespace, then title: C/B (Alpha), C/A (Zeta), W/mainPage and
	// the generated X/listing/titleOrdered/v1 entry.
	want := []uint32{layout.EntryIndex["C/B"], layout.EntryIndex["C/A"], layout.EntryIndex["W/mainPage"], layout.EntryIndex["X/listing/titleOrdered/v1"]}
	for i, w := range want {
		if got := le.Uint32(data[layout.TitlePtrPos+int64(4*i):]); got != w {
			t.Fatalf("title pointer %d = %d, want %d (all wanted %v)", i, got, w, want)
		}
	}
	if layout.ClusterPtrPos != layout.TitlePtrPos+int64(4*len(want)) {
		t.Fatalf("cluster pointers at %d, want right after the title list (%d)", layout.ClusterPtrPos, layout.TitlePtrPos+int64(4*len(want)))
	}
}

func TestClusterBodyOffsets(t *testing.T) {
	body := ClusterBody([][]byte{[]byte("ab"), []byte("c")}, false)
	want := []byte{12, 0, 0, 0, 14, 0, 0, 0, 15, 0, 0, 0, 'a', 'b', 'c'}
	if !bytes.Equal(body, want) {
		t.Fatalf("ClusterBody = % x, want % x", body, want)
	}

	// Extended clusters use 8-byte offsets: three offsets = 24 bytes of table.
	body = ClusterBody([][]byte{[]byte("ab"), []byte("c")}, true)
	want = []byte{
		24, 0, 0, 0, 0, 0, 0, 0,
		26, 0, 0, 0, 0, 0, 0, 0,
		27, 0, 0, 0, 0, 0, 0, 0,
		'a', 'b', 'c',
	}
	if !bytes.Equal(body, want) {
		t.Fatalf("extended ClusterBody = % x, want % x", body, want)
	}
}
