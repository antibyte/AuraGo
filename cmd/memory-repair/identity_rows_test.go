package main

import (
	"errors"
	"testing"

	"aurago/internal/memory"
)

func repairIdentity(t *testing.T, f *repairFixture) string {
	t.Helper()
	v, err := openVectors(f.vectors)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := v.GetByID(t.Context(), "a")
	if err != nil {
		t.Fatal(err)
	}
	c, b, ok := memory.AnalysisDocumentParts(doc.Content)
	if !ok {
		t.Fatal("invalid fixture")
	}
	id, ok := memory.AnalysisMemoryIdentity(c, b, doc.Metadata["domain"])
	if !ok {
		t.Fatal("invalid identity")
	}
	return id
}

func TestMergeCurrentIdentityAndExpirySchemaResumes(t *testing.T) {
	f := newRepairFixture(t, "a", "b")
	identity := repairIdentity(t, f)
	f.exec(t, `INSERT INTO memory_document_identities VALUES('b',?)`, identity)
	f.exec(t, `INSERT INTO memory_expiry VALUES('a','2030-01-01 00:00:00'),('b','2030-01-01 00:00:00')`)
	plan := f.plan(t)
	if len(plan.Groups) != 1 {
		t.Fatalf("plan: %+v", plan.Review)
	}
	interrupted := errors.New("fixture interruption")
	_, failed := applyGroups(t.Context(), f.db, f.vectors, plan, func(stage string) error {
		if stage == "after_sql_commit" {
			return interrupted
		}
		return nil
	})
	if len(failed) != 1 || f.count(t) != 2 {
		t.Fatal("missing checkpoint")
	}
	for _, table := range []string{"memory_document_identities", "memory_expiry"} {
		var count int
		if err := f.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil || count != 2 {
			t.Fatalf("early cleanup %s: %d %v", table, count, err)
		}
	}
	complete, failed := applyGroups(t.Context(), f.db, f.vectors, plan, nil)
	if complete != 1 || len(failed) > 0 || f.count(t) != 1 {
		t.Fatalf("resume: %d %+v", complete, failed)
	}
	for _, table := range []string{"memory_document_identities", "memory_expiry"} {
		var id string
		if err := f.db.QueryRow(`SELECT doc_id FROM ` + table).Scan(&id); err != nil || id != "a" {
			t.Fatalf("canonical %s: %s %v", table, id, err)
		}
	}
}

func TestMergeIdentityConflictsAndPendingArchiveRequireReview(t *testing.T) {
	for _, mode := range []string{"identity", "archive_identity", "active_archive", "pending_cleanup", "expiry", "missing_expiry", "invalid_expiry"} {
		t.Run(mode, func(t *testing.T) {
			f := newRepairFixture(t, "a", "b")
			switch mode {
			case "identity":
				f.exec(t, `INSERT INTO memory_document_identities VALUES('b','conflict')`)
			case "archive_identity":
				f.exec(t, `INSERT INTO memory_archive_identities VALUES('b','conflict')`)
			case "active_archive":
				f.exec(t, `INSERT INTO memory_archive_identities VALUES('b',?)`, repairIdentity(t, f))
			case "pending_cleanup":
				f.exec(t, `INSERT INTO memory_archive_cleanup(doc_id,expected_hash) VALUES('b','pending')`)
			case "expiry":
				f.exec(t, `INSERT INTO memory_expiry VALUES('a','2030-01-01 00:00:00'),('b','2030-02-01 00:00:00')`)
			case "missing_expiry":
				f.exec(t, `INSERT INTO memory_expiry VALUES('b','2030-01-01 00:00:00')`)
			case "invalid_expiry":
				f.exec(t, `INSERT INTO memory_expiry VALUES('a','bad'),('b','bad')`)
			}
			if p := f.plan(t); len(p.Groups) != 0 || len(p.Review) == 0 || f.count(t) != 2 {
				t.Fatalf("unsafe plan %+v", p)
			}
		})
	}
}

func TestMergeKeepsHistoricalArchiveIdentityForDeletedID(t *testing.T) {
	f := newRepairFixture(t, "a", "b")
	f.curate(t, "a", "archive")
	identity := repairIdentity(t, f)
	f.exec(t, `INSERT INTO memory_archive_identities(doc_id,identity) VALUES('b',?)`, identity)
	p := f.plan(t)
	if len(p.Groups) != 1 {
		t.Fatalf("review %+v", p.Review)
	}
	complete, failed := applyGroups(t.Context(), f.db, f.vectors, p, nil)
	if complete != 1 || len(failed) > 0 {
		t.Fatalf("merge %d %+v", complete, failed)
	}
	var got string
	if err := f.db.QueryRow(`SELECT identity FROM memory_archive_identities WHERE doc_id='b'`).Scan(&got); err != nil || got != identity {
		t.Fatalf("lost historical archive %q %v", got, err)
	}
}

func TestMergeLegacyPlanWithEmptyAdditiveTablesRemainsResumable(t *testing.T) {
	f := newRepairFixture(t, "a", "b")
	p := f.plan(t)
	if len(p.Groups) != 1 {
		t.Fatal(p.Review)
	}
	for _, table := range []string{"memory_document_identities", "memory_archive_identities", "memory_archive_cleanup", "memory_expiry"} {
		if _, exists := p.Groups[0].Before.Rows[table]; exists {
			t.Fatalf("empty %s changed legacy fingerprint", table)
		}
	}
	// This is exactly the old plan shape, persisted before the new reader existed.
	_, failed := applyGroups(t.Context(), f.db, f.vectors, p, func(stage string) error {
		if stage == "after_sql_commit" {
			return errors.New("stop")
		}
		return nil
	})
	if len(failed) != 1 {
		t.Fatal("expected staged legacy plan")
	}
	complete, failed := applyGroups(t.Context(), f.db, f.vectors, p, nil)
	if complete != 1 || len(failed) > 0 {
		t.Fatalf("legacy resume %d %+v", complete, failed)
	}
}
