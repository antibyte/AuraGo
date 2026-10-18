package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func (kg *KnowledgeGraph) prepareContactPrivacyTargets(ctx context.Context, source string, deleteMemories bool) ([]privacyDeletionTarget, error) {
	var targets []privacyDeletionTarget
	appendTarget := func(store, id, fingerprint string) {
		state, reason := "pending", ""
		if !deleteMemories {
			state, reason = PrivacyDeletionRetainedReview, "outside_selected_scope"
		}
		targets = append(targets, privacyDeletionTarget{Store: store, TargetID: id, ExpectedFingerprint: fingerprint, SourceMarker: source, State: state, ReasonCode: reason})
	}
	rows, err := kg.db.QueryContext(ctx, `SELECT id,label,properties,protected FROM kg_nodes WHERE id=? OR json_extract(properties,'$.contact_sync_id')=? ORDER BY id`, source, source)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, label, properties string
		var protected int
		if err := rows.Scan(&id, &label, &properties, &protected); err != nil {
			rows.Close()
			return nil, err
		}
		appendTarget("kg_contact_node", id, privacyFingerprint(id, label, properties, fmt.Sprint(protected)))
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = kg.db.QueryContext(ctx, `SELECT source,target,relation,properties FROM kg_edges WHERE source=? OR target=? OR json_extract(properties,'$.contact_sync_id')=? ORDER BY source,target,relation`, source, source, source)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var a, b, relation, properties string
		if err := rows.Scan(&a, &b, &relation, &properties); err != nil {
			rows.Close()
			return nil, err
		}
		appendTarget("kg_contact_edge", graphPrivacyEdgeID(a, b, relation), privacyFingerprint(a, b, relation, properties))
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	// Capture complete per-claim identities, without retaining the claim text.
	rows, err = kg.db.QueryContext(ctx, `SELECT id FROM kg_claims WHERE source_kind=? AND source_message_id=? ORDER BY id`, contactSyncSource, source)
	if err != nil {
		return nil, err
	}
	var claimIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		claimIDs = append(claimIDs, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	tx, err := kg.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	for _, id := range claimIDs {
		values, _, err := readPrivacyClaimFingerprintValues(ctx, tx, id)
		if err != nil {
			return nil, err
		}
		appendTarget("kg_contact_claim", id, privacyFingerprint(values...))
	}
	return targets, nil
}

func (kg *KnowledgeGraph) applyContactPrivacyTarget(ctx context.Context, target privacyDeletionTarget) (bool, error) {
	tx, err := kg.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	// Reserve the graph writer before inspecting protection, provenance and refs.
	if _, err := tx.ExecContext(ctx, `UPDATE kg_nodes SET id=id WHERE 0`); err != nil {
		return false, err
	}
	var removeIndex func() error
	switch target.Store {
	case "kg_contact_node":
		removeIndex = func() error { return kg.removeSemanticNodeIndex(target.TargetID) }
		var label, properties string
		var protected int
		err := tx.QueryRowContext(ctx, `SELECT label,properties,protected FROM kg_nodes WHERE id=?`, target.TargetID).Scan(&label, &properties, &protected)
		if errors.Is(err, sql.ErrNoRows) {
			_ = tx.Rollback()
			return true, removeIndex()
		}
		if err != nil {
			return false, err
		}
		if protected != 0 || privacyFingerprint(target.TargetID, label, properties, fmt.Sprint(protected)) != target.ExpectedFingerprint {
			return false, nil
		}
		var props map[string]string
		if err := json.Unmarshal([]byte(properties), &props); err != nil {
			return false, err
		}
		if props["contact_sync_id"] != target.SourceMarker || props["source"] != contactSyncSource {
			return false, nil
		}
		for key, value := range props {
			switch key {
			case "role":
				if value != "" {
					return false, nil
				}
			case "type", "source", "contact_sync_id", "email", "phone", "mobile", "relationship", "birthday":
			default:
				return false, nil // Extra knowledge is not owned by contact sync.
			}
		}
		var referenced bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM kg_claims WHERE subject_id=? OR object_id=?) OR EXISTS(SELECT 1 FROM kg_edges WHERE source=? OR target=?)`, target.TargetID, target.TargetID, target.TargetID, target.TargetID).Scan(&referenced); err != nil {
			return false, err
		}
		if referenced {
			return false, nil
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM kg_nodes WHERE id=?`, target.TargetID); err != nil {
			return false, err
		}
	case "kg_contact_edge":
		var identity []string
		if json.Unmarshal([]byte(target.TargetID), &identity) != nil || len(identity) != 3 {
			return false, fmt.Errorf("invalid contact edge identity")
		}
		a, b, relation := identity[0], identity[1], identity[2]
		removeIndex = func() error { return kg.removeSemanticEdgeIndex(a, b, relation) }
		var properties string
		err := tx.QueryRowContext(ctx, `SELECT properties FROM kg_edges WHERE source=? AND target=? AND relation=?`, a, b, relation).Scan(&properties)
		if errors.Is(err, sql.ErrNoRows) {
			_ = tx.Rollback()
			return true, removeIndex()
		}
		if err != nil {
			return false, err
		}
		if privacyFingerprint(a, b, relation, properties) != target.ExpectedFingerprint {
			return false, nil
		}
		var props map[string]string
		if err := json.Unmarshal([]byte(properties), &props); err != nil {
			return false, err
		}
		if props["contact_sync_id"] != target.SourceMarker || props["source"] != contactSyncSource || privacyPropertyTrue(props["protected"]) || privacyPropertyTrue(props["keep_forever"]) {
			return false, nil
		}
		blocked, err := contactPrivacyProtectedEndpoints(ctx, tx, a, b)
		if err != nil || blocked {
			return false, err
		}
		var referenced bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM kg_claims WHERE subject_id=? AND object_id=? AND predicate=?)`, a, b, relation).Scan(&referenced); err != nil {
			return false, err
		}
		if referenced {
			return false, nil
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM kg_edges WHERE source=? AND target=? AND relation=?`, a, b, relation); err != nil {
			return false, err
		}
	case "kg_contact_claim":
		values, _, err := readPrivacyClaimFingerprintValues(ctx, tx, target.TargetID)
		if errors.Is(err, sql.ErrNoRows) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		if privacyFingerprint(values...) != target.ExpectedFingerprint || values[10] != contactSyncSource || values[18] != target.SourceMarker || values[14] != "normal" || values[15] != "default" || values[12] != "candidate" && values[12] != "accepted" {
			return false, nil
		}
		blocked, err := contactPrivacyProtectedEndpoints(ctx, tx, values[1], values[3])
		if err != nil || blocked {
			return false, err
		}
		if err := cleanupKGClaimsByIDTx(tx, []string{target.TargetID}); err != nil {
			return false, err
		}
	default:
		return false, fmt.Errorf("unknown contact privacy target type")
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	if removeIndex != nil {
		if err := removeIndex(); err != nil {
			return false, err
		}
	}
	return true, nil
}

func privacyPropertyTrue(value string) bool { return value == "1" || strings.EqualFold(value, "true") }

func contactPrivacyProtectedEndpoints(ctx context.Context, tx *sql.Tx, a, b string) (bool, error) {
	var blocked bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM kg_nodes WHERE id IN (?,?) AND (protected<>0 OR LOWER(COALESCE(json_extract(properties,'$.protected'),'')) IN ('1','true') OR LOWER(COALESCE(json_extract(properties,'$.keep_forever'),'')) IN ('1','true')))`, a, b).Scan(&blocked)
	return blocked, err
}
