package flows

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

// Audit 2026-10-08, finding 1.8: the finish report of a live run carries the leaf
// outputs and the notify setting of the document the run executed, never those of the
// current document.

// audit18Republished is the flow after the run's revision: "first" is gone, so "search"
// is the only leaf, and failures are not reported. Under the old fallback a run of
// revision 1 reported {"search": …} (no leaf when it ran) and notify "off".
func audit18Republished(name string) *Flow {
	b := newFlow(name)
	start := b.node("start", TypeTriggerManual, nil)
	search := b.node("search", TypeWebSearch, map[string]any{"query": "wetter"})
	b.edge(start, PortOut, search)
	f := b.build()
	f.Settings.NotifyOnError = "off"
	return f
}

// audit18Republish publishes audit18Republished n times as the flow's next revisions.
func audit18Republish(t *testing.T, s *Service, flowID, name string, n int) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < n; i++ {
		rec, err := s.GetFlow(ctx, flowID)
		if err != nil {
			t.Fatal(err)
		}
		doc := audit18Republished(name)
		doc.Description = fmt.Sprintf("revision %d", rec.LiveRevision+1)
		rev, _, err := s.SaveDraft(ctx, flowID, doc, rec.DraftRevision)
		if err != nil {
			t.Fatalf("SaveDraft: %v", err)
		}
		if _, _, err := s.Publish(ctx, flowID, rev); err != nil {
			t.Fatalf("Publish: %v", err)
		}
	}
}

// audit18Fixture publishes svcRunSearchFlow(name, "first") as revision 1 and starts a
// live run of it that waits in its web search. The log keeps every record.
type audit18Fixture struct {
	s      *Service
	tools  *svcRunGateTools
	bridge *svcRunBridge
	logs   *svcLogs
	pub    *FlowRecord
	run    StartResult
}

func newAudit18Fixture(t *testing.T, name string) *audit18Fixture {
	t.Helper()
	fx := &audit18Fixture{tools: newSvcRunGateTools(), bridge: newSvcRunBridge(), logs: &svcLogs{}}
	fx.s = svcRunNewService(t, fx.tools, fx.bridge, slog.New(fx.logs), ServiceConfig{})
	t.Cleanup(fx.tools.open) // runs before the Shutdown
	fx.pub = svcRunPublish(t, fx.s, svcRunSearchFlow(name, "first"))
	var err error
	if fx.run, err = fx.s.RunNow(context.Background(), fx.pub.ID); err != nil {
		t.Fatalf("RunNow: %v", err)
	}
	fx.tools.waitCalled(t, fx.run.RunID)
	return fx
}

// republish publishes audit18Republished n times (one new live revision each).
func (fx *audit18Fixture) republish(t *testing.T, n int) {
	t.Helper()
	audit18Republish(t, fx.s, fx.pub.ID, fx.pub.Name, n)
}

func (fx *audit18Fixture) finish(t *testing.T) RunFinishedInfo {
	t.Helper()
	fx.tools.open()
	return fx.bridge.waitInfo(t, fx.run.RunID)
}

// The reproduction: more than maxStoredVersions publishes while a live run waits. The
// run's revision must survive the pruning, so its report carries revision 1's leaf and
// setting.
func TestAudit18RunOutlivesVersionPruning(t *testing.T) {
	fx := newAudit18Fixture(t, "Lang")
	fx.republish(t, maxStoredVersions+1)
	if _, err := fx.s.store.GetVersion(context.Background(), fx.pub.ID, 1); err != nil {
		t.Fatalf("the revision of the unfinished run was pruned: %v", err)
	}
	info := fx.finish(t)
	first, _ := info.Outputs["first"].(map[string]any)
	if info.Record.Revision != 1 || len(info.Outputs) != 1 || first["v"] != "first" || info.NotifyOnError != DefaultNotifyOnError {
		t.Fatalf("the run of revision 1 reported outputs %v and notify %q, want revision 1's leaf and setting",
			info.Outputs, info.NotifyOnError)
	}
}

// Review M2: the report uses the document the runner executed, which the Service keeps
// from OnRunStarted, not a stored version. Here every version of the flow is gone between
// FinishRun and the finish hook (a prune, or a delete behind the store's back), and the
// report still carries revision 1's leaf and setting, without reading the store.
func TestAudit18ReportUsesTheExecutedDocumentAfterAPrune(t *testing.T) {
	fx := newAudit18Fixture(t, "Weg")
	fx.republish(t, 1)
	ctx := context.Background()
	var pruned error
	hook := fx.s.runner.hooks.OnRunFinished
	// Set before the run is released (tools.open in finish), so the runner reads it after.
	fx.s.runner.hooks.OnRunFinished = func(rec RunRecord, res RunResult) {
		if rec.ID == fx.run.RunID {
			if _, err := fx.s.store.db.ExecContext(ctx, `DELETE FROM flow_versions WHERE flow_id = ?`, fx.pub.ID); err != nil {
				pruned = err
			} else if _, err := fx.s.store.GetRunDoc(ctx, rec.ID); err == nil {
				pruned = fmt.Errorf("the run's stored document is still readable")
			}
		}
		hook(rec, res)
	}
	info := fx.finish(t)
	if pruned != nil {
		t.Fatalf("test setup: %v", pruned)
	}
	first, _ := info.Outputs["first"].(map[string]any)
	if info.Record.Revision != 1 || len(info.Outputs) != 1 || first["v"] != "first" || info.NotifyOnError != DefaultNotifyOnError ||
		!info.Started || info.FlowName != "Weg" || info.MissionID != fx.pub.MissionID {
		t.Fatalf("the run of revision 1 reported outputs %v and notify %q (%+v), want revision 1's leaf and setting",
			info.Outputs, info.NotifyOnError, info)
	}
	if read := fx.logs.messages(slog.LevelDebug, "could not be read"); len(read) != 0 {
		t.Fatalf("the report read the store: %v", read)
	}
	if warned := fx.logs.messages(slog.LevelWarn, ""); len(warned) != 0 {
		t.Fatalf("logged: %v", warned)
	}
}

// The fallback for a case the runner does not produce: a run reported as started without
// its document (the hooks called directly) whose stored version is gone. The report
// carries no leaf outputs instead of the current document's, the current notify setting
// (the user's preference now), and a warning.
func TestAudit18UnknownRunDocumentReportsNoLeaves(t *testing.T) {
	bridge := newSvcRunBridge()
	logs := &svcLogs{}
	s := svcRunNewService(t, &fakeTools{}, bridge, slog.New(logs), ServiceConfig{})
	ctx := context.Background()
	pub := svcRunPublish(t, s, svcRunSearchFlow("Weg", "first"))
	audit18Republish(t, s, pub.ID, "Weg", 1)
	rec := RunRecord{ID: "run_aaaaaaaaa18u", FlowID: pub.ID, Revision: 1, Mode: ModeLive, TriggerNode: pub.Live.Nodes[0].ID,
		Status: RunRunning, StartedAt: storeNow}
	if err := s.store.CreateRun(ctx, rec, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.db.ExecContext(ctx, `DELETE FROM flow_versions WHERE flow_id = ? AND revision = 1`, pub.ID); err != nil {
		t.Fatal(err)
	}
	s.onRunStarted(rec, nil)
	s.onRunFinished(rec, RunResult{Status: RunSuccess, Outputs: map[string]map[string]any{
		"first": {"v": "first"}, "search": {"count": 0.0}}})
	got := bridge.reports(rec.ID)
	if len(got) != 1 {
		t.Fatalf("the run was reported %d times", len(got))
	}
	info := got[0]
	if info.Outputs == nil || len(info.Outputs) != 0 {
		t.Fatalf("outputs of a run whose document is unknown = %v, want none", info.Outputs)
	}
	if info.NotifyOnError != "off" || !info.Started || info.FlowName != "Weg" || info.MissionID != pub.MissionID {
		t.Fatalf("report of a run whose document is unknown = %+v", info)
	}
	if warned := logs.messages(slog.LevelWarn, "leaf outputs"); len(warned) != 1 {
		t.Fatalf("the missing document must be logged once at Warn, got %v", warned)
	}
}

// audit18PublishTimes saves f as the draft and publishes it, n times.
func audit18PublishTimes(t *testing.T, s *Store, f *Flow, n int) {
	t.Helper()
	ctx := context.Background()
	rec, err := s.GetFlow(ctx, f.ID)
	if err != nil {
		t.Fatal(err)
	}
	rev := rec.DraftRevision
	for i := 0; i < n; i++ {
		if rev, err = s.SaveDraft(ctx, f.ID, f, rev, storeNow); err != nil {
			t.Fatalf("SaveDraft: %v", err)
		}
		if _, err := s.Publish(ctx, f.ID, rev, storeNow); err != nil {
			t.Fatalf("Publish: %v", err)
		}
	}
}

// audit18Versions lists the stored version numbers of a flow, as text.
func audit18Versions(t *testing.T, s *Store, flowID string) string {
	t.Helper()
	rows, err := s.db.Query(`SELECT revision FROM flow_versions WHERE flow_id = ? ORDER BY revision`, flowID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var rev int
		if err := rows.Scan(&rev); err != nil {
			t.Fatal(err)
		}
		out = append(out, fmt.Sprint(rev))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return strings.Join(out, ",")
}

// audit18Range is "from,from+1,…,to".
func audit18Range(from, to int) string {
	var out []string
	for i := from; i <= to; i++ {
		out = append(out, fmt.Sprint(i))
	}
	return strings.Join(out, ",")
}

// Publish keeps the versions that queued, waiting and running live runs execute, beyond
// the last maxStoredVersions, and prunes them once the run has finished. Finished live
// runs and test runs (they store their own document) do not keep a version.
func TestAudit18VersionPruningKeepsVersionsOfUnfinishedRuns(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := createStoredFlow(t, s, "flow_aaaaaaaaba")
	audit18PublishTimes(t, s, f, 4) // live revisions 1 to 4
	runs := []RunRecord{
		{ID: "run_aaaaaaaaaaa1", Revision: 1, Mode: ModeLive, Status: RunQueued},
		{ID: "run_aaaaaaaaaaa2", Revision: 2, Mode: ModeLive, Status: RunWaiting},
		{ID: "run_aaaaaaaaaaa3", Revision: 3, Mode: ModeLive, Status: RunRunning},
		{ID: "run_aaaaaaaaaaa4", Revision: 4, Mode: ModeTest, Status: RunRunning},
		{ID: "run_aaaaaaaaaaa5", Revision: 4, Mode: ModeLive, Status: RunSuccess},
	}
	for _, r := range runs {
		r.FlowID, r.TriggerNode, r.StartedAt = f.ID, f.Nodes[0].ID, storeNow
		if err := s.CreateRun(ctx, r, f); err != nil {
			t.Fatalf("CreateRun %s: %v", r.ID, err)
		}
	}
	audit18PublishTimes(t, s, f, 56) // live revision 60
	if got, want := audit18Versions(t, s, f.ID), "1,2,3,"+audit18Range(11, 60); got != want {
		t.Fatalf("versions = %s, want %s", got, want)
	}
	if doc, err := s.GetRunDoc(ctx, "run_aaaaaaaaaaa1"); err != nil || doc == nil {
		t.Fatalf("GetRunDoc of the queued run = %v, %v", doc, err)
	}

	if err := s.FinishRun(ctx, "run_aaaaaaaaaaa1", RunResult{Status: RunSuccess, FinishedAt: storeNow}); err != nil {
		t.Fatal(err)
	}
	audit18PublishTimes(t, s, f, 1) // live revision 61
	if got, want := audit18Versions(t, s, f.ID), "2,3,"+audit18Range(12, 61); got != want {
		t.Fatalf("versions after the run finished = %s, want %s", got, want)
	}
}

// The run lookup of the version pruning goes through an index of flow_runs, never a scan
// of the table, so a publish stays cheap however many runs are stored.
func TestAudit18VersionPruningSearchesTheRunsByIndex(t *testing.T) {
	s := openTestStore(t)
	rows, err := s.db.Query(`EXPLAIN QUERY PLAN `+pruneVersionsSQL, "flow_x", "flow_x", maxStoredVersions, "flow_x")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var details []string
	for rows.Next() {
		var id, parent, notUsed int
		var detail string
		if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
			t.Fatal(err)
		}
		details = append(details, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	plan := strings.Join(details, "; ")
	searched := false
	for _, d := range details {
		if strings.Contains(d, "flow_runs") {
			if !strings.HasPrefix(d, "SEARCH flow_runs USING INDEX") {
				t.Fatalf("the version pruning reads flow_runs without an index search: %s", plan)
			}
			searched = true
		}
		if strings.HasPrefix(d, "SCAN") {
			t.Fatalf("the version pruning scans a table: %s", plan)
		}
	}
	if !searched {
		t.Fatalf("the version pruning does not look at the runs: %s", plan)
	}
	t.Logf("plan: %s", plan)
}
