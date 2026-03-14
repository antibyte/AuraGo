package prompts

import (
	"testing"
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
