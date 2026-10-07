package flows

import (
	"fmt"
	"math"
	"reflect"
	"testing"
)

// The trigger sampler is handed to describeNodeType, so a test can give it one that
// panics. The sample of a trigger that panics is left out and logged; a sampler that
// works is called with the sample node, and what it returns is copied.
func TestDescribeNodeTypeTriggerSample(t *testing.T) {
	reg := NewRegistry()
	if err := RegisterTriggerNodes(reg); err != nil {
		t.Fatal(err)
	}
	def := lookupDef(t, reg, TypeTriggerWebhook)
	tr := func(key string) string { return key }

	t.Run("panics", func(t *testing.T) {
		logs := catDescCaptureLogs(t)
		info := describeNodeType(def, tr, func(*Node) map[string]any { panic("sample boom") })
		if info.Sample != nil || !info.Trigger || !reflect.DeepEqual(info.Outputs, []string{PortOut}) || len(info.Params) == 0 {
			t.Errorf("panicking sampler: %+v", info)
		}
		if got, want := logs.fallbacks(), []string{TypeTriggerWebhook + "/TriggerSample"}; !reflect.DeepEqual(got, want) {
			t.Errorf("logged %v, want %v", got, want)
		}
	})

	t.Run("result is copied", func(t *testing.T) {
		kept := map[string]any{"payload": map[string]any{"a": []any{"x"}}, "n": 1.0}
		var got *Node
		info := describeNodeType(def, tr, func(n *Node) map[string]any { got = n; return kept })
		if got == nil || got.Type != TypeTriggerWebhook {
			t.Fatalf("the sampler got %+v", got)
		}
		if !reflect.DeepEqual(info.Sample, kept) {
			t.Fatalf("sample = %v", info.Sample)
		}
		info.Sample["payload"].(map[string]any)["a"].([]any)[0] = "changed"
		info.Sample["n"] = 2.0
		if kept["n"] != 1.0 || kept["payload"].(map[string]any)["a"].([]any)[0] != "x" {
			t.Errorf("the sampler's map was changed through the description: %v", kept)
		}
	})

	t.Run("nil and unencodable results", func(t *testing.T) {
		for name, sample := range map[string]map[string]any{"nil": nil, "NaN": {"f": math.NaN()}} {
			info := describeNodeType(def, tr, func(*Node) map[string]any { return sample })
			if len(info.Sample) != 0 {
				t.Errorf("%s: sample = %v", name, info.Sample)
			}
		}
	})

	t.Run("real catalog", func(t *testing.T) {
		if byType := catDescByType(DescribeNodeTypes(reg, nil)); byType[TypeTriggerWebhook].Sample["payload"] == nil {
			t.Error("the webhook trigger has no sample")
		}
	})
}

// Only the three known states reach the palette. Any other state, the empty one too, is
// blocked: it keeps its reason and config section, and gets a fixed reason without one.
func TestDescribeNodeTypesNormalizesAvailability(t *testing.T) {
	cases := []struct {
		name string
		in   Availability
		want Availability
	}{
		{"zero value", Availability{}, Availability{State: BlockedState, Reason: unknownStateReason}},
		{"unknown state", Availability{State: "ready"}, Availability{State: BlockedState, Reason: unknownStateReason}},
		{"unknown state with reason", Availability{State: "ready", Reason: "custom", ConfigSection: "mqtt"}, Availability{State: BlockedState, Reason: "custom", ConfigSection: "mqtt"}},
		{"available", Availability{State: AvailableState}, Availability{State: AvailableState}},
		{"needs setup", Availability{State: NeedsSetupState, Reason: "r", ConfigSection: "s"}, Availability{State: NeedsSetupState, Reason: "r", ConfigSection: "s"}},
		{"blocked", Availability{State: BlockedState, Reason: "readonly"}, Availability{State: BlockedState, Reason: "readonly"}},
		{"wrong case", Availability{State: "Available"}, Availability{State: BlockedState, Reason: unknownStateReason}},
	}
	reg := NewRegistry()
	for i, c := range cases {
		in := c.in
		reg.MustRegister(catDescHookDef(fmt.Sprintf("test.n%d", i), func(d *NodeDef) {
			d.AvailabilityFunc = func() Availability { return in }
		}))
	}
	byType := catDescByType(DescribeNodeTypes(reg, nil))
	for i, c := range cases {
		if got := byType[fmt.Sprintf("test.n%d", i)].Availability; got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
	if unknownStateReason == "" {
		t.Error("the fixed reason is empty")
	}
}
