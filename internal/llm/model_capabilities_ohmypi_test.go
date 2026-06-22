package llm

import (
	"testing"

	"aurago/internal/config"
)

func TestResolveProviderCapabilitiesPrefersOhMyPiBeforeHeuristics(t *testing.T) {
	result := ResolveProviderCapabilities(config.ProviderEntry{
		ID:    "main",
		Type:  "openai",
		Model: "gpt-4o",
	}, CapabilityFallback{})

	if !result.Known {
		t.Fatalf("expected known capabilities, got %+v", result)
	}
	if result.Source != CapabilitySourceOhMyPi {
		t.Fatalf("source = %q, want %q", result.Source, CapabilitySourceOhMyPi)
	}
	if !result.Multimodal {
		t.Fatalf("expected multimodal capability from oh-my-pi snapshot, got %+v", result)
	}
}

func TestResolveProviderCapabilitiesManualOverrideStillWins(t *testing.T) {
	result := ResolveProviderCapabilities(config.ProviderEntry{
		ID:    "main",
		Type:  "openai",
		Model: "gpt-4o",
		Capabilities: config.ProviderCapabilities{
			Auto:              boolPtr(false),
			ToolCalling:       false,
			StructuredOutputs: false,
			Multimodal:        false,
			DetectedModel:     "manual-model",
			Source:            CapabilitySourceManual,
		},
	}, CapabilityFallback{ToolCalling: true, StructuredOutputs: true, Multimodal: true})

	if result.Source != CapabilitySourceManual {
		t.Fatalf("source = %q, want manual", result.Source)
	}
	if result.ToolCalling || result.StructuredOutputs || result.Multimodal {
		t.Fatalf("manual false capabilities should win: %+v", result)
	}
	if result.DetectedModel != "manual-model" {
		t.Fatalf("detected model = %q", result.DetectedModel)
	}
}

func boolPtr(v bool) *bool {
	return &v
}
