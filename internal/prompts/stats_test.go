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
