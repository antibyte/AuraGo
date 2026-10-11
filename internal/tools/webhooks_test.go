package tools

import (
	"aurago/internal/testutil"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
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
	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(tmpDir, "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
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
		vault,
		false,
		false,
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
	reloaded.ApplyVaultSecrets(vault)
	if reloaded.Webhooks.Outgoing[0].ID != "hook_test" {
		t.Fatalf("webhook id = %q, want hook_test", reloaded.Webhooks.Outgoing[0].ID)
	}
	if reloaded.Webhooks.Outgoing[0].URL != "https://example.test/deploy" {
		t.Fatalf("vault-backed URL = %q", reloaded.Webhooks.Outgoing[0].URL)
	}
	stored, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(stored), "example.test/deploy") {
		t.Fatalf("config leaked outgoing URL:\n%s", stored)
	}
}

func TestManageOutgoingWebhooksReadOnlyBlocksMutations(t *testing.T) {
	cfg := &config.Config{}
	cfg.Webhooks.Enabled = true
	cfg.Webhooks.ReadOnly = true
	cfg.Webhooks.Outgoing = []config.OutgoingWebhook{{
		ID:     "hook_test",
		Name:   "Deploy",
		Method: "POST",
		URL:    "https://example.test/deploy",
	}}

	result := ManageOutgoingWebhooks(
		"delete",
		"hook_test",
		"",
		"",
		"",
		"",
		"",
		"",
		nil,
		nil,
		cfg,
		nil,
		false,
		false,
	)

	if !strings.Contains(result, `"status":"error"`) || !strings.Contains(result, "Read-Only") {
		t.Fatalf("expected read-only error, got: %s", result)
	}
	if len(cfg.Webhooks.Outgoing) != 1 {
		t.Fatalf("webhook count = %d, want unchanged 1", len(cfg.Webhooks.Outgoing))
	}
}

func TestManageOutgoingWebhooksListMasksSecrets(t *testing.T) {
	cfg := &config.Config{}
	cfg.Webhooks.Outgoing = []config.OutgoingWebhook{{
		ID:           "hook_secret",
		Name:         "Secret Hook",
		Method:       "POST",
		URL:          "https://example.test/hook",
		Headers:      map[string]string{"Authorization": "Bearer secret-token", "X-Plain": "visible"},
		PayloadType:  "custom",
		BodyTemplate: `{"token":"secret-token"}`,
	}}

	result := ManageOutgoingWebhooks("list", "", "", "", "", "", "", "", nil, nil, cfg, nil, false, false)

	if strings.Contains(result, "secret-token") {
		t.Fatalf("list output leaked secret: %s", result)
	}
	if !strings.Contains(result, outgoingWebhookMaskedValue) {
		t.Fatalf("list output did not contain masked marker: %s", result)
	}
	if strings.Contains(result, "https://example.test/hook") {
		t.Fatalf("list output leaked URL: %s", result)
	}
	if !strings.Contains(result, "visible") {
		t.Fatalf("list output should keep non-sensitive header visible: %s", result)
	}
}

func TestExecuteOutgoingWebhookRejectsUnsupportedMethodBeforeNetwork(t *testing.T) {
	t.Parallel()

	_, _, err := ExecuteOutgoingWebhook(context.Background(), config.OutgoingWebhook{
		Method: "TRACE",
		URL:    "https://example.invalid/hook",
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported outgoing webhook method") {
		t.Fatalf("error = %v, want unsupported method", err)
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

func TestExecuteOutgoingWebhookFormEncodesParams(t *testing.T) {
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Fatalf("Content-Type = %q, want application/x-www-form-urlencoded", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		if got := string(body); got != "message=hello+world&priority=5" {
			t.Fatalf("body = %q, want encoded form", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	oldClient := outgoingWebhookHTTPClient
	outgoingWebhookHTTPClient = server.Client()
	defer func() { outgoingWebhookHTTPClient = oldClient }()

	_, statusCode, err := ExecuteOutgoingWebhook(context.Background(), config.OutgoingWebhook{
		Method:      http.MethodPost,
		URL:         server.URL,
		PayloadType: "form",
	}, map[string]interface{}{"message": "hello world", "priority": 5})
	if err != nil {
		t.Fatalf("ExecuteOutgoingWebhook() error = %v", err)
	}
	if statusCode != http.StatusOK {
		t.Fatalf("statusCode = %d, want %d", statusCode, http.StatusOK)
	}
}

func TestExecuteOutgoingWebhookSendsRuntimeSecretHeaders(t *testing.T) {
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer runtime-secret" {
			t.Fatalf("Authorization = %q, want runtime secret", got)
		}
		if got := r.Header.Get("X-Plain"); got != "visible" {
			t.Fatalf("X-Plain = %q, want visible", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	oldClient := outgoingWebhookHTTPClient
	outgoingWebhookHTTPClient = server.Client()
	defer func() { outgoingWebhookHTTPClient = oldClient }()

	_, statusCode, err := ExecuteOutgoingWebhook(context.Background(), config.OutgoingWebhook{
		Method:        http.MethodPost,
		URL:           server.URL,
		Headers:       map[string]string{"X-Plain": "visible"},
		SecretHeaders: map[string]string{"Authorization": "Bearer runtime-secret"},
	}, nil)
	if err != nil {
		t.Fatalf("ExecuteOutgoingWebhook() error = %v", err)
	}
	if statusCode != http.StatusNoContent {
		t.Fatalf("statusCode = %d, want %d", statusCode, http.StatusNoContent)
	}
}

func TestManageOutgoingWebhooksUpdateKeepsOmittedFields(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
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
	vault, err := security.NewVault(strings.Repeat("b", 64), filepath.Join(tmpDir, "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}
	created := ManageOutgoingWebhooks(
		"create", "hook_keep", "Alarm", "ring", "PUT", "https://example.test/alarm",
		"custom", `{"room":"{{room}}"}`,
		map[string]string{"Authorization": "Bearer secret", "X-Plain": "visible"},
		[]interface{}{map[string]interface{}{"name": "room", "type": "string", "required": true}},
		cfg, vault, true, true,
	)
	if !strings.Contains(created, `"status":"success"`) {
		t.Fatalf("create: %s", created)
	}
	updated := ManageOutgoingWebhooks(
		"update", "hook_keep", "Renamed", "", "", "", "", "",
		nil, nil, cfg, vault, false, false,
	)
	if !strings.Contains(updated, `"status":"success"`) {
		t.Fatalf("update: %s", updated)
	}
	hook := cfg.Webhooks.Outgoing[0]
	if hook.Name != "Renamed" || hook.Method != "PUT" || hook.URL != "https://example.test/alarm" {
		t.Fatalf("name/method/url = %#v", hook)
	}
	if hook.BodyTemplate != `{"room":"{{room}}"}` {
		t.Fatalf("body = %q", hook.BodyTemplate)
	}
	if hook.SecretHeaders["Authorization"] != "Bearer secret" {
		t.Fatalf("secret header = %#v", hook.SecretHeaders)
	}
	if hook.Headers["X-Plain"] != "visible" {
		t.Fatalf("public headers = %#v", hook.Headers)
	}
	if len(hook.Parameters) != 1 || hook.Parameters[0].Name != "room" {
		t.Fatalf("parameters = %#v", hook.Parameters)
	}

	cleared := ManageOutgoingWebhooks(
		"update", "hook_keep", "", "", "", "", "", "",
		map[string]string{}, nil, cfg, vault, true, false,
	)
	if !strings.Contains(cleared, `"status":"success"`) {
		t.Fatalf("clear headers: %s", cleared)
	}
	hook = cfg.Webhooks.Outgoing[0]
	if len(hook.SecretHeaders) != 0 || len(hook.Headers) != 0 {
		t.Fatalf("headers were kept: public=%#v secret=%#v", hook.Headers, hook.SecretHeaders)
	}
	if hook.Method != "PUT" || hook.URL != "https://example.test/alarm" {
		t.Fatalf("clearing headers changed the hook: %#v", hook)
	}
}

func TestExecuteOutgoingWebhookDoesNotFollowCrossOriginRedirect(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	var hits atomic.Int32
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Header.Get("Authorization") != "" {
			t.Errorf("second host saw Authorization")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer second.Close()
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, second.URL+"/landed", http.StatusTemporaryRedirect)
	}))
	defer first.Close()

	_, _, err := ExecuteOutgoingWebhook(context.Background(), config.OutgoingWebhook{
		Method:        http.MethodPost,
		URL:           first.URL,
		SecretHeaders: map[string]string{"Authorization": "Bearer secret"},
	}, nil)
	if err == nil {
		t.Fatal("expected cross-origin redirect to fail")
	}
	if hits.Load() != 0 {
		t.Fatalf("second host saw %d requests", hits.Load())
	}
}
