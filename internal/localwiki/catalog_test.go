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

func TestParseOPDSToleratesBadNumbersInUnrelatedAndUnwantedEntries(t *testing.T) {
	feed := `<feed xmlns="http://www.w3.org/2005/Atom">
<entry><name>wikipedia_nb_all</name><flavour>mini</flavour><articleCount>lots</articleCount>
 <link rel="http://opds-spec.org/acquisition/open-access" href="/zim/wikipedia/wikipedia_nb_all_mini_2026-09.zim.meta4" length="4.5GB"/></entry>
<entry><name>wikipedia_sv_all</name><flavour>nopic</flavour><articleCount>-5</articleCount>
 <link rel="http://opds-spec.org/acquisition/open-access" href="/zim/wikipedia/wikipedia_sv_all_nopic_2026-09.zim.meta4" length="x"/></entry>
<entry><name>wikipedia_nb_all</name><flavour>nopic</flavour><articleCount>7</articleCount>
 <link rel="http://opds-spec.org/acquisition/open-access" href="/zim/wikipedia/wikipedia_nb_all_nopic_2026-08.zim.meta4" length="800"/>
 <link rel="http://opds-spec.org/acquisition/open-access" href="/zim/wikipedia/wikipedia_nb_all_nopic_2026-09.zim.meta4" length="4.5GB"/>
 <link rel="http://opds-spec.org/acquisition/open-access" href="/zim/wikipedia/wikipedia_nb_all_nopic_2026-10.zim.meta4" length="-1"/></entry>
<entry><name>wikipedia_nb_all</name><flavour>nopic</flavour><articleCount>1.5e3</articleCount>
 <link rel="http://opds-spec.org/acquisition/open-access" href="/zim/wikipedia/wikipedia_nb_all_nopic_2026-11.zim.meta4" length="900"/></entry>
<entry><name>wikipedia_nb_all</name><flavour>maxi</flavour><articleCount>-3</articleCount>
 <link rel="http://opds-spec.org/acquisition/open-access" href="/zim/wikipedia/wikipedia_nb_all_maxi_2026-09.zim.meta4" length="2000"/></entry>
<entry><name>wikipedia_nb_all</name><flavour>maxi</flavour><articleCount>9</articleCount>
 <link rel="http://opds-spec.org/acquisition/open-access" href="/zim/wikipedia/wikipedia_nb_all_maxi_2026-07.zim.meta4" length="3000"/></entry>
</feed>`
	got, err := parseOPDS([]byte(feed), "nb", mustURL(t, "https://catalog.example/catalog/v2/entries"))
	if err != nil {
		t.Fatalf("one malformed number made the whole feed fail: %v", err)
	}
	nopic := got[VariantNoPic]
	if nopic.Name != "wikipedia_nb_all_nopic_2026-08" || nopic.Size != 800 || nopic.ArticleCount != 7 {
		t.Fatalf("nopic = %+v, want the only entry and link with valid numbers", nopic)
	}
	maxi := got[VariantMaxi]
	if maxi.Name != "wikipedia_nb_all_maxi_2026-07" || maxi.Size != 3000 || maxi.ArticleCount != 9 {
		t.Fatalf("maxi = %+v, want the entry with a valid count (the newer one has -3 articles)", maxi)
	}
}

func TestParseOPDSKeepsAbsentCountsAsZero(t *testing.T) {
	feed := `<feed xmlns="http://www.w3.org/2005/Atom">
<entry><name>wikipedia_nb_all</name><flavour>nopic</flavour>
 <link rel="http://opds-spec.org/acquisition/open-access" href="/zim/wikipedia/wikipedia_nb_all_nopic_2026-08.zim.meta4"/></entry>
</feed>`
	got, err := parseOPDS([]byte(feed), "nb", mustURL(t, "https://catalog.example/catalog/v2/entries"))
	if err != nil {
		t.Fatal(err)
	}
	if edition := got[VariantNoPic]; edition.Name != "wikipedia_nb_all_nopic_2026-08" || edition.Size != 0 || edition.ArticleCount != 0 {
		t.Fatalf("nopic = %+v", edition)
	}
}

func TestParseOPDSRequiresAnAtomFeed(t *testing.T) {
	base := mustURL(t, "https://catalog.example/catalog/v2/entries")
	for name, body := range map[string]string{
		"captive portal":       `<html><head><title>Login</title></head><body>Sign in to the Wi-Fi</body></html>`,
		"xhtml":                `<html xmlns="http://www.w3.org/1999/xhtml"><body/></html>`,
		"other xml":            `<rss version="2.0"><channel/></rss>`,
		"feed without Atom ns": `<feed><entry><name>wikipedia_nb_all</name></entry></feed>`,
		"feed in another ns":   `<feed xmlns="urn:example:other"/>`,
		"empty":                ``,
	} {
		if got, err := parseOPDS([]byte(body), "nb", base); err == nil {
			t.Fatalf("%s: parseOPDS accepted a document that is not an Atom feed: %+v", name, got)
		}
	}
	got, err := parseOPDS([]byte(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"><id>x</id></feed>`), "nb", base)
	if err != nil || len(got) != 0 {
		t.Fatalf("an Atom feed without entries must be an empty catalog, got %+v, %v", got, err)
	}
}

func TestParseOPDSNewestIgnoresForeignHosts(t *testing.T) {
	feed := `<feed xmlns="http://www.w3.org/2005/Atom">
<entry><name>wikipedia_nb_all</name><flavour>nopic</flavour><articleCount>1</articleCount>
 <link rel="http://opds-spec.org/acquisition/open-access" href="https://lb.download.kiwix.org/zim/wikipedia/wikipedia_nb_all_nopic_2026-05.zim.meta4" length="10"/>
 <link rel="http://opds-spec.org/acquisition/open-access" href="https://evil.example/zim/wikipedia/wikipedia_nb_all_nopic_2026-09.zim.meta4" length="20"/>
 <link rel="http://opds-spec.org/acquisition/open-access" href="//evil.example/zim/wikipedia/wikipedia_nb_all_nopic_2026-08.zim.meta4" length="30"/>
 <link rel="http://opds-spec.org/acquisition/open-access" href="https://kiwix.org.evil.example/zim/wikipedia/wikipedia_nb_all_nopic_2026-07.zim.meta4" length="40"/>
 <link rel="http://opds-spec.org/acquisition/open-access" href="https://evilkiwix.org/zim/wikipedia/wikipedia_nb_all_nopic_2026-06.zim.meta4" length="50"/></entry>
<entry><name>wikipedia_nb_all</name><flavour>maxi</flavour><articleCount>2</articleCount>
 <link rel="http://opds-spec.org/acquisition/open-access" href="//evil.example/zim/wikipedia/wikipedia_nb_all_maxi_2026-09.zim.meta4" length="60"/></entry>
</feed>`
	got, err := parseOPDS([]byte(feed), "nb", mustURL(t, "https://opds.library.kiwix.org/catalog/v2/entries?name=wikipedia_nb_all"))
	if err != nil {
		t.Fatal(err)
	}
	nopic := got[VariantNoPic]
	if nopic.Name != "wikipedia_nb_all_nopic_2026-05" || nopic.Meta4URL != "https://lb.download.kiwix.org/zim/wikipedia/wikipedia_nb_all_nopic_2026-05.zim.meta4" {
		t.Fatalf("a foreign link won the newest edition: %+v", nopic)
	}
	if edition, ok := got[VariantMaxi]; ok {
		t.Fatalf("a protocol-relative foreign link was accepted: %+v", edition)
	}
	// A protocol-relative link to a Kiwix host resolves to https and stays valid.
	own, err := parseOPDS([]byte(`<feed xmlns="http://www.w3.org/2005/Atom"><entry><name>wikipedia_nb_all</name><flavour>maxi</flavour><articleCount>2</articleCount>
 <link rel="http://opds-spec.org/acquisition/open-access" href="//dl.kiwix.org/zim/wikipedia/wikipedia_nb_all_maxi_2026-09.zim.meta4" length="60"/></entry></feed>`),
		"nb", mustURL(t, "https://opds.library.kiwix.org/catalog/v2/entries"))
	if err != nil || own[VariantMaxi].Meta4URL != "https://dl.kiwix.org/zim/wikipedia/wikipedia_nb_all_maxi_2026-09.zim.meta4" {
		t.Fatalf("protocol-relative Kiwix link = %+v, %v", own, err)
	}
}

func TestEditionNameMonthMustBeValid(t *testing.T) {
	for name, want := range map[string]bool{
		"wikipedia_de_all_nopic_2026-01":  true,
		"wikipedia_de_all_nopic_2026-09":  true,
		"wikipedia_de_all_nopic_2026-10":  true,
		"wikipedia_de_all_nopic_2026-12":  true,
		"wikipedia_de_all_nopic_2026-12b": true,
		"wikipedia_de_all_nopic_2026-00":  false,
		"wikipedia_de_all_nopic_2026-13":  false,
		"wikipedia_de_all_nopic_2026-19":  false,
		"wikipedia_de_all_nopic_2026-99":  false,
		"wikipedia_de_all_nopic_2026-1":   false,
	} {
		if _, ok := parseEditionName(name); ok != want {
			t.Fatalf("parseEditionName(%q) ok = %v, want %v", name, ok, want)
		}
	}
	// A bogus month can never win "newest" over a real edition.
	feed := `<feed xmlns="http://www.w3.org/2005/Atom"><entry><name>wikipedia_nb_all</name><flavour>nopic</flavour><articleCount>1</articleCount>
 <link rel="http://opds-spec.org/acquisition/open-access" href="/zim/wikipedia/wikipedia_nb_all_nopic_2026-09.zim.meta4" length="10"/>
 <link rel="http://opds-spec.org/acquisition/open-access" href="/zim/wikipedia/wikipedia_nb_all_nopic_2026-13.zim.meta4" length="20"/>
 <link rel="http://opds-spec.org/acquisition/open-access" href="/zim/wikipedia/wikipedia_nb_all_nopic_2026-00.zim.meta4" length="30"/></entry></feed>`
	got, err := parseOPDS([]byte(feed), "nb", mustURL(t, "https://catalog.example/catalog/v2/entries"))
	if err != nil || got[VariantNoPic].Name != "wikipedia_nb_all_nopic_2026-09" {
		t.Fatalf("nopic = %+v, %v", got[VariantNoPic], err)
	}
}
