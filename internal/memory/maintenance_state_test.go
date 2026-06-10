package memory

import (
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestShouldSkipDailyReflectionBecauseMaintenance(t *testing.T) {
	t.Cleanup(ResetMaintenanceRunMarker)

	if ShouldSkipDailyReflectionBecauseMaintenance(nil) {
		t.Fatal("expected nil stm to not skip reflection")
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	if ShouldSkipDailyReflectionBecauseMaintenance(stm) {
		t.Fatal("expected skip=false before maintenance marker is recorded")
	}

	RecordMaintenanceRunCompleted(time.Now())
	if ShouldSkipDailyReflectionBecauseMaintenance(stm) {
		t.Fatal("expected skip=false when maintenance ran but no daily summary exists")
	}

	today := time.Now().Format("2006-01-02")
	if err := stm.InsertDailySummary(DailySummary{
		Date:    today,
		Summary: "Maintenance produced today's summary.",
	}); err != nil {
		t.Fatalf("InsertDailySummary: %v", err)
	}
	if !ShouldSkipDailyReflectionBecauseMaintenance(stm) {
		t.Fatal("expected skip=true when maintenance recently produced today's summary")
	}
}