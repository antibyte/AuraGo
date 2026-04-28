package tools

import (
	"aurago/internal/testutil"
	"net/http"
	"strings"
	"testing"
)

func TestVercelListProjectsIncludesScopeAndAuthorization(t *testing.T) {
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestVercelDeleteProjectSuccess(t *testing.T) {
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s, want DELETE", r.Method)
		}
		if got := r.URL.Path; got != "/v9/projects/my-project" {
			t.Fatalf("path = %s, want /v9/projects/my-project", got)
		}
		w.WriteHeader(http.StatusNoContent)
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

	result := VercelDeleteProject(VercelConfig{Token: "test-token"}, "my-project")
	if !strings.Contains(result, `"status":"ok"`) {
		t.Fatalf("expected ok result, got %s", result)
	}
	if !strings.Contains(result, `"project_id":"my-project"`) {
		t.Fatalf("expected project_id in response, got %s", result)
	}
}

func TestVercelRollbackSuccess(t *testing.T) {
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if got := r.URL.Path; got != "/v9/projects/my-project/rollback/dpl_123" {
			t.Fatalf("path = %s, want /v9/projects/my-project/rollback/dpl_123", got)
		}
		_, _ = w.Write([]byte(`{"url":"https://my-project.vercel.app"}`))
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

	result := VercelRollback(VercelConfig{Token: "test-token"}, "my-project", "dpl_123")
	if !strings.Contains(result, `"status":"ok"`) {
		t.Fatalf("expected ok result, got %s", result)
	}
	if !strings.Contains(result, `"deployment_id":"dpl_123"`) {
		t.Fatalf("expected deployment_id in response, got %s", result)
	}
}

func TestVercelCancelDeploySuccess(t *testing.T) {
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("method = %s, want PATCH", r.Method)
		}
		if got := r.URL.Path; got != "/v12/deployments/dpl_456/cancel" {
			t.Fatalf("path = %s, want /v12/deployments/dpl_456/cancel", got)
		}
		_, _ = w.Write([]byte(`{"state":"CANCELED"}`))
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

	result := VercelCancelDeploy(VercelConfig{Token: "test-token"}, "dpl_456")
	if !strings.Contains(result, `"status":"ok"`) {
		t.Fatalf("expected ok result, got %s", result)
	}
	if !strings.Contains(result, `"state":"CANCELED"`) {
		t.Fatalf("expected state in response, got %s", result)
	}
}

func TestVercelGetEnvSuccess(t *testing.T) {
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if got := r.URL.Path; got != "/v9/projects/my-project/env/API_URL" {
			t.Fatalf("path = %s, want /v9/projects/my-project/env/API_URL", got)
		}
		_, _ = w.Write([]byte(`{"id":"env_123","key":"API_URL","type":"plain","target":["production","preview"],"createdAt":1234567890,"updatedAt":1234567890}`))
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

	result := VercelGetEnv(VercelConfig{Token: "test-token"}, "my-project", "API_URL")
	if !strings.Contains(result, `"status":"ok"`) {
		t.Fatalf("expected ok result, got %s", result)
	}
	if !strings.Contains(result, `"key":"API_URL"`) {
		t.Fatalf("expected key in response, got %s", result)
	}
}
