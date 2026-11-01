package memory

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestAutomaticMemoryPublicationRestartAtWriteBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, trigger string
		corpus, ready bool
	}{
		{"journal persistence", `CREATE TRIGGER injected_failure BEFORE INSERT ON memory_maintenance_meta WHEN NEW.key LIKE 'memory.publication.%' BEGIN SELECT RAISE(ABORT,'journal unavailable'); END`, false, false},
		{"corpus staging", `CREATE TRIGGER injected_failure BEFORE INSERT ON documents BEGIN SELECT RAISE(ABORT,'corpus unavailable'); END`, true, false},
		{"metadata commit", `CREATE TRIGGER injected_failure BEFORE INSERT ON memory_document_identities BEGIN SELECT RAISE(ABORT,'identity unavailable'); END`, false, false},
		{"index activation", `CREATE TRIGGER injected_failure BEFORE UPDATE OF pending ON documents WHEN NEW.pending=0 BEGIN SELECT RAISE(ABORT,'activation unavailable'); END`, true, true},
		{"visibility release", `CREATE TRIGGER injected_failure BEFORE DELETE ON memory_maintenance_meta WHEN OLD.key LIKE 'memory.publication.%' BEGIN SELECT RAISE(ABORT,'journal completion unavailable'); END`, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stm, vector, dir := automaticPublicationFixture(t, func(context.Context, string) ([]float32, error) { return []float32{1, 0, 0}, nil })
			db := stm.db
			if tc.corpus {
				db = vector.corpus.db
			}
			if _, err := db.Exec(tc.trigger); err != nil {
				t.Fatal(err)
			}
			expiry := time.Now().Add(24 * time.Hour)
			stored, err := publishTestMemory(t, stm, vector, "Vim is the preferred editor.", &expiry)
			if err == nil {
				t.Fatal("injected publication failure was reported as success")
			}
			visible, readErr := vector.queryVisibleCollection(t.Context(), vector.collection, []float32{1, 0, 0}, 10)
			if readErr != nil || len(visible) != 0 {
				t.Fatalf("unfinished fact visible=%v err=%v", visible, readErr)
			}
			if _, err := db.Exec(`DROP TRIGGER injected_failure`); err != nil {
				t.Fatal(err)
			}
			memoryPath := filepath.Join(filepath.Dir(dir), "memory.db")
			if err := vector.Close(); err != nil {
				t.Fatal(err)
			}
			if err := stm.Close(); err != nil {
				t.Fatal(err)
			}
			stm, err = NewSQLiteMemory(memoryPath, stm.logger)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = stm.Close() })
			var calls atomic.Int32
			vector = newAutomaticPublicationVector(t, dir, stm, func(context.Context, string) ([]float32, error) { calls.Add(1); return []float32{1, 0, 0}, nil })
			if err := stm.RecoverAutomaticMemoryPublicationsContext(t.Context(), vector, func() error { return nil }); err != nil {
				t.Fatal(err)
			}
			if calls.Load() != 0 {
				t.Fatalf("restart needlessly embedded %d times", calls.Load())
			}
			var journals int
			if err := stm.db.QueryRow(`SELECT COUNT(*) FROM memory_maintenance_meta WHERE key LIKE 'memory.publication.%'`).Scan(&journals); err != nil || journals != 0 {
				t.Fatalf("journals=%d err=%v", journals, err)
			}
			visible, err = vector.queryVisibleCollection(t.Context(), vector.collection, []float32{1, 0, 0}, 10)
			want := 0
			if tc.ready {
				want = 1
			}
			if err != nil || len(visible) != want {
				t.Fatalf("recovered visible=%d want=%d err=%v", len(visible), want, err)
			}
			if tc.ready {
				if len(stored.CreatedIDs) != 1 {
					t.Fatalf("created IDs=%v", stored.CreatedIDs)
				}
				if _, exists, err := stm.MemoryExpiry(t.Context(), stored.CreatedIDs[0]); err != nil || !exists {
					t.Fatalf("recovered expiry=%v err=%v", exists, err)
				}
			}
		})
	}
}

type expiryMutationVector struct {
	*fakeRepairVectorDB
	beforeRead func()
}

func (v *expiryMutationVector) GetByID(id string) (string, error) {
	if v.beforeRead != nil {
		action := v.beforeRead
		v.beforeRead = nil
		action()
	}
	return v.fakeRepairVectorDB.GetByID(id)
}

func TestArchiveExpiredMemoriesRechecksRenewedOrRemovedDeadline(t *testing.T) {
	for _, clear := range []bool{false, true} {
		t.Run(map[bool]string{false: "renewed", true: "removed"}[clear], func(t *testing.T) {
			stm := newInMemorySQLiteMemory(t)
			if err := stm.UpsertMemoryMeta("fact"); err != nil {
				t.Fatal(err)
			}
			if err := stm.SetMemoryExpiry(t.Context(), []string{"fact"}, time.Now().Add(-time.Hour)); err != nil {
				t.Fatal(err)
			}
			vector := &expiryMutationVector{fakeRepairVectorDB: &fakeRepairVectorDB{docs: map[string]string{"fact": "The maintenance window is scheduled for tonight."}}}
			vector.beforeRead = func() {
				// Simulate an external SQL writer after candidate discovery. Public
				// expiry writes additionally share the automatic-write lock.
				var result sql.Result
				var err error
				if clear {
					result, err = stm.db.Exec(`DELETE FROM memory_expiry WHERE doc_id='fact'`)
				} else {
					result, err = stm.db.Exec(`UPDATE memory_expiry SET expires_at=? WHERE doc_id='fact'`, time.Now().Add(time.Hour).UTC().Format(memoryExpiryLayout))
				}
				if err != nil {
					t.Fatal(err)
				}
				if n, err := result.RowsAffected(); err != nil || n != 1 {
					t.Fatalf("expiry change=%d err=%v", n, err)
				}
			}
			count, err := stm.ArchiveExpiredMemoriesContext(t.Context(), vector)
			if err != nil || count != 0 {
				t.Fatalf("stale archive count=%d err=%v", count, err)
			}
			meta, err := stm.GetMemoryMeta("fact")
			if err != nil || IsMemoryArchived(meta) || vector.docs["fact"] == "" {
				t.Fatalf("renewed memory was archived: meta=%+v err=%v", meta, err)
			}
		})
	}
}
