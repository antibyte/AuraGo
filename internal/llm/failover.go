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

	primary       *openai.Client
	fallback      *openai.Client
	primaryType   string
	fallbackType  string
	primaryModel  string
	fallbackModel string

	isOnFallback       bool
	errorCount         int
	fallbackErrorCount int
	errorThreshold     int
	probeInterval      time.Duration

	stopCh chan struct{}

	logger *slog.Logger
}

func NewFailoverManager(cfg *config.Config, logger *slog.Logger) *FailoverManager {
	if cfg != nil && cfg.CircuitBreaker.LLMPerAttemptTimeoutSeconds > 0 {
		SetPerAttemptTimeout(time.Duration(cfg.CircuitBreaker.LLMPerAttemptTimeoutSeconds) * time.Second)
	}
	primary := NewClient(cfg)

	fm := &FailoverManager{
		primary:        primary,
		primaryType:    cfg.LLM.ProviderType,
		primaryModel:   cfg.LLM.Model,
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
	if cfg != nil && cfg.CircuitBreaker.LLMPerAttemptTimeoutSeconds > 0 {
		SetPerAttemptTimeout(time.Duration(cfg.CircuitBreaker.LLMPerAttemptTimeoutSeconds) * time.Second)
	}
	fm.Stop()

	newPrimary := NewClient(cfg)
	newStopCh := make(chan struct{})

	fm.mu.Lock()
	fm.primary = newPrimary
	fm.primaryType = cfg.LLM.ProviderType
	fm.primaryModel = cfg.LLM.Model
	fm.isOnFallback = false
	fm.errorCount = 0
	fm.fallbackErrorCount = 0
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
	client, model := fm.active()
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
	client, model := fm.active()
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

func (fm *FailoverManager) active() (*openai.Client, string) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	if fm.isOnFallback {
		return fm.fallback, fm.fallbackModel
	}
	return fm.primary, fm.primaryModel
}

func (fm *FailoverManager) ActiveProviderAndModel() (string, string) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	if fm.isOnFallback {
		return fm.fallbackType, fm.fallbackModel
	}
	return fm.primaryType, fm.primaryModel
}

func (fm *FailoverManager) recordError(err error) {
	if err == nil || IsContextError(err) {
		return
	}

	if IsNonRetryable(err) {
		fm.logger.Error("[LLM] Non-retryable error, not counting towards failover", "error", err, "category", ClassifyError(err))
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
	if !fm.isOnFallback {
		fm.errorCount = 0
	}
}

func (fm *FailoverManager) probeLoop(stopCh <-chan struct{}) {
	ticker := time.NewTicker(fm.probeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			fm.logger.Debug("LLM failover: probe loop stopped")
			return
		case <-ticker.C:
			fm.mu.RLock()
			onFallback := fm.isOnFallback
			fm.mu.RUnlock()

			if !onFallback {
				continue
			}

			fm.logger.Debug("LLM failover: probing primary endpoint...")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

			fm.mu.RLock()
			primaryClient := fm.primary
			primaryModel := fm.primaryModel
			fm.mu.RUnlock()

			// Prefer a lightweight health check that doesn't consume tokens.
			// ListModels works for OpenAI, OpenRouter, and some custom providers.
			// For providers that don't support it, we skip directly to the minimal
			// completion probe to avoid wasting tokens on a failing ListModels call.
			var err error
			if fm.primaryType == "openai" || fm.primaryType == "openrouter" || fm.primaryType == "custom" {
				_, err = primaryClient.ListModels(ctx)
			}
			if err != nil {
				_, err = primaryClient.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
					Model: primaryModel,
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

			fm.mu.Lock()
			fm.isOnFallback = false
			fm.errorCount = 0
			fm.fallbackErrorCount = 0
			fm.mu.Unlock()
			fm.logger.Info("LLM failover: primary recovered - switched back", "model", fm.primaryModel)
		}
	}
}
