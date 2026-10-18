package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// deleteExclusiveSessionGraph removes only graph records with explicit exclusive provenance.
func (kg *KnowledgeGraph) deleteExclusiveSessionGraph(ctx context.Context, session string) (int, int, error) {
	return kg.deleteExclusiveGraphSource(ctx, "session_id", session)
}

// DeleteContactSource preserves unrelated nodes, properties, relationships and evidence.
func (kg *KnowledgeGraph) DeleteContactSource(ctx context.Context, id string) (int, int, error) {
	if err := kg.FenceContactSource(ctx, id); err != nil {
		return 0, 0, err
	}
	return kg.deleteExclusiveGraphSource(ctx, "contact_sync_id", id)
}

// FenceContactSource stops delayed synchronization without deleting derived knowledge.
func (kg *KnowledgeGraph) FenceContactSource(ctx context.Context, id string) error {
	if _, err := kg.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS deleted_contact_sources(contact_id TEXT PRIMARY KEY,deleted_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		return fmt.Errorf("create contact deletion fence: %w", err)
	}
	if _, err := kg.db.ExecContext(ctx, `INSERT OR IGNORE INTO deleted_contact_sources(contact_id) VALUES(?)`, id); err != nil {
		return fmt.Errorf("record contact deletion fence: %w", err)
	}
	return nil
}

func (kg *KnowledgeGraph) CountContactSource(ctx context.Context, id string) (int, error) {
	var count int
	err := kg.db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM kg_nodes WHERE id=? OR json_extract(properties,'$.contact_sync_id')=?)+(SELECT COUNT(*) FROM kg_edges WHERE json_extract(properties,'$.contact_sync_id')=?)`, id, id, id).Scan(&count)
	return count, err
}

// IsContactSourceDeleted identifies an earlier deletion for an explicit cleanup retry.
func (kg *KnowledgeGraph) IsContactSourceDeleted(ctx context.Context, id string) (bool, error) {
	var exists bool
	if err := kg.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='deleted_contact_sources')`).Scan(&exists); err != nil || !exists {
		return false, err
	}
	var deleted bool
	err := kg.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM deleted_contact_sources WHERE contact_id=?)`, id).Scan(&deleted)
	return deleted, err
}

func (kg *KnowledgeGraph) deleteExclusiveGraphSource(ctx context.Context, key, value string) (outDeleted, outRetained int, outErr error) {
	committed := false
	defer func() {
		if !committed {
			outDeleted = 0
		}
	}()
	tx, err := kg.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("begin graph privacy deletion: %w", err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE kg_nodes SET id=id WHERE 0`); err != nil {
		return 0, 0, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,label,properties,protected FROM kg_nodes n WHERE json_extract(properties,?)=?
	 OR (?='session_id' AND EXISTS(SELECT 1 FROM kg_claims c WHERE (c.subject_id=n.id OR c.object_id=n.id) AND c.session_id=?))
	 OR (?='contact_sync_id' AND n.id=?)`, "$."+key, value, key, value, key, value)
	if err != nil {
		return 0, 0, err
	}
	var candidates []Node
	for rows.Next() {
		var n Node
		var props string
		if err = rows.Scan(&n.ID, &n.Label, &props, &n.Protected); err != nil {
			break
		}
		if err = json.Unmarshal([]byte(props), &n.Properties); err != nil {
			break
		}
		candidates = append(candidates, n)
	}
	err = errors.Join(err, rows.Err())
	rows.Close()
	if err != nil {
		return 0, 0, err
	}
	deleted, retained := 0, 0
	var removedEdges []Edge
	var removedNodes []string
	edgeRows, err := tx.QueryContext(ctx, `SELECT e.source,e.target,e.relation,e.properties,
	 EXISTS(SELECT 1 FROM kg_claims c WHERE c.subject_id=e.source AND c.object_id=e.target AND c.predicate=e.relation AND
	 CASE WHEN ?='session_id' THEN c.session_id<>? ELSE c.source_kind<>'contacts_sync' OR c.source_message_id<>? END),
	 EXISTS(SELECT 1 FROM kg_nodes n WHERE (n.id=e.source OR n.id=e.target) AND n.protected<>0)
	 FROM kg_edges e WHERE json_extract(e.properties,?)=? OR (?='contact_sync_id' AND (e.source=? OR e.target=?))`, key, value, value, "$."+key, value, key, value, value)
	if err != nil {
		return 0, 0, err
	}
	var exclusiveEdges []Edge
	for edgeRows.Next() {
		var e Edge
		var props string
		var foreign, protected bool
		if err = edgeRows.Scan(&e.Source, &e.Target, &e.Relation, &props, &foreign, &protected); err != nil {
			break
		}
		if err = json.Unmarshal([]byte(props), &e.Properties); err != nil {
			break
		}
		if foreign || protected || e.Properties[key] != value || e.Properties["protected"] == "true" || e.Properties["protected"] == "1" {
			retained++
			continue
		}
		if key == "contact_sync_id" && e.Properties["source"] != contactSyncSource {
			retained++
			continue
		}
		exclusiveEdges = append(exclusiveEdges, e)
	}
	err = errors.Join(err, edgeRows.Err())
	edgeRows.Close()
	if err != nil {
		return 0, retained, err
	}
	for _, edge := range exclusiveEdges {
		if err = cleanupKGClaimsForDeletedEdgeTx(tx, edge.Source, edge.Target, edge.Relation); err != nil {
			return 0, retained, err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM kg_edges WHERE source=? AND target=? AND relation=?`, edge.Source, edge.Target, edge.Relation); err != nil {
			return 0, retained, err
		}
		removedEdges = append(removedEdges, edge)
		deleted++
	}
	for _, node := range candidates {
		var foreign bool
		claimPredicate := `c.session_id<>?`
		if key == "contact_sync_id" {
			claimPredicate = `(c.source_kind<>'contacts_sync' OR c.source_message_id<>?)`
		}
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM kg_claims c WHERE (c.subject_id=? OR c.object_id=?) AND `+claimPredicate+`)
		 OR EXISTS(SELECT 1 FROM kg_edges e WHERE e.source=? OR e.target=?)`, node.ID, node.ID, value, node.ID, node.ID).Scan(&foreign); err != nil {
			return deleted, retained, err
		}
		foreign = foreign || node.Properties[key] != value
		if node.Protected || foreign {
			retained++
			continue
		}
		// A contact-owned marker does not claim arbitrary properties added by other sources.
		if key == "contact_sync_id" && node.Properties["source"] != contactSyncSource {
			foreign = true
		}
		if key == "session_id" {
			if node.Properties["contact_sync_id"] != "" {
				foreign = true
			}
			switch node.Properties["source"] {
			case "memory_analysis", "chat", "consolidation", "hierarchy":
			default:
				foreign = true
			}
		}
		for property := range node.Properties {
			switch property {
			case "role":
				if node.Properties[property] != "" {
					foreign = true
				}
			case "type", "source", "contact_sync_id", "email", "phone", "mobile", "relationship", "birthday":
			case "session_id", "description", "content", "summary", "confidence", "created_at", "updated_at", "domain", "category":
				if key == "contact_sync_id" {
					foreign = true
				}
			default:
				foreign = true
			}
		}
		if foreign {
			retained++
			continue
		}
		edges, err := privacyIncidentEdges(tx, node.ID)
		if err != nil {
			return deleted, retained, err
		}
		if err = cleanupKGClaimsForDeletedNodeTx(tx, node.ID); err != nil {
			return deleted, retained, err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM kg_edges WHERE source=? OR target=?`, node.ID, node.ID); err != nil {
			return deleted, retained, err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM kg_nodes WHERE id=? AND protected=0`, node.ID); err != nil {
			return deleted, retained, err
		}
		removedEdges = append(removedEdges, edges...)
		removedNodes = append(removedNodes, node.ID)
		deleted++
	}
	if err = tx.Commit(); err != nil {
		return 0, retained, err
	}
	committed = true
	for _, edge := range removedEdges {
		err = errors.Join(err, kg.removeSemanticEdgeIndex(edge.Source, edge.Target, edge.Relation))
	}
	for _, id := range removedNodes {
		err = errors.Join(err, kg.removeSemanticNodeIndex(id))
	}
	return deleted, retained, err
}

func privacyIncidentEdges(tx *sql.Tx, id string) ([]Edge, error) {
	rows, err := tx.Query(`SELECT source,target,relation FROM kg_edges WHERE source=? OR target=?`, id, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var edges []Edge
	for rows.Next() {
		var e Edge
		if err := rows.Scan(&e.Source, &e.Target, &e.Relation); err != nil {
			return nil, err
		}
		edges = append(edges, e)
	}
	return edges, rows.Err()
}
