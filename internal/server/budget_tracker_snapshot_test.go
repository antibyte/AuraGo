package server

import (
	"io"
	"log/slog"
	"testing"

	"aurago/internal/config"
)

func TestBudgetTrackerSnapshotSeesTrackerEnabledAfterBotStartup(t *testing.T) {
	server := &Server{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	server.CfgMu.Lock()
	server.reinitBudgetTracker(&config.Config{})
	server.CfgMu.Unlock()

	// Long-lived channel runtimes capture the getter, not its initial nil value.
	trackerSnapshot := server.budgetTrackerSnapshot
	if got := trackerSnapshot(); got != nil {
		t.Fatalf("tracker before budget enable = %p, want nil", got)
	}

	enabled := &config.Config{}
	enabled.Budget.Enabled = true
	enabled.Directories.DataDir = t.TempDir()
	server.CfgMu.Lock()
	server.reinitBudgetTracker(enabled)
	server.CfgMu.Unlock()

	tracker := trackerSnapshot()
	if tracker == nil {
		t.Fatal("captured tracker getter did not observe the hot-enabled tracker")
	}
	t.Cleanup(tracker.Flush)
}
