package server

import (
	"encoding/json"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/invasion"

	"gopkg.in/yaml.v3"
)

func TestEggSavedProviderBootstrapUsesMatchingVaultKey(t *testing.T) {
	s := newSharedKeyTestServer(t, "")
	manual := false
	cfg := &config.Config{Providers: []config.ProviderEntry{{ID: "worker-model", Type: "openrouter", BaseURL: "https://fixture.invalid/v1", Model: "saved-model", Capabilities: config.ProviderCapabilities{Auto: &manual, ToolCalling: true}}}}
	egg := invasion.EggRecord{Provider: "worker-model", Model: "override-model"}
	if err := s.Vault.WriteSecret("provider_worker-model_api_key", "fixture-provider-key"); err != nil {
		t.Fatal(err)
	}
	for _, override := range []bool{false, true} {
		want := "fixture-provider-key"
		if override {
			egg.APIKeyRef = "explicit-egg-key"
			want = "fixture-explicit-key"
			if err := s.Vault.WriteSecret(egg.APIKeyRef, want); err != nil {
				t.Fatal(err)
			}
		}
		data, err := s.eggBootstrap(cfg, egg, "", "", nil)
		if err != nil {
			t.Fatal(err)
		}
		var bootstrap invasion.EggBootstrap
		if err := json.Unmarshal(data, &bootstrap); err != nil {
			t.Fatal(err)
		}
		if bootstrap.Secrets["provider_main_api_key"] != want {
			t.Fatal("wrong provider key in private bootstrap")
		}
		generated, err := invasion.GenerateEggConfig(cfg, egg, invasion.NestRecord{}, "", "wss://fixture.invalid", "")
		if err != nil {
			t.Fatal(err)
		}
		var parsed struct {
			Providers []config.ProviderEntry `yaml:"providers"`
		}
		if err := yaml.Unmarshal(generated, &parsed); err != nil {
			t.Fatal(err)
		}
		p := parsed.Providers[0]
		if p.Type != "openrouter" || p.ID != "main" || p.Model != "override-model" || p.BaseURL != "https://fixture.invalid/v1" || !p.Capabilities.ToolCalling {
			t.Fatal("saved provider metadata lost")
		}
		if strings.Contains(string(generated), want) || strings.Contains(string(generated), "api_key:") {
			t.Fatal("provider key leaked into YAML")
		}
	}
	cfg.Providers[0].AuthType = "oauth2"
	if _, err := s.eggBootstrap(cfg, egg, "", "", nil); err == nil {
		t.Fatal("OAuth session accepted for static bootstrap")
	}
	cfg.Providers = []config.ProviderEntry{{ID: "keyless", Type: "ollama", BaseURL: "https://fixture.invalid"}}
	egg.Provider, egg.APIKeyRef = "keyless", ""
	for _, noVault := range []bool{false, true} {
		if noVault {
			s.Vault = nil
		}
		if _, err := s.eggBootstrap(cfg, egg, "", "", nil); err != nil {
			t.Fatalf("keyless provider rejected: %v", err)
		}
	}
}
