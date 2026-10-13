package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDecisionsConsentPersistenceAndSafeLegacyDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  port: 8088\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Decisions.Enabled || cfg.Decisions.CPUFallbackAllowed || cfg.Decisions.Backend != DecisionsBackendLocal || cfg.Decisions.Deployment != "auto" {
		t.Fatalf("unsafe legacy defaults: %+v", cfg.Decisions)
	}
	for _, allowed := range []bool{true, false} {
		cfg.Decisions = DecisionsConfig{Enabled: true, Backend: DecisionsBackendLocal, Deployment: "native", CPUFallbackAllowed: allowed}
		if err := cfg.Save(path); err != nil {
			t.Fatal(err)
		}
		reloaded, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if reloaded.Decisions != cfg.Decisions {
			t.Fatalf("permission changed across restart: %+v", reloaded.Decisions)
		}
		cfg = reloaded
	}
	if err := os.WriteFile(path, []byte("decisions:\n  deployment: remote\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("unsupported deployment accepted")
	}
}

func TestDecisionsProviderConfigRoundTripsAndValidatesOnLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	legacy := `providers:
  - id: openrouter
    name: OpenRouter
    type: openrouter
    base_url: https://openrouter.ai/api/v1
    model: ""
decisions:
  enabled: true
  backend: provider
  provider: openrouter
  model: meta-llama/llama-4-scout
`
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load(legacy provider config) error = %v", err)
	}
	if cfg.Decisions.Backend != DecisionsBackendProvider || cfg.Decisions.Provider != "openrouter" || cfg.Decisions.Model != "meta-llama/llama-4-scout" {
		t.Fatalf("provider-backed Decisions config was not loaded: %+v", cfg.Decisions)
	}
	if cfg.Providers[0].Usage != "" {
		t.Fatalf("legacy provider usage changed: %+v", cfg.Providers[0])
	}
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load(saved config) error = %v", err)
	}
	if reloaded.Decisions != cfg.Decisions {
		t.Fatalf("provider-backed config changed across save: got %+v, want %+v", reloaded.Decisions, cfg.Decisions)
	}
	if reloaded.Providers[0].Usage != "" {
		t.Fatalf("legacy general provider did not retain empty usage: %+v", reloaded.Providers[0])
	}
}

func TestValidateDecisionsProvider(t *testing.T) {
	valid := func() *Config {
		return &Config{
			Decisions: DecisionsConfig{
				Enabled: true, Backend: DecisionsBackendProvider, Provider: "router", Model: "vendor/model",
			},
			Providers: []ProviderEntry{{
				ID: "router", Type: "openrouter", BaseURL: "https://openrouter.ai/api/v1",
			}},
		}
	}
	for _, tc := range []struct {
		name   string
		mutate func(*Config)
	}{{
		name:   "missing provider selection",
		mutate: func(cfg *Config) { cfg.Decisions.Provider = "" },
	}, {
		name:   "missing model",
		mutate: func(cfg *Config) { cfg.Decisions.Model = " " },
	}, {
		name:   "unknown provider reference",
		mutate: func(cfg *Config) { cfg.Decisions.Provider = "missing" },
	}, {
		name:   "non OpenRouter provider",
		mutate: func(cfg *Config) { cfg.Providers[0].Type = "openai" },
	}, {
		name:   "non canonical OpenRouter endpoint",
		mutate: func(cfg *Config) { cfg.Providers[0].BaseURL = "https://router.example/api/v1" },
	}, {
		name: "decisions-only provider selected by ordinary LLM",
		mutate: func(cfg *Config) {
			cfg.Providers[0].Usage = ProviderUsageDecisions
			cfg.LLM.Provider = "router"
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid()
			tc.mutate(cfg)
			if err := ValidateDecisionsProvider(cfg); err == nil {
				t.Fatal("invalid provider config was accepted")
			}
		})
	}

	for _, baseURL := range []string{"", "HTTPS://openrouter.ai", "https://openrouter.ai", "https://openrouter.ai/api/v1", "https://openrouter.ai/api/v1/"} {
		t.Run("accepted URL "+baseURL, func(t *testing.T) {
			cfg := valid()
			cfg.Providers[0].BaseURL = baseURL
			if err := ValidateDecisionsProvider(cfg); err != nil {
				t.Fatalf("valid provider config rejected: %v", err)
			}
		})
	}
	for _, baseURL := range []string{"https://OPENROUTER.ai", "https://openrouter.ai:443", "https://openrouter.ai/other"} {
		t.Run("rejected URL "+baseURL, func(t *testing.T) {
			cfg := valid()
			cfg.Providers[0].BaseURL = baseURL
			if err := ValidateDecisionsProvider(cfg); err == nil {
				t.Fatalf("noncanonical OpenRouter URL %q was accepted", baseURL)
			}
		})
	}

	disabledDraft := &Config{Decisions: DecisionsConfig{
		Enabled: false, Backend: DecisionsBackendProvider, Provider: "missing", Model: "",
	}}
	if err := ValidateDecisionsProvider(disabledDraft); err != nil {
		t.Fatalf("disabled incomplete draft blocked an unrelated save: %v", err)
	}

	normalized := (DecisionsConfig{Provider: " router ", Model: " model "}).Normalize()
	if normalized.Provider != "router" || normalized.Model != "model" {
		t.Fatalf("provider/model whitespace was not normalized: %+v", normalized)
	}
}

func TestValidateProviderEntryUsage(t *testing.T) {
	for _, tc := range []struct {
		name    string
		entry   ProviderEntry
		wantErr bool
	}{{
		name:  "legacy empty usage",
		entry: ProviderEntry{ID: "legacy", Type: "openai"},
	}, {
		name:  "general usage",
		entry: ProviderEntry{ID: "general", Type: "openai", Usage: ProviderUsageGeneral},
	}, {
		name:  "decisions OpenRouter",
		entry: ProviderEntry{ID: "decision", Type: "openrouter", Usage: ProviderUsageDecisions},
	}, {
		name:  "decisions non OpenRouter",
		entry: ProviderEntry{ID: "decision", Type: "openai", Usage: ProviderUsageDecisions}, wantErr: true,
	}, {
		name:  "unknown usage",
		entry: ProviderEntry{ID: "unknown", Type: "openrouter", Usage: "speech"}, wantErr: true,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateProviderEntryUsage(tc.entry)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateProviderEntryUsage(%+v) error = %v; wantErr %v", tc.entry, err, tc.wantErr)
			}
		})
	}
}
