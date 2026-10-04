package flows

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	gotoken "go/token"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestCollectEffects(t *testing.T) {
	reg := newTestRegistry(t)
	reg.MustRegister(&NodeDef{Type: "test.dynamic", EffectsFunc: func(n *Node) []Effect {
		if Stringify(n.Params["op"]) == "delete" {
			return []Effect{EffectDeletes}
		}
		return nil
	}})
	b := newFlow("Effects")
	b.node("start", "test.trigger", nil)
	s1 := b.node("sink", "test.sink", nil)
	s2 := b.node("sink2", "test.sink", nil)
	d := b.node("dyn", "test.dynamic", map[string]any{"op": "delete"})
	off := b.node("off", "test.sink", nil)
	f := b.build()
	f.NodeByID(off).Settings.Disabled = true
	got := CollectEffects(f, reg)
	want := []EffectSummary{
		{Effect: EffectRunsCode, NodeIDs: []string{s1, s2}},
		{Effect: EffectDeletes, NodeIDs: []string{d}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CollectEffects = %+v, want %+v", got, want)
	}
	if IsRisky([]Effect{EffectSendsMessage}) || !IsRisky([]Effect{EffectSendsMessage, EffectDeletes}) {
		t.Fatal("IsRisky mismatch")
	}
}

// A node type whose EffectsFunc repeats an effect is listed once, unknown node
// types are skipped, and neither the flow nor the registry's definitions change.
func TestCollectEffectsDedupesAndIsReadOnly(t *testing.T) {
	reg := newTestRegistry(t)
	reg.MustRegister(&NodeDef{Type: "test.twice", EffectsFunc: func(*Node) []Effect {
		return []Effect{EffectWritesFiles, EffectWritesFiles, EffectSystemChange}
	}})
	b := newFlow("Dedupe")
	b.node("start", "test.trigger", nil)
	tw := b.node("twice", "test.twice", nil)
	b.node("ghost", "test.nope", nil)
	f := b.build()
	before, err := f.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	got := CollectEffects(f, reg)
	want := []EffectSummary{
		{Effect: EffectWritesFiles, NodeIDs: []string{tw}},
		{Effect: EffectSystemChange, NodeIDs: []string{tw}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CollectEffects = %+v, want %+v", got, want)
	}
	after, err := f.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("CollectEffects changed the flow")
	}
	if def, _ := reg.Lookup("test.sink"); !reflect.DeepEqual(def.Effects, []Effect{EffectRunsCode}) {
		t.Fatalf("registry definition changed: %v", def.Effects)
	}
}

func TestLintUntrustedData(t *testing.T) {
	reg := newTestRegistry(t)

	b := newFlow("Direct")
	tr := b.node("hook", "test.untrusted_trigger", nil)
	sink := b.node("shell", "test.sink", map[string]any{"command": "echo {{trigger.data.x}}"})
	b.edge(tr, PortOut, sink)
	issues := LintUntrustedData(b.build(), reg)
	if len(issues) != 1 || issues[0].Code != IssueUntrustedData || issues[0].NodeID != sink ||
		issues[0].Param != "command" || issues[0].Severity != SeverityWarning {
		t.Fatalf("direct = %+v", issues)
	}

	b = newFlow("Trusted")
	tr = b.node("start", "test.trigger", nil)
	sink = b.node("shell", "test.sink", map[string]any{"command": "{{trigger.data.x}}"})
	b.edge(tr, PortOut, sink)
	if issues := LintUntrustedData(b.build(), reg); len(issues) != 0 {
		t.Fatalf("trusted trigger = %+v", issues)
	}

	b = newFlow("Transitive")
	tr = b.node("start", "test.trigger", nil)
	web := b.node("web", "test.web", nil)
	set := b.node("copy", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "t", "value": "{{web.text}}"}}})
	sink = b.node("shell", "test.sink", map[string]any{"command": "{{copy.t}}"})
	b.edge(tr, PortOut, web)
	b.edge(web, PortOut, set)
	b.edge(set, PortOut, sink)
	issues = LintUntrustedData(b.build(), reg)
	if len(issues) != 1 || issues[0].NodeID != sink || !strings.Contains(issues[0].Message, "copy") {
		t.Fatalf("transitive = %+v", issues)
	}

	b = newFlow("Clean")
	tr = b.node("start", "test.trigger", nil)
	echo := b.node("echo", "test.echo", map[string]any{"value": "x"})
	sink = b.node("shell", "test.sink", map[string]any{"command": "{{echo.value}}"})
	b.edge(tr, PortOut, echo)
	b.edge(echo, PortOut, sink)
	if issues := LintUntrustedData(b.build(), reg); len(issues) != 0 {
		t.Fatalf("clean = %+v", issues)
	}
}

// harmlessRefParams returns count params that each hold one reference to the
// clean node "echo". With the prefix "a" they sort before the "command" sink
// param, with "z" after it.
func harmlessRefParams(prefix string, count int) map[string]any {
	params := make(map[string]any, count+1)
	for i := 0; i < count; i++ {
		params[fmt.Sprintf("%s%04d", prefix, i)] = "{{echo.value}}"
	}
	return params
}

// CollectTemplateRefs stops after maxTemplateRefs expressions and drops the rest
// in sorted key order. A flow author controls that order, so the lint must not
// read the cut-off as "no more references": it assumes the worst whenever a taint
// source exists.
func TestLintUntrustedDataRefCap(t *testing.T) {
	reg := newTestRegistry(t)
	reg.MustRegister(&NodeDef{Type: "test.sink2", Category: "test",
		Params: []ParamSpec{
			{Name: "zeta", Kind: ParamText, Templatable: true, SensitiveSink: true},
			{Name: "alpha", Kind: ParamText, Templatable: true, SensitiveSink: true},
			{Name: "plain", Kind: ParamText, Templatable: true},
		}})

	// The sink reference sits behind maxTemplateRefs harmless references.
	cappedParams := func(sinkValue string) map[string]any {
		params := harmlessRefParams("a", maxTemplateRefs)
		params["command"] = sinkValue
		return params
	}
	if refs, problems := CollectTemplateRefs(cappedParams("{{trigger.data.x}}")); len(refs) != maxTemplateRefs || len(problems) != 1 {
		t.Fatalf("setup: %d refs and %d problems, want the cap to cut off the sink reference", len(refs), len(problems))
	}

	t.Run("untrusted trigger", func(t *testing.T) {
		b := newFlow("CapTrigger")
		tr := b.node("hook", "test.untrusted_trigger", nil)
		echo := b.node("echo", "test.echo", nil)
		capped := b.node("capped", "test.sink", cappedParams("{{trigger.data.x}}"))
		b.edge(tr, PortOut, echo)
		b.edge(echo, PortOut, capped)
		issues := LintUntrustedData(b.build(), reg)
		if len(issues) != 1 || issues[0].Code != IssueUntrustedData || issues[0].NodeID != capped ||
			issues[0].Param != "command" || issues[0].Severity != SeverityWarning {
			t.Fatalf("issues = %+v", issues)
		}
	})

	t.Run("tainted upstream node", func(t *testing.T) {
		b := newFlow("CapUpstream")
		tr := b.node("start", "test.trigger", nil)
		echo := b.node("echo", "test.echo", nil)
		web := b.node("web", "test.web", nil)
		capped := b.node("capped", "test.sink", cappedParams("{{web.text}}"))
		after := b.node("after", "test.sink", map[string]any{"command": "{{capped.value}}"})
		b.edge(tr, PortOut, echo)
		b.edge(echo, PortOut, web)
		b.edge(web, PortOut, capped)
		b.edge(capped, PortOut, after)
		issues := LintUntrustedData(b.build(), reg)
		// The capped node counts as tainted, so what it feeds is flagged too.
		if len(issues) != 2 ||
			issues[0].NodeID != capped || issues[0].Param != "command" ||
			issues[1].NodeID != after || issues[1].Param != "command" || !strings.Contains(issues[1].Message, "capped") {
			t.Fatalf("issues = %+v", issues)
		}
	})

	t.Run("no taint source", func(t *testing.T) {
		b := newFlow("CapClean")
		tr := b.node("start", "test.trigger", nil)
		echo := b.node("echo", "test.echo", nil)
		capped := b.node("capped", "test.sink", cappedParams("{{trigger.data.x}}"))
		after := b.node("after", "test.sink", map[string]any{"command": "{{capped.value}}"})
		b.edge(tr, PortOut, echo)
		b.edge(echo, PortOut, capped)
		b.edge(capped, PortOut, after)
		if issues := LintUntrustedData(b.build(), reg); len(issues) != 0 {
			t.Fatalf("a cut-off without any untrusted source must not warn: %+v", issues)
		}
	})

	t.Run("one warning per sink param", func(t *testing.T) {
		// The sink reference comes first and is reported on its own; the cap that
		// follows it must not add a second warning for the same param.
		b := newFlow("CapDedupe")
		tr := b.node("hook", "test.untrusted_trigger", nil)
		params := harmlessRefParams("z", maxTemplateRefs)
		params["command"] = "{{trigger.data.x}}"
		capped := b.node("capped", "test.sink", params)
		b.edge(tr, PortOut, capped)
		issues := LintUntrustedData(b.build(), reg)
		if len(issues) != 1 || issues[0].NodeID != capped || issues[0].Param != "command" {
			t.Fatalf("issues = %+v", issues)
		}
	})

	t.Run("every sink param in definition order", func(t *testing.T) {
		b := newFlow("CapSinks")
		tr := b.node("hook", "test.untrusted_trigger", nil)
		capped := b.node("capped", "test.sink2", harmlessRefParams("a", maxTemplateRefs+5))
		b.edge(tr, PortOut, capped)
		issues := LintUntrustedData(b.build(), reg)
		if len(issues) != 2 || issues[0].Param != "zeta" || issues[1].Param != "alpha" {
			t.Fatalf("issues = %+v, want zeta then alpha (the definition's order)", issues)
		}
		for _, is := range issues {
			if is.NodeID != capped || is.Code != IssueUntrustedData || is.Severity != SeverityWarning {
				t.Fatalf("issue = %+v", is)
			}
		}
	})
}

func TestLintUntrustedDataBoundsMessages(t *testing.T) {
	reg := newTestRegistry(t)
	long := strings.Repeat("k", 5000)

	b := newFlow("LongNames")
	tr := b.node("start", "test.trigger", nil)
	web := b.node(long, "test.web", nil)
	sink := b.node(long+"s", "test.sink", map[string]any{"command": "{{" + long + ".text}}"})
	b.edge(tr, PortOut, web)
	b.edge(web, PortOut, sink)
	issues := LintUntrustedData(b.build(), reg)
	if len(issues) != 1 || issues[0].NodeID != sink {
		t.Fatalf("issues = %+v", issues)
	}
	if got := len(issues[0].Message); got > 300 {
		t.Fatalf("message is %d bytes long, want it bounded", got)
	}
	if strings.Contains(issues[0].Message, long[:100]) {
		t.Fatal("message repeats a long node key in full")
	}

	// A key with control characters must not break a log line.
	b = newFlow("OddKey")
	tr = b.node("hook", "test.untrusted_trigger", nil)
	sink = b.node("bad\nkey\r", "test.sink", map[string]any{"command": "{{trigger.data.x}}"})
	b.edge(tr, PortOut, sink)
	issues = LintUntrustedData(b.build(), reg)
	if len(issues) != 1 || strings.ContainsAny(issues[0].Message, "\r\n") {
		t.Fatalf("issues = %+v", issues)
	}

	// The cut-off warning is bounded the same way.
	b = newFlow("LongCapped")
	tr = b.node("hook", "test.untrusted_trigger", nil)
	sink = b.node(long, "test.sink", harmlessRefParams("a", maxTemplateRefs+1))
	b.edge(tr, PortOut, sink)
	issues = LintUntrustedData(b.build(), reg)
	if len(issues) != 1 || len(issues[0].Message) > 300 || strings.Contains(issues[0].Message, long[:100]) {
		t.Fatalf("capped issues = %+v", issues)
	}
}

func TestLintUntrustedDataIsDeterministicAndReadOnly(t *testing.T) {
	reg := newTestRegistry(t)
	b := newFlow("Many")
	tr := b.node("hook", "test.untrusted_trigger", nil)
	web := b.node("web", "test.web", nil)
	b.edge(tr, PortOut, web)
	prev := web
	for i := 0; i < 12; i++ {
		sink := b.node(fmt.Sprintf("sink%d", i), "test.sink", map[string]any{
			"command": "{{trigger.data.x}} {{web.text}} {{trigger.data.y | upper}}",
		})
		b.edge(prev, PortOut, sink)
		prev = sink
	}
	f := b.build()
	before, err := f.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	first := LintUntrustedData(f, reg)
	// Per sink: one warning for the trigger root and one for web (the second
	// trigger reference in the same param is reported once).
	if len(first) != 12*2 {
		t.Fatalf("got %d issues, want %d", len(first), 12*2)
	}
	for i := 0; i < 25; i++ {
		if again := LintUntrustedData(f, reg); !reflect.DeepEqual(first, again) {
			t.Fatalf("run %d differs:\n%+v\n%+v", i, first, again)
		}
	}
	after, err := f.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("LintUntrustedData changed the flow")
	}
}

// effectOrder drives the publish dialog. A new Effect constant that is not in it
// would be summarised only through the fallback in CollectEffects, so the list must
// stay complete: it holds every Effect constant declared in node.go exactly once.
func TestEffectOrderListsEveryEffect(t *testing.T) {
	file, err := parser.ParseFile(gotoken.NewFileSet(), "node.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	declared := map[string]string{} // value -> constant name
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != gotoken.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs := spec.(*ast.ValueSpec)
			if typ, ok := vs.Type.(*ast.Ident); !ok || typ.Name != "Effect" {
				continue
			}
			for i, name := range vs.Names {
				if i >= len(vs.Values) {
					t.Fatalf("constant %s has no explicit value", name.Name)
				}
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != gotoken.STRING {
					t.Fatalf("constant %s is not a string literal", name.Name)
				}
				value, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatal(err)
				}
				declared[value] = name.Name
			}
		}
	}
	if len(declared) < 6 {
		t.Fatalf("found only %d Effect constants in node.go: %v", len(declared), declared)
	}
	listed := map[Effect]int{}
	for _, e := range effectOrder {
		listed[e]++
	}
	for value, name := range declared {
		if got := listed[Effect(value)]; got != 1 {
			t.Errorf("%s (%q) appears %d times in effectOrder, want exactly once", name, value, got)
		}
	}
	for e := range listed {
		if _, ok := declared[string(e)]; !ok {
			t.Errorf("effectOrder lists %q, which is not a declared Effect constant", e)
		}
	}
}

// The summaries follow effectOrder whatever the document order is, and an effect
// missing from effectOrder is still listed, after the known ones and sorted.
func TestCollectEffectsOrderAndUnknownEffects(t *testing.T) {
	reg := newTestRegistry(t)
	reg.MustRegister(&NodeDef{Type: "test.deleter", Effects: []Effect{EffectDeletes}})
	reg.MustRegister(&NodeDef{Type: "test.custom", EffectsFunc: func(*Node) []Effect {
		return []Effect{"zz_custom", "aa_custom", "zz_custom"}
	}})
	b := newFlow("Order")
	b.node("start", "test.trigger", nil)
	custom := b.node("custom", "test.custom", nil)
	del := b.node("del", "test.deleter", nil)
	sink := b.node("sink", "test.sink", nil)
	del2 := b.node("del2", "test.deleter", nil)
	got := CollectEffects(b.build(), reg)
	want := []EffectSummary{
		{Effect: EffectRunsCode, NodeIDs: []string{sink}},
		{Effect: EffectDeletes, NodeIDs: []string{del, del2}},
		{Effect: "aa_custom", NodeIDs: []string{custom}},
		{Effect: "zz_custom", NodeIDs: []string{custom}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CollectEffects = %+v, want %+v", got, want)
	}
}

// lintCopyChain lints start -> src -> mid -> shell, where src has the given type,
// mid is the node under test and shell reads command, a sink parameter.
func lintCopyChain(t *testing.T, srcType, midType string, midParams map[string]any, command string) []Issue {
	t.Helper()
	b := newFlow("Copy")
	tr := b.node("start", "test.trigger", nil)
	src := b.node("src", srcType, nil)
	mid := b.node("mid", midType, midParams)
	sink := b.node("shell", "test.sink", map[string]any{"command": command})
	b.edge(tr, PortOut, src)
	b.edge(src, PortOut, mid)
	b.edge(mid, PortOut, sink)
	return LintUntrustedData(b.build(), newTestRegistry(t))
}

// logic.merge and logic.set with keep_input copy their inputs into their output
// without any template reference. Untrusted data must not lose its taint there.
func TestLintUntrustedDataThroughInputCopies(t *testing.T) {
	someFields := []any{map[string]any{"name": "t", "value": "x"}}
	tests := []struct {
		name    string
		src     string
		mid     string
		params  map[string]any
		command string
		warns   bool
	}{
		{"merge wait_all", "test.web", TypeMerge, map[string]any{"mode": "wait_all"}, "{{mid.src.text}}", true},
		{"merge default mode", "test.web", TypeMerge, nil, "{{mid.src.text}}", true},
		{"merge append", "test.web", TypeMerge, map[string]any{"mode": "append"}, "{{mid.items[0].text}}", true},
		{"set keep_input true", "test.web", TypeSet, map[string]any{"fields": []any{}, "keep_input": true}, "{{mid.text}}", true},
		{"set keep_input text true", "test.web", TypeSet, map[string]any{"fields": []any{}, "keep_input": "true"}, "{{mid.text}}", true},
		{"set keep_input template", "test.web", TypeSet, map[string]any{"fields": someFields, "keep_input": "{{start.x}}"}, "{{mid.text}}", true},
		{"set keep_input false", "test.web", TypeSet, map[string]any{"fields": someFields, "keep_input": false}, "{{mid.t}}", false},
		{"set without keep_input", "test.web", TypeSet, map[string]any{"fields": someFields}, "{{mid.t}}", false},
		{"merge of clean data", "test.echo", TypeMerge, map[string]any{"mode": "wait_all"}, "{{mid.src.value}}", false},
		{"set keep_input of clean data", "test.echo", TypeSet, map[string]any{"fields": []any{}, "keep_input": true}, "{{mid.value}}", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			issues := lintCopyChain(t, tc.src, tc.mid, tc.params, tc.command)
			if !tc.warns {
				if len(issues) != 0 {
					t.Fatalf("issues = %+v, want none", issues)
				}
				return
			}
			if len(issues) != 1 || issues[0].Code != IssueUntrustedData || issues[0].Param != "command" ||
				issues[0].Severity != SeverityWarning || !strings.Contains(issues[0].Message, "mid") {
				t.Fatalf("issues = %+v, want one warning on command that names mid", issues)
			}
		})
	}
}

// The taint of a copying node follows every incoming edge and passes on through
// further copying nodes.
func TestLintUntrustedDataThroughCopyChains(t *testing.T) {
	reg := newTestRegistry(t)

	// Only one of two merge inputs is tainted, then merge -> set(keep_input).
	b := newFlow("Chain")
	tr := b.node("start", "test.trigger", nil)
	clean := b.node("clean", "test.echo", nil)
	web := b.node("web", "test.web", nil)
	join := b.node("join", TypeMerge, nil)
	keep := b.node("keep", TypeSet, map[string]any{"fields": []any{}, "keep_input": true})
	sink := b.node("shell", "test.sink", map[string]any{"command": "{{keep.join.web.text}}"})
	b.edge(tr, PortOut, clean)
	b.edge(tr, PortOut, web)
	b.edge(clean, PortOut, join)
	b.edge(web, PortOut, join)
	b.edge(join, PortOut, keep)
	b.edge(keep, PortOut, sink)
	issues := LintUntrustedData(b.build(), reg)
	if len(issues) != 1 || issues[0].NodeID != sink || !strings.Contains(issues[0].Message, "keep") {
		t.Fatalf("issues = %+v", issues)
	}

	// Two clean inputs leave the merge clean.
	b = newFlow("CleanChain")
	tr = b.node("start", "test.trigger", nil)
	e1 := b.node("e1", "test.echo", nil)
	e2 := b.node("e2", "test.echo", nil)
	join = b.node("join", TypeMerge, nil)
	sink = b.node("shell", "test.sink", map[string]any{"command": "{{join.e1.value}}"})
	b.edge(tr, PortOut, e1)
	b.edge(tr, PortOut, e2)
	b.edge(e1, PortOut, join)
	b.edge(e2, PortOut, join)
	b.edge(join, PortOut, sink)
	if issues := LintUntrustedData(b.build(), reg); len(issues) != 0 {
		t.Fatalf("clean merge = %+v", issues)
	}
}

func TestPassesInputs(t *testing.T) {
	tests := []struct {
		name string
		node Node
		want bool
	}{
		{"merge", Node{Type: TypeMerge}, true},
		{"set keep_input true", Node{Type: TypeSet, Params: map[string]any{"keep_input": true}}, true},
		{"set keep_input yes", Node{Type: TypeSet, Params: map[string]any{"keep_input": "yes"}}, true},
		{"set keep_input template", Node{Type: TypeSet, Params: map[string]any{"keep_input": "a {{x.y}}"}}, true},
		{"set keep_input false", Node{Type: TypeSet, Params: map[string]any{"keep_input": false}}, false},
		{"set keep_input no", Node{Type: TypeSet, Params: map[string]any{"keep_input": "no"}}, false},
		{"set keep_input escaped template", Node{Type: TypeSet, Params: map[string]any{"keep_input": `\{{x}}`}}, false},
		{"set without keep_input", Node{Type: TypeSet}, false},
		{"echo", Node{Type: "test.echo"}, false},
		{"switch", Node{Type: TypeSwitch}, false},
	}
	for _, tc := range tests {
		if got := passesInputs(&tc.node); got != tc.want {
			t.Errorf("%s: passesInputs = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// A disabled node does not run: it neither warns nor taints what depends on it.
func TestLintUntrustedDataSkipsDisabledNodes(t *testing.T) {
	reg := newTestRegistry(t)

	// A disabled sink gets no warning; the enabled one next to it does.
	b := newFlow("DisabledSink")
	tr := b.node("hook", "test.untrusted_trigger", nil)
	off := b.node("off", "test.sink", map[string]any{"command": "{{trigger.data.x}}"})
	on := b.node("on", "test.sink", map[string]any{"command": "{{trigger.data.x}}"})
	b.edge(tr, PortOut, off)
	b.edge(tr, PortOut, on)
	f := b.build()
	f.NodeByID(off).Settings.Disabled = true
	issues := LintUntrustedData(f, reg)
	if len(issues) != 1 || issues[0].NodeID != on {
		t.Fatalf("disabled sink: issues = %+v, want only the enabled sink", issues)
	}

	// A disabled untrusted node taints nothing.
	b = newFlow("DisabledSource")
	tr = b.node("start", "test.trigger", nil)
	web := b.node("web", "test.web", nil)
	sink := b.node("shell", "test.sink", map[string]any{"command": "{{web.text}}"})
	b.edge(tr, PortOut, web)
	b.edge(web, PortOut, sink)
	f = b.build()
	f.NodeByID(web).Settings.Disabled = true
	if issues := LintUntrustedData(f, reg); len(issues) != 0 {
		t.Fatalf("disabled source: issues = %+v", issues)
	}

	// A disabled copying node does not carry the taint of an enabled source on.
	b = newFlow("DisabledMerge")
	tr = b.node("start", "test.trigger", nil)
	web = b.node("web", "test.web", nil)
	join := b.node("join", TypeMerge, nil)
	sink = b.node("shell", "test.sink", map[string]any{"command": "{{join.web.text}}"})
	b.edge(tr, PortOut, web)
	b.edge(web, PortOut, join)
	b.edge(join, PortOut, sink)
	f = b.build()
	f.NodeByID(join).Settings.Disabled = true
	if issues := LintUntrustedData(f, reg); len(issues) != 0 {
		t.Fatalf("disabled merge: issues = %+v", issues)
	}

	// A disabled untrusted trigger does not make the trigger root untrusted.
	b = newFlow("DisabledTrigger")
	hook := b.node("hook", "test.untrusted_trigger", nil)
	start := b.node("start", "test.trigger", nil)
	sink = b.node("shell", "test.sink", map[string]any{"command": "{{trigger.data.x}}"})
	b.edge(start, PortOut, sink)
	f = b.build()
	f.NodeByID(hook).Settings.Disabled = true
	if issues := LintUntrustedData(f, reg); len(issues) != 0 {
		t.Fatalf("disabled trigger: issues = %+v", issues)
	}
}

// Flows with a cycle cannot be put in order; the validator reports the cycle and
// the lint stays silent.
func TestLintUntrustedDataCyclicFlow(t *testing.T) {
	reg := newTestRegistry(t)
	build := func(backEdge bool) *Flow {
		b := newFlow("Cycle")
		tr := b.node("hook", "test.untrusted_trigger", nil)
		s1 := b.node("s1", "test.sink", map[string]any{"command": "{{trigger.data.x}}"})
		s2 := b.node("s2", "test.sink", map[string]any{"command": "{{trigger.data.x}}"})
		b.edge(tr, PortOut, s1)
		b.edge(s1, PortOut, s2)
		if backEdge {
			b.edge(s2, PortOut, s1)
		}
		return b.build()
	}
	if issues := LintUntrustedData(build(false), reg); len(issues) != 2 {
		t.Fatalf("control without the cycle: issues = %+v, want 2", issues)
	}
	if issues := LintUntrustedData(build(true), reg); issues != nil {
		t.Fatalf("cyclic flow: issues = %+v, want nil", issues)
	}
}

// A reference inside a list or object in a sink parameter still warns, on the
// top-level parameter.
func TestLintUntrustedDataNestedSinkRef(t *testing.T) {
	reg := newTestRegistry(t)
	for name, value := range map[string]any{
		"list":   []any{"ok", "{{trigger.data.x}}"},
		"object": map[string]any{"a": map[string]any{"b": []any{"{{trigger.data.x}}"}}},
	} {
		b := newFlow("Nested")
		tr := b.node("hook", "test.untrusted_trigger", nil)
		sink := b.node("shell", "test.sink", map[string]any{"command": value})
		b.edge(tr, PortOut, sink)
		issues := LintUntrustedData(b.build(), reg)
		if len(issues) != 1 || issues[0].NodeID != sink || issues[0].Param != "command" {
			t.Errorf("%s: issues = %+v, want one warning on command", name, issues)
		}
	}
}
