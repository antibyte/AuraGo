package flows

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Template is a parsed string that may contain {{ expression }} placeholders.
type Template struct {
	parts []templatePart
}

type templatePart struct {
	literal string
	expr    *Expr
}

// Expr is one {{ … }} expression: a root, a field path and filters.
type Expr struct {
	Root    string
	Path    []PathSeg
	Filters []FilterCall
	Source  string
}

// PathSeg is one ".field", "[index]" or `["field"]` step.
type PathSeg struct {
	Field   string
	Index   int
	IsIndex bool
}

// TemplateError reports a syntax problem with its byte offset in the source.
type TemplateError struct {
	Offset int
	Msg    string
}

func (e *TemplateError) Error() string {
	return fmt.Sprintf("template error at %d: %s", e.Offset, e.Msg)
}

// HasTemplate reports whether s contains an unescaped "{{".
func HasTemplate(s string) bool {
	for i := 0; i+1 < len(s); i++ {
		if s[i] == '\\' && strings.HasPrefix(s[i+1:], "{{") {
			i += 2
			continue
		}
		if s[i] == '{' && s[i+1] == '{' {
			return true
		}
	}
	return false
}

// ParseTemplate parses src. "\{{" produces a literal "{{".
func ParseTemplate(src string) (*Template, error) {
	t := &Template{}
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			t.parts = append(t.parts, templatePart{literal: lit.String()})
			lit.Reset()
		}
	}
	for i := 0; i < len(src); {
		if src[i] == '\\' && strings.HasPrefix(src[i+1:], "{{") {
			lit.WriteString("{{")
			i += 3
			continue
		}
		if strings.HasPrefix(src[i:], "{{") {
			end, err := findExprEnd(src, i+2)
			if err != nil {
				return nil, err
			}
			expr, err := parseExpr(src[i+2:end], i+2)
			if err != nil {
				return nil, err
			}
			flush()
			t.parts = append(t.parts, templatePart{expr: expr})
			i = end + 2
			continue
		}
		lit.WriteByte(src[i])
		i++
	}
	flush()
	return t, nil
}

// Exprs returns the expressions of the template in source order.
func (t *Template) Exprs() []*Expr {
	var out []*Expr
	for _, p := range t.parts {
		if p.expr != nil {
			out = append(out, p.expr)
		}
	}
	return out
}

// IsSingleExpr reports whether the template is exactly one expression with no
// surrounding text; such templates keep the JSON type of their result.
func (t *Template) IsSingleExpr() bool {
	return len(t.parts) == 1 && t.parts[0].expr != nil
}

func findExprEnd(src string, from int) (int, error) {
	var quote byte
	for i := from; i < len(src); i++ {
		c := src[i]
		if quote != 0 {
			if c == '\\' {
				i++
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			quote = c
			continue
		}
		if c == '}' && i+1 < len(src) && src[i+1] == '}' {
			return i, nil
		}
	}
	return 0, &TemplateError{Offset: from - 2, Msg: "unclosed {{"}
}

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokIdent
	tokNumber
	tokString
	tokPunct
)

type token struct {
	kind tokenKind
	text string
	num  float64
	pos  int
}

type exprLexer struct {
	src  string
	pos  int
	base int
}

func isIdentStart(c byte) bool { return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isIdentPart(c byte) bool  { return isIdentStart(c) || isDigit(c) }
func isDigit(c byte) bool      { return c >= '0' && c <= '9' }

func (l *exprLexer) next() (token, error) {
	for l.pos < len(l.src) && (l.src[l.pos] == ' ' || l.src[l.pos] == '\t' || l.src[l.pos] == '\n' || l.src[l.pos] == '\r') {
		l.pos++
	}
	if l.pos >= len(l.src) {
		return token{kind: tokEOF, pos: l.base + l.pos}, nil
	}
	start := l.pos
	c := l.src[l.pos]
	switch {
	case isIdentStart(c):
		for l.pos < len(l.src) && isIdentPart(l.src[l.pos]) {
			l.pos++
		}
		return token{kind: tokIdent, text: l.src[start:l.pos], pos: l.base + start}, nil
	case isDigit(c) || (c == '-' && l.pos+1 < len(l.src) && isDigit(l.src[l.pos+1])):
		l.pos++
		for l.pos < len(l.src) && (isDigit(l.src[l.pos]) || l.src[l.pos] == '.') {
			l.pos++
		}
		text := l.src[start:l.pos]
		n, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return token{}, &TemplateError{Offset: l.base + start, Msg: "invalid number " + quoteForError(text)}
		}
		return token{kind: tokNumber, text: text, num: n, pos: l.base + start}, nil
	case c == '"' || c == '\'':
		l.pos++
		var b strings.Builder
		for l.pos < len(l.src) {
			ch := l.src[l.pos]
			if ch == '\\' && l.pos+1 < len(l.src) {
				switch next := l.src[l.pos+1]; next {
				case 'n':
					b.WriteByte('\n')
				case 't':
					b.WriteByte('\t')
				default:
					b.WriteByte(next)
				}
				l.pos += 2
				continue
			}
			if ch == c {
				l.pos++
				return token{kind: tokString, text: b.String(), pos: l.base + start}, nil
			}
			b.WriteByte(ch)
			l.pos++
		}
		return token{}, &TemplateError{Offset: l.base + start, Msg: "unterminated string"}
	case strings.IndexByte(".[]|(),", c) >= 0:
		l.pos++
		return token{kind: tokPunct, text: string(c), pos: l.base + start}, nil
	}
	// Report the whole character, not its first byte, so "größe" says 'ö' and
	// not 'Ã'; bytes that are not valid UTF-8 are shown in hex.
	if r, size := utf8.DecodeRuneInString(l.src[l.pos:]); r != utf8.RuneError || size > 1 {
		return token{}, &TemplateError{Offset: l.base + start, Msg: fmt.Sprintf("unexpected character %q", r)}
	}
	return token{}, &TemplateError{Offset: l.base + start, Msg: fmt.Sprintf("unexpected byte 0x%02x", c)}
}

type exprParser struct {
	lex *exprLexer
	tok token
}

func (p *exprParser) advance() error {
	tok, err := p.lex.next()
	if err != nil {
		return err
	}
	p.tok = tok
	return nil
}

func (p *exprParser) isPunct(s string) bool {
	return p.tok.kind == tokPunct && p.tok.text == s
}

func (p *exprParser) errorf(format string, args ...any) error {
	return &TemplateError{Offset: p.tok.pos, Msg: fmt.Sprintf(format, args...)}
}

func parseExpr(body string, base int) (*Expr, error) {
	p := &exprParser{lex: &exprLexer{src: body, base: base}}
	if err := p.advance(); err != nil {
		return nil, err
	}
	if p.tok.kind != tokIdent {
		return nil, p.errorf("expected a name")
	}
	expr := &Expr{Root: p.tok.text, Source: strings.TrimSpace(body)}
	if err := p.advance(); err != nil {
		return nil, err
	}
	for {
		if p.isPunct(".") {
			if err := p.advance(); err != nil {
				return nil, err
			}
			if p.tok.kind != tokIdent {
				return nil, p.errorf("expected a field name after '.'")
			}
			expr.Path = append(expr.Path, PathSeg{Field: p.tok.text})
			if err := p.advance(); err != nil {
				return nil, err
			}
			continue
		}
		if p.isPunct("[") {
			if err := p.advance(); err != nil {
				return nil, err
			}
			switch p.tok.kind {
			case tokNumber:
				if p.tok.num != math.Trunc(p.tok.num) {
					return nil, p.errorf("index must be a whole number")
				}
				// Bound the index like the filters bound their integer arguments;
				// int(f) of a float64 beyond the int range is implementation-defined.
				if math.Abs(p.tok.num) > math.MaxInt32 {
					return nil, p.errorf("index too large")
				}
				expr.Path = append(expr.Path, PathSeg{Index: int(p.tok.num), IsIndex: true})
			case tokString:
				expr.Path = append(expr.Path, PathSeg{Field: p.tok.text})
			default:
				return nil, p.errorf("expected a number or string inside [ ]")
			}
			if err := p.advance(); err != nil {
				return nil, err
			}
			if !p.isPunct("]") {
				return nil, p.errorf("expected ']'")
			}
			if err := p.advance(); err != nil {
				return nil, err
			}
			continue
		}
		break
	}
	for p.isPunct("|") {
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.tok.kind != tokIdent {
			return nil, p.errorf("expected a filter name after '|'")
		}
		call := FilterCall{Name: p.tok.text}
		namePos := p.tok.pos
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.isPunct("(") {
			if err := p.parseArgs(&call); err != nil {
				return nil, err
			}
		}
		if err := checkFilterCall(call); err != nil {
			return nil, &TemplateError{Offset: namePos, Msg: err.Error()}
		}
		expr.Filters = append(expr.Filters, call)
	}
	if p.tok.kind != tokEOF {
		// quoteForError cuts the echo: the token can be a huge string or name.
		return nil, p.errorf("unexpected %s", quoteForError(p.tok.text))
	}
	return expr, nil
}

func (p *exprParser) parseArgs(call *FilterCall) error {
	if err := p.advance(); err != nil {
		return err
	}
	for !p.isPunct(")") {
		arg, err := p.literal()
		if err != nil {
			return err
		}
		call.Args = append(call.Args, arg)
		if err := p.advance(); err != nil {
			return err
		}
		if p.isPunct(",") {
			if err := p.advance(); err != nil {
				return err
			}
			if p.isPunct(")") {
				return p.errorf("expected an argument after ','")
			}
			continue
		}
		if !p.isPunct(")") {
			return p.errorf("expected ',' or ')'")
		}
	}
	return p.advance()
}

func (p *exprParser) literal() (any, error) {
	switch p.tok.kind {
	case tokString:
		return p.tok.text, nil
	case tokNumber:
		return p.tok.num, nil
	case tokIdent:
		switch p.tok.text {
		case "true":
			return true, nil
		case "false":
			return false, nil
		case "null":
			return nil, nil
		}
	}
	return nil, p.errorf("expected a string, number, true, false or null")
}
