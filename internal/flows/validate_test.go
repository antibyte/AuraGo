package flows

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func findIssue(issues []Issue, code, nodeID string) *Issue {
	for i := range issues {
		if issues[i].Code == code && (nodeID == "" || issues[i].NodeID == nodeID) {
			return &issues[i]
		}
	}
	return nil
}

func draftCtx() ValidateContext {
	return ValidateContext{Mode: ModeDraft, Now: time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC), Location: time.UTC}
}

func publishCtx() ValidateContext {
	vc := draftCtx()
	vc.Mode = ModePublish
	return vc
}

func validFlow() (*flowBuilder, string, string) {
	b := newFlow("Valid")
	tr := b.node("start", "test.trigger", nil)
	echo := b.node("echo", "test.echo", map[string]any{"value": "{{trigger.data.msg}} {{start.fired_at}}"})
	b.edge(tr, PortOut, echo)
	return b, tr, echo
}

func TestValidateAcceptsValidFlow(t *testing.T) {
	b, _, _ := validFlow()
	if issues := Validate(b.build(), newTestRegistry(t), publishCtx()); len(issues) != 0 {
		t.Fatalf("valid flow has issues: %+v", issues)
	}
}

func TestValidateStructure(t *testing.T) {
	reg := newTestRegistry(t)
	cases := []struct {
		name   string
		code   string
		mutate func(f *Flow, tr, echo string)
	}{
		{"schema", IssueSchema, func(f *Flow, _, _ string) { f.Schema = 2 }},
		{"name", IssueNameRequired, func(f *Flow, _, _ string) { f.Name = "  " }},
		{"bad id", IssueNodeIDInvalid, func(f *Flow, _, _ string) { f.Nodes[1].ID = "x" }},
		{"duplicate id", IssueNodeIDDuplicate, func(f *Flow, _, _ string) { f.Nodes[1].ID = f.Nodes[0].ID }},
		{"reserved key", IssueNodeKeyReserved, func(f *Flow, _, _ string) { f.Nodes[1].Key = "trigger" }},
		{"invalid key", IssueNodeKeyInvalid, func(f *Flow, _, _ string) { f.Nodes[1].Key = "Echo" }},
		{"duplicate key", IssueNodeKeyDuplicate, func(f *Flow, _, _ string) { f.Nodes[1].Key = "start" }},
		{"missing node", IssueEdgeNodeMissing, func(f *Flow, _, echo string) {
			f.Edges = append(f.Edges, Edge{ID: "e_x", Source: PortRef{Node: "n_missingx", Port: PortOut}, Target: PortRef{Node: echo, Port: PortIn}})
		}},
		{"self edge", IssueEdgeSelf, func(f *Flow, _, echo string) {
			f.Edges = append(f.Edges, Edge{ID: "e_x", Source: PortRef{Node: echo, Port: PortOut}, Target: PortRef{Node: echo, Port: PortIn}})
		}},
		{"bad source port", IssueEdgePortInvalid, func(f *Flow, _, _ string) { f.Edges[0].Source.Port = PortTrue }},
		{"edge into trigger", IssueEdgePortInvalid, func(f *Flow, tr, echo string) {
			f.Edges = append(f.Edges, Edge{ID: "e_x", Source: PortRef{Node: echo, Port: PortOut}, Target: PortRef{Node: tr, Port: PortIn}})
		}},
		{"duplicate edge", IssueEdgeDuplicate, func(f *Flow, _, _ string) {
			dup := f.Edges[0]
			dup.ID = "e_dup"
			f.Edges = append(f.Edges, dup)
		}},
		{"duplicate edge id", IssueEdgeIDDuplicate, func(f *Flow, tr, _ string) {
			f.Nodes = append(f.Nodes, Node{ID: testNodeID(99), Key: "other", Type: "test.echo", Params: map[string]any{}})
			f.Edges = append(f.Edges, Edge{ID: f.Edges[0].ID, Source: PortRef{Node: tr, Port: PortOut}, Target: PortRef{Node: testNodeID(99), Port: PortIn}})
		}},
		{"too many nodes", IssueTooManyNodes, func(f *Flow, _, _ string) {
			for i := 0; i <= MaxNodes; i++ {
				f.Nodes = append(f.Nodes, Node{ID: testNodeID(100 + i), Key: "k" + testNodeID(100 + i)[2:], Type: "test.echo", Params: map[string]any{}})
			}
		}},
		{"too many edges", IssueTooManyNodes, func(f *Flow, tr, echo string) {
			for i := 0; i <= MaxEdges; i++ {
				f.Edges = append(f.Edges, Edge{ID: "e_" + strconv.Itoa(100+i), Source: PortRef{Node: tr, Port: PortOut}, Target: PortRef{Node: echo, Port: PortIn}})
			}
		}},
	}
	for _, tc := range cases {
		b, tr, echo := validFlow()
		f := b.build()
		tc.mutate(f, tr, echo)
		issue := findIssue(Validate(f, reg, draftCtx()), tc.code, "")
		if issue == nil || issue.Severity != SeverityError {
			t.Errorf("%s: want error %s, got %+v", tc.name, tc.code, Validate(f, reg, draftCtx()))
		}
	}
}

func TestValidatePublishRulesAreWarningsInDraft(t *testing.T) {
	reg := newTestRegistry(t)
	b := newFlow("No trigger")
	b.node("echo", "test.echo", nil)
	f := b.build()
	if is := findIssue(Validate(f, reg, draftCtx()), IssueNoTrigger, ""); is == nil || is.Severity != SeverityWarning {
		t.Fatalf("draft no trigger = %+v", is)
	}
	if is := findIssue(Validate(f, reg, publishCtx()), IssueNoTrigger, ""); is == nil || is.Severity != SeverityError {
		t.Fatalf("publish no trigger = %+v", is)
	}
	b2, tr, _ := validFlow()
	f2 := b2.build()
	f2.NodeByID(tr).Settings.Disabled = true
	if findIssue(Validate(f2, reg, publishCtx()), IssueNoTrigger, "") == nil {
		t.Fatal("a disabled trigger does not count")
	}
}

func TestValidatePublishRules(t *testing.T) {
	reg := newTestRegistry(t)
	check := func(name string, build func(b *flowBuilder, tr string) string, code string, sev Severity) {
		t.Helper()
		b := newFlow(name)
		tr := b.node("start", "test.trigger", nil)
		nodeID := build(b, tr)
		issues := Validate(b.build(), reg, publishCtx())
		is := findIssue(issues, code, nodeID)
		if is == nil || is.Severity != sev {
			t.Errorf("%s: want %s/%s on %s, got %+v", name, code, sev, nodeID, issues)
		}
	}
	connect := func(b *flowBuilder, tr, key, typ string, params map[string]any) string {
		id := b.node(key, typ, params)
		b.edge(tr, PortOut, id)
		return id
	}
	check("unknown type", func(b *flowBuilder, tr string) string { return connect(b, tr, "x", "nope.x", nil) }, IssueNodeTypeUnknown, SeverityError)
	check("unavailable", func(b *flowBuilder, tr string) string { return connect(b, tr, "tg", "test.unavailable", nil) }, IssueNodeUnavailable, SeverityError)
	check("required", func(b *flowBuilder, tr string) string { return connect(b, tr, "req", "test.required", nil) }, IssueParamRequired, SeverityError)
	check("visible required", func(b *flowBuilder, tr string) string {
		return connect(b, tr, "req", "test.required", map[string]any{"text": "t", "mode": "b"})
	}, IssueParamRequired, SeverityError)
	check("syntax", func(b *flowBuilder, tr string) string {
		return connect(b, tr, "e", "test.echo", map[string]any{"value": "{{oops"})
	}, IssueTemplateSyntax, SeverityError)
	check("unknown root", func(b *flowBuilder, tr string) string {
		return connect(b, tr, "e", "test.echo", map[string]any{"value": "{{ghost.x}}"})
	}, IssueTemplateUnknownRoot, SeverityError)
	check("not upstream", func(b *flowBuilder, tr string) string {
		connect(b, tr, "a", "test.echo", nil)
		return connect(b, tr, "b", "test.echo", map[string]any{"value": "{{a.value}}"})
	}, IssueTemplateNotUpstream, SeverityError)
	check("item root", func(b *flowBuilder, tr string) string {
		return connect(b, tr, "e", "test.echo", map[string]any{"value": "{{item.x}}"})
	}, IssueTemplateRootUnavailable, SeverityError)
	check("unknown field", func(b *flowBuilder, tr string) string {
		a := connect(b, tr, "a", "test.echo", nil)
		c := b.node("c", "test.echo", map[string]any{"value": "{{a.nope}}"})
		b.edge(a, PortOut, c)
		return c
	}, IssueTemplateUnknownField, SeverityWarning)
	check("orphan", func(b *flowBuilder, tr string) string { return b.node("lonely", "test.echo", nil) }, IssueNodeUnreachable, SeverityWarning)
	check("cycle", func(b *flowBuilder, tr string) string {
		a := connect(b, tr, "a", "test.echo", nil)
		c := b.node("c", "test.echo", nil)
		b.edge(a, PortOut, c)
		b.edge(c, PortOut, a)
		return a
	}, IssueCycle, SeverityError)
	check("node validator", func(b *flowBuilder, tr string) string {
		return connect(b, tr, "w", TypeWait, map[string]any{"mode": "duration", "seconds": 9999.0})
	}, IssueParamInvalid, SeverityError)
	check("untrusted", func(b *flowBuilder, tr string) string {
		hook := b.node("hook", "test.untrusted_trigger", nil)
		return connect(b, hook, "shell", "test.sink", map[string]any{"command": "{{trigger.data.cmd}}"})
	}, IssueUntrustedData, SeverityWarning)
}

func TestValidateSkipsDisabledNodesAndHiddenParams(t *testing.T) {
	reg := newTestRegistry(t)
	b := newFlow("Disabled")
	tr := b.node("start", "test.trigger", nil)
	req := b.node("req", "test.required", nil)
	ok := b.node("ok", "test.required", map[string]any{"text": "t"})
	b.edge(tr, PortOut, req)
	b.edge(tr, PortOut, ok)
	f := b.build()
	f.NodeByID(req).Settings.Disabled = true
	issues := Validate(f, reg, publishCtx())
	if findIssue(issues, IssueParamRequired, req) != nil {
		t.Fatalf("disabled nodes skip parameter checks: %+v", issues)
	}
	if findIssue(issues, IssueParamRequired, ok) != nil {
		t.Fatalf("hidden required parameters are not checked: %+v", issues)
	}
	if HasErrors(issues) {
		t.Fatalf("unexpected errors: %+v", issues)
	}
}

func TestValidateDraftDowngradesNodeValidatorErrors(t *testing.T) {
	reg := newTestRegistry(t)
	b := newFlow("Draft")
	tr := b.node("start", "test.trigger", nil)
	w := b.node("w", TypeWait, map[string]any{"mode": "duration", "seconds": 9999.0})
	b.edge(tr, PortOut, w)
	is := findIssue(Validate(b.build(), reg, draftCtx()), IssueParamInvalid, w)
	if is == nil || is.Severity != SeverityWarning {
		t.Fatalf("draft node validator = %+v", is)
	}
}

// issueCodes lists the codes of issues in order.
func issueCodes(issues []Issue) []string {
	codes := make([]string, len(issues))
	for i, is := range issues {
		codes[i] = is.Code
	}
	return codes
}

// timeValidate runs Validate and fails the test when it takes longer than limit.
func timeValidate(t *testing.T, f *Flow, reg *Registry, vc ValidateContext, limit time.Duration) []Issue {
	t.Helper()
	start := time.Now()
	issues := Validate(f, reg, vc)
	if elapsed := time.Since(start); elapsed > limit {
		t.Fatalf("Validate of %d nodes and %d edges took %v, want under %v", len(f.Nodes), len(f.Edges), elapsed, limit)
	}
	return issues
}

// ParseFlow only caps the document size, so Validate has to bound its own work:
// a flow over MaxNodes or MaxEdges gets one issue per limit and no per-node,
// per-edge or graph work at all.
func TestValidateOverTheLimitsReturnsEarly(t *testing.T) {
	reg := newTestRegistry(t)
	const bulkNodes, bulkEdges = 30000, 10000

	t.Run("nodes", func(t *testing.T) {
		// Every node has an invalid id, key and type; per-node work would report
		// each of them 30000 times.
		f := bulkGraphFlow(bulkNodes)
		for _, vc := range []ValidateContext{draftCtx(), publishCtx()} {
			issues := timeValidate(t, f, reg, vc, 2*time.Second)
			if len(issues) != 1 || issues[0].Code != IssueTooManyNodes || issues[0].Severity != SeverityError {
				t.Fatalf("mode %d: want exactly one %s error, got %d issues: %v", vc.Mode, IssueTooManyNodes, len(issues), issueCodes(issues))
			}
			if !strings.Contains(issues[0].Message, "nodes") || !strings.Contains(issues[0].Message, strconv.Itoa(MaxNodes)) {
				t.Errorf("node limit message = %q", issues[0].Message)
			}
		}
	})

	t.Run("edges", func(t *testing.T) {
		b, tr, echo := validFlow()
		f := b.build()
		for i := 0; i < bulkEdges; i++ {
			f.Edges = append(f.Edges, Edge{ID: "e_" + strconv.Itoa(100+i), Source: PortRef{Node: tr, Port: PortOut}, Target: PortRef{Node: echo, Port: PortIn}})
		}
		for _, vc := range []ValidateContext{draftCtx(), publishCtx()} {
			issues := timeValidate(t, f, reg, vc, 2*time.Second)
			if len(issues) != 1 || issues[0].Code != IssueTooManyNodes || issues[0].Severity != SeverityError {
				t.Fatalf("mode %d: want exactly one %s error, got %d issues: %v", vc.Mode, IssueTooManyNodes, len(issues), issueCodes(issues))
			}
			if !strings.Contains(issues[0].Message, "connections") || !strings.Contains(issues[0].Message, strconv.Itoa(MaxEdges)) {
				t.Errorf("edge limit message = %q", issues[0].Message)
			}
		}
	})

	t.Run("both", func(t *testing.T) {
		f := bulkGraphFlow(bulkNodes)
		for i := 1; i < bulkNodes && len(f.Edges) < bulkEdges; i++ {
			bulkEdge(f, i-1, i)
		}
		issues := timeValidate(t, f, reg, publishCtx(), 2*time.Second)
		if len(issues) != 2 || issues[0].Code != IssueTooManyNodes || issues[1].Code != IssueTooManyNodes ||
			issues[0].Message == issues[1].Message {
			t.Fatalf("want one node and one edge limit issue, got %+v", issues)
		}
	})

	t.Run("schema and name come first", func(t *testing.T) {
		f := bulkGraphFlow(bulkNodes)
		f.Schema = 2
		f.Name = ""
		issues := timeValidate(t, f, reg, publishCtx(), 2*time.Second)
		if want := []string{IssueSchema, IssueNameRequired, IssueTooManyNodes}; !reflect.DeepEqual(issueCodes(issues), want) {
			t.Fatalf("codes = %v, want %v", issueCodes(issues), want)
		}
	})
}

// The limits are inclusive: exactly MaxNodes nodes and MaxEdges edges are
// validated normally.
func TestValidateAtTheLimitsRunsInFull(t *testing.T) {
	reg := newTestRegistry(t)
	b := newFlow("At the limits")
	tr := b.node("start", "test.trigger", nil)
	prev := tr
	for i := 1; i < MaxNodes; i++ {
		id := b.node("k"+strconv.Itoa(i), "test.echo", nil)
		b.edge(prev, PortOut, id)
		prev = id
	}
	f := b.build()
	if len(f.Nodes) != MaxNodes {
		t.Fatalf("test setup: %d nodes", len(f.Nodes))
	}
	for len(f.Edges) < MaxEdges {
		dup := f.Edges[0]
		dup.ID = "e_dup" + strconv.Itoa(len(f.Edges))
		f.Edges = append(f.Edges, dup)
	}
	issues := timeValidate(t, f, reg, publishCtx(), 5*time.Second)
	if findIssue(issues, IssueTooManyNodes, "") != nil {
		t.Fatalf("%d nodes and %d edges are within the limits: %v", len(f.Nodes), len(f.Edges), issueCodes(issues[:1]))
	}
	if findIssue(issues, IssueEdgeDuplicate, "") == nil {
		t.Fatal("a flow within the limits must still get the structural checks")
	}
}

// longText is far beyond any message budget.
var longText = strings.Repeat("x", 10000)

// longKey returns a distinct, syntactically valid template root of 10000+ chars.
func longKey(i int) string { return strings.Repeat("k", 10000) + strconv.Itoa(i) }

// hugeValuesFlow echoes 10k-character ids, keys, types, ports, edge ids and
// template expressions through the messages the validator builds. LintUntrustedData
// gives up on cyclic flows, so the loop is optional: without it the flow also
// produces the untrusted-data warning.
func hugeValuesFlow(withLoop bool) *Flow {
	b := newFlow("Huge")
	tr := b.node("start", "test.trigger", nil)
	hook := b.node("hook", "test.untrusted_trigger", nil)
	conn := func(from, key, typ string, params map[string]any) string {
		id := b.node(key, typ, params)
		b.edge(from, PortOut, id)
		return id
	}
	a := conn(tr, longKey(1), "test.echo", map[string]any{"value": "{{trigger.data.x}}"})
	bNode := b.node(longKey(2), "test.echo", map[string]any{"value": "{{" + longKey(1) + ".nope}}"}) // unknown field, long key
	b.edge(a, PortOut, bNode)
	cNode := b.node(longKey(3), "test.echo", map[string]any{"value": "{{" + longKey(1) + "." + longText + "}}"}) // unknown field, long field
	b.edge(a, PortOut, cNode)
	b.node(longKey(4), "test.echo", nil) // unreachable
	// Not upstream: the root, the node key and the expression are all long.
	conn(tr, longKey(5), "test.echo", map[string]any{"value": "{{" + longKey(4) + ".value}}"})
	conn(tr, "f", "test.echo", map[string]any{"value": "{{ghost" + longText + ".x}}"})                    // unknown root
	conn(tr, "g", "test.echo", map[string]any{"value": "{{item." + longText + "}}"})                      // root only inside loops
	conn(tr, "h", "test.echo", map[string]any{"value": "{{start | " + longText + "}}"})                   // syntax: unknown filter
	conn(tr, "i", "test.echo", map[string]any{"value": "{{start.data" + strings.Repeat("[", 500) + "}}"}) // syntax
	conn(tr, "j", longText, nil)                                                                          // unknown type
	conn(tr, longKey(6), "test.required", nil)                                                            // required parameter
	conn(tr, longKey(7), "test.unavailable", nil)                                                         // not available
	conn(hook, longKey(10), "test.sink", map[string]any{"command": "{{trigger.data.cmd}}"})               // untrusted data into a sink
	if withLoop {
		loop1 := conn(tr, longKey(8), "test.echo", nil)
		loop2 := b.node(longKey(9), "test.echo", nil)
		b.edge(loop1, PortOut, loop2)
		b.edge(loop2, PortOut, loop1)
	}
	f := b.build()
	// Invalid node id and a duplicate of it.
	f.Nodes = append(f.Nodes,
		Node{ID: longText, Key: "badid", Type: "test.echo", Params: map[string]any{}},
		Node{ID: longText, Key: "badid2", Type: "test.echo", Params: map[string]any{}})
	// Edges with huge ids and ports.
	f.Edges = append(f.Edges,
		Edge{ID: longText, Source: PortRef{Node: tr, Port: PortOut}, Target: PortRef{Node: a, Port: PortIn}},
		Edge{ID: longText, Source: PortRef{Node: tr, Port: PortOut}, Target: PortRef{Node: a, Port: PortIn}},
		Edge{ID: "e_srcport", Source: PortRef{Node: tr, Port: longText}, Target: PortRef{Node: a, Port: PortIn}},
		Edge{ID: "e_dstport", Source: PortRef{Node: tr, Port: PortOut}, Target: PortRef{Node: a, Port: longText}})
	return f
}

func TestValidateMessagesStayBounded(t *testing.T) {
	reg := newTestRegistry(t)
	var all []Issue
	for _, withLoop := range []bool{false, true} {
		f := hugeValuesFlow(withLoop)
		for _, vc := range []ValidateContext{publishCtx(), draftCtx()} {
			issues := Validate(f, reg, vc)
			for _, is := range issues {
				if len(is.Message) > 300 {
					t.Errorf("mode %d, loop %v: %s message is %d bytes: %.120q...", vc.Mode, withLoop, is.Code, len(is.Message), is.Message)
				}
			}
			all = append(all, issues...)
		}
	}
	for _, code := range []string{
		IssueNodeKeyInvalid, IssueNodeIDInvalid, IssueEdgeIDDuplicate, IssueEdgePortInvalid,
		IssueNodeTypeUnknown, IssueNodeUnreachable, IssueNodeUnavailable, IssueParamRequired,
		IssueTemplateSyntax, IssueTemplateUnknownRoot, IssueTemplateNotUpstream,
		IssueTemplateRootUnavailable, IssueTemplateUnknownField, IssueCycle, IssueUntrustedData,
	} {
		if findIssue(all, code, "") == nil {
			t.Errorf("the huge-values flows produced no %s issue; the test no longer covers that message", code)
		}
	}
}

// Messages for short values keep the plan's readable wording.
func TestValidateMessagesAreReadableForShortValues(t *testing.T) {
	reg := newTestRegistry(t)
	b := newFlow("Readable")
	tr := b.node("start", "test.trigger", nil)
	a := b.node("a", "test.echo", map[string]any{"value": "{{ghost.x}}"})
	c := b.node("c", "test.echo", map[string]any{"value": "{{start.fired_at}} {{a.nope}}"})
	b.edge(tr, PortOut, a)
	b.edge(a, PortOut, c)
	issues := Validate(b.build(), reg, publishCtx())
	if is := findIssue(issues, IssueTemplateUnknownRoot, a); is == nil || is.Message != `{{ghost.x}} refers to an unknown node "ghost"` {
		t.Errorf("unknown root = %+v", is)
	}
	if is := findIssue(issues, IssueTemplateUnknownField, c); is == nil || is.Message != `{{a.nope}}: a has no output field "nope"` {
		t.Errorf("unknown field = %+v", is)
	}
}

func messyFlow(withLoop bool) *Flow {
	f := hugeValuesFlow(withLoop)
	b := &flowBuilder{f: f}
	// A few short-valued problems on top: nodes in an order unrelated to their
	// ids and keys, several refs per parameter and several parameters per node.
	z := b.node("zz", "test.echo", map[string]any{
		"value": "{{g3.x}} {{g1.x}} {{g2.x}}",
		"b":     "{{b2.x}}",
		"a":     "{{a1.x}}",
		"c":     map[string]any{"deep": []any{"{{c9.x}}"}},
	})
	b.edge(f.Nodes[0].ID, PortOut, z)
	b.node("aa", "test.required", nil)
	w := b.node("mm", TypeWait, map[string]any{"mode": "duration", "seconds": 9999.0})
	b.edge(f.Nodes[0].ID, PortOut, w)
	off := b.node("off", "test.required", nil)
	f.NodeByID(off).Settings.Disabled = true
	return f
}

func TestValidateIsDeterministic(t *testing.T) {
	reg := newTestRegistry(t)
	for _, withLoop := range []bool{false, true} {
		f := messyFlow(withLoop)
		for _, vc := range []ValidateContext{draftCtx(), publishCtx()} {
			first := Validate(f, reg, vc)
			if len(first) < 20 {
				t.Fatalf("loop %v, mode %d: the flow is meant to produce many issues, got %d", withLoop, vc.Mode, len(first))
			}
			for i := 0; i < 25; i++ {
				if again := Validate(f, reg, vc); !reflect.DeepEqual(first, again) {
					t.Fatalf("loop %v, mode %d, run %d: issues changed between runs\nfirst: %+v\nagain: %+v", withLoop, vc.Mode, i, first, again)
				}
			}
		}
	}
}

func TestValidateIssueOrderFollowsTheDocument(t *testing.T) {
	reg := newTestRegistry(t)
	b := newFlow("Order")
	tr := b.node("start", "test.trigger", nil)
	// Keys and ids sort differently than the document order.
	zNode := b.node("zz", "test.echo", map[string]any{"value": "{{ghost.x}}"})
	mNode := b.node("mm", "test.echo", map[string]any{"value": "{{ghost.x}}"})
	aNode := b.node("aa", "test.echo", map[string]any{"value": "{{ghost.x}}"})
	for _, id := range []string{zNode, mNode, aNode} {
		b.edge(tr, PortOut, id)
	}
	f := b.build()
	f.Edges = append(f.Edges,
		Edge{ID: "e_z", Source: PortRef{Node: "n_missingz", Port: PortOut}, Target: PortRef{Node: aNode, Port: PortIn}},
		Edge{ID: "e_a", Source: PortRef{Node: "n_missinga", Port: PortOut}, Target: PortRef{Node: aNode, Port: PortIn}},
		Edge{ID: "e_m", Source: PortRef{Node: "n_missingm", Port: PortOut}, Target: PortRef{Node: aNode, Port: PortIn}})
	issues := Validate(f, reg, publishCtx())

	var edgeOrder, rootOrder []string
	for _, is := range issues {
		switch is.Code {
		case IssueEdgeNodeMissing:
			edgeOrder = append(edgeOrder, is.EdgeID)
		case IssueTemplateUnknownRoot:
			rootOrder = append(rootOrder, is.NodeID)
		}
	}
	if want := []string{"e_z", "e_a", "e_m"}; !reflect.DeepEqual(edgeOrder, want) {
		t.Errorf("edge issues in order %v, want document order %v", edgeOrder, want)
	}
	if want := []string{zNode, mNode, aNode}; !reflect.DeepEqual(rootOrder, want) {
		t.Errorf("node issues in order %v, want document order %v", rootOrder, want)
	}
}

func TestValidateTemplateIssuesFollowCollectTemplateRefsOrder(t *testing.T) {
	reg := newTestRegistry(t)
	b := newFlow("Refs")
	tr := b.node("start", "test.trigger", nil)
	// Parameters are walked in sorted key order, so "a" comes before "b" and "value"
	// whatever order the map was filled in; refs within one string keep source order.
	n := b.node("e", "test.echo", map[string]any{
		"value": "{{g1.x}}",
		"b":     "{{g2.x}}",
		"a":     "{{g4.x}} {{g3.x}} {{g5.x}}",
	})
	b.edge(tr, PortOut, n)
	issues := Validate(b.build(), reg, publishCtx())
	var got []string
	for _, is := range issues {
		if is.Code == IssueTemplateUnknownRoot {
			got = append(got, is.Param+":"+is.Message[strings.Index(is.Message, "node ")+len("node "):])
		}
	}
	want := []string{`a:"g4"`, `a:"g3"`, `a:"g5"`, `b:"g2"`, `value:"g1"`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("template issues = %v, want %v", got, want)
	}
}

func TestValidateDoesNotMutateFlowOrRegistry(t *testing.T) {
	reg := newTestRegistry(t)
	defsBefore := snapshotDefs(t, reg)
	for _, withLoop := range []bool{false, true} {
		f := messyFlow(withLoop)
		before, err := json.Marshal(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, vc := range []ValidateContext{draftCtx(), publishCtx()} {
			Validate(f, reg, vc)
		}
		after, err := json.Marshal(f)
		if err != nil {
			t.Fatal(err)
		}
		if string(before) != string(after) {
			t.Fatalf("loop %v: Validate changed the flow", withLoop)
		}
	}
	if defsAfter := snapshotDefs(t, reg); defsBefore != defsAfter {
		t.Fatalf("Validate changed the registry's definitions\nbefore: %s\nafter:  %s", defsBefore, defsAfter)
	}
	// Nil params stay nil: Validate must not normalize.
	g := newFlow("Nil params")
	g.node("start", "test.trigger", nil)
	g.node("e", "test.echo", nil)
	flow := g.f
	Validate(flow, reg, publishCtx())
	if flow.Nodes[0].Params != nil || flow.Nodes[1].Params != nil {
		t.Fatal("Validate must not fill in nil params")
	}
}

// snapshotDefs serializes the data fields of every definition (the func fields
// cannot be marshaled).
func snapshotDefs(t *testing.T, reg *Registry) string {
	t.Helper()
	type data struct {
		Type         string
		Version      int
		Trigger      bool
		Inputs       []string
		Outputs      []string
		Params       []ParamSpec
		OutputFields []FieldSpec
		Effects      []Effect
	}
	var all []data
	for _, d := range reg.All() {
		all = append(all, data{d.Type, d.Version, d.Trigger, d.Inputs, d.Outputs, d.Params, d.OutputFields, d.Effects})
	}
	out, err := json.Marshal(all)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// A node's template references need its ancestor set only when one points at
// another node, and then once per node, however many references it holds.
func TestValidateComputesAncestorsOncePerNode(t *testing.T) {
	reg := newTestRegistry(t)
	const chain, refsPerNode = 200, 300
	b := newFlow("Ancestors")
	prev := b.node("start", "test.trigger", nil)
	// Node k1 only reads the trigger and run roots: it never needs ancestors.
	first := b.node("k1", "test.echo", map[string]any{"value": "{{trigger.data.x}} {{run.id}} {{flow.id}}"})
	b.edge(prev, PortOut, first)
	prev = first
	for i := 2; i <= chain; i++ {
		var sb strings.Builder
		for r := 0; r < refsPerNode; r++ {
			fmt.Fprintf(&sb, "{{k%d.value}}", 1+r%(i-1)) // always an upstream node
		}
		id := b.node("k"+strconv.Itoa(i), "test.echo", map[string]any{"value": sb.String()})
		b.edge(prev, PortOut, id)
		prev = id
	}
	f := b.build()
	v := newValidator(f, reg, publishCtx())
	v.run()
	if len(v.issues) != 0 {
		t.Fatalf("every reference points upstream, got issues: %+v", v.issues[:1])
	}
	if want := chain - 1; v.ancestorWalks != want {
		t.Fatalf("ancestor sets computed %d times, want %d (once per node that references another node)", v.ancestorWalks, want)
	}
}

// A template key resolves to the first node that has it, as Flow.NodeByKey does.
func TestValidateTemplateRootUsesFirstNodeWithKey(t *testing.T) {
	reg := newTestRegistry(t)
	b := newFlow("Duplicate keys")
	tr := b.node("start", "test.trigger", nil)
	first := b.node("dup", "test.echo", nil)
	second := b.node("dup", "test.echo", nil)
	user := b.node("user", "test.echo", map[string]any{"value": "{{dup.value}}"})
	b.edge(tr, PortOut, first)
	b.edge(tr, PortOut, second)
	b.edge(second, PortOut, user) // only the second "dup" is upstream
	issues := Validate(b.build(), reg, publishCtx())
	if findIssue(issues, IssueNodeKeyDuplicate, second) == nil {
		t.Fatalf("want a duplicate key issue: %+v", issues)
	}
	if findIssue(issues, IssueTemplateNotUpstream, user) == nil {
		t.Fatalf("{{dup.*}} means the first dup, which is not upstream: %+v", issues)
	}
}
