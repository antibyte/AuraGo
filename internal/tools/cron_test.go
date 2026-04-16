package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCronManagerPersistsToSQLiteStore(t *testing.T) {
	dir := t.TempDir()
	mgr := NewCronManager(dir)
	t.Cleanup(func() { _ = mgr.Close() })
	mgr.callback = func(prompt string) {}

	if _, err := mgr.ManageSchedule("add", "job-1", "0 * * * *", "run cleanup", "en"); err != nil {
		t.Fatalf("ManageSchedule add: %v", err)
	}

	target := filepath.Join(dir, systemTaskStoreFile)
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected sqlite system task store to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "crontab.json.tmp")); !os.IsNotExist(err) {
		t.Fatalf("expected no leftover temp file, got err=%v", err)
	}

	var jobs []CronJob
	loaded, err := mgr.store.load(systemTaskNamespaceCron, &jobs)
	if err != nil {
		t.Fatalf("load persisted cron jobs: %v", err)
	}
	if !loaded {
		t.Fatal("expected persisted cron jobs in sqlite store")
	}
	if len(jobs) != 1 || jobs[0].ID != "job-1" {
		data, _ := json.Marshal(jobs)
		t.Fatalf("unexpected persisted jobs: %s", data)
	}
}

func TestCronManagerMigratesLegacyJSON(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "crontab.json")
	if err := os.WriteFile(legacy, []byte(`[{"id":"legacy-job","cron_expr":"0 0 * * *","task_prompt":"legacy prompt"}]`), 0o644); err != nil {
		t.Fatalf("write legacy cron json: %v", err)
	}

	mgr := NewCronManager(dir)
	t.Cleanup(func() { _ = mgr.Close() })
	if err := mgr.Start(func(string) {}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	jobs := mgr.GetJobs()
	if len(jobs) != 1 || jobs[0].ID != "legacy-job" {
		t.Fatalf("unexpected migrated jobs: %+v", jobs)
	}

	var stored []CronJob
	loaded, err := mgr.store.load(systemTaskNamespaceCron, &stored)
	if err != nil {
		t.Fatalf("load migrated cron jobs: %v", err)
	}
	if !loaded || len(stored) != 1 || stored[0].ID != "legacy-job" {
		t.Fatalf("unexpected store contents after migration: %+v", stored)
	}
	if data, err := os.ReadFile(legacy); err != nil || !strings.Contains(string(data), "legacy-job") {
		t.Fatalf("expected legacy file to remain readable, err=%v data=%q", err, string(data))
	}
}

func TestCronManagerAcceptsSecondsFieldExpressions(t *testing.T) {
	mgr := NewCronManager(t.TempDir())
	t.Cleanup(func() { _ = mgr.Close() })
	mgr.callback = func(prompt string) {}
	result, err := mgr.ManageSchedule("add", "job-seconds", "0 */15 * * * *", "run every fifteen minutes", "en")
	if err != nil {
		t.Fatalf("ManageSchedule add with seconds field: %v", err)
	}
	if !strings.Contains(result, `"status": "success"`) {
		t.Fatalf("expected success response, got %s", result)
	}
}
