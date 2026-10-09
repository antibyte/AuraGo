package zim

import (
	"encoding/binary"
	"testing"

	"aurago/internal/zim/zimtest"
)

func titlesOf(entries []Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Title
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestArticleListV1(t *testing.T) {
	a := openSample(t)
	if !a.titles.v1 {
		t.Fatal("sample archive should use titleOrdered/v1")
	}
	want := []string{"Berlin", "Berlin (Stadt)", "Bern", "Hamburg", "Hauptseite", "Köln"}
	if a.ArticleCount() != len(want) {
		t.Fatalf("ArticleCount = %d, want %d", a.ArticleCount(), len(want))
	}
	for i, title := range want {
		e, err := a.ArticleAt(i)
		if err != nil || e.Title != title {
			t.Fatalf("ArticleAt(%d) = %q, %v; want %q", i, e.Title, err, title)
		}
	}
	for _, i := range []int{-1, len(want)} {
		_, err := a.ArticleAt(i)
		wantErr(t, err, ErrNotFound)
	}
}

func TestTitlePrefix(t *testing.T) {
	a := openSample(t)
	cases := []struct {
		prefix string
		limit  int
		want   []string
	}{
		{"Ber", 10, []string{"Berlin", "Berlin (Stadt)", "Bern"}},
		{"Ber", 2, []string{"Berlin", "Berlin (Stadt)"}},
		{"Berlin (", 10, []string{"Berlin (Stadt)"}},
		{"H", 10, []string{"Hamburg", "Hauptseite"}},
		{"Kö", 10, []string{"Köln"}},
		{"ber", 10, nil}, // byte-wise and case-sensitive
		{"Zürich", 10, nil},
		{"", 2, []string{"Berlin", "Berlin (Stadt)"}},
		{"Ber", 0, nil},
	}
	for _, tc := range cases {
		got, err := a.TitlePrefix(tc.prefix, tc.limit)
		if err != nil {
			t.Fatalf("TitlePrefix(%q) error = %v", tc.prefix, err)
		}
		if !equalStrings(titlesOf(got), tc.want) {
			t.Fatalf("TitlePrefix(%q, %d) = %q, want %q", tc.prefix, tc.limit, titlesOf(got), tc.want)
		}
	}
}

func TestCompressedTitleListFallsBackToV0(t *testing.T) {
	b := sampleBuilder()
	b.CompressTitleListV1 = true
	b.TitleListV0 = true
	path, _ := b.WriteFile(t)
	a := openPath(t, path)
	if a.titles.v1 {
		t.Fatal("compressed v1 listing must be ignored")
	}
	// v0 lists every C entry (also non-front ones like images), ordered by title.
	got, err := a.TitlePrefix("", 100)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Berlin", "Berlin (Stadt)", "Bern", "Hamburg", "Hauptseite", "Köln", "big.bin", "map.webp"}
	if !equalStrings(titlesOf(got), want) {
		t.Fatalf("v0 titles = %q, want %q", titlesOf(got), want)
	}
}

func TestArchiveWithoutTitleList(t *testing.T) {
	b := zimtest.New()
	b.TitleListV1 = false
	c := b.AddCluster(zimtest.CompressionZstd, false)
	b.AddArticle(c, 'C', "Solo", "Solo", "<p>solo</p>")
	path, _ := b.WriteFile(t)
	a := openPath(t, path)
	if a.ArticleCount() != 0 {
		t.Fatalf("ArticleCount = %d, want 0", a.ArticleCount())
	}
	if got, err := a.TitlePrefix("S", 5); err != nil || len(got) != 0 {
		t.Fatalf("TitlePrefix = %v, %v", got, err)
	}
	_, err := a.ArticleAt(0)
	wantErr(t, err, ErrNotFound)
}

func TestLegacyNamespaceArchive(t *testing.T) {
	b := zimtest.New()
	b.Minor = 0
	b.TitleListV1 = false
	b.TitleListV0 = true
	c := b.AddCluster(zimtest.CompressionZstd, false)
	b.AddArticle(c, 'A', "Zebra", "Zebra", "<p>zebra</p>")
	b.AddArticle(c, 'A', "Apfel", "Apfel", "<p>apfel</p>")
	b.Add(zimtest.Entry{Namespace: '-', Path: "style.css", MimeType: "text/css", Data: []byte("p{}"), Cluster: c})
	b.Add(zimtest.Entry{Namespace: 'I', Path: "logo.png", MimeType: "image/png", Data: []byte("png"), Cluster: c})
	b.AddMetadata(c, "Language", "deu")
	b.SetHeaderMainPage("A/Zebra")
	path, _ := b.WriteFile(t)
	a := openPath(t, path)
	if a.ContentNamespace() != 'A' {
		t.Fatalf("ContentNamespace = %c, want A", a.ContentNamespace())
	}
	if a.ArticleCount() != 2 {
		t.Fatalf("ArticleCount = %d, want 2 (A namespace only)", a.ArticleCount())
	}
	first, err := a.ArticleAt(0)
	if err != nil || first.Path != "Apfel" {
		t.Fatalf("ArticleAt(0) = %+v, %v", first, err)
	}
	if got := readEntry(t, a, mustEntry(t, a, 'A', "Zebra")); got != "<p>zebra</p>" {
		t.Fatalf("A/Zebra = %q", got)
	}
	if e, err := a.MainEntry(); err != nil || e.Path != "Zebra" {
		t.Fatalf("MainEntry = %+v, %v", e, err)
	}
}

func TestTitleReadsAfterCloseReportErrClosed(t *testing.T) {
	path, _ := sampleBuilder().WriteFile(t)
	a, err := Open(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	count := a.ArticleCount()
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if a.ArticleCount() != count {
		t.Fatalf("ArticleCount after Close = %d, want %d", a.ArticleCount(), count)
	}
	_, err = a.ArticleAt(0)
	wantErr(t, err, ErrClosed)
	_, err = a.TitlePrefix("Ber", 5)
	wantErr(t, err, ErrClosed)
}

// listingStart is the file offset of the first position of the v1 listing in a
// sampleBuilder archive: the listing lives alone in the last (uncompressed)
// cluster, behind the info byte and a two-entry offset table.
func listingStart(l *zimtest.Layout) int64 {
	return l.ClusterOffsets[len(l.ClusterOffsets)-1] + 1 + 8
}

func TestCorruptTitleListPositionsAreRejected(t *testing.T) {
	le := binary.LittleEndian
	const pos = 3 // "Hamburg": the first probe of a binary search over six entries
	point := func(d []byte, l *zimtest.Layout, idx uint32) { le.PutUint32(d[listingStart(l)+4*pos:], idx) }
	cases := []struct {
		name   string
		mutate func([]byte, *zimtest.Layout)
	}{
		{"index outside the archive", func(d []byte, l *zimtest.Layout) { point(d, l, uint32(len(l.EntryIndex))) }},
		{"xapian index entry", func(d []byte, l *zimtest.Layout) { point(d, l, l.EntryIndex["X/fulltext/xapian"]) }},
		{"metadata entry", func(d []byte, l *zimtest.Layout) { point(d, l, l.EntryIndex["M/Language"]) }},
		{"redirect outside the content namespace", func(d []byte, l *zimtest.Layout) { point(d, l, l.EntryIndex["W/mainPage"]) }},
		{"deprecated entry in the content namespace", func(d []byte, l *zimtest.Layout) {
			// "Hamburg" becomes a deprecated "deleted" entry (MIME index 0xFFFD).
			le.PutUint16(d[l.DirentOffset["C/Hamburg"]:], mimeDeleted)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, layout, err := sampleBuilder().Build()
			if err != nil {
				t.Fatal(err)
			}
			tc.mutate(data, layout)
			a := openPath(t, zimtest.WriteBytes(t, data))
			_, err = a.ArticleAt(pos)
			wantErr(t, err, ErrCorrupt)
			_, err = a.TitlePrefix("Ber", 5)
			wantErr(t, err, ErrCorrupt)
			// Untouched positions keep working.
			if e, err := a.ArticleAt(0); err != nil || e.Title != "Berlin" {
				t.Fatalf("ArticleAt(0) = %+v, %v", e, err)
			}
		})
	}
}

func TestLegacyTitleListRejectsEntriesOutsideTheContentNamespace(t *testing.T) {
	// Legacy archives list the 'A' block of the v0 pointer list; a damaged
	// pointer into another namespace must not leak out of ArticleAt.
	b := zimtest.New()
	b.Minor = 0
	b.TitleListV1 = false
	b.TitleListV0 = true
	c := b.AddCluster(zimtest.CompressionZstd, false)
	b.AddArticle(c, 'A', "Apfel", "Apfel", "<p>apfel</p>")
	b.Add(zimtest.Entry{Namespace: 'I', Path: "logo.png", MimeType: "image/png", Data: []byte("png"), Cluster: c})
	data, layout, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	// v0 position 0 is the first 'A' entry (the namespace range starts at 0 here).
	binary.LittleEndian.PutUint32(data[layout.TitlePtrPos:], layout.EntryIndex["I/logo.png"])
	a := openPath(t, zimtest.WriteBytes(t, data))
	_, err = a.ArticleAt(0)
	wantErr(t, err, ErrCorrupt)
}

// withListingPositions rewrites the end offset of the v1 listing blob so that
// it holds the given number of 4-byte positions (the bytes are available: the
// directory entries follow the cluster).
func withListingPositions(t *testing.T, b *zimtest.Builder, positions int) string {
	t.Helper()
	data, layout, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	last := layout.ClusterOffsets[len(layout.ClusterOffsets)-1]
	binary.LittleEndian.PutUint32(data[last+1+4:], uint32(8+4*positions))
	return zimtest.WriteBytes(t, data)
}

func TestOversizedV1ListingFallsBackToV0(t *testing.T) {
	b := sampleBuilder()
	b.TitleListV0 = true
	entries := len(mustBuild(t, b).EntryIndex)

	// Exactly one position per entry is still a plausible listing.
	a := openPath(t, withListingPositions(t, b, entries))
	if !a.titles.v1 || a.ArticleCount() != entries {
		t.Fatalf("listing with %d positions: v1 = %v, ArticleCount = %d", entries, a.titles.v1, a.ArticleCount())
	}

	// One more than there are entries cannot be a listing: use the v0 list.
	a = openPath(t, withListingPositions(t, b, entries+1))
	if a.titles.v1 {
		t.Fatal("oversized v1 listing must be ignored")
	}
	got, err := a.TitlePrefix("", 100)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Berlin", "Berlin (Stadt)", "Bern", "Hamburg", "Hauptseite", "Köln", "big.bin", "map.webp"}
	if !equalStrings(titlesOf(got), want) {
		t.Fatalf("v0 titles = %q, want %q", titlesOf(got), want)
	}
}

func TestOversizedV1ListingWithoutV0IsCorrupt(t *testing.T) {
	b := sampleBuilder()
	entries := len(mustBuild(t, b).EntryIndex)
	_, err := Open(withListingPositions(t, b, entries+1), Options{})
	wantErr(t, err, ErrCorrupt)
}

func mustBuild(t *testing.T, b *zimtest.Builder) *zimtest.Layout {
	t.Helper()
	_, layout, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	return layout
}

func TestV0TitleListAllowsDeprecatedEntries(t *testing.T) {
	// Old archives keep deprecated ("deleted"/"link target") entries inside the
	// content namespace block that the v0 list covers; they are listed, not corrupt.
	b := sampleBuilder()
	b.TitleListV1 = false
	b.TitleListV0 = true
	data, layout, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	binary.LittleEndian.PutUint16(data[layout.DirentOffset["C/Hamburg"]:], mimeDeleted)
	a := openPath(t, zimtest.WriteBytes(t, data))
	if a.titles.v1 {
		t.Fatal("test needs the v0 list")
	}
	got, err := a.TitlePrefix("", 100)
	if err != nil {
		t.Fatalf("TitlePrefix() error = %v", err)
	}
	if len(got) != a.ArticleCount() {
		t.Fatalf("TitlePrefix returned %d entries, want %d", len(got), a.ArticleCount())
	}
	deprecated := 0
	for _, e := range got {
		if e.kind == kindDeprecated {
			deprecated++
		}
	}
	if deprecated != 1 {
		t.Fatalf("listed %d deprecated entries, want 1", deprecated)
	}
	if e, err := a.ArticleAt(3); err != nil || e.kind != kindDeprecated {
		t.Fatalf("ArticleAt(3) = %+v, %v; want the deprecated entry", e, err)
	}

	// The namespace check still applies to v0: a pointer into X/ is corrupt.
	binary.LittleEndian.PutUint32(data[layout.TitlePtrPos+4*3:], layout.EntryIndex["X/fulltext/xapian"])
	a = openPath(t, zimtest.WriteBytes(t, data))
	_, err = a.ArticleAt(3)
	wantErr(t, err, ErrCorrupt)
}
