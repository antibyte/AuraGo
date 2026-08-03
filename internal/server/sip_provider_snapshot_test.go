package server

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/security"
)

func TestTelephoneBackendSnapshotResolvesOAuthFromVault(t *testing.T) {
	cfg := telephoneAgentTestConfig(t)
	provider := cfg.FindProvider("phone-agent")
	provider.AuthType = "oauth2"
	provider.APIKey = ""
	vault, err := security.NewVault(strings.Repeat("7", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	if err := vault.WriteSecret("oauth_phone-agent", `{"access_token":"telephone-oauth-token","expiry":"`+expiry+`"}`); err != nil {
		t.Fatal(err)
	}
	server := &Server{Cfg: cfg, Vault: vault}
	runner := NewVoiceActionRunner(server)
	snapshot, err := runner.buildTelephoneBackendSnapshot(context.Background(), cfg, cfg.SIP.Voice)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.agent.config.LLM.APIKey != "telephone-oauth-token" {
		t.Fatal("telephone OAuth token was not frozen into the private runtime snapshot")
	}
	if provider.APIKey != "" {
		t.Fatal("telephone OAuth resolution mutated the provider configuration")
	}
}

func TestTelephoneBackendSnapshotRejectsExpiredOAuth(t *testing.T) {
	cfg := telephoneAgentTestConfig(t)
	provider := cfg.FindProvider("phone-agent")
	provider.AuthType = "oauth2"
	provider.APIKey = ""
	vault, err := security.NewVault(strings.Repeat("8", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	if err := vault.WriteSecret("oauth_phone-agent", `{"access_token":"expired","expiry":"`+expiry+`"}`); err != nil {
		t.Fatal(err)
	}
	runner := NewVoiceActionRunner(&Server{Cfg: cfg, Vault: vault})
	if _, err := runner.buildTelephoneBackendSnapshot(context.Background(), cfg, cfg.SIP.Voice); err == nil || !strings.Contains(err.Error(), "expired_oauth") {
		t.Fatalf("expired telephone OAuth token error = %v", err)
	}
}

func TestTelephoneBackendSnapshotAcceptsSupportedKeylessProvider(t *testing.T) {
	cfg := telephoneAgentTestConfig(t)
	provider := cfg.FindProvider("phone-agent")
	provider.Type = "ollama"
	provider.BaseURL = "http://127.0.0.1:11434/v1"
	provider.APIKey = ""
	provider.Model = "qwen3:8b"
	runner := NewVoiceActionRunner(&Server{Cfg: cfg})
	snapshot, err := runner.buildTelephoneBackendSnapshot(context.Background(), cfg, cfg.SIP.Voice)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.agent.config.LLM.ProviderType != "ollama" || snapshot.agent.config.LLM.APIKey != "" {
		t.Fatalf("keyless provider snapshot = %+v", snapshot.agent.config.LLM)
	}
}
