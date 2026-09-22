package memory

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// InsertMaintenanceDailySummary never overwrites a summary written by another
// run or by the user, including a writer racing with the initial existence check.
func (s *SQLiteMemory) InsertMaintenanceDailySummary(summary DailySummary) (bool, error) {
	if _, err := time.Parse("2006-01-02", summary.Date); err != nil {
		return false, fmt.Errorf("maintenance summary date: %w", err)
	}
	if strings.TrimSpace(summary.Summary) == "" {
		return false, fmt.Errorf("maintenance summary is empty")
	}
	topics, err := json.Marshal(summary.KeyTopics)
	if err != nil {
		return false, fmt.Errorf("encode summary topics: %w", err)
	}
	usage, err := json.Marshal(summary.ToolUsage)
	if err != nil {
		return false, fmt.Errorf("encode summary usage: %w", err)
	}
	result, err := s.db.Exec(`INSERT INTO daily_summaries
		(date, summary, key_topics, tool_usage, sentiment, generated_at)
		VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT(date) DO NOTHING`,
		summary.Date, summary.Summary, string(topics), string(usage), summary.Sentiment,
		time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return false, fmt.Errorf("insert maintenance summary: %w", err)
	}
	n, err := result.RowsAffected()
	return n > 0, err
}
