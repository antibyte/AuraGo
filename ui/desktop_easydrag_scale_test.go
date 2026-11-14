package ui

import (
	"regexp"
	"strings"
	"testing"
	"unicode"
)

var (
	easyDragCSSComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	easyDragFontToken  = regexp.MustCompile(`^(var\(--ed-font-(title|head|card|body|meta|label)\)|inherit)$`)
	easyDragVarCall    = regexp.MustCompile(`var\((?:[^()]|\([^()]*\))*\)`)
	easyDragLengthLit  = regexp.MustCompile(`(^|[\s/])\d+(\.\d+)?(px|rem|em|pt|%)`)
)

// easyDragCSSDecl is one declaration of the stylesheet with its selector (or at-rule prelude) and its first line.
type easyDragCSSDecl struct {
	Line                  int
	Selector, Prop, Value string
}

// easyDragCSSDeclarations lists every declaration of css. Comments are blanked (the line numbers stay).
func easyDragCSSDeclarations(css string) []easyDragCSSDecl {
	css = easyDragCSSComment.ReplaceAllStringFunc(css, func(m string) string {
		return strings.Map(func(r rune) rune {
			if r == '\n' {
				return r
			}
			return ' '
		}, m)
	})
	var out []easyDragCSSDecl
	var stack []string
	var cur strings.Builder
	line, curLine, started := 1, 1, false
	flush := func() {
		seg := strings.TrimSpace(cur.String())
		cur.Reset()
		started = false
		if i := strings.Index(seg, ":"); i > 0 && len(stack) > 0 {
			out = append(out, easyDragCSSDecl{curLine, stack[len(stack)-1], strings.TrimSpace(seg[:i]), strings.TrimSpace(seg[i+1:])})
		}
	}
	for _, r := range css {
		switch r {
		case '{':
			stack = append(stack, strings.Join(strings.Fields(cur.String()), " "))
			cur.Reset()
			started = false
		case '}':
			flush()
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case ';':
			flush()
		default:
			if !started && !unicode.IsSpace(r) {
				started, curLine = true, line
			}
			cur.WriteRune(r)
		}
		if r == '\n' {
			line++
		}
	}
	return out
}

// TestEasyDragFontSizesUseTheScale: every font-size is one of the six tokens (title, head, card, body, meta,
// label) or inherits, and a font shorthand carries no literal size.
func TestEasyDragFontSizesUseTheScale(t *testing.T) {
	css := strings.ReplaceAll(readDesktopAssetText(t, "css/desktop-app-easydrag.css"), "\r\n", "\n")
	sizes := 0
	for _, d := range easyDragCSSDeclarations(css) {
		switch d.Prop {
		case "font-size":
			sizes++
			if !easyDragFontToken.MatchString(d.Value) {
				t.Errorf("line %d: %s { font-size: %s }: want var(--ed-font-title|head|card|body|meta|label)", d.Line, d.Selector, d.Value)
			}
		case "font":
			if lit := easyDragLengthLit.FindString(easyDragVarCall.ReplaceAllString(d.Value, "")); lit != "" {
				t.Errorf("line %d: %s { font: %s } has a literal size: use var(--ed-font-*) (and a unitless line height)", d.Line, d.Selector, d.Value)
			}
		}
	}
	if sizes < 90 {
		t.Fatalf("only %d font-size declarations found: the scan is broken", sizes)
	}
}
