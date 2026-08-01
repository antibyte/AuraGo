package server

import (
	"errors"
	"strings"

	"aurago/internal/config"
	"aurago/internal/llm"
)

const speechLabChatInputHeader = "X-AuraGo-Speech-Lab-Input"

var (
	errSpeechLabChatLLMUnavailable = errors.New("Speech Lab chat LLM is unavailable")
	speechLabChatClientFactory     = func(cfg *config.Config, providerType, baseURL, apiKey, accountID string) llm.ChatClient {
		return llm.NewClientFromProviderWithConfig(cfg, providerType, baseURL, apiKey, accountID)
	}
)

// speechLabChatTurnRuntime returns a turn-local provider snapshot only for a
// direct webchat request explicitly marked after local Speech Lab ASR.
func speechLabChatTurnRuntime(marked bool, isFollowUp bool, missionID string, baseCfg *config.Config, defaultClient llm.ChatClient) (*config.Config, llm.ChatClient, error) {
	if baseCfg == nil {
		return nil, nil, errSpeechLabChatLLMUnavailable
	}
	if !marked || isFollowUp || strings.TrimSpace(missionID) != "" ||
		!baseCfg.SpeechLab.Active() || !baseCfg.SpeechLab.ChatInputEnabled {
		return baseCfg, defaultClient, nil
	}

	providerID := strings.TrimSpace(baseCfg.SpeechLab.ChatLLMProviderID)
	if providerID == "" {
		return baseCfg, defaultClient, nil
	}
	provider := baseCfg.FindProvider(providerID)
	if provider == nil || strings.TrimSpace(provider.Model) == "" {
		return nil, nil, errSpeechLabChatLLMUnavailable
	}
	if strings.TrimSpace(provider.APIKey) == "" && !isLocalTelephoneProvider(provider.Type) {
		return nil, nil, errSpeechLabChatLLMUnavailable
	}

	turnCfg := *baseCfg
	turnCfg.LLM.Provider = provider.ID
	turnCfg.LLM.ProviderType = provider.Type
	turnCfg.LLM.BaseURL = provider.BaseURL
	turnCfg.LLM.APIKey = provider.APIKey
	turnCfg.LLM.AccountID = provider.AccountID
	turnCfg.LLM.Model = provider.Model
	turnCfg.FallbackLLM.Enabled = false
	client := speechLabChatClientFactory(&turnCfg, provider.Type, provider.BaseURL, provider.APIKey, provider.AccountID)
	if client == nil {
		return nil, nil, errSpeechLabChatLLMUnavailable
	}
	return &turnCfg, client, nil
}
