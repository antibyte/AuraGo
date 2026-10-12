package cydcreator

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"
)

// fakeRunner records every phase it runs and calls the scripted function of
// that phase (nil = return at once).
type fakeRunner struct {
	mu     sync.Mutex
	runs   []JobRun
	phases map[string]func(ctx context.Context, run JobRun) error
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{phases: map[string]func(ctx context.Context, run JobRun) error{}}
}

func (f *fakeRunner) on(phase string, fn func(ctx context.Context, run JobRun) error) {
	f.mu.Lock()
	f.phases[phase] = fn
	f.mu.Unlock()
}

func (f *fakeRunner) RunCYDJob(ctx context.Context, run JobRun) error {
	f.mu.Lock()
	f.runs = append(f.runs, run)
	fn := f.phases[run.Phase]
	f.mu.Unlock()
	if fn == nil {
		return nil
	}
	return fn(ctx, run)
}

func (f *fakeRunner) phaseList() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.runs))
	for _, r := range f.runs {
		out = append(out, r.Phase)
	}
	return out
}

func (f *fakeRunner) at(i int) JobRun {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.runs[i]
}

func (f *fakeRunner) last() JobRun {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.runs[len(f.runs)-1]
}

// scriptSuccess makes every agent phase do typical work: plan, one widget,
// one source file and the finish summary.
func scriptSuccess(t *testing.T, env *testEnv, r *fakeRunner) {
	t.Helper()
	r.on(PhasePlanning, func(ctx context.Context, run JobRun) error {
		_, err := env.svc.SubmitPlan(ctx, run.Job.ID, []byte(testPlanJSON))
		return err
	})
	r.on(PhaseLayout, func(ctx context.Context, run JobRun) error {
		_, err := env.svc.UpdateLayoutForJob(ctx, run.Job.ID, []LayoutOp{{Op: "add_widget", Screen: "home",
			Widget: &Widget{Type: "label", Name: "lbl_temp", X: 10, Y: 10, Props: map[string]any{"text": "22 °C"}}}})
		return err
	})
	r.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		if _, err := env.svc.WriteJobFile(ctx, run.Job.ID, "src/app/app_main.cpp", "#include \"ui.h\"\nvoid app_setup() {}\nvoid app_loop() {}\n// agent\n"); err != nil {
			return err
		}
		return env.svc.FinishJob(ctx, run.Job.ID, "Shows the temperature.")
	})
}

// newJobEnv is a service with a fake runner and one project.
func newJobEnv(t *testing.T, opts ...testServiceOptions) (*testEnv, *fakeRunner, ProjectSummary) {
	t.Helper()
	env, p := newProjectEnv(t, opts...)
	runner := newFakeRunner()
	env.svc.SetRunner(runner)
	return env, runner, p
}

// reopenService starts a second service on env's directories after env was
// closed (a restart). Random ids avoid collisions with the first run's
// sequential ids; the clock continues a day later so times stay ordered.
func reopenService(t *testing.T, env *testEnv) *testEnv {
	t.Helper()
	cfg := env.cfg
	cfg.IDs = RandomIDs{}
	cfg.Now = (&testClock{t: time.Date(2026, 10, 11, 8, 0, 0, 0, time.UTC)}).Now
	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService after restart: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return &testEnv{svc: svc, builder: env.builder, dir: env.dir, cfg: cfg}
}

// waitJob polls until the job has one of the statuses.
func waitJob(t *testing.T, env *testEnv, jobID string, statuses ...string) Job {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		j, err := env.svc.GetJob(context.Background(), jobID)
		if err == nil && slices.Contains(statuses, j.Status) {
			if !isActiveJobStatus(j.Status) || j.Status == JobAwaitingConfirmation {
				waitIdle(t, env)
			}
			return j
		}
		if time.Now().After(deadline) {
			t.Fatalf("job %s = %+v (%v), want status %v", jobID, j, err, statuses)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// waitIdle waits until no run holds the agent slot.
func waitIdle(t *testing.T, env *testEnv) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for env.svc.activeRun() != nil {
		if time.Now().After(deadline) {
			t.Fatal("the job run did not end")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// fakeTimers replaces the job timeout's real timers: every started timer is
// announced on started and fires only when the test calls fire.
type fakeTimers struct{ started chan *fakeTimer }

type fakeTimer struct {
	d time.Duration
	c chan time.Time
}

func (ft *fakeTimer) fire() { ft.c <- time.Now() }

func newFakeTimers() *fakeTimers { return &fakeTimers{started: make(chan *fakeTimer, 16)} }

func (f *fakeTimers) newTimer(d time.Duration) (<-chan time.Time, func() bool) {
	ft := &fakeTimer{d: d, c: make(chan time.Time, 1)}
	f.started <- ft
	return ft.c, func() bool { return true }
}

// next waits for the next started timer.
func (f *fakeTimers) next(t *testing.T) *fakeTimer {
	t.Helper()
	select {
	case ft := <-f.started:
		return ft
	case <-time.After(5 * time.Second):
		t.Fatal("no job timer was started")
		return nil
	}
}

// useFakeTimers installs fake job timers for the jobs env starts from now on.
func useFakeTimers(env *testEnv) *fakeTimers {
	f := newFakeTimers()
	env.svc.jobs.mu.Lock()
	env.svc.jobs.newTimer = f.newTimer
	env.svc.jobs.mu.Unlock()
	return f
}

// waitClosed waits until ch is closed.
func waitClosed(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

// waitGroupDone waits until wg is back to zero.
func waitGroupDone(t *testing.T, wg *sync.WaitGroup) {
	t.Helper()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	waitClosed(t, done, "the job goroutines")
}

func eventTypes(events []Event) []string {
	out := make([]string, 0, len(events))
	for _, e := range events {
		out = append(out, e.Type)
	}
	return out
}

// drain collects the events that arrived so far.
func drain(ch <-chan Event) []Event {
	var out []Event
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, ev)
		case <-time.After(50 * time.Millisecond):
			return out
		}
	}
}
