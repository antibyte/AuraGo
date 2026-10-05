package llm

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"

	"github.com/sashabaranov/go-openai"
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

func TestReconfigureRedactsURLAndResetsUnsetRetryControls(t *testing.T) {
	originalIntervals := defaultRetryIntervalsCopy()
	originalTimeout := perAttemptTimeout()
	originalFinalInterval := FinalRetryInterval()
	t.Cleanup(func() {
		defaultRetryIntervalsMu.Lock()
		defaultRetryIntervals = originalIntervals
		defaultRetryIntervalsMu.Unlock()
		SetPerAttemptTimeout(originalTimeout)
		ConfigureFinalRetryInterval(originalFinalInterval)
	})

	ConfigureDefaultRetryIntervals([]string{"9s"}, nil)
	SetPerAttemptTimeout(25 * time.Second)
	ConfigureFinalRetryInterval(5 * time.Second)
	var logOutput bytes.Buffer
	fm := &FailoverManager{
		logger: slog.New(slog.NewTextHandler(&logOutput, nil)),
		stopCh: make(chan struct{}),
		clientFactory: func(*config.Config) *openai.Client {
			return openai.NewClient("test-key")
		},
	}
	cfg := &config.Config{}
	cfg.LLM.BaseURL = "https://alice:password@example.test/v1?token=query-secret#fragment-secret"
	fm.Reconfigure(cfg)
	fm.Stop()

	logs := logOutput.String()
	for _, secret := range []string{"alice", "password", "query-secret", "fragment-secret"} {
		if strings.Contains(logs, secret) {
			t.Fatalf("reconfigure log exposed URL component %q: %s", secret, logs)
		}
	}
	if !strings.Contains(logs, "https://example.test/v1") {
		t.Fatalf("reconfigure log omitted redacted endpoint: %s", logs)
	}
	intervals := defaultRetryIntervalsCopy()
	if len(intervals) != 2 || intervals[0] != 30*time.Second || intervals[1] != 2*time.Minute {
		t.Fatalf("intervals after empty config = %v, want [30s 2m]", intervals)
	}
	if got := perAttemptTimeout(); got != defaultPerAttemptTimeout {
		t.Fatalf("timeout after empty config = %s, want %s", got, defaultPerAttemptTimeout)
	}
	if got := FinalRetryInterval(); got != 30*time.Second {
		t.Fatalf("final retry interval after empty config = %s, want 30s", got)
	}
}

func TestModelRouteFromConfigUsesOverridesOnlyForExactProviderRoute(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers = []config.ProviderEntry{{
		ID: "selected", Type: "ollama", BaseURL: "http://localhost:11434/v1", Model: "small-model",
		ContextWindow: 8192, MaxOutputTokens: 2048,
	}}
	cfg.LLM.Provider = "selected"
	cfg.LLM.ProviderType = "ollama"
	cfg.LLM.BaseURL = "http://localhost:11434/v1/"
	cfg.LLM.Model = "small-model"

	route := modelRouteFromConfig(cfg, false)
	if route.ContextWindowOverride != 8192 || route.MaxOutputTokensOverride != 2048 {
		t.Fatalf("exact route overrides = (%d, %d), want (8192, 2048)", route.ContextWindowOverride, route.MaxOutputTokensOverride)
	}

	cfg.LLM.Model = "job-model-override"
	route = modelRouteFromConfig(cfg, false)
	if route.ContextWindowOverride != 0 || route.MaxOutputTokensOverride != 0 {
		t.Fatalf("model override inherited provider limits: (%d, %d)", route.ContextWindowOverride, route.MaxOutputTokensOverride)
	}

	cfg.LLM.Model = "small-model"
	cfg.LLM.BaseURL = "http://other-host:11434/v1"
	route = modelRouteFromConfig(cfg, false)
	if route.ContextWindowOverride != 0 || route.MaxOutputTokensOverride != 0 {
		t.Fatalf("endpoint override inherited provider limits: (%d, %d)", route.ContextWindowOverride, route.MaxOutputTokensOverride)
	}
}

func TestFailoverRecordErrorSkipsRateLimit(t *testing.T) {
	fm := &FailoverManager{
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		errorThreshold: 1,
		errorCount:     0,
	}

	fm.recordError(&openai.APIError{HTTPStatusCode: http.StatusTooManyRequests, Message: "rate limited"})

	if fm.errorCount != 0 {
		t.Fatalf("errorCount = %d, want 0 for rate limit errors", fm.errorCount)
	}
	if fm.isOnFallback {
		t.Fatal("rate limit error should not switch to fallback")
	}
}

type immediateFailoverTestError struct{}

func (immediateFailoverTestError) Error() string           { return "managed local runtime unavailable" }
func (immediateFailoverTestError) ImmediateFailover() bool { return true }

type failoverRoundTripper func(*http.Request) (*http.Response, error)

func (fn failoverRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type failoverStreamBody struct {
	payload string
	sent    bool
}

func (body *failoverStreamBody) Read(buffer []byte) (int, error) {
	if body.sent {
		return 0, errors.New("stream interrupted after first chunk")
	}
	body.sent = true
	return copy(buffer, body.payload), nil
}

func (*failoverStreamBody) Close() error { return nil }

func failoverTestClient(transport http.RoundTripper) *openai.Client {
	cfg := openai.DefaultConfig("test-key")
	cfg.BaseURL = "http://failover.test/v1"
	cfg.HTTPClient = &http.Client{Transport: transport}
	return openai.NewClientWithConfig(cfg)
}

func completionResponse(content string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"id":"test","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"` +
				content + `"},"finish_reason":"stop"}]}`,
		)),
	}
}

func TestFailoverRecordErrorImmediatelySwitchesFromManagedLocalPrimary(t *testing.T) {
	fm := &FailoverManager{
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		fallback:       openai.NewClient("regular-provider-key"),
		fallbackModel:  "regular-large-model",
		errorThreshold: 5,
	}

	fm.recordError(immediateFailoverTestError{})

	if !fm.isOnFallback {
		t.Fatal("typed managed-runtime error did not switch immediately")
	}
	if fm.errorCount != 0 {
		t.Fatalf("errorCount = %d, want 0 after immediate switch", fm.errorCount)
	}
}

func TestImmediateLocalFailureRetriesSameCompletionExactlyOnce(t *testing.T) {
	primaryCalls := 0
	fallbackCalls := 0
	fm := &FailoverManager{
		primary: failoverTestClient(failoverRoundTripper(func(*http.Request) (*http.Response, error) {
			primaryCalls++
			return nil, immediateFailoverTestError{}
		})),
		fallback: failoverTestClient(failoverRoundTripper(func(*http.Request) (*http.Response, error) {
			fallbackCalls++
			return completionResponse("fallback-ok"), nil
		})),
		primaryModel:  "aurago-qwen",
		fallbackType:  "openai",
		fallbackModel: "gpt-4o",
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	response, err := fm.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "hello"}},
	})
	if err != nil || len(response.Choices) != 1 || response.Choices[0].Message.Content != "fallback-ok" {
		t.Fatalf("completion response=%#v err=%v", response, err)
	}
	if primaryCalls != 1 || fallbackCalls != 1 || !fm.isOnFallback {
		t.Fatalf("calls primary=%d fallback=%d onFallback=%v", primaryCalls, fallbackCalls, fm.isOnFallback)
	}
}

func TestImmediateLocalFailureRetriesStreamCreationOnly(t *testing.T) {
	fallbackCalls := 0
	fm := &FailoverManager{
		primary: failoverTestClient(failoverRoundTripper(func(*http.Request) (*http.Response, error) {
			return nil, immediateFailoverTestError{}
		})),
		fallback: failoverTestClient(failoverRoundTripper(func(*http.Request) (*http.Response, error) {
			fallbackCalls++
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body: io.NopCloser(strings.NewReader(
					"data: {\"id\":\"test\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"fallback\"}}]}\n\n" +
						"data: [DONE]\n\n",
				)),
			}, nil
		})),
		primaryModel:  "aurago-qwen",
		fallbackType:  "openai",
		fallbackModel: "gpt-4o",
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	stream, err := fm.CreateChatCompletionStream(context.Background(), openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	chunk, err := stream.Recv()
	if err != nil || len(chunk.Choices) != 1 || chunk.Choices[0].Delta.Content != "fallback" {
		t.Fatalf("stream chunk=%#v err=%v", chunk, err)
	}
	if fallbackCalls != 1 {
		t.Fatalf("fallback calls=%d, want 1", fallbackCalls)
	}
}

func TestStreamFailureAfterFirstByteDoesNotRetryFallback(t *testing.T) {
	fallbackCalls := 0
	fm := &FailoverManager{
		primary: failoverTestClient(failoverRoundTripper(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:       &failoverStreamBody{payload: "data: {\"id\":\"test\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"first\"}}]}\n\n"},
			}, nil
		})),
		fallback: failoverTestClient(failoverRoundTripper(func(*http.Request) (*http.Response, error) {
			fallbackCalls++
			return completionResponse("must-not-run"), nil
		})),
		primaryModel:   "aurago-qwen",
		fallbackType:   "openai",
		fallbackModel:  "gpt-4o",
		errorThreshold: 3,
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	stream, err := fm.CreateChatCompletionStream(context.Background(), openai.ChatCompletionRequest{})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	if _, err := stream.Recv(); err != nil {
		t.Fatalf("first streamed byte failed: %v", err)
	}
	if _, err := stream.Recv(); err == nil {
		t.Fatal("expected stream failure after first byte")
	}
	if fallbackCalls != 0 || fm.isOnFallback || fm.errorCount != 1 {
		t.Fatalf("stream failure state: fallback calls=%d onFallback=%v errorCount=%d", fallbackCalls, fm.isOnFallback, fm.errorCount)
	}
}

func TestRepeatedPostOpenStreamErrorsReachFailoverThresholdWithoutReplay(t *testing.T) {
	fallbackCalls := 0
	fm := &FailoverManager{
		primary: failoverTestClient(failoverRoundTripper(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:       &failoverStreamBody{payload: "data: {\"id\":\"test\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"first\"}}]}\n\n"},
			}, nil
		})),
		fallback: failoverTestClient(failoverRoundTripper(func(*http.Request) (*http.Response, error) {
			fallbackCalls++
			return completionResponse("must-not-run"), nil
		})),
		primaryModel:   "primary-model",
		fallbackType:   "openai",
		fallbackModel:  "fallback-model",
		errorThreshold: 3,
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	for i := 0; i < fm.errorThreshold; i++ {
		stream, err := fm.CreateChatCompletionStream(context.Background(), openai.ChatCompletionRequest{})
		if err != nil {
			t.Fatalf("stream %d creation failed: %v", i+1, err)
		}
		if _, err := stream.Recv(); err != nil {
			t.Fatalf("stream %d first chunk failed: %v", i+1, err)
		}
		if _, err := stream.Recv(); err == nil {
			t.Fatalf("stream %d unexpectedly completed", i+1)
		}
		_ = stream.Close()
		if i+1 < fm.errorThreshold && fm.isOnFallback {
			t.Fatalf("switched before threshold after %d errors", i+1)
		}
	}
	if !fm.isOnFallback || fallbackCalls != 0 {
		t.Fatalf("threshold outcome: onFallback=%v fallbackCalls=%d, want true/0 (no replay)", fm.isOnFallback, fallbackCalls)
	}
}

func TestStreamOpenDoesNotCountAsSuccessAndFinishMarkerResetsErrors(t *testing.T) {
	fm := &FailoverManager{
		primary: failoverTestClient(failoverRoundTripper(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body: io.NopCloser(strings.NewReader(
					"data: {\"id\":\"test\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
						"data: [DONE]\n\n",
				)),
			}, nil
		})),
		primaryModel:   "primary-model",
		errorThreshold: 3,
		errorCount:     2,
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	stream, err := fm.CreateChatCompletionStream(context.Background(), openai.ChatCompletionRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if fm.errorCount != 2 {
		t.Fatalf("opening an HTTP 200 stream reset errors to %d; want 2 until a finish marker", fm.errorCount)
	}
	if _, err := stream.Recv(); err != nil {
		t.Fatalf("finish chunk failed: %v", err)
	}
	if fm.errorCount != 0 {
		t.Fatalf("finish marker left error count at %d, want 0", fm.errorCount)
	}
	if _, err := stream.Recv(); err != io.EOF {
		t.Fatalf("post-finish Recv error = %v, want EOF", err)
	}
	if fm.errorCount != 0 {
		t.Fatalf("normal EOF after finish changed error count to %d", fm.errorCount)
	}
}

func TestStreamOutcomeFromOldConfigDoesNotAffectReconfiguredRoute(t *testing.T) {
	originalIntervals := defaultRetryIntervalsCopy()
	originalTimeout := perAttemptTimeout()
	t.Cleanup(func() {
		defaultRetryIntervalsMu.Lock()
		defaultRetryIntervals = originalIntervals
		defaultRetryIntervalsMu.Unlock()
		SetPerAttemptTimeout(originalTimeout)
	})

	fm := &FailoverManager{
		primary: failoverTestClient(failoverRoundTripper(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body:       &failoverStreamBody{payload: "data: {\"id\":\"old\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"first\"}}]}\n\n"},
			}, nil
		})),
		primaryModel:   "old-model",
		errorThreshold: 3,
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		stopCh:         make(chan struct{}),
		clientFactory: func(*config.Config) *openai.Client {
			return failoverTestClient(failoverRoundTripper(func(*http.Request) (*http.Response, error) {
				return completionResponse("new-route"), nil
			}))
		},
	}
	stream, err := fm.CreateChatCompletionStream(context.Background(), openai.ChatCompletionRequest{})
	if err != nil {
		t.Fatal(err)
	}
	fm.Reconfigure(&config.Config{})
	defer fm.Stop()
	fm.errorCount = 2
	if _, err := stream.Recv(); err != nil {
		t.Fatalf("old stream first chunk failed: %v", err)
	}
	if _, err := stream.Recv(); err == nil {
		t.Fatal("expected old stream to fail")
	}
	if fm.errorCount != 2 || fm.isOnFallback {
		t.Fatalf("stale stream changed current route state: errors=%d fallback=%v", fm.errorCount, fm.isOnFallback)
	}
}

func TestImmediateFallbackDoesNotRetryCancelledOrIncompatibleRequest(t *testing.T) {
	fm := &FailoverManager{
		fallback:      failoverTestClient(failoverRoundTripper(func(*http.Request) (*http.Response, error) { t.Fatal("fallback called"); return nil, nil })),
		fallbackType:  "openai",
		fallbackModel: "gpt-3.5-turbo",
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	imageRequest := openai.ChatCompletionRequest{Messages: []openai.ChatCompletionMessage{{
		Role: openai.ChatMessageRoleUser,
		MultiContent: []openai.ChatMessagePart{{
			Type:     openai.ChatMessagePartTypeImageURL,
			ImageURL: &openai.ChatMessageImageURL{URL: "https://example.invalid/image.png"},
		}},
	}}}
	if _, _, retry := fm.immediateFallbackForRequest(immediateFailoverTestError{}, false, imageRequest); retry {
		t.Fatal("incompatible multimodal request was retried")
	}
	if _, _, retry := fm.immediateFallbackForRequest(context.Canceled, false, openai.ChatCompletionRequest{}); retry {
		t.Fatal("cancelled request was retried")
	}
}

func TestImmediateFallbackFailureIsNotRetriedAgain(t *testing.T) {
	fallbackCalls := 0
	fm := &FailoverManager{
		primary: failoverTestClient(failoverRoundTripper(func(*http.Request) (*http.Response, error) {
			return nil, immediateFailoverTestError{}
		})),
		fallback: failoverTestClient(failoverRoundTripper(func(*http.Request) (*http.Response, error) {
			fallbackCalls++
			return nil, errors.New("fallback unavailable")
		})),
		primaryModel:  "aurago-qwen",
		fallbackType:  "openai",
		fallbackModel: "gpt-4o",
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	if _, err := fm.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{}); err == nil {
		t.Fatal("expected fallback failure")
	}
	if fallbackCalls != 1 {
		t.Fatalf("fallback calls=%d, want exactly 1", fallbackCalls)
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

func TestFallbackSupportsFeaturesUsesRegistryMultimodal(t *testing.T) {
	fm := &FailoverManager{
		fallbackType:  "openai",
		fallbackModel: "gpt-4o",
	}
	req := openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{Type: openai.ChatMessagePartTypeImageURL, ImageURL: &openai.ChatMessageImageURL{URL: "https://example.invalid/x.png"}},
				},
			},
		},
	}
	if !fm.fallbackSupportsFeatures(req) {
		t.Fatal("expected gpt-4o fallback to support image input")
	}

	fm.fallbackModel = "gpt-3.5-turbo"
	if fm.fallbackSupportsFeatures(req) {
		t.Fatal("expected gpt-3.5-turbo fallback to reject image input when registry says non-multimodal")
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

func TestLLMHTTPClientHasResponseAndHeaderTimeouts(t *testing.T) {
	client := buildLLMHTTPClient(&config.Config{}, "minimax", "", "https://api.example.test/v1")
	if client == nil {
		t.Fatal("buildLLMHTTPClient returned nil")
	}
	bounded, ok := client.Transport.(*responseTimeoutTransport)
	if client.Timeout != 0 || !ok || bounded.timeout != 3*time.Minute {
		t.Fatal("expected stream-aware response timeout instead of a global stream cutoff")
	}
	transport, ok := unwrapLLMTransport(client.Transport).(*http.Transport)
	if !ok {
		t.Fatalf("base transport = %T, want *http.Transport", unwrapLLMTransport(client.Transport))
	}
	if transport.ResponseHeaderTimeout <= 0 {
		t.Fatal("expected response header timeout")
	}
	if bounded.timeout < transport.ResponseHeaderTimeout {
		t.Fatalf("response timeout %s is smaller than response header timeout %s", bounded.timeout, transport.ResponseHeaderTimeout)
	}
}

func TestProviderSpecificHeaderTimeoutDoesNotExceedClientTimeout(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.AdaptiveTools.ProviderProfilesEnabled = true
	client := buildLLMHTTPClient(cfg, "minimax", "", "https://api.minimax.io/v1")
	if client == nil {
		t.Fatal("buildLLMHTTPClient returned nil")
	}
	transport, ok := unwrapLLMTransport(client.Transport).(*http.Transport)
	if !ok {
		t.Fatalf("base transport = %T, want *http.Transport", unwrapLLMTransport(client.Transport))
	}
	// ResponseHeaderTimeout is now capped at perAttemptTimeout() so the
	// transport never times out before the retry context does.
	minExpected := perAttemptTimeout()
	if minExpected <= 0 {
		minExpected = 120 * time.Second
	}
	if transport.ResponseHeaderTimeout < minExpected {
		t.Fatalf("ResponseHeaderTimeout = %s, want at least %s", transport.ResponseHeaderTimeout, minExpected)
	}
	if client.Transport.(*responseTimeoutTransport).timeout < transport.ResponseHeaderTimeout {
		t.Fatal("response timeout is smaller than response header timeout")
	}

	cfg.Agent.AdaptiveTools.ProviderProfilesEnabled = false
	client = buildLLMHTTPClient(cfg, "minimax", "", "https://api.minimax.io/v1")
	transport, ok = unwrapLLMTransport(client.Transport).(*http.Transport)
	if !ok {
		t.Fatalf("base transport = %T, want *http.Transport", unwrapLLMTransport(client.Transport))
	}
	if transport.ResponseHeaderTimeout < minExpected {
		t.Fatalf("opt-out ResponseHeaderTimeout = %s, want at least %s", transport.ResponseHeaderTimeout, minExpected)
	}
}

func unwrapLLMTransport(rt http.RoundTripper) http.RoundTripper {
	for {
		switch t := rt.(type) {
		case *providerURLTransport:
			rt = t.base
		case *usageObservationTransport:
			rt = t.base
		case *leadingSystemTransport:
			rt = t.base
		case *responseTimeoutTransport:
			rt = t.base
		case *miniMaxTransport:
			rt = t.base
		case *openAIPromptCacheTransport:
			rt = t.base
		case *aiGatewayAuthTransport:
			rt = t.base
		case *anthropicTransport:
			rt = t.base
		case *loggingTransport:
			rt = t.base
		case *rateLimitAwareTransport:
			rt = t.base
		default:
			return rt
		}
	}
}
