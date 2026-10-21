package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	ContactFencePending          = "pending"
	ContactFenceReconcilePending = "reconcile_pending"
	ContactFenceCommitted        = "committed"
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
	if err := kg.ensureContactSourceFence(ctx); err != nil {
		return err
	}
	if _, err := kg.db.ExecContext(ctx, `INSERT OR IGNORE INTO deleted_contact_sources(contact_id,operation_token,state) VALUES(?,'',?)`, id, ContactFenceCommitted); err != nil {
		return fmt.Errorf("record contact deletion fence: %w", err)
	}
	return nil
}

func (kg *KnowledgeGraph) ensureContactSourceFence(ctx context.Context) error {
	var migratedColumns int
	if err := kg.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('deleted_contact_sources') WHERE name IN ('operation_token','state')`).Scan(&migratedColumns); err != nil {
		return fmt.Errorf("inspect contact fence migration: %w", err)
	}
	if migratedColumns != 2 {
		var populated bool
		if err := kg.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM kg_nodes) OR EXISTS(SELECT 1 FROM kg_edges) OR EXISTS(SELECT 1 FROM kg_claims)`).Scan(&populated); err != nil {
			return fmt.Errorf("inspect graph before contact fence migration: %w", err)
		}
		if populated {
			if _, err := backupMainDatabase(kg.db, "contact-fences-v2"); err != nil {
				return fmt.Errorf("back up graph before contact fence migration: %w", err)
			}
		}
	}
	if _, err := kg.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS deleted_contact_sources(
		contact_id TEXT PRIMARY KEY, deleted_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		operation_token TEXT NOT NULL DEFAULT '', state TEXT NOT NULL DEFAULT 'committed')`); err != nil {
		return fmt.Errorf("create contact deletion fence: %w", err)
	}
	for _, column := range []string{
		`ALTER TABLE deleted_contact_sources ADD COLUMN operation_token TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE deleted_contact_sources ADD COLUMN state TEXT NOT NULL DEFAULT 'committed'`,
	} {
		if _, err := kg.db.ExecContext(ctx, column); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return fmt.Errorf("migrate contact deletion fence: %w", err)
		}
	}
	return nil
}

// BeginContactSourceDeletion persists a token-owned temporary fence. Existing
// legacy or committed fences are deliberately never replaced automatically.
func (kg *KnowledgeGraph) BeginContactSourceDeletion(ctx context.Context, id, token string) error {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(token) == "" {
		return fmt.Errorf("contact ID and operation token are required")
	}
	if err := kg.ensureContactSourceFence(ctx); err != nil {
		return err
	}
	tx, err := kg.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin contact source fence: %w", err)
	}
	defer tx.Rollback()
	var state string
	err = tx.QueryRowContext(ctx, `SELECT state FROM deleted_contact_sources WHERE contact_id=?`, id).Scan(&state)
	if err == nil {
		return fmt.Errorf("contact source is already fenced: %s", state)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read contact source fence: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO deleted_contact_sources(contact_id,operation_token,state) VALUES(?,?,?)`, id, token, ContactFencePending); err != nil {
		return fmt.Errorf("record contact source fence: %w", err)
	}
	return tx.Commit()
}

func (kg *KnowledgeGraph) CommitContactSourceDeletion(ctx context.Context, id, token string) error {
	return kg.updateOwnedContactFence(ctx, id, token, ContactFenceCommitted)
}

func (kg *KnowledgeGraph) MarkContactSourceReconcilePending(ctx context.Context, id, token string) error {
	return kg.updateOwnedContactFence(ctx, id, token, ContactFenceReconcilePending)
}

func (kg *KnowledgeGraph) updateOwnedContactFence(ctx context.Context, id, token, state string) error {
	if err := kg.ensureContactSourceFence(ctx); err != nil {
		return err
	}
	result, err := kg.db.ExecContext(ctx, `UPDATE deleted_contact_sources SET state=? WHERE contact_id=? AND operation_token=? AND (state IN (?,?) OR state=?)`, state, id, token, ContactFencePending, ContactFenceReconcilePending, state)
	if err != nil {
		return fmt.Errorf("update contact source fence: %w", err)
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		if err != nil {
			return fmt.Errorf("inspect contact source fence update: %w", err)
		}
		return fmt.Errorf("contact source fence ownership changed")
	}
	return nil
}

// ReleaseContactSourceDeletion removes only a temporary fence still owned by
// the failed operation. A committed or legacy fence cannot be compensated.
func (kg *KnowledgeGraph) ReleaseContactSourceDeletion(ctx context.Context, id, token string) error {
	if err := kg.ensureContactSourceFence(ctx); err != nil {
		return err
	}
	result, err := kg.db.ExecContext(ctx, `DELETE FROM deleted_contact_sources WHERE contact_id=? AND operation_token=? AND state IN (?,?)`, id, token, ContactFencePending, ContactFenceReconcilePending)
	if err != nil {
		return fmt.Errorf("release contact source fence: %w", err)
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		if err != nil {
			return fmt.Errorf("inspect contact source fence release: %w", err)
		}
		return fmt.Errorf("contact source fence ownership changed")
	}
	return nil
}

func (kg *KnowledgeGraph) ContactSourceFence(ctx context.Context, id string) (token, state string, exists bool, err error) {
	if err = kg.ensureContactSourceFence(ctx); err != nil {
		return "", "", false, err
	}
	err = kg.db.QueryRowContext(ctx, `SELECT operation_token,state FROM deleted_contact_sources WHERE contact_id=?`, id).Scan(&token, &state)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, fmt.Errorf("read contact source fence: %w", err)
	}
	return token, state, true, nil
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
	if len(exclusiveEdges) > 0 || len(removedNodes) > 0 {
		if err = cleanupKGClaimsAndOrphanEvidenceTx(tx); err != nil {
			return deleted, retained, err
		}
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
