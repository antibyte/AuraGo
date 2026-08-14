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
	ConservativeContextWindow = 32768
	ConservativeOutputTokens  = 4096
	ReasoningOutputTokens     = 8192

	modelLimitPositiveTTL = time.Hour
	modelLimitNegativeTTL = 5 * time.Minute
	modelLimitCacheSize   = 128
	modelLimitAsyncProbes = 4
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
	ProbeStatus       string
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

type modelLimitProbeCall struct {
	done       chan struct{}
	value      providerModelLimitProbe
	generation uint64
}

var providerModelLimitCache = struct {
	sync.Mutex
	entries    map[string]modelLimitCacheEntry
	inflight   map[string]*modelLimitProbeCall
	scheduled  map[string]uint64
	generation uint64
}{
	entries:   make(map[string]modelLimitCacheEntry),
	inflight:  make(map[string]*modelLimitProbeCall),
	scheduled: make(map[string]uint64),
}

var providerModelLimitProbeSlots = make(chan struct{}, modelLimitAsyncProbes)

var modelLimitNow = time.Now

// InvalidateModelLimitCache clears provider probe results after provider
// reconfiguration. Registry values are immutable and do not need invalidation.
func InvalidateModelLimitCache() {
	providerModelLimitCache.Lock()
	providerModelLimitCache.entries = make(map[string]modelLimitCacheEntry)
	providerModelLimitCache.inflight = make(map[string]*modelLimitProbeCall)
	providerModelLimitCache.scheduled = make(map[string]uint64)
	providerModelLimitCache.generation++
	providerModelLimitCache.Unlock()
}

// ResolveModelLimits applies override, registry, provider-probe and conservative
// fallback precedence. globalContextCap is an upper bound for every route and,
// for an otherwise unknown primary route, the legacy configured fallback value.
func ResolveModelLimits(ctx context.Context, route ModelRoute, globalContextCap int, logger *slog.Logger) ModelLimits {
	return resolveModelLimits(ctx, route, globalContextCap, logger, true)
}

// ResolveModelLimitsCached resolves effective limits without performing
// network I/O. Unknown routes use a valid cached probe result when available
// and otherwise report a pending probe while retaining conservative limits.
func ResolveModelLimitsCached(route ModelRoute, globalContextCap int) ModelLimits {
	return resolveModelLimits(context.Background(), route, globalContextCap, nil, false)
}

// RefreshModelLimits refreshes a missing or expired provider probe. Concurrent
// refreshes for the same provider identity and model share one network call.
func RefreshModelLimits(ctx context.Context, route ModelRoute, logger *slog.Logger) {
	_ = resolveModelLimits(ctx, route, 0, logger, true)
}

// ScheduleModelLimitRefresh starts at most one asynchronous refresh for a
// route and globally bounds background provider I/O. It is intended for
// latency-sensitive status endpoints; request budgeting continues to use the
// synchronous resolver when no cached metadata exists.
func ScheduleModelLimitRefresh(route ModelRoute, logger *slog.Logger) bool {
	key := providerModelLimitCacheKey(route)
	now := modelLimitNow()
	providerModelLimitCache.Lock()
	if cached, ok := providerModelLimitCache.entries[key]; ok && now.Before(cached.expiresAt) {
		providerModelLimitCache.Unlock()
		return false
	}
	if providerModelLimitCache.inflight[key] != nil {
		providerModelLimitCache.Unlock()
		return false
	}
	if _, ok := providerModelLimitCache.scheduled[key]; ok {
		providerModelLimitCache.Unlock()
		return false
	}
	generation := providerModelLimitCache.generation
	providerModelLimitCache.scheduled[key] = generation
	providerModelLimitCache.Unlock()

	go func() {
		providerModelLimitProbeSlots <- struct{}{}
		defer func() { <-providerModelLimitProbeSlots }()

		providerModelLimitCache.Lock()
		stillCurrent := providerModelLimitCache.generation == generation && providerModelLimitCache.scheduled[key] == generation
		providerModelLimitCache.Unlock()
		if stillCurrent {
			RefreshModelLimits(context.Background(), route, logger)
		}

		providerModelLimitCache.Lock()
		if providerModelLimitCache.scheduled[key] == generation {
			delete(providerModelLimitCache.scheduled, key)
		}
		providerModelLimitCache.Unlock()
	}()
	return true
}

func resolveModelLimits(ctx context.Context, route ModelRoute, globalContextCap int, logger *slog.Logger, allowProbe bool) ModelLimits {
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
		var probe providerModelLimitProbe
		var cacheHit bool
		if allowProbe {
			probe, cacheHit = cachedProviderModelLimitProbe(ctx, route, logger)
			if cacheHit {
				limits.ProbeStatus = "cached"
			} else {
				limits.ProbeStatus = "completed"
			}
		} else {
			probe, cacheHit = lookupCachedProviderModelLimitProbe(route)
			if cacheHit {
				limits.ProbeStatus = "cached"
			} else {
				limits.ProbeStatus = "pending"
			}
		}
		limits.ProbeCacheHit = cacheHit
		if limits.ContextWindow <= 0 && probe.ContextWindow > 0 {
			limits.ContextWindow = probe.ContextWindow
			limits.ContextSource = "provider_probe"
		}
		if limits.MaxOutputTokens <= 0 && probe.MaxOutputTokens > 0 {
			limits.MaxOutputTokens = probe.MaxOutputTokens
			limits.OutputSource = "provider_probe"
		}
	} else {
		limits.ProbeStatus = "not_needed"
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
	key := providerModelLimitCacheKey(route)
	now := modelLimitNow()

	providerModelLimitCache.Lock()
	if cached, ok := providerModelLimitCache.entries[key]; ok && now.Before(cached.expiresAt) {
		cached.lastAccess = now
		providerModelLimitCache.entries[key] = cached
		providerModelLimitCache.Unlock()
		return cached.value, true
	}
	delete(providerModelLimitCache.entries, key)
	if call := providerModelLimitCache.inflight[key]; call != nil {
		providerModelLimitCache.Unlock()
		select {
		case <-ctx.Done():
			return providerModelLimitProbe{}, false
		case <-call.done:
			return call.value, true
		}
	}
	call := &modelLimitProbeCall{
		done:       make(chan struct{}),
		generation: providerModelLimitCache.generation,
	}
	providerModelLimitCache.inflight[key] = call
	providerModelLimitCache.Unlock()

	value := probeProviderModelLimits(ctx, route, logger)
	call.value = value
	ttl := modelLimitNegativeTTL
	if value.ContextWindow > 0 || value.MaxOutputTokens > 0 {
		ttl = modelLimitPositiveTTL
	}
	providerModelLimitCache.Lock()
	if call.generation == providerModelLimitCache.generation && ctx.Err() == nil {
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
	}
	if providerModelLimitCache.inflight[key] == call {
		delete(providerModelLimitCache.inflight, key)
	}
	close(call.done)
	providerModelLimitCache.Unlock()
	return value, false
}

func lookupCachedProviderModelLimitProbe(route ModelRoute) (providerModelLimitProbe, bool) {
	key := providerModelLimitCacheKey(route)
	now := modelLimitNow()
	providerModelLimitCache.Lock()
	defer providerModelLimitCache.Unlock()
	cached, ok := providerModelLimitCache.entries[key]
	if !ok || !now.Before(cached.expiresAt) {
		delete(providerModelLimitCache.entries, key)
		return providerModelLimitProbe{}, false
	}
	cached.lastAccess = now
	providerModelLimitCache.entries[key] = cached
	return cached.value, true
}

func providerModelLimitCacheKey(route ModelRoute) string {
	return strings.ToLower(strings.TrimSpace(route.ProviderID)) + "\x00" +
		strings.ToLower(strings.TrimSpace(route.ProviderType)) + "\x00" +
		strings.TrimRight(strings.TrimSpace(route.BaseURL), "/") + "\x00" +
		strings.ToLower(strings.TrimSpace(route.Model))
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
