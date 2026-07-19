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

func TestResolveProviderCapabilitiesDetectsAgnesFlash(t *testing.T) {
	result := ResolveProviderCapabilities(config.ProviderEntry{
		ID:    "main",
		Type:  "agnes",
		Model: "agnes-2.0-flash",
	}, CapabilityFallback{})

	if !result.Known || result.Source != CapabilitySourceHeuristic {
		t.Fatalf("expected Agnes heuristic capabilities, got %+v", result)
	}
	if !result.ToolCalling || !result.Multimodal {
		t.Fatalf("expected Agnes tool calling and multimodal support, got %+v", result)
	}
	if result.StructuredOutputs {
		t.Fatalf("structured outputs are not documented for Agnes 2.0 Flash: %+v", result)
	}
}

func boolPtr(v bool) *bool {
	return &v
}
