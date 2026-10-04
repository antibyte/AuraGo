package flows

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func evalEnv() *Env {
	return &Env{Location: time.UTC, Roots: map[string]any{
		"w": map[string]any{
			"count":   5.0,
			"results": []any{map[string]any{"title": "A"}, map[string]any{"title": "B"}},
		},
		"s": "Welt",
		"a": map[string]any{"n": 2.0, "s": "t"},
	}}
}

func TestTemplateEvaluate(t *testing.T) {
	cases := []struct {
		src  string
		want any
	}{
		{"{{w.results}}", []any{map[string]any{"title": "A"}, map[string]any{"title": "B"}}},
		{"{{w.count}}", 5.0},
		{"Treffer: {{w.count}}", "Treffer: 5"},
		{"{{w.results[-1].title}}", "B"},
		{"{{w.results[9].title}}", nil},
		{"{{missing.x}}", nil},
		{"[{{missing.x}}]", "[]"},
		{"Hallo {{s | upper}}!", "Hallo WELT!"},
		{`{{w.results | pluck("title") | join("+")}}`, "A+B"},
		{"{{w.results[0]}} x", `{"title":"A"} x`},
		{"plain", "plain"},
	}
	env := evalEnv()
	for _, tc := range cases {
		tpl, err := ParseTemplate(tc.src)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.src, err)
		}
		got, err := tpl.Evaluate(env)
		if err != nil {
			t.Fatalf("evaluate %q: %v", tc.src, err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("evaluate %q = %#v, want %#v", tc.src, got, tc.want)
		}
	}
}

func TestTemplateEvaluatePaths(t *testing.T) {
	cases := []struct {
		src  string
		want any
	}{
		{"{{w.results[-2].title}}", "A"},
		{"{{w.results[-3]}}", nil},
		{"{{s[0]}}", nil},
		{"{{s.x}}", nil},
		{"{{w.count.x}}", nil},
		{`{{a["s"]}}`, "t"},
		{`{{w["results"][1]["title"]}}`, "B"},
		{`{{missing | default("d")}}`, "d"},
	}
	env := evalEnv()
	for _, tc := range cases {
		tpl, err := ParseTemplate(tc.src)
		if err != nil {
			t.Fatalf("parse %q: %v", tc.src, err)
		}
		got, err := tpl.Evaluate(env)
		if err != nil {
			t.Fatalf("evaluate %q: %v", tc.src, err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("evaluate %q = %#v, want %#v", tc.src, got, tc.want)
		}
	}
}

func TestTemplateEvaluateNilEnv(t *testing.T) {
	for src, want := range map[string]any{
		"{{x}}":                 nil,
		"{{x.y[0]}}":            nil,
		`{{x | default("d")}}`:  "d",
		"a{{x}}b":               "ab",
		`{{x | default("d")}}!`: "d!",
		`\{{x}} {{x | count}}`:  "{{x}} 0",
	} {
		tpl, err := ParseTemplate(src)
		if err != nil {
			t.Fatalf("parse %q: %v", src, err)
		}
		got, err := tpl.Evaluate(nil)
		if err != nil {
			t.Fatalf("evaluate %q: %v", src, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("evaluate %q = %#v, want %#v", src, got, want)
		}
	}
}

func TestTemplateEvaluateFilterError(t *testing.T) {
	tpl, err := ParseTemplate(`{{s | date("YYYY")}}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := tpl.Evaluate(evalEnv()); err == nil || !strings.Contains(err.Error(), "date") {
		t.Fatalf("expected a date filter error, got %v", err)
	}
}

func TestTemplateEvaluateErrorNamesShortExpression(t *testing.T) {
	src := `s | date("YYYY")`
	tpl, err := ParseTemplate("x {{" + src + "}} y")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, err = tpl.Evaluate(evalEnv())
	if err == nil {
		t.Fatal("expected an error")
	}
	// A short expression is echoed whole, so the author can find it.
	if !strings.Contains(err.Error(), strconv.Quote(src)) {
		t.Fatalf("error should name the expression %s, got %v", strconv.Quote(src), err)
	}
}

// A hand-built Expr must go through applyFilter's checks: an unknown filter or a
// wrong argument count is an error, never a nil-function panic.
func TestEvaluateHandBuiltExprRejectsBadFilters(t *testing.T) {
	cases := []struct {
		name    string
		filters []FilterCall
		want    string
	}{
		{"unknown", []FilterCall{{Name: "nope"}}, "unknown filter"},
		{"unknown after valid", []FilterCall{{Name: "upper"}, {Name: "nope"}}, "unknown filter"},
		{"too many args", []FilterCall{{Name: "upper", Args: []any{"x"}}}, "takes 0 argument"},
		{"too few args", []FilterCall{{Name: "replace", Args: []any{"x"}}}, "takes 2 argument"},
	}
	for _, tc := range cases {
		expr := &Expr{Root: "s", Filters: tc.filters, Source: "s | hand-built"}
		single := &Template{parts: []templatePart{{expr: expr}}}
		multi := &Template{parts: []templatePart{{literal: "a "}, {expr: expr}}}
		for label, tpl := range map[string]*Template{"single": single, "multi": multi} {
			got, err := tpl.Evaluate(evalEnv())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("%s/%s: want an error containing %q, got value %#v, err %v", tc.name, label, tc.want, got, err)
			}
		}
	}
}

// Expr.Source is stored whole, but error messages must not repeat it unbounded.
func TestEvaluateErrorBoundsExpressionEcho(t *testing.T) {
	huge := strings.Repeat("x", 100000)
	longRun := strings.Repeat("x", 50)

	check := func(label string, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s: expected an error", label)
		}
		msg := err.Error()
		if len(msg) > 400 {
			t.Errorf("%s: error is %d bytes long, want a bounded message", label, len(msg))
		}
		if strings.Contains(msg, longRun) {
			t.Errorf("%s: error repeats a long run of the expression: %.120s", label, msg)
		}
		if !strings.Contains(msg, "…") {
			t.Errorf("%s: a cut expression should end in an ellipsis: %.120s", label, msg)
		}
	}

	// Parsed: the huge text sits in a filter argument; round then fails on "Welt".
	tpl, err := ParseTemplate(`{{ s | replace("a", "` + huge + `") | round }}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(tpl.Exprs()[0].Source) < len(huge) {
		t.Fatal("Expr.Source must keep the whole expression")
	}
	_, err = tpl.Evaluate(evalEnv())
	check("parsed", err)
	_, err = ResolveParams(map[string]any{"p": []any{`pre {{ s | replace("a", "` + huge + `") | round }}`}}, evalEnv())
	check("resolved", err)

	// Hand-built: huge Source, huge filter name.
	hand := &Template{parts: []templatePart{{expr: &Expr{
		Root:    "s",
		Filters: []FilterCall{{Name: huge}},
		Source:  huge,
	}}}}
	_, err = hand.Evaluate(evalEnv())
	check("hand-built", err)
}

func TestResolveParams(t *testing.T) {
	params := map[string]any{
		"title":   "X {{a.n}}",
		"list":    []any{"{{a.n}}", 3.0},
		"nested":  map[string]any{"k": "{{a.s}}"},
		"plain":   true,
		"escaped": `\{{a.n}}`,
	}
	got, err := ResolveParams(params, evalEnv())
	if err != nil {
		t.Fatalf("ResolveParams: %v", err)
	}
	want := map[string]any{
		"title":   "X 2",
		"list":    []any{2.0, 3.0},
		"nested":  map[string]any{"k": "t"},
		"plain":   true,
		"escaped": "{{a.n}}",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolveParams = %#v, want %#v", got, want)
	}
	if params["title"] != "X {{a.n}}" {
		t.Fatal("ResolveParams must not modify its input")
	}
}

func TestResolveParamsKeepsNestedInput(t *testing.T) {
	params := map[string]any{
		"list":   []any{"{{a.n}}", 3.0},
		"nested": map[string]any{"k": "{{a.s}}"},
	}
	if _, err := ResolveParams(params, evalEnv()); err != nil {
		t.Fatalf("ResolveParams: %v", err)
	}
	if params["list"].([]any)[0] != "{{a.n}}" || params["nested"].(map[string]any)["k"] != "{{a.s}}" {
		t.Fatalf("nested input was modified: %#v", params)
	}
}

func TestResolveParamsEmptyAndPassthrough(t *testing.T) {
	got, err := ResolveParams(nil, evalEnv())
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("ResolveParams(nil) = %#v, %v; want an empty map", got, err)
	}
	for _, v := range []any{nil, true, 2.5, "no template", []any{}, map[string]any{}} {
		out, err := ResolveValue(v, evalEnv())
		if err != nil || !reflect.DeepEqual(out, v) {
			t.Errorf("ResolveValue(%#v) = %#v, %v; want the value unchanged", v, out, err)
		}
	}
}

func TestResolveParamsReportsPath(t *testing.T) {
	_, err := ResolveParams(map[string]any{"rows": []any{map[string]any{"left": "{{a"}}}, evalEnv())
	if err == nil || !strings.Contains(err.Error(), "rows") || !strings.Contains(err.Error(), "left") {
		t.Fatalf("error must name the parameter path, got %v", err)
	}
}

func TestResolveParamsBoundsKeyEcho(t *testing.T) {
	hugeKey := strings.Repeat("k", 100000)
	for label, params := range map[string]map[string]any{
		"top-level": {hugeKey: "{{a"},
		"nested":    {"p": map[string]any{hugeKey: "{{a"}},
	} {
		_, err := ResolveParams(params, evalEnv())
		if err == nil {
			t.Fatalf("%s: expected an error", label)
		}
		if len(err.Error()) > 400 || strings.Contains(err.Error(), strings.Repeat("k", 50)) {
			t.Errorf("%s: error repeats the key unbounded (%d bytes)", label, len(err.Error()))
		}
	}
	if got := echoKey("title"); got != "title" {
		t.Errorf("echoKey(short) = %q", got)
	}
	exact := strings.Repeat("ä", maxErrorEchoRunes)
	if got := echoKey(exact); got != exact {
		t.Errorf("echoKey(40 runes) = %q, want it unchanged", got)
	}
	if got := echoKey(exact + "ä"); got != exact+"…" {
		t.Errorf("echoKey(41 runes) = %q", got)
	}
}

func TestCollectTemplateRefs(t *testing.T) {
	params := map[string]any{
		"title": "{{a.x}} {{b.y}}",
		"rows":  []any{map[string]any{"left": "{{c}}", "right": "{{bad"}},
		"n":     1.0,
	}
	refs, problems := CollectTemplateRefs(params)
	var got []string
	for _, r := range refs {
		got = append(got, r.Param+"="+r.Expr.Root)
	}
	want := []string{"rows[0].left=c", "title=a", "title=b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("refs = %v, want %v", got, want)
	}
	if len(problems) != 1 || problems[0].Param != "rows[0].right" {
		t.Fatalf("problems = %+v", problems)
	}
	if TopParam("rows[0].left") != "rows" || TopParam("title") != "title" || TopParam("a.b") != "a" {
		t.Fatal("TopParam mismatch")
	}
}
