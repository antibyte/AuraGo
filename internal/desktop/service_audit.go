package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Audit records one desktop operation.
func (s *Service) Audit(ctx context.Context, action, target string, details interface{}, source string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	if strings.TrimSpace(source) == "" {
		source = SourceUser
	}
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("marshal desktop audit details: %w", err)
	}
	db := s.getDB()
	_, err = db.ExecContext(ctx, `INSERT INTO desktop_audit(action, target, source, details_json, created_at)
		VALUES(?, ?, ?, ?, ?)`, action, target, source, string(detailsJSON), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("write desktop audit: %w", err)
	}
	return nil
}
