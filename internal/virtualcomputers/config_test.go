package virtualcomputers

import (
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

func TestFromAuraConfigMapsStorageAndLedgerPath(t *testing.T) {
	cfg := &config.Config{}
	cfg.SQLite.VirtualComputersPath = "data/vc.db"
	cfg.VirtualComputers.Storage.Endpoint = "minio.home:9000"
	cfg.VirtualComputers.Storage.Bucket = "vc-data"
	cfg.VirtualComputers.Storage.Region = "home-1"
	cfg.VirtualComputers.Storage.UseSSL = true

	got := FromAuraConfig(cfg)
	if got.LedgerPath != "data/vc.db" || got.Storage.Mode != StorageModeExternalS3 || got.Storage.Endpoint != "minio.home:9000" || got.Storage.Bucket != "vc-data" || got.Storage.Region != "home-1" || !got.Storage.UseSSL {
		t.Fatalf("tool config = %+v", got)
	}
}

func TestFromAuraConfigRegistersStorageCredentialsAsSensitive(t *testing.T) {
	cfg := &config.Config{}
	cfg.VirtualComputers.S3AccessKeyID = "storage-access-sensitive"
	cfg.VirtualComputers.S3SecretKey = "storage-secret-sensitive"
	_ = FromAuraConfig(cfg)
	redacted := security.Scrub("storage-access-sensitive storage-secret-sensitive")
	if strings.Contains(redacted, "storage-access-sensitive") || strings.Contains(redacted, "storage-secret-sensitive") {
		t.Fatalf("storage credentials were not registered as sensitive: %s", redacted)
	}
}

func TestFromAuraConfigResolvesSelectedAnthropicProvider(t *testing.T) {
	cfg := &config.Config{
		Providers: []config.ProviderEntry{{
			ID: "agent-claude", Type: "anthropic", BaseURL: "https://api.anthropic.com/v1", APIKey: "provider-anthropic-key",
		}},
	}
	cfg.VirtualComputers.AllowAgentTasks = true
	cfg.VirtualComputers.AgentProvider = "agent-claude"

	got := FromAuraConfig(cfg)
	if got.BoringAnthropicKey != "provider-anthropic-key" {
		t.Fatalf("resolved Anthropic key = %q", got.BoringAnthropicKey)
	}
	if strings.Contains(security.Scrub("provider-anthropic-key"), "provider-anthropic-key") {
		t.Fatal("resolved provider key was not registered as sensitive")
	}
}

func TestFromAuraConfigDoesNotExposeAgentProviderWithoutOptIn(t *testing.T) {
	cfg := &config.Config{
		Providers: []config.ProviderEntry{{ID: "agent-claude", Type: "anthropic", APIKey: "provider-key"}},
	}
	cfg.VirtualComputers.AgentProvider = "agent-claude"
	cfg.VirtualComputers.BoringAnthropicKey = "legacy-key"
	cfg.VirtualComputers.BoringOpenRouterKey = "legacy-openrouter-key"

	got := FromAuraConfig(cfg)
	if got.BoringAnthropicKey != "" || got.BoringOpenRouterKey != "" {
		t.Fatalf("agent credentials exposed without opt-in: %+v", got)
	}
}

func TestFromAuraConfigRejectsIncompatibleAgentProvider(t *testing.T) {
	for _, provider := range []config.ProviderEntry{
		{ID: "router", Type: "openrouter", APIKey: "router-key"},
		{ID: "kimi", Type: "anthropic", BaseURL: "https://api.kimi.com/coding/v1", APIKey: "kimi-key"},
		{ID: "oauth", Type: "anthropic", BaseURL: "https://api.anthropic.com/v1", AuthType: "oauth2", APIKey: "oauth-token"},
		{ID: "userinfo", Type: "anthropic", BaseURL: "https://user@api.anthropic.com/v1", APIKey: "provider-key"},
	} {
		t.Run(provider.ID, func(t *testing.T) {
			cfg := &config.Config{Providers: []config.ProviderEntry{provider}}
			cfg.VirtualComputers.AllowAgentTasks = true
			cfg.VirtualComputers.AgentProvider = provider.ID

			got := FromAuraConfig(cfg)
			if got.BoringAnthropicKey != "" || got.BoringOpenRouterKey != "" {
				t.Fatalf("incompatible provider credentials exposed: %+v", got)
			}
		})
	}
}

func TestFromAuraConfigKeepsLegacyAgentCredentialFallback(t *testing.T) {
	cfg := &config.Config{}
	cfg.VirtualComputers.AllowAgentTasks = true
	cfg.VirtualComputers.BoringAnthropicKey = "legacy-anthropic-key"
	cfg.VirtualComputers.BoringOpenRouterKey = "legacy-openrouter-key"

	got := FromAuraConfig(cfg)
	if got.BoringAnthropicKey != "legacy-anthropic-key" || got.BoringOpenRouterKey != "legacy-openrouter-key" {
		t.Fatalf("legacy agent credentials = %+v", got)
	}
}
