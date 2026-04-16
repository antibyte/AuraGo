package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMissionQueueTryStartNextIsAtomic(t *testing.T) {
	q := NewMissionQueue()
	q.Enqueue("m1", "high", "", "")
	q.Enqueue("m2", "low", "", "")

	item, ok := q.TryStartNext()
	if !ok {
		t.Fatal("expected first mission to start")
	}
	if item.MissionID != "m1" {
		t.Fatalf("started mission = %q, want m1", item.MissionID)
	}

	if _, ok := q.TryStartNext(); ok {
		t.Fatal("did not expect a second mission to start while one is running")
	}

	if got := q.GetRunning(); got != "m1" {
		t.Fatalf("running mission = %q, want m1", got)
	}

	q.Done()
	item, ok = q.TryStartNext()
	if !ok {
		t.Fatal("expected second mission to start after Done")
	}
	if item.MissionID != "m2" {
		t.Fatalf("started mission = %q, want m2", item.MissionID)
	}
}

func TestScheduledMissionSurvivesRestart(t *testing.T) {
	tmpDir := t.TempDir()
	cronMgr := NewCronManager(tmpDir)
	if err := cronMgr.Start(func(prompt string) {}); err != nil {
		t.Fatalf("failed to start cron manager: %v", err)
	}
	defer cronMgr.Stop()

	// Create a mission manager and add a scheduled mission
	mm := NewMissionManagerV2(tmpDir, cronMgr)
	if err := mm.Start(); err != nil {
		t.Fatalf("failed to start mission manager: %v", err)
	}

	mission := &MissionV2{
		Name:          "Test Scheduled",
		Prompt:        "do something",
		ExecutionType: ExecutionScheduled,
		Schedule:      "0 0 * * *",
		Enabled:       true,
	}
	if err := mm.Create(mission); err != nil {
		t.Fatalf("failed to create mission: %v", err)
	}
	mm.Stop()
	cronMgr.Stop()

	// Simulate restart: create fresh cron and mission managers
	cronMgr2 := NewCronManager(tmpDir)
	if err := cronMgr2.Start(func(prompt string) {}); err != nil {
		t.Fatalf("failed to restart cron manager: %v", err)
	}
	defer cronMgr2.Stop()

	mm2 := NewMissionManagerV2(tmpDir, cronMgr2)
	if err := mm2.Start(); err != nil {
		t.Fatalf("failed to restart mission manager: %v", err)
	}
	defer mm2.Stop()

	// The scheduled mission should be re-registered in the cron manager
	jobs := cronMgr2.GetJobs()
	found := false
	for _, job := range jobs {
		if job.ID == "mission_"+mission.ID {
			found = true
			if job.CronExpr != "0 0 * * *" {
				t.Fatalf("expected cron expr 0 0 * * *, got %q", job.CronExpr)
			}
			break
		}
	}
	if !found {
		t.Fatal("scheduled mission was not re-registered in cron manager after restart")
	}
}

func TestScheduledMissionIdempotentRegistration(t *testing.T) {
	tmpDir := t.TempDir()
	cronMgr := NewCronManager(tmpDir)
	if err := cronMgr.Start(func(prompt string) {}); err != nil {
		t.Fatalf("failed to start cron manager: %v", err)
	}
	defer cronMgr.Stop()

	mm := NewMissionManagerV2(tmpDir, cronMgr)
	if err := mm.Start(); err != nil {
		t.Fatalf("failed to start mission manager: %v", err)
	}
	defer mm.Stop()

	mission := &MissionV2{
		Name:          "Test Idempotent",
		Prompt:        "do something",
		ExecutionType: ExecutionScheduled,
		Schedule:      "0 0 * * *",
		Enabled:       true,
	}
	if err := mm.Create(mission); err != nil {
		t.Fatalf("failed to create mission: %v", err)
	}

	// Trigger a second Start() (e.g. config reload) – should not duplicate jobs
	mm.Stop()
	if err := mm.Start(); err != nil {
		t.Fatalf("failed to re-start mission manager: %v", err)
	}

	jobs := cronMgr.GetJobs()
	count := 0
	for _, job := range jobs {
		if job.ID == "mission_"+mission.ID {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 cron job for mission, found %d", count)
	}
}

func TestCronManagerIdempotentAdd(t *testing.T) {
	tmpDir := t.TempDir()
	cronMgr := NewCronManager(tmpDir)
	if err := cronMgr.Start(func(prompt string) {}); err != nil {
		t.Fatalf("failed to start cron manager: %v", err)
	}
	defer cronMgr.Stop()

	_, err := cronMgr.ManageSchedule("add", "job1", "0 0 * * *", "prompt1", "")
	if err != nil {
		t.Fatalf("failed to add job: %v", err)
	}
	_, err = cronMgr.ManageSchedule("add", "job1", "0 0 * * *", "prompt1", "")
	if err != nil {
		t.Fatalf("failed to re-add job: %v", err)
	}

	jobs := cronMgr.GetJobs()
	count := 0
	for _, job := range jobs {
		if job.ID == "job1" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 cron job after idempotent add, found %d", count)
	}
}

func TestCronManagerLoadsFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	cronMgr := NewCronManager(tmpDir)
	if err := cronMgr.Start(func(prompt string) {}); err != nil {
		t.Fatalf("failed to start cron manager: %v", err)
	}

	_, err := cronMgr.ManageSchedule("add", "persisted", "0 0 * * *", "prompt", "")
	if err != nil {
		t.Fatalf("failed to add job: %v", err)
	}
	cronMgr.Stop()

	// Verify file exists
	data, err := os.ReadFile(filepath.Join(tmpDir, "crontab.json"))
	if err != nil {
		t.Fatalf("crontab file not saved: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("crontab file is empty")
	}

	// Restart and check job is loaded
	cronMgr2 := NewCronManager(tmpDir)
	if err := cronMgr2.Start(func(prompt string) {}); err != nil {
		t.Fatalf("failed to restart cron manager: %v", err)
	}
	defer cronMgr2.Stop()

	jobs := cronMgr2.GetJobs()
	found := false
	for _, job := range jobs {
		if job.ID == "persisted" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("persisted job not loaded after restart")
	}
}

func TestScheduledMissionNotRegisteredWhenDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	cronMgr := NewCronManager(tmpDir)
	if err := cronMgr.Start(func(prompt string) {}); err != nil {
		t.Fatalf("failed to start cron manager: %v", err)
	}
	defer cronMgr.Stop()

	mm := NewMissionManagerV2(tmpDir, cronMgr)
	if err := mm.Start(); err != nil {
		t.Fatalf("failed to start mission manager: %v", err)
	}
	defer mm.Stop()

	mission := &MissionV2{
		Name:          "Disabled Scheduled",
		Prompt:        "do something",
		ExecutionType: ExecutionScheduled,
		Schedule:      "0 0 * * *",
	}
	if err := mm.Create(mission); err != nil {
		t.Fatalf("failed to create mission: %v", err)
	}

	// Create always enables, so disable via update and verify removal
	mission.Enabled = false
	if err := mm.Update(mission.ID, mission); err != nil {
		t.Fatalf("failed to update mission: %v", err)
	}

	jobs := cronMgr.GetJobs()
	for _, job := range jobs {
		if job.ID == "mission_"+mission.ID {
			t.Fatal("disabled scheduled mission should not be registered in cron manager")
		}
	}
}
