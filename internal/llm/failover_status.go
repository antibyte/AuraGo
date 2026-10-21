package llm

import "time"

// FailoverProbeStatus describes the last background recovery probe; reading it
// never schedules a new probe and never exposes connection credentials.
type FailoverProbeStatus struct {
	State     string        `json:"state"`
	Updated   time.Time     `json:"updated,omitempty"`
	Category  ErrorCategory `json:"category,omitempty"`
	NextAfter time.Time     `json:"next_after,omitempty"`
}
type FailoverStatus struct {
	OnFallback     bool                `json:"on_fallback"`
	PrimaryErrors  int                 `json:"primary_errors"`
	FallbackErrors int                 `json:"fallback_errors"`
	Probe          FailoverProbeStatus `json:"probe"`
}

func (fm *FailoverManager) PassiveStatus() FailoverStatus {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	probe := fm.probeStatus
	if probe.State == "" {
		probe.State = "idle"
	}
	return FailoverStatus{OnFallback: fm.isOnFallback, PrimaryErrors: fm.errorCount, FallbackErrors: fm.fallbackErrorCount, Probe: probe}
}
func (fm *FailoverManager) setProbeStatus(generation int, state string, err error, next time.Time) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	if fm.generation != generation {
		return
	}
	fm.probeStatus = FailoverProbeStatus{State: state, Updated: time.Now(), Category: ClassifyError(err), NextAfter: next}
}
