package decisions

// Request is a bounded decision-scoring request. Images are resolved and
// authorized by the server before this package receives them.
type Request struct {
	State     any        `json:"state"`
	Questions []Question `json:"questions"`
	Images    []Image    `json:"images,omitempty"`
}

// Question describes one decision to score against its candidate options.
type Question struct {
	ID           string   `json:"id"`
	Type         string   `json:"type"`
	Instructions string   `json:"instructions"`
	Options      []Option `json:"options"`
}

// Option is one candidate. Value, when present, contributes to the expected
// numeric score returned for its question.
type Option struct {
	ID    string   `json:"id"`
	Label string   `json:"label"`
	Value *float64 `json:"value,omitempty"`
}

// Image contains trusted image bytes supplied by the server. Data is never
// serialized into status or logs.
type Image struct {
	Data []byte `json:"-"`
	MIME string `json:"mime_type"`
}

// Response contains normalized choices. Calibration is "uncalibrated" unless
// the selected model has a separately verified calibration contract.
type Response struct {
	Model         string            `json:"model"`
	ModelRevision string            `json:"model_revision"`
	Answers       map[string]Answer `json:"answers"`
	Calibration   string            `json:"calibration"`
	Runtime       RuntimeInfo       `json:"runtime"`
	Usage         Usage             `json:"usage"`
}

// Answer is the normalized result for one Question.
type Answer struct {
	Type          string             `json:"type"`
	Decision      string             `json:"decision"`
	Probabilities map[string]float64 `json:"probabilities"`
	Confidence    float64            `json:"confidence"`
	Choice        string             `json:"choice,omitempty"`
	Noul          *float64           `json:"noul,omitempty"`
	Score         *float64           `json:"score,omitempty"`
}

// RuntimeInfo describes the runtime that produced a response and its
// observed device attestation.
type RuntimeInfo struct {
	Deployment          string `json:"deployment,omitempty"`
	Backend             string `json:"backend,omitempty"`
	Device              string `json:"device,omitempty"`
	RuntimeBuild        string `json:"runtime_build,omitempty"`
	ModelRevision       string `json:"model_revision,omitempty"`
	ModelSHA256         string `json:"model_sha256,omitempty"`
	ProjectorRevision   string `json:"projector_revision,omitempty"`
	ProjectorSHA256     string `json:"projector_sha256,omitempty"`
	RuntimeBinarySHA256 string `json:"runtime_binary_sha256,omitempty"`
	GPUOffloadVerified  bool   `json:"gpu_offload_verified"`
	DeviceAttested      bool   `json:"device_attested"`
}

// Usage contains aggregate counts only; request text and images are excluded.
type Usage struct {
	Questions     int `json:"questions"`
	Candidates    int `json:"candidates"`
	Images        int `json:"images"`
	PromptTokens  int `json:"prompt_tokens"`
	ImageTokens   int `json:"image_tokens"`
	ContextTokens int `json:"context_tokens"`
}

// Status is a detached, safe-to-publish snapshot of the managed service.
type Status struct {
	Enabled            bool        `json:"enabled"`
	Installed          bool        `json:"installed"`
	Ready              bool        `json:"ready"`
	State              string      `json:"state"`
	Code               string      `json:"code,omitempty"`
	ErrorMessage       string      `json:"error_message,omitempty"`
	Deployment         string      `json:"deployment"`
	ExecutionMode      string      `json:"execution_mode,omitempty"`
	Backend            string      `json:"backend,omitempty"`
	Device             string      `json:"device,omitempty"`
	ModelRevision      string      `json:"model_revision,omitempty"`
	ModelSHA256        string      `json:"model_sha256,omitempty"`
	ProjectorRevision  string      `json:"projector_revision,omitempty"`
	ProjectorSHA256    string      `json:"projector_sha256,omitempty"`
	DeviceAttested     bool        `json:"device_attested"`
	GPUOffloadVerified bool        `json:"gpu_offload_verified"`
	CPUFallbackAllowed bool        `json:"cpu_fallback_allowed"`
	CPUFallbackActive  bool        `json:"cpu_fallback_active"`
	CPUFallbackReason  string      `json:"cpu_fallback_reason,omitempty"`
	Progress           Progress    `json:"progress"`
	Runtime            RuntimeInfo `json:"runtime"`
}

// Progress reports a bounded artifact/install phase without exposing paths.
type Progress struct {
	Phase   string  `json:"phase,omitempty"`
	Current int64   `json:"current_bytes,omitempty"`
	Total   int64   `json:"total_bytes,omitempty"`
	Percent float64 `json:"percent,omitempty"`
}
