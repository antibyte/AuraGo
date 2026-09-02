package localllm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestLingSelectionProfilesAndCacheIsolation(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.LocalLLM = config.LocalLLMConfig{ModelVariant: "q4_k_m", MTP: "off", ContextSize: 16384}
	m := NewManager(cfg, nil, nil)
	defer m.Close()
	qwen, _, err := m.selectedArtifactsFor(cfg.LocalLLM)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(m.modelDir, 0700); err != nil {
		t.Fatal(err)
	}
	oldFile := filepath.Join(m.modelDir, qwen.Name)
	if err := os.WriteFile(oldFile, []byte("existing Qwen download"), 0600); err != nil {
		t.Fatal(err)
	}
	oldFingerprint := m.Status().DesiredFingerprint
	m.mu.Lock()
	m.status.ToolCallVerified = true
	m.promptSeed = &promptCacheSeed{Fingerprint: "old-seed"}
	m.promptCacheQualified = true
	m.mu.Unlock()
	ling := cfg.LocalLLM
	ling.ModelFamily, ling.ModelVariant = "ling", "q4_k_l"
	m.Configure(ling)
	status := m.Status()
	if status.ModelAlias != "aurago-ling" || status.ModelFamily != "ling" || status.EngineCommit != LingEngineCommit || status.DesiredFingerprint == oldFingerprint || status.ToolCallVerified {
		t.Fatalf("model change did not invalidate routing verification: %+v", status)
	}
	if m.promptSeed != nil || m.promptCacheQualified {
		t.Fatal("Qwen prompt cache survived model switch")
	}
	if _, err := os.Stat(oldFile); err != nil {
		t.Fatal("model switch removed Qwen download", err)
	}
	model, draft, err := m.selectedArtifactsFor(ling)
	if err != nil || draft != nil || model.Size != 5096544352 || model.SHA256 != "4c25f349d6ea6872907c6fbd827d4b90abfd420320394a8cf420ce9b60abee68" {
		t.Fatalf("wrong Ling artifact: %+v %v", model, err)
	}
	ling.MTP = "auto"
	if _, _, err := m.selectedArtifactsFor(ling); err == nil {
		t.Fatal("Ling accepted a draft mode")
	}
	ling.MTP = "off"
	for _, backend := range []string{"cuda", "sycl", "vulkan"} {
		profile := HardwareProfile{SelectedBackend: backend, SelectedDevice: "gpu0", Devices: []GPUDevice{{ID: "gpu0", ComputeCapability: "7.5"}}}
		perf := performanceProfileFor(profile, ling)
		params := strings.Join(resolvedParametersForPlan(ling, false, profile), " ")
		if !strings.Contains(params, "--alias=aurago-ling") || !strings.Contains(params, "LLAMA_KVFLASH=0") || !strings.Contains(params, "--spec-type=none") {
			t.Fatal(params)
		}
		if backend == "cuda" {
			if perf.Name != "ling-cuda-sm75-full-context-v1" || perf.CacheTypeK != "q8_0" || !strings.Contains(params, "LLAMA_CMOE_PREFILL_BATCH=2048") {
				t.Fatalf("wrong CUDA profile: %+v", perf)
			}
			profile.Devices[0].ComputeCapability = "8.9"
			if strings.Contains(performanceProfileFor(profile, ling).Name, "sm75") {
				t.Fatal("SM75 tuning leaked to another architecture")
			}
		} else if perf.CacheTypeK != "f16" || strings.Contains(params, "LLAMA_CMOE_") {
			t.Fatalf("CUDA profile leaked to %s", backend)
		}
	}
	profile := HardwareProfile{SelectedBackend: "vulkan", SelectedDevice: "gpu0", Devices: []GPUDevice{{ID: "gpu0", Vendor: "intel", Device: "0xe20b"}}}
	for _, backend := range []string{"vulkan", "sycl", "cuda"} {
		profile.SelectedBackend = backend
		params := strings.Join(performanceParameters(ling, profile), " ")
		if strings.Contains(params, "GGML_VK_DISABLE_F16=1") != (backend == "vulkan") {
			t.Fatalf("B580 workaround leaked across backends: %s", params)
		}
	}
	profile.SelectedBackend = "vulkan"
	profile.Devices[0].Device = "0x56a0"
	if strings.Contains(strings.Join(performanceParameters(ling, profile), " "), "GGML_VK_DISABLE_F16") {
		t.Fatal("B580 workaround leaked to another Intel device")
	}
}

func TestLingStartupAttestationRejectsEngineBatchAndKVFlashDrift(t *testing.T) {
	for _, backend := range []string{"cuda", "sycl", "vulkan"} {
		t.Run(backend, func(t *testing.T) {
			plan := runtimePlan{
				Config:  config.LocalLLMConfig{ModelFamily: "ling", Backend: backend, ModelVariant: "q4_k_l", MTP: "off", ContextSize: 16384},
				Profile: HardwareProfile{SelectedBackend: backend, SelectedDevice: "gpu0", Devices: []GPUDevice{{ID: "gpu0", DockerID: "0"}}},
				Model:   LingManifest().Artifacts["normal_q4_k_l"],
				Image:   Image{Reference: "image@sha256:" + strings.Repeat("c", 64)},
			}
			plan.ResolvedParameters = resolvedParametersForPlan(plan.Config, false, plan.Profile)
			valid := startupManifest{
				GPUOffload: true, KVOffload: true, MemoryProfileVerified: true,
				ImageDigest: "sha256:" + strings.Repeat("c", 64), TargetSHA256: plan.Model.SHA256,
				PhysicalDevice: "gpu0", ActualDevice: resolvedRuntimeDevice(plan.Profile),
				ResolvedParameters: plan.ResolvedParameters, LlamaCPPCommit: LingEngineCommit,
				PerformanceProfile: "ling-full-context-v1", BatchSize: 512, UBatchSize: 512,
				CacheTypeK: "f16", CacheTypeV: "f16", FlashAttention: "auto", CacheRAMMiB: 4096,
				ContextCheckpoints: 32, CheckpointMinStep: 2048, CacheIdleSlots: "on", SlotsEndpoint: "off",
			}
			if backend == "cuda" {
				valid.PerformanceProfile = "ling-cuda-full-context-v1"
				valid.BatchSize, valid.UBatchSize = 64, 64
				valid.PrefillBatchSize, valid.PrefillUBatchSize = 2048, 2048
				valid.CacheTypeK, valid.CacheTypeV, valid.FlashAttention = "q8_0", "q8_0", "on"
			}
			if err := validateStartupManifest(plan, valid); err != nil {
				t.Fatal(err)
			}
			for name, change := range map[string]func(*startupManifest){
				"engine":   func(v *startupManifest) { v.LlamaCPPCommit = LlamaCPPCommit },
				"prefill":  func(v *startupManifest) { v.PrefillBatchSize++ },
				"decode":   func(v *startupManifest) { v.UBatchSize++ },
				"eviction": func(v *startupManifest) { v.KVFlashTokens = 8192 },
				"f16":      func(v *startupManifest) { v.VulkanDisableF16 = "1" },
				"context":  func(v *startupManifest) { v.ResolvedParameters = []string{"--ctx-size=8192"} },
				"GPU":      func(v *startupManifest) { v.GPUOffload = false },
			} {
				invalid := valid
				change(&invalid)
				if err := validateStartupManifest(plan, invalid); err == nil {
					t.Fatalf("accepted %s drift", name)
				}
			}
		})
	}
}

func TestNVIDIAComputeCapabilitySelectsLingSM75(t *testing.T) {
	devices := []GPUDevice{{ID: "0000:01:00.0", Vendor: "nvidia"}}
	enrichNVIDIADevices(devices, "00000000:01:00.0, GPU-example, 6144, 580.0, 7.5")
	profile := HardwareProfile{SelectedBackend: "cuda", SelectedDevice: devices[0].ID, Devices: devices}
	if devices[0].ComputeCapability != "7.5" || devices[0].VRAMBytes != 6144<<20 || lingPerformanceProfile(profile).Name != "ling-cuda-sm75-full-context-v1" {
		t.Fatalf("SM75 hardware detection failed: %+v", devices[0])
	}
}
