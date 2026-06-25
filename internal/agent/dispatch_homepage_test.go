package agent

import (
	"context"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestDispatchHomepageDestroyRequiresForce(t *testing.T) {
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Homepage.AllowContainerManagement = true
	cfg.Homepage.WorkspacePath = t.TempDir()

	output, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "homepage",
		Operation: "destroy",
	}, &DispatchContext{Cfg: cfg, Logger: testLogger})
	if !ok {
		t.Fatal("expected homepage operation to be handled")
	}
	if !strings.Contains(output, "requires force=true") {
		t.Fatalf("expected destroy without force to be rejected, got %s", output)
	}
}

func TestDispatchHomepageInitProjectRegistersProjectDir(t *testing.T) {
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Homepage.AllowContainerManagement = true
	cfg.Docker.Host = "tcp://127.0.0.1:1"
	cfg.Homepage.WorkspacePath = t.TempDir()
	db, err := tools.InitHomepageRegistryDB(t.TempDir() + "/homepage.db")
	if err != nil {
		t.Fatalf("InitHomepageRegistryDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	output, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "homepage",
		Operation: "init_project",
		Framework: "html",
		Name:      "site-a",
	}, &DispatchContext{Cfg: cfg, Logger: testLogger, HomepageRegistryDB: db})
	if !ok {
		t.Fatal("expected homepage operation to be handled")
	}
	if !strings.Contains(output, `"project_dir":"site-a"`) {
		t.Fatalf("expected tool output to include project_dir, got %s", output)
	}

	proj, err := tools.GetProjectByDir(db, "site-a")
	if err != nil {
		t.Fatalf("expected registered project by dir: %v", err)
	}
	if proj.Name != "site-a" {
		t.Fatalf("registered project name = %q, want site-a", proj.Name)
	}
}

func TestDispatchHomepageWriteFileRecordsLedgerRevision(t *testing.T) {
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Docker.Host = "tcp://127.0.0.1:1"
	cfg.Homepage.WorkspacePath = t.TempDir()
	db, err := tools.InitHomepageRegistryDB(t.TempDir() + "/homepage.db")
	if err != nil {
		t.Fatalf("InitHomepageRegistryDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := tools.EnsureHomepageProjectForDir(db, tools.HomepageConfig{WorkspacePath: cfg.Homepage.WorkspacePath}, "site-a", "site-a", "html"); err != nil {
		t.Fatalf("EnsureHomepageProjectForDir failed: %v", err)
	}

	output, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "homepage",
		Operation: "write_file",
		Path:      "site-a/index.html",
		Content:   "<h1>Hello</h1>",
	}, &DispatchContext{Cfg: cfg, Logger: testLogger, HomepageRegistryDB: db})
	if !ok {
		t.Fatal("expected homepage operation to be handled")
	}
	if !strings.Contains(output, `"status":"ok"`) {
		t.Fatalf("expected successful write, got %s", output)
	}

	var revisionCount, eventCount, fileStateCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM homepage_revisions WHERE project_dir = 'site-a'").Scan(&revisionCount); err != nil {
		t.Fatalf("count revisions: %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM homepage_site_events WHERE event_type = 'file_written'").Scan(&eventCount); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM homepage_site_file_state WHERE rel_path = 'index.html'").Scan(&fileStateCount); err != nil {
		t.Fatalf("count file state: %v", err)
	}
	if revisionCount != 1 || eventCount != 1 || fileStateCount != 1 {
		t.Fatalf("ledger counts revision=%d event=%d file_state=%d, want all 1", revisionCount, eventCount, fileStateCount)
	}
}
