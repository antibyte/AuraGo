package flows

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func lookupLogic(t *testing.T, typ string) *NodeDef {
	t.Helper()
	reg := NewRegistry()
	if err := RegisterLogicNodes(reg); err != nil {
		t.Fatalf("RegisterLogicNodes: %v", err)
	}
	def, ok := reg.Lookup(typ)
	if !ok {
		t.Fatalf("%s not registered", typ)
	}
	return def
}

// runDef executes a definition directly; it is safe to call from goroutines.
func runDef(def *NodeDef, params map[string]any, inputs []NodeInput, svc *Services) (ExecResult, error) {
	if svc == nil {
		svc = &Services{Location: time.UTC}
	}
	return def.Execute(context.Background(), ExecInput{
		Node: &Node{ID: testNodeID(1), Type: def.Type, Params: params}, Params: params, Inputs: inputs, Services: svc,
	})
}

func execLogic(t *testing.T, typ string, params map[string]any, inputs []NodeInput, svc *Services) (ExecResult, error) {
	t.Helper()
	return runDef(lookupLogic(t, typ), params, inputs, svc)
}

func cond(left any, op string, right any) map[string]any {
	return map[string]any{"match": "all", "rows": []any{map[string]any{"left": left, "op": op, "right": right}}}
}

func TestIfNode(t *testing.T) {
	res, err := execLogic(t, TypeIf, map[string]any{"condition": cond(5.0, "gt", 3.0)}, nil, nil)
	if err != nil || !reflect.DeepEqual(res.Ports, []string{PortTrue}) || res.Output["result"] != true {
		t.Fatalf("true branch = %+v, %v", res, err)
	}
	res, err = execLogic(t, TypeIf, map[string]any{"condition": cond(1.0, "gt", 3.0)}, nil, nil)
	if err != nil || !reflect.DeepEqual(res.Ports, []string{PortFalse}) {
		t.Fatalf("false branch = %+v, %v", res, err)
	}
	_, err = execLogic(t, TypeIf, map[string]any{"condition": "nonsense"}, nil, nil)
	if ne := asNodeError(err); ne.Code != "FLOW_CONDITION_INVALID" {
		t.Fatalf("invalid condition error = %v", err)
	}
}

func TestSwitchNode(t *testing.T) {
	cases := []any{
		map[string]any{"label": "klein", "condition": cond(2.0, "lt", 5.0)},
		map[string]any{"label": "auch klein", "condition": cond(2.0, "lt", 10.0)},
	}
	res, err := execLogic(t, TypeSwitch, map[string]any{"cases": cases, "mode": "first"}, nil, nil)
	if err != nil || !reflect.DeepEqual(res.Ports, []string{"case_1"}) || res.Output["case"] != "klein" {
		t.Fatalf("first = %+v, %v", res, err)
	}
	res, err = execLogic(t, TypeSwitch, map[string]any{"cases": cases, "mode": "all"}, nil, nil)
	if err != nil || !reflect.DeepEqual(res.Ports, []string{"case_1", "case_2"}) {
		t.Fatalf("all = %+v, %v", res, err)
	}
	none := []any{map[string]any{"label": "gross", "condition": cond(2.0, "gt", 5.0)}}
	res, err = execLogic(t, TypeSwitch, map[string]any{"cases": none}, nil, nil)
	if err != nil || !reflect.DeepEqual(res.Ports, []string{PortDefault}) || res.Output["case"] != "" {
		t.Fatalf("default = %+v, %v", res, err)
	}
	reg := NewRegistry()
	_ = RegisterLogicNodes(reg)
	def, _ := reg.Lookup(TypeSwitch)
	if got := def.OutputPorts(&Node{Params: map[string]any{"cases": cases}}); !reflect.DeepEqual(got, []string{"case_1", "case_2", PortDefault}) {
		t.Fatalf("switch ports = %v", got)
	}
}

func TestMergeNode(t *testing.T) {
	inputs := []NodeInput{
		{Key: "a", Output: map[string]any{"items": []any{1.0, 2.0}}},
		{Key: "b", Output: map[string]any{"x": "y"}},
	}
	res, err := execLogic(t, TypeMerge, map[string]any{}, inputs, nil)
	if err != nil || res.Output["a"] == nil || res.Output["b"] == nil {
		t.Fatalf("wait_all = %+v, %v", res, err)
	}
	res, err = execLogic(t, TypeMerge, map[string]any{"mode": "append", "field": "items"}, inputs, nil)
	want := []any{1.0, 2.0, map[string]any{"x": "y"}}
	if err != nil || !reflect.DeepEqual(res.Output["items"], want) || res.ItemCount != 3 {
		t.Fatalf("append = %+v, %v", res, err)
	}
}

func TestSetNode(t *testing.T) {
	fields := []any{
		map[string]any{"name": "n", "type": "number", "value": "42"},
		map[string]any{"name": "t", "type": "text", "value": 7.0},
		map[string]any{"name": "b", "type": "bool", "value": "ja"},
		map[string]any{"name": "l", "type": "list", "value": `[1,2]`},
		map[string]any{"name": "o", "type": "object", "value": map[string]any{"k": "v"}},
		map[string]any{"name": "a", "value": []any{"x"}},
	}
	inputs := []NodeInput{{Key: "prev", Output: map[string]any{"keep": "me"}}}
	res, err := execLogic(t, TypeSet, map[string]any{"fields": fields, "keep_input": true}, inputs, nil)
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	want := map[string]any{"keep": "me", "n": 42.0, "t": "7", "b": true, "l": []any{1.0, 2.0}, "o": map[string]any{"k": "v"}, "a": []any{"x"}}
	if !reflect.DeepEqual(res.Output, want) {
		t.Fatalf("set output = %#v", res.Output)
	}
	_, err = execLogic(t, TypeSet, map[string]any{"fields": []any{map[string]any{"name": "n", "type": "number", "value": "abc"}}}, nil, nil)
	if ne := asNodeError(err); ne.Code != "FLOW_VALUE_TYPE" {
		t.Fatalf("conversion error = %v", err)
	}
}

func TestStopNode(t *testing.T) {
	res, err := execLogic(t, TypeStop, map[string]any{"status": "error", "message": "nein"}, nil, nil)
	if err != nil || res.Stop == nil || res.Stop.Status != RunError || res.Stop.Message != "nein" {
		t.Fatalf("stop = %+v, %v", res, err)
	}
	res, _ = execLogic(t, TypeStop, map[string]any{}, nil, nil)
	if res.Stop == nil || res.Stop.Status != RunSuccess {
		t.Fatalf("stop default = %+v", res)
	}
}

func TestWaitNode(t *testing.T) {
	clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	svc := &Services{Clock: clock, Location: time.UTC}
	def := lookupLogic(t, TypeWait)
	done := make(chan ExecResult, 1)
	go func() {
		res, err := runDef(def, map[string]any{"mode": "duration", "seconds": 90.0}, nil, svc)
		if err != nil {
			t.Errorf("wait: %v", err)
		}
		done <- res
	}()
	clock.WaitForWaiters(t, 1)
	clock.Advance(90 * time.Second)
	select {
	case res := <-done:
		if res.Output["waited_seconds"] != 90.0 {
			t.Fatalf("waited = %#v", res.Output)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("wait did not finish")
	}

	clock = newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	svc = &Services{Clock: clock, Location: time.UTC}
	go func() {
		res, err := runDef(def, map[string]any{"mode": "until", "until": "07:30"}, nil, svc)
		if err != nil {
			t.Errorf("wait until: %v", err)
		}
		done <- res
	}()
	clock.WaitForWaiters(t, 1)
	clock.Advance(30 * time.Minute)
	if res := <-done; res.Output["waited_seconds"] != 1800.0 {
		t.Fatalf("waited until = %#v", res.Output)
	}

	svc = &Services{Clock: newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC)), Location: time.UTC}
	_, err := execLogic(t, TypeWait, map[string]any{"mode": "until", "until": "06:00"}, nil, svc)
	if ne := asNodeError(err); ne.Code != "FLOW_WAIT_TOO_LONG" {
		t.Fatalf("too long = %v", err)
	}
	_, err = execLogic(t, TypeWait, map[string]any{"mode": "duration", "seconds": 7200.0}, nil, svc)
	if ne := asNodeError(err); ne.Code != "FLOW_PARAM_INVALID" {
		t.Fatalf("too many seconds = %v", err)
	}
}

func TestLogicNodeValidators(t *testing.T) {
	reg := NewRegistry()
	_ = RegisterLogicNodes(reg)
	check := func(typ string, params map[string]any, wantCode string) {
		t.Helper()
		def, _ := reg.Lookup(typ)
		issues := def.Validate(&Node{ID: testNodeID(1), Type: typ, Params: params}, ValidateContext{Mode: ModePublish})
		if wantCode == "" {
			if len(issues) != 0 {
				t.Errorf("%s %v: unexpected issues %+v", typ, params, issues)
			}
			return
		}
		if len(issues) == 0 || issues[0].Code != wantCode {
			t.Errorf("%s %v: issues %+v, want %s", typ, params, issues, wantCode)
		}
	}
	check(TypeIf, map[string]any{"condition": cond("a", "nope", "b")}, IssueParamInvalid)
	check(TypeIf, map[string]any{"condition": cond("a", "eq", "b")}, "")
	check(TypeWait, map[string]any{"mode": "duration", "seconds": 0.0}, IssueParamInvalid)
	check(TypeWait, map[string]any{"mode": "duration", "seconds": "{{trigger.data.s}}"}, "")
	check(TypeWait, map[string]any{"mode": "until", "until": "25:00"}, IssueParamInvalid)
	check(TypeWait, map[string]any{"mode": "until", "until": "23:59"}, "")
	check(TypeSet, map[string]any{"fields": []any{map[string]any{"name": ""}}}, IssueParamRequired)
	check(TypeSwitch, map[string]any{"cases": []any{map[string]any{"condition": cond("a", "bad", "b")}}}, IssueParamInvalid)
}

func snapshotOf(t *testing.T, v any) string {
	t.Helper()
	s, err := marshalCompact(v)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	return s
}

// Nodes receive data that can share memory with the run (single-expression
// templates return the referenced value itself), so merge and set must leave
// their inputs and params untouched and hand out fresh containers.
func TestMergeNodeDoesNotMutateInputs(t *testing.T) {
	// Spare capacity makes an append onto the received list write into shared memory.
	list := make([]any, 2, 8)
	list[0], list[1] = 1.0, 2.0
	inputs := []NodeInput{
		{Key: "a", Output: map[string]any{"items": list}},
		{Key: "b", Output: map[string]any{"x": "y"}},
	}
	params := map[string]any{"mode": "append", "field": "items"}
	beforeInputs, beforeParams := snapshotOf(t, inputs), snapshotOf(t, params)

	res, err := execLogic(t, TypeMerge, params, inputs, nil)
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	items, _ := res.Output["items"].([]any)
	if len(items) != 3 {
		t.Fatalf("append items = %#v", res.Output["items"])
	}
	// A consumer that owns the result may change and extend it.
	items[0] = "changed"
	_ = append(items[:1], "tail")
	res.Output["extra"] = true
	if got := snapshotOf(t, inputs); got != beforeInputs {
		t.Fatalf("inputs changed: %s, want %s", got, beforeInputs)
	}
	if got := snapshotOf(t, params); got != beforeParams {
		t.Fatalf("params changed: %s, want %s", got, beforeParams)
	}
	if spare := list[:3][2]; spare != nil {
		t.Fatalf("append wrote into the spare capacity of an input list: %#v", spare)
	}

	res, err = execLogic(t, TypeMerge, map[string]any{}, inputs, nil)
	if err != nil {
		t.Fatalf("wait_all: %v", err)
	}
	res.Output["extra"] = true
	delete(res.Output, "a")
	if got := snapshotOf(t, inputs); got != beforeInputs {
		t.Fatalf("wait_all changed inputs: %s, want %s", got, beforeInputs)
	}
}

func TestSetNodeDoesNotMutateInputs(t *testing.T) {
	inputs := []NodeInput{{Key: "prev", Output: map[string]any{"keep": "me", "n": "old", "list": []any{"a"}}}}
	params := map[string]any{
		"keep_input": true,
		"fields": []any{
			map[string]any{"name": "n", "type": "number", "value": "5"},
			map[string]any{"name": "added", "type": "text", "value": "v"},
			map[string]any{"name": "keep", "value": "overwritten"},
		},
	}
	beforeInputs, beforeParams := snapshotOf(t, inputs), snapshotOf(t, params)

	res, err := execLogic(t, TypeSet, params, inputs, nil)
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if res.Output["n"] != 5.0 || res.Output["keep"] != "overwritten" || res.Output["added"] != "v" {
		t.Fatalf("set output = %#v", res.Output)
	}
	res.Output["injected"] = true
	delete(res.Output, "list")
	if got := snapshotOf(t, inputs); got != beforeInputs {
		t.Fatalf("inputs changed: %s, want %s", got, beforeInputs)
	}
	if got := snapshotOf(t, params); got != beforeParams {
		t.Fatalf("params changed: %s, want %s", got, beforeParams)
	}
}

// A param that resolves to null is present with a nil value and must not
// panic or be mistaken for a configured value.
func TestLogicNodesNullParams(t *testing.T) {
	codeOf := func(err error) string {
		if err == nil {
			return ""
		}
		return asNodeError(err).Code
	}

	// if / switch
	for _, params := range []map[string]any{
		nil,
		{"condition": nil},
		{"condition": map[string]any(nil)},
		{"condition": []any(nil)},
	} {
		if _, err := execLogic(t, TypeIf, params, nil, nil); codeOf(err) != "FLOW_CONDITION_INVALID" {
			t.Errorf("if %v: error = %v", params, err)
		}
	}
	res, err := execLogic(t, TypeSwitch, map[string]any{"cases": nil, "mode": nil}, nil, nil)
	if err != nil || !reflect.DeepEqual(res.Ports, []string{PortDefault}) {
		t.Errorf("switch null cases = %+v, %v", res, err)
	}
	for _, cases := range [][]any{{nil}, {map[string]any{"label": nil, "condition": nil}}} {
		if _, err := execLogic(t, TypeSwitch, map[string]any{"cases": cases}, nil, nil); codeOf(err) != "FLOW_CONDITION_INVALID" {
			t.Errorf("switch cases %v: error = %v", cases, err)
		}
	}

	// merge
	inputs := []NodeInput{{Key: "a", Output: nil}, {Key: "b", Output: map[string]any{"x": 1.0}}}
	res, err = execLogic(t, TypeMerge, map[string]any{"mode": nil, "field": nil}, inputs, nil)
	if err != nil || len(res.Output) != 2 {
		t.Errorf("merge null params = %+v, %v", res, err)
	}
	res, err = execLogic(t, TypeMerge, map[string]any{"mode": "append", "field": nil}, inputs, nil)
	if err != nil || res.ItemCount != 2 {
		t.Errorf("merge append null field = %+v, %v", res, err)
	}

	// wait
	for _, params := range []map[string]any{
		nil,
		{"mode": nil, "seconds": nil},
		{"mode": "duration", "seconds": nil},
		{"mode": "until", "until": nil},
	} {
		if _, err := execLogic(t, TypeWait, params, nil, nil); codeOf(err) != "FLOW_PARAM_INVALID" {
			t.Errorf("wait %v: error = %v", params, err)
		}
	}

	// set
	res, err = execLogic(t, TypeSet, map[string]any{"fields": nil, "keep_input": nil}, nil, nil)
	if err != nil || len(res.Output) != 0 {
		t.Errorf("set null fields = %+v, %v", res, err)
	}
	res, err = execLogic(t, TypeSet, map[string]any{
		"fields": []any{nil, map[string]any{"name": nil}, map[string]any{"name": "t", "type": "text", "value": nil}, map[string]any{"name": "a", "value": nil}},
	}, nil, nil)
	if err != nil || !reflect.DeepEqual(res.Output, map[string]any{"t": "", "a": nil}) {
		t.Errorf("set null values = %#v, %v", res.Output, err)
	}
	for _, typ := range []string{"number", "list", "object"} {
		fields := []any{map[string]any{"name": "x", "type": typ, "value": nil}}
		if _, err := execLogic(t, TypeSet, map[string]any{"fields": fields}, nil, nil); codeOf(err) != "FLOW_VALUE_TYPE" {
			t.Errorf("set null as %s: error = %v", typ, err)
		}
	}

	// stop
	res, err = execLogic(t, TypeStop, map[string]any{"status": nil, "message": nil}, nil, nil)
	if err != nil || res.Stop == nil || res.Stop.Status != RunSuccess || res.Stop.Message != "" {
		t.Errorf("stop null params = %+v, %v", res, err)
	}

	// Execute without services must not panic either.
	def := lookupLogic(t, TypeIf)
	res, err = def.Execute(context.Background(), ExecInput{Params: map[string]any{"condition": cond(1.0, "eq", 1.0)}})
	if err != nil || !reflect.DeepEqual(res.Ports, []string{PortTrue}) {
		t.Errorf("if without services = %+v, %v", res, err)
	}
}

// Errors and issues that echo user text stay short whatever the user typed.
func TestLogicNodeErrorsAreBounded(t *testing.T) {
	const limit = 300
	huge := strings.Repeat("ü", 10000)
	check := func(what string, msg string) {
		t.Helper()
		if msg == "" {
			t.Errorf("%s: no error", what)
			return
		}
		if len(msg) > limit {
			t.Errorf("%s: message has %d bytes, want at most %d: %.80s...", what, len(msg), limit, msg)
		}
	}
	errText := func(err error) string {
		if err == nil {
			return ""
		}
		return err.Error()
	}

	_, err := execLogic(t, TypeIf, map[string]any{"condition": cond("a", huge, "b")}, nil, nil)
	check("if operator", errText(err))
	_, err = execLogic(t, TypeIf, map[string]any{"condition": cond("a", "matches", "("+huge)}, nil, nil)
	check("if pattern", errText(err))
	_, err = execLogic(t, TypeSwitch, map[string]any{"cases": []any{map[string]any{"condition": cond("a", huge, "b")}}}, nil, nil)
	check("switch operator", errText(err))

	_, err = execLogic(t, TypeWait, map[string]any{"mode": "until", "until": huge}, nil, nil)
	check("wait until", errText(err))

	_, err = execLogic(t, TypeSet, map[string]any{"fields": []any{map[string]any{"name": huge, "type": "number", "value": huge}}}, nil, nil)
	check("set long name and value", errText(err))
	_, err = execLogic(t, TypeSet, map[string]any{"fields": []any{map[string]any{"name": "x", "type": huge, "value": "v"}}}, nil, nil)
	check("set unknown type", errText(err))
	_, err = execLogic(t, TypeSet, map[string]any{"fields": []any{map[string]any{"name": "x", "type": "number", "value": []any{huge}}}}, nil, nil)
	check("set list as number", errText(err))

	reg := NewRegistry()
	_ = RegisterLogicNodes(reg)
	validate := func(typ string, params map[string]any) string {
		def, _ := reg.Lookup(typ)
		issues := def.Validate(&Node{ID: testNodeID(1), Type: typ, Params: params}, ValidateContext{Mode: ModePublish})
		if len(issues) == 0 {
			return ""
		}
		return issues[0].Message
	}
	check("if validator", validate(TypeIf, map[string]any{"condition": cond("a", huge, "b")}))
	check("switch validator", validate(TypeSwitch, map[string]any{"cases": []any{map[string]any{"condition": cond("a", huge, "b")}}}))
	check("set duplicate name", validate(TypeSet, map[string]any{"fields": []any{
		map[string]any{"name": huge}, map[string]any{"name": huge},
	}}))
}

func TestWaitNodeHonoursCancellation(t *testing.T) {
	clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	svc := &Services{Clock: clock, Location: time.UTC}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	def := lookupLogic(t, TypeWait)
	_, err := def.Execute(ctx, ExecInput{Params: map[string]any{"mode": "duration", "seconds": 30.0}, Services: svc})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled wait = %v", err)
	}
}

func TestLogicNodePorts(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterLogicNodes(reg); err != nil {
		t.Fatalf("RegisterLogicNodes: %v", err)
	}
	if err := RegisterLogicNodes(reg); err == nil {
		t.Fatal("registering the logic nodes twice should fail")
	}
	want := map[string][]string{
		TypeIf:     {PortTrue, PortFalse},
		TypeSwitch: {PortDefault},
		TypeMerge:  {PortOut},
		TypeWait:   {PortOut},
		TypeSet:    {PortOut},
		TypeStop:   {},
	}
	for typ, ports := range want {
		def, ok := reg.Lookup(typ)
		if !ok {
			t.Fatalf("%s not registered", typ)
		}
		got := def.OutputPorts(&Node{})
		if len(got) != len(ports) || (len(ports) > 0 && !reflect.DeepEqual(got, ports)) {
			t.Errorf("%s ports = %v, want %v", typ, got, ports)
		}
		if def.Category != "logic" || def.Execute == nil {
			t.Errorf("%s: category %q, execute set %v", typ, def.Category, def.Execute != nil)
		}
	}
	stop, _ := reg.Lookup(TypeStop)
	if got := stop.OutputPorts(&Node{Settings: NodeSettings{OnError: ErrorPort}}); !reflect.DeepEqual(got, []string{PortError}) {
		t.Errorf("stop with error port = %v", got)
	}
}
