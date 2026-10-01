package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"

	"aurago/internal/memory"
	chromem "github.com/philippgille/chromem-go"
)

type mergeJournal struct {
	Version   int    `json:"version"`
	GroupHash string `json:"group_hash"`
	SQLHash   string `json:"sql_hash"`
	State     string `json:"state"`
}
type checkpoint func(stage string) error

func check(hook checkpoint, stage string) error {
	if hook != nil {
		return hook(stage)
	}
	return nil
}

func readVectors(ctx context.Context, group mergeGroup, vectors *chromem.Collection, allowMissing bool) error {
	for _, expected := range group.Before.Documents {
		if err := ctx.Err(); err != nil {
			return err
		}
		actual, err := vectors.GetByID(ctx, expected.ID)
		if allowMissing && expected.ID != group.Canonical && missingVector(err, expected.ID) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read vector %s: %w", expected.ID, err)
		}
		if fingerprint(actual) != fingerprint(expected) {
			return fmt.Errorf("vector %s changed", expected.ID)
		}
	}
	return nil
}
func missingVector(err error, id string) bool {
	return err != nil && err.Error() == fmt.Sprintf("document with ID '%v' not found", id)
}
func writeJournal(ctx context.Context, tx *sql.Tx, key string, state mergeJournal) error {
	encoded, err := json.Marshal(state)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO memory_maintenance_meta(key,value) VALUES (?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, string(encoded))
	return err
}

func mergeActivity(group mergeGroup) (record, error) {
	result := record{}
	for _, meta := range group.Before.Rows["memory_meta"] {
		for _, key := range []string{"access_count", "useful_count", "useless_count"} {
			value, previous := number(meta, key), number(result, key)
			if value < 0 || previous > math.MaxInt64-value {
				return nil, fmt.Errorf("invalid or overflowing activity counter")
			}
			result[key] = previous + value
		}
		for _, key := range []string{"last_accessed", "last_event_at", "last_effectiveness_at"} {
			value := text(meta, key)
			if value == "" {
				continue
			}
			at, err := parseTime(value)
			if err != nil {
				return nil, err
			}
			previous := text(result, key)
			if previous == "" {
				result[key] = value
				continue
			}
			old, err := parseTime(previous)
			if err != nil {
				return nil, err
			}
			if at.After(old) {
				result[key] = value
			}
		}
	}
	return result, nil
}

func transferReferences(ctx context.Context, tx *sql.Tx, group mergeGroup) error {
	activity, err := mergeActivity(group)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE memory_meta SET access_count=?,useful_count=?,useless_count=?,
		last_accessed=?,last_event_at=?,last_effectiveness_at=? WHERE doc_id=?`, activity["access_count"], activity["useful_count"], activity["useless_count"],
		activity["last_accessed"], activity["last_event_at"], activity["last_effectiveness_at"], group.Canonical); err != nil {
		return err
	}
	if err := transferSourceRows(ctx, tx, group); err != nil {
		return err
	}
	if err := transferFailureRows(ctx, tx, group); err != nil {
		return err
	}
	for _, id := range group.IDs {
		if id == group.Canonical {
			continue
		}
		for table, column := range map[string]string{"memory_usage_log": "memory_id", "memory_curation_events": "doc_id"} {
			if _, err := tx.ExecContext(ctx, `UPDATE `+table+` SET `+column+`=? WHERE `+column+`=?`, group.Canonical, id); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE memory_meta SET verification_status='archived',archived_at=CURRENT_TIMESTAMP,
			archived_reason=?,last_event_at=CURRENT_TIMESTAMP WHERE doc_id=?`, "pending exact analysis merge into "+group.Canonical, id); err != nil {
			return err
		}
	}
	for _, row := range group.Before.Rows["episodic_memories"] {
		var refs []string
		if err := json.Unmarshal([]byte(text(row, "related_doc_ids")), &refs); err != nil {
			return err
		}
		var merged []string
		for _, id := range refs {
			id = mapped(group, id)
			if !contains(merged, id) {
				merged = append(merged, id)
			}
		}
		encoded, _ := json.Marshal(merged)
		if _, err := tx.ExecContext(ctx, `UPDATE episodic_memories SET related_doc_ids=? WHERE id=?`, string(encoded), row["id"]); err != nil {
			return err
		}
	}
	for _, row := range group.Before.Rows["memory_conflicts"] {
		left, right := mapped(group, text(row, "doc_id_left")), mapped(group, text(row, "doc_id_right"))
		leftValue, rightValue := text(row, "left_value"), text(row, "right_value")
		if left > right {
			left, right = right, left
			leftValue, rightValue = rightValue, leftValue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE memory_conflicts SET doc_id_left=?,doc_id_right=?,left_value=?,right_value=?,
			winning_doc_id=?,superseded_doc_id=? WHERE id=?`, left, right, leftValue, rightValue, mapped(group, text(row, "winning_doc_id")), mapped(group, text(row, "superseded_doc_id")), row["id"]); err != nil {
			return err
		}
	}
	for _, row := range group.Before.Rows["memory_maintenance_meta"] {
		if _, err := tx.ExecContext(ctx, `UPDATE memory_maintenance_meta SET value=? WHERE key=?`, mapped(group, text(row, "value")), text(row, "key")); err != nil {
			return err
		}
	}
	audit, _ := json.Marshal(map[string]any{"version": 1, "plan_hash": group.Hash, "original_ids": group.IDs, "canonical": group.Canonical})
	if _, err := tx.ExecContext(ctx, `INSERT INTO memory_curation_events(doc_id,action,actor,previous_status,new_status,reason)
		SELECT doc_id,'analysis_merge','operator_repair',verification_status,verification_status,? FROM memory_meta WHERE doc_id=?`, string(audit), group.Canonical); err != nil {
		return err
	}
	return nil
}

func applyGroup(ctx context.Context, db *sql.DB, vectorPath string, group mergeGroup, hook checkpoint) (bool, error) {
	if group.Hash == "" || group.Hash != fingerprint(group.Before) || !contains(group.IDs, group.Canonical) || len(group.IDs) < 2 || len(group.IDs) != len(group.Before.Documents) {
		return false, fmt.Errorf("invalid group plan")
	}
	var identity string
	for i, doc := range group.Before.Documents {
		concept, body, ok := memory.AnalysisDocumentParts(doc.Content)
		key, valid := memory.AnalysisMemoryIdentity(concept, body, doc.Metadata["domain"])
		if !ok || !valid || group.IDs[i] != doc.ID || doc.Metadata["chunk_index"] != "" || len(doc.Content) > 4000 || (identity != "" && identity != key) {
			return false, fmt.Errorf("group identity is unproven")
		}
		identity = key
	}
	key := journalPrefix + group.Hash
	states, err := rowsFor(ctx, db, `SELECT value FROM memory_maintenance_meta WHERE key=?`, key)
	if err != nil {
		return false, err
	}
	state := mergeJournal{Version: 1, GroupHash: group.Hash}
	if len(states) > 0 {
		if err := json.Unmarshal([]byte(text(states[0], "value")), &state); err != nil {
			return false, err
		}
		if state.Version != 1 || state.GroupHash != group.Hash {
			return false, fmt.Errorf("merge journal does not match plan")
		}
		if state.State == "done" {
			return false, nil
		}
		if state.State != "references_committed" || state.SQLHash == "" {
			return false, fmt.Errorf("invalid merge journal state")
		}
	} else {
		vectors, err := openVectors(vectorPath)
		if err != nil {
			return false, err
		}
		if err := readVectors(ctx, group, vectors, false); err != nil {
			return false, err
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return false, err
		}
		defer tx.Rollback()
		if _, err := tx.ExecContext(ctx, `UPDATE memory_maintenance_meta SET value=value WHERE key=?`, key); err != nil {
			return false, err
		}
		current, err := sqlSnapshot(ctx, tx, group.IDs)
		if err != nil {
			return false, err
		}
		if fingerprint(current) != fingerprint(group.Before.Rows) {
			return false, fmt.Errorf("group metadata or evidence changed")
		}
		canonical, err := selectCanonical(group)
		if err != nil {
			return false, err
		}
		if canonical != group.Canonical {
			return false, fmt.Errorf("canonical selection changed")
		}
		if err := validateReferences(group); err != nil {
			return false, err
		}
		if err := transferReferences(ctx, tx, group); err != nil {
			return false, fmt.Errorf("transfer references: %w", err)
		}
		after, err := sqlSnapshot(ctx, tx, group.IDs)
		if err != nil {
			return false, err
		}
		state.State = "references_committed"
		state.SQLHash = fingerprint(after)
		if err := writeJournal(ctx, tx, key, state); err != nil {
			return false, err
		}
		if err := check(hook, "before_sql_commit"); err != nil {
			return false, err
		}
		if err := tx.Commit(); err != nil {
			return false, fmt.Errorf("commit reference transfer; retain tombstones on uncertainty: %w", err)
		}
		if err := check(hook, "after_sql_commit"); err != nil {
			return false, err
		}
	}
	// ponytail: reload the on-disk collection per deletion; an offline maintenance
	// run favors durable proof over speed. Batch verified reloads if this is costly.
	for _, id := range group.IDs {
		if id == group.Canonical {
			continue
		}
		if err := deleteStagedVector(ctx, db, vectorPath, group, state, key, id, hook); err != nil {
			return false, err
		}
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE memory_maintenance_meta SET value=value WHERE key=?`, key); err != nil {
		return false, err
	}
	current, err := sqlSnapshot(ctx, tx, group.IDs)
	if err != nil {
		return false, err
	}
	if fingerprint(current) != state.SQLHash {
		return false, fmt.Errorf("group changed before final cleanup")
	}
	for _, id := range group.IDs {
		if id != group.Canonical {
			if _, err := tx.ExecContext(ctx, `DELETE FROM memory_meta WHERE doc_id=?`, id); err != nil {
				return false, err
			}
		}
	}
	state.State = "done"
	if err := writeJournal(ctx, tx, key, state); err != nil {
		return false, err
	}
	if err := check(hook, "before_cleanup_commit"); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func deleteStagedVector(ctx context.Context, db *sql.DB, path string, group mergeGroup, state mergeJournal, key, id string, hook checkpoint) error {
	if err := check(hook, "before_vector_delete:"+id); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE memory_maintenance_meta SET value=value WHERE key=?`, key); err != nil {
		return err
	}
	current, err := sqlSnapshot(ctx, tx, group.IDs)
	if err != nil {
		return err
	}
	if fingerprint(current) != state.SQLHash {
		return fmt.Errorf("staged group changed; retained tombstones")
	}
	vectors, err := openVectors(path)
	if err != nil {
		return err
	}
	if err := readVectors(ctx, group, vectors, true); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := vectors.GetByID(ctx, id); err == nil {
		if err := vectors.Delete(ctx, nil, nil, id); err != nil {
			return fmt.Errorf("delete vector %s; group remains pending: %w", id, err)
		}
	} else if !missingVector(err, id) {
		return err
	}
	if err := check(hook, "after_vector_delete:"+id); err != nil {
		return err
	}
	persisted, err := openVectors(path)
	if err != nil {
		return err
	}
	if _, err := persisted.GetByID(ctx, id); !missingVector(err, id) {
		return fmt.Errorf("vector %s deletion not durably confirmed", id)
	}
	return tx.Commit()
}

func applyGroups(ctx context.Context, db *sql.DB, vectorPath string, plan mergePlan, hook checkpoint) (int, []reviewItem) {
	var complete int
	var failures []reviewItem
	for _, group := range plan.Groups {
		changed, err := applyGroup(ctx, db, vectorPath, group, hook)
		if err != nil {
			failures = append(failures, reviewItem{group.IDs, err.Error()})
		} else if changed {
			complete++
		}
		if ctx.Err() != nil {
			break
		}
	}
	return complete, failures
}
