package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// AuditRequestInfo stores HTTP attribution for request-originated desktop audit events.
type AuditRequestInfo struct {
	ClientIP    string
	SessionHash string
	UserAgent   string
}

// Audit records one desktop operation.
func (s *Service) Audit(ctx context.Context, action, target string, details interface{}, source string) error {
	return s.AuditWithRequest(ctx, action, target, details, source, AuditRequestInfo{})
}

// AuditWithRequest records one desktop operation with optional HTTP request attribution.
func (s *Service) AuditWithRequest(ctx context.Context, action, target string, details interface{}, source string, requestInfo AuditRequestInfo) error {
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
	_, err = db.ExecContext(ctx, `INSERT INTO desktop_audit(action, target, source, details_json, client_ip, session_hash, user_agent, created_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		action,
		target,
		source,
		string(detailsJSON),
		strings.TrimSpace(requestInfo.ClientIP),
		strings.TrimSpace(requestInfo.SessionHash),
		strings.TrimSpace(requestInfo.UserAgent),
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("write desktop audit: %w", err)
	}
	return nil
}
