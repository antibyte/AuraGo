package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVirtualComputersDefaults(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.VirtualComputers.Provider != "boring_computers" {
		t.Fatalf("provider = %q", cfg.VirtualComputers.Provider)
	}
	if cfg.VirtualComputers.DefaultTTLSeconds != 600 {
		t.Fatalf("default ttl = %d", cfg.VirtualComputers.DefaultTTLSeconds)
	}
	if cfg.VirtualComputers.MaxTTLSeconds != 900 {
		t.Fatalf("max ttl = %d", cfg.VirtualComputers.MaxTTLSeconds)
	}
	if cfg.VirtualComputers.ControlPlane.Mode != "ssh_host" {
		t.Fatalf("control plane mode = %q", cfg.VirtualComputers.ControlPlane.Mode)
	}
	if cfg.VirtualComputers.ControlPlane.BoringdURL != "http://127.0.0.1:18082" {
		t.Fatalf("boringd url = %q", cfg.VirtualComputers.ControlPlane.BoringdURL)
	}
	if cfg.VirtualComputers.Storage.Bucket != "boring-volumes" || !cfg.VirtualComputers.Storage.UseSSL {
		t.Fatalf("storage defaults = %+v", cfg.VirtualComputers.Storage)
	}
	wantDB := filepath.Join(filepath.Dir(configPath), "data", "virtual_computers.db")
	if cfg.SQLite.VirtualComputersPath != wantDB {
		t.Fatalf("virtual computers db = %q, want %q", cfg.SQLite.VirtualComputersPath, wantDB)
	}
}

func TestVirtualComputersStorageConfigLoadsNonSecrets(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte("virtual_computers:\n  storage:\n    endpoint: minio.home:9000\n    bucket: vc-data\n    region: home-1\n    use_ssl: false\n")
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	storage := cfg.VirtualComputers.Storage
	if storage.Endpoint != "minio.home:9000" || storage.Bucket != "vc-data" || storage.Region != "home-1" || storage.UseSSL {
		t.Fatalf("storage = %+v", storage)
	}
}

func TestVirtualComputersAgentProviderLoadsAsNonSecretReference(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte("virtual_computers:\n  allow_agent_tasks: true\n  agent_provider: anthropic-main\n")
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.VirtualComputers.AllowAgentTasks || cfg.VirtualComputers.AgentProvider != "anthropic-main" {
		t.Fatalf("agent settings = %+v", cfg.VirtualComputers)
	}
}

func TestVirtualComputersLegacyBoringdURLsMigrate(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "homepage port", url: "http://127.0.0.1:8080", want: "http://127.0.0.1:18082"},
		{name: "internal loopback port", url: "http://127.0.0.1:18080", want: "http://127.0.0.1:18082"},
		{name: "custom port", url: "http://127.0.0.1:19000", want: "http://127.0.0.1:19000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "config.yaml")
			data := []byte("virtual_computers:\n  control_plane:\n    boringd_url: " + tt.url + "\n")
			if err := os.WriteFile(configPath, data, 0o644); err != nil {
				t.Fatalf("write config: %v", err)
			}
			cfg, err := Load(configPath)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := cfg.VirtualComputers.ControlPlane.BoringdURL; got != tt.want {
				t.Fatalf("boringd url = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestApplyVaultSecretsLoadsVirtualComputersSecrets(t *testing.T) {
	cfg := &Config{}
	cfg.ApplyVaultSecrets(&testSecretVault{data: map[string]string{
		"virtual_computers_boring_token":     "boring-token",
		"virtual_computers_ssh_secret":       "ssh-secret",
		"virtual_computers_anthropic_key":    "anthropic-key",
		"virtual_computers_openrouter_key":   "openrouter-key",
		"virtual_computers_s3_access_key_id": "s3-id",
		"virtual_computers_s3_secret_key":    "s3-secret",
	}})
	if cfg.VirtualComputers.BoringToken != "boring-token" {
		t.Fatalf("boring token not loaded")
	}
	if cfg.VirtualComputers.ControlPlane.SSHSecret != "ssh-secret" {
		t.Fatalf("ssh secret not loaded")
	}
	if cfg.VirtualComputers.BoringAnthropicKey != "anthropic-key" {
		t.Fatalf("anthropic key not loaded")
	}
	if cfg.VirtualComputers.BoringOpenRouterKey != "openrouter-key" {
		t.Fatalf("openrouter key not loaded")
	}
	if cfg.VirtualComputers.S3AccessKeyID != "s3-id" || cfg.VirtualComputers.S3SecretKey != "s3-secret" {
		t.Fatalf("s3 secrets not loaded")
	}
}
