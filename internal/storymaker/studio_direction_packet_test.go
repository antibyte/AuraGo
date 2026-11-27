package storymaker

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// Every example of the fill packet is a scene the validator accepts: it is built from the scene's own direction.
func TestFillExamplesValidate(t *testing.T) {
	cat := loadReal(t)
	s := loadAgentFixture(t, "agent-varied")
	for i, d := range DirectionPlan(&s, "", cat) {
		example := packetFillExample(s.Cast, d, "", "de")
		// The example teaches cue_phrase, never a numbered cue, and its phrase must resolve like a model's would.
		beat := example["beats"].([]any)[0].(map[string]any)
		if beat["cue_phrase"] == nil || beat["cue"] != nil || strings.Contains(example["narration"].(string), "{") {
			t.Errorf("example %d (%s) must use cue_phrase and a plain narration: %v", i+1, d.Role, example)
		}
		if _, refusal := resolveCuePhrases(example); refusal != nil {
			t.Fatalf("example %d (%s): %+v", i+1, d.Role, refusal)
		}
		raw, err := json.Marshal(example)
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

// With a suggested block the example shows that block at the board and the actor points at it; every such example validates.
func TestFillPacketExampleUsesTheSuggestedBlock(t *testing.T) {
	cat := loadReal(t)
	s := loadAgentFixture(t, "agent-varied")
	dir := DirectionPlan(&s, "", cat)[2]
	plainExample := packetFillExample(s.Cast, dir, "", "de")
	if beat := plainExample["beats"].([]any)[0].(map[string]any); beat["item"] == nil || beat["diagram"] != nil {
		t.Fatalf("without a block the example shows its prop: %v", beat)
	}
	for block, want := range map[string]string{"flow": "diagram", "cycle": "diagram", "stack": "diagram", "compare": "diagram", "chart": "chart"} {
		example := packetFillExample(s.Cast, dir, block, "de")
		beats := example["beats"].([]any)
		show := beats[0].(map[string]any)
		if show["do"] != "show" || show["item"] != nil || show[want] == nil || show["at"] != "board" || show["cue_phrase"] == nil {
			t.Errorf("%s: show beat = %v", block, show)
		}
		if d, ok := show["diagram"].(map[string]any); ok && d["layout"] != block {
			t.Errorf("%s: diagram layout = %v", block, d["layout"])
		}
		if gesture := beats[1].(map[string]any); gesture["gesture"] != "point" || gesture["target"] != "board" || gesture["with"] != "prev" {
			t.Errorf("%s: gesture beat = %v", block, gesture)
		}
		if _, refusal := resolveCuePhrases(example); refusal != nil {
			t.Fatalf("%s: %+v", block, refusal)
		}
		raw, err := json.Marshal(example)
		if err != nil {
			t.Fatal(err)
		}
		var sc Scene
		if err := json.Unmarshal(raw, &sc); err != nil {
			t.Fatalf("%s: %v: %s", block, err, raw)
		}
		sc.ID = "s1"
		one := s
		one.Scenes = []Scene{sc}
		if issues := Validate(&one, cat, DefaultLimits()); HasErrors(issues) {
			t.Errorf("%s: %s\n%s", block, dump(issues), raw)
		}
	}
}

// The worked example speaks the job's language: German for a German job, English for every other one, and its cue phrase
// still resolves.
func TestFillPacketExampleLanguage(t *testing.T) {
	cat := loadReal(t)
	s := loadAgentFixture(t, "agent-varied")
	dir := DirectionPlan(&s, "", cat)[2]
	for language, want := range map[string]string{"de": "Paket", "de-AT": "Paket", "en": "Packet", "en-US": "Packet", "fr": "Packet", "": "Packet"} {
		example := packetFillExample(s.Cast, dir, "flow", language)
		raw, _ := json.Marshal(example)
		if !strings.Contains(string(raw), `"label":"`+want+`"`) {
			t.Errorf("language %q: example = %s, want the label %q", language, raw, want)
		}
		if (want == "Packet") != strings.HasPrefix(example["narration"].(string), "The ") {
			t.Errorf("language %q: narration = %q", language, example["narration"])
		}
		if _, refusal := resolveCuePhrases(example); refusal != nil {
			t.Errorf("language %q: %+v", language, refusal)
		}
	}
	chart := packetFillExample(s.Cast, dir, "chart", "en")["beats"].([]any)[0].(map[string]any)["data"].(map[string]any)
	if labels := chart["labels"].([]string); labels[0] != "Allowed" {
		t.Errorf("chart labels = %v", labels)
	}
}
