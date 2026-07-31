package localllm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestPreparePromptCacheRequestBuildsValueFreeStableSeed(t *testing.T) {
	body := `{
		"model":"aurago-qwen",
		"stream":true,
		"messages":[
			{"role":"system","content":"STATIC PREFIX\n# TURN CONTEXT\nsecret-current-turn"},
			{"role":"user","content":"private user text"}
		],
		"tools":[
			{"type":"function","function":{"name":"z_tool","parameters":{"type":"object"}}},
			{"type":"function","function":{"name":"a_tool","parameters":{"type":"object"}}}
		]
	}`
	request, err := http.NewRequest(http.MethodPost, "http://localhost/v1/chat/completions", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	prepared, seed, stream, seedError, err := preparePromptCacheRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	if seedError != "" {
		t.Fatalf("seed error = %q", seedError)
	}
	if seed == nil || !stream || seed.ToolCount != 2 {
		t.Fatalf("seed=%#v stream=%v", seed, stream)
	}
	seedText := string(seed.ApplyTemplateBody)
	for _, forbidden := range []string{"secret-current-turn", "private user text", "TURN CONTEXT"} {
		if strings.Contains(seedText, forbidden) {
			t.Fatalf("seed contains dynamic value %q: %s", forbidden, seedText)
		}
	}
	var seedRequest struct {
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	if json.Unmarshal(seed.ApplyTemplateBody, &seedRequest) != nil ||
		len(seedRequest.Messages) != 2 || seedRequest.Messages[1].Content != promptCacheProbeText {
		t.Fatalf("seed is missing the fixed template probe: %s", seedText)
	}
	if strings.Index(seedText, "a_tool") > strings.Index(seedText, "z_tool") {
		t.Fatalf("tool schemas are not canonicalized: %s", seedText)
	}
	if strings.Contains(seedText, promptCacheRenderBoundary) {
		t.Fatalf("qualification probe contains the internal render boundary: %s", seedText)
	}
	raw, _ := io.ReadAll(prepared.Body)
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || rawJSONBool(object["cache_prompt"]) {
		t.Fatalf("prepared request did not explicitly disable cache: %s", raw)
	}
	var streamOptions map[string]json.RawMessage
	_ = json.Unmarshal(object["stream_options"], &streamOptions)
	if !rawJSONBool(streamOptions["include_usage"]) {
		t.Fatalf("stream usage was not enabled: %s", raw)
	}
}

func TestPromptCacheResponseMeasurementsUseCacheN(t *testing.T) {
	manager := promptCacheObservationTestManager(t)
	plan := promptCacheObservationPlan{Generation: 1, SeedFingerprint: "seed", CacheEnabled: true}
	manager.observePromptCacheResponse(plan, []byte(`{"timings":{"cache_n":800,"prompt_n":200}}`), false, 25*time.Millisecond, true)
	status := manager.Status().PromptCache
	if !status.LastHit || status.Hits != 1 || status.Requests != 1 ||
		status.CachedTokens != 800 || status.ProcessedTokens != 200 || status.HitRate != 0.8 {
		t.Fatalf("prompt cache status = %#v", status)
	}
}

func TestPromptCacheResponseMeasurementsUseOfficialTokenFields(t *testing.T) {
	manager := promptCacheObservationTestManager(t)
	plan := promptCacheObservationPlan{Generation: 1, SeedFingerprint: "seed", CacheEnabled: true}
	manager.observePromptCacheResponse(
		plan,
		[]byte(`{"tokens_cached":900,"tokens_evaluated":100}`),
		false,
		12*time.Millisecond,
		true,
	)
	status := manager.Status().PromptCache
	if !status.LastHit || status.CachedTokens != 900 || status.ProcessedTokens != 100 ||
		status.HitRate != 0.9 {
		t.Fatalf("prompt cache status = %#v", status)
	}
}

func TestPromptCacheResponseIgnoresIncompleteAndStaleObservations(t *testing.T) {
	manager := promptCacheObservationTestManager(t)
	current := promptCacheObservationPlan{Generation: 1, SeedFingerprint: "seed", CacheEnabled: true}
	manager.observePromptCacheResponse(current, []byte(`{"timings":{"cache_n":900,"prompt_n":100}}`), false, time.Millisecond, false)
	manager.observePromptCacheResponse(
		promptCacheObservationPlan{Generation: 0, SeedFingerprint: "old", CacheEnabled: true},
		[]byte(`{"timings":{"cache_n":900,"prompt_n":100}}`), false, time.Millisecond, true,
	)
	if status := manager.Status().PromptCache; status.Requests != 0 || status.CachedTokens != 0 {
		t.Fatalf("incomplete or stale response changed cache status: %#v", status)
	}
}

func TestCacheObservingBodyCloseWaitsForReadAndPublishesOnce(t *testing.T) {
	upstream := &blockingPromptCacheBody{started: make(chan struct{}), closed: make(chan struct{})}
	var calls int
	var complete bool
	body := newCacheObservingBody(upstream, false, func(_ []byte, _ time.Duration, finished bool) {
		calls++
		complete = finished
	}, time.Now())
	readDone := make(chan error, 1)
	go func() {
		buffer := make([]byte, 8)
		_, err := body.Read(buffer)
		readDone <- err
	}()
	<-upstream.started
	if err := body.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-readDone; !errors.Is(err, io.EOF) {
		t.Fatalf("read error = %v", err)
	}
	if calls != 1 || !complete {
		t.Fatalf("observer calls=%d complete=%v", calls, complete)
	}
}

func TestCacheObservingBodyRejectsIncompleteSSE(t *testing.T) {
	var calls int
	var complete bool
	body := newCacheObservingBody(
		io.NopCloser(strings.NewReader(`data: {"choices":[{"delta":{"content":"partial"}}]}`)),
		true,
		func(_ []byte, _ time.Duration, finished bool) {
			calls++
			complete = finished
		},
		time.Now(),
	)
	_, _ = io.ReadAll(body)
	_ = body.Close()
	if calls != 1 || complete {
		t.Fatalf("observer calls=%d complete=%v", calls, complete)
	}
}

func TestTruncateRenderedPromptAtBoundaryDropsSyntheticTurn(t *testing.T) {
	const prefix = "STATIC PREFIX"
	rendered := "<tools>safe</tools>\n<system>" + prefix +
		promptCacheRenderBoundary + "</system>\n<user>" + promptCacheProbeText + "</user>"
	truncated, err := truncateRenderedPromptAtBoundary(rendered)
	if err != nil {
		t.Fatal(err)
	}
	if truncated != "<tools>safe</tools>\n<system>"+prefix {
		t.Fatalf("truncated prompt = %q", truncated)
	}
	if strings.Contains(truncated, promptCacheProbeText) {
		t.Fatalf("synthetic turn remained in warm-up prompt: %q", truncated)
	}
}

func TestPromptCacheDecisionV1IsIgnored(t *testing.T) {
	manager := &Manager{
		generation: 1, desiredFingerprint: "runtime", modelDir: t.TempDir(),
		promptSeed: &promptCacheSeed{Generation: 1, Fingerprint: "seed"},
		status:     Status{PerformanceProfile: performanceProfileGeneric},
	}
	manager.mu.Lock()
	fingerprint := manager.promptCacheFingerprintLocked()
	manager.mu.Unlock()
	payload, _ := json.Marshal(map[string]any{
		"schema_version": 1,
		"fingerprint":    fingerprint,
		"accepted":       true,
		"state":          "warm",
	})
	if err := os.WriteFile(filepath.Join(manager.modelDir, promptCacheFileName), payload, 0o600); err != nil {
		t.Fatal(err)
	}
	manager.mu.Lock()
	_, ok := manager.loadPromptCacheDecisionLocked()
	manager.mu.Unlock()
	if ok {
		t.Fatal("schema-v1 cache decision was trusted")
	}
}

func TestPromptCacheDecisionWriterDoesNotHoldPromptSlotAndCoalesces(t *testing.T) {
	blocked := make(chan struct{})
	started := make(chan struct{})
	var writes int
	manager := promptCacheObservationTestManager(t)
	manager.promptDecisionWrite = func(_ string, _ []byte, _ os.FileMode) error {
		writes++
		if writes == 1 {
			close(started)
			<-blocked
		}
		return nil
	}
	manager.mu.Lock()
	entry, ok := manager.promptCacheDecisionEntryLocked(true, true)
	manager.mu.Unlock()
	if !ok {
		t.Fatal("expected terminal cache decision")
	}
	manager.queuePromptCacheDecision(entry, true)
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	release, err := manager.acquirePromptSlot(ctx)
	cancel()
	if err != nil {
		t.Fatalf("decision writer blocked prompt slot: %v", err)
	}
	release()
	close(blocked)
	if err := manager.waitPromptCachePersistence(context.Background()); err != nil {
		t.Fatal(err)
	}
	manager.mu.Lock()
	entry, ok = manager.promptCacheDecisionEntryLocked(true, false)
	manager.mu.Unlock()
	if !ok {
		t.Fatal("expected metrics snapshot")
	}
	manager.queuePromptCacheDecision(entry, false)
	manager.queuePromptCacheDecision(entry, false)
	time.Sleep(20 * time.Millisecond)
	if writes != 1 {
		t.Fatalf("non-terminal snapshots were not coalesced: writes=%d", writes)
	}
	manager.Close()
}

func TestPromptCacheDecisionWriteFailureKeepsQualifiedRAMCache(t *testing.T) {
	manager := promptCacheObservationTestManager(t)
	manager.promptDecisionWrite = func(string, []byte, os.FileMode) error {
		return errors.New("disk unavailable")
	}
	manager.mu.Lock()
	entry, ok := manager.promptCacheDecisionEntryLocked(true, true)
	manager.mu.Unlock()
	if !ok {
		t.Fatal("expected terminal cache decision")
	}
	manager.queuePromptCacheDecision(entry, true)
	if err := manager.waitPromptCachePersistence(context.Background()); err != nil {
		t.Fatal(err)
	}
	status := manager.Status().PromptCache
	if !status.Qualified || status.DecisionPersisted ||
		status.ErrorCode != "prompt_cache_decision_write_failed" {
		t.Fatalf("write failure disabled RAM cache or was hidden: %#v", status)
	}
}

func TestPromptCacheQualificationRejectsMissingDiscoverTool(t *testing.T) {
	manager := &Manager{}
	_, err := manager.qualifyPromptCache(
		context.Background(),
		runtimePlan{},
		"",
		promptCacheSeed{ApplyTemplateBody: []byte(`{"tools":[]}`)},
	)
	if errorCode(err) != "prompt_cache_probe_tool_unavailable" {
		t.Fatalf("error = %v", err)
	}
}

func TestPromptCacheQualificationReservesTimeForRealTurn(t *testing.T) {
	parent, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, qualificationCancel, err := promptCacheQualificationContext(parent); err == nil {
		qualificationCancel()
		t.Fatal("qualification started without reserving 30 seconds for the real turn")
	}
}

func TestPromptCacheQualificationThresholdsAndSemantics(t *testing.T) {
	validCall := `{"name":"discover_tools","arguments":"{\"operation\":\"search\",\"query\":\"aurago_prompt_cache_probe\"}"}`
	baseCold := promptCacheProbeResult{
		ToolCall: validCall, TTFT: 100 * time.Millisecond, Complete: true,
	}
	baseWarm := promptCacheProbeResult{
		ToolCall: validCall, TTFT: 30 * time.Millisecond, CachedTokens: 800,
		PromptTokens: 1000, Complete: true,
	}
	if result, err := validatePromptCacheQualification(baseCold, baseWarm, 1000); err != nil ||
		result.CachedTokens != 800 || result.ProcessedTokens != 200 {
		t.Fatalf("boundary qualification result=%#v error=%v", result, err)
	}
	tests := []struct {
		name string
		edit func(*promptCacheProbeResult)
		code string
	}{
		{
			name: "semantic mismatch",
			edit: func(value *promptCacheProbeResult) {
				value.ToolCall = `{"name":"invoke_tool","arguments":"{}"}`
			},
			code: "prompt_cache_semantic_mismatch",
		},
		{
			name: "reuse below threshold",
			edit: func(value *promptCacheProbeResult) {
				value.CachedTokens = 799
			},
			code: "prompt_cache_reuse_below_80_percent",
		},
		{
			name: "ttft gain below threshold",
			edit: func(value *promptCacheProbeResult) {
				value.TTFT = 31 * time.Millisecond
			},
			code: "prompt_cache_ttft_gain_below_70_percent",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			warm := baseWarm
			test.edit(&warm)
			_, err := validatePromptCacheQualification(baseCold, warm, 1000)
			if errorCode(err) != test.code {
				t.Fatalf("error=%v want=%s", err, test.code)
			}
		})
	}
}

func TestPromptCacheTemplateBoundarySurvivesTemplateTrimming(t *testing.T) {
	seed := promptCacheSeed{
		SystemPrefix: "STATIC PREFIX\n\n",
		ApplyTemplateBody: []byte(`{
			"messages":[
				{"role":"system","content":"STATIC PREFIX\n\n"},
				{"role":"user","content":"probe"}
			],
			"tools":[]
		}`),
	}
	body, err := promptCacheTemplateBody(seed)
	if err != nil {
		t.Fatal(err)
	}
	var request struct {
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	if json.Unmarshal(body, &request) != nil || len(request.Messages) != 2 {
		t.Fatalf("template request invalid: %s", body)
	}
	system := strings.TrimSpace(request.Messages[0].Content)
	rendered := "<|im_start|>system\n" + system + "<|im_end|>"
	prefix, err := truncateRenderedPromptAtBoundary(rendered)
	if err != nil {
		t.Fatal(err)
	}
	if prefix != "<|im_start|>system\nSTATIC PREFIX\n\n" {
		t.Fatalf("rendered prefix = %q", prefix)
	}
	if strings.Contains(prefix, promptCacheRenderBoundary) {
		t.Fatalf("render boundary leaked into cached prefix: %q", prefix)
	}
}

func TestPerformanceProfileAMDVulkanIsIsolated(t *testing.T) {
	amd := HardwareProfile{
		SelectedBackend: "vulkan", SelectedDevice: "0000:05:00.0",
		Devices: []GPUDevice{{
			ID: "0000:05:00.0", Vendor: "amd", Discrete: false,
		}},
	}
	profile := performanceProfileFor(amd)
	if profile.Name != performanceProfileAMDVulkanFast || profile.CacheRAMMiB != 2048 ||
		profile.RADVPerfTest != "nogttspill" || profile.Poll == nil || *profile.Poll != 0 {
		t.Fatalf("AMD Vulkan profile = %#v", profile)
	}
	nvidia := amd
	nvidia.Devices[0].Vendor = "nvidia"
	generic := performanceProfileFor(nvidia)
	if generic.Name == performanceProfileAMDVulkanFast || generic.RADVPerfTest != "" || generic.Poll != nil {
		t.Fatalf("AMD-only tuning leaked to generic Vulkan: %#v", generic)
	}
}

func TestPerformanceProfileSYCLArcB580IsExactAndDeviceBound(t *testing.T) {
	b580 := HardwareProfile{
		SelectedBackend: "sycl", SelectedDevice: "0000:03:00.0",
		Devices: []GPUDevice{{
			ID: "0000:03:00.0", Vendor: "intel", Device: "0xe20b", Discrete: true,
		}},
	}
	profile := performanceProfileFor(b580)
	if profile.Name != performanceProfileSYCLArc || profile.BatchSize != 2048 ||
		profile.UBatchSize != 2048 || profile.Threads != 8 || profile.ThreadsBatch != 8 ||
		profile.FlashAttention != "off" || profile.CacheRAMMiB != 8192 {
		t.Fatalf("B580 SYCL profile = %#v", profile)
	}
	other := b580
	other.Devices = append([]GPUDevice(nil), b580.Devices...)
	other.Devices[0].Device = "0x56a0"
	generic := performanceProfileFor(other)
	if generic.Name != performanceProfileGeneric || generic.UBatchSize != 512 ||
		generic.Threads != 0 || generic.ThreadsBatch != 0 || generic.FlashAttention != "auto" {
		t.Fatalf("generic SYCL device inherited B580 tuning: %#v", generic)
	}
	manager := &Manager{}
	b580.Fingerprint = "same-hardware-fingerprint"
	other.Fingerprint = b580.Fingerprint
	cfg := config.LocalLLMConfig{Backend: "sycl", ContextSize: 16384}
	if manager.computeDesiredFingerprint(cfg, b580) == manager.computeDesiredFingerprint(cfg, other) {
		t.Fatal("desired-state fingerprint does not attest the resolved SYCL performance profile")
	}
}

func TestPromptCacheWarmupPersistsOnlyValueFreeDecision(t *testing.T) {
	const secretSuffix = "must-never-be-persisted"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/apply-template":
			payload, _ := io.ReadAll(request.Body)
			if !strings.Contains(string(payload), "STATIC PREFIX") ||
				!strings.Contains(string(payload), promptCacheRenderBoundary) {
				t.Errorf("apply-template body = %s", payload)
			}
			_, _ = io.WriteString(
				writer,
				`{"prompt":"rendered STATIC PREFIX `+secretSuffix+promptCacheRenderBoundary+` synthetic turn"}`,
			)
		case "/completion":
			_, _ = io.WriteString(writer, `{"timings":{"cache_n":0,"prompt_n":1000}}`)
		case "/v1/chat/completions":
			var payload map[string]json.RawMessage
			_ = json.NewDecoder(request.Body).Decode(&payload)
			cache := rawJSONBool(payload["cache_prompt"])
			if !cache {
				time.Sleep(40 * time.Millisecond)
			}
			writer.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(writer, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"name":"discover_tools","arguments":"{\"operation\":\"search\",\"query\":\"aurago_prompt_cache_probe\"}"}}]}}],"timings":{"cache_n":`)
			if cache {
				_, _ = io.WriteString(writer, "900")
			} else {
				_, _ = io.WriteString(writer, "0")
			}
			_, _ = io.WriteString(writer, `,"prompt_n":100}}`+"\n\ndata: [DONE]\n\n")
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, portText, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(portText)
	modelDir := t.TempDir()
	manager := &Manager{
		generation: 1, desiredFingerprint: "runtime-fingerprint", modelDir: modelDir,
		cfg:     config.LocalLLMConfig{ListenPort: port},
		profile: HardwareProfile{SelectedBackend: "cpu"},
		status: Status{
			PerformanceProfile: performanceProfileGeneric,
			PromptCache:        PromptCacheStatus{State: "cold"},
		},
		promptDecisionWrite: config.WriteFileAtomic,
	}
	manager.promptSeed = &promptCacheSeed{
		Generation: 1, Fingerprint: "seed-fingerprint", ToolsetFingerprint: "tools",
		SystemPrefix:      "STATIC PREFIX " + secretSuffix,
		ApplyTemplateBody: []byte(`{"messages":[{"role":"system","content":"STATIC PREFIX ` + secretSuffix + `"},{"role":"user","content":"probe"}],"tools":[{"type":"function","function":{"name":"discover_tools","parameters":{"type":"object"}}}]}`),
	}
	plan := runtimePlan{
		Generation: 1,
		Config:     config.LocalLLMConfig{ListenPort: port},
		Profile:    HardwareProfile{SelectedBackend: "cpu"},
	}
	manager.ensurePromptCacheWarm(context.Background(), plan, "runtime-key")
	if err := manager.waitPromptCachePersistence(context.Background()); err != nil {
		t.Fatal(err)
	}
	status := manager.Status().PromptCache
	if status.State != "warm" || !status.Qualified || !status.DecisionPersisted || status.SeedTokens != 1000 {
		t.Fatalf("warmup status = %#v", status)
	}
	payload, err := os.ReadFile(filepath.Join(modelDir, promptCacheFileName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), secretSuffix) || strings.Contains(string(payload), "STATIC PREFIX") {
		t.Fatalf("decision file contains prompt data: %s", payload)
	}
}

func promptCacheObservationTestManager(t *testing.T) *Manager {
	t.Helper()
	lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())
	manager := &Manager{
		generation: 1, desiredFingerprint: "runtime", modelDir: t.TempDir(),
		promptSeed:           &promptCacheSeed{Generation: 1, Fingerprint: "seed"},
		promptCacheQualified: true,
		promptSlot:           make(chan struct{}, 1),
		idleStop:             make(chan struct{}),
		lifecycleCtx:         lifecycleCtx,
		lifecycleCancel:      lifecycleCancel,
		status: Status{
			PromptCache: PromptCacheStatus{State: "warm", Qualified: true},
		},
		promptDecisionWrite: config.WriteFileAtomic,
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = manager.waitPromptCachePersistence(ctx)
	})
	return manager
}

type blockingPromptCacheBody struct {
	started chan struct{}
	closed  chan struct{}
	once    sync.Once
}

func (body *blockingPromptCacheBody) Read(_ []byte) (int, error) {
	body.once.Do(func() { close(body.started) })
	<-body.closed
	return 0, io.EOF
}

func (body *blockingPromptCacheBody) Close() error {
	select {
	case <-body.closed:
	default:
		close(body.closed)
	}
	return nil
}
