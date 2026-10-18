package memory

import (
	"context"
	"encoding/json"
	"fmt"
)

// prepareSessionGraphDeletionTargets records the exact graph rows selected by
// the existing session-owned cleanup policy and their expected post-scrub
// identities. The target IDs and digests contain no transcript text.
func (kg *KnowledgeGraph) prepareSessionGraphDeletionTargets(ctx context.Context, sessionID, marker string) ([]privacyDeletionTarget, error) {
	if kg == nil {
		return nil, nil
	}
	var targets []privacyDeletionTarget
	for _, table := range []string{"kg_nodes", "kg_edges"} {
		if table == "kg_nodes" {
			rows, err := kg.db.QueryContext(ctx, `SELECT n.id,n.label,n.properties,n.protected FROM kg_nodes n
				WHERE json_extract(n.properties,'$.session_id')=? OR EXISTS(SELECT 1 FROM kg_claims c WHERE (c.subject_id=n.id OR c.object_id=n.id) AND c.session_id=?) ORDER BY n.id`, sessionID, sessionID)
			if err != nil {
				return nil, fmt.Errorf("prepare graph privacy nodes: %w", err)
			}
			for rows.Next() {
				var id, label, properties string
				var protected int
				if err := rows.Scan(&id, &label, &properties, &protected); err != nil {
					rows.Close()
					return nil, err
				}
				scrubbed, _, _, err := scrubGraphSessionProperties(properties, sessionID, marker)
				if err != nil {
					rows.Close()
					return nil, fmt.Errorf("fingerprint graph privacy node: %w", err)
				}
				fingerprint := privacyFingerprint(id, label, scrubbed, fmt.Sprint(protected))
				targets = append(targets, privacyDeletionTarget{Store: "kg_node", TargetID: id, ExpectedFingerprint: fingerprint, SourceMarker: marker, State: "pending"})
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				return nil, err
			}
			rows.Close()
		} else {
			rows, err := kg.db.QueryContext(ctx, `SELECT e.source,e.target,e.relation,e.properties FROM kg_edges e
				WHERE json_extract(e.properties,'$.session_id')=? OR EXISTS(SELECT 1 FROM kg_claims c WHERE c.subject_id=e.source AND c.object_id=e.target AND c.predicate=e.relation AND c.session_id=?)
				ORDER BY e.source,e.target,e.relation`, sessionID, sessionID)
			if err != nil {
				return nil, fmt.Errorf("prepare graph privacy edges: %w", err)
			}
			for rows.Next() {
				var source, target, relation, properties string
				if err := rows.Scan(&source, &target, &relation, &properties); err != nil {
					rows.Close()
					return nil, err
				}
				scrubbed, _, _, err := scrubGraphSessionProperties(properties, sessionID, marker)
				if err != nil {
					rows.Close()
					return nil, fmt.Errorf("fingerprint graph privacy edge: %w", err)
				}
				fingerprint := privacyFingerprint(source, target, relation, scrubbed)
				id := graphPrivacyEdgeID(source, target, relation)
				targets = append(targets, privacyDeletionTarget{Store: "kg_edge", TargetID: id, ExpectedFingerprint: fingerprint, SourceMarker: marker, State: "pending"})
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				return nil, err
			}
			rows.Close()
		}
	}
	claimRows, err := kg.db.QueryContext(ctx, `SELECT id,subject_id,predicate,object_id,object_literal,asserted_in_graph,learned_at,COALESCE(accepted_at,''),confidence,confidence_label,source_kind,ingestion_run_id,status,superseded_by,privacy_class,retention_policy,COALESCE(evidence_id,'')
		FROM kg_claims WHERE session_id=? ORDER BY id`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("prepare graph privacy claims: %w", err)
	}
	for claimRows.Next() {
		var id, subject, predicate, objectID, objectLiteral, graph, learnedAt, acceptedAt, confidenceLabel, sourceKind, ingestionRun, status, supersededBy, privacyClass, retentionPolicy, evidenceID string
		var confidence float64
		if err := claimRows.Scan(&id, &subject, &predicate, &objectID, &objectLiteral, &graph, &learnedAt, &acceptedAt, &confidence, &confidenceLabel, &sourceKind, &ingestionRun, &status, &supersededBy, &privacyClass, &retentionPolicy, &evidenceID); err != nil {
			claimRows.Close()
			return nil, err
		}
		claimFingerprint := privacyFingerprint(subject, predicate, objectID, objectLiteral, graph, learnedAt, acceptedAt, fmt.Sprint(confidence), confidenceLabel, sourceKind, ingestionRun, status, supersededBy, privacyClass, retentionPolicy, evidenceID, marker, "")
		targets = append(targets, privacyDeletionTarget{Store: "kg_claim", TargetID: id, ExpectedFingerprint: claimFingerprint, SourceMarker: marker, State: "pending"})
	}
	if err := claimRows.Err(); err != nil {
		claimRows.Close()
		return nil, err
	}
	claimRows.Close()
	return targets, nil
}

func graphPrivacyEdgeID(source, target, relation string) string {
	encoded, _ := json.Marshal([]string{source, target, relation})
	return string(encoded)
}
