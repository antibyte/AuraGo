package chibisensei

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/kaggle"
)

// flakyDriver injects driver errors: Job fails while jobErrs > 0 (always when negative); RemoveFile of a name that
// contains a key of removeErr fails that many times (always when negative).
type flakyDriver struct {
	*fakeKaggle
	mu        sync.Mutex
	jobErrs   int
	removeErr map[string]int
}

func (d *flakyDriver) Job(ctx context.Context, id string) (kaggle.Job, error) {
	d.mu.Lock()
	fail := d.jobErrs != 0
	if d.jobErrs > 0 {
		d.jobErrs--
	}
	d.mu.Unlock()
	if fail {
		return kaggle.Job{}, errors.New("status read refused")
	}
	return d.fakeKaggle.Job(ctx, id)
}

func (d *flakyDriver) RemoveFile(project, name string) error {
	d.mu.Lock()
	for key, n := range d.removeErr {
		if n != 0 && strings.Contains(name, key) {
			if n > 0 {
				d.removeErr[key]--
			}
			d.mu.Unlock()
			return errors.New("remove refused")
		}
	}
	d.mu.Unlock()
	return d.fakeKaggle.RemoveFile(project, name)
}

func (fx *runFixture) useDriver(d KaggleDriver) {
	fx.o = NewOrchestrator(fx.st, fx.hub, fx.cat, d, fx.dataDir, func() OrchestratorConfig { return fx.cfg })
	fx.o.now = fx.clk.Now
}

func (fx *runFixture) step() {
	fx.t.Helper()
	if _, err := fx.o.Step(context.Background(), fx.run.ID); err != nil {
		fx.t.Fatalf("step: %v", err)
	}
}

// warnEvents returns code and message of every warn event of the run.
func warnEvents(t *testing.T, fx *runFixture) [][2]string {
	t.Helper()
	evs, err := fx.st.ListRunEvents(context.Background(), fx.run.ID, 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	var out [][2]string
	for _, e := range evs {
		if e.Kind != "warn" {
			continue
		}
		var d struct{ Code, Message string }
		_ = json.Unmarshal(e.Data, &d)
		out = append(out, [2]string{d.Code, d.Message})
	}
	return out
}

func TestOrchestratorCancelInCleaningAndBeforeCleaning(t *testing.T) {
	ctx := context.Background()
	t.Run("a cancel in cleaning does nothing", func(t *testing.T) {
		fx := newRunFixture(t, fixtureOpts{profile: "qwen35-0p8b", base: true})
		fx.drive(fx.untilState(RunCleaning))
		events := countEvents(t, fx.st, fx.run.ID, "state")
		got, err := fx.o.Cancel(ctx, fx.run.ID)
		if err != nil || got.CancelRequested || got.State != RunCleaning {
			t.Fatalf("cancel in cleaning = %s cancel_requested %v %v", got.State, got.CancelRequested, err)
		}
		if n := countEvents(t, fx.st, fx.run.ID, "state"); n != events {
			t.Errorf("state events %d -> %d", events, n)
		}
		r := fx.drive(nil)
		if r.State != RunSucceeded || r.CancelRequested || r.ErrorCode != "" {
			t.Fatalf("run = %s %s cancel_requested %v", r.State, r.ErrorCode, r.CancelRequested)
		}
	})
	t.Run("a cancel stored before cleaning wins", func(t *testing.T) {
		fx := newRunFixture(t, fixtureOpts{profile: "qwen35-0p8b", base: true})
		r := fx.drive(fx.untilState(RunCleaning))
		if r.Remote.EndState != RunSucceeded {
			t.Fatalf("end state %s", r.Remote.EndState)
		}
		// The row a cancel stored while the verifying step was in flight leaves behind.
		if _, err := fx.st.UpdateRun(ctx, r.ID, RunEvent{}, func(x *Run) error { x.CancelRequested = true; return nil }); err != nil {
			t.Fatal(err)
		}
		r = fx.drive(nil)
		if r.State != RunCancelled || r.ErrorCode != codeCancelled || r.Remote.GGUF != nil {
			t.Fatalf("run = %s %s gguf %v", r.State, r.ErrorCode, r.Remote.GGUF)
		}
		if len(fx.artifactKinds(r)) == 0 {
			t.Error("downloaded artifacts were dropped")
		}
		if len(r.Leftovers) != 0 {
			t.Errorf("leftovers %v", r.Leftovers)
		}
	})
}

func TestOrchestratorRefusalBeforeSubmitFailsAtOnce(t *testing.T) {
	cases := []struct {
		name  string
		state string
		cause error
		code  string
	}{
		{"read-only desktop in uploading", RunUploading, fmt.Errorf("grant check: %w", ErrDesktopReadOnly), "desktop_readonly"},
		{"refused permission in uploading", RunUploading, fmt.Errorf("kaggle refused: %w: %w", ErrKaggleRefused, errors.New("grant off")), "kaggle_refused"},
		{"refused permission in creating_dataset", RunCreatingDataset, fmt.Errorf("kaggle refused: %w", ErrKaggleRefused), "kaggle_refused"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fx := newRunFixture(t, fixtureOpts{profile: "gemma3-270m"})
			fx.drive(fx.untilState(c.state))
			fx.f.SetAccount("tester", c.cause)
			fx.step()
			r := fx.get()
			if r.State != RunCleaning || r.ErrorCode != c.code {
				t.Fatalf("after one step: %s %s", r.State, r.ErrorCode)
			}
			r = fx.drive(nil)
			if r.State != RunFailed || r.ErrorCode != c.code {
				t.Fatalf("end: %s %s", r.State, r.ErrorCode)
			}
		})
	}
}

func TestOrchestratorRefusalAfterSubmitWaits(t *testing.T) {
	fx := newRunFixture(t, fixtureOpts{profile: "qwen35-0p8b", base: true})
	fx.drive(fx.untilState(RunDownloading))
	fx.f.SetAccount("tester", fmt.Errorf("kaggle refused: %w", ErrKaggleRefused))
	for i := 0; i < 6; i++ {
		fx.step()
		fx.clk.Advance(time.Minute)
	}
	r := fx.get()
	if r.State != RunDownloading || r.Remote.BlockedCode != "kaggle_refused" {
		t.Fatalf("run = %s blocked %q", r.State, r.Remote.BlockedCode)
	}
	if got := warnEvents(t, fx); len(got) != 1 || got[0][0] != "kaggle_refused" {
		t.Errorf("warn events = %v", got)
	}
	fx.f.SetAccount("tester", nil)
	if r = fx.drive(nil); r.State != RunSucceeded {
		t.Fatalf("run = %s %s", r.State, r.ErrorCode)
	}
}

func TestOrchestratorStatusReadFailureAfterDeadline(t *testing.T) {
	setup := func(t *testing.T) (*runFixture, *flakyDriver, time.Time) {
		fx := newRunFixture(t, fixtureOpts{profile: "qwen35-0p8b", base: true})
		d := &flakyDriver{fakeKaggle: fx.f}
		fx.useDriver(d)
		r := fx.drive(func(r Run) bool { return r.State == RunRunning && r.Deadline != "" })
		deadline, err := time.Parse(time.RFC3339, r.Deadline)
		if err != nil {
			t.Fatalf("deadline %q: %v", r.Deadline, err)
		}
		return fx, d, deadline
	}
	setNow := func(fx *runFixture, at time.Time) { fx.clk.Advance(at.Sub(fx.clk.Now())) }
	t.Run("two failed reads then the completed session", func(t *testing.T) {
		fx, d, deadline := setup(t)
		run := fx.get()
		fx.f.SetJobStatus(run.Remote.Step(StepSaveKernel).JobID, KaggleJobCompleted, "")
		d.jobErrs = 2
		setNow(fx, deadline.Add(time.Minute))
		for i := 0; i < 2; i++ {
			fx.step()
			if r := fx.get(); r.State != RunRunning {
				t.Fatalf("failed read %d moved the run to %s %s", i, r.State, r.ErrorCode)
			}
		}
		fx.step()
		if r := fx.get(); r.State != RunDownloading {
			t.Fatalf("state %s %s, want downloading", r.State, r.ErrorCode)
		}
	})
	t.Run("reads failing past the grace period time out", func(t *testing.T) {
		fx, d, deadline := setup(t)
		d.jobErrs = -1
		setNow(fx, deadline.Add(29*time.Minute))
		fx.step()
		if r := fx.get(); r.State != RunRunning {
			t.Fatalf("state %s inside the grace period", r.State)
		}
		setNow(fx, deadline.Add(31*time.Minute))
		fx.step()
		if r := fx.get(); r.State != RunCleaning || r.ErrorCode != "remote_timeout" {
			t.Fatalf("state %s %s", r.State, r.ErrorCode)
		}
	})
}

func TestOrchestratorStagedFileRemovalRetried(t *testing.T) {
	setup := func(t *testing.T, failures int) (*runFixture, string) {
		fx := newRunFixture(t, fixtureOpts{profile: "gemma3-270m"})
		name := fx.drive(fx.untilState(RunUploading)).Remote.Staged[0].Name
		fx.useDriver(&flakyDriver{fakeKaggle: fx.f, removeErr: map[string]int{name: failures}})
		return fx, name
	}
	t.Run("a transient failure is retried without a warning", func(t *testing.T) {
		// The first failure falls in the dataset step (its staged files are removed again by cleaning), the
		// second in the first cleaning round.
		fx, _ := setup(t, 2)
		r := fx.drive(nil)
		if r.State != RunSucceeded || r.Remote.Cleanup[cleanupStaged] != "done" {
			t.Fatalf("run = %s %s staged cleanup %q", r.State, r.ErrorCode, r.Remote.Cleanup[cleanupStaged])
		}
		for _, w := range warnEvents(t, fx) {
			t.Errorf("unexpected warn %v", w)
		}
		for _, n := range fx.f.ProjectFiles() {
			if strings.HasPrefix(n, "r"+Run12(r.ID)+"-") {
				t.Errorf("staged file %s kept", n)
			}
		}
	})
	t.Run("a persistent failure ends the run with a warning that names the file", func(t *testing.T) {
		fx, name := setup(t, -1)
		r := fx.drive(nil)
		if r.State != RunSucceeded || r.Remote.Cleanup[cleanupStaged] != cleanFailed {
			t.Fatalf("run = %s %s staged cleanup %q", r.State, r.ErrorCode, r.Remote.Cleanup[cleanupStaged])
		}
		warns := warnEvents(t, fx)
		if len(warns) != 1 || warns[0][0] != "staged_left" || warns[0][1] != name {
			t.Fatalf("warn events = %v, want staged_left naming %s", warns, name)
		}
	})
}

func TestOrchestratorJobReadErrorInCleaningWaits(t *testing.T) {
	fx := newRunFixture(t, fixtureOpts{profile: "gemma3-270m"})
	d := &flakyDriver{fakeKaggle: fx.f}
	fx.useDriver(d)
	r := fx.drive(fx.untilState(RunCleaning))
	// A transfer job that still looks active in the run row and whose status cannot be read.
	if _, err := fx.st.UpdateRun(context.Background(), r.ID, RunEvent{}, func(x *Run) error {
		x.Remote.Jobs[0].Status = KaggleJobRunning
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	d.jobErrs = -1
	for i := 0; i < 3; i++ {
		fx.step()
	}
	if r = fx.get(); r.State != RunCleaning || r.Remote.Cleanup[cleanupStaged] != "" {
		t.Fatalf("cleaning went on while a job could not be read: %s %v", r.State, r.Remote.Cleanup)
	}
	d.jobErrs = 0
	if r = fx.drive(nil); r.State != RunSucceeded {
		t.Fatalf("run = %s %s", r.State, r.ErrorCode)
	}
}

func TestOrchestratorLeftoversListOnlyWhatKaggleHas(t *testing.T) {
	cases := []struct {
		name, code string
		outcome    fakeOutcome
	}{
		{"slot busy", "kaggle_slot_busy", fakeOutcome{Status: KaggleJobFailed, Error: "concurrent Kaggle compute limit reached"}},
		{"submit failed", "submit_failed", fakeOutcome{Status: KaggleJobFailed, Error: "the notebook was rejected"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fx := newRunFixture(t, fixtureOpts{profile: "gemma3-270m",
				before: func(fx *runFixture) { fx.f.Script("save_kernel", c.outcome) }})
			r := fx.drive(nil)
			if r.State != RunFailed || r.ErrorCode != c.code {
				t.Fatalf("run = %s %s", r.State, r.ErrorCode)
			}
			if len(r.Leftovers) != 0 || len(warnEvents(t, fx)) != 0 {
				t.Errorf("leftovers %v warnings %v", r.Leftovers, warnEvents(t, fx))
			}
			if n := len(fx.f.Mutations("delete_kernel")); n != 0 {
				t.Errorf("delete_kernel requests = %d for a notebook that never existed", n)
			}
			if got := r.Remote.Cleanup[cleanupNotebook]; got != cleanAbsent {
				t.Errorf("notebook cleanup = %q", got)
			}
			if n := len(fx.f.Mutations("delete_dataset")); n != 1 {
				t.Errorf("delete_dataset requests = %d", n)
			}
		})
	}
}
