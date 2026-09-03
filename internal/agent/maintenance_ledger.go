package agent

import (
	"regexp"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
)

type maintenanceRunLedger struct {
	phaseResults memory.MaintenancePhaseResults
	failed       bool
	currentPhase string
	phaseStarted time.Time
}

func newMaintenanceRunLedger() *maintenanceRunLedger {
	return &maintenanceRunLedger{phaseStarted: time.Now()}
}

func (l *maintenanceRunLedger) addError(msg string) {
	if l == nil || msg == "" {
		return
	}
	code := sanitizeMaintenanceErrorCode(msg)
	if code == "" {
		return
	}
	l.phaseResults.Errors = append(l.phaseResults.Errors, code)
	for i := range l.phaseResults.Phases {
		if l.phaseResults.Phases[i].Name == l.currentPhase {
			l.phaseResults.Phases[i].ErrorCodes = appendUniqueMaintenanceCode(l.phaseResults.Phases[i].ErrorCodes, code)
		}
	}
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
	if l.phaseResults.Deferred > 0 {
		return "partial"
	}
	return "completed"
}

func (l *maintenanceRunLedger) beginPhase(name string) {
	if l == nil {
		return
	}
	name = sanitizeMaintenanceErrorCode(name)
	l.currentPhase = name
	l.phaseStarted = time.Now()
	for _, phase := range l.phaseResults.Phases {
		if phase.Name == name {
			return
		}
	}
	l.phaseResults.Phases = append(l.phaseResults.Phases, memory.MaintenancePhaseResult{Name: name, Status: "running"})
}

func (l *maintenanceRunLedger) addProcessed(phase string, count int) {
	if l == nil || count <= 0 {
		return
	}
	l.phaseResults.Processed += count
	for i := range l.phaseResults.Phases {
		if l.phaseResults.Phases[i].Name == phase {
			l.phaseResults.Phases[i].Processed += count
		}
	}
}

func (l *maintenanceRunLedger) addDeferred(phase string, count int) {
	if l == nil || count <= 0 {
		return
	}
	l.phaseResults.Deferred += count
	for i := range l.phaseResults.Phases {
		if l.phaseResults.Phases[i].Name == phase {
			l.phaseResults.Phases[i].Deferred += count
		}
	}
}

func (l *maintenanceRunLedger) phaseDeferred(name string) int {
	if l == nil {
		return 0
	}
	for _, phase := range l.phaseResults.Phases {
		if phase.Name == name {
			return phase.Deferred
		}
	}
	return 0
}

func (l *maintenanceRunLedger) finishPhase(name string, deferred bool) {
	if l == nil {
		return
	}
	now := time.Now()
	matched := false
	for i := range l.phaseResults.Phases {
		phase := &l.phaseResults.Phases[i]
		if phase.Name != name {
			continue
		}
		if phase.DurationMS == 0 {
			phase.DurationMS = now.Sub(l.phaseStarted).Milliseconds()
		}
		switch {
		case deferred || phase.Deferred > 0:
			phase.Status = "partial"
		case len(phase.ErrorCodes) > 0:
			phase.Status = "partial"
		default:
			phase.Status = "completed"
		}
		matched = true
		break
	}
	if matched && l.currentPhase == name {
		l.currentPhase = ""
	}
}

var maintenanceErrorCodePattern = regexp.MustCompile(`[^a-z0-9_]+`)

func sanitizeMaintenanceErrorCode(message string) string {
	code := strings.TrimSpace(message)
	if colon := strings.IndexByte(code, ':'); colon >= 0 {
		code = code[:colon]
	}
	code = strings.ToLower(strings.TrimSpace(code))
	code = maintenanceErrorCodePattern.ReplaceAllString(code, "_")
	return strings.Trim(code, "_")
}

func appendUniqueMaintenanceCode(codes []string, code string) []string {
	for _, existing := range codes {
		if existing == code {
			return codes
		}
	}
	return append(codes, code)
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
