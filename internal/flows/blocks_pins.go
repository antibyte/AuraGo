package flows

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

func verifyExistingBlockVersionPins(ctx context.Context, tx *sql.Tx, parent string, revision int, blocks []resolvedBlock) error {
	if err := verifyBlockPinsTx(ctx, tx, blocks); err != nil {
		return err
	}
	want := make(map[blockRef][32]byte, len(blocks))
	for _, block := range blocks {
		want[block.Ref] = block.Hash
	}
	rows, err := tx.QueryContext(ctx, `SELECT block_flow_id,block_revision,block_hash FROM flow_block_version_pins WHERE parent_flow_id=? AND parent_revision=?`, parent, revision)
	if err != nil {
		return err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var ref blockRef
		var hash []byte
		if err := rows.Scan(&ref.FlowID, &ref.Revision, &hash); err != nil {
			return err
		}
		expected, ok := want[ref]
		if !ok || !bytes.Equal(hash, expected[:]) {
			return errBlockPinConflict
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count != len(want) {
		return errBlockPinConflict
	}
	return nil
}

// The v3 migration backs up v2 and installs both tables transactionally.
// Published parents own their pins; run pins independently protect draft-test snapshots.
const blockVersionPinsSchema = `CREATE TABLE flow_block_version_pins (
	parent_flow_id TEXT NOT NULL,
	parent_revision INTEGER NOT NULL,
	block_flow_id TEXT NOT NULL,
	block_revision INTEGER NOT NULL,
	block_hash BLOB NOT NULL CHECK(length(block_hash)=32),
	PRIMARY KEY(parent_flow_id,parent_revision,block_flow_id,block_revision),
	FOREIGN KEY(parent_flow_id,parent_revision) REFERENCES flow_versions(flow_id,revision) ON DELETE CASCADE,
	FOREIGN KEY(block_flow_id,block_revision) REFERENCES flow_versions(flow_id,revision) ON DELETE RESTRICT
)`

const blockRunPinsSchema = `CREATE TABLE flow_block_run_pins (
	run_id TEXT NOT NULL REFERENCES flow_runs(id) ON DELETE CASCADE,
	block_flow_id TEXT NOT NULL,
	block_revision INTEGER NOT NULL,
	block_hash BLOB NOT NULL CHECK(length(block_hash)=32),
	PRIMARY KEY(run_id,block_flow_id,block_revision),
	FOREIGN KEY(block_flow_id,block_revision) REFERENCES flow_versions(flow_id,revision) ON DELETE RESTRICT
)`

const blockVersionPinsIndex = `CREATE INDEX idx_block_version_pins_child ON flow_block_version_pins(block_flow_id,block_revision)`
const blockRunPinsIndex = `CREATE INDEX idx_block_run_pins_child ON flow_block_run_pins(block_flow_id,block_revision)`

var errBlockPinConflict = fmt.Errorf("%w: reusable block version changed", ErrBlockUnavailable)

// verifyBlockPinsTx re-reads each immutable revision inside the caller's write
// transaction. This is required even if resolution happened immediately prior:
// a delete or replacement may have occurred between resolution and admission.
func verifyBlockPinsTx(ctx context.Context, tx *sql.Tx, blocks []resolvedBlock) error {
	if tx == nil || len(blocks) > maxBlockVersions {
		return errBlockPinConflict
	}
	seen := make(map[blockRef]bool, len(blocks))
	for _, block := range blocks {
		if !blockFlowID.MatchString(block.Ref.FlowID) || block.Ref.Revision <= 0 || seen[block.Ref] || block.Document == nil {
			return errBlockPinConflict
		}
		seen[block.Ref] = true
		snapshot, err := json.Marshal(block.Document)
		if err != nil || sha256.Sum256(snapshot) != block.Hash {
			return errBlockPinConflict
		}
		var raw string
		query := `SELECT v.json FROM flow_versions v JOIN flows f ON f.id=v.flow_id
			WHERE v.flow_id=? AND v.revision=? AND f.kind='block' AND f.mission_id='' AND length(CAST(v.json AS BLOB))<=?
			AND (` + validStoredDocumentSQL("f.draft_json") + `) AND (f.live_json='' OR (` + validStoredDocumentSQL("f.live_json") + `))`
		if err := tx.QueryRowContext(ctx, query,
			block.Ref.FlowID, block.Ref.Revision, MaxDocumentBytes).Scan(&raw); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errBlockPinConflict
			}
			return fmt.Errorf("read reusable block version: %w", err)
		}
		doc, err := ParseFlow([]byte(raw))
		if err != nil || doc.ID != block.Ref.FlowID || doc.Kind != KindBlock {
			return errBlockPinConflict
		}
		canonical, err := json.Marshal(doc)
		if err != nil || sha256.Sum256(canonical) != block.Hash {
			return errBlockPinConflict
		}
	}
	// Every published intermediate block owns the exact transitive hashes it
	// was published with; a fresh lookup must not silently substitute a child.
	byRef := make(map[blockRef]resolvedBlock, len(blocks))
	for _, block := range blocks {
		byRef[block.Ref] = block
	}
	for _, block := range blocks {
		seen := make(map[blockRef]bool)
		var descendants []resolvedBlock
		var visit func(*Flow) error
		visit = func(doc *Flow) error {
			for _, node := range doc.Nodes {
				if node.Type != blockCallType && node.Type != blockEachType {
					continue
				}
				ref, err := blockReference(node)
				if err != nil || ref == block.Ref {
					return errBlockPinConflict
				}
				child, ok := byRef[ref]
				if !ok {
					return errBlockPinConflict
				}
				if seen[ref] {
					continue
				}
				seen[ref] = true
				if err := visit(child.Document); err != nil {
					return err
				}
				descendants = append(descendants, child)
			}
			return nil
		}
		if err := visit(block.Document); err != nil {
			return err
		}
		if err := verifyExistingBlockVersionPins(ctx, tx, block.Ref.FlowID, block.Ref.Revision, descendants); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return errBlockPinConflict
		}
	}
	return nil
}

// pinBlockVersionTx runs in the same write transaction that inserts the
// parent published version. Its caller must roll back the whole transaction on
// any failure, including conflict or capacity exhaustion.
func pinBlockVersionTx(ctx context.Context, tx *sql.Tx, parentFlow string, parentRevision int, blocks []resolvedBlock) error {
	if !blockFlowID.MatchString(parentFlow) || parentRevision <= 0 {
		return errBlockPinConflict
	}
	if err := verifyBlockPinsTx(ctx, tx, blocks); err != nil {
		return err
	}
	for _, block := range blocks {
		if _, err := tx.ExecContext(ctx, `INSERT INTO flow_block_version_pins(parent_flow_id,parent_revision,block_flow_id,block_revision,block_hash) VALUES(?,?,?,?,?)`,
			parentFlow, parentRevision, block.Ref.FlowID, block.Ref.Revision, block.Hash[:]); err != nil {
			return fmt.Errorf("pin published block version: %w", err)
		}
	}
	return nil
}

// pinBlockRunTx uses a separate run-owned lifetime for a draft test. Parent
// draft saves or publication do not change its snapshot or release its pins.
func pinBlockRunTx(ctx context.Context, tx *sql.Tx, runID string, blocks []resolvedBlock) error {
	if runID == "" || len(runID) > 128 {
		return errBlockPinConflict
	}
	if err := verifyBlockPinsTx(ctx, tx, blocks); err != nil {
		return err
	}
	for _, block := range blocks {
		if _, err := tx.ExecContext(ctx, `INSERT INTO flow_block_run_pins(run_id,block_flow_id,block_revision,block_hash) VALUES(?,?,?,?)`,
			runID, block.Ref.FlowID, block.Ref.Revision, block.Hash[:]); err != nil {
			return fmt.Errorf("pin test-run block version: %w", err)
		}
	}
	return nil
}
