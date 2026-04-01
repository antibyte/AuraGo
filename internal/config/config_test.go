package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type testSecretVault struct {
	data map[string]string
}

func (v *testSecretVault) ReadSecret(key string) (string, error) {
	return v.data[key], nil
}

func (v *testSecretVault) WriteSecret(key, value string) error {
	if v.data == nil {
		v.data = map[string]string{}
	}
	v.data[key] = value
	return nil
}

func TestLoadAbsolutePaths(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	workspacePath := "/tmp/workspace"
	if os.PathSeparator == '\\' {
		workspacePath = "C:\\absolute\\path\\workspace"
	}

	configContent := `
directories:
  data_dir: './data'
  workspace_dir: '` + workspacePath + `'
  skills_dir: '../skills'
sqlite:
  short_term_path: './data/short_term.db'
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Calculate expected paths
	absConfigDir, _ := filepath.Abs(tmpDir)
	expectedDataDir := filepath.Join(absConfigDir, "./data")
	expectedWorkspaceDir := workspacePath
	expectedSkillsDir := filepath.Join(absConfigDir, "../skills")
	expectedShortTermPath := filepath.Join(absConfigDir, "./data/short_term.db")

	if cfg.Directories.DataDir != expectedDataDir {
		t.Errorf("expected DataDir %s, got %s", expectedDataDir, cfg.Directories.DataDir)
	}
	if cfg.Directories.WorkspaceDir != expectedWorkspaceDir {
		t.Errorf("expected WorkspaceDir %s, got %s", expectedWorkspaceDir, cfg.Directories.WorkspaceDir)
	}
	if cfg.Directories.SkillsDir != expectedSkillsDir {
		t.Errorf("expected SkillsDir %s, got %s", expectedSkillsDir, cfg.Directories.SkillsDir)
	}
	if cfg.SQLite.ShortTermPath != expectedShortTermPath {
		t.Errorf("expected ShortTermPath %s, got %s", expectedShortTermPath, cfg.SQLite.ShortTermPath)
	}
}

func TestGetSpecialist(t *testing.T) {
	cfg := &Config{}
	cfg.CoAgents.Specialists.Coder.Enabled = true

	tests := []struct {
		role    string
		wantNil bool
	}{
		{"researcher", false},
		{"coder", false},
		{"designer", false},
		{"security", false},
		{"writer", false},
		{"unknown", true},
		{"", true},
	}
	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			got := cfg.GetSpecialist(tt.role)
			if (got == nil) != tt.wantNil {
				t.Errorf("GetSpecialist(%q) nil=%v, wantNil=%v", tt.role, got == nil, tt.wantNil)
			}
		})
	}

	// Verify the coder specialist we set is actually enabled
	coder := cfg.GetSpecialist("coder")
	if coder == nil || !coder.Enabled {
		t.Error("expected coder specialist to be enabled")
	}
}

func TestValidSpecialistRoles(t *testing.T) {
	expected := []string{"researcher", "coder", "designer", "security", "writer"}
	for _, role := range expected {
		if !ValidSpecialistRoles[role] {
			t.Errorf("expected %q in ValidSpecialistRoles", role)
		}
	}
	if ValidSpecialistRoles["unknown"] {
		t.Error("unexpected role 'unknown' in ValidSpecialistRoles")
	}
}

func TestLoadUpgradesLegacyIndexingExtensions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
indexing:
  enabled: true
  directories:
    - ./knowledge
  extensions:
    - .txt
    - .md
    - .json
    - .csv
    - .log
    - .yaml
    - .yml
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	for _, want := range []string{".pdf", ".docx", ".xlsx", ".pptx", ".odt", ".rtf"} {
		if !slices.Contains(cfg.Indexing.Extensions, want) {
			t.Fatalf("expected upgraded indexing extensions to include %s, got %v", want, cfg.Indexing.Extensions)
		}
	}
}

func TestLoadKeepsCustomIndexingExtensions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
indexing:
  enabled: true
  directories:
    - ./knowledge
  extensions:
    - .txt
    - .md
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(cfg.Indexing.Extensions) != 2 || cfg.Indexing.Extensions[0] != ".txt" || cfg.Indexing.Extensions[1] != ".md" {
		t.Fatalf("expected custom indexing extensions to stay unchanged, got %v", cfg.Indexing.Extensions)
	}
}

func TestLoadAdaptiveSystemPromptTokenBudgetDefaultsToTrue(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
agent:
  system_prompt_token_budget: 12000
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if !cfg.Agent.AdaptiveSystemPromptTokenBudget {
		t.Fatal("expected adaptive_system_prompt_token_budget to default to true")
	}
}

func TestLoadAdaptiveSystemPromptTokenBudgetExplicitDisablePreserved(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
agent:
  system_prompt_token_budget: 12000
  adaptive_system_prompt_token_budget: false
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Agent.AdaptiveSystemPromptTokenBudget {
		t.Fatal("expected explicit adaptive_system_prompt_token_budget=false to be preserved")
	}
}

func TestMigrateEggModeSharedKeyToVault(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
egg_mode:
  enabled: true
  master_url: ws://master.local/ws
  shared_key: deadbeef
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	vault := &testSecretVault{data: map[string]string{}}

	MigrateEggModeSharedKeyToVault(configPath, vault, slog.Default())

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	cfg.ApplyVaultSecrets(vault)

	if cfg.EggMode.SharedKey != "deadbeef" {
		t.Fatalf("shared key = %q, want deadbeef", cfg.EggMode.SharedKey)
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config after migration: %v", err)
	}
	if strings.Contains(string(raw), "shared_key:") {
		t.Fatalf("expected shared_key to be removed from config.yaml, got:\n%s", string(raw))
	}
}

func TestConfigSaveWritesUpdatedField(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
server:
  ui_language: en
auth:
  enabled: false
personality:
  core_personality: neutral
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	cfg.Server.UILanguage = "de"
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !strings.Contains(string(raw), "ui_language: de") {
		t.Fatalf("expected saved config to contain updated ui_language, got:\n%s", string(raw))
	}
}

func TestConfigSavePersistsOutgoingWebhooks(t *testing.T) {
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
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	cfg.Webhooks.Outgoing = []OutgoingWebhook{{
		ID:          "hook_1",
		Name:        "Deploy",
		Description: "Trigger deploy",
		Method:      "POST",
		URL:         "https://example.test/deploy",
		Headers:     map[string]string{"X-Test": "1"},
		Parameters: []WebhookParameter{{
			Name:        "service",
			Type:        "string",
			Description: "Service name",
			Required:    true,
		}},
		PayloadType: "json",
	}}

	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reloaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if len(reloaded.Webhooks.Outgoing) != 1 {
		t.Fatalf("outgoing webhook count = %d, want 1", len(reloaded.Webhooks.Outgoing))
	}
	if reloaded.Webhooks.Outgoing[0].URL != "https://example.test/deploy" {
		t.Fatalf("saved webhook url = %q", reloaded.Webhooks.Outgoing[0].URL)
	}
}

func TestLoadResolvesHelperLLMFromProvider(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
providers:
  - id: main
    type: openrouter
    base_url: https://openrouter.ai/api/v1
    api_key: main-secret
    model: main-model
  - id: helper
    type: openai
    base_url: https://helper.example/v1
    api_key: helper-secret
    model: helper-default
llm:
  provider: main
  helper_enabled: true
  helper_provider: helper
  helper_model: helper-override
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.LLM.HelperEnabled {
		t.Fatal("expected llm.helper_enabled=true to be preserved")
	}
	if cfg.LLM.HelperProviderType != "openai" {
		t.Fatalf("helper provider type = %q, want openai", cfg.LLM.HelperProviderType)
	}
	if cfg.LLM.HelperBaseURL != "https://helper.example/v1" {
		t.Fatalf("helper base url = %q", cfg.LLM.HelperBaseURL)
	}
	if cfg.LLM.HelperAPIKey != "" {
		t.Fatalf("helper api key = %q, want empty until vault/env resolution", cfg.LLM.HelperAPIKey)
	}
	if cfg.LLM.HelperResolvedModel != "helper-override" {
		t.Fatalf("helper resolved model = %q, want helper-override", cfg.LLM.HelperResolvedModel)
	}
}

func TestLoadDoesNotFallbackHelperLLMToMainProvider(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
providers:
  - id: main
    type: openrouter
    base_url: https://openrouter.ai/api/v1
    api_key: main-secret
    model: main-model
llm:
  provider: main
  helper_enabled: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.LLM.HelperEnabled {
		t.Fatal("expected llm.helper_enabled=true to be preserved")
	}
	if cfg.LLM.HelperProviderType != "" {
		t.Fatalf("helper provider type = %q, want empty", cfg.LLM.HelperProviderType)
	}
	if cfg.LLM.HelperBaseURL != "" {
		t.Fatalf("helper base url = %q, want empty", cfg.LLM.HelperBaseURL)
	}
	if cfg.LLM.HelperAPIKey != "" {
		t.Fatalf("helper api key = %q, want empty", cfg.LLM.HelperAPIKey)
	}
	if cfg.LLM.HelperResolvedModel != "" {
		t.Fatalf("helper resolved model = %q, want empty", cfg.LLM.HelperResolvedModel)
	}
}
