package flows

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

// catDescShared is a definition that hands out its own slices and defaults, the way a
// definition written in a hurry does, and whose hooks scribble on the sample node.
func catDescShared(seen *[]string) *NodeDef {
	shared := []Effect{EffectSendsMessage}
	scribble := func(n *Node) {
		*seen = append(*seen, fmt.Sprintf("%v|%v|%v|%v", n.Params["list"], n.Params["obj"], n.Params["tags"], n.Params["mode"]))
		n.Params["list"].([]any)[0] = "HOOK"
		n.Params["obj"].(map[string]any)["a"] = "HOOK"
		n.Params["tags"].([]any)[0] = "HOOK"
		delete(n.Params, "mode")
	}
	return &NodeDef{
		Type: "test.shared", Category: "test", Label: "Shared",
		Inputs: []string{"in"}, Outputs: []string{"a", "b"},
		Params: []ParamSpec{
			{Name: "list", Kind: ParamJSON, Default: []any{"x", map[string]any{"k": []any{1.0}}}},
			{Name: "obj", Kind: ParamJSON, Default: map[string]any{"a": []any{"b"}}},
			{Name: "tags", Kind: ParamTags, Default: []string{"t"}},
			{Name: "mode", Kind: ParamSelect, Default: "one", Options: []Option{{Value: "one", Label: "One"}},
				VisibleIf: &Visibility{Param: "list", Equals: []string{"p", "q"}}},
			{Name: "nested", Kind: ParamFields, Fields: []ParamSpec{{Name: "inner", Kind: ParamJSON, Default: []any{"i"},
				VisibleIf: &Visibility{Param: "x", Equals: []string{"y"}}}}},
		},
		OutputsFunc:      func(n *Node) []string { scribble(n); return []string{"a", "b"} },
		OutputFieldsFunc: func(n *Node) []FieldSpec { scribble(n); return nil },
		EffectsFunc:      func(n *Node) []Effect { scribble(n); return shared },
	}
}

// DescribeNodeTypes hands out copies: a caller that changes the result cannot change the
// definition or the next description, and a hook that changes its sample node cannot
// change the definition or what the next hook sees.
func TestDescribeNodeTypesSharesNothingWithTheDefinition(t *testing.T) {
	var seen []string
	def := catDescShared(&seen)
	reg := NewRegistry()
	reg.MustRegister(def)
	if err := RegisterTriggerNodes(reg); err != nil {
		t.Fatal(err)
	}
	pristine := catDescShared(new([]string))

	first := DescribeNodeTypes(reg, nil)
	before, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	info := catDescByType(first)["test.shared"]

	// Change everything reachable from the description.
	info.Inputs[0], info.Outputs[0], info.Effects[0] = "M", "M", "M"
	info.Params[0].Default.([]any)[0] = "M"
	info.Params[0].Default.([]any)[1].(map[string]any)["k"] = "M"
	info.Params[1].Default.(map[string]any)["a"] = "M"
	info.Params[2].Default.([]any)[0] = "M"
	info.Params[3].Options[0].Value = "M"
	info.Params[3].VisibleIf.Equals[0], info.Params[3].VisibleIf.Param = "M", "M"
	info.Params[4].Fields[0].Default.([]any)[0] = "M"
	info.Params[4].Fields[0].VisibleIf.Equals[0] = "M"
	hook := catDescByType(first)[TypeTriggerWebhook]
	hook.Sample["payload"], hook.Sample["extra"] = "M", "M"
	for _, v := range hook.Sample {
		if m, ok := v.(map[string]any); ok {
			m["extra"] = "M"
		}
	}

	if !reflect.DeepEqual(def.Params, pristine.Params) || !reflect.DeepEqual(def.Inputs, pristine.Inputs) ||
		!reflect.DeepEqual(def.Outputs, pristine.Outputs) {
		t.Errorf("the definition was changed through the description: %+v", def.Params)
	}
	// What the hook returned as its own slice is untouched.
	if got := def.EffectsFunc(&Node{Params: sampleParams(def)}); got[0] != EffectSendsMessage {
		t.Errorf("the effects slice of the hook was changed: %v", got)
	}
	second, err := json.Marshal(DescribeNodeTypes(reg, nil))
	if err != nil {
		t.Fatal(err)
	}
	if string(second) != string(before) {
		t.Errorf("the second description differs:\n%s\n%s", before, second)
	}

	// Every hook call, whatever scribbling came before, saw the defaults.
	want := "[x map[k:[1]]]|map[a:[b]]|[t]|one"
	if len(seen) < 3*2 {
		t.Fatalf("hooks ran %d times", len(seen))
	}
	for i, s := range seen {
		if s != want {
			t.Errorf("hook call %d saw %q", i, s)
		}
	}
}

// cloneParamValue copies what it can, and drops what it cannot copy instead of sharing it.
func TestCloneParamValue(t *testing.T) {
	list := []any{"a", map[string]any{"k": []any{1.0}}}
	got, ok := cloneParamValue(list).([]any)
	if !ok || !reflect.DeepEqual(got, list) {
		t.Fatalf("clone = %#v", got)
	}
	got[1].(map[string]any)["k"].([]any)[0] = "changed"
	if list[1].(map[string]any)["k"].([]any)[0] != 1.0 {
		t.Error("the clone shares memory with the original")
	}
	for _, v := range []any{nil, "s", true, 3, 2.5, int64(7), uint8(1)} {
		if cloneParamValue(v) != v {
			t.Errorf("scalar %#v changed", v)
		}
	}
	if v := cloneParamValue(map[string]any{"f": func() {}}); v != nil {
		t.Errorf("a value JSON cannot encode = %#v", v)
	}
	if v := cloneParamValue([]string(nil)); v != nil {
		t.Errorf("nil list = %#v", v)
	}
}
