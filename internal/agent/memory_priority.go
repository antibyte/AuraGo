package agent

import (
	"strings"
	"time"

	"aurago/internal/memory"
)

func adjustedMemoryPriority(meta memory.MemoryMeta, now time.Time) int {
	if now.IsZero() {
		now = time.Now()
	}
	priority := meta.AccessCount - memoryDaysSince(meta.LastAccessed, now)
	priority -= memoryEffectivenessPenalty(meta)
	return priority
}

func memoryEffectivenessPenalty(meta memory.MemoryMeta) int {
	total := meta.UsefulCount + meta.UselessCount
	if total < 2 || meta.UsefulCount >= meta.UselessCount {
		return 0
	}
	penalty := 1 + (meta.UselessCount - meta.UsefulCount)
	if penalty > 4 {
		return 4
	}
	return penalty
}

func memoryDaysSince(lastAccessed string, now time.Time) int {
	value := strings.TrimSpace(lastAccessed)
	if value == "" {
		return 0
	}
	parsed, err := time.Parse(time.RFC3339, strings.Replace(value, " ", "T", 1)+"Z")
	if err != nil {
		return 0
	}
	return int(now.Sub(parsed).Hours() / 24)
}
