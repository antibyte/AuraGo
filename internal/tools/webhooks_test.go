package tools

import (
	"aurago/internal/testutil"
	"context"
	"net/http"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestManageOutgoingWebhooksPersistsConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"
	configContent := `
server:
  ui_language: en
auth:
  enabled: false
personality:
  core_personality: neutral
webhooks:
  enabled: true
  readonly: false
  outgoing: []
`
	if err := config.WriteFileAtomic(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	result := ManageOutgoingWebhooks(
		"create",
		"hook_test",
		"Deploy",
		"Trigger deploy",
		"POST",
		"https://example.test/deploy",
		"json",
		"",
		map[string]string{"X-Test": "1"},
		[]interface{}{map[string]interface{}{"name": "service", "type": "string", "description": "Service", "required": true}},
		cfg,
	)

	if !strings.Contains(result, `"status":"success"`) || !strings.Contains(result, `"persisted": true`) {
		t.Fatalf("unexpected result: %s", result)
	}

	reloaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if len(reloaded.Webhooks.Outgoing) != 1 {
		t.Fatalf("outgoing webhook count = %d, want 1", len(reloaded.Webhooks.Outgoing))
	}
	if reloaded.Webhooks.Outgoing[0].ID != "hook_test" {
		t.Fatalf("webhook id = %q, want hook_test", reloaded.Webhooks.Outgoing[0].ID)
	}
}

func TestExecuteOutgoingWebhookRejectsOversizedResponseBody(t *testing.T) {
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("w", int(maxHTTPResponseSize)+1)))
	}))
	defer server.Close()

	oldClient := outgoingWebhookHTTPClient
	outgoingWebhookHTTPClient = server.Client()
	defer func() { outgoingWebhookHTTPClient = oldClient }()

	_, _, err := ExecuteOutgoingWebhook(context.Background(), config.OutgoingWebhook{
		Method: http.MethodPost,
		URL:    server.URL,
	}, map[string]interface{}{"message": "hello"})
	if err == nil || !strings.Contains(err.Error(), "response body exceeds limit") {
		t.Fatalf("expected oversized response error, got %v", err)
	}
}
