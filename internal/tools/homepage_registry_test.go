package tools

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestInitHomepageRegistryDB(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test_homepage.db"))
	if err != nil {
		t.Fatalf("InitHomepageRegistryDB failed: %v", err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM homepage_projects").Scan(&count); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 rows, got %d", count)
	}
}

func TestRegisterAndGetProject(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	proj := HomepageProject{
		Name:        "My Portfolio",
		Description: "Personal portfolio website",
		Framework:   "astro",
		URL:         "https://mysite.example.com",
		ProjectDir:  "/workspace/portfolio",
		Status:      "active",
		Tags:        []string{"portfolio", "personal"},
	}

	id, _, regErr := RegisterProject(db, proj)
	if regErr != nil {
		t.Fatalf("RegisterProject failed: %v", regErr)
	}
	if id <= 0 {
		t.Fatalf("expected positive ID, got %d", id)
	}

	got, getErr := GetProject(db, id)
	if getErr != nil {
		t.Fatalf("GetProject failed: %v", getErr)
	}
	if got.Name != "My Portfolio" {
		t.Errorf("name = %q, want %q", got.Name, "My Portfolio")
	}
	if got.Framework != "astro" {
		t.Errorf("framework = %q, want %q", got.Framework, "astro")
	}
}

func TestRegisterProjectDedup(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	proj := HomepageProject{
		Name:       "TestSite",
		Framework:  "react",
		ProjectDir: "/workspace/testsite",
		Status:     "active",
	}

	id1, _, _ := RegisterProject(db, proj)
	id2, _, _ := RegisterProject(db, proj) // duplicate name

	if id1 != id2 {
		t.Errorf("expected dedup to return same ID: got %d and %d", id1, id2)
	}
}

func TestGetProjectByName(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	RegisterProject(db, HomepageProject{Name: "SiteA", Framework: "vue"})

	got, getErr := GetProjectByName(db, "SiteA")
	if getErr != nil {
		t.Fatalf("GetProjectByName failed: %v", getErr)
	}
	if got.Framework != "vue" {
		t.Errorf("framework = %q, want %q", got.Framework, "vue")
	}
}

func TestGetProjectByDir(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	RegisterProject(db, HomepageProject{Name: "DirSite", ProjectDir: "/workspace/dirsite", Framework: "svelte"})

	got, getErr := GetProjectByDir(db, "/workspace/dirsite")
	if getErr != nil {
		t.Fatalf("GetProjectByDir failed: %v", getErr)
	}
	if got.Name != "DirSite" {
		t.Errorf("name = %q, want %q", got.Name, "DirSite")
	}
}

func TestSearchProjects(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	RegisterProject(db, HomepageProject{Name: "Portfolio", Description: "Personal site", Framework: "astro"})
	RegisterProject(db, HomepageProject{Name: "Blog", Description: "Tech blog", Framework: "hugo"})

	results, _, searchErr := SearchProjects(db, "portfolio", "", nil, 10, 0)
	if searchErr != nil {
		t.Fatalf("SearchProjects failed: %v", searchErr)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'portfolio', got %d", len(results))
	}

	results, _, _ = SearchProjects(db, "astro", "", nil, 10, 0)
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'astro', got %d", len(results))
	}
}

func TestLogEditAndDeploy(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	id, _, _ := RegisterProject(db, HomepageProject{Name: "EditTest", Framework: "next"})

	if err := LogEdit(db, id, "Added contact form"); err != nil {
		t.Fatalf("LogEdit failed: %v", err)
	}

	proj, _ := GetProject(db, id)
	if proj.LastEditReason != "Added contact form" {
		t.Errorf("last_edit_reason = %q, want %q", proj.LastEditReason, "Added contact form")
	}
	if proj.LastEditedAt == "" {
		t.Error("last_edited_at should be set after LogEdit")
	}

	if err := LogDeploy(db, id, "https://example.com"); err != nil {
		t.Fatalf("LogDeploy failed: %v", err)
	}

	proj, _ = GetProject(db, id)
	if proj.LastDeployURL != "https://example.com" {
		t.Errorf("last_deploy_url = %q, want %q", proj.LastDeployURL, "https://example.com")
	}
	if proj.LastDeployedAt == "" {
		t.Error("last_deployed_at should be set after LogDeploy")
	}
}

func TestLogProblemAndResolve(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	id, _, _ := RegisterProject(db, HomepageProject{Name: "ProblemTest", Framework: "gatsby"})

	if err := LogProblem(db, id, "Mobile nav broken"); err != nil {
		t.Fatalf("LogProblem failed: %v", err)
	}

	proj, _ := GetProject(db, id)
	if proj.KnownProblems == "" || !strings.Contains(proj.KnownProblems, "Mobile nav broken") {
		t.Errorf("known_problems = %q, should contain %q", proj.KnownProblems, "Mobile nav broken")
	}

	if err := ResolveProblem(db, id, "Mobile nav broken"); err != nil {
		t.Fatalf("ResolveProblem failed: %v", err)
	}

	proj, _ = GetProject(db, id)
	if proj.KnownProblems != "" {
		t.Errorf("known_problems after resolve = %q, want empty", proj.KnownProblems)
	}
}

func TestListProjects(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	RegisterProject(db, HomepageProject{Name: "A", Status: "active"})
	RegisterProject(db, HomepageProject{Name: "B", Status: "archived"})
	RegisterProject(db, HomepageProject{Name: "C", Status: "active"})

	all, _, _ := ListProjects(db, "", 100, 0)
	if len(all) != 3 {
		t.Errorf("expected 3 projects, got %d", len(all))
	}

	active, _, _ := ListProjects(db, "active", 100, 0)
	if len(active) != 2 {
		t.Errorf("expected 2 active projects, got %d", len(active))
	}
}
