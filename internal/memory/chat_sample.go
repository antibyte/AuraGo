package memory

import (
	"context"
	"fmt"
)

// SampleVisibleChatText returns a bounded read-only sample, never tool or internal turns.
func (s *SQLiteMemory) SampleVisibleChatText(ctx context.Context) ([]string, error) {
	// ponytail: RANDOM scans chat rows; use indexed sampling if large histories make this measurable.
	rows, err := s.db.QueryContext(ctx, `SELECT session_id, role, content FROM messages
  WHERE is_internal = 0 AND is_tool_output = 0 AND role IN ('user', 'assistant')
  AND session_id NOT IN ('heartbeat', 'space-agent-bridge')
  AND length(content) BETWEEN 12 AND 32768 ORDER BY RANDOM() LIMIT 8`)
	if err != nil {
		return nil, fmt.Errorf("sample visible chat: %w", err)
	}
	defer rows.Close()
	var texts []string
	for rows.Next() {
		var session, role, text string
		if err := rows.Scan(&session, &role, &text); err != nil {
			return nil, fmt.Errorf("read chat sample: %w", err)
		}
		if !ShouldHideAutonomousMessage(session, role, text) {
			texts = append(texts, text)
		}
	}
	return texts, rows.Err()
}
