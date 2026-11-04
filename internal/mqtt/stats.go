package mqtt

import (
	"sync"
	"sync/atomic"
	"time"
)

type runtimeStats struct {
	receivedMessages         uint64
	publishedMessages        uint64
	publishErrors            uint64
	subscribeErrors          uint64
	reconnects               uint64
	droppedRelayMessages     uint64
	droppedMissionJobs       uint64
	droppedPayloadMessages   uint64
	truncatedPayloadMessages uint64
	binaryPayloadMessages    uint64
	oversizedMissionPayloads uint64
	rateLimitedJobs          uint64
	debouncedRelayMessages   uint64
	relayQueueOverflows      uint64
	missionQueueOverflows    uint64
	expiredRelayMessages     uint64
	expiredMissionJobs       uint64
	quarantinedMessages      uint64

	mu             sync.RWMutex
	connectedOnce  bool
	connectedAt    string
	disconnectedAt string
	lastError      string
	lastErrorAt    string
}

var stats runtimeStats

func recordConnected() {
	stats.mu.Lock()
	defer stats.mu.Unlock()
	if stats.connectedOnce {
		atomic.AddUint64(&stats.reconnects, 1)
	} else {
		stats.connectedOnce = true
	}
	stats.connectedAt = time.Now().UTC().Format(time.RFC3339)
	stats.disconnectedAt = ""
	stats.lastError = ""
	stats.lastErrorAt = ""
}

func recordDisconnected(err error) {
	stats.mu.Lock()
	defer stats.mu.Unlock()
	stats.disconnectedAt = time.Now().UTC().Format(time.RFC3339)
	if err != nil {
		stats.lastError = err.Error()
		stats.lastErrorAt = stats.disconnectedAt
	}
}

func recordError(err error) {
	if err == nil {
		return
	}
	stats.mu.Lock()
	defer stats.mu.Unlock()
	stats.lastError = err.Error()
	stats.lastErrorAt = time.Now().UTC().Format(time.RFC3339)
}

// RecordDroppedRelayMessage tracks relay drops that happen outside the MQTT queue.
func RecordDroppedRelayMessage() {
	atomic.AddUint64(&stats.droppedRelayMessages, 1)
}

// RecordQuarantinedMessage records withheld content without starting a model run.
func RecordQuarantinedMessage() {
	atomic.AddUint64(&stats.quarantinedMessages, 1)
}

func RuntimeStats() map[string]interface{} {
	stats.mu.RLock()
	connectedAt := stats.connectedAt
	disconnectedAt := stats.disconnectedAt
	lastError := stats.lastError
	lastErrorAt := stats.lastErrorAt
	stats.mu.RUnlock()

	return map[string]interface{}{
		"received_messages":          atomic.LoadUint64(&stats.receivedMessages),
		"published_messages":         atomic.LoadUint64(&stats.publishedMessages),
		"publish_errors":             atomic.LoadUint64(&stats.publishErrors),
		"subscribe_errors":           atomic.LoadUint64(&stats.subscribeErrors),
		"reconnects":                 atomic.LoadUint64(&stats.reconnects),
		"dropped_relay_messages":     atomic.LoadUint64(&stats.droppedRelayMessages),
		"dropped_mission_jobs":       atomic.LoadUint64(&stats.droppedMissionJobs),
		"dropped_payload_messages":   atomic.LoadUint64(&stats.droppedPayloadMessages),
		"truncated_payload_messages": atomic.LoadUint64(&stats.truncatedPayloadMessages),
		"binary_payload_messages":    atomic.LoadUint64(&stats.binaryPayloadMessages),
		"oversized_mission_payloads": atomic.LoadUint64(&stats.oversizedMissionPayloads),
		"rate_limited_jobs":          atomic.LoadUint64(&stats.rateLimitedJobs),
		"debounced_relay_messages":   atomic.LoadUint64(&stats.debouncedRelayMessages),
		"relay_queue_overflows":      atomic.LoadUint64(&stats.relayQueueOverflows),
		"mission_queue_overflows":    atomic.LoadUint64(&stats.missionQueueOverflows),
		"expired_relay_messages":     atomic.LoadUint64(&stats.expiredRelayMessages),
		"expired_mission_jobs":       atomic.LoadUint64(&stats.expiredMissionJobs),
		"quarantined_messages":       atomic.LoadUint64(&stats.quarantinedMessages),
		"connected_at":               connectedAt,
		"disconnected_at":            disconnectedAt,
		"last_error":                 lastError,
		"last_error_at":              lastErrorAt,
	}
}
