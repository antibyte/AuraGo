package llm

import (
	"sync"
	"time"
)

// HealthEvent describes an LLM provider failure in a form that can be consumed
// by runtime health monitors without coupling the LLM package to UI warnings.
type HealthEvent struct {
	Operation         string
	Provider          string
	Model             string
	ErrorCategory     ErrorCategory
	ErrorSummary      string
	Attempt           int
	Retryable         bool
	PerAttemptTimeout time.Duration
	Timestamp         time.Time
}

// WithTimestamp returns a copy of the event with a concrete timestamp.
func (e HealthEvent) WithTimestamp(ts time.Time) HealthEvent {
	e.Timestamp = ts
	return e
}

// HealthSuccess identifies a successful LLM call that can reset provider
// health streaks.
type HealthSuccess struct {
	Provider  string
	Model     string
	Operation string
	Timestamp time.Time
}

// HealthReporter receives provider health events from retry and streaming code.
type HealthReporter interface {
	ReportLLMHealthEvent(HealthEvent)
	ReportLLMHealthSuccess(HealthSuccess)
}

var healthReporterState struct {
	mu       sync.RWMutex
	reporter HealthReporter
}

// SetHealthReporter installs the process-wide health reporter. Passing nil
// disables reporting.
func SetHealthReporter(reporter HealthReporter) {
	healthReporterState.mu.Lock()
	defer healthReporterState.mu.Unlock()
	healthReporterState.reporter = reporter
}

// ReportLLMHealthEvent reports an LLM provider failure if a reporter is wired.
func ReportLLMHealthEvent(event HealthEvent) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	healthReporterState.mu.RLock()
	reporter := healthReporterState.reporter
	healthReporterState.mu.RUnlock()
	if reporter != nil {
		reporter.ReportLLMHealthEvent(event)
	}
}

// ReportLLMHealthSuccess reports an LLM provider success if a reporter is wired.
func ReportLLMHealthSuccess(success HealthSuccess) {
	if success.Timestamp.IsZero() {
		success.Timestamp = time.Now()
	}
	healthReporterState.mu.RLock()
	reporter := healthReporterState.reporter
	healthReporterState.mu.RUnlock()
	if reporter != nil {
		reporter.ReportLLMHealthSuccess(success)
	}
}
