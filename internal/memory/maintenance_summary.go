package memory

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// MissingMaintenanceSummaryDates selects completed local days with journal
// content. Yesterday has priority; older gaps are filled oldest first.
func (s *SQLiteMemory) MissingMaintenanceSummaryDates(start time.Time) ([]string, error) {
	yesterday := start.AddDate(0, 0, -1).Format("2006-01-02")
	oldest := start.AddDate(0, 0, -7).Format("2006-01-02")
	rows, err := s.db.Query(`SELECT DISTINCT j.date FROM journal_entries j
		LEFT JOIN daily_summaries d ON d.date = j.date
		WHERE j.date >= ? AND j.date <= ? AND d.date IS NULL
		ORDER BY CASE WHEN j.date = ? THEN 0 ELSE 1 END, j.date LIMIT 3`, oldest, yesterday, yesterday)
	if err != nil {
		return nil, fmt.Errorf("select maintenance summary dates: %w", err)
	}
	defer rows.Close()
	var dates []string
	for rows.Next() {
		var date string
		if err := rows.Scan(&date); err != nil {
			return nil, fmt.Errorf("scan maintenance summary date: %w", err)
		}
		dates = append(dates, date)
	}
	return dates, rows.Err()
}

// CountMissingMaintenanceSummaries includes work beyond the three-day run cap.
func (s *SQLiteMemory) CountMissingMaintenanceSummaries(start time.Time) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(DISTINCT j.date) FROM journal_entries j
		LEFT JOIN daily_summaries d ON d.date = j.date
		WHERE j.date >= ? AND j.date <= ? AND d.date IS NULL`,
		start.AddDate(0, 0, -7).Format("2006-01-02"), start.AddDate(0, 0, -1).Format("2006-01-02")).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count maintenance summary backlog: %w", err)
	}
	return count, nil
}

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
