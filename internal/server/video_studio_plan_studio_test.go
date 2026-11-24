package server

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/videostudio/storyplan"
)

const (
	vspIdea      = "Ein Fischer faehrt bei Sonnenaufgang hinaus."
	vspPromptOne = "Slow dolly in. An old fisherman unties a wooden boat. Small harbour at dawn, mist on the water. Soft golden backlight."
	vspPromptTwo = "Tracking shot from behind. The boat glides out of the harbour. Calm bay at sunrise. Warm low sun."
)

// vspFixture is a Video Studio server whose video model is MiniMax Hailuo-02 (plan mode supported, 6 and 10 second
// clips), with one project holding a fresh 12 second plan. It returns the server, a write token and the project id.
func vspFixture(t *testing.T) (*Server, string, string) {
	t.Helper()
	s, _, writeToken := videoStudioAPIFixture(t)
	s.Cfg.VideoGeneration.Enabled = true
	s.Cfg.VideoGeneration.APIKey = "fixture-key"
	s.Cfg.VideoGeneration.ProviderType = "minimax"
	s.Cfg.VideoGeneration.Model = "hailuo-02"
	s.Cfg.VideoStudio.Plan.Enabled = true
	project, _ := videoStudioAPICreate(t, s, writeToken)
	vspSeed(t, s, project.ID, vspNewPlan(t, s, 12))
	return s, writeToken, project.ID
}

func vspServices(t *testing.T, s *Server) (*desktop.Service, *videoStudioManager, *config.Config) {
	t.Helper()
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	manager, err := s.videoStudioManager()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := normalizedVideoStudioConfig(s)
	if err != nil {
		t.Fatal(err)
	}
	return svc, manager, cfg
}

// vspNewPlan is a fresh plan of target seconds for the configured profile.
func vspNewPlan(t *testing.T, s *Server, target int) storyplan.Plan {
	t.Helper()
	_, _, cfg := vspServices(t, s)
	profile := videoStudioGenerationProfile(cfg, "")
	if !profile.PlanSupported {
		t.Fatalf("the fixture profile does not support plan mode: %+v", profile)
	}
	return storyplan.NewPlan("plan_fixture", vspIdea, target, "de", storyplan.ProfileRef{Provider: profile.Provider, Model: profile.Model, Resolution: profile.Resolution})
}

// vspScene is a complete 6 second scene that starts with a cut and has no keyframe.
func vspScene(id, title, prompt string) storyplan.Scene {
	return storyplan.Scene{
		ID: id, Title: title, VideoPrompt: prompt, GenerateSeconds: 6, UseSeconds: 6,
		TransitionIn: storyplan.Transition{Kind: storyplan.TransitionCut},
		Keyframe:     storyplan.Keyframe{Source: storyplan.KeyframeNone},
	}
}

// vspSeed writes plan as the project's plan.json, replacing what is there.
func vspSeed(t *testing.T, s *Server, projectID string, plan storyplan.Plan) {
	t.Helper()
	svc, manager, cfg := vspServices(t, s)
	lock := manager.projectLock(projectID)
	lock.Lock()
	defer lock.Unlock()
	if _, err := writeVideoStudioPlan(context.Background(), svc, manager, cfg, projectID, plan, "*"); err != nil {
		t.Fatal(err)
	}
}

func vspReadPlan(t *testing.T, s *Server, projectID string) storyplan.Plan {
	t.Helper()
	svc, _, _ := vspServices(t, s)
	plan, _, _, err := readVideoStudioPlan(context.Background(), svc, projectID)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func vspStudio(t *testing.T, s *Server, projectID, mode, sceneID string) *videoStudioPlanStudio {
	t.Helper()
	svc, manager, cfg := vspServices(t, s)
	st, err := newVideoStudioPlanStudio(context.Background(), s, svc, manager, cfg, projectID, mode, sceneID)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func vspCall(t *testing.T, st *videoStudioPlanStudio, tool string, args map[string]any) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal(st.Call(context.Background(), agent.ToolCall{Action: tool, Params: args}), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func vspIssueCodes(result map[string]any) []string {
	var codes []string
	list, _ := result["issues"].([]any)
	for _, raw := range list {
		if issue, ok := raw.(map[string]any); ok {
			code, _ := issue["code"].(string)
			codes = append(codes, code)
		}
	}
	return codes
}

func TestVideoStudioPlanStudioWritesPatchesAndFinishes(t *testing.T) {
	s, _, projectID := vspFixture(t)
	st := vspStudio(t, s, projectID, agent.VideoStudioPlanModeCreate, "")

	st.SetPhase(agent.VideoStudioPlanPhaseOutline)
	read := vspCall(t, st, agent.VideoStudioPlanToolRead, nil)
	if plan, _ := read["plan"].(map[string]any); read["status"] != "ok" || plan["idea"] != vspIdea || read["availability"] == nil || read["profile"] == nil {
		t.Fatalf("read = %v", read)
	}
	if got := vspCall(t, st, agent.VideoStudioPlanToolPatchScene, map[string]any{"scene_id": "s1", "scene": map[string]any{"title": "x"}}); got["code"] != "tool_not_in_phase" {
		t.Fatalf("patch in the outline phase = %v", got)
	}
	// No plan wrapper: the flat scenes are taken. The idea and the server fields in the call are not.
	outline := vspCall(t, st, agent.VideoStudioPlanToolWrite, map[string]any{
		"idea": "Something else",
		"scenes": []any{
			map[string]any{"id": "s1", "title": "Hafen im Nebel", "generate_seconds": 6, "use_seconds": 6, "transition_in": map[string]any{"kind": "cut"}, "state": "ready", "clip_asset_id": "forged"},
			map[string]any{"id": "s2", "title": "Ausfahrt", "generate_seconds": 6, "use_seconds": 6.5, "transition_in": map[string]any{"kind": "cut"}},
		},
	})
	if outline["status"] != "ok" || outline["phase_complete"] != true || !st.PhaseDone() {
		t.Fatalf("outline write = %v, phase done = %v", outline, st.PhaseDone())
	}
	saved := vspReadPlan(t, s, projectID)
	if saved.Idea != vspIdea || len(saved.Scenes) != 2 || saved.Scenes[0].ClipAssetID != "" || saved.Scenes[0].State != storyplan.SceneRuntimePending {
		t.Fatalf("saved outline = %+v", saved)
	}

	st.SetPhase(agent.VideoStudioPlanPhaseDetails)
	if st.PhaseDone() {
		t.Fatal("the details phase is done before any prompt")
	}
	// A plan sent as JSON text is read like an object; the parts it leaves out stay.
	if got := vspCall(t, st, agent.VideoStudioPlanToolWrite, map[string]any{"plan": `{"style":"35 mm film look, muted teal and amber."}`}); got["status"] != "ok" {
		t.Fatalf("style write = %v", got)
	}
	if got := vspCall(t, st, agent.VideoStudioPlanToolPatchScene, map[string]any{"scene_id": "s1", "scene": map[string]any{"video_prompt": vspPromptOne, "keyframe": map[string]any{"source": "none"}}}); got["status"] != "ok" || got["phase_complete"] != false {
		t.Fatalf("first patch = %v", got)
	}
	if got := vspCall(t, st, agent.VideoStudioPlanToolPatchScene, map[string]any{"scene_id": "s2", "scene": map[string]any{"video_prompt": vspPromptTwo}}); got["phase_complete"] != true || !st.PhaseDone() {
		t.Fatalf("second patch = %v", got)
	}
	saved = vspReadPlan(t, s, projectID)
	if saved.Style != "35 mm film look, muted teal and amber." || len(saved.Scenes) != 2 || saved.Scenes[0].Title != "Hafen im Nebel" || saved.Scenes[0].VideoPrompt != vspPromptOne || saved.Scenes[1].VideoPrompt != vspPromptTwo {
		t.Fatalf("saved details = %+v", saved)
	}
	if got := vspCall(t, st, agent.VideoStudioPlanToolPatchScene, map[string]any{"scene_id": "s9", "scene": map[string]any{"title": "x"}}); got["code"] != "scene_not_found" {
		t.Fatalf("patch of an unknown scene = %v", got)
	}

	st.SetPhase(agent.VideoStudioPlanPhaseCheck)
	refused := vspCall(t, st, agent.VideoStudioPlanToolFinish, nil)
	if refused["code"] != "blocking_issues" || !slices.Contains(vspIssueCodes(refused), "use_seconds_invalid") || st.PhaseDone() {
		t.Fatalf("finish with blocking issues = %v", refused)
	}
	if got := vspCall(t, st, agent.VideoStudioPlanToolPatchScene, map[string]any{"scene_id": "s2", "scene": map[string]any{"use_seconds": 6}}); got["status"] != "ok" {
		t.Fatalf("fix = %v", got)
	}
	if got := vspCall(t, st, agent.VideoStudioPlanToolCheck, nil); got["status"] != "ok" || got["blocking"] != false {
		t.Fatalf("check = %v", got)
	}
	if got := vspCall(t, st, agent.VideoStudioPlanToolFinish, nil); got["status"] != "ok" || !st.PhaseDone() {
		t.Fatalf("finish = %v", got)
	}
	if st.Writes() != 5 {
		t.Fatalf("writes = %d, want 5", st.Writes())
	}
}

func TestVideoStudioPlanStudioReviseScopeAndConflict(t *testing.T) {
	s, _, projectID := vspFixture(t)
	plan := vspNewPlan(t, s, 18)
	plan.Status = storyplan.StatusReview
	plan.Scenes = []storyplan.Scene{vspScene("s1", "Hafen im Nebel", vspPromptOne), vspScene("s2", "Ausfahrt", vspPromptTwo), vspScene("s3", "Die Bucht", vspPromptTwo)}
	vspSeed(t, s, projectID, plan)
	st := vspStudio(t, s, projectID, agent.VideoStudioPlanModeRevise, "s2")
	st.SetPhase(agent.VideoStudioPlanPhaseRevise)

	for _, refused := range []map[string]any{
		{"scene_id": "s1", "scene": map[string]any{"title": "Anders"}},
		{"scene_id": "s3", "scene": map[string]any{"title": "Anders", "transition_in": map[string]any{"kind": "continue"}}},
	} {
		if got := vspCall(t, st, agent.VideoStudioPlanToolPatchScene, refused); got["code"] != "scene_not_allowed" {
			t.Fatalf("patch %v = %v", refused, got)
		}
	}
	if got := vspCall(t, st, agent.VideoStudioPlanToolPatchScene, map[string]any{"scene_id": "s3", "scene": map[string]any{"transition_in": map[string]any{"kind": "continue"}}}); got["status"] != "ok" {
		t.Fatalf("transition of the next scene = %v", got)
	}
	night := "Tracking shot from behind. The boat glides out of the harbour under a starry sky. Moonlight on calm water."
	if got := vspCall(t, st, agent.VideoStudioPlanToolPatchScene, map[string]any{"scene_id": "s2", "scene": map[string]any{"video_prompt": night}}); got["status"] != "ok" {
		t.Fatalf("patch of the job's scene = %v", got)
	}
	saved := vspReadPlan(t, s, projectID)
	if saved.Scenes[0].Title != "Hafen im Nebel" || saved.Scenes[1].VideoPrompt != night || saved.Scenes[2].TransitionIn.Kind != storyplan.TransitionContinue || saved.Scenes[2].Title != "Die Bucht" {
		t.Fatalf("saved revision = %+v", saved.Scenes)
	}

	// A write outside the job: the studio's next write finds a newer plan and stops the phase.
	saved.Style = "changed outside the job"
	vspSeed(t, s, projectID, saved)
	if got := vspCall(t, st, agent.VideoStudioPlanToolPatchScene, map[string]any{"scene_id": "s2", "scene": map[string]any{"title": "Nacht"}}); got["code"] != "plan_conflict" || !st.Conflict() || !st.PhaseDone() {
		t.Fatalf("write over a newer plan = %v, conflict = %v", got, st.Conflict())
	}
	if after := vspReadPlan(t, s, projectID); after.Scenes[1].Title != "Ausfahrt" || after.Style != "changed outside the job" {
		t.Fatalf("the conflicting write was saved: %+v", after)
	}
}
