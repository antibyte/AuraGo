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

func TestLoadRemoteControlDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.RemoteControl.DiscoveryPort != 8092 {
		t.Fatalf("discovery_port = %d, want 8092", cfg.RemoteControl.DiscoveryPort)
	}
	if cfg.RemoteControl.MaxFileSizeMB != 50 {
		t.Fatalf("max_file_size_mb = %d, want 50", cfg.RemoteControl.MaxFileSizeMB)
	}
	if !cfg.RemoteControl.AuditLog {
		t.Fatal("expected remote_control.audit_log to default to true")
	}
	if cfg.RemoteControl.ReadOnly {
		t.Fatal("expected remote_control.readonly to default to false")
	}
}

func TestLoadRemoteControlAuditLogExplicitFalsePreserved(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
remote_control:
  audit_log: false
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.RemoteControl.AuditLog {
		t.Fatal("expected explicit remote_control.audit_log=false to be preserved")
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

func TestLoadUptimeKumaDefaults(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.UptimeKuma.RequestTimeout != 15 {
		t.Fatalf("request_timeout = %d, want 15", cfg.UptimeKuma.RequestTimeout)
	}
	if cfg.UptimeKuma.PollIntervalSeconds != 30 {
		t.Fatalf("poll_interval_seconds = %d, want 30", cfg.UptimeKuma.PollIntervalSeconds)
	}
	if cfg.UptimeKuma.RelayToAgent {
		t.Fatal("expected relay_to_agent to default to false")
	}
}

func TestApplyVaultSecretsLoadsUptimeKumaAPIKey(t *testing.T) {
	cfg := &Config{}
	vault := &testSecretVault{data: map[string]string{
		"uptime_kuma_api_key": "uk2_secret_from_vault",
	}}

	cfg.ApplyVaultSecrets(vault)

	if cfg.UptimeKuma.APIKey != "uk2_secret_from_vault" {
		t.Fatalf("APIKey = %q, want uptime kuma secret", cfg.UptimeKuma.APIKey)
	}
}

func TestConfigSaveOmitsUptimeKumaAPIKey(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  ui_language: en\n"), 0o644); err != nil {
		t.Fatalf("failed to seed config file: %v", err)
	}
	cfg := &Config{}
	cfg.UptimeKuma.Enabled = true
	cfg.UptimeKuma.BaseURL = "https://uptime.local"
	cfg.UptimeKuma.APIKey = "uk2_should_not_be_serialized"

	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	if strings.Contains(string(raw), "uk2_should_not_be_serialized") || strings.Contains(string(raw), "api_key:") {
		t.Fatalf("expected uptime kuma API key to stay out of YAML, got:\n%s", string(raw))
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

func TestLoadResolvesHelperOwnedSubsystemsFromHelperLLM(t *testing.T) {
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
    model: helper-model
llm:
  provider: main
  helper_enabled: true
  helper_provider: helper
personality:
  engine_v2: true
memory_analysis:
  enabled: true
tools:
  web_scraper:
    enabled: true
    summary_mode: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Personality.V2ProviderType != "openai" {
		t.Fatalf("personality v2 provider type = %q, want openai", cfg.Personality.V2ProviderType)
	}
	if cfg.Personality.V2ResolvedURL != "https://helper.example/v1" {
		t.Fatalf("personality v2 url = %q", cfg.Personality.V2ResolvedURL)
	}
	if cfg.Personality.V2ResolvedModel != "helper-model" {
		t.Fatalf("personality v2 model = %q, want helper-model", cfg.Personality.V2ResolvedModel)
	}
	if cfg.MemoryAnalysis.ProviderType != "openai" {
		t.Fatalf("memory analysis provider type = %q, want openai", cfg.MemoryAnalysis.ProviderType)
	}
	if cfg.MemoryAnalysis.BaseURL != "https://helper.example/v1" {
		t.Fatalf("memory analysis base url = %q", cfg.MemoryAnalysis.BaseURL)
	}
	if cfg.MemoryAnalysis.ResolvedModel != "helper-model" {
		t.Fatalf("memory analysis model = %q, want helper-model", cfg.MemoryAnalysis.ResolvedModel)
	}
	if cfg.Tools.WebScraper.SummaryBaseURL != "https://helper.example/v1" {
		t.Fatalf("web scraper summary base url = %q", cfg.Tools.WebScraper.SummaryBaseURL)
	}
	if cfg.Tools.WebScraper.SummaryModel != "helper-model" {
		t.Fatalf("web scraper summary model = %q, want helper-model", cfg.Tools.WebScraper.SummaryModel)
	}
}

func TestLoadMigratesLegacyPersonalityV2InlineFieldsToHelperLLM(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
llm:
  provider: openrouter
  base_url: https://openrouter.ai/api/v1
  api_key: main-secret
  model: main-model
personality:
  engine_v2: true
  v2_model: helper-model
  v2_url: https://helper.example/v1
  v2_api_key: helper-secret
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.LLM.HelperEnabled {
		t.Fatal("expected legacy v2 inline fields to enable helper llm migration")
	}
	if cfg.LLM.HelperProvider != "helper" {
		t.Fatalf("helper provider = %q, want helper", cfg.LLM.HelperProvider)
	}
	if cfg.LLM.HelperProviderType != "openai" {
		t.Fatalf("helper provider type = %q, want openai", cfg.LLM.HelperProviderType)
	}
	if cfg.LLM.HelperBaseURL != "https://helper.example/v1" {
		t.Fatalf("helper base url = %q", cfg.LLM.HelperBaseURL)
	}
	if cfg.LLM.HelperAPIKey != "helper-secret" {
		t.Fatalf("helper api key = %q, want helper-secret", cfg.LLM.HelperAPIKey)
	}
	if cfg.LLM.HelperResolvedModel != "helper-model" {
		t.Fatalf("helper resolved model = %q, want helper-model", cfg.LLM.HelperResolvedModel)
	}
}

func TestLoadDoesNotFallbackHelperOwnedSubsystemsToMainLLM(t *testing.T) {
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
personality:
  engine_v2: true
tools:
  web_scraper:
    enabled: true
    summary_mode: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Personality.V2ResolvedModel != "" || cfg.Personality.V2ResolvedURL != "" || cfg.Personality.V2ResolvedKey != "" {
		t.Fatalf("expected personality v2 helper path to stay unresolved without helper llm, got model=%q url=%q", cfg.Personality.V2ResolvedModel, cfg.Personality.V2ResolvedURL)
	}
	if cfg.MemoryAnalysis.ResolvedModel != "" || cfg.MemoryAnalysis.BaseURL != "" || cfg.MemoryAnalysis.APIKey != "" || cfg.MemoryAnalysis.ProviderType != "" {
		t.Fatalf("expected memory analysis helper path to stay unresolved without helper llm, got model=%q url=%q", cfg.MemoryAnalysis.ResolvedModel, cfg.MemoryAnalysis.BaseURL)
	}
	if cfg.Tools.WebScraper.SummaryModel != "" || cfg.Tools.WebScraper.SummaryBaseURL != "" || cfg.Tools.WebScraper.SummaryAPIKey != "" {
		t.Fatalf("expected web scraper summary helper path to stay unresolved without helper llm, got model=%q url=%q", cfg.Tools.WebScraper.SummaryModel, cfg.Tools.WebScraper.SummaryBaseURL)
	}
}

func TestApplyOAuthTokensDoesNotOverwriteProviderStaticSecrets(t *testing.T) {
	cfg := &Config{}
	cfg.Providers = []ProviderEntry{
		{
			ID:       "oauth-provider",
			Type:     "openai",
			AuthType: "oauth2",
			APIKey:   "",
		},
	}
	cfg.LLM.Provider = "oauth-provider"
	cfg.ResolveProviders()

	vault := &testSecretVault{data: map[string]string{
		"oauth_oauth-provider": `{"access_token":"oauth-access-token"}`,
	}}
	cfg.ApplyOAuthTokens(vault)

	if cfg.Providers[0].APIKey != "" {
		t.Fatalf("provider api key = %q, want empty static secret field", cfg.Providers[0].APIKey)
	}
	if cfg.LLM.APIKey != "oauth-access-token" {
		t.Fatalf("resolved llm api key = %q, want oauth-access-token", cfg.LLM.APIKey)
	}
}

func TestApplyOAuthTokensUpdatesHelperAndDerivedSubsystems(t *testing.T) {
	cfg := &Config{}
	cfg.Providers = []ProviderEntry{
		{
			ID:        "helper",
			Type:      "workers-ai",
			BaseURL:   "",
			Model:     "@cf/meta/llama-3.1-8b-instruct",
			AccountID: "cf-account",
			AuthType:  "oauth2",
		},
	}
	cfg.LLM.HelperEnabled = true
	cfg.LLM.HelperProvider = "helper"
	cfg.ResolveProviders()

	if cfg.LLM.HelperAccountID != "cf-account" {
		t.Fatalf("helper account id = %q, want cf-account", cfg.LLM.HelperAccountID)
	}

	cfg.Personality.V2ResolvedURL = cfg.LLM.HelperBaseURL
	cfg.Personality.V2ResolvedModel = cfg.LLM.HelperResolvedModel
	cfg.MemoryAnalysis.BaseURL = cfg.LLM.HelperBaseURL
	cfg.MemoryAnalysis.ResolvedModel = cfg.LLM.HelperResolvedModel
	cfg.Tools.WebScraper.SummaryBaseURL = cfg.LLM.HelperBaseURL
	cfg.Tools.WebScraper.SummaryModel = cfg.LLM.HelperResolvedModel

	vault := &testSecretVault{data: map[string]string{
		"oauth_helper": `{"access_token":"helper-oauth-token"}`,
	}}
	cfg.ApplyOAuthTokens(vault)

	if cfg.LLM.HelperAPIKey != "helper-oauth-token" {
		t.Fatalf("helper api key = %q, want helper-oauth-token", cfg.LLM.HelperAPIKey)
	}
	if cfg.Personality.V2ResolvedKey != "helper-oauth-token" {
		t.Fatalf("personality oauth key = %q, want helper-oauth-token", cfg.Personality.V2ResolvedKey)
	}
	if cfg.MemoryAnalysis.APIKey != "helper-oauth-token" {
		t.Fatalf("memory analysis oauth key = %q, want helper-oauth-token", cfg.MemoryAnalysis.APIKey)
	}
	if cfg.Tools.WebScraper.SummaryAPIKey != "helper-oauth-token" {
		t.Fatalf("web scraper summary oauth key = %q, want helper-oauth-token", cfg.Tools.WebScraper.SummaryAPIKey)
	}
}

func TestApplyOAuthTokensUsesCurrentPersonalityProviderField(t *testing.T) {
	cfg := &Config{}
	cfg.Providers = []ProviderEntry{
		{
			ID:       "personality",
			Type:     "openai",
			AuthType: "oauth2",
		},
	}
	cfg.Personality.V2Provider = "personality"

	vault := &testSecretVault{data: map[string]string{
		"oauth_personality": `{"access_token":"personality-token"}`,
	}}
	cfg.ApplyOAuthTokens(vault)

	if cfg.Personality.V2ResolvedKey != "personality-token" {
		t.Fatalf("personality resolved key = %q, want personality-token", cfg.Personality.V2ResolvedKey)
	}
}
