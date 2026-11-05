package tools

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/security"
)

func TestDurableMissionHistoryProjection(t *testing.T) {
	db, err := InitMissionHistoryDB(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	started := time.Now().Add(-2 * time.Hour).UTC()
	rec := DurableMissionHistory{RunID: "run_123", MissionID: "mission_1", MissionName: "Example", Status: "running", StartedAt: started}
	if err := ProjectDurableMissionHistory(ctx, db, rec); err != nil {
		t.Fatal(err)
	}
	ordinary, err := RecordMissionStart(db, "mission_2", "Ordinary", "manual", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE mission_history SET started_at = ? WHERE id = ?`, started.Format(time.RFC3339), ordinary); err != nil {
		t.Fatal(err)
	}
	if n, err := ReconcileStaleRunningMarks(db, time.Hour, slog.Default()); err != nil || n != 1 {
		t.Fatalf("stale cleanup = %d, %v; want 1 ordinary run", n, err)
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM mission_history WHERE id = ?`, DurableHistoryPrefix+rec.RunID).Scan(&status); err != nil || status != "running" {
		t.Fatalf("durable run after cleanup = %q, %v", status, err)
	}
	rec.Status = "waiting"
	if err := ProjectDurableMissionHistory(ctx, db, rec); err != nil {
		t.Fatal(err)
	}
	finished := started.Add(time.Minute)
	rec.Status, rec.FinishedAt = "success", &finished
	if err := ProjectDurableMissionHistory(ctx, db, rec); err != nil {
		t.Fatal(err)
	}
	rec.Status, rec.FinishedAt = "waiting", nil
	if err := ProjectDurableMissionHistory(ctx, db, rec); err != nil {
		t.Fatal(err)
	}
	var count int
	var output, triggerData string
	if err := db.QueryRow(`SELECT count(*), status, output, trigger_data FROM mission_history WHERE id = ?`, DurableHistoryPrefix+rec.RunID).Scan(&count, &status, &output, &triggerData); err != nil {
		t.Fatal(err)
	}
	if count != 1 || status != "success" || output != "" || triggerData != "" {
		t.Fatalf("projection count=%d status=%q output=%q trigger=%q", count, status, output, triggerData)
	}
	rec.MissionID = "another_mission"
	if err := ProjectDurableMissionHistory(ctx, db, rec); err == nil {
		t.Fatal("different owner must not overwrite a projection")
	}
}

func TestDurableMissionHistoryRejectsInvalidMetadataAndStorageFailure(t *testing.T) {
	db, err := InitMissionHistoryDB(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	rec := DurableMissionHistory{RunID: "run_123", MissionID: "mission_1", Status: "waiting", StartedAt: time.Now()}
	bad := rec
	bad.ErrorCode = "contains user text"
	if err := ProjectDurableMissionHistory(context.Background(), db, bad); err == nil {
		t.Fatal("invalid error text accepted")
	}
	bad = rec
	bad.Status = "success"
	if err := ProjectDurableMissionHistory(context.Background(), db, bad); err == nil {
		t.Fatal("terminal status without finish time accepted")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := ProjectDurableMissionHistory(context.Background(), db, rec); err == nil {
		t.Fatal("closed history DB reported success")
	}
}

func TestDurableMissionHistoryRedactsSecretsRegisteredAfterWrite(t *testing.T) {
	db, err := InitMissionHistoryDB(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	const secret = "durable-later-registered-secret"
	rec := DurableMissionHistory{RunID: "run_later", MissionID: "mission_1", MissionName: "Flow " + secret, Status: "waiting", StartedAt: time.Now()}
	if err := ProjectDurableMissionHistory(context.Background(), db, rec); err != nil {
		t.Fatal(err)
	}
	release := security.RegisterScopedSensitiveExact(secret)
	t.Cleanup(release)
	single, err := GetMissionRun(db, DurableHistoryPrefix+rec.RunID)
	if err != nil || single == nil || strings.Contains(single.MissionName, secret) {
		t.Fatalf("single durable history entry did not redact its name: %v", err)
	}
	page, err := QueryMissionHistory(db, MissionHistoryFilter{MissionID: rec.MissionID})
	if err != nil || len(page.Entries) != 1 || strings.Contains(page.Entries[0].MissionName, secret) {
		t.Fatalf("durable history page did not redact its name: %v", err)
	}
}

func TestDurableMissionHistoryReportsUnknownWithoutSource(t *testing.T) {
	db, err := InitMissionHistoryDB(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	m := NewMissionManagerV2(t.TempDir(), nil)
	m.SetHistoryDB(db)
	now := time.Now().UTC()
	for _, rec := range []DurableMissionHistory{
		{RunID: "run_missing", MissionID: "mission_1", Status: "waiting", StartedAt: now},
		{RunID: "run_known", MissionID: "mission_1", Status: "running", StartedAt: now},
		{RunID: "run_done", MissionID: "mission_1", Status: "success", StartedAt: now, FinishedAt: &now},
	} {
		if err := ProjectDurableMissionHistory(context.Background(), db, rec); err != nil {
			t.Fatal(err)
		}
	}
	var calls int
	m.SetDurableHistorySource(func(ctx context.Context, runID, missionID string) (bool, error) {
		calls++
		_, _ = m.Get(missionID) // the callback must not run under the manager lock
		return runID == "run_known", nil
	})
	page, err := m.QueryMissionHistory(context.Background(), MissionHistoryFilter{})
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]*MissionRun)
	for _, run := range page.Entries {
		byID[run.ID] = run
	}
	if calls != 2 || byID[DurableHistoryPrefix+"run_missing"].Status != "unknown" || byID[DurableHistoryPrefix+"run_missing"].LastKnownStatus != "waiting" || byID[DurableHistoryPrefix+"run_known"].Status != "running" || byID[DurableHistoryPrefix+"run_done"].Status != "success" {
		t.Fatalf("incorrect source observations: calls=%d entries=%+v", calls, byID)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	page, err = m.QueryMissionHistory(ctx, MissionHistoryFilter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, run := range page.Entries {
		if run.ID == DurableHistoryPrefix+"run_known" && (run.Status != "unknown" || run.LastKnownStatus != "running") {
			t.Fatalf("cancelled source check reported a current status: %+v", run)
		}
	}
}

func TestDurableMissionProjectionSharesOrdinaryRunningState(t *testing.T) {
	dir := t.TempDir()
	db, err := InitMissionHistoryDB(filepath.Join(dir, "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	manager := NewMissionManagerV2(dir, nil)
	manager.SetHistoryDB(db)
	const secret = "durable-history-secret-canary"
	release := security.RegisterScopedSensitiveExact(secret)
	t.Cleanup(release)
	var completions atomic.Int64
	manager.mu.Lock()
	manager.missions["mission_1"] = &MissionV2{ID: "mission_1", Name: "Flow " + secret, FlowID: "flow_1", ExecutionType: ExecutionFlow, Status: MissionStatusIdle}
	manager.onMissionComplete = func(string, string, string) { completions.Add(1) }
	manager.mu.Unlock()
	ordinary := manager.FlowRunStarted("mission_1", "manual", "")
	if ordinary == "" {
		t.Fatal("ordinary flow did not start")
	}
	started := time.Now().UTC()
	rec := DurableMissionHistory{RunID: "run_durable", MissionID: "mission_1", FlowID: "flow_1", Status: "waiting", StartedAt: started}
	for i := 0; i < 2; i++ {
		if err := manager.ProjectDurableFlowRun(context.Background(), rec); err != nil {
			t.Fatal(err)
		}
	}
	var savedName string
	if err := db.QueryRow(`SELECT mission_name FROM mission_history WHERE id = ?`, DurableHistoryPrefix+rec.RunID).Scan(&savedName); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(savedName, secret) {
		t.Fatal("registered secret in mission name was persisted")
	}
	manager.mu.Lock()
	if manager.flowActive["mission_1"] != 2 {
		t.Fatalf("replayed projection duplicated a running slot: %d", manager.flowActive["mission_1"])
	}
	manager.mu.Unlock()
	finished := started.Add(time.Second)
	rec.Status, rec.FinishedAt = "success", &finished
	if err := manager.ProjectDurableFlowRun(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	rec.Status, rec.FinishedAt = "waiting", nil
	if err := manager.ProjectDurableFlowRun(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	mission, _ := manager.Get("mission_1")
	if mission.Status != MissionStatusRunning || mission.RunCount != 0 || completions.Load() != 0 || len(manager.queue.List()) != 0 {
		t.Fatalf("durable completion disturbed ordinary run or accounting: %+v", mission)
	}
	manager.FlowRunFinished("mission_1", ordinary, MissionResultSuccess, "ok", nil)
	mission, _ = manager.Get("mission_1")
	if mission.Status != MissionStatusIdle || mission.RunCount != 1 {
		t.Fatalf("ordinary completion was not preserved: %+v", mission)
	}
	ordinary = manager.FlowRunStarted("mission_1", "manual", "")
	rec.RunID = "run_second"
	if err := manager.ProjectDurableFlowRun(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	manager.FlowRunFinished("mission_1", ordinary, MissionResultSuccess, "ok", nil)
	mission, _ = manager.Get("mission_1")
	if mission.Status != MissionStatusRunning || mission.RunCount != 2 {
		t.Fatalf("ordinary completion cleared a waiting durable run: %+v", mission)
	}
	rec.Status, rec.FinishedAt = "success", &finished
	if err := manager.ProjectDurableFlowRun(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	mission, _ = manager.Get("mission_1")
	if mission.Status != MissionStatusIdle || mission.RunCount != 2 {
		t.Fatalf("durable completion changed ordinary accounting: %+v", mission)
	}
	bad := rec
	bad.RunID, bad.FlowID = "run_other", "flow_wrong"
	if err := manager.ProjectDurableFlowRun(context.Background(), bad); err == nil {
		t.Fatal("foreign flow ownership accepted")
	}
}
