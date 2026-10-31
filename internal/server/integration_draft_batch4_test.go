package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/testutil"
	"aurago/internal/tools"
)

// Config UX wave 1, batch 4: the test buttons of these integrations send the
// unsaved form values as {"draft": ...}. The saved section stays empty in every
// test, so a request that gets through proves the draft values were used.

// canceledDraftRequest builds a draft test request whose context is already
// canceled: a handler that took the draft credentials fails inside the upstream
// call (no network is touched) instead of reporting the credential as missing.
func canceledDraftRequest(path, body string) *http.Request {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)).WithContext(ctx)
}

func TestHereNowTestUsesDraftKey(t *testing.T) {
	s := newDraftTestServer(t)
	handler := handleHereNowTestConnection(s)

	rec := postDraft(handler, "/api/here-now/test-connection", "")
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "API key not found") {
		t.Fatalf("without a draft: status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, canceledDraftRequest("/api/here-now/test-connection", `{"draft":{"here_now":{"api_key":"draft-here-now-key"}}}`))
	if strings.Contains(rec.Body.String(), "API key not found") {
		t.Fatalf("draft key was ignored: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "draft-here-now-key") {
		t.Fatalf("response leaked the key: %s", rec.Body.String())
	}
}

func TestNetlifyTestUsesDraftToken(t *testing.T) {
	s := newDraftTestServer(t)
	handler := handleNetlifyTestConnection(s)

	rec := postDraft(handler, "/api/netlify/test-connection", `{}`)
	if !strings.Contains(rec.Body.String(), "token is missing") {
		t.Fatalf("without a draft: body=%s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, canceledDraftRequest("/api/netlify/test-connection", `{"draft":{"netlify":{"token":"draft-netlify-token"}}}`))
	if strings.Contains(rec.Body.String(), "token is missing") {
		t.Fatalf("draft token was ignored: body=%s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "draft-netlify-token") {
		t.Fatalf("response leaked the token: %s", rec.Body.String())
	}
}

func TestVercelTestUsesDraftToken(t *testing.T) {
	s := newDraftTestServer(t)
	handler := handleVercelTestConnection(s)

	rec := postDraft(handler, "/api/vercel/test-connection", `{}`)
	if !strings.Contains(rec.Body.String(), "token is missing") {
		t.Fatalf("without a draft: body=%s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, canceledDraftRequest("/api/vercel/test-connection", `{"draft":{"vercel":{"token":"draft-vercel-token"}}}`))
	if strings.Contains(rec.Body.String(), "token is missing") {
		t.Fatalf("draft token was ignored: body=%s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "draft-vercel-token") {
		t.Fatalf("response leaked the token: %s", rec.Body.String())
	}
}

func TestTailscaleTestUsesDraftKey(t *testing.T) {
	s := newDraftTestServer(t)
	handler := handleTailscaleAPITest(s)

	// Nothing typed and nothing saved: the probe reports the missing credential.
	rec := postDraft(handler, "/api/tailscale/test", "")
	if !strings.Contains(rec.Body.String(), "no_credentials") {
		t.Fatalf("without a draft: body=%s", rec.Body.String())
	}

	// The integration does not have to be switched on to test the typed values.
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, canceledDraftRequest("/api/tailscale/test", `{"draft":{"tailscale":{"api_key":"tskey-api-draft","tailnet":"example.com"}}}`))
	if strings.Contains(rec.Body.String(), "no_credentials") || strings.Contains(rec.Body.String(), "not enabled") {
		t.Fatalf("draft key was ignored: body=%s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "tskey-api-draft") {
		t.Fatalf("response leaked the key: %s", rec.Body.String())
	}
}

func TestGitHubReposUsesDraftTokenAndOwner(t *testing.T) {
	s := newDraftTestServer(t)
	var gotToken, gotOwner string
	old := githubListReposForServer
	githubListReposForServer = func(ctx context.Context, cfg tools.GitHubConfig, owner string) string {
		gotToken, gotOwner = cfg.Token, cfg.Owner
		return `{"status":"ok","count":1,"repos":[{"name":"demo","full_name":"draft-owner/demo"}]}`
	}
	defer func() { githubListReposForServer = old }()

	rec := postDraft(handleGitHubReposForUI(s), "/api/github/repos", `{"draft":{"github":{"token":"draft-gh-token","owner":"draft-owner"}}}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if gotToken != "draft-gh-token" || gotOwner != "draft-owner" {
		t.Fatalf("listing used token=%q owner=%q", gotToken, gotOwner)
	}
	if strings.Contains(rec.Body.String(), "draft-gh-token") {
		t.Fatalf("response leaked the token: %s", rec.Body.String())
	}

	// GET still lists with the saved (empty) configuration: disabled, nothing is sent.
	called := false
	githubListReposForServer = func(ctx context.Context, cfg tools.GitHubConfig, owner string) string {
		called = true
		return `{"status":"ok","repos":[]}`
	}
	getRec := httptest.NewRecorder()
	handleGitHubReposForUI(s).ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/api/github/repos", nil))
	if called || !strings.Contains(getRec.Body.String(), "not enabled") {
		t.Fatalf("GET must keep using the saved config: called=%v body=%s", called, getRec.Body.String())
	}
}

func TestHomepageTestUsesDraftTarget(t *testing.T) {
	s := newDraftTestServer(t)
	handler := handleHomepageTestConnection(s)

	rec := postDraft(handler, "/api/homepage/test-connection", "")
	if !strings.Contains(rec.Body.String(), "host and user are required") {
		t.Fatalf("without a draft: body=%s", rec.Body.String())
	}

	// A closed local port: the connection error names the drafted host and port.
	body := `{"draft":{"homepage":{"deploy_host":"127.0.0.1","deploy_port":1,"deploy_user":"deploy","deploy_password":"draft-pass","deploy_path":"/srv"}}}`
	rec = postDraft(handler, "/api/homepage/test-connection", body)
	if !strings.Contains(rec.Body.String(), "127.0.0.1:1") {
		t.Fatalf("draft target was ignored: body=%s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "draft-pass") {
		t.Fatalf("response leaked the password: %s", rec.Body.String())
	}
}

func TestVirtualComputersStorageTestUsesDraftEndpointAndKeys(t *testing.T) {
	var hits int
	storage := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if !strings.Contains(r.Header.Get("Authorization"), "Credential=draft-access/") {
			t.Errorf("storage request used other credentials: %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer storage.Close()
	s := newDraftTestServer(t)
	mux := http.NewServeMux()
	registerVirtualComputersRoutes(mux, s)

	// Saved configuration: volumes are off and nothing is configured.
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/virtual-computers/storage/test", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("without a draft: status=%d body=%s", rec.Code, rec.Body.String())
	}

	body := fmt.Sprintf(`{"draft":{"virtual_computers":{"allow_volumes":true,"storage":{"mode":"external_s3","endpoint":%q,"bucket":"boring-volumes","use_ssl":false},"s3_access_key_id":"draft-access","s3_secret_key":"draft-secret"}}}`, strings.TrimPrefix(storage.URL, "http://"))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/virtual-computers/storage/test", strings.NewReader(body)))
	if rec.Code != http.StatusOK || hits == 0 {
		t.Fatalf("draft storage was not tested: status=%d hits=%d body=%s", rec.Code, hits, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "draft-secret") {
		t.Fatalf("response leaked the secret key: %s", rec.Body.String())
	}
	if got := s.ConfigSnapshot().VirtualComputers.Storage.Endpoint; got != "" {
		t.Fatalf("saved snapshot was changed: %q", got)
	}
}
