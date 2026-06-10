package memory

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"aurago/internal/security"
)

// CompressedToolOutput stores the original and compressed form of a tool result
// so the LLM can retrieve the original on demand.
type CompressedToolOutput struct {
	ID                int64
	SessionID         string
	ToolCallID        string
	ToolName          string
	OriginalContent   string
	CompressedContent string
	CompressionRatio  float64
	FilterUsed        string
	CreatedAt         time.Time
	AccessedAt        *time.Time
	AccessCount       int
}

// InitCompressedOutputTable creates the compressed_tool_outputs table if it does not exist.
func (s *SQLiteMemory) InitCompressedOutputTable() error {
	schema := `
	CREATE TABLE IF NOT EXISTS compressed_tool_outputs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		tool_call_id TEXT NOT NULL,
		tool_name TEXT NOT NULL,
		original_content TEXT NOT NULL,
		compressed_content TEXT NOT NULL,
		compression_ratio REAL,
		filter_used TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		accessed_at DATETIME,
		access_count INTEGER DEFAULT 0,
		UNIQUE(session_id, tool_call_id)
	);
	CREATE INDEX IF NOT EXISTS idx_cto_session ON compressed_tool_outputs(session_id);
	CREATE INDEX IF NOT EXISTS idx_cto_created ON compressed_tool_outputs(created_at);`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("compressed_tool_outputs schema: %w", err)
	}
	return nil
}

// StoreCompressedOutput archives a tool output. The original content is scrubbed
// for sensitive values before persistence.
func (s *SQLiteMemory) StoreCompressedOutput(ctx context.Context, out *CompressedToolOutput) error {
	if out == nil {
		return nil
	}
	// Scrub secrets from the original before archiving.
	original := security.Scrub(out.OriginalContent)
	original = security.RedactSensitiveInfo(original)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO compressed_tool_outputs
			(session_id, tool_call_id, tool_name, original_content, compressed_content, compression_ratio, filter_used)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id, tool_call_id) DO UPDATE SET
			original_content = excluded.original_content,
			compressed_content = excluded.compressed_content,
			compression_ratio = excluded.compression_ratio,
			filter_used = excluded.filter_used,
			created_at = excluded.created_at,
			accessed_at = NULL,
			access_count = 0`,
		out.SessionID,
		out.ToolCallID,
		out.ToolName,
		original,
		out.CompressedContent,
		out.CompressionRatio,
		out.FilterUsed,
	)
	if err != nil {
		return fmt.Errorf("store compressed output: %w", err)
	}
	return nil
}

// RetrieveCompressedOutput fetches a previously archived original by tool_call_id.
func (s *SQLiteMemory) RetrieveCompressedOutput(ctx context.Context, sessionID, toolCallID string) (*CompressedToolOutput, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, session_id, tool_call_id, tool_name, original_content, compressed_content,
			compression_ratio, filter_used, created_at, accessed_at, access_count
		FROM compressed_tool_outputs
		WHERE session_id = ? AND tool_call_id = ?`,
		sessionID, toolCallID)

	out := &CompressedToolOutput{}
	var accessed sql.NullTime
	err := row.Scan(
		&out.ID, &out.SessionID, &out.ToolCallID, &out.ToolName,
		&out.OriginalContent, &out.CompressedContent,
		&out.CompressionRatio, &out.FilterUsed,
		&out.CreatedAt, &accessed, &out.AccessCount,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no compressed output found for tool_call_id=%s", toolCallID)
		}
		return nil, fmt.Errorf("retrieve compressed output: %w", err)
	}
	if accessed.Valid {
		out.AccessedAt = &accessed.Time
	}
	return out, nil
}

// MarkCompressedOutputAccessed updates the access tracking for an archived output.
func (s *SQLiteMemory) MarkCompressedOutputAccessed(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE compressed_tool_outputs
		SET access_count = access_count + 1, accessed_at = ?
		WHERE id = ?`,
		time.Now().UTC(), id)
	return err
}

// HasCompressedOutputsForSession returns true if the session has any archived outputs.
func (s *SQLiteMemory) HasCompressedOutputsForSession(ctx context.Context, sessionID string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM compressed_tool_outputs WHERE session_id = ?",
		sessionID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// SetCompressedOutputCreatedAt updates created_at for an archived tool output.
func (s *SQLiteMemory) SetCompressedOutputCreatedAt(ctx context.Context, sessionID, toolCallID string, createdAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE compressed_tool_outputs SET created_at = ? WHERE session_id = ? AND tool_call_id = ?`,
		createdAt.UTC(), sessionID, toolCallID)
	if err != nil {
		return fmt.Errorf("set compressed output created_at: %w", err)
	}
	return nil
}

// CleanupCompressedOutputs removes archived outputs older than maxAge.
func (s *SQLiteMemory) CleanupCompressedOutputs(ctx context.Context, maxAge time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-maxAge)
	res, err := s.db.ExecContext(ctx,
		"DELETE FROM compressed_tool_outputs WHERE created_at < ?",
		cutoff)
	if err != nil {
		return 0, fmt.Errorf("cleanup compressed outputs: %w", err)
	}
	return res.RowsAffected()
}
