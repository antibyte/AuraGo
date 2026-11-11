package ui

import (
	"math"
	"regexp"
	"strings"
	"testing"
)

// S0.5 (UI-03): category colour marks identity, never state. These tests read the tile and icon pairs of the three
// bands in desktop-app-easydrag.css and the status and accent tokens of every theme context.

var easyDragS0Categories = []string{"trigger", "logic", "ai", "web", "documents", "notify", "smart_home", "planner", "tool"}

var easyDragS0StatusTokens = []string{"--ds-color-success-fg", "--ds-color-warning-fg", "--ds-color-danger-fg", "--ds-color-info-fg"}

// easyDragS0GroupTokens render the stored group colour names (model.js:31, groups.go:42).
var easyDragS0GroupTokens = map[string]string{
	"--ed-group-blue": "var(--ed-accent)", "--ed-group-green": "var(--ed-cat-web)", "--ed-group-amber": "var(--ed-cat-smart_home)",
	"--ed-group-rose": "var(--ed-cat-notify)", "--ed-group-violet": "var(--ed-cat-trigger)", "--ed-group-neutral": "var(--ed-muted)",
}

const (
	easyDragS0DarkBand      = ".ed-app"
	easyDragS0LightBand     = `.desktop-body[data-theme="lollipop"] .ed-app`
	easyDragS0MinimalBand   = `.desktop-body[data-theme="minimal"]:not([data-scheme="dark"]) .ed-app`
	easyDragS0CyberpunkBand = `.desktop-body[data-theme="cyberpunk"] .ed-app`
	easyDragS0BlueFallback  = "var(--ed-cat-planner)"
)

var (
	easyDragS0StatusRead   = regexp.MustCompile(`--ed-(ok|warn|danger|info)\b|--ds-color-(success|warning|danger|info)-`)
	easyDragS0AccentPreset = regexp.MustCompile(`^\.desktop-body\[data-accent="([a-z]+)"\]$`)
	easyDragS0CatRead      = regexp.MustCompile(`var\(\s*--ed-cat(?:-[a-z_]+)*\s*[,)]`)
	easyDragS0RefCat       = regexp.MustCompile(`^\.ed-ref\.cat-[a-z_]+$`)
)

// easyDragS0CatIdentity are the identity marks that may carry category colour (spec §8).
var easyDragS0CatIdentity = map[string]bool{
	".ed-node-tile": true, ".ed-tile": true, ".ed-palette-tile": true, ".ed-detail-tile": true, ".ed-tree-tile": true,
	".ed-palette-cat-dot": true, ".ed-cat-dot": true, ".ed-flow-preview rect": true, ".ed-minimap-nodes rect": true, ".ed-ref": true,
}

type easyDragS0RuleRef struct{ File, Selector string }

type easyDragS0ThemeContext struct {
	Name       string
	Band       string              // selector of the band rule that colours this context
	Scope      string              // selector a theme-scoped ".ed-app" rule uses for this context
	Chain      []easyDragS0RuleRef // where its status and accent tokens come from, most specific first
	UserAccent bool
}

type easyDragS0Band struct{ Tile, Ink map[string][3]float64 }

// easyDragOKLab converts an sRGB colour (channels 0-255) to OKLab (L, a, b).
func easyDragOKLab(rgb [3]float64) [3]float64 {
	var lin [3]float64
	for i, channel := range rgb {
		c := channel / 255
		if c <= 0.04045 {
			lin[i] = c / 12.92
		} else {
			lin[i] = math.Pow((c+0.055)/1.055, 2.4)
		}
	}
	l := math.Cbrt(0.4122214708*lin[0] + 0.5363325363*lin[1] + 0.0514459929*lin[2])
	m := math.Cbrt(0.2119034982*lin[0] + 0.6806995451*lin[1] + 0.1073969566*lin[2])
	s := math.Cbrt(0.0883024619*lin[0] + 0.2817188376*lin[1] + 0.6299787005*lin[2])
	return [3]float64{
		0.2104542553*l + 0.7936177850*m - 0.0040720468*s,
		1.9779984951*l - 2.4285922050*m + 0.4505937099*s,
		0.0259040371*l + 0.7827717662*m - 0.8086757660*s,
	}
}

// easyDragOKLCH returns OKLCH lightness, chroma and hue (degrees, 0-360).
func easyDragOKLCH(rgb [3]float64) (lightness, chroma, hue float64) {
	lab := easyDragOKLab(rgb)
	chroma = math.Hypot(lab[1], lab[2])
	hue = math.Atan2(lab[2], lab[1]) * 180 / math.Pi
	if hue < 0 {
		hue += 360
	}
	return lab[0], chroma, hue
}

func easyDragOKLabDistance(a, b [3]float64) float64 {
	x, y := easyDragOKLab(a), easyDragOKLab(b)
	return math.Sqrt((x[0]-y[0])*(x[0]-y[0]) + (x[1]-y[1])*(x[1]-y[1]) + (x[2]-y[2])*(x[2]-y[2]))
}

// easyDragHueGap is the angle between two hues, 0-180.
func easyDragHueGap(a, b float64) float64 {
	d := math.Mod(math.Abs(a-b), 360)
	if d > 180 {
		d = 360 - d
	}
	return d
}

func easyDragS0Sheet(t *testing.T, sheets map[string]cssSheet, file string) cssSheet {
	t.Helper()
	if sheet, ok := sheets[file]; ok {
		return sheet
	}
	sheet := parseCSSSheet(strings.ReplaceAll(readDesktopAssetText(t, file), "\r\n", "\n"))
	sheets[file] = sheet
	return sheet
}

// easyDragS0RuleDecls merges the declarations of every top-level rule that lists selector (later rules win).
func easyDragS0RuleDecls(sheet cssSheet, selector string) map[string]string {
	want := canonicalCSSSelector(collapseCSSSpace(selector))
	decls := map[string]string{}
	for _, rule := range sheet.Rules {
		if len(rule.Context) > 0 {
			continue
		}
		for _, part := range splitCSSSelectorList(rule.Selector) {
			if canonicalCSSSelector(part) != want {
				continue
			}
			for _, decl := range rule.Decls {
				decls[decl.Property] = cssDeclarationValue(decl.Value)
			}
			break
		}
	}
	return decls
}

func easyDragS0HexToken(t *testing.T, sheets map[string]cssSheet, ctx easyDragS0ThemeContext, token string) [3]float64 {
	t.Helper()
	for _, ref := range ctx.Chain {
		value, ok := easyDragS0RuleDecls(easyDragS0Sheet(t, sheets, ref.File), ref.Selector)[token]
		if !ok {
			continue
		}
		rgb, ok := parseCSSHexColor(value)
		if !ok {
			t.Fatalf("%s: %s in %s %s is %q, not a hex colour", ctx.Name, token, ref.File, ref.Selector, value)
		}
		return rgb
	}
	t.Fatalf("%s: no rule of %v declares %s", ctx.Name, ctx.Chain, token)
	return [3]float64{}
}

func easyDragS0ReadBand(t *testing.T, sheet cssSheet, selector string) easyDragS0Band {
	t.Helper()
	decls := easyDragS0RuleDecls(sheet, selector)
	band := easyDragS0Band{Tile: map[string][3]float64{}, Ink: map[string][3]float64{}}
	for _, cat := range easyDragS0Categories {
		for token, into := range map[string]map[string][3]float64{"--ed-cat-" + cat: band.Tile, "--ed-cat-" + cat + "-ink": band.Ink} {
			rgb, ok := parseCSSHexColor(decls[token])
			if !ok {
				t.Errorf("%s: %s must be a six-digit hex colour, got %q", selector, token, decls[token])
				continue
			}
			into[cat] = rgb
		}
	}
	return band
}

// easyDragS0ThemeContexts lists the 16 contexts: the 14 registry themes, Fruity and Minimal in both schemes.
func easyDragS0ThemeContexts(t *testing.T) []easyDragS0ThemeContext {
	t.Helper()
	root := easyDragS0RuleRef{File: "css/desktop-base.css", Selector: ":root"}
	var contexts []easyDragS0ThemeContext
	for _, theme := range parseDesktopThemeRegistry(t) {
		file := "css/desktop-theme-" + strings.ReplaceAll(theme.ID, "_", "-") + ".css"
		if desktopThemeBuiltInShellThemes[theme.ID] {
			file = "css/desktop-base.css"
		}
		scope := `.desktop-body[data-theme="` + theme.ID + `"]`
		own := easyDragS0RuleRef{File: file, Selector: scope}
		ctx := easyDragS0ThemeContext{Name: theme.ID, Band: easyDragS0DarkBand, Scope: scope, Chain: []easyDragS0RuleRef{own, root}, UserAccent: theme.Accent == "user"}
		switch theme.ID {
		case "cyberpunk":
			ctx.Band = easyDragS0CyberpunkBand
		case "lollipop":
			ctx.Band = easyDragS0LightBand
		case "fruity":
			light, dark := ctx, ctx
			light.Name, dark.Name = "fruity-light", "fruity-dark"
			dark.Chain = []easyDragS0RuleRef{{File: file, Selector: scope + `[data-fruity-mode="dark"]`}, own, root}
			contexts = append(contexts, light, dark)
			continue
		case "minimal":
			light, dark := ctx, ctx
			light.Name, light.Band, light.Scope = "minimal-light", easyDragS0MinimalBand, scope+`:not([data-scheme="dark"])`
			dark.Name, dark.Scope = "minimal-dark", scope+`[data-scheme="dark"]`
			dark.Chain = []easyDragS0RuleRef{{File: file, Selector: dark.Scope}, own, root}
			contexts = append(contexts, light, dark)
			continue
		}
		contexts = append(contexts, ctx)
	}
	return contexts
}

// TestEasyDragCategoryColourTokens checks the category tiles of every band against the status colours of every theme
// context (16): a coloured tile keeps 30° of hue from each status colour, every tile and group colour keeps 0.20 OKLab
// distance from each, every icon reaches 3:1 on its tile, and group "blue" is the accent only where the accent keeps
// that distance as well.
func TestEasyDragCategoryColourTokens(t *testing.T) {
	sheets := map[string]cssSheet{}
	ed := easyDragS0Sheet(t, sheets, "css/desktop-app-easydrag.css")
	bands := map[string]easyDragS0Band{}
	for _, selector := range []string{easyDragS0DarkBand, easyDragS0LightBand, easyDragS0MinimalBand, easyDragS0CyberpunkBand} {
		bands[selector] = easyDragS0ReadBand(t, ed, selector)
	}
	if t.Failed() {
		t.FailNow()
	}

	root := easyDragS0RuleDecls(ed, easyDragS0DarkBand)
	for token, want := range easyDragS0GroupTokens {
		if got := root[token]; got != want {
			t.Errorf(".ed-app %s = %q, want %q", token, got, want)
		}
	}
	fallback := map[string]bool{}
	for _, rule := range ed.Rules {
		for _, decl := range rule.Decls {
			if !strings.HasPrefix(decl.Property, "--ed-group") {
				continue
			}
			if easyDragS0StatusRead.MatchString(decl.Value) {
				t.Errorf("line %d: %s %s reads a status colour (%s); groups use identity colours", rule.Line, rule.Selector, decl.Property, decl.Value)
			}
			if decl.Property != "--ed-group-blue" || (rule.Selector == easyDragS0DarkBand && len(rule.Context) == 0) {
				continue
			}
			if cssDeclarationValue(decl.Value) != easyDragS0BlueFallback || len(rule.Context) > 0 {
				t.Errorf("line %d: %s may only set --ed-group-blue to %s", rule.Line, rule.Selector, easyDragS0BlueFallback)
				continue
			}
			for _, part := range splitCSSSelectorList(rule.Selector) {
				fallback[strings.TrimSuffix(canonicalCSSSelector(part), " .ed-app")] = true
			}
		}
	}

	var presets [][3]float64
	for _, rule := range easyDragS0Sheet(t, sheets, "css/desktop-base.css").Rules {
		if len(rule.Context) > 0 || !easyDragS0AccentPreset.MatchString(canonicalCSSSelector(rule.Selector)) {
			continue
		}
		for _, decl := range rule.Decls {
			if rgb, ok := parseCSSHexColor(cssDeclarationValue(decl.Value)); ok && decl.Property == "--vd-accent" {
				presets = append(presets, rgb)
			}
		}
	}
	if len(presets) == 0 {
		t.Fatal(`desktop-base.css declares no accent preset (.desktop-body[data-accent="..."])`)
	}

	contexts := easyDragS0ThemeContexts(t)
	if len(contexts) != 16 {
		t.Fatalf("got %d theme contexts, want 16 (14 themes, Fruity and Minimal in both schemes)", len(contexts))
	}
	scopes := map[string]bool{}
	for _, ctx := range contexts {
		scopes[ctx.Scope] = true
		band := bands[ctx.Band]
		status := map[string][3]float64{}
		for _, token := range easyDragS0StatusTokens {
			status[token] = easyDragS0HexToken(t, sheets, ctx, token)
		}
		nearest := func(rgb [3]float64) (float64, string) {
			best, near := math.Inf(1), ""
			for token, s := range status {
				if d := easyDragOKLabDistance(rgb, s); d < best {
					best, near = d, token
				}
			}
			return best, near
		}
		for _, cat := range easyDragS0Categories {
			tile, ink := band.Tile[cat], band.Ink[cat]
			if _, chroma, hue := easyDragOKLCH(tile); chroma >= 0.05 {
				for token, s := range status {
					if _, _, statusHue := easyDragOKLCH(s); easyDragHueGap(hue, statusHue) < 30 {
						t.Errorf("%s: the %s tile (hue %.0f) is %.0f° from %s (hue %.0f), want at least 30°", ctx.Name, cat, hue, easyDragHueGap(hue, statusHue), token, statusHue)
					}
				}
			}
			if d, near := nearest(tile); d < 0.20 {
				t.Errorf("%s: the %s tile is %.3f from %s in OKLab, want at least 0.20", ctx.Name, cat, d, near)
			}
			if ratio := cssContrastRatio(ink, tile); ratio < 3 {
				t.Errorf("%s: the %s icon on its tile is %.2f:1, want at least 3:1", ctx.Name, cat, ratio)
			}
		}
		accents := [][3]float64{easyDragS0HexToken(t, sheets, ctx, "--vd-accent")}
		if ctx.UserAccent {
			accents = append(accents, presets...)
		}
		worst, worstNear := math.Inf(1), ""
		for _, accent := range accents {
			if d, near := nearest(accent); d < worst {
				worst, worstNear = d, near
			}
		}
		switch need := worst < 0.20; {
		case need && !fallback[ctx.Scope]:
			t.Errorf("%s: an accent comes within %.3f of %s; list %q in the --ed-group-blue fallback rule", ctx.Name, worst, worstNear, ctx.Scope+" .ed-app")
		case !need && fallback[ctx.Scope]:
			t.Errorf("%s: every accent keeps %.3f from the status colours; remove %q from the --ed-group-blue fallback rule", ctx.Name, worst, ctx.Scope+" .ed-app")
		}
		groups := map[string][3]float64{"green": band.Tile["web"], "amber": band.Tile["smart_home"], "rose": band.Tile["notify"], "violet": band.Tile["trigger"], "blue": accents[0]}
		if fallback[ctx.Scope] {
			groups["blue"] = band.Tile["planner"]
		}
		for name, rgb := range groups {
			if d, near := nearest(rgb); d < 0.20 {
				t.Errorf("%s: group %s is %.3f from %s in OKLab, want at least 0.20", ctx.Name, name, d, near)
			}
		}
	}
	for scope := range fallback {
		if !scopes[scope] {
			t.Errorf("the --ed-group-blue fallback names %q, which is no theme context", scope)
		}
	}
}

func easyDragS0CatReadAllowed(part string) bool {
	part = canonicalCSSSelector(collapseCSSSpace(part))
	switch {
	case part == ".ed-app", strings.HasPrefix(part, ".ed-app [data-cat"):
		return true
	case strings.HasPrefix(part, `.desktop-body[data-theme="`) && strings.HasSuffix(part, " .ed-app"):
		return true
	case easyDragS0CatIdentity[part], easyDragS0RefCat.MatchString(part):
		return true
	}
	return false
}

// checkEasyDragCategoryReads fails every declaration that reads --ed-cat, --ed-cat-ink or --ed-cat-<name> outside the
// token and band blocks, the [data-cat] aliases and the identity marks.
func checkEasyDragCategoryReads(t *testing.T, css string) {
	t.Helper()
	sheet := parseCSSSheet(strings.ReplaceAll(css, "\r\n", "\n"))
	if len(sheet.Problems) > 0 {
		t.Fatalf("desktop-app-easydrag.css does not parse: %v", sheet.Problems)
	}
	for _, rule := range sheet.Rules {
		for _, decl := range rule.Decls {
			if !easyDragS0CatRead.MatchString(decl.Value) {
				continue
			}
			for _, part := range splitCSSSelectorList(rule.Selector) {
				if !easyDragS0CatReadAllowed(part) {
					t.Errorf("line %d: %s reads a category colour in %s: %s; category colour marks identity only (tiles, dots, previews, reference chips)", rule.Line, part, decl.Property, decl.Value)
				}
			}
		}
	}
}

// TestEasyDragStateMarksUseAccent pins the interaction and change marks to the accent and keeps category colour off
// cards and wires: the unpublished chip and the Publish dot are not warnings, the drag ghost is an interaction, cards
// carry no category stripe and wires no category attribute.
func TestEasyDragStateMarksUseAccent(t *testing.T) {
	sheet := parseCSSSheet(strings.ReplaceAll(readDesktopAssetText(t, "css/desktop-app-easydrag.css"), "\r\n", "\n"))
	want := map[string]map[string]string{
		".ed-chip--accent":                    {"background": "color-mix(in srgb, var(--ed-accent) 16%, transparent)", "color": "var(--ed-accent-ink)"},
		".ed-btn--primary.has-changes::after": {"background": "var(--ed-accent)"},
		".ed-drag-ghost.is-over-canvas":       {"border-color": "var(--ed-accent)"},
	}
	for selector, props := range want {
		decls := easyDragS0RuleDecls(sheet, selector)
		for prop, value := range props {
			if got := decls[prop]; got != value {
				t.Errorf("%s { %s } = %q, want %q", selector, prop, got, value)
			}
		}
	}
	for _, rule := range sheet.Rules {
		for _, part := range splitCSSSelectorList(rule.Selector) {
			if strings.HasPrefix(part, ".ed-node") && strings.HasSuffix(part, "::before") {
				t.Errorf("line %d: %s draws a card stripe; a card shows its category only on its tile", rule.Line, part)
			}
		}
	}
	if strings.Contains(readDesktopAssetText(t, "js/desktop/apps/easydrag-wires.js"), "g.dataset.cat") {
		t.Error("easydrag-wires.js marks wires with a category (g.dataset.cat); wires show state only")
	}
}
