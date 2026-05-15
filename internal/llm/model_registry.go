package llm

import (
	"strings"
)

// GetModelInfo looks up a model in the known registry by provider and model ID.
// The key format is "provider/model_id" (e.g. "openai/gpt-4o").
func GetModelInfo(provider, modelID string) (ModelRegistryEntry, bool) {
	key := strings.ToLower(provider) + "/" + strings.ToLower(modelID)
	if entry, ok := KnownModelRegistry[key]; ok {
		return entry, true
	}
	return ModelRegistryEntry{}, false
}

// GetModelInfoByID searches the registry for a model by its ID only,
// ignoring the provider. Returns the first match (or any match, since
// model IDs are typically unique across providers).
func GetModelInfoByID(modelID string) (ModelRegistryEntry, bool) {
	lower := strings.ToLower(modelID)
	for key, entry := range KnownModelRegistry {
		if strings.HasSuffix(strings.ToLower(key), "/"+lower) {
			return entry, true
		}
	}
	return ModelRegistryEntry{}, false
}

// GetModelsForProvider returns all registered models for a given provider.
func GetModelsForProvider(provider string) []ModelRegistryEntry {
	prefix := strings.ToLower(provider) + "/"
	var result []ModelRegistryEntry
	for key, entry := range KnownModelRegistry {
		if strings.HasPrefix(key, prefix) {
			result = append(result, entry)
		}
	}
	return result
}

// DetectContextWindowFromRegistry attempts to find the context window
// for a model from the static registry. It tries an exact match first,
// then falls back to prefix matching on the model ID.
func DetectContextWindowFromRegistry(provider, modelID string) (int, bool) {
	// Try exact provider + model match
	if entry, ok := GetModelInfo(provider, modelID); ok && entry.ContextWindow > 0 {
		return entry.ContextWindow, true
	}

	// Try model-only match (provider-agnostic)
	if entry, ok := GetModelInfoByID(modelID); ok && entry.ContextWindow > 0 {
		return entry.ContextWindow, true
	}

	// Prefix match on model ID against all registry entries for the provider
	lowerModel := strings.ToLower(modelID)
	for _, entry := range GetModelsForProvider(provider) {
		if strings.HasPrefix(lowerModel, strings.ToLower(entry.ID)) && entry.ContextWindow > 0 {
			return entry.ContextWindow, true
		}
	}

	return 0, false
}

// GetPricingFromRegistry returns pricing information from the static registry.
func GetPricingFromRegistry(provider, modelID string) (ModelPricing, bool) {
	entry, ok := GetModelInfo(provider, modelID)
	if !ok {
		// Try provider-agnostic match
		entry, ok = GetModelInfoByID(modelID)
	}
	if !ok {
		return ModelPricing{}, false
	}

	return ModelPricing{
		ModelID:          entry.ID,
		InputPerMillion:  entry.InputPricePer1M,
		OutputPerMillion: entry.OutputPricePer1M,
	}, true
}

// GetCapabilitiesFromRegistry returns capability flags from the static registry.
func GetCapabilitiesFromRegistry(provider, modelID string) (toolCall, reasoning, structuredOutput, multimodal bool, ok bool) {
	entry, ok := GetModelInfo(provider, modelID)
	if !ok {
		entry, ok = GetModelInfoByID(modelID)
	}
	if !ok {
		return false, false, false, false, false
	}
	return entry.SupportsToolCall, entry.SupportsReasoning, entry.SupportsStructuredOutput, entry.SupportsMultimodal, true
}
