package zim

import (
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
