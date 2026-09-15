package gamemaker

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"sync/atomic"
	"testing"
	"time"
)

func testVisualCapture(t *testing.T) VisualCapture {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	img.Set(1, 1, color.RGBA{R: 255, A: 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return VisualCapture{Image: "data:image/png;base64," + base64.StdEncoding.EncodeToString(b.Bytes()), Width: 10, Height: 10, Scenario: "start", At: time.Now().UTC().Format(time.RFC3339)}
}

func TestVisualEventsReachStudio(t *testing.T) {
	s := newTestService(t)
	p := createTestProject(t, s, "2d")
	for _, kind := range []string{"visual_progress", "visual_result"} {
		if err := s.EmitAgentEvent(context.Background(), p.ID, "", kind, map[string]any{"status": "reviewed"}); err != nil {
			t.Fatalf("Studio event %s rejected: %v", kind, err)
		}
	}
}

func TestVisualRepairRevalidatesNewBuildExactlyOnce(t *testing.T) {
	s := newTestService(t)
	s.opts.JobTimeout = 3 * time.Minute
	p := createTestProject(t, s, "2d")
	var reviews, repairs atomic.Int32
	var firstBuild string
	s.SetRunner(visualRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "visual" {
			if len(run.Captures) != 1 {
				return errors.New("missing build-bound screenshot")
			}
			n := reviews.Add(1)
			if n == 1 {
				firstBuild = run.Result.Visual.BuildID
				run.Result.Visual.Findings = []VisualFinding{{Image: 0, Severity: "defect", Confidence: .99, Observation: "Duplicate sprite", Suggestion: "Remove the extra sprite"}}
			} else if firstBuild == run.Result.Visual.BuildID {
				return errors.New("old build reviewed again")
			}
			run.Result.Visual.Status = "reviewed"
			return nil
		}
		if run.Stage == "repair" {
			repairs.Add(1)
			if len(run.Diagnostics) != 1 || len(run.Captures) != 1 {
				return errors.New("visual repair lost context")
			}
			source, err := s.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
			if err != nil {
				return err
			}
			return s.WriteJobFile(ctx, run.Job.ID, "src/main.ts", source+"\n// visual repair fixture\n")
		}
		return (testRunner{service: s}).RunGameMakerJob(ctx, run)
	}))
	job, err := s.StartJob(context.Background(), p.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	last := ""
	capture := testVisualCapture(t)
	for time.Now().Before(deadline) {
		s.mu.RLock()
		id := ""
		if s.previewCheck != nil {
			id = s.previewCheck.ID
		}
		s.mu.RUnlock()
		if id != "" && id != last {
			grant, e := s.CreatePreviewGrant(p.ID)
			if e == nil {
				_ = s.ReportPreview(p.ID, PreviewReport{Token: grant.Token, Type: "ready", CanvasVisible: true})
				_ = s.ReportPreview(p.ID, PreviewReport{Token: grant.Token, Type: "gameplay", Captures: []VisualCapture{capture}, Observations: successfulObservationFixture(grant.Scenarios)})
				last = id
			}
		}
		current, e := s.GetJob(context.Background(), job.ID)
		if e != nil {
			t.Fatal(e)
		}
		if !activeJobStatus(current.Status) {
			if current.Status != "ready" || reviews.Load() != 2 || repairs.Load() != 1 {
				t.Fatalf("job=%+v reviews=%d repairs=%d", current, reviews.Load(), repairs.Load())
			}
			s.mu.RLock()
			cached := s.lastValidation[job.ID]
			retained := cached != nil && (len(cached.Captures) > 0 || len(cached.Images) > 0)
			s.mu.RUnlock()
			if retained {
				t.Fatal("screenshots retained after completion")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("visual repair fixture timed out")
}

func TestVisualCaptureIsOptionalAndBuildBound(t *testing.T) {
	s := newTestService(t)
	check := &previewCheck{ID: "build", JobID: "job", BoundToken: "current", ReadyAt: time.Now()}
	s.previewCheck = check
	s.activeJobID = "job"
	s.tokens["current"] = previewToken{ProjectID: "project", JobID: "job", ValidationID: "build", ExpiresAt: time.Now().Add(time.Minute)}
	s.tokens["old"] = previewToken{ProjectID: "project", JobID: "job", ValidationID: "old", ExpiresAt: time.Now().Add(time.Minute)}
	capture := testVisualCapture(t)
	if err := s.ReportPreview("project", PreviewReport{Token: "old", Type: "capture", Captures: []VisualCapture{capture}}); err != nil {
		t.Fatal(err)
	}
	if check.CaptureReceived {
		t.Fatal("stale screenshot accepted")
	}
	if err := s.ReportPreview("project", PreviewReport{Token: "current", Type: "capture", Captures: []VisualCapture{capture}}); err != nil {
		t.Fatal(err)
	}
	if len(check.Captures) != 1 || check.GameplayReceived {
		t.Fatal("independent capture must not certify gameplay")
	}
	check.CaptureReceived = false
	capture.Width = 99
	if err := s.ReportPreview("project", PreviewReport{Token: "current", Type: "capture", Captures: []VisualCapture{capture}}); err != nil {
		t.Fatal(err)
	}
	if !check.CaptureReceived || len(check.Captures) != 0 || len(check.Diagnostics) != 0 {
		t.Fatal("invalid optional image affected technical verdict")
	}
}

type visualRunner func(context.Context, JobRun) error

func (f visualRunner) RunGameMakerJob(ctx context.Context, run JobRun) error { return f(ctx, run) }

func TestVisualRepairRequiresConcreteDefectAndBudget(t *testing.T) {
	s := newTestService(t)
	for _, tc := range []struct {
		name, severity string
		confidence     float64
		budget         bool
		repair         bool
	}{{"taste", "suggestion", .99, true, false}, {"uncertain", "defect", .5, true, false}, {"exhausted", "defect", .99, false, false}, {"defect", "defect", .99, true, true}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if !tc.budget {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, time.Second)
				defer cancel()
			}
			repairs := 0
			reviews := 0
			runner := visualRunner(func(ctx context.Context, run JobRun) error {
				if run.Stage == "visual" {
					reviews++
					run.Result.Visual = VisualReview{Status: "reviewed", Findings: []VisualFinding{{Severity: tc.severity, Confidence: tc.confidence, Observation: "Two sprites"}}}
					return nil
				}
				repairs++
				if len(run.Diagnostics) != 1 || run.Diagnostics[0].Level != "visual" {
					t.Error("missing bounded repair context")
				}
				return errors.New("stop before mutating a fixture")
			})
			s.reviewAndRepairVisual(ctx, runner, JobRun{}, BuildResult{OK: true}, "startup")
			if reviews != 1 || (repairs == 1) != tc.repair || repairs > 1 {
				t.Fatalf("reviews=%d repairs=%d", reviews, repairs)
			}
		})
	}
}

func TestManualVisualReviewRejectsStaleRevisionAndCancellation(t *testing.T) {
	s := newTestService(t)
	p := createTestProject(t, s, "2d")
	s.tokens["token"] = previewToken{ProjectID: p.ID, Revision: p.CurrentRevision, ExpiresAt: time.Now().Add(time.Minute)}
	report := PreviewReport{Token: "token", Captures: []VisualCapture{testVisualCapture(t)}}
	s.runner = visualRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage != "visual" || run.Job.ID != "" {
			t.Fatal("manual analysis received a writing job")
		}
		run.Result.Visual = VisualReview{Status: "reviewed"}
		return nil
	})
	if result, err := s.ReviewPreview(context.Background(), p.ID, report, "", ""); err != nil || result.Status != "reviewed" {
		t.Fatalf("manual review: %+v %v", result, err)
	}
	grant := s.tokens["token"]
	grant.Revision++
	s.tokens["token"] = grant
	if _, err := s.ReviewPreview(context.Background(), p.ID, report, "", ""); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("stale revision accepted: %v", err)
	}
	grant.Revision = p.CurrentRevision
	s.tokens["token"] = grant
	s.runner = visualRunner(func(ctx context.Context, run JobRun) error {
		s.mu.Lock()
		delete(s.tokens, "token")
		s.mu.Unlock()
		return nil
	})
	if _, err := s.ReviewPreview(context.Background(), p.ID, report, "", ""); !errors.Is(err, ErrInvalidToken) {
		t.Fatal("late response from expired preview accepted")
	}
}
