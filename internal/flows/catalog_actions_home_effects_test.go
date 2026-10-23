package flows

import (
	"reflect"
	"strings"
	"testing"
)

// A literal service that runs code or changes the system says so in the effects, which is
// what makes the publish dialog ask (IsRisky). A service whose domain the document does not
// settle (a template, text that is no service, a plain service on an entity that is a
// template or no id) counts as the worst case, because the run decides it. Nothing written
// yet (no service, no entity for a plain service) is still a plain device control: the run
// refuses it.
func TestHomeAssistantRiskyServiceEffects(t *testing.T) {
	def := lookupDef(t, homeRegistry(t), TypeHomeAssistant)
	controls := []Effect{EffectControlsDevices}
	code := []Effect{EffectControlsDevices, EffectRunsCode}
	system := []Effect{EffectControlsDevices, EffectSystemChange}
	worst := []Effect{EffectControlsDevices, EffectRunsCode, EffectSystemChange}
	for _, c := range []struct {
		name   string
		params map[string]any
		want   []Effect
		risky  bool
	}{
		{"light", map[string]any{"entity": "light.a", "service": "light.turn_on"}, controls, false},
		{"plain service", map[string]any{"entity": "light.a", "service": "turn_on"}, controls, false},
		{"toggle", map[string]any{"entity": "switch.a", "service": "homeassistant.toggle"}, controls, false},
		{"turn_off", map[string]any{"entity": "light.a", "service": "homeassistant.turn_off"}, controls, false},
		{"script", map[string]any{"entity": "script.a", "service": "script.turn_on"}, controls, false},
		{"shell_command", map[string]any{"entity": "light.a", "service": "shell_command.backup"}, code, true},
		{"python_script", map[string]any{"entity": "light.a", "service": "python_script.run"}, code, true},
		{"hassio", map[string]any{"entity": "light.a", "service": "hassio.host_reboot"}, system, true},
		{"hassio addon", map[string]any{"entity": "light.a", "service": "hassio.addon_stdin"}, system, true},
		{"restart", map[string]any{"entity": "light.a", "service": "homeassistant.restart"}, system, true},
		{"stop", map[string]any{"entity": "light.a", "service": "homeassistant.stop"}, system, true},
		{"written with blanks", map[string]any{"entity": "light.a", "service": "  shell_command.backup "}, code, true},
		// A plain service takes the entity's domain, when the entity is written out as well.
		{"plain shell_command", map[string]any{"entity": "shell_command.x", "service": "backup"}, code, true},
		{"plain restart on a homeassistant entity", map[string]any{"entity": "homeassistant.x", "service": "restart"}, system, true},
		{"plain restart on a light", map[string]any{"entity": "light.a", "service": "restart"}, controls, false},
		{"plain service, entity a template", map[string]any{"entity": "{{x.e}}", "service": "restart"}, worst, true},
		{"plain service, no entity", map[string]any{"service": "restart"}, controls, false},
		{"plain service, entity unreadable", map[string]any{"entity": []any{"shell_command.x"}, "service": "backup"}, worst, true},
		{"plain service, entity no id", map[string]any{"entity": "Shell Command", "service": "backup"}, worst, true},
		// The domain is not settled by the document: the worst case.
		{"template service", map[string]any{"entity": "light.a", "service": "{{x.s}}"}, worst, true},
		{"template domain", map[string]any{"entity": "light.a", "service": "{{x.d}}.restart"}, worst, true},
		{"upper case is not a service", map[string]any{"entity": "light.a", "service": "Shell_Command.x"}, worst, true},
		{"service of the wrong type", map[string]any{"entity": "light.a", "service": []any{"shell_command.x"}}, worst, true},
		{"huge service", map[string]any{"entity": "light.a", "service": "shell_command." + strings.Repeat("a", 1<<16)}, worst, true},
		{"template service and template entity", map[string]any{"entity": "{{x.e}}", "service": "{{x.s}}"}, worst, true},
		// Nothing written yet: the node cannot run, so nothing is hidden.
		{"no service", map[string]any{"entity": "light.a"}, controls, false},
		{"blank service", map[string]any{"entity": "light.a", "service": "  "}, controls, false},
		// A template as the operation can still be a service call.
		{"template operation", map[string]any{"operation": "{{x.o}}", "entity": "light.a", "service": "shell_command.x"}, code, true},
		// A get_state that is written out runs nothing, whatever the hidden service says.
		{"get_state", map[string]any{"operation": "get_state", "entity": "light.a", "service": "shell_command.x"}, nil, false},
		{"get_state, entity a template", map[string]any{"operation": "get_state", "entity": "{{x.e}}", "service": "{{x.s}}"}, nil, false},
	} {
		node := &Node{ID: testNodeID(1), Key: "n", Type: TypeHomeAssistant, Params: c.params}
		got := def.EffectsOf(node)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: effects %v, want %v", c.name, got, c.want)
		}
		if IsRisky(got) != c.risky {
			t.Errorf("%s: IsRisky = %v", c.name, IsRisky(got))
		}
	}
	// The plan's expectation for a bare node holds.
	if got := def.EffectsOf(&Node{Params: map[string]any{}}); !reflect.DeepEqual(got, controls) {
		t.Errorf("a bare node: %v", got)
	}

	// The publish dialog lists each node under the effects it has.
	b := newFlow("Risky")
	b.node("start", TypeTriggerManual, nil)
	b.node("a", TypeHomeAssistant, map[string]any{"entity": "light.a", "service": "shell_command.backup"})
	b.node("b", TypeHomeAssistant, map[string]any{"entity": "light.a", "service": "homeassistant.restart"})
	b.node("c", TypeHomeAssistant, map[string]any{"entity": "light.a", "service": "light.turn_on"})
	reg := triggerRegistry(t)
	if err := registerHomeNodes(reg, nil); err != nil {
		t.Fatal(err)
	}
	f := b.build()
	byEffect := map[Effect][]string{}
	for _, s := range CollectEffects(f, reg) {
		byEffect[s.Effect] = s.NodeIDs
	}
	if !reflect.DeepEqual(byEffect[EffectRunsCode], []string{f.Nodes[1].ID}) || !reflect.DeepEqual(byEffect[EffectSystemChange], []string{f.Nodes[2].ID}) ||
		len(byEffect[EffectControlsDevices]) != 3 {
		t.Errorf("collected effects %v", byEffect)
	}
}
