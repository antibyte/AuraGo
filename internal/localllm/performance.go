package localllm

import (
	"fmt"
	"strings"

	"aurago/internal/config"
)

const (
	performanceProfileAMDVulkanFast = "vulkan-amd-fast-v1"
	performanceProfileSYCLArc       = "sycl-arc-v1"
	performanceProfileGeneric       = "generic-safe-v1"
)

type runtimePerformanceProfile struct {
	Name               string
	CacheRAMMiB        int
	BatchSize          int
	UBatchSize         int
	Threads            int
	ThreadsBatch       int
	FlashAttention     string
	SplitMode          string
	Poll               *int
	Priority           *int
	RADVPerfTest       string
	CacheTypeK         string
	CacheTypeV         string
	ContextCheckpoints int
	CheckpointMinStep  int
	CacheReuse         int
	CacheIdleSlots     bool
	SlotsEndpoint      bool
}

func performanceProfileFor(profile HardwareProfile, configs ...config.LocalLLMConfig) runtimePerformanceProfile {
	if len(configs) > 0 && configs[0].Family() == "ling" {
		return lingPerformanceProfile(profile)
	}
	result := runtimePerformanceProfile{
		Name: performanceProfileGeneric, CacheRAMMiB: 8192,
		BatchSize: 2048, UBatchSize: 512, FlashAttention: "auto",
		CacheTypeK: "f16", CacheTypeV: "f16",
		ContextCheckpoints: 32, CheckpointMinStep: 2048,
		CacheIdleSlots: true,
	}
	if profile.SelectedBackend == "cpu" {
		result.CacheRAMMiB = 1024
		return result
	}
	gpu, err := profile.selectedGPU()
	if err == nil && !gpu.Discrete {
		result.CacheRAMMiB = 2048
	}
	if profile.SelectedBackend == "sycl" && err == nil &&
		strings.EqualFold(gpu.Vendor, "intel") &&
		strings.EqualFold(strings.TrimSpace(gpu.Device), "0xe20b") {
		result.Name = performanceProfileSYCLArc
		result.BatchSize = 2048
		result.UBatchSize = 2048
		result.Threads = 8
		result.ThreadsBatch = 8
		result.FlashAttention = "off"
		return result
	}
	if profile.SelectedBackend == "vulkan" && err == nil && strings.EqualFold(gpu.Vendor, "amd") {
		poll, priority := 0, 1
		result.Name = performanceProfileAMDVulkanFast
		result.BatchSize = 2048
		result.UBatchSize = 512
		result.Threads = 8
		result.ThreadsBatch = 8
		result.FlashAttention = "on"
		result.SplitMode = "none"
		result.Poll = &poll
		result.Priority = &priority
		result.RADVPerfTest = "nogttspill"
	}
	return result
}

func performanceParameters(cfg config.LocalLLMConfig, profile HardwareProfile) []string {
	perf := performanceProfileFor(profile, cfg)
	values := []string{
		"--cache-type-k=" + perf.CacheTypeK,
		"--cache-type-v=" + perf.CacheTypeV,
		fmt.Sprintf("--cache-ram=%d", perf.CacheRAMMiB),
		fmt.Sprintf("--ctx-checkpoints=%d", perf.ContextCheckpoints),
		fmt.Sprintf("--checkpoint-min-step=%d", perf.CheckpointMinStep),
		fmt.Sprintf("--cache-reuse=%d", perf.CacheReuse),
		"--cache-idle-slots",
		"--no-slots",
	}
	if cfg.Family() == "ling" {
		values = append(values, "--no-cpu-moe", "--chat-template-kwargs={\"enable_thinking\":false}", "LLAMA_KVFLASH=0")
		if perf.Name == "ling-vulkan-b580-full-context-v1" {
			values = append(values, "GGML_VK_DISABLE_F16=1")
		}
		if profile.SelectedBackend == "cuda" {
			values = append(values, "--backend-sampling", "LLAMA_CMOE_PREFILL_BATCH=2048", "LLAMA_CMOE_PREFILL_UBATCH=2048", "LLAMA_CMOE_DECODE_BATCH=64", "LLAMA_CMOE_DECODE_UBATCH=64")
		}
	}
	if perf.BatchSize > 0 {
		values = append(values, fmt.Sprintf("--batch-size=%d", perf.BatchSize))
	}
	if perf.UBatchSize > 0 {
		values = append(values, fmt.Sprintf("--ubatch-size=%d", perf.UBatchSize))
	}
	if perf.Threads > 0 {
		values = append(values, fmt.Sprintf("--threads=%d", perf.Threads))
	}
	if perf.ThreadsBatch > 0 {
		values = append(values, fmt.Sprintf("--threads-batch=%d", perf.ThreadsBatch))
	}
	if perf.FlashAttention != "" {
		values = append(values, "--flash-attn="+perf.FlashAttention)
	}
	if perf.SplitMode != "" {
		values = append(values, "--split-mode="+perf.SplitMode)
	}
	if perf.Poll != nil {
		values = append(values, fmt.Sprintf("--poll=%d", *perf.Poll))
	}
	if perf.Priority != nil {
		values = append(values, fmt.Sprintf("--prio=%d", *perf.Priority))
	}
	_ = cfg
	return values
}
