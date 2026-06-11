package agent

import (
	"fmt"
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
	parsed, err := parseMemoryTimestamp(value)
	if err != nil {
		return 0
	}
	return int(now.Sub(parsed).Hours() / 24)
}

func parseMemoryTimestamp(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC(), nil
		}
	}
	if strings.Contains(value, " ") && !strings.HasSuffix(value, "Z") {
		return time.Parse(time.RFC3339, strings.Replace(value, " ", "T", 1)+"Z")
	}
	return time.Time{}, fmt.Errorf("unsupported memory timestamp %q", value)
}
