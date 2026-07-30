package localllm

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	prepared, seed, stream, err := preparePromptCacheRequest(request)
	if err != nil {
		t.Fatal(err)
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
	if !strings.Contains(seedText, promptCacheProbeText) {
		t.Fatalf("seed is missing the fixed template probe: %s", seedText)
	}
	if strings.Index(seedText, "a_tool") > strings.Index(seedText, "z_tool") {
		t.Fatalf("tool schemas are not canonicalized: %s", seedText)
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
	manager := &Manager{status: Status{PromptCache: PromptCacheStatus{State: "warm"}}}
	manager.observePromptCacheResponse([]byte(`{"timings":{"cache_n":800,"prompt_n":200}}`), false, 25*time.Millisecond)
	status := manager.Status().PromptCache
	if !status.LastHit || status.Hits != 1 || status.Requests != 1 ||
		status.CachedTokens != 800 || status.ProcessedTokens != 200 || status.HitRate != 0.8 {
		t.Fatalf("prompt cache status = %#v", status)
	}
}

func TestPromptCacheResponseMeasurementsUseOfficialTokenFields(t *testing.T) {
	manager := &Manager{status: Status{PromptCache: PromptCacheStatus{State: "warm"}}}
	manager.observePromptCacheResponse(
		[]byte(`{"tokens_cached":900,"tokens_evaluated":100}`),
		false,
		12*time.Millisecond,
	)
	status := manager.Status().PromptCache
	if !status.LastHit || status.CachedTokens != 900 || status.ProcessedTokens != 100 ||
		status.HitRate != 0.9 {
		t.Fatalf("prompt cache status = %#v", status)
	}
}

func TestTruncateRenderedPromptAtStaticPrefixDropsSyntheticTurn(t *testing.T) {
	const prefix = "STATIC PREFIX"
	rendered := "<tools>safe</tools>\n<system>" + prefix +
		"</system>\n<user>" + promptCacheProbeText + "</user>"
	truncated, err := truncateRenderedPromptAtStaticPrefix(rendered, prefix)
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

func TestPromptCacheWarmupPersistsOnlyValueFreeDecision(t *testing.T) {
	const secretSuffix = "must-never-be-persisted"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/apply-template":
			payload, _ := io.ReadAll(request.Body)
			if !strings.Contains(string(payload), "STATIC PREFIX") {
				t.Errorf("apply-template body = %s", payload)
			}
			_, _ = io.WriteString(
				writer,
				`{"prompt":"rendered STATIC PREFIX `+secretSuffix+` synthetic turn"}`,
			)
		case "/completion":
			_, _ = io.WriteString(writer, `{"timings":{"cache_n":0,"prompt_n":1200}}`)
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
	}
	manager.promptSeed = &promptCacheSeed{
		Generation: 1, Fingerprint: "seed-fingerprint", ToolsetFingerprint: "tools",
		SystemPrefix:      "STATIC PREFIX " + secretSuffix,
		ApplyTemplateBody: []byte(`{"messages":[{"role":"system","content":"STATIC PREFIX ` + secretSuffix + `"}],"tools":[]}`),
	}
	plan := runtimePlan{
		Generation: 1,
		Config:     config.LocalLLMConfig{ListenPort: port},
		Profile:    HardwareProfile{SelectedBackend: "cpu"},
	}
	manager.ensurePromptCacheWarm(context.Background(), plan, "runtime-key")
	status := manager.Status().PromptCache
	if status.State != "warm" || status.SeedTokens != 1200 {
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
