package tools

import (
	"aurago/internal/testutil"
	"net/http"
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
