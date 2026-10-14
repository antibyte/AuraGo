package ui

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// cydDesignerMarkers are the public entry points and seam usages each designer module must keep.
var cydDesignerMarkers = map[string][]string{
	"cyd-creator-geometry.js":         {"P.geometry = {", "function snapMove(", "function resizeRect(", "function distribute("},
	"cyd-creator-model.js":            {"P.model = {", "function createStore(", "coalesceMs", "RESERVED_NAMES", "function applyOps("},
	"cyd-creator-render.js":           {"P.render = {", "function widget(spec, w, theme, assetUrl, opts)", "'image-rendering': 'pixelated'"},
	"cyd-creator-canvas.js":           {"P.designerCanvas = {", "cydc-device-frame", "dataset.ccWidgetId", "dataset.ccScreenId"},
	"cyd-creator-interact.js":         {"P.designerInteract = {", "function startPaletteDrag(", "function handleKey("},
	"cyd-creator-palette.js":          {"P.designerPalette = {", "'data-cc-palette': s.type"},
	"cyd-creator-inspector-fields.js": {"P.designerFields = {", "async function rasterizeSVG("},
	"cyd-creator-inspector.js":        {"P.designerInspector = {", "d.inst.uploadAsset", "d.inst.code.ensureHook"},
	"cyd-creator-screens.js":          {"P.designerScreens = {", "d.inst.confirm(", "d.SYSTEM_WIFI"},
	"cyd-creator-designer.js":         {"P.designer = { create", "mountCanvas(el)", "mountPalette(el)", "mountScreens(el)", "mountInspector(el)", "reveal(target)", "dispose()", "'layout:issues'", "'lock:changed'", "'readonly:changed'", "'designer:reveal'"},
}

// cydDesignerForbidden lists tokens a designer module must not contain: "*" applies to every module
// in cydDesignerScripts (also the ones later tasks add), a file name adds its own. The designer
// reaches the server only through the shell's seam (inst.uploadAsset, the layout store that the shell
// autosaves), never through P.api, inst.api or the layout saver, and builds DOM only from nodes and
// text, never from HTML strings.
var cydDesignerForbidden = map[string][]string{
	"*": {"P.api.", "inst.api.", "layoutSaver", "innerHTML", "insertAdjacentHTML"},
}

// TestCYDCreatorDesignerStaticContracts checks each designer module (one subtest per file) for its
// entry points and the repository UI rules: no native dialogs, none of the forbidden tokens and no
// theme colour literals in the renderer, canvas or interaction code. The css subtest checks the
// designer section of the app stylesheet: the self-hosted preview fonts, the shell's tokens, the
// stage metrics fitZoom relies on and the narrow-window layout of Design mode.
func TestCYDCreatorDesignerStaticContracts(t *testing.T) {
	native := regexp.MustCompile(`\b(alert|confirm|prompt)\s*\(`)
	hex := regexp.MustCompile(`#[0-9a-fA-F]{6}\b`)
	for _, name := range cydDesignerScripts {
		t.Run(name, func(t *testing.T) {
			text := readDesktopAssetText(t, "js/desktop/apps/"+name)
			for _, marker := range cydDesignerMarkers[name] {
				if !strings.Contains(text, marker) {
					t.Errorf("%s misses %q", name, marker)
				}
			}
			for _, token := range append(append([]string{}, cydDesignerForbidden["*"]...), cydDesignerForbidden[name]...) {
				if strings.Contains(text, token) {
					t.Errorf("%s contains the forbidden %q", name, token)
				}
			}
			for _, m := range native.FindAllStringIndex(text, -1) {
				if !strings.HasSuffix(text[:m[0]], "inst.") {
					t.Errorf("%s calls a native dialog: %q", name, text[m[0]:m[1]])
				}
			}
			if name == "cyd-creator-render.js" || name == "cyd-creator-canvas.js" || name == "cyd-creator-interact.js" {
				if found := hex.FindString(text); found != "" {
					t.Errorf("%s hard-codes the colour %s; device colours come from the API themes", name, found)
				}
			}
		})
	}
	t.Run("css", func(t *testing.T) {
		css := readDesktopAssetText(t, "css/desktop-app-cyd-creator.css")
		for _, marker := range []string{"CYD Creator designer (plan 4b) — begin", "CYD Creator designer (plan 4b) — end", "font-family: 'CYD Montserrat'", "font-family: 'CYD JetBrains Mono'", ".cydc-device-frame", ".cydc-handle-se", "@media (prefers-reduced-motion: reduce)"} {
			if !strings.Contains(css, marker) {
				t.Errorf("desktop-app-cyd-creator.css misses %q", marker)
			}
		}
		begin := strings.Index(css, "/* ==== CYD Creator designer (plan 4b) — begin")
		end := strings.Index(css, "/* ==== CYD Creator designer (plan 4b) — end")
		if begin < 0 || end < begin {
			t.Fatal("the designer section must sit between the delimiter comments \"CYD Creator designer (plan 4b) — begin\" and \"— end\"")
		}
		section := css[begin:end]
		checkCYDDesignerSectionTokens(t, css, section)
		checkCYDDesignerScope(t, section)
		checkCYDDesignerStageMetrics(t, section)
		// Screen row tools: showing them (hover, focus) must not move anything, or a press that
		// takes the pointer or the focus out of a row shifts the rows below between press and
		// release. Rows other than the active one stay on one line, and the rules that show their
		// tools only change visibility.
		oneLine := false
		for _, rule := range cydCSSRules(section) {
			if cydTargets(rule, "cydc-screen-row") && strings.Contains(rule.selector, ":not(.is-active)") {
				for _, d := range cydDeclarations(rule.body) {
					oneLine = oneLine || (d[0] == "flex-wrap" && d[1] == "nowrap")
				}
			}
			if !cydTargets(rule, "cydc-row-tools") || !regexp.MustCompile(`:hover|:focus`).MatchString(rule.selector) {
				continue
			}
			for _, d := range cydDeclarations(rule.body) {
				if d[0] != "visibility" {
					t.Errorf("%s { %s: %s }: row tools shown on hover/focus may only change visibility (a reflow moves the rows)", rule.selector, d[0], d[1])
				}
			}
		}
		if !oneLine {
			t.Error("screen rows other than the active one must stay on one line (flex-wrap: nowrap), so showing their tools keeps the row height")
		}
		// The dock keeps the height the user drags (cyd-creator-dock.js limits only its first,
		// unsaved height in Design mode): no designer rule caps or sets it.
		for _, rule := range cydCSSRules(section) {
			if !cydTargets(rule, "cydc-dock") {
				continue
			}
			for _, d := range cydDeclarations(rule.body) {
				if strings.Contains(d[0], "height") || strings.Contains(d[0], "block-size") {
					t.Errorf("%s { %s: %s }: the dock's height belongs to cyd-creator-dock.js (the user's resize)", rule.selector, d[0], d[1])
				}
			}
		}
		// Narrow windows: the shell hides .cydc-left at ≤ 820 px; Design mode must bring screens,
		// layers and palette back (Code mode keeps the shell layout).
		for _, marker := range []string{"@container cydc-app (max-width: 820px)", `.cydc-app:not(.no-designer):not([data-cc-mode="code"]) .cydc-left { display: grid;`} {
			if !strings.Contains(section, marker) {
				t.Errorf("the designer section misses %q", marker)
			}
		}
	})
}

// cydCSSRule is one innermost rule of a stylesheet: its selector list (an at-rule's prelude is not
// part of it) and its declarations.
type cydCSSRule struct{ selector, body string }

// cydCSSRules returns the innermost rules of css with comments removed: the rules inside @media,
// @container and @supports blocks, @font-face, and the keyframe selectors of @keyframes.
func cydCSSRules(css string) []cydCSSRule {
	clean := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(css, "")
	var out []cydCSSRule
	for _, m := range regexp.MustCompile(`([^{}]*)\{([^{}]*)\}`).FindAllStringSubmatch(clean, -1) {
		out = append(out, cydCSSRule{strings.TrimSpace(m[1]), m[2]})
	}
	return out
}

// cydSelectors splits a selector list at its top-level commas (not inside :is(…) or […]).
func cydSelectors(list string) []string {
	var out []string
	depth, start := 0, 0
	for i, r := range list {
		switch r {
		case '(', '[':
			depth++
		case ')', ']':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(list[start:i]))
				start = i + 1
			}
		}
	}
	return append(out, strings.TrimSpace(list[start:]))
}

// cydDeclarations returns the property → value pairs of a rule body in order (a repeated property
// keeps its last value, like the cascade inside one rule).
func cydDeclarations(body string) [][2]string {
	var out [][2]string
	for _, decl := range strings.Split(body, ";") {
		name, value, ok := strings.Cut(decl, ":")
		if ok {
			out = append(out, [2]string{strings.TrimSpace(name), strings.TrimSpace(value)})
		}
	}
	return out
}

// cydNamedColour matches CSS named colours in a declaration value. System colours of forced-colors
// mode (Canvas, CanvasText, Highlight, HighlightText, GrayText, …) and transparent / currentColor
// stay allowed.
var cydNamedColour = regexp.MustCompile(`(?i)\b(white|black|red|green|blue|yellow|orange|purple|pink|gr[ae]y|silver|maroon|navy|teal|lime|aqua|cyan|magenta|fuchsia|olive|brown|gold|indigo|violet|coral|salmon|crimson|tomato|khaki|beige|ivory|lavender|turquoise|tan|chocolate|orchid|plum|azure|snow|wheat|linen)\b`)

// checkCYDDesignerSectionTokens holds the designer section to the shell's token rules: literal
// colours (hex, colour functions, named colours) only in its own token block (the first rule),
// chrome colours only from --cc-* tokens the stylesheet defines (plus the inline --cc-grid and
// --cc-depth the canvas and layers set), and none of the plan's former --ccd-* re-derivations of
// the desktop theme.
func checkCYDDesignerSectionTokens(t *testing.T, css, section string) {
	t.Helper()
	if strings.Contains(css, "--ccd-") {
		t.Error("desktop-app-cyd-creator.css uses --ccd-* tokens; the designer reads the shell's --cc-* tokens")
	}
	open := strings.Index(section, ".cydc-app, .cydc-drag-ghost {")
	if open < 0 {
		t.Fatal("the designer section has no token block for .cydc-app and the drag ghost")
	}
	head := section[:open]
	rules := section[open:]
	rules = rules[strings.Index(rules, "}")+1:]
	for name, text := range map[string]string{"before its token block": head, "after its token block": rules} {
		if hex := regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`).FindString(text); hex != "" {
			t.Errorf("literal colour %s in the designer section %s", hex, name)
		}
		if fn := regexp.MustCompile(`\b(?:rgba?|hsla?)\(`).FindString(text); fn != "" {
			t.Errorf("literal colour function %s…) in the designer section %s", fn, name)
		}
		for _, rule := range cydCSSRules(text) {
			for _, d := range cydDeclarations(rule.body) {
				if named := cydNamedColour.FindString(d[1]); named != "" {
					t.Errorf("named colour %q in the designer section %s (%s { %s: %s })", named, name, rule.selector, d[0], d[1])
				}
			}
		}
	}
	defined := map[string]bool{"--cc-grid": true, "--cc-depth": true}
	for _, m := range regexp.MustCompile(`(--cc-[a-z0-9-]+)\s*:`).FindAllStringSubmatch(css, -1) {
		defined[m[1]] = true
	}
	for _, m := range regexp.MustCompile(`var\((--[a-z0-9-]+)`).FindAllStringSubmatch(section, -1) {
		if !defined[m[1]] {
			t.Errorf("the designer section reads %s, which is no --cc-* token of desktop-app-cyd-creator.css", m[1])
		}
	}
}

// checkCYDDesignerScope keeps every designer rule inside the CYD Creator window: each selector
// starts at .cydc-app, or at the palette drag ghost that hangs in document.body (@font-face and
// keyframe selectors aside).
func checkCYDDesignerScope(t *testing.T, section string) {
	t.Helper()
	scoped := regexp.MustCompile(`^\.cydc-(?:app|drag-ghost(?:-inner)?)(?:$|[\s.:\[>+~])`)
	for _, rule := range cydCSSRules(section) {
		if strings.HasPrefix(rule.selector, "@") || rule.selector == "from" || rule.selector == "to" || strings.HasSuffix(rule.selector, "%") {
			continue
		}
		for _, sel := range cydSelectors(rule.selector) {
			if !scoped.MatchString(sel) {
				t.Errorf("designer selector %q is not scoped to .cydc-app (or the drag ghost)", sel)
			}
		}
	}
}

// cydTargets reports whether one of the rule's selectors has class (e.g. "cydc-canvas-stage") in its
// subject, the last compound selector.
func cydTargets(rule cydCSSRule, class string) bool {
	subject := regexp.MustCompile(`\.` + regexp.QuoteMeta(class) + `(?:$|[^-\w])`)
	for _, sel := range cydSelectors(rule.selector) {
		parts := regexp.MustCompile(`[\s>+~]+`).Split(strings.TrimSpace(sel), -1)
		if subject.MatchString(parts[len(parts)-1]) {
			return true
		}
	}
	return false
}

// checkCYDDesignerStageMetrics pins the canvas geometry that fitZoom in cyd-creator-canvas.js
// subtracts, in every rule of the section: the stage padding is PAD, the device frame's padding is
// BEZEL, and the device label's margin-top + min-height is LABEL_H.
func checkCYDDesignerStageMetrics(t *testing.T, section string) {
	t.Helper()
	canvas := readDesktopAssetText(t, "js/desktop/apps/cyd-creator-canvas.js")
	constant := func(name string) string {
		m := regexp.MustCompile(`const ` + name + ` = (\d+);`).FindStringSubmatch(canvas)
		if m == nil {
			t.Fatalf("cyd-creator-canvas.js has no const %s", name)
		}
		return m[1]
	}
	rules := cydCSSRules(section)
	for _, pin := range []struct{ constant, class string }{{"PAD", "cydc-canvas-stage"}, {"BEZEL", "cydc-device-frame"}} {
		want := constant(pin.constant) + "px"
		found := false
		for _, rule := range rules {
			if !cydTargets(rule, pin.class) {
				continue
			}
			for _, d := range cydDeclarations(rule.body) {
				if !strings.HasPrefix(d[0], "padding") {
					continue
				}
				found = true
				if d[0] != "padding" || d[1] != want {
					t.Errorf("%s { %s: %s } must be padding: %s (cyd-creator-canvas.js %s, used by fitZoom)", rule.selector, d[0], d[1], want, pin.constant)
				}
			}
		}
		if !found {
			t.Errorf("no designer rule gives .%s its padding: %s (cyd-creator-canvas.js %s)", pin.class, want, pin.constant)
		}
	}
	px := regexp.MustCompile(`^(\d+)px$`)
	label := map[string]int{}
	for _, rule := range rules {
		if !cydTargets(rule, "cydc-device-label") {
			continue
		}
		for _, d := range cydDeclarations(rule.body) {
			switch {
			case d[0] == "margin-top" || d[0] == "min-height":
				m := px.FindStringSubmatch(d[1])
				if m == nil {
					t.Errorf("%s { %s: %s } must be whole px (LABEL_H)", rule.selector, d[0], d[1])
					continue
				}
				n, _ := strconv.Atoi(m[1])
				label[d[0]] = n
			case d[0] == "margin" || strings.HasPrefix(d[0], "margin-block") || d[0] == "height" || d[0] == "max-height" || d[0] == "block-size":
				t.Errorf("%s { %s: %s }: the device label's height is margin-top + min-height (LABEL_H)", rule.selector, d[0], d[1])
			}
		}
	}
	if got, want := label["margin-top"]+label["min-height"], constant("LABEL_H"); strconv.Itoa(got) != want {
		t.Errorf(".cydc-device-label margin-top %dpx + min-height %dpx = %d, cyd-creator-canvas.js LABEL_H = %s (used by fitZoom)", label["margin-top"], label["min-height"], got, want)
	}
}

// TestCYDCreatorDesignerModulesLoadInOrder pins the designer scripts in DESKTOP_APP_ASSETS between
// cyd-creator-events.js and cyd-creator-code.js.
func TestCYDCreatorDesignerModulesLoadInOrder(t *testing.T) {
	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	entry := strings.Index(loader, "'cyd-creator': {")
	if entry < 0 {
		t.Fatal("module-loader.js has no cyd-creator entry")
	}
	end := strings.Index(loader[entry:], "]")
	block := loader[entry : entry+end]
	order := append([]string{"cyd-creator-events.js"}, cydDesignerScripts...)
	order = append(order, "cyd-creator-code.js")
	last := -1
	for _, name := range order {
		at := strings.Index(block, "'/js/desktop/apps/"+name+"'")
		if at < 0 {
			t.Fatalf("cyd-creator assets miss %s", name)
		}
		if at < last {
			t.Fatalf("cyd-creator assets load %s out of order", name)
		}
		last = at
	}
}
