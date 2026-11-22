package storymaker

import (
	"context"
	"strings"
	"testing"
)

const cuePhraseScene = `"environment":"env.office","speaker":"anna","actors":[{"cast":"anna","at":"left","pose":"present"}]`

func TestStudioSceneWriteResolvesCuePhrase(t *testing.T) {
	ctx := context.Background()
	studio, ps, id := newStudioWriteRig(t, nil)
	studio.SetPhase(PhaseFill, "s1")
	answer := studioWriteCall(t, studio, ToolStory, "put_scene", `{"scene_id":"s1","data":{`+cuePhraseScene+`,
		"narration":"Eine Firewall schuetzt das Netz. Das Netz bleibt sicher.",
		"beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop","cue_phrase":"das netz"}]}}`)
	if answer["status"] != "ok" {
		t.Fatalf("answer = %v, want ok", answer)
	}
	if next, _ := answer["next_action"].(string); !strings.Contains(next, "occurs 2 times") || !strings.Contains(next, "This scene is saved.") {
		t.Fatalf("next_action = %q, want the saved text and the note", next)
	}
	raw, _, err := ps.LoadStory(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	story, _, err := DecodeStory(raw, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	s1 := story.Scenes[0]
	if want := "Eine Firewall schuetzt {1}das Netz. Das Netz bleibt sicher."; s1.Narration != want || s1.Beats[0].Cue != 1 {
		t.Fatalf("saved narration %q cue %d, want %q and 1", s1.Narration, s1.Beats[0].Cue, want)
	}
	if strings.Contains(string(raw), "cue_phrase") {
		t.Fatal("cue_phrase reached story.json")
	}

	// A patch that only sends a beat finds the phrase in the saved narration, whose marker it keeps.
	answer = studioWriteCall(t, studio, ToolStory, "patch_scene", `{"scene_id":"s1","data":{
		"beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop","cue_phrase":"Firewall"}]}}`)
	if answer["status"] != "ok" {
		t.Fatalf("patch answer = %v, want ok", answer)
	}
	raw, _, _ = ps.LoadStory(ctx, id)
	story, _, _ = DecodeStory(raw, DefaultLimits())
	if got := story.Scenes[0].Narration; got != "Eine {2}Firewall schuetzt {1}das Netz. Das Netz bleibt sicher." {
		t.Fatalf("narration after the patch = %q", got)
	}
}

func TestStudioSceneWriteRefusesCuePhrase(t *testing.T) {
	ctx := context.Background()
	studio, ps, id := newStudioWriteRig(t, nil)
	studio.SetPhase(PhaseFill, "s1")
	_, before, _ := ps.LoadStory(ctx, id)
	answer := studioWriteCall(t, studio, ToolStory, "put_scene", `{"scene_id":"s1","data":{`+cuePhraseScene+`,
		"beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop","cue_phrase":"Firewal"}]}}`)
	if answer["status"] != "refused" || answer["code"] != "cue_phrase_not_found" || !strings.Contains(answer["message"].(string), `"Firewall"`) {
		t.Fatalf("answer = %v, want cue_phrase_not_found naming Firewall", answer)
	}
	if _, now, _ := ps.LoadStory(ctx, id); now != before {
		t.Fatal("a refused write changed the story")
	}
	if studio.writes["s1"] != 1 {
		t.Fatalf("writes = %d, want the refused write counted", studio.writes["s1"])
	}
}

func TestStudioDryRunResolvesCuePhrase(t *testing.T) {
	studio, _, _ := newStudioWriteRig(t, nil)
	studio.SetPhase(PhaseFill, "s1")
	answer := studioWriteCall(t, studio, ToolStory, "dry_run", `{"scene_id":"s1","data":{`+cuePhraseScene+`,
		"beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop","cue_phrase":"schuetzt"}]}}`)
	if answer["status"] != "ok" || len(answer["issues"].([]any)) != 0 {
		t.Fatalf("answer = %v, want ok without issues", answer)
	}
	bad := studioWriteCall(t, studio, ToolStory, "dry_run", `{"scene_id":"s1","data":{`+cuePhraseScene+`,
		"beats":[{"do":"show","item":"prop.lock","at":"right","entry":"pop","cue_phrase":"nirgends"}]}}`)
	if bad["status"] != "refused" || bad["code"] != "cue_phrase_not_found" {
		t.Fatalf("answer = %v, want cue_phrase_not_found", bad)
	}
}
