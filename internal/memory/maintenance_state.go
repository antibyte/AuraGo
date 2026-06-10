package memory

import (
	"sync"
	"time"
)

var maintenanceRunMarker struct {
	sync.RWMutex
	completedAt time.Time
}

// RecordMaintenanceRunCompleted stores the timestamp of the most recent nightly maintenance run.
func RecordMaintenanceRunCompleted(at time.Time) {
	maintenanceRunMarker.Lock()
	defer maintenanceRunMarker.Unlock()
	maintenanceRunMarker.completedAt = at
}

// MaintenanceRunCompletedWithin reports whether maintenance finished within the given duration.
func MaintenanceRunCompletedWithin(d time.Duration) bool {
	maintenanceRunMarker.RLock()
	defer maintenanceRunMarker.RUnlock()
	if maintenanceRunMarker.completedAt.IsZero() {
		return false
	}
	return time.Since(maintenanceRunMarker.completedAt) < d
}

// ResetMaintenanceRunMarker clears the in-process maintenance marker (tests only).
func ResetMaintenanceRunMarker() {
	maintenanceRunMarker.Lock()
	defer maintenanceRunMarker.Unlock()
	maintenanceRunMarker.completedAt = time.Time{}
}

// ShouldSkipDailyReflectionBecauseMaintenance returns true when nightly maintenance recently
// produced a daily summary, so the 03:00 reflection loop can avoid duplicate LLM work.
func maintenanceRecentlyCompleted(stm *SQLiteMemory) bool {
	if MaintenanceRunCompletedWithin(24 * time.Hour) {
		return true
	}
	if stm == nil {
		return false
	}
	record, err := stm.GetLatestMaintenanceRun()
	if err != nil || record == nil || record.FinishedAt == "" {
		return false
	}
	finishedAt, err := time.Parse(time.RFC3339, record.FinishedAt)
	if err != nil {
		return false
	}
	return time.Since(finishedAt) < 24*time.Hour
}

func ShouldSkipDailyReflectionBecauseMaintenance(stm *SQLiteMemory) bool {
	if stm == nil || !maintenanceRecentlyCompleted(stm) {
		return false
	}
	today := time.Now().Format("2006-01-02")
	if summary, _ := stm.GetDailySummary(today); summary != nil {
		return true
	}
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	if summary, _ := stm.GetDailySummary(yesterday); summary != nil {
		return true
	}
	return false
}