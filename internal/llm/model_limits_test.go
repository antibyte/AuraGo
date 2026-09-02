package llm

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
			name:        "agnes registry supplies current limits",
			route:       ModelRoute{ProviderType: "agnes", Model: "agnes-2.5-flash", Primary: true},
			wantContext: 512000, wantOutput: 65536,
			wantCtxSource: "model_registry", wantOutSource: "model_registry", wantReasoning: true,
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
			name:        "unknown fallback stays conservative",
			route:       ModelRoute{ProviderType: "custom", Model: "unknown-fallback"},
			wantContext: 32768, wantOutput: ConservativeOutputTokens,
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

func TestManagedLingLimitsBeforeRuntimeStarts(t *testing.T) {
	var probes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		probes.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	for _, primary := range []bool{true, false} {
		for _, globalCap := range []int{131072, 8192} {
			route := ModelRoute{ProviderType: "aurago-local", Model: "aurago-ling", BaseURL: server.URL, Primary: primary}
			limits := ResolveModelLimits(context.Background(), route, globalCap, nil)
			if limits.ContextWindow != min(16384, globalCap) || limits.MaxOutputTokens != 4096 || limits.Unknown || limits.Reasoning {
				t.Fatalf("wrong managed Ling limits: %+v", limits)
			}
			if !strings.Contains(limits.MetadataSource, "managed_runtime") || limits.ProbeStatus != "not_needed" {
				t.Fatalf("managed runtime depended on a probe: %+v", limits)
			}
		}
	}
	if probes.Load() != 0 {
		t.Fatal("budgeting a stopped managed model contacted the inference server")
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

func TestResolveModelLimitsSingleflightsConcurrentProviderProbe(t *testing.T) {
	InvalidateModelLimitCache()
	defer InvalidateModelLimitCache()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		time.Sleep(40 * time.Millisecond)
		fmt.Fprint(w, `{"data":[{"id":"shared-model","context_length":32768,"max_output_tokens":2048}]}`)
	}))
	defer server.Close()

	route := ModelRoute{ProviderID: "shared", ProviderType: "custom", BaseURL: server.URL, Model: "shared-model"}
	var wg sync.WaitGroup
	results := make(chan ModelLimits, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- ResolveModelLimits(context.Background(), route, 0, logger)
		}()
	}
	wg.Wait()
	close(results)
	for result := range results {
		if result.ContextWindow != 32768 || result.MaxOutputTokens != 2048 {
			t.Fatalf("singleflight result = %+v", result)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("provider probe calls = %d, want 1", got)
	}
}

func TestScheduleModelLimitRefreshBoundsBackgroundProviderIO(t *testing.T) {
	InvalidateModelLimitCache()
	defer InvalidateModelLimitCache()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	const routeCount = 10
	var models strings.Builder
	models.WriteString(`{"data":[`)
	for i := 0; i < routeCount; i++ {
		if i > 0 {
			models.WriteByte(',')
		}
		fmt.Fprintf(&models, `{"id":"async-%d","context_length":32768,"max_output_tokens":2048}`, i)
	}
	models.WriteString(`]}`)

	var active atomic.Int32
	var maximum atomic.Int32
	var calls atomic.Int32
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		current := active.Add(1)
		for {
			observed := maximum.Load()
			if current <= observed || maximum.CompareAndSwap(observed, current) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		active.Add(-1)
		_, _ = io.WriteString(w, models.String())
		if calls.Add(1) == routeCount {
			close(done)
		}
	}))
	defer server.Close()

	for i := 0; i < routeCount; i++ {
		if !ScheduleModelLimitRefresh(ModelRoute{
			ProviderID: fmt.Sprintf("provider-%d", i), ProviderType: "custom", BaseURL: server.URL, Model: fmt.Sprintf("async-%d", i),
		}, logger) {
			t.Fatalf("route %d was not scheduled", i)
		}
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("background probes did not complete; calls=%d", calls.Load())
	}
	if got := maximum.Load(); got > modelLimitAsyncProbes {
		t.Fatalf("concurrent background probes = %d, limit = %d", got, modelLimitAsyncProbes)
	}
}

func TestProviderModelLimitCacheSeparatesProviderIdentities(t *testing.T) {
	InvalidateModelLimitCache()
	defer InvalidateModelLimitCache()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		fmt.Fprint(w, `{"data":[{"id":"same-model","context_length":16384,"max_output_tokens":1024}]}`)
	}))
	defer server.Close()

	for _, providerID := range []string{"tenant-a", "tenant-b"} {
		_ = ResolveModelLimits(context.Background(), ModelRoute{
			ProviderID: providerID, ProviderType: "custom", BaseURL: server.URL, Model: "same-model",
		}, 0, logger)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("provider-specific probe calls = %d, want 2", got)
	}
}

func TestModelLimitInvalidationRejectsStaleInflightResult(t *testing.T) {
	InvalidateModelLimitCache()
	defer InvalidateModelLimitCache()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var calls atomic.Int32
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		call := calls.Add(1)
		contextWindow := 22222
		if call == 1 {
			close(firstStarted)
			<-releaseFirst
			contextWindow = 11111
		}
		fmt.Fprintf(w, `{"data":[{"id":"changing-model","context_length":%d,"max_output_tokens":2048}]}`, contextWindow)
	}))
	defer server.Close()

	route := ModelRoute{ProviderID: "changing", ProviderType: "custom", BaseURL: server.URL, Model: "changing-model"}
	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		_ = ResolveModelLimits(context.Background(), route, 0, logger)
	}()
	<-firstStarted
	InvalidateModelLimitCache()
	second := ResolveModelLimits(context.Background(), route, 0, logger)
	if second.ContextWindow != 22222 {
		t.Fatalf("post-invalidation context = %d, want 22222", second.ContextWindow)
	}
	close(releaseFirst)
	<-firstDone
	cached := ResolveModelLimitsCached(route, 0)
	if cached.ContextWindow != 22222 || cached.ProbeStatus != "cached" {
		t.Fatalf("stale in-flight result replaced refreshed cache: %+v", cached)
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
	route := ModelRoute{ProviderID: "cancelled", ProviderType: "custom", BaseURL: server.URL, Model: "cancelled"}
	got := ResolveModelLimits(ctx, route, 0, logger)
	if time.Since(started) > time.Second {
		t.Fatal("cancelled probe did not return promptly")
	}
	if got.ContextWindow != ConservativeContextWindow {
		t.Fatalf("context = %d, want conservative fallback", got.ContextWindow)
	}
	if cached := ResolveModelLimitsCached(route, 0); cached.ProbeStatus != "pending" {
		t.Fatalf("cancelled probe was cached: %+v", cached)
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
