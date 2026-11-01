package ecalc

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Node is one node of a parsed expression. T selects the shape:
//
//	"num"  V, Raw (the literal as written)
//	"id"   Name
//	"neg"  A       "pos"  A
//	"bin"  Op, L, R (Op is + - * / ^ < <= > >= == !=)
//	"call" Name, Args
//	"eq"   Parts (formulas only: expr = expr = ...)
type Node struct {
	T     string
	V     float64
	Raw   string
	Name  string
	Op    string
	A     *Node
	L, R  *Node
	Args  []*Node
	Parts []*Node
}

// Env holds the values an expression may reference: scalars by name and lists (for sum and par) by name.
type Env struct {
	Vals  map[string]float64
	Lists map[string][]float64
}

// funcArity is the fixed function whitelist: name -> {min, max} argument count (max -1 = unbounded).
var funcArity = map[string][2]int{
	"sqrt": {1, 1}, "ln": {1, 1}, "log10": {1, 1}, "exp": {1, 1}, "sin": {1, 1}, "cos": {1, 1},
	"tan": {1, 1}, "atan": {1, 1}, "abs": {1, 1}, "round": {1, 1},
	"min": {2, -1}, "max": {2, -1}, "if": {3, 3}, "sum": {1, 1}, "par": {1, 1},
}

func isFunc(name string) bool {
	_, ok := funcArity[name]
	return ok
}

type tokKind int

const (
	tokEOF tokKind = iota
	tokNum
	tokIdent
	tokOp
)

type token struct {
	kind tokKind
	text string
	pos  int
}

func isDigit(c byte) bool      { return c >= '0' && c <= '9' }
func isIdentStart(c byte) bool { return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isIdentPart(c byte) bool  { return isIdentStart(c) || isDigit(c) }

// scanNumber returns the end of the number literal starting at s[i]:
// digits ['.' digits] [('e'|'E') ['+'|'-'] digits]  or  '.' digits ...
func scanNumber(s string, i int) int {
	j := i
	for j < len(s) && isDigit(s[j]) {
		j++
	}
	if j+1 < len(s) && s[j] == '.' && isDigit(s[j+1]) {
		j++
		for j < len(s) && isDigit(s[j]) {
			j++
		}
	}
	if j < len(s) && (s[j] == 'e' || s[j] == 'E') {
		k := j + 1
		if k < len(s) && (s[k] == '+' || s[k] == '-') {
			k++
		}
		if k < len(s) && isDigit(s[k]) {
			for k < len(s) && isDigit(s[k]) {
				k++
			}
			j = k
		}
	}
	return j
}

func lex(src string) ([]token, error) {
	var toks []token
	i := 0
	for i < len(src) {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case isDigit(c) || (c == '.' && i+1 < len(src) && isDigit(src[i+1])):
			j := scanNumber(src, i)
			toks = append(toks, token{tokNum, src[i:j], i})
			i = j
		case isIdentStart(c):
			j := i + 1
			for j < len(src) && isIdentPart(src[j]) {
				j++
			}
			toks = append(toks, token{tokIdent, src[i:j], i})
			i = j
		default:
			if i+1 < len(src) {
				switch two := src[i : i+2]; two {
				case "<=", ">=", "==", "!=":
					toks = append(toks, token{tokOp, two, i})
					i += 2
					continue
				}
			}
			if strings.IndexByte("+-*/^(),<>=", c) < 0 {
				r, _ := utf8.DecodeRuneInString(src[i:])
				return nil, fmt.Errorf("unexpected character %q at position %d", r, i)
			}
			toks = append(toks, token{tokOp, src[i : i+1], i})
			i++
		}
	}
	toks = append(toks, token{tokEOF, "", len(src)})
	return toks, nil
}

type parser struct {
	toks []token
	i    int
}

func (p *parser) peek() token { return p.toks[p.i] }

func (p *parser) next() token {
	t := p.toks[p.i]
	if t.kind != tokEOF {
		p.i++
	}
	return t
}

func (p *parser) isOp(s string) bool {
	t := p.peek()
	return t.kind == tokOp && t.text == s
}

func (t token) String() string {
	if t.kind == tokEOF {
		return "end of input"
	}
	return fmt.Sprintf("%q", t.text)
}

func (p *parser) unexpected() error {
	t := p.peek()
	return fmt.Errorf("unexpected %s at position %d", t, t.pos)
}

// ParseExpr parses one expression (no '=').
func ParseExpr(src string) (*Node, error) {
	toks, err := lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	n, err := p.parseCmp()
	if err != nil {
		return nil, err
	}
	if p.peek().kind != tokEOF {
		return nil, p.unexpected()
	}
	return n, nil
}

// ParseFormula parses a display formula: expr ('=' expr)*. The result is an "eq" node with one part per expression.
func ParseFormula(src string) (*Node, error) {
	toks, err := lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	var parts []*Node
	for {
		n, err := p.parseCmp()
		if err != nil {
			return nil, err
		}
		parts = append(parts, n)
		if !p.isOp("=") {
			break
		}
		p.next()
	}
	if p.peek().kind != tokEOF {
		return nil, p.unexpected()
	}
	return &Node{T: "eq", Parts: parts}, nil
}

// cmp := add (('<' | '<=' | '>' | '>=' | '==' | '!=') add)?
func (p *parser) parseCmp() (*Node, error) {
	l, err := p.parseAdd()
	if err != nil {
		return nil, err
	}
	if t := p.peek(); t.kind == tokOp {
		switch t.text {
		case "<", "<=", ">", ">=", "==", "!=":
			p.next()
			r, err := p.parseAdd()
			if err != nil {
				return nil, err
			}
			return &Node{T: "bin", Op: t.text, L: l, R: r}, nil
		}
	}
	return l, nil
}

func (p *parser) parseAdd() (*Node, error) {
	l, err := p.parseMul()
	if err != nil {
		return nil, err
	}
	for p.isOp("+") || p.isOp("-") {
		op := p.next().text
		r, err := p.parseMul()
		if err != nil {
			return nil, err
		}
		l = &Node{T: "bin", Op: op, L: l, R: r}
	}
	return l, nil
}

func (p *parser) parseMul() (*Node, error) {
	l, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.isOp("*") || p.isOp("/") {
		op := p.next().text
		r, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		l = &Node{T: "bin", Op: op, L: l, R: r}
	}
	return l, nil
}

// unary := ('-' | '+') unary | pow
func (p *parser) parseUnary() (*Node, error) {
	if p.isOp("-") || p.isOp("+") {
		t := "neg"
		if p.next().text == "+" {
			t = "pos"
		}
		a, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &Node{T: t, A: a}, nil
	}
	return p.parsePow()
}

// pow := atom ('^' unary)?  (right-associative; -2^2 = -(2^2); 2^-1 is fine)
func (p *parser) parsePow() (*Node, error) {
	base, err := p.parseAtom()
	if err != nil {
		return nil, err
	}
	if p.isOp("^") {
		p.next()
		exp, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &Node{T: "bin", Op: "^", L: base, R: exp}, nil
	}
	return base, nil
}

func (p *parser) parseAtom() (*Node, error) {
	t := p.peek()
	switch {
	case t.kind == tokNum:
		p.next()
		v, err := strconv.ParseFloat(t.text, 64)
		if err != nil && !errors.Is(err, strconv.ErrRange) {
			return nil, fmt.Errorf("bad number %q at position %d", t.text, t.pos)
		}
		return &Node{T: "num", V: v, Raw: t.text}, nil
	case t.kind == tokIdent:
		p.next()
		if !p.isOp("(") {
			return &Node{T: "id", Name: t.text}, nil
		}
		return p.parseCall(t)
	case t.kind == tokOp && t.text == "(":
		p.next()
		n, err := p.parseCmp()
		if err != nil {
			return nil, err
		}
		if !p.isOp(")") {
			return nil, fmt.Errorf("expected ')' but found %s at position %d", p.peek(), p.peek().pos)
		}
		p.next()
		return n, nil
	}
	return nil, p.unexpected()
}

func (p *parser) parseCall(name token) (*Node, error) {
	arity, ok := funcArity[name.text]
	if !ok {
		return nil, fmt.Errorf("unknown function %q at position %d", name.text, name.pos)
	}
	p.next() // '('
	var args []*Node
	if !p.isOp(")") {
		for {
			a, err := p.parseCmp()
			if err != nil {
				return nil, err
			}
			args = append(args, a)
			if !p.isOp(",") {
				break
			}
			p.next()
		}
	}
	if !p.isOp(")") {
		return nil, fmt.Errorf("expected ')' but found %s at position %d", p.peek(), p.peek().pos)
	}
	p.next()
	if len(args) < arity[0] || (arity[1] >= 0 && len(args) > arity[1]) {
		return nil, fmt.Errorf("wrong number of arguments (%d) for function %q at position %d", len(args), name.text, name.pos)
	}
	return &Node{T: "call", Name: name.text, Args: args}, nil
}

// Sexp prints the tree in prefix notation: (op l r), (neg a), (pos a), (fn arg ...), (= part ...); numbers as
// written, identifiers by name. The shared vectors compare trees in this form.
func (n *Node) Sexp() string {
	switch n.T {
	case "num":
		return n.Raw
	case "id":
		return n.Name
	case "neg", "pos":
		return "(" + n.T + " " + n.A.Sexp() + ")"
	case "bin":
		return "(" + n.Op + " " + n.L.Sexp() + " " + n.R.Sexp() + ")"
	case "call":
		return "(" + n.Name + sexpList(n.Args) + ")"
	case "eq":
		return "(=" + sexpList(n.Parts) + ")"
	}
	return "?"
}

func sexpList(nodes []*Node) string {
	var b strings.Builder
	for _, a := range nodes {
		b.WriteByte(' ')
		b.WriteString(a.Sexp())
	}
	return b.String()
}

// Deps returns the sorted, unique identifiers an expression uses, without function names and without pi.
func Deps(n *Node) []string {
	seen := map[string]bool{}
	var walk func(*Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		if n.T == "id" && n.Name != "pi" && !isFunc(n.Name) {
			seen[n.Name] = true
		}
		walk(n.A)
		walk(n.L)
		walk(n.R)
		for _, a := range n.Args {
			walk(a)
		}
		for _, a := range n.Parts {
			walk(a)
		}
	}
	walk(n)
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Eval evaluates the tree with IEEE-754 double semantics: division by zero gives +-Inf or NaN, never an error.
// Unknown identifiers, lists used outside sum/par, and a sum/par argument that is not a list are errors.
func Eval(n *Node, env Env) (float64, error) {
	switch n.T {
	case "num":
		return n.V, nil
	case "id":
		if n.Name == "pi" {
			return math.Pi, nil
		}
		if v, ok := env.Vals[n.Name]; ok {
			return v, nil
		}
		if _, ok := env.Lists[n.Name]; ok {
			return 0, fmt.Errorf("list %q used outside sum() or par()", n.Name)
		}
		return 0, fmt.Errorf("unknown identifier %q", n.Name)
	case "neg", "pos":
		a, err := Eval(n.A, env)
		if err != nil {
			return 0, err
		}
		if n.T == "neg" {
			return -a, nil
		}
		return a, nil
	case "bin":
		l, err := Eval(n.L, env)
		if err != nil {
			return 0, err
		}
		r, err := Eval(n.R, env)
		if err != nil {
			return 0, err
		}
		return evalBin(n.Op, l, r), nil
	case "call":
		return evalCall(n, env)
	}
	return 0, fmt.Errorf("cannot evaluate a %q node", n.T)
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func evalBin(op string, l, r float64) float64 {
	switch op {
	case "+":
		return l + r
	case "-":
		return l - r
	case "*":
		return l * r
	case "/":
		return l / r
	case "^":
		return math.Pow(l, r)
	case "<":
		return b2f(l < r)
	case "<=":
		return b2f(l <= r)
	case ">":
		return b2f(l > r)
	case ">=":
		return b2f(l >= r)
	case "==":
		return b2f(l == r)
	case "!=":
		return b2f(l != r)
	}
	return math.NaN()
}

// log10 is math.Log10 made exact for exact powers of ten, like the JavaScript Math.log10.
func log10(x float64) float64 {
	r := math.Log10(x)
	if n := math.Round(r); math.Abs(r-n) < 1e-9 && math.Abs(n) <= 300 && x == math.Pow10(int(n)) {
		return n
	}
	return r
}

func evalCall(n *Node, env Env) (float64, error) {
	switch n.Name {
	case "if":
		c, err := Eval(n.Args[0], env)
		if err != nil {
			return 0, err
		}
		if c != 0 && !math.IsNaN(c) {
			return Eval(n.Args[1], env)
		}
		return Eval(n.Args[2], env)
	case "sum", "par":
		id := n.Args[0]
		list, ok := env.Lists[id.Name]
		if id.T != "id" || !ok {
			return 0, fmt.Errorf("%s() needs a list variable", n.Name)
		}
		s := 0.0
		for _, x := range list {
			if n.Name == "sum" {
				s += x
			} else {
				s += 1 / x
			}
		}
		if n.Name == "par" {
			return 1 / s, nil
		}
		return s, nil
	}
	args := make([]float64, len(n.Args))
	for i, a := range n.Args {
		v, err := Eval(a, env)
		if err != nil {
			return 0, err
		}
		args[i] = v
	}
	switch n.Name {
	case "sqrt":
		return math.Sqrt(args[0]), nil
	case "ln":
		return math.Log(args[0]), nil
	case "log10":
		return log10(args[0]), nil
	case "exp":
		return math.Exp(args[0]), nil
	case "sin":
		return math.Sin(args[0]), nil
	case "cos":
		return math.Cos(args[0]), nil
	case "tan":
		return math.Tan(args[0]), nil
	case "atan":
		return math.Atan(args[0]), nil
	case "abs":
		return math.Abs(args[0]), nil
	case "round":
		return math.Round(args[0]), nil
	case "min", "max":
		m := args[0]
		for _, a := range args[1:] {
			if n.Name == "min" {
				m = math.Min(m, a)
			} else {
				m = math.Max(m, a)
			}
		}
		return m, nil
	}
	return 0, fmt.Errorf("unknown function %q", n.Name)
}
