package memory

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ScrubSessionTranscriptEvidence removes captured conversation copies, preserving derived claims.
func (kg *KnowledgeGraph) ScrubSessionTranscriptEvidence(ctx context.Context, sessionID string) error {
	tx, err := kg.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin graph transcript purge: %w", err)
	}
	defer tx.Rollback()
	type sourceUpdate struct {
		table               string
		row                 int64
		a, b, c, properties string
		protected           bool
		values              map[string]string
	}
	var updates []sourceUpdate
	marker := fmt.Sprintf("deleted:%x", sha256.Sum256([]byte(sessionID)))
	for _, table := range []string{"kg_nodes", "kg_edges"} {
		columns := "rowid,id,label,'',properties,protected"
		if table == "kg_edges" {
			columns = "id,source,target,relation,properties,0"
		}
		rows, queryErr := tx.QueryContext(ctx, `SELECT `+columns+` FROM `+table+` WHERE instr(properties,'"session_id"')>0 OR instr(properties,'"source_sessions"')>0`)
		if queryErr != nil {
			return fmt.Errorf("read graph source properties: %w", queryErr)
		}
		for rows.Next() {
			var update sourceUpdate
			update.table = table
			var properties string
			if err = rows.Scan(&update.row, &update.a, &update.b, &update.c, &properties, &update.protected); err != nil {
				break
			}
			var changed bool
			update.properties, update.values, changed, err = scrubGraphSessionProperties(properties, sessionID, marker)
			if err != nil {
				break
			}
			if changed {
				updates = append(updates, update)
			}
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
		if err != nil {
			return fmt.Errorf("scan graph source properties: %w", err)
		}
	}
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	for _, update := range updates {
		if _, err = tx.ExecContext(ctx, `UPDATE `+update.table+` SET properties=?,updated_at=?,semantic_indexed_at=NULL WHERE rowid=?`, update.properties, stamp, update.row); err != nil {
			return fmt.Errorf("anonymize graph source properties: %w", err)
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE kg_evidence SET raw_text='',source_message_id='',session_id='',channel='',actor='',source_uri='' WHERE session_id=?`, sessionID); err != nil {
		return fmt.Errorf("purge graph transcript evidence: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE kg_claims SET session_id='',source_message_id='' WHERE session_id=?`, sessionID); err != nil {
		return fmt.Errorf("anonymize graph claim source: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit graph transcript purge: %w", err)
	}
	for _, update := range updates {
		var indexErr error
		if update.table == "kg_nodes" {
			indexErr = kg.removeSemanticNodeIndex(update.a)
			if indexErr == nil {
				indexErr = kg.upsertSemanticNodeIndexContext(ctx, Node{ID: update.a, Label: update.b, Properties: update.values, Protected: update.protected})
			}
		} else {
			indexErr = kg.removeSemanticEdgeIndex(update.a, update.b, update.c)
			if indexErr == nil {
				indexErr = kg.upsertSemanticEdgeIndexContext(ctx, Edge{Source: update.a, Target: update.b, Relation: update.c, Properties: update.values})
			}
		}
		if indexErr == nil && kg.semanticIndex() != nil {
			query := `UPDATE ` + update.table + ` SET semantic_indexed_at=? WHERE rowid=? AND properties=? AND updated_at=?`
			args := []any{stamp, update.row, update.properties, stamp}
			if update.table == "kg_nodes" {
				query += ` AND label=? AND protected=?`
				args = append(args, update.b, boolToInt(update.protected))
			}
			result, writeErr := kg.db.ExecContext(ctx, query, args...)
			indexErr = writeErr
			if indexErr == nil {
				count, countErr := result.RowsAffected()
				indexErr = countErr
				if count != 1 && indexErr == nil {
					indexErr = fmt.Errorf("graph source changed during privacy reindex")
				}
			}
			if indexErr != nil {
				if update.table == "kg_nodes" {
					kg.markSemanticNodeDirty(update.a)
				} else {
					kg.markSemanticEdgeDirty(update.a, update.b, update.c)
				}
			}
		}
		err = errors.Join(err, indexErr)
	}
	return err
}

func scrubGraphSessionProperties(payload, session, marker string) (string, map[string]string, bool, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &fields); err != nil {
		return "", nil, false, err
	}
	changed := false
	var single string
	if json.Unmarshal(fields["session_id"], &single) == nil && single == session {
		fields["session_id"], _ = json.Marshal(marker)
		changed = true
	}
	if raw, exists := fields["source_sessions"]; exists {
		var sessions []string
		encoded := raw
		var wrapped string
		if json.Unmarshal(raw, &wrapped) == nil {
			encoded = []byte(wrapped)
		}
		if json.Unmarshal(encoded, &sessions) == nil {
			matched := false
			for i, value := range sessions {
				if value == session {
					sessions[i] = marker
					matched = true
				}
			}
			if matched {
				value, _ := json.Marshal(sessions)
				fields["source_sessions"], _ = json.Marshal(string(value))
				changed = true
			}
		}
	}
	if !changed {
		return payload, nil, false, nil
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return "", nil, false, err
	}
	values := make(map[string]string, len(fields))
	for key, raw := range fields {
		var value string
		if json.Unmarshal(raw, &value) != nil {
			value = string(raw)
		}
		values[key] = value
	}
	return string(encoded), values, true, nil
}
