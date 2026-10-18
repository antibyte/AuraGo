package memory

import (
	"database/sql"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/contacts"
)

type contactPrivacyFixture struct {
	stm  *SQLiteMemory
	kg   *KnowledgeGraph
	db   *sql.DB
	path string
	id   string
}

func newContactPrivacyFixture(t *testing.T) *contactPrivacyFixture {
	t.Helper()
	dir := t.TempDir()
	f := &contactPrivacyFixture{path: filepath.Join(dir, "contacts.db")}
	var err error
	f.stm, err = NewSQLiteMemory(filepath.Join(dir, "stm.db"), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.stm.Close() })
	f.kg, err = NewKnowledgeGraph(filepath.Join(dir, "kg.db"), "", slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.kg.Close() })
	f.db, err = contacts.InitDB(f.path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.db.Close() })
	f.id, err = contacts.Create(f.db, contacts.Contact{Name: "Synthetic contact"})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.kg.SyncContact(t.Context(), "contact_"+f.id, "Synthetic contact", nil); err != nil {
		t.Fatal(err)
	}
	return f
}

// This is the durable state left by a crash immediately before or after DELETE,
// before the request could persist the outcome. No request is replayed.
func journalUnconfirmedContactDeletion(t *testing.T, f *contactPrivacyFixture) string {
	t.Helper()
	jobID, token := newPrivacyDeletionID(), newPrivacyDeletionID()
	source := "contact_" + f.id
	targets, err := f.kg.prepareContactPrivacyTargets(t.Context(), source, true)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := f.stm.db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := f.stm.createPrivacyDeletionJobTx(t.Context(), tx, jobID, "contact", PrivacyDeletionAwaitingContactDelete, true, source, token, time.Now().UTC(), targets); err != nil {
		t.Fatal(err)
	}
	if err := f.stm.setPrivacyPublicationMarkerTx(t.Context(), tx, jobID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := f.kg.BeginContactSourceDeletion(t.Context(), source, token); err != nil {
		t.Fatal(err)
	}
	return jobID
}

func TestContactPrivacyRestartWithPresentContactNeverRepeatsDelete(t *testing.T) {
	f := newContactPrivacyFixture(t)
	jobID := journalUnconfirmedContactDeletion(t, f)
	if err := f.stm.ProcessDueContactPrivacyDeletionJobs(t.Context(), f.db, f.kg, time.Now().UTC(), 10); err != nil {
		t.Fatal(err)
	}
	if _, err := contacts.GetByID(f.db, f.id); err != nil {
		t.Fatalf("worker deleted contact instead of observing it: %v", err)
	}
	job, err := f.stm.GetPrivacyDeletionJob(t.Context(), jobID)
	if err != nil || job.DeletionStatus != PrivacyDeletionAborted {
		t.Fatalf("job=%+v err=%v", job, err)
	}
	if err := f.kg.SyncContact(t.Context(), "contact_"+f.id, "Still editable", nil); err != nil {
		t.Fatalf("own fence was not released: %v", err)
	}
	if err := f.stm.ProcessDueContactPrivacyDeletionJobs(t.Context(), f.db, f.kg, time.Now().Add(time.Hour), 10); err != nil {
		t.Fatal(err)
	}
	if _, err := contacts.GetByID(f.db, f.id); err != nil {
		t.Fatalf("aborted operation resumed deletion: %v", err)
	}
}

func TestContactPrivacyUnknownOutcomeBacksOffAndReconcilesAfterReopen(t *testing.T) {
	f := newContactPrivacyFixture(t)
	jobID := journalUnconfirmedContactDeletion(t, f)
	if err := f.db.Close(); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := f.stm.ProcessDueContactPrivacyDeletionJobs(t.Context(), f.db, f.kg, now, 10); err == nil {
		t.Fatal("unreadable contacts store was treated as absent")
	}
	job, err := f.stm.GetPrivacyDeletionJob(t.Context(), jobID)
	if err != nil {
		t.Fatal(err)
	}
	next, err := time.Parse(time.RFC3339Nano, job.NextRetryAt)
	if err != nil || job.DeletionStatus != PrivacyDeletionReconcilePending || !next.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("unknown outcome job=%+v next=%v err=%v", job, next, err)
	}
	if fenced, err := f.kg.IsContactSourceDeleted(t.Context(), "contact_"+f.id); err != nil || !fenced {
		t.Fatalf("unknown outcome lost fence: %v %v", fenced, err)
	}
	f.db, err = contacts.InitDB(f.path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.stm.RetryPrivacyDeletionJob(t.Context(), jobID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := f.stm.ProcessDueContactPrivacyDeletionJobs(t.Context(), f.db, f.kg, time.Now().UTC(), 10); err != nil {
		t.Fatal(err)
	}
	job, err = f.stm.GetPrivacyDeletionJob(t.Context(), jobID)
	if err != nil || job.DeletionStatus != PrivacyDeletionAborted {
		t.Fatalf("reconcile job=%+v err=%v", job, err)
	}
	if _, err := contacts.GetByID(f.db, f.id); err != nil {
		t.Fatalf("reconciliation deleted a present contact: %v", err)
	}
}

func TestContactPrivacyConfirmedAbsenceDeletesOnlyUnchangedSelectedTargets(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(map[bool]string{false: "unchanged", true: "protected_after_acceptance"}[changed], func(t *testing.T) {
			f := newContactPrivacyFixture(t)
			jobID := journalUnconfirmedContactDeletion(t, f)
			if err := contacts.Delete(f.db, f.id); err != nil {
				t.Fatal(err)
			}
			if changed {
				if _, err := f.kg.SetNodeProtected("contact_"+f.id, true); err != nil {
					t.Fatal(err)
				}
			}
			if err := f.kg.AddNode("unrelated-late-node", "Unrelated fact", nil); err != nil {
				t.Fatal(err)
			}
			if err := f.stm.ProcessDueContactPrivacyDeletionJobs(t.Context(), f.db, f.kg, time.Now().UTC(), 10); err != nil {
				t.Fatal(err)
			}
			job, err := f.stm.GetPrivacyDeletionJob(t.Context(), jobID)
			if err != nil {
				t.Fatal(err)
			}
			if changed && (job.RetainedCount != 1 || job.DeletionStatus != PrivacyDeletionRetainedReview) || !changed && (job.DeletedCount != 1 || job.DeletionStatus != PrivacyDeletionCompleted) {
				t.Fatalf("job=%+v", job)
			}
			node, err := f.kg.GetNode("contact_" + f.id)
			if changed && (err != nil || node == nil) || !changed && err == nil && node != nil {
				t.Fatalf("changed=%v node=%+v err=%v", changed, node, err)
			}
			if node, err := f.kg.GetNode("unrelated-late-node"); err != nil || node == nil {
				t.Fatalf("unselected fact was lost: %v", err)
			}
			var targets int
			if err := f.stm.db.QueryRow(`SELECT COUNT(*) FROM privacy_deletion_targets WHERE deletion_job_id=?`, jobID).Scan(&targets); err != nil || targets != 0 {
				t.Fatalf("terminal targets=%d err=%v", targets, err)
			}
			if _, err := f.stm.RetryPrivacyDeletionJob(t.Context(), jobID, time.Now()); !errors.Is(err, ErrPrivacyDeletionJobNotRetryable) {
				t.Fatalf("terminal retry overrides decision: %v", err)
			}
		})
	}
}

func TestLegacyContactFenceDoesNotCreateDeletionJob(t *testing.T) {
	f := newContactPrivacyFixture(t)
	if err := f.kg.FenceContactSource(t.Context(), "contact_"+f.id); err != nil {
		t.Fatal(err)
	}
	if err := f.stm.ProcessDueContactPrivacyDeletionJobs(t.Context(), f.db, f.kg, time.Now().Add(24*time.Hour), 10); err != nil {
		t.Fatal(err)
	}
	jobs, err := f.stm.ListPrivacyDeletionJobs(t.Context(), 10)
	if err != nil || len(jobs) != 0 {
		t.Fatalf("legacy fence became a job: %+v %v", jobs, err)
	}
	if _, err := contacts.GetByID(f.db, f.id); err != nil {
		t.Fatal(err)
	}
	if fenced, err := f.kg.IsContactSourceDeleted(t.Context(), "contact_"+f.id); err != nil || !fenced {
		t.Fatalf("legacy fence was compensated: %v %v", fenced, err)
	}
}

func TestContactFenceMigrationBacksUpLegacyStateBeforeAddingOwnership(t *testing.T) {
	f := newContactPrivacyFixture(t)
	if _, err := f.kg.db.Exec(`CREATE TABLE deleted_contact_sources(contact_id TEXT PRIMARY KEY,deleted_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP); INSERT INTO deleted_contact_sources(contact_id) VALUES(?)`, "contact_"+f.id); err != nil {
		t.Fatal(err)
	}
	path, err := mainDatabasePath(f.kg.db)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.kg.ensureContactSourceFence(t.Context()); err != nil {
		t.Fatal(err)
	}
	backups, err := filepath.Glob(path + ".contact-fences-v2-*.bak")
	if err != nil || len(backups) != 1 {
		t.Fatalf("migration backup files=%v err=%v", backups, err)
	}
	copyDB, err := sql.Open("sqlite", snapshotSQLiteURI(backups[0], "mode=ro&immutable=1"))
	if err != nil {
		t.Fatal(err)
	}
	defer copyDB.Close()
	var columns, contactsInBackup int
	if err := copyDB.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('deleted_contact_sources') WHERE name='operation_token'`).Scan(&columns); err != nil || columns != 0 {
		t.Fatalf("backup captured post-migration state: %d %v", columns, err)
	}
	if err := copyDB.QueryRow(`SELECT COUNT(*) FROM kg_nodes WHERE id=?`, "contact_"+f.id).Scan(&contactsInBackup); err != nil || contactsInBackup != 1 {
		t.Fatalf("backup lost existing graph node: %d %v", contactsInBackup, err)
	}
	token, state, exists, err := f.kg.ContactSourceFence(t.Context(), "contact_"+f.id)
	if err != nil || !exists || token != "" || state != ContactFenceCommitted {
		t.Fatalf("legacy fence acquired new authority: tokenPresent=%v state=%s exists=%v err=%v", token != "", state, exists, err)
	}
}
