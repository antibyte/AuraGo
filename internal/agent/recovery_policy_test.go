package agent

import (
	"testing"
	"time"

	"aurago/internal/config"
)

func TestBuildRecoveryPolicyUsesConfigOverrides(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.Recovery.MaxProvider422Recoveries = 4
	cfg.Agent.Recovery.MinMessagesForEmptyRetry = 8
	cfg.Agent.Recovery.DuplicateConsecutiveHits = 5
	cfg.Agent.Recovery.DuplicateFrequencyHits = 6
	cfg.Agent.Recovery.IdenticalToolErrorHits = 7

	policy := buildRecoveryPolicy(cfg)

	if policy.MaxProvider422Recoveries != 4 {
		t.Fatalf("MaxProvider422Recoveries = %d, want 4", policy.MaxProvider422Recoveries)
	}
	if policy.MinMessagesForEmptyRetry != 8 {
		t.Fatalf("MinMessagesForEmptyRetry = %d, want 8", policy.MinMessagesForEmptyRetry)
	}
	if policy.DuplicateConsecutiveHits != 5 {
		t.Fatalf("DuplicateConsecutiveHits = %d, want 5", policy.DuplicateConsecutiveHits)
	}
	if policy.DuplicateFrequencyHits != 6 {
		t.Fatalf("DuplicateFrequencyHits = %d, want 6", policy.DuplicateFrequencyHits)
	}
	if policy.IdenticalToolErrorHits != 7 {
		t.Fatalf("IdenticalToolErrorHits = %d, want 7", policy.IdenticalToolErrorHits)
	}
}

func TestBuildRecoveryPolicyFallsBackToDefaults(t *testing.T) {
	policy := buildRecoveryPolicy(&config.Config{})
	defaults := defaultRecoveryPolicy()

	if policy.MaxProvider422Recoveries != defaults.MaxProvider422Recoveries {
		t.Fatalf("MaxProvider422Recoveries = %d, want %d", policy.MaxProvider422Recoveries, defaults.MaxProvider422Recoveries)
	}
	if policy.MinMessagesForEmptyRetry != defaults.MinMessagesForEmptyRetry {
		t.Fatalf("MinMessagesForEmptyRetry = %d, want %d", policy.MinMessagesForEmptyRetry, defaults.MinMessagesForEmptyRetry)
	}
	if len(policy.EmptyRetryIntervals) != len(defaults.EmptyRetryIntervals) {
		t.Fatalf("EmptyRetryIntervals len = %d, want %d", len(policy.EmptyRetryIntervals), len(defaults.EmptyRetryIntervals))
	}
}

func TestEmptyRetryIntervalsDefault(t *testing.T) {
	policy := defaultRecoveryPolicy()
	intervals := policy.emptyRetryIntervals()
	if len(intervals) != 1 {
		t.Fatalf("expected 1 default interval, got %d", len(intervals))
	}
	if intervals[0] != 5*time.Second {
		t.Fatalf("default interval = %v, want 5s", intervals[0])
	}
}

func TestEmptyRetryIntervalsEmptySliceReturnsDefault(t *testing.T) {
	policy := RecoveryPolicy{EmptyRetryIntervals: []time.Duration{}}
	intervals := policy.emptyRetryIntervals()
	if len(intervals) != 1 || intervals[0] != 5*time.Second {
		t.Fatalf("expected default 5s interval, got %v", intervals)
	}
}

func TestEmptyRetryBaseDelayReturnsFirstInterval(t *testing.T) {
	policy := RecoveryPolicy{EmptyRetryIntervals: []time.Duration{3 * time.Second, 10 * time.Second}}
	delay := policy.emptyRetryBaseDelay()
	if delay != 3*time.Second {
		t.Fatalf("emptyRetryBaseDelay = %v, want 3s", delay)
	}
}

func TestEmptyRetryBaseDelayEmptyReturnsDefault(t *testing.T) {
	policy := RecoveryPolicy{EmptyRetryIntervals: []time.Duration{}}
	delay := policy.emptyRetryBaseDelay()
	if delay != 5*time.Second {
		t.Fatalf("emptyRetryBaseDelay = %v, want 5s default", delay)
	}
}
