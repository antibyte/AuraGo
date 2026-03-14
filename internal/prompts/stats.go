package prompts

import (
	"sort"
	"sync"
	"time"
)

// PromptBuildRecord holds stats from a single BuildSystemPrompt invocation.
type PromptBuildRecord struct {
	Timestamp     time.Time `json:"timestamp"`
	Tier          string    `json:"tier"`
	RawLen        int       `json:"raw_len"`
	OptimizedLen  int       `json:"optimized_len"`
	SavedChars    int       `json:"saved_chars"`
	Tokens        int       `json:"tokens"`
	TokenBudget   int       `json:"token_budget"`
	ModulesLoaded int       `json:"modules_loaded"`
	ModulesUsed   int       `json:"modules_used"`
	GuidesCount   int       `json:"guides_count"`
	ShedSections  []string  `json:"shed_sections,omitempty"`
	BudgetShed    bool      `json:"budget_shed"`
	MessageCount  int       `json:"message_count"`
}

// PromptStatsAggregated is the JSON-friendly aggregate returned by the dashboard API.
type PromptStatsAggregated struct {
	TotalBuilds        int                 `json:"total_builds"`
	AvgRawLen          int                 `json:"avg_raw_len"`
	AvgOptimizedLen    int                 `json:"avg_optimized_len"`
	AvgSavedChars      int                 `json:"avg_saved_chars"`
	AvgTokens          int                 `json:"avg_tokens"`
	AvgOptimizationPct float64             `json:"avg_optimization_pct"`
	TotalSavedChars    int64               `json:"total_saved_chars"`
	BudgetShedCount    int                 `json:"budget_shed_count"`
	TierDistribution   map[string]int      `json:"tier_distribution"`
	ShedSectionCounts  map[string]int      `json:"shed_section_counts"`
	AvgModulesLoaded   int                 `json:"avg_modules_loaded"`
	AvgModulesUsed     int                 `json:"avg_modules_used"`
	AvgGuidesCount     int                 `json:"avg_guides_count"`
	Recent             []PromptBuildRecord `json:"recent"`
}

// promptStatsCollector is a package-level ring buffer for prompt build metrics.
type promptStatsCollector struct {
	mu      sync.RWMutex
	records []PromptBuildRecord
	maxSize int
}

var globalStats = &promptStatsCollector{
	maxSize: 200, // Keep last 200 builds
}

// RecordBuild appends a build record to the ring buffer.
func RecordBuild(rec PromptBuildRecord) {
	globalStats.mu.Lock()
	defer globalStats.mu.Unlock()

	if len(globalStats.records) >= globalStats.maxSize {
		// Shift: drop oldest 20% and reallocate to release memory
		drop := globalStats.maxSize / 5
		remaining := len(globalStats.records) - drop
		newRecords := make([]PromptBuildRecord, remaining, globalStats.maxSize)
		copy(newRecords, globalStats.records[drop:])
		globalStats.records = newRecords
	}
	globalStats.records = append(globalStats.records, rec)
}

// GetAggregatedStats computes aggregate prompt build statistics.
func GetAggregatedStats() PromptStatsAggregated {
	globalStats.mu.RLock()
	defer globalStats.mu.RUnlock()

	n := len(globalStats.records)
	agg := PromptStatsAggregated{
		TotalBuilds:       n,
		TierDistribution:  make(map[string]int),
		ShedSectionCounts: make(map[string]int),
	}
	if n == 0 {
		agg.Recent = []PromptBuildRecord{}
		return agg
	}

	var sumRaw, sumOpt, sumSaved, sumTokens int64
	var sumModLoaded, sumModUsed, sumGuides int64
	var sumOptPct float64

	for _, r := range globalStats.records {
		sumRaw += int64(r.RawLen)
		sumOpt += int64(r.OptimizedLen)
		sumSaved += int64(r.SavedChars)
		sumTokens += int64(r.Tokens)
		sumModLoaded += int64(r.ModulesLoaded)
		sumModUsed += int64(r.ModulesUsed)
		sumGuides += int64(r.GuidesCount)
		agg.TotalSavedChars += int64(r.SavedChars)

		if r.RawLen > 0 {
			sumOptPct += float64(r.SavedChars) / float64(r.RawLen) * 100.0
		}

		if r.BudgetShed {
			agg.BudgetShedCount++
		}

		agg.TierDistribution[r.Tier]++

		for _, s := range r.ShedSections {
			agg.ShedSectionCounts[s]++
		}
	}

	agg.AvgRawLen = int(sumRaw / int64(n))
	agg.AvgOptimizedLen = int(sumOpt / int64(n))
	agg.AvgSavedChars = int(sumSaved / int64(n))
	agg.AvgTokens = int(sumTokens / int64(n))
	agg.AvgOptimizationPct = sumOptPct / float64(n)
	agg.AvgModulesLoaded = int(sumModLoaded / int64(n))
	agg.AvgModulesUsed = int(sumModUsed / int64(n))
	agg.AvgGuidesCount = int(sumGuides / int64(n))

	// Return last 20 records as recent history
	recentCount := 20
	if n < recentCount {
		recentCount = n
	}
	agg.Recent = make([]PromptBuildRecord, recentCount)
	copy(agg.Recent, globalStats.records[n-recentCount:])

	return agg
}

// ─── Tool Usage Tracking ───────────────────────────────────────────────────

// ToolUsageRecord captures a single tool invocation.
type ToolUsageRecord struct {
	Tool      string    `json:"tool"`
	Operation string    `json:"operation,omitempty"`
	Success   bool      `json:"success"`
	Timestamp time.Time `json:"timestamp"`
}

// ToolUsageStats aggregates per-tool call counts.
type ToolUsageStats struct {
	TotalCalls   int `json:"total_calls"`
	SuccessCount int `json:"success_count"`
	FailureCount int `json:"failure_count"`
}

// ToolUsageAggregated is the JSON-friendly aggregate returned by the dashboard API.
type ToolUsageAggregated struct {
	TotalCalls int                       `json:"total_calls"`
	ByTool     map[string]ToolUsageStats `json:"by_tool"`
	TopTools   []ToolRanking             `json:"top_tools"`
	Recent     []ToolUsageRecord         `json:"recent"`
}

// ToolRanking is a (tool, count) pair for sorted output.
type ToolRanking struct {
	Tool  string `json:"tool"`
	Count int    `json:"count"`
}

type toolUsageCollector struct {
	mu      sync.RWMutex
	records []ToolUsageRecord
	maxSize int
}

var globalToolStats = &toolUsageCollector{
	maxSize: 500,
}

// RecordToolUsage appends a tool call record to the ring buffer.
func RecordToolUsage(tool, operation string, success bool) {
	globalToolStats.mu.Lock()
	defer globalToolStats.mu.Unlock()

	if len(globalToolStats.records) >= globalToolStats.maxSize {
		drop := globalToolStats.maxSize / 5
		remaining := len(globalToolStats.records) - drop
		newRecords := make([]ToolUsageRecord, remaining, globalToolStats.maxSize)
		copy(newRecords, globalToolStats.records[drop:])
		globalToolStats.records = newRecords
	}
	globalToolStats.records = append(globalToolStats.records, ToolUsageRecord{
		Tool:      tool,
		Operation: operation,
		Success:   success,
		Timestamp: time.Now(),
	})
}

// GetToolUsageStats computes aggregated tool usage statistics.
func GetToolUsageStats() ToolUsageAggregated {
	globalToolStats.mu.RLock()
	defer globalToolStats.mu.RUnlock()

	n := len(globalToolStats.records)
	agg := ToolUsageAggregated{
		TotalCalls: n,
		ByTool:     make(map[string]ToolUsageStats),
	}
	if n == 0 {
		agg.TopTools = []ToolRanking{}
		agg.Recent = []ToolUsageRecord{}
		return agg
	}

	for _, r := range globalToolStats.records {
		s := agg.ByTool[r.Tool]
		s.TotalCalls++
		if r.Success {
			s.SuccessCount++
		} else {
			s.FailureCount++
		}
		agg.ByTool[r.Tool] = s
	}

	// Build sorted top-tools list
	rankings := make([]ToolRanking, 0, len(agg.ByTool))
	for tool, stats := range agg.ByTool {
		rankings = append(rankings, ToolRanking{Tool: tool, Count: stats.TotalCalls})
	}
	sort.Slice(rankings, func(i, j int) bool {
		return rankings[i].Count > rankings[j].Count
	})
	if len(rankings) > 10 {
		rankings = rankings[:10]
	}
	agg.TopTools = rankings

	// Recent 20
	recentCount := 20
	if n < recentCount {
		recentCount = n
	}
	agg.Recent = make([]ToolUsageRecord, recentCount)
	copy(agg.Recent, globalToolStats.records[n-recentCount:])

	return agg
}

// GetFrequentTools returns the names of the top-N most frequently used tools.
// This is used by PrepareDynamicGuides to boost guide prediction accuracy.
func GetFrequentTools(topN int) []string {
	globalToolStats.mu.RLock()
	defer globalToolStats.mu.RUnlock()

	counts := make(map[string]int)
	for _, r := range globalToolStats.records {
		counts[r.Tool]++
	}

	type kv struct {
		Tool  string
		Count int
	}
	ranked := make([]kv, 0, len(counts))
	for t, c := range counts {
		ranked = append(ranked, kv{t, c})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Count > ranked[j].Count
	})

	result := make([]string, 0, topN)
	for i := 0; i < topN && i < len(ranked); i++ {
		result = append(result, ranked[i].Tool)
	}
	return result
}
