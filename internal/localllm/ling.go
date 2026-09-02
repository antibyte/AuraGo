package localllm

import (
	"strings"

	"aurago/internal/config"
)

const LingEngineCommit = "f37a34cd4e502284ca297e141a6c4013bd151b18"

func engineCommit(cfg config.LocalLLMConfig) string {
	if cfg.Family() == "ling" {
		return LingEngineCommit
	}
	return LlamaCPPCommit
}

func (m *Manager) manifestFor(cfg config.LocalLLMConfig) Manifest {
	if cfg.Family() == "ling" {
		return LingManifest()
	}
	return m.manifest
}

// LingManifest stays closed until the public model revision and all image digests
// have been verified. An unpublished artifact must never use a mutable URL.
func LingManifest() Manifest {
	return Manifest{
		Version: ReleaseManifestVersion, LlamaCPPCommit: LingEngineCommit,
		Artifacts: map[string]Artifact{
			"normal_q4_k_l": {
				Name: "AuraGo-Ling-3.0-tiny-Q4_K_L.gguf", Path: "AuraGo-Ling-3.0-tiny-Q4_K_L.gguf",
				Repository: "antibyte/AuraGo-Ling", Revision: "9ba7fb67a5cf43d9599e84b08cd17c1f9c537a35", Size: 5096544352,
				SHA256: "4c25f349d6ea6872907c6fbd827d4b90abfd420320394a8cf420ce9b60abee68",
			},
		},
	}
}

func lingPerformanceProfile(profile HardwareProfile) runtimePerformanceProfile {
	result := runtimePerformanceProfile{
		Name: "ling-full-context-v1", CacheRAMMiB: 4096,
		BatchSize: 512, UBatchSize: 512, FlashAttention: "auto",
		CacheTypeK: "f16", CacheTypeV: "f16",
		ContextCheckpoints: 32, CheckpointMinStep: 2048, CacheIdleSlots: true,
	}
	if profile.SelectedBackend == "cpu" {
		result.CacheRAMMiB = 1024
	}
	if gpu, err := profile.selectedGPU(); err == nil && profile.SelectedBackend == "vulkan" &&
		strings.EqualFold(gpu.Vendor, "intel") && strings.EqualFold(strings.TrimSpace(gpu.Device), "0xe20b") {
		result.Name = "ling-vulkan-b580-full-context-v1"
	}
	if profile.SelectedBackend == "cuda" {
		result.Name = "ling-cuda-full-context-v1"
		result.BatchSize, result.UBatchSize = 64, 64
		result.FlashAttention = "on"
		result.CacheTypeK, result.CacheTypeV = "q8_0", "q8_0"
		if gpu, err := profile.selectedGPU(); err == nil && gpu.ComputeCapability == "7.5" {
			result.Name = "ling-cuda-sm75-full-context-v1"
		}
	}
	return result
}
