package ecalc

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// testWords is the data of the test-only code kind.
type testWords struct {
	Words []string `yaml:"words"`
}

// The code kinds are a process-wide registry (the real ones register themselves in an init function), so the test kind
// is registered once, here.
func init() {
	RegisterCodeKind("test_words", CodeKind{
		Load: func(raw *yaml.Node, fail func(string, ...any)) any {
			var d testWords
			if err := decodeNode(raw, &d); err != nil {
				fail("data: %v", err)
				return nil
			}
			if len(d.Words) == 0 {
				fail("data: words must not be empty")
			}
			return d
		},
		JSON: func(data any, lang string) any {
			return map[string]any{"lang": lang, "words": data.(testWords).Words}
		},
	})
}

var baseCode = strings.ReplaceAll(`- id: t_code
  kind: code
  category: codes
  code: test_words
  name: NAME16
  doc: {en: "Doc.", de: "Doku."}
  tags: [test]
  source: "Textbook"
  data: {words: [alpha, beta]}
`, "NAME16", flow16("T"))

func TestLoadCodeItem(t *testing.T) {
	cat, err := Load(treeFS(t, map[string]string{"codes/k.yaml": baseCode}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	c := cat.Code("t_code")
	if c == nil || len(cat.Codes) != 1 || c.CodeKind != "test_words" || c.Category != "codes" || c.Source != "Textbook" || c.Name["en"] != "T" {
		t.Fatalf("unexpected code item %+v", c)
	}
	if d, ok := c.Data.(testWords); !ok || !reflect.DeepEqual(d.Words, []string{"alpha", "beta"}) {
		t.Errorf("Data = %#v, want the typed value that the kind returned", c.Data)
	}
	if kind, ok := cat.Item("t_code"); !ok || kind != "code" || cat.Calculator("t_code") != nil || cat.Table("t_code") != nil {
		t.Errorf("Item(t_code) = %q, %v", kind, ok)
	}
	if !contains(CodeKindIDs(), "test_words") {
		t.Errorf("CodeKindIDs() = %v lacks test_words", CodeKindIDs())
	}

	body, _, err := cat.LocalizedJSON("de")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(body, &doc); err != nil || len(doc.Items) != 1 {
		t.Fatalf("decode: %v (%d items)", err, len(doc.Items))
	}
	it := doc.Items[0]
	if len(it) != 10 {
		t.Errorf("the code item has %d keys, want 10: %v", len(it), it)
	}
	if it["kind"] != "code" || it["id"] != "t_code" || it["category"] != "codes" || it["code"] != "test_words" || it["name"] != "T" || it["name_en"] != "T" ||
		it["doc"] != "Doku." || it["source"] != "Textbook" || !reflect.DeepEqual(it["tags"], []any{"test"}) {
		t.Errorf("code item = %v", it)
	}
	if !reflect.DeepEqual(it["data"], map[string]any{"lang": "de", "words": []any{"alpha", "beta"}}) {
		t.Errorf("data = %v, want what the JSON function of the kind rendered for the language", it["data"])
	}
	checkOtherItemJSON(t, it)

	// A doc is optional for a code item; it is written as "".
	bare, err := Load(treeFS(t, map[string]string{"codes/k.yaml": mutate(t, baseCode, "  doc: {en: \"Doc.\", de: \"Doku.\"}\n", "")}))
	if err != nil {
		t.Fatalf("a code item without doc: %v", err)
	}
	if bare.Code("t_code").Doc != nil {
		t.Errorf("Doc = %v, want none", bare.Code("t_code").Doc)
	}
}

func TestLoadRejectsBrokenCodeItems(t *testing.T) {
	cases := []struct{ name, old, new, want string }{
		{"unknown code kind", `code: test_words`, `code: nope`, `code "nope" is not a registered code kind (`},
		{"wrong kind", `kind: code`, `kind: table`, "kind must be code in codes/"},
		{"wrong category", `category: codes`, `category: tables`, "category must be codes"},
		{"missing locale", `, zh: "T"}`, `}`, `missing "zh" text`},
		{"doc without german", `doc: {en: "Doc.", de: "Doku."}`, `doc: {en: "Doc."}`, `doc: missing "de" text`},
		{"missing source", `source: "Textbook"`, `source: ""`, "source is required"},
		{"no tags", `tags: [test]`, `tags: []`, "at least one tag is required"},
		{"unknown field", `tags: [test]`, "tags: [test]\n  colour: red", "colour"},
		{"bad id", `id: t_code`, `id: T-code`, "id must match"},
		{"data the kind rejects", `data: {words: [alpha, beta]}`, `data: {words: []}`, "t_code: data: words must not be empty"},
		{"data with an unknown field", `data: {words: [alpha, beta]}`, `data: {words: [alpha], extra: 1}`, "extra"},
	}
	for _, c := range cases {
		_, err := Load(treeFS(t, map[string]string{"codes/k.yaml": mutate(t, baseCode, c.old, c.new)}))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v, want it to contain %q", c.name, err, c.want)
		}
	}

	// The id space is shared with the other kinds, and a code item does not belong in the other directories.
	_, err := Load(treeFS(t, map[string]string{"tables/t.yaml": baseTable, "codes/k.yaml": strings.Replace(baseCode, "id: t_code", "id: t_sec", 1)}))
	if err == nil || !strings.Contains(err.Error(), "id is already used in tables/t.yaml") {
		t.Errorf("code with the id of a table: error = %v", err)
	}
	_, err = Load(treeFS(t, map[string]string{"codes/k.yaml": baseCalc}))
	if err == nil || !strings.Contains(err.Error(), "field vars not found in type ecalc.rawCode") {
		t.Errorf("calculator in codes/: error = %v", err)
	}
	_, err = Load(treeFS(t, map[string]string{"codes/k.yaml": ""}))
	if err == nil || !strings.Contains(err.Error(), "the file is empty") {
		t.Errorf("empty file: error = %v", err)
	}
}

func TestRegisterCodeKindRefusesWhatCannotWork(t *testing.T) {
	ok := CodeKind{Load: func(*yaml.Node, func(string, ...any)) any { return nil }, JSON: func(any, string) any { return nil }}
	for name, call := range map[string]func(){
		"a second registration": func() { RegisterCodeKind("test_words", ok) },
		"a bad id":              func() { RegisterCodeKind("Bad-Id", ok) },
		"no Load":               func() { RegisterCodeKind("test_noload", CodeKind{JSON: ok.JSON}) },
		"no JSON":               func() { RegisterCodeKind("test_nojson", CodeKind{Load: ok.Load}) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s must panic", name)
				}
			}()
			call()
		}()
	}
}
