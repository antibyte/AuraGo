package server

import (
	"context"
	"testing"

	"aurago/internal/storymaker"
)

func smScriptFreeTurns() map[string][]smScriptTurn {
	put := func(scene, title string) smScriptTurn {
		turn := smScriptStoryCall("put_code", scene, map[string]any{"source": "export default function scene(api) { api.duration(5); } // " + title})
		turn.args["expected_rev"] = storymaker.MustNotExist
		return turn
	}
	return map[string][]smScriptTurn{
		storymaker.PhaseOutline: {smScriptStoryCall("outline", "", smScriptOutlineData())},
		storymaker.PhaseFill: {
			smScriptStoryCall("get_code", "s1", nil), put("s1", "one"), smScriptStoryCall("put_scene", "s1", map[string]any{"kind": "code"}),
			smScriptStoryCall("get_code", "s2", nil), put("s2", "two"), smScriptStoryCall("put_scene", "s2", map[string]any{"kind": "code"}),
		},
	}
}

func TestStorymakerAgentScriptedFreeCreate(t *testing.T) {
	for _, mode := range []struct {
		model string
		text  bool
	}{{smScriptNativeModel, false}, {smScriptTextModel, true}} {
		t.Run(mode.model, func(t *testing.T) {
			client := &smScriptClient{textMode: mode.text, turns: smScriptFreeTurns()}
			env := newSMScriptEnvMode(t, mode.model, client, storymaker.ModeFree)
			job := smScriptWait(t, env.rt.agent, smScriptEnqueue(t, env.rt.agent, env).ID)
			smScriptRequireReady(t, job)
			st := smScriptStory(t, env)
			if len(st.Scenes) != 2 || len(st.Cast) != 0 {
				t.Fatalf("story = %d scenes, %d cast, want 2 and 0", len(st.Scenes), len(st.Cast))
			}
			for i, line := range []string{smScriptFirstLine, smScriptSecondLine} {
				if st.Scenes[i].Kind != "code" || st.Scenes[i].Narration != line {
					t.Fatalf("scene %d = %+v", i, st.Scenes[i])
				}
				if _, err := env.rt.projects.LoadCodeBundle(context.Background(), env.id, st.Scenes[i].ID); err != nil {
					t.Fatalf("scene %d has no bundle: %v", i, err)
				}
			}
			for i, req := range client.seen() {
				if smScriptPhase(req) == storymaker.PhaseCast {
					t.Fatalf("request %d ran the cast phase of a free project", i)
				}
			}
			if client.saw("storymaker_catalog") && client.saw(`"operation":"search"`) {
				t.Fatal("a free project searched the catalog")
			}
			project, err := env.rt.projects.Get(context.Background(), env.id)
			if err != nil || project.Mode != storymaker.ModeFree {
				t.Fatalf("project = %+v, %v", project, err)
			}
		})
	}
}
