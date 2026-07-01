package tools

import (
	"aurago/internal/testutil"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitHubRepoEndpointEscapesOwnerAndRepo(t *testing.T) {
	got := githubRepoEndpoint("acme org", "repo/name", "issues")
	want := "/repos/acme%20org/repo%2Fname/issues"
	if got != want {
		t.Fatalf("githubRepoEndpoint() = %q, want %q", got, want)
	}
}

func TestGitHubContentEndpointEscapesPathSegments(t *testing.T) {
	got := githubContentEndpoint("acme", "repo", "docs/My File?.md")
	want := "/repos/acme/repo/contents/docs/My%20File%3F.md"
	if got != want {
		t.Fatalf("githubContentEndpoint() = %q, want %q", got, want)
	}
}

func TestGitHubRequestRejectsOversizedResponseBody(t *testing.T) {
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("g", int(maxHTTPResponseSize)+1)))
	}))
	defer server.Close()

	oldClient := githubHTTPClient
	githubHTTPClient = server.Client()
	defer func() { githubHTTPClient = oldClient }()

	_, _, err := githubRequest(GitHubConfig{Token: "token", BaseURL: server.URL}, http.MethodGet, "/repos", nil)
	if err == nil || !strings.Contains(err.Error(), "response body exceeds limit") {
		t.Fatalf("expected oversized response error, got %v", err)
	}
}

func TestGitHubDirectMutationsRespectReadOnly(t *testing.T) {
	cfg := GitHubConfig{Token: "token", Owner: "owner", ReadOnly: true}

	tests := map[string]string{
		"create repo":           GitHubCreateRepo(cfg, "repo", "", nil),
		"delete repo":           GitHubDeleteRepo(cfg, "owner", "repo"),
		"create issue":          GitHubCreateIssue(cfg, "owner", "repo", "title", "", nil),
		"close issue":           GitHubCloseIssue(cfg, "owner", "repo", 1),
		"create or update file": GitHubCreateOrUpdateFile(cfg, "owner", "repo", "README.md", "content", "message", "", "main"),
	}

	for name, got := range tests {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(got, `"status":"error"`) || !strings.Contains(strings.ToLower(got), "read-only") {
				t.Fatalf("expected read-only error, got %s", got)
			}
		})
	}
}

func TestGitHubDirectRepoAccessRespectsAllowedRepos(t *testing.T) {
	cfg := GitHubConfig{Token: "token", Owner: "owner", AllowedRepos: []string{"allowed"}}

	got := GitHubGetRepo(cfg, "owner", "blocked")
	if !strings.Contains(got, `"status":"error"`) || !strings.Contains(got, "allowed repos") {
		t.Fatalf("expected allowed repos error, got %s", got)
	}

	got = GitHubCreateIssue(cfg, "owner", "blocked", "title", "", nil)
	if !strings.Contains(got, `"status":"error"`) || !strings.Contains(got, "allowed repos") {
		t.Fatalf("expected allowed repos error for mutation, got %s", got)
	}
}

func TestGitHubEmptyAllowedReposBlocksUntrustedRepo(t *testing.T) {
	cfg := GitHubConfig{Token: "token", Owner: "owner"}

	got := GitHubGetRepo(cfg, "owner", "repo")
	if !strings.Contains(got, `"status":"error"`) || !strings.Contains(got, "allowed repos") {
		t.Fatalf("expected empty allowlist to block untrusted repo, got %s", got)
	}
}

func TestGitHubTrustedAgentCreatedRepoAllowsEmptyAllowedRepos(t *testing.T) {
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"name":              "repo",
			"full_name":         "owner/repo",
			"private":           true,
			"default_branch":    "main",
			"html_url":          "https://github.com/owner/repo",
			"clone_url":         "https://github.com/owner/repo.git",
			"open_issues_count": 0,
			"stargazers_count":  0,
			"forks_count":       0,
		})
	}))
	defer server.Close()

	oldClient := githubHTTPClient
	githubHTTPClient = server.Client()
	defer func() { githubHTTPClient = oldClient }()

	cfg := GitHubConfig{Token: "token", Owner: "owner", BaseURL: server.URL, TrustedRepos: []string{"owner/repo"}}
	got := GitHubGetRepo(cfg, "owner", "repo")
	if !strings.Contains(got, `"status":"ok"`) || !strings.Contains(got, `"full_name":"owner/repo"`) {
		t.Fatalf("expected trusted repo to be allowed, got %s", got)
	}
}

func TestGitHubBareAllowedRepoIsScopedToConfiguredOwner(t *testing.T) {
	cfg := GitHubConfig{Token: "token", Owner: "owner", AllowedRepos: []string{"repo"}}

	if !GitHubRepoAllowed(cfg, "owner", "repo") {
		t.Fatal("expected bare allowlist entry to allow configured owner repo")
	}
	if GitHubRepoAllowed(cfg, "other", "repo") {
		t.Fatal("bare allowlist entry must not allow same repo name under another owner")
	}
}

func TestGitHubFullNameAllowedRepoIsOwnerSafe(t *testing.T) {
	cfg := GitHubConfig{Token: "token", Owner: "owner", AllowedRepos: []string{"other/repo"}}

	if !GitHubRepoAllowed(cfg, "other", "repo") {
		t.Fatal("expected owner/repo allowlist entry to allow exact owner")
	}
	if GitHubRepoAllowed(cfg, "owner", "repo") {
		t.Fatal("owner/repo allowlist entry must not allow a different owner")
	}
}

func TestGitHubListReposFiltersAllowedAndTrustedRepos(t *testing.T) {
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/repos" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]map[string]interface{}{
			{"name": "allowed", "full_name": "owner/allowed", "html_url": "https://github.com/owner/allowed", "clone_url": "https://github.com/owner/allowed.git"},
			{"name": "trusted", "full_name": "owner/trusted", "html_url": "https://github.com/owner/trusted", "clone_url": "https://github.com/owner/trusted.git"},
			{"name": "blocked", "full_name": "owner/blocked", "html_url": "https://github.com/owner/blocked", "clone_url": "https://github.com/owner/blocked.git"},
		})
	}))
	defer server.Close()

	oldClient := githubHTTPClient
	githubHTTPClient = server.Client()
	defer func() { githubHTTPClient = oldClient }()

	cfg := GitHubConfig{
		Token:        "token",
		Owner:        "owner",
		BaseURL:      server.URL,
		AllowedRepos: []string{"owner/allowed"},
		TrustedRepos: []string{"owner/trusted"},
	}
	got := GitHubListRepos(cfg, "")
	if !strings.Contains(got, `"count":2`) {
		t.Fatalf("expected filtered repo count 2, got %s", got)
	}
	if !strings.Contains(got, `"full_name":"owner/allowed"`) || !strings.Contains(got, `"full_name":"owner/trusted"`) {
		t.Fatalf("expected allowed and trusted repos, got %s", got)
	}
	if strings.Contains(got, "owner/blocked") {
		t.Fatalf("blocked repo leaked from filtered list: %s", got)
	}

	cfg.ListReposUnrestricted = true
	got = GitHubListRepos(cfg, "")
	if !strings.Contains(got, `"count":3`) || !strings.Contains(got, "owner/blocked") {
		t.Fatalf("expected unrestricted list to include all repos, got %s", got)
	}
}

func TestGitHubCreateRepoTracksAgentCreatedProject(t *testing.T) {
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/user/repos" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"name":      "repo",
			"full_name": "owner/repo",
			"private":   true,
			"html_url":  "https://github.com/owner/repo",
			"clone_url": "https://github.com/owner/repo.git",
		})
	}))
	defer server.Close()

	oldClient := githubHTTPClient
	githubHTTPClient = server.Client()
	defer func() { githubHTTPClient = oldClient }()

	workspaceDir := t.TempDir()
	cfg := GitHubConfig{Token: "token", Owner: "owner", BaseURL: server.URL, WorkspaceDir: workspaceDir, DefaultPrivate: true}
	got := GitHubCreateRepo(cfg, "repo", "demo purpose", nil)
	if !strings.Contains(got, `"status":"ok"`) {
		t.Fatalf("expected create success, got %s", got)
	}

	data, err := os.ReadFile(filepath.Join(workspaceDir, "github", "projects.json"))
	if err != nil {
		t.Fatalf("expected tracked projects file: %v", err)
	}
	var projects []TrackedProject
	if err := json.Unmarshal(data, &projects); err != nil {
		t.Fatalf("unmarshal projects: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("projects len = %d, want 1", len(projects))
	}
	if !projects[0].AgentCreated || projects[0].FullName != "owner/repo" || projects[0].RepoURL == "" || projects[0].CloneURL == "" {
		t.Fatalf("unexpected tracked project: %+v", projects[0])
	}
}
