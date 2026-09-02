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

// LingManifest pins the publicly verified model and runtime images. Hardware
// profiles remain experimental until their native Linux qualification passes.
func LingManifest() Manifest {
	return Manifest{
		Version: ReleaseManifestVersion, ReleaseReady: true, LlamaCPPCommit: LingEngineCommit,
		Artifacts: map[string]Artifact{
			"normal_q4_k_l": {
				Name: "AuraGo-Ling-3.0-tiny-Q4_K_L.gguf", Path: "AuraGo-Ling-3.0-tiny-Q4_K_L.gguf",
				Repository: "antibyte/AuraGo-Ling", Revision: "c9d1e3fc16984c6b5c3d4a7838665e0c591143c3", Size: 5096544352,
				SHA256: "4c25f349d6ea6872907c6fbd827d4b90abfd420320394a8cf420ce9b60abee68",
			},
		},
		Images: map[string]Image{
			"cuda": {
				Backend: "cuda",
				Reference: "ghcr.io/antibyte/aurago-llm-cuda@" +
					"sha256:fa54bad3acc9a9d3dd85c9ce2a7def19d8730c13c99a73aad35142667227c0e4",
			},
			"sycl": {
				Backend: "sycl",
				Reference: "ghcr.io/antibyte/aurago-llm-sycl@" +
					"sha256:94cffa92588b7d087dabffe4aa425ad30475eea43a884031a3789b16e4c6449e",
			},
			"vulkan": {
				Backend: "vulkan",
				Reference: "ghcr.io/antibyte/aurago-llm-vulkan@" +
					"sha256:feae8bfdb9a9c6613dc3a8529acff3618cc0c5866de69a8d172c58098a1f8aea",
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
