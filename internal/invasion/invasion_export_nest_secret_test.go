package invasion

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/dbutil"
)

const legacyNestID = "12345678-abcd-ef12-3456-7890abcdef12"

// createLegacyNestsDB writes a nests table from before export_nest_secret.
// withDockerTLS selects the K17 schema; otherwise the pre-K17 one.
func createLegacyNestsDB(t *testing.T, path string, withDockerTLS, withRow bool) {
	t.Helper()
	legacy, err := dbutil.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	extra := ""
	if withDockerTLS {
		extra = `desired_config_rev TEXT DEFAULT '', applied_config_rev TEXT DEFAULT '', docker_tls TEXT DEFAULT '',`
	}
	if _, err := legacy.Exec(`CREATE TABLE nests (
		id TEXT PRIMARY KEY, name TEXT NOT NULL, notes TEXT DEFAULT '', access_type TEXT NOT NULL DEFAULT 'ssh',
		host TEXT NOT NULL DEFAULT '', port INTEGER NOT NULL DEFAULT 22, username TEXT DEFAULT '',
		vault_secret_id TEXT DEFAULT '', active INTEGER NOT NULL DEFAULT 1, egg_id TEXT DEFAULT '',
		hatch_status TEXT DEFAULT 'idle', last_hatch_at TEXT DEFAULT '', hatch_error TEXT DEFAULT '',
		route TEXT DEFAULT 'direct', route_config TEXT DEFAULT '', deploy_method TEXT DEFAULT 'ssh',
		target_arch TEXT DEFAULT 'linux/amd64', ` + extra + ` created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if withRow {
		if _, err := legacy.Exec(`INSERT INTO nests (id, name, host, port, username, vault_secret_id, deploy_method, created_at, updated_at)
			VALUES (?, 'legacy', '10.0.0.5', 22, 'deploy', ?, 'ssh', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`, legacyNestID, "nest_"+legacyNestID); err != nil {
			t.Fatal(err)
		}
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
}

func backups(t *testing.T, path string) []string {
	t.Helper()
	matches, err := filepath.Glob(path + exportNestSecretBackupSuffix + "*")
	if err != nil {
		t.Fatal(err)
	}
	return matches
}

func TestInitDBGrandfathersExportNestSecretForExistingNests(t *testing.T) {
	for name, withDockerTLS := range map[string]bool{"pre-K17 schema": false, "K17 schema": true} {
		t.Run(name, func(t *testing.T) {
			path := tempDB(t)
			createLegacyNestsDB(t, path, withDockerTLS, true)
			db, err := InitDB(path)
			if err != nil {
				t.Fatalf("InitDB: %v", err)
			}
			legacy, err := GetNest(db, legacyNestID)
			if err != nil || !legacy.ExportNestSecret {
				t.Fatalf("legacy nest ExportNestSecret = %v, %v; want true (grandfathered)", legacy.ExportNestSecret, err)
			}
			freshID, err := CreateNest(db, NestRecord{Name: "fresh", Active: true, DeployMethod: "ssh", VaultSecretID: "nest_fresh"})
			if err != nil {
				t.Fatal(err)
			}
			if fresh, _ := GetNest(db, freshID); fresh.ExportNestSecret {
				t.Fatal("a new nest must not export its secret unless the option is enabled")
			}
			db.Close()
			again, err := InitDB(path) // the second run is a no-op
			if err != nil {
				t.Fatalf("second InitDB: %v", err)
			}
			defer again.Close()
			if n, _ := GetNest(again, legacyNestID); !n.ExportNestSecret {
				t.Fatal("second InitDB changed the grandfathered value")
			}
			if n, _ := GetNest(again, freshID); n.ExportNestSecret {
				t.Fatal("second InitDB changed the new nest")
			}
		})
	}
}

func TestInitDBBacksUpAnExistingDatabaseBeforeAddingExportNestSecret(t *testing.T) {
	path := tempDB(t)
	createLegacyNestsDB(t, path, false, true)
	db, err := InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	copies := backups(t, path)
	if len(copies) != 1 || copies[0] != path+exportNestSecretBackupSuffix {
		t.Fatalf("backups = %v, want exactly %s", copies, path+exportNestSecretBackupSuffix)
	}
	backup, err := sql.Open("sqlite", copies[0])
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	var rows, hasColumn int
	if err := backup.QueryRow(`SELECT COUNT(*) FROM nests`).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("backup rows = %d, %v; want the legacy nest", rows, err)
	}
	if err := backup.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('nests') WHERE name = 'export_nest_secret'`).Scan(&hasColumn); err != nil || hasColumn != 0 {
		t.Fatalf("backup has export_nest_secret = %d, %v; want the pre-migration schema", hasColumn, err)
	}
	again, err := InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	again.Close()
	if n := len(backups(t, path)); n != 1 {
		t.Fatalf("second InitDB wrote %d backups in total, want 1", n)
	}
}

func TestInitDBBacksUpALegacyDatabaseWithoutNests(t *testing.T) {
	path := tempDB(t)
	createLegacyNestsDB(t, path, true, false)
	db, err := InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	if n := len(backups(t, path)); n != 1 {
		t.Fatalf("backups = %d, want 1 for an existing database", n)
	}
}

func TestInitDBSkipsTheBackupForANewDatabase(t *testing.T) {
	path := tempDB(t)
	db, err := InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	if n := len(backups(t, path)); n != 0 {
		t.Fatalf("a new database wrote %d backups, want 0", n)
	}
}

func TestInitDBBackupNeverOverwritesAnEarlierCopy(t *testing.T) {
	path := tempDB(t)
	createLegacyNestsDB(t, path, true, true)
	earlier := path + exportNestSecretBackupSuffix
	if err := os.WriteFile(earlier, []byte("earlier copy"), 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := InitDB(path)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	if raw, _ := os.ReadFile(earlier); string(raw) != "earlier copy" {
		t.Fatal("InitDB overwrote an earlier backup")
	}
	if n := len(backups(t, path)); n != 2 {
		t.Fatalf("backups = %d, want the earlier copy plus a timestamped one", n)
	}
}

func TestNestExportNestSecretPersistsThroughEveryQuery(t *testing.T) {
	db, err := InitDB(tempDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	id, err := CreateNest(db, NestRecord{Name: "exporting", Active: true, DeployMethod: "ssh", VaultSecretID: "nest_a", ExportNestSecret: true})
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := GetNest(db, id); !n.ExportNestSecret {
		t.Fatal("GetNest lost export_nest_secret")
	}
	if n, _ := GetNestByName(db, "exporting"); !n.ExportNestSecret {
		t.Fatal("GetNestByName lost export_nest_secret")
	}
	for name, list := range map[string]func(*sql.DB) ([]NestRecord, error){"ListNests": ListNests, "ListActiveNests": ListActiveNests} {
		nests, err := list(db)
		if err != nil || len(nests) != 1 || !nests[0].ExportNestSecret {
			t.Fatalf("%s = %+v, %v; want export_nest_secret kept", name, nests, err)
		}
	}
	if err := UpdateNestHatchStatus(db, id, "running", ""); err != nil {
		t.Fatal(err)
	}
	if err := ToggleNestActive(db, id, true); err != nil {
		t.Fatal(err)
	}
	n, _ := GetNest(db, id)
	if !n.ExportNestSecret {
		t.Fatal("a targeted update reset export_nest_secret")
	}
	n.ExportNestSecret = false
	if err := UpdateNest(db, n); err != nil {
		t.Fatal(err)
	}
	if n, _ := GetNest(db, id); n.ExportNestSecret {
		t.Fatal("UpdateNest did not store export_nest_secret = false")
	}
}
