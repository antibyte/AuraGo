package cydcreator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode"
)

func TestThemeFontRoles(t *testing.T) {
	want := map[string]themeFonts{
		"night":         {fontRef{"montserrat", 16}, fontRef{"montserrat", 14}, fontRef{"montserrat", 20}, fontRef{"jetbrains_mono", 16}},
		"terminal":      {fontRef{"jetbrains_mono", 16}, fontRef{"jetbrains_mono", 14}, fontRef{"jetbrains_mono", 20}, fontRef{"jetbrains_mono", 16}},
		"high_contrast": {fontRef{"montserrat", 20}, fontRef{"montserrat", 16}, fontRef{"montserrat", 24}, fontRef{"jetbrains_mono", 20}},
	}
	for id, w := range want {
		th, err := ThemeByID(id)
		if err != nil {
			t.Fatal(err)
		}
		if got := themeFontRoles(th); got != w {
			t.Errorf("%s roles = %+v, want %+v", id, got, w)
		}
	}
	edge := themeFontRoles(Theme{Font: "inter", FontSize: 12})
	if edge.Small.Size != 12 || edge.Large.Size != 14 {
		t.Errorf("smallest base: %+v", edge)
	}
	edge = themeFontRoles(Theme{Font: "inter", FontSize: 48})
	if edge.Small.Size != 40 || edge.Large.Size != 48 {
		t.Errorf("largest base: %+v", edge)
	}
}

func TestProjectFontsAndWidgetFonts(t *testing.T) {
	p := loadReferenceProject(t)
	th, _ := ThemeByID("night")
	w := &p.Screens[0].Widgets[1] // lbl_temp: montserrat 28
	w.Props["font"] = ""
	w.Props["size"] = 28.0
	b := &p.Screens[1].Widgets[2] // btn_back
	b.Props["font"] = "inter"
	var got []string
	for _, f := range projectFonts(p, th) {
		got = append(got, f.symbol())
	}
	want := []string{"ui_font_inter_16", "ui_font_jetbrains_mono_16", "ui_font_montserrat_14", "ui_font_montserrat_16",
		"ui_font_montserrat_20", "ui_font_montserrat_28"}
	if !slices.Equal(got, want) {
		t.Fatalf("projectFonts = %v, want %v", got, want)
	}
}

func TestGlyphChars(t *testing.T) {
	p := loadReferenceProject(t)
	p.Name = "Wetter ☀"
	p.Screens[0].Title = "Ωmega"
	p.Screens[0].Widgets[1].Props["text"] = "22 °C € \u0085 ż" // ° and ż are inside the ranges, U+0085 is a control
	p.Screens[1].Widgets = append(p.Screens[1].Widgets,
		Widget{ID: "w_0100", Type: "dropdown", X: 10, Y: 10, W: 100, H: 40, Props: map[string]any{"options": []any{"Ā", "→"}}},
		Widget{ID: "w_0101", Type: "msgbox", X: 10, Y: 10, W: 100, H: 80}, // default title "Hinweis" is ASCII
		Widget{ID: "w_0102", Type: "button", Name: "btn_hint", X: 10, Y: 60, W: 100, H: 40,
			Events: []EventBinding{{On: "clicked", Action: Action{Type: "logic", Hint: "hints are not shown: ♥"}}}},
	)
	got := string(glyphChars(p, []string{"Verbinde …", "Zertifikat – neu"}))
	if want := "Ω–…€→☀"; got != want {
		t.Fatalf("glyphChars = %q, want %q", got, want)
	}
}

// cyd-build rejects fonts.json "chars" holding anything below U+007F or any
// control (Cc) / surrogate (Cs) character, and the fonts already contain
// U+0020-U+007E and U+00A0-U+017F: the glyph set must never list DEL, the C1
// controls, surrogates or anything inside the ranges.
func TestNeedsExtraGlyphNeverEmitsRejectedCharacters(t *testing.T) {
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if !needsExtraGlyph(r) {
			continue
		}
		if r < 0xA0 || (r >= 0xA0 && r <= 0x17F) || unicode.IsControl(r) || unicode.Is(unicode.Cs, r) {
			t.Fatalf("needsExtraGlyph(U+%04X) = true, but cyd-build rejects it or the fonts already hold it", r)
		}
	}
	for _, r := range []rune{0x7F, 0x80, 0x85, 0x9F, 0xD800, 0xDFFF} {
		if needsExtraGlyph(r) {
			t.Errorf("needsExtraGlyph(U+%04X) = true", r)
		}
	}
	for _, r := range []rune{0x180, 0x2013, 0x2026, 0x20AC, 0x1F600, 0xFFFD} {
		if !needsExtraGlyph(r) {
			t.Errorf("needsExtraGlyph(U+%04X) = false", r)
		}
	}
}

func TestGlyphCharsSkipsDELAndC1Controls(t *testing.T) {
	p := loadReferenceProject(t)
	var text strings.Builder
	for r := rune(0x7F); r <= 0x9F; r++ { // DEL and the whole C1 block
		text.WriteRune(r)
	}
	text.WriteRune(0x2013)
	p.Screens[0].Widgets[1].Props["text"] = text.String()
	p.Name = "A" + string(rune(0x7F)) + "B"
	got := glyphChars(p, []string{string(rune(0x85)) + "x"})
	if want := []rune{0x2013}; !slices.Equal(got, want) {
		t.Fatalf("glyphChars = %U, want %U", got, want)
	}
	th, _ := ThemeByID(p.Theme)
	raw, err := renderFontsJSON(p, th)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range string(raw) {
		if r == 0x7F || (r >= 0x80 && r <= 0x9F) {
			t.Fatalf("fonts.json holds the rejected character U+%04X", r)
		}
	}
}

func TestRenderFontsJSONMatchesReference(t *testing.T) {
	p := loadReferenceProject(t)
	th, _ := ThemeByID(p.Theme)
	got, err := renderFontsJSON(p, th)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join(referenceDir, "src", "generated", "fonts.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("fonts.json:\n%s", firstDiff(string(want), string(got)))
	}
}

func TestRenderFontsJSONCharLimit(t *testing.T) {
	p := loadReferenceProject(t)
	th, _ := ThemeByID(p.Theme)
	next := rune(0x4E00) // CJK ideographs, 3 UTF-8 bytes each
	for i := 0; i < 13; i++ {
		var b strings.Builder
		for j := 0; j < 340; j++ {
			b.WriteRune(next)
			next++
		}
		p.Screens[1].Widgets = append(p.Screens[1].Widgets, Widget{ID: "w_09" + string(rune('a'+i)), Type: "label",
			X: 0, Y: 0, W: 10, H: 10, Props: map[string]any{"text": b.String()}})
	}
	_, err := renderFontsJSON(p, th)
	if ErrorCode(err) != CodeInvalidLayout || !strings.Contains(err.Error(), "4096") {
		t.Fatalf("err = %v, want CYD_INVALID_LAYOUT about the 4096 character limit", err)
	}
}

// uniqueGlyph returns a distinct CJK ideograph per index: never inside the
// font ranges and never part of a template string.
func uniqueGlyph(i int) rune { return rune(0x4E00 + i) }

// addGlyphLabels appends labels to the second screen that together carry n
// distinct code points starting at uniqueGlyph(0).
func addGlyphLabels(p *Project, n int) {
	const perLabel = 340
	for next, id := 0, 0; next < n; id++ {
		var b strings.Builder
		for j := 0; j < perLabel && next < n; j++ {
			b.WriteRune(uniqueGlyph(next))
			next++
		}
		p.Screens[1].Widgets = append(p.Screens[1].Widgets, Widget{ID: fmt.Sprintf("w_%04d", 1000+id),
			Type: "label", W: 10, H: 10, Props: map[string]any{"text": b.String()}})
	}
}

// The 4096 limit is inclusive: exactly 4096 extra characters render, one more
// is a layout error.
func TestRenderFontsJSONCharLimitBoundary(t *testing.T) {
	th, _ := ThemeByID("night")
	base := loadReferenceProject(t)
	strs, err := TemplateStrings(base.Language)
	if err != nil {
		t.Fatal(err)
	}
	have := len(glyphChars(base, strs))
	for _, tc := range []struct {
		total int
		ok    bool
	}{{maxFontChars, true}, {maxFontChars + 1, false}} {
		p := loadReferenceProject(t)
		addGlyphLabels(&p, tc.total-have)
		if got := len(glyphChars(p, strs)); got != tc.total {
			t.Fatalf("fixture holds %d extra characters, want %d", got, tc.total)
		}
		raw, err := renderFontsJSON(p, th)
		if !tc.ok {
			if ErrorCode(err) != CodeInvalidLayout || !strings.Contains(err.Error(), "4096") {
				t.Errorf("%d characters: err = %v, want CYD_INVALID_LAYOUT about the 4096 limit", tc.total, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%d characters: %v", tc.total, err)
		}
		var doc struct {
			Fonts []struct {
				Chars string `json:"chars"`
			} `json:"fonts"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		if len(doc.Fonts) == 0 {
			t.Fatal("no fonts")
		}
		for _, f := range doc.Fonts {
			if n := len([]rune(f.Chars)); n != maxFontChars {
				t.Errorf("fonts.json chars hold %d code points, want %d", n, maxFontChars)
			}
		}
	}
}

// Every string, text and list prop of every widget feeds the glyph set, so a
// new catalog prop that shows text cannot silently miss its characters. A QR
// code's text is encoded into the matrix and is the one exception.
func TestGlyphCharsCoversEveryTextProp(t *testing.T) {
	covered := map[string]int{}
	n := 0
	for _, spec := range WidgetCatalog() {
		for _, ps := range spec.Props {
			if ps.Kind != PropString && ps.Kind != PropText && ps.Kind != PropList {
				continue
			}
			if spec.Type == "qrcode" && ps.Name == "text" {
				continue
			}
			n++
			covered[ps.Kind]++
			r := uniqueGlyph(n)
			t.Run(spec.Type+"."+ps.Name, func(t *testing.T) {
				p := loadReferenceProject(t)
				var val any = string(r)
				if ps.Kind == PropList {
					val = []any{string(r)}
				}
				p.Screens[1].Widgets = append(p.Screens[1].Widgets, Widget{ID: "w_0900", Type: spec.Type, W: 10, H: 10,
					Props: map[string]any{ps.Name: val}})
				if got := glyphChars(p, nil); !slices.Contains(got, r) {
					t.Errorf("glyphChars = %U, lacks %U from %s.%s", got, r, spec.Type, ps.Name)
				}
			})
		}
	}
	for _, kind := range []string{PropString, PropText, PropList} {
		if covered[kind] == 0 {
			t.Errorf("the catalog has no %s prop to cover", kind)
		}
	}
}

func TestGlyphCharsIgnoresQRCodeText(t *testing.T) {
	p := loadReferenceProject(t)
	qr := Widget{ID: "w_0900", Type: "qrcode", W: 100, H: 100, Props: map[string]any{"text": "https://例え.example/☀?q=ü€"}}
	p.Screens[1].Widgets = append(p.Screens[1].Widgets, qr)
	if got := glyphChars(p, nil); len(got) != 0 {
		t.Fatalf("a QR code text added glyphs %U", got)
	}
	// The same text in a label is drawn and counts.
	label := Widget{ID: "w_0901", Type: "label", W: 100, H: 20, Props: map[string]any{"text": "例€"}}
	p.Screens[1].Widgets = append(p.Screens[1].Widgets, label)
	if got, want := glyphChars(p, nil), []rune("€例"); !slices.Equal(got, want) {
		t.Fatalf("glyphChars = %U, want %U", got, want)
	}
}

// genPropList hands out a copy: the catalog default and a caller's own slice
// must survive a sort or edit of the result.
func TestGenPropListReturnsCopy(t *testing.T) {
	spec, ok := widgetSpecRef("dropdown")
	if !ok {
		t.Fatal("no dropdown spec")
	}
	def := genPropList(&Widget{Type: "dropdown"}, spec, "options")
	if len(def) < 2 {
		t.Fatalf("default options = %q", def)
	}
	want := slices.Clone(def)
	slices.Reverse(def)
	def[0] = "changed"
	if again := genPropList(&Widget{Type: "dropdown"}, spec, "options"); !slices.Equal(again, want) {
		t.Fatalf("the catalog default changed: %q, want %q", again, want)
	}
	own := []string{"b", "a"}
	got := genPropList(&Widget{Type: "dropdown", Props: map[string]any{"options": own}}, spec, "options")
	slices.Sort(got)
	if !slices.Equal(own, []string{"b", "a"}) {
		t.Fatalf("the caller's slice was modified: %q", own)
	}
}
