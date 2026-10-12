package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExecuteAPIRequestAllowsConfiguredLocalOllamaEndpoint(t *testing.T) {
	ConfigureRuntimePermissions(RuntimePermissions{AllowNetworkRequests: true})
	t.Cleanup(func() {
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected request path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(server.Close)

	got := ExecuteAPIRequestWithOptions(
		http.MethodPost,
		server.URL+"/v1/chat/completions",
		`{"model":"phi3:latest","messages":[]}`,
		nil,
		APIRequestOptions{AllowedLocalOllamaBaseURL: server.URL},
	)

	var out APIResult
	if err := json.Unmarshal([]byte(got), &out); err != nil {
		t.Fatalf("decode result: %v; raw=%s", err, got)
	}
	if out.Status != "success" || out.StatusCode != http.StatusOK || !strings.Contains(out.Body, `"ok":true`) {
		t.Fatalf("unexpected result: %+v raw=%s", out, got)
	}
}

func TestExecuteAPIRequestKeepsSSRFForNonOllamaLocalPaths(t *testing.T) {
	ConfigureRuntimePermissions(RuntimePermissions{AllowNetworkRequests: true})
	t.Cleanup(func() {
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})

	got := ExecuteAPIRequestWithOptions(
		http.MethodGet,
		"http://127.0.0.1:11434/admin",
		"",
		nil,
		APIRequestOptions{AllowedLocalOllamaBaseURL: "http://127.0.0.1:11434"},
	)

	var out APIResult
	if err := json.Unmarshal([]byte(got), &out); err != nil {
		t.Fatalf("decode result: %v; raw=%s", err, got)
	}
	if out.Status != "error" || !strings.Contains(out.Message, "SSRF protection") {
		t.Fatalf("expected SSRF rejection for non-Ollama path, got %+v raw=%s", out, got)
	}
}

func TestExecuteAPIRequestRejectsOllamaAdminPaths(t *testing.T) {
	// Residual: POST /api/generate stays allowed, so api_request can still send keep_alive.
	ConfigureRuntimePermissions(RuntimePermissions{AllowNetworkRequests: true})
	t.Cleanup(func() {
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("admin path reached the server: %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(server.Close)

	paths := []string{"/api/pull", "/api/delete", "/api/push", "/api/create", "/api/copy", "/api/blobs/sha256:abc"}
	for _, path := range paths {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			got := ExecuteAPIRequestWithOptions(method, server.URL+path, "", nil, APIRequestOptions{AllowedLocalOllamaBaseURL: server.URL})
			var out APIResult
			if err := json.Unmarshal([]byte(got), &out); err != nil {
				t.Fatalf("%s %s decode: %v raw=%s", method, path, err, got)
			}
			if out.Status != "error" || !strings.Contains(out.Message, "SSRF protection") {
				t.Fatalf("%s %s: expected SSRF rejection, got %+v raw=%s", method, path, out, got)
			}
		}
	}
}
