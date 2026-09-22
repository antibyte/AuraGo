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

// maintenancePhaseOutcome is internal; the persisted ledger remains stable.
// Helpers report observed work and failures without deciding public run status.
type maintenancePhaseOutcome struct {
	Skipped    bool
	Failed     bool
	Processed  int
	Deferred   int
	ErrorCodes []string
}

func (l *maintenanceRunLedger) applyOutcome(name string, outcome maintenancePhaseOutcome) {
	if outcome.Skipped {
		l.skipPhase(name)
		return
	}
	l.addProcessed(name, outcome.Processed)
	l.addDeferred(name, outcome.Deferred)
	for _, code := range outcome.ErrorCodes {
		l.addError(code)
	}
	if outcome.Failed {
		l.markFailed()
	}
	l.finishPhase(name, false)
}

func (l *maintenanceRunLedger) recordError(code string, err error) {
	if err != nil {
		l.addError(code + ": " + err.Error())
	}
}

// recordNamedError preserves failures from combined work even when cancellation
// prevents reaching the phase that owns the failed half.
func (l *maintenanceRunLedger) recordNamedError(name, code string, err error) {
	if l == nil || err == nil {
		return
	}
	code = sanitizeMaintenanceErrorCode(code)
	l.phaseResults.Errors = appendUniqueMaintenanceCode(l.phaseResults.Errors, code)
	for i := range l.phaseResults.Phases {
		phase := &l.phaseResults.Phases[i]
		if phase.Name == name {
			phase.ErrorCodes = appendUniqueMaintenanceCode(phase.ErrorCodes, code)
			if phase.Status != "failed" {
				phase.Status = "partial"
			}
			return
		}
	}
	l.phaseResults.Phases = append(l.phaseResults.Phases, memory.MaintenancePhaseResult{
		Name: name, Status: "partial", ErrorCodes: []string{code},
	})
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
	for i := range l.phaseResults.Phases {
		if l.phaseResults.Phases[i].Name == l.currentPhase {
			l.phaseResults.Phases[i].Status = "failed"
		}
	}
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
	for _, phase := range l.phaseResults.Phases {
		if phase.Status == "partial" || phase.Status == "running" {
			return "partial"
		}
	}
	return "completed"
}

func (l *maintenanceRunLedger) beginPhase(name string) {
	if l == nil {
		return
	}
	name = sanitizeMaintenanceErrorCode(name)
	if l.currentPhase != "" && l.currentPhase != name {
		l.addError("phase_not_finished")
		l.finishPhase(l.currentPhase, true)
	}
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

func (l *maintenanceRunLedger) addPhaseCode(phase, code string) {
	if l == nil {
		return
	}
	code = sanitizeMaintenanceErrorCode(code)
	if code == "" {
		return
	}
	for i := range l.phaseResults.Phases {
		if l.phaseResults.Phases[i].Name == phase {
			l.phaseResults.Phases[i].ErrorCodes = appendUniqueMaintenanceCode(l.phaseResults.Phases[i].ErrorCodes, code)
			return
		}
	}
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
		if phase.Status != "running" && phase.Status != "failed" {
			return
		}
		if phase.DurationMS == 0 {
			phase.DurationMS = now.Sub(l.phaseStarted).Milliseconds()
		}
		switch {
		case phase.Status == "failed":
		case deferred || phase.Deferred > 0:
			phase.Status = "partial"
		case len(phase.ErrorCodes) > 0:
			phase.Status = "partial"
		default:
			phase.Status = "completed"
		}
		if phase.Status == "partial" && len(phase.ErrorCodes) == 0 {
			phase.ErrorCodes = append(phase.ErrorCodes, "deferred_work")
		}
		matched = true
		break
	}
	if matched && l.currentPhase == name {
		l.currentPhase = ""
	}
}

var maintenanceErrorCodePattern = regexp.MustCompile(`[^a-z0-9_]+`)

func (l *maintenanceRunLedger) skipPhase(name string) {
	if l == nil || l.currentPhase != name {
		return
	}
	l.finishPhase(name, false)
	for i := range l.phaseResults.Phases {
		phase := &l.phaseResults.Phases[i]
		if phase.Name == name && phase.Status == "completed" {
			phase.Status = "skipped"
		}
	}
}

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
	Errors            []error
	Deferred          int
}

// ComputeNextMaintenanceRun returns the next scheduled maintenance time in local time.
func ComputeNextMaintenanceRun(cfg *config.Config, now time.Time) time.Time {
	return computeNextMaintenanceRunCalendar(cfg, now)
}
