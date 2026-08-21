package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type hereNowRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn hereNowRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func hereNowTestResponse(status int, body string, headers ...map[string]string) *http.Response {
	header := make(http.Header)
	if len(headers) > 0 {
		for key, value := range headers[0] {
			header.Set(key, value)
		}
	}
	return &http.Response{
		StatusCode: status,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func newHereNowTestClient(t *testing.T, defaultAccount string, transport hereNowRoundTripFunc) *HereNowClient {
	t.Helper()
	httpClient := &http.Client{Transport: transport}
	client, err := newHereNowClient("https://here.now", "test-api-key", defaultAccount, httpClient)
	if err != nil {
		t.Fatalf("newHereNowClient: %v", err)
	}
	client.uploadClient = func(string) (*http.Client, error) { return httpClient, nil }
	client.sleep = func(context.Context, time.Duration) error { return nil }
	return client
}

func TestHereNowRequestHeadersPaginationAndStructuredErrors(t *testing.T) {
	var requests int
	client := newHereNowTestClient(t, "workspace-default", func(req *http.Request) (*http.Response, error) {
		requests++
		if got := req.Header.Get("Authorization"); got != "Bearer test-api-key" {
			t.Fatalf("Authorization = %q", got)
		}
		if got := req.Header.Get("X-HereNow-Client"); got != hereNowClientHeader {
			t.Fatalf("X-HereNow-Client = %q", got)
		}
		switch requests {
		case 1:
			if got := req.Header.Get("X-HereNow-Account"); got != "workspace-default" {
				t.Fatalf("account header = %q", got)
			}
			if req.URL.RawQuery != "" {
				t.Fatalf("account Site listing must not send scope=all pagination: %s", req.URL.RawQuery)
			}
			return hereNowTestResponse(http.StatusOK, `{"sites":[]}`), nil
		case 2:
			if got := req.Header.Get("X-HereNow-Account"); got != "" {
				t.Fatalf("scope=all leaked default account header %q", got)
			}
			query := req.URL.Query()
			if query.Get("scope") != "all" || query.Get("cursor") != "next-1" || query.Get("limit") != "50" {
				t.Fatalf("unexpected scope=all query: %s", req.URL.RawQuery)
			}
			return hereNowTestResponse(http.StatusOK, `{"sites":[],"nextCursor":null}`), nil
		default:
			if got := req.Header.Get("X-HereNow-Account"); got != "" {
				t.Fatalf("accounts request leaked default selector %q", got)
			}
			return hereNowTestResponse(http.StatusBadRequest, `{"error":"bad","code":"invalid_request","message":"invalid selector","details":{"field":"account"},"docs_url":"https://here.now/docs","retry_after":7}`), nil
		}
	})

	if _, err := client.ListSites(context.Background(), "", "ignored", 99, false); err != nil {
		t.Fatalf("ListSites: %v", err)
	}
	if _, err := client.ListSites(context.Background(), "ignored", "next-1", 50, true); err != nil {
		t.Fatalf("ListSites all: %v", err)
	}
	_, err := client.ListAccounts(context.Background())
	var apiErr *HereNowAPIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("ListAccounts error = %T %v", err, err)
	}
	if apiErr.StatusCode != http.StatusBadRequest || apiErr.ProviderError != "bad" || apiErr.Code != "invalid_request" || apiErr.Message != "invalid selector" || string(apiErr.Details) != `{"field":"account"}` || apiErr.RetryAfter != "7" || apiErr.DocsURL != "https://here.now/docs" {
		t.Fatalf("structured error = %+v", apiErr)
	}
	if requests != 3 {
		t.Fatalf("non-retryable 4xx requests = %d, want 3 total", requests)
	}
}

func TestHereNowRejectsInactiveOrUnsupportedAccounts(t *testing.T) {
	client := newHereNowTestClient(t, "suspended-team", func(req *http.Request) (*http.Response, error) {
		return hereNowTestResponse(http.StatusOK, `{"accounts":[{"accountId":"suspended-team","type":"org","status":"suspended","selector":{"id":"suspended-team"}},{"accountId":"failed-team","type":"org","status":"active","provisioningStatus":"failed","selector":{"id":"failed-team"}},{"accountId":"device","type":"service","status":"active","selector":{"id":"device"}}]}`), nil
	})
	if _, err := client.resolveAccountSelection(context.Background(), "suspended-team"); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("inactive account selection error = %v", err)
	}
	if _, err := client.resolveAccountSelection(context.Background(), "device"); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("unsupported account selection error = %v", err)
	}
	if _, err := client.resolveAccountSelection(context.Background(), "failed-team"); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("unprovisioned account selection error = %v", err)
	}
}

func TestHereNowRetriesRateLimitsAndNetworkErrors(t *testing.T) {
	t.Run("rate_limit", func(t *testing.T) {
		var calls int
		var delays []time.Duration
		client := newHereNowTestClient(t, "", func(req *http.Request) (*http.Response, error) {
			calls++
			if calls < 3 {
				return hereNowTestResponse(http.StatusTooManyRequests, `{"error":"slow","code":"rate_limit_exceeded","message":"slow down"}`, map[string]string{"Retry-After": "2"}), nil
			}
			return hereNowTestResponse(http.StatusOK, `{"accounts":[],"currentAccountId":null}`), nil
		})
		client.sleep = func(_ context.Context, delay time.Duration) error {
			delays = append(delays, delay)
			return nil
		}
		if _, err := client.ListAccounts(context.Background()); err != nil {
			t.Fatalf("ListAccounts: %v", err)
		}
		if calls != 3 || len(delays) != 2 || delays[0] != 2*time.Second || delays[1] != 2*time.Second {
			t.Fatalf("calls=%d delays=%v", calls, delays)
		}
	})

	t.Run("network", func(t *testing.T) {
		var calls int
		client := newHereNowTestClient(t, "", func(req *http.Request) (*http.Response, error) {
			calls++
			if calls < 3 {
				return nil, errors.New("temporary network failure")
			}
			return hereNowTestResponse(http.StatusOK, `{"accounts":[],"currentAccountId":null}`), nil
		})
		if _, err := client.ListAccounts(context.Background()); err != nil {
			t.Fatalf("ListAccounts: %v", err)
		}
		if calls != 3 {
			t.Fatalf("network attempts = %d, want 3", calls)
		}
	})

	t.Run("network retry limit", func(t *testing.T) {
		var calls int
		client := newHereNowTestClient(t, "", func(req *http.Request) (*http.Response, error) {
			calls++
			return nil, errors.New("temporary network failure")
		})
		if _, err := client.ListAccounts(context.Background()); err == nil {
			t.Fatal("exhausted network retries unexpectedly succeeded")
		}
		if calls != 4 {
			t.Fatalf("network attempts = %d, want initial request plus three retries", calls)
		}
	})
}

func TestHereNowNonIdempotentMutationDoesNotRetryUnknownOutcome(t *testing.T) {
	tests := []struct {
		name      string
		response  *http.Response
		transport error
	}{
		{name: "network", transport: errors.New("request failed with token=must-not-leak")},
		{name: "rate limit", response: hereNowTestResponse(http.StatusTooManyRequests, `{"error":"slow","code":"rate_limit_exceeded","message":"slow down"}`, map[string]string{"Retry-After": "3"})},
		{name: "server error", response: hereNowTestResponse(http.StatusServiceUnavailable, `{"error":"unavailable","code":"temporarily_unavailable","message":"try later"}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls int
			client := newHereNowTestClient(t, "", func(req *http.Request) (*http.Response, error) {
				calls++
				return tt.response, tt.transport
			})
			_, err := client.PatchMetadata(context.Background(), "", "site", map[string]interface{}{"displayName": "Name"})
			var outcomeErr *HereNowOutcomeUnknownError
			if !errors.As(err, &outcomeErr) || calls != 1 || outcomeErr.RetrySafe {
				t.Fatalf("calls=%d error=%T %v", calls, err, err)
			}
			encoded, marshalErr := json.Marshal(HereNowErrorPayload(err))
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			text := string(encoded)
			for _, marker := range []string{`"code":"here_now_outcome_unknown"`, `"outcome":"unknown"`, `"retry_safe":false`, `"operation":"update Site metadata"`} {
				if !strings.Contains(text, marker) {
					t.Fatalf("structured error %s missing %s", text, marker)
				}
			}
			if strings.Contains(text, "must-not-leak") {
				t.Fatalf("transport detail leaked: %s", text)
			}
		})
	}
}

func TestBuildHereNowManifestHashesSkipsAndRejectsSensitiveSources(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "node_modules", "pkg"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules", "pkg", "index.js"), []byte("skip"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, homepageArtifactManifestName), []byte(`{"generatedAt":"changes-every-run"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := buildHereNowSnapshot(root, ".")
	if err != nil {
		t.Fatalf("buildHereNowSnapshot: %v", err)
	}
	defer snapshot.Close()
	manifest := snapshot.files
	if len(manifest) != 1 || manifest[0].Path != "index.html" || manifest[0].Size != 5 || manifest[0].ContentType != "text/html; charset=utf-8" {
		t.Fatalf("manifest = %+v", manifest)
	}
	wantHash := sha256.Sum256([]byte("hello"))
	if manifest[0].Hash != hex.EncodeToString(wantHash[:]) {
		t.Fatalf("hash = %q", manifest[0].Hash)
	}

	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("TOKEN=value"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := buildHereNowSnapshot(root, "."); err == nil || !strings.Contains(err.Error(), "sensitive file") {
		t.Fatalf(".env error = %v", err)
	}
}

func TestBuildHereNowSnapshotRejectsExpandedCredentialSources(t *testing.T) {
	tests := []struct {
		name    string
		relPath string
		content string
	}{
		{name: "envrc", relPath: ".envrc", content: "export TOKEN=value"},
		{name: "aws credentials", relPath: ".aws/credentials", content: "secret"},
		{name: "deploy key", relPath: "ops/deploy_key_prod", content: "secret"},
		{name: "vault json", relPath: "ops/vault.json", content: "{}"},
		{name: "java keystore", relPath: "certs/app.jks", content: "binary"},
		{name: "java keystore with unusual name", relPath: "public/store.bin", content: "\xfe\xed\xfe\xedbinary"},
		{name: "private key content", relPath: "public/readme.txt", content: "prefix\n-----BEGIN OPENSSH PRIVATE KEY-----\nsecret"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			full := filepath.Join(root, filepath.FromSlash(tt.relPath))
			if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, []byte(tt.content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := buildHereNowSnapshot(root, "."); err == nil || !strings.Contains(err.Error(), "sensitive") && !strings.Contains(err.Error(), "private key") && !strings.Contains(strings.ToLower(err.Error()), "keystore") {
				t.Fatalf("credential source error = %v", err)
			}
		})
	}
}

func TestBuildHereNowSnapshotSkipsGitWorktreePointerFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("site"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: C:/outside/worktree"), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := buildHereNowSnapshot(root, ".")
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	if len(snapshot.files) != 1 || snapshot.files[0].Path != "index.html" {
		t.Fatalf("Git worktree pointer was published: %+v", snapshot.files)
	}
}

func TestBuildHereNowSnapshotKeepsStableBytesAndRejectsIntermediateSymlink(t *testing.T) {
	t.Run("stable private snapshot", func(t *testing.T) {
		workspace := t.TempDir()
		project := filepath.Join(workspace, "project", "dist")
		if err := os.MkdirAll(project, 0o700); err != nil {
			t.Fatal(err)
		}
		source := filepath.Join(project, "index.html")
		if err := os.WriteFile(source, []byte("original"), 0o600); err != nil {
			t.Fatal(err)
		}
		snapshot, err := buildHereNowSnapshot(workspace, "project/dist")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(source, []byte("replacement"), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(snapshot.files[0].SourcePath)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "original" {
			t.Fatalf("snapshot bytes = %q", got)
		}
		snapshotPath := snapshot.root
		if err := snapshot.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(snapshotPath); !os.IsNotExist(err) {
			t.Fatalf("snapshot directory still exists: %v", err)
		}
	})

	t.Run("intermediate symlink", func(t *testing.T) {
		workspace := t.TempDir()
		realProject := filepath.Join(workspace, "real", "dist")
		if err := os.MkdirAll(realProject, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(realProject, "index.html"), []byte("ok"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(workspace, "real"), filepath.Join(workspace, "project")); err != nil {
			t.Skipf("symlink unavailable on this platform: %v", err)
		}
		if _, err := buildHereNowSnapshot(workspace, "project/dist"); err == nil || !strings.Contains(err.Error(), "symbolic links") {
			t.Fatalf("intermediate symlink error = %v", err)
		}
	})
}

func TestBuildHereNowManifestRejectsSymlinksAndTooManyFiles(t *testing.T) {
	t.Run("symlink", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(root, "target.txt")
		if err := os.WriteFile(target, []byte("target"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(root, "link.txt")); err != nil {
			t.Skipf("symlink unavailable on this platform: %v", err)
		}
		if _, err := buildHereNowSnapshot(root, "."); err == nil || !strings.Contains(err.Error(), "symbolic links") {
			t.Fatalf("symlink error = %v", err)
		}
	})

	t.Run("file_limit", func(t *testing.T) {
		root := t.TempDir()
		for i := 0; i <= hereNowMaxFiles; i++ {
			name := filepath.Join(root, "file-"+strconv.Itoa(i)+".txt")
			if err := os.WriteFile(name, nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := buildHereNowSnapshot(root, "."); err == nil || !strings.Contains(err.Error(), "at most 1000 files") {
			t.Fatalf("file limit error = %v", err)
		}
	})
}

func TestHereNowPublishDirectoryContractAndIdempotentFinalize(t *testing.T) {
	root := t.TempDir()
	content := []byte("<!doctype html><title>AuraGo</title>")
	if err := os.WriteFile(filepath.Join(root, "index.html"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	accountID := "11111111-1111-1111-1111-111111111111"
	var publishBody map[string]interface{}
	var finalizeCalls int
	var uploaded []byte
	transport := hereNowRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "here.now" && req.Header.Get("Authorization") != "" {
			t.Fatalf("API key leaked to %s", req.URL.Host)
		}
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/api/v1/accounts":
			if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("replacement after snapshot"), 0o600); err != nil {
				t.Fatal(err)
			}
			return hereNowTestResponse(http.StatusOK, `{"accounts":[{"accountId":"`+accountID+`","type":"personal","displayName":"Personal","status":"active","provisioningStatus":"active","role":"admin","selector":{"id":"`+accountID+`","subdomain":null}}],"currentAccountId":"`+accountID+`"}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/api/v1/publish":
			if req.Header.Get("Authorization") != "Bearer test-api-key" || req.Header.Get("X-HereNow-Client") != hereNowClientHeader || req.Header.Get("X-HereNow-Account") != "" {
				t.Fatalf("publish headers = %#v", req.Header)
			}
			if err := json.NewDecoder(req.Body).Decode(&publishBody); err != nil {
				t.Fatal(err)
			}
			return hereNowTestResponse(http.StatusOK, `{"slug":"site-1","siteUrl":"https://site-1.here.now","status":"pending","isLive":false,"requiresFinalize":true,"anonymous":false,"publishStatus":{"requestAuth":"api_key","ownership":"personal","accountId":"`+accountID+`","persistence":"permanent","expiresAt":null,"state":"pending"},"upload":{"versionId":"version-1","uploads":[{"path":"index.html","method":"PUT","url":"https://uploads.example.test/object?signature=secret","headers":{"Content-Type":"text/html; charset=utf-8"}}],"skipped":[],"finalizeUrl":"https://here.now/api/v1/publish/site-1/finalize","expiresInSeconds":900}}`), nil
		case req.Method == http.MethodPut && req.URL.Host == "uploads.example.test":
			if req.Header.Get("Authorization") != "" || req.Header.Get("X-HereNow-Client") != "" {
				t.Fatalf("upload received AuraGo API headers: %#v", req.Header)
			}
			uploaded, _ = io.ReadAll(req.Body)
			return hereNowTestResponse(http.StatusOK, ""), nil
		case req.Method == http.MethodPost && req.URL.Path == "/api/v1/publish/site-1/finalize":
			finalizeCalls++
			if finalizeCalls == 1 {
				return nil, errors.New("connection closed after finalize")
			}
			var body map[string]interface{}
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["versionId"] != "version-1" {
				t.Fatalf("finalize body = %#v", body)
			}
			if _, exists := body["account"]; exists {
				t.Fatalf("personal finalize unexpectedly sent account: %#v", body)
			}
			return hereNowTestResponse(http.StatusOK, `{"success":true,"slug":"site-1","siteUrl":"https://site-1.here.now","currentVersionId":"version-live","previousVersionId":null,"unchanged":true,"replayed":true,"publishStatus":{"requestAuth":"api_key","ownership":"personal","accountId":"`+accountID+`","persistence":"permanent","expiresAt":null,"state":"live"}}`), nil
		case req.Method == http.MethodGet && req.URL.Host == "site-1.here.now":
			if req.Header.Get("Range") != "bytes=0-0" || req.Header.Get("Authorization") != "" {
				t.Fatalf("verification headers = %#v", req.Header)
			}
			return hereNowTestResponse(http.StatusPartialContent, "<"), nil
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	client := newHereNowTestClient(t, "", transport)
	result, err := client.PublishWorkspaceDirectory(context.Background(), root, ".", HereNowPublishOptions{})
	if err != nil {
		t.Fatalf("PublishDirectory: %v", err)
	}
	if !result.Verified || result.Slug != "site-1" || result.CurrentVersionID != "version-live" || result.Account != accountID || !result.Unchanged || !result.Replayed || result.VerifiedURL != "https://site-1.here.now" {
		t.Fatalf("result = %+v", result)
	}
	if finalizeCalls != 2 {
		t.Fatalf("finalize calls = %d, want idempotent retry", finalizeCalls)
	}
	if string(uploaded) != string(content) {
		t.Fatalf("uploaded content = %q", uploaded)
	}
	if ttl, exists := publishBody["ttlSeconds"]; !exists || ttl != nil {
		t.Fatalf("ttlSeconds must be explicit null: %#v", publishBody)
	}
	files, ok := publishBody["files"].([]interface{})
	if !ok || len(files) != 1 {
		t.Fatalf("files = %#v", publishBody["files"])
	}
	file := files[0].(map[string]interface{})
	wantHash := sha256.Sum256(content)
	if file["path"] != "index.html" || file["hash"] != hex.EncodeToString(wantHash[:]) || int64(file["size"].(float64)) != int64(len(content)) {
		t.Fatalf("manifest file = %#v", file)
	}
}

func TestHereNowPublishStartUsesPUTForExactUpdateSlug(t *testing.T) {
	accountID := "11111111-1111-1111-1111-111111111111"
	client := newHereNowTestClient(t, "", func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPut || req.URL.Path != "/api/v1/publish/existing-site" {
			t.Fatalf("update request = %s %s", req.Method, req.URL.Path)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if ttl, present := body["ttlSeconds"]; !present || ttl != nil {
			t.Fatalf("update must explicitly clear TTL: %#v", body)
		}
		if _, present := body["claimToken"]; present {
			t.Fatalf("authenticated update included claim token: %#v", body)
		}
		return hereNowTestResponse(http.StatusOK, `{"slug":"existing-site","siteUrl":"https://existing-site.here.now","requiresFinalize":true,"anonymous":false,"publishStatus":{"requestAuth":"api_key","ownership":"personal","accountId":"`+accountID+`","persistence":"permanent","expiresAt":null,"state":"pending"},"upload":{"versionId":"version-update","uploads":[],"skipped":["index.html"],"finalizeUrl":"https://here.now/api/v1/publish/existing-site/finalize","expiresInSeconds":900}}`), nil
	})
	started, err := client.publishStart(context.Background(), []hereNowPublishFile{{Path: "index.html", Size: 1, Hash: strings.Repeat("a", 64)}}, HereNowPublishOptions{Slug: "existing-site"}, hereNowAccountSelection{AccountID: accountID, Ownership: "personal"})
	if err != nil {
		t.Fatalf("publishStart update: %v", err)
	}
	if started.Slug != "existing-site" || started.Upload.VersionID != "version-update" {
		t.Fatalf("update response = %+v", started)
	}
}

func TestHereNowRefreshesExpiredUploadURLOnceForWorkspace(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("workspace"), 0o600); err != nil {
		t.Fatal(err)
	}
	personalID := "11111111-1111-1111-1111-111111111111"
	workspaceID := "22222222-2222-2222-2222-222222222222"
	var oldUploads, newUploads, refreshes int
	pending := func(uploadURL string) string {
		return `{"slug":"workspace-site","siteUrl":"https://workspace-site.team.here.now","status":"pending","isLive":false,"requiresFinalize":true,"anonymous":false,"publishStatus":{"requestAuth":"api_key","ownership":"workspace","accountId":"` + workspaceID + `","persistence":"permanent","expiresAt":null,"state":"pending"},"upload":{"versionId":"version-workspace","uploads":[{"path":"index.html","method":"PUT","url":"` + uploadURL + `","headers":{}}],"skipped":[],"finalizeUrl":"https://here.now/api/v1/publish/workspace-site/finalize","expiresInSeconds":900}}`
	}
	client := newHereNowTestClient(t, "team", func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/api/v1/accounts":
			return hereNowTestResponse(http.StatusOK, `{"accounts":[{"accountId":"`+personalID+`","type":"personal","displayName":"Personal","status":"active","provisioningStatus":"active","role":"admin"},{"accountId":"`+workspaceID+`","type":"org","subdomain":"team","displayName":"Team","status":"active","provisioningStatus":"active","role":"admin","selector":{"id":"`+workspaceID+`","subdomain":"team"}}],"currentAccountId":"`+personalID+`"}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/api/v1/publish":
			if req.Header.Get("X-HereNow-Account") != workspaceID {
				t.Fatalf("workspace header = %q", req.Header.Get("X-HereNow-Account"))
			}
			var body map[string]interface{}
			_ = json.NewDecoder(req.Body).Decode(&body)
			if body["account"] != workspaceID || body["ttlSeconds"] != nil {
				t.Fatalf("workspace publish body = %#v", body)
			}
			return hereNowTestResponse(http.StatusOK, pending("https://upload.example.test/expired")), nil
		case req.Method == http.MethodPut && req.URL.Path == "/expired":
			oldUploads++
			return hereNowTestResponse(http.StatusForbidden, "expired"), nil
		case req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/uploads/refresh"):
			refreshes++
			return hereNowTestResponse(http.StatusOK, pending("https://upload.example.test/fresh")), nil
		case req.Method == http.MethodPut && req.URL.Path == "/fresh":
			newUploads++
			return hereNowTestResponse(http.StatusOK, ""), nil
		case req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/finalize"):
			return hereNowTestResponse(http.StatusOK, `{"success":true,"slug":"workspace-site","siteUrl":"https://workspace-site.team.here.now","currentVersionId":"version-workspace","publishStatus":{"requestAuth":"api_key","ownership":"workspace","accountId":"`+workspaceID+`","persistence":"permanent","expiresAt":null,"state":"live"}}`), nil
		case req.Method == http.MethodGet && req.URL.Host == "workspace-site.team.here.now":
			return hereNowTestResponse(http.StatusOK, "ok"), nil
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	result, err := client.PublishWorkspaceDirectory(context.Background(), root, ".", HereNowPublishOptions{})
	if err != nil {
		t.Fatalf("PublishDirectory: %v", err)
	}
	if oldUploads != 1 || refreshes != 1 || newUploads != 1 || result.Account != workspaceID {
		t.Fatalf("old=%d refresh=%d new=%d result=%+v", oldUploads, refreshes, newUploads, result)
	}
}

func TestHereNowRejectsAnonymousPublishAndUnsafeUploadTargets(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "index.html")
	if err := os.WriteFile(filePath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	accountID := "11111111-1111-1111-1111-111111111111"
	client := newHereNowTestClient(t, "", func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/api/v1/accounts" {
			return hereNowTestResponse(http.StatusOK, `{"accounts":[{"accountId":"`+accountID+`","type":"personal"}],"currentAccountId":"`+accountID+`"}`), nil
		}
		return hereNowTestResponse(http.StatusOK, `{"slug":"anon","siteUrl":"https://anon.here.now","status":"pending","isLive":false,"requiresFinalize":true,"anonymous":true,"publishStatus":{"requestAuth":"none","ownership":"anonymous","accountId":null,"persistence":"expiring","expiresAt":"2026-01-01T00:00:00Z","state":"pending"},"upload":{"versionId":"v","uploads":[],"finalizeUrl":"https://here.now/api/v1/publish/anon/finalize","expiresInSeconds":900}}`), nil
	})
	if _, err := client.PublishWorkspaceDirectory(context.Background(), root, ".", HereNowPublishOptions{}); err == nil || !strings.Contains(err.Error(), "anonymous publishing is disabled") {
		t.Fatalf("anonymous publish error = %v", err)
	}

	for _, raw := range []string{"http://upload.example/file", "https://user:pass@upload.example/file", "https://upload.example/file#token"} {
		if err := validateHereNowUploadURL(raw); err == nil {
			t.Fatalf("unsafe upload URL accepted: %s", raw)
		}
	}
	privateClient, err := newHereNowClient("https://here.now", "test-api-key", "", &http.Client{Transport: hereNowRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("API transport must not be used")
	})})
	if err != nil {
		t.Fatal(err)
	}
	err = privateClient.uploadOne(context.Background(), hereNowUploadTarget{Path: "index.html", Method: http.MethodPut, URL: "https://127.0.0.1/upload?signature=must-not-leak"}, hereNowPublishFile{Path: "index.html", SourcePath: filePath, Size: 1, ContentType: "text/html"})
	if err == nil {
		t.Fatal("private upload target was accepted")
	}
	if strings.Contains(err.Error(), "must-not-leak") {
		t.Fatalf("presigned URL token leaked through validation error: %v", err)
	}

	var redirectCalls int
	redirectClient := newHereNowTestClient(t, "", func(req *http.Request) (*http.Response, error) {
		redirectCalls++
		return hereNowTestResponse(http.StatusFound, "", map[string]string{"Location": "https://127.0.0.1/private"}), nil
	})
	err = redirectClient.uploadOne(context.Background(), hereNowUploadTarget{Path: "index.html", Method: http.MethodPut, URL: "https://uploads.example.test/object"}, hereNowPublishFile{Path: "index.html", SourcePath: filePath, Size: 1, ContentType: "text/html"})
	if err == nil || redirectCalls != 1 {
		t.Fatalf("upload redirect was followed or accepted: calls=%d err=%v", redirectCalls, err)
	}

	networkClient := newHereNowTestClient(t, "", func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("Put " + req.URL.String() + ": connection failed")
	})
	err = networkClient.uploadOne(context.Background(), hereNowUploadTarget{Path: "index.html", Method: http.MethodPut, URL: "https://uploads.example.test/object?signature=must-not-leak"}, hereNowPublishFile{Path: "index.html", SourcePath: filePath, Size: 1, ContentType: "text/html"})
	if err == nil || strings.Contains(err.Error(), "must-not-leak") {
		t.Fatalf("presigned URL token leaked through network error: %v", err)
	}
}

func TestHereNowUploadConcurrencyIsBounded(t *testing.T) {
	root := t.TempDir()
	files := make([]hereNowPublishFile, 0, 8)
	targets := make([]hereNowUploadTarget, 0, 8)
	for i := 0; i < 8; i++ {
		path := filepath.Join(root, "file-"+strconv.Itoa(i)+".txt")
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		rel := filepath.Base(path)
		files = append(files, hereNowPublishFile{Path: rel, SourcePath: path, Size: 1, ContentType: "text/plain"})
		targets = append(targets, hereNowUploadTarget{Path: rel, Method: http.MethodPut, URL: "https://upload.example.test/" + rel})
	}
	var active atomic.Int32
	var maximum atomic.Int32
	client := newHereNowTestClient(t, "", func(req *http.Request) (*http.Response, error) {
		current := active.Add(1)
		for {
			seen := maximum.Load()
			if current <= seen || maximum.CompareAndSwap(seen, current) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		active.Add(-1)
		return hereNowTestResponse(http.StatusOK, ""), nil
	})
	if err := client.uploadAll(context.Background(), targets, files); err != nil {
		t.Fatalf("uploadAll: %v", err)
	}
	if got := maximum.Load(); got < 2 || got > hereNowUploadWorkers {
		t.Fatalf("maximum concurrent uploads = %d, want 2..%d", got, hereNowUploadWorkers)
	}
}

func TestHereNowSiteURLVerificationAcceptsProtectedSitesButRejectsUnsafeRedirects(t *testing.T) {
	t.Run("protected", func(t *testing.T) {
		client := newHereNowTestClient(t, "", func(req *http.Request) (*http.Response, error) {
			return hereNowTestResponse(http.StatusForbidden, ""), nil
		})
		if ok, err := client.VerifySiteURL(context.Background(), "https://site.here.now"); !ok || err != nil {
			t.Fatalf("VerifySiteURL = %v, %v", ok, err)
		}
	})

	t.Run("safe redirect reaches final response", func(t *testing.T) {
		var calls int
		client := newHereNowTestClient(t, "", func(req *http.Request) (*http.Response, error) {
			calls++
			if req.URL.Host == "site.here.now" {
				return hereNowTestResponse(http.StatusFound, "", map[string]string{"Location": "https://auth.here.now/login?next=site"}), nil
			}
			if req.URL.Host == "auth.here.now" {
				return hereNowTestResponse(http.StatusOK, "login"), nil
			}
			t.Fatalf("unexpected verification host %s", req.URL.Host)
			return nil, nil
		})
		if ok, err := client.VerifySiteURL(context.Background(), "https://site.here.now"); !ok || err != nil || calls != 2 {
			t.Fatalf("VerifySiteURL = %v, %v calls=%d", ok, err, calls)
		}
	})

	for _, tt := range []struct {
		name     string
		status   int
		location string
	}{
		{name: "unsafe redirect", status: http.StatusFound, location: "https://127.0.0.1/private"},
		{name: "redirect final 404", status: http.StatusFound, location: "https://auth.here.now/missing"},
		{name: "rate limited", status: http.StatusTooManyRequests},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := newHereNowTestClient(t, "", func(req *http.Request) (*http.Response, error) {
				if req.URL.Path == "/missing" {
					return hereNowTestResponse(http.StatusNotFound, ""), nil
				}
				headers := map[string]string{}
				if tt.location != "" {
					headers["Location"] = tt.location
				}
				return hereNowTestResponse(tt.status, "", headers), nil
			})
			if ok, err := client.VerifySiteURL(context.Background(), "https://site.here.now"); ok || err == nil {
				t.Fatalf("unsafe verification accepted: %v, %v", ok, err)
			}
		})
	}

	t.Run("redirect loop", func(t *testing.T) {
		client := newHereNowTestClient(t, "", func(req *http.Request) (*http.Response, error) {
			return hereNowTestResponse(http.StatusFound, "", map[string]string{"Location": "https://site.here.now"}), nil
		})
		if ok, err := client.VerifySiteURL(context.Background(), "https://site.here.now"); ok || err == nil || !strings.Contains(err.Error(), "loop") {
			t.Fatalf("redirect loop result = %v, %v", ok, err)
		}
	})
}

func TestHereNowUploadRejectsCredentialHeaders(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "index.html")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	var calls int
	client := newHereNowTestClient(t, "", func(req *http.Request) (*http.Response, error) {
		calls++
		return hereNowTestResponse(http.StatusOK, ""), nil
	})
	err := client.uploadOne(context.Background(), hereNowUploadTarget{
		Path: "index.html", Method: http.MethodPut, URL: "https://uploads.example.test/object",
		Headers: map[string]string{"Authorization": "Bearer provider-supplied-value"},
	}, hereNowPublishFile{Path: "index.html", SourcePath: path, Size: 1, ContentType: "text/html"})
	if err == nil || calls != 0 {
		t.Fatalf("credential-bearing upload header accepted: calls=%d err=%v", calls, err)
	}
}

func TestHereNowUpdateAccessPreservesCompleteAllowlistAndNeverSendsInvites(t *testing.T) {
	personalID := "11111111-1111-1111-1111-111111111111"
	workspaceID := "22222222-2222-2222-2222-222222222222"
	var patchCalls int
	client := newHereNowTestClient(t, "team", func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/api/v1/accounts" {
			return hereNowTestResponse(http.StatusOK, `{"accounts":[{"accountId":"`+personalID+`","type":"personal"},{"accountId":"`+workspaceID+`","type":"org","subdomain":"team","selector":{"id":"`+workspaceID+`","subdomain":"team"}}],"currentAccountId":"`+personalID+`"}`), nil
		}
		if req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/access") {
			return hereNowTestResponse(http.StatusOK, `{"access":{"mode":"restricted","accessPolicyVersion":1,"allowedEmails":["keep@example.com"],"allowedDomains":["keep.example"]}}`), nil
		}
		patchCalls++
		if req.Header.Get("X-HereNow-Account") != "" {
			t.Fatalf("personal account request must omit workspace header, got %q", req.Header.Get("X-HereNow-Account"))
		}
		var body map[string]interface{}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["mode"] != "restricted" || body["notify"] != false {
			t.Fatalf("access patch = %#v", body)
		}
		if emails, ok := body["allowedEmails"].([]interface{}); !ok || len(emails) != 1 || emails[0] != "keep@example.com" {
			t.Fatalf("email allowlist was not preserved: %#v", body)
		}
		if domains, ok := body["allowedDomains"].([]interface{}); !ok || len(domains) != 0 {
			t.Fatalf("domain allowlist was not explicitly cleared: %#v", body)
		}
		return hereNowTestResponse(http.StatusOK, `{"access":{"mode":"restricted","accessPolicyVersion":2,"allowedEmails":["keep@example.com"],"allowedDomains":[]}}`), nil
	})
	if _, err := client.UpdateAccess(context.Background(), "", "site", "restricted", nil, &[]string{}); err == nil || !strings.Contains(err.Error(), "personal Sites") {
		t.Fatalf("workspace restricted error = %v", err)
	}
	client.defaultAccount = ""
	if _, err := client.UpdateAccess(context.Background(), "", "site", "restricted", nil, &[]string{}); err != nil {
		t.Fatalf("personal restricted access: %v", err)
	}
	if patchCalls != 1 {
		t.Fatalf("patch calls = %d, want 1", patchCalls)
	}
}

func TestHereNowUpdateAccessRejectsIncompletePolicyBeforePatch(t *testing.T) {
	accountID := "11111111-1111-1111-1111-111111111111"
	for _, malformed := range []string{
		`{}`,
		`{"access":{"mode":"restricted","allowedEmails":[],"allowedDomains":[]}}`,
		`{"access":{"mode":"restricted","accessPolicyVersion":1}}`,
		`{"access":{"mode":"unknown","accessPolicyVersion":1,"allowedEmails":[],"allowedDomains":[]}}`,
	} {
		t.Run(malformed, func(t *testing.T) {
			var patchCalls int
			client := newHereNowTestClient(t, "", func(req *http.Request) (*http.Response, error) {
				switch {
				case req.URL.Path == "/api/v1/accounts":
					return hereNowTestResponse(http.StatusOK, `{"accounts":[{"accountId":"`+accountID+`","type":"personal"}]}`), nil
				case req.Method == http.MethodGet:
					return hereNowTestResponse(http.StatusOK, malformed), nil
				default:
					patchCalls++
					return hereNowTestResponse(http.StatusOK, `{}`), nil
				}
			})
			if _, err := client.UpdateAccess(context.Background(), "", "site", "restricted", nil, nil); err == nil {
				t.Fatal("incomplete access policy was accepted")
			}
			if patchCalls != 0 {
				t.Fatalf("malformed access response caused %d PATCH calls", patchCalls)
			}
		})
	}
}

func TestHereNowUpdateAccessAllowsWorkspaceMembersMode(t *testing.T) {
	workspaceID := "22222222-2222-2222-2222-222222222222"
	var patchCalls int
	client := newHereNowTestClient(t, "team", func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/api/v1/accounts":
			return hereNowTestResponse(http.StatusOK, `{"accounts":[{"accountId":"`+workspaceID+`","type":"org","status":"active","subdomain":"team","selector":{"id":"`+workspaceID+`","subdomain":"team"}}]}`), nil
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/access"):
			return hereNowTestResponse(http.StatusOK, `{"access":{"mode":"anyone_with_link","accessPolicyVersion":1,"allowedEmails":[],"allowedDomains":[]}}`), nil
		case req.Method == http.MethodPatch && strings.HasSuffix(req.URL.Path, "/access"):
			patchCalls++
			if req.Header.Get("X-HereNow-Account") != workspaceID {
				t.Fatalf("workspace account header = %q", req.Header.Get("X-HereNow-Account"))
			}
			var body map[string]interface{}
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["mode"] != "account_members" || body["notify"] != false {
				t.Fatalf("workspace access patch = %#v", body)
			}
			return hereNowTestResponse(http.StatusOK, `{"access":{"mode":"account_members","accessPolicyVersion":2,"allowedEmails":[],"allowedDomains":[]}}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
			return nil, nil
		}
	})
	if _, err := client.UpdateAccess(context.Background(), "", "site", "account_members", nil, nil); err != nil {
		t.Fatalf("workspace account_members: %v", err)
	}
	if patchCalls != 1 {
		t.Fatalf("workspace patch calls = %d", patchCalls)
	}
}

func TestHereNowDuplicateVerifiesPermanentLiveCopy(t *testing.T) {
	accountID := "11111111-1111-1111-1111-111111111111"
	client := newHereNowTestClient(t, "", func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/api/v1/accounts":
			return hereNowTestResponse(http.StatusOK, `{"accounts":[{"accountId":"`+accountID+`","type":"personal"}],"currentAccountId":"`+accountID+`"}`), nil
		case req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/duplicate"):
			return hereNowTestResponse(http.StatusOK, `{"slug":"copy-site","siteUrl":"https://copy-site.here.now","sourceSlug":"source","status":"active","currentVersionId":"version-copy","filesCount":2}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/api/v1/publish/copy-site":
			return hereNowTestResponse(http.StatusOK, `{"slug":"copy-site","siteUrl":"https://copy-site.here.now","status":"active","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z","expiresAt":null,"currentVersionId":"version-copy","pendingVersionId":null,"manifest":[]}`), nil
		case req.Method == http.MethodGet && req.URL.Host == "copy-site.here.now":
			return hereNowTestResponse(http.StatusOK, "ok"), nil
		default:
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	raw, err := client.DuplicateSite(context.Background(), "", "source", nil)
	if err != nil {
		t.Fatalf("DuplicateSite: %v", err)
	}
	var output map[string]interface{}
	if err := json.Unmarshal(raw, &output); err != nil {
		t.Fatal(err)
	}
	if output["verified"] != true || output["account"] != accountID || output["verifiedUrl"] != "https://copy-site.here.now" {
		t.Fatalf("duplicate output = %#v", output)
	}
}

func TestHereNowPasswordTransitionsRequireAndVerifyExplicitModes(t *testing.T) {
	t.Run("set password", func(t *testing.T) {
		var accessReads, patches int
		client := newHereNowTestClient(t, "", func(req *http.Request) (*http.Response, error) {
			switch {
			case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/access"):
				accessReads++
				mode := "anyone_with_link"
				if accessReads == 2 {
					mode = "password"
				}
				return hereNowTestResponse(http.StatusOK, `{"access":{"mode":"`+mode+`","accessPolicyVersion":`+strconv.Itoa(accessReads)+`,"allowedEmails":[],"allowedDomains":[]}}`), nil
			case req.Method == http.MethodPatch && strings.HasSuffix(req.URL.Path, "/metadata"):
				patches++
				var body map[string]interface{}
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body["password"] != "vault-only-password" {
					t.Fatalf("password patch = %#v", body)
				}
				return hereNowTestResponse(http.StatusOK, `{"success":true}`), nil
			default:
				t.Fatalf("unexpected request %s %s", req.Method, req.URL)
				return nil, nil
			}
		})
		if _, err := client.SetPassword(context.Background(), "", "site", "vault-only-password"); err != nil {
			t.Fatal(err)
		}
		if accessReads != 2 || patches != 1 {
			t.Fatalf("access reads=%d patches=%d", accessReads, patches)
		}
	})

	t.Run("remove password", func(t *testing.T) {
		var accessReads, patches int
		client := newHereNowTestClient(t, "", func(req *http.Request) (*http.Response, error) {
			switch {
			case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/access"):
				accessReads++
				mode := "password"
				if accessReads == 2 {
					mode = "anyone_with_link"
				}
				return hereNowTestResponse(http.StatusOK, `{"access":{"mode":"`+mode+`","accessPolicyVersion":`+strconv.Itoa(accessReads)+`,"allowedEmails":[],"allowedDomains":[]}}`), nil
			case req.Method == http.MethodPatch && strings.HasSuffix(req.URL.Path, "/metadata"):
				patches++
				var body map[string]interface{}
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				password, exists := body["password"]
				if !exists || password != nil {
					t.Fatalf("password removal patch = %#v", body)
				}
				return hereNowTestResponse(http.StatusOK, `{"success":true}`), nil
			default:
				t.Fatalf("unexpected request %s %s", req.Method, req.URL)
				return nil, nil
			}
		})
		if _, err := client.RemovePassword(context.Background(), "", "site"); err != nil {
			t.Fatal(err)
		}
		if accessReads != 2 || patches != 1 {
			t.Fatalf("access reads=%d patches=%d", accessReads, patches)
		}
	})

	t.Run("malformed policy blocks mutation", func(t *testing.T) {
		var patches int
		client := newHereNowTestClient(t, "", func(req *http.Request) (*http.Response, error) {
			if req.Method == http.MethodPatch {
				patches++
			}
			return hereNowTestResponse(http.StatusOK, `{}`), nil
		})
		if _, err := client.SetPassword(context.Background(), "", "site", "vault-only-password"); err == nil {
			t.Fatal("malformed access policy allowed password mutation")
		}
		if patches != 0 {
			t.Fatalf("malformed policy caused %d password patches", patches)
		}
	})
}

func TestHereNowPasswordVaultKeyIsStableAndSystemManaged(t *testing.T) {
	first := HereNowSitePasswordVaultKey(" Account-A ", " My-Site ")
	second := HereNowSitePasswordVaultKey("account-a", "my-site")
	if first != second || !strings.HasPrefix(first, "HERE_NOW_SITE_PASSWORD_") {
		t.Fatalf("password keys = %q / %q", first, second)
	}
	if IsPythonAccessibleSecret(first) {
		t.Fatalf("password Vault key must be hidden from agent code: %s", first)
	}
	if first == HereNowSitePasswordVaultKey("account-b", "my-site") {
		t.Fatal("password Vault keys must be scoped by account")
	}
}
