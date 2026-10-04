package flows

import (
	"strings"
	"testing"
)

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
