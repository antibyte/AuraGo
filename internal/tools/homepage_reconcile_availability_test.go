package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureHomepageProjectForDirRejectsFileBeforeRegistration(t *testing.T) {
	db := newHomepageLedgerTestDB(t)
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "index.html"), []byte("page"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureHomepageProjectForDir(db, HomepageConfig{WorkspacePath: workspace}, "index.html", "", ""); err == nil {
		t.Fatal("an individual file must not become a project")
	}
	sites, err := ListHomepageManagedSites(db)
	if err != nil || len(sites) != 0 {
		t.Fatalf("file registration changed inventory: sites=%v err=%v", sites, err)
	}
}

func TestReconcileHomepageProjectRetainsUnavailableHistory(t *testing.T) {
	for _, replacedByFile := range []bool{false, true} {
		name := "missing"
		if replacedByFile {
			name = "not-directory"
		}
		t.Run(name, func(t *testing.T) {
			db := newHomepageLedgerTestDB(t)
			cfg := HomepageConfig{WorkspacePath: t.TempDir()}
			projectPath := filepath.Join(cfg.WorkspacePath, "site")
			if err := os.Mkdir(projectPath, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(projectPath, "index.html"), []byte("original page"), 0o600); err != nil {
				t.Fatal(err)
			}
			saved := SaveHomepageRevisionAndState(cfg, db, "site", "Initial page", "test", "test", nil, nil)
			if len(saved.Warnings) != 0 || saved.RevisionID == 0 {
				t.Fatalf("save baseline: %+v", saved)
			}
			proj, err := GetProjectByDir(db, "site")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(projectPath, "index.html")); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(projectPath); err != nil {
				t.Fatal(err)
			}
			if replacedByFile {
				if err := os.WriteFile(projectPath, []byte("individual file"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			for range 2 {
				state, err := ReconcileHomepageProject(cfg, db, "site", nil)
				if err != nil || state.CurrentRevisionID != saved.RevisionID || state.DriftStatus != "local_changed" || state.LastReconciledAt == "" {
					t.Fatalf("reconcile unavailable project: state=%+v err=%v", state, err)
				}
			}
			site, err := GetHomepageManagedSite(db, proj.ID)
			if err != nil || site.LocalStatus != "missing" || site.Status != "active" {
				t.Fatalf("availability: site=%+v err=%v", site, err)
			}
			var reconciled, revisions, tracked int
			if err := db.QueryRow("SELECT COUNT(*) FROM homepage_site_events WHERE project_id=? AND event_type='reconciled'", proj.ID).Scan(&reconciled); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow("SELECT COUNT(*) FROM homepage_revisions WHERE project_id=?", proj.ID).Scan(&revisions); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow("SELECT COUNT(*) FROM homepage_site_file_state WHERE project_id=?", proj.ID).Scan(&tracked); err != nil {
				t.Fatal(err)
			}
			if reconciled != 1 || revisions != 1 || tracked != 1 {
				t.Fatalf("unavailable history changed or repeated: events=%d revisions=%d files=%d", reconciled, revisions, tracked)
			}
			if replacedByFile {
				if err := os.Remove(projectPath); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Mkdir(projectPath, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(projectPath, "index.html"), []byte("original page"), 0o600); err != nil {
				t.Fatal(err)
			}
			state, err := ReconcileHomepageProject(cfg, db, "site", nil)
			if err != nil || state.DriftStatus != "not_deployed" || state.CurrentRevisionID != saved.RevisionID {
				t.Fatalf("reappeared project: state=%+v err=%v", state, err)
			}
		})
	}
}
