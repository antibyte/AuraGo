package agent

import (
	"testing"
	"time"

	"aurago/internal/memory"
)

func TestAdjustedMemoryPriorityPenalizesUnderperformingMemories(t *testing.T) {
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	priority := adjustedMemoryPriority(memory.MemoryMeta{
		AccessCount:        3,
		LastAccessed:       "2026-03-31 12:00:00",
		UsefulCount:        1,
		UselessCount:       4,
		VerificationStatus: "unverified",
	}, now)

	if priority != -2 {
		t.Fatalf("priority = %d, want -2", priority)
	}
}

func TestAdjustedMemoryPriorityLeavesHelpfulMemoriesUntouched(t *testing.T) {
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	priority := adjustedMemoryPriority(memory.MemoryMeta{
		AccessCount:  3,
		LastAccessed: "2026-03-31 12:00:00",
		UsefulCount:  4,
		UselessCount: 1,
	}, now)

	if priority != 2 {
		t.Fatalf("priority = %d, want 2", priority)
	}
}
