package memory

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const maintenanceCompatibilityTimeout = 30 * time.Second

func maintenanceCompatibilityContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), maintenanceCompatibilityTimeout)
}

// GetMemoryMaintenanceState returns a persisted maintenance value. Missing
// keys are represented by an empty value so first-run maintenance starts at
// the beginning of its keyset.
func (s *SQLiteMemory) GetMemoryMaintenanceState(key string) (string, error) {
	ctx, cancel := maintenanceCompatibilityContext()
	defer cancel()
	return s.GetMemoryMaintenanceStateContext(ctx, key)
}

// GetMemoryMaintenanceStateContext returns a persisted maintenance value with
// cancellation support. Missing keys are represented by an empty value.
func (s *SQLiteMemory) GetMemoryMaintenanceStateContext(ctx context.Context, key string) (string, error) {
	if s == nil || s.db == nil {
		return "", fmt.Errorf("memory maintenance state store is unavailable")
	}
	ctx = maintenanceContext(ctx)
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("memory maintenance state key is required")
	}
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM memory_maintenance_meta WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get memory maintenance state %s: %w", key, err)
	}
	return value, nil
}

// SetMemoryMaintenanceState upserts a persisted maintenance value.
func (s *SQLiteMemory) SetMemoryMaintenanceState(key, value string) error {
	ctx, cancel := maintenanceCompatibilityContext()
	defer cancel()
	return s.SetMemoryMaintenanceStateContext(ctx, key, value)
}

// SetMemoryMaintenanceStateContext upserts a persisted maintenance value.
func (s *SQLiteMemory) SetMemoryMaintenanceStateContext(ctx context.Context, key, value string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("memory maintenance state store is unavailable")
	}
	ctx = maintenanceContext(ctx)
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("memory maintenance state key is required")
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO memory_maintenance_meta (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value); err != nil {
		return fmt.Errorf("set memory maintenance state %s: %w", key, err)
	}
	return nil
}

// ClearMemoryMaintenanceState removes a persisted maintenance value.
func (s *SQLiteMemory) ClearMemoryMaintenanceState(key string) error {
	ctx, cancel := maintenanceCompatibilityContext()
	defer cancel()
	return s.ClearMemoryMaintenanceStateContext(ctx, key)
}

// ClearMemoryMaintenanceStateContext removes a persisted maintenance value.
func (s *SQLiteMemory) ClearMemoryMaintenanceStateContext(ctx context.Context, key string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("memory maintenance state store is unavailable")
	}
	ctx = maintenanceContext(ctx)
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("memory maintenance state key is required")
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM memory_maintenance_meta WHERE key = ?`, key); err != nil {
		return fmt.Errorf("clear memory maintenance state %s: %w", key, err)
	}
	return nil
}

// GetMemoryMetaAfterContext retrieves bounded metadata in stable doc_id order.
func (s *SQLiteMemory) GetMemoryMetaAfterContext(ctx context.Context, cursor string, limit int) ([]MemoryMeta, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("memory metadata store is unavailable")
	}
	ctx = maintenanceContext(ctx)
	if limit <= 0 {
		limit = 500
	}
	query := `SELECT ` + memoryMetaSelectColumns + ` FROM memory_meta ORDER BY doc_id ASC LIMIT ?`
	args := []any{limit}
	if cursor != "" {
		query = `SELECT ` + memoryMetaSelectColumns + ` FROM memory_meta WHERE doc_id > ? ORDER BY doc_id ASC LIMIT ?`
		args = []any{cursor, limit}
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	metas := make([]MemoryMeta, 0, limit)
	for rows.Next() {
		var meta MemoryMeta
		if err := rows.Scan(
			&meta.DocID,
			&meta.AccessCount,
			&meta.LastAccessed,
			&meta.LastEventAt,
			&meta.ExtractionConfidence,
			&meta.VerificationStatus,
			&meta.SourceType,
			&meta.SourceReliability,
			&meta.UsefulCount,
			&meta.UselessCount,
			&meta.LastEffectivenessAt,
			&meta.Protected,
			&meta.KeepForever,
			&meta.ArchivedAt,
			&meta.ArchivedReason,
			&meta.LastReviewedAt,
			&meta.ReviewNote,
			&meta.CurationRevision,
		); err != nil {
			return nil, err
		}
		metas = append(metas, meta)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return metas, nil
}
