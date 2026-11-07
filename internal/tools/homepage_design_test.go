package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// markHomepageDockerUnavailable pins the Docker availability cache to
// "unavailable" for a host key that no other test uses, so a test takes the
// local filesystem path without probing the real Docker daemon.
func markHomepageDockerUnavailable(host string) {
	dockerAvailabilityMu.Lock()
	dockerAvailabilityResults[host] = dockerAvailabilityEntry{available: false, expiry: time.Now().Add(time.Minute)}
	dockerAvailabilityMu.Unlock()
}

func TestApplyHomepageDesignWritesProjectDesignFile(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	host := "unit-test-apply-design"
	markHomepageDockerUnavailable(host)
	cfg := HomepageConfig{WorkspacePath: workspace, DockerHost: host}

	if err := ApplyHomepageDesign(cfg, "my-site", "genesis"); err != nil {
		t.Fatalf("ApplyHomepageDesign: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(workspace, "my-site", "DESIGN.md"))
	if err != nil {
		t.Fatalf("read DESIGN.md: %v", err)
	}
	if !strings.Contains(string(data), "## Colors") {
		t.Fatalf("DESIGN.md does not carry the template:\n%s", data)
	}
}

func TestApplyHomepageDesignRejectsUnknownTemplateAndTraversal(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	host := "unit-test-apply-design-invalid"
	markHomepageDockerUnavailable(host)
	cfg := HomepageConfig{WorkspacePath: workspace, DockerHost: host}

	if err := ApplyHomepageDesign(cfg, "my-site", "does-not-exist"); err == nil {
		t.Fatal("expected an error for an unknown template")
	}
	if err := ApplyHomepageDesign(cfg, "../escape", "genesis"); err == nil {
		t.Fatal("expected an error for a traversing project_dir")
	}
	if err := ApplyHomepageDesign(cfg, "", "genesis"); err == nil {
		t.Fatal("expected an error for a missing project_dir")
	}
	if _, err := os.Stat(filepath.Join(workspace, "escape")); err == nil {
		t.Fatal("a rejected project_dir must not be written")
	}
}

func TestHomepageProjectDesignRoundTrip(t *testing.T) {
	t.Parallel()
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "homepage.db"))
	if err != nil {
		t.Fatalf("init registry: %v", err)
	}
	defer db.Close()

	if _, _, err := RegisterProject(db, HomepageProject{Name: "RoundTrip", ProjectDir: "round-trip", Framework: "astro"}); err != nil {
		t.Fatalf("register project: %v", err)
	}
	if _, err := GetHomepageProjectDesign(db, "round-trip"); err != nil {
		t.Fatalf("read empty design: %v", err)
	}
	if err := SetHomepageProjectDesign(db, "round-trip", "shopvibe"); err != nil {
		t.Fatalf("set design: %v", err)
	}
	stored, err := GetHomepageProjectDesign(db, "round-trip")
	if err != nil {
		t.Fatalf("read design: %v", err)
	}
	if stored != "shopvibe" {
		t.Fatalf("stored design = %q, want shopvibe", stored)
	}
	if err := SetHomepageProjectDesign(db, "round-trip", "nope"); err == nil {
		t.Fatal("expected an error for an unknown template")
	}
}

func TestManagedSiteExposesDesignID(t *testing.T) {
	t.Parallel()
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "homepage.db"))
	if err != nil {
		t.Fatalf("init registry: %v", err)
	}
	defer db.Close()

	if _, _, err := RegisterProject(db, HomepageProject{Name: "SiteWithDesign", ProjectDir: "site-with-design", Framework: "astro"}); err != nil {
		t.Fatalf("register project: %v", err)
	}
	if err := SetHomepageProjectDesign(db, "site-with-design", "docuforge"); err != nil {
		t.Fatalf("set design: %v", err)
	}
	sites, err := ListHomepageManagedSites(db)
	if err != nil {
		t.Fatalf("list sites: %v", err)
	}
	if len(sites) != 1 {
		t.Fatalf("sites = %d, want 1", len(sites))
	}
	if sites[0].DesignID != "docuforge" {
		t.Fatalf("site design = %q, want docuforge", sites[0].DesignID)
	}
}

func TestHomepageInitProjectWithDesignAppliesDesignFile(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	host := "unit-test-init-project-design"
	markHomepageDockerUnavailable(host)
	cfg := HomepageConfig{WorkspacePath: workspace, DockerHost: host}

	result := HomepageInitProjectWithDesign(cfg, "html", "ki-news", "", "genesis", slogDiscard())
	if !strings.Contains(result, `"status":"ok"`) {
		t.Fatalf("expected init_project to succeed, got: %s", result)
	}
	if !strings.Contains(result, `"design_id":"genesis"`) || !strings.Contains(result, `"design_applied":true`) {
		t.Fatalf("result does not report the applied design: %s", result)
	}
	data, err := os.ReadFile(filepath.Join(workspace, "ki-news", "DESIGN.md"))
	if err != nil {
		t.Fatalf("read DESIGN.md: %v", err)
	}
	if !strings.Contains(string(data), "## Colors") {
		t.Fatalf("DESIGN.md does not carry the template:\n%s", data)
	}
}

func TestHomepageInitProjectWithDesignWithoutDesignWritesNoDesignFile(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	host := "unit-test-init-project-no-design"
	markHomepageDockerUnavailable(host)
	cfg := HomepageConfig{WorkspacePath: workspace, DockerHost: host}

	result := HomepageInitProjectWithDesign(cfg, "html", "plain-site", "", "", slogDiscard())
	if !strings.Contains(result, `"status":"ok"`) {
		t.Fatalf("expected init_project to succeed, got: %s", result)
	}
	if _, err := os.Stat(filepath.Join(workspace, "plain-site", "DESIGN.md")); err == nil {
		t.Fatal("no DESIGN.md must be written when no design was requested")
	}
}

func TestHomepageInitProjectWithDesignKeepsProjectWhenDesignFails(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	host := "unit-test-init-project-bad-design"
	markHomepageDockerUnavailable(host)
	cfg := HomepageConfig{WorkspacePath: workspace, DockerHost: host}

	result := HomepageInitProjectWithDesign(cfg, "html", "half-site", "", "does-not-exist", slogDiscard())
	if !strings.Contains(result, `"status":"ok"`) {
		t.Fatalf("the scaffold must survive a failing design, got: %s", result)
	}
	if !strings.Contains(result, `"design_applied":false`) || !strings.Contains(result, `"design_error"`) {
		t.Fatalf("result does not report the design failure: %s", result)
	}
	if _, err := os.Stat(filepath.Join(workspace, "half-site", "index.html")); err != nil {
		t.Fatalf("the project must still exist after a design failure: %v", err)
	}
}
