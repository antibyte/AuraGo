[← Plan index](README.md) · Spec: [plans/2026-11-12-easydrag-s0c-spec.md](../2026-11-12-easydrag-s0c-spec.md)

## Part A1: Trigger start failures become failed runs

Spec: `plans/2026-11-12-easydrag-s0c-spec.md` §1 (decision 3), §3.1, §7 row 6, §8, §11. The tasks run in the order A1.1 to A1.3, before A2 to A7 (same worktree and branch).

**Shell.** Git Bash. Every command starts with `cd <worktree> && ...` because the working directory resets between calls. `WT` below means `C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects`.

**Worktree and baseline.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo && git worktree add .worktrees/easydrag-s0c-defects -b feat/easydrag-s0c-defects main
mkdir -p C:/disposable/easydrag-s0c D:/tmp/go-build
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && git log -1 --format=%h && git status --short
```

Expected: the worktree is created from `main` (b52103d51 or later); `git status --short` prints nothing. Run the baseline once before A1.1. A failure that is already on main is reported to the coordinator, not fixed.

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows/... ./internal/tools -run '(?i)flow' -count=1 2>&1 | tail -15
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && go test ./internal/server -run '(?i)flow' -count=1 2>&1 | tail -15
```

Expected: `ok` for `internal/flows`, `internal/flows/...` sub-packages that have tests, `internal/tools` and `internal/server` (the server run takes a few minutes). Copy any `FAIL` line into the report.

**Conventions of this part.**
- Go temp dir: `GOTMPDIR='D:\tmp\go-build'` for `internal/flows` and `internal/tools`; none for `internal/server` (a temp dir on D: breaks a cross-drive `filepath.Rel` there).
- Gates before every Go commit: `go vet` on the touched packages and `gofmt -l` on the touched files. `gofmt -l` must print nothing; if it lists a file, run `gofmt -w` on it and check again.
- Escapes: the Write and Edit tools turn backslash-u sequences into characters. This part has none; keep it that way.
- Commit: one per task. `git add <exact paths>`, then `git commit -F C:/disposable/easydrag-s0c/<message file>`. Write each message file with the Write tool. Never amend, rebase or push. Every message ends with the GitNexus line, a blank line and the Co-Authored-By line (the implementer fills in its model line).
- GitNexus cannot index a worktree on this machine. Run IMPACT from the main checkout path (the command is in each task); HIGH or CRITICAL is reported, UNKNOWN is confirmed with `grep -rn`.

**What the code does today (verified on b52103d51; the spec's line numbers are a few lines off).**
- `Service.TriggerFromMission` (`internal/flows/service_runs.go:235`) ends in `startLive(..., gated=true)` (`:269`). `startLive` returns `ErrNotPublished` and `ErrNoTrigger` before the runner sees the request, else whatever `Runner.StartContext` returns. No run is recorded in any of these cases.
- The runner's gate refusal never reaches the caller as `RunsPausedCode`. `RunsPausedCode` (`runner.go:124`) is only the code of a queued run that the gate stopped (`runner.go:742`). A gate refusal before the start (`runner.go:469-473`, only for block and durable flows) returns the bridge's raw error. The server's gate is `automaticFlowRunsAllowed` (`flows_service.go:337`: `tools.ErrFlowsUnavailable`, `tools.ErrFlowRunsPaused`), and the server's hook (`flows_bridge.go:362`) already asks it before it calls `TriggerFromMission`. `internal/flows` cannot import `internal/tools`, so the Service tells a gate refusal by asking its own `RunGate` again (`s.automaticRunsAllowed()`).
- `internal/flows` tests build a Service on the real `Runner`; there is no runner interface to fake. The streak test therefore drives the real runner with real start errors (`ErrNoTrigger` from an unknown trigger node id, `ErrNotPublished` from an unpublished flow, `ErrRunnerClosed` after `Shutdown`).
- Mission notes are English constants in `internal/tools/missions_v2_flows.go` (`flowRunsPausedOutput` and the others, stored as the mission's `LastOutput`). No `ui/lang` file holds them. Only the code `FLOW_START_FAILED` is translated, as `easydrag.ui.error_flow_start_failed` in `ui/lang/easydrag/*.json`.
- The members of the `easydrag.ui.error_` family are listed in `TestEasyDragUIKeysExistInAllLocales` (`ui/desktop_easydrag_test.go:253-264`); `FLOW_RUNS_PAUSED` is not in it, `flow_request_cancelled` is the last member today.

**Design in one paragraph.** A counted start failure (`ErrNotPublished`, `ErrNoTrigger`, any other start error that is not a deliberate refusal) is classified synchronously in `TriggerFromMission`. A small map `Service.startStreaks` (flow id to reason, under `Service.mu`) decides whether this failure opens a new streak. Only then does the Service count one goroutine on `Service.startFailures` (a `sync.WaitGroup`, `Add` only under `mu` while not closed, waited for by `Shutdown`) and spawn `recordStartFailure`, which writes the failed run (`Store.CreateRun` with status `error`, then `Store.FinishRun`, which scrubs the message) and calls the optional `StartFailureReporter`. The server implements the reporter; Mission Control (`internal/tools`) gets the start failure as a wrapped `tools.ErrFlowStartFailed`, and notes it like a refusal.

---

### Task A1.1: `internal/flows`, classify, record and report a start failure

**Files:**
- Create: `WT/internal/flows/service_start_failures.go`
- Create: `WT/internal/flows/service_start_failures_test.go`
- Modify: `WT/internal/flows/service.go:63-73` (new optional interface after `RunGate`), `:160-194` (two fields), `:219-220` (map in `NewService`), `:340-344` (`Shutdown` waits)
- Modify: `WT/internal/flows/service_runs.go:235-252` (`TriggerFromMission`)
- Modify: `WT/internal/flows/service_runs.go:646-718` (`onTimerFired`, Date/Time timers count too)
- Modify: `WT/internal/flows/service_mission.go:433-444` (`deleteLocked` clears the streak)
- Test: `TestStartFailureStreak` in `WT/internal/flows/service_start_failures_test.go`

**IMPACT (run before editing, from the main checkout; warn on HIGH or CRITICAL, confirm UNKNOWN with `grep -rn`):**

```bash
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "TriggerFromMission" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "onTimerFired" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "deleteLocked" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "Shutdown" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "NewService" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
```

Expected: `onTimerFired` is passed to `newTimerService` in `NewService` as the timer callback (a function value, so the index may show UNKNOWN callers; confirm with `grep -rn "onTimerFired" internal --include=*.go`: `service.go:236` and tests). `TriggerFromMission` is called by `flowMissionHooks.StartFlowRunReported` (server) and tests; `NewService` and `Shutdown` have many callers (server boot, every test fixture) and may come out HIGH. This task only adds fields and a wait, so a HIGH is reported and the change goes ahead.

- [ ] **Step 1: Write the failing test.** Create `WT/internal/flows/service_start_failures_test.go`:

```go
package flows

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// svcStartFailBridge is svcGateBridge (a RunGate a test can close) with the optional
// StartFailureReporter, so a test sees what the Service reports.
type svcStartFailBridge struct {
	*svcGateBridge
	failMu sync.Mutex
	failed []RunFinishedInfo
}

func newSvcStartFailBridge() *svcStartFailBridge {
	return &svcStartFailBridge{svcGateBridge: &svcGateBridge{svcRunBridge: newSvcRunBridge()}}
}

func (b *svcStartFailBridge) FlowStartFailed(info RunFinishedInfo) {
	b.failMu.Lock()
	b.failed = append(b.failed, info)
	b.failMu.Unlock()
}

func (b *svcStartFailBridge) reported() []RunFinishedInfo {
	b.failMu.Lock()
	defer b.failMu.Unlock()
	return append([]RunFinishedInfo(nil), b.failed...)
}

// waitStartFailures waits for the record goroutines of start failures; the Service counts them
// before TriggerFromMission returns, so this is deterministic.
func waitStartFailures(s *Service) { s.startFailures.Wait() }

// startFailureRuns returns the stored runs of the flow that carry FLOW_START_FAILED.
func startFailureRuns(t *testing.T, s *Service, flowID string) []RunRecord {
	t.Helper()
	runs, err := s.Store().ListRuns(context.Background(), flowID, RunFilter{Limit: 50})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	var out []RunRecord
	for _, r := range runs {
		if r.ErrorCode == StartFailedCode {
			out = append(out, r)
		}
	}
	return out
}

func TestStartFailureStreak(t *testing.T) {
	// One record per streak: fail, fail, success, fail records two failed runs.
	t.Run("a success ends the streak", func(t *testing.T) {
		bridge := newSvcStartFailBridge()
		s := svcRunNewService(t, &fakeTools{}, bridge, nil, ServiceConfig{})
		runs := svcRunObserve(s)
		pub := svcRunPublish(t, s, simpleFlow("Streak"))
		fire := func(node string) (StartResult, error) { return s.TriggerFromMission(pub.MissionID, node, "webhook", nil) }
		for i := 0; i < 2; i++ {
			if _, err := fire("n_nope"); !errors.Is(err, ErrNoTrigger) {
				t.Fatalf("fire %d: err = %v, want ErrNoTrigger", i, err)
			}
		}
		res, err := fire("")
		if err != nil {
			t.Fatalf("a valid start: %v", err)
		}
		runs.wait(t, res.RunID)
		if _, err := fire("n_nope"); !errors.Is(err, ErrNoTrigger) {
			t.Fatalf("the fire after the success: err = %v, want ErrNoTrigger", err)
		}
		waitStartFailures(s)
		got := startFailureRuns(t, s, pub.ID)
		if len(got) != 2 || len(bridge.reported()) != 2 {
			t.Fatalf("recorded %d failed runs, reported %d; want 2 and 2", len(got), len(bridge.reported()))
		}
		rec := got[0]
		if rec.Status != RunError || rec.Mode != ModeLive || rec.Revision != pub.LiveRevision || rec.TriggerNode != "n_nope" ||
			rec.TriggerType != "webhook" || rec.FinishedAt == nil || !rec.StartedAt.Equal(*rec.FinishedAt) || !strings.Contains(rec.ErrorMessage, "trigger") {
			t.Fatalf("failed run = %+v", rec)
		}
		info := bridge.reported()[0]
		if info.Started || info.MissionID != pub.MissionID || info.FlowName != "Streak" || info.Record.ErrorCode != StartFailedCode ||
			info.Result.Status != RunError || info.Outputs == nil {
			t.Fatalf("reported info = %+v", info)
		}
	})

	// A different reason is a new streak: not_published, then no_trigger, records two.
	t.Run("a new reason records again", func(t *testing.T) {
		bridge := newSvcStartFailBridge()
		s := svcRunNewService(t, &fakeTools{}, bridge, nil, ServiceConfig{})
		ctx := context.Background()
		rec, err := s.CreateFlow(ctx, CreateRequest{Name: "Reasons"})
		if err != nil {
			t.Fatal(err)
		}
		rev, _, err := s.SaveDraft(ctx, rec.ID, simpleFlow("Reasons"), rec.DraftRevision)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.TriggerFromMission(rec.MissionID, "", "manual", nil); !errors.Is(err, ErrNotPublished) {
			t.Fatalf("unpublished: err = %v", err)
		}
		pub, _, err := s.Publish(ctx, rec.ID, rev)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.TriggerFromMission(rec.MissionID, "n_nope", "manual", nil); !errors.Is(err, ErrNoTrigger) {
			t.Fatalf("published, unknown trigger: err = %v", err)
		}
		waitStartFailures(s)
		got := startFailureRuns(t, s, rec.ID)
		if len(got) != 2 {
			t.Fatalf("recorded %d failed runs, want 2", len(got))
		}
		byRevision := map[int]RunRecord{got[0].Revision: got[0], got[1].Revision: got[1]}
		if r, ok := byRevision[0]; !ok || r.ErrorMessage != "the flow is not published" {
			t.Fatalf("the unpublished record (revision 0) = %+v", byRevision)
		}
		if r, ok := byRevision[pub.LiveRevision]; !ok || !strings.Contains(r.ErrorMessage, "trigger") {
			t.Fatalf("the no-trigger record (revision %d) = %+v", pub.LiveRevision, byRevision)
		}
	})

	// A refusal on purpose records nothing: a closed RunGate (runs paused) and a closed runner.
	t.Run("refusals record nothing", func(t *testing.T) {
		bridge := newSvcStartFailBridge()
		s := svcRunNewService(t, &fakeTools{}, bridge, nil, ServiceConfig{})
		ctx := context.Background()
		rec, err := s.CreateFlow(ctx, CreateRequest{Name: "Paused"})
		if err != nil {
			t.Fatal(err)
		}
		bridge.svcGateBridge.err = errors.New("flow runs are paused while missions are read-only")
		if _, err := s.TriggerFromMission(rec.MissionID, "", "manual", nil); !errors.Is(err, ErrNotPublished) {
			t.Fatalf("paused, unpublished: err = %v", err)
		}
		waitStartFailures(s)
		if got := startFailureRuns(t, s, rec.ID); len(got) != 0 {
			t.Fatalf("a refused trigger recorded %d runs", len(got))
		}
		bridge.svcGateBridge.err = nil // the same failure now counts, so the gate was the reason before
		if _, err := s.TriggerFromMission(rec.MissionID, "", "manual", nil); !errors.Is(err, ErrNotPublished) {
			t.Fatalf("unpaused, unpublished: err = %v", err)
		}
		waitStartFailures(s)
		if got := startFailureRuns(t, s, rec.ID); len(got) != 1 {
			t.Fatalf("after the gate opened: %d runs, want 1", len(got))
		}

		closed := newSvcStartFailBridge()
		s2 := svcRunNewService(t, &fakeTools{}, closed, nil, ServiceConfig{})
		pub := svcRunPublish(t, s2, simpleFlow("Closing"))
		if err := s2.Shutdown(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := s2.TriggerFromMission(pub.MissionID, "", "manual", nil); !errors.Is(err, ErrRunnerClosed) {
			t.Fatalf("closed runner: err = %v", err)
		}
		waitStartFailures(s2)
		if got := startFailureRuns(t, s2, pub.ID); len(got) != 0 || len(closed.reported()) != 0 {
			t.Fatalf("a closed runner recorded %d runs and reported %d", len(got), len(closed.reported()))
		}
	})

	// A Date/Time timer has no caller to answer, so its start failures count like a trigger's.
	// onTimerFired is the unexported fire function the timer service calls; the test calls it
	// directly, which is deterministic (the callback runs on the caller's goroutine).
	t.Run("a timer start that fails twice records one run", func(t *testing.T) {
		bridge := newSvcStartFailBridge()
		s := svcRunNewService(t, &fakeTools{}, bridge, nil, ServiceConfig{})
		pub := svcRunPublish(t, s, simpleFlow("Timed"))
		at := time.Date(2026, 10, 3, 8, 0, 0, 0, time.UTC)
		for i := 0; i < 2; i++ {
			// The node is no enabled trigger of the published revision: ErrNoTrigger, a consumed occurrence.
			if err := s.onTimerFired(pub.ID, "n_nope", at.Add(time.Duration(i)*time.Minute)); err != nil {
				t.Fatalf("fire %d: %v", i, err)
			}
		}
		waitStartFailures(s)
		got := startFailureRuns(t, s, pub.ID)
		if len(got) != 1 || got[0].TriggerType != "datetime" || got[0].TriggerNode != "n_nope" || len(bridge.reported()) != 1 {
			t.Fatalf("two failed timer starts recorded %+v, reported %d; want one run of type datetime", got, len(bridge.reported()))
		}

		// A timer of a flow without a published revision is the other early return.
		rec, err := s.CreateFlow(context.Background(), CreateRequest{Name: "Timed draft"})
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			if err := s.onTimerFired(rec.ID, "n_nope", at.Add(time.Duration(i)*time.Minute)); err != nil {
				t.Fatalf("draft fire %d: %v", i, err)
			}
		}
		waitStartFailures(s)
		if got := startFailureRuns(t, s, rec.ID); len(got) != 1 || got[0].ErrorMessage != "the flow is not published" {
			t.Fatalf("two timers of an unpublished flow recorded %+v, want one not-published run", got)
		}
	})

	// The classification itself: what counts and what does not.
	t.Run("classification", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			err  error
			want string
		}{
			{"not published", ErrNotPublished, "not_published"},
			{"no trigger", fmt.Errorf("%w: x", ErrNoTrigger), "no_trigger"},
			{"other start error", errors.New("a node could not be prepared"), "start_error: a node could not be prepared"},
			{"flow deleted", fmt.Errorf("record run: %w", ErrNotFound), ""},
			{"runner closed", ErrRunnerClosed, ""},
			{"queue full", ErrRunnerQueueFull, ""},
			{"switched off", ErrFlowDisabled, ""},
			{"mission missing", ErrFlowMissionMissing, ""},
			{"mission control unavailable", ErrMissionControlUnavailable, ""},
			{"caller gave up", context.Canceled, ""},
		} {
			if got := startFailureReason(tc.err); got != tc.want {
				t.Errorf("%s: reason = %q, want %q", tc.name, got, tc.want)
			}
			if got := IsStartFailure(tc.err); got != (tc.want != "") {
				t.Errorf("%s: IsStartFailure = %v", tc.name, got)
			}
		}
	})
}
```

- [ ] **Step 2: Run it and see it fail to compile.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -run 'TestStartFailureStreak' -count=1 2>&1 | head -12
```

Expected: build errors `undefined: StartFailedCode`, `undefined: startFailureReason`, `undefined: IsStartFailure` and `s.startFailures undefined`.

- [ ] **Step 3: Add the optional bridge interface and the two fields.** In `WT/internal/flows/service.go` insert after the `RunGate` interface (after line 73):

```go
// StartFailureReporter is an optional MissionBridge extension, found by type assertion. A
// trigger start that failed for a reason of the flow (Service.TriggerFromMission: the flow is not
// published, the trigger is not an enabled one, the runner could not start the run) is recorded
// as a failed live run (StartFailedCode). FlowStartFailed is called off the caller's goroutine,
// once per failure streak, after that run was recorded. info.Started is false, no
// FlowRunStarted came before, and no FlowRunFinished follows; info.Record and info.Result hold
// the failed run, info.NotifyOnError the flow's setting. Like every bridge method it follows
// the bridge rule: no call into a Service method that takes a flow lock.
type StartFailureReporter interface {
	FlowStartFailed(info RunFinishedInfo)
}
```

In the `Service` struct (after `reconciling sync.WaitGroup`, line 193) add:

```go
	// startStreaks holds, by flow id, the reason of the flow's current start-failure streak
	// (see recordStartFailure); guarded by mu. A successful start and a delete remove the entry.
	startStreaks map[string]string
	// startFailures counts the goroutines that record a start failure. Add happens only under mu
	// while !closed, so it never races with Shutdown's Wait.
	startFailures sync.WaitGroup
```

In `NewService`, change the struct literal (line 219-220) to:

```go
	s := &Service{store: store, reg: reg, services: services, bridge: bridge, cfg: cfg, logger: logger,
		history: map[string]runHistory{}, startStreaks: map[string]string{}, stop: make(chan struct{}), done: make(chan struct{})}
```

In `Shutdown`, the goroutine that waits for reconciliations (lines 340-344) now waits for the record goroutines too:

```go
	reconciled := make(chan struct{})
	go func() {
		s.reconciling.Wait()
		s.startFailures.Wait()
		close(reconciled)
	}()
```

The doc comment of `Shutdown` (line 314-317) gets the words "and for the goroutines that record a start failure" after "for a ReconcileMissions in progress".

- [ ] **Step 4: Write the implementation.** Create `WT/internal/flows/service_start_failures.go`:

```go
package flows

import (
	"context"
	"errors"
	"time"
)

// StartFailedCode is the error code of a live run that never started because the trigger could
// not start it (see Service.recordStartFailure). The client translates it as
// easydrag.ui.error_flow_start_failed.
const StartFailedCode = "FLOW_START_FAILED"

// startFailureTimeout bounds the store writes of one recorded start failure.
const startFailureTimeout = 30 * time.Second

// Streak reasons of the failures that have a reason of their own; any other start error is
// "start_error: " and its text (cut), so a repeated identical error is one streak too.
const (
	startFailureNotPublished = "not_published"
	startFailureNoTrigger    = "no_trigger"
	startFailureOther        = "start_error: "
)

// startFailureReason classifies the error of a gated live start. It returns "" for a refusal or a
// condition that is not a fault of the flow (the failure is not recorded):
//   - the flow was deleted meanwhile (ErrNotFound), the runner is shutting down (ErrRunnerClosed),
//     the queue is full (ErrQueueFull, also ErrRunnerQueueFull) or the caller's context ended;
//   - the flow is switched off or Mission Control cannot say (ErrFlowDisabled,
//     ErrFlowMissionMissing, ErrMissionControlUnavailable), the refusals of the block admission.
//
// Otherwise it returns the reason that names the streak.
func startFailureReason(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrNotPublished):
		return startFailureNotPublished
	case errors.Is(err, ErrNoTrigger):
		return startFailureNoTrigger
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrRunnerClosed), errors.Is(err, ErrQueueFull),
		errors.Is(err, ErrFlowDisabled), errors.Is(err, ErrFlowMissionMissing), errors.Is(err, ErrMissionControlUnavailable),
		errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return ""
	}
	return startFailureOther + truncateRunes(err.Error(), 200)
}

// IsStartFailure reports whether err, returned by TriggerFromMission, is a start failure that
// the Service records as a failed run. The server uses it to mark the error for Mission Control.
// A refusal of the RunGate is not visible here (see Service.afterTriggerStart); the server's
// hook asks that gate before it starts anything.
func IsStartFailure(err error) bool { return startFailureReason(err) != "" }

// startFailureMessage says in plain words why a start failed. The Store scrubs it when it is
// written, like every run error; it is cut to maxErrorMessageRunes first.
func startFailureMessage(reason string, err error) string {
	switch reason {
	case startFailureNotPublished:
		return "the flow is not published"
	case startFailureNoTrigger:
		return "the trigger that fired is not an enabled trigger of the published flow"
	}
	return truncateRunes("the flow could not start: "+err.Error(), maxErrorMessageRunes)
}

// afterTriggerStart is TriggerFromMission's bookkeeping after startLive: a start (started or
// queued) ends the flow's failure streak; a start failure that is not a refusal opens a new
// streak, and only then a goroutine records it. The streak is decided here, synchronously,
// because TriggerFromMission can run inside another flow's DeleteFlow (through
// MissionBridge.FlowRunFinished) with that flow's lock held: nothing slow may run on this
// goroutine, and the streak bounds the goroutines to one per flow and reason. A closed RunGate
// (runs paused) is a refusal, not a fault of the flow, so it records nothing.
func (s *Service) afterTriggerStart(rec *FlowRecord, nodeID, triggerType string, res StartResult, err error) {
	if err == nil {
		if res.Status == StartStarted || res.Status == StartQueued {
			s.clearStartFailure(rec.ID)
		}
		return
	}
	reason := startFailureReason(err)
	if reason == "" || s.automaticRunsAllowed() != nil {
		return
	}
	if !s.beginStartFailure(rec.ID, reason) {
		return
	}
	go s.recordStartFailure(rec, nodeID, triggerType, reason, err)
}

// beginStartFailure reports whether a failure with reason opens a new streak of the flow, and
// then counts the goroutine that records it. It reports false for a repeat of the current
// streak and after Shutdown began.
func (s *Service) beginStartFailure(flowID, reason string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.startStreaks[flowID] == reason {
		return false
	}
	s.startStreaks[flowID] = reason
	s.startFailures.Add(1)
	return true
}

// clearStartFailure ends the flow's failure streak: a start succeeded, or the flow is gone.
func (s *Service) clearStartFailure(flowID string) {
	s.mu.Lock()
	delete(s.startStreaks, flowID)
	s.mu.Unlock()
}

// recordStartFailure writes the failed run of a start that did not happen and reports it to
// the bridge's optional StartFailureReporter. The run is live, has the flow's live revision (0
// when there is none), the trigger node and type, status error, StartFailedCode, a plain
// message, StartedAt = FinishedAt = now and no steps and no trigger data (a start that failed
// never ran on it). It runs on its own goroutine (afterTriggerStart) and ends the WaitGroup
// count. When the flow is gone (ErrNotFound) it records nothing and clears the streak; on any
// other store error it logs at Warn and clears the streak, so the next failure tries again.
func (s *Service) recordStartFailure(rec *FlowRecord, nodeID, triggerType, reason string, startErr error) {
	defer s.startFailures.Done()
	ctx, cancel := context.WithTimeout(context.Background(), startFailureTimeout)
	defer cancel()
	now := s.now()
	msg := startFailureMessage(reason, startErr)
	run := RunRecord{ID: NewRunID(), FlowID: rec.ID, Revision: rec.LiveRevision, Mode: ModeLive,
		TriggerNode: truncateRunes(nodeID, 80), TriggerType: cleanTriggerType(triggerType),
		Status: RunError, StartedAt: now}
	if run.TriggerNode == "" {
		doc := rec.Live
		if doc == nil {
			doc = rec.Draft
		}
		if doc != nil {
			run.TriggerNode = s.pickTrigger(doc, "")
		}
	}
	res := RunResult{Status: RunError, ErrorCode: StartFailedCode, ErrorMessage: msg, StartedAt: now, FinishedAt: now}
	err := s.store.CreateRun(ctx, run, nil)
	if err == nil {
		err = s.store.FinishRun(ctx, run.ID, res)
	}
	if err != nil {
		s.clearStartFailure(rec.ID)
		s.logLookup("a trigger start failure could not be recorded as a run", run.ID, err, "flow", rec.ID)
		return
	}
	run.ErrorCode, run.ErrorMessage, run.FinishedAt = StartFailedCode, msg, &now
	reporter, ok := s.bridge.(StartFailureReporter)
	if !ok {
		return
	}
	notify := ""
	if doc := rec.Live; doc != nil {
		notify = doc.Settings.NotifyOnError
	} else if rec.Draft != nil {
		notify = rec.Draft.Settings.NotifyOnError
	}
	defer func() {
		if v := recover(); v != nil {
			s.logger.Error("the bridge panicked on a start failure", "flow", rec.ID, "run", run.ID, "panic", v)
		}
	}()
	reporter.FlowStartFailed(RunFinishedInfo{MissionID: rec.MissionID, Started: false, FlowName: rec.Name,
		NotifyOnError: notify, Record: run, Result: res, Outputs: map[string]any{}})
}
```

- [ ] **Step 5: Call it from `TriggerFromMission`.** In `WT/internal/flows/service_runs.go`, the last line of `TriggerFromMission` (line 251) becomes:

Before:

```go
	return s.startLive(context.Background(), rec, nodeID, triggerType, data, true)
}
```

After:

```go
	res, err := s.startLive(context.Background(), rec, nodeID, triggerType, data, true)
	s.afterTriggerStart(rec, nodeID, triggerType, res, err)
	return res, err
}
```

Add one paragraph to the doc comment of `TriggerFromMission`, before "It takes no flow lock":

```go
// A start that fails for a reason of the flow (ErrNotPublished, ErrNoTrigger, any start error
// that is no refusal; see startFailureReason) is also recorded as a failed live run with
// StartFailedCode and reported to the bridge's StartFailureReporter, once per failure streak
// and on a goroutine of its own (afterTriggerStart). The caller still gets the error. A start
// (started or queued) ends the streak.
//
```

- [ ] **Step 5b: Date/Time timers count too.** In `WT/internal/flows/service_runs.go`, `onTimerFired` (today `:663-718`), two calls.

Lock context, checked: the timer service calls `onTimerFired` through `TimerService.call` (`timers.go:495-510`) on its own timer goroutine, holding neither a flow lock nor the timer service's mutex (the callback is a plain `fn(tm.FlowID, tm.NodeID, tm.FireAt)`); it never runs inside `DeleteFlow`. The goroutine design therefore already covers it, and it also keeps the timer loop free of store and bridge work. A timer that failed to start is retried by the timer service with a backoff and the same `scheduled_for`; the streak makes each retry a repeat, so a retrying timer records once.

Early return for a flow without a published revision. Before:

```go
	if rec.Live == nil {
		s.logger.Warn("a date and time trigger fired for a flow without a published revision; no run was started",
			"flow", flowID, "node", nodeID)
		return nil
	}
```

After:

```go
	if rec.Live == nil {
		s.logger.Warn("a date and time trigger fired for a flow without a published revision; no run was started",
			"flow", flowID, "node", nodeID)
		s.afterTriggerStart(rec, nodeID, "datetime", StartResult{}, ErrNotPublished)
		return nil
	}
```

The `startLive` result. Before:

```go
	res, err := s.startLive(context.Background(), rec, nodeID, "datetime", data, true)
	switch {
```

After:

```go
	res, err := s.startLive(context.Background(), rec, nodeID, "datetime", data, true)
	s.afterTriggerStart(rec, nodeID, "datetime", res, err)
	switch {
```

`afterTriggerStart` sees the same errors as for a mission trigger: `ErrNoTrigger` and other start errors record (once per streak), `ErrNotFound`, `ErrQueueFull`, `ErrRunnerClosed` and the rest record nothing, a started or queued run ends the streak, and `StartSkipped` changes nothing. The gate and switch-off checks at the top of `onTimerFired` return before either call, so a paused or switched-off flow never reaches them. The return values of `onTimerFired` do not change. Add one sentence to its doc comment: "A start failure that is a fault of the flow (a flow without a published revision, a node that is no enabled trigger, any other start error that is no refusal) is also recorded as a failed run, once per failure streak (afterTriggerStart)."

- [ ] **Step 6: Clear the streak when a flow is deleted.** In `WT/internal/flows/service_mission.go`, `deleteLocked` (line 433-444) gets one line before `return nil`:

Before:

```go
	s.runner.CancelFlow(id)
	s.runner.forgetDeletedDurableFlow(id)
	return nil
}
```

After:

```go
	s.runner.CancelFlow(id)
	s.runner.forgetDeletedDurableFlow(id)
	s.clearStartFailure(id)
	return nil
}
```

- [ ] **Step 7: Run the test and see it pass.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -run 'TestStartFailureStreak' -count=1 -v 2>&1 | grep -E "^(=== RUN|--- (PASS|FAIL)|PASS|FAIL|ok)" 
```

Expected: six `--- PASS` lines (`TestStartFailureStreak` and its five subtests) and `ok  aurago/internal/flows`. If "a success ends the streak" fails with 3 records, the streak was not cleared on `StartStarted`; if it fails with `ListRuns` showing the run without `ErrorCode`, `FinishRun` did not run.

- [ ] **Step 8: Run the race detector on the new test, then the package.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -run 'TestStartFailureStreak' -race -count=3 2>&1 | tail -5
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows/... -count=1 2>&1 | tail -8
```

Expected: `ok` for both. (If `-race` is unavailable on this machine, `go test` says `-race requires cgo`; then skip the first command and say so in the report.)

- [ ] **Step 9: Gates.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && GOTMPDIR='D:\tmp\go-build' go vet ./internal/flows/ && gofmt -l internal/flows/service.go internal/flows/service_runs.go internal/flows/service_mission.go internal/flows/service_start_failures.go internal/flows/service_start_failures_test.go
```

Expected: no output.

- [ ] **Step 10: Commit.** Write `C:/disposable/easydrag-s0c/a1-1-message.txt` with the Write tool:

```
feat(flows): record a trigger start failure as a failed run

TriggerFromMission now records a start that fails for a reason of the flow
(not published, no matching trigger, any other start error that is no
refusal) as a failed live run with FLOW_START_FAILED, once per failure
streak, and reports it to the new optional StartFailureReporter on the
bridge. The streak is decided synchronously under Service.mu; the record
and the report run on a goroutine that Shutdown waits for, because
TriggerFromMission can run inside another flow's DeleteFlow. A start
(started or queued) and a delete end the streak. Date/Time timers
(onTimerFired) count like triggers. Deliberate refusals (a
closed RunGate, a closed runner, a deleted flow, a full queue, a switched
off flow) record nothing.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <your model line>
```

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && git add internal/flows/service.go internal/flows/service_runs.go internal/flows/service_mission.go internal/flows/service_start_failures.go internal/flows/service_start_failures_test.go && git commit -F C:/disposable/easydrag-s0c/a1-1-message.txt && git log -1 --format=%h
```

---

### Task A1.2: `internal/server`, the bridge reports a start failure

**Files:**
- Modify: `WT/internal/server/flows_bridge.go:294` (new method after `FlowRunFinished`) and the interface assertion block at `:133-135`
- Modify: `WT/internal/flows/AGENTS.md` (one bullet after line 1071)
- Create: `WT/internal/server/flows_start_failure_test.go`
- Test: `TestFlowStartFailureIsRecordedOncePerStreak`

Read first, in full: `WT/internal/server/flows_bridge.go:243-313` (`FlowRunFinished`, `noteRunsPaused`) and `WT/internal/server/flows_bridge_notify.go:44-133` (`flowIssue`, `notifyFlowFailure`). The rule that governs this task: the bridge method may call neither a Service method that takes a flow lock nor `mm.FlowRunFinishedAtDepth` (no running slot was taken for this run). It is safe to do planner and notification work here, because the Service calls it from its own goroutine, not from `DeleteFlow`.

**IMPACT:**

```bash
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "FlowRunFinished" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "flowMissionBridge" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
```

Expected: this task adds a method and changes no existing one, so the blast radius is zero for existing callers. Report the output as it is.

- [ ] **Step 1: Write the failing test.** Create `WT/internal/server/flows_start_failure_test.go`:

```go
package server

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/flows"
	"aurago/internal/planner"
)

// A flow mission whose flow is not published fires its trigger twice. The first start failure
// is recorded as one failed run (FLOW_START_FAILED) with one desktop notification and one
// planner issue; the second belongs to the same streak and records nothing.
func TestFlowStartFailureIsRecordedOncePerStreak(t *testing.T) {
	allowMissionMutationsForTest(t)
	c15LoadI18n()
	s := c16StartedServer(t, nil)
	rec := createTestFlow(t, s, waitFlowJSON) // a draft; it is never published
	plannerDB, err := planner.InitDB(filepath.Join(t.TempDir(), "planner.db"))
	if err != nil {
		t.Fatalf("planner db: %v", err)
	}
	t.Cleanup(func() { _ = plannerDB.Close() })
	hub := desktop.NewHub(4)
	events, cancel, err := hub.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cancel)
	s.PlannerDB, s.DesktopHub = plannerDB, hub

	drain := func() (notifications, changed int) {
		for {
			select {
			case ev := <-events:
				switch ev.Type {
				case "notification":
					notifications++
				case "flows_changed":
					changed++
				}
			default:
				return
			}
		}
	}
	issues := func() []planner.OperationalIssueListItem {
		page, err := planner.ListOperationalIssues(s.PlannerDB, planner.OperationalIssueListFilter{Status: "all", Limit: 50})
		if err != nil {
			t.Fatal(err)
		}
		return page.Items
	}
	failedRuns := func() []flows.RunRecord {
		runs, err := s.Flows.Runs(context.Background(), rec.ID, flows.RunFilter{Limit: 50})
		if err != nil {
			t.Fatal(err)
		}
		var out []flows.RunRecord
		for _, r := range runs {
			if r.ErrorCode == flows.StartFailedCode {
				out = append(out, r)
			}
		}
		return out
	}
	hooks := flowMissionHooks{s: s}
	fire := func() {
		t.Helper()
		if _, err := hooks.StartFlowRunReported(rec.MissionID, "", "webhook", ""); !errors.Is(err, flows.ErrNotPublished) {
			t.Fatalf("a trigger of an unpublished flow: err = %v, want ErrNotPublished", err)
		}
	}

	fire()
	// The record and the report run on the Service's goroutine: wait for the planner issue,
	// the last thing FlowStartFailed does before the notification.
	deadline := time.Now().Add(5 * time.Second)
	for len(issues()) == 0 || len(failedRuns()) == 0 {
		if time.Now().After(deadline) {
			t.Fatalf("the start failure was not recorded: %d runs, %d issues", len(failedRuns()), len(issues()))
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Shutdown waits for every record goroutine, so what is counted next is complete.
	fire()
	if err := s.Flows.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	runs := failedRuns()
	if len(runs) != 1 || runs[0].Status != flows.RunError || runs[0].Mode != flows.ModeLive || runs[0].Revision != 0 {
		t.Fatalf("failed runs after two fires = %+v, want exactly one", runs)
	}
	if got := issues(); len(got) != 1 || got[0].Occurrences != 1 {
		t.Fatalf("planner issues after two fires = %+v, want one issue with one occurrence", got)
	}
	if notifications, changed := drain(); notifications != 1 || changed != 1 {
		t.Fatalf("desktop events: %d notifications and %d flows_changed, want 1 and 1", notifications, changed)
	}
}
```

- [ ] **Step 2: Run it and see it fail for the stated reason.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && go test ./internal/server -run 'TestFlowStartFailureIsRecordedOncePerStreak' -count=1 2>&1 | tail -8
```

Expected: it compiles (A1.1 is in) and fails with `the start failure was not recorded: 1 runs, 0 issues`: the run is recorded by the Service, but the bridge has no `FlowStartFailed`, so no planner issue and no notification exist.

- [ ] **Step 3: Implement.** In `WT/internal/server/flows_bridge.go`, extend the assertion block (line 133-135):

```go
var _ flows.MissionReconciler = flowMissionBridge{}
var _ flows.RunGate = flowMissionBridge{}
var _ flows.LiveIssuesNoter = flowMissionBridge{}
var _ flows.StartFailureReporter = flowMissionBridge{}
```

Insert after `FlowRunFinished` (after line 294, before `noteRunsPaused`):

```go
// FlowStartFailed implements flows.StartFailureReporter: a trigger start that failed and was
// recorded as a failed run (flows.StartFailedCode). It runs on the Service's own goroutine,
// once per failure streak, so the planner and notification work here is safe. It gives the
// failure the same follow-up as a failed run (planner issue, failure notification by
// settings.notify_on_error, open editors refresh) but none of what a started run owns:
// FlowRunStarted was never called, so mm.FlowRunFinishedAtDepth must not run (it would release a
// running slot that no run took and count a run).
func (b flowMissionBridge) FlowStartFailed(info flows.RunFinishedInfo) {
	_, message := flowOutcome(info, flowRunOutputs{})
	b.s.flowIssue(info, true, message)
	b.s.notifyFlowFailure(info, message)
	b.s.broadcastFlowsChanged(info.Record.FlowID, "run_finished")
}
```

`flowOutcome` (`flows_bridge_output.go:145`) turns an error result into "FLOW_START_FAILED: the flow is not published", scrubbed and bounded, the same text a failed run shows. `flowRunOutputs{}` is the zero value (no outputs); verify the type name with `grep -n "type flowRunOutputs" internal/server/flows_bridge_output.go` and use the real name if it differs.

- [ ] **Step 4: Document it for the next reader.** In `WT/internal/flows/AGENTS.md`, after the bullet that ends with "(`MissionManagerV2.NoteFlowRunRefused`, reason re-asked from `automaticFlowRunsAllowed`)." (line 1071), add:

```markdown
  A trigger start that fails for a reason of the flow is recorded by the Service as a failed live run
  (`TriggerFromMission` → `afterTriggerStart` → `recordStartFailure`, code `FLOW_START_FAILED`, no steps, one
  record per failure streak, kept in `Service.startStreaks`) and reported through the optional
  `StartFailureReporter`. The bridge's `FlowStartFailed` gives it a planner issue, the failure notification and a
  `run_finished` broadcast, and never calls `FlowRunFinishedAtDepth` (no running slot was taken). Refusals
  (closed `RunGate`, `ErrRunnerClosed`, `ErrNotFound`, `ErrQueueFull`, a switched-off flow) record nothing. Date/Time
  timers (`onTimerFired`) count like mission triggers; Run now and test runs answer their caller and do not record.
```

- [ ] **Step 5: Run the test and see it pass.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && go test ./internal/server -run 'TestFlowStartFailureIsRecordedOncePerStreak' -count=1 -v 2>&1 | grep -E "^(--- |PASS|FAIL|ok)"
```

Expected: `--- PASS: TestFlowStartFailureIsRecordedOncePerStreak` and `ok`. If the notification count is 0, check `notifyFlowFailure`'s `s.ConfigSnapshot()` is non-nil in this fixture (it is for `c16StartedServer`); if it is 2, the second fire recorded again: the streak map is not working in the Service.

- [ ] **Step 6: Run the server flow tests.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && go test ./internal/server -run '(?i)flow' -count=1 2>&1 | tail -8
```

Expected: `ok`.

- [ ] **Step 7: Gates.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && go vet ./internal/server/ && gofmt -l internal/server/flows_bridge.go internal/server/flows_start_failure_test.go
```

Expected: no output.

- [ ] **Step 8: Commit.** Write `C:/disposable/easydrag-s0c/a1-2-message.txt`:

```
feat(server): report a trigger start failure to the planner and the desktop

flowMissionBridge implements flows.StartFailureReporter: a start failure
that the flow service recorded as a failed run gets a planner issue, the
failure notification chosen by notify_on_error and a run_finished
broadcast. It does not call FlowRunFinishedAtDepth, because no running
slot was taken. Two fires of the trigger of an unpublished flow record
one run, one notification and one issue.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <your model line>
```

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && git add internal/server/flows_bridge.go internal/server/flows_start_failure_test.go internal/flows/AGENTS.md && git commit -F C:/disposable/easydrag-s0c/a1-2-message.txt && git log -1 --format=%h
```

---

### Task A1.3: the mission side and the locales

Mission Control learns about the failure (`internal/tools`), the server marks the error for it, and the client can translate the code (`ui/lang/easydrag`).

**Files:**
- Modify: `WT/internal/tools/missions_v2_flows.go:125-162` (new sentinel and error type), `:134-149` (`startFlowRunReported`), `:867-918` (`startFlowRun`, `flowRefusal`, `flowRefusalOutput`), `:920-947` (new output constants, skip note), `:968-972` (`NoteFlowRunRefused` comment)
- Modify: `WT/internal/tools/missions_v2_planner.go:133-163` (`processPlannerFlowAdmissions`)
- Modify: `WT/internal/server/flows_bridge.go:362-375` (`StartFlowRunReported`)
- Modify: `WT/ui/lang/easydrag/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json` (one key each)
- Modify: `WT/ui/desktop_easydrag_test.go:264` (member of the `easydrag.ui.error_` family)
- Create: `WT/internal/tools/missions_v2_flows_start_failed_test.go`
- Modify: `WT/internal/tools/missions_v2_flows_paused_test.go:129-131` (one planner table row)
- Test: the tests named in the steps below

**IMPACT (list the callers and report them):**

```bash
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "startFlowRun" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "startFlowRunReported" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "flowRefusal" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "processPlannerFlowAdmissions" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "StartFlowRunReported" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo
```

Callers expected for `startFlowRun` (confirm each with `grep -rn "m.startFlowRun(" internal/tools --include=*.go`): `fireFlowEvent` (`missions_v2_flows.go:811`), `fireFlowSchedule` (`:847`), `dispatchFlowEvents` (`:1154`). The planner does not call `startFlowRun`; it calls `hooks.StartFlowRun` itself (`missions_v2_planner.go:141`) and is changed in the same way. `startFlowRunReported` callers: `TriggerMissionWithOptions` (`missions_v2.go:1360`) and `RunNow` (`:2034`). If the tool reports UNKNOWN for any of them, confirm with the `grep` above. Warn if the result is HIGH or CRITICAL; the change is additive (the sentinel is new, `flowRefusal` gains one case).

Hand-over to a user is unchanged: `RunNow` and the trigger API still return the error to their caller (`startFlowRunReported` returns it), so a user who presses Run on an unpublished flow sees the error and also gets one recorded failed run (Mission Control's own Run goes through `TriggerFromMission`, as the spec's "gated live start" does).

#### Part 1: the sentinel, the note and the skip note

- [ ] **Step 1: Write the failing tests.** Create `WT/internal/tools/missions_v2_flows_start_failed_test.go`:

```go
package tools

import (
	"errors"
	"testing"
)

// A trigger whose start failed for a reason of the flow (the service recorded a failed run) is
// noted on the mission like a refusal, once per change, and the next run result replaces it.
func TestFlowTriggerStartFailureNotesTheMission(t *testing.T) {
	mm, hooks, webhooks := newFlowTestManager(t)
	id := publishTestFlow(t, mm, pausedWebhookSpec())

	hooks.setErr(&FlowStartFailedError{Err: errors.New("the flow has not been published yet")})
	webhooks.Fire("hook-1", []byte(`{}`))
	webhooks.Fire("hook-1", []byte(`{}`))
	hooks.waitStart(t)
	hooks.waitStart(t)
	eventually(t, "the start-failed note", func() bool {
		m, _ := mm.Get(id)
		return m.LastResult == MissionResultError && m.LastOutput == flowStartFailedOutput && m.RunCount == 0
	})

	hooks.setErr(nil)
	webhooks.Fire("hook-1", []byte(`{}`))
	hooks.waitStart(t)
	mm.FlowRunFinished(id, mm.FlowRunStarted(id, "webhook", ""), MissionResultSuccess, "alles gut", nil)
	if m, _ := mm.Get(id); m.LastResult != MissionResultSuccess || m.LastOutput != "alles gut" {
		t.Fatalf("flow mission after the next run: %+v", m)
	}
}

func TestFlowStartFailedErrorKeepsTheCauseAndItsText(t *testing.T) {
	cause := errors.New("the flow has not been published yet")
	err := error(&FlowStartFailedError{Err: cause})
	if !errors.Is(err, ErrFlowStartFailed) || !errors.Is(err, cause) || err.Error() != cause.Error() {
		t.Fatalf("FlowStartFailedError = %v", err)
	}
	if flowRefusal(err) != ErrFlowStartFailed || flowRefusalOutput(ErrFlowStartFailed) != flowStartFailedOutput {
		t.Fatalf("flowRefusal = %v", flowRefusal(err))
	}
}

// A skipped start (the flow's skip policy dropped it) gets a quiet note: the output only, not
// the result, and only when the mission does not show a skip already.
func TestFlowSkippedStartGetsAQuietNoteOnce(t *testing.T) {
	mm, hooks, webhooks := newFlowTestManager(t)
	reporter := &fb4ReporterHooks{fakeFlowHooks: hooks, outcome: FlowStartOutcome{Status: "skipped"}}
	mm.SetFlowHooks(reporter)
	id := publishTestFlow(t, mm, pausedWebhookSpec())
	mm.FlowRunFinished(id, mm.FlowRunStarted(id, "webhook", ""), MissionResultSuccess, "alles gut", nil)

	webhooks.Fire("hook-1", []byte(`{}`))
	hooks.waitStart(t)
	eventually(t, "the skip note", func() bool {
		m, _ := mm.Get(id)
		return m.LastOutput == flowStartSkippedOutput
	})
	if m, _ := mm.Get(id); m.LastResult != MissionResultSuccess {
		t.Fatalf("a skip must not change the result: %+v", m)
	}
	mm.mu.Lock()
	again := mm.noteFlowSkipLocked(id)
	mm.mu.Unlock()
	if again {
		t.Fatal("a second skip changed the mission")
	}
}
```

Add one row to the planner table in `WT/internal/tools/missions_v2_flows_paused_test.go:129-131`:

```go
		{"paused", ErrFlowRunsPaused, "canceled", flowRunsPausedOutput, 0},
		{"unavailable and wrapped", fmt.Errorf("start: %w", ErrFlowsUnavailable), "canceled", flowsUnavailableOutput, 0},
		{"start failed", &FlowStartFailedError{Err: errors.New("the flow has not been published yet")}, "canceled", flowStartFailedOutput, 0},
		{"other error", errors.New("store down"), "unknown", unknownNote, 1},
```

- [ ] **Step 2: Run and see it fail to compile.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && GOTMPDIR='D:\tmp\go-build' go test ./internal/tools -run '(TestFlowTriggerStartFailureNotesTheMission|TestFlowStartFailedErrorKeepsTheCauseAndItsText|TestFlowSkippedStartGetsAQuietNoteOnce|TestPlannerFlowAdmissionRefusedOnPurposeIsCanceled)' -count=1 2>&1 | head -8
```

Expected: `undefined: FlowStartFailedError`, `undefined: ErrFlowStartFailed`, `undefined: flowStartFailedOutput`, `undefined: flowStartSkippedOutput`, `undefined: mm.noteFlowSkipLocked`.

- [ ] **Step 3: Add the sentinel and the error type.** In `WT/internal/tools/missions_v2_flows.go`, after `ErrFlowRunsPaused` (line 162):

```go
// ErrFlowStartFailed marks a flow start that failed for a reason of the flow (it is not
// published, its trigger is not an enabled one, the runner could not start the run). The flow
// service recorded the failure as a failed run (FLOW_START_FAILED) and reported it once per
// streak; Mission Control notes it on the flow mission like a refusal (flowRefusal) instead of
// only logging it. Wrap the cause with FlowStartFailedError.
var ErrFlowStartFailed = errors.New("the flow could not start")

// FlowStartFailedError wraps the cause of a start failure so errors.Is finds ErrFlowStartFailed
// and the cause alike, and keeps the cause's text, which the mission API shows its caller.
type FlowStartFailedError struct{ Err error }

func (e *FlowStartFailedError) Error() string        { return e.Err.Error() }
func (e *FlowStartFailedError) Unwrap() error        { return e.Err }
func (e *FlowStartFailedError) Is(target error) bool { return target == ErrFlowStartFailed }
```

- [ ] **Step 4: Report the outcome of every start, not only Mission Control's own.** Replace `startFlowRunReported` (lines 134-149) with a shared helper and the existing function on top of it:

Before:

```go
func startFlowRunReported(hooks FlowHooks, missionID, triggerType, triggerData string) error {
	reporter, ok := hooks.(FlowRunReporter)
	if !ok {
		return hooks.StartFlowRun(missionID, "", triggerType, triggerData)
	}
	out, err := reporter.StartFlowRunReported(missionID, "", triggerType, triggerData)
	if err != nil {
		return err
	}
	if out.Status == "skipped" {
		return &MissionTriggerSkippedError{Reason: FlowStartSkippedReason}
	}
	return nil
}
```

After:

```go
func startFlowRunReported(hooks FlowHooks, missionID, triggerType, triggerData string) error {
	out, err := startFlowHooks(hooks, missionID, "", triggerType, triggerData)
	if err != nil {
		return err
	}
	if out.Status == "skipped" {
		return &MissionTriggerSkippedError{Reason: FlowStartSkippedReason}
	}
	return nil
}

// startFlowHooks starts a flow run through hooks and reports what the start did: with a
// FlowRunReporter its outcome, else the plain StartFlowRun and an empty outcome.
func startFlowHooks(hooks FlowHooks, missionID, nodeID, triggerType, triggerData string) (FlowStartOutcome, error) {
	if reporter, ok := hooks.(FlowRunReporter); ok {
		return reporter.StartFlowRunReported(missionID, nodeID, triggerType, triggerData)
	}
	return FlowStartOutcome{}, hooks.StartFlowRun(missionID, nodeID, triggerType, triggerData)
}
```

- [ ] **Step 4b: `startFlowRun`, `flowRefusal`, `flowRefusalOutput` and the notes.** In the same file.

`startFlowRun` (lines 867-893). Before:

```go
func (m *MissionManagerV2) startFlowRun(hooks FlowHooks, missionID, nodeID, triggerType, data string) {
	var err error
	if hooks == nil {
		err = ErrFlowsUnavailable
	} else {
		err = hooks.StartFlowRun(missionID, nodeID, triggerType, data)
	}
	if err == nil {
		return
	}
```

After:

```go
func (m *MissionManagerV2) startFlowRun(hooks FlowHooks, missionID, nodeID, triggerType, data string) {
	var out FlowStartOutcome
	var err error
	if hooks == nil {
		err = ErrFlowsUnavailable
	} else {
		out, err = startFlowHooks(hooks, missionID, nodeID, triggerType, data)
	}
	if err == nil {
		if out.Status == "skipped" {
			m.mu.Lock()
			if m.noteFlowSkipLocked(missionID) {
				m.saveFlowNotesLocked()
			}
			m.mu.Unlock()
		}
		return
	}
```

(The rest of the function, from `reason := flowRefusal(err)`, stays.) Update its doc comment: "Without hooks, or when they refuse on purpose or report a start failure (flowRefusal), the mission shows why ... A start the flow skipped gets a quiet note (noteFlowSkipLocked)."

`flowRefusal` gets a fourth case, and its comment names it:

```go
// flowRefusal returns the sentinel behind a flow run that was refused on purpose, ErrFlowRunsPaused,
// ErrFlowsUnavailable or ErrFlowQueueFull, or that failed to start for a reason of the flow
// (ErrFlowStartFailed), and nil for every other error.
func flowRefusal(err error) error {
	switch {
	case errors.Is(err, ErrFlowRunsPaused):
		return ErrFlowRunsPaused
	case errors.Is(err, ErrFlowsUnavailable):
		return ErrFlowsUnavailable
	case errors.Is(err, ErrFlowQueueFull):
		return ErrFlowQueueFull
	case errors.Is(err, ErrFlowStartFailed):
		return ErrFlowStartFailed
	}
	return nil
}
```

`flowRefusalOutput` gets a case:

```go
	case ErrFlowQueueFull:
		return flowQueueFullOutput
	case ErrFlowStartFailed:
		return flowStartFailedOutput
	}
```

New constants after `flowRunsPausedOutput` (line 930), and the quiet skip note after `noteFlowRefusalLocked` (line 947):

```go
// flowStartFailedOutput is the LastOutput of a flow mission whose trigger fired but could not
// start a run for a reason of the flow (ErrFlowStartFailed). The reason is in the flow's run
// history (FLOW_START_FAILED); the German word of the note is "Start fehlgeschlagen".
const flowStartFailedOutput = "The EasyDrag flow could not start (see its run history); this trigger started no run"

// flowStartSkippedOutput is the LastOutput of a flow mission whose trigger the flow's skip policy
// dropped, because a run of the flow was still going (noteFlowSkipLocked).
const flowStartSkippedOutput = "A trigger was skipped because a run of this EasyDrag flow was still active"

// noteFlowSkipLocked shows on a flow mission that a trigger was skipped (the flow's skip policy).
// It is quiet: it sets the output only and leaves the result, the counters and the status alone,
// and only when the mission does not show a skip already, so a trigger that keeps firing while
// the flow runs rewrites nothing. The next run result replaces it. It reports whether the mission
// changed and does not save. Caller holds m.mu for writing.
func (m *MissionManagerV2) noteFlowSkipLocked(missionID string) bool {
	mission, ok := m.missions[missionID]
	if !ok || !isFlowMission(mission) || mission.LastOutput == flowStartSkippedOutput {
		return false
	}
	mission.LastOutput = flowStartSkippedOutput
	return true
}
```

`NoteFlowRunRefused`'s comment (line 968-971) says "reason is ErrFlowRunsPaused or ErrFlowsUnavailable"; add "(a refusal or ErrFlowStartFailed)" and leave the code unchanged.

- [ ] **Step 5: The planner.** In `WT/internal/tools/missions_v2_planner.go:133-163`, the dispatch closure takes the outcome and the accepted branch notes a skip.

Before:

```go
			hooks, id, node, data := m.flowHooks, target.MissionID, target.NodeID, event.Data
			m.mu.Unlock()
			err := func() (err error) {
				defer func() {
					if recover() != nil {
						err = fmt.Errorf("planner flow dispatch outcome unknown")
					}
				}()
				return hooks.StartFlowRun(id, node, string(TriggerPlannerAppointmentDue), data)
			}()
```

After:

```go
			hooks, id, node, data := m.flowHooks, target.MissionID, target.NodeID, event.Data
			m.mu.Unlock()
			var out FlowStartOutcome
			err := func() (err error) {
				defer func() {
					if recover() != nil {
						err = fmt.Errorf("planner flow dispatch outcome unknown")
					}
				}()
				out, err = startFlowHooks(hooks, id, node, string(TriggerPlannerAppointmentDue), data)
				return err
			}()
```

and the last branch:

Before:

```go
			} else {
				target.State = "accepted"
			}
```

After:

```go
			} else {
				target.State = "accepted"
				if out.Status == "skipped" && m.noteFlowSkipLocked(id) {
					m.saveFlowNotesLocked()
				}
			}
```

A start failure needs no planner change: `flowRefusal(err)` now returns `ErrFlowStartFailed`, so the target is settled as "canceled" with the note, like a refusal.

- [ ] **Step 6: Run the tests and see them pass.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && GOTMPDIR='D:\tmp\go-build' go test ./internal/tools -run '(TestFlowTriggerStartFailureNotesTheMission|TestFlowStartFailedErrorKeepsTheCauseAndItsText|TestFlowSkippedStartGetsAQuietNoteOnce|TestPlannerFlowAdmissionRefusedOnPurposeIsCanceled|TestFlowTriggerOtherStartErrorsStillWarnEveryTime|TestFB4)' -count=1 -v 2>&1 | grep -E "^(--- |PASS|FAIL|ok)"
```

Expected: every line `--- PASS` (the `TestFlowTriggerOtherStartErrorsStillWarnEveryTime` line proves an ordinary error such as "store down" still only warns), then `ok`.

- [ ] **Step 7: The whole flow slice of `internal/tools`.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && GOTMPDIR='D:\tmp\go-build' go test ./internal/tools -run '(?i)flow' -count=1 2>&1 | tail -6
```

Expected: `ok`.

#### Part 2: the server marks the error

- [ ] **Step 8: Write the failing assertion.** In `WT/internal/server/flows_start_failure_test.go` (A1.2's test), change `fire` so it also checks the mission marker:

```go
	fire := func() {
		t.Helper()
		_, err := hooks.StartFlowRunReported(rec.MissionID, "", "webhook", "")
		if !errors.Is(err, flows.ErrNotPublished) || !errors.Is(err, tools.ErrFlowStartFailed) {
			t.Fatalf("a trigger of an unpublished flow: err = %v, want ErrNotPublished marked as tools.ErrFlowStartFailed", err)
		}
	}
```

and add `"aurago/internal/tools"` to its imports. Also add, below the existing `TestFB4FlowStartErrorMarksAFullQueue` test in `WT/internal/server/flows_bridge_fb4_test.go`, nothing: a full queue must stay a full queue, and the existing test covers `flowStartError`.

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && go test ./internal/server -run 'TestFlowStartFailureIsRecordedOncePerStreak' -count=1 2>&1 | tail -6
```

Expected: FAIL with `want ErrNotPublished marked as tools.ErrFlowStartFailed`.

- [ ] **Step 9: Implement.** In `WT/internal/server/flows_bridge.go`, `StartFlowRunReported` (lines 369-372). Before:

```go
	res, err := h.s.Flows.TriggerFromMission(missionID, nodeID, triggerType, flows.NormalizeTriggerData(triggerType, triggerData))
	if err != nil {
		return tools.FlowStartOutcome{}, flowStartError(err)
	}
```

After:

```go
	res, err := h.s.Flows.TriggerFromMission(missionID, nodeID, triggerType, flows.NormalizeTriggerData(triggerType, triggerData))
	if err != nil {
		err = flowStartError(err)
		if flows.IsStartFailure(err) {
			// The Service recorded a failed run for it (once per streak); Mission Control notes it.
			err = &tools.FlowStartFailedError{Err: err}
		}
		return tools.FlowStartOutcome{}, err
	}
```

The doc comment above gets one sentence: "A start that fails for a reason of the flow (flows.IsStartFailure) is marked tools.ErrFlowStartFailed, so Mission Control notes it on the mission."

`flows.IsStartFailure` is false for `ErrQueueFull` (also when `flowStartError` wrapped it as `tools.ErrFlowQueueFull`), so a full queue stays a queue refusal.

- [ ] **Step 10: Run the server tests.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && go test ./internal/server -run '(TestFlowStartFailureIsRecordedOncePerStreak|TestFB4|(?i)flow)' -count=1 2>&1 | tail -6
```

Expected: `ok`.

#### Part 3: the locale key in 16 files

The key is `easydrag.ui.error_flow_start_failed`. The client builds it from the code (`easydrag-core.js:418-423`, `tr(t, 'easydrag.ui.error_' + code.toLowerCase(), code)`), so no JS changes. The locale files are not strictly sorted today (a plain sort reorders them), so the key is inserted right after `easydrag.ui.error_flow_shutdown`, where it belongs alphabetically (`sh` < `st`), and the rest of the file is not touched.

Texts (the term for "flow" follows each locale's `error_flow_not_published`; German uses lowercase "du" where a sentence addresses the reader, this one does not):

| File | Text |
|---|---|
| cs | Tok se nepodařilo spustit. |
| da | Flowet kunne ikke starte. |
| de | Der Flow konnte nicht starten. |
| el | Η ροή δεν μπόρεσε να ξεκινήσει. |
| en | The flow could not start. |
| es | El flujo no pudo iniciarse. |
| fr | Le flux n’a pas pu démarrer. |
| hi | फ़्लो शुरू नहीं हो सका। |
| it | Il flusso non è stato avviato. |
| ja | フローを開始できませんでした。 |
| nl | De flow kon niet starten. |
| no | Flyten kunne ikke starte. |
| pl | Nie udało się uruchomić przepływu. |
| pt | O fluxo não pôde ser iniciado. |
| sv | Flödet kunde inte starta. |
| zh | 流程无法启动。 |

- [ ] **Step 11: Write the failing check.** In `WT/ui/desktop_easydrag_test.go`, the last member line of the `easydrag.ui.error_` family (line 264) becomes:

```go
			"flow_mission_control_unavailable", "flow_runner_stopped", "flow_vault_unavailable", "flow_request_cancelled", "flow_start_failed"},
```

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragUIKeysExistInAllLocales' -count=1 2>&1 | tail -20
```

Expected: FAIL with sixteen lines `lang/easydrag/<lang>.json misses easydrag.ui.error_flow_start_failed`.

- [ ] **Step 12: Insert the key.** Write `C:/disposable/easydrag-s0c/a1-add-start-failed-key.mjs` with the Write tool (a script file in the disposable folder, run with the worktree as argument):

```js
// Inserts "easydrag.ui.error_flow_start_failed" after "easydrag.ui.error_flow_shutdown" in the
// 16 files of ui/lang/easydrag, keeping each file byte for byte otherwise.
import fs from 'node:fs';
import path from 'node:path';

const root = process.argv[2];
const KEY = 'easydrag.ui.error_flow_start_failed';
const AFTER = 'easydrag.ui.error_flow_shutdown';
const texts = {
    cs: 'Tok se nepodařilo spustit.',
    da: 'Flowet kunne ikke starte.',
    de: 'Der Flow konnte nicht starten.',
    el: 'Η ροή δεν μπόρεσε να ξεκινήσει.',
    en: 'The flow could not start.',
    es: 'El flujo no pudo iniciarse.',
    fr: 'Le flux n’a pas pu démarrer.',
    hi: 'फ़्लो शुरू नहीं हो सका।',
    it: 'Il flusso non è stato avviato.',
    ja: 'フローを開始できませんでした。',
    nl: 'De flow kon niet starten.',
    no: 'Flyten kunne ikke starte.',
    pl: 'Nie udało się uruchomić przepływu.',
    pt: 'O fluxo não pôde ser iniciado.',
    sv: 'Flödet kunde inte starta.',
    zh: '流程无法启动。',
};
let done = 0;
for (const [lang, text] of Object.entries(texts)) {
    const file = path.join(root, 'ui', 'lang', 'easydrag', lang + '.json');
    const src = fs.readFileSync(file, 'utf8');
    if (src.includes('"' + KEY + '"')) throw new Error(lang + ' has the key already');
    const eol = src.includes('\r\n') ? '\r\n' : '\n';
    const re = new RegExp('(\\n  "' + AFTER.replace(/\./g, '\\.') + '": .*,)\\r?\\n');
    if (!re.test(src)) throw new Error(lang + ': anchor line not found');
    const out = src.replace(re, (_, line) => line + eol + '  ' + JSON.stringify(KEY) + ': ' + JSON.stringify(text) + ',' + eol);
    JSON.parse(out); // still valid JSON
    fs.writeFileSync(file, out);
    done++;
}
console.log('inserted into ' + done + ' files');
```

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && node C:/disposable/easydrag-s0c/a1-add-start-failed-key.mjs . && git diff --stat -- ui/lang/easydrag | tail -3
```

Expected: `inserted into 16 files`, and a diff stat of 16 files with 1 insertion each (`16 files changed, 16 insertions(+)`). If a file has `\r\n` line ends the script keeps them. The script writes the text through `JSON.stringify`, which leaves the non-ASCII characters as they are (the locale files hold them as literal characters).

- [ ] **Step 13: Run the locale tests.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)(TestEasyDragUIKeysExistInAllLocales|easydrag.*lang|easydrag.*locale)' -count=1 2>&1 | tail -6
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -run 'TestCatalogTranslationsAreComplete' -count=1 2>&1 | tail -4
```

Expected: `ok` for both. (`TestCatalogTranslationsAreComplete` checks the catalog keys, not this one; it proves the 16 files still parse and agree on the keys it owns.)

- [ ] **Step 14: Run the German text check by eye.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && grep -h "error_flow_start_failed" ui/lang/easydrag/*.json | wc -l && grep -h "error_flow_start_failed" ui/lang/easydrag/de.json
```

Expected: `16`, then `  "easydrag.ui.error_flow_start_failed": "Der Flow konnte nicht starten.",`.

- [ ] **Step 15: Gates.**

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && GOTMPDIR='D:\tmp\go-build' go vet ./internal/tools/ ./ui/ && go vet ./internal/server/ && gofmt -l internal/tools/missions_v2_flows.go internal/tools/missions_v2_planner.go internal/tools/missions_v2_flows_start_failed_test.go internal/tools/missions_v2_flows_paused_test.go internal/server/flows_bridge.go internal/server/flows_start_failure_test.go ui/desktop_easydrag_test.go
```

Expected: no output.

- [ ] **Step 16: Commit.** Write `C:/disposable/easydrag-s0c/a1-3-message.txt`:

```
feat(missions): note a flow start failure and a skip on the mission

The server marks a trigger start failure that the flow service recorded
(flows.IsStartFailure) as tools.ErrFlowStartFailed. Mission Control notes
it on the flow mission like a refusal (new note "could not start"), and
the planner settles such a target as canceled instead of unknown.
startFlowRun and the planner use the reported start outcome; a skipped
start gets a quiet note, only when the mission did not show a skip
already. FLOW_START_FAILED gets its client text
(easydrag.ui.error_flow_start_failed) in all 16 EasyDrag locales.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: <your model line>
```

```bash
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-s0c-defects && git add internal/tools/missions_v2_flows.go internal/tools/missions_v2_planner.go internal/tools/missions_v2_flows_start_failed_test.go internal/tools/missions_v2_flows_paused_test.go internal/server/flows_bridge.go internal/server/flows_start_failure_test.go ui/desktop_easydrag_test.go ui/lang/easydrag/cs.json ui/lang/easydrag/da.json ui/lang/easydrag/de.json ui/lang/easydrag/el.json ui/lang/easydrag/en.json ui/lang/easydrag/es.json ui/lang/easydrag/fr.json ui/lang/easydrag/hi.json ui/lang/easydrag/it.json ui/lang/easydrag/ja.json ui/lang/easydrag/nl.json ui/lang/easydrag/no.json ui/lang/easydrag/pl.json ui/lang/easydrag/pt.json ui/lang/easydrag/sv.json ui/lang/easydrag/zh.json && git commit -F C:/disposable/easydrag-s0c/a1-3-message.txt && git log -1 --format=%h
```

- [ ] **Step 17: Hand the evidence to the coordinator (not into any repo file).** Per the S0c conventions no part edits the spec or the roadmap. The final report of this part carries: the three commit hashes; the test names `TestStartFailureStreak`, `TestFlowStartFailureIsRecordedOncePerStreak`, `TestFlowTriggerStartFailureNotesTheMission`, `TestFlowSkippedStartGetsAQuietNoteOnce`; the line "ledger row 6 (start outcome discarded, StartResult surfaced): fixed by A1, evidence `internal/flows/service_start_failures.go` `afterTriggerStart`, `TestStartFailureStreak`"; and the deviations below. The coordinator records them on main when the part lands.

---

## Coverage: spec item to task

| Spec §3.1 item | Task |
|---|---|
| Which failures count (counted list, refusals not counted, only gated live starts) | A1.1 (`startFailureReason`, `afterTriggerStart`; timers added by coordinator decision, deviation 9; Run now and test runs untouched) |
| 1. classify in `startLive`/`TriggerFromMission`, `recordStartFailure` | A1.1 |
| 2. one record per streak (map flow id to reason, success clears, delete clears) | A1.1 (`startStreaks`, `beginStartFailure`, `clearStartFailure`, `deleteLocked`) |
| 3. the record (live, revision or 0, trigger node and type, error, `FLOW_START_FAILED`, plain message cut and scrubbed, started = finished, no steps, no data) | A1.1 (`recordStartFailure`) |
| 4. the report (`StartFailureReporter`, `FlowStartFailed` with `flowIssue`, `notifyFlowFailure`, broadcast, no `FlowRunFinishedAtDepth`) | A1.1 (interface), A1.2 (bridge) |
| 5. off the caller's goroutine, bounded to one per flow per streak | A1.1 (goroutine, `startFailures` WaitGroup, Shutdown waits) |
| 6. mission side (`startFlowRun`, planner, "Start fehlgeschlagen" note, quiet skip note) | A1.3 Part 1 and 2 |
| 7. translations (`easydrag.ui.error_flow_start_failed`, 16 locales, note text) | A1.3 Part 3 (note text is an English constant, see deviation 3) |
| Test, `internal/flows` streak | A1.1 `TestStartFailureStreak` |
| Test, `internal/server` end to end | A1.2 `TestFlowStartFailureIsRecordedOncePerStreak` |
| §7 row 6 (ledger) | A1.3 Step 17 (report to the coordinator) |
| §11 "start-failure noise" (one-minute unpublished schedule) | A1.1 streak rule; the existing `flowFailureNotifier` bounds the notifications; the timer path counts too (deviation 9), so a one-minute unpublished schedule is one record per streak |

## Deviations from the spec

1. **No runner fake.** The spec's "with a runner fake" assumes an interface. `Service` holds the concrete `*Runner`, and no test fakes it. The streak test uses the real runner and real start errors (`ErrNoTrigger` from an unknown node id, `ErrNotPublished`, `ErrRunnerClosed` after `Shutdown`). `ErrNotFound` and the other non-counted errors are covered by the table test `classification` on `startFailureReason`, because a deleted flow cannot be produced deterministically between the flow read and the start.
2. **Gate refusal is told by asking the gate again, not by `RunsPausedCode`.** `RunsPausedCode` is only the code of a queued run the gate stopped (`runner.go:742`); a refusal before the start returns the bridge's raw error, a type `internal/flows` cannot name. `afterTriggerStart` therefore records nothing while `s.automaticRunsAllowed()` returns an error. The server's hook already refuses before it calls the Service, so this matters only for block and durable flows (`runner.go:469`) and for another bridge. The test closes the gate of `svcGateBridge`.
3. **The mission note is an English constant, not a translated key.** Mission notes (`flowRunsPausedOutput` and the others) are English strings stored as `LastOutput`; no `ui/lang` folder holds them, and the missions UI shows them as they are. The new note `flowStartFailedOutput` follows that, and its doc comment names the German word "Start fehlgeschlagen". Only the code `FLOW_START_FAILED` is translated (`easydrag.ui.error_flow_start_failed`). There is no code-member map entry other than the `easydrag.ui.error_` family list in `TestEasyDragUIKeysExistInAllLocales`, which A1.3 extends.
4. **Mission Control learns of the failure through a new wrapped error.** The spec says "noted like the refusals". The mission side cannot tell a start failure from any other error (`errors.New("store down")` must keep only warning, a test pins that). The server's hook therefore wraps counted failures as `tools.FlowStartFailedError` (which satisfies `errors.Is(err, tools.ErrFlowStartFailed)`, keeps the cause and its text), decided by the new exported `flows.IsStartFailure`. `flowRefusal` gets a fourth case; that makes the planner settle such a target as "canceled" with no further change.
5. **More errors are "not counted" than the spec lists.** In addition to the spec's list: `ErrQueueFull` (and `ErrRunnerQueueFull`; Mission Control already notes it as `ErrFlowQueueFull`), `ErrFlowDisabled`, `ErrFlowMissionMissing`, `ErrMissionControlUnavailable` (the admission refusals of a switched-off flow) and the caller's cancelled or expired context. They are refusals or not a fault of the flow, the same class as the spec's list.
6. **Reason of an "other" start error.** The spec says "the start error's own FLOW code". The Service has no error-to-code function (the API maps errors to codes in the server). The streak reason of such an error is `"start_error: "` plus the first 200 runes of its text, so a repeated identical error is one streak. The record's code is always `FLOW_START_FAILED`.
7. **`recordStartFailure` takes the reason too.** Signature `recordStartFailure(rec, nodeID, triggerType, reason, err)` instead of `(rec, nodeID, triggerType, err)`, because the caller already computed the reason for the streak.
8. **The failed run is inserted with status `error` directly** (`Store.CreateRun`, then `Store.FinishRun` for code, message and finish time), so it never looks like an active run between the two writes. A crash between them leaves an `error` row without a code, which is harmless.
9. **Timers count, because they have no caller to answer** (coordinator decision; the spec limits the scope to `TriggerFromMission`). `onTimerFired` calls `afterTriggerStart` for its unpublished-flow early return and for its `startLive` result, with the same classification and streak rules (trigger type `datetime`); its return values are unchanged. Mission Control's own Run (`RunNow` of a flow mission) goes through `TriggerFromMission`, so a user's Run on an unpublished flow now records one failed run besides returning the error to the user; that is inside the spec's "gated live start" wording.
10. **Quiet skip note.** The spec says "a quiet note only when the previous note was not a skip". `noteFlowSkipLocked` sets `LastOutput` only (result, counters and status stay) and compares with its own constant, so a trigger that keeps firing while the flow runs rewrites nothing; the next run result replaces the note.

## Spec points that need a decision

1. (Settled by the coordinator: Date/Time timers count, deviation 9. Part C's manual sentence about start failures should name schedules too.)
2. **Mission note wording and language.** The note is English like its siblings. If the owner wants "Start fehlgeschlagen" shown in German, the missions UI needs its own translation of `LastOutput` values, which none of the existing notes have. Recommendation: leave it English; the run history and the notification carry the translated text.
