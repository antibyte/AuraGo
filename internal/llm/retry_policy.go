package llm

import (
	"aurago/internal/config"
	"context"
	"sync"
	"time"
)

// RetryPolicy is copied once per operation. Managers never change shared defaults.
type RetryPolicy struct {
	Intervals         []time.Duration
	FinalInterval     time.Duration
	PerAttemptTimeout time.Duration
	TotalTimeout      time.Duration
}

func providerAttemptTimeout(cfg *config.Config) time.Duration {
	if cfg == nil {
		return retryPolicyFor(nil).PerAttemptTimeout
	}
	return RetryPolicyFromConfig(cfg).PerAttemptTimeout
}

func (p RetryPolicy) clone() RetryPolicy {
	p.Intervals = append([]time.Duration(nil), p.Intervals...)
	return p
}

func RetryPolicyFromConfig(cfg *config.Config) RetryPolicy {
	p := RetryPolicy{Intervals: []time.Duration{30 * time.Second, 2 * time.Minute}, FinalInterval: 30 * time.Second, PerAttemptTimeout: 120 * time.Second, TotalTimeout: 600 * time.Second}
	if cfg == nil {
		return p
	}
	var intervals []time.Duration
	for _, raw := range cfg.CircuitBreaker.RetryIntervals {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			intervals = append(intervals, d)
		}
	}
	if len(intervals) > 0 {
		p.Intervals = intervals
	}
	if d, err := time.ParseDuration(cfg.CircuitBreaker.FinalRetryInterval); err == nil && d > 0 {
		p.FinalInterval = d
	}
	if d := time.Duration(cfg.CircuitBreaker.LLMPerAttemptTimeoutSeconds) * time.Second; d > p.PerAttemptTimeout {
		p.PerAttemptTimeout = d
	}
	if d := time.Duration(cfg.CircuitBreaker.LLMTimeoutSeconds) * time.Second; d > 0 {
		p.TotalTimeout = d
	}
	return p
}

func retryPolicyFor(client ChatClient) RetryPolicy {
	if owner, ok := client.(interface{ RetryPolicySnapshot() RetryPolicy }); ok {
		return owner.RetryPolicySnapshot().clone()
	}
	defaultPolicyMu.RLock()
	defer defaultPolicyMu.RUnlock()
	return RetryPolicy{Intervals: defaultRetryIntervalsCopy(), FinalInterval: FinalRetryInterval(), PerAttemptTimeout: perAttemptTimeout(), TotalTimeout: 600 * time.Second}
}

func (fm *FailoverManager) RetryPolicySnapshot() RetryPolicy {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	if fm.retryPolicy.PerAttemptTimeout <= 0 {
		return RetryPolicyFromConfig(nil)
	}
	return fm.retryPolicy.clone()
}

// ConfigureRetryPolicy changes future retry operations without discarding route
// health or the frozen policy already held by an active task.
func (fm *FailoverManager) ConfigureRetryPolicy(cfg *config.Config) {
	policy := RetryPolicyFromConfig(cfg)
	fm.mu.Lock()
	fm.retryPolicy = policy
	fm.mu.Unlock()
}
func (c *TaskRouteClient) RetryPolicySnapshot() RetryPolicy {
	if c.retryPolicy.PerAttemptTimeout <= 0 {
		return RetryPolicyFromConfig(nil)
	}
	return c.retryPolicy.clone()
}

// openingContext has a stoppable deadline: accepted streams retain caller
// cancellation, but no opening timer can cancel their later response reads.
func openingContext(parent context.Context, limit time.Duration, stage string) (context.Context, context.CancelFunc, func()) {
	ctx, cancel := context.WithCancelCause(parent)
	var mu sync.Mutex
	active := true
	timer := time.AfterFunc(limit, func() {
		mu.Lock()
		defer mu.Unlock()
		if active {
			cancel(&TransportTimeoutError{Stage: stage, After: limit})
		}
	})
	stop := func() { mu.Lock(); active = false; timer.Stop(); mu.Unlock() }
	return ctx, func() { stop(); cancel(context.Canceled) }, stop
}
