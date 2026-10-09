package localwiki

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return parsed
}

// The fixture is the real OPDS answer for wikipedia_de_all from 2026-10-08,
// trimmed of thumbnail links.
func TestParseOPDSRealGermanCatalog(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "opds_wikipedia_de_all.xml"))
	if err != nil {
		t.Fatal(err)
	}
	base := mustURL(t, "https://opds.library.kiwix.org/catalog/v2/entries?name=wikipedia_de_all&count=-1")
	got, err := parseOPDS(data, "de", base)
	if err != nil {
		t.Fatalf("parseOPDS: %v", err)
	}
	want := map[Variant]CatalogEdition{
		VariantNoPic: {
			Name: "wikipedia_de_all_nopic_2026-10", Date: "2026-10", Size: 18585337856, ArticleCount: 5153780,
			Meta4URL: "https://lb.download.kiwix.org/zim/wikipedia/wikipedia_de_all_nopic_2026-10.zim.meta4",
		},
		VariantMaxi: {
			Name: "wikipedia_de_all_maxi_2026-01", Date: "2026-01", Size: 52392226816, ArticleCount: 5041970,
			Meta4URL: "https://lb.download.kiwix.org/zim/wikipedia/wikipedia_de_all_maxi_2026-01.zim.meta4",
		},
	}
	if len(got) != len(want) {
		t.Fatalf("parseOPDS returned %d variants (mini must be ignored): %+v", len(got), got)
	}
	for variant, edition := range want {
		if got[variant] != edition {
			t.Fatalf("%s = %+v, want %+v", variant, got[variant], edition)
		}
	}
}

func TestParseOPDSPicksNewestAndRejectsForeignOrInsecureLinks(t *testing.T) {
	feed := `<feed xmlns="http://www.w3.org/2005/Atom">
<entry><name>wikipedia_nb_all</name><flavour>nopic</flavour><articleCount>1</articleCount>
 <link rel="http://opds-spec.org/acquisition/open-access" href="/zim/wikipedia/wikipedia_nb_all_nopic_2026-07.zim.meta4" length="10"/></entry>
<entry><name>wikipedia_nb_all</name><flavour>nopic</flavour><articleCount>2</articleCount>
 <link rel="http://opds-spec.org/acquisition/open-access" href="/zim/wikipedia/wikipedia_nb_all_nopic_2026-07b.zim.meta4" length="20"/></entry>
<entry><name>wikipedia_nb_all</name><flavour>maxi</flavour><articleCount>3</articleCount>
 <link rel="http://opds-spec.org/acquisition/open-access" href="http://insecure.example/wikipedia_nb_all_maxi_2026-09.zim.meta4" length="30"/></entry>
<entry><name>wikipedia_nn_all</name><flavour>maxi</flavour><articleCount>4</articleCount>
 <link rel="http://opds-spec.org/acquisition/open-access" href="/zim/wikipedia/wikipedia_nn_all_maxi_2026-09.zim.meta4" length="40"/></entry>
</feed>`
	got, err := parseOPDS([]byte(feed), "nb", mustURL(t, "https://catalog.example/catalog/v2/entries"))
	if err != nil {
		t.Fatalf("parseOPDS: %v", err)
	}
	nopic := got[VariantNoPic]
	if nopic.Name != "wikipedia_nb_all_nopic_2026-07b" || nopic.Size != 20 || nopic.ArticleCount != 2 ||
		nopic.Meta4URL != "https://catalog.example/zim/wikipedia/wikipedia_nb_all_nopic_2026-07b.zim.meta4" {
		t.Fatalf("newest nopic = %+v", nopic)
	}
	if _, ok := got[VariantMaxi]; ok {
		t.Fatalf("an http:// or foreign-language maxi link was accepted: %+v", got[VariantMaxi])
	}
	if _, err := parseOPDS([]byte("<feed"), "nb", mustURL(t, "https://catalog.example/")); err == nil {
		t.Fatal("broken XML was accepted")
	}
}

func TestEditionNames(t *testing.T) {
	parsed, ok := parseEditionName("wikipedia_zh_all_mini_2026-07b")
	if ok {
		t.Fatalf("mini must not parse: %+v", parsed)
	}
	parsed, ok = parseEditionName("wikipedia_de_all_nopic_2026-10")
	if !ok || parsed.Kiwix != "de" || parsed.Variant != VariantNoPic || parsed.Date != "2026-10" || parsed.Suffix != "" {
		t.Fatalf("parsed = %+v, %v", parsed, ok)
	}
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"wikipedia_de_all_nopic_2026-10", "wikipedia_de_all_nopic_2026-09", true},
		{"wikipedia_de_all_nopic_2026-07b", "wikipedia_de_all_nopic_2026-07", true},
		{"wikipedia_de_all_nopic_2026-07", "wikipedia_de_all_nopic_2026-07b", false},
		{"wikipedia_de_all_nopic_2026-09", "wikipedia_de_all_nopic_2026-09", false},
		{"garbage", "wikipedia_de_all_nopic_2026-09", false},
	} {
		if got := editionNewer(tc.a, tc.b); got != tc.want {
			t.Fatalf("editionNewer(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}
