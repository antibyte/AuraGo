package memory

import (
	"log/slog"
	"path/filepath"
	"testing"
	"time"
)

func TestClaimMaintenanceDayPersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	started := time.Date(2026, 9, 22, 4, 0, 0, 0, time.UTC)

	first, err := NewSQLiteMemory(path, slog.Default())
	if err != nil {
		t.Fatalf("open first memory: %v", err)
	}
	claimed, err := first.ClaimMaintenanceDay(started)
	if err != nil || !claimed {
		t.Fatalf("first claim = (%v, %v), want (true, nil)", claimed, err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first memory: %v", err)
	}

	second, err := NewSQLiteMemory(path, slog.Default())
	if err != nil {
		t.Fatalf("reopen memory: %v", err)
	}
	defer second.Close()
	claimed, err = second.ClaimMaintenanceDay(started)
	if err != nil || claimed {
		t.Fatalf("same-day restart claim = (%v, %v), want (false, nil)", claimed, err)
	}
	nextDay := started.AddDate(0, 0, 1)
	claimed, err = second.ClaimMaintenanceDay(nextDay)
	if err != nil || !claimed {
		t.Fatalf("next-day claim = (%v, %v), want (true, nil)", claimed, err)
	}
}

func TestClaimMaintenanceDaySeedsFromLatestRunAfterUpgrade(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	started := time.Date(2026, 9, 22, 4, 0, 0, 0, time.UTC)
	first, err := NewSQLiteMemory(path, slog.Default())
	if err != nil {
		t.Fatalf("open first memory: %v", err)
	}
	if err := first.InsertMaintenanceRun(started, started.Add(time.Minute), "completed", MaintenancePhaseResults{}); err != nil {
		t.Fatalf("insert maintenance run: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first memory: %v", err)
	}

	second, err := NewSQLiteMemory(path, slog.Default())
	if err != nil {
		t.Fatalf("reopen memory: %v", err)
	}
	defer second.Close()
	claimed, err := second.ClaimMaintenanceDay(started.Add(30 * time.Minute))
	if err != nil || claimed {
		t.Fatalf("upgrade seed claim = (%v, %v), want (false, nil)", claimed, err)
	}
	state, err := second.GetMemoryMaintenanceState(MaintenanceAutomaticDayKey)
	if err != nil || state != "2026-09-22" {
		t.Fatalf("seeded state = (%q, %v), want 2026-09-22", state, err)
	}
	claimed, err = second.ClaimMaintenanceDay(started.AddDate(0, 0, 1))
	if err != nil || !claimed {
		t.Fatalf("post-upgrade next-day claim = (%v, %v), want (true, nil)", claimed, err)
	}
}

func TestClaimMaintenanceDayFailsClosedWithoutMemory(t *testing.T) {
	claimed, err := (*SQLiteMemory)(nil).ClaimMaintenanceDay(time.Now())
	if err == nil || claimed {
		t.Fatalf("nil memory claim = (%v, %v), want (false, error)", claimed, err)
	}
}
