package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCronManagerSaveIsAtomic(t *testing.T) {
	dir := t.TempDir()
	mgr := NewCronManager(dir)
	mgr.callback = func(prompt string) {}

	if _, err := mgr.ManageSchedule("add", "job-1", "0 * * * * *", "run cleanup", "en"); err != nil {
		t.Fatalf("ManageSchedule add: %v", err)
	}

	target := filepath.Join(dir, "crontab.json")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected cron file to exist: %v", err)
	}
	if _, err := os.Stat(target + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("expected no leftover temp file, got err=%v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read cron file: %v", err)
	}
	if !strings.Contains(string(data), "job-1") {
		t.Fatalf("cron file does not contain saved job: %s", string(data))
	}
}
