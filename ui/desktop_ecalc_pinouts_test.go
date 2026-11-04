package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"aurago/internal/ecalc"
)

// ecalcPinoutKeys lists every desktop.* string of the pinout pages (ECalc phase 4): the sections of the Pinouts list and the
// strings of the page. The names of the pin types and of the wire colours come from the catalog, not from here.
var ecalcPinoutKeys = []string{
	"desktop.ecalc_group_audio", "desktop.ecalc_group_boards", "desktop.ecalc_group_data", "desktop.ecalc_group_debug", "desktop.ecalc_group_ics", "desktop.ecalc_group_pc_power",
	"desktop.ecalc_pin_copy", "desktop.ecalc_pin_drawing", "desktop.ecalc_pin_face", "desktop.ecalc_pin_filter", "desktop.ecalc_pin_function", "desktop.ecalc_pin_legend",
	"desktop.ecalc_pin_name", "desktop.ecalc_pin_no_match", "desktop.ecalc_pin_notes", "desktop.ecalc_pin_number", "desktop.ecalc_pin_pick", "desktop.ecalc_pin_rows",
	"desktop.ecalc_pin_rows_filtered", "desktop.ecalc_pin_select", "desktop.ecalc_pin_type", "desktop.ecalc_pin_variant", "desktop.ecalc_pin_view", "desktop.ecalc_pin_wire",
}

var ecalcPinoutPlaceholders = map[string][]string{
	"desktop.ecalc_pin_drawing":       {"{name}", "{count}"},
	"desktop.ecalc_pin_rows":          {"{total}"},
	"desktop.ecalc_pin_rows_filtered": {"{shown}", "{total}"},
	"desktop.ecalc_pin_select":        {"{number}"},
}

func TestECalcPinoutLocaleKeys(t *testing.T) {
	t.Parallel()
	if len(ecalcPinoutKeys) != 24 {
		t.Errorf("%d pinout keys are listed, want the 24 of the plan", len(ecalcPinoutKeys))
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		words := readMergedLangMap(t, lang)
		for _, key := range ecalcPinoutKeys {
			if strings.TrimSpace(words[key]) == "" {
				t.Errorf("%s misses %s", lang, key)
			}
		}
		for key, placeholders := range ecalcPinoutPlaceholders {
			for _, placeholder := range placeholders {
				if !strings.Contains(words[key], placeholder) {
					t.Errorf("%s: %s must keep the %s placeholder: %q", lang, key, placeholder, words[key])
				}
			}
		}
	}
}

// The scripts are wired into the loader after the code page and before the shell, the shell routes the kind and knows the
// six sections of the Pinouts list in the order of ecalc.PinoutGroups.
func TestECalcPinoutPagesAreWired(t *testing.T) {
	t.Parallel()
	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	start := strings.Index(loader, "        ecalc: {")
	if start < 0 {
		t.Fatal("module-loader.js has no ecalc entry in DESKTOP_APP_ASSETS")
	}
	entry := loader[start : start+strings.Index(loader[start:], "\n        },")]
	offset := 0
	for _, script := range []string{"ecalc-table-page.js", "ecalc-code-draw.js", "ecalc-code-page.js", "ecalc-pinout-draw.js", "ecalc-pinout-page.js", "ecalc-shell.js", "/ecalc.js'"} {
		index := strings.Index(entry[offset:], script)
		if index < 0 {
			t.Fatalf("the ecalc loader entry lists %s out of order or not at all", script)
		}
		offset += index + len(script)
	}
	scripts := ecalcScripts(t)
	for _, name := range []string{"ecalc-pinout-draw.js", "ecalc-pinout-page.js"} {
		if _, ok := scripts[name]; !ok {
			t.Errorf("%s/%s is missing", ecalcDir, name)
		}
	}
	// The scripts of the app: the 21 of phase 2, the 5 of phase 3 (tables, table page, codes, code draw, code page) and the 2 of phase 4.
	if got := strings.Count(entry, "/js/desktop/apps/ecalc/"); got != 28 {
		t.Errorf("the ecalc loader entry lists %d scripts, want 28", got)
	}
	shell := scripts["ecalc-shell.js"]
	for _, marker := range []string{"window.ECalcPinoutPage || null", "kind === 'pinout'", "pinTypes: catalog.pin_types || [], wires: catalog.wires || []"} {
		if !strings.Contains(shell, marker) {
			t.Errorf("ecalc-shell.js is missing %q", marker)
		}
	}
	match := regexp.MustCompile(`const PINOUT_GROUPS = \[([^\]]*)\];`).FindStringSubmatch(shell)
	if match == nil {
		t.Fatal("ecalc-shell.js has no PINOUT_GROUPS")
	}
	var groups []string
	for _, quoted := range regexp.MustCompile(`'([a-z_]+)'`).FindAllStringSubmatch(match[1], -1) {
		groups = append(groups, quoted[1])
	}
	if !slices.Equal(groups, ecalc.PinoutGroups) {
		t.Errorf("PINOUT_GROUPS = %v, want ecalc.PinoutGroups = %v", groups, ecalc.PinoutGroups)
	}
}

// The drawing is a string builder like the code drawings: no document access, no colour in the code but the two fallbacks for
// a pin type the data does not know, and it draws exactly the outlines that the engine accepts.
func TestECalcPinoutDrawIsADomFreeStringBuilder(t *testing.T) {
	t.Parallel()
	draw := ecalcScripts(t)["ecalc-pinout-draw.js"]
	for _, marker := range []string{
		"window.ECalcPinoutDraw = { layout, svg, OUTLINES, fitNumber, safeColor, textWidth };", "function layout(shape, pins, options)", "function svg(spec)", "const safeColor =",
		`class="ec-pin"`, `class="ec-pin-hit"`, `'ec-pin-body'`, `'ec-pin-ring'`, `class="ec-pin-mark"`, `class="ec-pin-shell"`, `class="ec-pin-art"`, `class="ec-leader"`, `class="ec-wire-stub"`,
		`' fill="' + hex + '"'`, `data-type="`, `data-pin="`, `role="img"`, "translate(' + view.width + ' 0) scale(-1 1)",
	} {
		if !strings.Contains(draw, marker) {
			t.Errorf("ecalc-pinout-draw.js is missing %q", marker)
		}
	}
	body := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(draw, "")
	for label, pattern := range map[string]string{
		"document access": `\bdocument\b`,
		"window access":   `\bwindow\.(?:document|ECalcCore|localStorage)\b`,
		"DOM write":       `\b(?:innerHTML|createElement|appendChild|setAttribute)\b`,
		"colour function": `\b(?:rgba?|hsla?)\(`,
		"inline style":    `\bstyle=`,
		"text transform":  `<text[^>]*scale\(`,
	} {
		if match := regexp.MustCompile(pattern).FindString(body); match != "" {
			t.Errorf("ecalc-pinout-draw.js must not contain a %s (%q)", label, match)
		}
	}
	// Two fallback colours only; the HEX pattern of safeColor is a regular expression, not a colour.
	cleaned := regexp.MustCompile(`const HEX = [^\n]*`).ReplaceAllString(body, "")
	if got := regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`).FindAllString(cleaned, -1); !slices.Equal(got, []string{"#9e9e9e", "#000000"}) {
		t.Errorf("ecalc-pinout-draw.js holds the colours %v, want only the two fallbacks #9e9e9e and #000000", got)
	}
	list := regexp.MustCompile(`const OUTLINES = \[([^\]]*)\];`).FindStringSubmatch(body)
	if list == nil {
		t.Fatal("ecalc-pinout-draw.js has no OUTLINES list")
	}
	var outlines []string
	for _, quoted := range regexp.MustCompile(`'([a-z_0-9]+)'`).FindAllStringSubmatch(list[1], -1) {
		outlines = append(outlines, quoted[1])
	}
	if !slices.Equal(outlines, ecalc.PinoutOutlines) {
		t.Errorf("OUTLINES = %v, want ecalc.PinoutOutlines = %v", outlines, ecalc.PinoutOutlines)
	}
	for _, outline := range ecalc.PinoutOutlines {
		if !regexp.MustCompile(`(?m)^\s+`+outline+`\(`).MatchString(body) && !strings.Contains(body, outline+": {") {
			t.Errorf("ecalc-pinout-draw.js has no shell builder for the outline %s", outline)
		}
	}
}

// The page keeps the choices in the store under the item id, speaks only in desktop.ecalc_* keys, and never reaches into the
// theme or writes an inline style.
func TestECalcPinoutPageUsesTheStore(t *testing.T) {
	t.Parallel()
	page := ecalcScripts(t)["ecalc-pinout-page.js"]
	for _, marker := range []string{
		"window.ECalcPinoutPage = { create };", "store.saveCalc(item.id, { choices: { variant: st.variant, view: st.view } })", "store.calcState(item.id)", `aria-live="polite"`,
		"el.dataset.pinout = item.id;", "Draw.layout(shape, pins, { view: st.view })", "Draw.svg({", `class="ec-pin-rowbtn"`, "CSS.escape(", "window.ECalcCalcPage.copyText(plainText())", "store.flush()",
		"event.key === 'Escape'", "focusFilter()", "layout()", "refresh()", "dispose()",
	} {
		if !strings.Contains(page, marker) {
			t.Errorf("ecalc-pinout-page.js is missing %q", marker)
		}
	}
	body := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(page, "")
	for label, pattern := range map[string]string{
		"colour function": `\b(?:rgba?|hsla?)\(`,
		"console call":    `\bconsole\.`,
		"storage access":  `\b(?:localStorage|sessionStorage)\b`,
		"throw":           `\bthrow\b`,
		"inline style":    `\.style\.|\bstyle=`,
		"scrollIntoView":  `\bscrollIntoView\b`,
	} {
		if match := regexp.MustCompile(pattern).FindString(body); match != "" {
			t.Errorf("ecalc-pinout-page.js must not contain a %s (%q)", label, match)
		}
	}
	// Only the colour fallback of the swatch may be a hex colour in the page.
	if got := regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`).FindAllString(body, -1); !slices.Equal(got, []string{"#9e9e9e"}) {
		t.Errorf("ecalc-pinout-page.js holds the colours %v, want only the fallback #9e9e9e", got)
	}
	// Every pinout key (but the group names, which the shell composes) is used by the page, and the page uses no other.
	listed := map[string]bool{}
	for _, key := range ecalcPinoutKeys {
		listed[key] = true
		if !strings.HasPrefix(key, "desktop.ecalc_group_") && !strings.Contains(page, "'"+key+"'") {
			t.Errorf("ecalc-pinout-page.js never uses the key %s", key)
		}
	}
	for _, match := range regexp.MustCompile(`['"](desktop.ecalc_pin_[a-z0-9_]+)['"]`).FindAllStringSubmatch(page, -1) {
		if !listed[match[1]] {
			t.Errorf("ecalc-pinout-page.js uses the key %s, which ecalcPinoutKeys does not list", match[1])
		}
	}
}

// The stylesheet part of the pinouts is the last section of the file, after the code pages, and takes tokens only.
func TestECalcPinoutStylesheetSection(t *testing.T) {
	t.Parallel()
	css := readDesktopAssetText(t, "css/desktop-app-ecalc.css")
	start, end := strings.Index(css, "/* pinouts */"), strings.LastIndex(css, "/* end of pinouts */")
	if start < 0 || end < start {
		t.Fatal("css/desktop-app-ecalc.css has no /* pinouts */ ... /* end of pinouts */ section")
	}
	if codes := strings.LastIndex(css, "/* end of code pages */"); codes < 0 || codes > start {
		t.Error("the pinouts section follows the code pages section")
	}
	if strings.TrimSpace(css[end+len("/* end of pinouts */"):]) != "" {
		t.Error("the pinouts section is the last section of css/desktop-app-ecalc.css")
	}
	section := css[start:end]
	for _, marker := range []string{
		".ec-shell {", ".ec-shell-flange {", ".ec-shell-tongue {", ".ec-pin-body {", `.ec-pin[data-type="nc"] .ec-pin-body { stroke-dasharray`, `.ec-pin[data-selected="true"] .ec-pin-ring`,
		`.ec-pin[data-dim="true"]`, ".ec-pin-num-out { fill: var(--vd-text); }", `.ec-pinout-page[data-narrow="false"] .ec-pin-main`, ".ec-pin-art-host {", ".ec-pin-scroll {",
		".ec-pin-rowbtn", `.ec-pin-table tbody tr[data-selected="true"]`, "@media (prefers-reduced-motion: no-preference)",
	} {
		if !strings.Contains(section, marker) {
			t.Errorf("the pinouts section is missing %q", marker)
		}
	}
	// The fill of a contact and the ink of its number are attributes of the drawing, never a rule of the stylesheet.
	if regexp.MustCompile(`\.ec-pin-body\s*\{[^}]*\bfill\s*:`).MatchString(section) || regexp.MustCompile(`\.ec-pin-num\s*\{[^}]*\bfill\s*:`).MatchString(section) {
		t.Error("the stylesheet must not set the fill of a contact or of the number inside it: the colour comes from the data")
	}
}

// The fixture of the pinout tests covers every outline of rows, every shape type, the two-view and the variant cases, and is the
// same text that the node test and the browser test evaluate.
func TestECalcPinoutFixtureCoversEveryShape(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "scripts", "ecalc-pinout-fixtures.js"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for _, outline := range ecalc.PinoutOutlines {
		if !strings.Contains(source, "'"+outline+"'") {
			t.Errorf("the fixture has no item with the outline %s", outline)
		}
	}
	for _, kind := range []string{"type: 'circular'", "type: 'dip'", "type: 'to'", "type: 'jack'", "type: 'board'", "package: 'to92'", "package: 'to220'", "contact", "variants"} {
		if !strings.Contains(source, kind) {
			t.Errorf("the fixture never uses %s", kind)
		}
	}
	for _, contact := range []string{"'round'", "'square'", "'blade'"} {
		if !strings.Contains(source, contact) {
			t.Errorf("the fixture has no %s contact", contact)
		}
	}
}
