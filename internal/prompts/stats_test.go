package prompts

import (
	"testing"
	"time"
)

func TestRecordToolUsageAndStats(t *testing.T) {
	// Reset global state
	globalToolStats.mu.Lock()
	globalToolStats.records = nil
	globalToolStats.mu.Unlock()

	// Record some tool usage
	RecordToolUsage("shell", "execute", true)
	RecordToolUsage("filesystem", "read", true)
	RecordToolUsage("shell", "execute", false)
	RecordToolUsage("docker", "list", true)
	RecordToolUsage("shell", "execute", true)

	stats := GetToolUsageStats()

	if stats.TotalCalls != 5 {
		t.Errorf("TotalCalls = %d, want 5", stats.TotalCalls)
	}

	shellStats := stats.ByTool["shell"]
	if shellStats.TotalCalls != 3 {
		t.Errorf("shell TotalCalls = %d, want 3", shellStats.TotalCalls)
	}
	if shellStats.SuccessCount != 2 {
		t.Errorf("shell SuccessCount = %d, want 2", shellStats.SuccessCount)
	}
	if shellStats.FailureCount != 1 {
		t.Errorf("shell FailureCount = %d, want 1", shellStats.FailureCount)
	}

	if len(stats.TopTools) == 0 {
		t.Fatal("TopTools should not be empty")
	}
	if stats.TopTools[0].Tool != "shell" {
		t.Errorf("Top tool should be shell, got %q", stats.TopTools[0].Tool)
	}

	if len(stats.Recent) != 5 {
		t.Errorf("Recent should have 5 entries, got %d", len(stats.Recent))
	}
}

func TestGetFrequentTools(t *testing.T) {
	// Reset global state
	globalToolStats.mu.Lock()
	globalToolStats.records = nil
	globalToolStats.mu.Unlock()

	RecordToolUsage("shell", "execute", true)
	RecordToolUsage("shell", "execute", true)
	RecordToolUsage("shell", "execute", true)
	RecordToolUsage("filesystem", "read", true)
	RecordToolUsage("filesystem", "read", true)
	RecordToolUsage("docker", "list", true)

	top := GetFrequentTools(2)
	if len(top) != 2 {
		t.Fatalf("GetFrequentTools(2) returned %d tools, want 2", len(top))
	}
	if top[0] != "shell" {
		t.Errorf("Top tool should be shell, got %q", top[0])
	}
	if top[1] != "filesystem" {
		t.Errorf("Second tool should be filesystem, got %q", top[1])
	}
}

func TestToolUsageRingBuffer(t *testing.T) {
	// Reset and use small buffer
	globalToolStats.mu.Lock()
	globalToolStats.records = nil
	origMax := globalToolStats.maxSize
	globalToolStats.maxSize = 10
	globalToolStats.mu.Unlock()

	defer func() {
		globalToolStats.mu.Lock()
		globalToolStats.maxSize = origMax
		globalToolStats.records = nil
		globalToolStats.mu.Unlock()
	}()

	// Fill beyond max
	for i := 0; i < 15; i++ {
		RecordToolUsage("tool", "op", true)
	}

	stats := GetToolUsageStats()
	if stats.TotalCalls > 10 {
		t.Errorf("Ring buffer should cap at maxSize, got %d", stats.TotalCalls)
	}
}

func TestToolUsageStatsEmpty(t *testing.T) {
	globalToolStats.mu.Lock()
	globalToolStats.records = nil
	globalToolStats.mu.Unlock()

	stats := GetToolUsageStats()
	if stats.TotalCalls != 0 {
		t.Errorf("Empty stats TotalCalls = %d, want 0", stats.TotalCalls)
	}
	if stats.TopTools == nil {
		t.Error("TopTools should be non-nil empty slice")
	}
	if stats.Recent == nil {
		t.Error("Recent should be non-nil empty slice")
	}
}

func TestDecayScore(t *testing.T) {
	now := time.Now()

	// Tool used 10 times, just now → score ≈ 10 (weightSuccess=false keeps original behaviour)
	s1 := decayScore(10, 0, now, 7, false)
	if s1 < 9.9 || s1 > 10.1 {
		t.Errorf("decayScore(10, now, 7) = %f, want ~10", s1)
	}

	// Tool used 10 times, 7 days ago → score ≈ 5 (half-life)
	s2 := decayScore(10, 0, now.Add(-7*24*time.Hour), 7, false)
	if s2 < 4.9 || s2 > 5.1 {
		t.Errorf("decayScore(10, 7d ago, 7) = %f, want ~5", s2)
	}

	// Tool used 10 times, 14 days ago → score ≈ 2.5
	s3 := decayScore(10, 0, now.Add(-14*24*time.Hour), 7, false)
	if s3 < 2.4 || s3 > 2.6 {
		t.Errorf("decayScore(10, 14d ago, 7) = %f, want ~2.5", s3)
	}

	// Tool used 5 times, 30 days ago → score very low
	s4 := decayScore(5, 0, now.Add(-30*24*time.Hour), 7, false)
	if s4 > 0.5 {
		t.Errorf("decayScore(5, 30d ago, 7) = %f, want <0.5", s4)
	}

	// weightSuccess=true with all successes should give higher score than without
	s5 := decayScore(10, 10, now, 7, true)
	if s5 < s1 {
		t.Errorf("decayScore with all successes should be >= score without weighting: %f < %f", s5, s1)
	}
}

func TestGetFrequentToolsWeighted(t *testing.T) {
	// Reset adaptive state
	adaptiveToolState.mu.Lock()
	adaptiveToolState.entries = make(map[string]*ToolUsageEntry)
	adaptiveToolState.mu.Unlock()

	now := time.Now()

	// Load test data
	LoadAdaptiveToolState([]ToolUsageEntry{
		{ToolName: "shell", TotalCount: 50, LastUsed: now.Add(-1 * 24 * time.Hour)},     // recent, high count
		{ToolName: "docker", TotalCount: 30, LastUsed: now.Add(-2 * 24 * time.Hour)},    // recent, medium count
		{ToolName: "ansible", TotalCount: 20, LastUsed: now.Add(-21 * 24 * time.Hour)},  // old, medium count
		{ToolName: "forgotten", TotalCount: 5, LastUsed: now.Add(-60 * 24 * time.Hour)}, // very old, low count → should be filtered
	})

	result := GetFrequentToolsWeighted(3, 7, false)
	if len(result) < 2 || len(result) > 3 {
		t.Fatalf("GetFrequentToolsWeighted(3, 7) returned %d tools, want 2-3", len(result))
	}
	if result[0] != "shell" {
		t.Errorf("Top tool should be shell, got %q", result[0])
	}
	if result[1] != "docker" {
		t.Errorf("Second tool should be docker, got %q", result[1])
	}
}

func TestGetFrequentToolsWeightedEmpty(t *testing.T) {
	// Reset adaptive state
	adaptiveToolState.mu.Lock()
	adaptiveToolState.entries = make(map[string]*ToolUsageEntry)
	adaptiveToolState.mu.Unlock()

	result := GetFrequentToolsWeighted(10, 7, false)
	if len(result) != 0 {
		t.Errorf("Empty state should return 0 tools, got %d", len(result))
	}
}

func TestRecordAdaptiveToolUsage(t *testing.T) {
	adaptiveToolState.mu.Lock()
	adaptiveToolState.entries = make(map[string]*ToolUsageEntry)
	adaptiveToolState.mu.Unlock()

	RecordAdaptiveToolUsage("shell", true)
	RecordAdaptiveToolUsage("shell", true)
	RecordAdaptiveToolUsage("docker", false)

	scores := GetAdaptiveToolScores(7, false)
	if len(scores) != 2 {
		t.Fatalf("Expected 2 tools, got %d", len(scores))
	}
	// shell should be first (higher count)
	if scores[0].Tool != "shell" {
		t.Errorf("Top tool should be shell, got %q", scores[0].Tool)
	}
	if scores[0].Count != 2 {
		t.Errorf("shell count should be 2, got %d", scores[0].Count)
	}
}

// ── Prompt build stats tests ──────────────────────────────────────────────────

func resetPromptStats() {
	globalStats.mu.Lock()
	globalStats.records = nil
	globalStats.mu.Unlock()
}

func TestPromptBuildRecordSavingsBreakdown(t *testing.T) {
	resetPromptStats()
	defer resetPromptStats()

	RecordBuild(PromptBuildRecord{
		Timestamp:     time.Now(),
		Tier:          "full",
		RawLen:        10000,
		OptimizedLen:  6000,
		SavedChars:    200,
		FormatSavings: 200,
		ShedSavings:   1800,
		FilterSavings: 2000,
		Tokens:        1500,
		ModulesLoaded: 10,
		ModulesUsed:   7,
	})

	agg := GetAggregatedStats()
	if agg.TotalBuilds != 1 {
		t.Fatalf("TotalBuilds = %d, want 1", agg.TotalBuilds)
	}

	// True total savings = RawLen - OptimizedLen = 4000
	wantTrueSaved := 4000
	if agg.AvgSavedChars != wantTrueSaved {
		t.Errorf("AvgSavedChars = %d, want %d (true total = RawLen - OptimizedLen)", agg.AvgSavedChars, wantTrueSaved)
	}
	if agg.TotalSavedChars != int64(wantTrueSaved) {
		t.Errorf("TotalSavedChars = %d, want %d", agg.TotalSavedChars, wantTrueSaved)
	}

	// Breakdown averages
	if agg.AvgFormatSavings != 200 {
		t.Errorf("AvgFormatSavings = %d, want 200", agg.AvgFormatSavings)
	}
	if agg.AvgShedSavings != 1800 {
		t.Errorf("AvgShedSavings = %d, want 1800", agg.AvgShedSavings)
	}
	if agg.AvgFilterSavings != 2000 {
		t.Errorf("AvgFilterSavings = %d, want 2000", agg.AvgFilterSavings)
	}
}

func TestPromptStatsAvgOptimizationPct(t *testing.T) {
	resetPromptStats()
	defer resetPromptStats()

	// Build with 40% true reduction (6000 of 10000 chars removed)
	RecordBuild(PromptBuildRecord{
		Timestamp:    time.Now(),
		Tier:         "full",
		RawLen:       10000,
		OptimizedLen: 6000, // 40% removed
		SavedChars:   200,
	})

	agg := GetAggregatedStats()

	// avg_optimization_pct should be 40.0 (not 2.0 from SavedChars/RawLen)
	if agg.AvgOptimizationPct < 39.9 || agg.AvgOptimizationPct > 40.1 {
		t.Errorf("AvgOptimizationPct = %.2f, want ~40.0 (true RawLen-OptimizedLen/RawLen)", agg.AvgOptimizationPct)
	}
}

func TestPromptStatsShedRate(t *testing.T) {
	resetPromptStats()
	defer resetPromptStats()

	// 2 builds: 1 with budget shed, 1 without
	RecordBuild(PromptBuildRecord{
		Timestamp: time.Now(), Tier: "full",
		RawLen: 1000, OptimizedLen: 900,
		BudgetShed: true, ShedSavings: 80,
	})
	RecordBuild(PromptBuildRecord{
		Timestamp: time.Now(), Tier: "full",
		RawLen: 1000, OptimizedLen: 980,
		BudgetShed: false,
	})

	agg := GetAggregatedStats()
	if agg.BudgetShedCount != 1 {
		t.Errorf("BudgetShedCount = %d, want 1", agg.BudgetShedCount)
	}
	// ShedRate = 1/2 * 100 = 50%
	if agg.ShedRatePct < 49.9 || agg.ShedRatePct > 50.1 {
		t.Errorf("ShedRatePct = %.2f, want 50.0", agg.ShedRatePct)
	}
}

func TestPromptStatsModuleFilterRate(t *testing.T) {
	resetPromptStats()
	defer resetPromptStats()

	// 10 modules loaded, 6 used → 40% filtered
	RecordBuild(PromptBuildRecord{
		Timestamp:     time.Now(),
		Tier:          "full",
		RawLen:        5000,
		OptimizedLen:  4000,
		ModulesLoaded: 10,
		ModulesUsed:   6,
	})

	agg := GetAggregatedStats()
	if agg.AvgModuleFilterRatePct < 39.9 || agg.AvgModuleFilterRatePct > 40.1 {
		t.Errorf("AvgModuleFilterRatePct = %.2f, want 40.0", agg.AvgModuleFilterRatePct)
	}
}

func TestPromptStatsTotalsBreakdown(t *testing.T) {
	resetPromptStats()
	defer resetPromptStats()

	RecordBuild(PromptBuildRecord{
		Timestamp: time.Now(), Tier: "full",
		RawLen: 10000, OptimizedLen: 5000,
		FormatSavings: 500, ShedSavings: 2000, FilterSavings: 2500,
	})
	RecordBuild(PromptBuildRecord{
		Timestamp: time.Now(), Tier: "compact",
		RawLen: 8000, OptimizedLen: 4000,
		FormatSavings: 300, ShedSavings: 1500, FilterSavings: 2200,
	})

	agg := GetAggregatedStats()
	if agg.TotalFormatSavings != 800 {
		t.Errorf("TotalFormatSavings = %d, want 800", agg.TotalFormatSavings)
	}
	if agg.TotalShedSavings != 3500 {
		t.Errorf("TotalShedSavings = %d, want 3500", agg.TotalShedSavings)
	}
	if agg.TotalFilterSavings != 4700 {
		t.Errorf("TotalFilterSavings = %d, want 4700", agg.TotalFilterSavings)
	}
	// TotalSavedChars = sum of (RawLen - OptimizedLen) = 5000 + 4000 = 9000
	if agg.TotalSavedChars != 9000 {
		t.Errorf("TotalSavedChars = %d, want 9000", agg.TotalSavedChars)
	}
}

func TestPromptStatsEmptyState(t *testing.T) {
	resetPromptStats()
	defer resetPromptStats()

	agg := GetAggregatedStats()
	if agg.TotalBuilds != 0 {
		t.Errorf("empty: TotalBuilds = %d, want 0", agg.TotalBuilds)
	}
	if agg.ShedRatePct != 0 {
		t.Errorf("empty: ShedRatePct = %.2f, want 0", agg.ShedRatePct)
	}
	if agg.AvgModuleFilterRatePct != 0 {
		t.Errorf("empty: AvgModuleFilterRatePct = %.2f, want 0", agg.AvgModuleFilterRatePct)
	}
}
