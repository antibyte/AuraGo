package llm

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"

	"github.com/sashabaranov/go-openai"
)

type FailoverManager struct {
	mu sync.RWMutex

	primary        *openai.Client
	fallback       *openai.Client
	primaryType    string
	fallbackType   string
	primaryModel   string
	fallbackModel  string
	primaryBaseURL string
	primaryAPIKey  string
	primaryRoute   ModelRoute
	fallbackRoute  ModelRoute

	isOnFallback       bool
	errorCount         int
	fallbackErrorCount int
	errorThreshold     int
	probeInterval      time.Duration
	generation         int

	stopCh chan struct{}

	logger *slog.Logger

	clientFactory func(*config.Config) *openai.Client
}

type failoverProbeSnapshot struct {
	generation     int
	onFallback     bool
	primaryClient  *openai.Client
	primaryType    string
	primaryModel   string
	primaryBaseURL string
	primaryAPIKey  string
}

// FailoverOption customizes client construction without changing existing callers.
type FailoverOption func(*FailoverManager)

// WithClientFactory supplies clients for managed runtimes while preserving normal provider behavior.
func WithClientFactory(factory func(*config.Config) *openai.Client) FailoverOption {
	return func(manager *FailoverManager) {
		if factory != nil {
			manager.clientFactory = factory
		}
	}
}

func NewFailoverManager(cfg *config.Config, logger *slog.Logger, options ...FailoverOption) *FailoverManager {
	if cfg != nil {
		if cfg.CircuitBreaker.LLMPerAttemptTimeoutSeconds > 0 {
			// Enforce a floor of 120s so large-prompt scenarios (Virtual Desktop)
			// never get a dangerously short per-attempt timeout from old configs.
			timeout := time.Duration(cfg.CircuitBreaker.LLMPerAttemptTimeoutSeconds) * time.Second
			if timeout < 120*time.Second {
				timeout = 120 * time.Second
			}
			SetPerAttemptTimeout(timeout)
		}
		ConfigureDefaultRetryIntervals(cfg.CircuitBreaker.RetryIntervals, logger)
		cfg.CircuitBreaker.FinalRetryInterval = configureFinalRetryInterval(cfg.CircuitBreaker.FinalRetryInterval, logger)
	}
	fm := &FailoverManager{
		primaryType:    cfg.LLM.ProviderType,
		primaryModel:   cfg.LLM.Model,
		primaryBaseURL: cfg.LLM.BaseURL,
		primaryAPIKey:  cfg.LLM.APIKey,
		errorThreshold: 3,
		probeInterval:  60 * time.Second,
		stopCh:         make(chan struct{}),
		logger:         logger,
		clientFactory:  NewClient,
	}
	fm.primaryRoute = modelRouteFromConfig(cfg, false)
	for _, option := range options {
		option(fm)
	}
	fm.primary = fm.clientFactory(cfg)

	fb := cfg.FallbackLLM
	if !fb.Enabled || (fb.BaseURL == "" && fb.AccountID == "") {
		return fm
	}

	fallbackCfg := *cfg
	fallbackCfg.LLM.ProviderType = fb.ProviderType
	fallbackCfg.LLM.BaseURL = fb.BaseURL
	fallbackCfg.LLM.APIKey = fb.APIKey
	fallbackCfg.LLM.Model = fb.Model
	fallbackCfg.LLM.AccountID = fb.AccountID
	fm.fallback = fm.clientFactory(&fallbackCfg)
	fm.fallbackType = fb.ProviderType
	fm.fallbackModel = fb.Model
	fm.fallbackRoute = modelRouteFromConfig(cfg, true)

	if fb.ErrorThreshold > 0 {
		fm.errorThreshold = fb.ErrorThreshold
	}
	if fb.ProbeIntervalSeconds > 0 {
		fm.probeInterval = time.Duration(fb.ProbeIntervalSeconds) * time.Second
	}

	go fm.probeLoop(fm.stopCh)
	return fm
}

func (fm *FailoverManager) Reconfigure(cfg *config.Config) {
	if cfg != nil {
		if cfg.CircuitBreaker.LLMPerAttemptTimeoutSeconds > 0 {
			// Enforce a floor of 120s so large-prompt scenarios (Virtual Desktop)
			// never get a dangerously short per-attempt timeout from old configs.
			timeout := time.Duration(cfg.CircuitBreaker.LLMPerAttemptTimeoutSeconds) * time.Second
			if timeout < 120*time.Second {
				timeout = 120 * time.Second
			}
			SetPerAttemptTimeout(timeout)
		}
		ConfigureDefaultRetryIntervals(cfg.CircuitBreaker.RetryIntervals, fm.logger)
		cfg.CircuitBreaker.FinalRetryInterval = configureFinalRetryInterval(cfg.CircuitBreaker.FinalRetryInterval, fm.logger)
	}
	fm.Stop()

	newPrimary := fm.clientFactory(cfg)
	newStopCh := make(chan struct{})

	fm.mu.Lock()
	fm.primary = newPrimary
	fm.primaryType = cfg.LLM.ProviderType
	fm.primaryModel = cfg.LLM.Model
	fm.primaryBaseURL = cfg.LLM.BaseURL
	fm.primaryAPIKey = cfg.LLM.APIKey
	fm.primaryRoute = modelRouteFromConfig(cfg, false)
	fm.isOnFallback = false
	fm.errorCount = 0
	fm.fallbackErrorCount = 0
	fm.generation++
	fm.stopCh = newStopCh

	fb := cfg.FallbackLLM
	startProbe := false
	if fb.Enabled && (fb.BaseURL != "" || fb.AccountID != "") {
		fallbackCfg := *cfg
		fallbackCfg.LLM.ProviderType = fb.ProviderType
		fallbackCfg.LLM.BaseURL = fb.BaseURL
		fallbackCfg.LLM.APIKey = fb.APIKey
		fallbackCfg.LLM.Model = fb.Model
		fallbackCfg.LLM.AccountID = fb.AccountID
		fm.fallback = fm.clientFactory(&fallbackCfg)
		fm.fallbackType = fb.ProviderType
		fm.fallbackModel = fb.Model
		fm.fallbackRoute = modelRouteFromConfig(cfg, true)
		if fb.ErrorThreshold > 0 {
			fm.errorThreshold = fb.ErrorThreshold
		}
		if fb.ProbeIntervalSeconds > 0 {
			fm.probeInterval = time.Duration(fb.ProbeIntervalSeconds) * time.Second
		}
		startProbe = true
	} else {
		fm.fallback = nil
		fm.fallbackType = ""
		fm.fallbackModel = ""
		fm.fallbackRoute = ModelRoute{}
	}
	fm.mu.Unlock()
	InvalidateModelLimitCache()

	if startProbe {
		go fm.probeLoop(newStopCh)
	}
	fm.logger.Info("[LLM] FailoverManager reconfigured", "model", cfg.LLM.Model, "provider", cfg.LLM.ProviderType, "base_url", cfg.LLM.BaseURL)
}

func (fm *FailoverManager) Stop() {
	select {
	case <-fm.stopCh:
	default:
		close(fm.stopCh)
	}
}

func (fm *FailoverManager) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	client, model, onFallback := fm.active()
	if onFallback && !fm.fallbackSupportsFeatures(req) {
		fm.logger.Warn("[LLM] Fallback model does not support request features, using primary", "fallback_model", model)
		fm.mu.RLock()
		client = fm.primary
		model = fm.primaryModel
		fm.mu.RUnlock()
	}
	reqCopy := req
	reqCopy.Model = model

	resp, err := client.CreateChatCompletion(ctx, reqCopy)
	if err != nil {
		if fallbackClient, fallbackModel, retry := fm.immediateFallbackForRequest(err, onFallback, req); retry {
			reqCopy.Model = fallbackModel
			resp, err = fallbackClient.CreateChatCompletion(ctx, reqCopy)
			if err != nil {
				fm.recordError(err)
			} else {
				fm.recordSuccess()
			}
			return resp, err
		}
		fm.recordError(err)
	} else {
		fm.recordSuccess()
	}
	return resp, err
}

func (fm *FailoverManager) CreateChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	client, model, onFallback := fm.active()
	if onFallback && !fm.fallbackSupportsFeatures(req) {
		fm.logger.Warn("[LLM] Fallback model does not support request features, using primary", "fallback_model", model)
		fm.mu.RLock()
		client = fm.primary
		model = fm.primaryModel
		fm.mu.RUnlock()
	}
	reqCopy := req
	reqCopy.Model = model

	stream, err := client.CreateChatCompletionStream(ctx, reqCopy)
	if err != nil {
		if fallbackClient, fallbackModel, retry := fm.immediateFallbackForRequest(err, onFallback, req); retry {
			reqCopy.Model = fallbackModel
			stream, err = fallbackClient.CreateChatCompletionStream(ctx, reqCopy)
			if err != nil {
				fm.recordError(err)
			} else {
				fm.recordSuccess()
			}
			return stream, err
		}
		fm.recordError(err)
	} else {
		fm.recordSuccess()
	}
	return stream, err
}

func (fm *FailoverManager) immediateFallbackForRequest(
	err error,
	wasOnFallback bool,
	req openai.ChatCompletionRequest,
) (*openai.Client, string, bool) {
	if err == nil || wasOnFallback || IsContextError(err) || !fm.fallbackSupportsFeatures(req) {
		return nil, "", false
	}
	var immediate interface{ ImmediateFailover() bool }
	if !errors.As(err, &immediate) || !immediate.ImmediateFailover() {
		return nil, "", false
	}
	fm.mu.Lock()
	defer fm.mu.Unlock()
	if fm.fallback == nil || fm.isOnFallback {
		return nil, "", false
	}
	fm.logger.Warn("LLM failover: managed local runtime unavailable, retrying current request on fallback",
		"model", fm.fallbackModel, "error", err)
	fm.isOnFallback = true
	fm.errorCount = 0
	return fm.fallback, fm.fallbackModel, true
}

func (fm *FailoverManager) active() (*openai.Client, string, bool) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	if fm.isOnFallback && fm.fallback != nil {
		return fm.fallback, fm.fallbackModel, true
	}
	return fm.primary, fm.primaryModel, false
}

func (fm *FailoverManager) ActiveProviderAndModel() (string, string) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	if fm.isOnFallback {
		return fm.fallbackType, fm.fallbackModel
	}
	return fm.primaryType, fm.primaryModel
}

// CandidateRoutes returns every route that can serve this request. The active
// route is listed first, but callers must budget against all returned routes.
func (fm *FailoverManager) CandidateRoutes(req openai.ChatCompletionRequest) []ModelRoute {
	fm.mu.RLock()
	primary := fm.primaryRoute
	fallback := fm.fallbackRoute
	onFallback := fm.isOnFallback
	hasFallback := fm.fallback != nil && strings.TrimSpace(fallback.Model) != ""
	fm.mu.RUnlock()

	fallbackUsable := hasFallback && routeSupportsRequest(fallback, req)
	routes := make([]ModelRoute, 0, 2)
	if onFallback && fallbackUsable {
		routes = append(routes, fallback)
	}
	if strings.TrimSpace(primary.Model) != "" {
		routes = append(routes, primary)
	}
	if !onFallback && fallbackUsable {
		routes = append(routes, fallback)
	}
	return routes
}

func (fm *FailoverManager) fallbackSupportsFeatures(req openai.ChatCompletionRequest) bool {
	fm.mu.RLock()
	route := fm.fallbackRoute
	if strings.TrimSpace(route.Model) == "" {
		// Compatibility for focused tests and embedders that construct a manager
		// directly instead of going through NewFailoverManager.
		route = ModelRoute{
			ProviderID:   fm.fallbackType,
			ProviderType: fm.fallbackType,
			Model:        fm.fallbackModel,
		}
	}
	fm.mu.RUnlock()
	return routeSupportsRequest(route, req)
}

func routeSupportsRequest(route ModelRoute, req openai.ChatCompletionRequest) bool {
	if strings.TrimSpace(route.Model) == "" {
		return false
	}

	hasImage := false
	for _, m := range req.Messages {
		for _, part := range m.MultiContent {
			if part.Type == openai.ChatMessagePartTypeImageURL {
				hasImage = true
				break
			}
		}
		if hasImage {
			break
		}
	}
	requiresTools := len(req.Tools) > 0
	requiresStructured := req.ResponseFormat != nil &&
		req.ResponseFormat.Type != "" &&
		req.ResponseFormat.Type != openai.ChatCompletionResponseFormatTypeText
	if !hasImage && !requiresTools && !requiresStructured {
		return true
	}

	caps := ResolveProviderCapabilities(config.ProviderEntry{
		ID:    route.ProviderID,
		Type:  route.ProviderType,
		Model: route.Model,
	}, CapabilityFallback{})
	if !caps.Known {
		// Preserve existing custom-provider behavior for text/tool requests.
		// Unknown vision remains fail-closed because replaying image content to
		// a text-only model is predictably incompatible.
		return !hasImage
	}
	return (!hasImage || caps.Multimodal) &&
		(!requiresTools || caps.ToolCalling) &&
		(!requiresStructured || caps.StructuredOutputs)
}

func modelRouteFromConfig(cfg *config.Config, fallback bool) ModelRoute {
	if cfg == nil {
		return ModelRoute{}
	}
	if fallback {
		route := ModelRoute{
			ProviderID:   cfg.FallbackLLM.Provider,
			ProviderType: cfg.FallbackLLM.ProviderType,
			BaseURL:      cfg.FallbackLLM.BaseURL,
			APIKey:       cfg.FallbackLLM.APIKey,
			Model:        cfg.FallbackLLM.Model,
		}
		if provider := cfg.FindProvider(cfg.FallbackLLM.Provider); provider != nil {
			route.ContextWindowOverride = provider.ContextWindow
			route.MaxOutputTokensOverride = provider.MaxOutputTokens
		}
		return route
	}
	route := ModelRoute{
		ProviderID:   cfg.LLM.Provider,
		ProviderType: cfg.LLM.ProviderType,
		BaseURL:      cfg.LLM.BaseURL,
		APIKey:       cfg.LLM.APIKey,
		Model:        cfg.LLM.Model,
		Primary:      true,
	}
	if provider := cfg.FindProvider(cfg.LLM.Provider); provider != nil {
		route.ContextWindowOverride = provider.ContextWindow
		route.MaxOutputTokensOverride = provider.MaxOutputTokens
	}
	return route
}

func (fm *FailoverManager) recordError(err error) {
	if err == nil || IsContextError(err) {
		return
	}

	if IsNonRetryable(err) {
		fm.logger.Error("[LLM] Non-retryable error, not counting towards failover", "error", err, "category", ClassifyError(err))
		return
	}

	if IsRateLimit(err) {
		fm.logger.Debug("[LLM] Rate limit error, not counting towards failover", "error", err)
		return
	}

	var immediate interface{ ImmediateFailover() bool }
	if errors.As(err, &immediate) && immediate.ImmediateFailover() {
		fm.mu.Lock()
		defer fm.mu.Unlock()
		if fm.fallback != nil && !fm.isOnFallback {
			fm.logger.Warn("LLM failover: managed local runtime unavailable, switching immediately",
				"model", fm.fallbackModel, "error", err)
			fm.isOnFallback = true
			fm.errorCount = 0
		}
		return
	}

	fm.mu.Lock()
	defer fm.mu.Unlock()

	fm.errorCount++
	if fm.fallback == nil || fm.isOnFallback {
		if fm.isOnFallback {
			fm.fallbackErrorCount++
			fm.logger.Warn("LLM failover: error on fallback endpoint", "error", err, "fallback_error_count", fm.fallbackErrorCount)
			if fm.fallbackErrorCount >= fm.errorThreshold*3 {
				fm.logger.Error("LLM failover: fallback endpoint has excessive errors, consider reconfiguring", "fallback_error_count", fm.fallbackErrorCount, "threshold", fm.errorThreshold*3)
			}
		}
		return
	}

	fm.logger.Warn("LLM failover: primary error recorded", "error", err, "count", fm.errorCount, "threshold", fm.errorThreshold)
	if fm.errorCount >= fm.errorThreshold {
		fm.logger.Warn("LLM failover: switching to fallback endpoint", "model", fm.fallbackModel)
		fm.isOnFallback = true
		fm.errorCount = 0
	}
}

func (fm *FailoverManager) recordSuccess() {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.errorCount = 0
	fm.fallbackErrorCount = 0
}

func (fm *FailoverManager) probeLoop(stopCh <-chan struct{}) {
	fm.mu.RLock()
	interval := fm.probeInterval
	fm.mu.RUnlock()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			fm.logger.Debug("LLM failover: probe loop stopped")
			return
		case <-ticker.C:
			snapshot := fm.primaryProbeSnapshot()
			if !snapshot.onFallback {
				continue
			}

			fm.logger.Debug("LLM failover: probing primary endpoint...")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

			// Safe: primaryClient is a pointer copy. Even if Reconfigure()
			// replaces fm.primary after the unlock, the old pointer still
			// references the original client and remains valid for the probe.
			// The local copy is used (not fm.primary) for all HTTP calls.

			err := probePrimaryHealth(ctx, snapshot.primaryClient, snapshot.primaryType, snapshot.primaryBaseURL, snapshot.primaryAPIKey)
			if err != nil {
				fm.logger.Debug("LLM failover: token-free probe failed, trying minimal completion fallback", "error", err)
				_, err = snapshot.primaryClient.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
					Model: snapshot.primaryModel,
					Messages: []openai.ChatCompletionMessage{
						{Role: openai.ChatMessageRoleUser, Content: "ok"},
					},
					MaxTokens: 1,
				})
			}
			cancel()

			if err != nil {
				if IsContextError(err) {
					fm.logger.Debug("LLM failover: primary probe context error (inconclusive)", "error", err)
					continue
				}
				fm.logger.Debug("LLM failover: primary still unavailable", "error", err)
				continue
			}

			fm.completePrimaryProbe(snapshot.generation, snapshot.primaryModel)
		}
	}
}

func (fm *FailoverManager) primaryProbeSnapshot() failoverProbeSnapshot {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	return failoverProbeSnapshot{
		generation:     fm.generation,
		onFallback:     fm.isOnFallback,
		primaryClient:  fm.primary,
		primaryType:    fm.primaryType,
		primaryModel:   fm.primaryModel,
		primaryBaseURL: fm.primaryBaseURL,
		primaryAPIKey:  fm.primaryAPIKey,
	}
}

func (fm *FailoverManager) completePrimaryProbe(generation int, model string) {
	fm.mu.Lock()
	if generation != fm.generation {
		fm.mu.Unlock()
		fm.logger.Debug("LLM failover: ignoring stale primary probe", "probe_generation", generation, "current_generation", fm.generation)
		return
	}
	fm.isOnFallback = false
	fm.errorCount = 0
	fm.fallbackErrorCount = 0
	fm.mu.Unlock()
	fm.logger.Info("LLM failover: primary recovered - switched back", "model", model)
}
