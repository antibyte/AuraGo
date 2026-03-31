package security

import (
	"sync/atomic"
	"time"
)

// GuardianMetrics tracks LLM Guardian usage statistics.
// All counters are atomic for concurrent access without locks.
type GuardianMetrics struct {
	TotalChecks    atomic.Int64
	CachedChecks   atomic.Int64
	Blocks         atomic.Int64
	Quarantines    atomic.Int64
	Allows         atomic.Int64
	Errors         atomic.Int64
	Clarifications atomic.Int64
	ContentScans   atomic.Int64
	TotalTokens    atomic.Int64
	TotalLatencyMs atomic.Int64
	MaxLatencyMs   atomic.Int64
	LastCheckTime  atomic.Int64 // Unix millis of last check
}

// Record updates metrics based on a guardian result.
func (m *GuardianMetrics) Record(result GuardianResult) {
	m.TotalChecks.Add(1)
	m.TotalTokens.Add(int64(result.TokensUsed))
	m.TotalLatencyMs.Add(result.Duration.Milliseconds())
	m.LastCheckTime.Store(time.Now().UnixMilli())
	for {
		current := m.MaxLatencyMs.Load()
		next := result.Duration.Milliseconds()
		if next <= current || m.MaxLatencyMs.CompareAndSwap(current, next) {
			break
		}
	}

	if result.Cached {
		m.CachedChecks.Add(1)
	}
	switch result.Decision {
	case DecisionBlock:
		m.Blocks.Add(1)
	case DecisionQuarantine:
		m.Quarantines.Add(1)
	case DecisionAllow:
		m.Allows.Add(1)
	}
}

// RecordError increments the error counter.
func (m *GuardianMetrics) RecordError() {
	m.Errors.Add(1)
}

// RecordClarification records a clarification attempt and its result.
func (m *GuardianMetrics) RecordClarification(result GuardianResult) {
	m.Clarifications.Add(1)
	m.Record(result)
}

// RecordContentScan records a content scan and its result.
func (m *GuardianMetrics) RecordContentScan(result GuardianResult) {
	m.ContentScans.Add(1)
	m.Record(result)
}

// Reset zeroes all metric counters. Useful for testing and periodic metric resets.
func (m *GuardianMetrics) Reset() {
	m.TotalChecks.Store(0)
	m.CachedChecks.Store(0)
	m.Blocks.Store(0)
	m.Quarantines.Store(0)
	m.Allows.Store(0)
	m.Errors.Store(0)
	m.Clarifications.Store(0)
	m.ContentScans.Store(0)
	m.TotalTokens.Store(0)
	m.TotalLatencyMs.Store(0)
	m.MaxLatencyMs.Store(0)
	m.LastCheckTime.Store(0)
}

// Snapshot returns a copy of current metrics as a plain struct (for JSON serialization).
type MetricsSnapshot struct {
	TotalChecks    int64   `json:"total_checks"`
	CachedChecks   int64   `json:"cached_checks"`
	Blocks         int64   `json:"blocks"`
	Quarantines    int64   `json:"quarantines"`
	Allows         int64   `json:"allows"`
	Errors         int64   `json:"errors"`
	Clarifications int64   `json:"clarifications"`
	ContentScans   int64   `json:"content_scans"`
	TotalTokens    int64   `json:"total_tokens"`
	AverageLatency float64 `json:"average_latency_ms"`
	MaxLatencyMs   int64   `json:"max_latency_ms"`
	CacheHitRate   float64 `json:"cache_hit_rate"`
	LastCheckTime  int64   `json:"last_check_time"`
}

// Snapshot returns an atomic snapshot of all metrics.
func (m *GuardianMetrics) Snapshot() MetricsSnapshot {
	total := m.TotalChecks.Load()
	cached := m.CachedChecks.Load()
	var hitRate float64
	if total > 0 {
		hitRate = float64(cached) / float64(total)
	}
	var avgLatency float64
	if total > 0 {
		avgLatency = float64(m.TotalLatencyMs.Load()) / float64(total)
	}
	return MetricsSnapshot{
		TotalChecks:    total,
		CachedChecks:   cached,
		Blocks:         m.Blocks.Load(),
		Quarantines:    m.Quarantines.Load(),
		Allows:         m.Allows.Load(),
		Errors:         m.Errors.Load(),
		Clarifications: m.Clarifications.Load(),
		ContentScans:   m.ContentScans.Load(),
		TotalTokens:    m.TotalTokens.Load(),
		AverageLatency: avgLatency,
		MaxLatencyMs:   m.MaxLatencyMs.Load(),
		CacheHitRate:   hitRate,
		LastCheckTime:  m.LastCheckTime.Load(),
	}
}
