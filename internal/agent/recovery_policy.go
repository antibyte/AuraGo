package agent

import "aurago/internal/config"

// RecoveryPolicy centralizes retry and circuit-breaker thresholds so recovery
// behavior is defined in one place instead of being spread across helpers.
type RecoveryPolicy struct {
	MaxProvider422Recoveries int
	MinMessagesForEmptyRetry int
	DuplicateConsecutiveHits int
	DuplicateFrequencyHits   int
	IdenticalToolErrorHits   int
}

func defaultRecoveryPolicy() RecoveryPolicy {
	return RecoveryPolicy{
		MaxProvider422Recoveries: 3,
		MinMessagesForEmptyRetry: 5,
		DuplicateConsecutiveHits: 2,
		DuplicateFrequencyHits:   3,
		IdenticalToolErrorHits:   3,
	}
}

func buildRecoveryPolicy(cfg *config.Config) RecoveryPolicy {
	// Current version intentionally keeps compatibility with the historic fixed
	// thresholds. Centralizing them here makes later tuning explicit.
	_ = cfg
	return defaultRecoveryPolicy()
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
