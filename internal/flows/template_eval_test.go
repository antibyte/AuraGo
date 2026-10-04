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

func TestCollectTemplateRefsIgnoresEscapedBraces(t *testing.T) {
	refs, problems := CollectTemplateRefs(map[string]any{
		"e":    `\{{x}}`,
		"mid":  `a \{{y}} b`,
		"list": []any{`\{{z}}`},
	})
	if len(refs) != 0 || len(problems) != 0 {
		t.Fatalf("escaped braces are text: refs = %+v, problems = %+v", refs, problems)
	}
	refs, problems = CollectTemplateRefs(map[string]any{"t": `\{{x}} {{real}}`})
	if len(refs) != 1 || refs[0].Expr.Root != "real" || len(problems) != 0 {
		t.Fatalf("refs = %+v, problems = %+v; want only the unescaped expression", refs, problems)
	}
}

func TestTemplateEvaluateNestedMapPath(t *testing.T) {
	env := &Env{Location: time.UTC, Roots: map[string]any{
		"a": map[string]any{"b": map[string]any{"c": "deep", "n": 7.0}},
	}}
	for src, want := range map[string]any{
		"{{a.b.c}}":         "deep",
		"{{a.b.n}}":         7.0,
		"{{a.b.c.d}}":       nil,
		"{{a.x.c}}":         nil,
		`<{{a["b"]["c"]}}>`: "<deep>",
	} {
		tpl, err := ParseTemplate(src)
		if err != nil {
			t.Fatalf("parse %q: %v", src, err)
		}
		got, err := tpl.Evaluate(env)
		if err != nil {
			t.Fatalf("evaluate %q: %v", src, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("evaluate %q = %#v, want %#v", src, got, want)
		}
	}
}

func TestEvaluateNilTemplate(t *testing.T) {
	var tpl *Template
	for _, env := range []*Env{nil, evalEnv()} {
		got, err := tpl.Evaluate(env)
		if got != nil || err != nil {
			t.Fatalf("nil template: got %#v, %v; want nil, nil", got, err)
		}
	}
}

func TestTemplateEvaluateOutputCap(t *testing.T) {
	mib := strings.Repeat("x", 1<<20)
	env := &Env{Location: time.UTC, Roots: map[string]any{"a": mib, "empty": ""}}
	evaluate := func(src string) (any, error) {
		t.Helper()
		tpl, err := ParseTemplate(src)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		return tpl.Evaluate(env)
	}

	// 200 copies of a 1 MiB value would be 200 MiB: it must stop at the cap.
	got, err := evaluate(strings.Repeat("{{a}}", 200))
	if err == nil || got != nil || !strings.Contains(err.Error(), "8 MiB") {
		t.Fatalf("200 x 1 MiB: got %T, err %v; want an error naming the 8 MiB limit", got, err)
	}
	if len(err.Error()) > 100 {
		t.Errorf("the error should be short, got %d bytes", len(err.Error()))
	}

	// Exactly at the cap is fine, one byte over is not.
	got, err = evaluate(strings.Repeat("{{a}}", 8))
	if s, _ := got.(string); err != nil || len(s) != maxTemplateOutputBytes {
		t.Fatalf("8 x 1 MiB should fit exactly, got len %d, err %v", len(s), err)
	}
	if _, err = evaluate(strings.Repeat("{{a}}", 8) + "!"); err == nil {
		t.Fatal("8 MiB + 1 byte: expected an error")
	}

	// Literal parts count too, wherever they sit.
	if _, err = evaluate(strings.Repeat("y", 5<<20) + strings.Repeat("{{a}}", 4)); err == nil {
		t.Fatal("5 MiB literal + 4 MiB value: expected an error")
	}
	if _, err = evaluate(strings.Repeat("{{a}}", 4) + strings.Repeat("y", 5<<20)); err == nil {
		t.Fatal("4 MiB value + 5 MiB literal: expected an error")
	}
	if _, err = evaluate(strings.Repeat("y", maxTemplateOutputBytes+1) + "{{empty}}"); err == nil {
		t.Fatal("a literal over the cap: expected an error")
	}

	// A single expression keeps its value; the cap is for assembled text.
	got, err = evaluate("{{a}}")
	if s, _ := got.(string); err != nil || len(s) != 1<<20 {
		t.Fatalf("single expression: len %d, err %v", len(s), err)
	}

	// Through ResolveParams the error names the parameter.
	_, err = ResolveParams(map[string]any{"body": strings.Repeat("{{a}}", 200)}, env)
	if err == nil || !strings.HasPrefix(err.Error(), "body: ") {
		t.Fatalf("ResolveParams: got %v", err)
	}
}

func nestedMaps(n int, leaf any) any {
	v := leaf
	for i := 0; i < n; i++ {
		v = map[string]any{"k": v}
	}
	return v
}

func nestedLists(n int, leaf any) any {
	v := leaf
	for i := 0; i < n; i++ {
		v = []any{v}
	}
	return v
}

func TestResolveDepthLimit(t *testing.T) {
	const tooDeep = "parameters nested deeper than 32 levels"
	env := evalEnv()
	builders := map[string]func(int, any) any{"maps": nestedMaps, "lists": nestedLists}
	for name, build := range builders {
		// ResolveValue: the value itself is level 1, so 32 containers pass.
		got, err := ResolveValue(build(maxParamDepth, "{{a.n}}"), env)
		if err != nil {
			t.Fatalf("%s: depth %d must pass: %v", name, maxParamDepth, err)
		}
		if !reflect.DeepEqual(got, build(maxParamDepth, 2.0)) {
			t.Errorf("%s: leaf was not resolved at the deepest allowed level", name)
		}
		for _, n := range []int{maxParamDepth + 1, 40} {
			_, err = ResolveValue(build(n, "{{a.n}}"), env)
			if err == nil || !strings.Contains(err.Error(), tooDeep) {
				t.Fatalf("%s: depth %d: got %v, want %q", name, n, err, tooDeep)
			}
			if len(err.Error()) > 400 {
				t.Errorf("%s: depth %d: the wrapped error is %d bytes long", name, n, len(err.Error()))
			}
		}

		// ResolveParams: the parameter map is level 1, its values start at level 2.
		params, err := ResolveParams(map[string]any{"p": build(maxParamDepth-1, "{{a.n}}")}, env)
		if err != nil || !reflect.DeepEqual(params["p"], build(maxParamDepth-1, 2.0)) {
			t.Fatalf("%s: params depth %d must pass: %v", name, maxParamDepth, err)
		}
		for _, n := range []int{maxParamDepth, 40} {
			_, err = ResolveParams(map[string]any{"p": build(n, "{{a.n}}")}, env)
			if err == nil || !strings.HasPrefix(err.Error(), "p: ") || !strings.Contains(err.Error(), tooDeep) {
				t.Fatalf("%s: params depth %d: got %v, want %q under p", name, n, err, tooDeep)
			}
		}
	}
}

func TestCollectTemplateRefsDepthLimit(t *testing.T) {
	const tooDeep = "parameters nested deeper than 32 levels"
	cases := []struct {
		name  string
		build func(int, any) any
		step  string
	}{
		{"maps", nestedMaps, ".k"},
		{"lists", nestedLists, "[0]"},
	}
	for _, tc := range cases {
		// Deepest allowed: the leaf expression at level 32 is still collected.
		refs, problems := CollectTemplateRefs(map[string]any{"p": tc.build(maxParamDepth-1, "{{a.n}}")})
		if len(problems) != 0 || len(refs) != 1 || refs[0].Param != "p"+strings.Repeat(tc.step, maxParamDepth-1) {
			t.Fatalf("%s: depth %d: refs = %+v, problems = %+v", tc.name, maxParamDepth, refs, problems)
		}

		// One level too deep, and 40: one problem at the first container over the
		// limit, nothing below it collected, siblings still walked.
		for _, n := range []int{maxParamDepth, 40} {
			refs, problems = CollectTemplateRefs(map[string]any{"p": tc.build(n, "{{a.n}}"), "q": "{{b}}"})
			if len(refs) != 1 || refs[0].Param != "q" || refs[0].Expr.Root != "b" {
				t.Fatalf("%s: depth %d: refs = %+v, want only q", tc.name, n, refs)
			}
			wantPath := "p" + strings.Repeat(tc.step, maxParamDepth-1)
			if len(problems) != 1 || problems[0].Param != wantPath ||
				problems[0].Err == nil || problems[0].Err.Error() != tooDeep {
				t.Fatalf("%s: depth %d: problems = %+v, want one at %q", tc.name, n, problems, wantPath)
			}
		}
	}
}

// Single-expression templates hand out the value from env without copying it.
// This test pins that: it is documented on Evaluate, ResolveValue and
// ResolveParams, and callers rely on treating such values as read-only. Making
// Evaluate copy instead must be a deliberate change, not a side effect.
func TestSingleExpressionSharesContainersWithEnv(t *testing.T) {
	env := evalEnv()
	w := env.Roots["w"].(map[string]any)
	results := w["results"].([]any)
	ptr := func(v any) uintptr { return reflect.ValueOf(v).Pointer() }

	tpl, err := ParseTemplate("{{w.results}}")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got, err := tpl.Evaluate(env)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if ptr(got) != ptr(results) {
		t.Error("Evaluate no longer returns the list shared with env; update the aliasing docs and every caller that relies on it")
	}

	resolved, err := ResolveParams(map[string]any{"list": "{{w.results}}", "obj": "{{w}}"}, env)
	if err != nil {
		t.Fatalf("ResolveParams: %v", err)
	}
	if ptr(resolved["list"]) != ptr(results) || ptr(resolved["obj"]) != ptr(w) {
		t.Error("ResolveParams no longer returns env containers for single-expression strings")
	}

	// The containers of the input itself are rebuilt, never shared with the result.
	in := map[string]any{"l": []any{1.0}, "m": map[string]any{"a": 1.0}}
	out, err := ResolveParams(in, env)
	if err != nil {
		t.Fatalf("ResolveParams: %v", err)
	}
	if ptr(out["l"]) == ptr(in["l"]) || ptr(out["m"]) == ptr(in["m"]) {
		t.Error("ResolveParams must rebuild the containers of its input")
	}
}

// Text that comes out of a template, or that sits in env data, is plain data:
// it is never parsed as a template again.
func TestResolvedValuesAreNotReevaluated(t *testing.T) {
	env := &Env{Location: time.UTC, Roots: map[string]any{
		"t": "{{a.n}}",
		"l": []any{"{{a.n}}"},
		"a": map[string]any{"n": 2.0},
	}}
	got, err := ResolveParams(map[string]any{
		"single": "{{t}}",
		"mixed":  "x {{t}}",
		"list":   "{{l}}",
		"deep":   []any{map[string]any{"v": "{{t}}"}},
	}, env)
	if err != nil {
		t.Fatalf("ResolveParams: %v", err)
	}
	want := map[string]any{
		"single": "{{a.n}}",
		"mixed":  "x {{a.n}}",
		"list":   []any{"{{a.n}}"},
		"deep":   []any{map[string]any{"v": "{{a.n}}"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolveParams = %#v, want %#v", got, want)
	}
}

func TestResolveErrorsAreDeterministic(t *testing.T) {
	broken := func() map[string]any {
		m := make(map[string]any)
		for i := 0; i < 8; i++ {
			m["k"+strconv.Itoa(i)] = "{{bad" + strings.Repeat("x", i)
		}
		return m
	}
	env := evalEnv()
	cases := map[string]func() error{
		"params": func() error { _, err := ResolveParams(broken(), env); return err },
		"nested": func() error { _, err := ResolveParams(map[string]any{"p": broken()}, env); return err },
		"value":  func() error { _, err := ResolveValue(broken(), env); return err },
	}
	for name, run := range cases {
		first := run()
		if first == nil {
			t.Fatalf("%s: expected an error", name)
		}
		if !strings.Contains(first.Error(), "k0: ") || strings.Contains(first.Error(), "k1: ") {
			t.Fatalf("%s: the error should name the first key in order, got %v", name, first)
		}
		for i := 0; i < 20; i++ {
			if again := run(); again == nil || again.Error() != first.Error() {
				t.Fatalf("%s: run %d gave %v, first run gave %v", name, i, again, first)
			}
		}
	}
}

func TestEchoKeyQuotesUnprintableKeys(t *testing.T) {
	for key, want := range map[string]string{
		"title":    "title",
		"my key":   "my key",
		"größe":    "größe",
		"a\nFAKE":  `"a\nFAKE"`,
		"a\x1b[0m": `"a\x1b[0m"`,
		"a\xffb":   `"a\xffb"`,
	} {
		if got := echoKey(key); got != want {
			t.Errorf("echoKey(%q) = %s, want %s", key, got, want)
		}
	}
	// Cut first, then quoted: the result stays short.
	long := strings.Repeat("\n", 500)
	got := echoKey(long)
	if len(got) > 150 || strings.Contains(got, "\n") || !strings.HasSuffix(got, `…"`) {
		t.Errorf("echoKey(500 newlines) = %q", got)
	}

	_, err := ResolveParams(map[string]any{"a\nFAKE": "{{a"}, evalEnv())
	if err == nil || strings.Contains(err.Error(), "\n") || !strings.HasPrefix(err.Error(), `"a\nFAKE": `) {
		t.Fatalf("a key with a line break must not break the error line, got %q", err)
	}
}

const tooManyRefsMsg = "more than 1000 template expressions in the parameters"

func paramBytes(refs []TemplateRef, problems []TemplateProblem) int {
	n := 0
	for _, r := range refs {
		n += len(r.Param)
	}
	for _, p := range problems {
		n += len(p.Param)
	}
	return n
}

// A huge key must not be copied into the path of each of its refs: with 5000
// templated siblings under a 100 KB key that used to retain hundreds of MB.
func TestCollectTemplateRefsBoundsPathsOfHugeKeys(t *testing.T) {
	hugeKey := strings.Repeat("k", 100000)
	siblings := func() map[string]any {
		m := make(map[string]any, 5000)
		for i := 0; i < 5000; i++ {
			m["s"+strconv.Itoa(i)] = "{{a.n}}"
		}
		return m
	}
	cases := map[string]struct {
		params  map[string]any
		wantTop string
	}{
		"top-level key": {map[string]any{hugeKey: siblings()}, truncateForError(hugeKey)},
		"nested key":    {map[string]any{"rows": map[string]any{hugeKey: siblings()}}, "rows"},
		"list in key":   {map[string]any{"rows": map[string]any{hugeKey: []any{siblings()}}}, "rows"},
	}
	for name, tc := range cases {
		refs, problems := CollectTemplateRefs(tc.params)
		if len(refs) != maxTemplateRefs {
			t.Fatalf("%s: %d refs, want %d", name, len(refs), maxTemplateRefs)
		}
		if len(problems) != 1 || problems[0].Err == nil || problems[0].Err.Error() != tooManyRefsMsg {
			t.Fatalf("%s: problems = %+v, want the cap problem only", name, problems)
		}
		if got := paramBytes(refs, problems); got > 2<<20 {
			t.Errorf("%s: the paths hold %d bytes, want well under 2 MB", name, got)
		}
		for _, r := range refs {
			if strings.Contains(r.Param, strings.Repeat("k", 50)) {
				t.Fatalf("%s: a path repeats the huge key: %.80s", name, r.Param)
			}
			if TopParam(r.Param) != tc.wantTop {
				t.Fatalf("%s: TopParam(%.60s) = %.60q, want %.60q", name, r.Param, TopParam(r.Param), tc.wantTop)
			}
		}
	}

	// Also without the cap: a few refs under a huge key keep short paths.
	refs, problems := CollectTemplateRefs(map[string]any{hugeKey: map[string]any{hugeKey: "{{a}}"}})
	if len(problems) != 0 || len(refs) != 1 {
		t.Fatalf("refs = %+v, problems = %+v", refs, problems)
	}
	if want := truncateForError(hugeKey) + "." + truncateForError(hugeKey); refs[0].Param != want {
		t.Errorf("Param = %.80q, want both keys cut", refs[0].Param)
	}
}

func TestCollectTemplateRefsTopParamKeepsShortNames(t *testing.T) {
	hugeKey := strings.Repeat("k", 100000)
	name40 := strings.Repeat("n", maxErrorEchoRunes)
	name41 := name40 + "n"
	refs, problems := CollectTemplateRefs(map[string]any{
		"title": "{{a}}",
		"rows":  []any{map[string]any{"left": "{{b}}"}},
		"cfg":   map[string]any{hugeKey: "{{c}}"},
		name40:  "{{d}}",
		name41:  "{{e}}",
	})
	if len(problems) != 0 {
		t.Fatalf("problems = %+v", problems)
	}
	want := map[string]string{
		"a": "title",
		"b": "rows",
		"c": "cfg",
		"d": name40, // 40 runes: the full name comes back
		"e": name40 + "…",
	}
	if len(refs) != len(want) {
		t.Fatalf("refs = %+v", refs)
	}
	for _, r := range refs {
		if got := TopParam(r.Param); got != want[r.Expr.Root] {
			t.Errorf("TopParam of the ref to %s = %.60q, want %.60q", r.Expr.Root, got, want[r.Expr.Root])
		}
	}
}

func TestCollectTemplateRefsCap(t *testing.T) {
	// Exactly at the cap: no problem. One over: exactly one cap problem.
	refs, problems := CollectTemplateRefs(map[string]any{"t": strings.Repeat("{{a}}", maxTemplateRefs)})
	if len(refs) != maxTemplateRefs || len(problems) != 0 {
		t.Fatalf("1000 refs: %d refs, problems = %+v", len(refs), problems)
	}
	refs, problems = CollectTemplateRefs(map[string]any{"t": strings.Repeat("{{a}}", maxTemplateRefs+1)})
	if len(refs) != maxTemplateRefs || len(problems) != 1 ||
		problems[0].Param != "t" || problems[0].Err.Error() != tooManyRefsMsg {
		t.Fatalf("1001 refs: %d refs, problems = %+v", len(refs), problems)
	}

	// The same across many parameters; the problem names the one that overflowed.
	params := make(map[string]any)
	for i := 0; i <= maxTemplateRefs; i++ {
		params["p"+strconv.Itoa(10000+i)] = "{{a}}"
	}
	refs, problems = CollectTemplateRefs(params)
	if len(refs) != maxTemplateRefs || len(problems) != 1 || problems[0].Param != "p"+strconv.Itoa(10000+maxTemplateRefs) {
		t.Fatalf("1001 params: %d refs, problems = %+v", len(refs), problems)
	}

	// Refs and problems share the budget; nothing is collected after the cap.
	params = make(map[string]any)
	for i := 0; i < maxTemplateRefs-1; i++ {
		params["a"+strconv.Itoa(10000+i)] = "{{a}}"
	}
	for _, k := range []string{"z0", "z1", "z2"} {
		params[k] = "{{bad"
	}
	params["zz"] = "{{late}}"
	refs, problems = CollectTemplateRefs(params)
	if len(refs) != maxTemplateRefs-1 {
		t.Fatalf("%d refs, want %d", len(refs), maxTemplateRefs-1)
	}
	if len(problems) != 2 || problems[0].Param != "z0" || problems[1].Param != "z1" || problems[1].Err.Error() != tooManyRefsMsg {
		t.Fatalf("problems = %+v, want the z0 parse problem and the cap problem at z1", problems)
	}
	if _, ok := problems[0].Err.(*TemplateError); !ok {
		t.Errorf("the first problem should be the parse error, got %T", problems[0].Err)
	}

	// Depth problems count against the same budget.
	params = map[string]any{"deep": nestedMaps(40, "{{a}}")}
	for i := 0; i < maxTemplateRefs; i++ {
		params["x"+strconv.Itoa(10000+i)] = "{{a}}"
	}
	refs, problems = CollectTemplateRefs(params)
	if len(refs)+len(problems) != maxTemplateRefs+1 || problems[len(problems)-1].Err.Error() != tooManyRefsMsg {
		t.Fatalf("%d refs, problems = %+v", len(refs), problems)
	}
}

// Parameters built in Go can share a child between two keys, so a walk over
// them is exponential in the depth. The cap stops it long before that matters.
func TestCollectTemplateRefsSharedChildrenTerminate(t *testing.T) {
	child := any(map[string]any{"leaf": "{{a.n}}"})
	for i := 0; i < 20; i++ {
		child = map[string]any{"l": child, "r": child}
	}
	start := time.Now()
	refs, problems := CollectTemplateRefs(map[string]any{"p": child})
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("the walk took %v", elapsed)
	}
	if len(refs) != maxTemplateRefs || len(problems) != 1 || problems[0].Err.Error() != tooManyRefsMsg {
		t.Fatalf("%d refs, problems = %+v", len(refs), problems)
	}
	if got := paramBytes(refs, problems); got > 1<<20 {
		t.Errorf("the paths hold %d bytes", got)
	}
}

// A single expression is not covered by the cap on assembled template text, so
// the join filter bounds its own result.
func TestTemplateEvaluateJoinIsBounded(t *testing.T) {
	items := make([]any, 200)
	for i := range items {
		items[i] = "x"
	}
	env := &Env{Location: time.UTC, Roots: map[string]any{"x": items}}
	sep := strings.Repeat("-", 1<<20)
	for _, src := range []string{
		`{{x | join("` + sep + `")}}`,
		`a {{x | join("` + sep + `")}} b`,
	} {
		tpl, err := ParseTemplate(src)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		got, err := tpl.Evaluate(env)
		if err == nil || got != nil {
			t.Fatalf("200 items x 1 MiB separator: got %T, err %v; want an error", got, err)
		}
		if !strings.Contains(err.Error(), "join result would exceed") || len(err.Error()) > 400 {
			t.Errorf("the error should be short and name the limit, got %d bytes: %.200s", len(err.Error()), err)
		}
	}
}
