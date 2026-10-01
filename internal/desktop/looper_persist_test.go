package desktop

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
)

func TestLooperRunStoreKeepsRunConfigForReuse(t *testing.T) {
	t.Parallel()
	store := openLooperTestStore(t)
	ctx := context.Background()
	cfg := LooperRunConfig{
		Goal: "Write a story", Work: "Improve it", Evaluate: "Judge it", Finish: "Open it",
		MaxRounds: 7, TargetScore: 88, StallRounds: 2, ProviderID: "main", Model: "model-x", PresetName: "Story",
	}
	id, err := store.SaveRun(ctx, LooperRunRecord{PresetName: "Story", Status: "completed", Rounds: 3, MaxRounds: 7, Config: &cfg})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	plainID, err := store.SaveRun(ctx, LooperRunRecord{PresetName: "Old", Status: "completed"})
	if err != nil {
		t.Fatalf("save plain: %v", err)
	}

	got, err := store.GetRun(ctx, id)
	if err != nil || got.Config == nil || *got.Config != cfg {
		t.Fatalf("stored config = %+v err=%v, want %+v", got.Config, err, cfg)
	}
	plain, err := store.GetRun(ctx, plainID)
	if err != nil || plain.Config != nil {
		t.Fatalf("a run saved without settings must report none, got %+v err=%v", plain.Config, err)
	}
	listed, err := store.ListRuns(ctx)
	if err != nil || len(listed) != 2 {
		t.Fatalf("list: %v %d", err, len(listed))
	}
	for _, rec := range listed {
		if rec.Config != nil {
			t.Fatal("the list view must stay light and omit the settings")
		}
	}
}

func TestLooperPresetStoreAddsRunConfigColumnToOlderDatabases(t *testing.T) {
	t.Parallel()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	// The run history exactly as it looked before settings were stored.
	if _, err := db.ExecContext(ctx, `CREATE TABLE desktop_looper_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT, preset_name TEXT DEFAULT '', goal_excerpt TEXT DEFAULT '',
		status TEXT NOT NULL, rounds INTEGER DEFAULT 0, max_rounds INTEGER DEFAULT 0, best_score INTEGER DEFAULT 0,
		final_score INTEGER DEFAULT 0, target_score INTEGER DEFAULT 0, input_tokens INTEGER DEFAULT 0,
		output_tokens INTEGER DEFAULT 0, cost_usd REAL DEFAULT 0, error TEXT DEFAULT '',
		started_at DATETIME, finished_at DATETIME, logs_json TEXT DEFAULT '[]')`); err != nil {
		t.Fatalf("legacy table: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO desktop_looper_runs(preset_name, status, started_at, finished_at) VALUES('Legacy', 'completed', '', '')`); err != nil {
		t.Fatalf("legacy row: %v", err)
	}

	store := NewLooperPresetStore(db)
	if err := store.Init(ctx); err != nil {
		t.Fatalf("init on a legacy database: %v", err)
	}
	if err := store.Init(ctx); err != nil {
		t.Fatalf("a second init must stay harmless: %v", err)
	}
	legacy, err := store.GetRun(ctx, 1)
	if err != nil || legacy.PresetName != "Legacy" || legacy.Config != nil {
		t.Fatalf("legacy run = %+v err=%v", legacy, err)
	}
	id, err := store.SaveRun(ctx, LooperRunRecord{Status: "completed", Config: &LooperRunConfig{Goal: "g"}})
	if err != nil {
		t.Fatalf("save after migration: %v", err)
	}
	if got, err := store.GetRun(ctx, id); err != nil || got.Config == nil || got.Config.Goal != "g" {
		t.Fatalf("migrated database must keep settings: %+v err=%v", got.Config, err)
	}
}

func TestLooperActiveRunCheckpointRoundTrip(t *testing.T) {
	t.Parallel()
	store := openLooperTestStore(t)
	ctx := context.Background()

	if _, ok, err := store.LoadActiveRun(ctx); err != nil || ok {
		t.Fatalf("a fresh database has no checkpoint: ok=%v err=%v", ok, err)
	}
	started := time.Now().UTC().Truncate(time.Millisecond)
	rec := LooperActiveRun{
		Config:    LooperRunConfig{Goal: "g", Work: "w", Evaluate: "e", MaxRounds: 5, TargetScore: 80, PresetName: "P"},
		StartedAt: started,
		State: LooperRunState{
			Status: "running", Round: 2, MaxRounds: 5, BestScore: 71, ScoreHistory: []int{60, 71}, TargetScore: 80,
			PresetName: "P", GoalExcerpt: "g", Logs: []LooperLogEntry{{Round: 1, Step: "work"}, {Round: 1, Step: "evaluate", Score: 60}},
			LogsFrom: 0, LogTotal: 2, InputTokens: 100, OutputTokens: 40, EstimatedCostUSD: 0.01, ElapsedMS: 4200,
		},
		Resume: LooperResumeState{Round: 2, BestScore: 71, ScoreHistory: []int{60, 71}, LastFeedback: "tighten", LastWorkSummary: "did it"},
	}
	if err := store.SaveActiveRun(ctx, rec); err != nil {
		t.Fatalf("save: %v", err)
	}
	rec.State.Round = 3
	if err := store.SaveActiveRun(ctx, rec); err != nil {
		t.Fatalf("a later checkpoint replaces the earlier one: %v", err)
	}
	got, ok, err := store.LoadActiveRun(ctx)
	if err != nil || !ok {
		t.Fatalf("load: ok=%v err=%v", ok, err)
	}
	if got.State.Round != 3 || got.Config.Goal != "g" || got.Resume.LastFeedback != "tighten" || len(got.State.Logs) != 2 || !got.StartedAt.Equal(started) {
		t.Fatalf("checkpoint did not round-trip: %+v", got)
	}
	if err := store.ClearActiveRun(ctx); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if _, ok, _ := store.LoadActiveRun(ctx); ok {
		t.Fatal("a cleared checkpoint must be gone")
	}
}

func TestLooperRunStateHolderRestoreWaitsForResume(t *testing.T) {
	t.Parallel()

	source := NewLooperRunStateHolder()
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := source.TryStart(6, cancel); err != nil {
		t.Fatalf("TryStart: %v", err)
	}
	source.SetRunInfo(85, true, "Story", "Write it")
	source.AppendLog(LooperLogEntry{Round: 1, Step: "work"})
	source.AppendLog(LooperLogEntry{Round: 1, Step: "evaluate", Score: 66})
	source.RecordEvaluation(66, "tighten", "ok")
	source.SetRound(2)
	source.AddUsage(500, 200, 0.02)
	time.Sleep(15 * time.Millisecond)
	checkpoint := source.State()

	holder := NewLooperRunStateHolder()
	holder.Restore(checkpoint, LooperResumeState{Round: 1, BestScore: 66, ScoreHistory: []int{66}, LastFeedback: "tighten"}, "interrupted")
	state := holder.State()
	if !state.Paused || state.Running || state.Status != "paused" || state.PauseReason != "interrupted" || state.ResumeFrom != 1 {
		t.Fatalf("a restored run must wait for Resume: %+v", state)
	}
	if state.PresetName != "Story" || state.TargetScore != 85 || len(state.Logs) != 2 || state.LogTotal != 2 || state.RunID == 0 {
		t.Fatalf("restored run lost its description or logs: %+v", state)
	}
	if state.ElapsedMS < checkpoint.ElapsedMS || state.InputTokens != 500 {
		t.Fatalf("elapsed time and usage must carry over: elapsed %d vs %d, tokens %d", state.ElapsedMS, checkpoint.ElapsedMS, state.InputTokens)
	}
	if rs, ok := holder.GetResumeState(); !ok || rs.Round != 1 || rs.LastFeedback != "tighten" {
		t.Fatalf("resume snapshot = %+v ok=%v", rs, ok)
	}

	// Logs continue at the next absolute index after the restore.
	holder.AppendLog(LooperLogEntry{Round: 2, Step: "work"})
	if tail := holder.StateSince(2); tail.LogsFrom != 2 || tail.LogTotal != 3 || len(tail.Logs) != 1 {
		t.Fatalf("log window after restore: from %d total %d len %d", tail.LogsFrom, tail.LogTotal, len(tail.Logs))
	}

	_, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	if err := holder.TryStartResume(6, 1, cancel2); err != nil {
		t.Fatalf("a restored run must be resumable: %v", err)
	}
	if resumed := holder.State(); !resumed.Running || resumed.PauseReason != "" {
		t.Fatalf("resuming clears the pause reason: %+v", resumed)
	}
	holder.Restore(checkpoint, LooperResumeState{Round: 1}, "interrupted")
	if still := holder.State(); !still.Running || strings.TrimSpace(still.PauseReason) != "" {
		t.Fatal("Restore must not touch a run that is executing")
	}
}

func TestLooperRunStateHolderMarksApproximateCost(t *testing.T) {
	t.Parallel()
	holder := NewLooperRunStateHolder()
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := holder.TryStart(3, cancel); err != nil {
		t.Fatalf("TryStart: %v", err)
	}
	if holder.State().CostApproximate {
		t.Fatal("a fresh run is not approximate")
	}
	holder.MarkCostApproximate()
	if !holder.State().CostApproximate {
		t.Fatal("approximate pricing must be visible")
	}
	holder.SetIdle()
	if err := holder.TryStart(3, cancel); err != nil {
		t.Fatalf("second TryStart: %v", err)
	}
	if holder.State().CostApproximate {
		t.Fatal("the flag belongs to one run")
	}
}
