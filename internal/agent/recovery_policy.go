package agent

import (
	"time"

	"aurago/internal/config"
)

// RecoveryPolicy centralizes retry and circuit-breaker thresholds so recovery
// behavior is defined in one place instead of being spread across helpers.
type RecoveryPolicy struct {
	MaxProvider422Recoveries int
	MinMessagesForEmptyRetry int
	DuplicateConsecutiveHits int
	DuplicateFrequencyHits   int
	IdenticalToolErrorHits   int
	EmptyRetryIntervals      []time.Duration
}

func defaultRecoveryPolicy() RecoveryPolicy {
	return RecoveryPolicy{
		MaxProvider422Recoveries: 3,
		MinMessagesForEmptyRetry: 5,
		DuplicateConsecutiveHits: 2,
		DuplicateFrequencyHits:   3,
		IdenticalToolErrorHits:   3,
		EmptyRetryIntervals:      []time.Duration{5 * time.Second},
	}
}

func buildRecoveryPolicy(cfg *config.Config) RecoveryPolicy {
	policy := defaultRecoveryPolicy()
	if cfg == nil {
		return policy
	}
	if cfg.Agent.Recovery.MaxProvider422Recoveries > 0 {
		policy.MaxProvider422Recoveries = cfg.Agent.Recovery.MaxProvider422Recoveries
	}
	if cfg.Agent.Recovery.MinMessagesForEmptyRetry > 0 {
		policy.MinMessagesForEmptyRetry = cfg.Agent.Recovery.MinMessagesForEmptyRetry
	}
	if cfg.Agent.Recovery.DuplicateConsecutiveHits > 0 {
		policy.DuplicateConsecutiveHits = cfg.Agent.Recovery.DuplicateConsecutiveHits
	}
	if cfg.Agent.Recovery.DuplicateFrequencyHits > 0 {
		policy.DuplicateFrequencyHits = cfg.Agent.Recovery.DuplicateFrequencyHits
	}
	if cfg.Agent.Recovery.IdenticalToolErrorHits > 0 {
		policy.IdenticalToolErrorHits = cfg.Agent.Recovery.IdenticalToolErrorHits
	}
	return policy
}

func (p RecoveryPolicy) maxProvider422Recoveries() int {
	if p.MaxProvider422Recoveries <= 0 {
		return defaultRecoveryPolicy().MaxProvider422Recoveries
	}
	return p.MaxProvider422Recoveries
}

func (p RecoveryPolicy) minMessagesForEmptyRetry() int {
	if p.MinMessagesForEmptyRetry <= 0 {
		return defaultRecoveryPolicy().MinMessagesForEmptyRetry
	}
	return p.MinMessagesForEmptyRetry
}

func (p RecoveryPolicy) duplicateConsecutiveHits() int {
	if p.DuplicateConsecutiveHits <= 0 {
		return defaultRecoveryPolicy().DuplicateConsecutiveHits
	}
	return p.DuplicateConsecutiveHits
}

func (p RecoveryPolicy) duplicateFrequencyHits() int {
	if p.DuplicateFrequencyHits <= 0 {
		return defaultRecoveryPolicy().DuplicateFrequencyHits
	}
	return p.DuplicateFrequencyHits
}

func (p RecoveryPolicy) identicalToolErrorHits() int {
	if p.IdenticalToolErrorHits <= 0 {
		return defaultRecoveryPolicy().IdenticalToolErrorHits
	}
	return p.IdenticalToolErrorHits
}

func (p RecoveryPolicy) emptyRetryIntervals() []time.Duration {
	if len(p.EmptyRetryIntervals) == 0 {
		return defaultRecoveryPolicy().EmptyRetryIntervals
	}
	return p.EmptyRetryIntervals
}

func (p RecoveryPolicy) emptyRetryBaseDelay() time.Duration {
	intervals := p.emptyRetryIntervals()
	if len(intervals) == 0 {
		return 5 * time.Second
	}
	return intervals[0]
}
