package ui

import (
	"io/fs"
	"strings"
	"testing"
	"unicode"
)

// A desktop icon is tracked by the shell: iconMarkup() records what it was called with in data-vd-icon-* attributes and
// refreshThemeIconElements() re-renders the icon with those values whenever the settings or the icon theme change.
// An app that calls iconMarkup(key, fallback) without a size gets whatever default the render path of the moment has
// (16 px for the symbol fallback, 42 px for themed artwork and sprites), so the same button could change its icon size
// on the next refresh. Every call from an app therefore passes the size, the fourth argument.

// iconMarkupCall is one call of iconMarkup in a JavaScript source.
type iconMarkupCall struct {
	Line   int
	Args   int
	Spread bool // an argument is spread (...args), the real count is unknown
	Member bool // this.iconMarkup(...), ctx.iconMarkup?.(...): the call reaches the shell function or a runtime wrapper
}

func isJSIdentChar(c byte) bool {
	return c == '_' || c == '$' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// jsRegexAllowed reports whether a / at index i starts a regular expression literal rather than a division.
func jsRegexAllowed(src string, i int) bool {
	for j := i - 1; j >= 0; j-- {
		if c := src[j]; c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			return strings.IndexByte("(,=:[!&|?{};+-*%<>~^", c) >= 0
		}
	}
	return true
}

// scanJSArgs walks JavaScript from i, which is just behind an opening bracket, to the matching closing bracket. It
// understands strings, template literals with nested expressions (skipJSString), comments and regular expression
// literals (skipJSRegex), so a comma or a parenthesis inside them is not mistaken for code. It returns the number of
// top-level comma separated arguments (a trailing comma adds none), whether one of them is spread, and the index behind
// the closing bracket.
func scanJSArgs(src string, i int) (args int, spread bool, end int) {
	depth, content := 0, false
	for i < len(src) {
		c := src[i]
		switch {
		case c == '\'' || c == '"' || c == '`':
			i, content = skipJSString(src, i), true
			continue
		case c == '/' && strings.HasPrefix(src[i:], "//"):
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		case c == '/' && strings.HasPrefix(src[i:], "/*"):
			if stop := strings.Index(src[i+2:], "*/"); stop >= 0 {
				i += stop + 4
			} else {
				i = len(src)
			}
			continue
		case c == '/' && jsRegexAllowed(src, i):
			if next := skipJSRegex(src, i); next > i+1 {
				i, content = next, true
				continue
			}
		case c == '(' || c == '[' || c == '{':
			depth++
			content = true
		case c == ')' || c == ']' || c == '}':
			if depth == 0 {
				if content {
					args++
				}
				return args, spread, i + 1
			}
			depth--
		case c == ',' && depth == 0:
			if content {
				args++
			}
			content = false
		case strings.HasPrefix(src[i:], "...") && !content:
			spread, content = true, true
		case !unicode.IsSpace(rune(c)):
			content = true
		}
		i++
	}
	return args, spread, len(src)
}

// iconMarkupCalls lists every call of iconMarkup in src. Function declarations, longer names (miniIconMarkup), property
// names that are no call and commented-out lines are not calls.
func iconMarkupCalls(src string) []iconMarkupCall {
	var calls []iconMarkupCall
	const name = "iconMarkup"
	for from := 0; ; {
		at := strings.Index(src[from:], name)
		if at < 0 {
			return calls
		}
		start := from + at
		from = start + len(name)
		if start > 0 && isJSIdentChar(src[start-1]) {
			continue
		}
		open := from
		if strings.HasPrefix(src[open:], "?.") {
			open += 2
		}
		if open >= len(src) || src[open] != '(' {
			continue
		}
		lineStart := strings.LastIndexByte(src[:start], '\n') + 1
		prefix := strings.TrimSpace(src[lineStart:start])
		if strings.HasPrefix(prefix, "//") || strings.HasPrefix(prefix, "*") || strings.HasPrefix(prefix, "/*") || strings.HasSuffix(prefix, "function") {
			continue
		}
		args, spread, _ := scanJSArgs(src, open+1)
		calls = append(calls, iconMarkupCall{
			Line: 1 + strings.Count(src[:start], "\n"), Args: args, Spread: spread, Member: start > 0 && src[start-1] == '.',
		})
		from = open + 1 // calls nested in the arguments are found too
	}
}

// iconMarkupWrapperForwardsSize reports whether src declares its own `function iconMarkup(...)` that hands the icon to
// the shell with an explicit size: its body calls a member iconMarkup with at least four arguments. The callers of such a
// wrapper cannot leave a tracked icon sizeless, whatever they pass to the wrapper.
func iconMarkupWrapperForwardsSize(src string) bool {
	for from := 0; ; {
		at := strings.Index(src[from:], "function iconMarkup(")
		if at < 0 {
			return false
		}
		start := from + at
		from = start + len("function iconMarkup(")
		_, _, afterParams := scanJSArgs(src, from)
		brace := strings.IndexByte(src[afterParams:], '{')
		if brace < 0 {
			continue
		}
		_, _, bodyEnd := scanJSArgs(src, afterParams+brace+1)
		for _, call := range iconMarkupCalls(src[afterParams+brace+1 : bodyEnd]) {
			if call.Member && (call.Args >= 4 || call.Spread) {
				return true
			}
		}
	}
}

// sizelessIconMarkupCalls returns the calls in src that pass fewer than the four arguments key, fallback, class, size.
func sizelessIconMarkupCalls(src string) []iconMarkupCall {
	wrapped := iconMarkupWrapperForwardsSize(src)
	var sizeless []iconMarkupCall
	for _, call := range iconMarkupCalls(src) {
		if call.Args >= 4 || call.Spread || wrapped && !call.Member {
			continue
		}
		sizeless = append(sizeless, call)
	}
	return sizeless
}

func TestDesktopAppIconMarkupCallsPassAnExplicitSize(t *testing.T) {
	t.Parallel()

	calls := 0
	err := fs.WalkDir(Content, "js/desktop/apps", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "vendor" || entry.Name() == "bundles" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".js") {
			return nil
		}
		data, err := Content.ReadFile(path)
		if err != nil {
			return err
		}
		src := string(data)
		calls += len(iconMarkupCalls(src))
		for _, call := range sizelessIconMarkupCalls(src) {
			t.Errorf("%s:%d: iconMarkup(...) passes %d argument(s). Pass the size as the fourth argument (iconMarkup(key, fallback, className, size)); "+
				"a sizeless icon renders at the default of its render path and is re-rendered at another size when the desktop refreshes its icons", path, call.Line, call.Args)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls < 100 {
		t.Fatalf("scanned only %d iconMarkup calls under js/desktop/apps; the scanner or the walk is broken", calls)
	}
}

func TestIconMarkupCallScanner(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		src  string
		want []int // the argument counts of the sizeless calls
	}{
		{"two arguments", "x = iconMarkup('a', 'b');", []int{2}},
		{"one argument", "x = iconMarkup('a');", []int{1}},
		{"explicit size", "x = iconMarkup('a', 'b', 'cls', 16);", nil},
		{"usage after the size", "x = iconMarkup('a', 'b', 'cls', 16, 'action');", nil},
		{"member call is checked", "x = this.iconMarkup('a', 'b');", []int{2}},
		{"optional member call is checked", "x = ctx.iconMarkup?.('a', 'b');", []int{2}},
		{"optional member call with size", "x = ctx.iconMarkup?.('a', '', 'notes-icon', 18, 'action');", nil},
		{"template literal expression", "h = `<b>${iconMarkup('a', 'b')}</b>`;", []int{2}},
		{"commas and brackets in strings", `x = iconMarkup(t('a, b'), ")", 'c', 16);`, nil},
		{"commas and brackets in strings, sizeless", `x = iconMarkup(t('a, b'), ")");`, []int{2}},
		{"nested template with commas", "x = iconMarkup(`a,${f(1, 2, 3, 4)}`, 'x');", []int{2}},
		{"regular expression with a quote", `x = iconMarkup(s.replace(/'/g, ''), 'b');`, []int{2}},
		{"nested calls are both found", "x = iconMarkup('a', 'b', iconMarkup('c', 'd'));", []int{3, 2}},
		{"trailing comma adds no argument", "x = iconMarkup('a', 'b', 'c',);", []int{3}},
		{"call split over lines", "x = iconMarkup(\n  'a',\n  'b'\n);", []int{2}},
		{"spread is unknown", "x = iconMarkup(...args);", nil},
		{"miniIconMarkup is another function", "x = miniIconMarkup('a');", nil},
		{"a property that is no call", "const { iconMarkup } = ctx; const o = { iconMarkup: ctx.iconMarkup };", nil},
		{"commented out call", "// x = iconMarkup('a');\n/* iconMarkup('a') */\n * iconMarkup('a')", nil},
		{"function declaration", "function iconMarkup(key, fallback) { return ''; }", nil},
		{"wrapper that forwards the size", "function iconMarkup(key, fallback) { return state.iconMarkup(key, fallback, 'cs-cp-icon', 16); }\nx = iconMarkup('a', 'b');", nil},
		{"wrapper that forwards the size, a member call stays checked", "function iconMarkup(key, fallback) { return state.iconMarkup(key, fallback, 'cs-cp-icon', 16); }\nx = this.iconMarkup('a', 'b');", []int{2}},
		{"wrapper without a size", "function iconMarkup(key, fallback) { return state.iconMarkup(key, fallback); }\nx = iconMarkup('a', 'b');", []int{2, 2}},
		{"wrapper that renders no icon", "function iconMarkup(key, fallback) { return esc(fallback); }\nx = iconMarkup('a', 'b');", []int{2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []int
			for _, call := range sizelessIconMarkupCalls(tc.src) {
				got = append(got, call.Args)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("sizeless calls = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("sizeless calls = %v, want %v", got, tc.want)
				}
			}
		})
	}

	t.Run("lines", func(t *testing.T) {
		calls := sizelessIconMarkupCalls("a\n\nx = iconMarkup('a', 'b');\ny = 1;\nz = iconMarkup('a');")
		if len(calls) != 2 || calls[0].Line != 3 || calls[1].Line != 5 {
			t.Fatalf("calls = %+v, want lines 3 and 5", calls)
		}
	})
}
