package server

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestHomepageLedgerReconcilerRetainsUnavailableAndRemoteOnlySites(t *testing.T) {
	db, err := tools.InitHomepageRegistryDB(filepath.Join(t.TempDir(), "homepage.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Homepage.WorkspacePath = t.TempDir()
	if err := os.WriteFile(filepath.Join(cfg.Homepage.WorkspacePath, "index.html"), []byte("legacy page"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, project := range []tools.HomepageProject{
		{Name: "legacy-file", ProjectDir: "index.html"},
		{Name: "missing-directory", ProjectDir: "missing-site"},
		{Name: "archived", ProjectDir: "archived-site", Status: "archived"},
	} {
		if _, _, err := tools.RegisterProject(db, project); err != nil {
			t.Fatal(err)
		}
	}
	// Native provider ledgers retain remote-only identities without a local directory.
	if _, err := db.Exec("INSERT INTO homepage_projects(name,project_dir) VALUES('provider-only','')"); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	s := &Server{Cfg: cfg, HomepageRegistryDB: db, Logger: slog.New(slog.NewTextHandler(&logs, nil))}
	for range 2 {
		runHomepageLedgerReconcileOnce(s)
	}
	if strings.Contains(logs.String(), "reconcile failed") {
		t.Fatalf("unavailable inventory generated warnings: %s", logs.String())
	}
	sites, err := tools.ListHomepageManagedSites(db)
	if err != nil || len(sites) != 4 {
		t.Fatalf("inventory was lost: sites=%v err=%v", sites, err)
	}
	for _, site := range sites {
		switch site.Name {
		case "legacy-file", "missing-directory":
			if site.LocalStatus != "missing" || site.DriftStatus != "local_changed" {
				t.Fatalf("unavailable site was not recorded: %+v", site)
			}
		case "provider-only":
			if site.LocalStatus != "remote_only" || site.LocalRoot != "" {
				t.Fatalf("remote-only site acquired a local root: %+v", site)
			}
		case "archived":
			if site.LastReconciledAt != "" {
				t.Fatalf("archived site was reconciled: %+v", site)
			}
		}
	}
}
