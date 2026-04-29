package llm

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestFailoverManagerActiveProviderAndModelTracksFallback(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{}
	cfg.LLM.ProviderType = "openrouter"
	cfg.LLM.Model = "primary-model"
	cfg.FallbackLLM.Enabled = true
	cfg.FallbackLLM.ProviderType = "ollama"
	cfg.FallbackLLM.Model = "fallback-model"
	cfg.FallbackLLM.BaseURL = "http://localhost:11434/v1"
	cfg.FallbackLLM.ErrorThreshold = 1
	cfg.FallbackLLM.ProbeIntervalSeconds = 3600

	fm := NewFailoverManager(cfg, logger)
	defer fm.Stop()

	providerType, model := fm.ActiveProviderAndModel()
	if providerType != "openrouter" || model != "primary-model" {
		t.Fatalf("initial active endpoint = (%q, %q), want (%q, %q)", providerType, model, "openrouter", "primary-model")
	}

	fm.recordError(errors.New("connection timeout"))

	providerType, model = fm.ActiveProviderAndModel()
	if providerType != "ollama" || model != "fallback-model" {
		t.Fatalf("fallback active endpoint = (%q, %q), want (%q, %q)", providerType, model, "ollama", "fallback-model")
	}
}

func TestFailoverRecordSuccessResetsFallbackErrorCounter(t *testing.T) {
	fm := &FailoverManager{
		logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
		isOnFallback:       true,
		errorCount:         2,
		fallbackErrorCount: 5,
	}

	fm.recordSuccess()

	if fm.errorCount != 0 {
		t.Fatalf("errorCount = %d, want 0", fm.errorCount)
	}
	if fm.fallbackErrorCount != 0 {
		t.Fatalf("fallbackErrorCount = %d, want 0", fm.fallbackErrorCount)
	}
}

func TestFailoverStalePrimaryProbeDoesNotSwitchBack(t *testing.T) {
	fm := &FailoverManager{
		logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
		isOnFallback:       true,
		errorCount:         1,
		fallbackErrorCount: 2,
		generation:         2,
	}

	fm.completePrimaryProbe(1, "old-primary")

	if !fm.isOnFallback {
		t.Fatal("stale probe switched back to primary")
	}
	if fm.errorCount != 1 || fm.fallbackErrorCount != 2 {
		t.Fatalf("stale probe reset counters: error=%d fallback=%d", fm.errorCount, fm.fallbackErrorCount)
	}
}

func TestLLMHTTPClientHasGlobalAndHeaderTimeouts(t *testing.T) {
	client := buildLLMHTTPClient(&config.Config{}, "minimax", "", "https://api.example.test/v1")
	if client == nil {
		t.Fatal("buildLLMHTTPClient returned nil")
	}
	if client.Timeout <= 0 {
		t.Fatal("expected global HTTP client timeout")
	}
	transport, ok := unwrapLLMTransport(client.Transport).(*http.Transport)
	if !ok {
		t.Fatalf("base transport = %T, want *http.Transport", unwrapLLMTransport(client.Transport))
	}
	if transport.ResponseHeaderTimeout <= 0 {
		t.Fatal("expected response header timeout")
	}
	if client.Timeout < transport.ResponseHeaderTimeout {
		t.Fatalf("client timeout %s is smaller than response header timeout %s", client.Timeout, transport.ResponseHeaderTimeout)
	}
	if client.Timeout > 5*time.Minute {
		t.Fatalf("client timeout %s is unexpectedly large", client.Timeout)
	}
}

func unwrapLLMTransport(rt http.RoundTripper) http.RoundTripper {
	for {
		switch t := rt.(type) {
		case *miniMaxTransport:
			rt = t.base
		case *openAIPromptCacheTransport:
			rt = t.base
		case *aiGatewayAuthTransport:
			rt = t.base
		case *anthropicTransport:
			rt = t.base
		default:
			return rt
		}
	}
}
