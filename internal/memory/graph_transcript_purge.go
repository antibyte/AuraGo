package memory

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

// ScrubSessionTranscriptEvidence removes captured conversation copies, preserving derived claims.
func (kg *KnowledgeGraph) ScrubSessionTranscriptEvidence(ctx context.Context, sessionID string) error {
	if err := kg.ensureGraphPrivacyScrubReceipts(ctx); err != nil {
		return err
	}
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
	var nodeIDsJSON, edgeIDsJSON string
	if receiptErr := tx.QueryRowContext(ctx, `SELECT node_ids,edge_ids FROM privacy_scrub_index_pending WHERE session_marker=?`, marker).Scan(&nodeIDsJSON, &edgeIDsJSON); receiptErr != nil && !errors.Is(receiptErr, sql.ErrNoRows) {
		return fmt.Errorf("read graph privacy index cleanup receipt: %w", receiptErr)
	}
	nodeIDs, edgeIDs := readStringSet(nodeIDsJSON), readStringSet(edgeIDsJSON)
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
	stamp := formatKGTimestamp(time.Now())
	for _, update := range updates {
		if update.table == "kg_nodes" {
			nodeIDs[update.a] = struct{}{}
		} else {
			edgeIDs[graphPrivacyEdgeID(update.a, update.b, update.c)] = struct{}{}
		}
		if _, err = tx.ExecContext(ctx, `UPDATE `+update.table+` SET properties=?,updated_at=?,semantic_indexed_at=NULL WHERE rowid=?`, update.properties, stamp, update.row); err != nil {
			return fmt.Errorf("anonymize graph source properties: %w", err)
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE kg_evidence SET raw_text='',source_message_id='',session_id='',channel='',actor='',source_uri='',content_hash='' WHERE session_id=?`, sessionID); err != nil {
		return fmt.Errorf("purge graph transcript evidence: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE kg_claims SET session_id=?,source_message_id='' WHERE session_id=?`, marker, sessionID); err != nil {
		return fmt.Errorf("anonymize graph claim source: %w", err)
	}
	nodeIDsJSON, edgeIDsJSON = encodeStringSet(nodeIDs), encodeStringSet(edgeIDs)
	if _, err := tx.ExecContext(ctx, `INSERT INTO privacy_scrub_index_pending(session_marker,node_ids,edge_ids,updated_at) VALUES(?,?,?,?)
		ON CONFLICT(session_marker) DO UPDATE SET node_ids=excluded.node_ids,edge_ids=excluded.edge_ids,updated_at=excluded.updated_at`, marker, nodeIDsJSON, edgeIDsJSON, stamp); err != nil {
		return fmt.Errorf("persist graph privacy index cleanup receipt: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit graph transcript purge: %w", err)
	}
	for id := range nodeIDs {
		err = errors.Join(err, kg.removeSemanticNodeIndex(id))
	}
	for encoded := range edgeIDs {
		var identity []string
		if json.Unmarshal([]byte(encoded), &identity) != nil || len(identity) != 3 {
			err = errors.Join(err, fmt.Errorf("invalid graph privacy edge receipt"))
			continue
		}
		err = errors.Join(err, kg.removeSemanticEdgeIndex(identity[0], identity[1], identity[2]))
	}
	if err != nil {
		return err
	}
	if _, err := kg.db.ExecContext(ctx, `DELETE FROM privacy_scrub_index_pending WHERE session_marker=?`, marker); err != nil {
		return fmt.Errorf("clear graph privacy index cleanup receipt: %w", err)
	}
	return nil
}

func (kg *KnowledgeGraph) ensureGraphPrivacyScrubReceipts(ctx context.Context) error {
	var exists bool
	if err := kg.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='privacy_scrub_index_pending')`).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		var populated bool
		if err := kg.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM kg_nodes) OR EXISTS(SELECT 1 FROM kg_edges) OR EXISTS(SELECT 1 FROM kg_claims)`).Scan(&populated); err != nil {
			return err
		}
		if populated {
			if _, err := backupMainDatabase(kg.db, "privacy-scrub-receipts-v1"); err != nil {
				return fmt.Errorf("back up graph before privacy scrub receipt migration: %w", err)
			}
		}
	}
	_, err := kg.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS privacy_scrub_index_pending(session_marker TEXT PRIMARY KEY,node_ids TEXT NOT NULL,edge_ids TEXT NOT NULL,updated_at TEXT NOT NULL)`)
	return err
}

func readStringSet(encoded string) map[string]struct{} {
	var values []string
	_ = json.Unmarshal([]byte(encoded), &values)
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func encodeStringSet(values map[string]struct{}) string {
	ordered := make([]string, 0, len(values))
	for value := range values {
		ordered = append(ordered, value)
	}
	sort.Strings(ordered)
	encoded, _ := json.Marshal(ordered)
	return string(encoded)
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
