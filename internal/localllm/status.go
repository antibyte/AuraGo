package localllm

import "time"

// PromptCacheStatus contains only value-free measurements and fingerprints.
type PromptCacheStatus struct {
	State              string  `json:"state"`
	SeedFingerprint    string  `json:"seed_fingerprint,omitempty"`
	ToolsetFingerprint string  `json:"toolset_fingerprint,omitempty"`
	StableToolCount    int     `json:"stable_tool_count"`
	SeedTokens         uint64  `json:"seed_tokens,omitempty"`
	CacheRAMMiB        int     `json:"cache_ram_mib"`
	CheckpointProfile  string  `json:"checkpoint_profile,omitempty"`
	LastHit            bool    `json:"last_hit"`
	Hits               uint64  `json:"hits"`
	Requests           uint64  `json:"requests"`
	CachedTokens       uint64  `json:"cached_tokens"`
	ProcessedTokens    uint64  `json:"processed_tokens"`
	HitRate            float64 `json:"hit_rate"`
	WarmupDurationMS   float64 `json:"warmup_duration_ms,omitempty"`
	ColdTTFTMS         float64 `json:"cold_ttft_ms,omitempty"`
	WarmTTFTMS         float64 `json:"warm_ttft_ms,omitempty"`
	ErrorCode          string  `json:"error_code,omitempty"`
}

// Status is the sanitized manager state returned by the admin API.
type Status struct {
	State                 string            `json:"state"`
	Progress              float64           `json:"progress"`
	Compatibility         string            `json:"compatibility"`
	Warnings              []string          `json:"warnings"`
	Backend               string            `json:"backend,omitempty"`
	PhysicalDevice        string            `json:"physical_device,omitempty"`
	ResolvedProfile       string            `json:"resolved_profile,omitempty"`
	ActualDevice          string            `json:"actual_device,omitempty"`
	VRAMBytes             int64             `json:"vram_bytes,omitempty"`
	ModelSHA256           string            `json:"model_sha256,omitempty"`
	DraftSHA256           string            `json:"draft_sha256,omitempty"`
	ImageDigest           string            `json:"image_digest,omitempty"`
	ContextSize           int               `json:"context_size"`
	ResolvedParameters    []string          `json:"resolved_parameters,omitempty"`
	GPUOffloadVerified    bool              `json:"gpu_offload_verified"`
	MemoryProfileVerified bool              `json:"memory_profile_verified"`
	ToolCallVerified      bool              `json:"tool_call_verified"`
	MTP                   MTPDecision       `json:"mtp"`
	ActiveRequests        int               `json:"active_requests"`
	IdleDeadline          *time.Time        `json:"idle_deadline,omitempty"`
	PendingRestart        bool              `json:"pending_restart"`
	DesiredFingerprint    string            `json:"desired_fingerprint,omitempty"`
	AppliedFingerprint    string            `json:"applied_fingerprint,omitempty"`
	VerifiedFingerprint   string            `json:"verified_fingerprint,omitempty"`
	VerifiedContextSize   int               `json:"verified_context_size,omitempty"`
	Operation             string            `json:"operation,omitempty"`
	OperationInProgress   bool              `json:"operation_in_progress"`
	LastHealthCheck       *time.Time        `json:"last_health_check,omitempty"`
	HardwareFingerprint   string            `json:"hardware_fingerprint,omitempty"`
	AcknowledgementDue    bool              `json:"acknowledgement_required"`
	ErrorCode             string            `json:"error_code,omitempty"`
	Recommendation        string            `json:"recommendation,omitempty"`
	ReleaseManifestReady  bool              `json:"release_manifest_ready"`
	Role                  string            `json:"role"`
	ConfigRevision        string            `json:"config_revision,omitempty"`
	PerformanceProfile    string            `json:"performance_profile,omitempty"`
	PromptCache           PromptCacheStatus `json:"prompt_cache"`
}

// UnavailableError triggers immediate failover when AuraGo-Qwen is primary.
type UnavailableError struct {
	Code string
	Err  error
}

func (e *UnavailableError) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Err.Error()
	}
	return e.Code
}

func (e *UnavailableError) Unwrap() error           { return e.Err }
func (e *UnavailableError) ImmediateFailover() bool { return true }
