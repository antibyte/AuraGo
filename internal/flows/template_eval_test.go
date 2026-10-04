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
