package tools

import (
	"aurago/internal/testutil"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

	_, _, _, err := githubRequest(context.Background(), GitHubConfig{Token: "token", BaseURL: server.URL}, http.MethodGet, "/repos", nil)
	if err == nil || !strings.Contains(err.Error(), "response body exceeds limit") {
		t.Fatalf("expected oversized response error, got %v", err)
	}
}

func TestGitHubRequestUsesCallerContext(t *testing.T) {
	started := make(chan struct{})
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
	defer server.Close()

	oldClient := githubHTTPClient
	githubHTTPClient = server.Client()
	defer func() { githubHTTPClient = oldClient }()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _, _, _ = githubRequest(ctx, GitHubConfig{Token: "token", BaseURL: server.URL}, http.MethodGet, "/user", nil)
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not reach the server")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("canceled caller context did not finish githubRequest")
	}
}

func TestGitHubDirectMutationsRespectReadOnly(t *testing.T) {
	cfg := GitHubConfig{Token: "token", Owner: "owner", ReadOnly: true}

	tests := map[string]string{
		"create repo":           GitHubCreateRepo(context.Background(), cfg, "repo", "", nil),
		"delete repo":           GitHubDeleteRepo(context.Background(), cfg, "owner", "repo"),
		"create issue":          GitHubCreateIssue(context.Background(), cfg, "owner", "repo", "title", "", nil),
		"close issue":           GitHubCloseIssue(context.Background(), cfg, "owner", "repo", 1),
		"create or update file": GitHubCreateOrUpdateFile(context.Background(), cfg, "owner", "repo", "README.md", "content", "message", "", "main"),
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

	got := GitHubGetRepo(context.Background(), cfg, "owner", "blocked")
	if !strings.Contains(got, `"status":"error"`) || !strings.Contains(got, "allowed repos") {
		t.Fatalf("expected allowed repos error, got %s", got)
	}

	got = GitHubCreateIssue(context.Background(), cfg, "owner", "blocked", "title", "", nil)
	if !strings.Contains(got, `"status":"error"`) || !strings.Contains(got, "allowed repos") {
		t.Fatalf("expected allowed repos error for mutation, got %s", got)
	}
}

func TestGitHubEmptyAllowedReposBlocksUntrustedRepo(t *testing.T) {
	cfg := GitHubConfig{Token: "token", Owner: "owner"}

	got := GitHubGetRepo(context.Background(), cfg, "owner", "repo")
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
	got := GitHubGetRepo(context.Background(), cfg, "owner", "repo")
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
	got := GitHubListRepos(context.Background(), cfg, "", 0)
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
	got = GitHubListRepos(context.Background(), cfg, "", 0)
	if !strings.Contains(got, `"count":3`) || !strings.Contains(got, "owner/blocked") {
		t.Fatalf("expected unrestricted list to include all repos, got %s", got)
	}
}

func TestGitHubCreateRepoTracksAgentCreatedProject(t *testing.T) {
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/user" {
			_, _ = w.Write([]byte(`{"login":"owner"}`))
			return
		}
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
	cfg := GitHubConfig{Token: "token", Owner: "owner", BaseURL: server.URL, WorkspaceDir: workspaceDir, DataDir: t.TempDir(), DefaultPrivate: true}
	got := GitHubCreateRepo(context.Background(), cfg, "repo", "demo purpose", nil)
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
	if got := GitHubTrustedProjectRepos(cfg.DataDir, cfg.BaseURL); len(got) != 1 || got[0] != "owner/repo" {
		t.Fatalf("missing protected trust: %v", got)
	}
	if got := GitHubTrustedProjectRepos(cfg.DataDir, "https://different.example/api"); len(got) != 0 {
		t.Fatal("trust crossed API identity")
	}
}

func TestGitHubProjectDirRejectsTraversal(t *testing.T) {
	workspace := t.TempDir()
	outside := filepath.Join(filepath.Dir(workspace), "github-escape")
	t.Cleanup(func() { _ = os.RemoveAll(outside) })

	for _, name := range []string{"../github-escape", "..", `..\github-escape`, "/tmp/github-escape", `C:\github-escape`} {
		if _, err := githubProjectDir(workspace, name); err == nil {
			t.Fatalf("name %q created a directory", name)
		}
	}
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Fatalf("escape directory exists: %v", err)
	}
}

func TestGitHubProjectDirCreatesInsideWorkspace(t *testing.T) {
	workspace := t.TempDir()
	rel, err := githubProjectDir(workspace, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if rel != filepath.Join("github", "demo") {
		t.Fatalf("relative path = %q", rel)
	}
	info, err := os.Stat(filepath.Join(workspace, rel))
	if err != nil || !info.IsDir() {
		t.Fatalf("project dir: %v", err)
	}
}

func TestGitHubUntrackMatchesCase(t *testing.T) {
	workspace := t.TempDir()
	if err := saveProjects(workspace, []TrackedProject{{Name: "Demo", Purpose: "x"}}); err != nil {
		t.Fatal(err)
	}
	raw := GitHubUntrackProject(workspace, "demo")
	if !strings.Contains(raw, `"status":"ok"`) && !strings.Contains(raw, `"status": "ok"`) {
		t.Fatalf("untrack = %s", raw)
	}
}

func TestGitHubSearchReposOmitsPrivateUnlistedRepos(t *testing.T) {
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/repositories" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"total_count":3,"items":[
			{"full_name":"owner/public","private":false,"stargazers_count":1,"html_url":"https://github.com/owner/public"},
			{"full_name":"owner/secret","private":true,"stargazers_count":2,"html_url":"https://github.com/owner/secret"},
			{"full_name":"owner/allowed","private":true,"stargazers_count":3,"html_url":"https://github.com/owner/allowed"}
		]}`))
	}))
	defer server.Close()
	old := githubHTTPClient
	githubHTTPClient = server.Client()
	defer func() { githubHTTPClient = old }()

	cfg := GitHubConfig{Token: "token", Owner: "owner", BaseURL: server.URL, AllowedRepos: []string{"owner/allowed"}}
	var result struct {
		Repos []struct {
			FullName string `json:"full_name"`
			Private  bool   `json:"private"`
		} `json:"repos"`
	}
	if err := json.Unmarshal([]byte(GitHubSearchRepos(context.Background(), cfg, "demo", 10)), &result); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, repo := range result.Repos {
		seen[repo.FullName] = repo.Private
	}
	if _, ok := seen["owner/public"]; !ok || seen["owner/secret"] || !seen["owner/allowed"] {
		t.Fatalf("repos = %#v", seen)
	}
}

func TestGitHubCreateRepoUsesOrgEndpointWhenOwnerDiffers(t *testing.T) {
	var created string
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/user":
			_, _ = w.Write([]byte(`{"login":"andi"}`))
		case r.Method == http.MethodPost:
			created = r.URL.Path
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"name":"demo","full_name":"acme/demo","private":true,"html_url":"https://github.com/acme/demo","clone_url":"https://github.com/acme/demo.git"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	old := githubHTTPClient
	githubHTTPClient = server.Client()
	defer func() { githubHTTPClient = old }()

	cfg := GitHubConfig{Token: "token", Owner: "acme", BaseURL: server.URL, DataDir: t.TempDir(), WorkspaceDir: t.TempDir()}
	raw := GitHubCreateRepo(context.Background(), cfg, "demo", "purpose", nil)
	if created != "/orgs/acme/repos" {
		t.Fatalf("create path = %q, body %s", created, raw)
	}
}

func TestGitHubCreateRepoUsesUserEndpointWhenLoginMatches(t *testing.T) {
	var created string
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/user":
			_, _ = w.Write([]byte(`{"login":"acme"}`))
		case r.Method == http.MethodPost:
			created = r.URL.Path
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"name":"demo","full_name":"acme/demo","private":true,"html_url":"https://github.com/acme/demo","clone_url":"https://github.com/acme/demo.git"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	old := githubHTTPClient
	githubHTTPClient = server.Client()
	defer func() { githubHTTPClient = old }()

	cfg := GitHubConfig{Token: "token", Owner: "acme", BaseURL: server.URL, DataDir: t.TempDir(), WorkspaceDir: t.TempDir()}
	raw := GitHubCreateRepo(context.Background(), cfg, "demo", "purpose", nil)
	if created != "/user/repos" {
		t.Fatalf("create path = %q, body %s", created, raw)
	}
}

func TestGitHubCreateRepoStopsWhenUserLookupFails(t *testing.T) {
	var created string
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/user":
			w.WriteHeader(http.StatusUnauthorized)
		case r.Method == http.MethodPost:
			created = r.URL.Path
			w.WriteHeader(http.StatusCreated)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	old := githubHTTPClient
	githubHTTPClient = server.Client()
	defer func() { githubHTTPClient = old }()

	cfg := GitHubConfig{Token: "token", Owner: "acme", BaseURL: server.URL, DataDir: t.TempDir(), WorkspaceDir: t.TempDir()}
	raw := GitHubCreateRepo(context.Background(), cfg, "demo", "purpose", nil)
	if created != "" || !strings.Contains(raw, `"status":"error"`) {
		t.Fatalf("create path = %q, body %s", created, raw)
	}
}

func TestGitHubDecodeFileContentStripsBase64Newlines(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("hello"))
	wrapped := encoded[:4] + "\n" + encoded[4:]
	text, encoding, truncated := githubDecodeFileContent("base64", wrapped)
	if text != "hello" || encoding != "utf-8" || truncated {
		t.Fatalf("got %q %s truncated=%v", text, encoding, truncated)
	}
}

func TestGitHubFileContentBase64AcceptsPlainText(t *testing.T) {
	got := githubFileContentBase64("hello")
	want := base64.StdEncoding.EncodeToString([]byte("hello"))
	if got != want {
		t.Fatalf("plain text encoded to %q, want %q", got, want)
	}
	if githubFileContentBase64(want) != want {
		t.Fatal("round-trip base64 was encoded again")
	}
}

func TestGitHubGetFileContentListsDirectory(t *testing.T) {
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"name":"README.md","path":"README.md","type":"file","size":1}]`))
	}))
	defer server.Close()
	old := githubHTTPClient
	githubHTTPClient = server.Client()
	defer func() { githubHTTPClient = old }()

	cfg := GitHubConfig{Token: "token", Owner: "owner", BaseURL: server.URL, AllowedRepos: []string{"owner/repo"}}
	raw := GitHubGetFileContent(context.Background(), cfg, "owner", "repo", "docs", "")
	if !strings.Contains(raw, `"type":"dir"`) || !strings.Contains(raw, `"name":"README.md"`) {
		t.Fatalf("directory listing = %s", raw)
	}
}

func TestGitHubGetFileContentDecodesText(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("hello\n"))
	wrapped := encoded[:4] + "\n" + encoded[4:]
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"name":     "README.md",
			"path":     "README.md",
			"encoding": "base64",
			"content":  wrapped,
		})
	}))
	defer server.Close()
	old := githubHTTPClient
	githubHTTPClient = server.Client()
	defer func() { githubHTTPClient = old }()

	cfg := GitHubConfig{Token: "token", Owner: "owner", BaseURL: server.URL, AllowedRepos: []string{"owner/repo"}}
	var result struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal([]byte(GitHubGetFileContent(context.Background(), cfg, "owner", "repo", "README.md", "")), &result); err != nil {
		t.Fatal(err)
	}
	if result.Content != "hello\n" || result.Encoding != "utf-8" {
		t.Fatalf("content = %q encoding = %q", result.Content, result.Encoding)
	}
}

func TestGitHubListReposFollowsNextPageAndAppliesLimit(t *testing.T) {
	var serverURL string
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("page") {
		case "", "1":
			w.Header().Set("Link", "<"+serverURL+"/user/repos?per_page=100&page=2>; rel=\"next\"")
			_, _ = w.Write([]byte(`[{"name":"one","full_name":"owner/one","private":false}]`))
		case "2":
			_, _ = w.Write([]byte(`[{"name":"two","full_name":"owner/two","private":false}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL
	old := githubHTTPClient
	githubHTTPClient = server.Client()
	defer func() { githubHTTPClient = old }()

	cfg := GitHubConfig{Token: "token", Owner: "owner", BaseURL: server.URL, AllowedRepos: []string{"one", "two"}}
	var result struct {
		Count int `json:"count"`
		Repos []struct {
			Name string `json:"name"`
		} `json:"repos"`
	}
	if err := json.Unmarshal([]byte(GitHubListRepos(context.Background(), cfg, "", 0)), &result); err != nil {
		t.Fatal(err)
	}
	if result.Count != 2 {
		t.Fatalf("count = %d, want both pages", result.Count)
	}
	if err := json.Unmarshal([]byte(GitHubListRepos(context.Background(), cfg, "", 1)), &result); err != nil {
		t.Fatal(err)
	}
	if result.Count != 1 || result.Repos[0].Name != "one" {
		t.Fatalf("limited list = %#v", result)
	}
}
