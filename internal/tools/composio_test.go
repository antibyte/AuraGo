package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestComposioClientListToolkitsPaginatesAndSendsAPIKey(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			t.Fatalf("x-api-key header = %q, want test-key", r.Header.Get("x-api-key"))
		}
		if r.URL.Path != "/toolkits" {
			t.Fatalf("path = %q, want /toolkits", r.URL.Path)
		}
		requests++
		switch r.URL.Query().Get("cursor") {
		case "":
			_, _ = w.Write([]byte(`{"items":[{"slug":"github","name":"GitHub"}],"next_cursor":"next-page"}`))
		case "next-page":
			_, _ = w.Write([]byte(`{"items":[{"slug":"gmail","name":"Gmail"}]}`))
		default:
			t.Fatalf("unexpected cursor %q", r.URL.Query().Get("cursor"))
		}
	}))
	defer server.Close()

	client := NewComposioClient(ComposioClientConfig{
		BaseURL:        server.URL,
		APIKey:         "test-key",
		Timeout:        time.Second,
		MaxResultBytes: 32 * 1024,
	})

	page, err := client.ListToolkits(context.Background(), ComposioListQuery{Limit: 1})
	if err != nil {
		t.Fatalf("ListToolkits() error = %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Slug != "github" || page.NextCursor != "next-page" {
		t.Fatalf("unexpected first page: %+v", page)
	}

	page, err = client.ListToolkits(context.Background(), ComposioListQuery{Limit: 1, Cursor: page.NextCursor})
	if err != nil {
		t.Fatalf("ListToolkits(next) error = %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Slug != "gmail" {
		t.Fatalf("unexpected second page: %+v", page)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
}

func TestComposioClientNormalizesErrorsAndCapsResults(t *testing.T) {
	t.Run("api_error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"message":"rate limited"}`, http.StatusTooManyRequests)
		}))
		defer server.Close()

		client := NewComposioClient(ComposioClientConfig{
			BaseURL:        server.URL,
			APIKey:         "test-key",
			Timeout:        time.Second,
			MaxResultBytes: 32 * 1024,
		})
		_, err := client.ListTools(context.Background(), ComposioToolQuery{ToolkitSlug: "github"})
		if err == nil || !strings.Contains(err.Error(), "429") || !strings.Contains(err.Error(), "rate limited") {
			t.Fatalf("ListTools() error = %v, want normalized 429 rate limit error", err)
		}
	})

	t.Run("result_cap", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"items":[{"slug":"tool","description":"` + strings.Repeat("x", 256) + `"}]}`))
		}))
		defer server.Close()

		client := NewComposioClient(ComposioClientConfig{
			BaseURL:        server.URL,
			APIKey:         "test-key",
			Timeout:        time.Second,
			MaxResultBytes: 64,
		})
		_, err := client.ListTools(context.Background(), ComposioToolQuery{ToolkitSlug: "github"})
		if err == nil || !strings.Contains(err.Error(), "exceeds composio result size limit") {
			t.Fatalf("ListTools() error = %v, want size limit error", err)
		}
	})
}

func TestComposioClientExecuteToolPostsArguments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tools/execute/GITHUB_GET_REPO" {
			t.Fatalf("path = %q, want execute endpoint", r.URL.Path)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload["connected_account_id"] != "acct_1" {
			t.Fatalf("connected_account_id = %v, want acct_1", payload["connected_account_id"])
		}
		args, ok := payload["arguments"].(map[string]interface{})
		if !ok || args["owner"] != "aurago" {
			t.Fatalf("arguments = %#v, want owner", payload["arguments"])
		}
		_, _ = w.Write([]byte(`{"status":"success","data":{"name":"repo"}}`))
	}))
	defer server.Close()

	client := NewComposioClient(ComposioClientConfig{
		BaseURL:        server.URL,
		APIKey:         "test-key",
		Timeout:        time.Second,
		MaxResultBytes: 32 * 1024,
	})
	result, err := client.ExecuteTool(context.Background(), ComposioExecuteRequest{
		ToolSlug:           "GITHUB_GET_REPO",
		ConnectedAccountID: "acct_1",
		Arguments:          map[string]interface{}{"owner": "aurago"},
	})
	if err != nil {
		t.Fatalf("ExecuteTool() error = %v", err)
	}
	if !strings.Contains(string(result.Raw), `"repo"`) {
		t.Fatalf("raw result = %s, want response payload", result.Raw)
	}
}

func TestComposioPolicyRequiresSelectedToolkitAndBlocksRisk(t *testing.T) {
	cfg := ComposioPolicyConfig{
		Enabled:          true,
		ReadOnly:         true,
		AllowDestructive: false,
		Toolkits: []ComposioToolkitPolicy{
			{Slug: "github", Enabled: true},
			{Slug: "gmail", Enabled: true, AllowedToolSlugs: []string{"GMAIL_SEND_EMAIL"}},
			{Slug: "slack", Enabled: true, BlockedToolSlugs: []string{"SLACK_GET_CHANNEL"}},
		},
	}

	if decision := EvaluateComposioToolPolicy(cfg, ComposioToolInfo{Slug: "GITHUB_GET_REPOSITORY", ToolkitSlug: "github"}); !decision.Allowed {
		t.Fatalf("expected read tool to be allowed, got %+v", decision)
	}
	if decision := EvaluateComposioToolPolicy(cfg, ComposioToolInfo{Slug: "GITHUB_CREATE_ISSUE", ToolkitSlug: "github"}); decision.Allowed || !strings.Contains(decision.Reason, "read-only") {
		t.Fatalf("expected create tool blocked by read-only, got %+v", decision)
	}
	if decision := EvaluateComposioToolPolicy(cfg, ComposioToolInfo{Slug: "GITHUB_DELETE_REPOSITORY", ToolkitSlug: "github"}); decision.Allowed || !strings.Contains(decision.Reason, "destructive") {
		t.Fatalf("expected delete tool blocked as destructive, got %+v", decision)
	}
	if decision := EvaluateComposioToolPolicy(cfg, ComposioToolInfo{Slug: "DROPBOX_GET_FILE", ToolkitSlug: "dropbox"}); decision.Allowed || !strings.Contains(decision.Reason, "not enabled") {
		t.Fatalf("expected unselected toolkit blocked, got %+v", decision)
	}
	if decision := EvaluateComposioToolPolicy(cfg, ComposioToolInfo{Slug: "GMAIL_SEND_EMAIL", ToolkitSlug: "gmail"}); !decision.Allowed {
		t.Fatalf("expected per-toolkit allowlist override to allow send, got %+v", decision)
	}
	if decision := EvaluateComposioToolPolicy(cfg, ComposioToolInfo{Slug: "SLACK_GET_CHANNEL", ToolkitSlug: "slack"}); decision.Allowed || !strings.Contains(decision.Reason, "blocked") {
		t.Fatalf("expected blocked slug denied, got %+v", decision)
	}
}
