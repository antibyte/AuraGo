package memory

import (
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ConversationEntry is a bounded same-session search result from activity,
// active chat messages, or archived chat messages.
type ConversationEntry struct {
	ID        string `json:"id"`
	Source    string `json:"source"`
	Role      string `json:"role,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
	Content   string `json:"content"`
	score     int
}

// SearchSessionConversationEntries searches every conversation store without
// ever broadening an empty or unknown session into a cross-session query.
func (s *SQLiteMemory) SearchSessionConversationEntries(sessionID, keyword string, limit int) ([]ConversationEntry, error) {
	sessionID = strings.TrimSpace(sessionID)
	terms := activitySearchTerms(keyword)
	if sessionID == "" || len(terms) == 0 {
		return nil, nil
	}
	if limit <= 0 || limit > 4 {
		limit = 4
	}

	activity, err := s.SearchSessionActivityTurns(sessionID, keyword, limit*2)
	if err != nil {
		return nil, err
	}
	entries := make([]ConversationEntry, 0, len(activity)+limit*4)
	for _, turn := range activity {
		content := strings.TrimSpace(strings.Join([]string{
			turn.Intent, turn.UserRequest, turn.UserGoal,
			strings.Join(turn.ActionsTaken, " | "),
			strings.Join(turn.Outcomes, " | "),
			strings.Join(turn.ImportantPoints, " | "),
			strings.Join(turn.PendingItems, " | "),
		}, "\n"))
		entries = append(entries, ConversationEntry{
			ID: "activity:" + strconv.FormatInt(turn.ID, 10), Source: "activity",
			Timestamp: turn.Timestamp, Content: content, score: conversationRelevance(content, terms),
		})
	}

	messageEntries, err := s.searchSessionConversationMessages(sessionID, terms, limit*2)
	if err != nil {
		return nil, err
	}
	entries = append(entries, messageEntries...)
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].score != entries[j].score {
			return entries[i].score > entries[j].score
		}
		return entries[i].Timestamp > entries[j].Timestamp
	})
	if len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}

func (s *SQLiteMemory) searchSessionConversationMessages(sessionID string, terms []string, limit int) ([]ConversationEntry, error) {
	clauses := make([]string, len(terms))
	patterns := make([]any, len(terms))
	for i, term := range terms {
		clauses[i] = `LOWER(content) LIKE ? ESCAPE '\'`
		patterns[i] = "%" + escapeLike(term) + "%"
	}
	where := strings.Join(clauses, " OR ")
	query := fmt.Sprintf(`
		SELECT id, source, role, content, timestamp FROM (
			SELECT id, 'message' AS source, role, substr(content, 1, 4000) AS content,
				CAST(timestamp AS TEXT) AS timestamp
			FROM messages WHERE session_id = ? AND (%s)
			UNION ALL
			SELECT id, 'archive' AS source, role, substr(content, 1, 4000) AS content,
				CAST(COALESCE(original_timestamp, archived_at) AS TEXT) AS timestamp
			FROM archived_messages WHERE session_id = ? AND (%s)
		) ORDER BY timestamp DESC LIMIT ?`, where, where)
	args := make([]any, 0, 2+len(patterns)*2+1)
	args = append(args, sessionID)
	args = append(args, patterns...)
	args = append(args, sessionID)
	args = append(args, patterns...)
	args = append(args, limit*2)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query session conversation messages: %w", err)
	}
	defer rows.Close()

	entries := make([]ConversationEntry, 0, limit*2)
	for rows.Next() {
		var id int64
		var entry ConversationEntry
		if err := rows.Scan(&id, &entry.Source, &entry.Role, &entry.Content, &entry.Timestamp); err != nil {
			return nil, fmt.Errorf("scan session conversation message: %w", err)
		}
		entry.ID = entry.Source + ":" + strconv.FormatInt(id, 10)
		entry.score = conversationRelevance(entry.Content, terms)
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func activitySearchTerms(keyword string) []string {
	seen := make(map[string]struct{})
	terms := make([]string, 0, 12)
	for _, field := range strings.Fields(strings.ToLower(keyword)) {
		term := strings.Trim(field, `.,;:!?()[]{}<>"'`)
		if len([]rune(term)) < 3 {
			continue
		}
		if _, ok := seen[term]; ok {
			continue
		}
		seen[term] = struct{}{}
		terms = append(terms, term)
		if len(terms) == 12 {
			break
		}
	}
	return terms
}

func conversationRelevance(content string, terms []string) int {
	lower := strings.ToLower(content)
	score := 0
	for _, term := range terms {
		if strings.Contains(lower, term) {
			score++
		}
	}
	return score
}

// GetSessionConversationEntry resolves a conversation reference only inside
// the server-bound session.
func (s *SQLiteMemory) GetSessionConversationEntry(sessionID, reference string) (ConversationEntry, error) {
	sessionID = strings.TrimSpace(sessionID)
	kind, rawID, ok := strings.Cut(strings.TrimSpace(reference), ":")
	id, err := strconv.ParseInt(rawID, 10, 64)
	if sessionID == "" || !ok || id <= 0 || err != nil {
		return ConversationEntry{}, sql.ErrNoRows
	}
	if kind == "activity" {
		turn, err := s.GetSessionActivityTurn(sessionID, id)
		if err != nil {
			return ConversationEntry{}, err
		}
		content := strings.TrimSpace(strings.Join([]string{
			turn.Intent, turn.UserRequest, turn.UserGoal,
			strings.Join(turn.ActionsTaken, " | "), strings.Join(turn.Outcomes, " | "),
			strings.Join(turn.ImportantPoints, " | "), strings.Join(turn.PendingItems, " | "),
		}, "\n"))
		return ConversationEntry{ID: reference, Source: kind, Timestamp: turn.Timestamp, Content: content}, nil
	}
	if kind != "message" && kind != "archive" {
		return ConversationEntry{}, sql.ErrNoRows
	}
	table, timestamp := "messages", "timestamp"
	if kind == "archive" {
		table, timestamp = "archived_messages", "COALESCE(original_timestamp, archived_at)"
	}
	query := fmt.Sprintf(`SELECT role, substr(content, 1, 16000), CAST(%s AS TEXT) FROM %s WHERE session_id = ? AND id = ? LIMIT 1`, timestamp, table)
	entry := ConversationEntry{ID: reference, Source: kind}
	if err := s.db.QueryRow(query, sessionID, id).Scan(&entry.Role, &entry.Content, &entry.Timestamp); err != nil {
		return ConversationEntry{}, err
	}
	return entry, nil
}
