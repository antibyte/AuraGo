package flows

import (
	"reflect"
	"strings"
	"testing"
)

// A literal service that runs code or changes the system says so in the effects, which is
// what makes the publish dialog ask (IsRisky). A service that is a template, or cannot be
// read, stays a plain device control.
func TestHomeAssistantRiskyServiceEffects(t *testing.T) {
	def := lookupDef(t, homeRegistry(t), TypeHomeAssistant)
	controls := []Effect{EffectControlsDevices}
	code := []Effect{EffectControlsDevices, EffectRunsCode}
	system := []Effect{EffectControlsDevices, EffectSystemChange}
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
		{"plain service, entity a template", map[string]any{"entity": "{{x.e}}", "service": "restart"}, controls, false},
		{"plain service, no entity", map[string]any{"service": "restart"}, controls, false},
		{"plain service, entity unreadable", map[string]any{"entity": []any{"shell_command.x"}, "service": "backup"}, controls, false},
		// Not readable now, so as before.
		{"template service", map[string]any{"entity": "light.a", "service": "{{x.s}}"}, controls, false},
		{"template domain", map[string]any{"entity": "light.a", "service": "{{x.d}}.restart"}, controls, false},
		{"upper case is not a service", map[string]any{"entity": "light.a", "service": "Shell_Command.x"}, controls, false},
		{"no service", map[string]any{"entity": "light.a"}, controls, false},
		{"service of the wrong type", map[string]any{"entity": "light.a", "service": []any{"shell_command.x"}}, controls, false},
		{"huge service", map[string]any{"entity": "light.a", "service": "shell_command." + strings.Repeat("a", 1<<16)}, controls, false},
		// A template as the operation can still be a service call.
		{"template operation", map[string]any{"operation": "{{x.o}}", "entity": "light.a", "service": "shell_command.x"}, code, true},
		// A get_state that is written out runs nothing, whatever the hidden service says.
		{"get_state", map[string]any{"operation": "get_state", "entity": "light.a", "service": "shell_command.x"}, nil, false},
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
