package llm

import (
	"sync"
	"time"
)

// ProviderHealthSnapshot contains passive observations, never provider secrets.
type ProviderHealthSnapshot struct {
	LastSuccess *HealthSuccess `json:"last_success,omitempty"`
	LastFailure *HealthEvent   `json:"last_failure,omitempty"`
}

var passiveHealth = struct {
	sync.RWMutex
	entries map[string]ProviderHealthSnapshot
}{entries: make(map[string]ProviderHealthSnapshot)}

// ProviderHealth returns only observations explicitly attributed to this ID.
func ProviderHealth(id string) ProviderHealthSnapshot {
	passiveHealth.RLock()
	defer passiveHealth.RUnlock()
	state := passiveHealth.entries[id]
	if state.LastSuccess != nil {
		copy := *state.LastSuccess
		state.LastSuccess = &copy
	}
	if state.LastFailure != nil {
		copy := *state.LastFailure
		state.LastFailure = &copy
	}
	return state
}
func recordPassiveFailure(event HealthEvent) {
	if event.ProviderID == "" {
		return
	}
	passiveHealth.Lock()
	defer passiveHealth.Unlock()
	if len(passiveHealth.entries) >= 256 {
		if _, ok := passiveHealth.entries[event.ProviderID]; !ok {
			return
		}
	}
	state := passiveHealth.entries[event.ProviderID]
	state.LastFailure = &event
	passiveHealth.entries[event.ProviderID] = state
}
func recordPassiveSuccess(event HealthSuccess) {
	if event.ProviderID == "" {
		return
	}
	passiveHealth.Lock()
	defer passiveHealth.Unlock()
	if len(passiveHealth.entries) >= 256 {
		if _, ok := passiveHealth.entries[event.ProviderID]; !ok {
			return
		}
	}
	state := passiveHealth.entries[event.ProviderID]
	state.LastSuccess = &event
	passiveHealth.entries[event.ProviderID] = state
}
func ReconfigureHealthReporter(threshold int) {
	healthReporterState.mu.RLock()
	reporter := healthReporterState.reporter
	healthReporterState.mu.RUnlock()
	if configurable, ok := reporter.(interface{ Configure(int) }); ok {
		configurable.Configure(threshold)
	}
}

// HealthEvent describes an LLM provider failure in a form that can be consumed
// by runtime health monitors without coupling the LLM package to UI warnings.
type HealthEvent struct {
	ProviderID        string `json:"provider_id,omitempty"`
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
	ProviderID string `json:"provider_id,omitempty"`
	Provider   string
	Model      string
	Operation  string
	Timestamp  time.Time
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
	event.ErrorSummary = redactErrorText(event.ErrorSummary)
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	healthReporterState.mu.RLock()
	recordPassiveFailure(event)
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
	recordPassiveSuccess(success)
	reporter := healthReporterState.reporter
	healthReporterState.mu.RUnlock()
	if reporter != nil {
		reporter.ReportLLMHealthSuccess(success)
	}
}
