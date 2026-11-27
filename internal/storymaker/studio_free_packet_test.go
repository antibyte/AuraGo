package storymaker

import (
	"context"
	"strings"
	"testing"
)

func freeLintHas(issues []Issue, code string) bool {
	for _, is := range issues {
		if is.Code == code {
			return true
		}
	}
	return false
}

func TestLintForJobModeAppliesTheNarrationBudgetToFreeCodeScenes(t *testing.T) {
	story := &Story{Scenes: []Scene{{ID: "s1", Kind: "code", Narration: strings.Repeat("Wort ", 40)}}}
	if freeLintHas(LintForJob(story), CodeOverBudget) {
		t.Error("the catalog mode keeps skipping code scenes")
	}
	if !freeLintHas(LintForJobMode(story, ModeFree), CodeOverBudget) {
		t.Error("the free mode must apply the narration budget to code scenes")
	}
	story.Scenes[0].Silent = true
	if freeLintHas(LintForJobMode(story, ModeFree), CodeOverBudget) {
		t.Error("a silent scene has no narration budget")
	}
}

func TestFreePacketsCarryNoCatalogOrCast(t *testing.T) {
	ctx := context.Background()
	s, _, _ := newFreeStudioRig(t)
	banned := []string{"environments", "environment_guide", "cast", "characters", "suggested_props", "direction", "example_beats", "character_looks"}
	for _, phase := range []string{PhaseOutline, PhaseFill, PhaseCheck, PhaseRevise} {
		s.SetPhase(phase, "s1")
		packet, err := s.Packet(ctx, false)
		if err != nil {
			t.Fatalf("%s packet: %v", phase, err)
		}
		for _, key := range banned {
			if _, has := packet[key]; has {
				t.Errorf("%s packet carries %s", phase, key)
			}
		}
		if story, ok := packet["story"].(map[string]any); ok {
			for _, row := range story["scenes"].([]map[string]any) {
				if _, has := row["environment"]; has {
					t.Errorf("%s packet summary names an environment", phase)
				}
			}
		}
	}
	s.SetPhase(PhaseFill, "s1")
	packet, err := s.Packet(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	scene, _ := packet["scene"].(map[string]any)
	if scene["id"] != "s1" || scene["total"] != 2 || packet["max_code_bytes"] != StudioCodeBytes {
		t.Fatalf("fill packet scene = %v, max_code_bytes = %v", scene, packet["max_code_bytes"])
	}
	if next, _ := packet["next_scene"].(map[string]any); next["id"] != "s2" {
		t.Fatalf("next_scene = %v", packet["next_scene"])
	}
}

func TestFreeFillPacketCarriesCodeRevCardAndSafeBox(t *testing.T) {
	ctx := context.Background()
	s, ps, projectID := newFreeStudioRig(t)
	s.SetPhase(PhaseFill, "s1")
	packet, err := s.Packet(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	story, _, err := ps.LoadStory(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	st, _, err := DecodeStory(story, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	ApplyDefaults(&st)
	if packet["code_rev"] != MustNotExist {
		t.Fatalf("code_rev = %v, want %q", packet["code_rev"], MustNotExist)
	}
	if box, _ := packet["safe_box"].(SafeBox); box != SafeBoxFor(st.Format.Aspect) {
		t.Fatalf("safe_box = %v, want %+v", packet["safe_box"], SafeBoxFor(st.Format.Aspect))
	}
	card, _ := packet["api_card"].(string)
	if !strings.Contains(card, "api.safe()") || !strings.Contains(card, "Palette of this film: $bg #") || len(card) > 1800 {
		t.Fatalf("api_card has %d bytes: %s", len(card), card)
	}
	if next, _ := packet["next"].(string); !strings.HasPrefix(next, "put_code with expected_rev = code_rev") {
		t.Fatalf("next = %q", next)
	}

	put := studioWriteCall(t, s, ToolStory, "put_code", `{"scene_id":"s1","expected_rev":"`+MustNotExist+`","data":{"source":"export default function scene(api) { api.duration(5); }"}}`)
	if put["status"] != "ok" || put["rev"] == nil {
		t.Fatalf("put_code with code_rev and no get_code = %v", put)
	}
	again, err := s.Packet(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if again["code_rev"] != put["rev"] {
		t.Fatalf("code_rev = %v, want the rev of the put_code answer %v", again["code_rev"], put["rev"])
	}
}
