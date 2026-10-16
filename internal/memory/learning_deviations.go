package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"aurago/internal/security"
)

func (s *SQLiteMemory) RecordLearningDeviation(ctx context.Context, run, hash, reason string, revisions map[string]int) error {
	if run == "" || hash == "" || len(reason) == 0 || len(reason) > 1000 {
		return fmt.Errorf("invalid learning deviation")
	}
	if security.Scrub(reason) != reason {
		return fmt.Errorf("learning deviation contains sensitive content")
	}
	b, err := json.Marshal(revisions)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO memory_learning_deviations(run,action_hash,reason,revisions,created_at) VALUES(?,?,?,?,?) ON CONFLICT(run,action_hash) DO NOTHING`, run, hash, reason, string(b), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("record learning deviation: %w", err)
	}
	if n, _ := result.RowsAffected(); n > 0 {
		return s.RecordLearningMetric(ctx, "overrides", 1)
	}
	return nil
}
