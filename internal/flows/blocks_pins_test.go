package flows

import (
	"context"
	"errors"
	"testing"
)

func TestBlockPinsProtectPublishedAndDraftRunDependencies(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	for _, stmt := range []string{blockVersionPinsSchema, blockRunPinsSchema, blockVersionPinsIndex, blockRunPinsIndex} {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	child := blockTestDoc("flow_child")
	parent := sampleFlow("flow_parent")
	for _, doc := range []*Flow{child, parent} {
		if _, err := s.CreateFlow(ctx, doc, "", storeNow); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Publish(ctx, doc.ID, 1, storeNow); err != nil {
			t.Fatal(err)
		}
	}
	root := blockTestDoc("flow_root", blockRef{child.ID, 1})
	blocks, err := resolveBlockDependencies(ctx, root, s.GetVersion)
	if err != nil || len(blocks) != 1 {
		t.Fatalf("resolve: %+v %v", blocks, err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO flow_runs(id,flow_id,revision,mode,status,started_at) VALUES('run_draft',?,0,'test','running',?)`, parent.ID, formatTime(storeNow)); err != nil {
		t.Fatal(err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE flows_meta SET value=value WHERE key='schema_version'`); err != nil {
		t.Fatal(err)
	}
	if err = pinBlockVersionTx(ctx, tx, parent.ID, 1, blocks); err != nil {
		t.Fatal(err)
	}
	if err = pinBlockRunTx(ctx, tx, "run_draft", blocks); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM flow_versions WHERE flow_id=? AND revision=1`, child.ID); err == nil {
		t.Fatal("referenced block version was pruned")
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM flow_versions WHERE flow_id=? AND revision=1`, parent.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM flow_versions WHERE flow_id=? AND revision=1`, child.ID); err == nil {
		t.Fatal("draft run pin was released with the parent version")
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM flow_runs WHERE id='run_draft'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM flow_versions WHERE flow_id=? AND revision=1`, child.ID); err != nil {
		t.Fatalf("released block version remains pinned: %v", err)
	}
}

func TestBlockPinsRejectChangedOrMissingVersion(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	for _, stmt := range []string{blockVersionPinsSchema, blockRunPinsSchema, blockVersionPinsIndex, blockRunPinsIndex} {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	child := blockTestDoc("flow_child")
	if _, err := s.CreateFlow(ctx, child, "", storeNow); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Publish(ctx, child.ID, 1, storeNow); err != nil {
		t.Fatal(err)
	}
	root := blockTestDoc("flow_root", blockRef{child.ID, 1})
	blocks, err := resolveBlockDependencies(ctx, root, s.GetVersion)
	if err != nil {
		t.Fatal(err)
	}
	blocks[0].Hash[0] ^= 1
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE flows_meta SET value=value WHERE key='schema_version'`); err != nil {
		t.Fatal(err)
	}
	if err = verifyBlockPinsTx(ctx, tx, blocks); !errors.Is(err, errBlockPinConflict) {
		t.Fatalf("changed hash: %v", err)
	}
	valid := blocks[0]
	valid.Hash[0] ^= 1
	missing := blocks[0]
	missing.Ref.Revision = 2
	if err = pinBlockRunTx(ctx, tx, "run_test", []resolvedBlock{valid, missing}); !errors.Is(err, errBlockPinConflict) {
		t.Fatalf("partially invalid pin set: %v", err)
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM flow_block_run_pins`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed pin set left rows: %d, %v", count, err)
	}
	blocks[0].Ref.Revision = 2
	if err = verifyBlockPinsTx(ctx, tx, blocks); !errors.Is(err, errBlockPinConflict) {
		t.Fatalf("missing revision: %v", err)
	}
}
