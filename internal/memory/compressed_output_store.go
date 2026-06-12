package memory

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"aurago/internal/security"
)

// CompressedToolOutput stores the original and compressed form of a tool result
// so the LLM can retrieve the original on demand.
type CompressedToolOutput struct {
	ID                int64
	SessionID         string
	ToolCallID        string
	OutputRef         string
	ToolName          string
	OriginalContent   string
	CompressedContent string
	SummaryContent    string
	ViewContent       string
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
		output_ref TEXT,
		tool_name TEXT NOT NULL,
		original_content TEXT NOT NULL,
		compressed_content TEXT NOT NULL,
		summary_content TEXT DEFAULT '',
		view_content TEXT DEFAULT '',
		compression_ratio REAL,
		filter_used TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		accessed_at DATETIME,
		access_count INTEGER DEFAULT 0,
		UNIQUE(session_id, tool_call_id)
	);
	CREATE INDEX IF NOT EXISTS idx_cto_session ON compressed_tool_outputs(session_id);
	CREATE INDEX IF NOT EXISTS idx_cto_created ON compressed_tool_outputs(created_at);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_cto_session_output_ref ON compressed_tool_outputs(session_id, output_ref)
		WHERE output_ref IS NOT NULL AND output_ref <> '';`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("compressed_tool_outputs schema: %w", err)
	}
	for _, migration := range []struct {
		name string
		ddl  string
	}{
		{name: "output_ref", ddl: "ALTER TABLE compressed_tool_outputs ADD COLUMN output_ref TEXT"},
		{name: "summary_content", ddl: "ALTER TABLE compressed_tool_outputs ADD COLUMN summary_content TEXT DEFAULT ''"},
		{name: "view_content", ddl: "ALTER TABLE compressed_tool_outputs ADD COLUMN view_content TEXT DEFAULT ''"},
	} {
		if err := s.ensureCompressedOutputColumn(migration.name, migration.ddl); err != nil {
			return err
		}
	}
	if _, err := s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_cto_session_output_ref ON compressed_tool_outputs(session_id, output_ref)
		WHERE output_ref IS NOT NULL AND output_ref <> ''`); err != nil {
		return fmt.Errorf("compressed_tool_outputs output_ref index: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) ensureCompressedOutputColumn(name, ddl string) error {
	rows, err := s.db.Query(`PRAGMA table_info(compressed_tool_outputs)`)
	if err != nil {
		return fmt.Errorf("inspect compressed_tool_outputs: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var columnName, columnType string
		var notNull int
		var defaultValue interface{}
		var pk int
		if err := rows.Scan(&cid, &columnName, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("scan compressed_tool_outputs columns: %w", err)
		}
		if columnName == name {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate compressed_tool_outputs columns: %w", err)
	}
	if _, err := s.db.Exec(ddl); err != nil {
		return fmt.Errorf("migrate compressed_tool_outputs add %s: %w", name, err)
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
	summary := security.RedactSensitiveInfo(security.Scrub(out.SummaryContent))
	view := security.RedactSensitiveInfo(security.Scrub(out.ViewContent))
	outputRef := strings.TrimSpace(out.OutputRef)
	if outputRef == "" {
		outputRef = StableToolOutputRef(out.SessionID, out.ToolCallID)
	}
	out.OutputRef = outputRef

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO compressed_tool_outputs
			(session_id, tool_call_id, output_ref, tool_name, original_content, compressed_content, summary_content, view_content, compression_ratio, filter_used)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id, tool_call_id) DO UPDATE SET
			output_ref = excluded.output_ref,
			original_content = excluded.original_content,
			compressed_content = excluded.compressed_content,
			summary_content = excluded.summary_content,
			view_content = excluded.view_content,
			compression_ratio = excluded.compression_ratio,
			filter_used = excluded.filter_used,
			created_at = excluded.created_at,
			accessed_at = NULL,
			access_count = 0`,
		out.SessionID,
		out.ToolCallID,
		outputRef,
		out.ToolName,
		original,
		out.CompressedContent,
		summary,
		view,
		out.CompressionRatio,
		out.FilterUsed,
	)
	if err != nil {
		return fmt.Errorf("store compressed output: %w", err)
	}
	return nil
}

// StableToolOutputRef returns the deterministic ref used to expose archived
// tool outputs to the agent without leaking the raw content into context.
func StableToolOutputRef(sessionID, toolCallID string) string {
	sum := sha256.Sum256([]byte(sessionID + "\x00" + toolCallID))
	return "toolout_" + hex.EncodeToString(sum[:8])
}

// RetrieveCompressedOutput fetches a previously archived original by tool_call_id.
func (s *SQLiteMemory) RetrieveCompressedOutput(ctx context.Context, sessionID, toolCallID string) (*CompressedToolOutput, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, session_id, tool_call_id, output_ref, tool_name, original_content, compressed_content,
			summary_content, view_content,
			compression_ratio, filter_used, created_at, accessed_at, access_count
		FROM compressed_tool_outputs
		WHERE session_id = ? AND tool_call_id = ?`,
		sessionID, toolCallID)

	out := &CompressedToolOutput{}
	var accessed sql.NullTime
	var storedOutputRef sql.NullString
	var summaryContent sql.NullString
	var viewContent sql.NullString
	err := row.Scan(
		&out.ID, &out.SessionID, &out.ToolCallID, &storedOutputRef, &out.ToolName,
		&out.OriginalContent, &out.CompressedContent, &summaryContent, &viewContent,
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
	out.OutputRef = storedOutputRef.String
	out.SummaryContent = summaryContent.String
	out.ViewContent = viewContent.String
	if out.OutputRef == "" {
		out.OutputRef = StableToolOutputRef(out.SessionID, out.ToolCallID)
		_, _ = s.db.ExecContext(ctx,
			`UPDATE compressed_tool_outputs SET output_ref = ? WHERE id = ? AND (output_ref IS NULL OR output_ref = '')`,
			out.OutputRef, out.ID)
	}
	return out, nil
}

// RetrieveCompressedOutputByRef fetches a previously archived original by stable output_ref.
func (s *SQLiteMemory) RetrieveCompressedOutputByRef(ctx context.Context, sessionID, outputRef string) (*CompressedToolOutput, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, session_id, tool_call_id, output_ref, tool_name, original_content, compressed_content,
			summary_content, view_content,
			compression_ratio, filter_used, created_at, accessed_at, access_count
		FROM compressed_tool_outputs
		WHERE session_id = ? AND output_ref = ?`,
		sessionID, outputRef)

	out := &CompressedToolOutput{}
	var accessed sql.NullTime
	var storedOutputRef sql.NullString
	var summaryContent sql.NullString
	var viewContent sql.NullString
	err := row.Scan(
		&out.ID, &out.SessionID, &out.ToolCallID, &storedOutputRef, &out.ToolName,
		&out.OriginalContent, &out.CompressedContent, &summaryContent, &viewContent,
		&out.CompressionRatio, &out.FilterUsed,
		&out.CreatedAt, &accessed, &out.AccessCount,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no compressed output found for output_ref=%s", outputRef)
		}
		return nil, fmt.Errorf("retrieve compressed output by ref: %w", err)
	}
	if accessed.Valid {
		out.AccessedAt = &accessed.Time
	}
	out.OutputRef = storedOutputRef.String
	out.SummaryContent = summaryContent.String
	out.ViewContent = viewContent.String
	if out.OutputRef == "" {
		out.OutputRef = StableToolOutputRef(out.SessionID, out.ToolCallID)
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
