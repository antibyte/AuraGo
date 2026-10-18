package ui

import (
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Every themed skin lives in ui/css/desktop-theme-<id>.css and is bundled into desktop-shell.bundle.css.
// A skin may only restyle its own theme (every selector is rooted at .desktop-body[data-theme="<id>"]),
// must define the whole token contract on that root rule, and animates only when the user allows motion.

// desktopThemeCSSMaxLines bounds one skin file.
const desktopThemeCSSMaxLines = 900

// desktopThemeCrazyMaxInfinite bounds the endless animations of the Crazy skin.
const desktopThemeCrazyMaxInfinite = 6

// desktopThemeThumbnailMaxBytes bounds one picker thumbnail.
const desktopThemeThumbnailMaxBytes = 8192

// desktopThemeKeyframeProperties are the only properties a theme keyframe may animate: the compositor-only
// ones plus the timing helpers that do not trigger layout or paint.
var desktopThemeKeyframeProperties = map[string]bool{
	"transform": true, "opacity": true, "offset": true, "animation-timing-function": true,
}

// desktopThemeBuiltInShellThemes keep their tokens in the shared shell CSS and have no skin file.
var desktopThemeBuiltInShellThemes = map[string]bool{"standard": true, "fruity": true}

// ---------------------------------------------------------------------------------------------------
// A small CSS walker: brace matching with string and comment awareness, at-rule nesting for @media and
// @supports, plus @keyframes and @font-face blocks. It is only as strict as the theme contract needs.

type cssDeclaration struct {
	Property string
	Value    string
}

type cssStyleRule struct {
	Selector string
	Decls    []cssDeclaration
	Context  []string // enclosing @media / @supports / ... preludes, outermost first
	Line     int
}

type cssKeyframeBlock struct {
	Selector string
	Decls    []cssDeclaration
}

type cssKeyframes struct {
	Name   string
	Blocks []cssKeyframeBlock
	Line   int
}

type cssFontFace struct {
	Decls []cssDeclaration
	Line  int
}

type cssSheet struct {
	Text      string // the source without comments, line structure kept
	Rules     []cssStyleRule
	Keyframes []cssKeyframes
	FontFaces []cssFontFace
	Other     []string // block at-rules the contract does not look into (@property, @page, ...)
	Problems  []string // structural problems: unbalanced braces, rules nested in rules
}

var cssAtRuleNamePattern = regexp.MustCompile(`^@([-\w]+)`)

func parseCSSSheet(source string) cssSheet {
	sheet := cssSheet{Text: stripCSSComments(source)}
	sheet.walk(0, len(sheet.Text), nil)
	return sheet
}

// skipCSSString returns the index after the string that opens at src[i].
func skipCSSString(src string, i int) int {
	quote := src[i]
	for i++; i < len(src); i++ {
		switch src[i] {
		case '\\':
			i++
		case quote:
			return i + 1
		case '\n':
			return i
		}
	}
	return len(src)
}

// stripCSSComments blanks comments but keeps their newlines, so line numbers stay valid.
func stripCSSComments(src string) string {
	var out strings.Builder
	for i := 0; i < len(src); {
		switch {
		case src[i] == '"' || src[i] == '\'':
			end := skipCSSString(src, i)
			out.WriteString(src[i:end])
			i = end
		case src[i] == '/' && i+1 < len(src) && src[i+1] == '*':
			end := len(src)
			if closeAt := strings.Index(src[i+2:], "*/"); closeAt >= 0 {
				end = i + 2 + closeAt + 2
			}
			out.WriteByte(' ')
			out.WriteString(strings.Repeat("\n", strings.Count(src[i:end], "\n")))
			i = end
		default:
			out.WriteByte(src[i])
			i++
		}
	}
	return out.String()
}

// findCSSBlockEnd returns the index of the } that closes the { at src[open], or -1.
func findCSSBlockEnd(src string, open int) int {
	depth := 0
	for i := open; i < len(src); i++ {
		switch src[i] {
		case '"', '\'':
			i = skipCSSString(src, i) - 1
		case '\\':
			i++
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// readCSSPrelude reads from pos to the next {, ; or } outside strings, parentheses and brackets.
func readCSSPrelude(src string, pos, limit int) (prelude string, at int, delim byte) {
	depth := 0
	for i := pos; i < limit; i++ {
		switch c := src[i]; c {
		case '"', '\'':
			i = skipCSSString(src, i) - 1
		case '\\':
			i++
		case '(', '[':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		case '{', ';', '}':
			if depth == 0 {
				return src[pos:i], i, c
			}
		}
	}
	return src[pos:limit], limit, 0
}

func collapseCSSSpace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func (s *cssSheet) line(pos int) int {
	return 1 + strings.Count(s.Text[:pos], "\n")
}

func (s *cssSheet) problem(pos int, format string, args ...interface{}) {
	s.Problems = append(s.Problems, fmt.Sprintf("line %d: %s", s.line(pos), fmt.Sprintf(format, args...)))
}

func (s *cssSheet) walk(pos, limit int, context []string) {
	for pos < limit {
		for pos < limit && strings.IndexByte(" \t\r\n\f", s.Text[pos]) >= 0 {
			pos++
		}
		if pos >= limit {
			return
		}
		prelude, at, delim := readCSSPrelude(s.Text, pos, limit)
		switch delim {
		case ';':
			pos = at + 1
		case '}':
			s.problem(at, "unmatched }")
			pos = at + 1
		case 0:
			if text := collapseCSSSpace(prelude); text != "" {
				s.problem(pos, "text without a block: %.40q", text)
			}
			return
		default: // '{'
			end := findCSSBlockEnd(s.Text, at)
			if end < 0 || end >= limit {
				s.problem(at, "the block of %.40q is not closed", collapseCSSSpace(prelude))
				return
			}
			s.block(collapseCSSSpace(prelude), at+1, end, context, s.line(pos))
			pos = end + 1
		}
	}
}

func (s *cssSheet) block(prelude string, from, to int, context []string, line int) {
	body := s.Text[from:to]
	if !strings.HasPrefix(prelude, "@") {
		if cssBodyHasBlock(body) {
			s.Problems = append(s.Problems, fmt.Sprintf("line %d: %.40q nests rules; theme skins use flat rules", line, prelude))
		}
		s.Rules = append(s.Rules, cssStyleRule{Selector: prelude, Decls: parseCSSDeclarations(body), Context: context, Line: line})
		return
	}
	name := ""
	if match := cssAtRuleNamePattern.FindStringSubmatch(prelude); match != nil {
		name = strings.ToLower(match[1])
	}
	switch {
	case name == "media" || name == "supports" || name == "layer" || name == "container" || name == "scope":
		s.walk(from, to, append(append([]string(nil), context...), prelude))
	case strings.HasSuffix(name, "keyframes"):
		keyframes := cssKeyframes{Name: strings.Trim(strings.TrimSpace(prelude[len(name)+1:]), `"'`), Line: line}
		for pos := from; pos < to; {
			for pos < to && strings.IndexByte(" \t\r\n\f", s.Text[pos]) >= 0 {
				pos++
			}
			if pos >= to {
				break
			}
			selector, at, delim := readCSSPrelude(s.Text, pos, to)
			end := -1
			if delim == '{' {
				end = findCSSBlockEnd(s.Text, at)
			}
			if end < 0 || end >= to {
				s.problem(pos, "malformed @keyframes block near %.40q", collapseCSSSpace(selector))
				break
			}
			keyframes.Blocks = append(keyframes.Blocks, cssKeyframeBlock{Selector: collapseCSSSpace(selector), Decls: parseCSSDeclarations(s.Text[at+1 : end])})
			pos = end + 1
		}
		s.Keyframes = append(s.Keyframes, keyframes)
	case name == "font-face":
		s.FontFaces = append(s.FontFaces, cssFontFace{Decls: parseCSSDeclarations(body), Line: line})
	default:
		s.Other = append(s.Other, prelude)
	}
}

func cssBodyHasBlock(body string) bool {
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '"', '\'':
			i = skipCSSString(body, i) - 1
		case '\\':
			i++
		case '{':
			return true
		}
	}
	return false
}

func parseCSSDeclarations(body string) []cssDeclaration {
	var decls []cssDeclaration
	flush := func(item string) {
		item = strings.TrimSpace(item)
		colon := strings.Index(item, ":")
		if colon <= 0 {
			return
		}
		property := strings.TrimSpace(item[:colon])
		if !strings.HasPrefix(property, "--") {
			property = strings.ToLower(property)
		}
		decls = append(decls, cssDeclaration{Property: property, Value: strings.TrimSpace(item[colon+1:])})
	}
	depth, start := 0, 0
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '"', '\'':
			i = skipCSSString(body, i) - 1
		case '\\':
			i++
		case '(', '[':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		case ';':
			if depth == 0 {
				flush(body[start:i])
				start = i + 1
			}
		}
	}
	flush(body[start:])
	return decls
}

// splitCSSSelectorList splits a selector list on the commas outside parentheses, brackets and strings.
func splitCSSSelectorList(selector string) []string {
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(selector); i++ {
		switch selector[i] {
		case '"', '\'':
			i = skipCSSString(selector, i) - 1
		case '\\':
			i++
		case '(', '[':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(selector[start:i]))
				start = i + 1
			}
		}
	}
	return append(parts, strings.TrimSpace(selector[start:]))
}

func cssDeclarationValue(value string) string {
	value = strings.TrimSpace(value)
	if cut := strings.Index(strings.ToLower(value), "!important"); cut >= 0 {
		value = strings.TrimSpace(value[:cut])
	}
	return value
}

var cssThemeAttributeSelector = regexp.MustCompile(`\[\s*(data-theme|data-animations)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\]\s]+))\s*\]`)

// canonicalCSSSelector writes the data-theme and data-animations attribute selectors one way (double quotes,
// no spaces), so equivalent spellings of the same selector compare equal.
func canonicalCSSSelector(selector string) string {
	return cssThemeAttributeSelector.ReplaceAllStringFunc(selector, func(match string) string {
		parts := cssThemeAttributeSelector.FindStringSubmatch(match)
		return "[" + parts[1] + `="` + parts[2] + parts[3] + parts[4] + `"]`
	})
}

// ---------------------------------------------------------------------------------------------------
// Colour maths for the contrast contract.

var cssHexColor = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

func parseCSSHexColor(value string) (rgb [3]float64, ok bool) {
	match := cssHexColor.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return rgb, false
	}
	digits := match[1]
	if len(digits) == 3 {
		digits = string([]byte{digits[0], digits[0], digits[1], digits[1], digits[2], digits[2]})
	}
	for i := range rgb {
		channel, _ := strconv.ParseUint(digits[2*i:2*i+2], 16, 8)
		rgb[i] = float64(channel)
	}
	return rgb, true
}

func cssRelativeLuminance(rgb [3]float64) float64 {
	var linear [3]float64
	for i, channel := range rgb {
		c := channel / 255
		if c <= 0.03928 {
			linear[i] = c / 12.92
		} else {
			linear[i] = math.Pow((c+0.055)/1.055, 2.4)
		}
	}
	return 0.2126*linear[0] + 0.7152*linear[1] + 0.0722*linear[2]
}

func cssContrastRatio(a, b [3]float64) float64 {
	la, lb := cssRelativeLuminance(a), cssRelativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// ---------------------------------------------------------------------------------------------------
// The contract.

// desktopThemeRequiredTokens must all be declared on the root rule of every skin.
func desktopThemeRequiredTokens() []string {
	tokens := []string{
		"color-scheme",
		"--vd-bg", "--vd-surface", "--vd-surface-strong", "--vd-border", "--vd-border-bright", "--vd-text", "--vd-muted",
		"--vd-shadow", "--vd-shadow-window", "--vd-inner-glow", "--vd-window-radius", "--vd-font-ui",
		"--vd-theme-app-bg", "--vd-theme-panel-bg", "--vd-theme-panel-bg-strong", "--vd-theme-chrome-bg",
		"--vd-theme-overlay-bg", "--vd-theme-control-bg", "--vd-theme-control-hover", "--vd-theme-border",
		"--vd-theme-border-strong", "--vd-theme-muted", "--vd-theme-accent-soft", "--vd-theme-accent-border",
		"--vd-theme-radius-panel", "--vd-theme-radius-modal", "--vd-theme-shadow-panel", "--vd-theme-shadow-modal",
		"--vd-theme-blur",
		"--vd-chrome-highlight", "--vd-chrome-ring", "--vd-chrome-hairline",
		"--ds-color-fg-primary", "--ds-color-fg-secondary", "--ds-color-fg-muted", "--ds-color-fg-disabled",
		"--ds-color-bg-canvas", "--ds-color-bg-raised", "--ds-color-bg-overlay", "--ds-color-bg-modal",
		"--ds-color-border-subtle", "--ds-color-border-default", "--ds-color-border-strong", "--ds-color-border-focus",
		"--ds-color-accent-500", "--ds-color-accent-600", "--ds-color-accent-glow",
		"--ds-color-control-active", "--ds-color-inner-highlight",
		"--ds-blur-fine", "--ds-blur-coarse",
	}
	for level := 0; level <= 3; level++ {
		tokens = append(tokens, fmt.Sprintf("--ds-color-surface-%d", level))
	}
	for level := 1; level <= 5; level++ {
		tokens = append(tokens, fmt.Sprintf("--ds-elevation-%d", level))
	}
	return tokens
}

// desktopThemeAccentTokens are declared by themes with a fixed accent and by no other theme.
var desktopThemeAccentTokens = []string{"--vd-accent", "--vd-accent-r", "--vd-accent-g", "--vd-accent-b", "--vd-accent-glow"}

func desktopThemeSkinPath(theme desktopThemeDescriptor) string {
	return "css/desktop-theme-" + strings.ReplaceAll(theme.ID, "_", "-") + ".css"
}

// desktopThemeCSSViolations lists every way css breaks the skin contract of theme.
func desktopThemeCSSViolations(theme desktopThemeDescriptor, css string) []string {
	var violations []string
	add := func(format string, args ...interface{}) {
		violations = append(violations, fmt.Sprintf(format, args...))
	}
	sheet := parseCSSSheet(css)
	for _, problem := range sheet.Problems {
		add("%s", problem)
	}
	dashed := strings.ReplaceAll(theme.ID, "_", "-")

	if lines := len(strings.Split(strings.TrimRight(css, "\r\n"), "\n")); lines > desktopThemeCSSMaxLines {
		add("the file has %d lines, the limit is %d", lines, desktopThemeCSSMaxLines)
	}

	// Scope: nothing outside the theme root.
	scope := regexp.MustCompile(`^\.desktop-body\[data-theme="` + regexp.QuoteMeta(theme.ID) + `"\]`)
	const noAnimations = `:not([data-animations="false"])`
	for _, rule := range sheet.Rules {
		selectors := splitCSSSelectorList(rule.Selector)
		for _, selector := range selectors {
			if !scope.MatchString(canonicalCSSSelector(selector)) {
				add(`line %d: selector %.80q must start with .desktop-body[data-theme="%s"]`, rule.Line, selector, theme.ID)
			}
		}
		for _, decl := range rule.Decls {
			if decl.Property != "animation" && decl.Property != "animation-name" {
				continue
			}
			if strings.EqualFold(cssDeclarationValue(decl.Value), "none") {
				continue
			}
			for _, selector := range selectors {
				if !strings.Contains(canonicalCSSSelector(selector), noAnimations) {
					add("line %d: %s on %.80q must sit behind %s", rule.Line, decl.Property, selector, noAnimations)
				}
			}
		}
	}

	// Keyframes: namespaced, compositor-only.
	for _, keyframes := range sheet.Keyframes {
		if prefix := "vd-theme-" + dashed + "-"; !strings.HasPrefix(keyframes.Name, prefix) {
			add("line %d: @keyframes %q must start with %q", keyframes.Line, keyframes.Name, prefix)
		}
		for _, block := range keyframes.Blocks {
			for _, decl := range block.Decls {
				if !desktopThemeKeyframeProperties[decl.Property] {
					add("line %d: @keyframes %s step %s animates %q; allowed are transform, opacity, offset and animation-timing-function", keyframes.Line, keyframes.Name, block.Selector, decl.Property)
				}
			}
		}
	}

	// Fonts: namespaced family names.
	for _, face := range sheet.FontFaces {
		family, found := "", false
		for _, decl := range face.Decls {
			if decl.Property == "font-family" {
				family, found = cssDeclarationValue(decl.Value), true
			}
		}
		if !found || !(strings.HasPrefix(family, `"Desktop Theme `) || strings.HasPrefix(family, `'Desktop Theme `)) {
			add(`line %d: @font-face font-family %q must start with "Desktop Theme "`, face.Line, family)
		}
	}

	if theme.ID == "crazy" {
		if count := len(regexp.MustCompile(`\binfinite\b`).FindAllString(sheet.Text, -1)); count > desktopThemeCrazyMaxInfinite {
			add("the Crazy skin has %d infinite animations, the limit is %d", count, desktopThemeCrazyMaxInfinite)
		}
	}

	// Token contract of the root rule.
	if len(sheet.Rules) == 0 {
		add("the file has no style rule")
		return violations
	}
	root := sheet.Rules[0]
	if want := `.desktop-body[data-theme="` + theme.ID + `"]`; canonicalCSSSelector(root.Selector) != want || len(root.Context) != 0 {
		add("the first rule must be the plain root rule %s, got %.80q", want, root.Selector)
	}
	tokens := map[string]string{}
	for _, decl := range root.Decls {
		tokens[decl.Property] = cssDeclarationValue(decl.Value)
	}
	for _, token := range desktopThemeRequiredTokens() {
		if tokens[token] == "" {
			add("the root rule does not define %s", token)
		}
	}
	if scheme := tokens["color-scheme"]; scheme != "" && !strings.EqualFold(scheme, theme.Scheme) {
		add("color-scheme is %q, the registry says %q", scheme, theme.Scheme)
	}
	for _, token := range desktopThemeAccentTokens {
		switch has := tokens[token] != ""; {
		case theme.Accent == "fixed" && !has:
			add("the fixed accent needs %s on the root rule", token)
		case theme.Accent != "fixed" && has:
			add("the accent follows the user (accent: %q), so the root rule must not set %s", theme.Accent, token)
		}
	}

	colour := func(token string, opaque bool) ([3]float64, bool) {
		value, present := tokens[token]
		if !present {
			return [3]float64{}, false // already reported as missing
		}
		rgb, ok := parseCSSHexColor(value)
		if !ok {
			add("%s is %q, want a hex colour", token, value)
		} else if opaque && len(strings.TrimPrefix(value, "#")) != 6 {
			add("%s is %q, want an opaque #rrggbb", token, value)
		}
		return rgb, ok
	}
	appBg, appBgOK := colour("--vd-theme-app-bg", true)
	panelBg, panelBgOK := colour("--vd-theme-panel-bg", true)
	text, textOK := colour("--vd-text", false)
	muted, mutedOK := colour("--vd-theme-muted", false)
	ratio := func(foreground, background [3]float64, foregroundName, backgroundName string) {
		if got := cssContrastRatio(foreground, background); got < 4.5 {
			add("contrast of %s on %s is %.2f:1, want at least 4.5:1", foregroundName, backgroundName, got)
		}
	}
	if textOK && appBgOK {
		ratio(text, appBg, "--vd-text", "--vd-theme-app-bg")
	}
	if textOK && panelBgOK {
		ratio(text, panelBg, "--vd-text", "--vd-theme-panel-bg")
	}
	if mutedOK && appBgOK {
		ratio(muted, appBg, "--vd-theme-muted", "--vd-theme-app-bg")
	}
	return violations
}

var (
	svgNamespaceDeclaration = regexp.MustCompile(`\sxmlns(?::xlink)?\s*=\s*["']http://www\.w3\.org/(?:2000/svg|1999/xlink)["']`)
	svgEventHandler         = regexp.MustCompile(`(?i)\son[a-z]+\s*=`)
)

// desktopThemeThumbnailViolations checks one picker thumbnail: small, inert and self-contained. The two
// standard XML namespace declarations are the only URLs an image may carry (a standalone SVG needs xmlns).
func desktopThemeThumbnailViolations(svg string) []string {
	var violations []string
	if len(svg) > desktopThemeThumbnailMaxBytes {
		violations = append(violations, fmt.Sprintf("the thumbnail is %d bytes, the limit is %d", len(svg), desktopThemeThumbnailMaxBytes))
	}
	lower := strings.ToLower(svgNamespaceDeclaration.ReplaceAllString(svg, " "))
	for _, forbidden := range []string{"<script", "<foreignobject", "http://", "https://", "javascript:"} {
		if strings.Contains(lower, forbidden) {
			violations = append(violations, fmt.Sprintf("the thumbnail contains %q", forbidden))
		}
	}
	if svgEventHandler.MatchString(svg) {
		violations = append(violations, "the thumbnail contains an on*= event handler attribute")
	}
	return violations
}

func TestDesktopThemeCSSContract(t *testing.T) {
	t.Parallel()
	themes := parseDesktopThemeRegistry(t)
	var skinned []desktopThemeDescriptor
	for _, theme := range themes {
		if !desktopThemeBuiltInShellThemes[theme.ID] {
			skinned = append(skinned, theme)
		}
	}

	t.Run("bundle order", func(t *testing.T) {
		parts := desktopBundleParts(t, "ui/css/desktop-shell.bundle.css")
		live := indexOfBundlePart(parts, "ui/css/desktop-wallpaper-live.css")
		polish := indexOfBundlePart(parts, "ui/css/desktop-polish.css")
		if live < 0 || polish < 0 {
			t.Fatalf("desktop-shell.bundle.css must list desktop-wallpaper-live.css (index %d) and desktop-polish.css (index %d)", live, polish)
		}
		if polish != len(parts)-1 {
			t.Errorf("desktop-polish.css must be the last part of desktop-shell.bundle.css, it is at %d of %d", polish, len(parts))
		}
		want := map[string]bool{}
		for _, theme := range skinned {
			path := "ui/" + desktopThemeSkinPath(theme)
			want[path] = true
			if index := indexOfBundlePart(parts, path); index <= live || index >= polish {
				t.Errorf("%s is at %d in desktop-shell.bundle.css, want after desktop-wallpaper-live.css (%d) and before desktop-polish.css (%d)", path, index, live, polish)
			}
		}
		for _, part := range parts {
			if strings.HasPrefix(part, "ui/css/desktop-theme-") && !want[part] {
				t.Errorf("desktop-shell.bundle.css lists %s, which is no registry theme skin", part)
			}
		}
		onDisk, err := filepath.Glob(filepath.Join("css", "desktop-theme-*.css"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range onDisk {
			if path := "ui/" + filepath.ToSlash(file); !want[path] {
				t.Errorf("%s exists but belongs to no registry theme", path)
			}
		}
	})

	for _, theme := range skinned {
		theme := theme
		t.Run(theme.ID, func(t *testing.T) {
			for _, violation := range desktopThemeCSSViolations(theme, readDesktopThemeSource(t, desktopThemeSkinPath(theme))) {
				t.Errorf("%s: %s", desktopThemeSkinPath(theme), violation)
			}
		})
	}

	t.Run("thumbnails", func(t *testing.T) {
		for _, theme := range themes {
			path := "img/themes/" + theme.ID + ".svg"
			data, err := Content.ReadFile(path)
			if err != nil {
				t.Errorf("%s: %v", path, err)
				continue
			}
			for _, violation := range desktopThemeThumbnailViolations(string(data)) {
				t.Errorf("%s: %s", path, violation)
			}
		}
	})
}

func TestDesktopThemeCSSWalkerAndChecker(t *testing.T) {
	t.Parallel()
	sample := `/* a } comment { with braces */
.desktop-body[data-theme="crazy"] { color-scheme: dark; content: "}{"; --vd-text: #fff }
@media (min-width: 900px) {
  .desktop-body[data-theme="crazy"] .a, .desktop-body[data-theme="crazy"]:is(.b, .c) { color: red; }
}
@keyframes vd-theme-crazy-spin { from { transform: none } 50%, 100% { transform: rotate(1turn); opacity: 0.5 } }
@font-face { font-family: "Desktop Theme Mono"; src: url(data:font/woff2;base64,AAAA); }
`
	sheet := parseCSSSheet(sample)
	if len(sheet.Problems) != 0 {
		t.Fatalf("walker problems on a valid sample: %v", sheet.Problems)
	}
	if len(sheet.Rules) != 2 || len(sheet.Keyframes) != 1 || len(sheet.FontFaces) != 1 {
		t.Fatalf("walker found %d rules, %d keyframes, %d font faces; want 2, 1, 1", len(sheet.Rules), len(sheet.Keyframes), len(sheet.FontFaces))
	}
	if root := sheet.Rules[0]; len(root.Decls) != 3 || root.Decls[1].Value != `"}{"` || root.Line != 2 {
		t.Errorf("root rule parsed as %+v", root)
	}
	if nested := sheet.Rules[1]; len(nested.Context) != 1 || len(splitCSSSelectorList(nested.Selector)) != 2 {
		t.Errorf("rule inside @media parsed as %+v", nested)
	}
	if kf := sheet.Keyframes[0]; kf.Name != "vd-theme-crazy-spin" || len(kf.Blocks) != 2 || kf.Blocks[1].Selector != "50%, 100%" || len(kf.Blocks[1].Decls) != 2 {
		t.Errorf("keyframes parsed as %+v", kf)
	}

	crazy := desktopThemeDescriptor{ID: "crazy", Scheme: "dark", Accent: "user"}
	bad := `.vd-window { animation: spin 1s infinite }
@media (min-width: 1px) { .desktop-body[data-theme="crazy"] .x { animation-name: vd-theme-crazy-spin } }
@keyframes spin { to { left: 10px } }
@font-face { font-family: "Arial" }
.desktop-body[data-theme="crazy"] .y { animation: none }`
	joined := strings.Join(desktopThemeCSSViolations(crazy, bad), "\n")
	for _, want := range []string{
		`selector ".vd-window" must start with`,
		"animation on \".vd-window\" must sit behind :not([data-animations=\"false\"])",
		"animation-name on \".desktop-body[data-theme=\\\"crazy\\\"] .x\" must sit behind",
		`@keyframes "spin" must start with "vd-theme-crazy-"`,
		`animates "left"`,
		`font-family "\"Arial\"" must start with`,
		"the first rule must be the plain root rule",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("the checker did not report %q; reported:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "] .y") {
		t.Errorf("animation: none must be allowed anywhere; reported:\n%s", joined)
	}

	spelled := `.desktop-body[data-theme='crazy'] { color-scheme: dark }
.desktop-body[data-theme=crazy]:not([data-animations=false]) .x, .desktop-body[data-theme="crazy"]:not([data-animations='false']) .y { animation: vd-theme-crazy-spin 1s }`
	for _, violation := range desktopThemeCSSViolations(crazy, spelled) {
		if strings.Contains(violation, "must start with") || strings.Contains(violation, "must sit behind") || strings.Contains(violation, "plain root rule") {
			t.Errorf("equivalent attribute selector spellings must pass: %s", violation)
		}
	}

	if ratio := cssContrastRatio([3]float64{0, 0, 0}, [3]float64{255, 255, 255}); math.Abs(ratio-21) > 0.01 {
		t.Errorf("black on white has contrast %.2f, want 21", ratio)
	}
	if violations := desktopThemeThumbnailViolations(`<svg xmlns="http://www.w3.org/2000/svg"><rect onclick="x()"/></svg>`); len(violations) != 1 {
		t.Errorf("thumbnail checker reported %v, want only the event handler", violations)
	}
	if violations := desktopThemeThumbnailViolations(`<svg xmlns="http://www.w3.org/2000/svg"><image href="https://example.com/a.png"/></svg>`); len(violations) != 1 {
		t.Errorf("thumbnail checker reported %v, want only the remote reference", violations)
	}
}
