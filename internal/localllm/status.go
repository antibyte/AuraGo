package localllm

import "time"

// Status is the sanitized manager state returned by the admin API.
type Status struct {
	State                 string      `json:"state"`
	Progress              float64     `json:"progress"`
	Compatibility         string      `json:"compatibility"`
	Warnings              []string    `json:"warnings"`
	Backend               string      `json:"backend,omitempty"`
	PhysicalDevice        string      `json:"physical_device,omitempty"`
	ResolvedProfile       string      `json:"resolved_profile,omitempty"`
	ActualDevice          string      `json:"actual_device,omitempty"`
	VRAMBytes             int64       `json:"vram_bytes,omitempty"`
	ModelSHA256           string      `json:"model_sha256,omitempty"`
	DraftSHA256           string      `json:"draft_sha256,omitempty"`
	ImageDigest           string      `json:"image_digest,omitempty"`
	ContextSize           int         `json:"context_size"`
	ResolvedParameters    []string    `json:"resolved_parameters,omitempty"`
	GPUOffloadVerified    bool        `json:"gpu_offload_verified"`
	MemoryProfileVerified bool        `json:"memory_profile_verified"`
	ToolCallVerified      bool        `json:"tool_call_verified"`
	MTP                   MTPDecision `json:"mtp"`
	ActiveRequests        int         `json:"active_requests"`
	IdleDeadline          *time.Time  `json:"idle_deadline,omitempty"`
	PendingRestart        bool        `json:"pending_restart"`
	DesiredFingerprint    string      `json:"desired_fingerprint,omitempty"`
	AppliedFingerprint    string      `json:"applied_fingerprint,omitempty"`
	VerifiedFingerprint   string      `json:"verified_fingerprint,omitempty"`
	VerifiedContextSize   int         `json:"verified_context_size,omitempty"`
	Operation             string      `json:"operation,omitempty"`
	OperationInProgress   bool        `json:"operation_in_progress"`
	LastHealthCheck       *time.Time  `json:"last_health_check,omitempty"`
	HardwareFingerprint   string      `json:"hardware_fingerprint,omitempty"`
	AcknowledgementDue    bool        `json:"acknowledgement_required"`
	ErrorCode             string      `json:"error_code,omitempty"`
	Recommendation        string      `json:"recommendation,omitempty"`
	ReleaseManifestReady  bool        `json:"release_manifest_ready"`
	Role                  string      `json:"role"`
	ConfigRevision        string      `json:"config_revision,omitempty"`
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
