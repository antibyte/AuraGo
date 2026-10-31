package server

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

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

// blockingDecisions never answers before its context ends.
type blockingDecisions struct {
	*fakeDecisions
	started chan struct{}
}

func (d *blockingDecisions) Evaluate(ctx context.Context, _ decisions.Request) (decisions.Response, error) {
	select {
	case d.started <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return decisions.Response{}, ctx.Err()
}

func TestGameVisualPrecheckStaysInsideTheDeadlineMargin(t *testing.T) {
	run := func(t *testing.T, remaining time.Duration) (time.Duration, gamemaker.BuildResult, bool) {
		s, fake := hubLocal(t)
		blocker := &blockingDecisions{fakeDecisions: fake, started: make(chan struct{}, 1)}
		s.Decisions = blocker
		s.Cfg.DecisionConsumers.GameMakerVisual = config.DecisionConsumerSwitch{Enabled: true}
		capture := gamemaker.VisualCapture{Image: "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(imageTestJPEG(t)), Scenario: "start"}
		ctx, cancel := context.WithTimeout(context.Background(), remaining)
		defer cancel()
		result := gamemaker.BuildResult{}
		start := time.Now()
		if err := (&gameMakerAgentRunner{server: s}).reviewGameImages(ctx, s.Cfg, nil, gamemaker.JobRun{Result: &result, Captures: []gamemaker.VisualCapture{capture}}); err != nil {
			t.Fatal(err)
		}
		elapsed := time.Since(start)
		select {
		case <-blocker.started:
			return elapsed, result, true
		default:
			return elapsed, result, false
		}
	}
	// Under the margin: the precheck never starts and the review is not delayed.
	if elapsed, result, started := run(t, 30*time.Second); started || result.Visual.Precheck != nil || elapsed > time.Second {
		t.Fatalf("short deadline: started=%v elapsed=%v precheck=%+v", started, elapsed, result.Visual.Precheck)
	}
	// Just over the margin: it is bounded by the room left, here about two seconds.
	elapsed, result, _ := run(t, gameVisualPrecheckMargin+2*time.Second)
	if elapsed > 4*time.Second || result.Visual.Precheck != nil || result.Visual.Status != "skipped" || result.Visual.Reason != "no_vision_route" {
		t.Fatalf("elapsed=%v visual=%+v", elapsed, result.Visual)
	}
}
