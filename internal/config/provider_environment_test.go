package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func clearProviderEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{"LLM_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "CO_AGENTS_LLM_API_KEY", "EMBEDDINGS_API_KEY", "VISION_API_KEY", "WHISPER_API_KEY", "FALLBACK_LLM_API_KEY"} {
		t.Setenv(name, "")
	}
}

func TestProviderEnvironmentOverridesSurviveVaultAndOAuthResolution(t *testing.T) {
	for _, fixture := range []struct {
		name string
		key  func(*Config) *string
	}{
		{"LLM_API_KEY", func(c *Config) *string { return &c.LLM.APIKey }},
		{"CO_AGENTS_LLM_API_KEY", func(c *Config) *string { return &c.CoAgents.LLM.APIKey }},
		{"EMBEDDINGS_API_KEY", func(c *Config) *string { return &c.Embeddings.APIKey }},
		{"VISION_API_KEY", func(c *Config) *string { return &c.Vision.APIKey }},
		{"WHISPER_API_KEY", func(c *Config) *string { return &c.Whisper.APIKey }},
		{"FALLBACK_LLM_API_KEY", func(c *Config) *string { return &c.FallbackLLM.APIKey }},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			clearProviderEnvironment(t)
			t.Setenv(fixture.name, "environmentFixture123456")
			path := filepath.Join(t.TempDir(), "config.yaml")
			yaml := "providers:\n  - id: main\n    type: openai\n    base_url: https://provider.example.invalid/v1\n    model: fixture\nllm:\n  provider: main\n  helper_enabled: true\n  helper_provider: main\nfallback_llm:\n  provider: main\nco_agents:\n  llm:\n    provider: main\nvision:\n  provider: main\nwhisper:\n  provider: main\nembeddings:\n  provider: main\n"
			if err := os.WriteFile(path, []byte(yaml), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			vault := &testSecretVault{data: map[string]string{"provider_main_api_key": "vaultFixture123456"}}
			cfg.ApplyVaultSecrets(vault)
			for i := 0; i < 2; i++ {
				cfg.ResolveProviders()
				cfg.ApplyOAuthTokens(vault)
				if *fixture.key(cfg) != "environmentFixture123456" {
					t.Fatal("environment override lost during runtime resolution")
				}
				for _, binding := range cfg.providerCredentialBindings() {
					if binding.id == "main" && binding.key != fixture.key(cfg) && *binding.key != "vaultFixture123456" {
						t.Fatal("slot override leaked into another provider binding")
					}
				}
			}
			if cfg.Providers[0].APIKey != "vaultFixture123456" {
				t.Fatal("slot override changed provider credentials")
			}
			if err := cfg.Save(path); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil || strings.Contains(string(data), "environmentFixture") || strings.Contains(string(data), "vaultFixture") {
				t.Fatal("runtime credentials were persisted")
			}
		})
	}
}

func TestProviderEnvironmentPrimaryPrecedence(t *testing.T) {
	clearProviderEnvironment(t)
	cfg := &Config{Providers: []ProviderEntry{{ID: "main", Type: "openai", APIKey: "vaultFixture"}}}
	cfg.LLM.Provider = "main"
	for _, name := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "LLM_API_KEY"} {
		t.Setenv(name, name)
		cfg.ResolveProviders()
		if cfg.LLM.APIKey != name {
			t.Fatalf("primary precedence failed for %s", name)
		}
	}
}

func TestProviderEnvironmentPreservesProtectedBindings(t *testing.T) {
	for _, kind := range []string{"oauth", "oauth_missing", "masked", "missing", "managed", "builtin_embeddings"} {
		t.Run(kind, func(t *testing.T) {
			clearProviderEnvironment(t)
			for _, name := range []string{"LLM_API_KEY", "EMBEDDINGS_API_KEY"} {
				t.Setenv(name, "environmentFixture123456")
			}
			cfg := &Config{Providers: []ProviderEntry{{ID: "main", Type: "openai", APIKey: "vaultFixture123456"}}}
			cfg.LLM.Provider = "main"
			vault := &testSecretVault{data: map[string]string{}}
			want := ""
			switch kind {
			case "oauth", "oauth_missing":
				cfg.Providers[0].AuthType = "oauth2"
				if kind == "oauth" {
					vault.data["oauth_main"] = `{"access_token":"oauthFixture123456"}`
					want = "oauthFixture123456"
				}
			case "masked":
				cfg.Providers[0].APIKey = "••••••••"
				want = "••••••••"
			case "missing":
				cfg.LLM.Provider = "missing"
			case "managed":
				cfg.LLM.Provider = LocalLLMProviderID
				cfg.LocalLLM.RuntimeAPIKey = "managedFixture123456"
				want = "managedFixture123456"
			case "builtin_embeddings":
				cfg.Embeddings.Provider = "local-granite"
			}
			cfg.ResolveProviders()
			cfg.ApplyOAuthTokens(vault)
			if kind == "builtin_embeddings" {
				if cfg.Embeddings.APIKey == "environmentFixture123456" {
					t.Fatal("environment override replaced builtin embedding credentials")
				}
			} else if cfg.LLM.APIKey != want {
				t.Fatal("environment override bypassed protected provider state")
			}
		})
	}
}

func TestPersistOutgoingWebhooksResolvesSectionProviderCredentials(t *testing.T) {
	clearProviderEnvironment(t)
	for _, provider := range []string{"manifest", "omniroute"} {
		t.Run(provider, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			data := fmt.Sprintf("providers:\n  - id: selected\n    type: %s\n    model: auto\nllm:\n  provider: selected\n%s:\n  mode: external\n  external_base_url: https://provider.example.invalid/v1\n", provider, provider)
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			vault := &testSecretVault{data: map[string]string{provider + "_api_key": "sectionFixture123456"}}
			cfg.ApplyVaultSecrets(vault)
			cfg.ResolveProviders()
			cfg.ApplyOAuthTokens(vault)
			updated, err := PersistOutgoingWebhooks(path, cfg, nil, vault)
			if err != nil {
				t.Fatal(err)
			}
			if updated.LLM.APIKey != "sectionFixture123456" || updated.Providers[0].APIKey != "sectionFixture123456" {
				t.Fatal("webhook reload lost section-backed provider credentials")
			}
		})
	}
}
