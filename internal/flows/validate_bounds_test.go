package flows

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// This file tests that Validate's output and its work stay bounded however the
// document is shaped, and that the draft severities follow the publish rules.

// hugeText returns a string of n copies of the letter c.
func hugeText(c string, n int) string { return strings.Repeat(c, n) }

// refsText returns a string of n template references to root.x.
func refsText(root string, n int) string { return strings.Repeat("{{"+root+".x}}", n) }

// assertIssuesBounded fails when a field of an issue is over its limit (the cut
// adds one ellipsis rune) or when the marshaled issues are over limitBytes.
func assertIssuesBounded(t *testing.T, label string, issues []Issue, limitBytes int) {
	t.Helper()
	for i, is := range issues {
		for _, f := range []struct {
			name  string
			value string
			limit int
		}{
			{"node_id", is.NodeID, maxIssueIDRunes},
			{"edge_id", is.EdgeID, maxIssueIDRunes},
			{"param", is.Param, maxIssueParamRunes},
			{"message", is.Message, maxIssueMessageRunes},
		} {
			if n := utf8.RuneCountInString(f.value); n > f.limit+1 {
				t.Fatalf("%s: issue %d (%s) has a %s of %d runes, limit %d", label, i, is.Code, f.name, n, f.limit)
			}
		}
	}
	data, err := json.Marshal(issues)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > limitBytes {
		t.Fatalf("%s: %d issues marshal to %d bytes, want at most %d", label, len(issues), len(data), limitBytes)
	}
}

// One node with a 1 MiB id and 1000 template references used to produce 1001
// issues that each carried the whole id.
func TestValidateBoundsHugeIdsAndIssueCount(t *testing.T) {
	reg := newTestRegistry(t)
	build := func() *Flow {
		id, edgeID := hugeText("e", 1<<20), hugeText("g", 1<<20)
		f := &Flow{Schema: SchemaVersion, ID: "flow_test", Kind: KindFlow, Name: "Huge",
			Nodes: []Node{
				{ID: testNodeID(1), Key: "start", Type: "test.trigger"},
				{ID: id, Key: "e", Type: "test.echo", Params: map[string]any{"value": refsText("ghost", 1000)}},
			},
			Edges: []Edge{{ID: edgeID, Source: PortRef{Node: testNodeID(1), Port: PortOut}, Target: PortRef{Node: id, Port: PortIn}}},
		}
		f.Normalize()
		return f
	}

	t.Run("publish", func(t *testing.T) {
		issues := timeValidate(t, build(), reg, publishCtx(), 3*time.Second)
		assertIssuesBounded(t, "publish", issues, 200_000)
		last := issues[len(issues)-1]
		if len(issues) != maxIssues+1 || last.Code != IssueTooManyIssues || last.Severity != SeverityError {
			t.Fatalf("want %d issues ending in an error %s, got %d issues, last %+v", maxIssues+1, IssueTooManyIssues, len(issues), last)
		}
		if !strings.HasSuffix(issues[0].NodeID, "…") || !strings.HasPrefix(issues[0].NodeID, "eeee") {
			t.Errorf("an over-long id is cut, not dropped: %.80q", issues[0].NodeID)
		}
	})

	t.Run("draft", func(t *testing.T) {
		// Draft: the template problems are warnings; the invalid id is the one error.
		issues := timeValidate(t, build(), reg, draftCtx(), 3*time.Second)
		assertIssuesBounded(t, "draft", issues, 200_000)
		last := issues[len(issues)-1]
		if len(issues) != maxIssues+1 || last.Code != IssueTooManyIssues || last.Severity != SeverityWarning {
			t.Fatalf("want %d issues ending in a warning %s, got %d issues, last %+v", maxIssues+1, IssueTooManyIssues, len(issues), last)
		}
		if findIssue(issues, IssueNodeIDInvalid, "") == nil {
			t.Fatal("the one error must survive the cap")
		}
	})
}

// Issues from every source get their fields cut: the structure checks, the
// publish rules, a node's own validator and the untrusted-data lint.
func TestValidateBoundsFieldsOfEveryIssueSource(t *testing.T) {
	reg := newTestRegistry(t)
	idW, idS, idE := hugeText("w", 1<<20), hugeText("s", 1<<20), hugeText("e", 1<<20)
	edgeID := hugeText("g", 1<<20)
	tr, hook := testNodeID(1), testNodeID(2)
	f := &Flow{Schema: SchemaVersion, ID: "flow_test", Kind: KindFlow, Name: "Sources",
		Nodes: []Node{
			{ID: tr, Key: "start", Type: "test.trigger"},
			{ID: hook, Key: "hook", Type: "test.untrusted_trigger"},
			{ID: idW, Key: "w", Type: TypeWait, Params: map[string]any{"mode": "duration", "seconds": 9999.0}},
			{ID: idS, Key: "s", Type: "test.sink", Params: map[string]any{"command": "{{trigger.data.cmd}}"}},
			{ID: idE, Key: "e", Type: "test.echo", Params: map[string]any{"value": "{{ghost.x}} " + strings.Repeat("p", 5000)}},
		},
		Edges: []Edge{
			{ID: edgeID, Source: PortRef{Node: tr, Port: PortOut}, Target: PortRef{Node: idW, Port: PortIn}},
			{ID: edgeID, Source: PortRef{Node: hook, Port: PortOut}, Target: PortRef{Node: idS, Port: PortIn}},
			{ID: "e_3", Source: PortRef{Node: tr, Port: PortOut}, Target: PortRef{Node: idE, Port: PortIn}},
		},
	}
	f.Normalize()
	// A parameter with a huge name: its path is cut to 40 runes per key by
	// CollectTemplateRefs and to 200 runes in total here.
	nested := map[string]any{}
	cur := nested
	for i := 0; i < 20; i++ {
		next := map[string]any{}
		cur[hugeText("k", 100)+strconv.Itoa(i)] = next
		cur = next
	}
	cur["leaf"] = "{{ghost.x}}"
	f.Nodes[4].Params["deep"] = nested

	issues := Validate(f, reg, publishCtx())
	assertIssuesBounded(t, "sources", issues, 100_000)
	for _, code := range []string{
		IssueNodeIDInvalid,       // structure
		IssueEdgeIDDuplicate,     // structure, huge edge id
		IssueTemplateUnknownRoot, // publish rule, huge node id and a deep parameter path
		IssueParamInvalid,        // the wait node's own validator
		IssueUntrustedData,       // lint
	} {
		is := findIssue(issues, code, "")
		if is == nil {
			t.Fatalf("no %s issue; the test no longer covers that source", code)
		}
		if is.NodeID != "" && !strings.HasPrefix(is.NodeID, strings.Repeat(is.NodeID[:1], 60)) {
			t.Errorf("%s: a cut id keeps its start: %.80q", code, is.NodeID)
		}
	}
	deep := false
	for _, is := range issues {
		if is.Code == IssueTemplateUnknownRoot && strings.HasPrefix(is.Param, "deep.") {
			deep = utf8.RuneCountInString(is.Param) == maxIssueParamRunes+1
		}
	}
	if !deep {
		t.Error("the deep parameter path must be cut to the Param limit")
	}
	if is := findIssue(issues, IssueEdgeIDDuplicate, ""); is.EdgeID == "" || utf8.RuneCountInString(is.EdgeID) != maxIssueIDRunes+1 {
		t.Errorf("edge id = %d runes, want %d", utf8.RuneCountInString(is.EdgeID), maxIssueIDRunes+1)
	}
	// Real ids are untouched.
	b, _, echo := validFlow()
	b.f.Nodes[1].Params["value"] = "{{ghost.x}}"
	if is := findIssue(Validate(b.build(), reg, publishCtx()), IssueTemplateUnknownRoot, ""); is == nil || is.NodeID != echo {
		t.Fatalf("a normal id must survive unchanged: %+v", is)
	}
}

func TestValidateIssueCapKeepsErrorsFirstInOrder(t *testing.T) {
	reg := newTestRegistry(t)
	b := newFlow("Cap order")
	tr := b.node("start", "test.trigger", nil)
	src := b.node("src", "test.echo", nil)
	b.edge(tr, PortOut, src)
	warns := map[string]any{}
	for i := 0; i < 700; i++ {
		warns[fmt.Sprintf("p%03d", i)] = "{{src.nope}}" // unknown field: always a warning
	}
	c := b.node("c", "test.echo", warns)
	b.edge(src, PortOut, c)
	errs := map[string]any{}
	for i := 0; i < 10; i++ {
		errs[fmt.Sprintf("q%d", i)] = "{{ghost.x}}" // unknown root: an error when publishing
	}
	d := b.node("d", "test.echo", errs)
	b.edge(src, PortOut, d)

	issues := Validate(b.build(), reg, publishCtx())
	if len(issues) != maxIssues+1 {
		t.Fatalf("got %d issues, want %d", len(issues), maxIssues+1)
	}
	var params []string
	for _, is := range issues[:maxIssues] {
		params = append(params, is.Param)
	}
	// All 10 errors are kept; the 490 earliest warnings fill the other slots; the
	// warnings come before the errors, as they were found.
	var want []string
	for i := 0; i < 490; i++ {
		want = append(want, fmt.Sprintf("p%03d", i))
	}
	for i := 0; i < 10; i++ {
		want = append(want, fmt.Sprintf("q%d", i))
	}
	if !reflect.DeepEqual(params, want) {
		t.Fatalf("kept params differ\ngot  %v...\nwant %v...", params[480:], want[480:])
	}
	last := issues[maxIssues]
	if last.Code != IssueTooManyIssues || last.Severity != SeverityWarning || last.Message != "210 more problems are not shown" {
		t.Fatalf("summary = %+v, want a warning for 210 dropped warnings", last)
	}
	if !HasErrors(issues) {
		t.Fatal("the kept errors still fail validation")
	}
}

func TestValidateIssueCapSummarySeverity(t *testing.T) {
	reg := newTestRegistry(t)
	flowWithRefs := func(n int) *Flow {
		b := newFlow("Refs")
		tr := b.node("start", "test.trigger", nil)
		e := b.node("e", "test.echo", map[string]any{"value": refsText("ghost", n)})
		b.edge(tr, PortOut, e)
		return b.build()
	}
	summaryOf := func(t *testing.T, issues []Issue) *Issue {
		t.Helper()
		var found *Issue
		for i := range issues {
			if issues[i].Code == IssueTooManyIssues {
				if found != nil || i != len(issues)-1 {
					t.Fatalf("want one summary, at the end: %+v", issues[i])
				}
				found = &issues[i]
			}
		}
		return found
	}

	// An error is among the dropped issues: the summary is an error.
	issues := Validate(flowWithRefs(600), reg, publishCtx())
	if s := summaryOf(t, issues); s == nil || s.Severity != SeverityError || s.Message != "100 more problems are not shown" {
		t.Errorf("dropped errors: summary = %+v", s)
	}
	// Only warnings are dropped (draft mode makes the template problems
	// warnings): the summary is a warning, and the draft can still be saved.
	issues = Validate(flowWithRefs(600), reg, draftCtx())
	if s := summaryOf(t, issues); s == nil || s.Severity != SeverityWarning || s.Message != "100 more problems are not shown" {
		t.Errorf("dropped warnings: summary = %+v", s)
	}
	if HasErrors(issues) {
		t.Error("a draft with only warnings must stay saveable after the cap")
	}
	// Exactly maxIssues issues fit; one more does not.
	for _, mode := range []ValidateContext{publishCtx(), draftCtx()} {
		issues = Validate(flowWithRefs(maxIssues), reg, mode)
		if len(issues) != maxIssues || summaryOf(t, issues) != nil {
			t.Errorf("mode %d: exactly %d issues must come back whole, got %d (summary %v)", mode.Mode, maxIssues, len(issues), summaryOf(t, issues))
		}
		issues = Validate(flowWithRefs(maxIssues+1), reg, mode)
		if s := summaryOf(t, issues); len(issues) != maxIssues+1 || s == nil || s.Message != "1 more problems are not shown" {
			t.Errorf("mode %d: %d issues, summary %+v", mode.Mode, len(issues), s)
		}
	}
}

// emit keeps only the first maxIssues errors and warnings so a document with a
// huge number of issues is never held in memory; the result must still be what
// selecting from the complete list would give.
func TestValidatorIssueCapMatchesReferenceSelection(t *testing.T) {
	for _, tc := range []struct{ errs, warns int }{
		{0, 0}, {0, 1200}, {1200, 0}, {300, 900}, {600, 600}, {499, 2}, {500, 1}, {500, 0},
		{1, 499}, {1, 500}, {10, 700}, {2000, 2000},
	} {
		var seq []Issue
		for e, w := 0, 0; e < tc.errs || w < tc.warns; {
			sev := SeverityWarning
			if w >= tc.warns || (e < tc.errs && e*tc.warns <= w*tc.errs) {
				sev = SeverityError
				e++
			} else {
				w++
			}
			seq = append(seq, Issue{Code: "TEST", Severity: sev, Message: strconv.Itoa(len(seq))})
		}

		// Reference: all errors up to maxIssues, the earliest warnings in the
		// slots that remain, original order.
		errTotal := 0
		for _, is := range seq {
			if is.Severity == SeverityError {
				errTotal++
			}
		}
		slots := maxIssues - min(errTotal, maxIssues)
		var want []Issue
		dropped, droppedError := 0, false
		kept := map[Severity]int{}
		for _, is := range seq {
			limit := maxIssues
			if is.Severity != SeverityError {
				limit = slots
			}
			if kept[is.Severity] < limit {
				kept[is.Severity]++
				want = append(want, is)
				continue
			}
			dropped++
			droppedError = droppedError || is.Severity == SeverityError
		}
		if dropped > 0 {
			sev := SeverityWarning
			if droppedError {
				sev = SeverityError
			}
			want = append(want, Issue{Code: IssueTooManyIssues, Severity: sev, Message: fmt.Sprintf("%d more problems are not shown", dropped)})
		}

		v := &validator{}
		for _, is := range seq {
			v.emit(is)
		}
		v.finish()
		if !reflect.DeepEqual(v.issues, want) {
			t.Errorf("errors=%d warnings=%d: kept %d issues (summary %v), want %d (summary %v)",
				tc.errs, tc.warns, len(v.issues), lastIssue(v.issues), len(want), lastIssue(want))
		}
		if len(v.issues) > maxIssues+1 {
			t.Errorf("errors=%d warnings=%d: %d issues returned", tc.errs, tc.warns, len(v.issues))
		}
	}
}

func lastIssue(issues []Issue) *Issue {
	if len(issues) == 0 {
		return nil
	}
	return &issues[len(issues)-1]
}

// A logic.switch builds a port per case whenever its ports are asked for; with
// many cases and many edges leaving it, that was one build per edge.
func TestValidateManyEdgesFromABigSwitchIsFast(t *testing.T) {
	reg := newTestRegistry(t)
	const cases, edges = 100000, 1990
	b := newFlow("Switch")
	tr := b.node("start", "test.trigger", nil)
	one := cond("a", "eq", "b")
	list := make([]any, cases)
	for i := range list {
		list[i] = map[string]any{"condition": one}
	}
	sw := b.node("sw", TypeSwitch, map[string]any{"cases": list})
	b.edge(tr, PortOut, sw)
	var targets []string
	for i := 0; i < 20; i++ {
		targets = append(targets, b.node("t"+strconv.Itoa(i), "test.echo", nil))
	}
	for i := 0; i < edges; i++ {
		b.edge(sw, casePort(i), targets[i%len(targets)])
	}
	f := b.build()
	issues := timeValidate(t, f, reg, publishCtx(), 2*time.Second)
	// The switch has more cases than allowed; nothing else is wrong.
	if len(issues) != 1 || issues[0].Code != IssueParamInvalid || issues[0].NodeID != sw {
		t.Fatalf("issues = %+v", issues)
	}
}

// The definition callbacks run once per source node, whatever the number of
// edges or references.
func TestValidateCallsDefinitionCallbacksOncePerNode(t *testing.T) {
	reg := newTestRegistry(t)
	portCalls, fieldCalls := 0, 0
	const nFields = 20000
	reg.MustRegister(&NodeDef{Type: "test.ports", Category: "test",
		OutputsFunc: func(*Node) []string {
			portCalls++
			return []string{PortOut, "a", "b"}
		}})
	reg.MustRegister(&NodeDef{Type: "test.fields", Category: "test",
		OutputFieldsFunc: func(*Node) []FieldSpec {
			fieldCalls++
			fs := make([]FieldSpec, nFields)
			for i := range fs {
				fs[i] = FieldSpec{Name: "f" + strconv.Itoa(i), Type: "any"}
			}
			return fs
		}})

	b := newFlow("Callbacks")
	tr := b.node("start", "test.trigger", nil)
	ports := b.node("ports", "test.ports", nil)
	src := b.node("src", "test.fields", nil)
	b.edge(tr, PortOut, ports)
	b.edge(tr, PortOut, src)
	for i := 0; i < 20; i++ {
		var sb strings.Builder
		for r := 0; r < 900; r++ {
			fmt.Fprintf(&sb, "{{src.f%d}}", (r*37+i)%nFields)
		}
		c := b.node("c"+strconv.Itoa(i), "test.echo", map[string]any{"value": sb.String()})
		b.edge(src, PortOut, c)
	}
	// 300 targets on each of the 3 ports: 900 distinct edges leave the ports node.
	for i := 0; i < 300; i++ {
		x := b.node("x"+strconv.Itoa(i), "test.echo", nil)
		for _, port := range []string{PortOut, "a", "b"} {
			b.edge(ports, port, x)
		}
	}
	f := b.build()
	start := time.Now()
	issues := Validate(f, reg, publishCtx())
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("Validate took %v with %d fields and %d references", elapsed, nFields, 20*900)
	}
	if len(issues) != 0 {
		t.Fatalf("every reference names a declared field and every edge is valid: %+v", issues[:1])
	}
	if portCalls != 1 {
		t.Errorf("OutputsFunc ran %d times for a node with 900 outgoing edges, want 1", portCalls)
	}
	if fieldCalls != 1 {
		t.Errorf("OutputFieldsFunc ran %d times for a node read by %d references, want 1", fieldCalls, 20*900)
	}
}

// ruleCase builds one publish-rule problem and says how severe it is in each mode.
type ruleCase struct {
	name      string
	noTrigger bool
	build     func(b *flowBuilder, tr string) string
	code      string
	publish   Severity
	draft     Severity
}

func TestValidateSeveritiesInDraftAndPublish(t *testing.T) {
	reg := newTestRegistry(t)
	connect := func(b *flowBuilder, tr, key, typ string, params map[string]any) string {
		id := b.node(key, typ, params)
		b.edge(tr, PortOut, id)
		return id
	}
	echo := func(value string) func(b *flowBuilder, tr string) string {
		return func(b *flowBuilder, tr string) string {
			return connect(b, tr, "e", "test.echo", map[string]any{"value": value})
		}
	}
	const (
		E = SeverityError
		W = SeverityWarning
	)
	cases := []ruleCase{
		// Publish rules: errors when publishing, warnings in a draft.
		{name: "no trigger", noTrigger: true, build: func(b *flowBuilder, _ string) string { b.node("echo", "test.echo", nil); return "" }, code: IssueNoTrigger, publish: E, draft: W},
		{name: "unknown type", build: func(b *flowBuilder, tr string) string { return connect(b, tr, "x", "nope.x", nil) }, code: IssueNodeTypeUnknown, publish: E, draft: W},
		{name: "unavailable", build: func(b *flowBuilder, tr string) string { return connect(b, tr, "tg", "test.unavailable", nil) }, code: IssueNodeUnavailable, publish: E, draft: W},
		{name: "required", build: func(b *flowBuilder, tr string) string { return connect(b, tr, "req", "test.required", nil) }, code: IssueParamRequired, publish: E, draft: W},
		{name: "visible required", build: func(b *flowBuilder, tr string) string {
			return connect(b, tr, "req", "test.required", map[string]any{"text": "t", "mode": "b"})
		}, code: IssueParamRequired, publish: E, draft: W},
		{name: "syntax", build: echo("{{oops"), code: IssueTemplateSyntax, publish: E, draft: W},
		{name: "unknown root", build: echo("{{ghost.x}}"), code: IssueTemplateUnknownRoot, publish: E, draft: W},
		{name: "not upstream", build: func(b *flowBuilder, tr string) string {
			connect(b, tr, "a", "test.echo", nil)
			return connect(b, tr, "b", "test.echo", map[string]any{"value": "{{a.value}}"})
		}, code: IssueTemplateNotUpstream, publish: E, draft: W},
		{name: "item root", build: echo("{{item.x}}"), code: IssueTemplateRootUnavailable, publish: E, draft: W},
		{name: "cycle", build: func(b *flowBuilder, tr string) string {
			a := connect(b, tr, "a", "test.echo", nil)
			c := b.node("c", "test.echo", nil)
			b.edge(a, PortOut, c)
			b.edge(c, PortOut, a)
			return a
		}, code: IssueCycle, publish: E, draft: W},
		{name: "node validator", build: func(b *flowBuilder, tr string) string {
			return connect(b, tr, "w", TypeWait, map[string]any{"mode": "duration", "seconds": 9999.0})
		}, code: IssueParamInvalid, publish: E, draft: W},
		// Always warnings.
		{name: "unknown field", build: func(b *flowBuilder, tr string) string {
			a := connect(b, tr, "a", "test.echo", nil)
			c := b.node("c", "test.echo", map[string]any{"value": "{{a.nope}}"})
			b.edge(a, PortOut, c)
			return c
		}, code: IssueTemplateUnknownField, publish: W, draft: W},
		{name: "orphan", build: func(b *flowBuilder, _ string) string { return b.node("lonely", "test.echo", nil) }, code: IssueNodeUnreachable, publish: W, draft: W},
		{name: "untrusted", build: func(b *flowBuilder, tr string) string {
			hook := b.node("hook", "test.untrusted_trigger", nil)
			return connect(b, hook, "shell", "test.sink", map[string]any{"command": "{{trigger.data.cmd}}"})
		}, code: IssueUntrustedData, publish: W, draft: W},
		// Structural problems: errors in both modes.
		{name: "duplicate key", build: func(b *flowBuilder, tr string) string { return connect(b, tr, "start", "test.echo", nil) }, code: IssueNodeKeyDuplicate, publish: E, draft: E},
		{name: "self edge", build: func(b *flowBuilder, tr string) string {
			a := connect(b, tr, "a", "test.echo", nil)
			b.edge(a, PortOut, a)
			return a
		}, code: IssueEdgeSelf, publish: E, draft: E},
		{name: "bad output port", build: func(b *flowBuilder, tr string) string {
			a := b.node("a", "test.echo", nil)
			b.edge(tr, PortTrue, a)
			return tr
		}, code: IssueEdgePortInvalid, publish: E, draft: E},
	}
	for _, tc := range cases {
		for _, mode := range []struct {
			name string
			vc   ValidateContext
			want Severity
		}{{"publish", publishCtx(), tc.publish}, {"draft", draftCtx(), tc.draft}} {
			b := newFlow(tc.name)
			tr := ""
			if !tc.noTrigger {
				tr = b.node("start", "test.trigger", nil)
			}
			nodeID := tc.build(b, tr)
			issues := Validate(b.build(), reg, mode.vc)
			is := findIssue(issues, tc.code, nodeID)
			if is == nil || is.Severity != mode.want {
				t.Errorf("%s in %s mode: want %s/%s on %q, got %+v", tc.name, mode.name, tc.code, mode.want, nodeID, issues)
			}
		}
	}
}

// Template sources are echoed in messages; control characters in them must not
// reach logs or API consumers raw.
func TestValidateEchoesTemplateSourcesSafely(t *testing.T) {
	reg := newTestRegistry(t)
	ctl := `"a` + "\x1b[31m\nb" + `"` // a string literal with ESC and a newline in it
	b := newFlow("Control characters")
	tr := b.node("start", "test.trigger", nil)
	a := b.node("a", "test.echo", nil)
	d := b.node("d", "test.echo", nil)
	e := b.node("e", "test.echo", map[string]any{
		"loop":  "{{item.x | default(" + ctl + ")}}",
		"root":  "{{ghost.x | default(" + ctl + ")}}",
		"up":    "{{d.value | default(" + ctl + ")}}",
		"field": "{{a.nope | default(" + ctl + ")}}",
	})
	b.edge(tr, PortOut, a)
	b.edge(tr, PortOut, d)
	b.edge(a, PortOut, e)
	issues := Validate(b.build(), reg, publishCtx())
	for _, code := range []string{IssueTemplateRootUnavailable, IssueTemplateUnknownRoot, IssueTemplateNotUpstream, IssueTemplateUnknownField} {
		is := findIssue(issues, code, e)
		if is == nil {
			t.Fatalf("no %s issue: %+v", code, issues)
		}
		for i := 0; i < len(is.Message); i++ {
			if c := is.Message[i]; c < 0x20 || c == 0x7f {
				t.Errorf("%s: message has the raw control byte %#x: %q", code, c, is.Message)
				break
			}
		}
		if !strings.Contains(is.Message, `\x1b`) || !strings.Contains(is.Message, `\n`) {
			t.Errorf("%s: the control characters must show up escaped: %q", code, is.Message)
		}
	}
	// Ordinary sources stay readable.
	if is := findIssue(Validate(func() *Flow {
		bb := newFlow("Plain")
		t0 := bb.node("start", "test.trigger", nil)
		bb.edge(t0, PortOut, bb.node("e", "test.echo", map[string]any{"value": `{{ghost.x | default("hi")}}`}))
		return bb.build()
	}(), reg, publishCtx()), IssueTemplateUnknownRoot, ""); is == nil || is.Message != `{{ghost.x | default("hi")}} refers to an unknown node "ghost"` {
		t.Errorf("plain source message = %+v", is)
	}
}
