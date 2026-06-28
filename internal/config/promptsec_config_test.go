package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPromptSecDefaults(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "promptsec_config_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("{}\n"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Guardian.MaxScanBytes != 16*1024 {
		t.Errorf("expected Guardian.MaxScanBytes=16384, got %d", cfg.Guardian.MaxScanBytes)
	}
	if cfg.Guardian.ScanEdgeBytes != 6*1024 {
		t.Errorf("expected Guardian.ScanEdgeBytes=6144, got %d", cfg.Guardian.ScanEdgeBytes)
	}
	if cfg.Guardian.PromptSec.Preset != "strict" {
		t.Errorf("expected PromptSec.Preset=strict, got %q", cfg.Guardian.PromptSec.Preset)
	}
	if !cfg.Guardian.PromptSec.Spotlight {
		t.Error("expected PromptSec.Spotlight=true")
	}
	if !cfg.Guardian.PromptSec.Canary {
		t.Error("expected PromptSec.Canary=true")
	}
	if !cfg.Guardian.PromptSec.Sanitizer.Normalize {
		t.Error("expected PromptSec.Sanitizer.Normalize=true")
	}
	if !cfg.Guardian.PromptSec.Sanitizer.Dehomoglyph {
		t.Error("expected PromptSec.Sanitizer.Dehomoglyph=true")
	}
	if !cfg.Guardian.PromptSec.Sanitizer.Decode {
		t.Error("expected PromptSec.Sanitizer.Decode=true")
	}
	if cfg.Guardian.PromptSec.Embedding.Enabled {
		t.Error("expected PromptSec.Embedding.Enabled=false")
	}
	if cfg.Guardian.PromptSec.Embedding.Threshold != 0.65 {
		t.Errorf("expected PromptSec.Embedding.Threshold=0.65, got %f", cfg.Guardian.PromptSec.Embedding.Threshold)
	}
	if cfg.Guardian.PromptSec.Policy != "" {
		t.Errorf("expected PromptSec.Policy=\"\", got %q", cfg.Guardian.PromptSec.Policy)
	}
	if cfg.Guardian.PromptSec.Taint.Enabled {
		t.Error("expected PromptSec.Taint.Enabled=false")
	}
	if cfg.Guardian.PromptSec.Structure.Enabled {
		t.Error("expected PromptSec.Structure.Enabled=false")
	}
	if cfg.Guardian.PromptSec.LLMJudge.Enabled {
		t.Error("expected PromptSec.LLMJudge.Enabled=false")
	}
	if cfg.Guardian.PromptSec.UseSanitizedOutput {
		t.Error("expected PromptSec.UseSanitizedOutput=false")
	}
}

func TestPromptSecLoadFromYAML(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "promptsec_config_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
guardian:
  max_scan_bytes: 8192
  scan_edge_bytes: 2048
  promptsec:
    preset: moderate
    spotlight: false
    canary: false
    sanitizer:
      normalize: false
      dehomoglyph: true
      decode: false
    embedding:
      enabled: true
      threshold: 0.7
    policy: "rag"
    custom_policy:
      disallowed_tasks:
        - translation
        - roleplay
    taint:
      enabled: true
      default_level: suspicious
    structure:
      enabled: true
      mode: xml
    llm_judge:
      enabled: true
      mode: always
      timeout_secs: 5
      policy: "Block pivots."
    use_sanitized_output: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Guardian.MaxScanBytes != 8192 {
		t.Errorf("expected MaxScanBytes=8192, got %d", cfg.Guardian.MaxScanBytes)
	}
	if cfg.Guardian.PromptSec.Preset != "moderate" {
		t.Errorf("expected Preset=moderate, got %q", cfg.Guardian.PromptSec.Preset)
	}
	if cfg.Guardian.PromptSec.Spotlight {
		t.Error("expected Spotlight=false")
	}
	if cfg.Guardian.PromptSec.Sanitizer.Normalize {
		t.Error("expected Sanitizer.Normalize=false")
	}
	if !cfg.Guardian.PromptSec.Sanitizer.Dehomoglyph {
		t.Error("expected Sanitizer.Dehomoglyph=true")
	}
	if !cfg.Guardian.PromptSec.Embedding.Enabled {
		t.Error("expected Embedding.Enabled=true")
	}
	if cfg.Guardian.PromptSec.Embedding.Threshold != 0.7 {
		t.Errorf("expected Embedding.Threshold=0.7, got %f", cfg.Guardian.PromptSec.Embedding.Threshold)
	}
	if cfg.Guardian.PromptSec.Policy != "rag" {
		t.Errorf("expected Policy=rag, got %q", cfg.Guardian.PromptSec.Policy)
	}
	if len(cfg.Guardian.PromptSec.CustomPolicy.DisallowedTasks) != 2 {
		t.Errorf("expected 2 custom policy tasks, got %d", len(cfg.Guardian.PromptSec.CustomPolicy.DisallowedTasks))
	}
	if !cfg.Guardian.PromptSec.Taint.Enabled {
		t.Error("expected Taint.Enabled=true")
	}
	if cfg.Guardian.PromptSec.Taint.DefaultLevel != "suspicious" {
		t.Errorf("expected Taint.DefaultLevel=suspicious, got %q", cfg.Guardian.PromptSec.Taint.DefaultLevel)
	}
	if !cfg.Guardian.PromptSec.Structure.Enabled {
		t.Error("expected Structure.Enabled=true")
	}
	if cfg.Guardian.PromptSec.Structure.Mode != "xml" {
		t.Errorf("expected Structure.Mode=xml, got %q", cfg.Guardian.PromptSec.Structure.Mode)
	}
	if !cfg.Guardian.PromptSec.LLMJudge.Enabled {
		t.Error("expected LLMJudge.Enabled=true")
	}
	if cfg.Guardian.PromptSec.LLMJudge.Mode != "always" {
		t.Errorf("expected LLMJudge.Mode=always, got %q", cfg.Guardian.PromptSec.LLMJudge.Mode)
	}
	if cfg.Guardian.PromptSec.LLMJudge.TimeoutSecs != 5 {
		t.Errorf("expected LLMJudge.TimeoutSecs=5, got %d", cfg.Guardian.PromptSec.LLMJudge.TimeoutSecs)
	}
	if !cfg.Guardian.PromptSec.UseSanitizedOutput {
		t.Error("expected UseSanitizedOutput=true")
	}
}

func TestPromptSecPartialSanitizerConfigKeepsMissingDefaults(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "promptsec_config_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
guardian:
  promptsec:
    sanitizer:
      normalize: false
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Guardian.PromptSec.Sanitizer.Normalize {
		t.Error("expected explicitly configured normalize=false to be preserved")
	}
	if !cfg.Guardian.PromptSec.Sanitizer.Dehomoglyph {
		t.Error("expected missing dehomoglyph to keep default true")
	}
	if !cfg.Guardian.PromptSec.Sanitizer.Decode {
		t.Error("expected missing decode to keep default true")
	}
}
