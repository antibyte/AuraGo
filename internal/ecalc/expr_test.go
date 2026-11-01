package ecalc

import (
	"reflect"
	"strings"
	"testing"
)

type exprVector struct {
	Src   string               `json:"src"`
	Env   map[string]float64   `json:"env"`
	Lists map[string][]float64 `json:"lists"`
	Want  any                  `json:"want"`
}

type astVector struct {
	Src string `json:"src"`
	AST string `json:"ast"`
}

type depsVector struct {
	Src  string   `json:"src"`
	Deps []string `json:"deps"`
}

func TestExprVectors(t *testing.T) {
	var f struct {
		Expr         []exprVector `json:"expr"`
		EvalError    []exprVector `json:"eval_error"`
		Parse        []astVector  `json:"parse"`
		ParseError   []string     `json:"parse_error"`
		Formula      []astVector  `json:"formula"`
		FormulaError []string     `json:"formula_error"`
		Deps         []depsVector `json:"deps"`
	}
	readJSON(t, "testdata/vectors/core.json", &f)
	if len(f.Expr) == 0 || len(f.Parse) == 0 || len(f.Deps) == 0 {
		t.Fatal("core.json lacks the expression sections")
	}

	for _, v := range f.Expr {
		n, err := ParseExpr(v.Src)
		if err != nil {
			t.Errorf("expr %q: parse: %v", v.Src, err)
			continue
		}
		got, err := Eval(n, Env{Vals: v.Env, Lists: v.Lists})
		if err != nil {
			t.Errorf("expr %q: eval: %v", v.Src, err)
			continue
		}
		if want := vecNum(t, v.Want); !near(got, want, 1e-12) {
			t.Errorf("expr %q = %v, want %v", v.Src, got, want)
		}
	}
	for _, v := range f.EvalError {
		n, err := ParseExpr(v.Src)
		if err != nil {
			t.Errorf("eval_error %q: parse: %v", v.Src, err)
			continue
		}
		if got, err := Eval(n, Env{Vals: v.Env, Lists: v.Lists}); err == nil {
			t.Errorf("eval_error %q: evaluated to %v, want an error", v.Src, got)
		}
	}
	for _, v := range f.Parse {
		n, err := ParseExpr(v.Src)
		if err != nil {
			t.Errorf("parse %q: %v", v.Src, err)
			continue
		}
		if got := n.Sexp(); got != v.AST {
			t.Errorf("parse %q = %s, want %s", v.Src, got, v.AST)
		}
	}
	for _, src := range f.ParseError {
		if n, err := ParseExpr(src); err == nil {
			t.Errorf("parse_error %q: parsed as %s, want an error", src, n.Sexp())
		}
	}
	for _, v := range f.Formula {
		n, err := ParseFormula(v.Src)
		if err != nil {
			t.Errorf("formula %q: %v", v.Src, err)
			continue
		}
		if got := n.Sexp(); got != v.AST {
			t.Errorf("formula %q = %s, want %s", v.Src, got, v.AST)
		}
	}
	for _, src := range f.FormulaError {
		if n, err := ParseFormula(src); err == nil {
			t.Errorf("formula_error %q: parsed as %s, want an error", src, n.Sexp())
		}
	}
	for _, v := range f.Deps {
		n, err := ParseExpr(v.Src)
		if err != nil {
			t.Errorf("deps %q: %v", v.Src, err)
			continue
		}
		got := Deps(n)
		if got == nil {
			got = []string{}
		}
		if !reflect.DeepEqual(got, v.Deps) {
			t.Errorf("deps %q = %v, want %v", v.Src, got, v.Deps)
		}
	}
}

func TestParseErrorsNamePosition(t *testing.T) {
	if _, err := ParseExpr("1 + $"); err == nil || !strings.Contains(err.Error(), "position 4") {
		t.Errorf("lexer error = %v, want a message naming position 4", err)
	}
	if _, err := ParseExpr("sqrt(1, 2)"); err == nil || !strings.Contains(err.Error(), `"sqrt"`) {
		t.Errorf("arity error = %v, want a message naming sqrt", err)
	}
	if _, err := ParseExpr("foo(1)"); err == nil || !strings.Contains(err.Error(), "unknown function") {
		t.Errorf("unknown function error = %v", err)
	}
}
