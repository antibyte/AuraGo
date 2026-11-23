package storymaker

import (
	"context"
	"testing"
)

func TestFreeStudioWritesOnlyCodeScenes(t *testing.T) {
	ctx := context.Background()
	s, ps, id := newFreeStudioRig(t)
	s.SetPhase(PhaseFill, "s1")

	for name, data := range map[string]map[string]any{
		"actors":      {"actors": []any{map[string]any{"cast": "anna", "at": "left"}}},
		"beats":       {"beats": []any{}},
		"props":       {"props": []any{}},
		"environment": {"environment": "env.office"},
		"speaker":     {"speaker": "anna"},
		"kind":        {"kind": "storyboard"},
	} {
		got := codeCall(t, s, "put_scene", "s1", data, "")
		if got["status"] != "refused" || got["code"] != "free_mode_code_only" {
			t.Errorf("%s: put_scene = %v", name, got)
		}
	}
	if got := codeCall(t, s, "patch_scene", "s1", map[string]any{"beats": []any{}}, ""); got["code"] != "free_mode_code_only" {
		t.Errorf("patch_scene beats = %v", got)
	}

	// Without a bundled source the activation fails the same way as in the catalog mode.
	if got := codeCall(t, s, "put_scene", "s1", map[string]any{}, ""); got["code"] != CodeCodeError {
		t.Fatalf("activation without source = %v", got)
	}
	read := codeCall(t, s, "get_code", "s1", nil, "")
	write := codeCall(t, s, "put_code", "s1", map[string]any{"source": freeMotionExample}, read["rev"].(string))
	if write["status"] != "ok" {
		t.Fatalf("put_code = %v", write)
	}
	// data without kind still ends as a code scene, and the outline narration stays.
	if got := codeCall(t, s, "put_scene", "s1", map[string]any{}, ""); got["status"] != "ok" || !s.PhaseDone() {
		t.Fatalf("activation = %v", got)
	}
	raw, _, err := ps.LoadStory(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	story, _, err := DecodeStory(raw, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	if story.Scenes[0].Kind != "code" || story.Scenes[0].Narration != "Eine Firewall schuetzt das Netz." {
		t.Fatalf("scene s1 = %+v", story.Scenes[0])
	}
}

func TestFreeStudioSetMetaTakesNoCastAndNoVoice(t *testing.T) {
	ctx := context.Background()
	s, _, _ := newFreeStudioRig(t)
	s.SetPhase(PhaseRevise, "")
	cast := map[string]any{"data": map[string]any{"cast": []any{map[string]any{"id": "anna", "name": "Anna", "character": "char.office-woman-1"}}}}
	if got := answerOf(t, s.Call(ctx, StudioCall{Tool: ToolStory, Operation: "set_meta", Args: cast})); got["code"] != "free_mode_code_only" {
		t.Fatalf("set_meta cast = %v", got)
	}
	voice := map[string]any{"data": map[string]any{"voice": map[string]any{"provider": "phonon", "voice": "x"}}}
	if got := answerOf(t, s.Call(ctx, StudioCall{Tool: ToolStory, Operation: "set_meta", Args: voice})); got["code"] != "free_mode_code_only" {
		t.Fatalf("set_meta voice = %v", got)
	}
	title := map[string]any{"data": map[string]any{"title": "Sicher im Netz"}}
	if got := answerOf(t, s.Call(ctx, StudioCall{Tool: ToolStory, Operation: "set_meta", Args: title})); got["status"] != "ok" {
		t.Fatalf("set_meta title = %v", got)
	}
}
