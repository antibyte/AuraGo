package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	privacyTargetCorrectionReceipt = "kg_correction_receipt"
	privacyTargetCorrectionVector  = "correction_vector"
	privacyTargetGraphTranscript   = "kg_transcript_scrub"
)

// sessionCorrectionCleanupTargets returns only opaque receipts and vector IDs.
// These targets are persisted before correction payloads are scrubbed so an
// unavailable external store cannot make a deletion appear complete.
func (s *SQLiteMemory) sessionCorrectionCleanupTargets(ctx context.Context, sessionID string) ([]privacyDeletionTarget, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,payload FROM memory_corrections WHERE session_id=? ORDER BY id`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("read session corrections for privacy cleanup: %w", err)
	}
	corrections := make([]MemoryCorrection, 0)
	for rows.Next() {
		var id, payload string
		if err := rows.Scan(&id, &payload); err != nil {
			rows.Close()
			return nil, fmt.Errorf("read session correction: %w", err)
		}
		var correction MemoryCorrection
		if err := json.Unmarshal([]byte(payload), &correction); err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode session correction for privacy cleanup: %w", err)
		}
		if correction.ID != id {
			rows.Close()
			return nil, fmt.Errorf("session correction identity is inconsistent")
		}
		corrections = append(corrections, correction)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate session corrections: %w", err)
	}
	rows.Close()

	targets := make([]privacyDeletionTarget, 0)
	seen := map[string]bool{}
	add := func(target privacyDeletionTarget) {
		key := target.Store + "\x00" + target.TargetID
		if target.TargetID == "" || seen[key] {
			return
		}
		seen[key] = true
		targets = append(targets, target)
	}
	for _, correction := range corrections {
		for index, item := range correction.Items {
			if item.Reference.Store == "kg_node" || item.Reference.Store == "kg_edge" {
				add(privacyDeletionTarget{
					Store: privacyTargetCorrectionReceipt, TargetID: correction.ID + ":" + strconv.Itoa(index),
					SourceMarker: correction.ID, State: "pending", ReasonCode: "correction_receipt_cleanup",
				})
			}
		}
		vectorRows, err := s.db.QueryContext(ctx, `SELECT doc_id FROM memory_correction_vectors WHERE correction_id=? AND published=0 ORDER BY doc_id`, correction.ID)
		if err != nil {
			return nil, fmt.Errorf("read staged correction vectors for privacy cleanup: %w", err)
		}
		for vectorRows.Next() {
			var id string
			if err := vectorRows.Scan(&id); err != nil {
				vectorRows.Close()
				return nil, fmt.Errorf("read staged correction vector: %w", err)
			}
			add(privacyDeletionTarget{
				Store: privacyTargetCorrectionVector, TargetID: id,
				SourceMarker: correction.ID, ExpectedFingerprint: privacyFingerprint(correction.ID, id, "unpublished"),
				State: "pending", ReasonCode: "unpublished_correction_vector",
			})
		}
		if err := vectorRows.Err(); err != nil {
			vectorRows.Close()
			return nil, fmt.Errorf("iterate staged correction vectors: %w", err)
		}
		vectorRows.Close()
	}
	return targets, nil
}

// scrubSessionCorrectionsTx makes every correction from the deleted session
// terminal and removes its evidence and replay snapshots. A committed effect
// stays in its owning store; the durable receipt cleanup targets are handled
// separately by the privacy worker.
func (s *SQLiteMemory) scrubSessionCorrectionsTx(ctx context.Context, tx *sql.Tx, sessionID string, kg *KnowledgeGraph) error {
	rows, err := tx.QueryContext(ctx, `SELECT id,session_id,payload FROM memory_corrections ORDER BY id`)
	if err != nil {
		return fmt.Errorf("read corrections for session evidence scrub: %w", err)
	}
	type record struct{ id, sessionID, payload string }
	records := make([]record, 0)
	for rows.Next() {
		var current record
		if err := rows.Scan(&current.id, &current.sessionID, &current.payload); err != nil {
			rows.Close()
			return fmt.Errorf("scan correction for session evidence scrub: %w", err)
		}
		records = append(records, current)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate corrections for session evidence scrub: %w", err)
	}
	rows.Close()

	for _, current := range records {
		var correction MemoryCorrection
		if err := json.Unmarshal([]byte(current.payload), &correction); err != nil {
			if current.sessionID == sessionID {
				return fmt.Errorf("decode correction during session evidence scrub: %w", err)
			}
			continue
		}
		owned := current.sessionID == sessionID || correction.SessionID == sessionID
		changed := false
		if owned {
			ids := make([]string, 0)
			for index, item := range correction.Items {
				if item.Reference.Store == "kg_node" || item.Reference.Store == "kg_edge" {
					ids = append(ids, correction.ID+":"+strconv.Itoa(index))
					if kg == nil && item.State != "done" {
						return privacyNeedsReconciliation("correction_graph_unavailable", fmt.Errorf("cannot classify an uncompleted graph correction without its receipt store"))
					}
				}
			}
			receipts := map[string]json.RawMessage{}
			if kg != nil && len(ids) > 0 {
				receipts, err = kg.correctionReceipts(ctx, ids)
				if err != nil {
					return privacyNeedsReconciliation("correction_graph_receipts_unavailable", fmt.Errorf("resolve committed graph correction receipts: %w", err))
				}
			}
			for index := range correction.Items {
				item := &correction.Items[index]
				committed := item.State == "done"
				if item.Reference.Store == "kg_node" || item.Reference.Store == "kg_edge" {
					_, hasReceipt := receipts[correction.ID+":"+strconv.Itoa(index)]
					committed = committed || hasReceipt
				}
				if committed {
					item.State = "done"
				} else {
					item.State = "canceled"
				}
				// Completed effects stay in their owning store. The correction
				// row is replay data, so clear its copy even for committed items.
				item.MemoryCorrectionChange = MemoryCorrectionChange{}
				item.Before = ""
				item.Snapshot = nil
				item.After = nil
				item.ReplacementID = ""
				item.Error = ""
			}
			correction.Evidence = nil
			correction.Review = nil
			correction.ReviewLimited = false
			correction.Reason = ""
			correction.RunID = ""
			correction.SessionID = sessionPrivacyMarker(sessionID)
			correction.Status = "canceled"
			changed = true
		}
		if scrubCorrectionLearningSnapshots(&correction, sessionID) {
			changed = true
		}
		if !changed {
			continue
		}
		payload, err := json.Marshal(&correction)
		if err != nil {
			return fmt.Errorf("encode scrubbed correction: %w", err)
		}
		newSessionID := current.sessionID
		if owned {
			newSessionID = sessionPrivacyMarker(sessionID)
		}
		result, err := tx.ExecContext(ctx, `UPDATE memory_corrections SET session_id=?,payload=? WHERE id=? AND session_id=?`, newSessionID, string(payload), current.id, current.sessionID)
		if err != nil {
			return fmt.Errorf("persist scrubbed correction: %w", err)
		}
		if affected, err := result.RowsAffected(); err != nil || affected != 1 {
			if err != nil {
				return fmt.Errorf("persist scrubbed correction result: %w", err)
			}
			return fmt.Errorf("persist scrubbed correction: expected one row, updated %d", affected)
		}
	}
	return nil
}

func scrubCorrectionLearningSnapshots(correction *MemoryCorrection, sessionID string) bool {
	changed := false
	for index := range correction.Items {
		item := &correction.Items[index]
		for _, field := range []*json.RawMessage{&item.Snapshot, &item.After} {
			if len(*field) == 0 {
				continue
			}
			var snapshot correctionSnapshot
			if json.Unmarshal(*field, &snapshot) != nil || snapshot.Learning == nil {
				continue
			}
			if scrubLearningSessionSources(snapshot.Learning, sessionID) {
				encoded, err := json.Marshal(snapshot)
				if err == nil {
					*field = encoded
					changed = true
				}
			}
		}
	}
	return changed
}

func scrubLearningSessionSources(learning *Learning, sessionID string) bool {
	marker := sessionPrivacyMarker(sessionID)
	changed := false
	for index := range learning.Sources {
		source := &learning.Sources[index]
		if source.Session != sessionID {
			continue
		}
		source.Reference = ""
		source.Session = marker
		source.Quote = ""
		source.Hash = ""
		changed = true
	}
	return changed
}

func scrubLearningEvidenceTx(ctx context.Context, tx *sql.Tx, sessionID string) error {
	rows, err := tx.QueryContext(ctx, `SELECT id,payload FROM memory_learnings ORDER BY id`)
	if err != nil {
		return fmt.Errorf("read learning evidence for privacy scrub: %w", err)
	}
	type learningRecord struct{ id, payload string }
	records := make([]learningRecord, 0)
	for rows.Next() {
		var current learningRecord
		if err := rows.Scan(&current.id, &current.payload); err != nil {
			rows.Close()
			return fmt.Errorf("scan learning evidence: %w", err)
		}
		records = append(records, current)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate learning evidence: %w", err)
	}
	rows.Close()
	for _, current := range records {
		var learning Learning
		if err := json.Unmarshal([]byte(current.payload), &learning); err != nil {
			return fmt.Errorf("decode learning evidence for privacy scrub: %w", err)
		}
		if !scrubLearningSessionSources(&learning, sessionID) {
			continue
		}
		if learning.Status == "active" {
			learning.Status = "revalidation"
		}
		learning.Revision++
		learning.UpdatedAt = time.Now().UTC()
		if err := saveLearning(ctx, tx, learning, "", "privacy"); err != nil {
			return fmt.Errorf("persist scrubbed learning evidence: %w", err)
		}
	}

	revisions, err := tx.QueryContext(ctx, `SELECT id,revision,payload FROM memory_learning_revisions ORDER BY id,revision`)
	if err != nil {
		return fmt.Errorf("read learning revisions for privacy scrub: %w", err)
	}
	type revisionRecord struct {
		id       string
		revision int
		payload  string
	}
	allRevisions := make([]revisionRecord, 0)
	for revisions.Next() {
		var current revisionRecord
		if err := revisions.Scan(&current.id, &current.revision, &current.payload); err != nil {
			revisions.Close()
			return fmt.Errorf("scan learning revision for privacy scrub: %w", err)
		}
		allRevisions = append(allRevisions, current)
	}
	if err := revisions.Err(); err != nil {
		revisions.Close()
		return fmt.Errorf("iterate learning revisions: %w", err)
	}
	revisions.Close()
	for _, current := range allRevisions {
		var revision LearningRevision
		if err := json.Unmarshal([]byte(current.payload), &revision); err != nil {
			return fmt.Errorf("decode learning revision for privacy scrub: %w", err)
		}
		if !scrubLearningSessionSources(&revision.Learning, sessionID) {
			continue
		}
		revision.Reason = ""
		payload, err := json.Marshal(revision)
		if err != nil {
			return fmt.Errorf("encode scrubbed learning revision: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE memory_learning_revisions SET payload=? WHERE id=? AND revision=?`, string(payload), current.id, current.revision); err != nil {
			return fmt.Errorf("persist scrubbed learning revision: %w", err)
		}
	}
	return nil
}

func (s *SQLiteMemory) applyPrivacyCorrectionVectorTarget(ctx context.Context, ltm VectorDB, jobID string, target privacyDeletionTarget) (bool, error) {
	cv, ok := ltm.(*ChromemVectorDB)
	if !ok || cv == nil || cv.IsDisabled() {
		return false, privacyNeedsReconciliation("correction_vector_store_unavailable", fmt.Errorf("staged correction vector ownership cannot be verified"))
	}
	if target.SourceMarker == "" || target.ExpectedFingerprint != privacyFingerprint(target.SourceMarker, target.TargetID, "unpublished") {
		return false, privacyNeedsReconciliation("correction_vector_manifest_invalid", fmt.Errorf("staged correction vector manifest is inconsistent"))
	}
	verifyOwner := func(q interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
	}) error {
		var owner string
		var published bool
		if err := q.QueryRowContext(ctx, `SELECT correction_id,published FROM memory_correction_vectors WHERE doc_id=?`, target.TargetID).Scan(&owner, &published); err != nil {
			return err
		}
		if owner != target.SourceMarker || published {
			return fmt.Errorf("staged correction vector owner changed")
		}
		var status, archivedAt, archivedReason, sourceType string
		var protected, keepForever bool
		if err := q.QueryRowContext(ctx, `SELECT COALESCE(verification_status,''),COALESCE(archived_at,''),COALESCE(archived_reason,''),COALESCE(source_type,''),protected,keep_forever FROM memory_meta WHERE doc_id=?`, target.TargetID).Scan(&status, &archivedAt, &archivedReason, &sourceType, &protected, &keepForever); err != nil {
			return err
		}
		if status != "archived" || archivedAt == "" || archivedReason != "staged correction "+target.SourceMarker || sourceType != "memory_correction" || protected || keepForever {
			return fmt.Errorf("staged correction vector metadata changed")
		}
		return nil
	}
	if err := verifyOwner(s.db); err != nil {
		return false, privacyNeedsReconciliation("correction_vector_owner_uncertain", err)
	}
	deleted, err := cv.deleteUnpublishedCorrectionDocument(ctx, target.TargetID, target.SourceMarker)
	if err != nil {
		return false, err
	}
	if !deleted {
		return false, privacyNeedsReconciliation("correction_vector_owner_uncertain", fmt.Errorf("staged correction vector ownership changed"))
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if err := verifyOwner(tx); err != nil {
		return false, privacyNeedsReconciliation("correction_vector_owner_uncertain", err)
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM memory_correction_vectors WHERE doc_id=? AND correction_id=? AND published=0`, target.TargetID, target.SourceMarker)
	if err != nil {
		return false, err
	}
	if changed, err := result.RowsAffected(); err != nil || changed != 1 {
		if err != nil {
			return false, err
		}
		return false, privacyNeedsReconciliation("correction_vector_owner_uncertain", fmt.Errorf("staged correction vector owner changed during cleanup"))
	}
	for _, statement := range []string{`DELETE FROM memory_meta WHERE doc_id=? AND verification_status='archived' AND archived_reason=? AND source_type='memory_correction' AND protected=0 AND keep_forever=0`, `DELETE FROM memory_extraction_sources WHERE doc_id=?`, `DELETE FROM memory_usage_log WHERE memory_id=?`} {
		var args []any
		if strings.Contains(statement, "archived_reason=?") {
			args = []any{target.TargetID, "staged correction " + target.SourceMarker}
		} else {
			args = []any{target.TargetID}
		}
		if _, err := tx.ExecContext(ctx, statement, args...); err != nil {
			return false, err
		}
	}
	if err := markPrivacyTargetStateTx(ctx, tx, jobID, target, "deleted", ""); err != nil {
		return false, err
	}
	return true, tx.Commit()
}
