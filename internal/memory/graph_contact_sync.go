package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const contactSyncSource = "contacts_sync"

// SyncContact reconciles only contact-owned fields and explicitly owned edges.
// The write lock precedes all protection, ownership and provenance checks.
func (kg *KnowledgeGraph) SyncContact(ctx context.Context, id, label string, fields map[string]string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if !strings.HasPrefix(id, "contact_") || strings.TrimSpace(label) == "" {
		return fmt.Errorf("contact sync requires a contact ID and name")
	}
	tx, err := kg.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin contact sync: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE kg_nodes SET id=id WHERE id=?`, id); err != nil {
		return fmt.Errorf("lock contact sync: %w", err)
	}
	_, properties, protected, _, err := loadKnowledgeGraphNode(tx, id)
	if err != nil {
		return fmt.Errorf("load contact node: %w", err)
	}
	if protected != 0 {
		return fmt.Errorf("contact sync review required: protected node")
	}
	for _, key := range []string{"email", "phone", "mobile", "relationship", "birthday"} {
		delete(properties, key)
		if value := strings.TrimSpace(fields[key]); value != "" {
			properties[key] = value
		}
	}
	properties["type"] = "person"
	properties = validateNodeSchema(properties)
	encoded, err := json.Marshal(properties)
	if err != nil {
		return fmt.Errorf("encode contact node: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO kg_nodes (id,label,properties,updated_at) VALUES (?,?,?,CURRENT_TIMESTAMP)
	 ON CONFLICT(id) DO UPDATE SET label=excluded.label,properties=excluded.properties,updated_at=CURRENT_TIMESTAMP`, id, strings.TrimSpace(label), string(encoded)); err != nil {
		return fmt.Errorf("write contact node: %w", err)
	}

	target := ""
	if relationship := properties["relationship"]; relationship != "" {
		target = "org_" + strings.ToLower(strings.ReplaceAll(relationship, " ", "_"))
	}
	rows, err := tx.QueryContext(ctx, `SELECT e.target,e.properties,COALESCE(n.protected,0),
	 EXISTS(SELECT 1 FROM kg_claims c WHERE c.subject_id=e.source AND c.object_id=e.target AND c.predicate=e.relation
	 AND (c.source_kind<>? OR c.source_message_id<>?))
	 FROM kg_edges e LEFT JOIN kg_nodes n ON n.id=e.target
	 WHERE e.source=? AND e.relation='belongs_to' AND `+activeKGEdgePredicate("e"), contactSyncSource, id, id)
	if err != nil {
		return fmt.Errorf("read contact relationships: %w", err)
	}
	var stale []string
	var reviewErr error
	targetExists := false
	for rows.Next() {
		var existing, payload string
		var targetProtected int
		var foreignClaim bool
		if err := rows.Scan(&existing, &payload, &targetProtected, &foreignClaim); err != nil {
			rows.Close()
			return fmt.Errorf("scan contact relationship: %w", err)
		}
		if existing == target {
			targetExists = true
			continue
		}
		var edgeProperties map[string]string
		if err := json.Unmarshal([]byte(payload), &edgeProperties); err != nil {
			rows.Close()
			return fmt.Errorf("decode contact relationship: %w", err)
		}
		if edgeProperties["contact_sync_id"] != id || foreignClaim || targetProtected != 0 || strings.EqualFold(edgeProperties["protected"], "true") {
			reviewErr = errors.Join(reviewErr, fmt.Errorf("contact sync review required: retained protected or unowned relationship"))
			continue
		}
		stale = append(stale, existing)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return fmt.Errorf("iterate contact relationships: %w", err)
	}
	for _, oldTarget := range stale {
		if err := cleanupKGClaimsForDeletedEdgeTx(tx, id, oldTarget, "belongs_to"); err != nil {
			return fmt.Errorf("clean contact relationship provenance: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM kg_edges WHERE source=? AND target=? AND relation='belongs_to'`, id, oldTarget); err != nil {
			return fmt.Errorf("remove contact relationship: %w", err)
		}
	}
	var createdOrg *Node
	var createdEdge *Edge
	if target != "" && !targetExists {
		_, _, _, found, err := loadKnowledgeGraphNode(tx, target)
		if err != nil {
			return fmt.Errorf("load contact organization: %w", err)
		}
		if !found {
			orgProps := map[string]string{"type": "organization", "source": contactSyncSource}
			orgJSON, _ := json.Marshal(orgProps)
			if _, err := tx.ExecContext(ctx, `INSERT INTO kg_nodes (id,label,properties,updated_at) VALUES (?,?,?,CURRENT_TIMESTAMP)`, target, properties["relationship"], string(orgJSON)); err != nil {
				return fmt.Errorf("create contact organization: %w", err)
			}
			createdOrg = &Node{ID: target, Label: properties["relationship"], Properties: orgProps}
		}
		now := time.Now()
		edgeProps := ensureKnowledgeGraphEdgeQualityProperties(map[string]string{"contact_sync_id": id}, contactSyncSource, now)
		edgeJSON, _ := json.Marshal(edgeProps)
		if _, err := tx.ExecContext(ctx, `INSERT INTO kg_edges (source,target,relation,properties,updated_at,status) VALUES (?,?,'belongs_to',?,CURRENT_TIMESTAMP,?)`, id, target, string(edgeJSON), string(KGClaimAccepted)); err != nil {
			return fmt.Errorf("create contact relationship: %w", err)
		}
		claimID := newKGClaimID(now)
		if _, err := tx.ExecContext(ctx, `INSERT INTO kg_claims (id,subject_id,predicate,object_id,accepted_at,confidence,source_kind,source_message_id,status)
		 VALUES (?,?,'belongs_to',?,CURRENT_TIMESTAMP,1,?,?,?)`, claimID, id, target, contactSyncSource, id, string(KGClaimAccepted)); err != nil {
			return fmt.Errorf("record contact relationship provenance: %w", err)
		}
		if err := kg.detectKGConflictsTx(tx, claimID, id, target, "belongs_to", edgeProps); err != nil {
			return err
		}
		createdEdge = &Edge{Source: id, Target: target, Relation: "belongs_to", Properties: edgeProps}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit contact sync: %w", err)
	}
	kg.indexSemanticNodeAfterWrite(Node{ID: id, Label: strings.TrimSpace(label), Properties: properties})
	if createdOrg != nil {
		kg.indexSemanticNodeAfterWrite(*createdOrg)
	}
	if createdEdge != nil {
		kg.indexSemanticEdgeAfterWrite(*createdEdge)
	}
	if err := kg.removeSemanticEdgeIndexBatch(id, stale, "belongs_to"); err != nil {
		reviewErr = errors.Join(reviewErr, fmt.Errorf("remove contact relationship index: %w", err))
	}
	return errors.Join(reviewErr, ctx.Err())
}
