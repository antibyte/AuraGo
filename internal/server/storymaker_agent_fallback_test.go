package server

import (
	"context"
	"testing"
	"time"

	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
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

// A job the user stopped (its run context is cancelled) must not get fallback boards, even when it reaches finish.
// A job that only ran into its own deadline still does (TestStorymakerAgentFallbackBoardAtJobEnd).
func TestStorymakerAgentFallbackSkippedAfterUserStop(t *testing.T) {
	turns := smScriptHappyTurns()
	turns[storymaker.PhaseFill] = []smScriptTurn{smScriptStoryCall("put_scene", "s1", smScriptSceneOne())}
	client := &smScriptClient{turns: turns}
	env := newSMScriptEnv(t, smScriptNativeModel, client)
	m, err := render.NewManager(render.ManagerOptions{
		Dir:     t.TempDir(),
		Kinds:   []string{render.KindAgent},
		Workers: 1,
		Run: func(ctx context.Context, job render.Job, payload any, h *render.Handle) (render.Output, error) {
			j, err := env.s.newStorymakerAgentJob(ctx, env.rt, job, payload.(storymakerAgentPayload), h)
			if err != nil {
				return render.Output{}, err
			}
			defer j.mem.Close()
			jobCtx, cancelJob := context.WithTimeout(ctx, time.Minute)
			defer cancelJob()
			j.runCreate(jobCtx)
			// The user stops the job after its loops, before the closing steps.
			runCtx, stop := context.WithCancel(ctx)
			stop()
			return j.finish(runCtx, jobCtx)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	smScriptWait(t, m, smScriptEnqueue(t, m, env).ID)
	s2 := smScriptStory(t, env).Scenes[1]
	if s2.Environment == storymaker.BoardEnvironmentID || len(s2.Beats) != 0 {
		t.Fatalf("s2 = %+v, a stopped job must leave the unfilled scene untouched", s2)
	}
}

// A cast phase that never ends keeps the job from the fill passes, so outlineScenes stays empty. The scenes of the saved
// outline still end as boards.
func TestStorymakerAgentFallbackBoardWhenTheCastPhaseFails(t *testing.T) {
	turns := smScriptHappyTurns()
	delete(turns, storymaker.PhaseCast)
	delete(turns, storymaker.PhaseFill)
	client := &smScriptClient{turns: turns}
	env := newSMScriptEnv(t, smScriptNativeModel, client)
	job := smScriptWait(t, env.rt.agent, smScriptEnqueue(t, env.rt.agent, env).ID)
	if job.Agent == nil || job.Agent.Outcome != storymaker.OutcomeIncomplete {
		t.Fatalf("job = %+v, agent = %+v, want an incomplete outcome", job, job.Agent)
	}
	fallback := map[string]bool{}
	for _, c := range job.Agent.OpenChecks {
		if c.Code == storymaker.CodeSceneUnfilled {
			t.Errorf("a scene stayed unfilled: %+v", job.Agent.OpenChecks)
		}
		if c.Code == storymaker.CodeSceneFallback {
			fallback[c.SceneID] = true
		}
	}
	if !fallback["s1"] || !fallback["s2"] {
		t.Fatalf("open checks = %+v, want scene_fallback for s1 and s2", job.Agent.OpenChecks)
	}
	for _, sc := range smScriptStory(t, env).Scenes {
		if sc.Environment != storymaker.BoardEnvironmentID || len(sc.Beats) != 1 || sc.Beats[0].Style != "headline" {
			t.Errorf("scene %s = %+v, want a headline on env.board", sc.ID, sc)
		}
	}
}
