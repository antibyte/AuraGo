package server

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/flows"
	"aurago/internal/i18n"
	"aurago/internal/planner"
	"aurago/internal/tools"
	"aurago/ui"
)

// c15Env is a server with Mission Control, a mission history, a planner database and a
// desktop hub whose events the test reads.
type c15Env struct {
	s      *Server
	bridge flowMissionBridge
	hist   *sql.DB
	events <-chan desktop.Event
}

func c15NewEnv(t *testing.T) *c15Env {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{}
	cfg.Tools.Missions.Enabled = true
	tools.ConfigureRuntimePermissions(tools.RuntimePermissionsFromConfig(cfg))
	mm := tools.NewMissionManagerV2(dir, nil)
	t.Cleanup(mm.Stop)
	hist, err := tools.InitMissionHistoryDB(filepath.Join(dir, "history.db"))
	if err != nil {
		t.Fatalf("history db: %v", err)
	}
	t.Cleanup(func() { _ = hist.Close() })
	mm.SetHistoryDB(hist)
	plannerDB, err := planner.InitDB(filepath.Join(dir, "planner.db"))
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
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), MissionManagerV2: mm,
		PlannerDB: plannerDB, DesktopHub: hub}
	return &c15Env{s: s, bridge: flowMissionBridge{s: s}, hist: hist, events: events}
}

// c15Mission creates an enabled flow mission with a manual trigger.
func (e *c15Env) c15Mission(t *testing.T, flowID, name string) string {
	t.Helper()
	id, err := e.bridge.CreateFlowMission(flowID, name)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.bridge.SyncFlowMission(id, name, []flows.TriggerBinding{{NodeID: "n_aaaaaaaa", Kind: flows.BindingManual}}); err != nil {
		t.Fatal(err)
	}
	if err := e.bridge.SetFlowMissionEnabled(id, true); err != nil {
		t.Fatal(err)
	}
	return id
}

// c15Drain returns the desktop events sent so far.
func (e *c15Env) c15Drain() []desktop.Event {
	var out []desktop.Event
	for {
		select {
		case ev := <-e.events:
			out = append(out, ev)
		default:
			return out
		}
	}
}

func c15Count(events []desktop.Event, typ string) int {
	n := 0
	for _, ev := range events {
		if ev.Type == typ {
			n++
		}
	}
	return n
}

func (e *c15Env) c15Issues(t *testing.T) []planner.OperationalIssueListItem {
	t.Helper()
	page, err := planner.ListOperationalIssues(e.s.PlannerDB, planner.OperationalIssueListFilter{Status: "all", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	return page.Items
}

func (e *c15Env) c15Run(t *testing.T, id string) *tools.MissionRun {
	t.Helper()
	run, err := tools.GetMissionRun(e.hist, id)
	if err != nil {
		t.Fatalf("history run %s: %v", id, err)
	}
	return run
}

// c15LogBuffer collects log output from any goroutine.
type c15LogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *c15LogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *c15LogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func c15Logger(buf *c15LogBuffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// c15LoadI18n loads the real translations, ui/lang/easydrag included.
func c15LoadI18n() {
	i18n.Load(ui.Content, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// c15Failure is a started live run of flowID that failed.
func c15Failure(flowID, runID, notify string) flows.RunFinishedInfo {
	return flows.RunFinishedInfo{Started: true, FlowName: "Bericht", NotifyOnError: notify,
		Record: flows.RunRecord{ID: runID, FlowID: flowID},
		Result: flows.RunResult{Status: flows.RunError, ErrorCode: "FLOW_TOOL_ERROR", ErrorMessage: "kaputt"}}
}

// A run that never started (cancelled while queued, or ended by a shutdown) leaves the
// mission's status, counters and history alone, records no planner issue and sends no
// notification; open editors are still told to refresh.
func TestC15UnstartedRunLeavesMissionControlAlone(t *testing.T) {
	e := c15NewEnv(t)
	id := e.c15Mission(t, "flow_c15aaaaaaa", "Bericht")
	histID := e.bridge.FlowRunStarted(id, flows.RunRecord{ID: "run_c15started1", FlowID: "flow_c15aaaaaaa", TriggerType: "manual"})
	if histID == "" {
		t.Fatal("the started run has no history entry")
	}
	before, _ := e.s.MissionManagerV2.Get(id)
	e.c15Drain()
	for _, res := range []flows.RunResult{
		{Status: flows.RunCancelled, ErrorCode: "FLOW_CANCELLED", ErrorMessage: "the run was stopped"},
		{Status: flows.RunError, ErrorCode: "FLOW_SHUTDOWN", ErrorMessage: "the server stopped"},
	} {
		e.bridge.FlowRunFinished(flows.RunFinishedInfo{MissionID: id, FlowName: "Bericht", NotifyOnError: "desktop",
			Record: flows.RunRecord{ID: "run_c15queued01", FlowID: "flow_c15aaaaaaa"}, Result: res, Outputs: map[string]any{}})
	}
	after, _ := e.s.MissionManagerV2.Get(id)
	if after.Status != tools.MissionStatusRunning || after.RunCount != before.RunCount ||
		after.LastResult != before.LastResult || after.LastOutput != before.LastOutput {
		t.Fatalf("mission changed by unstarted runs: before %+v, after %+v", before, after)
	}
	if run := e.c15Run(t, histID); run.Status != "running" {
		t.Fatalf("the started run's history entry = %q", run.Status)
	}
	if issues := e.c15Issues(t); len(issues) != 0 {
		t.Fatalf("planner issues = %+v", issues)
	}
	events := e.c15Drain()
	if c15Count(events, "notification") != 0 || c15Count(events, "flows_changed") != 2 {
		t.Fatalf("events = %+v", events)
	}
	// The running slot of the started run is intact: its end releases it and counts.
	e.bridge.FlowRunFinished(flows.RunFinishedInfo{MissionID: id, HistoryID: histID, FlowName: "Bericht", Started: true,
		Record: flows.RunRecord{ID: "run_c15started1", FlowID: "flow_c15aaaaaaa"}, Result: flows.RunResult{Status: flows.RunSuccess}})
	if m, _ := e.s.MissionManagerV2.Get(id); m.Status != tools.MissionStatusIdle || m.RunCount != before.RunCount+1 {
		t.Fatalf("mission after the started run = %+v", m)
	}
}

// A started run whose flow and mission are gone (no mission id, no name) still completes
// its history entry, records a planner issue named after the flow id and notifies.
func TestC15StartedRunWithoutMission(t *testing.T) {
	c15LoadI18n()
	e := c15NewEnv(t)
	histID, err := tools.RecordMissionStart(e.hist, "mission_c15gone", "Gone", "manual", "{}")
	if err != nil {
		t.Fatal(err)
	}
	e.bridge.FlowRunFinished(flows.RunFinishedInfo{Started: true, HistoryID: histID,
		Record: flows.RunRecord{ID: "run_c15nomission", FlowID: "flow_c15gone0001"},
		Result: flows.RunResult{Status: flows.RunError, ErrorCode: "FLOW_TOOL_ERROR", ErrorMessage: "kaputt"}})
	if run := e.c15Run(t, histID); run.Status != "error" || !strings.Contains(run.ErrorMsg, "kaputt") {
		t.Fatalf("history entry = %+v", run)
	}
	issues := e.c15Issues(t)
	if len(issues) != 1 || issues[0].Source != "flow" || !strings.Contains(issues[0].Title, "Flow flow_c15gone0001 failed") {
		t.Fatalf("planner issues = %+v", issues)
	}
	events := e.c15Drain()
	if n := c15Count(events, "notification"); n != 1 {
		t.Fatalf("notifications = %d", n)
	}
	for _, ev := range events {
		if payload, ok := ev.Payload.(map[string]any); ok && ev.Type == "notification" &&
			payload["message"] != "flow_c15gone0001: FLOW_TOOL_ERROR: kaputt" {
			t.Fatalf("notification = %+v", payload)
		}
	}
	// Without even a history entry nothing is left to record but the issue.
	e.bridge.FlowRunFinished(flows.RunFinishedInfo{Started: true,
		Record: flows.RunRecord{ID: "run_c15nomission2", FlowID: "flow_c15gone0001"}, Result: flows.RunResult{Status: flows.RunSuccess}})
	if issues := e.c15Issues(t); len(issues) != 1 || issues[0].Status != "done" {
		t.Fatalf("planner issues after a success = %+v", issues)
	}
}

// Hook errors are logged by kind: a flow that is gone at Debug, a timeout and anything
// else at Warn, the error text bounded.
func TestC15HookErrorsAreLoggedByKind(t *testing.T) {
	logs := &c15LogBuffer{}
	h := flowMissionHooks{s: &Server{Logger: c15Logger(logs)}}
	h.logError("gone", "m1", fmt.Errorf("lookup: %w", flows.ErrNotFound))
	h.logError("busy", "m2", fmt.Errorf("lock: %w", context.DeadlineExceeded))
	h.logError("other", "m3", errors.New(strings.Repeat("x", 5000)))
	out := logs.String()
	for _, want := range []string{`level=DEBUG msg="gone: the flow is gone already" mission_id=m1`,
		`level=WARN msg="busy: the flow stayed busy" mission_id=m2 timeout=2m0s`, `level=WARN msg=other mission_id=m3`} {
		if !strings.Contains(out, want) {
			t.Errorf("logs lack %q:\n%s", want, out)
		}
	}
	if strings.Count(out, "x") > flowErrorRunes+10 {
		t.Fatalf("the error text is not bounded: %d", strings.Count(out, "x"))
	}
}

// The hooks reach the flow service: Mission Control's delete removes the flow, and a
// switch of a mission no flow holds is no error.
func TestC15HooksFollowTheFlowService(t *testing.T) {
	e := c15NewEnv(t)
	logs := &c15LogBuffer{}
	e.s.Logger = c15Logger(logs)
	store, err := flows.OpenStore(filepath.Join(t.TempDir(), "flows.db"), e.s.Logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	reg := flows.NewRegistry()
	if err := flows.RegisterCatalog(reg, flows.StaticEnv{}); err != nil {
		t.Fatal(err)
	}
	svc := flows.NewService(store, reg, nil, e.bridge, flows.ServiceConfig{}, e.s.Logger)
	t.Cleanup(func() { _ = svc.Shutdown(context.Background()) })
	e.s.Flows = svc
	hooks := flowMissionHooks{s: e.s}

	rec, err := svc.CreateFlow(context.Background(), flows.CreateRequest{Name: "Hooked"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := e.s.MissionManagerV2.Get(rec.MissionID); !ok {
		t.Fatal("the flow has no mission")
	}
	hooks.FlowEnabledChanged(rec.MissionID, false)
	hooks.FlowEnabledChanged("mission_c15nobody", true)
	if err := e.s.MissionManagerV2.DeleteFlowMission(rec.MissionID); err != nil {
		t.Fatal(err)
	}
	e.c15Drain()
	hooks.FlowMissionDeleted(rec.MissionID)
	if _, err := svc.GetFlow(context.Background(), rec.ID); !errors.Is(err, flows.ErrNotFound) {
		t.Fatalf("the flow survived its mission: %v", err)
	}
	if c15Count(e.c15Drain(), "flows_changed") != 1 {
		t.Fatal("open editors were not told about the delete")
	}
	if strings.Contains(logs.String(), "level=WARN") {
		t.Fatalf("unexpected warnings:\n%s", logs.String())
	}
}
