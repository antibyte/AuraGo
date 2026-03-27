package agent

import (
	"testing"

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

	if policy != defaults {
		t.Fatalf("policy = %+v, want defaults %+v", policy, defaults)
	}
}
