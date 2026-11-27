package server

import (
	"testing"

	"aurago/internal/storymaker"
)

// A scene the model never fills ends as a board with its key sentence, and the job lists it as an open check.
func TestStorymakerAgentFallbackBoardAtJobEnd(t *testing.T) {
	turns := smScriptHappyTurns()
	turns[storymaker.PhaseFill] = []smScriptTurn{smScriptStoryCall("put_scene", "s1", smScriptSceneOne())}
	client := &smScriptClient{turns: turns}
	env := newSMScriptEnv(t, smScriptNativeModel, client)
	job := smScriptWait(t, env.rt.agent, smScriptEnqueue(t, env.rt.agent, env).ID)
	if job.Agent == nil || job.Agent.Outcome != storymaker.OutcomeIncomplete {
		t.Fatalf("job = %+v, agent = %+v, want an incomplete outcome", job, job.Agent)
	}
	var fallback bool
	for _, c := range job.Agent.OpenChecks {
		switch {
		case c.Code == storymaker.CodeSceneUnfilled:
			t.Errorf("the empty scene must not stay unfilled: %+v", job.Agent.OpenChecks)
		case c.Code == storymaker.CodeSceneFallback && c.SceneID == "s2":
			fallback = true
		}
	}
	if !fallback {
		t.Fatalf("open checks = %+v, want scene_fallback for s2", job.Agent.OpenChecks)
	}
	st := smScriptStory(t, env)
	smScriptRequireFilled(t, st, "s1")
	s2 := st.Scenes[1]
	if s2.Environment != storymaker.BoardEnvironmentID || len(s2.Actors) != 0 || len(s2.Beats) != 1 || s2.Beats[0].Style != "headline" || s2.Beats[0].Text == "" {
		t.Fatalf("s2 = %+v, want a headline on env.board", s2)
	}
	if job.Agent.ScenesDone != 2 {
		t.Errorf("scenes done = %d, want 2", job.Agent.ScenesDone)
	}
}
