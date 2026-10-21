package llm

import (
	"sort"
	"strings"

	"aurago/internal/llm/catalog"
)

func sortedRegistryKeys() []string {
	keys := make([]string, 0, len(KnownModelRegistry))
	for key := range KnownModelRegistry {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// GetModelInfo looks up a model in the known registry by provider and model ID.
// The key format is "provider/model_id" (e.g. "openai/gpt-4o").
func GetModelInfo(provider, modelID string) (ModelRegistryEntry, bool) {
	if snapshot, err := catalog.Load(); err == nil {
		if model, ok := snapshot.FindModel(provider, modelID); ok {
			return catalogModelToRegistryEntry(model), true
		}
	}
	key := strings.ToLower(provider) + "/" + strings.ToLower(modelID)
	if entry, ok := KnownModelRegistry[key]; ok {
		return entry, true
	}
	return ModelRegistryEntry{}, false
}

// GetModelInfoByID searches the registry for a model by its ID only,
// ignoring the provider. Remaining registry candidates use a stable key order;
// the result always records its model-only provenance.
func GetModelInfoByID(modelID string) (ModelRegistryEntry, bool) {
	if snapshot, err := catalog.Load(); err == nil {
		if model, ok := snapshot.FindModelByID(modelID); ok {
			entry := catalogModelToRegistryEntry(model)
			entry.MetadataSource = "model_only_fallback"
			return entry, true
		}
	}
	lower := strings.ToLower(modelID)
	for _, key := range sortedRegistryKeys() {
		entry := KnownModelRegistry[key]
		if strings.HasSuffix(strings.ToLower(key), "/"+lower) {
			entry.MetadataSource = "model_only_fallback"
			return entry, true
		}
	}
	return ModelRegistryEntry{}, false
}

func getModelsDevInfo(provider, modelID string) (ModelRegistryEntry, bool) {
	key := strings.ToLower(provider) + "/" + strings.ToLower(modelID)
	if entry, ok := KnownModelRegistry[key]; ok {
		return entry, true
	}
	return ModelRegistryEntry{}, false
}

func getModelsDevInfoByID(modelID string) (ModelRegistryEntry, bool) {
	lower := strings.ToLower(modelID)
	for _, key := range sortedRegistryKeys() {
		entry := KnownModelRegistry[key]
		if strings.HasSuffix(strings.ToLower(key), "/"+lower) {
			entry.MetadataSource = "model_only_fallback"
			return entry, true
		}
	}
	return ModelRegistryEntry{}, false
}

// GetModelsForProvider returns all registered models for a given provider.
func GetModelsForProvider(provider string) []ModelRegistryEntry {
	prefix := strings.ToLower(provider) + "/"
	var result []ModelRegistryEntry
	seen := map[string]bool{}
	if snapshot, err := catalog.Load(); err == nil {
		for _, model := range snapshot.ModelsForProvider(provider) {
			entry := catalogModelToRegistryEntry(model)
			result = append(result, entry)
			seen[strings.ToLower(entry.ID)] = true
		}
	}
	for _, key := range sortedRegistryKeys() {
		entry := KnownModelRegistry[key]
		if strings.HasPrefix(key, prefix) && !seen[strings.ToLower(entry.ID)] {
			result = append(result, entry)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if len(result[i].ID) != len(result[j].ID) {
			return len(result[i].ID) > len(result[j].ID)
		}
		return result[i].ID < result[j].ID
	})
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
	if !ok && strings.EqualFold(provider, "cloudflare-rest") {
		return ModelPricing{}, false
	}
	if !ok {
		// Try provider-agnostic match
		entry, ok = GetModelInfoByID(modelID)
	}
	if !ok {
		return ModelPricing{}, false
	}

	return ModelPricing{
		Source:           entry.MetadataSource,
		Estimated:        !strings.EqualFold(provider, entry.Provider),
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

func catalogModelToRegistryEntry(model catalog.Model) ModelRegistryEntry {
	return ModelRegistryEntry{
		ID:                       model.ID,
		Name:                     model.Name,
		Provider:                 model.Provider,
		ContextWindow:            model.ContextWindow,
		MaxOutputTokens:          model.MaxTokens,
		SupportsToolCall:         model.SupportsTools,
		SupportsReasoning:        model.Reasoning,
		SupportsStructuredOutput: model.StructuredOutputs,
		SupportsMultimodal:       model.Multimodal,
		InputPricePer1M:          model.Cost.Input,
		OutputPricePer1M:         model.Cost.Output,
		CacheReadPricePer1M:      model.Cost.CacheRead,
		CacheWritePricePer1M:     model.Cost.CacheWrite,
		MetadataSource:           model.Source,
	}
}
