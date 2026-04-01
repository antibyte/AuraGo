package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestAnsibleRequestRejectsOversizedResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("a", int(maxHTTPResponseSize)+1)))
	}))
	defer server.Close()

	_, _, err := ansibleRequest(AnsibleConfig{URL: server.URL}, http.MethodGet, "/status", nil)
	if err == nil || !strings.Contains(err.Error(), "response body exceeds limit") {
		t.Fatalf("expected oversized response error, got %v", err)
	}
}

func TestGoogleWorkspaceRequestRejectsOversizedResponseBody(t *testing.T) {
	oldClient := gwHTTPClient
	t.Cleanup(func() { gwHTTPClient = oldClient })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("g", int(maxHTTPResponseSize)+1)))
	}))
	defer server.Close()

	gwHTTPClient = server.Client()
	client := &GWorkspaceClient{AccessToken: "token"}
	_, _, err := client.request(http.MethodGet, server.URL, nil)
	if err == nil || !strings.Contains(err.Error(), "response body exceeds limit") {
		t.Fatalf("expected oversized response error, got %v", err)
	}
}

func TestHARequestRejectsOversizedResponseBody(t *testing.T) {
	oldClient := haHTTPClient
	t.Cleanup(func() { haHTTPClient = oldClient })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("h", int(maxHTTPResponseSize)+1)))
	}))
	defer server.Close()

	haHTTPClient = server.Client()
	_, _, err := haRequest(HAConfig{URL: server.URL, AccessToken: "token"}, http.MethodGet, "/api/states", "")
	if err == nil || !strings.Contains(err.Error(), "response body exceeds limit") {
		t.Fatalf("expected oversized response error, got %v", err)
	}
}

func TestVirusTotalDoRequestRejectsOversizedResponseBody(t *testing.T) {
	oldClient := virustotalHTTPClient
	t.Cleanup(func() { virustotalHTTPClient = oldClient })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("v", int(maxHTTPResponseSize)+1)))
	}))
	defer server.Close()

	virustotalHTTPClient = server.Client()
	_, err := virustotalDoRequest("api-key", http.MethodGet, server.URL, nil, "")
	if err == nil || !strings.Contains(err.Error(), "response body exceeds limit") {
		t.Fatalf("expected oversized response error, got %v", err)
	}
}

func TestGenerateOpenAIRejectsOversizedResponseBody(t *testing.T) {
	oldClient := imageGenHTTPClient
	t.Cleanup(func() { imageGenHTTPClient = oldClient })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("i", int(maxHTTPResponseSize)+1)))
	}))
	defer server.Close()

	imageGenHTTPClient = server.Client()
	_, _, err := generateOpenAI(ImageGenConfig{BaseURL: server.URL, APIKey: "token", Model: "dall-e-3"}, "prompt", ImageGenOptions{})
	if err == nil || !strings.Contains(err.Error(), "response body exceeds limit") {
		t.Fatalf("expected oversized response error, got %v", err)
	}
}

func TestGotenbergHealthRejectsOversizedResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("j", int(maxHTTPResponseSize)+1)))
	}))
	defer server.Close()

	out := GotenbergHealth(t.Context(), &config.GotenbergConfig{URL: server.URL, Timeout: 5})
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if parsed["status"] != "error" {
		t.Fatalf("expected error status, got %v", parsed["status"])
	}
	message, _ := parsed["message"].(string)
	if !strings.Contains(message, "response body exceeds limit") {
		t.Fatalf("expected oversized response message, got %q", message)
	}
}
