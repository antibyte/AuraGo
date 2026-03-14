package prompts

import (
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
