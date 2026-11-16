## Part A: Server data (next run, "tested", change count)

Spec: `plans/2026-11-16-easydrag-w1c-spec.md` §1 decisions 1, 2 and 6, §3. Part A gives the editor three read-only facts about a flow and puts them into the flow API: `next_run` (when the flow starts by itself), `tested` (a test or simulation of exactly the current draft revision succeeded) and `changes` (steps and connections that differ from the live version). Mission Control's next run and the flow API answer through one function. No UI code changes except the shared browser fixture (A.6), which Parts B and D write against.

**Shell and worktree.** Git Bash. `W` below means `C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1c-server`, branch `feat/easydrag-w1c-server`, cut from main when the part starts (A.0). Every command starts with `cd W && …` (spelled out in full in A.0 only). Never edit, stage or commit in the main checkout. Go commands carry `GOTOOLCHAIN=go1.27.4`; `GOTMPDIR='D:\tmp\go-build'` for `internal/flows`, `internal/tools`, `cmd/...` and `ui`, none for `internal/server`. Tests with a cold build cache take a few minutes the first time (the `internal/server` package about 5; `internal/flows` about 2).

**Depends on.** Nothing (W1a and W1b are on main). Part A lands first: Parts B and D read its fixture names and the API fields.

**How the edits are made.** New files are written with the Write tool (full content below). Edits to existing files are *edit groups*: the blocks between the group's opening and closing comment markers in this file, each with the exact old text and the exact new text. A group is applied with

```
cd W && node C:/disposable/easydrag-w1c/apply-plan-edits.mjs plans/2026-11-16-easydrag-w1c/part-a-server.md <GROUP>
```

(add `--check` to only verify). Every old text must match exactly once or nothing is written; the file keeps its line endings. Groups of this part: `A1`, `A2`, `A2S`, `A3`, `A4`, `A5`, `A5F`, `A5D`. The coordinator commits this plan to `plans/2026-11-16-easydrag-w1c/` on main before A.0.

**IMPACT results** (GitNexus, run while planning against main d87930dfc; the index reports `status: behind`, so each task also greps the worktree):

| Symbol | Result | Callers |
|---|---|---|
| `MissionManagerV2.nextFlowRun` (tools) | LOW, 4 impacted, 1 direct | `MissionManagerV2.NextRun` only |
| `MissionManagerV2.NextRun` (tools) | LOW, 6 impacted | `mission_v2_handlers.go:71`, the Mission Control payloads. The name `NextRun` has 7 symbols in the index; `flows.Service.NextRun` is new |
| `flows.DiffFlows` | LOW, 6 impacted, 3 direct | `PublishPreview`, `BlockPublishPreview`, `flows_ai_proposal.go:78` |
| `Service.Simulate` | LOW, 2 impacted | `flows_handlers_runs.go:130`, `scenarios.go:645` (grep confirms exactly these two) |
| `Service.SimulateBlock` | **UNKNOWN** (no callers resolved) | grep: one caller, `flows_blocks_handlers.go:54`, plus tests |
| `Server.flowRoute` (the GET/PUT of a flow) | LOW, 2 impacted (the index also holds the fixture's JS `flowRoute`) | the flows router |
| `Server.flowBlockDocument` | LOW, 1 impacted | `flows_blocks.go:104` |
| `MissionManagerV2.SetFlowHooks` | LOW, 2 impacted | `server.go`, `flows_service.go` |

No HIGH or CRITICAL. The UNKNOWN one is resolved by grep (above): the change to `SimulateBlock` only adds a marker write after its result is built; its signature and result do not change.

**Every commit body ends with** `GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.`, a blank line, then `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Write each message file with the Write tool into `C:/disposable/easydrag-w1c/` and commit with `git commit -F`. One commit per task; never amend, rebase or push inside a task.

**Gates before each commit:** `go vet` of the touched packages, and `gofmt -l <the files the task touches>` printing nothing. (Do not run `gofmt -l` on a whole package: on main `internal/flows/durable_checkpoint_recovery.go`, `durable_checkpoint_state.go` and `services.go` are already not gofmt-clean and are not part of this work.)

### The design in one page

**Next run.**
- `flows.Service.NextRun(flowID) (NextRun, bool)` with `NextRun{At time.Time; NodeID string}` is the one answer. `ok` is false for an unknown flow, a block, a flow without a live version, a flow whose mission is switched off, and a flow without a time-based trigger that fires later.
- Two sources, the earliest fire **after now** wins (a tie goes to the smaller node id): the flow's Date/Time timers from the store (new `Store.NextTimerAfter`: it skips a stored `fire_at` that has passed, which `NextTimerAt` returns because it is a raw `MIN`), and the schedule triggers through `flows.ScheduleNexter`, which the server injects (`Service.SetScheduleNexter`), so the flows package never imports the cron code.
- It takes no flow lock and decodes no flow document (`Store.flowRunBinding` reads `mission_id`, `kind` and whether a live document exists). It asks `MissionBridge.FlowMissionEnabled`, which Mission Control answers under its read lock; that is safe because the manager never holds its lock while it calls the hooks (see `nextFlowRun`).
- Server side, `tools.MissionManagerV2.NextFlowScheduleRun(flowID, now)` is the `ScheduleNexter`: the earliest `cron.NextRun` of the flow mission's schedule jobs (only `flowCronSource` jobs; a prompt mission that owns a job with the same id does not count) and its node id.
- Mission Control: the flow hooks get an optional `tools.FlowNextRunner` (`SharedNextFlowRun(missionID)`); `nextFlowRun` uses it when the hooks have it, so Mission Control and the flow API are one function. Hooks without it (every test double in `internal/tools`) keep the old combination (`flowScheduleRun` plus `FlowHooks.NextFlowRun`), so no existing test changes.

**Tested.** Verified in A.3 step 1: test runs are stored with the draft revision and their status (`flow_runs.revision`, `mode = 'test'`, `status`); simulations (flow, scenario and block) are not stored, and retention prunes old test runs. The marker must be durable, so a successful simulation and a test run that ends with success (`Service.onRunFinished`, which today returns at once for test runs) both write the revision they ran into `flows_meta` (the existing key/value table) under `tested:<flow id>`; only a higher revision replaces the value. **No schema migration.** `IsTested(ctx, flowID, draftRevision)` is true when the stored marker equals that exact revision, or a successful stored test run of that revision exists (kept as a second source: it covers the instant between the run's row being saved and the hook writing the marker, and runs from before this code). Blocks have no test runs (only `StartTestRun` of a flow); their marker comes from `SimulateBlock`. Every save bumps the draft revision, so a save makes the flow untested.

**Change count.** `flows.CountChanges(live, draft) (ChangeCount{Steps, Connections}, ok)`; `ok` is false without a live version. It shares `DiffFlows`' counting (`DiffFlows` is split into `countDiff`, the counts, and the per-step details), so it equals the publish preview's counts by construction.

**API.** Top-level fields, next to `flow`, `enabled`, `issues` (the flow record and the list endpoint are unchanged):
- `GET /api/desktop/flows/{id}`: `tested`, `changes` (flows with a live version), `next_run` (`{"at": RFC 3339 in the server's location, "node": id}`, only when there is one).
- The save answer of `PUT /api/desktop/flows/{id}`: `draft_revision`, `issues`, and the same three fields (`tested` is false by construction).
- Blocks (`/api/desktop/flow-blocks/{id}`, GET and PUT): `tested` and `changes`, never `next_run`.

---

### Task A.0: Preamble (run once, before A.1)

- [ ] **Step 0.1: verify main and that nothing of this part exists yet**

```
cd C:/Users/Andi/Documents/repo/AuraGo && git log --oneline -3 && git status --short | head -5
ls internal/flows/next_run.go internal/flows/tested.go internal/flows/change_count.go internal/tools/missions_v2_flow_next.go internal/server/flows_draft_facts.go 2>&1 | head -5
grep -n "^func (m \*MissionManagerV2) nextFlowRun\|^func DiffFlows\|^func (s \*Service) Simulate(" internal/tools/missions_v2_flow_runs.go internal/flows/service.go internal/flows/simulation.go
```
Expected: main at d87930dfc or later (other sessions' uncommitted edits in the main checkout are not yours); the `ls` reports `No such file or directory` five times; the grep finds the three functions once each. If one of the five files exists, another session started this part: stop and tell the coordinator. If a function moved, each task's edit group stops with `FAIL … no match`: re-read the site and tell the coordinator.

- [ ] **Step 0.2: create the worktree and the scratch directories**

```
cd C:/Users/Andi/Documents/repo/AuraGo && git worktree add .worktrees/easydrag-w1c-server -b feat/easydrag-w1c-server main && cd .worktrees/easydrag-w1c-server && git branch --show-current && git log --oneline -1 && ls plans/2026-11-16-easydrag-w1c/part-a-server.md
mkdir -p C:/disposable/easydrag-w1c D:/tmp/go-build
```
Expected: `feat/easydrag-w1c-server`, main's head commit, and the plan file listed. If the plan file is missing the coordinator has not committed the plan: stop.

- [ ] **Step 0.3: baseline of the packages this part changes**

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -run 'TestServiceNextTimerPerFlow|TestStoreNextTimerAtUsesThePrimaryKey|TestDiffFlows' -count=1
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./internal/tools -run 'TestNextRunForFlowMissions|TestFlowNextRunAndDeleteUseOnlyFlowCronJobs' -count=1
```
Expected: `ok  	aurago/internal/flows` and `ok  	aurago/internal/tools`. A FAIL here is not yours: report it before going on.

---

### Task A.1: `flows.Service.NextRun` and `flows.ScheduleNexter`

**Files:**
- Create: `internal/flows/next_run.go`, `internal/flows/store_next_run.go`
- Modify: `internal/flows/store_misc.go` (the `nextTimerSQL` constant, about line 160), `internal/flows/service.go` (the `Service` struct, about line 213)
- Test: `internal/flows/next_run_test.go` (new)

- [ ] **Step 1: check the facts the code relies on**

```
cd W && grep -n "func (s \*Store) NextTimerAt\|^const nextTimerSQL\|func (s \*Store) flowIDByMission\|func (s \*Service) now()" internal/flows/*.go
cd W && grep -n "func newSvcFixture\|func svcDateTimeFlow\|func svcWithin\|func testNodeID" internal/flows/*_test.go
```
Expected: each found once. `NextTimerAt` stays as it is (Mission Control's old path and its test use it); `NextRun` gets its own store method.

- [ ] **Step 2: write the failing test**

Create `internal/flows/next_run_test.go`:

```go
package flows

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// fakeNexter is a ScheduleNexter that answers one fixed fire and records the arguments.
type fakeNexter struct {
	at     time.Time
	node   string
	ok     bool
	flowID string
	now    time.Time
	calls  int
}

func (n *fakeNexter) NextScheduleRun(flowID string, now time.Time) (time.Time, string, bool) {
	n.calls++
	n.flowID, n.now = flowID, now
	return n.at, n.node, n.ok
}

// nextRunFixture publishes a flow with one Date/Time trigger at +24 h and switches it on.
func nextRunFixture(t *testing.T) (*svcFixture, *FlowRecord, time.Time) {
	t.Helper()
	fx := newSvcFixture(t)
	at := fx.clock.Now().Add(24 * time.Hour)
	rec := fx.published(t, "Zeit", svcDateTimeFlow("Zeit", 3, at.Format(time.RFC3339)))
	if err := fx.s.SetEnabled(context.Background(), rec.ID, true); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	return fx, rec, at
}

func TestNextRunDateTimeTrigger(t *testing.T) {
	fx, rec, at := nextRunFixture(t)
	got, ok := fx.s.NextRun(rec.ID)
	if !ok || !got.At.Equal(at) || got.NodeID != testNodeID(3) {
		t.Fatalf("NextRun = %+v %v, want %v at %s", got, ok, at, testNodeID(3))
	}
	byMission, ok := fx.s.NextRunForMission(rec.MissionID)
	if !ok || !byMission.At.Equal(got.At) || byMission.NodeID != got.NodeID {
		t.Fatalf("NextRunForMission = %+v %v, want %+v", byMission, ok, got)
	}
}

func TestNextRunScheduleThroughTheNexter(t *testing.T) {
	fx := newSvcFixture(t)
	b := newFlow("Takt")
	manual := b.node("start", TypeTriggerManual, nil)
	set := b.node("greet", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "x", "value": "1"}}})
	b.edge(manual, PortOut, set)
	rec := fx.published(t, "Takt", b.build())
	if err := fx.s.SetEnabled(context.Background(), rec.ID, true); err != nil {
		t.Fatal(err)
	}
	if got, ok := fx.s.NextRun(rec.ID); ok {
		t.Fatalf("NextRun without a time-based trigger = %+v", got)
	}
	fire := fx.clock.Now().Add(3 * time.Hour)
	nexter := &fakeNexter{at: fire, node: testNodeID(5), ok: true}
	fx.s.SetScheduleNexter(nexter)
	got, ok := fx.s.NextRun(rec.ID)
	if !ok || !got.At.Equal(fire) || got.NodeID != testNodeID(5) {
		t.Fatalf("NextRun = %+v %v, want %v at %s", got, ok, fire, testNodeID(5))
	}
	if nexter.flowID != rec.ID || !nexter.now.Equal(fx.clock.Now()) {
		t.Fatalf("the nexter was asked for %q at %v, want %q at %v", nexter.flowID, nexter.now, rec.ID, fx.clock.Now())
	}
	fx.s.SetScheduleNexter(nil)
	if got, ok := fx.s.NextRun(rec.ID); ok {
		t.Fatalf("NextRun after removing the nexter = %+v", got)
	}
}

func TestNextRunTheEarliestOfBothSourcesWins(t *testing.T) {
	fx, rec, at := nextRunFixture(t)
	earlier := fx.clock.Now().Add(2 * time.Hour)
	fx.s.SetScheduleNexter(&fakeNexter{at: earlier, node: testNodeID(5), ok: true})
	if got, ok := fx.s.NextRun(rec.ID); !ok || !got.At.Equal(earlier) || got.NodeID != testNodeID(5) {
		t.Fatalf("NextRun = %+v %v, want the schedule at %v", got, ok, earlier)
	}
	fx.s.SetScheduleNexter(&fakeNexter{at: at.Add(time.Hour), node: testNodeID(5), ok: true})
	if got, ok := fx.s.NextRun(rec.ID); !ok || !got.At.Equal(at) || got.NodeID != testNodeID(3) {
		t.Fatalf("NextRun = %+v %v, want the timer at %v", got, ok, at)
	}
}

func TestNextRunPausedFlowHasNone(t *testing.T) {
	fx, rec, _ := nextRunFixture(t)
	fx.s.SetScheduleNexter(&fakeNexter{at: fx.clock.Now().Add(time.Hour), node: testNodeID(5), ok: true})
	if err := fx.s.SetEnabled(context.Background(), rec.ID, false); err != nil {
		t.Fatal(err)
	}
	if got, ok := fx.s.NextRun(rec.ID); ok {
		t.Fatalf("NextRun of a paused flow = %+v", got)
	}
}

func TestNextRunUnpublishedFlowHasNone(t *testing.T) {
	fx := newSvcFixture(t)
	rec, err := fx.s.CreateFlow(context.Background(), CreateRequest{Name: "Entwurf"})
	if err != nil {
		t.Fatal(err)
	}
	nexter := &fakeNexter{at: fx.clock.Now().Add(time.Hour), node: testNodeID(5), ok: true}
	fx.s.SetScheduleNexter(nexter)
	if got, ok := fx.s.NextRun(rec.ID); ok {
		t.Fatalf("NextRun of an unpublished flow = %+v", got)
	}
	if _, ok := fx.s.NextRun("flow_unknown"); ok {
		t.Fatal("NextRun of an unknown flow answered")
	}
}

func TestNextRunSkipsAStoredFireInThePast(t *testing.T) {
	fx, rec, at := nextRunFixture(t)
	ctx := context.Background()
	past := fx.clock.Now().Add(-time.Hour)
	if err := fx.s.Store().ReplaceTimers(ctx, rec.ID, []TimerRecord{
		{NodeID: testNodeID(3), FireAt: past},
		{NodeID: testNodeID(4), FireAt: at},
	}); err != nil {
		t.Fatal(err)
	}
	if raw, ok, err := fx.s.Store().NextTimerAt(ctx, rec.ID); err != nil || !ok || !raw.Equal(past) {
		t.Fatalf("setup: the raw minimum is %v %v %v, want the past fire %v", raw, ok, err, past)
	}
	if got, ok := fx.s.NextRun(rec.ID); !ok || !got.At.Equal(at) || got.NodeID != testNodeID(4) {
		t.Fatalf("NextRun = %+v %v, want the later timer %v at %s", got, ok, at, testNodeID(4))
	}
	if err := fx.s.Store().ReplaceTimers(ctx, rec.ID, []TimerRecord{{NodeID: testNodeID(3), FireAt: past}}); err != nil {
		t.Fatal(err)
	}
	if got, ok := fx.s.NextRun(rec.ID); ok {
		t.Fatalf("NextRun with only a past fire = %+v", got)
	}
	fx.s.SetScheduleNexter(&fakeNexter{at: past, node: testNodeID(5), ok: true})
	if got, ok := fx.s.NextRun(rec.ID); ok {
		t.Fatalf("NextRun with a past schedule fire = %+v", got)
	}
}

func TestNextRunTakesNoFlowLock(t *testing.T) {
	fx, rec, at := nextRunFixture(t)
	unlock, err := fx.s.locks.lock(context.Background(), rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	err = svcWithin(t, 5*time.Second, "NextRun of a locked flow", func() error {
		if got, ok := fx.s.NextRun(rec.ID); !ok || !got.At.Equal(at) {
			return fmt.Errorf("NextRun = %+v %v, want %v", got, ok, at)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 3: run it and see it fail**

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./internal/flows 2>&1 | head -6
```
Expected: the package does not compile: `fx.s.NextRun undefined (type *Service has no field or method NextRun)`, `fx.s.NextRunForMission undefined …`, `fx.s.SetScheduleNexter undefined …`. (Go cannot run a test in a package that does not compile; that is the red step.)

- [ ] **Step 4: implement**

Create `internal/flows/store_next_run.go`:

```go
package flows

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// flowRunBindingSQL reads what NextRun needs of a flow row without decoding its documents.
const flowRunBindingSQL = `SELECT mission_id, kind, live_json <> '' FROM flows WHERE id = ?`

// flowRunBinding returns the mission of a flow and whether it has a live version. A flow
// that does not exist is ErrNotFound, a block ErrWrongFlowKind.
func (s *Store) flowRunBinding(ctx context.Context, flowID string) (missionID string, live bool, err error) {
	var kind string
	err = s.db.QueryRowContext(ctx, flowRunBindingSQL, flowID).Scan(&missionID, &kind, &live)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, ErrNotFound
	}
	if err != nil {
		return "", false, err
	}
	if kind != string(KindFlow) {
		return "", false, ErrWrongFlowKind
	}
	return missionID, live, nil
}

// nextTimerAfterSQL reads the earliest timer of one flow that fires after a given time.
// Stored times share one layout (timeLayout, UTC), so the text comparison is the time
// comparison; storedFireAtGlob keeps out rows that are not in that layout. flow_id is the
// first column of the table's primary key, so only that flow's rows are read.
const nextTimerAfterSQL = `SELECT node_id, fire_at FROM flow_timers WHERE flow_id = ? AND fire_at > ? AND fire_at GLOB ` +
	storedFireAtGlob + ` ORDER BY fire_at, node_id LIMIT 1`

// NextTimerAfter returns the earliest timer of the flow that fires after the given time; ok
// is false when there is none. A timer whose time has passed (in retry) is left out, unlike
// NextTimerAt, which returns the raw minimum.
func (s *Store) NextTimerAfter(ctx context.Context, flowID string, after time.Time) (TimerRecord, bool, error) {
	var node, fireAt string
	err := s.db.QueryRowContext(ctx, nextTimerAfterSQL, flowID, formatTime(after)).Scan(&node, &fireAt)
	if errors.Is(err, sql.ErrNoRows) {
		return TimerRecord{}, false, nil
	}
	if err != nil {
		return TimerRecord{}, false, err
	}
	at := parseTime(fireAt)
	if at.IsZero() {
		return TimerRecord{}, false, nil
	}
	return TimerRecord{FlowID: flowID, NodeID: node, FireAt: at}, true, nil
}
```

Create `internal/flows/next_run.go`:

```go
package flows

import (
	"context"
	"time"
)

// NextRun is the next time a flow starts by itself: the earliest Date/Time timer or schedule
// fire of its live version, and the trigger node that fires.
type NextRun struct {
	At     time.Time
	NodeID string
}

// ScheduleNexter tells when the schedule triggers of a flow fire next. The server implements it
// over the cron engine and injects it with Service.SetScheduleNexter, so this package does not
// import the cron code. NextScheduleRun returns the earliest fire after now among the flow's
// schedule jobs and the id of the schedule node that owns it; ok is false when there is none
// (no schedule trigger, or the engine has not started). It must not call into a Service method
// that takes a flow lock.
type ScheduleNexter interface {
	NextScheduleRun(flowID string, now time.Time) (time.Time, string, bool)
}

// nextRunBudget bounds the store reads of one NextRun call.
const nextRunBudget = 2 * time.Second

// SetScheduleNexter injects the schedule source of NextRun; nil removes it. Call it once while
// the server wires the service; it is safe to call at any time.
func (s *Service) SetScheduleNexter(n ScheduleNexter) {
	s.mu.Lock()
	s.nexter = n
	s.mu.Unlock()
}

func (s *Service) scheduleNexter() ScheduleNexter {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nexter
}

// NextRun returns the next time the flow starts by itself: the earliest fire after now of the
// live version's Date/Time timers and schedule triggers. ok is false for a flow that is
// unknown, is a block, has no live version, is switched off, or has no time-based trigger
// that fires later. A stored fire time that has passed (a timer in retry, or not yet
// processed) is not "next".
//
// It is read-only and takes no flow lock: one lookup of the flow row, the mission's switch
// (MissionBridge.FlowMissionEnabled), one indexed timer query and the ScheduleNexter. A
// MissionBridge may therefore call it synchronously. Mission Control's next run and the
// flow API both answer through it.
func (s *Service) NextRun(flowID string) (NextRun, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), nextRunBudget)
	defer cancel()
	return s.nextRun(ctx, flowID)
}

// NextRunForMission is NextRun for the flow behind a Mission Control mission; ok is false
// when no single flow holds the mission.
func (s *Service) NextRunForMission(missionID string) (NextRun, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), nextRunBudget)
	defer cancel()
	flowID, err := s.store.flowIDByMission(ctx, missionID)
	if err != nil {
		return NextRun{}, false
	}
	return s.nextRun(ctx, flowID)
}

func (s *Service) nextRun(ctx context.Context, flowID string) (NextRun, bool) {
	missionID, live, err := s.store.flowRunBinding(ctx, flowID)
	if err != nil || !live || missionID == "" || !s.bridge.FlowMissionEnabled(missionID) {
		return NextRun{}, false
	}
	now := s.now()
	var best NextRun
	found := false
	consider := func(at time.Time, node string) {
		if at.IsZero() || !at.After(now) {
			return
		}
		if !found || at.Before(best.At) || (at.Equal(best.At) && node < best.NodeID) {
			best, found = NextRun{At: at, NodeID: node}, true
		}
	}
	if tm, ok, err := s.store.NextTimerAfter(ctx, flowID, now); err == nil && ok {
		consider(tm.FireAt, tm.NodeID)
	}
	if n := s.scheduleNexter(); n != nil {
		if at, node, ok := n.NextScheduleRun(flowID, now); ok {
			consider(at, node)
		}
	}
	return best, found
}
```

Apply the group A1 (the glob constant is split out so both timer queries share it; the `Service` struct gets the injected nexter):

```
cd W && node C:/disposable/easydrag-w1c/apply-plan-edits.mjs plans/2026-11-16-easydrag-w1c/part-a-server.md A1
```
Expected: `ok   edit   internal/flows/store_misc.go …`, `ok   edit   internal/flows/service.go …`, `applied 2 edit(s) of group A1 in 2 file(s).`

<!-- edits:A1 -->

**Edit** `internal/flows/store_misc.go`

Old:

```text
const nextTimerSQL = `SELECT MIN(fire_at) FROM flow_timers WHERE flow_id = ? AND fire_at GLOB ` +
	`'[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z'`
```

New:

```text
const nextTimerSQL = `SELECT MIN(fire_at) FROM flow_timers WHERE flow_id = ? AND fire_at GLOB ` + storedFireAtGlob

// storedFireAtGlob is the GLOB pattern of the stored fire_at layout (timeLayout).
const storedFireAtGlob = `'[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z'`
```

**Edit** `internal/flows/service.go`

Old:

```text
	// Purge admission and preflight are counted under mu before touching the store.
	purging sync.WaitGroup
}
```

New:

```text
	// Purge admission and preflight are counted under mu before touching the store.
	purging sync.WaitGroup
	// nexter is the schedule source of NextRun (SetScheduleNexter); guarded by mu.
	nexter ScheduleNexter
}
```

<!-- /edits -->

- [ ] **Step 5: run the new tests and the timer tests**

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -run 'TestNextRun|TestServiceNextTimerPerFlow|TestStoreNextTimerAtUsesThePrimaryKey' -count=1 -v 2>&1 | grep -E "^(--- |ok|FAIL|PASS)"
```
Expected (times vary, the first of them is slow because the store opens a database):
```
--- PASS: TestNextRunDateTimeTrigger
--- PASS: TestNextRunScheduleThroughTheNexter
--- PASS: TestNextRunTheEarliestOfBothSourcesWins
--- PASS: TestNextRunPausedFlowHasNone
--- PASS: TestNextRunUnpublishedFlowHasNone
--- PASS: TestNextRunSkipsAStoredFireInThePast
--- PASS: TestNextRunTakesNoFlowLock
--- PASS: TestServiceNextTimerPerFlow
--- PASS: TestStoreNextTimerAtUsesThePrimaryKey
PASS
ok  	aurago/internal/flows
```

- [ ] **Step 6: gates**

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./internal/flows && gofmt -l internal/flows/next_run.go internal/flows/store_next_run.go internal/flows/next_run_test.go internal/flows/store_misc.go internal/flows/service.go
cd W && git diff --stat
```
Expected: no output from vet and gofmt; the diff names only `store_misc.go` and `service.go` (the new files are untracked).

- [ ] **Step 7: commit**

Write `C:/disposable/easydrag-w1c/msg-a1.txt` with the Write tool:

```
feat(flows): Service.NextRun, one answer to "when does this flow start"

NextRun returns the earliest fire after now of the live version's Date/Time
timers and schedule triggers, with the trigger node. The schedule half comes
through the new ScheduleNexter interface that the server injects, so this
package does not import the cron code. A flow that is unknown, a block,
unpublished or switched off has none, and a stored fire time that has passed
(a timer in retry) is not "next": the new Store.NextTimerAfter skips it where
NextTimerAt returns the raw minimum. Read-only: no flow lock, no document
decoded. NextRunForMission is the same for a Mission Control mission id.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
```
cd W && git add internal/flows/next_run.go internal/flows/store_next_run.go internal/flows/next_run_test.go internal/flows/store_misc.go internal/flows/service.go && git commit -F C:/disposable/easydrag-w1c/msg-a1.txt && git status --short | head -3
```
Expected: one commit, a clean status.

---

### Task A.2: Mission Control and the server use the same function

**Files:**
- Create: `internal/tools/missions_v2_flow_next.go`, `internal/server/flows_schedule_nexter.go`
- Modify: `internal/tools/missions_v2_flow_runs.go` (`nextFlowRun`, about lines 415-459), `internal/server/flows_bridge.go` (`flowMissionHooks.NextFlowRun`, about line 466), `internal/server/flows_service.go` (`SetFlowHooks` call, about line 96)
- Test: `internal/tools/missions_v2_flow_next_run_test.go` (new), `internal/server/flows_next_run_test.go` (new)

Package boundaries: `internal/tools` and `internal/flows` do not import each other (the server is the only package that knows both), so the shared function reaches Mission Control through an interface that `internal/tools` defines (`FlowNextRunner`) and the server implements.

- [ ] **Step 1: IMPACT and the facts**

```
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "nextFlowRun" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo 2>&1 | grep -E '"(risk|impactedCount)"' | head -3
cd W && grep -rn "nextFlowRun(\|NextFlowRun(" internal --include=*.go | grep -v _test.go
cd W && grep -rn "aurago/internal/flows\"" internal/tools/*.go | head -2; grep -rn "aurago/internal/tools\"" internal/flows/*.go | head -2
```
Expected: `LOW` and a count of 4 (or the index's current number; HIGH or CRITICAL would be a warning to the coordinator); `nextFlowRun` is called only from `NextRun` (`missions_v2.go:2454`) and defined in `missions_v2_flow_runs.go`; the hooks' `NextFlowRun` is defined in `flows_bridge.go` and the interface in `missions_v2_flows.go`; the last two greps print nothing (no import between the two packages, so no cycle can form).

- [ ] **Step 2: write the failing tools tests**

Create `internal/tools/missions_v2_flow_next_run_test.go`:

```go
package tools

import (
	"testing"
	"time"
)

// sharedFlowHooks is fakeFlowHooks plus the optional shared next-run function.
type sharedFlowHooks struct {
	*fakeFlowHooks
	shared time.Time
	asked  []string
}

func (h *sharedFlowHooks) SharedNextFlowRun(missionID string) (time.Time, bool) {
	h.asked = append(h.asked, missionID)
	return h.shared, !h.shared.IsZero()
}

func newFlowCronManager(t *testing.T) (*MissionManagerV2, *CronManager) {
	t.Helper()
	dir := tempSystemTaskDir(t)
	cronMgr := NewCronManager(dir)
	if err := cronMgr.Start(func(string) {}); err != nil {
		t.Fatalf("cron start: %v", err)
	}
	t.Cleanup(func() { _ = cronMgr.Close() })
	mm := NewMissionManagerV2(dir, cronMgr)
	t.Cleanup(mm.Stop)
	return mm, cronMgr
}

// NextFlowScheduleRun answers by flow id with the earliest schedule job and its node, and
// only for a fire after now.
func TestNextFlowScheduleRunByFlowID(t *testing.T) {
	mm, cronMgr := newFlowCronManager(t)
	mm.SetFlowHooks(newFakeFlowHooks())
	specs := []FlowTriggerSpec{
		{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerSchedule, Schedule: "0 0 1 1 *"},
		{NodeID: "n_bbbbbbbb", TriggerType: FlowTriggerSchedule, Schedule: "30 6 * * *"},
		{NodeID: "n_cccccccc", TriggerType: FlowTriggerDateTime},
	}
	id := publishTestFlow(t, mm, specs...)
	want, ok := cronMgr.NextRun(flowCronJobID(id, "n_bbbbbbbb"))
	if !ok {
		t.Fatal("the cron engine has no next run for the daily job")
	}
	at, node, ok := mm.NextFlowScheduleRun("flow_aaaaaaaaaa", time.Now())
	if !ok || !at.Equal(want) || node != "n_bbbbbbbb" {
		t.Fatalf("NextFlowScheduleRun = %v %q %v, want %v n_bbbbbbbb", at, node, ok, want)
	}
	if _, _, ok := mm.NextFlowScheduleRun("flow_aaaaaaaaaa", want.Add(time.Second)); ok {
		t.Fatal("a fire that is not after now was reported")
	}
	if _, _, ok := mm.NextFlowScheduleRun("flow_unknown", time.Now()); ok {
		t.Fatal("an unknown flow has a schedule")
	}
}

// With flow hooks that implement FlowNextRunner, Mission Control's next run IS the shared
// answer (no second computation); a disabled mission has none whatever the hooks say.
func TestNextRunOfAFlowMissionUsesTheSharedFunction(t *testing.T) {
	mm, cronMgr := newFlowCronManager(t)
	hooks := &sharedFlowHooks{fakeFlowHooks: newFakeFlowHooks()}
	mm.SetFlowHooks(hooks)
	id := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerSchedule, Schedule: "0 0 1 1 *"})
	if _, ok := cronMgr.NextRun(flowCronJobID(id, "n_aaaaaaaa")); !ok {
		t.Fatal("setup: the cron job has no next run")
	}
	hooks.shared = time.Date(2031, 3, 4, 5, 6, 0, 0, time.UTC)
	if next, ok := mm.NextRun(id); !ok || !next.Equal(hooks.shared) {
		t.Fatalf("NextRun = %v %v, want the shared answer %v", next, ok, hooks.shared)
	}
	if len(hooks.asked) != 1 || hooks.asked[0] != id {
		t.Fatalf("the shared function was asked for %v, want [%s]", hooks.asked, id)
	}
	hooks.shared = time.Time{}
	if next, ok := mm.NextRun(id); ok {
		t.Fatalf("NextRun = %v, want none when the shared function has none", next)
	}
	hooks.shared = time.Date(2031, 3, 4, 5, 6, 0, 0, time.UTC)
	if err := mm.SetFlowMissionEnabled(id, expectedFlowID(t, mm, id), false); err != nil {
		t.Fatal(err)
	}
	if next, ok := mm.NextRun(id); ok {
		t.Fatalf("a disabled flow has a next run %v", next)
	}
}
```

- [ ] **Step 3: run them and see them fail**

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./internal/tools 2>&1 | head -5
```
Expected: `mm.NextFlowScheduleRun undefined (type *MissionManagerV2 has no field or method NextFlowScheduleRun)` (the package does not compile).

- [ ] **Step 4: implement the tools side**

Create `internal/tools/missions_v2_flow_next.go`:

```go
package tools

import "time"

// FlowNextRunner is an optional extension of FlowHooks, found by type assertion. SharedNextFlowRun
// answers the next time the flow behind missionID starts by itself, over both sources (Date/Time
// timers and schedule jobs); in AuraGo it is flows.Service.NextRunForMission, the function the
// flow API also uses, so Mission Control and EasyDrag always show the same time. Without it
// nextFlowRun combines the schedule jobs with FlowHooks.NextFlowRun itself.
type FlowNextRunner interface {
	SharedNextFlowRun(missionID string) (time.Time, bool)
}

// flowScheduleRun returns the earliest next fire among the cron jobs of the schedule triggers in
// specs and the node id that owns it. Only flowCronSource jobs count: a prompt mission whose id
// is "<flow>__<node>" owns a job with the same id. cron may be nil (no engine: nothing).
func flowScheduleRun(cron *CronManager, missionID string, specs []FlowTriggerSpec) (time.Time, string, bool) {
	if cron == nil || len(specs) == 0 {
		return time.Time{}, "", false
	}
	flowJobs := make(map[string]bool, len(specs))
	for _, job := range cron.GetJobs() {
		if job.Source == flowCronSource {
			flowJobs[job.ID] = true
		}
	}
	var next time.Time
	var node string
	for _, spec := range specs {
		if spec.TriggerType != FlowTriggerSchedule {
			continue
		}
		jobID := flowCronJobID(missionID, spec.NodeID)
		if !flowJobs[jobID] {
			continue
		}
		at, ok := cron.NextRun(jobID)
		if !ok || at.IsZero() {
			continue
		}
		if next.IsZero() || at.Before(next) || (at.Equal(next) && spec.NodeID < node) {
			next, node = at, spec.NodeID
		}
	}
	return next, node, !next.IsZero()
}

// NextFlowScheduleRun implements flows.ScheduleNexter for the server: the earliest fire after now
// among the schedule jobs of the flow with this id, and the schedule node that owns it. It
// answers for the mission as registered, whatever its switch says (flows.Service.NextRun
// checks the switch). Like nextFlowRun it holds m.mu only while it reads the mission.
func (m *MissionManagerV2) NextFlowScheduleRun(flowID string, now time.Time) (time.Time, string, bool) {
	m.mu.RLock()
	var (
		missionID string
		specs     []FlowTriggerSpec
	)
	for id, mission := range m.missions {
		if isFlowMission(mission) && mission.FlowID == flowID {
			missionID = id
			specs = copyFlowTriggers(mission.FlowTriggers)
			break
		}
	}
	cron := m.cron
	m.mu.RUnlock()
	if missionID == "" {
		return time.Time{}, "", false
	}
	at, node, ok := flowScheduleRun(cron, missionID, specs)
	if !ok || !at.After(now) {
		return time.Time{}, "", false
	}
	return at, node, true
}
```

Apply the group A2 (`nextFlowRun` uses the shared hooks when they have them, and `flowScheduleRun` otherwise):

```
cd W && node C:/disposable/easydrag-w1c/apply-plan-edits.mjs plans/2026-11-16-easydrag-w1c/part-a-server.md A2
```
Expected: `ok   edit   internal/tools/missions_v2_flow_runs.go …` and `applied 1 edit(s) of group A2 in 1 file(s).`

<!-- edits:A2 -->

**Edit** `internal/tools/missions_v2_flow_runs.go`

Old:

```text
// nextFlowRun returns the earliest schedule or Date/Time trigger of an enabled flow
// mission. isFlow is false for every other mission. Only flowCronSource jobs count: a
// prompt mission whose id is "<flow>__<node>" owns a job with the same id.
func (m *MissionManagerV2) nextFlowRun(id string) (next time.Time, ok bool, isFlow bool) {
	m.mu.RLock()
	mission, exists := m.missions[id]
	if !exists || !isFlowMission(mission) {
		m.mu.RUnlock()
		return time.Time{}, false, false
	}
	enabled := mission.Enabled
	var jobs []string
	for _, spec := range mission.FlowTriggers {
		if spec.TriggerType == FlowTriggerSchedule {
			jobs = append(jobs, flowCronJobID(id, spec.NodeID))
		}
	}
	hooks, cron := m.flowHooks, m.cron
	m.mu.RUnlock()
	if !enabled {
		return time.Time{}, false, true
	}
	consider := func(t time.Time, found bool) {
		if found && !t.IsZero() && (next.IsZero() || t.Before(next)) {
			next = t
		}
	}
	if cron != nil && len(jobs) > 0 {
		flowJobs := make(map[string]bool, len(jobs))
		for _, job := range cron.GetJobs() {
			if job.Source == flowCronSource {
				flowJobs[job.ID] = true
			}
		}
		for _, job := range jobs {
			if flowJobs[job] {
				consider(cron.NextRun(job))
			}
		}
	}
	if hooks != nil {
		consider(hooks.NextFlowRun(id))
	}
	return next, !next.IsZero(), true
}
```

New:

```text
// nextFlowRun returns the earliest schedule or Date/Time trigger of an enabled flow
// mission. isFlow is false for every other mission. When the flow hooks implement
// FlowNextRunner (AuraGo's do) the answer is theirs, the shared function that the flow API
// also uses. Otherwise it combines the schedule jobs (flowScheduleRun) with
// FlowHooks.NextFlowRun.
func (m *MissionManagerV2) nextFlowRun(id string) (next time.Time, ok bool, isFlow bool) {
	m.mu.RLock()
	mission, exists := m.missions[id]
	if !exists || !isFlowMission(mission) {
		m.mu.RUnlock()
		return time.Time{}, false, false
	}
	enabled := mission.Enabled
	specs := copyFlowTriggers(mission.FlowTriggers)
	hooks, cron := m.flowHooks, m.cron
	m.mu.RUnlock()
	if !enabled {
		return time.Time{}, false, true
	}
	if shared, isShared := hooks.(FlowNextRunner); isShared {
		at, found := shared.SharedNextFlowRun(id)
		return at, found && !at.IsZero(), true
	}
	consider := func(t time.Time, found bool) {
		if found && !t.IsZero() && (next.IsZero() || t.Before(next)) {
			next = t
		}
	}
	at, _, found := flowScheduleRun(cron, id, specs)
	consider(at, found)
	if hooks != nil {
		consider(hooks.NextFlowRun(id))
	}
	return next, !next.IsZero(), true
}
```

<!-- /edits -->

- [ ] **Step 5: run the tools tests, new and old**

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./internal/tools -run 'TestNextFlowScheduleRunByFlowID|TestNextRunOfAFlowMissionUsesTheSharedFunction|TestFlowNextRunAndDeleteUseOnlyFlowCronJobs|TestNextRunForFlowMissions' -count=1 -v 2>&1 | grep -E "^(--- |ok|FAIL|PASS)"
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./internal/tools -run '(?i)(flow|mission)' -count=1 2>&1 | tail -3
```
Expected: four `--- PASS` lines and `ok  	aurago/internal/tools`; then `ok  	aurago/internal/tools	…s` (about a minute). The two existing tests (`TestFlowNextRunAndDeleteUseOnlyFlowCronJobs`, `TestNextRunForFlowMissions`) pass unchanged: they use hooks without `FlowNextRunner`, which keep the old combination.

- [ ] **Step 6: write the failing server test (Mission Control equals the flow's next run)**

Create `internal/server/flows_next_run_test.go`:

```go
package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/tools"
)

// newFlowsCronTestServer is newFlowsTestServer with a started cron engine, so schedule
// triggers register their jobs.
func newFlowsCronTestServer(t *testing.T) *Server {
	t.Helper()
	s, _, _ := testDesktopPermissionServer(t)
	dir := t.TempDir()
	s.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	s.Cfg.Flows.Enabled = true
	s.Cfg.Tools.Missions.Enabled = true
	s.Cfg.Directories.DataDir = dir
	s.Cfg.Directories.WorkspaceDir = filepath.Join(dir, "workspace")
	s.Cfg.Directories.ToolsDir = filepath.Join(dir, "tools")
	s.Cfg.Directories.SkillsDir = filepath.Join(dir, "skills")
	s.Cfg.SQLite.GameMakerPath = filepath.Join(dir, "game_maker.db")
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{SchedulerEnabled: true, MissionsEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)
	cronMgr := tools.NewCronManager(dir)
	if err := cronMgr.Start(func(string) {}); err != nil {
		t.Fatalf("cron start: %v", err)
	}
	t.Cleanup(func() { _ = cronMgr.Close() })
	s.CronManager = cronMgr
	s.MissionManagerV2 = tools.NewMissionManagerV2(dir, cronMgr)
	s.initFlows()
	if s.Flows == nil {
		t.Fatal("flows were not initialised")
	}
	if err := s.MissionManagerV2.Start(); err != nil {
		t.Fatalf("missions: %v", err)
	}
	t.Cleanup(s.MissionManagerV2.Stop)
	if err := flowsStart(context.Background(), s.Flows); err != nil {
		t.Fatalf("flows: %v", err)
	}
	s.afterFlowsStart(context.Background(), s.Flows, s.Logger)
	t.Cleanup(func() { s.shutdownFlows(context.Background()) })
	return s
}

// timeFlowJSON is a flow with an every-six-hours schedule trigger (node n_aaaaaaaa) and/or a
// Date/Time trigger at the given time (node n_cccccccc), both feeding one set step.
func timeFlowJSON(withSchedule bool, at time.Time) string {
	nodes := `{"id":"n_bbbbbbbb","key":"greet","type":"logic.set","type_version":1,"label":"Gruss","position":{"x":300,"y":0},"params":{"fields":[{"name":"x","value":"1"}]}}`
	edges := ""
	if withSchedule {
		nodes += `,{"id":"n_aaaaaaaa","key":"takt","type":"trigger.schedule","type_version":1,"label":"Takt","position":{"x":0,"y":0},"params":{"mode":"interval_hours","hours":6}}`
		edges += `{"id":"e_aaaaaaaa","source":{"node":"n_aaaaaaaa","port":"out"},"target":{"node":"n_bbbbbbbb","port":"in"}}`
	}
	if !at.IsZero() {
		nodes += fmt.Sprintf(`,{"id":"n_cccccccc","key":"wann","type":"trigger.datetime","type_version":1,"label":"Wann","position":{"x":0,"y":120},"params":{"at":%q}}`, at.Format(time.RFC3339))
		if edges != "" {
			edges += ","
		}
		edges += `{"id":"e_cccccccc","source":{"node":"n_cccccccc","port":"out"},"target":{"node":"n_bbbbbbbb","port":"in"}}`
	}
	return `{"schema":1,"name":"Zeit","nodes":[` + nodes + `],"edges":[` + edges + `]}`
}

// Mission Control's next run and the flow's NextRun are one function: they agree for a
// schedule, a Date/Time trigger and both, and both are empty while the flow is paused.
func TestMissionControlNextRunEqualsTheFlowNextRun(t *testing.T) {
	s := newFlowsCronTestServer(t)
	ctx := context.Background()
	at := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	cases := []struct {
		name     string
		schedule bool
		datetime time.Time
	}{
		{"schedule only", true, time.Time{}},
		{"date and time only", false, at},
		{"both", true, at},
	}
	for _, c := range cases {
		rec := createTestFlow(t, s, timeFlowJSON(c.schedule, c.datetime))
		if _, _, err := s.Flows.Publish(ctx, rec.ID, rec.DraftRevision); err != nil {
			t.Fatalf("%s: Publish: %v", c.name, err)
		}
		if got, ok := s.MissionManagerV2.NextRun(rec.MissionID); ok {
			t.Fatalf("%s: a flow that is switched off has a Mission Control next run %v", c.name, got)
		}
		if err := s.Flows.SetEnabled(ctx, rec.ID, true); err != nil {
			t.Fatalf("%s: SetEnabled: %v", c.name, err)
		}
		flowNext, ok := s.Flows.NextRun(rec.ID)
		if !ok {
			t.Fatalf("%s: NextRun has no next run", c.name)
		}
		missionNext, ok := s.MissionManagerV2.NextRun(rec.MissionID)
		if !ok || !missionNext.Equal(flowNext.At) {
			t.Fatalf("%s: Mission Control = %v %v, NextRun = %v", c.name, missionNext, ok, flowNext.At)
		}
		// The shared answer is the earlier of the two sources.
		var want time.Time
		wantNode := ""
		if c.schedule {
			job, ok := s.CronManager.NextRun("mission_" + rec.MissionID + "__n_aaaaaaaa")
			if !ok {
				t.Fatalf("%s: the cron engine has no job for the schedule trigger", c.name)
			}
			want, wantNode = job, "n_aaaaaaaa"
		}
		if !c.datetime.IsZero() && (want.IsZero() || c.datetime.Before(want)) {
			want, wantNode = c.datetime, "n_cccccccc"
		}
		if !flowNext.At.Equal(want) || flowNext.NodeID != wantNode {
			t.Fatalf("%s: NextRun = %v at %s, want %v at %s", c.name, flowNext.At, flowNext.NodeID, want, wantNode)
		}
		if err := s.Flows.SetEnabled(ctx, rec.ID, false); err != nil {
			t.Fatalf("%s: SetEnabled(false): %v", c.name, err)
		}
		if got, ok := s.Flows.NextRun(rec.ID); ok {
			t.Fatalf("%s: a paused flow has a next run %+v", c.name, got)
		}
		if got, ok := s.MissionManagerV2.NextRun(rec.MissionID); ok {
			t.Fatalf("%s: a paused flow has a Mission Control next run %v", c.name, got)
		}
	}
}
```

Run it before the server is wired:
```
cd W && GOTOOLCHAIN=go1.27.4 go test ./internal/server -run 'TestMissionControlNextRunEqualsTheFlowNextRun' -count=1 2>&1 | grep -E "^(--- |ok|FAIL|\s+flows_next_run_test)" | head -4
```
Expected: `--- FAIL: TestMissionControlNextRunEqualsTheFlowNextRun` with `schedule only: NextRun has no next run`: the flow service has no schedule source yet. (The first run compiles the `internal/server` package: minutes.)

- [ ] **Step 7: wire the server**

Create `internal/server/flows_schedule_nexter.go`:

```go
package server

import (
	"time"

	"aurago/internal/flows"
)

// flowScheduleNexter gives the flow service the schedule half of its next run: the cron
// engine's next fire of the flow's schedule triggers (flows.ScheduleNexter). The flows
// package does not import the cron code.
type flowScheduleNexter struct{ s *Server }

var _ flows.ScheduleNexter = flowScheduleNexter{}

// NextScheduleRun implements flows.ScheduleNexter.
func (n flowScheduleNexter) NextScheduleRun(flowID string, now time.Time) (time.Time, string, bool) {
	if n.s == nil || n.s.MissionManagerV2 == nil {
		return time.Time{}, "", false
	}
	return n.s.MissionManagerV2.NextFlowScheduleRun(flowID, now)
}
```

Apply the group A2S (the hooks implement `tools.FlowNextRunner`; `initFlows` injects the nexter):

```
cd W && node C:/disposable/easydrag-w1c/apply-plan-edits.mjs plans/2026-11-16-easydrag-w1c/part-a-server.md A2S
```
Expected: `ok   edit   internal/server/flows_bridge.go …`, `ok   edit   internal/server/flows_service.go …`, `applied 2 edit(s) of group A2S in 2 file(s).`

<!-- edits:A2S -->

**Edit** `internal/server/flows_bridge.go`

Old:

```text
// NextFlowRun implements tools.FlowHooks.
func (h flowMissionHooks) NextFlowRun(missionID string) (time.Time, bool) {
	if h.s.Flows == nil {
		return time.Time{}, false
	}
	return h.s.Flows.NextTimer(context.Background(), missionID)
}
```

New:

```text
// NextFlowRun implements tools.FlowHooks: the Date/Time timers only. Mission Control asks
// SharedNextFlowRun first, which covers the schedules too.
func (h flowMissionHooks) NextFlowRun(missionID string) (time.Time, bool) {
	if h.s.Flows == nil {
		return time.Time{}, false
	}
	return h.s.Flows.NextTimer(context.Background(), missionID)
}

var _ tools.FlowNextRunner = flowMissionHooks{}

// SharedNextFlowRun implements tools.FlowNextRunner: the next time the flow starts by itself,
// from flows.Service.NextRunForMission, the function behind "next_run" in the flow API. So
// Mission Control and EasyDrag show the same time.
func (h flowMissionHooks) SharedNextFlowRun(missionID string) (time.Time, bool) {
	if h.s.Flows == nil {
		return time.Time{}, false
	}
	next, ok := h.s.Flows.NextRunForMission(missionID)
	return next.At, ok
}
```

**Edit** `internal/server/flows_service.go`

Old:

```text
	s.MissionManagerV2.SetFlowHooks(flowMissionHooks{s: s})
}
```

New:

```text
	s.MissionManagerV2.SetFlowHooks(flowMissionHooks{s: s})
	svc.SetScheduleNexter(flowScheduleNexter{s: s})
}
```

<!-- /edits -->

- [ ] **Step 8: run the server tests**

```
cd W && GOTOOLCHAIN=go1.27.4 go test ./internal/server -run 'TestMissionControlNextRunEqualsTheFlowNextRun' -count=1 -v 2>&1 | grep -E "^(--- |ok|FAIL|PASS)"
cd W && GOTOOLCHAIN=go1.27.4 go test ./internal/server -run '(?i)flow' -count=1 2>&1 | tail -3
```
Expected: `--- PASS: TestMissionControlNextRunEqualsTheFlowNextRun`, `ok`; then `ok  	aurago/internal/server	…s` (about a minute). A deadlock or a timeout in the flow tests means `SharedNextFlowRun` is reached with the manager's lock held: `nextFlowRun` releases `m.mu` before it calls the hooks, check that the edit kept `m.mu.RUnlock()` before the `FlowNextRunner` branch.

- [ ] **Step 9: gates and commit**

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./internal/tools && GOTOOLCHAIN=go1.27.4 go vet ./internal/server && gofmt -l internal/tools/missions_v2_flow_next.go internal/tools/missions_v2_flow_next_run_test.go internal/tools/missions_v2_flow_runs.go internal/server/flows_schedule_nexter.go internal/server/flows_next_run_test.go internal/server/flows_bridge.go internal/server/flows_service.go
```
Expected: no output.

Write `C:/disposable/easydrag-w1c/msg-a2.txt`:

```
feat(flows): Mission Control's next run and the flow's next run are one function

The flow hooks implement the optional tools.FlowNextRunner, which answers with
flows.Service.NextRunForMission, so nextFlowRun asks it instead of combining
the cron jobs and the timers itself. The server injects the schedule half:
MissionManagerV2.NextFlowScheduleRun, keyed by flow id, is the
flows.ScheduleNexter. Hooks without FlowNextRunner (every test double in
internal/tools) keep the old combination, now through the shared
flowScheduleRun helper. A server test proves that Mission Control and NextRun
agree for a schedule, a Date/Time trigger and both, and are empty while the
flow is paused.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
```
cd W && git add internal/tools/missions_v2_flow_next.go internal/tools/missions_v2_flow_next_run_test.go internal/tools/missions_v2_flow_runs.go internal/server/flows_schedule_nexter.go internal/server/flows_next_run_test.go internal/server/flows_bridge.go internal/server/flows_service.go && git commit -F C:/disposable/easydrag-w1c/msg-a2.txt && git status --short | head -3
```
Expected: one commit, a clean status.

---

### Task A.3: The "tested" marker

**Files:**
- Create: `internal/flows/tested.go`
- Modify: `internal/flows/service_runs.go` (`Service.onRunFinished`, about line 542), `internal/flows/simulation.go` (`Simulate`, about line 116), `internal/flows/scenarios.go` (`simulateLoadedScenario`, lines 645 and 679), `internal/flows/blocks_simulation.go` (`SimulateBlock`, line 85), `internal/flows/store.go` (`Store.DeleteFlow`, about line 795), `internal/flows/blocks_service.go` (the block delete, about line 322)
- Test: `internal/flows/tested_test.go` (new)

- [ ] **Step 1: verify the storage finding (what is stored, what is not)**

```
cd W && grep -n "Revision: rec.DraftRevision, Mode: ModeTest" internal/flows/service_runs.go
cd W && grep -n "AND revision=?\|SELECT draft_json FROM flows WHERE id=? AND draft_revision=?" internal/flows/store_runs.go | head -3
cd W && grep -n "ModeSimulation" internal/flows/simulation.go internal/flows/blocks_simulation.go | grep -v "^.*//" | head -6
cd W && grep -n "createRun\|CreateRun\|INSERT INTO flow_runs" internal/flows/simulation.go internal/flows/blocks_simulation.go internal/flows/scenarios.go; echo "(no store call above means simulations leave no record)"
cd W && grep -rn "flows_meta" internal/flows/*.go | grep -v _test.go | grep -v "schema_version" | head -3; echo "(no other flows_meta key exists today)"
```
Expected, and what to do for each outcome:
- Line 1 finds `StartRun… Revision: rec.DraftRevision, Mode: ModeTest` in `StartTestRun`: **test runs carry the draft revision** (`flow_runs.revision`) and their status. Line 2 finds the stored-test-run document lookup by draft revision. If line 1 finds nothing, the revision of a test run is no longer the draft's: stop and report; the derivation below is wrong.
- Line 3 shows `ModeSimulation` only in the `RunRequest` and the result: **simulations are not stored**; line 4 prints no store call before the note. If a store call appears, a record exists after all: use it instead of `flows_meta`, and tell the coordinator.
- The `flows_meta` lines are the table's `CREATE` statement (`store.go`, `flows_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`) and the schema-version checks (`checkStoreSchema`, the v2 migration), which read only the row `schema_version`: the table is an existing key/value table that nothing else uses, so the marker is a row with key `tested:<flow id>` and **no migration is needed**. If a line shows another code path that writes or reads other keys, or a check that rejects unknown keys, stop here and put it under "Decisions needed".
- Retention: finished test runs are pruned (`DefaultMaxTestRunsPerFlow` = 20 per flow, 30 days), so the stored run cannot be the only evidence (the owner wants a durable marker). `Service.onRunFinished` is the hook the runner calls after a run is persisted, outside all locks. Check with `grep -n "if rec.Mode == ModeTest" -A3 internal/flows/service_runs.go` that it returns at once for test runs (the last edit of the group below replaces that); if it does not, stop and re-read it.

- [ ] **Step 2: write the failing tests**

Create `internal/flows/tested_test.go`:

```go
package flows

import (
	"context"
	"testing"
	"time"
)

// testedFlow is a manual trigger feeding a Stop step that ends the run with the given status.
func testedFlow(name, status string) *Flow {
	b := newFlow(name)
	start := b.node("start", TypeTriggerManual, nil)
	halt := b.node("halt", TypeStop, map[string]any{"status": status, "message": "Ende"})
	b.edge(start, PortOut, halt)
	return b.build()
}

func mustCreate(t *testing.T, s *Service, doc *Flow) *FlowRecord {
	t.Helper()
	rec, err := s.CreateFlow(context.Background(), CreateRequest{Import: doc})
	if err != nil {
		t.Fatalf("CreateFlow: %v", err)
	}
	return rec
}

func TestTestedAfterASuccessfulSimulation(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	rec := mustCreate(t, s, testedFlow("Ok", "success"))
	if s.IsTested(ctx, rec.ID, rec.DraftRevision) {
		t.Fatal("a new flow is tested")
	}
	res, err := s.Simulate(ctx, rec.ID, SimulationRequest{BaseRevision: rec.DraftRevision})
	if err != nil || res.Status != RunSuccess {
		t.Fatalf("Simulate = %+v, %v", res, err)
	}
	if !s.IsTested(ctx, rec.ID, rec.DraftRevision) {
		t.Fatal("a successful simulation did not mark the draft revision")
	}
	if s.IsTested(ctx, rec.ID, rec.DraftRevision+1) {
		t.Fatal("the next revision is tested before anything ran")
	}
	// A save makes a new revision, which is untested.
	rev, _, err := s.SaveDraft(ctx, rec.ID, testedFlow("Ok", "success"), rec.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	if s.IsTested(ctx, rec.ID, rev) {
		t.Fatal("a save left the flow tested")
	}
	// An older simulation finishing late does not erase the newer mark.
	if _, err := s.Simulate(ctx, rec.ID, SimulationRequest{BaseRevision: rev}); err != nil {
		t.Fatal(err)
	}
	if err := s.store.markSimulated(ctx, rec.ID, rec.DraftRevision); err != nil {
		t.Fatal(err)
	}
	if !s.IsTested(ctx, rec.ID, rev) {
		t.Fatal("an older revision's mark replaced the newer one")
	}
}

func TestTestedNotAfterAFailedSimulation(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	rec := mustCreate(t, s, testedFlow("Fehler", "error"))
	res, err := s.Simulate(ctx, rec.ID, SimulationRequest{BaseRevision: rec.DraftRevision})
	if err != nil || res.Status != RunError {
		t.Fatalf("Simulate = %+v, %v; want a failed simulation", res, err)
	}
	if s.IsTested(ctx, rec.ID, rec.DraftRevision) {
		t.Fatal("a failed simulation marked the draft revision")
	}
	if _, err := s.Simulate(ctx, rec.ID, SimulationRequest{BaseRevision: rec.DraftRevision + 7}); err == nil {
		t.Fatal("a simulation of a wrong revision answered")
	}
	if s.IsTested(ctx, rec.ID, rec.DraftRevision) || s.IsTested(ctx, rec.ID, rec.DraftRevision+7) {
		t.Fatal("a refused simulation marked a revision")
	}
}

func TestTestedFollowsTheStoredTestRuns(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	bad := mustCreate(t, s, testedFlow("Fehler", "error"))
	started, err := s.StartTestRun(ctx, bad.ID, TestRunRequest{BaseRevision: bad.DraftRevision})
	if err != nil {
		t.Fatal(err)
	}
	if run := waitRun(t, s, started.RunID); run.Status != RunError {
		t.Fatalf("test run status = %s, want error", run.Status)
	}
	if s.IsTested(ctx, bad.ID, bad.DraftRevision) {
		t.Fatal("a failed test run marked the draft revision")
	}
	good := mustCreate(t, s, testedFlow("Ok", "success"))
	started, err = s.StartTestRun(ctx, good.ID, TestRunRequest{BaseRevision: good.DraftRevision})
	if err != nil {
		t.Fatal(err)
	}
	if run := waitRun(t, s, started.RunID); run.Status != RunSuccess {
		t.Fatalf("test run status = %s, want success", run.Status)
	}
	if !s.IsTested(ctx, good.ID, good.DraftRevision) {
		t.Fatal("a successful test run did not mark the draft revision")
	}
	rev, _, err := s.SaveDraft(ctx, good.ID, testedFlow("Ok", "success"), good.DraftRevision)
	if err != nil {
		t.Fatal(err)
	}
	if s.IsTested(ctx, good.ID, rev) {
		t.Fatal("a save left the flow tested")
	}
}

func TestTestedAfterABlockSimulationAndItsDelete(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block, err := s.CreateBlock(ctx, "Leer")
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := block.Draft.Clone()
	delete(doc.Nodes[1].Params, "values")
	rev, issues, err := s.SaveBlockDraft(ctx, block.ID, doc, block.DraftRevision)
	if err != nil {
		t.Fatalf("save: %v %+v", err, issues)
	}
	if s.IsTested(ctx, block.ID, rev) {
		t.Fatal("a new block draft is tested")
	}
	if res, err := s.SimulateBlock(ctx, block.ID, BlockSimulationRequest{BaseRevision: rev}); err != nil || res.Status != RunSuccess {
		t.Fatalf("SimulateBlock = %+v, %v", res, err)
	}
	if !s.IsTested(ctx, block.ID, rev) {
		t.Fatal("a successful block simulation did not mark the draft revision")
	}
	if err := s.DeleteBlock(ctx, block.ID, rev, 0); err != nil {
		t.Fatalf("DeleteBlock: %v", err)
	}
	if _, ok, err := s.store.simulatedRevision(ctx, block.ID); err != nil || ok {
		t.Fatalf("the marker of a deleted block is still stored: %v %v", ok, err)
	}
}

// A test run that ended with success keeps its flow tested after retention prunes the run's
// row; a failed or cancelled test run writes no marker.
func TestTestedSurvivesPruningOfTheTestRun(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	good := mustCreate(t, s, testedFlow("Ok", "success"))
	started, err := s.StartTestRun(ctx, good.ID, TestRunRequest{BaseRevision: good.DraftRevision})
	if err != nil {
		t.Fatal(err)
	}
	if run := waitRun(t, s, started.RunID); run.Status != RunSuccess {
		t.Fatalf("test run status = %s, want success", run.Status)
	}
	// The hook runs just after the row is saved: wait for the marker.
	deadline := time.Now().Add(3 * time.Second)
	for {
		if revision, ok, err := s.store.simulatedRevision(ctx, good.ID); err == nil && ok && revision == good.DraftRevision {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("a successful test run wrote no marker")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := s.store.db.ExecContext(ctx, `DELETE FROM flow_runs WHERE flow_id = ?`, good.ID); err != nil {
		t.Fatal(err)
	}
	if !s.IsTested(ctx, good.ID, good.DraftRevision) {
		t.Fatal("pruning the test run's row made the flow untested")
	}

	// The hook itself: only success writes.
	bad := mustCreate(t, s, testedFlow("Fehler", "error"))
	for _, status := range []RunStatus{RunError, RunCancelled} {
		s.onRunFinished(RunRecord{FlowID: bad.ID, Mode: ModeTest, Revision: bad.DraftRevision}, RunResult{Status: status})
		if _, ok, err := s.store.simulatedRevision(ctx, bad.ID); err != nil || ok {
			t.Fatalf("a test run that ended with %s wrote a marker (%v, %v)", status, ok, err)
		}
	}
	s.onRunFinished(RunRecord{FlowID: bad.ID, Mode: ModeTest, Revision: bad.DraftRevision}, RunResult{Status: RunSuccess})
	if !s.IsTested(ctx, bad.ID, bad.DraftRevision) {
		t.Fatal("a successful test run result did not mark the revision")
	}
}
```

- [ ] **Step 3: run them and see them fail**

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./internal/flows 2>&1 | head -5
```
Expected: `s.IsTested undefined (type *Service has no field or method IsTested)` and `s.store.markSimulated undefined …` (the package does not compile).

- [ ] **Step 4: implement**

Create `internal/flows/tested.go`:

```go
package flows

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"
)

// The "tested" marker. A test run or a simulation of the draft that finishes successfully marks
// that draft revision as tested; any later save creates a new revision, which is untested.
//
//   - A simulation is not stored, and retention prunes old test runs. So a successful
//     simulation, and a test run that ends with success (Service.onRunFinished), write the
//     revision they ran into flows_meta under the key testedMetaKey(flowID): the existing
//     key/value table, so there is no schema change. Only a higher revision replaces the value.
//   - Test runs are also stored with the draft revision they ran (flow_runs.revision, mode
//     "test", status "success"); IsTested reads them as well, which covers the moment between
//     the run's row being saved and the hook writing the marker, and runs from before this code.
//
// A failed test or simulation, one that was cancelled, and a simulation that did not run
// (an error from Simulate) mark nothing.

// testedMetaKey is the flows_meta key that holds the last successfully simulated draft
// revision of a flow or block.
func testedMetaKey(flowID string) string { return "tested:" + flowID }

// markTestedSQL writes the revision only when it is higher than the stored one, so a slow
// simulation of an old revision cannot erase the mark of a newer one.
const markTestedSQL = `INSERT INTO flows_meta (key, value) VALUES (?, ?)
	ON CONFLICT(key) DO UPDATE SET value = excluded.value
	WHERE CAST(flows_meta.value AS INTEGER) < CAST(excluded.value AS INTEGER)`

func (s *Store) markSimulated(ctx context.Context, flowID string, revision int) error {
	_, err := s.db.ExecContext(ctx, markTestedSQL, testedMetaKey(flowID), strconv.Itoa(revision))
	return err
}

func (s *Store) simulatedRevision(ctx context.Context, flowID string) (int, bool, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM flows_meta WHERE key = ?`, testedMetaKey(flowID)).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	revision, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false, nil
	}
	return revision, true, nil
}

// hasSuccessfulTestRun reports whether a test run of exactly this draft revision finished
// with success. The query reads the flow's rows through idx_flow_runs_flow_started.
func (s *Store) hasSuccessfulTestRun(ctx context.Context, flowID string, revision int) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM flow_runs WHERE flow_id = ? AND mode = 'test' AND revision = ? AND status = 'success' LIMIT 1`,
		flowID, revision).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// markTested records a successful simulation of the draft revision. It must not fail the
// simulation: a store error is logged and the answer goes out unmarked.
func (s *Service) markTested(ctx context.Context, flowID string, revision int) {
	if revision <= 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	if err := s.store.markSimulated(ctx, flowID, revision); err != nil {
		s.logger.Warn("flow simulation could not be marked as tested", "flow", truncateForError(flowID), "revision", revision, "error", err)
	}
}

// IsTested reports whether a test run or a simulation of exactly this draft revision of the
// flow (or block) finished successfully. It is read-only and takes no flow lock. A store error
// counts as "not tested".
func (s *Service) IsTested(ctx context.Context, flowID string, draftRevision int) bool {
	if draftRevision <= 0 {
		return false
	}
	if revision, ok, err := s.store.simulatedRevision(ctx, flowID); err == nil && ok && revision == draftRevision {
		return true
	}
	ok, err := s.store.hasSuccessfulTestRun(ctx, flowID, draftRevision)
	return err == nil && ok
}
```

Apply the group A3. `Simulate` becomes a thin wrapper that marks after a successful `simulate`; a scenario simulation calls `simulate` and marks only when its assertions all passed; `SimulateBlock` marks after a successful result; both deletes remove the key.

```
cd W && node C:/disposable/easydrag-w1c/apply-plan-edits.mjs plans/2026-11-16-easydrag-w1c/part-a-server.md A3
```
Expected: seven `ok   edit` lines and `applied 7 edit(s) of group A3 in 6 file(s).`

<!-- edits:A3 -->

**Edit** `internal/flows/simulation.go`

Old:

```text
// Simulate executes only trusted built-in pure logic; all other built-in nodes
// require a fixture. It never uses the production registry's executable hooks.
func (s *Service) Simulate(ctx context.Context, id string, req SimulationRequest) (*SimulationResult, error) {
```

New:

```text
// Simulate executes only trusted built-in pure logic; all other built-in nodes
// require a fixture. It never uses the production registry's executable hooks. A
// simulation that ends with success marks the draft revision it ran as tested (IsTested).
func (s *Service) Simulate(ctx context.Context, id string, req SimulationRequest) (*SimulationResult, error) {
	result, err := s.simulate(ctx, id, req)
	if err == nil && result.Status == RunSuccess {
		s.markTested(ctx, id, result.Revision)
	}
	return result, err
}

// simulate is Simulate without the tested marker; SimulateScenario marks only after its
// assertions passed.
func (s *Service) simulate(ctx context.Context, id string, req SimulationRequest) (*SimulationResult, error) {
```

**Edit** `internal/flows/scenarios.go`

Old:

```text
	result, err := s.Simulate(ctx, flowID, SimulationRequest{TriggerNode: scenario.TriggerNode, TriggerData: scenario.TriggerData,
```

New:

```text
	result, err := s.simulate(ctx, flowID, SimulationRequest{TriggerNode: scenario.TriggerNode, TriggerData: scenario.TriggerData,
```

**Edit** `internal/flows/scenarios.go`

Old:

```text
	return &ScenarioSimulationResult{SimulationResult: result, Assertions: checks}, nil
```

New:

```text
	if result.Status == RunSuccess {
		passed := true
		for _, check := range checks {
			passed = passed && check.Passed
		}
		if passed {
			s.markTested(ctx, flowID, result.Revision)
		}
	}
	return &ScenarioSimulationResult{SimulationResult: result, Assertions: checks}, nil
```

**Edit** `internal/flows/service_runs.go`

Old:

```text
	if rec.Mode == ModeTest {
		return
	}
	s.mu.Lock()
	h, executed := s.history[rec.ID]
```

New:

```text
	if rec.Mode == ModeTest {
		// A test run that ended with success marks its draft revision as tested; the marker
		// outlives the run's row, which retention may prune.
		if res.Status == RunSuccess {
			s.markTested(context.Background(), rec.FlowID, rec.Revision)
		}
		return
	}
	s.mu.Lock()
	h, executed := s.history[rec.ID]
```

**Edit** `internal/flows/blocks_simulation.go`

Old:

```text
	return response, nil
}
```

New:

```text
	if response.Status == RunSuccess {
		s.markTested(ctx, id, rec.DraftRevision)
	}
	return response, nil
}
```

**Edit** `internal/flows/store.go`

Old:

```text
	res, err := s.db.ExecContext(ctx, `DELETE FROM flows WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
```

New:

```text
	res, err := s.db.ExecContext(ctx, `DELETE FROM flows WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	// The tested marker (testedMetaKey) goes with the flow; a failure leaves a harmless row.
	_, _ = s.db.ExecContext(ctx, `DELETE FROM flows_meta WHERE key = ?`, testedMetaKey(id))
	return nil
}
```

**Edit** `internal/flows/blocks_service.go`

Old:

```text
		return fmt.Errorf("delete block: %w", err)
	}
	return tx.Commit()
```

New:

```text
		return fmt.Errorf("delete block: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM flows_meta WHERE key=?`, testedMetaKey(id)); err != nil {
		return fmt.Errorf("delete block: %w", err)
	}
	return tx.Commit()
```

<!-- /edits -->

- [ ] **Step 5: prove no old call remains and run the tests**

```
cd W && grep -rn "s\.Simulate(" internal/flows/*.go | grep -v _test.go
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -run 'TestTested' -count=1 -v 2>&1 | grep -E "^(--- |ok|FAIL|PASS)"
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -run '(?i)(simulat|scenario|block)' -count=1 2>&1 | tail -3
```
Expected: the grep prints nothing (the scenario path calls `s.simulate`); the five tests pass:
```
--- PASS: TestTestedAfterASuccessfulSimulation
--- PASS: TestTestedNotAfterAFailedSimulation
--- PASS: TestTestedFollowsTheStoredTestRuns
--- PASS: TestTestedAfterABlockSimulationAndItsDelete
--- PASS: TestTestedSurvivesPruningOfTheTestRun
PASS
ok  	aurago/internal/flows
```
then `ok  	aurago/internal/flows	…s` for the existing simulation, scenario and block tests.

- [ ] **Step 6: gates and commit**

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./internal/flows && gofmt -l internal/flows/tested.go internal/flows/tested_test.go internal/flows/service_runs.go internal/flows/simulation.go internal/flows/scenarios.go internal/flows/blocks_simulation.go internal/flows/store.go internal/flows/blocks_service.go
```
Expected: no output.

Write `C:/disposable/easydrag-w1c/msg-a3.txt`:

```
feat(flows): remember which draft revision a test or simulation passed

A test run or a simulation of the draft that ends with success marks that
draft revision as tested; any save makes a new, untested revision. A successful
Simulate or SimulateBlock (and a scenario whose assertions all passed) and a
test run that ends with success (Service.onRunFinished) write the revision
into flows_meta under tested:<flow id>, the existing key/value table, so
there is no schema change; only a higher revision replaces the value, and the
flow and block deletes remove the key. The marker therefore outlives the
retention pruning of the test run's row; IsTested still also accepts a stored
successful test run of that revision. A failed or cancelled run marks nothing.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
```
cd W && git add internal/flows/tested.go internal/flows/tested_test.go internal/flows/service_runs.go internal/flows/simulation.go internal/flows/scenarios.go internal/flows/blocks_simulation.go internal/flows/store.go internal/flows/blocks_service.go && git commit -F C:/disposable/easydrag-w1c/msg-a3.txt && git status --short | head -3
```
Expected: one commit, a clean status.

---

### Task A.4: The change count from the diff the publish preview uses

**Files:**
- Create: `internal/flows/change_count.go`
- Modify: `internal/flows/service.go` (`DiffFlows`, about line 736)
- Test: `internal/flows/change_count_test.go` (new)

The function the publish preview uses is `flows.DiffFlows(rec.Live, rec.Draft)` (`PublishPreview`, `BlockPublishPreview`). It also builds the per-step details, which a count does not need; it is split into `countDiff` (the counts) and the details so the count shares the counting and not the details.

- [ ] **Step 1: IMPACT and the facts**

```
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "DiffFlows" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo 2>&1 | grep -E '"(risk|impactedCount)"' | head -3
cd W && grep -rn "DiffFlows(\|func countDiff\|countDiff(" internal cmd --include=*.go | grep -v _test.go
cd W && grep -rn "countDiff\|diffCounts" internal/flows/*_test.go | head -3
```
Expected: `LOW` (the index counts 6 impacted); the callers are `service.go:725`, `blocks_service.go:150` and `server/flows_ai_proposal.go:78`; `countDiff` does not exist yet. The test helper `diffCounts` in `simulation_test.go` is why the new function is called `countDiff`: a second `diffCounts` in package `flows` would not compile.

- [ ] **Step 2: write the failing tests**

Create `internal/flows/change_count_test.go`:

```go
package flows

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// changeCountFlow is a manual trigger followed by a chain of n set steps.
func changeCountFlow(n int) *Flow {
	b := newFlow("Zaehlen")
	prev := b.node("start", TypeTriggerManual, nil)
	for i := 0; i < n; i++ {
		id := b.node(fmt.Sprintf("s%d", i), TypeSet, map[string]any{"fields": []any{map[string]any{"name": "x", "value": fmt.Sprint(i)}}})
		b.edge(prev, PortOut, id)
		prev = id
	}
	return b.build()
}

func TestCountChangesNeverPublished(t *testing.T) {
	if got, ok := CountChanges(nil, changeCountFlow(3)); ok {
		t.Fatalf("CountChanges without a live version = %+v, want none", got)
	}
}

// CountChanges and DiffFlows come from one counting: the button and the publish dialog agree.
func TestCountChangesEqualsTheDiffCounts(t *testing.T) {
	live := changeCountFlow(4)
	draft, err := live.Clone()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		edit  func(f *Flow)
		steps int
		conns int
	}{
		{"unchanged", func(f *Flow) {}, 0, 0},
		{"a node moved is no change", func(f *Flow) { f.Nodes[2].Position = Point{X: 900, Y: 40} }, 0, 0},
		{"a step changed", func(f *Flow) {
			f.Nodes[2].Params = map[string]any{"fields": []any{map[string]any{"name": "x", "value": "neu"}}}
		}, 1, 0},
		{"a step removed with its connection", func(f *Flow) {
			f.Nodes = f.Nodes[:len(f.Nodes)-1]
			f.Edges = f.Edges[:len(f.Edges)-1]
		}, 1, 1},
		{"a step added and connected", func(f *Flow) {
			id := testNodeID(90)
			f.Nodes = append(f.Nodes, Node{ID: id, Key: "neu", Type: TypeSet, TypeVersion: 1, Label: "neu", Params: map[string]any{}})
			f.Edges = append(f.Edges, Edge{ID: "e_90", Source: PortRef{Node: f.Nodes[1].ID, Port: PortOut}, Target: PortRef{Node: id, Port: PortIn}})
		}, 1, 1},
	}
	for _, c := range cases {
		doc, err := draft.Clone()
		if err != nil {
			t.Fatal(err)
		}
		c.edit(doc)
		got, ok := CountChanges(live, doc)
		diff := DiffFlows(live, doc)
		if !ok || got.Steps != diff.AddedNodes+diff.ChangedNodes+diff.RemovedNodes || got.Connections != diff.ChangedEdges {
			t.Fatalf("%s: CountChanges = %+v %v, DiffFlows = %+v", c.name, got, ok, diff)
		}
		if got.Steps != c.steps || got.Connections != c.conns {
			t.Fatalf("%s: CountChanges = %+v, want %d steps and %d connections", c.name, got, c.steps, c.conns)
		}
	}
}

// The count of a stored flow equals the counts of the publish preview of the same draft.
func TestCountChangesEqualsThePublishPreview(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	rec := mustCreate(t, s, changeCountFlow(3))
	if _, _, err := s.Publish(ctx, rec.ID, rec.DraftRevision); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	live, err := s.GetFlow(ctx, rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	edited := changeCountFlow(5) // two more steps and two more connections
	edited.Nodes[1].Label = "Anders"
	if _, _, err := s.SaveDraft(ctx, rec.ID, edited, live.DraftRevision); err != nil {
		t.Fatal(err)
	}
	cur, err := s.GetFlow(ctx, rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.PublishPreview(ctx, rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := CountChanges(cur.Live, cur.Draft)
	want := preview.Diff.AddedNodes + preview.Diff.ChangedNodes + preview.Diff.RemovedNodes
	if !ok || got.Steps != want || got.Connections != preview.Diff.ChangedEdges || got.Steps == 0 || got.Connections == 0 {
		t.Fatalf("CountChanges = %+v %v, preview diff = %+v", got, ok, preview.Diff)
	}
}

// BenchmarkCountChanges measures one count on the largest flow the service accepts
// (MaxNodes steps and MaxEdges connections, every step changed), which is what each save
// of such a flow pays. Run: go test ./internal/flows -run '^$' -bench CountChanges -benchmem
func BenchmarkCountChanges(b *testing.B) {
	live := changeCountFlow(MaxNodes - 1)
	for i := 0; len(live.Edges) < MaxEdges; i++ {
		live.Edges = append(live.Edges, Edge{ID: fmt.Sprintf("x_%d", i), Source: PortRef{Node: live.Nodes[1+i%(MaxNodes-2)].ID, Port: "alt"}, Target: PortRef{Node: live.Nodes[2+i%(MaxNodes-2)].ID, Port: PortIn}})
	}
	draft, err := live.Clone()
	if err != nil {
		b.Fatal(err)
	}
	for i := range draft.Nodes {
		draft.Nodes[i].Label += "!"
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := CountChanges(live, draft); !ok {
			b.Fatal("no count")
		}
	}
}

// TestSaveCostOfTheChangeCount measures what each save pays for the count: reading the stored
// flow (draft and live document) and counting. It logs a typical flow (25 steps) and the longest
// chain the service accepts (MaxNodes steps). The ceiling is far above what either takes, so
// only a real regression fails.
func TestSaveCostOfTheChangeCount(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	for _, steps := range []int{25, MaxNodes - 1} {
		rec, err := s.CreateFlow(ctx, CreateRequest{Import: changeCountFlow(steps)})
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := s.Publish(ctx, rec.ID, rec.DraftRevision); err != nil {
			t.Fatalf("Publish of %d steps: %v", steps, err)
		}
		const rounds = 20
		begin := time.Now()
		for i := 0; i < rounds; i++ {
			cur, err := s.GetFlow(ctx, rec.ID)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := CountChanges(cur.Live, cur.Draft); !ok {
				t.Fatal("no count")
			}
		}
		each := time.Since(begin) / rounds
		t.Logf("GetFlow + CountChanges on %d steps: %s per save", steps+1, each)
		if each > time.Second {
			t.Fatalf("one count costs %s per save on %d steps, want well below 1s", each, steps+1)
		}
	}
}
```

- [ ] **Step 3: run them and see them fail**

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./internal/flows 2>&1 | head -4
```
Expected: `undefined: CountChanges`.

- [ ] **Step 4: implement**

Create `internal/flows/change_count.go`:

```go
package flows

// ChangeCount is what a draft differs from the live version by, in the two numbers the
// editor's primary button names: steps added, changed or removed, and connections added or
// removed. It is DiffFlows' own counting (countDiff), without the per-step details, so the
// button and the publish dialog always agree.
type ChangeCount struct {
	Steps       int `json:"steps"`
	Connections int `json:"connections"`
}

// CountChanges compares the live revision with the draft. ok is false for a flow that was never
// published (no live revision): there is nothing to count against.
func CountChanges(live, draft *Flow) (ChangeCount, bool) {
	if live == nil {
		return ChangeCount{}, false
	}
	d := countDiff(live, draft)
	return ChangeCount{Steps: d.AddedNodes + d.ChangedNodes + d.RemovedNodes, Connections: d.ChangedEdges}, true
}
```

Apply the group A4 (`DiffFlows` keeps its signature and result; the counting moves into `countDiff`):

```
cd W && node C:/disposable/easydrag-w1c/apply-plan-edits.mjs plans/2026-11-16-easydrag-w1c/part-a-server.md A4
```
Expected: two `ok   edit` lines and `applied 2 edit(s) of group A4 in 1 file(s).`

<!-- edits:A4 -->

**Edit** `internal/flows/service.go`

Old:

```text
func DiffFlows(live, draft *Flow) DiffSummary {
	if draft == nil {
		draft = &Flow{}
	}
	if live == nil {
		d := DiffSummary{FirstPublish: true, AddedNodes: len(draft.Nodes)}
		d.Details, d.DetailsTruncated = diffDetails(nil, draft)
		return d
	}
```

New:

```text
func DiffFlows(live, draft *Flow) DiffSummary {
	if draft == nil {
		draft = &Flow{}
	}
	d := countDiff(live, draft)
	d.Details, d.DetailsTruncated = diffDetails(live, draft)
	return d
}

// countDiff is DiffFlows without the per-step details: only the counts. CountChanges
// shares it, so the editor's change count equals the publish preview's counts.
func countDiff(live, draft *Flow) DiffSummary {
	if draft == nil {
		draft = &Flow{}
	}
	if live == nil {
		return DiffSummary{FirstPublish: true, AddedNodes: len(draft.Nodes)}
	}
```

**Edit** `internal/flows/service.go`

Old:

```text
	for k := range liveEdges {
		if !draftEdges[k] {
			d.ChangedEdges++
		}
	}
	d.Details, d.DetailsTruncated = diffDetails(live, draft)
	return d
}
```

New:

```text
	for k := range liveEdges {
		if !draftEdges[k] {
			d.ChangedEdges++
		}
	}
	return d
}
```

<!-- /edits -->

- [ ] **Step 5: run the new tests and the existing diff tests; measure**

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -run 'TestCountChanges|TestSaveCostOfTheChangeCount|Diff' -count=1 -v 2>&1 | grep -E "^(--- |ok|FAIL|PASS|\s+change_count_test)"
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -run '^$' -bench CountChanges -benchmem 2>&1 | grep -E "Benchmark|ok"
```
Expected:
```
--- PASS: TestCountChangesNeverPublished
--- PASS: TestCountChangesEqualsTheDiffCounts
--- PASS: TestCountChangesEqualsThePublishPreview
    change_count_test.go:…: GetFlow + CountChanges on 26 steps: <1 ms per save
    change_count_test.go:…: GetFlow + CountChanges on 500 steps: ~20 ms per save
--- PASS: TestSaveCostOfTheChangeCount
--- PASS: TestDiffFlows … (the existing diff tests)
PASS
```
The numbers measured while planning (an 11th-gen i7 laptop): **0.8 ms per save on a 26-step flow, 22 ms per save on a 500-step chain** (the read of draft and live document plus the count; both numbers are in the test log), and `BenchmarkCountChanges` (the count alone on the largest document the service accepts, 500 steps and 2000 connections, every step changed) **17-25 ms, 1.3 MB, 11,000 allocations**. A save already decodes and validates the document and writes it, so the answer stays in the save response; the spec's fallback (the editor asks after a pause) is not needed. Write the numbers you measure into the task's final report. If `TestSaveCostOfTheChangeCount` fails on the 1 s ceiling the machine is overloaded (the numbers above were 25 times lower): run it again when idle before touching any code.

- [ ] **Step 6: gates and commit**

```
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go vet ./internal/flows && gofmt -l internal/flows/change_count.go internal/flows/change_count_test.go internal/flows/service.go
```
Expected: no output.

Write `C:/disposable/easydrag-w1c/msg-a4.txt`:

```
feat(flows): CountChanges, the editor's change count from the publish diff's counting

DiffFlows is split into countDiff (steps added, changed and removed,
connections changed) and the per-step details; DiffFlows returns the same
result as before. CountChanges(live, draft) returns {steps, connections} from
countDiff, and nothing for a flow without a live version, so the editor's
primary button and the publish dialog always agree (a test compares it with
the preview of a stored flow). Measured on the largest chain the service
accepts (500 steps): about 20 ms for reading the flow and counting.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
```
cd W && git add internal/flows/change_count.go internal/flows/change_count_test.go internal/flows/service.go && git commit -F C:/disposable/easydrag-w1c/msg-a4.txt && git status --short | head -3
```
Expected: one commit, a clean status.

---

### Task A.5: The API fields

**Files:**
- Create: `internal/server/flows_draft_facts.go`
- Modify: `internal/server/flows_handlers.go` (the GET and PUT of a flow, about lines 692-715), `internal/server/flows_blocks_handlers.go` (the GET and PUT of a block, about lines 157-200)
- Test: `internal/server/flows_draft_facts_test.go` (new; uses `timeFlowJSON` from A.2's `flows_next_run_test.go`)

- [ ] **Step 1: IMPACT and the facts**

```
node C:/Users/Andi/.gitnexus/stores/AuraGo-08496d38b623/checkouts/AuraGo-08496d38b623/run.cjs impact "flowBlockDocument" --direction upstream --repo C:/Users/Andi/Documents/repo/AuraGo 2>&1 | grep -E '"(risk|impactedCount)"' | head -3
cd W && grep -n "flowsJSON(w, http.StatusOK, map\[string\]any{\"flow\": rec, \"enabled\"\|\"draft_revision\": rev, \"issues\"\|\"draft_revision\": revision, \"issues\"" internal/server/flows_handlers.go internal/server/flows_blocks_handlers.go
cd W && grep -n "const flowBlocksPath\|func flowBlockCall\|func flowsCall\|func timeFlowJSON" internal/server/*.go
```
Expected: `LOW`; one line in `flows_handlers.go` (the GET), one in `flows_handlers.go` (the PUT) and one in `flows_blocks_handlers.go` (the PUT); the helpers are found once each (`timeFlowJSON` comes from A.2). The flow route is `/api/desktop/flows/{id}`, the block route `/api/desktop/flow-blocks/{id}`.

- [ ] **Step 2: write the failing handler tests**

Create `internal/server/flows_draft_facts_test.go`:

```go
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

func factsHas(answer map[string]any, key string) bool {
	_, has := answer[key]
	return has
}

// GET {flow} and the save answer carry "tested" and "changes".
func TestFlowDraftFactsTestedAndChanges(t *testing.T) {
	s, token := newFlowsTestServer(t)
	ctx := context.Background()
	rec := createTestFlow(t, s, greetFlowJSON)
	path := "/api/desktop/flows/" + rec.ID
	get := func() map[string]any {
		t.Helper()
		w := flowsCall(t, s, http.MethodGet, path, token, "")
		if w.Code != http.StatusOK {
			t.Fatalf("GET = %d %s", w.Code, w.Body.String())
		}
		return flowsBody(t, w)
	}
	save := func(doc string, revision int) map[string]any {
		t.Helper()
		w := flowsCall(t, s, http.MethodPut, path, token, fmt.Sprintf(`{"doc":%s,"base_revision":%d}`, doc, revision))
		if w.Code != http.StatusOK {
			t.Fatalf("PUT = %d %s", w.Code, w.Body.String())
		}
		return flowsBody(t, w)
	}

	one := get()
	if one["tested"] != false || factsHas(one, "changes") || factsHas(one, "next_run") {
		t.Fatalf("a new flow: tested=%v changes=%v next_run=%v", one["tested"], one["changes"], one["next_run"])
	}

	// A successful simulation of the draft marks it, and a reload (a new GET) still shows it.
	w := flowsCall(t, s, http.MethodPost, path+"/simulate", token, fmt.Sprintf(`{"base_revision":%d}`, rec.DraftRevision))
	if w.Code != http.StatusOK {
		t.Fatalf("simulate = %d %s", w.Code, w.Body.String())
	}
	if one = get(); one["tested"] != true {
		t.Fatalf("after a simulation tested = %v, want true", one["tested"])
	}

	// A save makes a new revision; the flow was never published, so there is no count.
	saved := save(greetFlowJSON, rec.DraftRevision)
	revision := int(saved["draft_revision"].(float64))
	if saved["tested"] != false || factsHas(saved, "changes") {
		t.Fatalf("save of an unpublished flow: tested=%v changes=%v", saved["tested"], saved["changes"])
	}
	if one = get(); one["tested"] != false {
		t.Fatalf("after a save tested = %v, want false", one["tested"])
	}

	// Published and then edited: the count equals the publish preview's diff.
	if _, _, err := s.Flows.Publish(ctx, rec.ID, revision); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if saved = save(strings.Replace(greetFlowJSON, `"label":"Gruss"`, `"label":"Hallo"`, 1), revision); saved["tested"] != false {
		t.Fatalf("save after a publish: tested = %v", saved["tested"])
	}
	changes, ok := saved["changes"].(map[string]any)
	if !ok || changes["steps"] != float64(1) || changes["connections"] != float64(0) {
		t.Fatalf("save answer changes = %v, want 1 step and 0 connections", saved["changes"])
	}
	preview := flowsBody(t, flowsCall(t, s, http.MethodGet, path+"/publish-preview", token, ""))["diff"].(map[string]any)
	steps := preview["added_nodes"].(float64) + preview["changed_nodes"].(float64) + preview["removed_nodes"].(float64)
	if changes["steps"] != steps || changes["connections"] != preview["changed_edges"] {
		t.Fatalf("changes %v differ from the publish preview %v", changes, preview)
	}
	if got := get()["changes"]; !reflect.DeepEqual(got, saved["changes"]) {
		t.Fatalf("GET changes = %v, save answer = %v", got, saved["changes"])
	}
}

// GET {flow} carries "next_run" for an enabled flow with a Date/Time trigger, and not otherwise.
func TestFlowDraftFactsNextRun(t *testing.T) {
	s, token := newFlowsTestServer(t)
	ctx := context.Background()
	at := time.Now().Add(48 * time.Hour).Truncate(time.Second)
	rec := createTestFlow(t, s, timeFlowJSON(false, at))
	path := "/api/desktop/flows/" + rec.ID
	nextRun := func() (map[string]any, bool) {
		t.Helper()
		body := flowsBody(t, flowsCall(t, s, http.MethodGet, path, token, ""))
		value, has := body["next_run"]
		if !has {
			return nil, false
		}
		return value.(map[string]any), true
	}
	if _, has := nextRun(); has {
		t.Fatal("an unpublished flow has a next run")
	}
	if _, _, err := s.Flows.Publish(ctx, rec.ID, rec.DraftRevision); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if _, has := nextRun(); has {
		t.Fatal("a flow that is switched off has a next run")
	}
	if err := s.Flows.SetEnabled(ctx, rec.ID, true); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	next, has := nextRun()
	if !has {
		t.Fatal("an enabled flow with a Date/Time trigger has no next run")
	}
	got, err := time.Parse(time.RFC3339, next["at"].(string))
	if err != nil || !got.Equal(at) || next["node"] != "n_cccccccc" {
		t.Fatalf("next_run = %v (%v), want %s at n_cccccccc", next, err, at.Format(time.RFC3339))
	}
	if err := s.Flows.SetEnabled(ctx, rec.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, has := nextRun(); has {
		t.Fatal("a paused flow has a next run")
	}
}

// The block document answers with "tested" too, never with "next_run", and a block
// simulation marks the draft revision.
func TestBlockDraftFactsTested(t *testing.T) {
	s, token := newFlowsTestServer(t)
	ctx := context.Background()
	block, err := s.Flows.CreateBlock(ctx, "Leer")
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := block.Draft.Clone()
	delete(doc.Nodes[1].Params, "values")
	revision, issues, err := s.Flows.SaveBlockDraft(ctx, block.ID, doc, block.DraftRevision)
	if err != nil {
		t.Fatalf("save: %v %+v", err, issues)
	}
	path := flowBlocksPath + "/" + block.ID
	get := func() map[string]any {
		t.Helper()
		w := flowBlockCall(t, s, http.MethodGet, path, token, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("GET = %d %s", w.Code, w.Body.String())
		}
		return flowsBody(t, w)
	}
	if one := get(); one["tested"] != false || factsHas(one, "next_run") || factsHas(one, "changes") {
		t.Fatalf("a new block draft: %v", one)
	}
	if w := flowBlockCall(t, s, http.MethodPost, path+"/simulate", token, map[string]any{"base_revision": revision}); w.Code != http.StatusOK {
		t.Fatalf("simulate = %d %s", w.Code, w.Body.String())
	}
	if one := get(); one["tested"] != true {
		t.Fatalf("after a simulation tested = %v, want true", one["tested"])
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	w := flowBlockCall(t, s, http.MethodPut, path, token, map[string]any{"doc": json.RawMessage(raw), "base_revision": revision})
	if w.Code != http.StatusOK {
		t.Fatalf("PUT = %d %s", w.Code, w.Body.String())
	}
	if saved := flowsBody(t, w); saved["tested"] != false || factsHas(saved, "next_run") {
		t.Fatalf("block save answer = %v", saved)
	}
}
```

- [ ] **Step 3: run them and see them fail**

```
cd W && GOTOOLCHAIN=go1.27.4 go test ./internal/server -run 'TestFlowDraftFacts|TestBlockDraftFacts' -count=1 2>&1 | grep -E "^(--- |ok|FAIL)|_test.go" | head -8
```
Expected: three `--- FAIL`, for example `a new flow: tested=<nil> changes=<nil> next_run=<nil>` (the answer has no `tested` yet), `an enabled flow with a Date/Time trigger has no next run`, `a new block draft: map[…]`.

- [ ] **Step 4: implement**

Create `internal/server/flows_draft_facts.go`:

```go
package server

import (
	"context"
	"time"

	"aurago/internal/flows"
)

// addFlowDraftFacts adds the three read-only facts the editor's header shows to the answer of
// GET {flow} (or GET block): "tested" (a test run or simulation of exactly the current draft
// revision succeeded), "changes" (steps and connections that differ from the live version; only
// for a flow that was published) and, when withNextRun is set, "next_run" ({"at": RFC 3339 in
// the server's location, "node": the trigger node}; only when the flow starts by itself).
// rec is a record that was read with its draft and live documents.
func (s *Server) addFlowDraftFacts(ctx context.Context, answer map[string]any, rec *flows.FlowRecord, withNextRun bool) {
	answer["tested"] = s.Flows.IsTested(ctx, rec.ID, rec.DraftRevision)
	if changes, ok := flows.CountChanges(rec.Live, rec.Draft); ok {
		answer["changes"] = changes
	}
	if !withNextRun {
		return
	}
	// time.Local is the location the flow service runs its timers and schedules in (initFlows).
	if next, ok := s.Flows.NextRun(rec.ID); ok {
		answer["next_run"] = map[string]string{"at": next.At.In(time.Local).Format(time.RFC3339), "node": next.NodeID}
	}
}

// addSavedDraftFacts is addFlowDraftFacts for the answer to a save, which carries the new draft
// revision. It reads the stored record again; when another save has moved on meanwhile the
// count would belong to a different revision, so only "tested" (for the revision this save
// made) is given.
func (s *Server) addSavedDraftFacts(ctx context.Context, answer map[string]any, id string, revision int, block bool) {
	read := s.Flows.GetFlow
	if block {
		read = s.Flows.GetBlock
	}
	rec, err := read(ctx, id)
	if err != nil || rec.DraftRevision != revision {
		answer["tested"] = s.Flows.IsTested(ctx, id, revision)
		return
	}
	s.addFlowDraftFacts(ctx, answer, rec, !block)
}
```

Apply the group A5 (the GET and PUT of a flow and of a block build their answer as a map, add the facts and send it):

```
cd W && node C:/disposable/easydrag-w1c/apply-plan-edits.mjs plans/2026-11-16-easydrag-w1c/part-a-server.md A5
```
Expected: four `ok   edit` lines and `applied 4 edit(s) of group A5 in 2 file(s).`

<!-- edits:A5 -->

**Edit** `internal/server/flows_handlers.go`

Old:

```text
			issues := s.Flows.Validate(rec.Draft, flows.ModeDraft)
			flowsJSON(w, http.StatusOK, map[string]any{"flow": rec, "enabled": s.flowEnabled(rec), "issues": nonNilIssues(issues),
				"live_issues": nonNilIssues(s.Flows.LiveIssues(rec)), "timers_running": s.Flows.TimersRunning()})
```

New:

```text
			issues := s.Flows.Validate(rec.Draft, flows.ModeDraft)
			answer := map[string]any{"flow": rec, "enabled": s.flowEnabled(rec), "issues": nonNilIssues(issues),
				"live_issues": nonNilIssues(s.Flows.LiveIssues(rec)), "timers_running": s.Flows.TimersRunning()}
			s.addFlowDraftFacts(ctx, answer, rec, true)
			flowsJSON(w, http.StatusOK, answer)
```

**Edit** `internal/server/flows_handlers.go`

Old:

```text
			s.broadcastFlowsChanged(id, "saved")
			flowsJSON(w, http.StatusOK, map[string]any{"draft_revision": rev, "issues": nonNilIssues(issues)})
```

New:

```text
			s.broadcastFlowsChanged(id, "saved")
			answer := map[string]any{"draft_revision": rev, "issues": nonNilIssues(issues)}
			s.addSavedDraftFacts(ctx, answer, id, rev, false)
			flowsJSON(w, http.StatusOK, answer)
```

**Edit** `internal/server/flows_blocks_handlers.go`

Old:

```text
		if contract, err := flows.BlockDraftContract(block.Draft); err == nil {
			answer["contract"] = contract
		}
		flowsJSON(w, http.StatusOK, answer)
```

New:

```text
		if contract, err := flows.BlockDraftContract(block.Draft); err == nil {
			answer["contract"] = contract
		}
		s.addFlowDraftFacts(ctx, answer, block, false)
		flowsJSON(w, http.StatusOK, answer)
```

**Edit** `internal/server/flows_blocks_handlers.go`

Old:

```text
		s.broadcastFlowsChanged(id, "saved")
		flowsJSON(w, http.StatusOK, map[string]any{"draft_revision": revision, "issues": nonNilIssues(issues)})
```

New:

```text
		s.broadcastFlowsChanged(id, "saved")
		answer := map[string]any{"draft_revision": revision, "issues": nonNilIssues(issues)}
		s.addSavedDraftFacts(ctx, answer, id, revision, true)
		flowsJSON(w, http.StatusOK, answer)
```

<!-- /edits -->

- [ ] **Step 5: run the tests**

```
cd W && GOTOOLCHAIN=go1.27.4 go test ./internal/server -run 'TestFlowDraftFacts|TestBlockDraftFacts|TestMissionControlNextRunEqualsTheFlowNextRun' -count=1 -v 2>&1 | grep -E "^(--- |ok|FAIL|PASS)"
cd W && GOTOOLCHAIN=go1.27.4 go test ./internal/server -run '(?i)flow' -count=1 2>&1 | tail -3
```
Expected: four `--- PASS` lines, `ok`; then `ok  	aurago/internal/server	…s`. The existing handler tests compare answers by key, so the added keys break none; if one does, it compared a whole answer: update it to ignore the three new keys and say so in the commit.

- [ ] **Step 6: gates and commit**

```
cd W && GOTOOLCHAIN=go1.27.4 go vet ./internal/server && gofmt -l internal/server/flows_draft_facts.go internal/server/flows_draft_facts_test.go internal/server/flows_handlers.go internal/server/flows_blocks_handlers.go
```
Expected: no output.

Write `C:/disposable/easydrag-w1c/msg-a5.txt`:

```
feat(server): the flow API says next run, tested and change count

GET /api/desktop/flows/{id} and the save answer carry "tested" (a test run or
simulation of exactly the current draft revision succeeded), "changes"
({steps, connections} against the live version, only for a flow that was
published) and "next_run" ({at in the server's location, node}, only when the
flow starts by itself). The block document (GET and PUT on
/api/desktop/flow-blocks/{id}) carries tested and changes. The save answer
reads the stored record again and gives the count only when it belongs to the
revision that save made. The flow record and the list endpoint are unchanged.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
```
cd W && git add internal/server/flows_draft_facts.go internal/server/flows_draft_facts_test.go internal/server/flows_handlers.go internal/server/flows_blocks_handlers.go && git commit -F C:/disposable/easydrag-w1c/msg-a5.txt && git status --short | head -3
```
Expected: one commit, a clean status.

---

### Task A.6: The shared fixture answers the three fields; the notes

**Files:**
- Modify: `ui/testdata/easydrag-fixture.js` (the `state` comment and literal, about lines 142-160; after `const seeded`, about line 185; the flow GET, about line 637; the save answer, about line 648; CRLF), `internal/flows/AGENTS.md` (append)
- Test: `scripts/test-easydrag-fixture-facts.mjs` (new; the name matches `scripts/test-easydrag*.mjs`, so T0 runs it)

The names Parts B and D write against: `edFixture.state.tested`, `edFixture.state.changes`, `edFixture.state.nextRun`. Parts B and D read these names and add other fixture state only with distinct names in their own tasks.

- [ ] **Step 1: check the fixture's shape**

```
cd W && grep -n "const seeded = addFlow\|if (method === 'GET') return reply({ flow: rec\|return reply({ draft_revision: rec.draft_revision, issues: validate(rec.draft, 'draft') });\|malformedSSESent: false, failType" ui/testdata/easydrag-fixture.js | cut -c1-120
cd W && grep -rn "easydrag-fixture" scripts/*.mjs ui/*_test.go | cut -c1-100
```
Expected: four lines (one each) and `ui/desktop_easydrag_browser_test.go` as the only user of the fixture (no Node test loads it today).

- [ ] **Step 2: write the failing Node test**

Create `scripts/test-easydrag-fixture-facts.mjs`:

```js
// W1c Part A: the shared browser fixture answers GET {flow} and the save with "tested", "changes" and
// "next_run", controlled by edFixture.state.tested / .changes / .nextRun (the names Parts B and D write
// against). The defaults are neutral: tested false, no changes, no next run. The fixture is loaded in a vm
// with the few browser globals it touches.
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import assert from 'node:assert/strict';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const source = fs.readFileSync(path.join(root, 'ui', 'testdata', 'easydrag-fixture.js'), 'utf8');

const window = { fixtureReady: Promise.resolve(), fetch: async () => new Response('{}', { status: 404 }), I18N: {} };
window.window = window;
const sandbox = {
    window, Response, Headers, URL, JSON, Date, Map, Set, Promise, setTimeout, clearTimeout, console,
    location: { origin: 'http://fixture.test' },
    document: { dispatchEvent() { return true; }, addEventListener() {} },
    CustomEvent: class { constructor(type, init) { this.type = type; this.detail = init && init.detail; } },
    EventSource: class {}
};
vm.createContext(sandbox);
vm.runInContext(source, sandbox, { filename: 'easydrag-fixture.js' });

const fx = window.edFixture;
assert.ok(fx && fx.state, 'edFixture.state exists');
const call = async (method, url, body) => {
    const res = await window.fetch(url, { method, body: body ? JSON.stringify(body) : undefined });
    return { status: res.status, body: await res.json() };
};
const flowPath = '/api/desktop/flows/' + fx.seededID;
const has = (body, key) => Object.prototype.hasOwnProperty.call(body, key);

// Neutral defaults: the answers carry tested:false and nothing else of the three.
assert.equal(fx.state.tested, false);
assert.equal(fx.state.changes, null);
assert.equal(fx.state.nextRun, null);
let got = await call('GET', flowPath);
assert.equal(got.status, 200);
assert.equal(got.body.tested, false);
assert.ok(!has(got.body, 'changes') && !has(got.body, 'next_run'), 'by default there is no changes and no next_run');
const doc = JSON.parse(JSON.stringify(fx.flow().rec.draft));
let saved = await call('PUT', flowPath, { doc, base_revision: fx.flow().rec.draft_revision });
assert.equal(saved.status, 200);
assert.equal(saved.body.tested, false);
assert.ok(!has(saved.body, 'changes') && !has(saved.body, 'next_run'), 'the default save answer has neither');

// The three names control the answer, set explicitly.
fx.state.tested = true;
fx.state.changes = { steps: 5, connections: 0 };
fx.state.nextRun = { at: '2031-03-03T07:00:00+01:00', node: 'n_start' };
got = await call('GET', flowPath);
assert.equal(got.body.tested, true);
assert.deepEqual(got.body.changes, { steps: 5, connections: 0 });
assert.deepEqual(got.body.next_run, { at: '2031-03-03T07:00:00+01:00', node: 'n_start' });
fx.state.changes = null;
fx.state.nextRun = null;
got = await call('GET', flowPath);
assert.equal(got.body.tested, true);
assert.ok(!has(got.body, 'changes') && !has(got.body, 'next_run'), 'null leaves the fields out');

// The save answer carries tested and changes (not next_run), and the fixture does not reset tested by itself.
fx.state.changes = { steps: 1, connections: 2 };
fx.state.nextRun = { at: '2031-03-03T07:00:00+01:00', node: 'n_start' };
saved = await call('PUT', flowPath, { doc, base_revision: fx.flow().rec.draft_revision });
assert.equal(saved.status, 200);
assert.equal(saved.body.tested, true);
assert.deepEqual(saved.body.changes, { steps: 1, connections: 2 });
assert.ok(!has(saved.body, 'next_run'), 'the save answer has no next_run');

// Like the server, a flow without a live version has no changes.
const draftOnly = fx.addDraft('Entwurf');
got = await call('GET', '/api/desktop/flows/' + draftOnly);
assert.ok(!has(got.body, 'changes'), 'a never published flow has no changes');
assert.equal(got.body.tested, true);

console.log('PASS fixture facts (tested, changes, next_run)');
```

Run it:
```
cd W && node scripts/test-easydrag-fixture-facts.mjs 2>&1 | head -6
```
Expected: an `AssertionError` on the first `tested` check (the fixture's answer has no `tested`): `Expected values to be strictly equal` with `undefined !== false`, or `fx.state.tested` undefined.

- [ ] **Step 3: implement**

Apply the group A5F. The three values live in `edFixture.state` with **neutral defaults** (`tested: false`, `changes: null`, `nextRun: null`), so no existing browser test sees a difference except the extra `tested: false`. The helper is called `factsOf(rec, withSchedule)` (the name Part B's fallback uses, so the two merge): the flow GET and the save answer add `tested` always, `changes` only for a flow with a live version and only when `state.changes` is set, and `next_run` only on the GET (the save answer has none) and only when `state.nextRun` is set. The fixture does not mimic the marker's transitions: a test sets `state.tested` before the call that should show it.

```
cd W && node C:/disposable/easydrag-w1c/apply-plan-edits.mjs plans/2026-11-16-easydrag-w1c/part-a-server.md A5F
```
Expected: five `ok   edit` lines and `applied 5 edit(s) of group A5F in 1 file(s).`

<!-- edits:A5F -->

**Edit** `ui/testdata/easydrag-fixture.js`

Old:

```text
    //   delay           milliseconds each step of a run takes
    const state = {
```

New:

```text
    //   delay           milliseconds each step of a run takes
    //   tested          GET {flow} and every save answer carry "tested": this value (false by default). The
    //                   fixture does not mimic the server's marker, so a save does not reset it: set it
    //                   before the call that should show it
    //   changes         {steps, connections}: "changes" of GET {flow} and of the save answer for a flow
    //                   with a live version (null by default: the field is left out)
    //   nextRun         {at, node}: "next_run" of GET {flow} (null by default: the field is left out)
    const state = {
```

**Edit** `ui/testdata/easydrag-fixture.js`

Old:

```text
malformedSSESent: false, failType: '', failPreview: false,
```

New:

```text
malformedSSESent: false, failType: '', failPreview: false, tested: false, changes: null, nextRun: null,
```

**Edit** `ui/testdata/easydrag-fixture.js`

Old:

```text
    const seeded = addFlow(briefing(), true, true);
```

New:

```text
    const seeded = addFlow(briefing(), true, true);

    // factsOf are the W1c facts of a flow answer: tested, changes (only for a flow with a live version) and next_run
    // (only when withSchedule is set: the GET), from state.tested, state.changes and state.nextRun. The neutral
    // defaults (false, null, null) leave every existing answer as it was apart from "tested": false.
    function factsOf(rec, withSchedule) {
        const out = { tested: !!state.tested };
        if (rec.live && state.changes) out.changes = clone(state.changes);
        if (withSchedule && state.nextRun) out.next_run = clone(state.nextRun);
        return out;
    }
```

**Edit** `ui/testdata/easydrag-fixture.js`

Old:

```text
            if (method === 'GET') return reply({ flow: rec, enabled: entry.enabled, issues: validate(rec.draft, 'draft') });
```

New:

```text
            if (method === 'GET') return reply(Object.assign({ flow: rec, enabled: entry.enabled, issues: validate(rec.draft, 'draft') }, factsOf(rec, true)));
```

**Edit** `ui/testdata/easydrag-fixture.js`

Old:

```text
            return reply({ draft_revision: rec.draft_revision, issues: validate(rec.draft, 'draft') });
```

New:

```text
            return reply(Object.assign({ draft_revision: rec.draft_revision, issues: validate(rec.draft, 'draft') }, factsOf(rec, false)));
```

<!-- /edits -->

Apply the group A5D (the notes for the next reader of `internal/flows`):

```
cd W && node C:/disposable/easydrag-w1c/apply-plan-edits.mjs plans/2026-11-16-easydrag-w1c/part-a-server.md A5D
```
Expected: `ok   append internal/flows/AGENTS.md` and `applied 1 edit(s) of group A5D in 1 file(s).`

<!-- edits:A5D -->

**Append** `internal/flows/AGENTS.md`

```text

## W1c draft facts: next run, "tested", change count

- `Service.NextRun(flowID)` (`next_run.go`) is the one answer to "when does this flow start by itself": the earliest fire after now of the live version's Date/Time timers (`Store.NextTimerAfter`, unlike `NextTimerAt` it skips a stored fire that has passed) and of the schedule triggers (the `ScheduleNexter` that the server injects with `SetScheduleNexter`; this package does not import the cron code). A flow that is unknown, a block, unpublished or switched off (`MissionBridge.FlowMissionEnabled`) has none. It takes no flow lock and reads no document (`flowRunBinding`), so a bridge may call it synchronously. Mission Control's `nextFlowRun` and `GET /api/desktop/flows/{id}` (`next_run`) both end here (`NextRunForMission`, `tools.FlowNextRunner`).
- "Tested" (`tested.go`): a test run or a simulation of exactly the current draft revision finished with success. A successful `Simulate` or `SimulateBlock` (and a scenario whose assertions all passed), and a test run that ends with success (`Service.onRunFinished`), write the revision into `flows_meta` under `tested:<flow id>`, and only a higher revision replaces it, so the marker outlives retention's pruning of the run row. `IsTested` also accepts a stored successful test run of that revision (`flow_runs`: mode `test`, `revision` = draft revision, status `success`), which covers the moment before the hook has written. No schema change. Every save bumps the draft revision, which makes the flow untested again. `Store.DeleteFlow` and the block delete remove the key. `IsTested` is read-only.
- `CountChanges` (`change_count.go`) is `DiffFlows`' own counting without the details (`countDiff`): steps added, changed or removed, and connections changed. The publish preview and the editor's primary button share it.
```

<!-- /edits -->

- [ ] **Step 4: run the test and the fast checks**

```
cd W && node --check ui/testdata/easydrag-fixture.js && node scripts/test-easydrag-fixture-facts.mjs
cd W && node scripts/test-easydrag.mjs 2>&1 | tail -2
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)easydrag' -count=1 -short 2>&1 | tail -3
cd W && node -e "const s=require('fs').readFileSync('ui/testdata/easydrag-fixture.js','utf8');console.log(s.split(String.fromCharCode(13,10)).length-1, s.split(String.fromCharCode(10)).length-1)"
```
Expected: `PASS fixture facts (tested, changes, next_run)`; `all EasyDrag checks passed`; `ok  	aurago/ui	…s`; the last command prints two equal numbers (`916 916` while main's fixture has 900 lines: every line still ends in CRLF; a first number below the second means a line lost its CR). The render baseline is not run: no shipped JS or CSS changed (the fixture lives in `ui/testdata`).

- [ ] **Step 5: the part-level run (T0 and the suites of the spec's §7)**

```
cd W && npm run test:easydrag:t0 2>&1 | tail -8
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./internal/flows/... -count=1 2>&1 | tail -3
cd W && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./internal/tools -run '(?i)(mission|flow)' -count=1 2>&1 | tail -3
cd W && GOTOOLCHAIN=go1.27.4 go test ./internal/server -run '(?i)flow' -count=1 2>&1 | tail -3
```
Expected: T0 ends without a FAIL line (90-200 s; it already includes the flows, tools and server flow suites, the others are the spec's explicit list); `ok  	aurago/internal/flows` (about 2 minutes), `ok  	aurago/internal/tools`, `ok  	aurago/internal/server`.

- [ ] **Step 6: commit**

Write `C:/disposable/easydrag-w1c/msg-a6.txt`:

```
test(easydrag): the shared fixture answers tested, changes and next_run

GET {flow} and the save answer of the browser fixture carry the three fields
that Part A added to the flow API, controlled by edFixture.state.tested,
.changes and .nextRun, whose neutral defaults (false, null, null) leave every
existing answer as it was apart from tested: false. changes needs a live
version and is left out while null; next_run is only on the GET and only when
set. A Node test loads the fixture, asserts the neutral defaults and pins the
three names for the header (B) and quick-add (D) work. The notes in
internal/flows/AGENTS.md record the next-run, tested and change-count rules.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
```
cd W && git add ui/testdata/easydrag-fixture.js scripts/test-easydrag-fixture-facts.mjs internal/flows/AGENTS.md && git commit -F C:/disposable/easydrag-w1c/msg-a6.txt && git status --short | head -3 && git log --oneline -7
```
Expected: one commit, a clean status, and six Part A commits on top of main (A.1 to A.6).

---

### Coverage table (spec item to task)

| Spec item | Task |
|---|---|
| §3.2 `Service.NextRun`, `NextRun{At, NodeID}`, `ScheduleNexter` (no cron import) | A.1 |
| §3.2 earliest fire after now, live version, active; paused, unpublished, no time trigger give nothing; a past stored fire is not "next" | A.1 (`TestNextRun*`) |
| §3.2 read-only, no flow lock | A.1 (`TestNextRunTakesNoFlowLock`) |
| §3.2 the server injects the schedule half from the cron side | A.2 (`NextFlowScheduleRun`, `flowScheduleNexter`, `initFlows`) |
| §3.2 Mission Control's `nextFlowRun` calls the shared function | A.2 (`FlowNextRunner`, the hooks, edit group A2) |
| §3.2 `next_run` in `GET /api/desktop/flows/{id}`, RFC 3339 in the server's location | A.5 |
| §3.3 rule: success marks R, failure does not, a save makes it false, a simulation marks, blocks mark | A.3 (`TestTested*`), A.5 (`TestFlowDraftFactsTestedAndChanges`, `TestBlockDraftFactsTested`) |
| §3.3 storage check first; no migration | A.3 step 1 and `tested.go` (`flows_meta`) |
| §3.3 `tested` in GET and in the save answer | A.5 |
| §3.4 `changes` from the publish diff's function; absent for a never published flow | A.4, A.5 |
| §3.4 cost measured on the largest flow | A.4 step 5 (numbers in the task and in the report) |
| §3.5 Go tests in `internal/flows` (six next-run cases) | A.1 |
| §3.5 Go test in `internal/tools`: Mission Control equals `NextRun`; existing tests green | A.2 (tools delegation test; the equality with the real service is in `internal/server`, see Deviations) |
| §3.5 Go tests for the marker and the count | A.3, A.4, A.5 |
| Brief: fixture fields `edFixture.state.tested`, `.changes`, `.nextRun` | A.6 |
| Acceptance 1 (`next_run` equals Mission Control's, schedules and Date/Time, paused/unpublished none) | A.2, A.5 |
| Acceptance 2 (`tested` survives a reload, false after a save, failed test false) | A.3, A.5 |
| Acceptance 3 (`changes` equals the preview's counts) | A.4, A.5 |

### Deviations from the spec

1. **The next-run source of Mission Control is switched by an optional interface, and the old combination stays for hooks without it.** The spec says `nextFlowRun`'s body becomes a call to the shared function. Done for AuraGo's real hooks (`FlowNextRunner`). Every `internal/tools` test double (`fakeFlowHooks` and four others) implements only `NextFlowRun`, and `TestFlowNextRunAndDeleteUseOnlyFlowCronJobs` and `TestNextRunForFlowMissions` expect the cron jobs and the hooks' timer to be combined inside `nextFlowRun`; the spec also says those tests stay green. The fallback is also what a manager without the flow service (a test, a server whose flows are off) shows.
2. **The "Mission Control equals NextRun" equality test is in `internal/server`, not `internal/tools`.** It needs the real flow service, the real cron engine and the real manager together, and only the server package can import both `tools` and `flows`. `internal/tools` gets a delegation test (the shared answer is Mission Control's answer) and a test of the schedule half (`NextFlowScheduleRun`).
3. **The three fields are top-level in the answers, not inside `flow`.** The flow record is shared with the list endpoint and the run and export paths; a top-level field leaves them untouched. Part B copies them onto `ed.flow` as the contract says.
4. **Blocks carry `tested` and `changes` too.** The spec's §4.5 gives blocks the same primary-button logic and §3.3 says the block simulation marks the same way, so the block document (`/api/desktop/flow-blocks/{id}`, a different route from `/api/desktop/flows/{id}`) answers with them; never with `next_run`.
5. **The save answer carries `next_run`.** A save does not change it (only a publish or a switch does), but the contract says the editor updates all three from every save response.
6. **The marker for real test runs is written by the finish hook, not only derived from the run row.** The spec offered "derive it from the newest successful test run"; retention prunes test runs (20 per flow, 30 days) and the owner chose "successful, durable". So `Service.onRunFinished` (which returned at once for test runs) writes the revision of a successful test run into `flows_meta` through the same function as simulations. The stored-run lookup stays as a second source (it closes the instant before the hook has written, and covers runs from before this code). The hook runs outside all locks after the run is persisted; a failed write is only logged.
7. **`DiffFlows` is refactored** (counts split from details; same result) instead of being called as is, because it also builds the per-step details, which a count that runs on every save does not need.
8. **A.6 is a separate task** (fixture, a Node test, the notes): different language and test from the Go work of A.5.

### Decisions needed

1. **A partial test or simulation marks the whole draft.** A test run of one step (`only_node`) and a simulation of one step end with success but exercise part of the flow. The client's `ed.testEvidence` treats them as tests of the revision today, and the spec says "a test or simulation … that finishes successfully". Recommendation: keep it (the same rule as the publish dialog's "tested" line). Telling them apart needs the run's `only_node`, which is not stored (a schema change, not allowed in W1c).
2. ~~Marker lifetime for real test runs~~ decided (coordinator and owner): durable. Done in A.3, see Deviation 6.
3. **Fields at the top level vs inside `flow`** (Deviation 3). Recommendation: top level, as built.

### What was verified while planning, and what was not

Re-verified after the durable-marker change (A.3 now also edits `service_runs.go`): all eight groups applied from the final plan file to a fresh copy of main's files, the result identical to the tested scratch tree, then `go vet`, `gofmt -l`, the whole `internal/flows/...` suite and `go test ./internal/server -run '(?i)flow'` (results in the hand-back).

Verified on a scratch copy of main d87930dfc (`git archive HEAD go.mod go.sum internal prompts ui`) with `GOTOOLCHAIN=go1.27.4`: every new file, test and edit group above was applied with `apply-plan-edits.mjs` (all `ok`), then `go vet ./internal/flows ./internal/tools ./internal/server` (clean), `gofmt -l` on every touched file (clean), `go test ./internal/flows/...` (ok, 122 s), `go test ./internal/tools -run '(?i)(flow|mission)'` (ok), `go test ./internal/server -run '(?i)flow'` (ok, 53 s) with the new tests among them, and `node scripts/test-easydrag-fixture-facts.mjs` plus `node --check` of the fixture. The measured costs in A.4 are from that run.

Not verified: the red steps were derived from the compiler's messages and the code, not run (the scratch copy had the implementation in place); `npm run test:easydrag:t0`, `node scripts/test-easydrag.mjs`, `go test ./ui` and `cmd/flowsdoc` were not run (the scratch copy has no scripts or `cmd`); the browser tests were not run (Part A changes no shipped UI); the edit groups were applied to the LF files of the scratch copy; for the CRLF fixture of a Windows checkout the group was applied to a CRLF copy of main's fixture (every line still ended in CRLF, the Node test passed against it), and A.6 step 4 checks the line endings again in the worktree.
