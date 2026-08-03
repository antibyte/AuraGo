package server

import (
	"context"
	"fmt"
	"strings"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/llm/catalog"
)

type telephoneProviderResolution struct {
	Provider *config.ProviderEntry
	Ready    bool
	Reason   string
}

// resolveTelephoneProvider is the single authentication boundary for the
// Telephone catalog, save validation, live test, and per-call snapshots.
func resolveTelephoneProvider(ctx context.Context, s *Server, cfg *config.Config, providerID string, acquireRuntimeToken bool) telephoneProviderResolution {
	if cfg == nil {
		return telephoneProviderResolution{Reason: "runtime_unavailable"}
	}
	provider := cfg.FindProvider(strings.TrimSpace(providerID))
	if provider == nil {
		return telephoneProviderResolution{Reason: "unknown_provider"}
	}
	status, apiKey := speechLabRuntimeChatProvider(provider, secretReaderForServer(s))
	if !status.Eligible || !status.Configured {
		return telephoneProviderResolution{Reason: status.Reason}
	}
	providerType := catalog.NormalizeProviderID(provider.Type)
	if providerType == "copilot" && acquireRuntimeToken {
		if err := ctx.Err(); err != nil {
			return telephoneProviderResolution{Reason: "cancelled"}
		}
		auth := llm.GetCopilotAuth()
		if auth == nil {
			return telephoneProviderResolution{Reason: "copilot_not_authenticated"}
		}
		var err error
		apiKey, err = auth.GetTokenContext(ctx)
		if err != nil || strings.TrimSpace(apiKey) == "" {
			return telephoneProviderResolution{Reason: "copilot_token_unavailable"}
		}
	}
	copy := *provider
	copy.Type = providerType
	copy.APIKey = apiKey
	return telephoneProviderResolution{Provider: &copy, Ready: true, Reason: "available"}
}

func secretReaderForServer(s *Server) config.SecretReader {
	if s == nil {
		return nil
	}
	return s.Vault
}

func requireTelephoneProvider(ctx context.Context, s *Server, cfg *config.Config, providerID string) (*config.ProviderEntry, error) {
	resolution := resolveTelephoneProvider(ctx, s, cfg, providerID, true)
	if !resolution.Ready || resolution.Provider == nil {
		return nil, fmt.Errorf("telephone provider unavailable: %s", resolution.Reason)
	}
	return resolution.Provider, nil
}
