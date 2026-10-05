package server

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"aurago/internal/flows"
	"aurago/internal/tools"
)

// Trigger settings decode strictly: an unknown key, a key in other case or a value of the
// wrong type fails with a short message instead of dropping a filter, and only trigger
// types that Mission Control starts flows for pass.
func TestC15TriggerConfigsAreStrict(t *testing.T) {
	mission := func(trigger string, cfg map[string]any) []flows.TriggerBinding {
		return []flows.TriggerBinding{{NodeID: "n_aaaaaaaa", Kind: flows.BindingMission, MissionTrigger: trigger, Config: cfg}}
	}
	long := strings.Repeat("k", 500)
	for _, tc := range []struct {
		name     string
		bindings []flows.TriggerBinding
		want     string
	}{
		{"unknown key", mission("webhook", map[string]any{"webhook_id": "h1", "foo": 1.0}), `trigger n_aaaaaaaa: unknown setting "foo"`},
		{"other case", mission("webhook", map[string]any{"Webhook_ID": "h1"}), `trigger n_aaaaaaaa: unknown setting "Webhook_ID"`},
		{"long key", mission("webhook", map[string]any{long: "x"}), `trigger n_aaaaaaaa: unknown setting "` + strings.Repeat("k", flowNameEchoRunes) + `…"`},
		{"wrong type", mission("webhook", map[string]any{"min_interval_seconds": "soon"}), `trigger n_aaaaaaaa: the setting "min_interval_seconds" has the wrong type`},
		{"fraction", mission("mqtt_message", map[string]any{"mqtt_topic": "a", "mqtt_min_interval_seconds": 1.5}), `the setting "mqtt_min_interval_seconds" has the wrong type`},
		{"egg event", mission("egg_hatched", nil), `waits for the event "egg_hatched", which Mission Control cannot start flows for`},
		{"flow-only type", mission("schedule", nil), `waits for the event "schedule"`},
		{"unknown type", mission("bogus", nil), `waits for the event "bogus"`},
		{"long node id", []flows.TriggerBinding{{NodeID: strings.Repeat("n", 500), Kind: "bogus"}}, `trigger ` + strings.Repeat("n", flowNameEchoRunes) + `… has an unknown binding "bogus"`},
	} {
		_, err := flowTriggerSpecs(tc.bindings)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", tc.name, err, tc.want)
		}
		if err != nil && utf8.RuneCountInString(err.Error()) > 200 {
			t.Errorf("%s: the error is not bounded: %d runes", tc.name, utf8.RuneCountInString(err.Error()))
		}
	}
	// MissionManagerV2.SyncFlowMission checks only node ids and would store any trigger
	// type, which is why the bridge checks the type.
	e := c15NewEnv(t)
	id := e.c15Mission(t, "flow_c15strict01", "Strict")
	if err := e.s.MissionManagerV2.SyncFlowMission(id, "Strict", []tools.FlowTriggerSpec{{NodeID: "n_aaaaaaaa", TriggerType: "bogus"}}); err != nil {
		t.Fatalf("SyncFlowMission validates trigger types now (%v); the bridge's check may be documented as a second line", err)
	}
	if err := e.bridge.SyncFlowMission(id, "Strict", mission("bogus", nil)); err == nil {
		t.Fatal("the bridge must refuse an unknown trigger type")
	}
}

// c15ParamSample returns a non-empty example value for a parameter.
func c15ParamSample(p flows.ParamSpec) any {
	switch {
	case len(p.Options) > 0 && p.Kind == flows.ParamMultiSelect:
		return []any{p.Options[0].Value}
	case len(p.Options) > 0:
		return p.Options[len(p.Options)-1].Value
	}
	switch p.Kind {
	case flows.ParamNumber:
		return 90.0
	case flows.ParamBool:
		return true
	case flows.ParamDateTime:
		return "2026-12-24T18:00:00Z"
	case flows.ParamCron:
		return "0 7 * * *"
	case flows.ParamJSON:
		return map[string]any{"c15": true}
	default:
		return "c15-" + p.Name
	}
}

// c15ParamVariants returns the parameter sets the catalog test binds: the defaults plus a
// sample for every required parameter without a default; the defaults plus a sample of
// every parameter; and that full set once per option of each parameter with options.
func c15ParamVariants(def *flows.NodeDef) []map[string]any {
	minimal, full := map[string]any{}, map[string]any{}
	for _, p := range def.Params {
		if p.Default != nil {
			minimal[p.Name] = p.Default
		} else if p.Required {
			minimal[p.Name] = c15ParamSample(p)
		}
		full[p.Name] = c15ParamSample(p)
	}
	variants := []map[string]any{minimal, full}
	for _, p := range def.Params {
		if p.Kind == flows.ParamMultiSelect {
			continue
		}
		for _, o := range p.Options {
			v := map[string]any{}
			for k, val := range full {
				v[k] = val
			}
			v[p.Name] = o.Value
			variants = append(variants, v)
		}
	}
	return variants
}

func c15JSONValue(t *testing.T, v any) any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// Every Mission Control trigger of the node catalog, bound by the flows package itself
// from default and sample parameters, decodes into tools.TriggerConfig with every value it
// sets, and the catalog uses exactly the trigger types the bridge accepts.
func TestC15CatalogTriggerConfigsDecode(t *testing.T) {
	reg := flows.NewRegistry()
	if err := flows.RegisterCatalog(reg, flows.StaticEnv{}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	missionTypes := map[string]bool{}
	seen := map[tools.TriggerType]bool{}
	for _, def := range reg.All() {
		if !def.Trigger {
			continue
		}
		for _, params := range c15ParamVariants(def) {
			node := flows.Node{ID: "n_aaaaaaaa", Key: "trigger", Type: def.Type, Params: params}
			bindings, err := flows.BindTriggers(&flows.Flow{Nodes: []flows.Node{node}}, reg, time.UTC, now)
			if err != nil || len(bindings) != 1 {
				t.Fatalf("%s %v: bindings %+v, err %v", def.Type, params, bindings, err)
			}
			b := bindings[0]
			if b.Kind != flows.BindingMission {
				break // manual, schedule and date/time do not depend on the variant
			}
			missionTypes[def.Type] = true
			specs, err := flowTriggerSpecs(bindings)
			if err != nil {
				t.Fatalf("%s %v: %v", def.Type, b.Config, err)
			}
			seen[specs[0].TriggerType] = true
			got, _ := c15JSONValue(t, specs[0].TriggerConfig).(map[string]any)
			for k, v := range b.Config {
				want := c15JSONValue(t, v)
				if want == nil || want == "" || want == false || want == 0.0 {
					if _, present := got[k]; present {
						t.Errorf("%s: empty %s came through as %v", def.Type, k, got[k])
					}
					continue
				}
				if !reflect.DeepEqual(got[k], want) {
					t.Errorf("%s: %s = %v in TriggerConfig, want %v (config %v)", def.Type, k, got[k], want, b.Config)
				}
			}
			for k := range got {
				if _, ok := b.Config[k]; !ok {
					t.Errorf("%s: TriggerConfig has %s, which the binding did not set", def.Type, k)
				}
			}
		}
	}
	if len(missionTypes) != 10 {
		t.Fatalf("mission trigger node types = %v, want 10", missionTypes)
	}
	if len(seen) != len(flowMissionTriggerTypes) {
		t.Fatalf("the catalog uses %v; the bridge accepts %v", seen, flowMissionTriggerTypes)
	}
}
