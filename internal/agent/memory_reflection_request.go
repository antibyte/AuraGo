package agent

import (
	"aurago/internal/config"
	"aurago/internal/llm"
)

func memoryReflectionBudget(cfg *config.Config, selected memoryAnalysisLLMConfig, model string) (*RequestBudget, bool) {
	providerID := cfg.MemoryAnalysis.Provider
	if helper := llm.ResolveHelperLLM(cfg); helper.Enabled && helper.Model != "" {
		providerID = helper.ProviderID
	} else if selected.model == "" {
		providerID = cfg.LLM.Provider
		selected.providerType, selected.baseURL = cfg.LLM.ProviderType, cfg.LLM.BaseURL
	}
	route := llm.ModelRoute{ProviderID: providerID, ProviderType: selected.providerType, BaseURL: selected.baseURL, Model: model}
	provider := config.ProviderEntry{ID: providerID, Type: selected.providerType, Model: model}
	if entry := cfg.FindProvider(providerID); entry != nil {
		provider = *entry
		provider.Model = model
		route.ContextWindowOverride = entry.ContextWindow
		route.MaxOutputTokensOverride = entry.MaxOutputTokens
	}
	limits := llm.ResolveModelLimitsCached(route, cfg.Agent.ContextWindow)
	output := 2200
	if limits.Reasoning {
		output = llm.ReasoningOutputTokens
	}
	return &RequestBudget{
		Routes: []RequestRouteBudget{{Limits: limits}}, CompletionReserve: min(output, limits.MaxOutputTokens),
		SafetyMargin: requestProtocolSafetyTokens,
	}, llm.ResolveProviderCapabilities(provider, llm.CapabilityFallback{}).StructuredOutputs
}

// Reduce independent source records for the single retry; never truncate JSON.
func compactMemoryReflectionInput(input memoryReflectionInput) memoryReflectionInput {
	input.JournalEntries = input.JournalEntries[:min(len(input.JournalEntries), 4)]
	input.CoreMemoryFacts = input.CoreMemoryFacts[:min(len(input.CoreMemoryFacts), 8)]
	input.FrequentErrors = input.FrequentErrors[:min(len(input.FrequentErrors), 4)]
	input.RecentErrors = input.RecentErrors[:min(len(input.RecentErrors), 4)]
	input.LearnedRules = input.LearnedRules[:min(len(input.LearnedRules), 4)]
	input.PreviousReflections = nil
	input.RecentActivity = nil
	input.KnowledgeGraph = Truncate(input.KnowledgeGraph, 1200)
	input.ScopeNote = "retry_with_reduced_source_sample"
	return input
}
