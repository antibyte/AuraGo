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

func TestReleaseMaintenanceDayClaimRequiresMatchingOwner(t *testing.T) {
	stm := newMaintenanceProgressStore(t, ":memory:")
	first := time.Date(2026, 9, 22, 4, 0, 0, 123456789, time.UTC)
	claimed, err := stm.ClaimMaintenanceDay(first)
	if err != nil || !claimed {
		t.Fatalf("first claim=(%v,%v), want true,nil", claimed, err)
	}

	wrongAttempt := first.Add(time.Second)
	if released, err := stm.ReleaseMaintenanceDayClaim(wrongAttempt); err != nil || released {
		t.Fatalf("wrong-attempt release=(%v,%v), want false,nil", released, err)
	}
	if claimed, err := stm.IsMaintenanceDayClaimed(first); err != nil || !claimed {
		t.Fatalf("wrong release cleared claim=(%v,%v), want true,nil", claimed, err)
	}

	if released, err := stm.ReleaseMaintenanceDayClaim(first); err != nil || !released {
		t.Fatalf("owner release=(%v,%v), want true,nil", released, err)
	}
	second := first.Add(2 * time.Second)
	claimed, err = stm.ClaimMaintenanceDay(second)
	if err != nil || !claimed {
		t.Fatalf("reclaim after release=(%v,%v), want true,nil", claimed, err)
	}
	if released, err := stm.ReleaseMaintenanceDayClaim(first); err != nil || released {
		t.Fatalf("late old-owner release=(%v,%v), want false,nil", released, err)
	}
	if claimed, err := stm.IsMaintenanceDayClaimed(second); err != nil || !claimed {
		t.Fatalf("late release cleared new claim=(%v,%v), want true,nil", claimed, err)
	}
}

func TestReleaseMaintenanceDayClaimPreservesLegacyDayOnlyClaim(t *testing.T) {
	stm := newMaintenanceProgressStore(t, ":memory:")
	started := time.Date(2026, 9, 22, 4, 0, 0, 0, time.UTC)
	if err := stm.SetMemoryMaintenanceState(MaintenanceAutomaticDayKey, started.Format("2006-01-02")); err != nil {
		t.Fatal(err)
	}
	if released, err := stm.ReleaseMaintenanceDayClaim(started); err != nil || released {
		t.Fatalf("legacy claim release=(%v,%v), want false,nil", released, err)
	}
	if claimed, err := stm.IsMaintenanceDayClaimed(started); err != nil || !claimed {
		t.Fatalf("legacy claim was lost=(%v,%v), want true,nil", claimed, err)
	}
	if claimed, err := stm.ClaimMaintenanceDay(started); err != nil || claimed {
		t.Fatalf("claim over legacy day-only state=(%v,%v), want false,nil", claimed, err)
	}
}
