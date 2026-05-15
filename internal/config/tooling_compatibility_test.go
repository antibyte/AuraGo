package config

import (
	"os"
	"path/filepath"
	"testing"
)

func loadConfigFromTestYAML(t *testing.T, yamlData string) *Config {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(yamlData), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return cfg
}

func TestLoadPreservesCriticalToolingCompatibilityFields(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
llm:
  use_native_functions: false
  structured_outputs: true
  helper_enabled: true
  helper_provider: helper
  helper_model: cheap-model
providers:
  - id: helper
    type: openrouter
    base_url: https://openrouter.ai/api/v1
    api_key: helper-secret
    model: helper-default
agent:
  max_tool_guides: 9
  recovery:
    max_provider_422_recoveries: 6
    min_messages_for_empty_retry: 7
    duplicate_consecutive_hits: 4
    duplicate_frequency_hits: 5
    identical_tool_error_hits: 6
  adaptive_tools:
    enabled: true
    max_tools: 17
    decay_half_life_days: 3
    weight_success_rate: false
    always_include: [docker, homepage, filesystem]
memory_analysis:
  enabled: true
  real_time: true
  query_expansion: true
llm_guardian:
  enabled: true
  allow_clarification: true
guardian:
	max_scan_bytes: 2048
	scan_edge_bytes: 768
homepage:
  allow_local_server: true
  allow_temporary_token_budget_overflow: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.LLM.UseNativeFunctions {
		t.Fatal("expected llm.use_native_functions=false to be preserved")
	}
	if !cfg.LLM.StructuredOutputs {
		t.Fatal("expected llm.structured_outputs=true to be preserved")
	}
	if !cfg.LLM.HelperEnabled {
		t.Fatal("expected llm.helper_enabled=true to be preserved")
	}
	if cfg.LLM.HelperProvider != "helper" {
		t.Fatalf("llm.helper_provider = %q, want helper", cfg.LLM.HelperProvider)
	}
	if cfg.LLM.HelperResolvedModel != "cheap-model" {
		t.Fatalf("llm.helper_resolved_model = %q, want cheap-model", cfg.LLM.HelperResolvedModel)
	}
	if !cfg.Agent.AdaptiveTools.Enabled {
		t.Fatal("expected adaptive_tools.enabled=true to be preserved")
	}
	if cfg.Agent.AdaptiveTools.MaxTools != 17 {
		t.Fatalf("adaptive_tools.max_tools = %d, want 17", cfg.Agent.AdaptiveTools.MaxTools)
	}
	if cfg.Agent.AdaptiveTools.DecayHalfLifeDays != 3 {
		t.Fatalf("adaptive_tools.decay_half_life_days = %v, want 3", cfg.Agent.AdaptiveTools.DecayHalfLifeDays)
	}
	if cfg.Agent.AdaptiveTools.WeightSuccessRate {
		t.Fatal("expected adaptive_tools.weight_success_rate=false to be preserved")
	}
	if len(cfg.Agent.AdaptiveTools.AlwaysInclude) != 3 {
		t.Fatalf("adaptive_tools.always_include len = %d, want 3", len(cfg.Agent.AdaptiveTools.AlwaysInclude))
	}
	if cfg.Agent.MaxToolGuides != 9 {
		t.Fatalf("agent.max_tool_guides = %d, want 9", cfg.Agent.MaxToolGuides)
	}
	if cfg.Agent.Recovery.MaxProvider422Recoveries != 6 {
		t.Fatalf("agent.recovery.max_provider_422_recoveries = %d, want 6", cfg.Agent.Recovery.MaxProvider422Recoveries)
	}
	if cfg.Agent.Recovery.MinMessagesForEmptyRetry != 7 {
		t.Fatalf("agent.recovery.min_messages_for_empty_retry = %d, want 7", cfg.Agent.Recovery.MinMessagesForEmptyRetry)
	}
	if cfg.Agent.Recovery.DuplicateConsecutiveHits != 4 {
		t.Fatalf("agent.recovery.duplicate_consecutive_hits = %d, want 4", cfg.Agent.Recovery.DuplicateConsecutiveHits)
	}
	if cfg.Agent.Recovery.DuplicateFrequencyHits != 5 {
		t.Fatalf("agent.recovery.duplicate_frequency_hits = %d, want 5", cfg.Agent.Recovery.DuplicateFrequencyHits)
	}
	if cfg.Agent.Recovery.IdenticalToolErrorHits != 6 {
		t.Fatalf("agent.recovery.identical_tool_error_hits = %d, want 6", cfg.Agent.Recovery.IdenticalToolErrorHits)
	}
	if !cfg.MemoryAnalysis.Enabled || !cfg.MemoryAnalysis.RealTime || !cfg.MemoryAnalysis.QueryExpansion {
		t.Fatal("expected memory_analysis compatibility fields to be preserved")
	}
	if !cfg.LLMGuardian.Enabled || !cfg.LLMGuardian.AllowClarification {
		t.Fatal("expected llm_guardian compatibility fields to be preserved")
	}
	if cfg.Guardian.MaxScanBytes != 2048 || cfg.Guardian.ScanEdgeBytes != 768 {
		t.Fatalf("expected guardian config to be preserved, got max=%d edge=%d", cfg.Guardian.MaxScanBytes, cfg.Guardian.ScanEdgeBytes)
	}
	if !cfg.Homepage.AllowLocalServer {
		t.Fatal("expected homepage.allow_local_server=true to be preserved")
	}
	if !cfg.Homepage.AllowTemporaryTokenBudgetOverflow {
		t.Fatal("expected homepage.allow_temporary_token_budget_overflow=true to be preserved")
	}
}

func TestLoadPreservesProviderCapabilityOverrides(t *testing.T) {
	cfg := loadConfigFromTestYAML(t, `
providers:
  - id: main
    type: openai
    base_url: https://api.openai.com/v1
    model: unknown-local-model
    capabilities:
      auto: false
      tool_calling: true
      structured_outputs: false
      multimodal: true
      detected_model: unknown-local-model
      source: manual
llm:
  provider: main
`)

	p := cfg.FindProvider("main")
	if p == nil {
		t.Fatal("provider main not found")
	}
	if p.Capabilities.AutoEnabled() {
		t.Fatal("expected manual provider capabilities")
	}
	if !p.Capabilities.ToolCalling {
		t.Fatal("expected tool_calling override to load")
	}
	if p.Capabilities.StructuredOutputs {
		t.Fatal("did not expect structured_outputs override")
	}
	if !p.Capabilities.Multimodal {
		t.Fatal("expected multimodal override to load")
	}
	if p.Capabilities.Source != "manual" {
		t.Fatalf("source = %q, want manual", p.Capabilities.Source)
	}
}

func TestAdaptiveToolsMaxToolsDefaultsToSixteenWhenEnabled(t *testing.T) {
	cfg := loadConfigFromTestYAML(t, `
agent:
  adaptive_tools:
    enabled: true
`)

	if cfg.Agent.AdaptiveTools.MaxTools != 16 {
		t.Fatalf("adaptive_tools.max_tools = %d, want 16", cfg.Agent.AdaptiveTools.MaxTools)
	}
}

func TestCoreMemoryDefaultsUseHardSmallCap(t *testing.T) {
	cfg := loadConfigFromTestYAML(t, `
agent:
  adaptive_tools:
    enabled: true
`)
	if cfg.Agent.CoreMemoryMaxEntries != 80 {
		t.Fatalf("core_memory_max_entries = %d, want 80", cfg.Agent.CoreMemoryMaxEntries)
	}
	if cfg.Agent.CoreMemoryCapMode != "hard" {
		t.Fatalf("core_memory_cap_mode = %q, want hard", cfg.Agent.CoreMemoryCapMode)
	}
	if !containsString(cfg.Agent.AdaptiveTools.AlwaysInclude, "virtual_desktop") {
		t.Fatalf("adaptive always_include missing virtual_desktop: %#v", cfg.Agent.AdaptiveTools.AlwaysInclude)
	}
}

func TestAdaptiveToolsDefaultAlwaysIncludeKeepsDDGSearch(t *testing.T) {
	cfg := loadConfigFromTestYAML(t, `
agent:
  adaptive_tools:
    enabled: true
`)

	if !containsString(cfg.Agent.AdaptiveTools.AlwaysInclude, "ddg_search") {
		t.Fatalf("adaptive always_include missing ddg_search: %#v", cfg.Agent.AdaptiveTools.AlwaysInclude)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestAdaptiveToolsNewFieldsDefaultWhenEnabled(t *testing.T) {
	cfg := loadConfigFromTestYAML(t, `
agent:
  adaptive_tools:
    enabled: true
`)

	if cfg.Agent.AdaptiveTools.MaxTotalTools != 32 {
		t.Fatalf("adaptive_tools.max_total_tools = %d, want 32", cfg.Agent.AdaptiveTools.MaxTotalTools)
	}
	if !cfg.Agent.AdaptiveTools.ProviderProfilesEnabled {
		t.Fatal("expected provider_profiles_enabled to default to true")
	}
	if cfg.Agent.AdaptiveTools.SessionToolRetentionTurns != 8 {
		t.Fatalf("session_tool_retention_turns = %d, want 8", cfg.Agent.AdaptiveTools.SessionToolRetentionTurns)
	}
}

func TestAdaptiveToolsNewFieldsPreserveExplicitValues(t *testing.T) {
	cfg := loadConfigFromTestYAML(t, `
agent:
  adaptive_tools:
    enabled: true
    max_total_tools: 24
    provider_profiles_enabled: false
    session_tool_retention_turns: 3
`)

	if cfg.Agent.AdaptiveTools.MaxTotalTools != 24 {
		t.Fatalf("adaptive_tools.max_total_tools = %d, want 24", cfg.Agent.AdaptiveTools.MaxTotalTools)
	}
	if cfg.Agent.AdaptiveTools.ProviderProfilesEnabled {
		t.Fatal("expected explicit provider_profiles_enabled=false to be preserved")
	}
	if cfg.Agent.AdaptiveTools.SessionToolRetentionTurns != 3 {
		t.Fatalf("session_tool_retention_turns = %d, want 3", cfg.Agent.AdaptiveTools.SessionToolRetentionTurns)
	}
}

func TestAdaptiveToolsProviderProfilesDefaultWhenAdaptiveDisabled(t *testing.T) {
	cfg := loadConfigFromTestYAML(t, `
agent:
  adaptive_tools:
    enabled: false
`)

	if !cfg.Agent.AdaptiveTools.ProviderProfilesEnabled {
		t.Fatal("expected provider_profiles_enabled to default to true even when adaptive filtering is disabled")
	}
	if cfg.Agent.AdaptiveTools.MaxTotalTools != 0 {
		t.Fatalf("max_total_tools = %d, want 0 while adaptive filtering is disabled", cfg.Agent.AdaptiveTools.MaxTotalTools)
	}
}

func TestLoadDefaultsAdaptiveWeightSuccessRateWhenOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
agent:
  adaptive_tools:
    enabled: true
    max_tools: 11
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.Agent.AdaptiveTools.WeightSuccessRate {
		t.Fatal("expected adaptive_tools.weight_success_rate to default to true when omitted")
	}
}

func TestLoadDefaultsRecoverySettingsWhenOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
agent:
  adaptive_tools:
    enabled: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Agent.Recovery.MaxProvider422Recoveries != 3 {
		t.Fatalf("default max_provider_422_recoveries = %d, want 3", cfg.Agent.Recovery.MaxProvider422Recoveries)
	}
	if cfg.Agent.Recovery.MinMessagesForEmptyRetry != 5 {
		t.Fatalf("default min_messages_for_empty_retry = %d, want 5", cfg.Agent.Recovery.MinMessagesForEmptyRetry)
	}
	if cfg.Agent.Recovery.DuplicateConsecutiveHits != 2 {
		t.Fatalf("default duplicate_consecutive_hits = %d, want 2", cfg.Agent.Recovery.DuplicateConsecutiveHits)
	}
	if cfg.Agent.Recovery.DuplicateFrequencyHits != 3 {
		t.Fatalf("default duplicate_frequency_hits = %d, want 3", cfg.Agent.Recovery.DuplicateFrequencyHits)
	}
	if cfg.Agent.Recovery.IdenticalToolErrorHits != 3 {
		t.Fatalf("default identical_tool_error_hits = %d, want 3", cfg.Agent.Recovery.IdenticalToolErrorHits)
	}
}

func TestLoadPreservesCoAgentPolicyFields(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
co_agents:
  enabled: true
  max_concurrent: 4
  budget_quota_percent: 35
  max_context_hints: 4
  max_context_hint_chars: 120
  max_result_bytes: 90000
  queue_when_busy: false
  cleanup_interval_minutes: 7
  cleanup_max_age_minutes: 45
  retry_policy:
    max_retries: 2
    retry_delay_seconds: 11
    retryable_error_patterns: ["deadline exceeded", "rate limit"]
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.CoAgents.Enabled {
		t.Fatal("expected co_agents.enabled=true to be preserved")
	}
	if cfg.CoAgents.MaxConcurrent != 4 {
		t.Fatalf("co_agents.max_concurrent = %d, want 4", cfg.CoAgents.MaxConcurrent)
	}
	if cfg.CoAgents.BudgetQuotaPercent != 35 {
		t.Fatalf("co_agents.budget_quota_percent = %d, want 35", cfg.CoAgents.BudgetQuotaPercent)
	}
	if cfg.CoAgents.MaxContextHints != 4 {
		t.Fatalf("co_agents.max_context_hints = %d, want 4", cfg.CoAgents.MaxContextHints)
	}
	if cfg.CoAgents.MaxContextHintChars != 120 {
		t.Fatalf("co_agents.max_context_hint_chars = %d, want 120", cfg.CoAgents.MaxContextHintChars)
	}
	if cfg.CoAgents.MaxResultBytes != 90000 {
		t.Fatalf("co_agents.max_result_bytes = %d, want 90000", cfg.CoAgents.MaxResultBytes)
	}
	if cfg.CoAgents.QueueWhenBusy {
		t.Fatal("expected co_agents.queue_when_busy=false to be preserved")
	}
	if cfg.CoAgents.CleanupIntervalMins != 7 {
		t.Fatalf("co_agents.cleanup_interval_minutes = %d, want 7", cfg.CoAgents.CleanupIntervalMins)
	}
	if cfg.CoAgents.CleanupMaxAgeMins != 45 {
		t.Fatalf("co_agents.cleanup_max_age_minutes = %d, want 45", cfg.CoAgents.CleanupMaxAgeMins)
	}
	if cfg.CoAgents.RetryPolicy.MaxRetries != 2 {
		t.Fatalf("co_agents.retry_policy.max_retries = %d, want 2", cfg.CoAgents.RetryPolicy.MaxRetries)
	}
	if cfg.CoAgents.RetryPolicy.RetryDelaySeconds != 11 {
		t.Fatalf("co_agents.retry_policy.retry_delay_seconds = %d, want 11", cfg.CoAgents.RetryPolicy.RetryDelaySeconds)
	}
	if len(cfg.CoAgents.RetryPolicy.RetryableErrorPatterns) != 2 {
		t.Fatalf("co_agents.retry_policy.retryable_error_patterns len = %d, want 2", len(cfg.CoAgents.RetryPolicy.RetryableErrorPatterns))
	}
}
