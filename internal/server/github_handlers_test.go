package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
)

func newGitHubHandlerTestServer(t *testing.T) (*Server, string) {
	t.Helper()

	workspaceDir := t.TempDir()
	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	if err := vault.WriteSecret("github_token", "token"); err != nil {
		t.Fatalf("WriteSecret: %v", err)
	}

	cfg := &config.Config{}
	cfg.GitHub.Enabled = true
	cfg.GitHub.Owner = "owner"
	cfg.GitHub.AllowedRepos = []string{"owner/allowed"}
	cfg.Directories.WorkspaceDir = workspaceDir

	return &Server{Cfg: cfg, Vault: vault, Logger: slog.Default()}, workspaceDir
}

func writeGitHubHandlerTrackedProjects(t *testing.T, workspaceDir string) {
	t.Helper()
	dir := filepath.Join(workspaceDir, "github")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	projects := []tools.TrackedProject{
		{Name: "trusted", FullName: "owner/trusted", Owner: "owner", Purpose: "created by agent", AgentCreated: true},
		{Name: "manual", FullName: "owner/manual", Owner: "owner", Purpose: "manual tracking", AgentCreated: false},
	}
	data, err := json.Marshal(projects)
	if err != nil {
		t.Fatalf("marshal projects: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "projects.json"), data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func fakeGitHubReposJSON() string {
	return `{"status":"ok","count":4,"repos":[
		{"name":"allowed","full_name":"owner/allowed","html_url":"https://github.com/owner/allowed","clone_url":"https://github.com/owner/allowed.git"},
		{"name":"trusted","full_name":"owner/trusted","html_url":"https://github.com/owner/trusted","clone_url":"https://github.com/owner/trusted.git"},
		{"name":"manual","full_name":"owner/manual","html_url":"https://github.com/owner/manual","clone_url":"https://github.com/owner/manual.git"},
		{"name":"blocked","full_name":"owner/blocked","html_url":"https://github.com/owner/blocked","clone_url":"https://github.com/owner/blocked.git"}
	]}`
}

func TestHandleDashboardGitHubReposFiltersAllowedAndTrustedOnly(t *testing.T) {
	s, workspaceDir := newGitHubHandlerTestServer(t)
	writeGitHubHandlerTrackedProjects(t, workspaceDir)

	oldListRepos := githubListReposForServer
	githubListReposForServer = func(cfg tools.GitHubConfig, owner string) string {
		if cfg.ListReposUnrestricted {
			t.Fatal("dashboard must not request unrestricted GitHub repo listing")
		}
		return fakeGitHubReposJSON()
	}
	defer func() { githubListReposForServer = oldListRepos }()

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/github-repos", nil)
	rec := httptest.NewRecorder()
	handleDashboardGitHubRepos(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body struct {
		Repos []map[string]interface{} `json:"repos"`
		Count int                      `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v\n%s", err, rec.Body.String())
	}
	if body.Count != 2 || len(body.Repos) != 2 {
		t.Fatalf("repo count = %d/%d, want 2; body=%s", body.Count, len(body.Repos), rec.Body.String())
	}
	seen := map[string]bool{}
	for _, repo := range body.Repos {
		seen[repo["full_name"].(string)] = true
	}
	if !seen["owner/allowed"] || !seen["owner/trusted"] {
		t.Fatalf("expected allowed and trusted repos, got %#v", seen)
	}
	if seen["owner/manual"] || seen["owner/blocked"] {
		t.Fatalf("manual or blocked repo leaked into dashboard: %#v", seen)
	}
}

func TestHandleGitHubReposForUIListsAllReposAndAnnotatesPolicy(t *testing.T) {
	s, workspaceDir := newGitHubHandlerTestServer(t)
	writeGitHubHandlerTrackedProjects(t, workspaceDir)

	oldListRepos := githubListReposForServer
	githubListReposForServer = func(cfg tools.GitHubConfig, owner string) string {
		if !cfg.ListReposUnrestricted {
			t.Fatal("config UI must request unrestricted listing so users can grant repos")
		}
		return fakeGitHubReposJSON()
	}
	defer func() { githubListReposForServer = oldListRepos }()

	req := httptest.NewRequest(http.MethodGet, "/api/github/repos", nil)
	rec := httptest.NewRecorder()
	handleGitHubReposForUI(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body struct {
		Status string                   `json:"status"`
		Repos  []map[string]interface{} `json:"repos"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v\n%s", err, rec.Body.String())
	}
	if body.Status != "ok" || len(body.Repos) != 4 {
		t.Fatalf("expected all repos in config response, got status=%q len=%d body=%s", body.Status, len(body.Repos), rec.Body.String())
	}

	byFullName := map[string]map[string]interface{}{}
	for _, repo := range body.Repos {
		byFullName[repo["full_name"].(string)] = repo
	}
	if byFullName["owner/allowed"]["allowed"] != true {
		t.Fatalf("allowed repo annotation missing: %#v", byFullName["owner/allowed"])
	}
	if byFullName["owner/trusted"]["agent_created"] != true {
		t.Fatalf("trusted repo annotation missing: %#v", byFullName["owner/trusted"])
	}
	if byFullName["owner/manual"]["agent_created"] == true {
		t.Fatalf("manual tracked repo must not be agent_created: %#v", byFullName["owner/manual"])
	}
	if byFullName["owner/blocked"]["allowed"] == true {
		t.Fatalf("blocked repo must not be allowed: %#v", byFullName["owner/blocked"])
	}
}
