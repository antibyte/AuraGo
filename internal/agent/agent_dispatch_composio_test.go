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
	for _, want := range []string{`"operation":"capabilities"`, `"toolkit_slug":"gmail"`, `"read_only":true`, `"allow_destructive":false`} {
		if !strings.Contains(out, want) {
			t.Fatalf("capabilities output missing %q: %s", want, out)
		}
	}
	for _, notWant := range []string{"cmp-secret", `"toolkit_slug":"slack"`} {
		if strings.Contains(out, notWant) {
			t.Fatalf("capabilities output leaked %q: %s", notWant, out)
		}
	}
}

func TestDispatchComposioCapabilitiesReportsToolkitConnection(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/connected_accounts" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("toolkit_slugs"); got != "gmail" {
			t.Fatalf("toolkit_slugs = %q, want gmail", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"id":"acct_1","status":"ACTIVE","toolkit":{"slug":"gmail"}}]}`))
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
	for _, want := range []string{`"toolkit_slug":"gmail"`, `"connection_status":"connected"`, `"connected_account_count":1`} {
		if !strings.Contains(out, want) {
			t.Fatalf("capabilities toolkit output missing %q: %s", want, out)
		}
	}
	if strings.Contains(out, "acct_1") {
		t.Fatalf("capabilities output must not expose account IDs: %s", out)
	}
}
