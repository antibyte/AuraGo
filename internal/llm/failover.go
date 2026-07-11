package llm

import (
	"context"
	"log/slog"
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

	isOnFallback       bool
	errorCount         int
	fallbackErrorCount int
	errorThreshold     int
	probeInterval      time.Duration
	generation         int

	stopCh chan struct{}

	logger *slog.Logger
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

func NewFailoverManager(cfg *config.Config, logger *slog.Logger) *FailoverManager {
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
	primary := NewClient(cfg)

	fm := &FailoverManager{
		primary:        primary,
		primaryType:    cfg.LLM.ProviderType,
		primaryModel:   cfg.LLM.Model,
		primaryBaseURL: cfg.LLM.BaseURL,
		primaryAPIKey:  cfg.LLM.APIKey,
		errorThreshold: 3,
		probeInterval:  60 * time.Second,
		stopCh:         make(chan struct{}),
		logger:         logger,
	}

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
	fm.fallback = NewClient(&fallbackCfg)
	fm.fallbackType = fb.ProviderType
	fm.fallbackModel = fb.Model

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

	newPrimary := NewClient(cfg)
	newStopCh := make(chan struct{})

	fm.mu.Lock()
	fm.primary = newPrimary
	fm.primaryType = cfg.LLM.ProviderType
	fm.primaryModel = cfg.LLM.Model
	fm.primaryBaseURL = cfg.LLM.BaseURL
	fm.primaryAPIKey = cfg.LLM.APIKey
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
		fm.fallback = NewClient(&fallbackCfg)
		fm.fallbackType = fb.ProviderType
		fm.fallbackModel = fb.Model
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
	}
	fm.mu.Unlock()

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
		fm.recordError(err)
	} else {
		fm.recordSuccess()
	}
	return stream, err
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

func (fm *FailoverManager) fallbackSupportsFeatures(req openai.ChatCompletionRequest) bool {
	fm.mu.RLock()
	fallbackType := fm.fallbackType
	fallbackModel := fm.fallbackModel
	fm.mu.RUnlock()

	if fallbackModel == "" {
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
	if !hasImage {
		return true
	}

	if _, _, _, multimodal, ok := GetCapabilitiesFromRegistry(fallbackType, fallbackModel); ok {
		return multimodal
	}

	caps := ResolveProviderCapabilities(config.ProviderEntry{
		ID:    fallbackType,
		Type:  fallbackType,
		Model: fallbackModel,
	}, CapabilityFallback{})
	if caps.Known {
		return caps.Multimodal
	}
	return false
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
