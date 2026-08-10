package llm

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestResolveModelLimitsPrecedenceAndCaps(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tests := []struct {
		name          string
		route         ModelRoute
		globalCap     int
		wantContext   int
		wantOutput    int
		wantCtxSource string
		wantOutSource string
		wantUnknown   bool
		wantCap       bool
		wantReasoning bool
	}{
		{
			name: "provider overrides win",
			route: ModelRoute{ProviderType: "openai", Model: "gpt-4o", Primary: true,
				ContextWindowOverride: 64000, MaxOutputTokensOverride: 12000},
			wantContext: 64000, wantOutput: 12000,
			wantCtxSource: "provider_override", wantOutSource: "provider_override",
		},
		{
			name:        "registry supplies both limits",
			route:       ModelRoute{ProviderType: "openai", Model: "gpt-4o", Primary: true},
			wantContext: 128000, wantOutput: 16384,
			wantCtxSource: "model_registry", wantOutSource: "model_registry",
		},
		{
			name:      "global limit caps known model",
			route:     ModelRoute{ProviderType: "openai", Model: "gpt-4o", Primary: true},
			globalCap: 32000, wantContext: 32000, wantOutput: 16384,
			wantCtxSource: "global_cap", wantOutSource: "model_registry", wantCap: true,
		},
		{
			name:      "unknown primary uses legacy global value",
			route:     ModelRoute{ProviderType: "custom", Model: "unknown-primary", Primary: true},
			globalCap: 12000, wantContext: 12000, wantOutput: ConservativeOutputTokens,
			wantCtxSource: "global_unknown_primary", wantOutSource: "conservative_default", wantUnknown: true,
		},
		{
			name:      "unknown fallback stays conservative",
			route:     ModelRoute{ProviderType: "custom", Model: "unknown-fallback"},
			globalCap: 12000, wantContext: ConservativeContextWindow, wantOutput: ConservativeOutputTokens,
			wantCtxSource: "conservative_default", wantOutSource: "conservative_default", wantUnknown: true,
		},
		{
			name:        "reasoning metadata is retained",
			route:       ModelRoute{ProviderType: "openai", Model: "o3", Primary: true},
			wantContext: 200000, wantOutput: 100000,
			wantCtxSource: "model_registry", wantOutSource: "model_registry", wantReasoning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InvalidateModelLimitCache()
			got := ResolveModelLimits(context.Background(), tt.route, tt.globalCap, logger)
			if got.ContextWindow != tt.wantContext || got.MaxOutputTokens != tt.wantOutput {
				t.Fatalf("limits = %d/%d, want %d/%d", got.ContextWindow, got.MaxOutputTokens, tt.wantContext, tt.wantOutput)
			}
			if got.ContextSource != tt.wantCtxSource || got.OutputSource != tt.wantOutSource {
				t.Fatalf("sources = %q/%q, want %q/%q", got.ContextSource, got.OutputSource, tt.wantCtxSource, tt.wantOutSource)
			}
			if got.Unknown != tt.wantUnknown || got.ContextCapApplied != tt.wantCap || got.Reasoning != tt.wantReasoning {
				t.Fatalf("flags = unknown:%v cap:%v reasoning:%v", got.Unknown, got.ContextCapApplied, got.Reasoning)
			}
		})
	}
}

func TestResolveModelLimitsUsesAndCachesProviderProbe(t *testing.T) {
	InvalidateModelLimitCache()
	defer InvalidateModelLimitCache()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"id":"probe-model","context_length":24576,"max_output_tokens":3072}]}`)
	}))
	defer server.Close()

	route := ModelRoute{ProviderType: "custom", BaseURL: server.URL, Model: "probe-model", Primary: true}
	first := ResolveModelLimits(context.Background(), route, 0, logger)
	second := ResolveModelLimits(context.Background(), route, 0, logger)
	if first.ContextWindow != 24576 || first.MaxOutputTokens != 3072 {
		t.Fatalf("probed limits = %+v", first)
	}
	if first.ContextSource != "provider_probe" || first.OutputSource != "provider_probe" {
		t.Fatalf("probed sources = %q/%q", first.ContextSource, first.OutputSource)
	}
	if second.ProbeCacheHit != true || calls.Load() != 1 {
		t.Fatalf("cache hit=%v calls=%d, want true/1", second.ProbeCacheHit, calls.Load())
	}
}

func TestResolveModelLimitsNegativeProbeTTL(t *testing.T) {
	InvalidateModelLimitCache()
	defer func() {
		modelLimitNow = time.Now
		InvalidateModelLimitCache()
	}()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer server.Close()

	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	modelLimitNow = func() time.Time { return now }
	route := ModelRoute{ProviderType: "custom", BaseURL: server.URL, Model: "missing", Primary: true}
	_ = ResolveModelLimits(context.Background(), route, 0, logger)
	firstCalls := calls.Load()
	_ = ResolveModelLimits(context.Background(), route, 0, logger)
	if calls.Load() != firstCalls {
		t.Fatalf("negative result was not cached: first=%d second=%d", firstCalls, calls.Load())
	}
	now = now.Add(modelLimitNegativeTTL + time.Second)
	_ = ResolveModelLimits(context.Background(), route, 0, logger)
	if calls.Load() <= firstCalls {
		t.Fatalf("negative cache did not expire: first=%d after=%d", firstCalls, calls.Load())
	}
}

func TestResolveModelLimitsHonorsCancellation(t *testing.T) {
	InvalidateModelLimitCache()
	defer InvalidateModelLimitCache()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	got := ResolveModelLimits(ctx, ModelRoute{ProviderType: "custom", BaseURL: server.URL, Model: "cancelled"}, 0, logger)
	if time.Since(started) > time.Second {
		t.Fatal("cancelled probe did not return promptly")
	}
	if got.ContextWindow != ConservativeContextWindow {
		t.Fatalf("context = %d, want conservative fallback", got.ContextWindow)
	}
}

func TestProviderModelLimitProbeCacheStaysBounded(t *testing.T) {
	InvalidateModelLimitCache()
	defer InvalidateModelLimitCache()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for i := 0; i < modelLimitCacheSize+20; i++ {
		ResolveModelLimits(context.Background(), ModelRoute{
			ProviderType: "custom",
			Model:        fmt.Sprintf("unknown-%d", i),
		}, 0, logger)
	}
	providerModelLimitCache.Lock()
	count := len(providerModelLimitCache.entries)
	providerModelLimitCache.Unlock()
	if count > modelLimitCacheSize {
		t.Fatalf("provider model-limit cache size = %d, limit = %d", count, modelLimitCacheSize)
	}
}
