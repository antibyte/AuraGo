package tools

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/dbutil"
)

func homepageMigrationFixture(t *testing.T, version int, duplicate bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "registry.db")
	db, err := dbutil.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	schema, err := os.ReadFile("testdata/homepage_registry_v0.sql")
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{string(schema), `
		INSERT INTO homepage_projects(id,name,project_dir,updated_at) VALUES
		(1,'old','site','2026-01-01'),(2,'new','site','2026-02-01');
		INSERT INTO homepage_deploy_targets(id,project_id,provider,provider_target_id,url) VALUES
		(7,1,'netlify','preview','https://preview.example.test'),
		(8,2,'netlify','production','https://production.example.test');
		INSERT INTO homepage_deployments(id,project_id,target_id,provider) VALUES
		(1,1,7,'netlify'),(2,2,8,'netlify');
		INSERT INTO homepage_remote_observations(id,project_id,target_id,provider) VALUES
		(1,1,7,'netlify'),(2,2,8,'netlify');`}
	if version == 1 {
		statements = append(statements, "PRAGMA foreign_keys=OFF")
		statements = append(statements, homepageRegistryV1Statements...)
		statements = append(statements, "PRAGMA foreign_keys=ON", "PRAGMA user_version=1",
			"UPDATE homepage_deploy_targets SET target_key=provider_target_id")
		if duplicate {
			statements = append(statements, "UPDATE homepage_deploy_targets SET target_key='same-target'")
		}
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestHomepageMigrationPreservesTargetIdentity(t *testing.T) {
	for _, tc := range []struct {
		name      string
		version   int
		duplicate bool
	}{
		{"v0 legacy identities", 0, false},
		{"v1 distinct targets", 1, false},
		{"v1 same target", 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := homepageMigrationFixture(t, tc.version, tc.duplicate)
			for attempt := 0; attempt < 2; attempt++ {
				db, err := InitHomepageRegistryDB(path)
				if err != nil {
					t.Fatal(err)
				}
				func() {
					defer db.Close()
					wantTargets, wantOldTarget := 2, 7
					if tc.duplicate {
						wantTargets, wantOldTarget = 1, 8
					}
					var count int
					if err := db.QueryRow("SELECT COUNT(*) FROM homepage_deploy_targets WHERE project_id=2").Scan(&count); err != nil || count != wantTargets {
						t.Fatalf("targets: got %d, want %d: %v", count, wantTargets, err)
					}
					for _, table := range []string{"homepage_deployments", "homepage_remote_observations"} {
						for id, want := range map[int]int{1: wantOldTarget, 2: 8} {
							var projectID, targetID int
							if err := db.QueryRow("SELECT project_id,target_id FROM "+table+" WHERE id=?", id).Scan(&projectID, &targetID); err != nil || projectID != 2 || targetID != want {
								t.Fatalf("%s/%d: project=%d target=%d, want 2/%d: %v", table, id, projectID, targetID, want, err)
							}
						}
					}
					if !tc.duplicate {
						var providerID, url, key string
						if err := db.QueryRow("SELECT provider_target_id,url,target_key FROM homepage_deploy_targets WHERE id=7").Scan(&providerID, &url, &key); err != nil || providerID != "preview" || url != "https://preview.example.test" || key == "" {
							t.Fatalf("target identity lost: %q %q %q: %v", providerID, url, key, err)
						}
					}
					if err := db.QueryRow("SELECT COUNT(*) FROM pragma_foreign_key_check").Scan(&count); err != nil || count != 0 {
						t.Fatalf("foreign key violations: %d: %v", count, err)
					}
					if err := db.QueryRow("SELECT COUNT(*) FROM homepage_site_events WHERE event_type='project_dir_merged'").Scan(&count); err != nil || count != 1 {
						t.Fatalf("merge should be recorded exactly once: %d: %v", count, err)
					}
				}()
			}
			backups, err := filepath.Glob(path + ".homepage-v*-*.bak")
			if err != nil || len(backups) != 1 {
				t.Fatalf("expected one original backup: %v: %v", backups, err)
			}
			backup, err := sql.Open("sqlite", "file:"+filepath.ToSlash(backups[0])+"?mode=ro")
			if err != nil {
				t.Fatal(err)
			}
			defer backup.Close()
			var version, targets int
			if err := backup.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != tc.version {
				t.Fatalf("backup version=%d: %v", version, err)
			}
			if err := backup.QueryRow("SELECT COUNT(*) FROM homepage_deploy_targets").Scan(&targets); err != nil || targets != 2 {
				t.Fatalf("backup lost targets: %d: %v", targets, err)
			}
		})
	}
}

func TestHomepageMigrationRollsBackTargetMerge(t *testing.T) {
	for _, version := range []int{0, 1} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			path := homepageMigrationFixture(t, version, true)
			db, err := dbutil.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err := db.Exec(`CREATE TRIGGER refuse_merge BEFORE DELETE ON homepage_projects
				BEGIN SELECT RAISE(ABORT, 'injected merge failure'); END`); err != nil {
				t.Fatal(err)
			}
			if err := migrateHomepageRegistry(db); err == nil || !strings.Contains(err.Error(), "injected merge failure") {
				t.Fatalf("expected injected failure, got %v", err)
			}
			var got int
			if err := db.QueryRow("PRAGMA user_version").Scan(&got); err != nil || got != version {
				t.Fatalf("version was not rolled back: %d: %v", got, err)
			}
			for _, table := range []string{"homepage_projects", "homepage_deploy_targets", "homepage_deployments", "homepage_remote_observations"} {
				if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&got); err != nil || got != 2 {
					t.Fatalf("%s lost rows: %d: %v", table, got, err)
				}
			}
			for _, table := range []string{"homepage_deployments", "homepage_remote_observations"} {
				if err := db.QueryRow("SELECT target_id FROM " + table + " WHERE project_id=1").Scan(&got); err != nil || got != 7 {
					t.Fatalf("%s link was not rolled back: %d: %v", table, got, err)
				}
			}
			if err := db.QueryRow("PRAGMA foreign_keys").Scan(&got); err != nil || got != 1 {
				t.Fatalf("foreign key enforcement not restored: %d: %v", got, err)
			}
		})
	}
}
