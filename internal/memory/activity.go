package memory

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ActivityDigest is the normalized structured summary for a single turn.
type ActivityDigest struct {
	Intent          string   `json:"intent"`
	UserGoal        string   `json:"user_goal"`
	ActionsTaken    []string `json:"actions_taken"`
	Outcomes        []string `json:"outcomes"`
	ImportantPoints []string `json:"important_points"`
	PendingItems    []string `json:"pending_items"`
	Importance      int      `json:"importance"`
	Entities        []string `json:"entities"`
}

// ActivityTurn stores a compact, queryable summary of a handled turn.
type ActivityTurn struct {
	ID               int64    `json:"id"`
	Timestamp        string   `json:"timestamp"`
	Date             string   `json:"date"`
	SessionID        string   `json:"session_id"`
	Channel          string   `json:"channel"`
	IsAutonomous     bool     `json:"is_autonomous"`
	UserRelevant     bool     `json:"user_relevant"`
	Status           string   `json:"status"`
	Importance       int      `json:"importance"`
	Intent           string   `json:"intent"`
	UserRequest      string   `json:"user_request"`
	UserGoal         string   `json:"user_goal"`
	ActionsTaken     []string `json:"actions_taken"`
	Outcomes         []string `json:"outcomes"`
	ImportantPoints  []string `json:"important_points"`
	PendingItems     []string `json:"pending_items"`
	ToolNames        []string `json:"tool_names"`
	LinkedJournalIDs []int64  `json:"linked_journal_ids"`
	LinkedNoteIDs    []int64  `json:"linked_note_ids"`
	LinkedMemoryIDs  []string `json:"linked_memory_ids"`
	Source           string   `json:"source"`
}

// DailyActivityRollup stores a condensed day view built from activity + memory.
type DailyActivityRollup struct {
	Date            string         `json:"date"`
	Summary         string         `json:"summary"`
	Highlights      []string       `json:"highlights"`
	ImportantPoints []string       `json:"important_points"`
	PendingItems    []string       `json:"pending_items"`
	TopIntents      []string       `json:"top_intents"`
	ToolUsage       map[string]int `json:"tool_usage"`
	UserGoals       []string       `json:"user_goals"`
	GeneratedAt     string         `json:"generated_at"`
	Source          string         `json:"source"`
}

// ActivityOverviewResponse is the API/UI payload for the recent timeline.
type ActivityOverviewResponse struct {
	OverviewSummary string                `json:"overview_summary"`
	Days            []DailyActivityRollup `json:"days"`
	Highlights      []string              `json:"highlights"`
	ImportantPoints []string              `json:"important_points"`
	PendingItems    []string              `json:"pending_items"`
	TopGoals        []string              `json:"top_goals"`
	Entries         []ActivityTurn        `json:"entries,omitempty"`
}

// InitActivityTables creates persistent activity/timeline tables.
func (s *SQLiteMemory) InitActivityTables() error {
	schema := `
	CREATE TABLE IF NOT EXISTS activity_turns (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		date TEXT NOT NULL,
		session_id TEXT DEFAULT '',
		channel TEXT DEFAULT '',
		is_autonomous BOOLEAN DEFAULT 0,
		user_relevant BOOLEAN DEFAULT 1,
		status TEXT DEFAULT 'completed',
		importance INTEGER DEFAULT 2,
		intent TEXT DEFAULT '',
		user_request TEXT DEFAULT '',
		user_goal TEXT DEFAULT '',
		actions_taken_json TEXT DEFAULT '[]',
		outcomes_json TEXT DEFAULT '[]',
		important_points_json TEXT DEFAULT '[]',
		pending_items_json TEXT DEFAULT '[]',
		tool_names_json TEXT DEFAULT '[]',
		linked_journal_ids_json TEXT DEFAULT '[]',
		linked_note_ids_json TEXT DEFAULT '[]',
		linked_memory_ids_json TEXT DEFAULT '[]',
		actions_taken_text TEXT DEFAULT '',
		outcomes_text TEXT DEFAULT '',
		important_points_text TEXT DEFAULT '',
		pending_items_text TEXT DEFAULT '',
		source TEXT DEFAULT 'runtime'
	);

	CREATE TABLE IF NOT EXISTS daily_activity_rollups (
		date TEXT PRIMARY KEY,
		summary TEXT DEFAULT '',
		highlights_json TEXT DEFAULT '[]',
		important_points_json TEXT DEFAULT '[]',
		pending_items_json TEXT DEFAULT '[]',
		top_intents_json TEXT DEFAULT '[]',
		tool_usage_json TEXT DEFAULT '{}',
		user_goals_json TEXT DEFAULT '[]',
		generated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		source TEXT DEFAULT 'maintenance'
	);

	CREATE INDEX IF NOT EXISTS idx_activity_turns_date ON activity_turns(date DESC, timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_activity_turns_session ON activity_turns(session_id, timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_activity_turns_relevance ON activity_turns(user_relevant, is_autonomous, date DESC);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("activity schema: %w", err)
	}

	if _, err := s.db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS activity_turns_fts USING fts5(
			intent,
			user_request,
			user_goal,
			outcomes,
			important_points,
			content='activity_turns',
			content_rowid='id'
		);
	`); err != nil && s.logger != nil {
		s.logger.Warn("Failed to initialize activity FTS index", "error", err)
	}

	for _, trigger := range []string{
		`CREATE TRIGGER IF NOT EXISTS activity_turns_ai AFTER INSERT ON activity_turns BEGIN
			INSERT INTO activity_turns_fts(rowid, intent, user_request, user_goal, outcomes, important_points)
			VALUES (new.id, new.intent, new.user_request, new.user_goal, new.outcomes_text, new.important_points_text);
		END;`,
		`CREATE TRIGGER IF NOT EXISTS activity_turns_ad AFTER DELETE ON activity_turns BEGIN
			INSERT INTO activity_turns_fts(activity_turns_fts, rowid, intent, user_request, user_goal, outcomes, important_points)
			VALUES('delete', old.id, old.intent, old.user_request, old.user_goal, old.outcomes_text, old.important_points_text);
		END;`,
		`CREATE TRIGGER IF NOT EXISTS activity_turns_au AFTER UPDATE ON activity_turns BEGIN
			INSERT INTO activity_turns_fts(activity_turns_fts, rowid, intent, user_request, user_goal, outcomes, important_points)
			VALUES('delete', old.id, old.intent, old.user_request, old.user_goal, old.outcomes_text, old.important_points_text);
			INSERT INTO activity_turns_fts(rowid, intent, user_request, user_goal, outcomes, important_points)
			VALUES (new.id, new.intent, new.user_request, new.user_goal, new.outcomes_text, new.important_points_text);
		END;`,
	} {
		if _, err := s.db.Exec(trigger); err != nil && s.logger != nil {
			s.logger.Warn("Failed to initialize activity trigger", "error", err)
		}
	}

	return nil
}

// InsertActivityTurn stores one handled user-visible turn.
func (s *SQLiteMemory) InsertActivityTurn(turn ActivityTurn) (int64, error) {
	if turn.Date == "" {
		turn.Date = time.Now().Format("2006-01-02")
	}
	if turn.Status == "" {
		turn.Status = "completed"
	}
	if turn.Source == "" {
		turn.Source = "runtime"
	}
	if turn.Importance < 1 || turn.Importance > 4 {
		turn.Importance = 2
	}
	if turn.Timestamp == "" {
		turn.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	if turn.UserGoal == "" {
		turn.UserGoal = turn.Intent
	}
	actionsJSON, _ := json.Marshal(normalizeStringSlice(turn.ActionsTaken, 8, 220))
	outcomesJSON, _ := json.Marshal(normalizeStringSlice(turn.Outcomes, 8, 320))
	importantJSON, _ := json.Marshal(normalizeStringSlice(turn.ImportantPoints, 8, 240))
	pendingJSON, _ := json.Marshal(normalizeStringSlice(turn.PendingItems, 8, 220))
	toolsJSON, _ := json.Marshal(normalizeStringSlice(turn.ToolNames, 12, 80))
	journalJSON, _ := json.Marshal(turn.LinkedJournalIDs)
	noteJSON, _ := json.Marshal(turn.LinkedNoteIDs)
	memoryJSON, _ := json.Marshal(normalizeStringSlice(turn.LinkedMemoryIDs, 12, 120))

	res, err := s.db.Exec(`
		INSERT INTO activity_turns (
			timestamp, date, session_id, channel, is_autonomous, user_relevant, status,
			importance, intent, user_request, user_goal,
			actions_taken_json, outcomes_json, important_points_json, pending_items_json,
			tool_names_json, linked_journal_ids_json, linked_note_ids_json, linked_memory_ids_json,
			actions_taken_text, outcomes_text, important_points_text, pending_items_text, source
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		turn.Timestamp, turn.Date, turn.SessionID, turn.Channel, turn.IsAutonomous, turn.UserRelevant, turn.Status,
		turn.Importance, truncateForActivity(turn.Intent, 220), truncateForActivity(turn.UserRequest, 1200), truncateForActivity(turn.UserGoal, 320),
		string(actionsJSON), string(outcomesJSON), string(importantJSON), string(pendingJSON),
		string(toolsJSON), string(journalJSON), string(noteJSON), string(memoryJSON),
		strings.Join(turn.ActionsTaken, "\n"), strings.Join(turn.Outcomes, "\n"), strings.Join(turn.ImportantPoints, "\n"), strings.Join(turn.PendingItems, "\n"), turn.Source,
	)
	if err != nil {
		return 0, fmt.Errorf("insert activity turn: %w", err)
	}
	return res.LastInsertId()
}

// SearchActivityTurnsInRange searches stored activity turns in an optional date window.
func (s *SQLiteMemory) SearchActivityTurnsInRange(keyword, fromDate, toDate string, limit int) ([]ActivityTurn, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	trimmedKeyword := strings.TrimSpace(keyword)
	var (
		rows interface {
			Close() error
			Next() bool
			Scan(dest ...interface{}) error
			Err() error
		}
		err error
	)
	if trimmedKeyword != "" {
		pattern := "%" + trimmedKeyword + "%"
		rows, err = s.db.Query(`
			SELECT id, timestamp, date, session_id, channel, is_autonomous, user_relevant, status,
				importance, intent, user_request, user_goal,
				actions_taken_json, outcomes_json, important_points_json, pending_items_json,
				tool_names_json, linked_journal_ids_json, linked_note_ids_json, linked_memory_ids_json, source
			FROM activity_turns
			WHERE (? = '' OR date >= ?)
			  AND (? = '' OR date <= ?)
			  AND (
				intent LIKE ? OR user_request LIKE ? OR user_goal LIKE ? OR
				outcomes_text LIKE ? OR important_points_text LIKE ? OR actions_taken_text LIKE ? OR pending_items_text LIKE ?
			  )
			ORDER BY date DESC, timestamp DESC
			LIMIT ?`,
			fromDate, fromDate, toDate, toDate,
			pattern, pattern, pattern, pattern, pattern, pattern, pattern,
			limit,
		)
	} else {
		rows, err = s.db.Query(`
			SELECT id, timestamp, date, session_id, channel, is_autonomous, user_relevant, status,
				importance, intent, user_request, user_goal,
				actions_taken_json, outcomes_json, important_points_json, pending_items_json,
				tool_names_json, linked_journal_ids_json, linked_note_ids_json, linked_memory_ids_json, source
			FROM activity_turns
			WHERE (? = '' OR date >= ?)
			  AND (? = '' OR date <= ?)
			ORDER BY date DESC, timestamp DESC
			LIMIT ?`,
			fromDate, fromDate, toDate, toDate, limit,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("query activity turns: %w", err)
	}
	defer rows.Close()
	return scanActivityTurns(rows)
}

// GetActivityTurnsForDate lists activity turns for one day.
func (s *SQLiteMemory) GetActivityTurnsForDate(date string, limit int) ([]ActivityTurn, error) {
	return s.SearchActivityTurnsInRange("", date, date, limit)
}

// UpsertDailyActivityRollup stores the latest day rollup.
func (s *SQLiteMemory) UpsertDailyActivityRollup(rollup DailyActivityRollup) error {
	if rollup.Date == "" {
		return fmt.Errorf("date is required")
	}
	if rollup.GeneratedAt == "" {
		rollup.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if rollup.Source == "" {
		rollup.Source = "maintenance"
	}
	highlightsJSON, _ := json.Marshal(normalizeStringSlice(rollup.Highlights, 8, 240))
	importantJSON, _ := json.Marshal(normalizeStringSlice(rollup.ImportantPoints, 10, 240))
	pendingJSON, _ := json.Marshal(normalizeStringSlice(rollup.PendingItems, 10, 240))
	intentsJSON, _ := json.Marshal(normalizeStringSlice(rollup.TopIntents, 8, 160))
	goalsJSON, _ := json.Marshal(normalizeStringSlice(rollup.UserGoals, 8, 200))
	toolUsageJSON, _ := json.Marshal(rollup.ToolUsage)
	_, err := s.db.Exec(`
		INSERT INTO daily_activity_rollups (
			date, summary, highlights_json, important_points_json, pending_items_json,
			top_intents_json, tool_usage_json, user_goals_json, generated_at, source
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(date) DO UPDATE SET
			summary=excluded.summary,
			highlights_json=excluded.highlights_json,
			important_points_json=excluded.important_points_json,
			pending_items_json=excluded.pending_items_json,
			top_intents_json=excluded.top_intents_json,
			tool_usage_json=excluded.tool_usage_json,
			user_goals_json=excluded.user_goals_json,
			generated_at=excluded.generated_at,
			source=excluded.source
	`, rollup.Date, truncateForActivity(rollup.Summary, 1800), string(highlightsJSON), string(importantJSON), string(pendingJSON), string(intentsJSON), string(toolUsageJSON), string(goalsJSON), rollup.GeneratedAt, rollup.Source)
	if err != nil {
		return fmt.Errorf("upsert daily activity rollup: %w", err)
	}
	return nil
}

// GetRecentDailyActivityRollups returns recent rollups newest first.
func (s *SQLiteMemory) GetRecentDailyActivityRollups(days int) ([]DailyActivityRollup, error) {
	if days <= 0 || days > 30 {
		days = 7
	}
	rows, err := s.db.Query(`
		SELECT date, summary, highlights_json, important_points_json, pending_items_json,
			top_intents_json, tool_usage_json, user_goals_json, generated_at, source
		FROM daily_activity_rollups
		ORDER BY date DESC
		LIMIT ?`, days)
	if err != nil {
		return nil, fmt.Errorf("query activity rollups: %w", err)
	}
	defer rows.Close()

	out := make([]DailyActivityRollup, 0, days)
	for rows.Next() {
		var rollup DailyActivityRollup
		var highlightsJSON, importantJSON, pendingJSON, intentsJSON, toolUsageJSON, userGoalsJSON string
		if err := rows.Scan(&rollup.Date, &rollup.Summary, &highlightsJSON, &importantJSON, &pendingJSON, &intentsJSON, &toolUsageJSON, &userGoalsJSON, &rollup.GeneratedAt, &rollup.Source); err != nil {
			return nil, fmt.Errorf("scan activity rollup: %w", err)
		}
		_ = json.Unmarshal([]byte(highlightsJSON), &rollup.Highlights)
		_ = json.Unmarshal([]byte(importantJSON), &rollup.ImportantPoints)
		_ = json.Unmarshal([]byte(pendingJSON), &rollup.PendingItems)
		_ = json.Unmarshal([]byte(intentsJSON), &rollup.TopIntents)
		_ = json.Unmarshal([]byte(toolUsageJSON), &rollup.ToolUsage)
		_ = json.Unmarshal([]byte(userGoalsJSON), &rollup.UserGoals)
		out = append(out, rollup)
	}
	return out, rows.Err()
}

// GenerateDailyActivityRollup builds a rollup for one day from stored activity + notes/journal.
func (s *SQLiteMemory) GenerateDailyActivityRollup(date string) (DailyActivityRollup, error) {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	turns, err := s.GetActivityTurnsForDate(date, 40)
	if err != nil {
		return DailyActivityRollup{}, err
	}

	notes, _ := s.ListNotes("", 0)
	journalEntries, _ := s.GetJournalEntries(date, date, nil, 10)
	episodicEntries, _ := s.SearchEpisodicMemoriesInRange("", date, date, 5)

	if len(turns) == 0 && len(journalEntries) == 0 && len(episodicEntries) == 0 && len(notes) == 0 {
		return DailyActivityRollup{}, nil
	}

	toolUsage := map[string]int{}
	highlights := make([]string, 0, 6)
	important := make([]string, 0, 8)
	pending := make([]string, 0, 8)
	intents := make([]string, 0, 6)
	goals := make([]string, 0, 6)
	for _, turn := range turns {
		for _, tool := range turn.ToolNames {
			toolUsage[tool]++
		}
		if turn.Intent != "" {
			intents = append(intents, turn.Intent)
		}
		if turn.UserGoal != "" {
			goals = append(goals, turn.UserGoal)
		}
		highlights = append(highlights, turn.Outcomes...)
		important = append(important, turn.ImportantPoints...)
		pending = append(pending, turn.PendingItems...)
	}
	for _, entry := range journalEntries {
		important = append(important, entry.Title)
	}
	for _, episode := range episodicEntries {
		highlights = append(highlights, episode.Title+": "+episode.Summary)
	}
	for _, note := range notes {
		if note.Done {
			continue
		}
		pending = append(pending, note.Title)
	}

	rollup := DailyActivityRollup{
		Date:            date,
		Summary:         buildActivityDaySummary(date, turns, journalEntries, episodicEntries, pending),
		Highlights:      uniqueNonEmptySlice(highlights, 5),
		ImportantPoints: uniqueNonEmptySlice(important, 6),
		PendingItems:    uniqueNonEmptySlice(pending, 5),
		TopIntents:      uniqueNonEmptySlice(intents, 4),
		ToolUsage:       toolUsage,
		UserGoals:       uniqueNonEmptySlice(goals, 4),
		GeneratedAt:     time.Now().UTC().Format(time.RFC3339),
		Source:          "maintenance",
	}
	if err := s.UpsertDailyActivityRollup(rollup); err != nil {
		return DailyActivityRollup{}, err
	}
	return rollup, nil
}

// BuildRecentActivityOverview assembles a recent multi-day overview.
func (s *SQLiteMemory) BuildRecentActivityOverview(days int, includeEntries bool) (*ActivityOverviewResponse, error) {
	if days <= 0 || days > 14 {
		days = 7
	}

	rollups, err := s.GetRecentDailyActivityRollups(days)
	if err != nil {
		return nil, err
	}
	if len(rollups) < days {
		for i := 0; i < days; i++ {
			date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
			found := false
			for _, existing := range rollups {
				if existing.Date == date {
					found = true
					break
				}
			}
			if found {
				continue
			}
			generated, genErr := s.GenerateDailyActivityRollup(date)
			if genErr == nil && generated.Date != "" {
				rollups = append(rollups, generated)
			}
		}
		sort.Slice(rollups, func(i, j int) bool { return rollups[i].Date > rollups[j].Date })
		if len(rollups) > days {
			rollups = rollups[:days]
		}
	}

	if len(rollups) == 0 {
		return s.bootstrapActivityOverview(days, includeEntries)
	}

	response := &ActivityOverviewResponse{
		Days:            rollups,
		Highlights:      make([]string, 0, 8),
		ImportantPoints: make([]string, 0, 10),
		PendingItems:    make([]string, 0, 8),
		TopGoals:        make([]string, 0, 8),
	}

	for _, day := range rollups {
		response.Highlights = append(response.Highlights, day.Highlights...)
		response.ImportantPoints = append(response.ImportantPoints, day.ImportantPoints...)
		response.PendingItems = append(response.PendingItems, day.PendingItems...)
		response.TopGoals = append(response.TopGoals, day.UserGoals...)
	}
	response.Highlights = uniqueNonEmptySlice(response.Highlights, 6)
	response.ImportantPoints = uniqueNonEmptySlice(response.ImportantPoints, 8)
	response.PendingItems = uniqueNonEmptySlice(response.PendingItems, 5)
	response.TopGoals = uniqueNonEmptySlice(response.TopGoals, 5)

	startDate := time.Now().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	if includeEntries {
		entries, err := s.SearchActivityTurnsInRange("", startDate, time.Now().Format("2006-01-02"), 30)
		if err == nil {
			response.Entries = entries
		}
	}

	response.OverviewSummary = buildRecentActivityOverviewSummary(days, rollups, response.PendingItems, response.Highlights)
	return response, nil
}

// BuildRecentActivityPromptOverview builds a compact prompt section for the last days.
func (s *SQLiteMemory) BuildRecentActivityPromptOverview(days int) (string, error) {
	overview, err := s.BuildRecentActivityOverview(days, false)
	if err != nil || overview == nil {
		return "", err
	}
	if overview.OverviewSummary == "" && len(overview.Days) == 0 {
		return "", nil
	}
	var sb strings.Builder
	if overview.OverviewSummary != "" {
		sb.WriteString("Summary: ")
		sb.WriteString(overview.OverviewSummary)
		sb.WriteString("\n")
	}
	if len(overview.Days) > 0 {
		sb.WriteString("Recent days:\n")
		for i, day := range overview.Days {
			if i >= 3 {
				break
			}
			sb.WriteString("- ")
			sb.WriteString(day.Date)
			sb.WriteString(": ")
			sb.WriteString(truncateForActivity(day.Summary, 220))
			sb.WriteString("\n")
		}
	}
	if len(overview.PendingItems) > 0 {
		sb.WriteString("Open items:\n")
		for i, item := range overview.PendingItems {
			if i >= 5 {
				break
			}
			sb.WriteString("- ")
			sb.WriteString(item)
			sb.WriteString("\n")
		}
	}
	if len(overview.Highlights) > 0 {
		sb.WriteString("Highlights:\n")
		for i, item := range overview.Highlights {
			if i >= 3 {
				break
			}
			sb.WriteString("- ")
			sb.WriteString(item)
			sb.WriteString("\n")
		}
	}
	return strings.TrimSpace(sb.String()), nil
}

func (s *SQLiteMemory) bootstrapActivityOverview(days int, includeEntries bool) (*ActivityOverviewResponse, error) {
	response := &ActivityOverviewResponse{}
	startDate := time.Now().AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	today := time.Now().Format("2006-01-02")

	summaries, _ := s.GetRecentDailySummaries(days)
	for _, summary := range summaries {
		response.Days = append(response.Days, DailyActivityRollup{
			Date:        summary.Date,
			Summary:     summary.Summary,
			Highlights:  uniqueNonEmptySlice(summary.KeyTopics, 4),
			ToolUsage:   summary.ToolUsage,
			GeneratedAt: summary.GeneratedAt,
			Source:      "bootstrap",
		})
	}

	notes, _ := s.ListNotes("", 0)
	for _, note := range notes {
		if !note.Done {
			response.PendingItems = append(response.PendingItems, note.Title)
		}
	}
	response.PendingItems = uniqueNonEmptySlice(response.PendingItems, 5)

	journalEntries, _ := s.GetJournalEntries(startDate, today, nil, 12)
	for _, entry := range journalEntries {
		response.Highlights = append(response.Highlights, entry.Title)
		response.ImportantPoints = append(response.ImportantPoints, entry.Title)
	}
	response.Highlights = uniqueNonEmptySlice(response.Highlights, 6)
	response.ImportantPoints = uniqueNonEmptySlice(response.ImportantPoints, 8)
	response.OverviewSummary = buildRecentActivityOverviewSummary(days, response.Days, response.PendingItems, response.Highlights)

	if includeEntries {
		entries, _ := s.SearchActivityTurnsInRange("", startDate, today, 30)
		response.Entries = entries
	}
	return response, nil
}

func scanActivityTurns(rows interface {
	Close() error
	Next() bool
	Scan(dest ...interface{}) error
	Err() error
}) ([]ActivityTurn, error) {
	out := make([]ActivityTurn, 0, 8)
	for rows.Next() {
		var turn ActivityTurn
		var actionsJSON, outcomesJSON, importantJSON, pendingJSON, toolNamesJSON, journalJSON, noteJSON, memoryJSON string
		if err := rows.Scan(
			&turn.ID, &turn.Timestamp, &turn.Date, &turn.SessionID, &turn.Channel, &turn.IsAutonomous, &turn.UserRelevant, &turn.Status,
			&turn.Importance, &turn.Intent, &turn.UserRequest, &turn.UserGoal,
			&actionsJSON, &outcomesJSON, &importantJSON, &pendingJSON,
			&toolNamesJSON, &journalJSON, &noteJSON, &memoryJSON, &turn.Source,
		); err != nil {
			return nil, fmt.Errorf("scan activity turn: %w", err)
		}
		_ = json.Unmarshal([]byte(actionsJSON), &turn.ActionsTaken)
		_ = json.Unmarshal([]byte(outcomesJSON), &turn.Outcomes)
		_ = json.Unmarshal([]byte(importantJSON), &turn.ImportantPoints)
		_ = json.Unmarshal([]byte(pendingJSON), &turn.PendingItems)
		_ = json.Unmarshal([]byte(toolNamesJSON), &turn.ToolNames)
		_ = json.Unmarshal([]byte(journalJSON), &turn.LinkedJournalIDs)
		_ = json.Unmarshal([]byte(noteJSON), &turn.LinkedNoteIDs)
		_ = json.Unmarshal([]byte(memoryJSON), &turn.LinkedMemoryIDs)
		out = append(out, turn)
	}
	return out, rows.Err()
}

func buildActivityDaySummary(date string, turns []ActivityTurn, journalEntries []JournalEntry, episodicEntries []EpisodicMemory, pending []string) string {
	parts := make([]string, 0, 4)
	if len(turns) > 0 {
		parts = append(parts, fmt.Sprintf("%s had %d recorded activity turns", date, len(turns)))
		topIntent := turns[0].Intent
		if topIntent != "" {
			parts = append(parts, "main focus: "+topIntent)
		}
	}
	if len(journalEntries) > 0 {
		parts = append(parts, fmt.Sprintf("%d journal highlights were logged", len(journalEntries)))
	}
	if len(episodicEntries) > 0 {
		parts = append(parts, fmt.Sprintf("%d episodic events were captured", len(episodicEntries)))
	}
	if len(pending) > 0 {
		parts = append(parts, fmt.Sprintf("%d open items remained relevant", len(uniqueNonEmptySlice(pending, 20))))
	}
	return strings.Join(parts, "; ")
}

func buildRecentActivityOverviewSummary(days int, rollups []DailyActivityRollup, pending []string, highlights []string) string {
	if len(rollups) == 0 && len(pending) == 0 && len(highlights) == 0 {
		return ""
	}
	parts := []string{fmt.Sprintf("Last %d days overview", days)}
	if len(rollups) > 0 {
		parts = append(parts, fmt.Sprintf("%d day summaries available", len(rollups)))
	}
	if len(highlights) > 0 {
		parts = append(parts, "key highlights: "+strings.Join(uniqueNonEmptySlice(highlights, 3), "; "))
	}
	if len(pending) > 0 {
		parts = append(parts, fmt.Sprintf("%d open items need attention", len(uniqueNonEmptySlice(pending, 20))))
	}
	return strings.Join(parts, ". ")
}

func truncateForActivity(text string, maxLen int) string {
	text = strings.TrimSpace(text)
	if maxLen <= 0 || len(text) <= maxLen {
		return text
	}
	if maxLen <= 1 {
		return text[:maxLen]
	}
	return strings.TrimSpace(text[:maxLen-1]) + "…"
}

func uniqueNonEmptySlice(in []string, limit int) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, item := range in {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func normalizeStringSlice(in []string, limit int, maxItemLen int) []string {
	out := make([]string, 0, len(in))
	for _, item := range uniqueNonEmptySlice(in, limit) {
		out = append(out, truncateForActivity(item, maxItemLen))
	}
	return out
}
