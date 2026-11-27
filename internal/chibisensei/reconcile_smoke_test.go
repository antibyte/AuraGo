package chibisensei

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSmoker is a scripted Smoker that records every input and the way each call ended.
type fakeSmoker struct {
	mu      sync.Mutex
	res     SmokeResult
	inputs  []SmokeInput
	ctxErrs []error
	started chan struct{}
	block   bool // the first call waits until its context ends
}

func (f *fakeSmoker) Smoke(ctx context.Context, in SmokeInput) SmokeResult {
	f.mu.Lock()
	f.inputs = append(f.inputs, in)
	first := len(f.inputs) == 1
	f.mu.Unlock()
	if f.block && first {
		if f.started != nil {
			close(f.started)
		}
		<-ctx.Done()
		f.mu.Lock()
		f.ctxErrs = append(f.ctxErrs, ctx.Err())
		f.mu.Unlock()
		return SmokeResult{State: "ok", MS: 1, Text: "late"}
	}
	f.mu.Lock()
	f.ctxErrs = append(f.ctxErrs, ctx.Err())
	f.mu.Unlock()
	return f.res
}

func (f *fakeSmoker) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.inputs)
}

func smokeFixture(t *testing.T, sm Smoker, opt fixtureOpts) *runFixture {
	t.Helper()
	if opt.profile == "" {
		opt.profile = "qwen35-0p8b"
	}
	opt.before = func(fx *runFixture) { fx.o.SetSmoker(sm) }
	return newRunFixture(t, opt)
}

func warnCodes(t *testing.T, fx *runFixture) []string {
	t.Helper()
	var out []string
	for _, w := range warnEvents(t, fx) {
		out = append(out, w[0])
	}
	return out
}

func TestOrchestratorSmokeOK(t *testing.T) {
	sm := &fakeSmoker{res: SmokeResult{State: "ok", MS: 812, Text: "Hello"}}
	fx := smokeFixture(t, sm, fixtureOpts{})
	r := fx.drive(nil)
	if r.State != RunSucceeded || r.Remote.WarningCode != "" {
		t.Fatalf("run = %s %s warning %q", r.State, r.ErrorCode, r.Remote.WarningCode)
	}
	if r.Remote.Smoke == nil || *r.Remote.Smoke != (SmokeResult{State: "ok", MS: 812, Text: "Hello"}) {
		t.Fatalf("smoke = %+v", r.Remote.Smoke)
	}
	if sm.calls() != 1 {
		t.Fatalf("calls = %d", sm.calls())
	}
	in := sm.inputs[0]
	wantPath := filepath.Join(ChibiDir(fx.dataDir), "models", r.ID, "model-q8_0.gguf")
	if in.RunID != r.ID || in.ModelKey != "tuned:"+r.ID || in.ModelPath != wantPath ||
		in.ModelBytes != int64(len(fx.outputs[OutModelGGUF])) || in.Profile == nil || in.Profile.ID != "qwen35-0p8b" {
		t.Errorf("input = %+v (want path %s)", in, wantPath)
	}
	if len(warnCodes(t, fx)) != 0 {
		t.Errorf("warnings = %v", warnCodes(t, fx))
	}
	evs, _ := fx.st.ListRunEvents(context.Background(), r.ID, 0, 1000)
	var found bool
	for i := len(evs) - 1; i >= 0 && !found; i-- {
		var d struct {
			To    string       `json:"to"`
			Smoke *SmokeResult `json:"smoke"`
		}
		if evs[i].Kind == "state" && json.Unmarshal(evs[i].Data, &d) == nil && d.To == RunCleaning {
			found = true
			if d.Smoke == nil || d.Smoke.State != "ok" || d.Smoke.Text != "Hello" {
				t.Errorf("cleaning event smoke = %+v", d.Smoke)
			}
		}
	}
	if !found {
		t.Error("no state event to cleaning")
	}
}

func TestOrchestratorSmokeFailedIsAWarning(t *testing.T) {
	sm := &fakeSmoker{res: SmokeResult{State: "failed", Code: "generation_failed"}}
	fx := smokeFixture(t, sm, fixtureOpts{})
	r := fx.drive(nil)
	if r.State != RunSucceeded || r.Remote.WarningCode != "smoke_failed" {
		t.Fatalf("run = %s %s warning %q", r.State, r.ErrorCode, r.Remote.WarningCode)
	}
	if got := warnCodes(t, fx); len(got) != 1 || got[0] != "smoke_failed" {
		t.Errorf("warn events = %v", got)
	}
	if r.Remote.Smoke == nil || r.Remote.Smoke.State != "failed" {
		t.Errorf("smoke = %+v", r.Remote.Smoke)
	}

	// An earlier warning keeps the run's warning code; the smoke warning stays an event.
	sm2 := &fakeSmoker{res: SmokeResult{State: "failed", Code: "start_failed"}}
	fx2 := smokeFixture(t, sm2, fixtureOpts{base: true})
	fx2.f.SetOutputs(withBaseGGUF(fx2.outOpts, "llama"))
	r2 := fx2.drive(nil)
	if r2.State != RunSucceeded || r2.Remote.WarningCode != "base_gguf_invalid" {
		t.Fatalf("run = %s %s warning %q", r2.State, r2.ErrorCode, r2.Remote.WarningCode)
	}
	got := warnCodes(t, fx2)
	if !strings.Contains(strings.Join(got, ","), "smoke_failed") {
		t.Errorf("warn events = %v", got)
	}
}

func TestOrchestratorSmokeSkippedRecorded(t *testing.T) {
	sm := &fakeSmoker{res: SmokeResult{State: "skipped", Code: "runtime_not_installed"}}
	fx := smokeFixture(t, sm, fixtureOpts{})
	r := fx.drive(nil)
	if r.State != RunSucceeded || r.Remote.WarningCode != "" {
		t.Fatalf("run = %s warning %q", r.State, r.Remote.WarningCode)
	}
	if r.Remote.Smoke == nil || r.Remote.Smoke.State != "skipped" || r.Remote.Smoke.Code != "runtime_not_installed" {
		t.Errorf("smoke = %+v", r.Remote.Smoke)
	}
	if len(warnCodes(t, fx)) != 0 {
		t.Errorf("warnings = %v", warnCodes(t, fx))
	}
}

func TestOrchestratorWithoutSmoker(t *testing.T) {
	fx := newRunFixture(t, fixtureOpts{profile: "qwen35-0p8b"})
	r := fx.drive(nil)
	if r.State != RunSucceeded || r.Remote.Smoke != nil {
		t.Fatalf("run = %s smoke %+v", r.State, r.Remote.Smoke)
	}
	evs, _ := fx.st.ListRunEvents(context.Background(), r.ID, 0, 1000)
	for _, e := range evs {
		if strings.Contains(string(e.Data), "smoke") {
			t.Errorf("event mentions the smoke without a smoker: %s", e.Data)
		}
	}
}

func TestOrchestratorSmokeNormalizesTheResult(t *testing.T) {
	sm := &fakeSmoker{res: SmokeResult{State: "weird", Code: strings.Repeat("c", 80), MS: -5}}
	fx := smokeFixture(t, sm, fixtureOpts{})
	r := fx.drive(nil)
	got := r.Remote.Smoke
	if got == nil || got.State != "failed" || got.Code != "generation_failed" || got.MS != 0 {
		t.Fatalf("smoke = %+v", got)
	}
	if r.Remote.WarningCode != "smoke_failed" {
		t.Errorf("warning = %q", r.Remote.WarningCode)
	}
	sm2 := &fakeSmoker{res: SmokeResult{State: "skipped", Code: strings.Repeat("c", 80), MS: 9}}
	fx2 := smokeFixture(t, sm2, fixtureOpts{})
	if got := fx2.drive(nil).Remote.Smoke; got == nil || len([]rune(got.Code)) != 40 {
		t.Errorf("code = %+v", got)
	}
}

func TestOrchestratorSmokeTextCut(t *testing.T) {
	sm := &fakeSmoker{res: SmokeResult{State: "ok", MS: 3, Text: strings.Repeat("ä", 500)}}
	fx := smokeFixture(t, sm, fixtureOpts{})
	r := fx.drive(nil)
	if r.Remote.Smoke == nil || len([]rune(r.Remote.Smoke.Text)) != 200 {
		t.Fatalf("smoke = %+v", r.Remote.Smoke)
	}
}

func TestOrchestratorCancelDuringSmoke(t *testing.T) {
	sm := &fakeSmoker{block: true, started: make(chan struct{})}
	fx := smokeFixture(t, sm, fixtureOpts{})
	fx.drive(fx.untilState(RunVerifying))
	ctx := context.Background()
	cancelDone := make(chan error, 1)
	go func() {
		<-sm.started
		_, err := fx.o.Cancel(ctx, fx.run.ID)
		cancelDone <- err
	}()
	progressed, err := fx.o.Step(ctx, fx.run.ID)
	if err != nil || !progressed {
		t.Fatalf("step = %v %v", progressed, err)
	}
	if err := <-cancelDone; err != nil {
		t.Fatal(err)
	}
	if len(sm.ctxErrs) != 1 || !errors.Is(sm.ctxErrs[0], context.Canceled) {
		t.Errorf("smoker context = %v", sm.ctxErrs)
	}
	if got := fx.get(); got.State != RunVerifying || got.Remote.Smoke != nil {
		t.Fatalf("after the aborted step: %s smoke %+v", got.State, got.Remote.Smoke)
	}
	r := fx.drive(nil)
	if r.State != RunCancelled || r.ErrorCode != codeCancelled || r.Remote.Smoke != nil {
		t.Fatalf("run = %s %s smoke %+v", r.State, r.ErrorCode, r.Remote.Smoke)
	}
	if sm.calls() != 1 {
		t.Errorf("smoker ran %d times", sm.calls())
	}
}

func TestOrchestratorRestartDuringSmoke(t *testing.T) {
	sm := &fakeSmoker{block: true, started: make(chan struct{}), res: SmokeResult{State: "ok", MS: 5, Text: "Hi"}}
	fx := smokeFixture(t, sm, fixtureOpts{})
	fx.drive(fx.untilState(RunVerifying))
	ctx, stop := context.WithCancel(context.Background())
	go func() {
		<-sm.started
		time.Sleep(10 * time.Millisecond)
		stop()
	}()
	if _, err := fx.o.Step(ctx, fx.run.ID); err == nil {
		t.Fatal("step with a cancelled context returned no error")
	}
	if got := fx.get(); got.State != RunVerifying || got.Remote.Smoke != nil {
		t.Fatalf("after the interrupted step: %s smoke %+v", got.State, got.Remote.Smoke)
	}
	fx.restart()
	fx.o.SetSmoker(sm)
	r := fx.drive(nil)
	if r.State != RunSucceeded || r.Remote.Smoke == nil || r.Remote.Smoke.State != "ok" {
		t.Fatalf("run = %s %s smoke %+v", r.State, r.ErrorCode, r.Remote.Smoke)
	}
	if sm.calls() != 2 {
		t.Errorf("smoker ran %d times, want 2", sm.calls())
	}
}

func TestOrchestratorCancelBeforeSmokeRegistration(t *testing.T) {
	sm := &fakeSmoker{res: SmokeResult{State: "ok", MS: 5, Text: "Hi"}}
	fx := smokeFixture(t, sm, fixtureOpts{})
	fx.drive(fx.untilState(RunVerifying))
	ctx := context.Background()

	// The stop commits after the step has read the run and before the smoke is registered: abortSmoke finds
	// nothing to end, so only the read after the registration can stop the smoke.
	var hookErr error
	smokeBeforeRegister = func(runID string) {
		smokeBeforeRegister = nil
		_, hookErr = fx.o.Cancel(ctx, runID)
	}
	t.Cleanup(func() { smokeBeforeRegister = nil })

	progressed, err := fx.o.Step(ctx, fx.run.ID)
	if hookErr != nil {
		t.Fatal(hookErr)
	}
	if err != nil || !progressed {
		t.Fatalf("step = %v %v", progressed, err)
	}
	if sm.calls() != 0 {
		t.Fatalf("smoker ran %d times after the stop", sm.calls())
	}
	if got := fx.get(); got.State != RunVerifying || got.Remote.Smoke != nil {
		t.Fatalf("after the aborted step: %s smoke %+v", got.State, got.Remote.Smoke)
	}
	r := fx.drive(nil)
	if r.State != RunCancelled || r.ErrorCode != codeCancelled || r.Remote.Smoke != nil {
		t.Fatalf("run = %s %s smoke %+v", r.State, r.ErrorCode, r.Remote.Smoke)
	}
	if sm.calls() != 0 {
		t.Errorf("smoker ran %d times", sm.calls())
	}
}
