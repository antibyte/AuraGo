package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// applySessionGraphPrivacyTarget deletes only the exact candidate captured in
// the pre-scrub manifest. Any changed, protected, or newly shared row is kept.
func (kg *KnowledgeGraph) applySessionGraphPrivacyTarget(ctx context.Context, target privacyDeletionTarget) (bool, error) {
	if kg == nil {
		return false, fmt.Errorf("knowledge graph is unavailable")
	}
	tx, err := kg.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin graph privacy target: %w", err)
	}
	defer tx.Rollback()
	deleted := false
	switch target.Store {
	case "kg_node":
		var id, label, properties string
		var protected int
		err = tx.QueryRowContext(ctx, `SELECT id,label,properties,protected FROM kg_nodes WHERE id=?`, target.TargetID).Scan(&id, &label, &properties, &protected)
		if err == sql.ErrNoRows {
			tx.Rollback()
			if err := kg.removeSemanticNodeIndex(target.TargetID); err != nil {
				return false, err
			}
			return true, nil // The exact candidate is already absent.
		}
		if err != nil {
			return false, err
		}
		if privacyFingerprint(id, label, properties, fmt.Sprint(protected)) != target.ExpectedFingerprint || protected != 0 {
			return false, nil
		}
		var props map[string]string
		if err := json.Unmarshal([]byte(properties), &props); err != nil {
			return false, fmt.Errorf("decode graph privacy node: %w", err)
		}
		if !sessionGraphNodeEligible(props, target.SourceMarker) {
			return false, nil
		}
		var shared, incident int
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM kg_claims WHERE subject_id=? OR object_id=?)`, id, id).Scan(&shared); err != nil {
			return false, err
		}
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM kg_edges WHERE source=? OR target=?`, id, id).Scan(&incident); err != nil {
			return false, err
		}
		if shared != 0 || incident != 0 {
			return false, nil
		}
		result, err := tx.ExecContext(ctx, `DELETE FROM kg_nodes WHERE id=? AND protected=0 AND properties=?`, id, properties)
		if err != nil {
			return false, err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return false, err
		}
		deleted = count == 1
	case "kg_edge":
		var identity []string
		if err := json.Unmarshal([]byte(target.TargetID), &identity); err != nil || len(identity) != 3 {
			return false, fmt.Errorf("invalid graph privacy edge identity")
		}
		source, destination, relation := identity[0], identity[1], identity[2]
		var properties string
		err = tx.QueryRowContext(ctx, `SELECT properties FROM kg_edges WHERE source=? AND target=? AND relation=?`, source, destination, relation).Scan(&properties)
		if err == sql.ErrNoRows {
			tx.Rollback()
			if err := kg.removeSemanticEdgeIndex(source, destination, relation); err != nil {
				return false, err
			}
			return true, nil
		}
		if err != nil {
			return false, err
		}
		if privacyFingerprint(source, destination, relation, properties) != target.ExpectedFingerprint {
			return false, nil
		}
		var props map[string]string
		if err := json.Unmarshal([]byte(properties), &props); err != nil {
			return false, err
		}
		if !sessionGraphEdgeEligible(props, target.SourceMarker) {
			return false, nil
		}
		var foreign int
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM kg_claims WHERE subject_id=? AND object_id=? AND predicate=?)`, source, destination, relation).Scan(&foreign); err != nil {
			return false, err
		}
		blockedEndpoints, err := contactPrivacyProtectedEndpoints(ctx, tx, source, destination)
		if err != nil {
			return false, err
		}
		if foreign != 0 || blockedEndpoints {
			return false, nil
		}
		result, err := tx.ExecContext(ctx, `DELETE FROM kg_edges WHERE source=? AND target=? AND relation=? AND properties=?`, source, destination, relation, properties)
		if err != nil {
			return false, err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return false, err
		}
		deleted = count == 1
	case "kg_claim":
		values, _, err := readPrivacyClaimFingerprintValues(ctx, tx, target.TargetID)
		if err == sql.ErrNoRows {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		if values[17] != target.SourceMarker || values[18] != "" || values[14] != "normal" || values[15] != "default" || !privacyClaimSourceDeletable(values[10]) || privacyFingerprint(values[1:]...) != target.ExpectedFingerprint {
			return false, nil
		}
		blocked, err := contactPrivacyProtectedEndpoints(ctx, tx, values[1], values[3])
		if err != nil {
			return false, err
		}
		if blocked {
			return false, nil
		}
		if err := cleanupKGClaimsByIDTx(tx, []string{target.TargetID}); err != nil {
			return false, err
		}
		deleted = true
	default:
		return false, fmt.Errorf("unsupported graph privacy target %q", target.Store)
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	if deleted {
		switch target.Store {
		case "kg_node":
			if err := kg.removeSemanticNodeIndex(target.TargetID); err != nil {
				return false, err
			}
		case "kg_edge":
			var identity []string
			_ = json.Unmarshal([]byte(target.TargetID), &identity)
			if len(identity) == 3 {
				if err := kg.removeSemanticEdgeIndex(identity[0], identity[1], identity[2]); err != nil {
					return false, err
				}
			}
		}
	}
	return deleted, nil
}

func privacyClaimSourceDeletable(source string) bool {
	switch source {
	case "memory_analysis", "consolidation", "chat", "hierarchy":
		return true
	default:
		return false
	}
}

func readPrivacyClaimFingerprintValues(ctx context.Context, tx *sql.Tx, id string) ([]string, float64, error) {
	var values [19]string
	var confidence float64
	err := tx.QueryRowContext(ctx, `SELECT id,subject_id,predicate,object_id,object_literal,asserted_in_graph,learned_at,COALESCE(accepted_at,''),confidence,confidence_label,source_kind,ingestion_run_id,status,superseded_by,privacy_class,retention_policy,COALESCE(evidence_id,''),session_id,source_message_id FROM kg_claims WHERE id=?`, id).Scan(&values[0], &values[1], &values[2], &values[3], &values[4], &values[5], &values[6], &values[7], &confidence, &values[9], &values[10], &values[11], &values[12], &values[13], &values[14], &values[15], &values[16], &values[17], &values[18])
	values[8] = fmt.Sprint(confidence)
	return values[:], confidence, err
}

func sessionGraphNodeEligible(props map[string]string, marker string) bool {
	if props["session_id"] != marker || privacyPropertyTrue(props["protected"]) || privacyPropertyTrue(props["keep_forever"]) || props["contact_sync_id"] != "" {
		return false
	}
	switch props["source"] {
	case "memory_analysis", "chat", "consolidation", "hierarchy":
	default:
		return false
	}
	for key, value := range props {
		switch key {
		case "role":
			if value != "" {
				return false
			}
		case "type", "source", "session_id", "description", "content", "summary", "confidence", "created_at", "updated_at", "domain", "category":
		default:
			return false
		}
	}
	return true
}

func sessionGraphEdgeEligible(props map[string]string, marker string) bool {
	return props["session_id"] == marker && !privacyPropertyTrue(props["protected"]) && !privacyPropertyTrue(props["keep_forever"]) && props["contact_sync_id"] == ""
}
