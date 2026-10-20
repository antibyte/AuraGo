package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func correctionSnapshotsEqual(left, right []byte) bool {
	var a, b any
	if json.Unmarshal(left, &a) != nil || json.Unmarshal(right, &b) != nil {
		return false
	}
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

func correctionItemMatches(item *MemoryCorrectionItem, actual, expected []byte) bool {
	if item.Reference.Store == "ltm" {
		var a, b correctionSnapshot
		if json.Unmarshal(actual, &a) != nil || json.Unmarshal(expected, &b) != nil || a.Meta == nil || b.Meta == nil {
			return false
		}
		for _, m := range []*MemoryMeta{a.Meta, b.Meta} {
			m.AccessCount = 0
			m.LastAccessed = ""
			m.LastEventAt = ""
			m.UsefulCount = 0
			m.UselessCount = 0
			m.LastEffectivenessAt = ""
		}
		x, _ := json.Marshal(a)
		y, _ := json.Marshal(b)
		return correctionSnapshotsEqual(x, y)
	}
	if item.Reference.Store != "kg_node" {
		return correctionSnapshotsEqual(actual, expected)
	}
	var a, b correctionSnapshot
	if json.Unmarshal(actual, &a) != nil || json.Unmarshal(expected, &b) != nil || a.Node == nil || b.Node == nil {
		return false
	}
	// Independent properties on the same entity may be corrected in separate steps.
	return a.Revision == b.Revision && a.Value == b.Value && a.Node.Protected == b.Node.Protected && a.Node.Properties["source"] == b.Node.Properties["source"] && a.Node.Properties["type"] == b.Node.Properties["type"]
}

// Apply replays only the stored plan. Each database commits its own receipt with its mutation.
func (c *MemoryCorrectionService) Apply(ctx context.Context, id, sessionID string) (result *MemoryCorrection, resultErr error) {
	if c.Memory == nil {
		return nil, fmt.Errorf("memory unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	c.Memory.correctionMu.Lock()
	defer c.Memory.correctionMu.Unlock()
	c.Memory.automaticMemoryMu.Lock()
	defer c.Memory.automaticMemoryMu.Unlock()
	if err := rejectDeletedChatSessionEvidence(ctx, c.Memory.db, sessionID); err != nil {
		return nil, err
	}
	correction, err := c.Memory.MemoryCorrection(id, sessionID)
	if err != nil {
		return nil, err
	}
	if correction.Status == "canceled" {
		return correction, fmt.Errorf("memory correction has been canceled")
	}
	// Check every gate and snapshot before the first mutation, including on resumed runs.
	for i := range correction.Items {
		item := &correction.Items[i]
		if err := ctx.Err(); err != nil {
			return correction, err
		}
		if err := c.checkAccess(item.Reference); err != nil {
			return c.blockedCorrection(ctx, correction, item, err)
		}
		if item.State == "done" {
			if err := c.verifyCorrectionItem(ctx, item); err != nil {
				return c.blockedCorrection(ctx, correction, item, err)
			}
			continue
		}
		if item.Reference.Store == "kg_node" || item.Reference.Store == "kg_edge" {
			var receipt string
			err := c.Graph.db.QueryRowContext(ctx, `SELECT after_json FROM kg_correction_receipts WHERE id=?`, id+":"+strconv.Itoa(i)).Scan(&receipt)
			if err == nil {
				item.After = json.RawMessage(receipt)
				if err := c.verifyCorrectionItem(ctx, item); err != nil {
					return c.blockedCorrection(ctx, correction, item, err)
				}
				continue
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return correction, err
			}
		}
		snap, err := c.snapshot(ctx, c.Memory.db, item.Reference)
		if err != nil {
			return c.blockedCorrection(ctx, correction, item, err)
		}
		if err := c.validateChange(ctx, item.MemoryCorrectionChange, snap, correction.Evidence); err != nil {
			return c.blockedCorrection(ctx, correction, item, err)
		}
		encoded, _ := json.Marshal(snap)
		if !correctionItemMatches(item, encoded, item.Snapshot) {
			return c.blockedCorrection(ctx, correction, item, fmt.Errorf("target changed since preview; prepare a new correction"))
		}
	}
	// Prepare all embeddings before acquiring the publication lease or staging IDs.
	if vector, ok := c.Vector.(*ChromemVectorDB); ok {
		ctx, err = vector.prepareCorrectionVectors(ctx, correction)
		if err != nil {
			return correction, err
		}
	}
	ctx, release, err := c.Memory.BeginMemoryPublication(ctx)
	if err != nil {
		return correction, err
	}
	defer release()
	marker := "snapshot.publication.correction." + correction.ID
	if err := c.Memory.SetMemoryMaintenanceState(marker, `{"version":1,"status":"publishing"}`); err != nil {
		return correction, err
	}
	complete := false
	defer func() {
		if complete {
			_, err := c.Memory.db.Exec(`DELETE FROM memory_maintenance_meta WHERE key=?`, marker)
			resultErr = errors.Join(resultErr, err)
		}
	}()
	ctx = context.WithValue(ctx, correctionDeferredIndexKey{}, true)
	for i := range correction.Items {
		item := &correction.Items[i]
		if item.Reference.Store == "ltm" && item.Operation == "replace" && item.State != "done" {
			if err := c.stageCorrectionVector(ctx, correction, item); err != nil {
				return c.correctionFailure(ctx, correction, item, err)
			}
		}
	}
	for i := range correction.Items {
		item := &correction.Items[i]
		if item.State == "done" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return c.correctionFailure(ctx, correction, item, err)
		}
		if item.Reference.Store == "kg_node" || item.Reference.Store == "kg_edge" {
			after, err := c.Graph.applyCorrection(ctx, id+":"+strconv.Itoa(i), correction, item)
			if len(after) > 0 {
				item.After = after
				item.State = "index_pending"
			}
			if err != nil {
				return c.correctionFailure(ctx, correction, item, err)
			}
			item.After, item.State, item.Error = after, "done", ""
			err = saveMemoryCorrection(ctx, c.Memory.db, correction)
			if err != nil {
				return correction, err
			}
		} else if err := c.applyLocalCorrection(ctx, correction, item); err != nil {
			// The SQLite journal, not the in-memory plan, decides whether Commit succeeded.
			persisted, readErr := c.Memory.MemoryCorrection(id, sessionID)
			if readErr != nil {
				return nil, errors.Join(err, readErr)
			}
			return c.correctionFailure(ctx, persisted, &persisted.Items[i], err)
		}
	}
	// All authoritative database mutations have committed. Derived semantic
	// indexing may call a provider, so it runs after releasing the snapshot lease.
	release()
	ctx = context.WithValue(ctx, correctionDeferredIndexKey{}, false)
	for i := range correction.Items {
		item := &correction.Items[i]
		if item.Reference.Store == "kg_node" || item.Reference.Store == "kg_edge" {
			var snapshot correctionSnapshot
			if err := json.Unmarshal(item.After, &snapshot); err != nil {
				return correction, err
			}
			if err := c.Graph.finishCorrectionIndex(ctx, item, snapshot); err != nil {
				item.State = "index_pending"
				return c.correctionFailure(ctx, correction, item, err)
			}
		}
		if err := c.verifyCorrectionItem(ctx, item); err != nil {
			return c.correctionFailure(ctx, correction, &correction.Items[i], err)
		}
	}
	correction.Status = "applied"
	if len(correction.Review) > 0 || correction.ReviewLimited {
		correction.Status = "applied_with_review"
	}
	if err := saveMemoryCorrection(ctx, c.Memory.db, correction); err != nil {
		return correction, err
	}
	complete = true
	return correction, nil
}

func (c *MemoryCorrectionService) blockedCorrection(ctx context.Context, correction *MemoryCorrection, item *MemoryCorrectionItem, cause error) (*MemoryCorrection, error) {
	correction.Status = "blocked"
	// Keep a committed receipt even when a later edit or permission change blocks replay.
	if item.State != "done" && len(item.After) == 0 {
		item.State = "blocked"
	}
	item.Error = "permission, protection or target revision changed; review before retry"
	return correction, errors.Join(cause, saveMemoryCorrection(ctx, c.Memory.db, correction))
}

func (c *MemoryCorrectionService) correctionFailure(ctx context.Context, correction *MemoryCorrection, item *MemoryCorrectionItem, cause error) (*MemoryCorrection, error) {
	correction.Status = "incomplete"
	item.Error = "correction step incomplete; inspect current state before retry"
	// A cancelled caller must not prevent recording an already committed mutation.
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), knowledgeGraphWriteTimeout)
	defer cancel()
	return correction, errors.Join(cause, saveMemoryCorrection(writeCtx, c.Memory.db, correction))
}

func (c *MemoryCorrectionService) stageCorrectionVector(ctx context.Context, correction *MemoryCorrection, item *MemoryCorrectionItem) error {
	writer, ok := c.Vector.(correctionVectorWriter)
	if !ok {
		return fmt.Errorf("vector backend does not support correction staging")
	}
	tx, err := c.Memory.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO memory_correction_vectors(doc_id,correction_id) VALUES(?,?) ON CONFLICT(doc_id) DO NOTHING`, item.ReplacementID, correction.ID); err != nil {
		return err
	}
	var owner string
	var published bool
	if err = tx.QueryRowContext(ctx, `SELECT correction_id,published FROM memory_correction_vectors WHERE doc_id=?`, item.ReplacementID).Scan(&owner, &published); err != nil {
		return err
	}
	if owner != correction.ID || published {
		return fmt.Errorf("replacement publication ownership changed")
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO memory_meta(doc_id,verification_status,archived_at,archived_reason,source_type) VALUES(?,'archived',CURRENT_TIMESTAMP,?,'memory_correction') ON CONFLICT(doc_id) DO NOTHING`, item.ReplacementID, "staged correction "+correction.ID); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	meta, err := c.Memory.GetMemoryMeta(item.ReplacementID)
	if err != nil {
		return err
	}
	if !IsMemoryArchived(meta) || meta.ArchivedReason != "staged correction "+correction.ID {
		return fmt.Errorf("staged vector metadata changed")
	}
	return writer.StoreCorrectionDocument(ctx, item.ReplacementID, item.Value)
}

func (c *MemoryCorrectionService) applyLocalCorrection(ctx context.Context, correction *MemoryCorrection, item *MemoryCorrectionItem) error {
	tx, err := c.Memory.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE memory_corrections SET id=id WHERE id=?`, correction.ID); err != nil {
		return err
	}
	current, err := c.snapshot(ctx, tx, item.Reference)
	if err != nil {
		return err
	}
	encoded, _ := json.Marshal(current)
	if !correctionItemMatches(item, encoded, item.Snapshot) {
		return fmt.Errorf("target changed before correction commit")
	}
	ref := item.Reference
	if item.Operation == "replace" && (ref.Store == "core" || ref.Store == "ltm") {
		if _, err = tx.ExecContext(ctx, `DELETE FROM memory_correction_guards WHERE identity=?`, correctionIdentity(item.Value)); err != nil {
			return err
		}
	}
	switch ref.Store {
	case "learning":
		l := *current.Learning
		if item.Operation == "archive" {
			l.Status = "archived"
		} else {
			var replacement Learning
			replacement, err = supersedeLearning(ctx, tx, l, item.Value, correction.Reason, "correction:"+correction.ID)
			item.ReplacementID = replacement.ID
			break
		}
		l.Revision++
		l.UpdatedAt = time.Now().UTC()
		err = saveLearning(ctx, tx, l, correction.Reason, "correction:"+correction.ID)
	case "core":
		if item.Operation == "archive" {
			_, err = tx.ExecContext(ctx, `DELETE FROM core_memory WHERE id=?`, ref.ID)
		} else if current.Absent {
			_, err = tx.ExecContext(ctx, `INSERT INTO core_memory(id,fact,normalized_fact) VALUES(?,?,?)`, ref.ID, item.Value, normalizeCoreMemoryFactForDedupe(item.Value))
		} else {
			_, err = tx.ExecContext(ctx, `UPDATE core_memory SET fact=?,normalized_fact=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`, item.Value, normalizeCoreMemoryFactForDedupe(item.Value), ref.ID)
		}
	case "profile":
		if item.Operation == "archive" {
			_, err = tx.ExecContext(ctx, `DELETE FROM user_profile WHERE category=? AND key=?`, ref.Category, ref.Key)
		} else if current.Absent {
			if err = checkCorrectionProfileCapacity(ctx, tx, ref.Category); err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO user_profile(category,key,value,confidence,source) VALUES(?,?,?,3,'memory_correction')`, ref.Category, ref.Key, normalizeProfileValue(item.Value))
		} else {
			_, err = tx.ExecContext(ctx, `UPDATE user_profile SET value=?,confidence=MAX(confidence,3),source='memory_correction',updated_at=CURRENT_TIMESTAMP WHERE category=? AND key=?`, normalizeProfileValue(item.Value), ref.Category, ref.Key)
		}
		if err == nil {
			_, err = tx.ExecContext(ctx, `INSERT INTO memory_profile_corrections(category,key,correction_id) VALUES(?,?,?) ON CONFLICT(category,key) DO UPDATE SET correction_id=excluded.correction_id`, ref.Category, ref.Key, correction.ID)
		}
	case "ltm":
		if item.Operation == "replace" {
			body, readErr := c.Vector.GetByIDFromCollection(item.ReplacementID, "aurago_memories")
			if readErr != nil {
				return readErr
			}
			if body != item.Value {
				return fmt.Errorf("staged replacement content changed")
			}
			publication, writeErr := tx.ExecContext(ctx, `UPDATE memory_correction_vectors SET published=1 WHERE doc_id=? AND correction_id=? AND published=0`, item.ReplacementID, correction.ID)
			if writeErr != nil {
				return writeErr
			}
			if n, err := publication.RowsAffected(); err != nil || n != 1 {
				return fmt.Errorf("replacement publication receipt missing or changed: %w", errors.Join(err, sql.ErrNoRows))
			}
			result, writeErr := tx.ExecContext(ctx, `UPDATE memory_meta SET verification_status='confirmed',source_type=?,source_reliability=1,extraction_confidence=1,archived_at=NULL,archived_reason='',last_reviewed_at=CURRENT_TIMESTAMP,review_note=?,last_event_at=CURRENT_TIMESTAMP WHERE doc_id=? AND archived_reason=? AND protected=0 AND keep_forever=0`, correctionReplacementSource(current.Meta), correction.Reason, item.ReplacementID, "staged correction "+correction.ID)
			if writeErr != nil {
				return writeErr
			}
			n, countErr := result.RowsAffected()
			if countErr != nil {
				return countErr
			}
			if n != 1 {
				return fmt.Errorf("replacement metadata changed")
			}
		}
		_, err = tx.ExecContext(ctx, `UPDATE memory_meta SET verification_status='archived',archived_at=CURRENT_TIMESTAMP,archived_reason=?,last_reviewed_at=CURRENT_TIMESTAMP,review_note=?,last_event_at=CURRENT_TIMESTAMP WHERE doc_id=?`, correction.Reason, correction.Reason, ref.ID)
		if err == nil {
			_, err = tx.ExecContext(ctx, `INSERT INTO memory_curation_events(doc_id,action,actor,previous_status,new_status,reason,dry_run) VALUES(?,'archive','memory_correction',?,'archived',?,0)`, ref.ID, current.Meta.VerificationStatus, correction.Reason)
		}
		if err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE memory_conflicts SET status='resolved',superseded_doc_id=?,winning_doc_id=CASE WHEN ?<>'' THEN ? WHEN doc_id_left=? THEN doc_id_right ELSE doc_id_left END,reason=?,resolved_at=CURRENT_TIMESTAMP WHERE status='open' AND (doc_id_left=? OR doc_id_right=?)`, ref.ID, item.ReplacementID, item.ReplacementID, ref.ID, correction.Reason, ref.ID, ref.ID)
		}
	}
	if err != nil {
		return err
	}
	if item.Operation == "archive" {
		_, err = tx.ExecContext(ctx, `INSERT INTO memory_correction_tombstones(reference,correction_id,previous_value) VALUES(?,?,?) ON CONFLICT(reference) DO UPDATE SET correction_id=excluded.correction_id,previous_value=excluded.previous_value`, correctionReferenceKey(ref), correction.ID, item.Before)
	} else {
		_, err = tx.ExecContext(ctx, `DELETE FROM memory_correction_tombstones WHERE reference=?`, correctionReferenceKey(ref))
	}
	if err != nil {
		return err
	}
	// Short scalar profile values are field-scoped; full facts get a shared extraction guard.
	if ref.Store == "core" || ref.Store == "ltm" {
		if _, err = tx.ExecContext(ctx, `INSERT INTO memory_correction_guards(identity,correction_id) VALUES(?,?) ON CONFLICT(identity) DO UPDATE SET correction_id=excluded.correction_id`, correctionIdentity(item.Before), correction.ID); err != nil {
			return err
		}
		if item.Operation == "replace" {
			if _, err = tx.ExecContext(ctx, `DELETE FROM memory_correction_guards WHERE identity=?`, correctionIdentity(item.Value)); err != nil {
				return err
			}
		}
	}
	item.State, item.Error = "done", ""
	item.After, _ = json.Marshal(map[string]string{"value": item.Value, "operation": item.Operation})
	if err = saveMemoryCorrection(ctx, tx, correction); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		// Re-read receipts before a retry; never retire either copy after an uncertain commit.
		item.State = "commit_unknown"
		return err
	}
	return nil
}

func (c *MemoryCorrectionService) verifyCorrectionItem(ctx context.Context, item *MemoryCorrectionItem) error {
	ref := item.Reference
	switch ref.Store {
	case "learning":
		old, err := c.Memory.Learning(ctx, ref.ID, c.LearningScope)
		if err != nil {
			return err
		}
		if item.Operation == "archive" && old.Status == "archived" {
			return nil
		}
		if item.Operation == "replace" && old.Status == "superseded" && old.SupersededBy == item.ReplacementID {
			next, err := c.Memory.Learning(ctx, item.ReplacementID, c.LearningScope)
			if err == nil && next.Status == "active" && next.Statement == item.Value {
				return nil
			}
		}
		return fmt.Errorf("learning no longer matches committed correction")
	case "ltm":
		meta, err := c.Memory.GetMemoryMeta(ref.ID)
		if err != nil {
			return err
		}
		if !IsMemoryArchived(meta) {
			return fmt.Errorf("old long-term memory remains active")
		}
		if item.Operation == "replace" {
			var published bool
			if err := c.Memory.db.QueryRowContext(ctx, `SELECT published FROM memory_correction_vectors WHERE doc_id=?`, item.ReplacementID).Scan(&published); err != nil {
				return err
			}
			if !published {
				return fmt.Errorf("replacement publication is incomplete")
			}
			m, err := c.Memory.GetMemoryMeta(item.ReplacementID)
			if err != nil {
				return err
			}
			if IsMemoryArchived(m) {
				return fmt.Errorf("replacement remains archived")
			}
			body, err := c.Vector.GetByIDFromCollection(item.ReplacementID, "aurago_memories")
			if err != nil {
				return err
			}
			if body != item.Value {
				return fmt.Errorf("replacement differs from saved plan")
			}
		}
	case "kg_node", "kg_edge":
		snapshot, err := c.Graph.correctionSnapshot(ctx, c.Graph.db, ref)
		if err != nil {
			return err
		}
		actual, _ := json.Marshal(snapshot)
		if !correctionItemMatches(item, actual, item.After) {
			return fmt.Errorf("graph verification differs from committed correction")
		}
		if ref.Store == "kg_edge" && item.Operation == "replace" {
			var accepted bool
			if err := c.Graph.db.QueryRowContext(ctx, `SELECT COUNT(*)=1 FROM kg_edges WHERE source=? AND target=? AND relation=? AND status='accepted' AND json_extract(properties,'$.memory_correction_id')=?`, ref.ID, item.Value, ref.Relation, item.ReplacementID).Scan(&accepted); err != nil {
				return err
			}
			if !accepted {
				return fmt.Errorf("replacement graph claim is no longer active")
			}
		}
	default:
		snapshot, err := c.snapshot(ctx, c.Memory.db, ref)
		if item.Operation == "archive" {
			if errors.Is(err, sql.ErrNoRows) || (err == nil && snapshot.Absent) {
				return nil
			}
			if err != nil {
				return err
			}
			return fmt.Errorf("archived fact remains active")
		}
		if err != nil {
			return err
		}
		if strings.TrimSpace(snapshot.Value) != strings.TrimSpace(item.Value) {
			return fmt.Errorf("corrected fact differs from saved plan")
		}
	}
	return nil
}
