package memory

import (
	"context"
	"encoding/json"
	"strings"
)

// ListLearningsForAdmin is exclusively for authenticated human management, never model retrieval.
func (s *SQLiteMemory) ListLearningsForAdmin(ctx context.Context, project, query, status string) ([]Learning, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM memory_learnings WHERE (?='' OR (?='global' AND project='') OR project=?) AND (?='' OR status=?) ORDER BY rowid DESC`, project, project, project, status, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Learning{}
	for rows.Next() {
		var b string
		var l Learning
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(b), &l); err != nil {
			return nil, err
		}
		if query != "" && !strings.Contains(strings.ToLower(l.Statement+" "+l.Rationale), strings.ToLower(query)) {
			continue
		}
		out = append(out, l)
		if len(out) == 200 {
			break
		}
	}
	return out, rows.Err()
}

func (s *SQLiteMemory) LearningForAdmin(ctx context.Context, id string) (Learning, error) {
	return readLearning(ctx, s.db, id)
}
