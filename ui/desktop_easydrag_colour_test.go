package ui

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// S0.5 (UI-03): category colour marks identity, never state. These tests read the tile, mark and group tokens of
// desktop-app-easydrag.css, the status and accent tokens of every theme context and the surfaces of every desktop
// material (desktop-materials*.css), and check them against each other (spec §8).

var easyDragS0Categories = []string{"trigger", "logic", "ai", "web", "documents", "notify", "smart_home", "planner", "tool"}

var easyDragS0StatusTokens = []string{"--ds-color-success-fg", "--ds-color-warning-fg", "--ds-color-danger-fg", "--ds-color-info-fg"}

// easyDragS0GroupNames are the stored group colour names (model.js:31, groups.go:42) that render as a colour; "neutral"
// is --ed-muted.
var easyDragS0GroupNames = []string{"blue", "green", "amber", "rose", "violet"}

// easyDragS0GroupWindows are the OKLCH hue windows (degrees, lo to hi, wrapping through 0 when lo > hi) in which a group
// colour still reads as its name.
var easyDragS0GroupWindows = map[string][2]float64{
	"blue": {225, 275}, "green": {130, 170}, "amber": {50, 90}, "rose": {345, 25}, "violet": {285, 320},
}

// easyDragS0GroupWidened widens one window in one context by the smallest step that keeps it clear of the status colours
// ("<context>/<group>" to the window). Each entry is reported in the commit that adds it.
var easyDragS0GroupWidened = map[string][2]float64{}

// easyDragS0MarkSoft are the marks that may stay below 3:1 on a surface (but not below 2:1): they are grey on purpose.
var easyDragS0MarkSoft = map[string]bool{"logic": true, "tool": true}

const (
	easyDragS0DarkBand      = ".ed-app"
	easyDragS0GroupMinHue   = 0.08 // OKLCH chroma a group colour needs to read as a colour
	easyDragS0StatusMin     = 0.20 // OKLab distance of a group colour or the change mark from the status colours
	easyDragS0GroupPairMin  = 0.10 // OKLab distance between two group colours
	easyDragS0MarkHueGapMin = 30.0 // hue gap of a mark from every status hue
)

var (
	easyDragS0StatusRead   = regexp.MustCompile(`--ed-(ok|warn|danger|info)\b|--ds-color-(success|warning|danger|info)-`)
	easyDragS0AccentPreset = regexp.MustCompile(`^\.desktop-body\[data-accent="([a-z]+)"\]$`)
	easyDragS0MarkMix      = regexp.MustCompile(`^color-mix\(in srgb, var\((--ed-cat-[a-z_]+)\) (\d+)%, (black|var\(--ed-cat-[a-z_]+-ink\))\)$`)
	easyDragS0VarName      = regexp.MustCompile(`^var\((--[a-z_-]+)\)$`)
)

// category token reads: --ed-cat, --ed-cat-ink, --ed-cat-<name>, --ed-mark, --ed-mark-<name>.
var (
	easyDragS0CatRead      = regexp.MustCompile(`var\(\s*--ed-(?:cat|mark)(?:-[a-z_]+)*\s*[,)]`)
	easyDragS0GroupRead    = regexp.MustCompile(`var\(\s*--ed-group-(?:blue|green|amber|rose|violet|neutral)\s*[,)]`)
	easyDragS0CatProp      = regexp.MustCompile(`^--ed-(?:cat|mark)(?:-[a-z_]+)*$`)
	easyDragS0CatAliasSel  = regexp.MustCompile(`^\.ed-app \[data-cat(?:="[a-z_]+")?\]$`)
	easyDragS0BlockSel     = regexp.MustCompile(`^(?:\.ed-app|\.desktop-body\[data-theme="[a-z_]+"\](?::not\(\[data-scheme="dark"\]\)|\[data-scheme="dark"\])? \.ed-app)$`)
	easyDragS0RefCat       = regexp.MustCompile(`^\.ed-ref\.cat-[a-z_]+$`)
	easyDragS0GroupReadSel = regexp.MustCompile(`^\.ed-app \.ed-group(?:--(?:blue|green|amber|rose|violet)|-frame|-head)$`)
)

// easyDragS0CatIdentity are the identity marks that may carry category colour (spec §8): tiles, dots, previews, rects.
var easyDragS0CatIdentity = map[string]bool{
	".ed-node-tile": true, ".ed-tile": true, ".ed-palette-tile": true, ".ed-detail-tile": true, ".ed-tree-tile": true,
	".ed-palette-cat-dot": true, ".ed-cat-dot": true, ".ed-flow-preview rect": true, ".ed-minimap-nodes rect": true, ".ed-ref": true,
}

type easyDragS0RuleRef struct{ File, Selector string }

type easyDragS0ThemeContext struct {
	Name       string
	Theme      string              // the registry id
	ThemeScope string              // `.desktop-body[data-theme="<id>"]`
	Scope      string              // selector a theme-scoped ".ed-app" rule uses for this context
	Chain      []easyDragS0RuleRef // where its status and accent tokens come from, most specific first
	Mat        []easyDragS0RuleRef // where its surfaces (--vd-mat-*-color) come from, most specific first
	Variant    desktopMaterialVariant
	UserAccent bool
}

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

// easyDragHueInWindow reports whether hue lies in [lo, hi] (degrees), wrapping through 0 when lo > hi.
func easyDragHueInWindow(hue float64, window [2]float64) bool {
	if window[0] <= window[1] {
		return hue >= window[0] && hue <= window[1]
	}
	return hue >= window[0] || hue <= window[1]
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
			if canonicalCSSSelector(collapseCSSSpace(part)) != want {
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

// easyDragS0EdDecls is what the .ed-app rules say in one context: the root block, then the theme and scheme blocks.
func easyDragS0EdDecls(ed cssSheet, ctx easyDragS0ThemeContext) map[string]string {
	decls := map[string]string{}
	for _, selector := range []string{easyDragS0DarkBand, ctx.ThemeScope + " .ed-app", ctx.Scope + " .ed-app"} {
		for property, value := range easyDragS0RuleDecls(ed, selector) {
			decls[property] = value
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

// easyDragS0Surfaces reads the three opaque surfaces EasyDrag draws on in one context from the desktop material rules:
// the canvas (the app background), the card (cards, minimap) and the well (the editor canvas, the flow previews).
func easyDragS0Surfaces(t *testing.T, sheets map[string]cssSheet, ctx easyDragS0ThemeContext) map[string][3]float64 {
	t.Helper()
	merged := map[string]string{}
	for _, ref := range ctx.Mat {
		for property, value := range easyDragS0RuleDecls(easyDragS0Sheet(t, sheets, ref.File), ref.Selector) {
			if _, ok := merged[property]; !ok {
				merged[property] = value
			}
		}
	}
	surfaces := map[string][3]float64{}
	for name, token := range map[string]string{"canvas": "--vd-mat-canvas-color", "card": "--vd-mat-card-color", "well": "--vd-mat-well-color"} {
		rgb, ok := parseCSSHexColor(merged[token])
		if !ok {
			t.Fatalf("%s: %s in %v is %q, not a hex colour", ctx.Name, token, ctx.Mat, merged[token])
		}
		surfaces[name] = rgb
	}
	return surfaces
}

func easyDragS0MinContrast(rgb [3]float64, surfaces map[string][3]float64) (float64, string) {
	worst, name := math.Inf(1), ""
	for surface, bg := range surfaces {
		if ratio := cssContrastRatio(rgb, bg); ratio < worst {
			worst, name = ratio, surface
		}
	}
	return worst, name
}

// easyDragS0EvalMark computes the colour of a --ed-mark-<cat> value: a hex colour, or an sRGB mix of a tile with its ink
// or black, as color-mix(in srgb, ...) does it.
func easyDragS0EvalMark(value string, decls map[string]string) ([3]float64, bool) {
	value = strings.TrimSpace(value)
	if rgb, ok := parseCSSHexColor(value); ok {
		return rgb, true
	}
	match := easyDragS0MarkMix.FindStringSubmatch(value)
	if match == nil {
		return [3]float64{}, false
	}
	percent, _ := strconv.ParseFloat(match[2], 64)
	first, ok := parseCSSHexColor(decls[match[1]])
	if !ok {
		return [3]float64{}, false
	}
	second := [3]float64{}
	if match[3] != "black" {
		if second, ok = parseCSSHexColor(decls[easyDragS0VarName.FindStringSubmatch(match[3])[1]]); !ok {
			return [3]float64{}, false
		}
	}
	var mixed [3]float64
	for i := range mixed {
		mixed[i] = math.Round(first[i]*percent/100 + second[i]*(100-percent)/100)
	}
	return mixed, true
}

// easyDragS0ThemeContexts lists the 16 contexts: the 14 registry themes, Fruity and Minimal in both schemes.
func easyDragS0ThemeContexts(t *testing.T) []easyDragS0ThemeContext {
	t.Helper()
	root := easyDragS0RuleRef{File: "css/desktop-base.css", Selector: ":root"}
	themes := parseDesktopThemeRegistry(t)
	variants := map[string]desktopMaterialVariant{}
	for _, variant := range desktopMaterialVariants(themes) {
		variants[variant.Name] = variant
	}
	var contexts []easyDragS0ThemeContext
	for _, theme := range themes {
		file := "css/desktop-theme-" + strings.ReplaceAll(theme.ID, "_", "-") + ".css"
		if desktopThemeBuiltInShellThemes[theme.ID] {
			file = "css/desktop-base.css"
		}
		scope := `.desktop-body[data-theme="` + theme.ID + `"]`
		own := easyDragS0RuleRef{File: file, Selector: scope}
		matFile := desktopMaterialsPath(theme)
		ctx := easyDragS0ThemeContext{
			Name: theme.ID, Theme: theme.ID, ThemeScope: scope, Scope: scope, Chain: []easyDragS0RuleRef{own, root}, UserAccent: theme.Accent == "user",
			Mat: []easyDragS0RuleRef{{File: matFile, Selector: scope}, {File: desktopMaterialsSharedPath, Selector: ".desktop-body"}}, Variant: variants[theme.ID],
		}
		switch theme.ID {
		case "fruity":
			light, dark := ctx, ctx
			light.Name, light.Scope = "fruity-light", scope+`:not([data-scheme="dark"])`
			dark.Name, dark.Scope, dark.Variant = "fruity-dark", scope+`[data-scheme="dark"]`, variants["fruity-dark"]
			dark.Chain = []easyDragS0RuleRef{{File: file, Selector: scope + `[data-fruity-mode="dark"]`}, own, root}
			dark.Mat = append([]easyDragS0RuleRef{{File: matFile, Selector: dark.Scope}}, dark.Mat...)
			contexts = append(contexts, light, dark)
			continue
		case "minimal":
			light, dark := ctx, ctx
			light.Name, light.Scope = "minimal-light", scope+`:not([data-scheme="dark"])`
			dark.Name, dark.Scope, dark.Variant = "minimal-dark", scope+`[data-scheme="dark"]`, variants["minimal-dark"]
			dark.Chain = []easyDragS0RuleRef{{File: file, Selector: dark.Scope}, own, root}
			dark.Mat = append([]easyDragS0RuleRef{{File: matFile, Selector: dark.Scope}}, dark.Mat...)
			contexts = append(contexts, light, dark)
			continue
		}
		contexts = append(contexts, ctx)
	}
	return contexts
}

// easyDragS0ChangeValue is --ed-change in one context, following the cascade: the root block, the theme and scheme
// blocks, then (for the user-accent themes) the accent preset block. preset "" is the theme's own accent.
func easyDragS0ChangeValue(ed cssSheet, ctx easyDragS0ThemeContext, preset string) string {
	levels := map[string]int{easyDragS0DarkBand: 0, ctx.ThemeScope + " .ed-app": 1, ctx.Scope + " .ed-app": 2}
	if preset != "" {
		levels[ctx.ThemeScope+`[data-accent="`+preset+`"] .ed-app`] = 3
	}
	value, best := "", -1
	for _, rule := range ed.Rules {
		if len(rule.Context) > 0 {
			continue
		}
		for _, part := range splitCSSSelectorList(rule.Selector) {
			level, ok := levels[canonicalCSSSelector(collapseCSSSpace(part))]
			if !ok || level < best {
				continue
			}
			for _, decl := range rule.Decls {
				if decl.Property == "--ed-change" {
					value, best = cssDeclarationValue(decl.Value), level
				}
			}
		}
	}
	return value
}

// easyDragS0ScopedSelectors fails every rule that sets a property matching prop under a selector that is neither the
// root block nor a "<theme or scheme scope> .ed-app" block of a real context (or an accent preset block of one).
func easyDragS0ScopedSelectors(t *testing.T, ed cssSheet, contexts []easyDragS0ThemeContext, presets map[string][3]float64, prop *regexp.Regexp) {
	t.Helper()
	valid := map[string]bool{easyDragS0DarkBand: true}
	for _, ctx := range contexts {
		valid[ctx.ThemeScope+" .ed-app"], valid[ctx.Scope+" .ed-app"] = true, true
		if ctx.UserAccent {
			for preset := range presets {
				valid[ctx.ThemeScope+`[data-accent="`+preset+`"] .ed-app`] = true
			}
		}
	}
	for _, rule := range ed.Rules {
		for _, decl := range rule.Decls {
			if !prop.MatchString(decl.Property) {
				continue
			}
			for _, part := range splitCSSSelectorList(rule.Selector) {
				if !valid[canonicalCSSSelector(collapseCSSSpace(part))] {
					t.Errorf("line %d: %s sets %s but names no theme context (or the root block)", rule.Line, part, decl.Property)
				}
			}
		}
	}
}

// TestEasyDragCategoryColourTokens checks, in each of the 16 theme contexts:
//   - tiles: a coloured tile keeps 30 degrees of hue and 0.20 OKLab from each status colour, its icon reaches 3:1 on it;
//   - groups: each coloured group token is a dedicated hex, keeps 0.20 OKLab from every status colour and 0.10 from the
//     other group colours, sits in its name's hue window with chroma 0.08 and reaches 3:1 on the editor canvas (the well);
//   - marks: each --ed-mark-<cat> reaches 3:1 on the canvas, card and well (2:1 for the grey ones) and keeps 30 degrees
//     of hue from every status colour;
//   - change: --ed-change keeps 0.20 OKLab from success in every context and on every accent preset of Standard and
//     Fruity, and is the accent wherever the accent keeps that distance.
func TestEasyDragCategoryColourTokens(t *testing.T) {
	sheets := map[string]cssSheet{}
	ed := easyDragS0Sheet(t, sheets, "css/desktop-app-easydrag.css")

	root := easyDragS0RuleDecls(ed, easyDragS0DarkBand)
	if got := root["--ed-group-neutral"]; got != "var(--ed-muted)" {
		t.Errorf(".ed-app --ed-group-neutral = %q, want var(--ed-muted)", got)
	}
	for _, rule := range ed.Rules {
		for _, decl := range rule.Decls {
			if strings.HasPrefix(decl.Property, "--ed-group") && easyDragS0StatusRead.MatchString(decl.Value) {
				t.Errorf("line %d: %s %s reads a status colour (%s); groups use their own colours", rule.Line, rule.Selector, decl.Property, decl.Value)
			}
		}
	}

	presets := map[string][3]float64{}
	for _, rule := range easyDragS0Sheet(t, sheets, "css/desktop-base.css").Rules {
		if len(rule.Context) > 0 {
			continue
		}
		match := easyDragS0AccentPreset.FindStringSubmatch(canonicalCSSSelector(rule.Selector))
		if match == nil {
			continue
		}
		for _, decl := range rule.Decls {
			if rgb, ok := parseCSSHexColor(cssDeclarationValue(decl.Value)); ok && decl.Property == "--vd-accent" {
				presets[match[1]] = rgb
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
	easyDragS0ScopedSelectors(t, ed, contexts, presets, regexp.MustCompile(`^--ed-(?:group-(?:blue|green|amber|rose|violet)|mark-[a-z_]+|change)$`))

	seen := map[string]bool{}
	once := func(key, format string, args ...interface{}) {
		if !seen[key] {
			seen[key] = true
			t.Errorf(format, args...)
		}
	}
	for _, ctx := range contexts {
		decls := easyDragS0EdDecls(ed, ctx)
		surfaces := easyDragS0Surfaces(t, sheets, ctx)
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

		// tiles and icons
		for _, cat := range easyDragS0Categories {
			tile, ok1 := parseCSSHexColor(decls["--ed-cat-"+cat])
			ink, ok2 := parseCSSHexColor(decls["--ed-cat-"+cat+"-ink"])
			if !ok1 || !ok2 {
				t.Errorf("%s: --ed-cat-%s and its -ink must be six-digit hex colours, got %q and %q", ctx.Name, cat, decls["--ed-cat-"+cat], decls["--ed-cat-"+cat+"-ink"])
				continue
			}
			if _, chroma, hue := easyDragOKLCH(tile); chroma >= 0.05 {
				for token, s := range status {
					if _, _, statusHue := easyDragOKLCH(s); easyDragHueGap(hue, statusHue) < 30 {
						t.Errorf("%s: the %s tile (hue %.0f) is %.0f° from %s (hue %.0f), want at least 30°", ctx.Name, cat, hue, easyDragHueGap(hue, statusHue), token, statusHue)
					}
				}
			}
			if d, near := nearest(tile); d < easyDragS0StatusMin {
				t.Errorf("%s: the %s tile is %.3f from %s in OKLab, want at least 0.20", ctx.Name, cat, d, near)
			}
			if ratio := cssContrastRatio(ink, tile); ratio < 3 {
				t.Errorf("%s: the %s icon on its tile is %.2f:1, want at least 3:1", ctx.Name, cat, ratio)
			}
		}

		// groups
		groups := map[string][3]float64{}
		for _, name := range easyDragS0GroupNames {
			token := "--ed-group-" + name
			rgb, ok := parseCSSHexColor(decls[token])
			if !ok {
				once(ctx.Name+token, "%s: %s is %q; a coloured group token is a dedicated hex colour (no alias of a tile or the accent)", ctx.Name, token, decls[token])
				continue
			}
			groups[name] = rgb
			if d, near := nearest(rgb); d < easyDragS0StatusMin {
				t.Errorf("%s: group %s %s is %.3f from %s in OKLab, want at least 0.20", ctx.Name, name, decls[token], d, near)
			}
			window := easyDragS0GroupWindows[name]
			if widened, ok := easyDragS0GroupWidened[ctx.Name+"/"+name]; ok {
				window = widened
			}
			if _, chroma, hue := easyDragOKLCH(rgb); !easyDragHueInWindow(hue, window) || chroma < easyDragS0GroupMinHue {
				t.Errorf("%s: group %s %s has hue %.0f and chroma %.3f, want hue %.0f-%.0f and chroma at least %.2f", ctx.Name, name, decls[token], hue, chroma, window[0], window[1], easyDragS0GroupMinHue)
			}
			// a group frame is drawn on the editor canvas (.ed-canvas), which is the well surface
			if ratio := cssContrastRatio(rgb, surfaces["well"]); ratio < 3 {
				t.Errorf("%s: group %s %s is %.2f:1 on the editor canvas (the well), want at least 3:1", ctx.Name, name, decls[token], ratio)
			}
		}
		for i, a := range easyDragS0GroupNames {
			for _, b := range easyDragS0GroupNames[i+1:] {
				ga, okA := groups[a]
				gb, okB := groups[b]
				if okA && okB {
					if d := easyDragOKLabDistance(ga, gb); d < easyDragS0GroupPairMin {
						t.Errorf("%s: groups %s and %s are %.3f apart in OKLab, want at least %.2f", ctx.Name, a, b, d, easyDragS0GroupPairMin)
					}
				}
			}
		}

		// identity marks
		for _, cat := range easyDragS0Categories {
			token := "--ed-mark-" + cat
			value, present := decls[token]
			if !present {
				once(token, "%s is not declared in .ed-app; identity marks read a mark token (spec §8)", token)
				continue
			}
			mark, ok := easyDragS0EvalMark(value, decls)
			if !ok {
				t.Errorf("%s: %s = %q is neither a hex colour nor a color-mix of the tile with its ink or black", ctx.Name, token, value)
				continue
			}
			need := 3.0
			if easyDragS0MarkSoft[cat] {
				need = 2.0
			}
			if ratio, surface := easyDragS0MinContrast(mark, surfaces); ratio < need {
				t.Errorf("%s: %s %s is %.2f:1 on the %s, want at least %.0f:1", ctx.Name, token, value, ratio, surface, need)
			}
			if _, chroma, hue := easyDragOKLCH(mark); chroma >= 0.05 {
				for token2, s := range status {
					if _, _, statusHue := easyDragOKLCH(s); easyDragHueGap(hue, statusHue) < easyDragS0MarkHueGapMin {
						t.Errorf("%s: %s (hue %.0f) is %.0f° from %s (hue %.0f), want at least %.0f°", ctx.Name, token, hue, easyDragHueGap(hue, statusHue), token2, statusHue, easyDragS0MarkHueGapMin)
					}
				}
			}
		}

		// the change mark
		success := status["--ds-color-success-fg"]
		ink := desktopMaterialInk(t, ctx.Variant)
		accent := easyDragS0HexToken(t, sheets, ctx, "--vd-accent")
		neutral := map[string]string{"var(--ed-text)": "--vd-text", "var(--ed-muted)": "--vd-theme-muted"}
		contenders := map[string][3]float64{"": accent}
		if ctx.UserAccent {
			for name, rgb := range presets {
				contenders[name] = rgb
			}
		}
		for preset, accentRGB := range contenders {
			label := ctx.Name
			if preset != "" {
				label += " with the " + preset + " accent"
			}
			value := easyDragS0ChangeValue(ed, ctx, preset)
			colour := accentRGB
			switch token, isNeutral := neutral[value]; {
			case value == "var(--ed-accent)":
			case isNeutral:
				rgb, ok := desktopMaterialColour("var("+token+")", ink)
				if !ok {
					t.Fatalf("%s: cannot resolve %s for the neutral change mark", label, token)
				}
				colour = rgb
			default:
				once(label+value, "%s: --ed-change is %q, want var(--ed-accent), var(--ed-text) or var(--ed-muted)", label, value)
				continue
			}
			if d := easyDragOKLabDistance(colour, success); d < easyDragS0StatusMin {
				t.Errorf("%s: the change mark (%s) is %.3f from success in OKLab, want at least 0.20", label, value, d)
			}
			accentClose := easyDragOKLabDistance(accentRGB, success) < easyDragS0StatusMin
			if _, isNeutral := neutral[value]; !accentClose && isNeutral {
				t.Errorf("%s: the accent keeps %.3f from success, so --ed-change must stay var(--ed-accent), not %s", label, easyDragOKLabDistance(accentRGB, success), value)
			}
		}
	}
}

func easyDragS0RulePart(part string) string { return canonicalCSSSelector(collapseCSSSpace(part)) }

// easyDragCategoryReadProblems lists every declaration that reads a category token (--ed-cat*, --ed-mark*) or a group
// token (--ed-group-<name>) outside the places the spec allows:
//   - category tokens: the alias rules `.ed-app [data-cat]` and `.ed-app [data-cat="<name>"]` and the `.ed-app` token and
//     band blocks, both only into --ed-cat* and --ed-mark* custom properties, and the named identity-mark rules;
//   - group tokens: the `.ed-app .ed-group-*` rules, only into --ed-group-color.
func easyDragCategoryReadProblems(css string) []string {
	sheet := parseCSSSheet(strings.ReplaceAll(css, "\r\n", "\n"))
	if len(sheet.Problems) > 0 {
		return []string{fmt.Sprintf("desktop-app-easydrag.css does not parse: %v", sheet.Problems)}
	}
	var problems []string
	for _, rule := range sheet.Rules {
		for _, decl := range rule.Decls {
			catRead, groupRead := easyDragS0CatRead.MatchString(decl.Value), easyDragS0GroupRead.MatchString(decl.Value)
			for _, raw := range splitCSSSelectorList(rule.Selector) {
				part := easyDragS0RulePart(raw)
				if catRead {
					switch {
					case easyDragS0CatAliasSel.MatchString(part), easyDragS0BlockSel.MatchString(part):
						if !easyDragS0CatProp.MatchString(decl.Property) {
							problems = append(problems, fmt.Sprintf("line %d: %s aliases a category colour into %s: %s; token and alias rules may only set --ed-cat* and --ed-mark* properties", rule.Line, part, decl.Property, decl.Value))
						}
					case easyDragS0CatIdentity[part]:
						if strings.HasPrefix(decl.Property, "--") && !easyDragS0CatProp.MatchString(decl.Property) {
							problems = append(problems, fmt.Sprintf("line %d: %s copies a category colour into %s: %s", rule.Line, part, decl.Property, decl.Value))
						}
					case easyDragS0RefCat.MatchString(part):
						if decl.Property != "--ed-cat" {
							problems = append(problems, fmt.Sprintf("line %d: %s reads a category colour in %s: %s; a reference chip sets only --ed-cat", rule.Line, part, decl.Property, decl.Value))
						}
					default:
						problems = append(problems, fmt.Sprintf("line %d: %s reads a category colour in %s: %s; category colour marks identity only (tiles, dots, previews, reference chips)", rule.Line, part, decl.Property, decl.Value))
					}
				}
				if groupRead && (!easyDragS0GroupReadSel.MatchString(part) || decl.Property != "--ed-group-color") {
					problems = append(problems, fmt.Sprintf("line %d: %s reads a group colour in %s: %s; only the .ed-app .ed-group-* rules read it, into --ed-group-color", rule.Line, part, decl.Property, decl.Value))
				}
			}
		}
	}
	return problems
}

// checkEasyDragCategoryReads fails every category or group colour read outside its allowed places.
func checkEasyDragCategoryReads(t *testing.T, css string) {
	t.Helper()
	for _, problem := range easyDragCategoryReadProblems(css) {
		t.Error(problem)
	}
}

// TestEasyDragCategoryLintRejectsBypasses feeds the lint the rules that used to slip through, and checks that the real
// stylesheet stays clean.
func TestEasyDragCategoryLintRejectsBypasses(t *testing.T) {
	css := readDesktopAssetText(t, "css/desktop-app-easydrag.css")
	if problems := easyDragCategoryReadProblems(css); len(problems) > 0 {
		t.Fatalf("the stylesheet itself breaks the lint: %v", problems)
	}
	for name, extra := range map[string]string{
		"a state class on a data-cat selector": ".ed-app [data-cat].status-success { border-color: var(--ed-cat); }",
		"a custom property alias in .ed-app":   ".ed-app { --ed-leak: var(--ed-cat-web); }",
		"an alias in a band block":             `.desktop-body[data-theme="lollipop"] .ed-app { --x: var(--ed-mark-web); }`,
		"an alias in a data-cat rule":          `.ed-app [data-cat="web"] { --leak: var(--ed-cat-web); }`,
		"a mark read on a state chip":          ".ed-chip--ok { background: var(--ed-mark-web); }",
		"a group colour read anywhere":         ".ed-btn { color: var(--ed-group-blue); }",
	} {
		problems := easyDragCategoryReadProblems(css + "\n" + extra + "\n")
		if len(problems) != 1 {
			t.Errorf("%s: %q gave %d problems %v, want exactly 1", name, extra, len(problems), problems)
		}
	}
}

// TestEasyDragStateMarksUseChange pins the interaction and change marks: the unpublished chip and the Publish dot read
// --ed-change (the accent, or a neutral colour where the accent looks like success), the drag ghost is an interaction,
// cards carry no category stripe and wires no category attribute.
func TestEasyDragStateMarksUseChange(t *testing.T) {
	sheet := parseCSSSheet(strings.ReplaceAll(readDesktopAssetText(t, "css/desktop-app-easydrag.css"), "\r\n", "\n"))
	want := map[string]map[string]string{
		".ed-chip--accent":                    {"background": "color-mix(in srgb, var(--ed-change) 16%, transparent)", "color": "var(--ed-change-ink)"},
		".ed-btn--primary.has-changes::after": {"background": "var(--ed-change)"},
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
	for selector, props := range map[string]map[string]string{
		".ed-minimap-nodes rect": {"fill": "var(--ed-mark)"},
		".ed-flow-preview rect":  {"stroke": "var(--ed-mark)"},
		".ed-cat-dot":            {"background": "var(--ed-mark)"},
		".ed-palette-cat-dot":    {"background": "var(--ed-mark)"},
	} {
		decls := easyDragS0RuleDecls(sheet, selector)
		for prop, value := range props {
			if got := decls[prop]; got != value {
				t.Errorf("%s { %s } = %q, want %q (identity marks read the mark token, not the tile)", selector, prop, got, value)
			}
		}
	}
	if got := easyDragS0RuleDecls(sheet, easyDragS0DarkBand)["--ed-change-ink"]; got != "color-mix(in srgb, var(--ed-change) 30%, var(--ed-text))" {
		t.Errorf(".ed-app --ed-change-ink = %q, want a mix of --ed-change with the text colour", got)
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
