package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	ConservativeContextWindow = 8192
	ConservativeOutputTokens  = 4096
	ReasoningOutputTokens     = 8192

	modelLimitPositiveTTL = time.Hour
	modelLimitNegativeTTL = 5 * time.Minute
	modelLimitCacheSize   = 128
)

// ModelRoute describes one provider/model combination that may serve a request.
// APIKey is used only for a bounded metadata probe and must never be logged.
type ModelRoute struct {
	ProviderID              string
	ProviderType            string
	BaseURL                 string
	APIKey                  string
	Model                   string
	ContextWindowOverride   int
	MaxOutputTokensOverride int
	Primary                 bool
}

// ModelLimits is the effective, request-safe limit set for a route.
type ModelLimits struct {
	Route             ModelRoute
	ContextWindow     int
	MaxOutputTokens   int
	ContextSource     string
	OutputSource      string
	Reasoning         bool
	ContextCapApplied bool
	Unknown           bool
	Warning           string
	ProbeCacheHit     bool
}

type providerModelLimitProbe struct {
	ContextWindow   int
	MaxOutputTokens int
}

type modelLimitCacheEntry struct {
	value      providerModelLimitProbe
	expiresAt  time.Time
	lastAccess time.Time
}

var providerModelLimitCache = struct {
	sync.Mutex
	entries map[string]modelLimitCacheEntry
}{entries: make(map[string]modelLimitCacheEntry)}

var modelLimitNow = time.Now

// InvalidateModelLimitCache clears provider probe results after provider
// reconfiguration. Registry values are immutable and do not need invalidation.
func InvalidateModelLimitCache() {
	providerModelLimitCache.Lock()
	providerModelLimitCache.entries = make(map[string]modelLimitCacheEntry)
	providerModelLimitCache.Unlock()
}

// ResolveModelLimits applies override, registry, provider-probe and conservative
// fallback precedence. globalContextCap is an upper bound for every route and,
// for an otherwise unknown primary route, the legacy configured fallback value.
func ResolveModelLimits(ctx context.Context, route ModelRoute, globalContextCap int, logger *slog.Logger) ModelLimits {
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		logger = slog.Default()
	}

	limits := ModelLimits{Route: route}
	registry, registryOK := resolveRegistryModelMetadata(route.ProviderType, route.Model)
	limits.Reasoning = registryOK && registry.SupportsReasoning

	if route.ContextWindowOverride > 0 {
		limits.ContextWindow = route.ContextWindowOverride
		limits.ContextSource = "provider_override"
	} else if registryOK && registry.ContextWindow > 0 {
		limits.ContextWindow = registry.ContextWindow
		limits.ContextSource = "model_registry"
	} else if contextWindow, ok := lookupKnownContextWindow(route.Model); ok {
		limits.ContextWindow = contextWindow
		limits.ContextSource = "model_registry"
	}

	if route.MaxOutputTokensOverride > 0 {
		limits.MaxOutputTokens = route.MaxOutputTokensOverride
		limits.OutputSource = "provider_override"
	} else if registryOK && registry.MaxOutputTokens > 0 {
		limits.MaxOutputTokens = registry.MaxOutputTokens
		limits.OutputSource = "model_registry"
	}

	if limits.ContextWindow <= 0 || limits.MaxOutputTokens <= 0 {
		probe, cacheHit := cachedProviderModelLimitProbe(ctx, route, logger)
		limits.ProbeCacheHit = cacheHit
		if limits.ContextWindow <= 0 && probe.ContextWindow > 0 {
			limits.ContextWindow = probe.ContextWindow
			limits.ContextSource = "provider_probe"
		}
		if limits.MaxOutputTokens <= 0 && probe.MaxOutputTokens > 0 {
			limits.MaxOutputTokens = probe.MaxOutputTokens
			limits.OutputSource = "provider_probe"
		}
	}

	unknownContext := limits.ContextWindow <= 0
	unknownOutput := limits.MaxOutputTokens <= 0
	if unknownContext && route.Primary && globalContextCap > 0 {
		limits.ContextWindow = globalContextCap
		limits.ContextSource = "global_unknown_primary"
	}
	if limits.ContextWindow <= 0 {
		limits.ContextWindow = ConservativeContextWindow
		limits.ContextSource = "conservative_default"
	}
	if limits.MaxOutputTokens <= 0 {
		limits.MaxOutputTokens = ConservativeOutputTokens
		limits.OutputSource = "conservative_default"
	}

	if globalContextCap > 0 && limits.ContextWindow > globalContextCap {
		limits.ContextWindow = globalContextCap
		limits.ContextSource = "global_cap"
		limits.ContextCapApplied = true
	}

	limits.Unknown = (unknownContext && route.ContextWindowOverride <= 0) ||
		(unknownOutput && route.MaxOutputTokensOverride <= 0)
	if limits.Unknown {
		limits.Warning = "unknown model limits; unresolved values use conservative defaults and agent.context_window may cap the primary route"
	}
	return limits
}

func resolveRegistryModelMetadata(provider, model string) (ModelRegistryEntry, bool) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	model = strings.TrimSpace(model)
	if entry, ok := GetModelInfo(provider, model); ok {
		return entry, true
	}
	if slash := strings.Index(model, "/"); slash > 0 && slash+1 < len(model) {
		modelProvider := strings.ToLower(strings.TrimSpace(model[:slash]))
		modelID := strings.TrimSpace(model[slash+1:])
		if entry, ok := GetModelInfo(modelProvider, modelID); ok {
			return entry, true
		}
		if entry, ok := GetModelInfoByID(modelID); ok {
			return entry, true
		}
	}
	if entry, ok := GetModelInfoByID(model); ok {
		return entry, true
	}
	return ModelRegistryEntry{}, false
}

func cachedProviderModelLimitProbe(ctx context.Context, route ModelRoute, logger *slog.Logger) (providerModelLimitProbe, bool) {
	key := strings.ToLower(strings.TrimSpace(route.ProviderType)) + "\x00" +
		strings.TrimRight(strings.TrimSpace(route.BaseURL), "/") + "\x00" + strings.ToLower(strings.TrimSpace(route.Model))
	now := modelLimitNow()

	providerModelLimitCache.Lock()
	if cached, ok := providerModelLimitCache.entries[key]; ok && now.Before(cached.expiresAt) {
		cached.lastAccess = now
		providerModelLimitCache.entries[key] = cached
		providerModelLimitCache.Unlock()
		return cached.value, true
	}
	delete(providerModelLimitCache.entries, key)
	providerModelLimitCache.Unlock()

	value := probeProviderModelLimits(ctx, route, logger)
	ttl := modelLimitNegativeTTL
	if value.ContextWindow > 0 || value.MaxOutputTokens > 0 {
		ttl = modelLimitPositiveTTL
	}
	providerModelLimitCache.Lock()
	providerModelLimitCache.entries[key] = modelLimitCacheEntry{value: value, expiresAt: now.Add(ttl), lastAccess: now}
	for len(providerModelLimitCache.entries) > modelLimitCacheSize {
		oldestKey := ""
		var oldest time.Time
		for candidate, entry := range providerModelLimitCache.entries {
			if oldestKey == "" || entry.lastAccess.Before(oldest) {
				oldestKey, oldest = candidate, entry.lastAccess
			}
		}
		delete(providerModelLimitCache.entries, oldestKey)
	}
	providerModelLimitCache.Unlock()
	return value, false
}

func probeProviderModelLimits(ctx context.Context, route ModelRoute, logger *slog.Logger) providerModelLimitProbe {
	provider := strings.ToLower(strings.TrimSpace(route.ProviderType))
	if strings.TrimSpace(route.BaseURL) == "" || strings.TrimSpace(route.Model) == "" || provider == "anthropic" {
		return providerModelLimitProbe{}
	}
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if provider == "ollama" {
		return probeOllamaModelLimits(probeCtx, route, logger)
	}
	return probeOpenAICompatibleModelLimits(probeCtx, route, logger)
}

func probeOllamaModelLimits(ctx context.Context, route ModelRoute, logger *slog.Logger) providerModelLimitProbe {
	base := strings.TrimSuffix(strings.TrimRight(route.BaseURL, "/"), "/v1")
	payload, err := json.Marshal(map[string]string{"name": route.Model})
	if err != nil {
		return providerModelLimitProbe{}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/show", bytes.NewReader(payload))
	if err != nil {
		return providerModelLimitProbe{}
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Debug("[ModelLimits] Ollama metadata probe failed", "model", route.Model, "error", err)
		return providerModelLimitProbe{}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return providerModelLimitProbe{}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return providerModelLimitProbe{}
	}
	var decoded struct {
		ModelInfo map[string]json.RawMessage `json:"model_info"`
	}
	if json.Unmarshal(body, &decoded) != nil {
		return providerModelLimitProbe{}
	}
	for key, raw := range decoded.ModelInfo {
		if key == "context_length" || strings.HasSuffix(key, ".context_length") {
			var value int
			if json.Unmarshal(raw, &value) == nil && value > 0 {
				return providerModelLimitProbe{ContextWindow: value}
			}
		}
	}
	return providerModelLimitProbe{}
}

func probeOpenAICompatibleModelLimits(ctx context.Context, route ModelRoute, logger *slog.Logger) providerModelLimitProbe {
	base := strings.TrimRight(route.BaseURL, "/")
	stripped := strings.TrimSuffix(base, "/v1")
	var candidates []string
	if strings.HasSuffix(stripped, "/api") {
		candidates = append(candidates, stripped+"/v1/models")
	} else {
		candidates = append(candidates, stripped+"/api/v1/models", stripped+"/v1/models", base+"/models")
	}
	for _, endpoint := range candidates {
		if err := ctx.Err(); err != nil {
			return providerModelLimitProbe{}
		}
		if limits, ok := queryModelLimitsEndpoint(ctx, endpoint, route.APIKey, route.Model, logger); ok {
			return limits
		}
	}
	return providerModelLimitProbe{}
}

func queryModelLimitsEndpoint(ctx context.Context, endpoint, apiKey, model string, logger *slog.Logger) (providerModelLimitProbe, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return providerModelLimitProbe{}, false
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Debug("[ModelLimits] Provider metadata probe failed", "model", model, "error", err)
		return providerModelLimitProbe{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return providerModelLimitProbe{}, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return providerModelLimitProbe{}, false
	}
	var decoded struct {
		Data []struct {
			ID                  string `json:"id"`
			ContextLength       int    `json:"context_length"`
			MaxCompletionTokens int    `json:"max_completion_tokens"`
			MaxOutputTokens     int    `json:"max_output_tokens"`
			TopProvider         struct {
				MaxCompletionTokens int `json:"max_completion_tokens"`
			} `json:"top_provider"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &decoded) != nil {
		return providerModelLimitProbe{}, false
	}
	for _, candidate := range decoded.Data {
		if !strings.EqualFold(strings.TrimSpace(candidate.ID), strings.TrimSpace(model)) {
			continue
		}
		output := candidate.MaxOutputTokens
		if output <= 0 {
			output = candidate.MaxCompletionTokens
		}
		if output <= 0 {
			output = candidate.TopProvider.MaxCompletionTokens
		}
		return providerModelLimitProbe{ContextWindow: candidate.ContextLength, MaxOutputTokens: output}, true
	}
	return providerModelLimitProbe{}, false
}
