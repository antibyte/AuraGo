package gamemaker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGameJobDeadlineHasOneBoundedFinishingWindow(t *testing.T) {
	for _, extend := range []bool{false, true} {
		t.Run(fmt.Sprint(extend), func(t *testing.T) {
			var calls atomic.Int32
			checked := make(chan struct{})
			started := time.Now()
			ctx, cancel := newGameJobContext(200*time.Millisecond, func(context.Context) bool {
				calls.Add(1)
				close(checked)
				return extend
			})
			defer cancel()
			deadline, ok := ctx.Deadline()
			if !ok || deadline.Sub(started) < 300*time.Millisecond || deadline.Sub(started) > 350*time.Millisecond {
				t.Fatal("the maximum deadline must be visible to LLM and validation calls")
			}
			<-checked
			if extend && ctx.Err() != nil {
				t.Fatal("productive job was cancelled at its normal deadline")
			}
			select {
			case <-ctx.Done():
			case <-time.After(time.Second):
				t.Fatal("job outlived its hard deadline")
			}
			if !errors.Is(context.Cause(ctx), context.DeadlineExceeded) || calls.Load() != 1 {
				t.Fatalf("cause=%v, extension checks=%d", context.Cause(ctx), calls.Load())
			}
			if extend && time.Since(started) < 300*time.Millisecond {
				t.Fatal("finishing window was not granted")
			}
		})
	}
	for _, budget := range []time.Duration{time.Minute, 30 * time.Minute, time.Hour} {
		if grace := jobTimeoutGrace(budget); grace > 15*time.Minute || grace > budget/2 {
			t.Fatalf("unbounded finishing window: %s", grace)
		}
	}
}

func TestGameJobCancelStopsFinishingWindow(t *testing.T) {
	for _, duringGrace := range []bool{false, true} {
		t.Run(fmt.Sprint(duringGrace), func(t *testing.T) {
			checked := make(chan struct{})
			ctx, cancel := newGameJobContext(100*time.Millisecond, func(context.Context) bool {
				close(checked)
				return true
			})
			if duringGrace {
				<-checked
			}
			cancel()
			<-ctx.Done()
			if !errors.Is(context.Cause(ctx), context.Canceled) {
				t.Fatalf("user cancellation changed into a timeout: %v", context.Cause(ctx))
			}
			if !duringGrace {
				time.Sleep(120 * time.Millisecond)
				select {
				case <-checked:
					t.Fatal("extension check survived cancellation")
				default:
				}
			}
		})
	}
}

func TestGameJobExtensionRequiresRecentCompiledSource(t *testing.T) {
	for _, dimension := range []string{"2d", "3d"} {
		t.Run(dimension, func(t *testing.T) {
			s := newTestService(t)
			p := createTestProject(t, s, dimension)
			s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
				if run.Stage == "planning" {
					base := "platformer"
					if dimension == "3d" {
						base = "flight"
					}
					return s.SetDesignJSON(ctx, run.Job.ID, []byte(fmt.Sprintf(`{"base":%q,"objective":"Custom game","features":["Distinct movement"]}`, base)))
				}
				id := run.Job.ID
				since := time.Now().Add(-time.Minute)
				check := func(want bool, what string) {
					t.Helper()
					if got := s.recentCompiledProgress(ctx, id, dimension, since); got != want {
						t.Errorf("%s: recent progress=%v, want %v", what, got, want)
					}
				}
				original, err := s.ReadJobFile(ctx, id, "src/main.ts")
				if err != nil {
					return err
				}
				if _, err := s.WriteJobFileChecked(ctx, id, "src/main.ts", original, ""); err != nil {
					return err
				}
				check(false, "unchanged starter with successful build")
				if _, err := s.WriteJobFileChecked(ctx, id, "src/main.ts", original+"\nexport const customRule = 5;", ""); err != nil {
					return err
				}
				check(true, "recent successful implementation")
				since = time.Now().Add(time.Second)
				_, _ = s.ReadJobFile(ctx, id, "src/main.ts")
				_, _ = s.emit(ctx, p.ID, id, "tool_call", map[string]any{"tool": "game_maker_file", "attempted": true})
				check(false, "reads and tool activity cannot refresh old source")
				since = time.Now().Add(-time.Minute)
				s.mu.Lock()
				s.acceptedPlans[id] = false
				s.mu.Unlock()
				check(false, "no accepted plan")
				s.mu.Lock()
				s.acceptedPlans[id] = true
				compiled := s.previewCheck
				s.previewCheck = &previewCheck{JobID: "other-job"}
				s.mu.Unlock()
				check(false, "other job build")
				s.mu.Lock()
				s.previewCheck = compiled
				s.mu.Unlock()
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				if s.recentCompiledProgress(cancelled, id, dimension, since) {
					t.Error("cancelled context received an extension")
				}
				broken, err := s.WriteJobFileChecked(ctx, id, "src/main.ts", "export const broken = ;", "")
				if err != nil || broken.Build.OK {
					return fmt.Errorf("invalid fixture build: %v", err)
				}
				check(false, "failed build invalidates previous success")
				return errors.New("progress gate verified")
			}))
			job, err := s.StartJob(context.Background(), p.ID, StartJobRequest{})
			if err != nil {
				t.Fatal(err)
			}
			if done := waitJob(t, s, job.ID); done.Error != "progress gate verified" || done.ResultRevision != 0 {
				t.Fatalf("unexpected job: %+v", done)
			}
		})
	}
}

func TestGameJobTimeoutStillReportsTimeout(t *testing.T) {
	s := newTestService(t)
	s.opts.JobTimeout = 100 * time.Millisecond
	p := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, _ JobRun) error {
		<-ctx.Done()
		return ctx.Err()
	}))
	job, err := s.StartJob(context.Background(), p.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.Status != "cancelled" || !strings.Contains(done.Error, "exceeded its time limit") {
		t.Fatalf("timeout classified as a user cancellation: %+v", done)
	}
}

func TestGameJobProductiveWorkReceivesExtensionDiagnostic(t *testing.T) {
	s := newTestService(t)
	s.opts.JobTimeout = time.Second
	p := createTestProject(t, s, "3d")
	events, unsubscribe := s.Subscribe(p.ID)
	defer unsubscribe()
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "planning" {
			return s.SetDesignJSON(ctx, run.Job.ID, []byte(`{"base":"flight","objective":"Fly through gates","features":["Custom gates"]}`))
		}
		deadline, _ := ctx.Deadline()
		// Make a real change in the final third of the base budget.
		wake := time.NewTimer(time.Until(deadline.Add(-750 * time.Millisecond)))
		defer wake.Stop()
		select {
		case <-wake.C:
		case <-ctx.Done():
			return ctx.Err()
		}
		original, err := s.ReadJobFile(ctx, run.Job.ID, "src/main.ts")
		if err != nil {
			return err
		}
		if _, err := s.WriteJobFileChecked(ctx, run.Job.ID, "src/main.ts", original+"\nexport const customGates = 7;", ""); err != nil {
			return err
		}
		for {
			select {
			case event := <-events:
				if event.Type != "diagnostic" || event.Payload["code"] != "job_time_extended" {
					continue
				}
				if event.Payload["extra_seconds"] != 0.5 || event.Payload["total_seconds"] != 1.5 || ctx.Err() != nil {
					t.Error("extension was not granted or reported with the actual limits")
				}
				return errors.New("extension verified; validation still required")
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}))
	job, err := s.StartJob(context.Background(), p.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.Error != "extension verified; validation still required" || done.ResultRevision != 0 {
		t.Fatalf("productive job got no finishing window: %+v", done)
	}
}
