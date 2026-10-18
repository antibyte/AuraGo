package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

func (s *SQLiteMemory) normalizeLegacyPendingMemoryWrites() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin legacy queue normalization: %w", err)
	}
	defer tx.Rollback()
	if err := normalizeLegacyPendingMemoryWritesTx(tx, ""); err != nil {
		return err
	}
	return tx.Commit()
}

// Only the strict, complete analysis envelope proves ownership of legacy queue rows.
func normalizeLegacyPendingMemoryWritesTx(tx *sql.Tx, purgeSession string) error {
	rows, err := tx.Query(`SELECT id,content FROM pending_memory_writes WHERE metadata_json IS NULL`)
	if err != nil {
		return fmt.Errorf("read legacy pending memory sources: %w", err)
	}
	type source struct {
		id      int64
		session string
	}
	var known []source
	for rows.Next() {
		var id int64
		var content string
		if err = rows.Scan(&id, &content); err != nil {
			break
		}
		if session, ok := MemoryAnalysisSession(content); ok && session != "" && (purgeSession == "" || purgeSession == session) {
			known = append(known, source{id, session})
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return fmt.Errorf("scan legacy pending memory sources: %w", err)
	}
	var fences bool
	if err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='deleted_chat_sessions')`).Scan(&fences); err != nil {
		return err
	}
	for _, entry := range known {
		deleted := purgeSession != ""
		if !deleted && fences {
			if err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM deleted_chat_sessions WHERE session_id=?)`, entry.session).Scan(&deleted); err != nil {
				return err
			}
		}
		if deleted {
			_, err = tx.Exec(`DELETE FROM pending_memory_writes WHERE id=? AND metadata_json IS NULL`, entry.id)
		} else {
			encoded, encodeErr := json.Marshal(pendingMemoryWriteMetadata{Version: 1, Details: &MemoryMetaUpdate{SourceType: "memory_analysis", SourceSessionID: entry.session, SourceRole: "unknown"}})
			if encodeErr != nil {
				return encodeErr
			}
			_, err = tx.Exec(`UPDATE pending_memory_writes SET metadata_json=? WHERE id=? AND metadata_json IS NULL`, string(encoded), entry.id)
		}
		if err != nil {
			return fmt.Errorf("normalize legacy pending memory source: %w", err)
		}
	}
	return nil
}
