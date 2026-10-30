package server

import (
	"context"
	"encoding/base64"
	"testing"

	"aurago/internal/config"
	"aurago/internal/decisions"
	"aurago/internal/gamemaker"
)

func gameVisualPrecheckResponse(canvas string, hud float64) decisions.Response {
	response := imageChoiceResponse("canvas", gameVisualCanvasOptions, canvas, 0.9)
	response.Answers["hud"] = decisions.Answer{Type: "noul", Noul: &hud, Probabilities: map[string]float64{"yes": hud, "no": 1 - hud}}
	return response
}

func gameVisualPrecheckRun(t *testing.T, response decisions.Response, supported, enabled bool) (gamemaker.BuildResult, *imageScriptedDecisions) {
	t.Helper()
	s, fake := hubLocal(t)
	if !supported {
		fake.status.ProjectorSHA256 = ""
	}
	scripted := &imageScriptedDecisions{fakeDecisions: fake, response: response}
	s.Decisions = scripted
	s.Cfg.DecisionConsumers.GameMakerVisual = config.DecisionConsumerSwitch{Enabled: enabled}
	capture := gamemaker.VisualCapture{Image: "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(imageTestJPEG(t)), Scenario: "start", Width: 64, Height: 48}
	runner := &gameMakerAgentRunner{server: s}
	result := gamemaker.BuildResult{Visual: gamemaker.VisualReview{BuildID: "current-build"}}
	// The default configuration has no vision route, so the review is skipped.
	if err := runner.reviewGameImages(context.Background(), s.Cfg, nil, gamemaker.JobRun{Result: &result, Captures: []gamemaker.VisualCapture{capture}}); err != nil {
		t.Fatal(err)
	}
	return result, scripted
}

func TestGameVisualPrecheckWhenNoVisionRoute(t *testing.T) {
	result, d := gameVisualPrecheckRun(t, gameVisualPrecheckResponse("blank", 0.1), true, true)
	pre := result.Visual.Precheck
	if result.Visual.Status != "skipped" || result.Visual.Reason != "no_vision_route" || pre == nil || pre.Canvas != "blank" || pre.HUDVisible == nil || *pre.HUDVisible {
		t.Fatalf("visual = %+v precheck = %+v", result.Visual, pre)
	}
	if d.calls != 1 || len(d.last.Images) != 1 || result.VisualStatus != "skipped" {
		t.Fatalf("calls=%d images=%d status=%q", d.calls, len(d.last.Images), result.VisualStatus)
	}
}

func TestGameVisualPrecheckNeverRepairs(t *testing.T) {
	result, _ := gameVisualPrecheckRun(t, gameVisualPrecheckResponse("error_text", 0.9), true, true)
	if result.Visual.Precheck == nil || result.Visual.Precheck.Canvas != "error_text" {
		t.Fatalf("precheck = %+v", result.Visual.Precheck)
	}
	if len(result.Visual.Findings) != 0 || len(result.Visual.RepairDiagnostics()) != 0 {
		t.Fatalf("precheck leaked into findings: %+v", result.Visual)
	}
}

func TestGameVisualPrecheckOffUnchanged(t *testing.T) {
	result, d := gameVisualPrecheckRun(t, gameVisualPrecheckResponse("blank", 0.1), true, false)
	if result.Visual.Precheck != nil || d.calls != 0 || result.Visual.Status != "skipped" || result.Visual.Reason != "no_vision_route" {
		t.Fatalf("visual = %+v calls=%d", result.Visual, d.calls)
	}
}

func TestGameVisualPrecheckImagesUnsupportedUnchanged(t *testing.T) {
	result, d := gameVisualPrecheckRun(t, gameVisualPrecheckResponse("blank", 0.1), false, true)
	if result.Visual.Precheck != nil || d.calls != 0 || result.Visual.Status != "skipped" {
		t.Fatalf("visual = %+v calls=%d", result.Visual, d.calls)
	}
}
