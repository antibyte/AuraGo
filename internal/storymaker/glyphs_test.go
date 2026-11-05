package storymaker

import (
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

// Face "a": d [233,0] is U+00E9 only.
// Face "b": d [65,2,10,3,21,0,132,0] decodes to 65-67, then 67+10=77 to 77+3=80, then 80+21=101, then 101+132=233.
const glyphsFixture = `{"schema":"storymaker-glyphs/1","faces":[
 {"id":"a","family":"A","file":"/fonts/a.woff2","weight":"300 700","unicode_range":"U+00E9-00E9","d":[233,0]},
 {"id":"b","family":"B","file":"/fonts/qr/b.otf","weight":"400","unicode_range":"","max_bytes":17825792,"d":[65,2,10,3,21,0,132,0]}
]}`

func loadGlyphsFixture(t *testing.T) *Glyphs {
	t.Helper()
	g, err := LoadGlyphs(fstest.MapFS{"glyphs.json": {Data: []byte(glyphsFixture)}}, "glyphs.json")
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestLoadGlyphsDecodesFacesInOrder(t *testing.T) {
	g := loadGlyphsFixture(t)
	want := []GlyphFace{
		{ID: "a", Family: "A", File: "/fonts/a.woff2", Weight: "300 700", UnicodeRange: "U+00E9-00E9"},
		{ID: "b", Family: "B", File: "/fonts/qr/b.otf", Weight: "400", MaxBytes: 17825792},
	}
	if !reflect.DeepEqual(g.Faces, want) {
		t.Fatalf("Faces = %+v", g.Faces)
	}
	// U+00E9 is in both faces, and the first one in order wins. 68 and 76 fall in the gap between the first two ranges of "b".
	for r, face := range map[rune]int{0xE9: 0, 65: 1, 67: 1, 68: -1, 76: -1, 77: 1, 80: 1, 81: -1, 101: 1} {
		if got := g.FaceOf(r); got != face {
			t.Errorf("FaceOf(%U) = %d, want %d", r, got, face)
		}
	}
}

func TestLoadGlyphsRefusesBadFiles(t *testing.T) {
	face := func(d string) string {
		return `{"schema":"storymaker-glyphs/1","faces":[{"id":"a","family":"A","file":"/fonts/a.woff2","weight":"400","unicode_range":"","d":` + d + `}]}`
	}
	cases := []struct {
		name string
		data string
		want string
	}{
		{"gap of zero", face("[65,2,0,1]"), "monotonic"},
		{"negative gap", face("[65,2,-3,1]"), "monotonic"},
		{"odd value count", face("[65,2,10]"), "pairs"},
		{"past the last code point", face("[1114111,1]"), "U+10FFFF"},
		{"wrong schema", strings.Replace(glyphsFixture, "storymaker-glyphs/1", "storymaker-glyphs/2", 1), "schema"},
		{"file outside the fonts folder", strings.Replace(glyphsFixture, "/fonts/a.woff2", "a.woff2", 1), "/fonts/"},
		{"over 1 MiB", `{"schema":"storymaker-glyphs/1","faces":[],"pad":"` + strings.Repeat("x", 1<<20) + `"}`, "larger than"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadGlyphs(fstest.MapFS{"glyphs.json": {Data: []byte(tc.data)}}, "glyphs.json")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %v, want one containing %q", err, tc.want)
			}
		})
	}
}

func TestGlyphsCoveredExemptions(t *testing.T) {
	g := loadGlyphsFixture(t)
	// U+2028 and U+2009 are JS whitespace, U+200C and U+200D are default-ignorable, U+0001 is a C0 control.
	for _, r := range []rune{0xE9, 0x2028, 0x2009, 0x200C, 0x200D, 0x0001} {
		if !g.Covered(r) {
			t.Errorf("Covered(%U) = false", r)
		}
	}
	for _, r := range []rune{0x0E01, 0x0301} {
		if g.Covered(r) {
			t.Errorf("Covered(%U) = true", r)
		}
	}
	if !IsDefaultIgnorable(0x200C) || !IsDefaultIgnorable(0x200D) || IsDefaultIgnorable('a') {
		t.Error("IsDefaultIgnorable does not follow the generated table")
	}
}

func TestGlyphsMissingUsesTheNFCFormAndKeepsOrder(t *testing.T) {
	g := loadGlyphsFixture(t)
	// e + U+0301 composes to U+00E9, which the fixture covers. Read raw, U+0301 would be missing.
	if got := g.Missing("e" + string(rune(0x0301))); got != nil {
		t.Errorf("NFC form: Missing = %U", got)
	}
	text := string([]rune{0x0E02, 0x0E01, 0x0E02, 'e'})
	if got, want := g.Missing(text), []rune{0x0E02, 0x0E01}; !reflect.DeepEqual(got, want) {
		t.Errorf("Missing = %U, want %U", got, want)
	}
}

func TestIsJSSpaceIsExactlyTheFrozenSet(t *testing.T) {
	for _, r := range []rune{0x09, 0x0D, 0x20, 0xA0, 0x1680, 0x2000, 0x200A, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF} {
		if !IsJSSpace(r) {
			t.Errorf("IsJSSpace(%U) = false", r)
		}
	}
	for _, r := range []rune{0x08, 0x0E, 0x85, 0x180E, 0x200B, 0x2030, 0x205E, 0xFEFE} {
		if IsJSSpace(r) {
			t.Errorf("IsJSSpace(%U) = true", r)
		}
	}
}

func TestMiniCatalogHasNoGlyphsAndSkipsTheCheck(t *testing.T) {
	c := loadMini(t)
	if c.Glyphs() != nil || c.GlyphsFile() != "" {
		t.Fatalf("mini catalog: Glyphs() = %v, GlyphsFile() = %q", c.Glyphs(), c.GlyphsFile())
	}
	thai := rune(0x0E01)
	if c.Glyphs().Missing(string(thai)) != nil || !c.Glyphs().Covered(thai) {
		t.Error("a nil Glyphs must skip the check")
	}
	var none *Catalog
	if none.Glyphs() != nil || none.GlyphsFile() != "" {
		t.Error("a nil catalog must have no glyphs")
	}
}

func TestLoadCatalogReadsGlyphsFromItsFolder(t *testing.T) {
	named := miniWith(t, func(s string) string {
		return strings.Replace(s, `"rig":{`, `"glyphs":{"file":"glyphs.json"},"rig":{`, 1)
	})
	fsys := fstest.MapFS{"img/catalog.json": {Data: named}, "img/glyphs.json": {Data: []byte(glyphsFixture)}}
	c, err := LoadCatalog(fsys, "img/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	if c.GlyphsFile() != "glyphs.json" || c.Glyphs().FaceOf(65) != 1 {
		t.Fatalf("GlyphsFile() = %q, FaceOf(65) = %d", c.GlyphsFile(), c.Glyphs().FaceOf(65))
	}
	delete(fsys, "img/glyphs.json")
	if _, err := LoadCatalog(fsys, "img/catalog.json"); err == nil || !strings.Contains(err.Error(), "glyphs") {
		t.Errorf("a missing glyphs file: error %v", err)
	}
	escaping := miniWith(t, func(s string) string {
		return strings.Replace(s, `"rig":{`, `"glyphs":{"file":"../glyphs.json"},"rig":{`, 1)
	})
	if _, err := LoadCatalog(fstest.MapFS{"img/catalog.json": {Data: escaping}}, "img/catalog.json"); err == nil || !strings.Contains(err.Error(), "relative") {
		t.Errorf("an escaping glyphs path: error %v", err)
	}
}
