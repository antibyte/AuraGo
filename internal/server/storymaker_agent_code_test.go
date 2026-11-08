package server

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
)

func TestStorymakerAgentScriptedCodeCreateAndRevise(t *testing.T) {
	source, err := os.ReadFile("../storymaker/reference/effects.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []struct {
		model string
		text  bool
	}{{smScriptNativeModel, false}, {smScriptTextModel, true}} {
		t.Run(mode.model, func(t *testing.T) {
			turns := smScriptHappyTurns()
			put := smScriptStoryCall("put_code", "s2", map[string]any{"source": string(source)})
			put.args["expected_rev"] = storymaker.MustNotExist
			turns[storymaker.PhaseFill] = []smScriptTurn{
				{tool: storymaker.ToolCatalog, args: map[string]any{"operation": "reference", "topic": "animation"}},
				smScriptStoryCall("put_scene", "s1", smScriptSceneOne()),
				{tool: storymaker.ToolCatalog, args: map[string]any{"operation": "reference", "topic": "code"}},
				smScriptStoryCall("get_code", "s2", nil), put,
				smScriptStoryCall("put_scene", "s2", map[string]any{"kind": "code"}),
			}
			client := &smScriptClient{textMode: mode.text, turns: turns}
			env := newSMScriptEnv(t, mode.model, client)
			job := smScriptWait(t, env.rt.agent, smScriptEnqueue(t, env.rt.agent, env).ID)
			smScriptRequireReady(t, job)
			st := smScriptStory(t, env)
			if st.Scenes[1].Kind != "code" || st.Scenes[1].Narration != smScriptSecondLine {
				t.Fatal("code activation lost narration", st.Scenes[1])
			}
			stored, etag, err := env.rt.projects.LoadCodeSource(context.Background(), env.id, "s2")
			if err != nil || string(stored) != string(source) {
				t.Fatal("source did not reach the project", err)
			}
			if _, err := env.rt.projects.LoadCodeBundle(context.Background(), env.id, "s2"); err != nil {
				t.Fatal(err)
			}
			for _, delivered := range []string{"Storyboard animation reference", "Restricted code-scene reference", "protected-packets", "max_bytes"} {
				if !client.saw(delivered) {
					t.Errorf("model never received %q", delivered)
				}
			}
			if client.saw("tool_scope_denied") || client.saw("operation_not_in_phase") {
				t.Fatal("code authoring was blocked by phase/scope")
			}
			// Change only source, leaving the story document untouched.
			revision := smScriptStoryCall("put_code", "s2", map[string]any{"source": strings.Replace(string(source), "Geschützt verbunden", "Daten geschützt", 1)})
			revision.args["expected_rev"] = strings.Trim(etag, `"`)[:12]
			client.mu.Lock()
			client.turns[storymaker.PhaseRevise] = []smScriptTurn{
				{tool: storymaker.ToolCatalog, args: map[string]any{"operation": "reference", "topic": "code"}},
				smScriptStoryCall("get_code", "s2", nil), revision,
			}
			client.mu.Unlock()
			queued, _, err := env.rt.agent.Enqueue(render.EnqueueRequest{ProjectID: env.id, Kind: render.KindAgent,
				Payload: storymakerAgentPayload{Kind: storymaker.AgentKindRevise, ProjectID: env.id, Instruction: "Ändere die Beschriftung in der zweiten Szene."}})
			if err != nil {
				t.Fatal(err)
			}
			revised := smScriptWait(t, env.rt.agent, queued.ID)
			smScriptRequireReady(t, revised)
			if !slices.Equal(revised.Agent.ChangedScenes, []string{"s2"}) || !slices.Equal(revised.Agent.UnchangedScenes, []string{"s1"}) {
				t.Fatal("code-only revision was not reported", revised.Agent)
			}
		})
	}
}
