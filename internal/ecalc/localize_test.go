package ecalc

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/catalog.en.json from the embedded catalog")

// catalogDoc is the part of the catalog JSON the tests look at.
type catalogDoc struct {
	Version    int    `json:"version"`
	Lang       string `json:"lang"`
	Categories []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"categories"`
	Items []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		NameEN string `json:"name_en"`
		Doc    string `json:"doc"`
		Vars   []struct {
			ID          string    `json:"id"`
			Name        string    `json:"name"`
			Default     *float64  `json:"default"`
			DefaultList []float64 `json:"default_list"`
		} `json:"vars"`
		Checks []struct {
			Warn string `json:"warn"`
		} `json:"checks"`
		Sets [][]string `json:"sets"`
	} `json:"items"`
}

func fetch(t *testing.T, lang string) (catalogDoc, []byte, string) {
	t.Helper()
	cat, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	body, etag, err := cat.LocalizedJSON(lang)
	if err != nil {
		t.Fatal(err)
	}
	var doc catalogDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("LocalizedJSON(%q) is not valid JSON: %v", lang, err)
	}
	return doc, body, etag
}

func TestCatalogSnapshot(t *testing.T) {
	_, body, _ := fetch(t, "en")
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, body, "", "  "); err != nil {
		t.Fatal(err)
	}
	pretty.WriteByte('\n')
	const path = "testdata/catalog.en.json"
	if *update {
		if err := os.WriteFile(path, pretty.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (create it with: go test ./internal/ecalc/ -run TestCatalogSnapshot -update)", err)
	}
	want = bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n"))
	if !bytes.Equal(pretty.Bytes(), want) {
		t.Errorf("%s is stale; regenerate it with: go test ./internal/ecalc/ -run TestCatalogSnapshot -update", path)
	}
}

func TestLocalizedJSONLanguagesAndFallback(t *testing.T) {
	en, _, _ := fetch(t, "en")
	cat, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	// The item count follows the catalog (content files add calculators); the 13 categories are fixed.
	if en.Version != 1 || en.Lang != "en" || len(en.Items) != len(cat.Calculators) || len(en.Items) < 12 || len(en.Categories) != 13 {
		t.Fatalf("en: version %d lang %q items %d (catalog %d) categories %d", en.Version, en.Lang, len(en.Items), len(cat.Calculators), len(en.Categories))
	}
	find := func(d catalogDoc, id string) int {
		for i, it := range d.Items {
			if it.ID == id {
				return i
			}
		}
		t.Fatalf("no item %s", id)
		return -1
	}
	vd := find(en, "voltage_divider")
	if en.Items[vd].Name != "Voltage divider" || en.Categories[0].Name != "Basics" || en.Items[vd].Vars[0].Name != "Input voltage" {
		t.Errorf("en strings: %q %q %q", en.Items[vd].Name, en.Categories[0].Name, en.Items[vd].Vars[0].Name)
	}

	de, _, _ := fetch(t, "de")
	if de.Lang != "de" || de.Items[vd].Name != "Spannungsteiler" || de.Categories[0].Name != "Grundlagen" ||
		de.Items[vd].Vars[0].Name != "Eingangsspannung" || !strings.HasPrefix(de.Items[vd].Doc, "Zwei in Reihe") ||
		!strings.HasPrefix(de.Items[vd].Checks[0].Warn, "V_out liegt nicht unter") {
		t.Errorf("de strings: %+v", de.Items[vd])
	}

	// French has a name but no doc or variable names: those fall back to English.
	fr, _, _ := fetch(t, "fr")
	if fr.Items[vd].Name != "Diviseur de tension" || fr.Items[vd].Doc != en.Items[vd].Doc || fr.Items[vd].Vars[0].Name != "Input voltage" {
		t.Errorf("fr fallback: %+v", fr.Items[vd])
	}

	// Unknown or empty languages are English; region tags and aliases are normalized.
	_, enBody, enTag := fetch(t, "en")
	for _, lang := range []string{"", "xx", "tlh", "en-US"} {
		if doc, body, tag := fetch(t, lang); !bytes.Equal(body, enBody) || tag != enTag || doc.Lang != "en" {
			t.Errorf("LocalizedJSON(%q) is not the English catalog (lang %q, etag %s, want %s)", lang, doc.Lang, tag, enTag)
		}
	}
	_, deBody, _ := fetch(t, "de")
	_, noBody, _ := fetch(t, "no")
	for lang, want := range map[string][]byte{"de-DE": deBody, "DE_de": deBody, " de ": deBody, "nb": noBody} {
		if _, body, _ := fetch(t, lang); !bytes.Equal(body, want) {
			t.Errorf("LocalizedJSON(%q) was not normalized", lang)
		}
	}
}

// Every item carries its English name, in every language and also in the English catalog itself, so that the app
// finds "voltage divider" in a German catalog without a second request.
func TestLocalizedJSONCarriesTheEnglishName(t *testing.T) {
	en, _, _ := fetch(t, "en")
	if len(en.Items) == 0 {
		t.Fatal("the English catalog has no items")
	}
	for _, lang := range Locales {
		doc, _, _ := fetch(t, lang)
		if len(doc.Items) != len(en.Items) {
			t.Fatalf("%s: %d items, want %d", lang, len(doc.Items), len(en.Items))
		}
		for i, it := range doc.Items {
			if it.NameEN == "" || it.NameEN != en.Items[i].Name || en.Items[i].NameEN != en.Items[i].Name {
				t.Errorf("%s %s: name_en = %q, want the English name %q", lang, it.ID, it.NameEN, en.Items[i].Name)
			}
		}
	}
	de, _, _ := fetch(t, "de")
	for _, it := range de.Items {
		if it.ID == "voltage_divider" && (it.Name != "Spannungsteiler" || it.NameEN != "Voltage divider") {
			t.Errorf("de voltage_divider: name %q, name_en %q", it.Name, it.NameEN)
		}
	}
}

func TestLocalizedJSONETagAndCache(t *testing.T) {
	cat, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	body, etag, _ := cat.LocalizedJSON("en")
	again, etag2, _ := cat.LocalizedJSON("en")
	_, deTag, _ := cat.LocalizedJSON("de")
	if etag != etag2 || etag == deTag || &body[0] != &again[0] {
		t.Errorf("the ETag must be stable per language, differ between languages and the body must be cached (%s %s %s)", etag, etag2, deTag)
	}
	if !regexp.MustCompile(`^"ecalc-[0-9a-f]{16}"$`).MatchString(etag) {
		t.Errorf("etag = %s, want \"ecalc-\" and 16 hex digits in quotes", etag)
	}

	// The cache is keyed by the normalized language, so arbitrary ?lang= values cannot make it grow.
	for i := 0; i < 200; i++ {
		if _, _, err := cat.LocalizedJSON(fmt.Sprintf("zz%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	cat.locMu.Lock()
	cached := len(cat.loc)
	cat.locMu.Unlock()
	if cached > len(Locales) {
		t.Errorf("the cache holds %d languages after 200 junk requests, want at most %d", cached, len(Locales))
	}
}

func TestLocalizedJSONNeverHasNullCollections(t *testing.T) {
	_, body, _ := fetch(t, "en")
	var doc struct {
		Units struct {
			Prefixes []any            `json:"prefixes"`
			Dims     []map[string]any `json:"dims"`
		} `json:"units"`
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Units.Prefixes == nil || len(doc.Units.Dims) == 0 {
		t.Fatal("units are missing")
	}
	for _, d := range doc.Units.Dims {
		for _, key := range []string{"aliases", "prefixes", "units"} {
			if _, ok := d[key].([]any); !ok {
				t.Errorf("dim %v: %s = %#v, want an array", d["id"], key, d[key])
			}
		}
		if _, ok := d["integer"].(bool); !ok {
			t.Errorf("dim %v: integer = %#v, want a boolean", d["id"], d["integer"])
		}
		for _, un := range d["units"].([]any) {
			um := un.(map[string]any)
			if _, ok := um["aliases"].([]any); !ok {
				t.Errorf("dim %v unit %v: aliases = %#v, want an array", d["id"], um["id"], um["aliases"])
			}
			if _, ok := um["radix"].(float64); !ok {
				t.Errorf("dim %v unit %v: radix = %#v, want a number", d["id"], um["id"], um["radix"])
			}
		}
	}
	for _, it := range doc.Items {
		for _, key := range []string{"tags", "vars", "choices", "rules", "derived", "checks", "formula", "sets"} {
			if _, ok := it[key].([]any); !ok {
				t.Errorf("item %v: %s = %#v, want an array", it["id"], key, it[key])
			}
		}
		if _, ok := it["finder"].(string); !ok {
			t.Errorf("item %v: finder = %#v, want a string", it["id"], it["finder"])
		}
		chart, hasChart := it["chart"]
		if _, isObject := chart.(map[string]any); !hasChart || (chart != nil && !isObject) {
			t.Errorf("item %v: chart = %#v, want null or an object", it["id"], chart)
		}
		for _, v := range it["vars"].([]any) {
			vm := v.(map[string]any)
			for _, key := range []string{"id", "dim", "label", "name", "default", "default_list", "unit", "eseries", "list", "input"} {
				if _, ok := vm[key]; !ok {
					t.Errorf("item %v var %v lacks the key %s", it["id"], vm["id"], key)
				}
			}
			if _, ok := vm["units"].([]any); !ok {
				t.Errorf("item %v var %v: units = %#v, want an array", it["id"], vm["id"], vm["units"])
			}
		}
		for _, d := range it["derived"].([]any) {
			if _, ok := d.(map[string]any)["units"].([]any); !ok {
				t.Errorf("item %v derived %v: units is not an array", it["id"], d.(map[string]any)["id"])
			}
		}
		for _, ch := range it["choices"].([]any) {
			for _, o := range ch.(map[string]any)["options"].([]any) {
				if _, ok := o.(map[string]any)["ignore"].([]any); !ok {
					t.Errorf("item %v choice option %v: ignore is not an array", it["id"], o.(map[string]any)["id"])
				}
			}
		}
	}
}

func TestLocalizedJSONDefaultsAndSets(t *testing.T) {
	en, _, _ := fetch(t, "en")
	byID := map[string]int{}
	for i, it := range en.Items {
		byID[it.ID] = i
	}
	vd := en.Items[byID["voltage_divider"]]
	if vd.Vars[0].Default == nil || *vd.Vars[0].Default != 12 || vd.Vars[3].Default != nil || vd.Vars[0].DefaultList != nil || len(vd.Sets) != 4 {
		t.Errorf("voltage_divider vars/sets: %+v", vd)
	}
	sp := en.Items[byID["series_parallel"]]
	if sp.Vars[0].Default != nil || len(sp.Vars[0].DefaultList) != 3 || sp.Vars[0].DefaultList[0] != 4700 {
		t.Errorf("series_parallel list default: %+v", sp.Vars[0])
	}
}
