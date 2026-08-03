package server

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/llm/catalog"
)

const speechLabChatTurnTokenHeader = "X-AuraGo-Speech-Lab-Turn-Token"

func (s *Server) speechLabTokens() *speechLabTurnTokenRegistry {
	if s == nil {
		return nil
	}
	s.speechLabTurnTokensMu.Lock()
	defer s.speechLabTurnTokensMu.Unlock()
	if s.speechLabTurnTokens == nil {
		s.speechLabTurnTokens = newSpeechLabTurnTokenRegistry(nil)
	}
	return s.speechLabTurnTokens
}

var (
	errSpeechLabChatLLMUnavailable = errors.New("Speech Lab chat LLM is unavailable")
	speechLabChatClientFactory     = func(cfg *config.Config, providerType, baseURL, apiKey, accountID string) llm.ChatClient {
		return llm.NewClientFromProviderWithConfig(cfg, providerType, baseURL, apiKey, accountID)
	}
)

type speechLabRuntimeChatStatus struct {
	Eligible   bool   `json:"eligible"`
	Configured bool   `json:"configured"`
	Reason     string `json:"reason"`
}

func speechLabRuntimeChatProvider(provider *config.ProviderEntry, vault config.SecretReader) (speechLabRuntimeChatStatus, string) {
	return speechLabRuntimeChatProviderWithAuth(provider, vault, time.Now, func() bool {
		auth := llm.GetCopilotAuth()
		return auth != nil && auth.HasGitHubToken()
	})
}

func speechLabRuntimeChatProviderWithAuth(provider *config.ProviderEntry, vault config.SecretReader, now func() time.Time, copilotConfigured func() bool) (speechLabRuntimeChatStatus, string) {
	eligible, reason := config.SpeechLabChatProviderEligibility(provider)
	status := speechLabRuntimeChatStatus{Eligible: eligible, Reason: reason}
	if !eligible || provider == nil {
		return status, ""
	}

	providerType := catalog.NormalizeProviderID(provider.Type)
	if providerType == "workers-ai" && strings.TrimSpace(provider.AccountID) == "" {
		status.Reason = "missing_account_id"
		return status, ""
	}
	if providerType == "copilot" {
		status.Configured = copilotConfigured != nil && copilotConfigured()
		if status.Configured {
			status.Reason = "available"
		} else {
			status.Reason = "copilot_not_authenticated"
		}
		return status, ""
	}

	if normalizeProviderAuthType(provider.AuthType) == "oauth2" {
		if vault == nil {
			status.Reason = "missing_credentials"
			return status, ""
		}
		raw, err := vault.ReadSecret("oauth_" + provider.ID)
		if err != nil || strings.TrimSpace(raw) == "" {
			status.Reason = "missing_credentials"
			return status, ""
		}
		var token config.OAuthToken
		if err := json.Unmarshal([]byte(raw), &token); err != nil || strings.TrimSpace(token.AccessToken) == "" {
			status.Reason = "invalid_oauth_token"
			return status, ""
		}
		if strings.TrimSpace(token.Expiry) != "" {
			expiry, err := time.Parse(time.RFC3339, token.Expiry)
			if err != nil {
				status.Reason = "invalid_oauth_expiry"
				return status, ""
			}
			if now == nil || !now().Before(expiry) {
				status.Reason = "expired_oauth"
				return status, ""
			}
		}
		status.Configured = true
		status.Reason = "available"
		return status, strings.TrimSpace(token.AccessToken)
	}

	apiKey := strings.TrimSpace(provider.APIKey)
	if apiKey == "" && vault != nil {
		if secret, err := vault.ReadSecret("provider_" + provider.ID + "_api_key"); err == nil {
			apiKey = strings.TrimSpace(secret)
		}
	}
	if apiKey != "" || providerTypeWorksWithoutKey(providerType) {
		status.Configured = true
		status.Reason = "available"
		return status, apiKey
	}
	status.Reason = "missing_credentials"
	return status, ""
}

// speechLabChatTurnRuntime returns a turn-local provider snapshot only for a
// direct webchat request explicitly marked after local Speech Lab ASR.
func speechLabChatTurnRuntime(marked bool, isFollowUp bool, missionID string, baseCfg *config.Config, defaultClient llm.ChatClient, vault config.SecretReader) (*config.Config, llm.ChatClient, error) {
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
	status, apiKey := speechLabRuntimeChatProvider(provider, vault)
	if !status.Eligible || !status.Configured {
		return nil, nil, errSpeechLabChatLLMUnavailable
	}

	turnCfg := *baseCfg
	providerType := catalog.NormalizeProviderID(provider.Type)
	turnCfg.LLM.Provider = provider.ID
	turnCfg.LLM.ProviderType = providerType
	turnCfg.LLM.BaseURL = provider.BaseURL
	turnCfg.LLM.APIKey = apiKey
	turnCfg.LLM.AccountID = provider.AccountID
	turnCfg.LLM.Model = provider.Model
	turnCfg.LLM.HelperEnabled = false
	turnCfg.FallbackLLM.Enabled = false
	client := speechLabChatClientFactory(&turnCfg, providerType, provider.BaseURL, apiKey, provider.AccountID)
	if client == nil {
		return nil, nil, errSpeechLabChatLLMUnavailable
	}
	return &turnCfg, client, nil
}
