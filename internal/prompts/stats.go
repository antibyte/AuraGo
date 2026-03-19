package prompts

import (
	"math"
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
	// Per-section character lengths (before optimization)
	SectionSizes map[string]int `json:"section_sizes,omitempty"`
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
	// AvgSectionSizes maps section name → average character count across all builds
	AvgSectionSizes map[string]int `json:"avg_section_sizes"`
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
	sectionSums := make(map[string]int64)
	sectionCounts := make(map[string]int)

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

		for sec, sz := range r.SectionSizes {
			sectionSums[sec] += int64(sz)
			sectionCounts[sec]++
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

	agg.AvgSectionSizes = make(map[string]int)
	for sec, sum := range sectionSums {
		if cnt := sectionCounts[sec]; cnt > 0 {
			agg.AvgSectionSizes[sec] = int(sum / int64(cnt))
		}
	}

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

// ─── Adaptive Tool Filtering (decay-weighted) ─────────────────────────────

// ToolUsageEntry holds persistent usage data for one tool (loaded from SQLite).
type ToolUsageEntry struct {
	ToolName     string
	TotalCount   int
	SuccessCount int
	LastUsed     time.Time
}

// ToolDecayScore holds a tool name and its computed decay-weighted score.
type ToolDecayScore struct {
	Tool  string  `json:"tool"`
	Score float64 `json:"score"`
	Count int     `json:"count"`
}

// adaptiveToolState holds the in-memory cache of persistent tool usage data.
var adaptiveToolState struct {
	mu      sync.RWMutex
	entries map[string]*ToolUsageEntry // tool_name → entry
}

func init() {
	adaptiveToolState.entries = make(map[string]*ToolUsageEntry)
}

// LoadAdaptiveToolState populates the in-memory cache from SQLite data.
// Call this once at startup.
func LoadAdaptiveToolState(entries []ToolUsageEntry) {
	adaptiveToolState.mu.Lock()
	defer adaptiveToolState.mu.Unlock()
	adaptiveToolState.entries = make(map[string]*ToolUsageEntry, len(entries))
	for i := range entries {
		e := entries[i]
		adaptiveToolState.entries[e.ToolName] = &e
	}
}

// RecordAdaptiveToolUsage updates the in-memory adaptive cache for a tool.
// success indicates whether the tool call completed without error.
func RecordAdaptiveToolUsage(toolName string, success bool) {
	adaptiveToolState.mu.Lock()
	defer adaptiveToolState.mu.Unlock()
	e, ok := adaptiveToolState.entries[toolName]
	if !ok {
		e = &ToolUsageEntry{ToolName: toolName}
		adaptiveToolState.entries[toolName] = e
	}
	e.TotalCount++
	if success {
		e.SuccessCount++
	}
	e.LastUsed = time.Now()
}

// decayScore computes: count × 0.5^(daysSinceLastUse / halfLifeDays) × successFactor.
// successFactor is 1.0 when weightSuccess is false; otherwise (0.3 + 0.7 * successRate)
// so that tools with failures are deprioritised without disappearing completely.
func decayScore(count, successCount int, lastUsed time.Time, halfLifeDays float64, weightSuccess bool) float64 {
	days := time.Since(lastUsed).Hours() / 24.0
	if days < 0 {
		days = 0
	}
	base := float64(count) * math.Pow(0.5, days/halfLifeDays)
	if !weightSuccess || count <= 0 {
		return base
	}
	successRate := float64(successCount) / float64(count)
	return base * (0.3 + 0.7*successRate)
}

// GetFrequentToolsWeighted returns the top-N tools ranked by decay-weighted score.
// halfLifeDays controls how fast old usage fades (7 = score halves every 7 days).
// weightSuccess penalises tools with a low success rate.
func GetFrequentToolsWeighted(topN int, halfLifeDays float64, weightSuccess bool) []string {
	adaptiveToolState.mu.RLock()
	defer adaptiveToolState.mu.RUnlock()

	if halfLifeDays <= 0 {
		halfLifeDays = 7.0
	}

	type scored struct {
		tool  string
		score float64
	}
	ranked := make([]scored, 0, len(adaptiveToolState.entries))
	for _, e := range adaptiveToolState.entries {
		s := decayScore(e.TotalCount, e.SuccessCount, e.LastUsed, halfLifeDays, weightSuccess)
		if s >= 0.5 { // minimum threshold to be considered
			ranked = append(ranked, scored{e.ToolName, s})
		}
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})

	result := make([]string, 0, topN)
	for i := 0; i < topN && i < len(ranked); i++ {
		result = append(result, ranked[i].tool)
	}
	return result
}

// GetAdaptiveToolScores returns all tool scores for dashboard display.
// weightSuccess penalises tools with a low success rate.
func GetAdaptiveToolScores(halfLifeDays float64, weightSuccess bool) []ToolDecayScore {
	adaptiveToolState.mu.RLock()
	defer adaptiveToolState.mu.RUnlock()

	if halfLifeDays <= 0 {
		halfLifeDays = 7.0
	}

	scores := make([]ToolDecayScore, 0, len(adaptiveToolState.entries))
	for _, e := range adaptiveToolState.entries {
		scores = append(scores, ToolDecayScore{
			Tool:  e.ToolName,
			Score: decayScore(e.TotalCount, e.SuccessCount, e.LastUsed, halfLifeDays, weightSuccess),
			Count: e.TotalCount,
		})
	}
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})
	return scores
}
