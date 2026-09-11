package desktop

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestLooperRunStateHolderTryStartAllowsOneConcurrentRun(t *testing.T) {
	t.Parallel()

	holder := NewLooperRunStateHolder()
	start := make(chan struct{})
	var wg sync.WaitGroup
	var successes int32

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, cancel := context.WithCancel(context.Background())
			if err := holder.TryStart(3, cancel); err != nil {
				cancel()
				return
			}
			atomic.AddInt32(&successes, 1)
		}()
	}

	close(start)
	wg.Wait()
	if successes != 1 {
		t.Fatalf("successful TryStart calls = %d, want 1", successes)
	}
	if state := holder.State(); !state.Running || state.MaxRounds != 3 || state.Status != "running" {
		t.Fatalf("state after TryStart = %+v, want running with max rounds", state)
	}
	holder.SetIdle()
	if state := holder.State(); state.Running {
		t.Fatalf("state after SetIdle = %+v, want idle", state)
	}
}

func TestLooperRunStateHolderPauseAndResumeSnapshot(t *testing.T) {
	t.Parallel()

	holder := NewLooperRunStateHolder()
	if _, ok := holder.GetResumeState(); ok {
		t.Fatal("expected no resume state initially")
	}

	_, cancel := context.WithCancel(context.Background())
	if err := holder.TryStart(20, cancel); err != nil {
		cancel()
		t.Fatalf("TryStart failed: %v", err)
	}

	holder.RequestPause()
	if !holder.IsPauseRequested() {
		t.Fatal("pause request should be visible")
	}

	rs := LooperResumeState{
		Round:           7,
		BestScore:       72,
		ScoreHistory:    []int{40, 55, 72},
		LastFeedback:    "strengthen the ending",
		LastWorkSummary: "rewrote chapter 3",
	}
	holder.SaveResumeState(rs)

	st := holder.State()
	if !st.Paused || st.ResumeFrom != 7 || st.ResumeSnapshot == nil || st.Status != "paused" {
		t.Fatalf("state after pause not correct: %+v", st)
	}
	if st.Running {
		t.Fatal("running must be false while paused")
	}

	got, ok := holder.GetResumeState()
	if !ok {
		t.Fatal("expected resume state to exist")
	}
	if got.Round != 7 || got.LastWorkSummary != "rewrote chapter 3" || got.BestScore != 72 {
		t.Fatalf("resume snapshot mismatch: %+v", got)
	}

	holder.SetIdle()
	st2 := holder.State()
	if !st2.Paused || st2.ResumeFrom != 7 || st2.Status != "paused" {
		t.Fatal("SetIdle must preserve pause/resume info")
	}

	holder.ClearResumeState()
	if _, ok := holder.GetResumeState(); ok {
		t.Fatal("resume state should be gone after ClearResumeState")
	}
}

func TestLooperRunStateHolderTerminalStatuses(t *testing.T) {
	t.Parallel()

	t.Run("stopped", func(t *testing.T) {
		holder := NewLooperRunStateHolder()
		_, cancel := context.WithCancel(context.Background())
		if err := holder.TryStart(5, cancel); err != nil {
			cancel()
			t.Fatalf("TryStart: %v", err)
		}
		holder.SetStopped()
		holder.SetIdle()
		st := holder.State()
		if !st.Stopped || st.Error != "" || st.Status != "stopped" {
			t.Fatalf("stopped state = %+v", st)
		}
	})

	t.Run("completed", func(t *testing.T) {
		holder := NewLooperRunStateHolder()
		_, cancel := context.WithCancel(context.Background())
		if err := holder.TryStart(5, cancel); err != nil {
			cancel()
			t.Fatal(err)
		}
		holder.SetStatus("completed")
		holder.SetIdle()
		if got := holder.State().Status; got != "completed" {
			t.Fatalf("status = %q", got)
		}
	})

	t.Run("stalled", func(t *testing.T) {
		holder := NewLooperRunStateHolder()
		_, cancel := context.WithCancel(context.Background())
		if err := holder.TryStart(5, cancel); err != nil {
			cancel()
			t.Fatal(err)
		}
		holder.SetStatus("stalled")
		holder.SetIdle()
		if got := holder.State().Status; got != "stalled" {
			t.Fatalf("status = %q", got)
		}
	})

	t.Run("max_rounds", func(t *testing.T) {
		holder := NewLooperRunStateHolder()
		_, cancel := context.WithCancel(context.Background())
		if err := holder.TryStart(5, cancel); err != nil {
			cancel()
			t.Fatal(err)
		}
		holder.SetStatus("max_rounds")
		holder.SetIdle()
		if got := holder.State().Status; got != "max_rounds" {
			t.Fatalf("status = %q", got)
		}
	})
}

func TestLooperResumeStateKeepsSnapshotUntilClear(t *testing.T) {
	t.Parallel()
	holder := NewLooperRunStateHolder()
	_, cancel := context.WithCancel(context.Background())
	if err := holder.TryStart(10, cancel); err != nil {
		cancel()
		t.Fatalf("TryStart: %v", err)
	}
	holder.SaveResumeState(LooperResumeState{
		Round:           3,
		LastWorkSummary: "first draft",
		LastFeedback:    "score 70",
	})
	rs, ok := holder.GetResumeState()
	if !ok || rs.LastWorkSummary != "first draft" || rs.Round != 3 {
		t.Fatalf("resume state = %+v ok=%v", rs, ok)
	}
	_, cancel2 := context.WithCancel(context.Background())
	if err := holder.TryStartResume(10, 3, cancel2); err != nil {
		cancel2()
		t.Fatalf("TryStartResume: %v", err)
	}
	if _, ok := holder.GetResumeState(); !ok {
		t.Fatal("snapshot should remain after TryStartResume")
	}
	holder.ClearResumeState()
	if _, ok := holder.GetResumeState(); ok {
		t.Fatal("snapshot should be gone after ClearResumeState")
	}
}

func TestStallWithoutImprovement(t *testing.T) {
	t.Parallel()
	if StallWithoutImprovement([]int{40, 55}, 3) {
		t.Fatal("short history must not stall")
	}
	if StallWithoutImprovement([]int{40, 55, 55, 55}, 3) {
		t.Fatal("later scores beat the prefix best")
	}
	if !StallWithoutImprovement([]int{40, 55, 50, 50, 50}, 3) {
		t.Fatal("expected stall when last 3 scores do not beat 55")
	}
	if StallWithoutImprovement([]int{70, 80, 90}, 0) {
		t.Fatal("stallN 0 disables detection")
	}
}

func openLooperTestStore(t *testing.T) *LooperPresetStore {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	store := NewLooperPresetStore(db)
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("init: %v", err)
	}
	return store
}

func TestLooperPresetStoreSeedsV2Builtins(t *testing.T) {
	t.Parallel()
	store := openLooperTestStore(t)
	presets, err := store.ListPresets(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	keys := map[string]bool{}
	for _, p := range presets {
		if p.IsBuiltin {
			keys[p.BuiltinKey] = true
			if strings.TrimSpace(p.Goal) == "" || strings.TrimSpace(p.Work) == "" || strings.TrimSpace(p.Evaluate) == "" {
				t.Fatalf("builtin %q missing v2 fields: %+v", p.Name, p)
			}
		}
		if p.Name == "Ralph Loop" || p.Name == "Story Iteration" {
			t.Fatalf("obsolete builtin still present: %s", p.Name)
		}
	}
	for _, key := range []string{"short_story", "python_tool", "research_briefing", "project_readme", "flashcards"} {
		if !keys[key] {
			t.Fatalf("missing builtin key %s", key)
		}
	}
}

func TestLooperPresetStorePersistsV2Fields(t *testing.T) {
	t.Parallel()
	store := openLooperTestStore(t)
	id, err := store.SavePreset(context.Background(), LooperPreset{
		Name:        "User Loop",
		Goal:        "Write a note",
		Work:        "Improve the note",
		Evaluate:    "Score clarity",
		Finish:      "Open the note",
		MaxRounds:   6,
		TargetScore: 80,
		StallRounds: 2,
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := store.GetPreset(context.Background(), id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Goal != "Write a note" || got.Work != "Improve the note" || got.Evaluate != "Score clarity" {
		t.Fatalf("v2 fields mismatch: %+v", got)
	}
	if got.MaxRounds != 6 || got.TargetScore != 80 || got.StallRounds != 2 || got.Finish != "Open the note" {
		t.Fatalf("settings mismatch: %+v", got)
	}
	if got.IsBuiltin {
		t.Fatal("user preset must not be builtin")
	}
}

func TestLooperPresetStoreMigratesLegacyUserPreset(t *testing.T) {
	t.Parallel()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(`CREATE TABLE desktop_looper_presets (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		is_builtin INTEGER DEFAULT 0,
		prepare TEXT NOT NULL,
		plan TEXT NOT NULL,
		action TEXT NOT NULL,
		test TEXT NOT NULL,
		exit_cond TEXT NOT NULL,
		finish TEXT DEFAULT '',
		provider_id TEXT DEFAULT '',
		model TEXT DEFAULT '',
		max_iter INTEGER DEFAULT 20
	)`); err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO desktop_looper_presets(name, is_builtin, prepare, plan, action, test, exit_cond, finish, max_iter)
		VALUES('Legacy User', 0, 'prep text', 'plan text', 'action text', 'test text', 'exit text', 'finish text', 12)`); err != nil {
		t.Fatalf("insert legacy: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO desktop_looper_presets(name, is_builtin, prepare, plan, action, test, exit_cond, finish, max_iter)
		VALUES('Ralph Loop', 1, 'old', 'old', 'old', 'old', 'old', 'old', 18)`); err != nil {
		t.Fatalf("insert old builtin: %v", err)
	}

	store := NewLooperPresetStore(db)
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("init: %v", err)
	}

	presets, err := store.ListPresets(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var user LooperPreset
	var foundUser bool
	for _, p := range presets {
		if p.Name == "Ralph Loop" {
			t.Fatal("old Ralph Loop builtin should be removed")
		}
		if p.Name == "Legacy User" {
			foundUser = true
			user = p
		}
	}
	if !foundUser {
		t.Fatal("user preset missing after migration")
	}
	if user.Goal != "prep text" {
		t.Fatalf("goal = %q", user.Goal)
	}
	if !strings.Contains(user.Work, "plan text") || !strings.Contains(user.Work, "action text") {
		t.Fatalf("work = %q", user.Work)
	}
	if !strings.Contains(user.Evaluate, "test text") || !strings.Contains(user.Evaluate, "Done when: exit text") {
		t.Fatalf("evaluate = %q", user.Evaluate)
	}
	if user.Finish != "finish text" || user.MaxRounds != 12 {
		t.Fatalf("finish/max = %q %d", user.Finish, user.MaxRounds)
	}
}

func TestLooperPresetStoreInitRefreshesBuiltinPresets(t *testing.T) {
	t.Parallel()
	store := openLooperTestStore(t)
	if _, err := store.db.Exec(`UPDATE desktop_looper_presets SET finish='Old finish' WHERE builtin_key='short_story' AND is_builtin=1`); err != nil {
		t.Fatalf("simulate old builtin: %v", err)
	}
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("refresh init: %v", err)
	}
	var finish string
	if err := store.db.QueryRow(`SELECT finish FROM desktop_looper_presets WHERE builtin_key='short_story'`).Scan(&finish); err != nil {
		t.Fatalf("read refreshed preset: %v", err)
	}
	if !strings.Contains(finish, "open_in_app") {
		t.Fatalf("finish prompt was not refreshed: %q", finish)
	}
}

func TestLooperRunStoreRetentionAndCRUD(t *testing.T) {
	t.Parallel()
	store := openLooperTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	for i := 1; i <= 22; i++ {
		if _, err := store.SaveRun(ctx, LooperRunRecord{
			PresetName:  "Demo",
			GoalExcerpt: strings.Repeat("g", 300),
			Status:      "completed",
			Rounds:      i,
			MaxRounds:   10,
			BestScore:   80,
			FinalScore:  70,
			StartedAt:   now,
			FinishedAt:  now,
			Logs: []LooperLogEntry{{
				Round:    i,
				Step:     "work",
				Response: strings.Repeat("x", 50),
			}},
		}); err != nil {
			t.Fatalf("save run %d: %v", i, err)
		}
	}
	runs, err := store.ListRuns(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(runs) != LooperRunRetention {
		t.Fatalf("retention = %d, want %d", len(runs), LooperRunRetention)
	}
	if runs[0].Rounds != 22 {
		t.Fatalf("newest rounds = %d, want 22", runs[0].Rounds)
	}
	if runs[0].Logs != nil {
		t.Fatal("list must omit logs")
	}
	if len([]rune(runs[0].GoalExcerpt)) > LooperGoalExcerptLimit {
		t.Fatalf("excerpt too long: %d", len([]rune(runs[0].GoalExcerpt)))
	}

	detail, err := store.GetRun(ctx, runs[0].ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(detail.Logs) != 1 || detail.Logs[0].Step != "work" {
		t.Fatalf("detail logs = %+v", detail.Logs)
	}
	if err := store.DeleteRun(ctx, detail.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := store.ClearRuns(ctx); err != nil {
		t.Fatalf("clear: %v", err)
	}
	left, err := store.ListRuns(ctx)
	if err != nil {
		t.Fatalf("list after clear: %v", err)
	}
	if len(left) != 0 {
		t.Fatalf("runs left = %d", len(left))
	}
}
