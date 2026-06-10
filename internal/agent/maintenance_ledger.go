package agent

import (
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
)

type maintenanceRunLedger struct {
	phaseResults memory.MaintenancePhaseResults
	failed       bool
}

func newMaintenanceRunLedger() *maintenanceRunLedger {
	return &maintenanceRunLedger{}
}

func (l *maintenanceRunLedger) addError(msg string) {
	if l == nil || msg == "" {
		return
	}
	l.phaseResults.Errors = append(l.phaseResults.Errors, msg)
}

func (l *maintenanceRunLedger) markFailed() {
	if l == nil {
		return
	}
	l.failed = true
}

func (l *maintenanceRunLedger) status() string {
	if l == nil {
		return "completed"
	}
	if l.failed {
		return "failed"
	}
	if len(l.phaseResults.Errors) > 0 {
		return "partial"
	}
	return "completed"
}

func (l *maintenanceRunLedger) results() memory.MaintenancePhaseResults {
	if l == nil {
		return memory.MaintenancePhaseResults{}
	}
	return l.phaseResults
}

type memoryHygieneStats struct {
	JournalRemoved    int
	NotesArchived     int
	CanonicalRepaired int
}

// ComputeNextMaintenanceRun returns the next scheduled maintenance time in local time.
func ComputeNextMaintenanceRun(cfg *config.Config, now time.Time) time.Time {
	if cfg == nil {
		now = time.Now()
		return time.Date(now.Year(), now.Month(), now.Day(), 4, 0, 0, 0, now.Location()).Add(24 * time.Hour)
	}
	hour, minute, err := parseTime(cfg.Maintenance.Time)
	if err != nil {
		hour, minute = 4, 0
	}
	nextRun := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if !now.Before(nextRun) {
		nextRun = nextRun.Add(24 * time.Hour)
	}
	return nextRun
}