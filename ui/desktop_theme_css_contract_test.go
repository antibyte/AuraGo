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
// A small CSS walker: brace matching with string and comment awareness. Every block at-rule that holds rules
// (@media, @supports, @layer, @container, @scope, @starting-style, and any at-rule it has never heard of) is
// walked like the top level; @keyframes and @font-face get their own handling. It is only as strict as the
// theme contract needs.

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
	Other     []string // declaration-only block at-rules, which hold no rule to scope (@property, @page, ...)
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
	case cssBodyHasBlock(body):
		// A body with nested blocks holds rules (or more at-rules): they are scoped like the top-level ones.
		s.walk(from, to, append(append([]string(nil), context...), prelude))
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

// firstCSSCompound returns the leading compound of a selector: the text before the first descendant, child or
// sibling combinator that sits outside parentheses, brackets and strings.
func firstCSSCompound(selector string) string {
	selector = strings.TrimSpace(selector)
	depth := 0
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
		case ' ', '\t', '\r', '\n', '\f', '>', '+', '~':
			if depth == 0 {
				return selector[:i]
			}
		}
	}
	return selector
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

// desktopThemeRequiredTokens must all be declared on the root rule of every skin. --vd-accent-contrast, the text
// colour that sits on the accent, belongs to every skin: a user-accent skin sets the shared white, a fixed-accent
// skin a colour that reads on its accent.
func desktopThemeRequiredTokens() []string {
	tokens := []string{
		"color-scheme",
		"--vd-bg", "--vd-surface", "--vd-surface-strong", "--vd-border", "--vd-border-bright", "--vd-text", "--vd-muted",
		"--vd-shadow", "--vd-shadow-window", "--vd-inner-glow", "--vd-window-radius", "--vd-font-ui",
		"--vd-accent-contrast",
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

// desktopThemeRootOnlyTokens are the tokens the contract measures (contrast, scheme); a skin sets them on the plain
// root rule and nowhere else.
var desktopThemeRootOnlyTokens = map[string]bool{
	"color-scheme": true, "--vd-theme-app-bg": true, "--vd-theme-panel-bg": true, "--vd-text": true,
	"--vd-theme-muted": true, "--vd-accent": true, "--vd-accent-contrast": true,
}

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

	// Scope: nothing outside the theme root. The motion gate must sit on the root compound itself
	// (.desktop-body[data-theme="<id>"]:not([data-animations="false"]) .el), where it also covers every descendant;
	// a gate on a later compound (".el:not(...)") leaves the element's own ancestors free to animate.
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
				if !strings.Contains(firstCSSCompound(canonicalCSSSelector(selector)), noAnimations) {
					add("line %d: %s on %.80q must sit behind %s on the root compound", rule.Line, decl.Property, selector, noAnimations)
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

	// Token contract. The contract reads the plain root rules (selector equal to the root, outside every at-rule),
	// so the tokens it measures may not be set anywhere else: a deeper selector or an at-rule would change what
	// the skin renders without changing what this test measured.
	if len(sheet.Rules) == 0 {
		add("the file has no style rule")
		return violations
	}
	rootSelector := `.desktop-body[data-theme="` + theme.ID + `"]`
	if first := sheet.Rules[0]; canonicalCSSSelector(first.Selector) != rootSelector || len(first.Context) != 0 {
		add("the first rule must be the plain root rule %s, got %.80q", rootSelector, first.Selector)
	}
	tokens := map[string]string{}
	for _, rule := range sheet.Rules {
		if len(rule.Context) == 0 && canonicalCSSSelector(rule.Selector) == rootSelector {
			for _, decl := range rule.Decls { // a later declaration wins, as in the cascade
				tokens[decl.Property] = cssDeclarationValue(decl.Value)
			}
			continue
		}
		inside := ""
		if len(rule.Context) != 0 {
			inside = " inside " + strings.Join(rule.Context, " > ")
		}
		for _, decl := range rule.Decls {
			if desktopThemeRootOnlyTokens[decl.Property] {
				add("line %d: %.80q%s sets %s; the contract reads only the plain root rule %s, so set it there", rule.Line, rule.Selector, inside, decl.Property, rootSelector)
			}
		}
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

	// The measured colours are plain hex: #rgb is expanded, a value with an alpha channel (#rgba, #rrggbbaa) is not opaque.
	colour := func(token string) ([3]float64, bool) {
		value := tokens[token]
		if value == "" {
			return [3]float64{}, false // missing, reported above
		}
		rgb, ok := parseCSSHexColor(value)
		if !ok {
			add("%s is %q, want an opaque hex colour (#rgb or #rrggbb)", token, value)
		}
		return rgb, ok
	}
	appBg, appBgOK := colour("--vd-theme-app-bg")
	panelBg, panelBgOK := colour("--vd-theme-panel-bg")
	text, textOK := colour("--vd-text")
	muted, mutedOK := colour("--vd-theme-muted")
	accentContrast, accentContrastOK := colour("--vd-accent-contrast")
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

	// The registry scheme has to match the palette. Compare the two luminances instead of testing the background
	// against a threshold: a mid-grey skin with black text (Amiga) is a light theme.
	if appBgOK && textOK {
		background, foreground := cssRelativeLuminance(appBg), cssRelativeLuminance(text)
		switch {
		case theme.Scheme == "light" && !(background > foreground):
			add("the registry scheme is light, but --vd-theme-app-bg (luminance %.3f) is not lighter than --vd-text (%.3f)", background, foreground)
		case theme.Scheme == "dark" && !(background < foreground):
			add("the registry scheme is dark, but --vd-theme-app-bg (luminance %.3f) is not darker than --vd-text (%.3f)", background, foreground)
		}
	}

	// Text that sits on the accent: a fixed accent must be readable under it; a user accent only carries the token.
	if theme.Accent == "fixed" {
		if accent, accentOK := colour("--vd-accent"); accentOK && accentContrastOK {
			ratio(accentContrast, accent, "--vd-accent-contrast", "--vd-accent")
		}
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

// desktopThemeProbeSkin renders a skin of the theme "probe" that satisfies the whole contract, so a test can break
// exactly one thing. The palette is dark (near-black background, near-white text) or, for a light theme, the reverse;
// a fixed accent is a dark blue with white text. An override replaces a token on the root rule; an empty override
// removes it. rest is appended after the root rule.
func desktopThemeProbeSkin(theme desktopThemeDescriptor, overrides map[string]string, rest string) string {
	values := map[string]string{"color-scheme": theme.Scheme, "--vd-accent-contrast": "#ffffff"}
	if theme.Scheme == "light" {
		values["--vd-theme-app-bg"], values["--vd-theme-panel-bg"], values["--vd-text"], values["--vd-theme-muted"] = "#ffffff", "#f2f2f2", "#111111", "#444444"
	} else {
		values["--vd-theme-app-bg"], values["--vd-theme-panel-bg"], values["--vd-text"], values["--vd-theme-muted"] = "#101010", "#181818", "#f5f5f5", "#bbbbbb"
	}
	order := desktopThemeRequiredTokens()
	if theme.Accent == "fixed" {
		values["--vd-accent"], values["--vd-accent-r"], values["--vd-accent-g"], values["--vd-accent-b"], values["--vd-accent-glow"] = "#0b5cad", "11", "92", "173", "none"
		order = append(order, desktopThemeAccentTokens...)
	}
	for _, token := range order {
		if _, ok := values[token]; !ok {
			values[token] = "none"
		}
	}
	for token, value := range overrides {
		values[token] = value
	}
	var skin strings.Builder
	skin.WriteString(`.desktop-body[data-theme="` + theme.ID + `"] {` + "\n")
	for _, token := range order {
		if value := values[token]; value != "" {
			fmt.Fprintf(&skin, "  %s: %s;\n", token, value)
		}
	}
	skin.WriteString("}\n" + rest)
	return skin.String()
}

func TestDesktopThemeCSSCheckerStrictness(t *testing.T) {
	t.Parallel()
	const root = `.desktop-body[data-theme="probe"]`
	dark := desktopThemeDescriptor{ID: "probe", Scheme: "dark", Accent: "user"}
	light := desktopThemeDescriptor{ID: "probe", Scheme: "light", Accent: "user"}
	fixed := desktopThemeDescriptor{ID: "probe", Scheme: "dark", Accent: "fixed"}
	lightPalette := map[string]string{"--vd-theme-app-bg": "#ffffff", "--vd-theme-panel-bg": "#f2f2f2", "--vd-text": "#111111", "--vd-theme-muted": "#444444"}
	darkPalette := map[string]string{"--vd-theme-app-bg": "#101010", "--vd-theme-panel-bg": "#181818", "--vd-text": "#f5f5f5", "--vd-theme-muted": "#bbbbbb"}
	amigaPalette := map[string]string{"--vd-theme-app-bg": "#aaaaaa", "--vd-theme-panel-bg": "#aaaaaa", "--vd-text": "#000000", "--vd-theme-muted": "#333333"}

	cases := []struct {
		name      string
		theme     desktopThemeDescriptor
		overrides map[string]string
		rest      string
		want      string // a substring of one violation; empty means the skin must be clean
	}{
		{name: "the probe skin itself is clean", theme: dark},
		{name: "the light probe skin is clean", theme: light},
		{name: "the fixed-accent probe skin is clean", theme: fixed},

		// The motion gate belongs on the root compound.
		{name: "gate on the root compound", theme: dark, rest: root + `:not([data-animations="false"]) .el { animation: vd-theme-probe-spin 1s }`},
		{name: "gate on the root compound, other spelling and child combinator", theme: dark, rest: root + `:not([data-animations='false']) > .el { animation-name: vd-theme-probe-spin }`},
		{name: "no gate needed for animation none", theme: dark, rest: root + ` .el { animation: none }`},
		{name: "no gate at all", theme: dark, rest: root + ` .el { animation: vd-theme-probe-spin 1s }`, want: "must sit behind"},
		{name: "gate on a later compound", theme: dark, rest: root + ` .el:not([data-animations="false"]) { animation: vd-theme-probe-spin 1s }`, want: "must sit behind"},
		{name: "gate behind a child combinator", theme: dark, rest: root + ` > .el:not([data-animations="false"]) { animation-name: vd-theme-probe-spin }`, want: "must sit behind"},
		{name: "gate behind a sibling combinator", theme: dark, rest: root + ` ~ .el:not([data-animations="false"]) { animation: vd-theme-probe-spin 1s }`, want: "must sit behind"},

		// Every block at-rule is walked for scoping.
		{name: "unscoped rule in @starting-style", theme: dark, rest: `@starting-style { .vd-window { opacity: 0 } }`, want: `selector ".vd-window" must start with`},
		{name: "unscoped rule in @container", theme: dark, rest: `@container (min-width: 1px) { .vd-window { color: red } }`, want: `selector ".vd-window" must start with`},
		{name: "unscoped rule in an unknown at-rule", theme: dark, rest: `@made-up-rule { .vd-window { color: red } }`, want: `selector ".vd-window" must start with`},
		{name: "unscoped rule in nested at-rules", theme: dark, rest: `@media (min-width: 1px) { @scope (.a) { .vd-window { color: red } } }`, want: `selector ".vd-window" must start with`},
		{name: "ungated animation in an unknown at-rule", theme: dark, rest: `@made-up-rule { ` + root + ` .el { animation: vd-theme-probe-spin 1s } }`, want: "must sit behind"},
		{name: "scoped rule in an unknown at-rule", theme: dark, rest: `@made-up-rule { ` + root + ` .el { color: red } }`},
		{name: "declaration-only at-rule", theme: dark, rest: `@property --vd-probe-size { syntax: "<length>"; inherits: false; initial-value: 0px }`},

		// The measured tokens live on the plain root rule(s) only.
		{name: "deeper rule sets --vd-text", theme: dark, rest: root + ` .app { --vd-text: #000000; }`, want: "sets --vd-text"},
		{name: "deeper rule sets color-scheme", theme: dark, rest: root + ` .app { color-scheme: light; }`, want: "sets color-scheme"},
		{name: "deeper rule sets --vd-theme-muted", theme: dark, rest: root + ` .app { --vd-theme-muted: #000000; }`, want: "sets --vd-theme-muted"},
		{name: "root rule with a qualifier sets --vd-accent-contrast", theme: dark, rest: root + `[data-scheme="dark"] { --vd-accent-contrast: #000000; }`, want: "sets --vd-accent-contrast"},
		{name: "root selector inside @media sets --vd-theme-app-bg", theme: dark, rest: `@media (min-width: 1px) { ` + root + ` { --vd-theme-app-bg: #ffffff } }`, want: "inside @media (min-width: 1px) sets --vd-theme-app-bg"},
		{name: "deeper rule sets --vd-accent", theme: fixed, rest: root + ` .app { --vd-accent: #ff0000; }`, want: "sets --vd-accent"},
		{name: "other tokens may live in deeper rules", theme: dark, rest: root + ` .app { --vd-border: red; --vd-probe-bar: 48px } @media (max-width: 1px) { ` + root + ` { --vd-probe-bar: 52px } }`},
		{name: "a second plain root rule supplies tokens", theme: dark, overrides: map[string]string{"--vd-font-ui": ""}, rest: root + ` { --vd-font-ui: system-ui }`},
		{name: "a later root rule wins", theme: dark, rest: root + ` { --vd-text: #101010 }`, want: "contrast of --vd-text on --vd-theme-app-bg"},

		// The registry scheme matches the palette (by luminance, not by a threshold).
		{name: "light scheme with a dark palette", theme: light, overrides: darkPalette, want: "the registry scheme is light"},
		{name: "dark scheme with a light palette", theme: dark, overrides: lightPalette, want: "the registry scheme is dark"},
		{name: "mid-grey with black text is a light scheme", theme: light, overrides: amigaPalette},
		{name: "mid-grey with black text is not a dark scheme", theme: dark, overrides: amigaPalette, want: "the registry scheme is dark"},

		// Opaque hex: #rgb is expanded, an alpha channel is rejected.
		{name: "three-digit hex backgrounds", theme: light, overrides: map[string]string{"--vd-theme-app-bg": "#fff", "--vd-theme-panel-bg": "#fff", "--vd-text": "#111", "--vd-theme-muted": "#444"}},
		{name: "eight-digit hex background", theme: light, overrides: map[string]string{"--vd-theme-app-bg": "#ffffff80"}, want: `--vd-theme-app-bg is "#ffffff80", want an opaque hex colour`},
		{name: "four-digit hex text", theme: dark, overrides: map[string]string{"--vd-text": "#fff8"}, want: `--vd-text is "#fff8", want an opaque hex colour`},

		// Text on the accent.
		{name: "the accent contrast is required", theme: dark, overrides: map[string]string{"--vd-accent-contrast": ""}, want: "the root rule does not define --vd-accent-contrast"},
		{name: "the accent contrast must be hex", theme: dark, overrides: map[string]string{"--vd-accent-contrast": "var(--x)"}, want: `--vd-accent-contrast is "var(--x)", want an opaque hex colour`},
		{name: "a user accent needs only the token", theme: dark, overrides: map[string]string{"--vd-accent-contrast": "#101010"}},
		{name: "white on yellow is unreadable", theme: fixed, overrides: map[string]string{"--vd-accent": "#f3e600"}, want: "contrast of --vd-accent-contrast on --vd-accent"},
		{name: "black on yellow reads", theme: fixed, overrides: map[string]string{"--vd-accent": "#f3e600", "--vd-accent-contrast": "#000000"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			violations := desktopThemeCSSViolations(tc.theme, desktopThemeProbeSkin(tc.theme, tc.overrides, tc.rest))
			joined := strings.Join(violations, "\n")
			switch {
			case tc.want == "" && len(violations) != 0:
				t.Errorf("want a clean skin, reported:\n%s", joined)
			case tc.want != "" && !strings.Contains(joined, tc.want):
				t.Errorf("the checker did not report %q; reported:\n%s", tc.want, joined)
			}
		})
	}

	for selector, want := range map[string]string{
		`.a:not(.b .c) > .d`:    `.a:not(.b .c)`,
		`.a[title="x y"] .b`:    `.a[title="x y"]`,
		`  .a:is(.b, .c)~.d`:    `.a:is(.b, .c)`,
		`.a:nth-child(2n + 1)`:  `.a:nth-child(2n + 1)`,
		`.a+.b`:                 `.a`,
		`.desktop-body .a > .b`: `.desktop-body`,
	} {
		if got := firstCSSCompound(selector); got != want {
			t.Errorf("firstCSSCompound(%q) = %q, want %q", selector, got, want)
		}
	}
}
