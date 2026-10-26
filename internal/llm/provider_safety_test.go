package llm

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestProviderSafetyTransportTimeoutRetriesWhileCallerRemainsActive(t *testing.T) {
	var calls atomic.Int32
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if calls.Add(1) == 1 {
			select {
			case <-r.Context().Done():
			case <-time.After(250 * time.Millisecond):
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
	}))
	defer fixture.Close()
	cc := openai.DefaultConfig("fixture")
	cc.BaseURL = fixture.URL
	cc.HTTPClient = &http.Client{Transport: &responseTimeoutTransport{base: http.DefaultTransport, timeout: 15 * time.Millisecond}}
	client := WrapOpenAIClient(openai.NewClientWithConfig(cc))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := ExecuteWithCustomRetry(ctx, client, openai.ChatCompletionRequest{Model: "fixture"}, nil, nil, []time.Duration{time.Millisecond}, time.Millisecond)
	if err != nil || calls.Load() != 2 || ctx.Err() != nil {
		t.Fatalf("transport deadline not retried: calls=%d err=%v", calls.Load(), err)
	}
}

func TestProviderSafetyErrorRedactionRetainsClassification(t *testing.T) {
	raw := &url.Error{Op: "Post", URL: "https://alice:password@example.invalid/v1?secret=query#fragment", Err: errors.New("connection reset")}
	for _, diagnostic := range []error{
		errors.New("request https://user:password@status.example.invalid/v1?secret=syntheticSecret#private failed after request"),
		&url.Error{Op: "Get", URL: "https://safe.example.invalid", Err: errors.New("proxy https://user:password@status.example.invalid/v1?secret=syntheticSecret#private refused")},
	} {
		clean := safeAPIError(diagnostic)
		for _, secret := range []string{"user:", "password", "secret=", "syntheticSecret", "#private"} {
			if strings.Contains(clean, secret) {
				t.Fatalf("embedded URL redaction leaked %s", secret)
			}
		}
		if !strings.Contains(clean, "failed after request") && !strings.Contains(clean, "refused") {
			t.Fatal("redaction swallowed the surrounding error")
		}
	}
	original := ClassifyError(raw)
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	client := &safetyPolicyClient{policy: RetryPolicy{PerAttemptTimeout: time.Second, TotalTimeout: 4 * time.Millisecond}, failure: raw}
	_, _ = ExecuteWithRetry(context.Background(), client, openai.ChatCompletionRequest{}, logger, nil)
	output.WriteString(safeAPIError(raw))
	for _, secret := range []string{"alice", "password", "secret=query", "fragment"} {
		if strings.Contains(output.String(), secret) {
			t.Fatalf("diagnostics exposed URL credential component %s", secret)
		}
	}
	if ClassifyError(raw) != original {
		t.Fatal("redaction changed source classification")
	}
	timeout := &TransportTimeoutError{Stage: "fixture", After: time.Second}
	if IsContextError(timeout) || !IsRetryable(timeout) || ClassifyError(timeout) != ErrCategoryTemporaryTransport {
		t.Fatal("owned transport timeout classified as caller cancellation")
	}
}

type safetyPolicyClient struct {
	policy  RetryPolicy
	calls   atomic.Int32
	ctx     context.Context
	failure error
}

func (c *safetyPolicyClient) RetryPolicySnapshot() RetryPolicy { return c.policy.clone() }
func (c *safetyPolicyClient) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.calls.Add(1)
	if c.failure != nil {
		return openai.ChatCompletionResponse{}, c.failure
	}
	<-ctx.Done()
	return openai.ChatCompletionResponse{}, ctx.Err()
}
func (c *safetyPolicyClient) CreateChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (CompletionStream, error) {
	c.calls.Add(1)
	c.ctx = ctx
	return &safetyDelayedStream{ctx: ctx}, nil
}

type safetyDelayedStream struct{ ctx context.Context }

func (s *safetyDelayedStream) Close() error { return nil }
func (s *safetyDelayedStream) Recv() (openai.ChatCompletionStreamResponse, error) {
	select {
	case <-time.After(25 * time.Millisecond):
		return openai.ChatCompletionStreamResponse{Choices: []openai.ChatCompletionStreamChoice{{Index: 0, FinishReason: openai.FinishReasonStop}}}, nil
	case <-s.ctx.Done():
		return openai.ChatCompletionStreamResponse{}, s.ctx.Err()
	}
}
func TestProviderSafetyTotalBudgetAndAcceptedStreams(t *testing.T) {
	policy := RetryPolicy{PerAttemptTimeout: 10 * time.Millisecond, TotalTimeout: 12 * time.Millisecond, Intervals: []time.Duration{time.Second}, FinalInterval: time.Second}
	client := &safetyPolicyClient{policy: policy, failure: errors.New("connection reset")}
	start := time.Now()
	_, err := ExecuteWithRetry(context.Background(), client, openai.ChatCompletionRequest{}, nil, nil)
	if err == nil || time.Since(start) > 250*time.Millisecond || client.calls.Load() != 1 {
		t.Fatalf("overall budget not enforced: %v", err)
	}
	client = &safetyPolicyClient{policy: policy}
	stream, cancel, err := ExecuteStreamWithRetry(context.Background(), client, openai.ChatCompletionRequest{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	defer stream.Close()
	if _, err = stream.Recv(); err != nil || client.calls.Load() != 1 {
		t.Fatalf("accepted stream was cancelled or repeated: %v", err)
	}
}

func TestProviderSafetyManagersKeepIndependentFrozenPolicies(t *testing.T) {
	first := &config.Config{}
	first.CircuitBreaker.RetryIntervals = []string{"1s"}
	first.CircuitBreaker.LLMPerAttemptTimeoutSeconds = 240
	first.CircuitBreaker.LLMTimeoutSeconds = 720
	fm := NewFailoverManager(first, slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer fm.Stop()
	task := NewTaskRouteClient(first, fm, nil)
	second := first.Clone()
	second.CircuitBreaker.RetryIntervals = []string{"9s"}
	second.CircuitBreaker.LLMTimeoutSeconds = 600
	fm.Reconfigure(second)
	if task.RetryPolicySnapshot().Intervals[0] != time.Second || task.RetryPolicySnapshot().TotalTimeout != 720*time.Second || fm.RetryPolicySnapshot().Intervals[0] != 9*time.Second {
		t.Fatal("reconfigure changed a frozen task policy")
	}
	other := NewFailoverManager(&config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer other.Stop()
	if fm.RetryPolicySnapshot().Intervals[0] != 9*time.Second || other.RetryPolicySnapshot().Intervals[0] != 30*time.Second {
		t.Fatal("manager policies interfered")
	}
}

func TestProviderSafetyExactCapabilitiesPrecedeForeignCatalog(t *testing.T) {
	const provider = "fixture-exact-provider"
	const model = "gpt-4o"
	key := provider + "/" + model
	if _, exists := KnownModelRegistry[key]; exists {
		t.Fatal("fixture key already exists")
	}
	KnownModelRegistry[key] = ModelRegistryEntry{ID: model, Provider: provider, SupportsToolCall: false}
	defer delete(KnownModelRegistry, key)
	result, ok := CapabilitiesFromRegistry(provider, model)
	if !ok || result.Source != CapabilitySourceModelsDev || result.ToolCalling {
		t.Fatalf("foreign catalog overrode exact provider metadata: %+v", result)
	}
}

func TestProviderSafetyRegistryFallbackIsStableAndEstimated(t *testing.T) {
	const id = "fixture-provider-safety-unique-model"
	keys := []string{"aaa-fixture/" + id, "zzz-fixture/" + id}
	for _, key := range keys {
		if _, exists := KnownModelRegistry[key]; exists {
			t.Fatal("fixture model already exists")
		}
	}
	KnownModelRegistry[keys[1]] = ModelRegistryEntry{ID: id, Provider: "zzz-fixture", ContextWindow: 200, InputPricePer1M: 2}
	KnownModelRegistry[keys[0]] = ModelRegistryEntry{ID: id, Provider: "aaa-fixture", ContextWindow: 100, InputPricePer1M: 1}
	defer func() {
		for _, key := range keys {
			delete(KnownModelRegistry, key)
		}
	}()
	for range 100 {
		info, ok := GetModelInfoByID(id)
		if !ok || info.Provider != "aaa-fixture" || info.MetadataSource != "model_only_fallback" {
			t.Fatal("unstable or unlabeled fallback")
		}
	}
	price, ok := GetPricingFromRegistry("other-fixture", id)
	if !ok || !price.Estimated || price.Source != "model_only_fallback" {
		t.Fatal("foreign price not labeled as estimate")
	}
	if _, ok := GetPricingFromRegistry("cloudflare-rest", id); ok {
		t.Fatal("Cloudflare REST inherited foreign billing rates")
	}
}

func TestProviderSafetyTaskSelectionUsesOnlyItsOAuthCredential(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.APIKey = "parentStaticKey123456"
	selected := config.ProviderEntry{ID: "oauth-side", Type: "openai", BaseURL: "https://same.example.invalid/v1", AuthType: "oauth2", APIKey: "inactiveStaticKey123456", RuntimeOAuthToken: "sideOAuthToken123456"}
	view := TaskProviderConfig(cfg, selected)
	if view.LLM.APIKey != selected.RuntimeOAuthToken || view.LLM.Provider != selected.ID {
		t.Fatal("selected OAuth route used the parent or inactive static credential")
	}
	selected.RuntimeOAuthToken = ""
	view = TaskProviderConfig(cfg, selected)
	if view.LLM.APIKey != "" || view.LLM.ProviderType != "invalid-provider" {
		t.Fatal("missing OAuth token fell back to another credential")
	}
}

func TestProviderSafetyMissingOrMaskedRouteMakesNoNetworkRequest(t *testing.T) {
	var calls atomic.Int32
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer fixture.Close()
	cfg := &config.Config{}
	cfg.LLM.Provider = "deleted"
	cfg.Providers = []config.ProviderEntry{{ID: "unrelated", Type: "openai"}}
	cfg.ResolveProviders()
	_, err := NewClient(cfg).CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{Model: "fixture"})
	if err == nil || calls.Load() != 0 {
		t.Fatal("missing provider did not fail locally")
	}
	_, err = NewClientFromProviderWithConfig(cfg, "openai", fixture.URL, "••••••••", "").CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{Model: "fixture"})
	if err == nil || calls.Load() != 0 {
		t.Fatal("masked credential reached transport")
	}
}
