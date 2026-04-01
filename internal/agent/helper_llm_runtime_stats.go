package agent

import (
	"sync"
	"time"
)

type HelperLLMRuntimeSnapshot struct {
	UpdatedAt  time.Time                          `json:"updated_at,omitempty"`
	Operations map[string]HelperLLMOperationStats `json:"operations"`
}

var (
	helperLLMRuntimeStatsMu sync.RWMutex
	helperLLMRuntimeStats   = make(map[string]HelperLLMOperationStats)
	helperLLMRuntimeUpdated time.Time
)

func recordHelperLLMRuntimeDelta(operation string, before, after HelperLLMOperationStats) {
	if operation == "" {
		return
	}

	requestsDelta := after.Requests - before.Requests
	cacheHitsDelta := after.CacheHits - before.CacheHits
	llmCallsDelta := after.LLMCalls - before.LLMCalls
	fallbacksDelta := after.Fallbacks - before.Fallbacks
	batchedItemsDelta := after.BatchedItems - before.BatchedItems
	savedCallsDelta := after.SavedCalls - before.SavedCalls
	lastDetailChanged := after.LastDetail != "" && after.LastDetail != before.LastDetail
	if requestsDelta == 0 && cacheHitsDelta == 0 && llmCallsDelta == 0 && fallbacksDelta == 0 && batchedItemsDelta == 0 && savedCallsDelta == 0 && !lastDetailChanged {
		return
	}

	delta := HelperLLMOperationStats{}
	if requestsDelta > 0 {
		delta.Requests = requestsDelta
	}
	if cacheHitsDelta > 0 {
		delta.CacheHits = cacheHitsDelta
	}
	if llmCallsDelta > 0 {
		delta.LLMCalls = llmCallsDelta
	}
	if fallbacksDelta > 0 {
		delta.Fallbacks = fallbacksDelta
	}
	if batchedItemsDelta > 0 {
		delta.BatchedItems = batchedItemsDelta
	}
	if savedCallsDelta > 0 {
		delta.SavedCalls = savedCallsDelta
	}
	if lastDetailChanged {
		delta.LastDetail = after.LastDetail
	}
	MergeHelperLLMRuntimeStats(operation, delta)
}

func MergeHelperLLMRuntimeStats(operation string, delta HelperLLMOperationStats) {
	if operation == "" {
		return
	}

	helperLLMRuntimeStatsMu.Lock()
	defer helperLLMRuntimeStatsMu.Unlock()

	current := helperLLMRuntimeStats[operation]
	if delta.Requests > 0 {
		current.Requests += delta.Requests
	}
	if delta.CacheHits > 0 {
		current.CacheHits += delta.CacheHits
	}
	if delta.LLMCalls > 0 {
		current.LLMCalls += delta.LLMCalls
	}
	if delta.Fallbacks > 0 {
		current.Fallbacks += delta.Fallbacks
	}
	if delta.BatchedItems > 0 {
		current.BatchedItems += delta.BatchedItems
	}
	if delta.SavedCalls > 0 {
		current.SavedCalls += delta.SavedCalls
	}
	if delta.LastDetail != "" {
		current.LastDetail = delta.LastDetail
	}
	helperLLMRuntimeStats[operation] = current
	helperLLMRuntimeUpdated = time.Now().UTC()
}

func SnapshotHelperLLMRuntimeStats() HelperLLMRuntimeSnapshot {
	helperLLMRuntimeStatsMu.RLock()
	defer helperLLMRuntimeStatsMu.RUnlock()

	operations := make(map[string]HelperLLMOperationStats, len(helperLLMRuntimeStats))
	for key, value := range helperLLMRuntimeStats {
		operations[key] = value
	}

	return HelperLLMRuntimeSnapshot{
		UpdatedAt:  helperLLMRuntimeUpdated,
		Operations: operations,
	}
}

func ResetHelperLLMRuntimeStats() {
	helperLLMRuntimeStatsMu.Lock()
	defer helperLLMRuntimeStatsMu.Unlock()
	helperLLMRuntimeStats = make(map[string]HelperLLMOperationStats)
	helperLLMRuntimeUpdated = time.Time{}
}
