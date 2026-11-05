package flows

import (
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFlowsV8MigrationPreservesStateAndRollback(t *testing.T) {
	for _, rollback := range []bool{false, true} {
		name := "commit"
		if rollback {
			name = "marker failure"
		}
		t.Run(name, func(t *testing.T) {
			path, writer := childHistoryV6Fixture(t)
			if err := migrateFlowsV7(writer, discardLogger(), false); err != nil {
				t.Fatal(err)
			}
			const summary = `{"state":"complete","invocation_count":1}`
			const trace = `{"frames":[{"node_id":"n_aaaaaaaa","outputs":{"preserved":"<original>"}}]}`
			if _, err := writer.Exec(`PRAGMA wal_checkpoint(TRUNCATE);
				INSERT INTO flow_run_steps(run_id,seq,node_id,node_key,status)
				VALUES('run_v7old',1,'n_aaaaaaaa','child','success')`); err != nil {
				t.Fatal(err)
			}
			if _, err := writer.Exec(`INSERT INTO flow_child_traces(run_id,parent_seq,summary_json,trace_json) VALUES('run_v7old',1,?,?)`, summary, trace); err != nil {
				t.Fatal(err)
			}
			if _, err := writer.Exec(`UPDATE flow_runs SET child_history_state='complete',child_history_bytes=?,child_history_metadata=? WHERE id='run_v7old'`, len(trace), len(summary)); err != nil {
				t.Fatal(err)
			}
			if info, err := os.Stat(path + "-wal"); err != nil || info.Size() <= 32 {
				t.Fatalf("fixture has no committed WAL: %v, %v", info, err)
			}
			want := groupsMigrationData(t, writer)
			assertData := func(db *sql.DB, version string) {
				t.Helper()
				if !reflect.DeepEqual(groupsMigrationData(t, db), want) {
					t.Fatal("migration changed documents, pins, owner, run history or ciphertext")
				}
				var gotVersion, gotSummary, gotTrace string
				if err := db.QueryRow(`SELECT value FROM flows_meta WHERE key='schema_version'`).Scan(&gotVersion); err != nil || gotVersion != version {
					t.Fatalf("schema = %q, want %q: %v", gotVersion, version, err)
				}
				if err := db.QueryRow(`SELECT summary_json,trace_json FROM flow_child_traces WHERE run_id='run_v7old' AND parent_seq=1`).Scan(&gotSummary, &gotTrace); err != nil || gotSummary != summary || gotTrace != trace {
					t.Fatalf("child trace changed: %v", err)
				}
			}
			if rollback {
				if _, err := writer.Exec(`CREATE TRIGGER reject_v8 BEFORE UPDATE OF value ON flows_meta
					WHEN OLD.value='7' AND NEW.value='8' BEGIN SELECT RAISE(ABORT,'injected'); END;
					DROP INDEX idx_flows_mission`); err != nil {
					t.Fatal(err)
				}
			}
			s, err := OpenStore(path, discardLogger())
			if rollback {
				if err == nil {
					s.Close()
					t.Fatal("marker failure accepted")
				}
				assertData(writer, "7")
				var partial int
				if err := writer.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name='flow_durable_gates' OR name LIKE 'idx_flow_durable_gates_%' OR name='idx_flows_mission'`).Scan(&partial); err != nil || partial != 0 {
					t.Fatalf("failed migration left partial or baseline writes: %d, %v", partial, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				assertData(s.db, "8")
				if err := s.Close(); err != nil {
					t.Fatal(err)
				}
				reopened, err := OpenStore(path, discardLogger())
				if err != nil {
					t.Fatal(err)
				}
				if err := reopened.Close(); err != nil {
					t.Fatal(err)
				}
			}
			backups, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".flows-v7-backup-*", "flows.db"))
			if err != nil || len(backups) != 1 {
				t.Fatalf("backup = %v, %v", backups, err)
			}
			backup, err := sql.Open("sqlite", backups[0])
			if err != nil {
				t.Fatal(err)
			}
			defer backup.Close()
			assertData(backup, "7")
			var integrity string
			if err := backup.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
				t.Fatalf("backup integrity = %q, %v", integrity, err)
			}
		})
	}
}
