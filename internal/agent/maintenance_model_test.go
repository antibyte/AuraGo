package agent

import (
	"testing"

	"aurago/internal/config"
)

func TestResolveConsolidationModelPrefersOverride(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Model = "main-model"
	cfg.Consolidation.Model = "cheap-model"

	if got := resolveConsolidationModel(cfg); got != "cheap-model" {
		t.Fatalf("resolveConsolidationModel() = %q, want %q", got, "cheap-model")
	}
}

func TestResolveConsolidationModelFallsBackToMainModel(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Model = "main-model"

	if got := resolveConsolidationModel(cfg); got != "main-model" {
		t.Fatalf("resolveConsolidationModel() = %q, want %q", got, "main-model")
	}
}