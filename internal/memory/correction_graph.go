package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type correctionEdge struct {
	Properties string `json:"properties"`
	Status     string `json:"status"`
	Revision   string `json:"revision"`
	Claims     string `json:"claims"`
}

func correctionGraphFieldAllowed(field string) bool {
	if field == "" {
		return false
	}
	switch field {
	case "id", "source", "source_file", "source_kind", "protected", "type", "contact_sync_id", "confidence", "memory_correction_id":
		return false
	}
	return !isSensitiveKnowledgeGraphPropertyKey(field)
}

func (kg *KnowledgeGraph) initCorrectionHistory() error {
	var exists int
	if err := kg.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='kg_correction_receipts'`).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		var count int
		if err := kg.db.QueryRow(`SELECT COUNT(*) FROM kg_nodes`).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			if _, err := backupMainDatabase(kg.db, "memory-corrections-v1"); err != nil {
				return err
			}
		}
	}
	_, err := kg.db.Exec(`CREATE TABLE IF NOT EXISTS kg_correction_receipts (id TEXT PRIMARY KEY, after_json TEXT NOT NULL);
	CREATE TABLE IF NOT EXISTS kg_correction_guards (reference TEXT NOT NULL, value TEXT NOT NULL, correction_id TEXT NOT NULL, PRIMARY KEY(reference,value));`)
	if err != nil {
		return err
	}
	return kg.initCorrectionFieldRevisions()
}

func (kg *KnowledgeGraph) correctionSnapshot(ctx context.Context, q correctionQueryer, ref MemoryReference) (correctionSnapshot, error) {
	var s correctionSnapshot
	if ref.Store == "kg_node" {
		if ref.ID == "" || !correctionGraphFieldAllowed(ref.Field) {
			return s, fmt.Errorf("graph correction requires an editable property or label")
		}
		node := &Node{ID: ref.ID}
		var props string
		if err := q.QueryRowContext(ctx, `SELECT label,properties,protected,
			'field-v1:'||COALESCE((SELECT revision FROM kg_correction_field_revisions WHERE node_id=kg_nodes.id AND field=''),0)
			||':'||COALESCE((SELECT revision FROM kg_correction_field_revisions WHERE node_id=kg_nodes.id AND field=?),0)
			FROM kg_nodes WHERE id=?`, ref.Field, ref.ID).Scan(&node.Label, &props, &node.Protected, &s.Revision); err != nil {
			return s, err
		}
		if node.Protected {
			return s, ErrKnowledgeGraphProtectedNode
		}
		if err := json.Unmarshal([]byte(props), &node.Properties); err != nil {
			return s, err
		}
		if node.Properties["source"] == "file_sync" || node.Properties["source"] == "contacts_sync" || node.Properties["source"] == "inventory_sync" {
			return s, fmt.Errorf("correct this fact in its owning source")
		}
		s.Node = node
		s.Value = node.Properties[ref.Field]
		if ref.Field == "label" {
			s.Value = node.Label
		}
		return s, nil
	}
	if ref.ID == "" || ref.Target == "" || ref.Relation == "" {
		return s, fmt.Errorf("edge source, target and relation are required")
	}
	var protected int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM kg_nodes WHERE id IN (?,?) AND protected=1`, ref.ID, ref.Target).Scan(&protected); err != nil {
		return s, err
	}
	if protected > 0 {
		return s, ErrKnowledgeGraphProtectedNode
	}
	edge := &correctionEdge{}
	if err := q.QueryRowContext(ctx, `SELECT properties,COALESCE(status,'accepted'),updated_at FROM kg_edges WHERE source=? AND target=? AND relation=?`, ref.ID, ref.Target, ref.Relation).Scan(&edge.Properties, &edge.Status, &edge.Revision); err != nil {
		return s, err
	}
	if err := q.QueryRowContext(ctx, `SELECT COALESCE(group_concat(v,'|'),'') FROM (SELECT id||':'||status||':'||COALESCE(superseded_by,'') AS v FROM kg_claims WHERE subject_id=? AND object_id=? AND predicate=? ORDER BY id)`, ref.ID, ref.Target, ref.Relation).Scan(&edge.Claims); err != nil {
		return s, err
	}
	var props map[string]string
	if err := json.Unmarshal([]byte(edge.Properties), &props); err != nil {
		return s, err
	}
	if props["protected"] == "true" || props["source"] == "file_sync" || props["source"] == "contacts_sync" {
		return s, fmt.Errorf("edge is protected or externally owned")
	}
	s.Value, s.Edge = ref.Target, edge
	s.Absent = edge.Status == string(KGClaimRetracted)
	return s, nil
}

func (kg *KnowledgeGraph) applyCorrection(ctx context.Context, receiptID string, correction *MemoryCorrection, item *MemoryCorrectionItem) (json.RawMessage, error) {
	tx, err := kg.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE kg_nodes SET id=id WHERE id=?`, item.Reference.ID); err != nil {
		return nil, err
	}
	current, err := kg.correctionSnapshot(ctx, tx, item.Reference)
	if err != nil {
		return nil, err
	}
	actual, _ := json.Marshal(current)
	var receipt string
	err = tx.QueryRowContext(ctx, `SELECT after_json FROM kg_correction_receipts WHERE id=?`, receiptID).Scan(&receipt)
	if err == nil {
		if !correctionItemMatches(item, actual, []byte(receipt)) {
			return nil, fmt.Errorf("graph changed after correction commit")
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return actual, kg.finishCorrectionIndex(ctx, item, current)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if !correctionItemMatches(item, actual, item.Snapshot) {
		return nil, fmt.Errorf("graph target changed since preview")
	}
	ref := item.Reference
	if ref.Store == "kg_node" {
		node := current.Node
		if ref.Field == "label" {
			if item.Operation == "archive" {
				node.Label = node.ID
			} else {
				node.Label = item.Value
			}
		} else if item.Operation == "archive" {
			delete(node.Properties, ref.Field)
		} else {
			node.Properties[ref.Field] = item.Value
		}
		if node.Properties["source"] == "core_memory" && ref.Field == "content" {
			node.Label = node.ID
			if item.Operation == "replace" {
				node.Label = item.Value
				if r := []rune(node.Label); len(r) > 50 {
					node.Label = string(r[:47]) + "..."
				}
			}
		}
		props, _ := json.Marshal(node.Properties)
		_, err = tx.ExecContext(ctx, `UPDATE kg_nodes SET label=?,properties=?,updated_at=CURRENT_TIMESTAMP,semantic_indexed_at=NULL WHERE id=? AND protected=0`, node.Label, string(props), ref.ID)
	} else {
		if current.Edge.Status != string(KGClaimAccepted) && !(current.Edge.Status == string(KGClaimRetracted) && item.Operation == "replace") {
			return nil, fmt.Errorf("only an active edge can be corrected")
		}
		winning := ""
		claimIDs, loadErr := listAcceptedKGClaimIDsForEdgeTx(tx, ref.ID, ref.Target, ref.Relation)
		if loadErr != nil {
			return nil, loadErr
		}
		if item.Operation == "replace" {
			var targetProtected bool
			if err = tx.QueryRowContext(ctx, `SELECT protected FROM kg_nodes WHERE id=?`, item.Value).Scan(&targetProtected); err != nil {
				return nil, err
			}
			if targetProtected {
				return nil, ErrKnowledgeGraphProtectedNode
			}
			var count int
			if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM kg_edges WHERE source=? AND target=? AND relation=? AND COALESCE(status,'accepted')='accepted'`, ref.ID, item.Value, ref.Relation).Scan(&count); err != nil {
				return nil, err
			}
			if count != 0 {
				return nil, fmt.Errorf("replacement edge already exists; resolve its claims explicitly")
			}
			replacementRef := ref
			replacementRef.Target = item.Value
			if _, readErr := kg.correctionSnapshot(ctx, tx, replacementRef); readErr != nil && !errors.Is(readErr, sql.ErrNoRows) {
				return nil, readErr
			}
			winning = newKGClaimID(time.Now())
			props, _ := json.Marshal(ensureKnowledgeGraphEdgeQualityProperties(map[string]string{"source": "memory_correction", "memory_correction_id": item.ReplacementID}, "memory_correction", time.Now()))
			if _, err = tx.ExecContext(ctx, `INSERT INTO kg_edges(source,target,relation,properties,status,updated_at) VALUES(?,?,?,?,'accepted',CURRENT_TIMESTAMP) ON CONFLICT(source,target,relation) DO UPDATE SET properties=excluded.properties,status='accepted',status_reason='',superseded_by_claim_id='',retracted_at=NULL,updated_at=CURRENT_TIMESTAMP`, ref.ID, item.Value, ref.Relation, string(props)); err != nil {
				return nil, err
			}
			provenance := KGProvenanceInput{SourceKind: "memory_correction", SessionID: correction.SessionID, IngestionRunID: correction.ID, SourceMessageID: correction.Evidence[0].Reference, RawText: correction.Evidence[0].Quote, EvidenceType: correction.Evidence[0].Kind, Confidence: 1}
			evidenceID, insertErr := kg.insertKGEvidenceTx(tx, provenance, time.Now())
			if insertErr != nil {
				return nil, insertErr
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO kg_claims(id,subject_id,predicate,object_id,asserted_in_graph,accepted_at,confidence,source_kind,ingestion_run_id,status,source_message_id,session_id,privacy_class,retention_policy,evidence_id) VALUES(?,?,?,?,'local:worldview',CURRENT_TIMESTAMP,1,'memory_correction',?,'accepted',?,?,'normal','default',?)`, winning, ref.ID, ref.Relation, item.Value, correction.ID, provenance.SourceMessageID, correction.SessionID, nullableString(evidenceID)); err != nil {
				return nil, err
			}
		}
		status := string(KGClaimRetracted)
		if item.Operation == "replace" {
			status = string(KGClaimSuperseded)
		}
		if item.Operation != "replace" || item.Value != ref.Target {
			if _, err = tx.ExecContext(ctx, `UPDATE kg_edges SET status=?,status_reason=?,superseded_by_claim_id=?,retracted_at=CASE WHEN ?='retracted' THEN CURRENT_TIMESTAMP ELSE retracted_at END,updated_at=CURRENT_TIMESTAMP WHERE source=? AND target=? AND relation=?`, status, correction.Reason, winning, status, ref.ID, ref.Target, ref.Relation); err != nil {
				return nil, err
			}
			if _, err = tx.ExecContext(ctx, `UPDATE kg_claims SET status=?,superseded_by=? WHERE subject_id=? AND object_id=? AND predicate=? AND status='accepted'`, status, winning, ref.ID, ref.Target, ref.Relation); err != nil {
				return nil, err
			}
		}
		err = resolveOpenKGConflictsForInactiveClaimsTx(tx, claimIDs, winning, correction.Reason)
	}
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO kg_correction_guards(reference,value,correction_id) VALUES(?,?,?) ON CONFLICT(reference,value) DO UPDATE SET correction_id=excluded.correction_id`, correctionReferenceKey(ref), correctionIdentity(item.Before), correction.ID); err != nil {
		return nil, err
	}
	if item.Operation == "replace" {
		replacementRef := ref
		if ref.Store == "kg_edge" {
			replacementRef.Target = item.Value
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM kg_correction_guards WHERE reference=? AND value=?`, correctionReferenceKey(replacementRef), correctionIdentity(item.Value)); err != nil {
			return nil, err
		}
	}
	after, err := kg.correctionSnapshot(ctx, tx, ref)
	if err != nil {
		return nil, err
	}
	encoded, _ := json.Marshal(after)
	if _, err = tx.ExecContext(ctx, `INSERT INTO kg_correction_receipts(id,after_json) VALUES(?,?)`, receiptID, string(encoded)); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return encoded, kg.finishCorrectionIndex(ctx, item, after)
}

func (kg *KnowledgeGraph) finishCorrectionIndex(ctx context.Context, item *MemoryCorrectionItem, snapshot correctionSnapshot) error {
	if snapshot.Node != nil {
		return kg.finishNodeWrite(ctx, *snapshot.Node, true)
	}
	ref := item.Reference
	if err := kg.removeSemanticEdgeIndex(ref.ID, ref.Target, ref.Relation); err != nil {
		return err
	}
	if item.Operation == "replace" && kg.semanticIndex() != nil {
		var props string
		if err := kg.db.QueryRowContext(ctx, `SELECT properties FROM kg_edges WHERE source=? AND target=? AND relation=?`, ref.ID, item.Value, ref.Relation).Scan(&props); err != nil {
			return err
		}
		p := map[string]string{}
		if err := json.Unmarshal([]byte(props), &p); err != nil {
			return err
		}
		if !kg.upsertSemanticEdgeIndex(Edge{Source: ref.ID, Target: item.Value, Relation: ref.Relation, Properties: p}) {
			return fmt.Errorf("replacement edge index remains pending")
		}
	}
	return ctx.Err()
}

func checkGraphCorrectionGuard(tx *sql.Tx, ref MemoryReference, value string) error {
	var blocked int
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM kg_correction_guards WHERE reference=? AND value=?)`, correctionReferenceKey(ref), correctionIdentity(value)).Scan(&blocked); err != nil {
		return err
	}
	if blocked != 0 {
		return fmt.Errorf("fact was corrected; use a new evidenced memory correction")
	}
	return nil
}

func checkNodeCorrectionGuards(tx *sql.Tx, id, label string, properties map[string]string) error {
	if err := checkGraphCorrectionGuard(tx, MemoryReference{Store: "kg_node", ID: id, Field: "label"}, label); err != nil {
		return err
	}
	for key, value := range properties {
		if strings.TrimSpace(value) != "" {
			if err := checkGraphCorrectionGuard(tx, MemoryReference{Store: "kg_node", ID: id, Field: key}, value); err != nil {
				return err
			}
		}
	}
	return nil
}
