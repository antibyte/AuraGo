package flows

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// These tables are not installed until a reviewed v3 migration backs up the
// complete v2 database and installs both tables in one transaction. Published
// parents own their pins; run pins independently protect draft-test snapshots.
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

var errBlockPinConflict = errors.New("reusable block version changed or is unavailable")

// verifyBlockPinsTx re-reads each immutable revision inside the caller's write
// transaction. This is required even if resolution happened immediately prior:
// a delete or replacement may have occurred between resolution and admission.
func verifyBlockPinsTx(ctx context.Context, tx *sql.Tx, blocks []resolvedBlock) error {
	if tx == nil || len(blocks) > maxBlockVersions {
		return errBlockPinConflict
	}
	seen := make(map[blockRef]bool, len(blocks))
	for _, block := range blocks {
		if !blockFlowID.MatchString(block.Ref.FlowID) || block.Ref.Revision <= 0 || seen[block.Ref] {
			return errBlockPinConflict
		}
		seen[block.Ref] = true
		var raw string
		if err := tx.QueryRowContext(ctx, `SELECT json FROM flow_versions WHERE flow_id=? AND revision=? AND length(CAST(json AS BLOB))<=?`,
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
