package server

import (
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/security"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const setupProviderCheckBodyLimit = 16 << 20

// setupProviderCheck is the outcome of the first-run API key check. Only
// metadata endpoints are used; no content is generated and no key is stored.
type setupProviderCheck struct {
	OK              bool
	Code            string
	HTTPStatus      int
	ModelChecked    bool
	CreditRemaining *float64
	FreeTier        *bool
}

// checkSetupProvider verifies the key against the provider's model list,
// confirms the chosen model exists and, for OpenRouter, validates the key on
// the key-only endpoint because /models is public there.
func checkSetupProvider(parent context.Context, s *Server, cfg *config.Config, p config.ProviderEntry) setupProviderCheck {
	release := security.RegisterScopedSensitiveExact(p.APIKey)
	defer release()
	if err := config.ValidateGatewayProvider(p); err != nil {
		return setupProviderCheck{Code: "provider_incomplete"}
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	if s != nil && s.integrationCtx != nil {
		stop := context.AfterFunc(s.integrationCtx, cancel)
		defer stop()
	}
	req, client, err := llm.ProviderMetadataProbe(ctx, cfg, p)
	if err != nil || req == nil || client == nil {
		return setupProviderCheck{Code: "metadata_probe_unavailable"}
	}
	status, body, truncated, err := doSetupProbe(client, req)
	if err != nil {
		if ctx.Err() != nil {
			return setupProviderCheck{Code: "probe_cancelled"}
		}
		return setupProviderCheck{Code: "provider_unreachable"}
	}
	if code := setupProbeStatusCode(status); code != "" {
		return setupProviderCheck{Code: code, HTTPStatus: status}
	}
	result := setupProviderCheck{OK: true, Code: "ok"}
	if !truncated {
		if checked, found := setupModelListContains(body, p.Type, p.Model); checked {
			result.ModelChecked = true
			if !found {
				result.OK = false
				result.Code = "model_not_found"
				return result
			}
		}
	}
	if isOpenRouterProvider(p) {
		keyReq, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(p.BaseURL, "/")+"/key", nil)
		if err != nil {
			return setupProviderCheck{Code: "provider_unreachable"}
		}
		keyReq.Header.Set("Authorization", "Bearer "+p.APIKey)
		keyStatus, keyBody, _, err := doSetupProbe(client, keyReq)
		if err != nil {
			if ctx.Err() != nil {
				return setupProviderCheck{Code: "probe_cancelled"}
			}
			return setupProviderCheck{Code: "provider_unreachable"}
		}
		if code := setupProbeStatusCode(keyStatus); code != "" {
			return setupProviderCheck{Code: code, HTTPStatus: keyStatus}
		}
		remaining, freeTier, ok := parseOpenRouterKeyInfo(keyBody)
		if ok {
			result.CreditRemaining = remaining
			result.FreeTier = &freeTier
		}
	}
	return result
}

func doSetupProbe(client *http.Client, req *http.Request) (status int, body []byte, truncated bool, err error) {
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, false, err
	}
	defer resp.Body.Close()
	body, err = io.ReadAll(io.LimitReader(resp.Body, setupProviderCheckBodyLimit+1))
	if err != nil {
		return resp.StatusCode, nil, false, err
	}
	if len(body) > setupProviderCheckBodyLimit {
		return resp.StatusCode, nil, true, nil
	}
	return resp.StatusCode, body, false, nil
}

func isOpenRouterProvider(p config.ProviderEntry) bool {
	if strings.ToLower(strings.TrimSpace(p.Type)) != "openrouter" {
		return false
	}
	u, err := url.Parse(strings.TrimSpace(p.BaseURL))
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Hostname(), "openrouter.ai")
}

// setupProbeStatusCode maps an HTTP status to a setup error code ("" = success).
func setupProbeStatusCode(status int) string {
	switch {
	case status >= 200 && status < 300:
		return ""
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return "key_invalid"
	case status == http.StatusTooManyRequests:
		return "rate_limited"
	default:
		return "provider_error"
	}
}

// setupModelListContains reports whether the body was a parseable, non-empty
// model list (checked) and whether the model is part of it (found).
func setupModelListContains(body []byte, providerType, model string) (checked, found bool) {
	if providerType == "ollama" {
		var tags struct {
			Models []struct {
				Name  string `json:"name"`
				Model string `json:"model"`
			} `json:"models"`
		}
		if json.Unmarshal(body, &tags) != nil || len(tags.Models) == 0 {
			return false, false
		}
		for _, m := range tags.Models {
			for _, id := range []string{m.Model, m.Name} {
				if id != "" && (id == model || id == model+":latest") {
					return true, true
				}
			}
		}
		return true, false
	}
	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		HasMore bool `json:"has_more"`
	}
	if json.Unmarshal(body, &list) != nil || len(list.Data) == 0 {
		return false, false
	}
	for _, m := range list.Data {
		if m.ID == model {
			return true, true
		}
	}
	// Anthropic pages its model list (20 per page by default); a miss on the
	// first page proves nothing.
	if list.HasMore {
		return false, false
	}
	return true, false
}

// parseOpenRouterKeyInfo reads the /key response. remaining is nil unless
// limit_remaining is a number; ok is false when the body is not a key object.
func parseOpenRouterKeyInfo(body []byte) (remaining *float64, freeTier bool, ok bool) {
	var info struct {
		Data *struct {
			LimitRemaining *float64 `json:"limit_remaining"`
			IsFreeTier     bool     `json:"is_free_tier"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &info) != nil || info.Data == nil {
		return nil, false, false
	}
	return info.Data.LimitRemaining, info.Data.IsFreeTier, true
}
