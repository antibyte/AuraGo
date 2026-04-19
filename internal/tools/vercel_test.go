package tools

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVercelListProjectsIncludesScopeAndAuthorization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if got := r.URL.Path; got != "/v9/projects" {
			t.Fatalf("path = %s, want /v9/projects", got)
		}
		if got := r.URL.Query().Get("teamId"); got != "team_123" {
			t.Fatalf("teamId = %q, want team_123", got)
		}
		if got := r.URL.Query().Get("slug"); got != "my-team" {
			t.Fatalf("slug = %q, want my-team", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("authorization = %q, want Bearer test-token", got)
		}
		_, _ = w.Write([]byte(`{"projects":[{"id":"prj_123","name":"homepage","framework":"vite"}]}`))
	}))
	defer server.Close()

	prevBaseURL := vercelBaseURL
	prevClient := vercelHTTPClient
	vercelBaseURL = server.URL
	vercelHTTPClient = server.Client()
	defer func() {
		vercelBaseURL = prevBaseURL
		vercelHTTPClient = prevClient
	}()

	result := VercelListProjects(VercelConfig{
		Token:    "test-token",
		TeamID:   "team_123",
		TeamSlug: "my-team",
	})
	if !strings.Contains(result, `"status":"ok"`) {
		t.Fatalf("expected ok result, got %s", result)
	}
	if !strings.Contains(result, `"name":"homepage"`) {
		t.Fatalf("expected project in response, got %s", result)
	}
}

func TestVercelAssignAliasUsesDeploymentEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if got := r.URL.Path; got != "/v2/deployments/dpl_123/aliases" {
			t.Fatalf("path = %s, want /v2/deployments/dpl_123/aliases", got)
		}
		_, _ = w.Write([]byte(`{"uid":"al_123","alias":"www.example.com","created":"2026-04-19T10:00:00Z"}`))
	}))
	defer server.Close()

	prevBaseURL := vercelBaseURL
	prevClient := vercelHTTPClient
	vercelBaseURL = server.URL
	vercelHTTPClient = server.Client()
	defer func() {
		vercelBaseURL = prevBaseURL
		vercelHTTPClient = prevClient
	}()

	result := VercelAssignAlias(VercelConfig{Token: "test-token"}, "dpl_123", "www.example.com")
	if !strings.Contains(result, `"status":"ok"`) {
		t.Fatalf("expected ok result, got %s", result)
	}
	if !strings.Contains(result, `"alias":"www.example.com"`) {
		t.Fatalf("expected alias in response, got %s", result)
	}
}
