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
