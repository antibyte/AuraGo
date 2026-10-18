package main

import (
	"errors"
	"path/filepath"
	"testing"

	"aurago/internal/memory"
)

func TestMergeCanonicalCorpusResumesOriginalPlanAndBlocksNewPlan(t *testing.T) {
	f := newRepairFixture(t)
	if err := memory.PrepareDocumentCorpus(f.vectors); err != nil {
		t.Fatal(err)
	}
	plan := f.plan(t)
	if len(plan.Groups) != 1 {
		t.Fatalf("canonical corpus preview=%+v", plan)
	}
	completed, failed := applyGroups(t.Context(), f.db, f.vectors, plan, func(stage string) error {
		if stage == "after_sql_commit" {
			return errors.New("interrupted after durable reference commit")
		}
		return nil
	})
	if completed != 0 || len(failed) != 1 {
		t.Fatalf("interruption=%d %+v", completed, failed)
	}
	preview := f.plan(t)
	if len(preview.Groups) != 0 || len(preview.Review) == 0 {
		t.Fatalf("new plan included unfinished IDs: %+v", preview)
	}
	completed, failed = applyGroups(t.Context(), f.db, f.vectors, plan, nil)
	if completed != 1 || len(failed) != 0 || f.count(t) != 1 {
		t.Fatalf("canonical resume=%d %+v count=%d", completed, failed, f.count(t))
	}
	completed, failed = applyGroups(t.Context(), f.db, f.vectors, plan, nil)
	if completed != 0 || len(failed) != 0 {
		t.Fatalf("repeat counted twice=%d %+v", completed, failed)
	}
	// Old gob files are retained, but reopening uses the canonical source and
	// must never resurrect documents deleted by the reviewed original plan.
	if err := memory.PrepareDocumentCorpus(f.vectors); err != nil {
		t.Fatal(err)
	}
	if f.count(t) != 1 {
		t.Fatal("legacy files resurrected a merged document")
	}
}

func TestCanonicalCorpusRepairBackupIsConsistentAndIndependent(t *testing.T) {
	f := newRepairFixture(t)
	if err := memory.PrepareDocumentCorpus(f.vectors); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(f.root, "reports", "backup")
	if err := backupStores(t.Context(), f.db, f.vectors, root); err != nil {
		t.Fatal(err)
	}
	backup, err := openVectors(filepath.Join(root, "vectors"))
	if err != nil {
		t.Fatal(err)
	}
	if backup.Count() != 3 {
		t.Fatalf("backup lost sources: %d", backup.Count())
	}
	if err := backup.Delete(t.Context(), nil, nil, "b"); err != nil {
		t.Fatal(err)
	}
	if backup.Count() != 2 || f.count(t) != 3 {
		t.Fatal("backup mutation affected original canonical source")
	}
}

func TestUnidentifiedLegacyMergeJournalBlocksFreshPlan(t *testing.T) {
	f := newRepairFixture(t)
	f.exec(t, `INSERT INTO memory_maintenance_meta(key,value) VALUES(?,?)`, journalPrefix+"legacy", `{"version":1,"group_hash":"legacy","state":"references_committed","sql_hash":"old"}`)
	preview := f.plan(t)
	if len(preview.Groups) != 0 || len(preview.Review) == 0 {
		t.Fatalf("unknown pending ownership allowed a new merge: %+v", preview)
	}
}
