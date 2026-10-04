package flows

import (
	"reflect"
	"strings"
	"testing"
)

// crashingRegistry adds node types whose definition hooks panic. Each of the first
// five has one crashing hook; test.crash_all has all of them.
func crashingRegistry(t *testing.T) *Registry {
	t.Helper()
	reg := newTestRegistry(t)
	ports := func(*Node) []string { panic("ports kaputt") }
	fields := func(*Node) []FieldSpec { panic("fields kaputt") }
	effects := func(*Node) []Effect { panic("effects kaputt") }
	avail := func() Availability { panic("availability kaputt") }
	validate := func(*Node, ValidateContext) []Issue { panic("validate kaputt") }
	reg.MustRegister(&NodeDef{Type: "test.crash_ports", Category: "test", OutputsFunc: ports})
	reg.MustRegister(&NodeDef{Type: "test.crash_fields", Category: "test", OutputFieldsFunc: fields})
	reg.MustRegister(&NodeDef{Type: "test.crash_effects", Category: "test", EffectsFunc: effects})
	reg.MustRegister(&NodeDef{Type: "test.crash_avail", Category: "test", AvailabilityFunc: avail})
	reg.MustRegister(&NodeDef{Type: "test.crash_validate", Category: "test", Validate: validate})
	reg.MustRegister(&NodeDef{Type: "test.crash_all", Category: "test",
		OutputsFunc: ports, OutputFieldsFunc: fields, EffectsFunc: effects, AvailabilityFunc: avail, Validate: validate})
	return reg
}

// crashFlow builds start -> crash -> after, where after reads {{crash.x}}, so that
// the edge leaving the crashing node and the reference into it both reach its hooks.
func crashFlow(typ string) (f *Flow, crashID string) {
	b := newFlow("Crash")
	tr := b.node("start", "test.trigger", nil)
	crashID = b.node("crash", typ, nil)
	after := b.node("after", "test.echo", map[string]any{"value": "{{crash.x}}"})
	b.edge(tr, PortOut, crashID)
	b.edge(crashID, PortOut, after)
	return b.build(), crashID
}

// A panicking definition hook becomes one issue for its node, as a warning in a
// draft and as an error when publishing, and Validate returns normally.
func TestValidateRecoversCrashingHooks(t *testing.T) {
	reg := crashingRegistry(t)
	for _, typ := range []string{"test.crash_ports", "test.crash_fields", "test.crash_effects",
		"test.crash_avail", "test.crash_validate", "test.crash_all"} {
		t.Run(typ, func(t *testing.T) {
			f, crashID := crashFlow(typ)
			for _, tc := range []struct {
				name string
				ctx  ValidateContext
				sev  Severity
			}{{"draft", draftCtx(), SeverityWarning}, {"publish", publishCtx(), SeverityError}} {
				issues := Validate(f, reg, tc.ctx)
				if len(issues) != 1 {
					t.Fatalf("%s: issues = %+v, want exactly one", tc.name, issues)
				}
				is := issues[0]
				if is.Code != IssueParamInvalid || is.NodeID != crashID || is.Severity != tc.sev ||
					!strings.HasPrefix(is.Message, "the node definition crashed: ") || !strings.Contains(is.Message, "kaputt") {
					t.Fatalf("%s: issue = %+v", tc.name, is)
				}
			}
		})
	}
}

// The other nodes are still validated, and the panic text is bounded.
func TestValidateGoesOnAfterACrashingHook(t *testing.T) {
	reg := crashingRegistry(t)
	long := strings.Repeat("x", 5000)
	reg.MustRegister(&NodeDef{Type: "test.crash_long", Category: "test",
		Validate: func(*Node, ValidateContext) []Issue { panic(long) }})
	b := newFlow("Mixed")
	tr := b.node("start", "test.trigger", nil)
	c1 := b.node("c1", "test.crash_all", nil)
	c2 := b.node("c2", "test.crash_long", nil)
	req := b.node("req", "test.required", nil)
	b.edge(tr, PortOut, c1)
	b.edge(tr, PortOut, c2)
	b.edge(tr, PortOut, req)
	issues := Validate(b.build(), reg, publishCtx())
	if len(issues) != 3 {
		t.Fatalf("issues = %+v, want one per crashing node and the missing parameter", issues)
	}
	if findIssue(issues, IssueParamRequired, req) == nil {
		t.Errorf("the clean node was not validated: %+v", issues)
	}
	if is := findIssue(issues, IssueParamInvalid, c1); is == nil || !strings.Contains(is.Message, "kaputt") {
		t.Errorf("c1: %+v", issues)
	}
	is := findIssue(issues, IssueParamInvalid, c2)
	if is == nil || len(is.Message) > 300 || strings.Contains(is.Message, long[:maxHookPanicRunes+1]) {
		t.Errorf("c2: the panic text must be cut: %+v", is)
	}
}

// A node whose EffectsFunc panics contributes no effects, and the other nodes are
// summarised as usual.
func TestCollectEffectsRecoversCrashingHooks(t *testing.T) {
	reg := crashingRegistry(t)
	b := newFlow("Effects")
	b.node("start", "test.trigger", nil)
	b.node("boom", "test.crash_effects", nil)
	b.node("all", "test.crash_all", nil)
	sink := b.node("sink", "test.sink", nil)
	got := CollectEffects(b.build(), reg)
	want := []EffectSummary{{Effect: EffectRunsCode, NodeIDs: []string{sink}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CollectEffects = %+v, want %+v", got, want)
	}
}

// The lint calls no definition hook, so crashing ones do not affect it.
func TestLintUntrustedDataIgnoresCrashingHooks(t *testing.T) {
	reg := crashingRegistry(t)
	b := newFlow("Lint")
	tr := b.node("hook", "test.untrusted_trigger", nil)
	mid := b.node("mid", "test.crash_all", nil)
	sink := b.node("shell", "test.sink", map[string]any{"command": "{{trigger.data.x}}"})
	b.edge(tr, PortOut, mid)
	b.edge(mid, PortOut, sink)
	issues := LintUntrustedData(b.build(), reg)
	if len(issues) != 1 || issues[0].Code != IssueUntrustedData || issues[0].NodeID != sink {
		t.Fatalf("issues = %+v", issues)
	}
}
