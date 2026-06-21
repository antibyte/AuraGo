package llm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm/catalog"
)

const (
	CapabilitySourceManual         = "manual"
	CapabilitySourceOpenRouter     = "openrouter"
	CapabilitySourceOhMyPi         = "oh-my-pi"
	CapabilitySourceModelsDev      = "models.dev"
	CapabilitySourceHeuristic      = "heuristic"
	CapabilitySourceLegacyFallback = "legacy_fallback"
)

// CapabilityFallback contains deprecated global LLM settings used only when no
// provider/model-specific capability metadata is available.
type CapabilityFallback struct {
	ToolCalling       bool
	StructuredOutputs bool
	Multimodal        bool
}

// ProviderCapabilityResult is the resolved provider/model feature set.
type ProviderCapabilityResult struct {
	ToolCalling       bool   `json:"tool_calling"`
	StructuredOutputs bool   `json:"structured_outputs"`
	Multimodal        bool   `json:"multimodal"`
	Reasoning         bool   `json:"reasoning,omitempty"`
	DetectedModel     string `json:"detected_model"`
	Source            string `json:"source"`
	Known             bool   `json:"known"`
}

// OpenRouterModelMetadata is the subset of OpenRouter /models metadata needed
// for capability detection.
type OpenRouterModelMetadata struct {
	ID                  string                 `json:"id"`
	SupportedParameters []string               `json:"supported_parameters"`
	Architecture        OpenRouterArchitecture `json:"architecture"`
}

type OpenRouterArchitecture struct {
	Modality        string   `json:"modality"`
	InputModalities []string `json:"input_modalities"`
}

// ResolveConfigProviderCapabilities resolves the effective capabilities for the
// main LLM provider in cfg.
func ResolveConfigProviderCapabilities(cfg *config.Config) ProviderCapabilityResult {
	if cfg == nil {
		return ProviderCapabilityResult{Source: CapabilitySourceLegacyFallback}
	}
	fallback := CapabilityFallback{
		ToolCalling:       cfg.LLM.UseNativeFunctions,
		StructuredOutputs: cfg.LLM.StructuredOutputs,
		Multimodal:        cfg.LLM.Multimodal,
	}
	if p := cfg.FindProvider(cfg.LLM.Provider); p != nil {
		return ResolveProviderCapabilities(*p, fallback)
	}
	return ResolveProviderCapabilities(config.ProviderEntry{
		ID:    cfg.LLM.Provider,
		Type:  cfg.LLM.ProviderType,
		Model: cfg.LLM.Model,
	}, fallback)
}

// ResolveProviderCapabilities applies provider capability precedence:
// manual override, stored auto-detected values for the current model,
// models.dev registry, local heuristics, and finally legacy global fallback.
func ResolveProviderCapabilities(provider config.ProviderEntry, fallback CapabilityFallback) ProviderCapabilityResult {
	model := strings.TrimSpace(provider.Model)
	caps := provider.Capabilities
	if !caps.AutoEnabled() {
		return ProviderCapabilityResult{
			ToolCalling:       caps.ToolCalling,
			StructuredOutputs: caps.StructuredOutputs,
			Multimodal:        caps.Multimodal,
			DetectedModel:     firstNonEmpty(caps.DetectedModel, model),
			Source:            firstNonEmpty(caps.Source, CapabilitySourceManual),
			Known:             true,
		}
	}

	if strings.TrimSpace(caps.Source) != "" &&
		strings.TrimSpace(caps.DetectedModel) != "" &&
		strings.EqualFold(strings.TrimSpace(caps.DetectedModel), model) {
		return ProviderCapabilityResult{
			ToolCalling:       caps.ToolCalling,
			StructuredOutputs: caps.StructuredOutputs,
			Multimodal:        caps.Multimodal,
			DetectedModel:     caps.DetectedModel,
			Source:            caps.Source,
			Known:             true,
		}
	}

	if detected, ok := CapabilitiesFromRegistry(provider.Type, model); ok {
		if heuristic, heuristicOK := capabilitiesFromHeuristics(provider.Type, model); heuristicOK {
			detected.ToolCalling = detected.ToolCalling || heuristic.ToolCalling
			detected.Multimodal = detected.Multimodal || heuristic.Multimodal
		}
		return detected
	}
	if detected, ok := capabilitiesFromHeuristics(provider.Type, model); ok {
		return detected
	}
	return ProviderCapabilityResult{
		ToolCalling:       fallback.ToolCalling,
		StructuredOutputs: fallback.StructuredOutputs,
		Multimodal:        fallback.Multimodal,
		DetectedModel:     model,
		Source:            CapabilitySourceLegacyFallback,
		Known:             false,
	}
}

// CapabilitiesFromRegistry returns capability flags from generated models.dev
// metadata.
func CapabilitiesFromRegistry(provider, modelID string) (ProviderCapabilityResult, bool) {
	if snapshot, err := catalog.Load(); err == nil {
		if model, ok := snapshot.FindModel(provider, modelID); ok {
			return ProviderCapabilityResult{
				ToolCalling:       model.SupportsTools,
				StructuredOutputs: model.StructuredOutputs,
				Multimodal:        model.Multimodal,
				Reasoning:         model.Reasoning,
				DetectedModel:     model.ID,
				Source:            CapabilitySourceOhMyPi,
				Known:             true,
			}, true
		}
	}
	entry, ok := getModelsDevInfo(provider, modelID)
	if !ok {
		entry, ok = getModelsDevInfoByID(modelID)
	}
	if !ok {
		return ProviderCapabilityResult{}, false
	}
	return ProviderCapabilityResult{
		ToolCalling:       entry.SupportsToolCall,
		StructuredOutputs: entry.SupportsStructuredOutput,
		Multimodal:        entry.SupportsMultimodal,
		Reasoning:         entry.SupportsReasoning,
		DetectedModel:     entry.ID,
		Source:            CapabilitySourceModelsDev,
		Known:             true,
	}, true
}

// CapabilitiesFromOpenRouterModel maps OpenRouter model metadata to AuraGo
// capability flags.
func CapabilitiesFromOpenRouterModel(model OpenRouterModelMetadata) (ProviderCapabilityResult, bool) {
	if strings.TrimSpace(model.ID) == "" && len(model.SupportedParameters) == 0 &&
		len(model.Architecture.InputModalities) == 0 && strings.TrimSpace(model.Architecture.Modality) == "" {
		return ProviderCapabilityResult{}, false
	}

	params := map[string]bool{}
	for _, p := range model.SupportedParameters {
		params[strings.ToLower(strings.TrimSpace(p))] = true
	}
	multimodal := openRouterArchitectureSupportsMultimodal(model.Architecture)
	return ProviderCapabilityResult{
		ToolCalling:       params["tools"] || params["tool_choice"],
		StructuredOutputs: params["structured_outputs"] || params["response_format"],
		Multimodal:        multimodal,
		DetectedModel:     model.ID,
		Source:            CapabilitySourceOpenRouter,
		Known:             true,
	}, true
}

// FetchOpenRouterModelCapabilities fetches live OpenRouter model metadata for a
// single model ID. The public models endpoint does not require an API key.
func FetchOpenRouterModelCapabilities(modelID string) (ProviderCapabilityResult, bool, error) {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return ProviderCapabilityResult{}, false, nil
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get("https://openrouter.ai/api/v1/models")
	if err != nil {
		return ProviderCapabilityResult{}, false, fmt.Errorf("fetch OpenRouter models: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ProviderCapabilityResult{}, false, fmt.Errorf("OpenRouter returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return ProviderCapabilityResult{}, false, fmt.Errorf("read OpenRouter models: %w", err)
	}
	var parsed struct {
		Data []OpenRouterModelMetadata `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ProviderCapabilityResult{}, false, fmt.Errorf("parse OpenRouter models: %w", err)
	}
	for _, model := range parsed.Data {
		if strings.EqualFold(model.ID, modelID) {
			caps, ok := CapabilitiesFromOpenRouterModel(model)
			return caps, ok, nil
		}
	}
	return ProviderCapabilityResult{}, false, nil
}

func openRouterArchitectureSupportsMultimodal(arch OpenRouterArchitecture) bool {
	for _, modality := range arch.InputModalities {
		if isMultimodalInput(modality) {
			return true
		}
	}
	modality := strings.ToLower(strings.TrimSpace(arch.Modality))
	if modality == "" {
		return false
	}
	inputSide := modality
	if idx := strings.Index(inputSide, "->"); idx >= 0 {
		inputSide = inputSide[:idx]
	}
	for _, part := range strings.FieldsFunc(inputSide, func(r rune) bool {
		return r == '+' || r == ',' || r == ' '
	}) {
		if isMultimodalInput(part) {
			return true
		}
	}
	return false
}

func isMultimodalInput(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "image", "images", "file", "files", "pdf", "audio", "video":
		return true
	default:
		return false
	}
}

func capabilitiesFromHeuristics(provider, model string) (ProviderCapabilityResult, bool) {
	lowerProvider := strings.ToLower(strings.TrimSpace(provider))
	lowerModel := strings.ToLower(strings.TrimSpace(model))
	if lowerProvider == "" && lowerModel == "" {
		return ProviderCapabilityResult{}, false
	}

	isDeepSeek := strings.Contains(lowerModel, "deepseek")
	isClaude := strings.Contains(lowerModel, "claude")
	isNemotron := strings.Contains(lowerModel, "nemotron")
	isStepFun := strings.HasPrefix(lowerModel, "step-") || strings.Contains(lowerModel, "/step-")
	isGPT := strings.HasPrefix(lowerModel, "gpt-") || strings.Contains(lowerModel, "/gpt-")
	isGemini := strings.Contains(lowerModel, "gemini")
	toolCalling := isDeepSeek || isClaude || isNemotron || isStepFun || isGPT || isGemini
	structured := toolCalling && lowerProvider != "ollama"
	multimodal := modelNameSuggestsMultimodal(lowerModel)
	if !toolCalling && !structured && !multimodal {
		return ProviderCapabilityResult{}, false
	}
	return ProviderCapabilityResult{
		ToolCalling:       toolCalling,
		StructuredOutputs: structured,
		Multimodal:        multimodal,
		DetectedModel:     model,
		Source:            CapabilitySourceHeuristic,
		Known:             true,
	}, true
}

func modelNameSuggestsMultimodal(model string) bool {
	for _, marker := range []string{"gpt-4o", "vision", "vl", "gemini", "claude-3", "claude-4", "kimi-k2.6"} {
		if strings.Contains(model, marker) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
