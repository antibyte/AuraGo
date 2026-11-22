package chibisensei

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"

	"github.com/google/uuid"
)

const keepBatches = 50

// History lists the newest batches with undone = 0, limit clamped to 1..50 (default 20).
func (s *Store) History(ctx context.Context, projectID string, limit int) ([]Batch, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > keepBatches {
		limit = keepBatches
	}
	rows, err := s.db.QueryContext(ctx, `SELECT batch_id, kind, row_count, created FROM batches
		WHERE project_id=? AND undone=0 ORDER BY created DESC, rowid DESC LIMIT ?`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Batch{}
	for rows.Next() {
		var b Batch
		if err := rows.Scan(&b.BatchID, &b.Kind, &b.Rows, &b.Created); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// UndoResult reports an undo.
type UndoResult struct {
	Restored int   `json:"restored"`
	DataRev  int64 `json:"data_rev"`
}

// restoreSnapshot overwrites a row with a snapshot, re-inserting it with the same id when it is gone.
func restoreSnapshot(ctx context.Context, tx *sql.Tx, projectID string, snap rowSnapshot) error {
	var exists int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM dataset_rows WHERE id=? AND project_id=?`, snap.ID, projectID).Scan(&exists)
	found := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var taken int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM dataset_rows WHERE project_id=? AND seq=? AND id!=?`,
		projectID, snap.Seq, snap.ID).Scan(&taken); err != nil {
		return err
	}
	if taken > 0 {
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq),0)+1 FROM dataset_rows WHERE project_id=?`, projectID).Scan(&snap.Seq); err != nil {
			return err
		}
	}
	if found {
		_, err = tx.ExecContext(ctx, `UPDATE dataset_rows SET seq=?,kind=?,messages_json=?,text=?,label=?,split=?,source=?,source_ref=?,batch_id=?,
			tags=?,content_hash=?,prompt_hash=?,est_tokens=?,flags=?,rev=?,created=?,updated=? WHERE id=?`,
			snap.Seq, snap.Kind, snap.MessagesJSON, snap.Text, snap.Label, snap.Split, snap.Source, snap.SourceRef, snap.BatchID,
			snap.Tags, snap.ContentHash, snap.PromptHash, snap.EstTokens, snap.Flags, snap.Rev, snap.Created, snap.Updated, snap.ID)
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO dataset_rows(id,project_id,seq,kind,messages_json,text,label,split,source,source_ref,batch_id,tags,
		content_hash,prompt_hash,est_tokens,flags,rev,created,updated) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		snap.ID, projectID, snap.Seq, snap.Kind, snap.MessagesJSON, snap.Text, snap.Label, snap.Split, snap.Source, snap.SourceRef,
		snap.BatchID, snap.Tags, snap.ContentHash, snap.PromptHash, snap.EstTokens, snap.Flags, snap.Rev, snap.Created, snap.Updated)
	return err
}

// checkUndoable verifies that no later change touched a row of the batch: a row the batch inserted or updated must
// still have the revision the batch left, a row the batch deleted must still be gone.
func checkUndoable(ctx context.Context, tx *sql.Tx, projectID, batchID string) error {
	type entry struct {
		id, rowID, afterRev int64
		op                  string
	}
	seen := map[int64]bool{}
	last := int64(math.MaxInt64)
	for {
		rows, err := tx.QueryContext(ctx, `SELECT id,row_id,op,after_rev FROM row_history WHERE batch_id=? AND project_id=? AND id<?
			ORDER BY id DESC LIMIT 1000`, batchID, projectID, last)
		if err != nil {
			return err
		}
		var page []entry
		for rows.Next() {
			var e entry
			if err := rows.Scan(&e.id, &e.rowID, &e.op, &e.afterRev); err != nil {
				rows.Close()
				return err
			}
			page = append(page, e)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if len(page) == 0 {
			return nil
		}
		last = page[len(page)-1].id
		for _, e := range page {
			if seen[e.rowID] {
				continue // only the newest entry of a row says what the batch left behind
			}
			seen[e.rowID] = true
			var rev int64
			err := tx.QueryRowContext(ctx, `SELECT rev FROM dataset_rows WHERE id=?`, e.rowID).Scan(&rev)
			exists := err == nil
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if e.op == "delete" {
				if exists {
					return ErrUndoConflict
				}
			} else if !exists || rev != e.afterRev {
				return ErrUndoConflict
			}
		}
	}
}

// Undo replays row_history of the batch in descending id order and marks the batch undone.
func (s *Store) Undo(ctx context.Context, projectID, batchID string) (UndoResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return UndoResult{}, err
	}
	defer tx.Rollback()
	p, err := loadProject(ctx, tx, projectID)
	if err != nil {
		return UndoResult{}, err
	}
	var undone int
	err = tx.QueryRowContext(ctx, `SELECT undone FROM batches WHERE batch_id=? AND project_id=?`, batchID, projectID).Scan(&undone)
	if errors.Is(err, sql.ErrNoRows) {
		return UndoResult{}, ErrNotFound
	}
	if err != nil {
		return UndoResult{}, err
	}
	if undone != 0 {
		return UndoResult{}, ErrAlreadyUndone
	}
	if err := checkUndoable(ctx, tx, projectID, batchID); err != nil {
		return UndoResult{}, err
	}
	type op struct {
		id, rowID int64
		op        string
		before    sql.NullString
	}
	restored := 0
	last := int64(math.MaxInt64)
	for {
		rows, err := tx.QueryContext(ctx, `SELECT id,row_id,op,before_json FROM row_history WHERE batch_id=? AND project_id=? AND id<?
			ORDER BY id DESC LIMIT 1000`, batchID, projectID, last)
		if err != nil {
			return UndoResult{}, err
		}
		var page []op
		for rows.Next() {
			var o op
			if err := rows.Scan(&o.id, &o.rowID, &o.op, &o.before); err != nil {
				rows.Close()
				return UndoResult{}, err
			}
			page = append(page, o)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return UndoResult{}, err
		}
		if len(page) == 0 {
			break
		}
		last = page[len(page)-1].id
		for _, o := range page {
			if o.op == "insert" {
				res, err := tx.ExecContext(ctx, `DELETE FROM dataset_rows WHERE id=? AND project_id=?`, o.rowID, projectID)
				if err != nil {
					return UndoResult{}, err
				}
				if n, _ := res.RowsAffected(); n > 0 {
					restored++
				}
				continue
			}
			if !o.before.Valid {
				continue
			}
			var snap rowSnapshot
			if err := json.Unmarshal([]byte(o.before.String), &snap); err != nil {
				return UndoResult{}, err
			}
			if err := restoreSnapshot(ctx, tx, projectID, snap); err != nil {
				return UndoResult{}, err
			}
			restored++
		}
	}
	now := s.stamp()
	if _, err := tx.ExecContext(ctx, `UPDATE batches SET undone=1 WHERE batch_id=?`, batchID); err != nil {
		return UndoResult{}, err
	}
	dataRev := p.DataRev + 1
	if _, err := tx.ExecContext(ctx, `UPDATE projects SET data_rev=?, updated=? WHERE id=?`, dataRev, now, projectID); err != nil {
		return UndoResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return UndoResult{}, err
	}
	s.changed(projectID, dataRev)
	return UndoResult{Restored: restored, DataRev: dataRev}, nil
}

// Dedupe keeps the first row by seq of every content_hash and deletes the others.
func (s *Store) Dedupe(ctx context.Context, projectID string) (BulkResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return BulkResult{}, err
	}
	defer tx.Rollback()
	p, err := loadProject(ctx, tx, projectID)
	if err != nil {
		return BulkResult{}, err
	}
	seen := map[string]bool{}
	var dups []int64
	var last int64
	for {
		rows, err := tx.QueryContext(ctx, `SELECT id, seq, content_hash FROM dataset_rows WHERE project_id=? AND seq>? ORDER BY seq LIMIT 1000`,
			projectID, last)
		if err != nil {
			return BulkResult{}, err
		}
		n := 0
		for rows.Next() {
			var id, seq int64
			var hash string
			if err := rows.Scan(&id, &seq, &hash); err != nil {
				rows.Close()
				return BulkResult{}, err
			}
			last = seq
			n++
			if seen[hash] {
				dups = append(dups, id)
			}
			seen[hash] = true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return BulkResult{}, err
		}
		if n == 0 {
			break
		}
	}
	if len(dups) == 0 {
		return BulkResult{DataRev: p.DataRev}, nil
	}
	batchID := uuid.NewString()
	now := s.stamp()
	for start := 0; start < len(dups); start += 500 {
		end := start + 500
		if end > len(dups) {
			end = len(dups)
		}
		snaps, err := snapshotsByIDs(ctx, tx, projectID, dups[start:end])
		if err != nil {
			return BulkResult{}, err
		}
		if err := deleteSnapshots(ctx, tx, projectID, batchID, snaps, now); err != nil {
			return BulkResult{}, err
		}
	}
	dataRev, err := s.finishBatch(ctx, tx, p, batchID, "dedupe", len(dups), now)
	if err != nil {
		return BulkResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return BulkResult{}, err
	}
	s.changed(projectID, dataRev)
	return BulkResult{Affected: len(dups), BatchID: batchID, DataRev: dataRev}, nil
}

// pruneHistory keeps the newest 50 batches of the project and deletes older ones with their row_history.
func pruneHistory(ctx context.Context, tx *sql.Tx, projectID string) error {
	const old = `SELECT batch_id FROM batches WHERE project_id=? AND open=0 AND batch_id NOT IN (
		SELECT batch_id FROM batches WHERE project_id=? ORDER BY created DESC, rowid DESC LIMIT 50)`
	if _, err := tx.ExecContext(ctx, `DELETE FROM row_history WHERE project_id=? AND batch_id IN (`+old+`)`, projectID, projectID, projectID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM batches WHERE project_id=? AND batch_id IN (`+old+`)`, projectID, projectID, projectID)
	return err
}
