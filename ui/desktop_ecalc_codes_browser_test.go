package ui

import (
	"regexp"
	"strings"
	"testing"
)

// The code pages of ECalc (phase 3, part UK): resistor and inductor colour bands, SMD resistor, capacitor and ceramic
// temperature codes. The tests at the top and at the end are source tests (they always run): the 45 translation keys in 16
// languages, the DOM-free drawing builders, the stylesheet section, the loader order, the use of the codes engine and of the
// store. The tests in the middle drive the pages inside the real desktop shell (AURAGO_RUN_BROWSER_SMOKE=1).

// ecalcCodeKeys are the strings of the code pages (desktop.ecalc_code_*); the page names every one of them as a literal.
var ecalcCodeKeys = []string{
	"desktop.ecalc_code_auto", "desktop.ecalc_code_band_aria", "desktop.ecalc_code_bands", "desktop.ecalc_code_class_1", "desktop.ecalc_code_class_2",
	"desktop.ecalc_code_colors", "desktop.ecalc_code_common", "desktop.ecalc_code_empty", "desktop.ecalc_code_error_invalid_code",
	"desktop.ecalc_code_error_invalid_value", "desktop.ecalc_code_error_unsupported", "desktop.ecalc_code_field_change",
	"desktop.ecalc_code_field_change_minus", "desktop.ecalc_code_field_change_plus", "desktop.ecalc_code_field_class", "desktop.ecalc_code_field_code_3digit",
	"desktop.ecalc_code_field_code_4digit", "desktop.ecalc_code_field_code_eia96", "desktop.ecalc_code_field_family", "desktop.ecalc_code_field_industry_code",
	"desktop.ecalc_code_field_layout", "desktop.ecalc_code_field_rkm", "desktop.ecalc_code_field_tc", "desktop.ecalc_code_field_tcr",
	"desktop.ecalc_code_field_temp_high", "desktop.ecalc_code_field_temp_low", "desktop.ecalc_code_field_tolerance", "desktop.ecalc_code_field_tolerance_minus",
	"desktop.ecalc_code_field_tolerance_plus", "desktop.ecalc_code_field_value_code", "desktop.ecalc_code_field_voltage", "desktop.ecalc_code_hint",
	"desktop.ecalc_code_marking", "desktop.ecalc_code_menu", "desktop.ecalc_code_none", "desktop.ecalc_code_notes", "desktop.ecalc_code_print",
	"desktop.ecalc_code_result", "desktop.ecalc_code_role_digit", "desktop.ecalc_code_role_identifier", "desktop.ecalc_code_role_multiplier",
	"desktop.ecalc_code_role_point", "desktop.ecalc_code_role_tcr", "desktop.ecalc_code_role_tolerance", "desktop.ecalc_code_role_value",
}

var ecalcCodePlaceholders = map[string][]string{
	"desktop.ecalc_code_band_aria": {"{n}", "{role}", "{color}"},
	"desktop.ecalc_code_menu":      {"{n}"},
}

func TestECalcCodeLocaleKeys(t *testing.T) {
	t.Parallel()
	if len(ecalcCodeKeys) != 45 {
		t.Errorf("%d code keys are listed, want the 45 of the plan", len(ecalcCodeKeys))
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		words := readMergedLangMap(t, lang)
		for _, key := range ecalcCodeKeys {
			if strings.TrimSpace(words[key]) == "" {
				t.Errorf("%s misses %s", lang, key)
			}
		}
		for key, placeholders := range ecalcCodePlaceholders {
			for _, placeholder := range placeholders {
				if !strings.Contains(words[key], placeholder) {
					t.Errorf("%s: %s must keep the %s placeholder: %q", lang, key, placeholder, words[key])
				}
			}
		}
	}
}

// The drawings are string builders like the chart: no document access, and no colour in the code except the hex that the
// caller passes in (the hex of the data) after the check in safeColor.
func TestECalcCodeDrawIsADomFreeStringBuilder(t *testing.T) {
	t.Parallel()
	draw := ecalcScripts(t)["ecalc-code-draw.js"]
	for _, marker := range []string{
		"window.ECalcCodeDraw = { bandsSVG, chipSVG, safeColor, POSITIONS };", "function bandsSVG(spec)", "function chipSVG(spec)", "const safeColor =",
		`class="ec-code-art"`, `class="ec-band"`, `class="ec-band-hit"`, `class="ec-band-fill`, `class="ec-band-ring"`, `class="ec-draw-body"`, `class="ec-chip-body"`, `class="ec-chip-text"`,
		`aria-haspopup="listbox"`, `aria-expanded=`, "tabindex=\"0\" role=\"button\"", "resistor: { 3:", "inductor: { 4:", "const colour = safeColor(band.color)",
	} {
		if !strings.Contains(draw, marker) {
			t.Errorf("ecalc-code-draw.js is missing %q", marker)
		}
	}
	body := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(draw, "")
	for label, pattern := range map[string]string{
		"document access": `\bdocument\b`,
		"window access":   `\bwindow\.(?:document|ECalcCore|localStorage)\b`,
		"DOM write":       `\b(?:innerHTML|createElement|appendChild|setAttribute)\b`,
		"hex colour":      `#[0-9a-fA-F]{3,8}\b`,
		"colour function": `\b(?:rgba?|hsla?)\(`,
		"inline style":    `\bstyle=`,
	} {
		if match := regexp.MustCompile(pattern).FindString(body); match != "" {
			t.Errorf("ecalc-code-draw.js must not contain a %s (%q)", label, match)
		}
	}
}
