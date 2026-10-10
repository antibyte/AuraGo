package cydcreator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"unicode"
)

// maxFontChars mirrors cyd-build's limit on fonts.json "chars" (code points).
const maxFontChars = 4096

// genFontRanges are the code point ranges every converted font contains:
// printable ASCII plus Latin-1 Supplement and Latin Extended-A.
var genFontRanges = []string{"0x20-0x7E", "0xA0-0x17F"}

// fontRef is one converted (family, size) font.
type fontRef struct {
	Family string
	Size   int
}

// symbol is the LVGL font symbol cyd-build creates: ui_font_<family>_<size>.
func (f fontRef) symbol() string { return fmt.Sprintf("ui_font_%s_%d", f.Family, f.Size) }

// themeFonts are the four font roles of cyd::ThemeColors.
type themeFonts struct {
	Normal, Small, Large, Mono fontRef
}

// themeFontRoles derives the roles from the theme: normal = theme font at the
// base size, small/large = the next FontSizes entry below/above the base (the
// base itself at either end), mono = JetBrains Mono at the base size.
func themeFontRoles(th Theme) themeFonts {
	base := th.FontSize
	small, large := base, base
	for _, s := range FontSizes {
		if s < base {
			small = s
		}
		if s > base && large == base {
			large = s
		}
	}
	return themeFonts{
		Normal: fontRef{th.Font, base},
		Small:  fontRef{th.Font, small},
		Large:  fontRef{th.Font, large},
		Mono:   fontRef{"jetbrains_mono", base},
	}
}

// widgetFont returns the font of a widget whose font or size prop is set
// ("" / 0 fall back to the theme font / base size); ok is false when the
// widget uses the inherited theme font.
func widgetFont(w *Widget, spec *WidgetSpec, th Theme) (fontRef, bool) {
	if _, has := spec.prop("font"); !has {
		return fontRef{}, false
	}
	family := genPropString(w, spec, "font")
	size := genPropInt(w, spec, "size")
	if family == "" && size == 0 {
		return fontRef{}, false
	}
	if family == "" {
		family = th.Font
	}
	if size == 0 {
		size = th.FontSize
	}
	return fontRef{family, size}, true
}

// projectFonts is the sorted set of fonts the project needs: the theme roles
// plus every widget font.
func projectFonts(p Project, th Theme) []fontRef {
	roles := themeFontRoles(th)
	set := map[fontRef]bool{roles.Normal: true, roles.Small: true, roles.Large: true, roles.Mono: true}
	p.WalkWidgets(func(_ *Screen, _ *Widget, w *Widget) bool {
		if spec, ok := widgetSpecRef(w.Type); ok {
			if f, ok := widgetFont(w, spec, th); ok {
				set[f] = true
			}
		}
		return true
	})
	out := make([]fontRef, 0, len(set))
	for f := range set {
		out = append(out, f)
	}
	slices.SortFunc(out, func(a, b fontRef) int {
		switch {
		case a.symbol() < b.symbol():
			return -1
		case a.symbol() > b.symbol():
			return 1
		}
		return 0
	})
	return out
}

// needsExtraGlyph reports whether r must be listed in fonts.json "chars":
// printable and outside genFontRanges. DEL and the C1 controls (U+007F-U+009F)
// are control characters, and surrogates cannot occur in valid UTF-8 text;
// cyd-build rejects both in "chars", so neither is ever listed.
func needsExtraGlyph(r rune) bool {
	return r > 0x7E && (r < 0xA0 || r > 0x17F) && !unicode.IsControl(r) && !unicode.Is(unicode.Cs, r)
}

// glyphChars collects every code point outside genFontRanges that the device
// can show: project name (splash), screen titles, every string/text/list prop
// (effective values, so catalog defaults count) and the template strings of
// the device language. A QR code's text is encoded into the QR matrix and
// never drawn as glyphs, so it does not count. The result is sorted by code
// point.
func glyphChars(p Project, templateStrings []string) []rune {
	set := map[rune]bool{}
	add := func(s string) {
		for _, r := range s {
			if needsExtraGlyph(r) {
				set[r] = true
			}
		}
	}
	add(p.Name)
	for _, s := range p.Screens {
		add(s.Title)
	}
	p.WalkWidgets(func(_ *Screen, _ *Widget, w *Widget) bool {
		spec, ok := widgetSpecRef(w.Type)
		if !ok {
			return true
		}
		for _, ps := range spec.Props {
			if w.Type == "qrcode" && ps.Name == "text" {
				continue
			}
			switch ps.Kind {
			case PropString, PropText:
				add(genPropString(w, spec, ps.Name))
			case PropList:
				for _, item := range genPropList(w, spec, ps.Name) {
					add(item)
				}
			}
		}
		return true
	})
	for _, s := range templateStrings {
		add(s)
	}
	out := make([]rune, 0, len(set))
	for r := range set {
		out = append(out, r)
	}
	slices.Sort(out)
	return out
}

type fontsJSONEntry struct {
	Symbol    string   `json:"symbol"`
	Family    string   `json:"family"`
	Size      int      `json:"size"`
	Ranges    []string `json:"ranges"`
	Chars     string   `json:"chars"`
	LVSymbols bool     `json:"lv_symbols"`
}

// renderFontsJSON writes src/generated/fonts.json (plan 1b "fonts.json
// composition"). More than maxFontChars extra characters is a layout error:
// cyd-build rejects such a file and the fonts would not fit the flash.
func renderFontsJSON(p Project, th Theme) ([]byte, error) {
	strs, err := TemplateStrings(p.Language)
	if err != nil {
		return nil, err
	}
	chars := glyphChars(p, strs)
	if len(chars) > maxFontChars {
		return nil, layoutError(fmt.Sprintf("the texts use %d different characters outside Latin-1 and Latin Extended-A, at most %d fit into the device fonts", len(chars), maxFontChars),
			map[string]any{"chars": len(chars), "limit": maxFontChars})
	}
	doc := struct {
		Fonts []fontsJSONEntry `json:"fonts"`
	}{Fonts: []fontsJSONEntry{}}
	for _, f := range projectFonts(p, th) {
		doc.Fonts = append(doc.Fonts, fontsJSONEntry{
			Symbol: f.symbol(), Family: f.Family, Size: f.Size,
			Ranges: slices.Clone(genFontRanges), Chars: string(chars), LVSymbols: true,
		})
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
