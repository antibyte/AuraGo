package outputcompress

import (
	"sort"
	"sync"
)

// CompressionAggregate holds cumulative compression statistics for the
// current server session. It is safe for concurrent use.
type CompressionAggregate struct {
	mu sync.RWMutex

	totalRawChars        int64
	totalCompressedChars int64
	compressionsApplied  int64
	compressionsSkipped  int64 // outputs that were too short, errors, or disabled

	// Per-tool counters
	toolRawChars        map[string]int64
	toolCompressedChars map[string]int64
	toolCount           map[string]int64

	// Per-filter counters
	filterCount map[string]int64
	filterSaved map[string]int64 // raw - compressed

	// Processing time tracking
	totalProcessingTimeMs int64

	// Error tracking
	errorsCount int64

	// Recent compression events (ring buffer, last 100)
	recent    []CompressionStats
	recentMax int
}

// CompressionSnapshot is a read-only JSON-friendly snapshot of the aggregate.
type CompressionSnapshot struct {
	Enabled              bool                     `json:"enabled"`
	TotalRawChars        int64                    `json:"total_raw_chars"`
	TotalCompressedChars int64                    `json:"total_compressed_chars"`
	TotalSavedChars      int64                    `json:"total_saved_chars"`
	AverageSavingsRatio  float64                  `json:"average_savings_ratio"`
	CompressionsApplied  int64                    `json:"compressions_applied"`
	CompressionsSkipped  int64                    `json:"compressions_skipped"`
	AverageProcessingMs  float64                  `json:"average_processing_ms"`
	ErrorsCount          int64                    `json:"errors_count"`
	TopTools             []ToolCompressionEntry   `json:"top_tools"`
	TopFilters           []FilterCompressionEntry `json:"top_filters"`
	RecentCompressions   []CompressionStats       `json:"recent_compressions,omitempty"`
}

// ToolCompressionEntry shows per-tool compression stats.
type ToolCompressionEntry struct {
	Tool         string  `json:"tool"`
	Count        int64   `json:"count"`
	RawChars     int64   `json:"raw_chars"`
	SavedChars   int64   `json:"saved_chars"`
	SavingsRatio float64 `json:"savings_ratio"`
}

// FilterCompressionEntry shows per-filter compression stats.
type FilterCompressionEntry struct {
	Filter     string `json:"filter"`
	Count      int64  `json:"count"`
	SavedChars int64  `json:"saved_chars"`
}

// globalAggregator is the singleton compression stats collector.
var globalAggregator = &CompressionAggregate{
	toolRawChars:        make(map[string]int64),
	toolCompressedChars: make(map[string]int64),
	toolCount:           make(map[string]int64),
	filterCount:         make(map[string]int64),
	filterSaved:         make(map[string]int64),
	recent:              make([]CompressionStats, 0, 100),
	recentMax:           100,
}

// RecordCompressionStats adds a compression result to the global aggregator.
// This should be called after every Compress() call that produced a ratio < 1.0.
func RecordCompressionStats(stats CompressionStats) {
	globalAggregator.record(stats)
}

// RecordCompressionSkipped increments the skipped counter for outputs that
// were not compressed (too short, errors, disabled, etc.).
func RecordCompressionSkipped() {
	globalAggregator.mu.Lock()
	globalAggregator.compressionsSkipped++
	globalAggregator.mu.Unlock()
}

// GetCompressionSnapshot returns a read-only snapshot of the global aggregate.
func GetCompressionSnapshot() CompressionSnapshot {
	return globalAggregator.snapshot()
}

// ResetCompressionStats clears all compression statistics (useful for testing).
func ResetCompressionStats() {
	globalAggregator.reset()
}

func (a *CompressionAggregate) record(stats CompressionStats) {
	saved := int64(stats.RawChars - stats.CompressedChars)
	if saved <= 0 {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.totalRawChars += int64(stats.RawChars)
	a.totalCompressedChars += int64(stats.CompressedChars)
	a.compressionsApplied++
	a.totalProcessingTimeMs += stats.ProcessingTimeMs

	// Track errors
	if stats.ErrorOccurred {
		a.errorsCount++
	}

	a.toolRawChars[stats.ToolName] += int64(stats.RawChars)
	a.toolCompressedChars[stats.ToolName] += int64(stats.CompressedChars)
	a.toolCount[stats.ToolName]++

	a.filterCount[stats.FilterUsed]++
	a.filterSaved[stats.FilterUsed] += saved

	// Ring buffer for recent events
	if len(a.recent) >= a.recentMax {
		a.recent = a.recent[1:]
	}
	a.recent = append(a.recent, stats)
}

func (a *CompressionAggregate) snapshot() CompressionSnapshot {
	a.mu.RLock()
	defer a.mu.RUnlock()

	saved := a.totalRawChars - a.totalCompressedChars
	ratio := 0.0
	if a.totalRawChars > 0 {
		ratio = float64(saved) / float64(a.totalRawChars)
	}

	avgProcessingMs := 0.0
	if a.compressionsApplied > 0 {
		avgProcessingMs = float64(a.totalProcessingTimeMs) / float64(a.compressionsApplied)
	}

	snap := CompressionSnapshot{
		Enabled:              true,
		TotalRawChars:        a.totalRawChars,
		TotalCompressedChars: a.totalCompressedChars,
		TotalSavedChars:      saved,
		AverageSavingsRatio:  ratio,
		CompressionsApplied:  a.compressionsApplied,
		CompressionsSkipped:  a.compressionsSkipped,
		AverageProcessingMs:  avgProcessingMs,
		ErrorsCount:          a.errorsCount,
		TopTools:             a.buildTopTools(10),
		TopFilters:           a.buildTopFilters(10),
		RecentCompressions:   a.copyRecent(20),
	}

	return snap
}

func (a *CompressionAggregate) buildTopTools(limit int) []ToolCompressionEntry {
	entries := make([]ToolCompressionEntry, 0, len(a.toolCount))
	for tool, count := range a.toolCount {
		raw := a.toolRawChars[tool]
		compressed := a.toolCompressedChars[tool]
		saved := raw - compressed
		ratio := 0.0
		if raw > 0 {
			ratio = float64(saved) / float64(raw)
		}
		entries = append(entries, ToolCompressionEntry{
			Tool:         tool,
			Count:        count,
			RawChars:     raw,
			SavedChars:   saved,
			SavingsRatio: ratio,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].SavedChars > entries[j].SavedChars
	})
	if len(entries) > limit {
		entries = entries[:limit]
	}
	return entries
}

func (a *CompressionAggregate) buildTopFilters(limit int) []FilterCompressionEntry {
	entries := make([]FilterCompressionEntry, 0, len(a.filterCount))
	for filter, count := range a.filterCount {
		entries = append(entries, FilterCompressionEntry{
			Filter:     filter,
			Count:      count,
			SavedChars: a.filterSaved[filter],
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].SavedChars > entries[j].SavedChars
	})
	if len(entries) > limit {
		entries = entries[:limit]
	}
	return entries
}

func (a *CompressionAggregate) copyRecent(limit int) []CompressionStats {
	if len(a.recent) == 0 {
		return nil
	}
	start := 0
	if len(a.recent) > limit {
		start = len(a.recent) - limit
	}
	cp := make([]CompressionStats, len(a.recent)-start)
	copy(cp, a.recent[start:])
	return cp
}

func (a *CompressionAggregate) reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.totalRawChars = 0
	a.totalCompressedChars = 0
	a.compressionsApplied = 0
	a.compressionsSkipped = 0
	a.toolRawChars = make(map[string]int64)
	a.toolCompressedChars = make(map[string]int64)
	a.toolCount = make(map[string]int64)
	a.filterCount = make(map[string]int64)
	a.filterSaved = make(map[string]int64)
	a.recent = make([]CompressionStats, 0, a.recentMax)
}
