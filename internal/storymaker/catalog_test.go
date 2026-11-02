package storymaker

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
)

var (
	miniOnce sync.Once
	miniCat  *Catalog
	miniErr  error
)

// loadMini loads the fixture catalog (shared by the tests; it is immutable).
func loadMini(t testing.TB) *Catalog {
	t.Helper()
	miniOnce.Do(func() {
		miniCat, miniErr = LoadCatalog(os.DirFS("testdata/catalog-mini"), "catalog.json")
	})
	if miniErr != nil {
		t.Fatalf("LoadCatalog(catalog-mini): %v", miniErr)
	}
	return miniCat
}

func TestLoadCatalogMini(t *testing.T) {
	c := loadMini(t)
	if c.Version() != "mini00000000" || c.Schema != CatalogSchema || c.Stage.Width != 1920 || c.Rig.ID != "rig.human/1" {
		t.Fatalf("header: %q %q %+v %+v", c.Version(), c.Schema, c.Stage, c.Rig)
	}
	if e, ok := c.Lookup("prop.lock"); !ok || e.Kind != "prop" || e.File != "props/lock.json" || e.NameKey != "storymaker.catalog.prop_lock" {
		t.Fatalf("prop.lock: %+v %v", e, ok)
	}
	if e, ok := c.Lookup("palette.sunny"); !ok || e.Kind != "palette" || e.Name != "sunny" {
		t.Fatalf("palette.sunny: %+v %v", e, ok)
	}
	if _, ok := c.Lookup("prop.nothing"); ok {
		t.Fatal("found a missing id")
	}
	e, _ := c.Lookup("pose.typing")
	var raw map[string]any
	if err := json.Unmarshal(e.Raw(), &raw); err != nil || raw["seated"] != true {
		t.Fatalf("raw entry lost kind-specific fields: %s", e.Raw())
	}
	if out, err := json.Marshal(e); err != nil || string(out) != string(e.Raw()) {
		t.Fatalf("entry must marshal as the catalog has it: %s %v", out, err)
	}
	cases := map[string][]string{
		"pose":        {"stand", "typing"},
		"expression":  {"neutral", "surprised", "happy"},
		"transition":  {"cut", "fade", "slide-left", "wipe"},
		"palette":     {"sunny"},
		"font":        {"default"},
		"environment": {"env.plain-stage", "env.office", "env.abstract-tech"},
		"trans":       {"cut", "fade", "slide-left", "wipe"}, // alias for the id prefix
	}
	for kind, want := range cases {
		if got := c.Names(kind); !reflect.DeepEqual(got, want) {
			t.Errorf("Names(%s) = %v, want %v", kind, got, want)
		}
	}
	if c.Names("banana") != nil {
		t.Error("unknown kind must return nil")
	}
	if !c.HasName("pose", "typing") || c.HasName("pose", "facepalm") || !c.HasName("text", "title") {
		t.Error("HasName")
	}
}

func TestCatalogSearch(t *testing.T) {
	c := loadMini(t)
	ids := func(es []Entry) []string {
		var out []string
		for _, e := range es {
			out = append(out, e.ID)
		}
		return out
	}
	cases := []struct {
		query, kind string
		limit       int
		want        []string
	}{
		{"padlock", "", 0, []string{"prop.lock"}},                              // tag
		{"lock", "prop", 0, []string{"prop.lock"}},                             // id and tag, kind filter
		{"security", "prop", 0, []string{"prop.lock", "prop.shield"}},          // tie keeps catalog order
		{"prop.shield", "", 0, []string{"prop.shield"}},                        // exact id first
		{"office woman", "", 0, []string{"char.office-woman-1", "env.office"}}, // two terms add up
		{"locks", "prop", 0, []string{"prop.lock"}},                            // plural
		{"office", "environment", 0, []string{"env.office"}},
		{"nothing-like-this", "", 0, nil},
		{"", "transition", 0, []string{"trans.cut", "trans.fade", "trans.slide-left", "trans.wipe"}}, // list a kind
		{"", "trans", 2, []string{"trans.cut", "trans.fade"}},                                        // alias and limit
		{"SECURITY", "prop", 1, []string{"prop.lock"}},                                               // case-insensitive
		{"x", "banana", 0, nil},                                                                      // unknown kind
	}
	for _, tc := range cases {
		got := ids(c.Search(tc.query, tc.kind, tc.limit))
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Search(%q, %q, %d) = %v, want %v", tc.query, tc.kind, tc.limit, got, tc.want)
		}
	}
	if n := len(c.Search("", "", 0)); n != 6 {
		t.Errorf("default limit: %d", n)
	}
	if n := len(c.Search("", "", 100)); n != 12 {
		t.Errorf("limit cap: %d", n)
	}
}

func TestCatalogSpots(t *testing.T) {
	c := loadMini(t)
	spots, ok := c.Spots("env.office", "16:9")
	if !ok || spots["desk"].Face != "right" || spots["center"].X != 50 || len(spots) != 9 {
		t.Fatalf("office spots: %v %+v", ok, spots)
	}
	// 9:16 replaces single spots of plain-stage and keeps the others.
	tall, _ := c.Spots("env.plain-stage", "9:16")
	if tall["left"].X != 42 || tall["right"].X != 58 || tall["center"].X != 50 || tall["board"].X != 74 {
		t.Fatalf("9:16 spots: %+v", tall)
	}
	if base, _ := c.Spots("env.plain-stage", "16:9"); base["left"].X != 30 {
		t.Fatalf("the base spots changed: %+v", base)
	}
	if got := c.SpotNames("env.office"); !reflect.DeepEqual(got, []string{"far-left", "left", "center", "right", "far-right", "board", "desk", "door", "window"}) {
		t.Fatalf("SpotNames: %v", got)
	}
	bg, ok := c.Spots("my.bg.park", "16:9")
	if !ok || len(bg) != 3 || bg["center"].X != 50 {
		t.Fatalf("project background spots: %v %+v", ok, bg)
	}
	if _, ok := c.Spots("env.nowhere", "16:9"); ok {
		t.Fatal("unknown environment has spots")
	}
	if _, ok := c.Spots("prop.lock", "16:9"); ok {
		t.Fatal("a prop is not an environment")
	}
	if c.FocusX("env.office", "9:16") != 40 || c.FocusX("env.office", "16:9") != 50 {
		t.Fatalf("FocusX: %v %v", c.FocusX("env.office", "9:16"), c.FocusX("env.office", "16:9"))
	}
}

// miniWith returns a copy of the mini catalog JSON with edits applied.
func miniWith(t *testing.T, edit func(string) string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/catalog-mini/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	return []byte(edit(string(data)))
}

func TestLoadCatalogRejectsBadManifests(t *testing.T) {
	cases := []struct {
		name string
		edit func(string) string
		want string
	}{
		{"schema", func(s string) string {
			return strings.Replace(s, `"storymaker-catalog/1"`, `"storymaker-catalog/9"`, 1)
		}, "schema"},
		{"empty version", func(s string) string { return strings.Replace(s, `"mini00000000"`, `""`, 1) }, "version"},
		{"unknown kind", func(s string) string { return strings.Replace(s, `"kind":"cycle"`, `"kind":"banana"`, 1) }, `unknown kind "banana"`},
		{"palette in entries", func(s string) string { return strings.Replace(s, `"kind":"cycle"`, `"kind":"palette"`, 1) }, "palettes list"},
		{"duplicate id", func(s string) string { return strings.Replace(s, `"id":"prop.shield"`, `"id":"prop.lock"`, 1) }, "duplicate id prop.lock"},
		{"wrong prefix", func(s string) string {
			return strings.Replace(s, `"id":"prop.shield","kind":"prop"`, `"id":"obj.shield","kind":"prop"`, 1)
		}, "must start with prop."},
		{"enum name differs from id", func(s string) string { return strings.Replace(s, `"name":"typing"`, `"name":"typed"`, 1) }, `name must be "typing"`},
		{"name key", func(s string) string {
			return strings.Replace(s, `"storymaker.catalog.prop_lock"`, `"storymaker.catalog.lock"`, 1)
		}, "name_key must be storymaker.catalog.prop_lock"},
		{"thumb path", func(s string) string {
			return strings.Replace(s, `"thumbs/prop.lock.svg"`, `"../thumbs/prop.lock.svg"`, 1)
		}, "thumb"},
		{"prop without size", func(s string) string { return strings.Replace(s, `"size":{"h":0.14},`, ``, 1) }, "size.h"},
		{"prop file escapes", func(s string) string { return strings.Replace(s, `"props/lock.json"`, `"/etc/passwd"`, 1) }, "file must be a relative path"},
		{"character param", func(s string) string { return strings.Replace(s, `"accessory":"none"`, `"accessory":""`, 1) }, "params.accessory"},
		{"environment base spot", func(s string) string {
			return strings.Replace(s, `"far-right":{"x":88,"y":88},"board":{"x":74,"y":62}`, `"board":{"x":74,"y":62}`, 1)
		}, "spots.far-right is required"},
		{"spot range", func(s string) string {
			return strings.Replace(s, `"board":{"x":74,"y":62}`, `"board":{"x":174,"y":62}`, 1)
		}, "between 0 and 100"},
		{"aspect key", func(s string) string {
			return strings.Replace(s, `"1:1":{"focus_x":50}}},`, `"4:3":{"focus_x":50}}},`, 1)
		}, `aspects key "4:3"`},
		{"expression mouth", func(s string) string {
			return strings.Replace(s, `"eyes":"wide","brows":"raised","mouth":"o"`, `"eyes":"wide","brows":"raised"`, 1)
		}, "eyes, brows and mouth"},
		{"mouth shape", func(s string) string { return strings.Replace(s, `"shape":"rest"`, `"shape":"banana"`, 1) }, "shape must be one of"},
		{"transition duration", func(s string) string { return strings.Replace(s, `"duration":0}`, `"x":0}`, 1) }, "duration"},
		{"text style", func(s string) string { return strings.Replace(s, `"max_lines":2`, `"max_lines":0`, 1) }, "max_lines"},
		{"palette token", func(s string) string { return strings.Replace(s, `"bg2":"#ffe0a3",`, ``, 1) }, "token bg2"},
		{"no default font", func(s string) string {
			return strings.Replace(s, `"name":"default","family"`, `"name":"body","family"`, 1)
		}, "font named default"},
		{"font path", func(s string) string {
			return strings.Replace(s, `"/fonts/placeholder.woff2"`, `"fonts/placeholder.woff2"`, 1)
		}, "/fonts/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := miniWith(t, tc.edit)
			if string(data) == string(miniWith(t, func(s string) string { return s })) {
				t.Fatal("the edit changed nothing")
			}
			fsys := fstest.MapFS{"catalog.json": {Data: data}}
			_, err := LoadCatalog(fsys, "catalog.json")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %v, want one containing %q", err, tc.want)
			}
		})
	}
	if _, err := LoadCatalog(fstest.MapFS{}, "catalog.json"); err == nil {
		t.Error("a missing catalog must fail")
	}
	if _, err := LoadCatalog(fstest.MapFS{"catalog.json": {Data: []byte("{")}}, "catalog.json"); err == nil {
		t.Error("broken JSON must fail")
	}
}

func TestCatalogVerifyFiles(t *testing.T) {
	c := loadMini(t)
	// The mini fixture references files that part C owns; the manifest alone loads fine.
	fsys := fstest.MapFS{}
	err := c.VerifyFiles(fsys, "img/storymaker")
	if err == nil || !strings.Contains(err.Error(), "img/storymaker/rig/human.json") || !strings.Contains(err.Error(), "27 missing files") {
		t.Fatalf("missing files not reported: %v", err)
	}
	for _, e := range c.Entries() {
		if e.Kind == "palette" {
			continue
		}
		if e.File != "" {
			fsys["img/storymaker/"+e.File] = &fstest.MapFile{Data: []byte("{}")}
		}
		fsys["img/storymaker/"+e.Thumb] = &fstest.MapFile{Data: []byte("<svg/>")}
	}
	fsys["img/storymaker/rig/human.json"] = &fstest.MapFile{Data: []byte("{}")}
	if err := c.VerifyFiles(fsys, "img/storymaker"); err == nil || !strings.Contains(err.Error(), "fonts/placeholder.woff2") || strings.Contains(err.Error(), "thumbs/") {
		t.Fatalf("only the font should be missing: %v", err)
	}
	fsys["fonts/placeholder.woff2"] = &fstest.MapFile{Data: []byte("font")}
	if err := c.VerifyFiles(fsys, "img/storymaker"); err != nil {
		t.Fatalf("complete tree rejected: %v", err)
	}
}
