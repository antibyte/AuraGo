package agent

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
)

func newTestAutoJournalDB(t *testing.T) *memory.SQLiteMemory {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	t.Cleanup(func() { stm.Close() })
	return stm
}

func newTestAutoJournalConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Tools.Journal.Enabled = true
	cfg.Journal.AutoEntries = true
	return cfg
}

func TestJournalAutoTriggerCreatesActivityEntryForShortTurn(t *testing.T) {
	cfg := newTestAutoJournalConfig()
	stm := newTestAutoJournalDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))

	JournalAutoTrigger(cfg, stm, logger, "default", []string{"execute_shell"}, "Check the latest deployment logs")

	entries, err := stm.GetJournalEntries("", "", []string{"activity"}, 10)
	if err != nil {
		t.Fatalf("GetJournalEntries(activity): %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 activity entry, got %d", len(entries))
	}
	if entries[0].Importance != 1 {
		t.Fatalf("expected low-importance activity entry, got %d", entries[0].Importance)
	}
	if entries[0].Title == "" || entries[0].Title[:9] != "Activity:" {
		t.Fatalf("unexpected title %q", entries[0].Title)
	}
	if entries[0].Content == "" || entries[0].Tags[0] != "activity" {
		t.Fatalf("unexpected activity payload: %+v", entries[0])
	}
}

func TestJournalAutoTriggerCreatesActivityEntryWithoutTools(t *testing.T) {
	cfg := newTestAutoJournalConfig()
	stm := newTestAutoJournalDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))

	JournalAutoTrigger(cfg, stm, logger, "default", nil, "Summarize what we decided about backups")

	entries, err := stm.GetJournalEntries("", "", []string{"activity"}, 10)
	if err != nil {
		t.Fatalf("GetJournalEntries(activity): %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 activity entry, got %d", len(entries))
	}
	if entries[0].Content == "" || !strings.Contains(entries[0].Content, "Tools used: none") {
		t.Fatalf("expected no-tools marker, got %q", entries[0].Content)
	}
}

func TestJournalAutoTriggerKeepsExistingAutoEntriesAlongsideActivity(t *testing.T) {
	cfg := newTestAutoJournalConfig()
	stm := newTestAutoJournalDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))

	JournalAutoTrigger(cfg, stm, logger, "default", []string{"execute_shell", "manage_memory", "docker", "docker"}, "Fix Docker env and store the preference")

	activityEntries, err := stm.GetJournalEntries("", "", []string{"activity"}, 10)
	if err != nil {
		t.Fatalf("GetJournalEntries(activity): %v", err)
	}
	if len(activityEntries) != 1 {
		t.Fatalf("expected 1 activity entry, got %d", len(activityEntries))
	}

	taskEntries, err := stm.GetJournalEntries("", "", []string{"task_completed"}, 10)
	if err != nil {
		t.Fatalf("GetJournalEntries(task_completed): %v", err)
	}
	if len(taskEntries) != 1 {
		t.Fatalf("expected 1 task_completed entry, got %d", len(taskEntries))
	}

	preferenceEntries, err := stm.GetJournalEntries("", "", []string{"preference"}, 10)
	if err != nil {
		t.Fatalf("GetJournalEntries(preference): %v", err)
	}
	if len(preferenceEntries) != 1 {
		t.Fatalf("expected 1 preference entry, got %d", len(preferenceEntries))
	}
}
