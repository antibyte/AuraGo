package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDispatchComposioCapabilitiesListsConfiguredToolkits(t *testing.T) {
	cfg := &config.Config{}
	cfg.Composio.Enabled = true
	cfg.Composio.APIKey = "cmp-secret"
	cfg.Composio.ReadOnly = true
	cfg.Composio.AllowDestructive = false
	cfg.Composio.Toolkits = []config.ComposioToolkitConfig{
		{Slug: "gmail", Enabled: true},
		{Slug: "slack", Enabled: false},
	}

	out := dispatchComposioCall(context.Background(), composioCallArgs{Operation: "capabilities"}, cfg)
	for _, want := range []string{`"operation":"capabilities"`, `"toolkit_slug":"gmail"`, `"read_only":true`, `"allow_destructive":false`, `"search_tools"`, `"execute_tool"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("capabilities output missing %q: %s", want, out)
		}
	}
	for _, notWant := range []string{"cmp-secret", `"toolkit_slug":"slack"`, "allowed_tool_count", "blocked_tool_count", "allowlist_enabled"} {
		if strings.Contains(out, notWant) {
			t.Fatalf("capabilities output leaked %q: %s", notWant, out)
		}
	}
}

func TestDispatchComposioSearchToolsRetriesWithoutNarrowQuery(t *testing.T) {
	queries := []string{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tools" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("toolkit_slug"); got != "gmail" {
			t.Fatalf("toolkit_slug = %q, want gmail", got)
		}
		queries = append(queries, r.URL.Query().Get("query"))
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("query") != "" {
			_, _ = w.Write([]byte(`{"items":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"items":[{"slug":"GMAIL_FETCH_EMAILS","description":"Fetch Gmail messages","toolkit":{"slug":"gmail"}}]}`))
	}))
	defer upstream.Close()

	cfg := &config.Config{}
	cfg.Composio.Enabled = true
	cfg.Composio.APIKey = "cmp-secret"
	cfg.Composio.BaseURL = upstream.URL
	cfg.Composio.RequestTimeoutSeconds = 2
	cfg.Composio.MaxResultBytes = 4096
	cfg.Composio.Toolkits = []config.ComposioToolkitConfig{{Slug: "gmail", Enabled: true}}

	out := dispatchComposioCall(context.Background(), composioCallArgs{Operation: "search_tools", ToolkitSlug: "gmail", Query: "latest mail summary"}, cfg)
	if !strings.Contains(out, "GMAIL_FETCH_EMAILS") || !strings.Contains(out, "query_relaxed") {
		t.Fatalf("search_tools should retry without the narrow query and return Gmail tools: %s", out)
	}
	if strings.Join(queries, ",") != "latest mail summary," {
		t.Fatalf("queries = %v, want narrow query then empty query", queries)
	}
}

func TestDispatchComposioCapabilitiesReportsToolkitConnection(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/connected_accounts":
			if got := r.URL.Query().Get("toolkit_slugs"); got != "gmail" {
				t.Fatalf("toolkit_slugs = %q, want gmail", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[{"id":"acct_1","status":"ACTIVE","toolkit":{"slug":"gmail"}}]}`))
		case "/tools":
			if got := r.URL.Query().Get("toolkit_slug"); got != "gmail" {
				t.Fatalf("toolkit_slug = %q, want gmail", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[
				{"slug":"GMAIL_FETCH_EMAILS","description":"Fetches Gmail messages","toolkit":{"slug":"gmail"}},
				{"slug":"GMAIL_FETCH_MESSAGE_BY_MESSAGE_ID","description":"Fetches one Gmail message","toolkit":{"slug":"gmail"}}
			]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	cfg := &config.Config{}
	cfg.Composio.Enabled = true
	cfg.Composio.APIKey = "cmp-secret"
	cfg.Composio.BaseURL = upstream.URL
	cfg.Composio.UserID = "aurago-default"
	cfg.Composio.RequestTimeoutSeconds = 2
	cfg.Composio.MaxResultBytes = 4096
	cfg.Composio.Toolkits = []config.ComposioToolkitConfig{{Slug: "gmail", Enabled: true}}

	out := dispatchComposioCall(context.Background(), composioCallArgs{Operation: "capabilities", ToolkitSlug: "gmail"}, cfg)
	for _, want := range []string{"toolkit_slug", "gmail", "connection_status", "connected", "GMAIL_FETCH_EMAILS", "GMAIL_FETCH_MESSAGE_BY_MESSAGE_ID", "execute_tool"} {
		if !strings.Contains(out, want) {
			t.Fatalf("capabilities toolkit output missing %q: %s", want, out)
		}
	}
	for _, notWant := range []string{"acct_1", "allowed_tool_count", "blocked_tool_count", "allowlist_enabled"} {
		if strings.Contains(out, notWant) {
			t.Fatalf("capabilities output must not expose %q: %s", notWant, out)
		}
	}
}

func TestDispatchComposioListConnectedAccountsSanitizesAccountIDs(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/connected_accounts" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"id":"acct_1","user_id":"aurago-default","status":"ACTIVE","toolkit":{"slug":"gmail"}}]}`))
	}))
	defer upstream.Close()

	cfg := &config.Config{}
	cfg.Composio.Enabled = true
	cfg.Composio.APIKey = "cmp-secret"
	cfg.Composio.BaseURL = upstream.URL
	cfg.Composio.UserID = "aurago-default"
	cfg.Composio.RequestTimeoutSeconds = 2
	cfg.Composio.MaxResultBytes = 4096
	cfg.Composio.Toolkits = []config.ComposioToolkitConfig{{Slug: "gmail", Enabled: true}}

	out := dispatchComposioCall(context.Background(), composioCallArgs{Operation: "list_connected_accounts", ToolkitSlug: "gmail"}, cfg)
	for _, want := range []string{`"connection_status":"connected"`, `"connected_account_count":1`, `"toolkit_slug":"gmail"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("sanitized list_connected_accounts output missing %q: %s", want, out)
		}
	}
	for _, notWant := range []string{"acct_1", "aurago-default"} {
		if strings.Contains(out, notWant) {
			t.Fatalf("list_connected_accounts output must not expose %q: %s", notWant, out)
		}
	}
}
