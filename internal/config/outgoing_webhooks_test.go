package config

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestOutgoingWebhookYAMLNeverIncludesVaultOnlyFields(t *testing.T) {
	t.Parallel()

	hook := OutgoingWebhook{
		ID:            "hook_1",
		Name:          "Deploy",
		Method:        "POST",
		URL:           "https://secret.example/hook?token=value",
		Headers:       map[string]string{"X-Plain": "visible"},
		SecretHeaders: map[string]string{"Authorization": "Bearer secret"},
		BodyTemplate:  `{"password":"secret"}`,
	}
	encoded, err := yaml.Marshal(hook)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}
	for _, secret := range []string{"secret.example", "Bearer secret", `password`, "token=value"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("YAML leaked %q:\n%s", secret, encoded)
		}
	}
	if !strings.Contains(string(encoded), "X-Plain") {
		t.Fatalf("YAML lost non-sensitive header:\n%s", encoded)
	}
}

func TestApplyVaultSecretsHydratesOutgoingWebhookBundle(t *testing.T) {
	t.Parallel()

	bundle, err := json.Marshal(OutgoingWebhookSecrets{
		URL:          "https://example.test/hook",
		BodyTemplate: `{"token":"secret"}`,
		Headers:      map[string]string{"Authorization": "Bearer secret"},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	cfg := &Config{}
	cfg.Webhooks.Outgoing = []OutgoingWebhook{{ID: "hook_1", Headers: map[string]string{"X-Plain": "visible"}}}
	vault := &testSecretVault{data: map[string]string{OutgoingWebhookSecretsVaultKey("hook_1"): string(bundle)}}

	cfg.ApplyVaultSecrets(vault)

	hook := cfg.Webhooks.Outgoing[0]
	if hook.URL != "https://example.test/hook" || !strings.Contains(hook.BodyTemplate, "secret") {
		t.Fatalf("runtime hook not hydrated: %#v", hook)
	}
	if hook.SecretHeaders["Authorization"] != "Bearer secret" {
		t.Fatalf("SecretHeaders = %#v", hook.SecretHeaders)
	}
	if hook.Headers["X-Plain"] != "visible" {
		t.Fatalf("Headers = %#v", hook.Headers)
	}
}

func TestMigratePlaintextSecretsToVaultMovesOutgoingSecretsAndPreservesComments(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	legacy := `# keep this top-level comment
server:
  ui_language: en # keep this field comment
webhooks:
  outgoing:
    - id: hook_1
      name: Deploy
      method: POST
      url: https://example.test/hook?token=secret
      headers:
        Authorization: Bearer secret
        X-Plain: visible
      payload_type: custom
      body_template: '{"token":"secret"}'
`
	if err := os.WriteFile(configPath, []byte(legacy), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	vault := &testSecretVault{data: map[string]string{}}

	MigratePlaintextSecretsToVault(configPath, vault, slog.Default())
	first, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	MigratePlaintextSecretsToVault(configPath, vault, slog.Default())
	second, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("migration is not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	for _, leaked := range []string{"token=secret", "Bearer secret", `body_template:`} {
		if strings.Contains(string(first), leaked) {
			t.Fatalf("migrated YAML leaked %q:\n%s", leaked, first)
		}
	}
	if !strings.Contains(string(first), "# keep this top-level comment") || !strings.Contains(string(first), "# keep this field comment") || !strings.Contains(string(first), "X-Plain: visible") {
		t.Fatalf("migration did not preserve comments/non-sensitive config:\n%s", first)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	cfg.ApplyVaultSecrets(vault)
	hook := cfg.Webhooks.Outgoing[0]
	if !strings.Contains(hook.URL, "token=secret") || hook.SecretHeaders["Authorization"] != "Bearer secret" || !strings.Contains(hook.BodyTemplate, "secret") {
		t.Fatalf("migrated runtime hook = %#v", hook)
	}
}

func TestPersistOutgoingWebhooksRollsBackVaultWhenConfigSaveFails(t *testing.T) {
	t.Parallel()

	vault := &testSecretVault{data: map[string]string{}}
	key := OutgoingWebhookSecretsVaultKey("hook_1")
	vault.data[key] = `{"url":"https://old.example/hook"}`
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("webhooks: [\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cfg := &Config{ConfigPath: configPath}
	incoming := []OutgoingWebhook{{ID: "hook_1", Name: "Hook", Method: "POST", URL: "https://new.example/hook"}}

	if _, err := PersistOutgoingWebhooks(cfg.ConfigPath, cfg, incoming, vault); err == nil {
		t.Fatal("PersistOutgoingWebhooks() error = nil, want config save failure")
	}
	if got := vault.data[key]; got != `{"url":"https://old.example/hook"}` {
		t.Fatalf("vault value after rollback = %q", got)
	}
	if len(cfg.Webhooks.Outgoing) != 0 {
		t.Fatalf("runtime config changed on failure: %#v", cfg.Webhooks.Outgoing)
	}
}

func TestPersistOutgoingWebhooksDeletesRemovedVaultBundleAfterSave(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("# preserve\nwebhooks:\n  outgoing:\n    - id: hook_1\n      name: Old\n      method: POST\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	vault := &testSecretVault{data: map[string]string{OutgoingWebhookSecretsVaultKey("hook_1"): `{"url":"https://old.example/hook"}`}}
	cfg := &Config{ConfigPath: configPath}
	cfg.Webhooks.Outgoing = []OutgoingWebhook{{ID: "hook_1", Name: "Old", Method: "POST", URL: "https://old.example/hook"}}

	updated, err := PersistOutgoingWebhooks(configPath, cfg, []OutgoingWebhook{}, vault)
	if err != nil {
		t.Fatalf("PersistOutgoingWebhooks() error = %v", err)
	}
	if len(updated.Webhooks.Outgoing) != 0 {
		t.Fatalf("outgoing hooks = %#v, want empty", updated.Webhooks.Outgoing)
	}
	if _, exists := vault.data[OutgoingWebhookSecretsVaultKey("hook_1")]; exists {
		t.Fatal("removed webhook vault bundle still exists")
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "# preserve") {
		t.Fatalf("config comment was lost:\n%s", data)
	}
}
