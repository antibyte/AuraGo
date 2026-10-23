package tools

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
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
		ProjectDir:  "portfolio",
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
	if got.ProjectDir != "portfolio" {
		t.Errorf("project_dir = %q, want %q", got.ProjectDir, "portfolio")
	}
}

func TestRegisterProjectRequiresProjectDir(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	_, _, err = RegisterProject(db, HomepageProject{Name: "MissingDir"})
	if err == nil || !strings.Contains(err.Error(), "project_dir is required") {
		t.Fatalf("expected project_dir required error, got %v", err)
	}
}

func TestDispatchHomepageRegistryRegisterRequiresProjectDir(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	tests := []struct {
		name       string
		projectDir string
		want       string
	}{
		{
			name:       "missing",
			projectDir: "",
			want:       "project_dir is required",
		},
		{
			name:       "invalid escaping",
			projectDir: `bad\"..`,
			want:       "path traversal detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DispatchHomepageRegistry(db, "register", "", "MissingDir", "", "html", tt.projectDir, "", "", "", "", "", nil, 0, "", 10, 0)
			var resp struct {
				Status  string `json:"status"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal([]byte(got), &resp); err != nil {
				t.Fatalf("response is not valid JSON: %v; got %s", err, got)
			}
			if resp.Status != "error" || !strings.Contains(resp.Message, tt.want) {
				t.Fatalf("expected %q error, got %+v from %s", tt.want, resp, got)
			}
		})
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
		ProjectDir: "testsite",
		Status:     "active",
	}

	id1, _, _ := RegisterProject(db, proj)
	id2, _, _ := RegisterProject(db, proj) // duplicate name

	if id1 != id2 {
		t.Errorf("expected dedup to return same ID: got %d and %d", id1, id2)
	}
}

func TestRegisterProjectSameNameIncompleteRowDoesNotDedupNewProjectDir(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	res, err := db.Exec(`INSERT INTO homepage_projects (name, project_dir, status) VALUES (?, '', 'active')`, "Site")
	if err != nil {
		t.Fatalf("insert incomplete project: %v", err)
	}
	existingID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("existing id: %v", err)
	}

	id, duplicate, err := RegisterProject(db, HomepageProject{Name: "Site", ProjectDir: "clients/site", Status: "active"})
	if err == nil {
		t.Fatalf("expected same-name project_dir mismatch error, got id=%d duplicate=%v", id, duplicate)
	}
	if duplicate && id == existingID {
		t.Fatalf("same-name incomplete row must not be returned as duplicate for a new project_dir")
	}
}

func TestGetProjectByName(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	RegisterProject(db, HomepageProject{Name: "SiteA", ProjectDir: "site-a", Framework: "vue"})

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

	RegisterProject(db, HomepageProject{Name: "DirSite", ProjectDir: "dirsite", Framework: "svelte"})

	got, getErr := GetProjectByDir(db, "dirsite")
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

	RegisterProject(db, HomepageProject{Name: "Portfolio", ProjectDir: "portfolio", Description: "Personal site", Framework: "astro"})
	RegisterProject(db, HomepageProject{Name: "Blog", ProjectDir: "blog", Description: "Tech blog", Framework: "hugo"})

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

	id, _, _ := RegisterProject(db, HomepageProject{Name: "EditTest", ProjectDir: "edit-test", Framework: "next"})

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

func TestDispatchHomepageRegistryLogDeployRejectsEmptyURL(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	id, _, err := RegisterProject(db, HomepageProject{Name: "DeployTest", ProjectDir: "deploy-test", Status: "active"})
	if err != nil {
		t.Fatalf("register project: %v", err)
	}

	got := DispatchHomepageRegistry(db, "log_deploy", "", "", "", "", "", " ", "", "", "", "", nil, id, "", 10, 0)
	var resp struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(got), &resp); err != nil {
		t.Fatalf("response is not valid JSON: %v; got %s", err, got)
	}
	if resp.Status != "error" || !strings.Contains(resp.Message, "deploy URL or target is required") {
		t.Fatalf("expected empty deploy target error, got %+v from %s", resp, got)
	}

	proj, err := GetProject(db, id)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if proj.LastDeployURL != "" || proj.LastDeployedAt != "" {
		t.Fatalf("empty log_deploy must not update deployment fields, got url=%q at=%q", proj.LastDeployURL, proj.LastDeployedAt)
	}
}

func TestUpdateProjectRejectsInvalidProjectDir(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	id, _, err := RegisterProject(db, HomepageProject{Name: "UpdateTest", ProjectDir: "update-test", Status: "active"})
	if err != nil {
		t.Fatalf("register project: %v", err)
	}

	for _, projectDir := range []string{".", filepath.Join(t.TempDir(), "absolute-site")} {
		t.Run(projectDir, func(t *testing.T) {
			if err := UpdateProject(db, id, map[string]interface{}{"project_dir": projectDir}); err == nil {
				t.Fatalf("expected invalid project_dir %q to be rejected", projectDir)
			}
		})
	}

	if err := UpdateProject(db, id, map[string]interface{}{"project_dir": "clients//site"}); err != nil {
		t.Fatalf("valid nested project_dir update failed: %v", err)
	}
	proj, err := GetProject(db, id)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if proj.ProjectDir != "clients/site" {
		t.Fatalf("project_dir = %q, want normalized clients/site", proj.ProjectDir)
	}
}

func TestDispatchHomepageRegistryUpdateRejectsInvalidProjectDir(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	id, _, err := RegisterProject(db, HomepageProject{Name: "DispatchUpdateTest", ProjectDir: "dispatch-update-test", Status: "active"})
	if err != nil {
		t.Fatalf("register project: %v", err)
	}

	got := DispatchHomepageRegistry(db, "update", "", "", "", "", ".", "", "", "", "", "", nil, id, "", 10, 0)
	var resp struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(got), &resp); err != nil {
		t.Fatalf("response is not valid JSON: %v; got %s", err, got)
	}
	if resp.Status != "error" || !strings.Contains(resp.Message, `project_dir "." is ambiguous`) {
		t.Fatalf("expected invalid project_dir error, got %+v from %s", resp, got)
	}
}

func TestLogProblemAndResolve(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	id, _, _ := RegisterProject(db, HomepageProject{Name: "ProblemTest", ProjectDir: "problem-test", Framework: "gatsby"})

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

	RegisterProject(db, HomepageProject{Name: "A", ProjectDir: "site-a", Status: "active"})
	RegisterProject(db, HomepageProject{Name: "B", ProjectDir: "site-b", Status: "archived"})
	RegisterProject(db, HomepageProject{Name: "C", ProjectDir: "site-c", Status: "active"})

	all, _, _ := ListProjects(db, "", 100, 0)
	if len(all) != 3 {
		t.Errorf("expected 3 projects, got %d", len(all))
	}

	active, _, _ := ListProjects(db, "active", 100, 0)
	if len(active) != 2 {
		t.Errorf("expected 2 active projects, got %d", len(active))
	}
}

// ─── One registry row per project directory ───────────────────────────────

// TestRegisterProjectSameDirectoryKeepsOneRow proves a project directory maps
// to exactly one registry row.
func TestRegisterProjectSameDirectoryKeepsOneRow(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	first, duplicate, err := RegisterProject(db, HomepageProject{Name: "Site", ProjectDir: "site-a", Status: "active"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if duplicate {
		t.Fatalf("the first registration must not be a duplicate")
	}
	second, duplicate, err := RegisterProject(db, HomepageProject{Name: "Site", ProjectDir: "site-a", Status: "active"})
	if err != nil {
		t.Fatalf("register again: %v", err)
	}
	if !duplicate || second != first {
		t.Fatalf("expected the same row %d, got %d (duplicate=%v)", first, second, duplicate)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM homepage_projects WHERE project_dir = 'site-a'").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row for the project directory, got %d", count)
	}
}

// TestRegisterProjectConcurrentRegistrationsKeepOneRow proves concurrent
// registrations of the same project directory create exactly one row.
func TestRegisterProjectConcurrentRegistrationsKeepOneRow(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	const workers = 8
	var wg sync.WaitGroup
	ids := make(chan int64, workers)
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, _, err := RegisterProject(db, HomepageProject{Name: "Site", ProjectDir: "site-a", Status: "active"})
			if err != nil {
				errs <- err
				return
			}
			ids <- id
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		t.Fatalf("register: %v", err)
	}
	first := int64(0)
	for id := range ids {
		if first == 0 {
			first = id
			continue
		}
		if id != first {
			t.Fatalf("expected every registration to return row %d, got %d", first, id)
		}
	}
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM homepage_projects WHERE project_dir = 'site-a'").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row for the project directory, got %d", count)
	}
}

// TestHomepageRegistryMigrationMergesDuplicateProjectDirs proves a registry
// with duplicate project directories migrates to a single row and keeps every
// child row.
func TestHomepageRegistryMigrationMergesDuplicateProjectDirs(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "registry.db")

	db, err := InitHomepageRegistryDB(dbPath)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	// Downgrade the schema and drop the unique index so the migration runs
	// again on the next open.
	if _, err := db.Exec("DROP INDEX IF EXISTS idx_hp_project_dir_unique"); err != nil {
		t.Fatalf("drop index: %v", err)
	}
	if _, err := db.Exec("PRAGMA user_version=1"); err != nil {
		t.Fatalf("downgrade schema: %v", err)
	}

	older, err := db.Exec(`INSERT INTO homepage_projects (name, project_dir, status, updated_at)
		VALUES ('Older', 'site-a', 'active', '2024-01-01 00:00:00')`)
	if err != nil {
		t.Fatalf("insert older: %v", err)
	}
	olderID, _ := older.LastInsertId()
	newer, err := db.Exec(`INSERT INTO homepage_projects (name, project_dir, status, updated_at)
		VALUES ('Newer', 'site-a', 'active', '2025-01-01 00:00:00')`)
	if err != nil {
		t.Fatalf("insert newer: %v", err)
	}
	newerID, _ := newer.LastInsertId()

	childInserts := []string{
		`INSERT INTO homepage_site_events (project_id, event_type, summary) VALUES (?, 'note', 'older event')`,
		`INSERT INTO homepage_revisions (project_id, project_dir) VALUES (?, 'site-a')`,
		`INSERT INTO homepage_remote_observations (project_id, provider) VALUES (?, 'netlify')`,
		`INSERT INTO homepage_deploy_targets (project_id, provider, target_key) VALUES (?, 'netlify', 'legacy:1')`,
		`INSERT INTO homepage_history (project_id, content) VALUES (?, 'older note')`,
	}
	for _, statement := range childInserts {
		if _, err := db.Exec(statement, olderID); err != nil {
			t.Fatalf("insert child row: %v", err)
		}
	}

	// Downgrade the schema so the migration runs again on the next open.
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	migrated, err := InitHomepageRegistryDB(dbPath)
	if err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	defer migrated.Close()

	var count int
	if err := migrated.QueryRow("SELECT COUNT(*) FROM homepage_projects WHERE project_dir = 'site-a'").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row for the project directory after the migration, got %d", count)
	}

	var survivor int64
	if err := migrated.QueryRow("SELECT id FROM homepage_projects WHERE project_dir = 'site-a'").Scan(&survivor); err != nil {
		t.Fatalf("read survivor: %v", err)
	}
	if survivor != newerID {
		t.Fatalf("expected the newest row %d to survive, got %d", newerID, survivor)
	}

	childCounts := map[string]string{
		"homepage_site_events":         "SELECT COUNT(*) FROM homepage_site_events WHERE project_id = ? AND event_type = 'note'",
		"homepage_revisions":           "SELECT COUNT(*) FROM homepage_revisions WHERE project_id = ?",
		"homepage_remote_observations": "SELECT COUNT(*) FROM homepage_remote_observations WHERE project_id = ?",
		"homepage_deploy_targets":      "SELECT COUNT(*) FROM homepage_deploy_targets WHERE project_id = ?",
		"homepage_history":             "SELECT COUNT(*) FROM homepage_history WHERE project_id = ?",
	}
	for table, query := range childCounts {
		var childCount int
		if err := migrated.QueryRow(query, survivor).Scan(&childCount); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if childCount != 1 {
			t.Fatalf("expected the %s child row to survive the merge, got %d", table, childCount)
		}
	}

	var mergeEvents int
	if err := migrated.QueryRow("SELECT COUNT(*) FROM homepage_site_events WHERE event_type = 'project_dir_merged'").Scan(&mergeEvents); err != nil {
		t.Fatalf("count merge events: %v", err)
	}
	if mergeEvents != 1 {
		t.Fatalf("expected 1 recorded merge, got %d", mergeEvents)
	}
}
