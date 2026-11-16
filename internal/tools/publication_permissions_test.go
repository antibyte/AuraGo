package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

type publicationTransportFunc func(*http.Request) (*http.Response, error)

func (f publicationTransportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOneDriveRefreshRetainsExistingGrant(t *testing.T) {
	old := odHTTPClient
	defer func() { odHTTPClient = old }()
	called := false
	odHTTPClient = &http.Client{Transport: publicationTransportFunc(func(r *http.Request) (*http.Response, error) {
		called = true
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Has("scope") || r.Form.Get("grant_type") != "refresh_token" {
			t.Error("refresh requested new scopes")
		}
		return &http.Response{StatusCode: 400, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})}
	client := &OneDriveClient{ClientID: "fixture", RefreshToken: "fixture-refresh", TenantID: "common"}
	if err := client.refreshIfNeeded(); err == nil || !called {
		t.Fatalf("refresh not tested: called=%v err=%v", called, err)
	}
}

func TestOneDrivePublicShareNeedsExplicitGrant(t *testing.T) {
	old := odHTTPClient
	defer func() { odHTTPClient = old }()
	for _, readonly := range []bool{false, true} {
		for _, allow := range []bool{false, true} {
			requests := 0
			odHTTPClient = &http.Client{Transport: publicationTransportFunc(func(r *http.Request) (*http.Response, error) {
				requests++
				body := `{"id":"fixture"}`
				if r.Method == http.MethodPost {
					body = `{"link":{"webUrl":"https://example.com/share"}}`
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}
			client := &OneDriveClient{AccessToken: "fixture-access", ReadOnly: readonly, AllowPublicShare: allow}
			result := client.ExecuteOneDrive("share", "report.txt", "", "", 0)
			if !readonly && allow {
				if requests != 2 || !strings.Contains(result, "share_url") {
					t.Fatalf("share failed: %s", result)
				}
			} else if requests != 0 || !strings.Contains(result, "error") {
				t.Fatalf("share bypassed grants: %s", result)
			}
		}
	}
}

func TestPaperlessDeletionNeedsExplicitGrant(t *testing.T) {
	for _, readonly := range []bool{false, true} {
		for _, allow := range []bool{false, true} {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(204) }))
			result := PaperlessDelete(PaperlessConfig{URL: srv.URL, ReadOnly: readonly, AllowDelete: allow}, "1")
			srv.Close()
			if (calls == 1) != (allow && !readonly) {
				t.Fatalf("readonly=%v allow=%v calls=%d result=%s", readonly, allow, calls, result)
			}
		}
	}
}

func TestHuggingFaceCreateDefaultsPrivate(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "true")
	for _, arg := range []string{`{}`, `{"private":null}`, `{"private":true}`, `{"private":false}`} {
		for _, allow := range []bool{false, true} {
			calls := 0
			var sent map[string]interface{}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				_ = json.NewDecoder(r.Body).Decode(&sent)
				_, _ = w.Write([]byte(`{}`))
			}))
			req := HuggingFaceRequest{Operation: "create_repo", RepoID: "owner/repo"}
			if err := json.Unmarshal([]byte(arg), &req); err != nil {
				t.Fatal(err)
			}
			result := RunHuggingFace(context.Background(), config.HuggingFaceConfig{Enabled: true, AllowWrites: true, AllowPublish: allow, AllowedNamespaces: []string{"owner"}, HubBaseURL: srv.URL}, "fixture", "", "", req)
			srv.Close()
			public := req.Private != nil && !*req.Private
			if public && !allow {
				if calls != 0 {
					t.Fatal("public creation bypassed grant")
				}
			} else if calls != 1 || sent["private"] != !public {
				t.Fatalf("privacy lost for %s: %v result=%s", arg, sent, result)
			}
		}
	}
}

func TestHuggingFaceMutationsCheckTargetVisibility(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "true")
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "file.txt"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"upload_file", "create_discussion", "comment_discussion"} {
		for _, visibility := range []string{`{"private":true}`, `{"private":false}`, `{}`, `{"private":"false"}`} {
			for _, allow := range []bool{false, true} {
				posts := 0
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodGet {
						_, _ = w.Write([]byte(visibility))
						return
					}
					posts++
					_, _ = io.Copy(io.Discard, r.Body)
					_, _ = w.Write([]byte(`{}`))
				}))
				result := RunHuggingFace(context.Background(), config.HuggingFaceConfig{Enabled: true, AllowWrites: true, AllowPublish: allow, AllowedNamespaces: []string{"owner"}, HubBaseURL: srv.URL, MaxUploadMB: 1}, "fixture", workspace, t.TempDir(), HuggingFaceRequest{Operation: operation, RepoID: "owner/repo", LocalPath: "file.txt", Path: "file.txt", Title: "title", Body: "body", Number: 1})
				srv.Close()
				want := visibility == `{"private":true}` || (visibility == `{"private":false}` && allow)
				if (posts == 1) != want {
					t.Fatalf("%s visibility=%s publish=%v posts=%d result=%s", operation, visibility, allow, posts, result)
				}
			}
		}
	}
}

func TestMCPDiscoveryAndExecutionUseSameExplicitGrants(t *testing.T) {
	for _, name := range []string{"read_document", "delete_item", "executeCommand", "run_code", "shell", "eval", "python"} {
		for _, allow := range []bool{false, true} {
			for _, listed := range []bool{false, true} {
				cfg := MCPServerConfig{Name: "fixture", Enabled: true, AllowDestructive: allow}
				if listed {
					cfg.AllowedTools = []string{name}
				}
				mgr := &MCPManager{configs: map[string]MCPServerConfig{"fixture": cfg}}
				want := allow || (listed && name == "read_document")
				if mcpToolVisible(cfg, name) != want || (mgr.requireToolAllowed("fixture", name) == nil) != want {
					t.Fatalf("name=%s destructive=%v listed=%v", name, allow, listed)
				}
			}
		}
	}
}
