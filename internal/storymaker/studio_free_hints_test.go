package storymaker

import (
	"context"
	"strings"
	"testing"
)

const catalogUnfilledFix = `Add actors and beats with patch_scene. Example: {"actors":[{"cast":"anna","at":"left","pose":"present"}],"beats":[{"do":"show","text":"Kurz gesagt","at":"right","entry":"pop"}]}`

func unfilledFix(t *testing.T, issues []Issue) string {
	t.Helper()
	for _, is := range issues {
		if is.Code == CodeSceneUnfilled {
			return is.Fix
		}
	}
	t.Fatalf("no scene_unfilled in %+v", issues)
	return ""
}

func TestFreeLintUnfilledSceneFixNamesCodeScenes(t *testing.T) {
	story := &Story{Scenes: []Scene{{ID: "s1", Narration: "Eine Firewall schuetzt das Netz."}}}
	fix := unfilledFix(t, LintForJobMode(story, ModeFree))
	want := `A free scene is a code scene: call get_code, then put_code with the source, then patch_scene with {"kind":"code"}. It has no actors or beats.`
	if fix != want {
		t.Errorf("free scene_unfilled fix = %q", fix)
	}
	// The catalog example (actors, beats) must not reach a free model.
	for _, catalogHint := range []string{"Add actors", `"actors":[`, `"cast"`, `"beats":[`, "Example"} {
		if strings.Contains(fix, catalogHint) {
			t.Errorf("free scene_unfilled fix still carries %q", catalogHint)
		}
	}
	if !strings.Contains(fix, "put_code") {
		t.Error("free scene_unfilled fix must name put_code")
	}
	for name, got := range map[string]string{
		"LintForJob":             unfilledFix(t, LintForJob(story)),
		"LintForJobMode catalog": unfilledFix(t, LintForJobMode(story, ModeCatalog)),
		"LintForJobMode no mode": unfilledFix(t, LintForJobMode(story, "")),
		"LintForJobMode unknown": unfilledFix(t, LintForJobMode(story, "junk")),
	} {
		if got != catalogUnfilledFix {
			t.Errorf("%s: the catalog fix changed: %q", name, got)
		}
	}
}

func TestFreePutCodeNextActionNamesPatchScene(t *testing.T) {
	s, _, _ := newFreeStudioRig(t)
	s.SetPhase(PhaseFill, "s1")
	read := codeCall(t, s, "get_code", "s1", nil, "")
	write := codeCall(t, s, "put_code", "s1", map[string]any{"source": freeMotionExample}, read["rev"].(string))
	if write["status"] != "ok" {
		t.Fatalf("put_code = %v", write)
	}
	if want := `Activate it with patch_scene and data {"kind":"code"}. Keep the narration.`; write["next_action"] != want {
		t.Errorf("free next_action = %q, want %q", write["next_action"], want)
	}
	if next, _ := write["next_action"].(string); strings.Contains(next, "put_scene") || strings.Contains(next, "actors") {
		t.Errorf("free next_action still names the catalog edit: %q", next)
	}
	// Once the scene is a code scene, the usual follow-up applies.
	if got := codeCall(t, s, "patch_scene", "s1", map[string]any{"kind": "code"}, ""); got["status"] != "ok" {
		t.Fatalf("patch_scene = %v", got)
	}
	again := codeCall(t, s, "put_code", "s1", map[string]any{"source": strings.Replace(freeMotionExample, "api.duration(8)", "api.duration(9)", 1)}, write["rev"].(string))
	if again["status"] != "ok" || again["next_action"] != "The source is bundled. Run storymaker_check after editing." {
		t.Errorf("put_code on a code scene = %v", again)
	}
}

func TestSceneDataExampleFollowsTheMode(t *testing.T) {
	free, _, _ := newFreeStudioRig(t)
	if got := free.sceneDataExample(); got != `{"kind":"code"}` {
		t.Errorf("free sceneDataExample = %q", got)
	}
	catalog, _, _ := newStudioWriteRig(t, nil)
	if got := catalog.sceneDataExample(); got != `{"environment":"env.office"}` {
		t.Errorf("catalog sceneDataExample = %q", got)
	}

	ctx := context.Background()
	for name, tc := range map[string]struct {
		studio *Studio
		want   string
	}{
		"free":    {free, `Send data as an object, for example {"kind":"code"}.`},
		"catalog": {catalog, `Send data as an object, for example {"environment":"env.office"}.`},
	} {
		tc.studio.SetPhase(PhaseFill, "s1")
		for _, op := range []string{"put_scene", "dry_run"} {
			res := tc.studio.Call(ctx, StudioCall{Tool: ToolStory, Operation: op, Args: map[string]any{"scene_id": "s1", "data": 5}})
			got := answerOf(t, res)
			if got["status"] != "refused" || got["code"] != "invalid_data" {
				t.Errorf("%s %s: %v", name, op, got)
				continue
			}
			if got["message"] != "data must be an object with the scene fields." || got["next_action"] != tc.want {
				t.Errorf("%s %s: message %q, next_action %q", name, op, got["message"], got["next_action"])
			}
		}
	}
}
