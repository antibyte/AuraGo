package flows

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// addRunAt stores a run of f with the given mode, status and start time.
func addRunAt(t *testing.T, s *Store, f *Flow, id string, mode RunMode, status RunStatus, startedAt time.Time) {
	t.Helper()
	rec := RunRecord{ID: id, FlowID: f.ID, Mode: mode, TriggerNode: f.Nodes[0].ID, Status: status, StartedAt: startedAt}
	if err := s.CreateRun(context.Background(), rec, nil); err != nil {
		t.Fatalf("CreateRun(%s): %v", id, err)
	}
}

func countRows(t *testing.T, s *Store, table string) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// LastLiveRuns drives from the flows table: a flow with only test runs or with no
// runs at all has no entry, and a newer test run never hides an older live one.
func TestStoreLastLiveRunsOnlyReportsFlowsWithNonTestRuns(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	mixed := createStoredFlow(t, s, "flow_aaaaaaaabp")
	testOnly := createStoredFlow(t, s, "flow_aaaaaaaabq")
	createStoredFlow(t, s, "flow_aaaaaaaabr") // no runs
	addRunAt(t, s, mixed, "run_aaaaaaaabpa", ModeLive, RunSuccess, storeNow.Add(-2*time.Minute))
	addRunAt(t, s, mixed, "run_aaaaaaaabpb", ModeAgent, RunError, storeNow.Add(-time.Minute))
	addRunAt(t, s, mixed, "run_aaaaaaaabpc", ModeTest, RunSuccess, storeNow)
	addRunAt(t, s, testOnly, "run_aaaaaaaabqa", ModeTest, RunSuccess, storeNow)

	last, err := s.LastLiveRuns(ctx)
	if err != nil || len(last) != 1 || last[mixed.ID].ID != "run_aaaaaaaabpb" || last[mixed.ID].Mode != ModeAgent ||
		last[mixed.ID].Status != RunError || !last[mixed.ID].StartedAt.Equal(storeNow.Add(-time.Minute)) {
		t.Fatalf("LastLiveRuns = %+v, %v", last, err)
	}
	if empty, err := openTestStore(t).LastLiveRuns(ctx); err != nil || len(empty) != 0 {
		t.Fatalf("LastLiveRuns on an empty store = %v, %v", empty, err)
	}
}

// Only GetRun reads trigger_data_json: the list queries would otherwise load up to
// MaxStoredOutputBytes per run (200 runs per ListRuns page).
func TestStoreRunListsLeaveTriggerDataToGetRun(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := createStoredFlow(t, s, "flow_aaaaaaaabt")
	rec := RunRecord{ID: "run_aaaaaaaabta", FlowID: f.ID, Revision: 3, Mode: ModeLive, TriggerNode: f.Nodes[0].ID,
		TriggerType: "webhook", TriggerData: map[string]any{"body": strings.Repeat("a", MaxStoredOutputBytes-11)},
		Status: RunQueued, StartedAt: storeNow, ParentRunID: "run_parent", ParentNodeID: "n_aaaaaaaa"}
	if err := s.CreateRun(ctx, rec, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishRun(ctx, rec.ID, RunResult{Status: RunError, ErrorCode: "X", ErrorMessage: "y", FinishedAt: storeNow.Add(time.Second), DurationMS: 1000}); err != nil {
		t.Fatal(err)
	}
	full, _, err := s.GetRun(ctx, rec.ID)
	if err != nil || len(full.TriggerData) != 1 {
		t.Fatalf("GetRun trigger data = %d keys, %v", len(full.TriggerData), err)
	}
	listed, err := s.ListRuns(ctx, f.ID, RunFilter{})
	if err != nil || len(listed) != 1 {
		t.Fatalf("ListRuns = %+v, %v", listed, err)
	}
	last, err := s.LastLiveRuns(ctx)
	if err != nil || len(last) != 1 {
		t.Fatalf("LastLiveRuns = %+v, %v", last, err)
	}
	for name, got := range map[string]RunRecord{"ListRuns": listed[0], "LastLiveRuns": last[f.ID]} {
		if len(got.TriggerData) != 0 {
			t.Errorf("%s returned %d trigger data keys, want none", name, len(got.TriggerData))
		}
		// Every other column is still there.
		if got.ID != rec.ID || got.FlowID != f.ID || got.Revision != 3 || got.Mode != ModeLive || got.TriggerNode != f.Nodes[0].ID ||
			got.TriggerType != "webhook" || got.Status != RunError || got.ErrorCode != "X" || got.ErrorMessage != "y" ||
			got.DurationMS != 1000 || got.ParentRunID != "run_parent" || got.ParentNodeID != "n_aaaaaaaa" ||
			!got.StartedAt.Equal(storeNow) || got.FinishedAt == nil || !got.FinishedAt.Equal(storeNow.Add(time.Second)) {
			t.Errorf("%s = %+v", name, got)
		}
	}
}

// A write for a row whose parent is gone reports the typed not-found error of
// that parent, not the driver's raw foreign-key failure.
func TestStoreWritesForMissingParentsReturnTypedNotFoundErrors(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	huge := strings.Repeat("z", 10000)
	bounded := func(name string, err, want error) {
		t.Helper()
		if msg := fmt.Sprint(err); !errors.Is(err, want) || len(msg) > 300 {
			t.Errorf("%s = %d bytes %.80q, want %v", name, len(msg), msg, want)
		}
	}

	// A run that does not exist: SaveStep.
	step := StepRecord{NodeID: "n_aaaaaaaa", Status: StepSuccess}
	bounded("SaveStep(missing run)", s.SaveStep(ctx, "run_missing", 1, step), ErrRunNotFound)
	bounded("SaveStep(huge run id)", s.SaveStep(ctx, "run_"+huge, 1, step), ErrRunNotFound)

	// A flow that does not exist: CreateRun, PutTestData, UpsertTimer, ReplaceTimers.
	rec := RunRecord{ID: "run_aaaaaaaabua", FlowID: "flow_missing00", Mode: ModeLive, Status: RunQueued, StartedAt: storeNow}
	bounded("CreateRun(missing flow)", s.CreateRun(ctx, rec, nil), ErrNotFound)
	rec.FlowID = "flow_" + huge
	bounded("CreateRun(huge flow id)", s.CreateRun(ctx, rec, nil), ErrNotFound)
	bounded("PutTestData(missing flow)",
		s.PutTestData(ctx, "flow_missing00", "n_aaaaaaaa", TestDataPinned, map[string]any{"a": 1.0}, storeNow), ErrNotFound)
	timer := TimerRecord{FlowID: "flow_missing00", NodeID: "n_aaaaaaaa", FireAt: storeNow.Add(time.Hour)}
	bounded("UpsertTimer(missing flow)", s.UpsertTimer(ctx, timer), ErrNotFound)
	bounded("ReplaceTimers(missing flow)", s.ReplaceTimers(ctx, "flow_missing00", []TimerRecord{timer}), ErrNotFound)
	// Clearing the timers of a flow that is gone is already satisfied.
	if err := s.ReplaceTimers(ctx, "flow_missing00", nil); err != nil {
		t.Errorf("ReplaceTimers(missing flow, no timers) = %v, want nil", err)
	}
	for _, table := range []string{"flow_runs", "flow_run_steps", "flow_test_data", "flow_timers"} {
		if n := countRows(t, s, table); n != 0 {
			t.Errorf("%s holds %d rows after the failed writes", table, n)
		}
	}

	// A flow that is deleted after its run was created: the cascade removes the run.
	f := createStoredFlow(t, s, "flow_aaaaaaaabu")
	addLiveRun(t, s, f, "run_aaaaaaaabub")
	if err := s.SaveStep(ctx, "run_aaaaaaaabub", 1, step); err != nil {
		t.Fatalf("SaveStep(existing run) = %v", err)
	}
	if err := s.DeleteFlow(ctx, f.ID); err != nil {
		t.Fatal(err)
	}
	bounded("SaveStep(run of a deleted flow)", s.SaveStep(ctx, "run_aaaaaaaabub", 2, step), ErrRunNotFound)
	if n := countRows(t, s, "flow_run_steps"); n != 0 {
		t.Errorf("flow_run_steps holds %d rows after the flow was deleted", n)
	}
}

func TestStoreRejectsZeroTimes(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := createStoredFlow(t, s, "flow_aaaaaaaabw")

	rec := RunRecord{ID: "run_aaaaaaaabwa", FlowID: f.ID, Mode: ModeLive, Status: RunQueued}
	if err := s.CreateRun(ctx, rec, nil); err == nil || !strings.Contains(err.Error(), "started_at") {
		t.Fatalf("CreateRun(zero StartedAt) = %v, want a started_at error", err)
	}
	if n := countRows(t, s, "flow_runs"); n != 0 {
		t.Fatalf("flow_runs holds %d rows after the rejected CreateRun", n)
	}

	keep := []TimerRecord{{NodeID: "n_aaaaaaaa", FireAt: storeNow.Add(time.Hour)}}
	if err := s.ReplaceTimers(ctx, f.ID, keep); err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("n", 10000)
	bad := []TimerRecord{{NodeID: "n_aaaaaaab", FireAt: storeNow}, {NodeID: long}}
	if err := s.ReplaceTimers(ctx, f.ID, bad); err == nil || !strings.Contains(err.Error(), "fire_at") || len(err.Error()) > 300 {
		t.Fatalf("ReplaceTimers(zero FireAt) = %.120v, want a bounded fire_at error", err)
	}
	if list, _ := s.ListTimers(ctx); len(list) != 1 || list[0].NodeID != "n_aaaaaaaa" {
		t.Fatalf("a rejected ReplaceTimers must leave the old timers: %+v", list)
	}
	if err := s.UpsertTimer(ctx, TimerRecord{FlowID: f.ID, NodeID: "n_aaaaaaab"}); err == nil || !strings.Contains(err.Error(), "fire_at") {
		t.Fatalf("UpsertTimer(zero FireAt) = %v, want a fire_at error", err)
	}
	if list, _ := s.ListTimers(ctx); len(list) != 1 {
		t.Fatalf("a rejected UpsertTimer must store nothing: %+v", list)
	}
}

// A test run's document is a convenience for the run view. One that is too big to
// read back (ParseFlow refuses documents above MaxDocumentBytes) is not stored, and
// it must not fail the run or make the run view show a published version that
// merely shares the draft's revision number.
func TestStoreCreateRunSkipsAnOversizedTestDocument(t *testing.T) {
	s := openTestStore(t)
	var logs bytes.Buffer
	s.logger = slog.New(slog.NewTextHandler(&logs, nil))
	ctx := context.Background()
	f := createStoredFlow(t, s, "flow_aaaaaaaabx")
	if _, err := s.Publish(ctx, f.ID, 1, storeNow); err != nil { // version 1 exists
		t.Fatal(err)
	}
	big := sampleFlow(f.ID)
	big.Name = strings.Repeat("n", MaxDocumentBytes)
	rec := RunRecord{ID: "run_aaaaaaaabxa", FlowID: f.ID, Revision: 1, Mode: ModeTest, TriggerNode: f.Nodes[0].ID,
		Status: RunQueued, StartedAt: storeNow}
	if err := s.CreateRun(ctx, rec, big); err != nil {
		t.Fatalf("CreateRun with an oversized document = %v, want the run to be recorded", err)
	}
	var stored int
	if err := s.db.QueryRowContext(ctx, `SELECT length(doc_json) FROM flow_runs WHERE id = ?`, rec.ID).Scan(&stored); err != nil || stored != 0 {
		t.Fatalf("doc_json holds %d bytes, %v; want none", stored, err)
	}
	if !strings.Contains(logs.String(), "too large") || strings.Contains(logs.String(), strings.Repeat("n", 100)) {
		t.Fatalf("expected one bounded warning, got %.200q", logs.String())
	}
	if _, err := s.GetRunDoc(ctx, rec.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetRunDoc(test run without a stored document) = %v, want ErrNotFound", err)
	}
	// A live run of the same revision still resolves to the published version.
	live := RunRecord{ID: "run_aaaaaaaabxb", FlowID: f.ID, Revision: 1, Mode: ModeLive, Status: RunQueued, StartedAt: storeNow}
	if err := s.CreateRun(ctx, live, nil); err != nil {
		t.Fatal(err)
	}
	if doc, err := s.GetRunDoc(ctx, live.ID); err != nil || doc.Name != "Sample" {
		t.Fatalf("GetRunDoc(live) = %+v, %v", doc, err)
	}
}

func TestStorePruneRunsRemovesTheStepsOfPrunedRuns(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := createStoredFlow(t, s, "flow_aaaaaaaaby")
	day := 24 * time.Hour
	add := func(id string, age time.Duration) {
		t.Helper()
		addRunAt(t, s, f, id, ModeLive, RunSuccess, storeNow.Add(-age))
		for seq := 1; seq <= 2; seq++ {
			if err := s.SaveStep(ctx, id, seq, StepRecord{NodeID: f.Nodes[0].ID, Status: StepSuccess}); err != nil {
				t.Fatal(err)
			}
		}
	}
	add("run_aaaaaaaabya", 40*day) // beyond the retention
	add("run_aaaaaaaabyb", 3*day)  // beyond the per-flow cap
	add("run_aaaaaaaabyc", 2*day)
	add("run_aaaaaaaabyd", day)
	if n := countRows(t, s, "flow_run_steps"); n != 8 {
		t.Fatalf("steps before pruning = %d", n)
	}
	if n, err := s.PruneRuns(ctx, 30, 2, storeNow); err != nil || n != 2 {
		t.Fatalf("PruneRuns = %d, %v", n, err)
	}
	if n := countRows(t, s, "flow_run_steps"); n != 4 {
		t.Fatalf("steps after pruning = %d, want the 4 steps of the 2 remaining runs", n)
	}
	var orphans int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM flow_run_steps WHERE run_id IN ('run_aaaaaaaabya', 'run_aaaaaaaabyb')`).Scan(&orphans); err != nil || orphans != 0 {
		t.Fatalf("steps of pruned runs = %d, %v", orphans, err)
	}
}

// Active runs are never pruned (not even beyond the retention) and do not use up
// the per-flow cap of finished runs, however new they are.
func TestStorePruneRunsKeepsActiveRunsOutOfTheCap(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := createStoredFlow(t, s, "flow_aaaaaaaabz")
	day := 24 * time.Hour
	addRunAt(t, s, f, "run_aaaaaaaabza", ModeLive, RunSuccess, storeNow.Add(-3*day)) // over the cap
	addRunAt(t, s, f, "run_aaaaaaaabzb", ModeLive, RunError, storeNow.Add(-2*day))
	addRunAt(t, s, f, "run_aaaaaaaabzc", ModeLive, RunCancelled, storeNow.Add(-day))
	addRunAt(t, s, f, "run_aaaaaaaabzd", ModeLive, RunQueued, storeNow)
	addRunAt(t, s, f, "run_aaaaaaaabze", ModeLive, RunWaiting, storeNow)
	addRunAt(t, s, f, "run_aaaaaaaabzf", ModeLive, RunRunning, storeNow)
	addRunAt(t, s, f, "run_aaaaaaaabzg", ModeLive, RunQueued, storeNow.Add(-100*day))
	addRunAt(t, s, f, "run_aaaaaaaabzh", ModeLive, RunWaiting, storeNow.Add(-100*day))

	n, err := s.PruneRuns(ctx, 30, 2, storeNow)
	if err != nil || n != 1 {
		t.Fatalf("PruneRuns = %d, %v; want only the oldest finished run", n, err)
	}
	if _, _, err := s.GetRun(ctx, "run_aaaaaaaabza"); !errors.Is(err, ErrRunNotFound) {
		t.Errorf("the oldest finished run must be pruned, got %v", err)
	}
	for _, id := range []string{"run_aaaaaaaabzb", "run_aaaaaaaabzc", "run_aaaaaaaabzd", "run_aaaaaaaabze", "run_aaaaaaaabzf", "run_aaaaaaaabzg", "run_aaaaaaaabzh"} {
		if _, _, err := s.GetRun(ctx, id); err != nil {
			t.Errorf("%s must remain: %v", id, err)
		}
	}
}

func TestStoreMarkInterruptedRunsCancelsWaitingRuns(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := createStoredFlow(t, s, "flow_aaaaaaaaca")
	boot := storeNow.Add(time.Minute)
	addRunAt(t, s, f, "run_aaaaaaaacaa", ModeLive, RunWaiting, storeNow)
	addRunAt(t, s, f, "run_aaaaaaaacab", ModeLive, RunWaiting, boot) // started by the new process
	addRunAt(t, s, f, "run_aaaaaaaacac", ModeLive, RunError, storeNow)
	n, err := s.MarkInterruptedRuns(ctx, boot, boot.Add(time.Second))
	if err != nil || n != 1 {
		t.Fatalf("MarkInterruptedRuns = %d, %v", n, err)
	}
	got, _, err := s.GetRun(ctx, "run_aaaaaaaacaa")
	if err != nil || got.Status != RunCancelled || got.ErrorCode != "FLOW_RESTARTED" || got.FinishedAt == nil ||
		!got.FinishedAt.Equal(boot.Add(time.Second)) {
		t.Fatalf("interrupted waiting run = %+v, %v", got, err)
	}
	if fresh, _, _ := s.GetRun(ctx, "run_aaaaaaaacab"); fresh.Status != RunWaiting {
		t.Fatalf("waiting run of the new process = %+v", fresh)
	}
	if done, _, _ := s.GetRun(ctx, "run_aaaaaaaacac"); done.Status != RunError || done.ErrorCode != "" {
		t.Fatalf("finished run = %+v", done)
	}
}
