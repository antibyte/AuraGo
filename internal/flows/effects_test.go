package flows

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	gotoken "go/token"
	"os"
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
// stay complete: it holds every Effect constant declared in the package's
// non-test source files exactly once.
func TestEffectOrderListsEveryEffect(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := gotoken.NewFileSet()
	declared := map[string]string{} // value -> constant name
	for _, entry := range entries {
		fileName := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(fileName, ".go") || strings.HasSuffix(fileName, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, fileName, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
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
	}
	if len(declared) < 6 {
		t.Fatalf("found only %d Effect constants in the package: %v", len(declared), declared)
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
