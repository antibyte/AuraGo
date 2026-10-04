package tools

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestGitHubWorkspaceMarkerCannotGrantTrust(t *testing.T) {
	workspace := t.TempDir()
	if err := saveProjects(workspace, []TrackedProject{{Owner: "victim", Name: "private", FullName: "victim/private", AgentCreated: true}}); err != nil {
		t.Fatal(err)
	}
	if got := GitHubTrustedProjectRepos(workspace); len(got) != 0 {
		t.Fatalf("editable project marker granted trust: %v", got)
	}
}

func TestGitHubDeleteRequiresSeparateGrant(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.WriteHeader(204) }))
	defer server.Close()
	old := githubHTTPClient
	githubHTTPClient = server.Client()
	defer func() { githubHTTPClient = old }()
	GitHubDeleteRepo(GitHubConfig{BaseURL: server.URL, Owner: "owner", Token: "fixture-only", AllowedRepos: []string{"owner/repo"}}, "owner", "repo")
	if requests.Load() != 0 {
		t.Fatal("repository deleted without explicit delete grant")
	}
}

func TestGitHubProtectedTrustBackupsAndCorruption(t *testing.T) {
	cfg := GitHubConfig{DataDir: t.TempDir()}
	if err := setGitHubCreatedTrust(cfg, "Owner/Repo", true); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cfg.DataDir, "github_trust.json")
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := setGitHubCreatedTrust(cfg, "owner/repo", true); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(path + ".bak")
	if err != nil || !bytes.Equal(backup, first) {
		t.Fatalf("last valid backup lost: %v", err)
	}
	if got := GitHubTrustedProjectRepos(cfg.DataDir); len(got) != 1 {
		t.Fatalf("duplicate grant: %v", got)
	}
	if err := setGitHubCreatedTrust(cfg, "owner/repo", false); err != nil {
		t.Fatal(err)
	}
	if got := GitHubTrustedProjectRepos(cfg.DataDir); len(got) != 0 {
		t.Fatalf("revoked grant remains: %v", got)
	}
	broken := []byte(`{"version":1,"grants":[`)
	if err := os.WriteFile(path, broken, 0600); err != nil {
		t.Fatal(err)
	}
	if got := GitHubTrustedProjectRepos(cfg.DataDir); len(got) != 0 {
		t.Fatal("corrupt ledger granted trust")
	}
	if err := setGitHubCreatedTrust(cfg, "owner/new", true); err == nil {
		t.Fatal("corrupt ledger overwritten")
	}
	current, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(current, broken) {
		t.Fatalf("corrupt evidence lost: %v", err)
	}
}

func TestGitHubDeleteRevokesTrustBeforeRequest(t *testing.T) {
	cfg := GitHubConfig{DataDir: t.TempDir(), Owner: "owner", AllowDelete: true}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if got := GitHubTrustedProjectRepos(cfg.DataDir, cfg.BaseURL); len(got) != 0 {
			t.Error("trust still present during deletion")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	cfg.BaseURL = server.URL
	if err := setGitHubCreatedTrust(cfg, "owner/repo", true); err != nil {
		t.Fatal(err)
	}
	cfg.TrustedRepos = GitHubTrustedProjectRepos(cfg.DataDir, cfg.BaseURL)
	old := githubHTTPClient
	githubHTTPClient = server.Client()
	defer func() { githubHTTPClient = old }()
	GitHubDeleteRepo(cfg, "owner", "repo")
	if requests.Load() != 1 {
		t.Fatalf("delete requests: %d", requests.Load())
	}
}
