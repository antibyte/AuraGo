package ui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The app materials give every desktop theme its own look inside the apps: a set of --vd-mat-* custom properties per
// theme (bars, sides, cards, wells, fields, buttons, selection, type), defined on the body and read by the app
// stylesheets with their old value as fallback. Standard and Fruity (light and dark) live in css/desktop-materials.css
// next to the defaults, every other theme in css/desktop-materials-<id>.css. The rules are in
// docs/superpowers/specs/2026-10-31-desktop-app-materials-design.md; this file checks them statically.

// desktopMaterialsMaxLines bounds one material file.
const desktopMaterialsMaxLines = 260

// desktopMaterialsMaxDataURI bounds one inline SVG pattern (the spec asks for about 2 KB).
const desktopMaterialsMaxDataURI = 4096

// desktopMaterialsSharedPath holds the defaults plus the Standard and Fruity sets.
const desktopMaterialsSharedPath = "css/desktop-materials.css"

// desktopMaterialSurfaces have a base colour --vd-mat-<x>-color (opaque, contrast-tested) and a full background
// --vd-mat-<x> whose last layer is that colour.
var desktopMaterialSurfaces = []string{"canvas", "bar", "side", "card", "well", "field", "btn", "primary", "selected"}

// desktopMaterialTokens is the whole vocabulary. Every theme rule defines every token deliberately.
var desktopMaterialTokens = []string{
	"--vd-mat-canvas-color", "--vd-mat-canvas",
	"--vd-mat-bar-color", "--vd-mat-bar", "--vd-mat-bar-edge",
	"--vd-mat-side-color", "--vd-mat-side", "--vd-mat-side-edge",
	"--vd-mat-card-color", "--vd-mat-card", "--vd-mat-card-border", "--vd-mat-card-radius", "--vd-mat-card-shadow",
	"--vd-mat-well-color", "--vd-mat-well", "--vd-mat-well-shadow",
	"--vd-mat-field-color", "--vd-mat-field", "--vd-mat-field-border", "--vd-mat-field-radius", "--vd-mat-field-shadow",
	"--vd-mat-btn-color", "--vd-mat-btn", "--vd-mat-btn-hover", "--vd-mat-btn-border", "--vd-mat-btn-radius", "--vd-mat-btn-shadow", "--vd-mat-btn-text",
	"--vd-mat-primary-color", "--vd-mat-primary", "--vd-mat-primary-hover", "--vd-mat-primary-text", "--vd-mat-primary-border", "--vd-mat-primary-shadow",
	"--vd-mat-selected-color", "--vd-mat-selected", "--vd-mat-selected-text", "--vd-mat-selected-edge",
	"--vd-mat-hover",
	"--vd-mat-divider", "--vd-mat-rule",
	"--vd-mat-heading-font", "--vd-mat-heading-weight", "--vd-mat-heading-color", "--vd-mat-heading-tracking", "--vd-mat-heading-transform",
	"--vd-mat-label-font", "--vd-mat-label-color", "--vd-mat-label-tracking", "--vd-mat-label-transform",
	"--vd-mat-mono-font", "--vd-mat-accent-2", "--vd-mat-glow",
}

// desktopMaterialInkMaxLuminance bounds the relative luminance of the literal ink on a user accent lightened with white
// (about 0.05 is where dark text stops reaching 4.5:1 on a mid-light surface).
const desktopMaterialInkMaxLuminance = 0.05

// desktopMaterialContrastPairs are measured at 4.5:1. A side is "text" (--vd-text), "muted" (--vd-theme-muted) or a
// material token name.
var desktopMaterialContrastPairs = func() [][2]string {
	var pairs [][2]string
	for _, background := range []string{"canvas", "bar", "side", "card", "well", "field", "btn"} {
		pairs = append(pairs, [2]string{"text", "--vd-mat-" + background + "-color"})
	}
	for _, background := range []string{"canvas", "bar", "side", "card"} {
		pairs = append(pairs, [2]string{"muted", "--vd-mat-" + background + "-color"})
	}
	pairs = append(pairs,
		[2]string{"--vd-mat-btn-text", "--vd-mat-btn-color"},
		[2]string{"--vd-mat-primary-text", "--vd-mat-primary-color"},
		[2]string{"--vd-mat-selected-text", "--vd-mat-selected-color"},
	)
	for _, background := range []string{"canvas", "card"} {
		pairs = append(pairs,
			[2]string{"--vd-mat-heading-color", "--vd-mat-" + background + "-color"},
			[2]string{"--vd-mat-label-color", "--vd-mat-" + background + "-color"},
		)
	}
	return pairs
}()

var (
	desktopMaterialOpaqueColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	// desktopMaterialUserAccent matches a value built from the user's accent colour. In a user-accent theme such a
	// value cannot be measured statically; its text colour is var(--vd-accent-contrast), or, when the accent is lightened
	// with white, a literal dark ink.
	desktopMaterialUserAccent = regexp.MustCompile(`var\(\s*--vd-accent(?:-[rgb])?\s*[,)]`)
	// desktopMaterialAccentWhiteMix matches color-mix(in srgb, var(--vd-accent) N%, #fff): the accent lightened with white.
	desktopMaterialAccentWhiteMix = regexp.MustCompile(`(?i)color-mix\([^()]*var\(\s*--vd-accent\s*\)[^()]*,\s*(?:#fff|#ffffff|white)\s*\)`)
	desktopMaterialVarRef         = regexp.MustCompile(`^var\(\s*(--[\w-]+)\s*(?:,\s*(.+))?\)$`)
	desktopMaterialURLOpen        = regexp.MustCompile(`(?i)url\(`)
	desktopMaterialMotion         = regexp.MustCompile(`(?i)\b(animation|transition)\b`)
)

// desktopMaterialVariant is one token set: a theme, or the dark scheme of a theme with a scheme setting.
type desktopMaterialVariant struct {
	Name     string
	Theme    desktopThemeDescriptor
	Path     string // repository-relative to ui/
	Selector string // the plain rule that holds the set
	Dark     bool
}

func desktopMaterialsPath(theme desktopThemeDescriptor) string {
	if desktopThemeBuiltInShellThemes[theme.ID] {
		return desktopMaterialsSharedPath
	}
	return "css/desktop-materials-" + strings.ReplaceAll(theme.ID, "_", "-") + ".css"
}

func desktopMaterialVariants(themes []desktopThemeDescriptor) []desktopMaterialVariant {
	var variants []desktopMaterialVariant
	for _, theme := range themes {
		root := `.desktop-body[data-theme="` + theme.ID + `"]`
		variants = append(variants, desktopMaterialVariant{Name: theme.ID, Theme: theme, Path: desktopMaterialsPath(theme), Selector: root})
		if theme.SchemeSetting != "" {
			variants = append(variants, desktopMaterialVariant{Name: theme.ID + "-dark", Theme: theme, Path: desktopMaterialsPath(theme), Selector: root + `[data-scheme="dark"]`, Dark: true})
		}
	}
	return variants
}

// desktopMaterialPlainRule merges the plain rules (outside every at-rule) with this exact selector; a later
// declaration wins, as in the cascade.
func desktopMaterialPlainRule(sheet cssSheet, selector string) (tokens map[string]string, found bool) {
	tokens = map[string]string{}
	for _, rule := range sheet.Rules {
		if len(rule.Context) == 0 && canonicalCSSSelector(rule.Selector) == selector {
			found = true
			for _, decl := range rule.Decls {
				tokens[decl.Property] = cssDeclarationValue(decl.Value)
			}
		}
	}
	return tokens, found
}

// desktopMaterialInk collects the tokens the contrast pairs read from the skin: --vd-text, --vd-theme-muted and
// --vd-accent-contrast. Skins hold them on their root rule (Minimal's dark root rule on top for the dark scheme);
// Standard and Fruity keep them in desktop-base.css and desktop-shell-overrides.css.
func desktopMaterialInk(t testing.TB, variant desktopMaterialVariant) map[string]string {
	t.Helper()
	ink := map[string]string{}
	merge := func(paths []string, selector string) {
		for _, path := range paths {
			if tokens, found := desktopMaterialPlainRule(parseCSSSheet(readDesktopThemeSource(t, path)), selector); found {
				for property, value := range tokens {
					ink[property] = value
				}
			}
		}
	}
	root := `.desktop-body[data-theme="` + variant.Theme.ID + `"]`
	if desktopThemeBuiltInShellThemes[variant.Theme.ID] {
		paths := []string{"css/desktop-base.css", "css/desktop-shell-overrides.css"}
		merge(paths, ".desktop-body")
		merge(paths, root)
		if variant.Dark {
			merge(paths, root+`[data-fruity-mode="dark"]`)
		}
		return ink
	}
	skin := []string{desktopThemeSkinPath(variant.Theme)}
	merge(skin, root)
	if variant.Dark {
		merge(skin, root+`[data-scheme="dark"]`)
	}
	return ink
}

// desktopMaterialColour resolves a value to a hex colour, following var(--token, fallback) references through the
// lookups in order. It reports what it could not resolve.
func desktopMaterialColour(value string, lookups ...map[string]string) ([3]float64, bool) {
	for depth := 0; depth < 6; depth++ {
		value = strings.TrimSpace(value)
		if rgb, ok := parseCSSHexColor(value); ok {
			return rgb, true
		}
		ref := desktopMaterialVarRef.FindStringSubmatch(value)
		if ref == nil {
			return [3]float64{}, false
		}
		next, found := "", false
		for _, lookup := range lookups {
			if candidate, ok := lookup[ref[1]]; ok && candidate != "" {
				next, found = candidate, true
				break
			}
		}
		if !found {
			next = ref[2]
		}
		value = next
	}
	return [3]float64{}, false
}

// desktopMaterialTokenViolations checks one token set: completeness, opaque base colours, the surface layering rule,
// the accent text pairing and the contrast pairs. ink holds the skin tokens (--vd-text, --vd-theme-muted ...).
func desktopMaterialTokenViolations(variant desktopMaterialVariant, tokens, ink map[string]string, measure bool) []string {
	var violations []string
	add := func(format string, args ...interface{}) {
		violations = append(violations, fmt.Sprintf("%s (%s): %s", variant.Name, variant.Path, fmt.Sprintf(format, args...)))
	}
	for _, token := range desktopMaterialTokens {
		if tokens[token] == "" {
			add("%s is not defined", token)
		}
	}
	if !measure {
		return violations
	}
	userAccent := variant.Theme.Accent == "user"
	accentBased := func(token string) bool {
		return userAccent && desktopMaterialUserAccent.MatchString(tokens[token])
	}

	for _, surface := range desktopMaterialSurfaces {
		colour := "--vd-mat-" + surface + "-color"
		if value := tokens[colour]; value != "" && !desktopMaterialOpaqueColor.MatchString(value) && !accentBased(colour) {
			add("%s is %q, want an opaque #rrggbb colour (only var(--vd-accent) tokens of a user-accent theme may differ)", colour, value)
		}
		background := "--vd-mat-" + surface
		if value := tokens[background]; value != "" && !strings.HasSuffix(strings.TrimSpace(value), "var("+colour+")") {
			add("%s is %q, its last layer must be var(%s)", background, value, colour)
		}
	}
	// The text on a pure accent surface is the accent contrast colour. On an accent lightened with white that colour
	// (white by default) would not read, so the text is a literal ink that stays dark whatever the accent is.
	for _, pair := range [][2]string{{"--vd-mat-primary-text", "--vd-mat-primary-color"}, {"--vd-mat-selected-text", "--vd-mat-selected-color"}} {
		if !userAccent {
			continue
		}
		text := tokens[pair[0]]
		switch {
		case tokens[pair[1]] == "var(--vd-accent)":
			if text != "var(--vd-accent-contrast)" {
				add("%s is %q, but %s is the user accent, so the text must be var(--vd-accent-contrast)", pair[0], text, pair[1])
			}
		case desktopMaterialAccentWhiteMix.MatchString(tokens[pair[1]]):
			ink, isHex := parseCSSHexColor(text)
			switch {
			case !isHex || !desktopMaterialOpaqueColor.MatchString(text):
				add("%s is %q, but %s is the user accent mixed with white, so the text must be a literal #rrggbb ink", pair[0], text, pair[1])
			case cssRelativeLuminance(ink) > desktopMaterialInkMaxLuminance:
				add("%s is %s with relative luminance %.3f, but %s is the user accent mixed with white, so the ink must be dark (relative luminance at most %.2f)", pair[0], text, cssRelativeLuminance(ink), pair[1], desktopMaterialInkMaxLuminance)
			}
		}
	}

	for _, token := range []string{"--vd-text", "--vd-theme-muted"} {
		if ink[token] == "" {
			add("the skin token %s was not found, so the contrast pairs that read it cannot be measured", token)
		}
	}
	lookups := []map[string]string{tokens, ink}
	side := func(name string) (token string, value string) {
		switch name {
		case "text":
			return "--vd-text", ink["--vd-text"]
		case "muted":
			return "--vd-theme-muted", ink["--vd-theme-muted"]
		}
		return name, tokens[name]
	}
	for _, pair := range desktopMaterialContrastPairs {
		foreground, foregroundValue := side(pair[0])
		background, backgroundValue := side(pair[1])
		if foregroundValue == "" || backgroundValue == "" {
			continue // an undefined token is reported above; an undefined skin token by the skin contract
		}
		if (userAccent && desktopMaterialUserAccent.MatchString(foregroundValue)) || (userAccent && desktopMaterialUserAccent.MatchString(backgroundValue)) {
			continue
		}
		fg, fgOK := desktopMaterialColour(foregroundValue, lookups...)
		bg, bgOK := desktopMaterialColour(backgroundValue, lookups...)
		if !fgOK || !bgOK {
			add("contrast of %s (%s) on %s (%s) cannot be measured: both must resolve to a hex colour", foreground, foregroundValue, background, backgroundValue)
			continue
		}
		if ratio := cssContrastRatio(fg, bg); ratio < 4.5 {
			add("contrast of %s (%s) on %s (%s) is %.2f:1, want at least 4.5:1", foreground, foregroundValue, background, backgroundValue, ratio)
		}
	}
	return violations
}

// desktopMaterialURLs lists the url(...) arguments of css. A url( inside a quoted argument (an SVG fill reference in
// a data URI) belongs to that argument.
func desktopMaterialURLs(css string) []string {
	var urls []string
	consumed := 0
	for _, loc := range desktopMaterialURLOpen.FindAllStringIndex(css, -1) {
		if loc[0] < consumed {
			continue
		}
		pos := loc[1]
		for pos < len(css) && strings.IndexByte(" \t\r\n", css[pos]) >= 0 {
			pos++
		}
		if pos >= len(css) {
			break
		}
		if css[pos] == '"' || css[pos] == '\'' {
			end := skipCSSString(css, pos)
			urls = append(urls, css[pos+1:end-1])
			consumed = end
			continue
		}
		if end := strings.IndexByte(css[pos:], ')'); end >= 0 {
			urls = append(urls, strings.TrimSpace(css[pos:pos+end]))
			consumed = pos + end
		}
	}
	return urls
}

// desktopMaterialFileViolations checks the rules every material file follows: scope, --vd-mat-* custom properties only, no motion,
// no external assets, namespaced fonts and a line budget. inScope says whether a selector belongs to the file.
func desktopMaterialFileViolations(path, css string, inScope func(selector string) bool, scope string) []string {
	var violations []string
	add := func(format string, args ...interface{}) {
		violations = append(violations, fmt.Sprintf("%s: %s", path, fmt.Sprintf(format, args...)))
	}
	sheet := parseCSSSheet(css)
	for _, problem := range sheet.Problems {
		add("%s", problem)
	}
	if lines := len(strings.Split(strings.TrimRight(css, "\r\n"), "\n")); lines > desktopMaterialsMaxLines {
		add("the file has %d lines, the limit is %d", lines, desktopMaterialsMaxLines)
	}
	if len(sheet.Rules) == 0 {
		add("the file has no style rule")
	}
	for _, rule := range sheet.Rules {
		if len(rule.Context) != 0 {
			add("line %d: %.80q sits inside %s; material files hold plain rules only", rule.Line, rule.Selector, strings.Join(rule.Context, " > "))
		}
		for _, selector := range splitCSSSelectorList(rule.Selector) {
			if !inScope(canonicalCSSSelector(selector)) {
				add("line %d: selector %.80q must %s", rule.Line, selector, scope)
			}
		}
		for _, decl := range rule.Decls {
			switch {
			case !strings.HasPrefix(decl.Property, "--"):
				add("line %d: %.80q sets %q; material files hold custom properties only", rule.Line, rule.Selector, decl.Property)
			case !strings.HasPrefix(decl.Property, "--vd-mat-"):
				add("line %d: %.80q sets %q; material files define --vd-mat-* tokens only and never redefine shell tokens", rule.Line, rule.Selector, decl.Property)
			}
		}
	}
	if len(sheet.Keyframes) != 0 || len(sheet.Other) != 0 {
		add("material files hold style rules and @font-face only, found %d @keyframes and %d other at-rules", len(sheet.Keyframes), len(sheet.Other))
	}
	if strings.Contains(sheet.Text, "!important") {
		add("!important is not allowed")
	}
	if found := desktopMaterialMotion.FindString(sheet.Text); found != "" {
		add("%q is not allowed: materials do not animate", found)
	}
	if strings.Contains(strings.ToLower(sheet.Text), "@import") {
		add("@import is not allowed")
	}
	for _, face := range sheet.FontFaces {
		family := ""
		for _, decl := range face.Decls {
			if decl.Property == "font-family" {
				family = cssDeclarationValue(decl.Value)
			}
		}
		if !(strings.HasPrefix(family, `"Desktop Theme `) || strings.HasPrefix(family, `'Desktop Theme `)) {
			add(`line %d: @font-face font-family %q must start with "Desktop Theme "`, face.Line, family)
		}
	}
	for _, address := range desktopMaterialURLs(sheet.Text) {
		lower := strings.ToLower(address)
		switch {
		case strings.HasPrefix(address, "/fonts/"):
		case strings.HasPrefix(lower, "data:image/svg+xml"):
			if len(address) > desktopMaterialsMaxDataURI {
				add("an inline SVG pattern is %d bytes, the limit is %d", len(address), desktopMaterialsMaxDataURI)
			}
			stripped := strings.NewReplacer("http://www.w3.org/2000/svg", "", "http://www.w3.org/1999/xlink", "").Replace(lower)
			for _, forbidden := range []string{"http:", "https:", "//", "javascript:", "<script", "%3cscript", "<foreignobject", "%3cforeignobject"} {
				if strings.Contains(stripped, forbidden) {
					add("an inline SVG pattern contains %q", forbidden)
				}
			}
		default:
			add("url(%.60s) is not allowed: only /fonts/ files and data:image/svg+xml patterns are", address)
		}
	}
	return violations
}

func desktopMaterialScope(theme desktopThemeDescriptor, path string) (inScope func(string) bool, description string) {
	if path == desktopMaterialsSharedPath {
		return func(selector string) bool {
				return selector == ".desktop-body" ||
					strings.HasPrefix(selector, `.desktop-body[data-theme="standard"]`) ||
					strings.HasPrefix(selector, `.desktop-body[data-theme="fruity"]`)
			},
			`be .desktop-body or start with .desktop-body[data-theme="standard"] / [data-theme="fruity"]`
	}
	root := `.desktop-body[data-theme="` + theme.ID + `"]`
	return func(selector string) bool { return strings.HasPrefix(selector, root) }, "start with " + root
}

func TestDesktopAppMaterialsTokens(t *testing.T) {
	t.Parallel()
	themes := parseDesktopThemeRegistry(t)

	t.Run("defaults", func(t *testing.T) {
		sheet := parseCSSSheet(readDesktopThemeSource(t, desktopMaterialsSharedPath))
		tokens, found := desktopMaterialPlainRule(sheet, ".desktop-body")
		if !found {
			t.Fatalf("%s has no .desktop-body rule with the default tokens", desktopMaterialsSharedPath)
		}
		defaults := desktopMaterialVariant{Name: "defaults", Path: desktopMaterialsSharedPath}
		for _, violation := range desktopMaterialTokenViolations(defaults, tokens, nil, false) {
			t.Error(violation)
		}
	})

	for _, variant := range desktopMaterialVariants(themes) {
		variant := variant
		t.Run(variant.Name, func(t *testing.T) {
			data, err := Content.ReadFile(filepath.ToSlash(variant.Path))
			if err != nil {
				t.Fatalf("%s: %v (every registry theme needs its material file)", variant.Name, err)
			}
			tokens, found := desktopMaterialPlainRule(parseCSSSheet(string(data)), variant.Selector)
			if !found {
				t.Fatalf("%s has no plain rule %s", variant.Path, variant.Selector)
			}
			for _, violation := range desktopMaterialTokenViolations(variant, tokens, desktopMaterialInk(t, variant), true) {
				t.Error(violation)
			}
		})
	}
}

func TestDesktopAppMaterialsFiles(t *testing.T) {
	t.Parallel()
	themes := parseDesktopThemeRegistry(t)

	paths := map[string]desktopThemeDescriptor{desktopMaterialsSharedPath: {ID: "standard"}}
	for _, theme := range themes {
		paths[desktopMaterialsPath(theme)] = theme
	}
	for path, theme := range paths {
		path, theme := path, theme
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := Content.ReadFile(filepath.ToSlash(path))
			if err != nil {
				t.Fatalf("%s: %v (every registry theme needs its material file)", theme.ID, err)
			}
			inScope, scope := desktopMaterialScope(theme, path)
			for _, violation := range desktopMaterialFileViolations(path, string(data), inScope, scope) {
				t.Error(violation)
			}
		})
	}

	onDisk, err := filepath.Glob(filepath.Join("css", "desktop-materials-*.css"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range onDisk {
		if path := filepath.ToSlash(file); paths[path].ID == "" {
			t.Errorf("%s exists but belongs to no registry theme", path)
		}
	}
}

func TestDesktopAppMaterialsBundle(t *testing.T) {
	t.Parallel()
	themes := parseDesktopThemeRegistry(t)
	want := []string{"ui/" + desktopMaterialsSharedPath}
	for _, theme := range themes {
		if !desktopThemeBuiltInShellThemes[theme.ID] {
			want = append(want, "ui/"+desktopMaterialsPath(theme))
		}
	}

	parts := desktopBundleParts(t, "ui/css/desktop-shell.bundle.css")
	lastSkin, cursors := -1, indexOfBundlePart(parts, "ui/css/desktop-cursors.css")
	for index, part := range parts {
		if strings.HasPrefix(part, "ui/css/desktop-theme-") {
			lastSkin = index
		}
	}
	if lastSkin < 0 || cursors < 0 {
		t.Fatalf("desktop-shell.bundle.css must list the theme skins (last at %d) and desktop-cursors.css (at %d)", lastSkin, cursors)
	}
	previous := lastSkin
	for _, path := range want {
		index := indexOfBundlePart(parts, path)
		switch {
		case index < 0:
			t.Errorf("desktop-shell.bundle.css does not list %s", path)
		case index <= previous:
			t.Errorf("%s is at %d in desktop-shell.bundle.css, want after the previous part (%d): desktop-materials.css first, then the theme files in skin order, after the last desktop-theme-*.css", path, index, previous)
		case index >= cursors:
			t.Errorf("%s is at %d in desktop-shell.bundle.css, want before desktop-cursors.css (%d)", path, index, cursors)
		default:
			previous = index
		}
	}
	listed := map[string]bool{}
	for _, path := range want {
		listed[path] = true
	}
	for _, part := range parts {
		if strings.HasPrefix(part, "ui/css/desktop-materials") && !listed[part] {
			t.Errorf("desktop-shell.bundle.css lists %s, which is no material file of a registry theme", part)
		}
	}
}

// TestDesktopAppMaterialsChecker keeps the checks honest: broken sets must be reported with the theme, the token and
// the measured ratio, and a good set must pass.
func TestDesktopAppMaterialsChecker(t *testing.T) {
	t.Parallel()
	good := map[string]string{}
	for _, token := range desktopMaterialTokens {
		good[token] = "none"
	}
	for _, surface := range desktopMaterialSurfaces {
		good["--vd-mat-"+surface+"-color"] = "#202020"
		good["--vd-mat-"+surface] = "linear-gradient(#fff, #000), var(--vd-mat-" + surface + "-color)"
	}
	good["--vd-mat-btn-text"], good["--vd-mat-primary-text"], good["--vd-mat-selected-text"] = "#ffffff", "#ffffff", "#ffffff"
	good["--vd-mat-heading-color"], good["--vd-mat-label-color"] = "#ffffff", "#e0e0e0"
	ink := map[string]string{"--vd-text": "#f0f0f0", "--vd-theme-muted": "#c0c0c0", "--vd-accent-contrast": "#ffffff"}
	variant := desktopMaterialVariant{Name: "sample", Path: "css/desktop-materials-sample.css", Theme: desktopThemeDescriptor{ID: "sample", Accent: "fixed"}}
	if violations := desktopMaterialTokenViolations(variant, good, ink, true); len(violations) != 0 {
		t.Fatalf("a complete, readable set reported violations: %v", violations)
	}

	broken := map[string]string{}
	for token, value := range good {
		broken[token] = value
	}
	delete(broken, "--vd-mat-glow")
	broken["--vd-mat-bar-color"] = "rgba(0, 0, 0, 0.5)"
	broken["--vd-mat-side"] = "var(--vd-mat-side-color), #000"
	broken["--vd-mat-card-color"] = "#e0e0e0"
	broken["--vd-mat-primary-color"] = "var(--vd-accent)"
	joined := strings.Join(desktopMaterialTokenViolations(variant, broken, ink, true), "\n")
	for _, want := range []string{
		"sample (css/desktop-materials-sample.css): --vd-mat-glow is not defined",
		"--vd-mat-bar-color is \"rgba(0, 0, 0, 0.5)\", want an opaque #rrggbb colour",
		"--vd-mat-side is \"var(--vd-mat-side-color), #000\", its last layer must be var(--vd-mat-side-color)",
		"contrast of --vd-text (#f0f0f0) on --vd-mat-card-color (#e0e0e0) is 1.",
		"--vd-mat-primary-color is \"var(--vd-accent)\", want an opaque #rrggbb colour",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("the checker did not report %q; it reported:\n%s", want, joined)
		}
	}
	// A user-accent theme may build a surface from the accent; its text colour is the accent contrast colour.
	accent := variant
	accent.Theme.Accent = "user"
	userSet := map[string]string{}
	for token, value := range good {
		userSet[token] = value
	}
	userSet["--vd-mat-primary-color"], userSet["--vd-mat-primary-text"] = "var(--vd-accent)", "#ffffff"
	if joined := strings.Join(desktopMaterialTokenViolations(accent, userSet, ink, true), "\n"); !strings.Contains(joined, "the text must be var(--vd-accent-contrast)") || strings.Contains(joined, "opaque") {
		t.Errorf("a user-accent primary needs var(--vd-accent-contrast) text and no opaque colour; it reported:\n%s", joined)
	}
	// An accent lightened with white keeps a dark literal ink: the shared accent contrast (white) and a light ink would not read on it.
	lightened := map[string]string{}
	for token, value := range good {
		lightened[token] = value
	}
	lightened["--vd-mat-primary-color"] = "color-mix(in srgb, var(--vd-accent) 80%, #fff)"
	lightened["--vd-mat-selected-color"] = "color-mix(in srgb, var(--vd-accent) 80%, #fff)"
	for text, want := range map[string]string{
		"var(--vd-accent-contrast)": "the user accent mixed with white, so the text must be a literal #rrggbb ink",
		"#ffffff":                   "the ink must be dark (relative luminance at most 0.05)",
		"#0b1220":                   "",
	} {
		lightened["--vd-mat-primary-text"], lightened["--vd-mat-selected-text"] = text, text
		joined := strings.Join(desktopMaterialTokenViolations(accent, lightened, ink, true), "\n")
		switch {
		case want == "" && joined != "":
			t.Errorf("a dark literal ink on a lightened accent must pass; it reported:\n%s", joined)
		case want != "" && (!strings.Contains(joined, "--vd-mat-primary-text") || !strings.Contains(joined, "--vd-mat-selected-text") || !strings.Contains(joined, want)):
			t.Errorf("the ink %q on a lightened accent must be reported with %q for primary and selected; it reported:\n%s", text, want, joined)
		}
	}

	inScope, scope := desktopMaterialScope(desktopThemeDescriptor{ID: "sample"}, "css/desktop-materials-sample.css")
	css := `.desktop-body[data-theme="sample"] { --vd-mat-canvas: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg'%3E%3C/svg%3E"), #000; --vd-mat-rule: url(/fonts/rule.woff2); }
.desktop-body[data-theme="other"] { --vd-mat-bar: #fff; }
.desktop-body[data-theme="sample"] { --vd-accent-contrast: #0b1220; }
.desktop-body[data-theme="sample"] .card { color: red !important; animation: spin 1s; background: url(https://example.com/a.png); }
@media (min-width: 1px) { .desktop-body[data-theme="sample"] { --vd-mat-glow: none; } }
@font-face { font-family: "Plain"; src: url(/fonts/plain.woff2); }`
	joined = strings.Join(desktopMaterialFileViolations("css/desktop-materials-sample.css", css, inScope, scope), "\n")
	for _, want := range []string{
		`selector ".desktop-body[data-theme=\"other\"]" must start with`,
		`sets "color"; material files hold custom properties only`,
		`sets "--vd-accent-contrast"; material files define --vd-mat-* tokens only and never redefine shell tokens`,
		"!important is not allowed",
		`"animation" is not allowed`,
		"url(https://example.com/a.png) is not allowed",
		"sits inside @media",
		`font-family "\"Plain\"" must start with`,
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("the file checker did not report %q; it reported:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "url(data:") || strings.Contains(joined, "url(/fonts/") {
		t.Errorf("an inline SVG pattern with the xmlns namespace and a /fonts/ URL are allowed; it reported:\n%s", joined)
	}
	if urls := desktopMaterialURLs(`a: url("data:image/svg+xml,%3Cpath fill='url(%23g)'/%3E"), url( /fonts/a.woff2 );`); len(urls) != 2 || !strings.HasSuffix(urls[0], "%3E") || urls[1] != "/fonts/a.woff2" {
		t.Errorf("url() scanning returned %q, want the data URI (with its inner url(%%23g)) and the font", urls)
	}
}

// desktopAppMaterialsExempt lists the app stylesheets that deliberately do not read the --vd-mat-* tokens, each with
// its reason. Everything else that an app loads must paint its regions from the materials (the adoption rules are in
// the design spec). Add a stylesheet here only when it is not a themed surface; an entry that reads the tokens or is
// no longer loaded by an app fails the test, so the list cannot go stale.
var desktopAppMaterialsExempt = map[string]string{
	"/css/xterm.css":                   "vendor: the xterm.js terminal emulator stylesheet",
	"/css/cm-chessboard.css":           "vendor: the cm-chessboard board widget stylesheet",
	"/css/hljs-github.min.css":         "vendor: highlight.js light syntax theme",
	"/css/hljs-github-dark.min.css":    "vendor: highlight.js dark syntax theme",
	"/css/radio.css":                   "theme-independent skin: the physical Radio receiver",
	"/css/teevee.css":                  "theme-independent skin: the physical TeeVee receiver",
	"/css/desktop-app-rtl-sdr.css":     "theme-independent skin: the RTL-SDR receiver front panel",
	"/css/galaxa-deluxe.css":           "theme-independent skin: the Galaxa arcade game",
	"/css/desktop-app-sip-phone.css":   "theme-independent skin: the SIP phone hardware chrome",
	"/css/desktop-app-minigolf.css":    "theme-independent skin: the Minigolf game scene",
	"/css/desktop-app-aura-drift.css":  "theme-independent skin: the Aura Drift racing game and its HUD",
	"/css/desktop-app-retro-games.css": "theme-independent skin: the Retro Games arcade player",
	"/css/ha-switchboard.css":          "theme-independent skin: the walnut and brass Home Assistant switchboard cabinet",
	"/css/go2rtc-source-form.css":      "no painted region: layout and hint rules only",
	"/css/theme-optics.css":            "no painted region: places the shared WebGL backdrop layer; its colours come from the per-theme recipes, not from shell tokens",
}

var desktopAppStylesheetPattern = regexp.MustCompile(`'(/css/[A-Za-z0-9_.\-]+\.css)'`)

// desktopAppStylesheets returns the /css/*.css files the desktop apps load: the shared app styles plus the styles of
// every entry of DESKTOP_APP_ASSETS in js/desktop/core/module-loader.js. Vendor engine styles under /js/vendor/ are
// not /css/ paths and are skipped by the pattern.
func desktopAppStylesheets(t testing.TB) []string {
	t.Helper()
	data, err := Content.ReadFile("js/desktop/core/module-loader.js")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "const commonAppStyles")
	assets := strings.Index(source, "const DESKTOP_APP_ASSETS = {")
	if start < 0 || assets < start {
		t.Fatal("module-loader.js no longer declares commonAppStyles before DESKTOP_APP_ASSETS; update desktopAppStylesheets")
	}
	end := strings.Index(source[assets:], "\n    };")
	if end < 0 {
		t.Fatal("module-loader.js: the end of DESKTOP_APP_ASSETS was not found")
	}
	seen := map[string]bool{}
	var sheets []string
	for _, match := range desktopAppStylesheetPattern.FindAllStringSubmatch(source[start:assets+end], -1) {
		if !seen[match[1]] {
			seen[match[1]] = true
			sheets = append(sheets, match[1])
		}
	}
	return sheets
}

// TestDesktopAppMaterialsAdoptedByApps is the adoption guard: every stylesheet an app loads reads the --vd-mat-*
// tokens, so a new app (or a rewritten one) cannot ship with flat, theme-blind surfaces, unless it is one of the
// documented exceptions above.
func TestDesktopAppMaterialsAdoptedByApps(t *testing.T) {
	t.Parallel()
	sheets := desktopAppStylesheets(t)
	if len(sheets) < 60 {
		t.Fatalf("found only %d app stylesheets in module-loader.js, the parser no longer matches the file", len(sheets))
	}
	loaded := map[string]bool{}
	for _, sheet := range sheets {
		loaded[sheet] = true
		data, err := Content.ReadFile(strings.TrimPrefix(sheet, "/"))
		if err != nil {
			t.Errorf("%s is loaded by an app but cannot be read: %v", sheet, err)
			continue
		}
		adopts := strings.Contains(string(data), "--vd-mat-")
		reason, exempt := desktopAppMaterialsExempt[sheet]
		switch {
		case !adopts && !exempt:
			t.Errorf("%s is loaded by an app but does not read the app materials (--vd-mat-*): paint its bars, sides, cards, wells, fields, buttons and selection from them with the old value as fallback, or add it to desktopAppMaterialsExempt with a reason", sheet)
		case adopts && exempt:
			t.Errorf("%s reads the app materials but is listed as exempt (%s): remove it from desktopAppMaterialsExempt", sheet, reason)
		}
	}
	for sheet := range desktopAppMaterialsExempt {
		if !loaded[sheet] {
			t.Errorf("desktopAppMaterialsExempt lists %s, which no app loads any more: remove the entry", sheet)
		}
	}
}
