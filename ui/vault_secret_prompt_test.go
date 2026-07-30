package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVaultSecretPromptUIKeepsSecretOutOfPersistentAndDisplayPaths(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("js", "chat", "chat-vault-secret.js"))
	if err != nil {
		t.Fatal(err)
	}
	js := string(source)
	for _, required := range []string{
		"/api/agent/vault-secret/status",
		"/api/agent/vault-secret/submit",
		"/api/agent/vault-secret/cancel",
		"input.type = 'password'",
		"input.autocomplete = 'new-password'",
		"input.value = ''",
		"prompt.textContent",
		"notice.textContent",
		"key.textContent",
		"keepalive: Boolean(keepalive)",
	} {
		if !strings.Contains(js, required) {
			t.Fatalf("chat-vault-secret.js missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"localStorage",
		"sessionStorage",
		"console.",
		"innerHTML",
		"URLSearchParams",
	} {
		if strings.Contains(js, forbidden) {
			t.Fatalf("chat-vault-secret.js contains forbidden secret path %q", forbidden)
		}
	}

	streaming, err := os.ReadFile(filepath.Join("js", "chat", "chat-streaming.js"))
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []string{"vault.secret.prompt", "vault.secret.ack"} {
		if !strings.Contains(string(streaming), event) {
			t.Fatalf("chat streaming missing %q", event)
		}
	}
}

func TestVaultSecretPromptTranslationsExistInAllChatLocales(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("lang", "chat", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 16 {
		t.Fatalf("chat locale count = %d, want 16", len(files))
	}
	required := []string{
		"chat.vault_secret_title",
		"chat.vault_secret_notice",
		"chat.vault_secret_key_label",
		"chat.vault_secret_input_label",
		"chat.vault_secret_cancel",
		"chat.vault_secret_save",
		"chat.vault_secret_error",
		"chat.vault_secret_error_code",
	}
	englishValues := map[string]string{}
	for _, file := range files {
		raw, readErr := os.ReadFile(file)
		if readErr != nil {
			t.Fatal(readErr)
		}
		var locale map[string]string
		if err := json.Unmarshal(raw, &locale); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		for _, key := range required {
			if strings.TrimSpace(locale[key]) == "" {
				t.Fatalf("%s missing %s", file, key)
			}
		}
		if filepath.Base(file) == "en.json" {
			for _, key := range required {
				englishValues[key] = locale[key]
			}
		}
	}
	for _, file := range files {
		if filepath.Base(file) == "en.json" {
			continue
		}
		raw, _ := os.ReadFile(file)
		var locale map[string]string
		_ = json.Unmarshal(raw, &locale)
		if locale["chat.vault_secret_notice"] == englishValues["chat.vault_secret_notice"] {
			t.Fatalf("%s uses the English Vault notice placeholder", file)
		}
	}
}

func TestVaultSecretPromptIsIncludedInGeneratedChatBundle(t *testing.T) {
	bundle, err := os.ReadFile(filepath.Join("js", "chat", "bundles", "chat-runtime.bundle.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(bundle), "/* ui/js/chat/chat-vault-secret.js */") {
		t.Fatal("generated chat runtime bundle omits chat-vault-secret.js")
	}
}
