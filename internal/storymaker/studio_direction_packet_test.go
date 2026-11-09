package storymaker

import (
	"context"
	"encoding/json"
	"testing"
)

// Every example of the fill packet is a scene the validator accepts: it is built from the scene's own direction.
func TestFillExamplesValidate(t *testing.T) {
	cat := loadReal(t)
	s := loadAgentFixture(t, "agent-varied")
	for i, d := range DirectionPlan(&s, "", cat) {
		raw, err := json.Marshal(packetFillExample(s.Cast, d))
		if err != nil {
			t.Fatal(err)
		}
		var sc Scene
		if err := json.Unmarshal(raw, &sc); err != nil {
			t.Fatalf("scene %d: %v: %s", i+1, err, raw)
		}
		sc.ID = "s1"
		one := s
		one.Scenes = []Scene{sc}
		if issues := Validate(&one, cat, DefaultLimits()); HasErrors(issues) {
			t.Errorf("example %d (%s): %s\n%s", i+1, d.Role, dump(issues), raw)
		}
	}
}

func TestFillPacketCarriesTheDirectionAndAnExampleFromIt(t *testing.T) {
	ctx := context.Background()
	studio, ps, projectID := newStudioWriteRig(t, nil)
	saveFixture(t, ps, projectID, "agent-firewall-run")

	studio.SetPhase(PhaseFill, "s3")
	packet, err := studio.Packet(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	dir, _ := packet["direction"].(map[string]any)
	example, _ := packet["example"].(map[string]any)
	if dir == nil || example == nil {
		t.Fatalf("packet lacks direction or example: %v", packet)
	}
	if dir["role"] != RoleSolution || dir["environment"] != "env.abstract-tech" || dir["note"] != directionNote {
		t.Errorf("direction of s3 = %v", dir)
	}
	if example["environment"] != dir["environment"] {
		t.Errorf("the example must use the direction's environment: %v vs %v", example["environment"], dir["environment"])
	}
	envs, _ := packet["environments"].([]map[string]any)
	if len(envs) == 0 || envs[0]["about"] == nil {
		t.Errorf("the environments of a fill packet carry the guide line: %v", envs)
	}
	// Different scenes get different directions and examples.
	studio.SetPhase(PhaseFill, "s1")
	other, err := studio.Packet(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	otherDir, _ := other["direction"].(map[string]any)
	if otherDir["role"] != RoleHook || otherDir["environment"] == dir["environment"] {
		t.Errorf("s1 must be the hook in another environment than s3: %v", otherDir)
	}
}

func TestOutlineAndCastPacketsCarryTheEnvironmentGuide(t *testing.T) {
	ctx := context.Background()
	studio, _, _ := newStudioWriteRig(t, nil)
	for _, phase := range []string{PhaseOutline, PhaseCast} {
		studio.SetPhase(phase, "")
		packet, err := studio.Packet(ctx, false)
		if err != nil {
			t.Fatal(err)
		}
		if names, _ := packet["environments"].([]string); len(names) == 0 {
			t.Errorf("%s: environments must stay a list of names: %v", phase, packet["environments"])
		}
		if guide, _ := packet["environment_guide"].([]map[string]any); len(guide) < 12 {
			t.Errorf("%s: environment_guide = %v", phase, packet["environment_guide"])
		}
	}
}
