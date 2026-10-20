package memory

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ConsolidationCheckpointKey binds resumable work to one immutable source row,
// its provenance, the extraction prompt contract, and the selected model.
type ConsolidationCheckpointKey struct {
	MessageID     int64
	SourceHash    string
	PromptVersion string
	ModelID       string
}

type ConsolidationCheckpoint struct {
	PartCount     int
	State         string
	PublishCursor int
}

type ConsolidationPartResult struct {
	PartIndex int
	FactsJSON string
}

// ConsolidationSourceHash binds checkpoints to exact archived content and its
// provenance without persisting another copy of the source in maintenance state.
func ConsolidationSourceHash(message ArchivedMessage) string {
	identity := struct {
		ID        int64  `json:"id"`
		SessionID string `json:"session_id"`
		Role      string `json:"role"`
		Content   string `json:"content"`
		Timestamp string `json:"timestamp"`
	}{message.ID, message.SessionID, message.Role, message.Content, message.Timestamp}
	encoded, _ := json.Marshal(identity)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func (s *SQLiteMemory) EnsureConsolidationCheckpoint(ctx context.Context, key ConsolidationCheckpointKey, partCount int) (ConsolidationCheckpoint, error) {
	if err := validateConsolidationCheckpointKey(key); err != nil {
		return ConsolidationCheckpoint{}, err
	}
	if partCount < 0 {
		return ConsolidationCheckpoint{}, fmt.Errorf("consolidation part count cannot be negative")
	}
	ctx = maintenanceContext(ctx)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ConsolidationCheckpoint{}, fmt.Errorf("begin consolidation checkpoint: %w", err)
	}
	defer tx.Rollback()
	if err := verifyConsolidationSourceFence(ctx, tx, key); err != nil {
		return ConsolidationCheckpoint{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM memory_consolidation_parts WHERE message_id=? AND NOT (source_hash=? AND prompt_version=? AND model_id=?)`, key.MessageID, key.SourceHash, key.PromptVersion, key.ModelID); err != nil {
		return ConsolidationCheckpoint{}, fmt.Errorf("remove stale consolidation parts: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM memory_consolidation_checkpoints WHERE message_id=? AND NOT (source_hash=? AND prompt_version=? AND model_id=?)`, key.MessageID, key.SourceHash, key.PromptVersion, key.ModelID); err != nil {
		return ConsolidationCheckpoint{}, fmt.Errorf("remove stale consolidation checkpoints: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO memory_consolidation_checkpoints
			(message_id, source_hash, prompt_version, model_id, part_count)
		VALUES (?, ?, ?, ?, ?)`, key.MessageID, key.SourceHash, key.PromptVersion, key.ModelID, partCount); err != nil {
		return ConsolidationCheckpoint{}, fmt.Errorf("create consolidation checkpoint: %w", err)
	}
	var checkpoint ConsolidationCheckpoint
	if err := tx.QueryRowContext(ctx, `
		SELECT part_count, state, publish_cursor
		FROM memory_consolidation_checkpoints
		WHERE message_id=? AND source_hash=? AND prompt_version=? AND model_id=?`,
		key.MessageID, key.SourceHash, key.PromptVersion, key.ModelID,
	).Scan(&checkpoint.PartCount, &checkpoint.State, &checkpoint.PublishCursor); err != nil {
		return ConsolidationCheckpoint{}, fmt.Errorf("read consolidation checkpoint: %w", err)
	}
	if checkpoint.PartCount != partCount {
		return ConsolidationCheckpoint{}, fmt.Errorf("consolidation checkpoint part count changed without prompt version change")
	}
	if err := tx.Commit(); err != nil {
		return ConsolidationCheckpoint{}, fmt.Errorf("commit consolidation checkpoint: %w", err)
	}
	return checkpoint, nil
}

func (s *SQLiteMemory) ListConsolidationPartResults(ctx context.Context, key ConsolidationCheckpointKey) ([]ConsolidationPartResult, error) {
	if err := validateConsolidationCheckpointKey(key); err != nil {
		return nil, err
	}
	ctx = maintenanceContext(ctx)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin consolidation part result read: %w", err)
	}
	defer tx.Rollback()
	if err := verifyConsolidationSourceFence(ctx, tx, key); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT part_index, facts_json
		FROM memory_consolidation_parts
		WHERE message_id=? AND source_hash=? AND prompt_version=? AND model_id=?
		ORDER BY part_index`, key.MessageID, key.SourceHash, key.PromptVersion, key.ModelID)
	if err != nil {
		return nil, fmt.Errorf("list consolidation part results: %w", err)
	}
	defer rows.Close()
	results := make([]ConsolidationPartResult, 0)
	for rows.Next() {
		var result ConsolidationPartResult
		if err := rows.Scan(&result.PartIndex, &result.FactsJSON); err != nil {
			return nil, fmt.Errorf("scan consolidation part result: %w", err)
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate consolidation part results: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close consolidation part results: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit consolidation part result read: %w", err)
	}
	return results, nil
}

// SaveConsolidationPartResult only inserts a validated result. The first
// durable result wins, so a retry cannot silently replace already reviewed work.
func (s *SQLiteMemory) SaveConsolidationPartResult(ctx context.Context, key ConsolidationCheckpointKey, partIndex int, factsJSON string) error {
	if err := validateConsolidationCheckpointKey(key); err != nil {
		return err
	}
	if partIndex < 0 || strings.TrimSpace(factsJSON) == "" {
		return fmt.Errorf("consolidation part result is incomplete")
	}
	ctx = maintenanceContext(ctx)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin consolidation part save: %w", err)
	}
	defer tx.Rollback()
	if err := verifyConsolidationSourceFence(ctx, tx, key); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO memory_consolidation_parts
			(message_id, source_hash, prompt_version, model_id, part_index, facts_json)
		SELECT ?, ?, ?, ?, ?, ?
		WHERE EXISTS (
			SELECT 1 FROM memory_consolidation_checkpoints
			WHERE message_id=? AND source_hash=? AND prompt_version=? AND model_id=? AND part_count>?
		)`,
		key.MessageID, key.SourceHash, key.PromptVersion, key.ModelID, partIndex, factsJSON,
		key.MessageID, key.SourceHash, key.PromptVersion, key.ModelID, partIndex,
	)
	if err != nil {
		return fmt.Errorf("save consolidation part result: %w", err)
	}
	if changed, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("confirm consolidation part result: %w", err)
	} else if changed == 0 {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM memory_consolidation_parts WHERE message_id=? AND source_hash=? AND prompt_version=? AND model_id=? AND part_index=?)`, key.MessageID, key.SourceHash, key.PromptVersion, key.ModelID, partIndex).Scan(&exists); err != nil {
			return fmt.Errorf("verify consolidation part result: %w", err)
		}
		if !exists {
			return fmt.Errorf("consolidation checkpoint is missing or part index is out of range")
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit consolidation part save: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) MarkConsolidationCheckpointReady(ctx context.Context, key ConsolidationCheckpointKey) error {
	if err := validateConsolidationCheckpointKey(key); err != nil {
		return err
	}
	ctx = maintenanceContext(ctx)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin consolidation ready transition: %w", err)
	}
	defer tx.Rollback()
	if err := verifyConsolidationSourceFence(ctx, tx, key); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE memory_consolidation_checkpoints
		SET state=CASE WHEN state='publishing' THEN state ELSE 'ready' END, updated_at=CURRENT_TIMESTAMP
		WHERE message_id=? AND source_hash=? AND prompt_version=? AND model_id=?
		  AND (SELECT COUNT(*) FROM memory_consolidation_parts p
		       WHERE p.message_id=memory_consolidation_checkpoints.message_id
		         AND p.source_hash=memory_consolidation_checkpoints.source_hash
		         AND p.prompt_version=memory_consolidation_checkpoints.prompt_version
		         AND p.model_id=memory_consolidation_checkpoints.model_id)=part_count`,
		key.MessageID, key.SourceHash, key.PromptVersion, key.ModelID)
	if err != nil {
		return fmt.Errorf("mark consolidation checkpoint ready: %w", err)
	}
	if count, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("confirm consolidation checkpoint ready: %w", err)
	} else if count == 0 {
		return fmt.Errorf("consolidation checkpoint is incomplete")
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit consolidation ready transition: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) AdvanceConsolidationPublishCursor(ctx context.Context, key ConsolidationCheckpointKey, expected int) (bool, error) {
	if err := validateConsolidationCheckpointKey(key); err != nil {
		return false, err
	}
	if expected < 0 {
		return false, fmt.Errorf("consolidation publish cursor cannot be negative")
	}
	ctx = maintenanceContext(ctx)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin consolidation cursor advance: %w", err)
	}
	defer tx.Rollback()
	if err := verifyConsolidationSourceFence(ctx, tx, key); err != nil {
		return false, err
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE memory_consolidation_checkpoints
		SET state='publishing', publish_cursor=publish_cursor+1, updated_at=CURRENT_TIMESTAMP
		WHERE message_id=? AND source_hash=? AND prompt_version=? AND model_id=?
		  AND state IN ('ready','publishing') AND publish_cursor=?`,
		key.MessageID, key.SourceHash, key.PromptVersion, key.ModelID, expected)
	if err != nil {
		return false, fmt.Errorf("advance consolidation publish cursor: %w", err)
	}
	count, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("confirm consolidation publish cursor: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit consolidation cursor advance: %w", err)
	}
	return count == 1, nil
}

func (s *SQLiteMemory) FinalizeConsolidationCheckpoint(ctx context.Context, key ConsolidationCheckpointKey) error {
	if err := validateConsolidationCheckpointKey(key); err != nil {
		return err
	}
	ctx = maintenanceContext(ctx)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin consolidation finalization: %w", err)
	}
	defer tx.Rollback()
	var source ArchivedMessage
	var consolidated bool
	err = tx.QueryRowContext(ctx, `
		SELECT id,session_id,role,content,original_timestamp,consolidated,consolidation_status,consolidation_retries
		FROM archived_messages WHERE id=?`, key.MessageID).Scan(
		&source.ID, &source.SessionID, &source.Role, &source.Content, &source.Timestamp,
		&consolidated, &source.ConsolidationStatus, &source.ConsolidationRetries,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("archived source is no longer available for consolidation")
		}
		return fmt.Errorf("read archived source for finalization: %w", err)
	}
	if ConsolidationSourceHash(source) != key.SourceHash {
		return fmt.Errorf("archived source provenance changed during consolidation")
	}
	if consolidated && source.ConsolidationStatus == "done" {
		// A previous finalization may have committed even if the driver returned
		// an ambiguous commit error. The exact source hash makes this retry safe.
		if _, err := tx.ExecContext(ctx, `DELETE FROM memory_consolidation_checkpoints WHERE message_id=? AND source_hash=? AND prompt_version=? AND model_id=?`, key.MessageID, key.SourceHash, key.PromptVersion, key.ModelID); err != nil {
			return fmt.Errorf("remove completed consolidation checkpoint: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit idempotent consolidation finalization: %w", err)
		}
		return nil
	}
	if consolidated || source.ConsolidationStatus != "in_progress" {
		return fmt.Errorf("archived source is no longer claimed for consolidation")
	}
	var complete bool
	if err := tx.QueryRowContext(ctx, `
		SELECT publish_cursor >= COALESCE((
			SELECT SUM(json_array_length(p.facts_json)) FROM memory_consolidation_parts p
			WHERE p.message_id=c.message_id AND p.source_hash=c.source_hash AND p.prompt_version=c.prompt_version AND p.model_id=c.model_id
		),0)
		FROM memory_consolidation_checkpoints c
		WHERE message_id=? AND source_hash=? AND prompt_version=? AND model_id=? AND state IN ('ready','publishing')`,
		key.MessageID, key.SourceHash, key.PromptVersion, key.ModelID).Scan(&complete); err != nil {
		return fmt.Errorf("verify consolidation publication completion: %w", err)
	}
	if !complete {
		return fmt.Errorf("consolidation publication cursor has not reached the final fact")
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE archived_messages
		SET consolidated=1, consolidation_status='done', consolidation_last_error='', consolidation_claimed_at=NULL
		WHERE id=? AND consolidated=0 AND consolidation_status='in_progress'`, key.MessageID)
	if err != nil {
		return fmt.Errorf("mark archived message consolidated: %w", err)
	}
	if count, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("confirm archived message consolidation: %w", err)
	} else if count == 0 {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM archived_messages WHERE id=? AND consolidated=1 AND consolidation_status='done')`, key.MessageID).Scan(&exists); err != nil {
			return fmt.Errorf("verify archived message completion: %w", err)
		}
		if !exists {
			return fmt.Errorf("archived message is no longer claimed for consolidation")
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM memory_consolidation_checkpoints WHERE message_id=? AND source_hash=? AND prompt_version=? AND model_id=?`, key.MessageID, key.SourceHash, key.PromptVersion, key.ModelID); err != nil {
		return fmt.Errorf("remove completed consolidation checkpoint: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit consolidation finalization: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) CountTerminalConsolidationFailures(ctx context.Context, maxRetries int) (int, error) {
	if maxRetries <= 0 {
		maxRetries = 3
	}
	ctx = maintenanceContext(ctx)
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM archived_messages WHERE consolidated=0 AND consolidation_status='failed' AND consolidation_retries>=?`, maxRetries).Scan(&count); err != nil {
		return 0, fmt.Errorf("count terminal consolidation failures: %w", err)
	}
	return count, nil
}

// RecoverTerminalConsolidationFailures is an explicit operator recovery hook.
// It requeues only the supplied bounded set and leaves durable source checkpoints intact.
func (s *SQLiteMemory) RecoverTerminalConsolidationFailures(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	if len(ids) > 1000 {
		ids = ids[:1000]
	}
	ctx = maintenanceContext(ctx)
	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	query := `UPDATE archived_messages SET consolidation_status='pending', consolidation_retries=0, next_retry_at=CURRENT_TIMESTAMP, consolidation_last_error='', consolidation_claimed_at=NULL WHERE consolidated=0 AND consolidation_status='failed' AND consolidation_retries>=3 AND id IN (` + strings.Join(placeholders, ",") + `)`
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("recover terminal consolidation failures: %w", err)
	}
	return res.RowsAffected()
}

func (s *SQLiteMemory) ClaimConsolidationCandidatesContext(ctx context.Context, limit int, maxRetries int) ([]ArchivedMessage, error) {
	ctx = maintenanceContext(ctx)
	if limit <= 0 {
		limit = 200
	} else if limit > 1000 {
		limit = 1000
	}
	if maxRetries <= 0 {
		maxRetries = 3
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin consolidation claim: %w", err)
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `
		SELECT id FROM archived_messages
		WHERE consolidated=0 AND `+consolidationCandidateEligibilitySQL+`
		  AND (consolidation_status='pending' OR (consolidation_status='failed' AND consolidation_retries<? AND next_retry_at<=CURRENT_TIMESTAMP))
		ORDER BY original_timestamp ASC, id ASC LIMIT ?`, maxRetries, limit)
	if err != nil {
		return nil, fmt.Errorf("select consolidation claim candidates: %w", err)
	}
	ids := make([]int64, 0, limit)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan consolidation claim candidate: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate consolidation claim candidates: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close consolidation claim candidates: %w", err)
	}
	if len(ids) == 0 {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit empty consolidation claim: %w", err)
		}
		return nil, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE archived_messages SET consolidation_status='in_progress', consolidation_claimed_at=CURRENT_TIMESTAMP WHERE consolidated=0 AND id IN (`+strings.Join(placeholders, ",")+`)`, args...); err != nil {
		return nil, fmt.Errorf("claim consolidation rows: %w", err)
	}
	dataRows, err := tx.QueryContext(ctx, `SELECT id,session_id,role,content,original_timestamp,consolidation_status,consolidation_retries FROM archived_messages WHERE id IN (`+strings.Join(placeholders, ",")+`) ORDER BY original_timestamp,id`, args...)
	if err != nil {
		return nil, fmt.Errorf("read claimed consolidation rows: %w", err)
	}
	msgs := make([]ArchivedMessage, 0, len(ids))
	for dataRows.Next() {
		var msg ArchivedMessage
		if err := dataRows.Scan(&msg.ID, &msg.SessionID, &msg.Role, &msg.Content, &msg.Timestamp, &msg.ConsolidationStatus, &msg.ConsolidationRetries); err != nil {
			dataRows.Close()
			return nil, fmt.Errorf("scan claimed consolidation row: %w", err)
		}
		msgs = append(msgs, msg)
	}
	if err := dataRows.Err(); err != nil {
		dataRows.Close()
		return nil, fmt.Errorf("iterate claimed consolidation rows: %w", err)
	}
	if err := dataRows.Close(); err != nil {
		return nil, fmt.Errorf("close claimed consolidation rows: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit consolidation claims: %w", err)
	}
	return msgs, nil
}

func (s *SQLiteMemory) FinalizeIneligibleConsolidationCandidatesContext(ctx context.Context, limit int) (int64, error) {
	ctx = maintenanceContext(ctx)
	if limit <= 0 || limit > 1000 {
		limit = 250
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE archived_messages
		SET consolidated=1, consolidation_status='excluded', consolidation_last_error='', consolidation_claimed_at=NULL
		WHERE id IN (
			SELECT id FROM archived_messages
			WHERE consolidated=0 AND consolidation_status IN ('pending','failed')
			  AND NOT (`+consolidationCandidateEligibilitySQL+`)
			ORDER BY id LIMIT ?
		)`, limit)
	if err != nil {
		return 0, fmt.Errorf("finalize ineligible consolidation candidates: %w", err)
	}
	return res.RowsAffected()
}

func (s *SQLiteMemory) ReclaimStaleConsolidationClaimsContext(ctx context.Context, maxAge time.Duration, limit int) (int64, error) {
	ctx = maintenanceContext(ctx)
	if maxAge <= 0 {
		maxAge = 30 * time.Minute
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	minutes := int(maxAge.Minutes())
	if minutes < 1 {
		minutes = 1
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE archived_messages
		SET consolidation_status='pending', consolidation_claimed_at=NULL,
		    consolidation_last_error=CASE WHEN TRIM(COALESCE(consolidation_last_error,''))='' THEN 'stale_in_progress_reclaimed' ELSE consolidation_last_error END
		WHERE id IN (
			SELECT id FROM archived_messages
			WHERE consolidated=0 AND consolidation_status='in_progress'
			  AND (consolidation_claimed_at IS NULL OR consolidation_claimed_at<datetime('now', ?))
			ORDER BY consolidation_claimed_at, id LIMIT ?
		)`, fmt.Sprintf("-%d minutes", minutes), limit)
	if err != nil {
		return 0, fmt.Errorf("reclaim stale consolidation claims: %w", err)
	}
	return res.RowsAffected()
}

// recoverMemoryMaintenance performs bounded crash recovery once after startup
// migrations. Source-bound checkpoint rows remain intact for the next claim.
func (s *SQLiteMemory) recoverMemoryMaintenance(ctx context.Context) error {
	if s == nil || s.db == nil {
		return nil
	}
	if _, err := s.ReclaimStaleConsolidationClaimsContext(ctx, 30*time.Minute, 1000); err != nil {
		return fmt.Errorf("recover stale memory maintenance claims: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) RecordConsolidationFailureContext(ctx context.Context, ids []int64, reason string) error {
	return s.markConsolidationFailureContext(ctx, ids, reason, true)
}

func (s *SQLiteMemory) DeferConsolidationClaimsContext(ctx context.Context, ids []int64) error {
	return s.markConsolidationFailureContext(ctx, ids, "infrastructure_unavailable", false)
}

func (s *SQLiteMemory) ReleaseConsolidationClaimsContext(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	ctx = maintenanceContext(ctx)
	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	_, err := s.db.ExecContext(ctx, `UPDATE archived_messages SET consolidation_status='pending', consolidation_claimed_at=NULL WHERE consolidated=0 AND consolidation_status='in_progress' AND id IN (`+strings.Join(placeholders, ",")+")", args...)
	if err != nil {
		return fmt.Errorf("release consolidation claims: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) markConsolidationFailureContext(ctx context.Context, ids []int64, reason string, consumeRetry bool) error {
	if len(ids) == 0 {
		return nil
	}
	ctx = maintenanceContext(ctx)
	reason = stableMaintenanceFailureReason(reason)
	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids)+2)
	increment := 0
	if consumeRetry {
		increment = 1
	}
	args = append(args, increment, reason)
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	query := `UPDATE archived_messages SET consolidation_status='failed', consolidation_retries=consolidation_retries+?, consolidation_last_error=?, consolidation_claimed_at=NULL, next_retry_at=datetime('now', '+' || CASE WHEN consolidation_retries<1 THEN 5 WHEN consolidation_retries<2 THEN 30 ELSE 120 END || ' minutes') WHERE consolidated=0 AND id IN (` + strings.Join(placeholders, ",") + `)`
	if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("record consolidation failure: %w", err)
	}
	return nil
}

func validateConsolidationCheckpointKey(key ConsolidationCheckpointKey) error {
	if key.MessageID <= 0 || strings.TrimSpace(key.SourceHash) == "" || strings.TrimSpace(key.PromptVersion) == "" || strings.TrimSpace(key.ModelID) == "" {
		return fmt.Errorf("consolidation checkpoint key is incomplete")
	}
	return nil
}

func verifyConsolidationSourceFence(ctx context.Context, tx *sql.Tx, key ConsolidationCheckpointKey) error {
	var source ArchivedMessage
	err := tx.QueryRowContext(ctx, `
		SELECT id,session_id,role,content,original_timestamp,consolidation_status,consolidation_retries
		FROM archived_messages WHERE id=? AND consolidated=0 AND consolidation_status='in_progress'`, key.MessageID).Scan(
		&source.ID, &source.SessionID, &source.Role, &source.Content, &source.Timestamp, &source.ConsolidationStatus, &source.ConsolidationRetries,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("archived source is no longer claimed for consolidation")
		}
		return fmt.Errorf("read archived source fence: %w", err)
	}
	if ConsolidationSourceHash(source) != key.SourceHash {
		return fmt.Errorf("archived source provenance changed during consolidation")
	}
	return nil
}

func maintenanceContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func stableMaintenanceFailureReason(reason string) string {
	code := strings.ToLower(strings.TrimSpace(reason))
	if code == "" {
		return "extraction_failed"
	}
	code = truncateUTF8Bytes(code, 300)
	// Runtime error strings can contain source data, so persist only a stable code.
	if strings.ContainsAny(code, " \t\r\n") || strings.Contains(code, "@") || strings.Contains(code, "://") {
		return "extraction_failed"
	}
	return code
}
