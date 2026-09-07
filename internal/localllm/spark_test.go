package localllm

import (
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestSparkSelectionRuntimeAndReleaseGate(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.LocalLLM = config.LocalLLMConfig{ModelFamily: "qwen", ModelVariant: "q4_k_m", MTP: "off", ContextSize: 16384}
	m := NewManager(cfg, nil, nil)
	defer m.Close()
	before := m.Status().DesiredFingerprint
	m.mu.Lock()
	m.status.ToolCallVerified = true
	m.promptSeed = &promptCacheSeed{Fingerprint: "qwen-seed"}
	m.mu.Unlock()
	spark := config.LocalLLMConfig{ModelFamily: "spark", ModelVariant: "q4_k_m", MTP: "off", ContextSize: 65536, Backend: "cpu"}
	m.Configure(spark)
	status := m.Status()
	if status.ModelAlias != "aurago-spark" || status.ModelName != "AuraGo-Spark" || status.EngineCommit != SparkEngineCommit ||
		status.DesiredFingerprint == before || status.ToolCallVerified || m.promptSeed != nil {
		t.Fatalf("Spark switch did not isolate runtime: %+v", status)
	}
	manifest := SparkManifest()
	if !status.ReleaseManifestReady || manifest.validate() != nil || len(manifest.Images) != 3 {
		t.Fatal("Spark runtime must have three published, digest-pinned images")
	}
	for _, backend := range []string{"cuda", "sycl", "vulkan"} {
		image := manifest.Images[backend]
		if image.Backend != backend || image.Supported || !isDigestPinned(image.Reference) {
			t.Fatalf("Spark %s must stay pinned and experimental: %+v", backend, image)
		}
	}
	model, draft, err := m.selectedArtifactsFor(spark)
	if err != nil || draft != nil || model.Repository != "antibyte/AuraGo-Spark" || model.Size != 2600224352 ||
		model.SHA256 != "3683da863f81a0f2f6c752fe47447845bf83f09fa7548d6f62407846f163039b" {
		t.Fatalf("wrong Spark artifact: %+v, %v", model, err)
	}
	for _, backend := range []string{"cpu", "cuda", "sycl", "vulkan"} {
		profile := HardwareProfile{SelectedBackend: backend}
		params := strings.Join(resolvedParametersForPlan(spark, false, profile), " ")
		for _, required := range []string{"--alias=aurago-spark", "--ctx-size=65536", "--reasoning=on", "--spec-type=none", `--chat-template-kwargs={"enable_thinking":true}`, "LLAMA_KVFLASH=0"} {
			if !strings.Contains(params, required) {
				t.Fatalf("%s missing %s: %s", backend, required, params)
			}
		}
		if strings.Contains(params, "LLAMA_CMOE_") || strings.Contains(params, "nogttspill") || strings.Contains(params, "--slot-prompt-similarity") {
			t.Fatalf("another model's tuning leaked into Spark: %s", params)
		}
	}
	image := Image{Reference: "example.invalid/runtime@sha256:" + strings.Repeat("a", 64)}
	spec, err := m.containerSpecValues(spark, status.DesiredFingerprint, HardwareProfile{SelectedBackend: "cpu"}, model, nil, image)
	if err != nil || !strings.Contains(strings.Join(spec.Env, "\n"), "AURAGO_REASONING=on\n") {
		t.Fatalf("Spark container lost Thinking: %+v, %v", spec, err)
	}
	for _, size := range []int{4096, 16384, 32768} {
		spark.ContextSize = size
		if _, _, err := m.selectedArtifactsFor(spark); err == nil {
			t.Fatalf("Spark accepted context %d", size)
		}
	}
}
