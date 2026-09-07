package localllm

// SparkEngineCommit is the latest reviewed Spark-capable hybrid revision.
const SparkEngineCommit = "a67aee47326f0036311876e102142f52f2b914b7"

// SparkManifest pins publicly verified artifacts; GPU backends remain experimental.
func SparkManifest() Manifest {
	return Manifest{
		Version: ReleaseManifestVersion, ReleaseReady: true, LlamaCPPCommit: SparkEngineCommit,
		Artifacts: map[string]Artifact{
			"normal_q4_k_m": {
				Name: "AuraGo-Spark-X2.5-4B-Q4_K_M.gguf", Path: "AuraGo-Spark-X2.5-4B-Q4_K_M.gguf",
				Repository: "antibyte/AuraGo-Spark", Revision: "407abe58aa453e7a3ec4069232ad766f0c5980e2",
				Size: 2600224352, SHA256: "3683da863f81a0f2f6c752fe47447845bf83f09fa7548d6f62407846f163039b",
			},
		},
		Images: map[string]Image{
			"cuda": {
				Backend: "cuda",
				Reference: "ghcr.io/antibyte/aurago-llm-cuda@" +
					"sha256:f82104d72aa068c770c40474437207f43cdb46d2ad67a6341db400027351b649",
			},
			"sycl": {
				Backend: "sycl",
				Reference: "ghcr.io/antibyte/aurago-llm-sycl@" +
					"sha256:1c60773df7610004be72df034f731393d4e4f95032deef8119c1170132cbad00",
			},
			"vulkan": {
				Backend: "vulkan",
				Reference: "ghcr.io/antibyte/aurago-llm-vulkan@" +
					"sha256:eb8ba0222ddecfe06bb31a24fc6aa6e8475799c828d078c19f2357b9706a27d0",
			},
		},
	}
}

func sparkPerformanceProfile(profile HardwareProfile) runtimePerformanceProfile {
	result := runtimePerformanceProfile{
		Name: "spark-experimental-v1", CacheRAMMiB: 1024,
		BatchSize: 512, UBatchSize: 512, FlashAttention: "auto",
		CacheTypeK: "f16", CacheTypeV: "f16",
		ContextCheckpoints: 32, CheckpointMinStep: 2048, CacheIdleSlots: true,
	}
	if profile.SelectedBackend == "cuda" {
		result.FlashAttention = "on"
		result.CacheTypeK, result.CacheTypeV = "q8_0", "q8_0"
	}
	return result
}
