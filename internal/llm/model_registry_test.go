package llm

import (
	"testing"

	"aurago/internal/config"
)

func TestGetModelInfo(t *testing.T) {
	tests := []struct {
		provider string
		modelID  string
		wantOK   bool
		wantCtx  int
	}{
		{"openai", "gpt-4o", true, 128000},
		{"anthropic", "claude-3-5-sonnet-20241022", true, 200000},
		{"deepseek", "deepseek-chat", true, 1000000},
		{"groq", "llama3-70b-8192", true, 8192},
		{"nonexistent", "unknown", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.provider+"/"+tt.modelID, func(t *testing.T) {
			entry, ok := GetModelInfo(tt.provider, tt.modelID)
			if ok != tt.wantOK {
				t.Fatalf("GetModelInfo(%q, %q) ok=%v, want %v", tt.provider, tt.modelID, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if entry.ContextWindow != tt.wantCtx {
				t.Errorf("ContextWindow=%d, want %d", entry.ContextWindow, tt.wantCtx)
			}
			if entry.ID == "" {
				t.Error("ID should not be empty")
			}
		})
	}
}

func TestGetModelInfoCaseInsensitive(t *testing.T) {
	entry, ok := GetModelInfo("OpenAI", "GPT-4O")
	if !ok {
		t.Fatal("Expected case-insensitive match")
	}
	if entry.ContextWindow != 128000 {
		t.Errorf("ContextWindow=%d, want 128000", entry.ContextWindow)
	}
}

func TestGetModelsForProvider(t *testing.T) {
	models := GetModelsForProvider("openai")
	if len(models) == 0 {
		t.Fatal("Expected OpenAI models in registry")
	}
	// Should have at least gpt-4o
	found := false
	for _, m := range models {
		if m.ID == "gpt-4o" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected gpt-4o in OpenAI models")
	}
}

func TestDetectContextWindowFromRegistry(t *testing.T) {
	tests := []struct {
		provider string
		modelID  string
		want     int
		wantOK   bool
	}{
		{"openai", "gpt-4o", 128000, true},
		{"anthropic", "claude-opus-4-0", 200000, true},
		{"deepseek", "deepseek-chat", 1000000, true},
		{"moonshot", "kimi-k2.5", 262144, true},
		{"nonexistent", "unknown", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.provider+"/"+tt.modelID, func(t *testing.T) {
			got, ok := DetectContextWindowFromRegistry(tt.provider, tt.modelID)
			if ok != tt.wantOK {
				t.Fatalf("DetectContextWindowFromRegistry(%q, %q) ok=%v, want %v", tt.provider, tt.modelID, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("DetectContextWindowFromRegistry(%q, %q) = %d, want %d", tt.provider, tt.modelID, got, tt.want)
			}
		})
	}
}

func TestGetPricingFromRegistry(t *testing.T) {
	pricing, ok := GetPricingFromRegistry("openai", "gpt-4o")
	if !ok {
		t.Fatal("Expected pricing for gpt-4o")
	}
	if pricing.InputPerMillion <= 0 {
		t.Error("Expected non-zero input price")
	}
	if pricing.OutputPerMillion <= 0 {
		t.Error("Expected non-zero output price")
	}
}

func TestGetCapabilitiesFromRegistry(t *testing.T) {
	// GPT-4o supports tool calling and structured output but not reasoning
	toolCall, reasoning, structuredOutput, multimodal, ok := GetCapabilitiesFromRegistry("openai", "gpt-4o")
	if !ok {
		t.Fatal("Expected capabilities for gpt-4o")
	}
	if !toolCall {
		t.Error("gpt-4o should support tool calling")
	}
	if reasoning {
		t.Error("gpt-4o should not support reasoning")
	}
	if !structuredOutput {
		t.Error("gpt-4o should support structured output")
	}
	if !multimodal {
		t.Error("gpt-4o should support multimodal image input")
	}

	// o3-pro supports reasoning
	_, reasoning, _, _, ok = GetCapabilitiesFromRegistry("openai", "o3-pro")
	if !ok {
		t.Fatal("Expected capabilities for o3-pro")
	}
	if !reasoning {
		t.Error("o3-pro should support reasoning")
	}
}

func TestCapabilitiesFromOpenRouterModelMetadata(t *testing.T) {
	caps, ok := CapabilitiesFromOpenRouterModel(OpenRouterModelMetadata{
		ID:                  "anthropic/claude-opus-4.7-fast",
		SupportedParameters: []string{"tools", "structured_outputs", "response_format"},
		Architecture: OpenRouterArchitecture{
			InputModalities: []string{"text", "image", "file"},
		},
	})
	if !ok {
		t.Fatal("expected OpenRouter metadata to produce capabilities")
	}
	if !caps.ToolCalling {
		t.Fatal("expected tools parameter to enable tool calling")
	}
	if !caps.StructuredOutputs {
		t.Fatal("expected structured_outputs parameter to enable structured outputs")
	}
	if !caps.Multimodal {
		t.Fatal("expected image input modality to enable multimodal")
	}
	if caps.Source != "openrouter" {
		t.Fatalf("source = %q, want openrouter", caps.Source)
	}
}

func TestResolveProviderCapabilitiesUsesLegacyFallbackForUnknownModel(t *testing.T) {
	caps := ResolveProviderCapabilities(config.ProviderEntry{
		Type:  "custom",
		Model: "totally-unknown-model",
	}, CapabilityFallback{
		ToolCalling:       true,
		StructuredOutputs: true,
		Multimodal:        true,
	})

	if caps.Source != CapabilitySourceLegacyFallback {
		t.Fatalf("source = %q, want legacy fallback", caps.Source)
	}
	if caps.Known {
		t.Fatal("expected unknown model to remain marked unknown")
	}
	if !caps.ToolCalling || !caps.StructuredOutputs || !caps.Multimodal {
		t.Fatalf("expected legacy fallback flags, got %+v", caps)
	}
}
