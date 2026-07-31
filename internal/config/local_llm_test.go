package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func validLocalLLMTestConfig() *Config {
	cfg := &Config{}
	cfg.LocalLLM = LocalLLMConfig{
		Enabled:            true,
		Backend:            "auto",
		ModelVariant:       "q4_k_m",
		MTP:                "off",
		ContextSize:        16384,
		IdleTimeoutMinutes: 15,
		ListenPort:         18081,
	}
	return cfg
}

func TestValidateLocalLLMConfigContextSizes(t *testing.T) {
	for _, size := range []int{16384, 32768} {
		cfg := validLocalLLMTestConfig()
		cfg.LocalLLM.ContextSize = size
		if err := ValidateLocalLLMConfig(cfg); err != nil {
			t.Fatalf("context size %d rejected: %v", size, err)
		}
	}
	for _, size := range []int{2048, 8192} {
		cfg := validLocalLLMTestConfig()
		cfg.LocalLLM.ContextSize = size
		if err := ValidateLocalLLMConfig(cfg); err == nil {
			t.Fatalf("obsolete context size %d was accepted", size)
		}
	}
}

func TestLoadMigratesObsoleteLocalLLMContextSizes(t *testing.T) {
	for _, size := range []int{2048, 8192} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "config.yaml")
			payload := []byte("local_llm:\n  context_size: " + fmt.Sprint(size) + "\n")
			if err := os.WriteFile(configPath, payload, 0o600); err != nil {
				t.Fatalf("write legacy config: %v", err)
			}
			cfg, err := Load(configPath)
			if err != nil {
				t.Fatalf("Load legacy config: %v", err)
			}
			if cfg.LocalLLM.ContextSize != 16384 {
				t.Fatalf("legacy context size %d migrated to %d, want 16384", size, cfg.LocalLLM.ContextSize)
			}
		})
	}
}

func TestValidateLocalLLMConfigRejectsReservedProviderEntry(t *testing.T) {
	cfg := validLocalLLMTestConfig()
	cfg.Providers = []ProviderEntry{{ID: LocalLLMProviderID, Type: "llamacpp"}}
	if err := ValidateLocalLLMConfig(cfg); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("ValidateLocalLLMConfig() error = %v, want reserved provider error", err)
	}
}

func TestValidateLocalLLMConfigRoutingRules(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*Config)
		wantError string
	}{
		{
			name: "disabled while referenced",
			configure: func(cfg *Config) {
				cfg.LocalLLM.Enabled = false
				cfg.FallbackLLM.Enabled = true
				cfg.FallbackLLM.Provider = LocalLLMProviderID
			},
			wantError: "cannot be disabled",
		},
		{
			name: "disabled after local fallback is detached",
			configure: func(cfg *Config) {
				cfg.LocalLLM.Enabled = false
				cfg.LLM.Provider = "cloud"
				cfg.FallbackLLM.Enabled = false
				cfg.FallbackLLM.Provider = ""
				cfg.Providers = []ProviderEntry{{ID: "cloud", Type: "openai"}}
			},
		},
		{
			name: "primary without regular fallback",
			configure: func(cfg *Config) {
				cfg.LLM.Provider = LocalLLMProviderID
			},
			wantError: "requires one regular fallback",
		},
		{
			name: "primary with regular fallback",
			configure: func(cfg *Config) {
				cfg.LLM.Provider = LocalLLMProviderID
				cfg.FallbackLLM.Enabled = true
				cfg.FallbackLLM.Provider = "cloud"
				cfg.Providers = []ProviderEntry{{ID: "cloud", Type: "openai"}}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validLocalLLMTestConfig()
			tt.configure(cfg)
			err := ValidateLocalLLMConfig(cfg)
			if tt.wantError == "" && err != nil {
				t.Fatalf("ValidateLocalLLMConfig() unexpected error: %v", err)
			}
			if tt.wantError != "" && (err == nil || !strings.Contains(err.Error(), tt.wantError)) {
				t.Fatalf("ValidateLocalLLMConfig() error = %v, want %q", err, tt.wantError)
			}
		})
	}
}

func TestResolveProvidersSynthesizesLocalProvider(t *testing.T) {
	cfg := validLocalLLMTestConfig()
	cfg.LocalLLM.RuntimeAPIKey = "runtime-secret"
	cfg.FallbackLLM.Enabled = true
	cfg.FallbackLLM.Provider = LocalLLMProviderID
	cfg.ResolveProviders()

	if cfg.FallbackLLM.ProviderType != "aurago-local" ||
		cfg.FallbackLLM.Model != LocalLLMModelAlias ||
		cfg.FallbackLLM.APIKey != "runtime-secret" {
		t.Fatalf("resolved fallback = %#v", cfg.FallbackLLM)
	}
	if cfg.FindProvider(LocalLLMProviderID) != nil {
		t.Fatal("reserved local provider must not be exposed as ProviderEntry")
	}
}

func TestLocalLLMRuntimeAPIKeyIsNeverSerialized(t *testing.T) {
	cfg := validLocalLLMTestConfig()
	cfg.LocalLLM.RuntimeAPIKey = "must-not-leak"
	for name, marshal := range map[string]func(any) ([]byte, error){
		"json": json.Marshal,
		"yaml": yaml.Marshal,
	} {
		payload, err := marshal(cfg)
		if err != nil {
			t.Fatalf("%s marshal: %v", name, err)
		}
		if strings.Contains(string(payload), "must-not-leak") ||
			strings.Contains(string(payload), LocalLLMRuntimeAPIKeyVaultKey) {
			t.Fatalf("%s serialization leaked the local runtime key: %s", name, payload)
		}
	}
}
