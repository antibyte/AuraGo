package localwiki

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fixture is the real .meta4 of wikipedia_de_all_nopic_2026-10 (fetched
// 2026-10-08 via lb.download.kiwix.org), trimmed to three piece hashes.
func TestParseMeta4RealGermanEdition(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "wikipedia_de_all_nopic_2026-10.zim.meta4"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := parseMeta4(data, "wikipedia_de_all_nopic_2026-10.zim")
	if err != nil {
		t.Fatalf("parseMeta4: %v", err)
	}
	if got.FileName != "wikipedia_de_all_nopic_2026-10.zim" || got.Size != 18585337585 ||
		got.SHA256 != "59af34df2f21e1824d96a534cfef0a7776b589ee215ad2a7f7bb3b6328ca30ba" {
		t.Fatalf("parsed = %+v", got)
	}
	if len(got.URLs) != 7 || got.URLs[0] != "https://ftp.fau.de/kiwix/zim/wikipedia/wikipedia_de_all_nopic_2026-10.zim" ||
		got.URLs[6] != "https://dumps.wikimedia.org/kiwix/zim/wikipedia/wikipedia_de_all_nopic_2026-10.zim" {
		t.Fatalf("mirrors = %v", got.URLs)
	}
}

func TestParseMeta4FiltersAndRejects(t *testing.T) {
	const sha = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	doc := func(name, size, hash, urls string) []byte {
		return []byte(`<metalink xmlns="urn:ietf:params:xml:ns:metalink"><file name="` + name + `"><size>` + size +
			`</size>` + hash + urls + `</file></metalink>`)
	}
	urls := `<url priority="3">https://c.example/f.zim</url><url priority="1">http://insecure.example/f.zim</url>` +
		`<url priority="2">https://b.example/f.zim</url><url>https://last.example/f.zim</url>`
	got, err := parseMeta4(doc("wikipedia_en_all_maxi_2026-08.zim", "42", `<hash type="sha-256">`+strings.ToUpper(sha)+`</hash>`, urls),
		"wikipedia_en_all_maxi_2026-08.zim")
	if err != nil {
		t.Fatalf("parseMeta4: %v", err)
	}
	if got.SHA256 != sha || strings.Join(got.URLs, ",") != "https://b.example/f.zim,https://c.example/f.zim,https://last.example/f.zim" {
		t.Fatalf("parsed = %+v", got)
	}
	for name, data := range map[string][]byte{
		"no sha-256":     doc("wikipedia_en_all_maxi_2026-08.zim", "42", `<hash type="md5">00</hash>`, urls),
		"short sha-256":  doc("wikipedia_en_all_maxi_2026-08.zim", "42", `<hash type="sha-256">abcd</hash>`, urls),
		"no size":        doc("wikipedia_en_all_maxi_2026-08.zim", "0", `<hash type="sha-256">`+sha+`</hash>`, urls),
		"other file":     doc("wikipedia_en_all_maxi_2026-07.zim", "42", `<hash type="sha-256">`+sha+`</hash>`, urls),
		"traversal name": doc("../wikipedia_en_all_maxi_2026-08.zim", "42", `<hash type="sha-256">`+sha+`</hash>`, urls),
		"not a metalink": []byte("<html"),
	} {
		if _, err := parseMeta4(data, "wikipedia_en_all_maxi_2026-08.zim"); err == nil {
			t.Fatalf("%s: parseMeta4 accepted", name)
		}
	}
}
