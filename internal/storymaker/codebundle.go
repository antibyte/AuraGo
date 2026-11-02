package storymaker

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/evanw/esbuild/pkg/api"
)

// BundleError is one problem found while bundling a code scene. Line and Column are
// 1-based and count characters (not bytes) of the author's source, so an editor can
// place a marker without conversion.
type BundleError struct {
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Message string `json:"message"`
}

// BundleResult is the outcome of BundleCodeScene. Errors non-empty means Code is nil.
type BundleResult struct {
	Code   []byte
	Errors []BundleError
}

const (
	// bundleGlobalName is the IIFE namespace esbuild assigns the scene module to.
	bundleGlobalName = "__smSceneModule"
	// bundleSuffix registers the scene function on the fixed global the code-scene worker reads.
	// A dotted GlobalName would make esbuild emit `var self = self || {}` and assign the
	// namespace object instead of the function, so the registration is appended by Go.
	bundleSuffix = "\nself.__smScene=" + bundleGlobalName + ".default;\n"

	maxBundleErrors = 20
)

// BundleCodeScene turns the author's ES module source into one classic script. The
// script defines the scene function on `self.__smScene`.
//
// Order: a size cap, esbuild's syntax check and the pre-check run on the author's own
// source, so every position refers to what the author typed. The pre-check is defence in
// depth only: the real barriers are the player CSP and the code-scene worker realm.
func BundleCodeScene(source []byte, sceneID string, maxBytes int) BundleResult {
	if maxBytes <= 0 {
		maxBytes = DefaultLimits().MaxCodeBytes
	}
	if len(source) > maxBytes {
		return BundleResult{Errors: []BundleError{{
			Line: 1, Column: 1,
			Message: fmt.Sprintf("The code scene is %d bytes; the limit is %d bytes. Split the scene or remove unused code.", len(source), maxBytes),
		}}}
	}
	if !validSlug(sceneID) {
		sceneID = "scene"
	}

	var errs []BundleError
	res := api.Transform(string(source), api.TransformOptions{
		Format:        api.FormatIIFE,
		GlobalName:    bundleGlobalName,
		Target:        api.ES2020,
		LegalComments: api.LegalCommentsNone,
		Loader:        api.LoaderJS,
		Sourcefile:    "scenes/" + sceneID + ".js",
	})
	for _, m := range res.Errors {
		errs = append(errs, esbuildError(m))
	}
	errs = append(errs, precheckSource(source)...)
	errs = normalizeBundleErrors(errs)
	if len(errs) > 0 {
		return BundleResult{Errors: errs}
	}
	out := make([]byte, 0, len(res.Code)+len(bundleSuffix))
	out = append(out, res.Code...)
	out = append(out, bundleSuffix...)
	return BundleResult{Code: out}
}

func esbuildError(m api.Message) BundleError {
	e := BundleError{Line: 1, Column: 1, Message: strings.TrimSpace(m.Text)}
	if loc := m.Location; loc != nil {
		e.Line = max(loc.Line, 1)
		col := min(max(loc.Column, 0), len(loc.LineText))
		e.Column = utf8.RuneCountInString(loc.LineText[:col]) + 1
	}
	return e
}

func normalizeBundleErrors(errs []BundleError) []BundleError {
	if len(errs) == 0 {
		return nil
	}
	sort.SliceStable(errs, func(i, j int) bool {
		if errs[i].Line != errs[j].Line {
			return errs[i].Line < errs[j].Line
		}
		return errs[i].Column < errs[j].Column
	})
	out := errs[:0]
	seen := map[BundleError]bool{}
	for _, e := range errs {
		if seen[e] {
			continue
		}
		seen[e] = true
		out = append(out, e)
		if len(out) == maxBundleErrors {
			break
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Pre-check: a small JavaScript tokenizer. esbuild's Go API exposes no AST, so the
// check works on tokens. It skips strings, template literals, comments and regular
// expression literals, so names inside them never trigger it.

type tokKind int

const (
	tokIdent tokKind = iota
	tokPunct
	tokString
	tokNumber
	tokTemplate
	tokRegex
)

type srcTok struct {
	kind      tokKind
	text      string // identifier name (escapes decoded), punctuation character or string value
	line, col int
}

var forbiddenIdents = map[string]bool{
	"eval": true, "Function": true, "globalThis": true, "self": true, "window": true, "document": true,
	"importScripts": true, "fetch": true, "XMLHttpRequest": true, "WebSocket": true,
}

// regexKeywords are the keywords after which a slash starts a regular expression.
var regexKeywords = map[string]bool{
	"return": true, "typeof": true, "instanceof": true, "in": true, "of": true, "new": true, "delete": true,
	"void": true, "throw": true, "case": true, "do": true, "else": true, "yield": true, "await": true,
}

// precheckSource runs the token checks. A slash after `)`, `}`, `++` or `--` is ambiguous
// (division or regular expression), so the source is tokenized twice, once with each
// reading, and the findings are merged: hiding a name from the check would need both
// readings to swallow it.
func precheckSource(src []byte) []BundleError {
	var errs []BundleError
	hasDefault := false
	for _, ambiguousIsRegex := range []bool{false, true} {
		toks := tokenizeJS(src, ambiguousIsRegex)
		e, d := checkTokens(toks)
		errs = append(errs, e...)
		hasDefault = hasDefault || d
	}
	if !hasDefault {
		errs = append(errs, BundleError{Line: 1, Column: 1, Message: "A code scene must export a default function: export default function scene(api) { ... }"})
	}
	return errs
}

func checkTokens(toks []srcTok) (errs []BundleError, hasDefault bool) {
	add := func(t srcTok, msg string) {
		errs = append(errs, BundleError{Line: t.line, Column: t.col, Message: msg})
	}
	for i, t := range toks {
		prev := srcTok{kind: -1}
		if i > 0 {
			prev = toks[i-1]
		}
		switch t.kind {
		case tokIdent:
			switch {
			case forbiddenIdents[t.text]:
				add(t, fmt.Sprintf("%s is not available in code scenes. Use only the api object passed to scene(api).", t.text))
			case t.text == "import":
				add(t, "import is not supported in code scenes: a scene is one file and gets everything from the api object.")
			case t.text == "constructor" && prev.kind == tokPunct && prev.text == ".":
				add(t, ".constructor is not available in code scenes.")
			case t.text == "export":
				if hasDefaultAfterExport(toks, i) {
					hasDefault = true
				}
				if exportsFromModule(toks, i) {
					add(t, "export ... from is not supported in code scenes. Use export default function scene(api) { ... }.")
				}
			}
		case tokString:
			if t.text == "constructor" && prev.kind == tokPunct && prev.text == "[" {
				add(t, `["constructor"] is not available in code scenes.`)
			} else if forbiddenIdents[t.text] && prev.kind == tokPunct && prev.text == "[" {
				add(t, fmt.Sprintf("[%q] is not available in code scenes.", t.text))
			}
		}
	}
	return errs, hasDefault
}

// hasDefaultAfterExport reports `export default ...` and `export { x as default }`.
func hasDefaultAfterExport(toks []srcTok, i int) bool {
	if i+1 >= len(toks) {
		return false
	}
	next := toks[i+1]
	if next.kind == tokIdent && next.text == "default" {
		return true
	}
	if next.kind == tokPunct && next.text == "{" {
		for j := i + 2; j < len(toks) && !(toks[j].kind == tokPunct && toks[j].text == "}"); j++ {
			if toks[j].kind == tokIdent && toks[j].text == "default" {
				return true
			}
		}
	}
	return false
}

// exportsFromModule reports `export * from ...` and `export { ... } from ...`.
func exportsFromModule(toks []srcTok, i int) bool {
	j := i + 1
	if j >= len(toks) || toks[j].kind != tokPunct {
		return false
	}
	switch toks[j].text {
	case "*":
		return true
	case "{":
		for ; j < len(toks); j++ {
			if toks[j].kind == tokPunct && toks[j].text == "}" {
				return j+1 < len(toks) && toks[j+1].kind == tokIdent && toks[j+1].text == "from"
			}
		}
	}
	return false
}

type jsScanner struct {
	src  []byte
	pos  int
	line int
	col  int
	toks []srcTok
	// braces records for every open `{` whether it opened a template substitution.
	braces []bool
	// ambiguousIsRegex selects the reading of a slash after `)`, `}`, `++` or `--`.
	ambiguousIsRegex bool
}

func tokenizeJS(src []byte, ambiguousIsRegex bool) []srcTok {
	s := &jsScanner{src: src, line: 1, col: 1, ambiguousIsRegex: ambiguousIsRegex}
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		switch {
		case c == '\n':
			s.next()
		case c == ' ' || c == '\t' || c == '\r' || c == '\v' || c == '\f':
			s.next()
		case c == '/' && s.peek(1) == '/':
			for s.pos < len(s.src) && s.src[s.pos] != '\n' {
				s.next()
			}
		case c == '/' && s.peek(1) == '*':
			s.next()
			s.next()
			for s.pos < len(s.src) && !(s.src[s.pos] == '*' && s.peek(1) == '/') {
				s.next()
			}
			s.next()
			s.next()
		case c == '"' || c == '\'':
			s.scanString(c)
		case c == '`':
			start := s.mark()
			s.next()
			s.scanTemplate(start)
		case c >= '0' && c <= '9' || c == '.' && s.peek(1) >= '0' && s.peek(1) <= '9':
			s.scanNumber()
		case c == '/':
			if s.slashStartsRegex() && s.scanRegex() {
				continue
			}
			s.punct()
		case c == '{':
			s.braces = append(s.braces, false)
			s.punct()
		case c == '}':
			if n := len(s.braces); n > 0 {
				top := s.braces[n-1]
				s.braces = s.braces[:n-1]
				if top {
					// End of a template substitution: the template text continues.
					start := s.mark()
					s.next()
					s.scanTemplate(start)
					continue
				}
			}
			s.punct()
		default:
			r, _ := utf8.DecodeRune(s.src[s.pos:])
			if isIdentStart(r) || c == '\\' {
				s.scanIdent()
			} else {
				s.punct()
			}
		}
	}
	return s.toks
}

type srcMark struct{ line, col int }

func (s *jsScanner) mark() srcMark { return srcMark{s.line, s.col} }

func (s *jsScanner) peek(n int) byte {
	if s.pos+n < len(s.src) {
		return s.src[s.pos+n]
	}
	return 0
}

// next consumes one character and keeps the line and column (in characters) current.
func (s *jsScanner) next() {
	if s.pos >= len(s.src) {
		return
	}
	if s.src[s.pos] == '\n' {
		s.pos++
		s.line++
		s.col = 1
		return
	}
	_, size := utf8.DecodeRune(s.src[s.pos:])
	s.pos += size
	s.col++
}

func (s *jsScanner) emit(kind tokKind, text string, m srcMark) {
	s.toks = append(s.toks, srcTok{kind: kind, text: text, line: m.line, col: m.col})
}

func (s *jsScanner) punct() {
	m := s.mark()
	r, size := utf8.DecodeRune(s.src[s.pos:])
	s.next()
	s.emit(tokPunct, string(r), m)
	_ = size
}

func isIdentStart(r rune) bool {
	return r == '$' || r == '_' || unicode.IsLetter(r)
}

func isIdentPart(r rune) bool {
	return r == '$' || r == '_' || r == 0x200c || r == 0x200d || unicode.IsLetter(r) || unicode.IsDigit(r) ||
		unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r) || unicode.Is(unicode.Pc, r)
}

// scanIdent reads an identifier. A unicode escape (backslash, u, then four hex digits or a braced
// code point) is decoded, so an escaped spelling of a forbidden name is still caught.
func (s *jsScanner) scanIdent() {
	m := s.mark()
	var b strings.Builder
	for s.pos < len(s.src) {
		r, _ := utf8.DecodeRune(s.src[s.pos:])
		if r == '\\' && s.peek(1) == 'u' {
			if v, n, ok := decodeUnicodeEscape(s.src[s.pos+2:]); ok {
				b.WriteRune(v)
				for i := 0; i < 2+n; i++ {
					s.next()
				}
				continue
			}
			break
		}
		if !isIdentPart(r) {
			break
		}
		b.WriteRune(r)
		s.next()
	}
	if b.Len() == 0 {
		// A stray backslash: consume it so the scan always moves forward.
		s.punct()
		return
	}
	s.emit(tokIdent, b.String(), m)
}

// decodeUnicodeEscape decodes the part after the backslash-u of a unicode escape: four hex digits or
// {hex...}. It returns the rune and the number of bytes consumed after the backslash-u.
func decodeUnicodeEscape(b []byte) (rune, int, bool) {
	if len(b) >= 1 && b[0] == '{' {
		end := 1
		for end < len(b) && b[end] != '}' && end < 9 {
			end++
		}
		if end >= len(b) || b[end] != '}' || end == 1 {
			return 0, 0, false
		}
		v, err := strconv.ParseUint(string(b[1:end]), 16, 32)
		if err != nil || v > unicode.MaxRune {
			return 0, 0, false
		}
		return rune(v), end + 1, true
	}
	if len(b) < 4 {
		return 0, 0, false
	}
	v, err := strconv.ParseUint(string(b[:4]), 16, 32)
	if err != nil {
		return 0, 0, false
	}
	return rune(v), 4, true
}

func (s *jsScanner) scanNumber() {
	m := s.mark()
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '_', c == '.':
			s.next()
		case (c == '+' || c == '-') && s.pos > 0 && (s.src[s.pos-1] == 'e' || s.src[s.pos-1] == 'E'):
			s.next()
		default:
			s.emit(tokNumber, "", m)
			return
		}
	}
	s.emit(tokNumber, "", m)
}

// scanString reads a quoted string and records its value with simple escapes decoded.
// A string ends at its closing quote or, when unterminated, at the end of the line.
func (s *jsScanner) scanString(quote byte) {
	m := s.mark()
	s.next()
	var b strings.Builder
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		if c == quote {
			s.next()
			break
		}
		if c == '\n' {
			break
		}
		if c == '\\' {
			s.next()
			if s.pos >= len(s.src) {
				break
			}
			e := s.src[s.pos]
			switch e {
			case 'x':
				if s.pos+2 < len(s.src) {
					if v, err := strconv.ParseUint(string(s.src[s.pos+1:s.pos+3]), 16, 8); err == nil {
						b.WriteRune(rune(v))
						s.next()
						s.next()
						s.next()
						continue
					}
				}
			case 'u':
				if v, n, ok := decodeUnicodeEscape(s.src[s.pos+1:]); ok {
					b.WriteRune(v)
					for i := 0; i < 1+n; i++ {
						s.next()
					}
					continue
				}
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case '\r':
				// Line continuation: skip a following \n as well.
				s.next()
				if s.pos < len(s.src) && s.src[s.pos] == '\n' {
					s.next()
				}
				continue
			case '\n':
				s.next()
				continue
			default:
				b.WriteByte(e)
			}
			s.next()
			continue
		}
		r, _ := utf8.DecodeRune(s.src[s.pos:])
		b.WriteRune(r)
		s.next()
	}
	s.emit(tokString, b.String(), m)
}

// scanTemplate reads template text up to the closing backtick or the next `${`. It runs
// right after the opening backtick or after the `}` that closed a substitution.
func (s *jsScanner) scanTemplate(start srcMark) {
	for s.pos < len(s.src) {
		c := s.src[s.pos]
		switch {
		case c == '\\':
			s.next()
			s.next()
		case c == '`':
			s.next()
			s.emit(tokTemplate, "", start)
			return
		case c == '$' && s.peek(1) == '{':
			s.next()
			s.next()
			s.braces = append(s.braces, true)
			s.emit(tokTemplate, "", start)
			return
		default:
			s.next()
		}
	}
	s.emit(tokTemplate, "", start)
}

// slashStartsRegex decides how a slash is read from the previous token.
func (s *jsScanner) slashStartsRegex() bool {
	n := len(s.toks)
	if n == 0 {
		return true
	}
	last := s.toks[n-1]
	switch last.kind {
	case tokNumber, tokString, tokTemplate, tokRegex:
		return false
	case tokIdent:
		return regexKeywords[last.text]
	}
	switch last.text {
	case ")", "}":
		return s.ambiguousIsRegex
	case "]":
		return false
	case "+", "-":
		if n >= 2 {
			p := s.toks[n-2]
			if p.kind == tokPunct && p.text == last.text && p.line == last.line && p.col+1 == last.col {
				return s.ambiguousIsRegex
			}
		}
	}
	return true
}

// scanRegex reads a regular expression literal on the current line. It reports false (and
// consumes nothing) when no closing slash is found before the end of the line.
func (s *jsScanner) scanRegex() bool {
	i := s.pos + 1
	inClass := false
	for ; i < len(s.src); i++ {
		c := s.src[i]
		if c == '\n' {
			return false
		}
		if c == '\\' {
			i++
			continue
		}
		if c == '[' {
			inClass = true
		} else if c == ']' {
			inClass = false
		} else if c == '/' && !inClass {
			break
		}
	}
	if i >= len(s.src) {
		return false
	}
	m := s.mark()
	for s.pos <= i {
		s.next()
	}
	for s.pos < len(s.src) && (s.src[s.pos] >= 'a' && s.src[s.pos] <= 'z') {
		s.next()
	}
	s.emit(tokRegex, "", m)
	return true
}
