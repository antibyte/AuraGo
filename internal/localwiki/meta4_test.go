package localwiki

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const (
	testMeta4File = "wikipedia_en_all_maxi_2026-08.zim"
	testMeta4SHA  = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
)

// meta4Doc builds a one-file metalink; hash and urls are raw XML.
func meta4Doc(name, size, hash, urls string) []byte {
	return []byte(`<metalink xmlns="urn:ietf:params:xml:ns:metalink"><file name="` + name + `"><size>` + size +
		`</size>` + hash + urls + `</file></metalink>`)
}

func meta4SHA256(value string) string { return `<hash type="sha-256">` + value + `</hash>` }

func meta4URL(host string, priority int) string {
	return fmt.Sprintf(`<url priority="%d">https://%s/zim/%s</url>`, priority, host, testMeta4File)
}

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
	urls := `<url priority="3">https://c.example/` + testMeta4File + `</url><url priority="1">http://insecure.example/` + testMeta4File + `</url>` +
		`<url priority="2">https://b.example/` + testMeta4File + `</url><url>https://last.example/` + testMeta4File + `</url>`
	got, err := parseMeta4(meta4Doc(testMeta4File, "42", meta4SHA256(strings.ToUpper(testMeta4SHA)), urls), testMeta4File)
	if err != nil {
		t.Fatalf("parseMeta4: %v", err)
	}
	want := "https://b.example/" + testMeta4File + ",https://c.example/" + testMeta4File + ",https://last.example/" + testMeta4File
	if got.SHA256 != testMeta4SHA || strings.Join(got.URLs, ",") != want {
		t.Fatalf("parsed = %+v", got)
	}
	for name, data := range map[string][]byte{
		"no sha-256":     meta4Doc(testMeta4File, "42", `<hash type="md5">00</hash>`, urls),
		"short sha-256":  meta4Doc(testMeta4File, "42", meta4SHA256("abcd"), urls),
		"no size":        meta4Doc(testMeta4File, "0", meta4SHA256(testMeta4SHA), urls),
		"negative size":  meta4Doc(testMeta4File, "-5", meta4SHA256(testMeta4SHA), urls),
		"other file":     meta4Doc("wikipedia_en_all_maxi_2026-07.zim", "42", meta4SHA256(testMeta4SHA), urls),
		"traversal name": meta4Doc("../"+testMeta4File, "42", meta4SHA256(testMeta4SHA), urls),
		"not a metalink": []byte("<html"),
	} {
		if _, err := parseMeta4(data, testMeta4File); err == nil {
			t.Fatalf("%s: parseMeta4 accepted", name)
		}
	}
}

// A hostile metalink must not be able to claim a size that wraps the disk
// math: everything above maxEditionBytes (or beyond int64) is refused.
func TestParseMeta4CapsTheClaimedSize(t *testing.T) {
	urls := meta4URL("a.example", 1)
	for _, size := range []string{"9223372036854775807", "9223372036854775808", "99999999999999999999", "1099511627777"} {
		if _, err := parseMeta4(meta4Doc(testMeta4File, size, meta4SHA256(testMeta4SHA), urls), testMeta4File); err == nil {
			t.Errorf("size %s accepted", size)
		}
	}
	got, err := parseMeta4(meta4Doc(testMeta4File, "1099511627776", meta4SHA256(testMeta4SHA), urls), testMeta4File)
	if err != nil || got.Size != maxEditionBytes {
		t.Fatalf("a size of exactly maxEditionBytes = %+v, %v", got, err)
	}
	if _, err := parseMeta4(meta4Doc(testMeta4File, " 127420000000 ", meta4SHA256(testMeta4SHA), urls), testMeta4File); err != nil {
		t.Fatalf("the largest real edition (127 GB) was rejected: %v", err)
	}
}

func TestParseMeta4PriorityAndHashLeniency(t *testing.T) {
	mirror := func(priority, host string) string {
		return `<url ` + priority + `>https://` + host + `/zim/` + testMeta4File + `</url>`
	}
	// One unusable priority only demotes its mirror; 4294967296 does not fit a
	// 32-bit int but is a valid (very low) priority.
	urls := mirror(`priority="abc"`, "abc.example") + mirror(`priority="4294967296"`, "big.example") +
		mirror(`priority="-1"`, "negative.example") + mirror(``, "none.example") +
		mirror(`priority="2"`, "two.example") + mirror(`priority="1"`, "one.example")
	got, err := parseMeta4(meta4Doc(testMeta4File, "42", meta4SHA256(testMeta4SHA), urls), testMeta4File)
	if err != nil {
		t.Fatalf("parseMeta4: %v", err)
	}
	var hosts []string
	for _, raw := range got.URLs {
		hosts = append(hosts, strings.Split(strings.TrimPrefix(raw, "https://"), "/")[0])
	}
	if want := []string{"one.example", "two.example", "abc.example", "negative.example", "none.example", "big.example"}; !reflect.DeepEqual(hosts, want) {
		t.Fatalf("mirror order = %v, want %v", hosts, want)
	}

	// Equal SHA-256 values (any case) and a malformed one next to a good one are fine ...
	hashes := meta4SHA256(testMeta4SHA) + meta4SHA256(strings.ToUpper(testMeta4SHA)) + meta4SHA256("zz")
	if got, err := parseMeta4(meta4Doc(testMeta4File, "42", hashes, urls), testMeta4File); err != nil || got.SHA256 != testMeta4SHA {
		t.Fatalf("duplicate equal hashes = %+v, %v", got, err)
	}
	// ... two different ones are not.
	other := strings.Repeat("b", 64)
	if _, err := parseMeta4(meta4Doc(testMeta4File, "42", meta4SHA256(testMeta4SHA)+meta4SHA256(other), urls), testMeta4File); err == nil {
		t.Fatal("conflicting SHA-256 values were accepted")
	}
}

func TestParseMeta4PicksTheRequestedFileOfSeveral(t *testing.T) {
	file := func(name, size, sha string) string {
		return `<file name="` + name + `"><size>` + size + `</size>` + meta4SHA256(sha) + `<url priority="1">https://m.example/zim/` + name + `</url></file>`
	}
	shaB := strings.Repeat("b", 64)
	doc := `<metalink xmlns="urn:ietf:params:xml:ns:metalink">` + file("wikipedia_en_all_maxi_2026-07.zim", "7", shaB) +
		file(testMeta4File, "8", testMeta4SHA) + `</metalink>`
	got, err := parseMeta4([]byte(doc), testMeta4File)
	if err != nil || got.Size != 8 || got.SHA256 != testMeta4SHA || len(got.URLs) != 1 || !strings.HasSuffix(got.URLs[0], "/"+testMeta4File) {
		t.Fatalf("parsed = %+v, %v", got, err)
	}
	twice := `<metalink xmlns="urn:ietf:params:xml:ns:metalink">` + file(testMeta4File, "8", testMeta4SHA) + file(testMeta4File, "9", shaB) + `</metalink>`
	if _, err := parseMeta4([]byte(twice), testMeta4File); err == nil {
		t.Fatal("a file listed twice was accepted")
	}
}

func TestParseMeta4MirrorRules(t *testing.T) {
	good := "https://good.example/zim/" + testMeta4File
	for name, bad := range map[string]string{
		"http":                "http://bad.example/zim/" + testMeta4File,
		"userinfo":            "https://user:pass@bad.example/zim/" + testMeta4File,
		"no host":             "https:///zim/" + testMeta4File,
		"relative":            "/zim/" + testMeta4File,
		"other file name":     "https://bad.example/zim/wikipedia_en_all_maxi_2026-07.zim",
		"directory":           "https://bad.example/zim/" + testMeta4File + "/",
		"suffix only":         "https://bad.example/zim/x" + testMeta4File,
		"ftp":                 "ftp://bad.example/zim/" + testMeta4File,
		"localhost":           "https://localhost/zim/" + testMeta4File,
		"loopback v4":         "https://127.0.0.1:8443/zim/" + testMeta4File,
		"loopback v6":         "https://[::1]/zim/" + testMeta4File,
		"private 10":          "https://10.1.2.3/zim/" + testMeta4File,
		"private 192.168":     "https://192.168.0.7/zim/" + testMeta4File,
		"link-local metadata": "https://169.254.169.254/zim/" + testMeta4File,
		"link-local v6":       "https://[fe80::1]/zim/" + testMeta4File,
		"mapped loopback":     "https://[::ffff:127.0.0.1]/zim/" + testMeta4File,
		"decimal loopback":    "https://2130706433/zim/" + testMeta4File,
	} {
		doc := meta4Doc(testMeta4File, "42", meta4SHA256(testMeta4SHA),
			`<url priority="1">`+bad+`</url><url priority="2">`+good+`</url>`)
		got, err := parseMeta4(doc, testMeta4File)
		if err != nil || !reflect.DeepEqual(got.URLs, []string{good}) {
			t.Errorf("%s: mirrors = %v, %v (want only the public one)", name, got.URLs, err)
		}
	}

	// Duplicates collapse onto their best priority and at most maxMirrors remain.
	var b strings.Builder
	b.WriteString(`<url priority="1">` + good + `</url><url priority="2">` + good + `</url>`)
	for i := 0; i < 30; i++ {
		b.WriteString(meta4URL(fmt.Sprintf("m%02d.example", i), 10+i))
	}
	got, err := parseMeta4(meta4Doc(testMeta4File, "42", meta4SHA256(testMeta4SHA), b.String()), testMeta4File)
	if err != nil || len(got.URLs) != maxMirrors || got.URLs[0] != good || got.URLs[1] != "https://m00.example/zim/"+testMeta4File {
		t.Fatalf("dedupe/cap: %d mirrors, first %v, err %v", len(got.URLs), got.URLs[:2], err)
	}

	// The configured (catalog) host may be local: tests run a fake Kiwix there.
	catalog, _ := url.Parse("https://127.0.0.1:8443")
	local := "https://127.0.0.1:8443/m1/" + testMeta4File
	other := "https://127.0.0.1:9999/m1/" + testMeta4File
	doc := meta4Doc(testMeta4File, "42", meta4SHA256(testMeta4SHA), `<url priority="1">`+local+`</url><url priority="2">`+other+`</url>`)
	if got, err := parseMeta4(doc, testMeta4File, catalog); err != nil || !reflect.DeepEqual(got.URLs, []string{local}) {
		t.Fatalf("trusted catalog host: %v, %v", got.URLs, err)
	}
	if got, err := parseMeta4(doc, testMeta4File); err != nil || len(got.URLs) != 0 {
		t.Fatalf("without a trusted host: %v, %v", got.URLs, err)
	}
}

func TestIsLocalHost(t *testing.T) {
	for host, want := range map[string]bool{
		"localhost": true, "LOCALHOST": true, "localhost.": true, "wiki.localhost": true,
		"127.0.0.1": true, "127.1.2.3": true, "0.0.0.0": true, "0.1.2.3": true, "::": true, "::1": true,
		"10.0.0.1": true, "172.16.5.5": true, "192.168.1.1": true, "100.64.0.1": true, "169.254.1.1": true,
		"fe80::1": true, "fe80::1%eth0": true, "fc00::1": true, "::ffff:10.0.0.1": true, "224.0.0.1": true, "ff02::1": true,
		"2130706433": true, "0x7f.1": true, "127.1": true, "": true,
		"ftp.fau.de": false, "download.kiwix.org": false, "8.8.8.8": false, "172.32.0.1": false, "100.128.0.1": false,
		"2606:4700:4700::1111": false, "localhost.example.com": false, "mirror2.example": false,
	} {
		if got := isLocalHost(host); got != want {
			t.Errorf("isLocalHost(%q) = %v, want %v", host, got, want)
		}
	}
}
