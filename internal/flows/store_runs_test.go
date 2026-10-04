package flows

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

func createStoredFlow(t *testing.T, s *Store, id string) *Flow {
	t.Helper()
	f := sampleFlow(id)
	if _, err := s.CreateFlow(context.Background(), f, "", storeNow); err != nil {
		t.Fatalf("CreateFlow: %v", err)
	}
	return f
}

func TestStoreRunsAndSteps(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := createStoredFlow(t, s, "flow_aaaaaaaaba")
	rec := RunRecord{ID: "run_aaaaaaaaaaaa", FlowID: f.ID, Mode: ModeTest, TriggerNode: f.Nodes[0].ID,
		TriggerType: "manual", TriggerData: map[string]any{"x": 1.0}, Status: RunQueued, StartedAt: storeNow}
	if err := s.CreateRun(ctx, rec, f); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := s.SetRunStatus(ctx, rec.ID, RunRunning); err != nil {
		t.Fatalf("SetRunStatus: %v", err)
	}
	step := StepRecord{NodeID: f.Nodes[1].ID, NodeKey: "echo", Attempt: 1, Status: StepSuccess,
		StartedAt: storeNow, FinishedAt: storeNow.Add(time.Second), DurationMS: 1000,
		Params: map[string]any{"value": "x"}, Output: map[string]any{"value": "x"}, Ports: []string{PortOut}, ItemCount: 2}
	if err := s.SaveStep(ctx, rec.ID, 3, step); err != nil {
		t.Fatalf("SaveStep: %v", err)
	}
	if err := s.SaveStep(ctx, rec.ID, 2, StepRecord{NodeID: f.Nodes[0].ID, NodeKey: "start", Status: StepSuccess}); err != nil {
		t.Fatalf("SaveStep trigger: %v", err)
	}
	if err := s.FinishRun(ctx, rec.ID, RunResult{Status: RunError, ErrorCode: "X", ErrorMessage: "y",
		FinishedAt: storeNow.Add(2 * time.Second), DurationMS: 2000}); err != nil {
		t.Fatalf("FinishRun: %v", err)
	}
	got, steps, err := s.GetRun(ctx, rec.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.Status != RunError || got.ErrorCode != "X" || got.FinishedAt == nil || got.DurationMS != 2000 ||
		got.TriggerData["x"] != 1.0 || got.Mode != ModeTest {
		t.Fatalf("run = %+v", got)
	}
	if len(steps) != 2 || steps[0].NodeKey != "start" {
		t.Fatalf("steps = %+v", steps)
	}
	s1 := steps[1]
	if s1.Status != StepSuccess || s1.DurationMS != 1000 || s1.ItemCount != 2 || !reflect.DeepEqual(s1.Ports, []string{PortOut}) ||
		s1.Output["value"] != "x" || s1.Params["value"] != "x" || !s1.FinishedAt.Equal(storeNow.Add(time.Second)) {
		t.Fatalf("step = %+v", s1)
	}
	doc, err := s.GetRunDoc(ctx, rec.ID)
	if err != nil || len(doc.Nodes) != 2 {
		t.Fatalf("GetRunDoc(test run) = %+v, %v", doc, err)
	}
	if _, _, err := s.GetRun(ctx, "run_missing"); !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("GetRun(missing) = %v", err)
	}
}

func TestStoreRunDocForLiveRunsUsesVersion(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := createStoredFlow(t, s, "flow_aaaaaaaabb")
	if _, err := s.Publish(ctx, f.ID, 1, storeNow); err != nil {
		t.Fatal(err)
	}
	rec := RunRecord{ID: "run_aaaaaaaaaaab", FlowID: f.ID, Revision: 1, Mode: ModeLive, TriggerNode: f.Nodes[0].ID, Status: RunQueued, StartedAt: storeNow}
	if err := s.CreateRun(ctx, rec, f); err != nil {
		t.Fatal(err)
	}
	doc, err := s.GetRunDoc(ctx, rec.ID)
	if err != nil || doc.Name != "Sample" {
		t.Fatalf("GetRunDoc(live) = %+v, %v", doc, err)
	}
}

func TestStoreListRunsAndLastLive(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := createStoredFlow(t, s, "flow_aaaaaaaabc")
	add := func(id string, mode RunMode, status RunStatus, age time.Duration) {
		t.Helper()
		rec := RunRecord{ID: id, FlowID: f.ID, Mode: mode, TriggerNode: f.Nodes[0].ID, Status: RunQueued, StartedAt: storeNow.Add(-age)}
		if err := s.CreateRun(ctx, rec, nil); err != nil {
			t.Fatal(err)
		}
		if err := s.FinishRun(ctx, id, RunResult{Status: status, FinishedAt: storeNow}); err != nil {
			t.Fatal(err)
		}
	}
	add("run_aaaaaaaaaaba", ModeTest, RunSuccess, time.Minute)
	add("run_aaaaaaaaaabb", ModeLive, RunError, 2*time.Minute)
	add("run_aaaaaaaaaabc", ModeLive, RunSuccess, 3*time.Minute)

	ids := func(runs []RunRecord) []string {
		var out []string
		for _, r := range runs {
			out = append(out, r.ID)
		}
		return out
	}
	all, err := s.ListRuns(ctx, f.ID, RunFilter{})
	if err != nil || !reflect.DeepEqual(ids(all), []string{"run_aaaaaaaaaaba", "run_aaaaaaaaaabb", "run_aaaaaaaaaabc"}) {
		t.Fatalf("ListRuns = %v, %v", ids(all), err)
	}
	if tests, _ := s.ListRuns(ctx, f.ID, RunFilter{Mode: ModeTest}); len(tests) != 1 {
		t.Fatalf("tests = %v", ids(tests))
	}
	if errs, _ := s.ListRuns(ctx, f.ID, RunFilter{Status: RunError}); len(errs) != 1 || errs[0].ID != "run_aaaaaaaaaabb" {
		t.Fatalf("errors = %v", ids(errs))
	}
	if page, _ := s.ListRuns(ctx, f.ID, RunFilter{Limit: 2, Offset: 2}); len(page) != 1 {
		t.Fatalf("page = %v", ids(page))
	}
	last, err := s.LastLiveRuns(ctx)
	if err != nil || last[f.ID].ID != "run_aaaaaaaaaabb" {
		t.Fatalf("LastLiveRuns = %+v, %v", last, err)
	}
}

func TestStoreMarkInterruptedRuns(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := createStoredFlow(t, s, "flow_aaaaaaaabd")
	for i, status := range []RunStatus{RunQueued, RunRunning, RunSuccess} {
		id := "run_aaaaaaaaaca" + string(rune('a'+i))
		if err := s.CreateRun(ctx, RunRecord{ID: id, FlowID: f.ID, Mode: ModeLive, Status: status, StartedAt: storeNow}, nil); err != nil {
			t.Fatal(err)
		}
	}
	// A run started by the new process (at or after the boot time) must survive.
	boot := storeNow.Add(time.Minute)
	if err := s.CreateRun(ctx, RunRecord{ID: "run_aaaaaaaaacad", FlowID: f.ID, Mode: ModeLive, Status: RunRunning, StartedAt: boot}, nil); err != nil {
		t.Fatal(err)
	}
	n, err := s.MarkInterruptedRuns(ctx, boot, boot.Add(time.Second))
	if err != nil || n != 2 {
		t.Fatalf("MarkInterruptedRuns = %d, %v", n, err)
	}
	got, _, _ := s.GetRun(ctx, "run_aaaaaaaaacab")
	if got.Status != RunCancelled || got.ErrorCode != "FLOW_RESTARTED" {
		t.Fatalf("interrupted run = %+v", got)
	}
	if fresh, _, _ := s.GetRun(ctx, "run_aaaaaaaaacad"); fresh.Status != RunRunning {
		t.Fatalf("run of the new process = %+v", fresh)
	}
}

func TestStorePruneRuns(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := createStoredFlow(t, s, "flow_aaaaaaaabe")
	day := 24 * time.Hour
	add := func(id string, age time.Duration, status RunStatus) {
		t.Helper()
		if err := s.CreateRun(ctx, RunRecord{ID: id, FlowID: f.ID, Mode: ModeLive, Status: status, StartedAt: storeNow.Add(-age)}, nil); err != nil {
			t.Fatal(err)
		}
		if err := s.SaveStep(ctx, id, 1, StepRecord{NodeID: f.Nodes[0].ID, Status: StepSuccess}); err != nil {
			t.Fatal(err)
		}
	}
	add("run_old40", 40*day, RunSuccess)
	add("run_old35", 35*day, RunError)
	add("run_day02", 2*day, RunSuccess)
	add("run_day01", day, RunSuccess)
	add("run_now00", 0, RunSuccess)
	add("run_busy50", 50*day, RunRunning)
	n, err := s.PruneRuns(ctx, 30, 2, storeNow)
	if err != nil || n != 3 {
		t.Fatalf("PruneRuns = %d, %v", n, err)
	}
	for _, id := range []string{"run_old40", "run_old35", "run_day02"} {
		if _, _, err := s.GetRun(ctx, id); !errors.Is(err, ErrRunNotFound) {
			t.Errorf("%s must be pruned, got %v", id, err)
		}
	}
	for _, id := range []string{"run_day01", "run_now00", "run_busy50"} {
		if _, _, err := s.GetRun(ctx, id); err != nil {
			t.Errorf("%s must remain: %v", id, err)
		}
	}
}

func TestStoreTestDataAndTimers(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := createStoredFlow(t, s, "flow_aaaaaaaabf")
	nodeID := f.Nodes[0].ID
	if _, ok, err := s.GetTestData(ctx, f.ID, nodeID, TestDataTriggerSample); ok || err != nil {
		t.Fatalf("missing test data = %v, %v", ok, err)
	}
	if err := s.PutTestData(ctx, f.ID, nodeID, TestDataTriggerSample, map[string]any{"a": 1.0}, storeNow); err != nil {
		t.Fatal(err)
	}
	if err := s.PutTestData(ctx, f.ID, nodeID, TestDataTriggerSample, map[string]any{"a": 2.0}, storeNow); err != nil {
		t.Fatal(err)
	}
	data, ok, err := s.GetTestData(ctx, f.ID, nodeID, TestDataTriggerSample)
	if !ok || err != nil || data["a"] != 2.0 {
		t.Fatalf("test data = %v %v %v", data, ok, err)
	}

	timers := []TimerRecord{
		{FlowID: f.ID, NodeID: "n_aaaaaaaa", FireAt: storeNow.Add(2 * time.Hour)},
		{FlowID: f.ID, NodeID: "n_aaaaaaab", FireAt: storeNow.Add(time.Hour), Repeat: RepeatYearly},
	}
	if err := s.ReplaceTimers(ctx, f.ID, timers); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListTimers(ctx)
	if err != nil || len(list) != 2 || list[0].NodeID != "n_aaaaaaab" || list[0].Repeat != RepeatYearly || !list[0].FireAt.Equal(storeNow.Add(time.Hour)) {
		t.Fatalf("ListTimers = %+v, %v", list, err)
	}
	list[0].FireAt = storeNow.Add(3 * time.Hour)
	if err := s.UpsertTimer(ctx, list[0]); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteTimer(ctx, f.ID, "n_aaaaaaaa"); err != nil {
		t.Fatal(err)
	}
	if list, _ = s.ListTimers(ctx); len(list) != 1 || !list[0].FireAt.Equal(storeNow.Add(3*time.Hour)) {
		t.Fatalf("after upsert/delete = %+v", list)
	}
	if err := s.DeleteFlow(ctx, f.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ = s.ListTimers(ctx); len(list) != 0 {
		t.Fatalf("timers must be deleted with their flow, got %+v", list)
	}
}

// addLiveRun stores a queued live run of f started at storeNow.
func addLiveRun(t *testing.T, s *Store, f *Flow, id string) {
	t.Helper()
	rec := RunRecord{ID: id, FlowID: f.ID, Mode: ModeLive, TriggerNode: f.Nodes[0].ID, Status: RunQueued, StartedAt: storeNow}
	if err := s.CreateRun(context.Background(), rec, nil); err != nil {
		t.Fatalf("CreateRun(%s): %v", id, err)
	}
}

// The step row has one column per truncation flag; a step must read back with
// exactly the flags it was saved with, and a re-save must overwrite them.
func TestStoreStepRoundTripKeepsBothTruncationFlags(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := createStoredFlow(t, s, "flow_aaaaaaaabg")
	addLiveRun(t, s, f, "run_aaaaaaaaabga")
	cases := []struct{ params, output bool }{{true, true}, {true, false}, {false, true}, {false, false}}
	for i, c := range cases {
		step := StepRecord{NodeID: f.Nodes[1].ID, NodeKey: "echo", Attempt: 1, Status: StepSuccess,
			Params: map[string]any{"p": float64(i)}, ParamsTruncated: c.params,
			Output: map[string]any{"o": float64(i)}, OutputTruncated: c.output}
		if c.params {
			step.Params = map[string]any{"_preview": `{"value":"xxx`}
		}
		if c.output {
			step.Output = map[string]any{"_preview": `{"value":"yyy`}
		}
		if err := s.SaveStep(ctx, "run_aaaaaaaaabga", i+1, step); err != nil {
			t.Fatalf("SaveStep(%d): %v", i+1, err)
		}
	}
	_, steps, err := s.GetRun(ctx, "run_aaaaaaaaabga")
	if err != nil || len(steps) != len(cases) {
		t.Fatalf("GetRun = %d steps, %v", len(steps), err)
	}
	for i, c := range cases {
		if steps[i].ParamsTruncated != c.params || steps[i].OutputTruncated != c.output {
			t.Errorf("step %d flags = params %v, output %v; want %v, %v", i+1,
				steps[i].ParamsTruncated, steps[i].OutputTruncated, c.params, c.output)
		}
	}
	if steps[0].Params["_preview"] != `{"value":"xxx` || steps[0].Output["_preview"] != `{"value":"yyy` {
		t.Fatalf("truncated step = %+v", steps[0])
	}
	// Saving seq 1 again without the flags clears them.
	if err := s.SaveStep(ctx, "run_aaaaaaaaabga", 1, StepRecord{NodeID: f.Nodes[1].ID, Status: StepSuccess}); err != nil {
		t.Fatal(err)
	}
	if _, steps, _ = s.GetRun(ctx, "run_aaaaaaaaabga"); steps[0].ParamsTruncated || steps[0].OutputTruncated {
		t.Fatalf("re-saved step kept its flags: %+v", steps[0])
	}
}

// The trigger data is a webhook or mail payload of up to MaxOutputBytes that is
// stored for every run, so the header keeps a preview beyond MaxStoredOutputBytes.
func TestStoreCreateRunBoundsTriggerData(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := createStoredFlow(t, s, "flow_aaaaaaaabh")
	// {"body":"<n bytes>"} encodes to n+11 bytes.
	atLimit := strings.Repeat("a", MaxStoredOutputBytes-11)
	overLimit := atLimit + "a"
	multiByte := strings.Repeat("é", MaxStoredOutputBytes/2)
	cases := []struct {
		id, body  string
		truncated bool
	}{
		{"run_aaaaaaaabha", "small", false},
		{"run_aaaaaaaabhb", atLimit, false},
		{"run_aaaaaaaabhc", overLimit, true},
		{"run_aaaaaaaabhd", strings.Repeat("b", MaxOutputBytes/2), true},
		{"run_aaaaaaaabhe", multiByte, true},
	}
	for _, c := range cases {
		data := map[string]any{"body": c.body}
		rec := RunRecord{ID: c.id, FlowID: f.ID, Mode: ModeLive, TriggerNode: f.Nodes[0].ID, TriggerData: data, Status: RunQueued, StartedAt: storeNow}
		if err := s.CreateRun(ctx, rec, nil); err != nil {
			t.Fatalf("CreateRun(%s): %v", c.id, err)
		}
		if data["body"] != c.body || len(data) != 1 {
			t.Errorf("%s: CreateRun modified the caller's trigger data", c.id)
		}
		var stored int
		if err := s.db.QueryRowContext(ctx, `SELECT length(trigger_data_json) FROM flow_runs WHERE id = ?`, c.id).Scan(&stored); err != nil {
			t.Fatal(err)
		}
		got, _, err := s.GetRun(ctx, c.id)
		if err != nil {
			t.Fatal(err)
		}
		if !c.truncated {
			if got.TriggerData["body"] != c.body || len(got.TriggerData) != 1 {
				t.Errorf("%s: trigger data was changed although it fits", c.id)
			}
			continue
		}
		preview, ok := got.TriggerData["_preview"].(string)
		if !ok || len(got.TriggerData) != 1 || len(preview) == 0 || len(preview) > storedPreviewBytes ||
			!utf8.ValidString(preview) || !strings.HasPrefix(preview, `{"body":"`) {
			t.Errorf("%s: stored trigger data = %d keys, preview of %d bytes (valid UTF-8: %v)", c.id,
				len(got.TriggerData), len(preview), utf8.ValidString(preview))
		}
		// The preview is JSON-encoded once more, which at most doubles special characters.
		if stored > 2*storedPreviewBytes+64 {
			t.Errorf("%s: trigger_data_json holds %d bytes, want a bounded preview", c.id, stored)
		}
	}
}

func TestStoreSetRunStatusAndFinishRunReportMissingRuns(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	if err := s.SetRunStatus(ctx, "run_missing", RunRunning); !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("SetRunStatus(missing) = %v, want ErrRunNotFound", err)
	}
	if err := s.FinishRun(ctx, "run_missing", RunResult{Status: RunSuccess, FinishedAt: storeNow}); !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("FinishRun(missing) = %v, want ErrRunNotFound", err)
	}
	// An id the caller controls must not blow up the message.
	huge := "run_" + strings.Repeat("x", 10000)
	for name, err := range map[string]error{
		"SetRunStatus": s.SetRunStatus(ctx, huge, RunRunning),
		"FinishRun":    s.FinishRun(ctx, huge, RunResult{Status: RunSuccess}),
		"GetRun":       func() error { _, _, err := s.GetRun(ctx, huge); return err }(),
		"GetRunDoc":    func() error { _, err := s.GetRunDoc(ctx, huge); return err }(),
	} {
		if msg := fmt.Sprint(err); !errors.Is(err, ErrRunNotFound) || len(msg) > 300 {
			t.Errorf("%s(huge id) = %d bytes: %.80q", name, len(msg), msg)
		}
	}

	f := createStoredFlow(t, s, "flow_aaaaaaaabi")
	addLiveRun(t, s, f, "run_aaaaaaaaabia")
	for i := 0; i < 2; i++ { // setting the status it already has is not "missing"
		if err := s.SetRunStatus(ctx, "run_aaaaaaaaabia", RunRunning); err != nil {
			t.Fatalf("SetRunStatus #%d: %v", i+1, err)
		}
	}
	// Deleting the flow cascades to its runs; finishing one afterwards is reported.
	if err := s.DeleteFlow(ctx, f.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishRun(ctx, "run_aaaaaaaaabia", RunResult{Status: RunSuccess}); !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("FinishRun(run of a deleted flow) = %v, want ErrRunNotFound", err)
	}
	var rows int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM flow_runs`).Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("flow_runs rows = %d, %v; a failed update must not create runs", rows, err)
	}
}

func TestStoreGetRunDocErrorsStayBounded(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := createStoredFlow(t, s, "flow_aaaaaaaabj")
	huge := "run_" + strings.Repeat("y", 10000)
	rec := RunRecord{ID: huge, FlowID: f.ID, Mode: ModeTest, TriggerNode: f.Nodes[0].ID, Status: RunQueued, StartedAt: storeNow}
	if err := s.CreateRun(ctx, rec, f); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE flow_runs SET doc_json = ? WHERE id = ?`, `{"schema": "not a number"}`, huge); err != nil {
		t.Fatal(err)
	}
	_, err := s.GetRunDoc(ctx, huge)
	msg := fmt.Sprint(err)
	if err == nil || len(msg) > 400 || strings.Contains(msg, huge) || !strings.Contains(msg, "run_yyy") {
		t.Fatalf("GetRunDoc(corrupt document) = %d bytes: %.120q", len(msg), msg)
	}
}

// Runs of one flow can share a started_at. Which one is "last" must not depend
// on insertion or scan order: like ListRuns, the highest id wins.
func TestStoreLastLiveRunsBreaksStartedAtTiesByID(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	asc := createStoredFlow(t, s, "flow_aaaaaaaabk")
	desc := createStoredFlow(t, s, "flow_aaaaaaaabl")
	for _, id := range []string{"run_aaaaaaaabka", "run_aaaaaaaabkb", "run_aaaaaaaabkc"} {
		addLiveRun(t, s, asc, id)
	}
	for _, id := range []string{"run_aaaaaaaablc", "run_aaaaaaaablb", "run_aaaaaaaabla"} {
		addLiveRun(t, s, desc, id)
	}
	for i := 0; i < 5; i++ {
		last, err := s.LastLiveRuns(ctx)
		if err != nil || len(last) != 2 || last[asc.ID].ID != "run_aaaaaaaabkc" || last[desc.ID].ID != "run_aaaaaaaablc" {
			t.Fatalf("LastLiveRuns = %v, %v", last, err)
		}
	}
	runs, err := s.ListRuns(ctx, asc.ID, RunFilter{})
	if err != nil || len(runs) != 3 || runs[0].ID != "run_aaaaaaaabkc" || runs[2].ID != "run_aaaaaaaabka" {
		t.Fatalf("ListRuns of tied runs = %+v, %v", runs, err)
	}
}

func TestStorePruneRunsBreaksStartedAtTiesByID(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := createStoredFlow(t, s, "flow_aaaaaaaabm")
	for _, id := range []string{"run_aaaaaaaabmb", "run_aaaaaaaabmc", "run_aaaaaaaabma"} {
		rec := RunRecord{ID: id, FlowID: f.ID, Mode: ModeLive, Status: RunSuccess, StartedAt: storeNow}
		if err := s.CreateRun(ctx, rec, nil); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := s.PruneRuns(ctx, 0, 1, storeNow); err != nil || n != 2 {
		t.Fatalf("PruneRuns = %d, %v", n, err)
	}
	if _, _, err := s.GetRun(ctx, "run_aaaaaaaabmc"); err != nil {
		t.Fatalf("the highest id of the tied runs must be kept: %v", err)
	}
}

// ReplaceTimers is one transaction that starts with its DELETE: a failure rolls
// everything back, and concurrent writers wait for the lock instead of failing.
func TestStoreReplaceTimersIsAtomic(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := createStoredFlow(t, s, "flow_aaaaaaaabn")
	keep := []TimerRecord{{NodeID: "n_aaaaaaaa", FireAt: storeNow.Add(time.Hour)}}
	if err := s.ReplaceTimers(ctx, f.ID, keep); err != nil {
		t.Fatal(err)
	}
	dup := []TimerRecord{{NodeID: "n_aaaaaaab", FireAt: storeNow}, {NodeID: "n_aaaaaaab", FireAt: storeNow}}
	if err := s.ReplaceTimers(ctx, f.ID, dup); err == nil {
		t.Fatal("ReplaceTimers with a duplicate node must fail")
	}
	list, err := s.ListTimers(ctx)
	if err != nil || len(list) != 1 || list[0].NodeID != "n_aaaaaaaa" {
		t.Fatalf("failed ReplaceTimers must leave the old timers: %+v, %v", list, err)
	}
	if err := s.ReplaceTimers(ctx, f.ID, nil); err != nil {
		t.Fatal(err)
	}
	if list, _ = s.ListTimers(ctx); len(list) != 0 {
		t.Fatalf("ReplaceTimers(nil) must clear the flow's timers: %+v", list)
	}
}

func TestStoreConcurrentRunAndTimerWritesNeverFailWithLockErrors(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := createStoredFlow(t, s, "flow_aaaaaaaabo")
	addLiveRun(t, s, f, "run_aaaaaaaaaboa")
	const workers, rounds = 6, 25
	errs := make(chan error, workers*rounds*4)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < rounds; i++ {
				at := storeNow.Add(time.Duration(i) * time.Minute)
				timers := []TimerRecord{{NodeID: "n_aaaaaaaa", FireAt: at}, {NodeID: "n_aaaaaaab", FireAt: at}}
				if err := s.ReplaceTimers(ctx, f.ID, timers); err != nil {
					errs <- fmt.Errorf("ReplaceTimers: %w", err)
				}
				step := StepRecord{NodeID: f.Nodes[1].ID, Status: StepSuccess}
				if err := s.SaveStep(ctx, "run_aaaaaaaaaboa", w*rounds+i+1, step); err != nil {
					errs <- fmt.Errorf("SaveStep: %w", err)
				}
				if err := s.SetRunStatus(ctx, "run_aaaaaaaaaboa", RunRunning); err != nil {
					errs <- fmt.Errorf("SetRunStatus: %w", err)
				}
				if _, err := s.PruneRuns(ctx, 30, 5, storeNow); err != nil {
					errs <- fmt.Errorf("PruneRuns: %w", err)
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if list, err := s.ListTimers(ctx); err != nil || len(list) != 2 {
		t.Fatalf("ListTimers = %+v, %v", list, err)
	}
	if _, steps, err := s.GetRun(ctx, "run_aaaaaaaaaboa"); err != nil || len(steps) != workers*rounds {
		t.Fatalf("GetRun = %d steps, %v", len(steps), err)
	}
}
