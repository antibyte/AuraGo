package virtualcomputers

import (
	"encoding/json"
	"time"
)

const (
	ProviderBoringComputers    = "boring_computers"
	ControlPlaneSSHHost        = "ssh_host"
	ControlPlaneLocalHost      = "local_host"
	MinTTLSeconds              = 15
	MaxTTLSeconds              = 900
	DefaultTTLSeconds          = 600
	AgentTaskKindShell         = "shell"
	AgentTaskKindDesktop       = "desktop"
	AgentTaskStatusQueued      = "queued"
	AgentTaskStatusRunning     = "running"
	AgentTaskStatusCompleted   = "completed"
	AgentTaskStatusFailed      = "failed"
	AgentTaskStatusCanceled    = "canceled"
	AgentTaskStatusInterrupted = "interrupted"
)

type AgentTask struct {
	ID              string           `json:"id"`
	MachineID       string           `json:"machine_id"`
	Kind            string           `json:"kind"`
	Instruction     string           `json:"instruction,omitempty"`
	Status          string           `json:"status"`
	PreviewPort     int              `json:"preview_port,omitempty"`
	Error           string           `json:"error,omitempty"`
	Events          []AgentTaskEvent `json:"events,omitempty"`
	EventsTruncated bool             `json:"events_truncated,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	StartedAt       *time.Time       `json:"started_at,omitempty"`
	CompletedAt     *time.Time       `json:"completed_at,omitempty"`
}

type AgentTaskEvent struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	Text      string    `json:"text,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type ControlPlaneConfig struct {
	Mode         string `json:"mode"`
	Host         string `json:"host"`
	SSHPort      int    `json:"ssh_port"`
	CredentialID string `json:"credential_id"`
	InstallDir   string `json:"install_dir"`
	BoringdURL   string `json:"boringd_url"`
}

type ToolConfig struct {
	Enabled             bool
	Provider            string
	AutoSetup           bool
	ReadOnly            bool
	ToolGate            bool
	ControlPlane        ControlPlaneConfig
	Storage             StorageConfig
	LedgerPath          string
	BoringdURL          string
	BoringToken         string
	BoringAnthropicKey  string
	BoringOpenRouterKey string
	S3AccessKeyID       string
	S3SecretKey         string
	DefaultTemplate     string
	DefaultTTLSeconds   int
	MaxTTLSeconds       int
	MaxRunningMachines  int
	MaxForks            int
	AllowInternet       bool
	AllowPersistent     bool
	AllowPublish        bool
	AllowVolumes        bool
	AllowAgentTasks     bool
}

type StorageConfig struct {
	Endpoint string
	Bucket   string
	Region   string
	UseSSL   bool
}

type Machine struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name,omitempty"`
	Template   string                 `json:"template,omitempty"`
	Status     string                 `json:"status,omitempty"`
	Mode       string                 `json:"mode,omitempty"`
	BootMS     int                    `json:"boot_ms,omitempty"`
	Display    bool                   `json:"display"`
	Persistent bool                   `json:"persistent"`
	Parent     string                 `json:"parent,omitempty"`
	TTLSeconds int                    `json:"ttl_seconds,omitempty"`
	ExpiresAt  *time.Time             `json:"expires_at,omitempty"`
	CreatedAt  *time.Time             `json:"created_at,omitempty"`
	WebPorts   []int                  `json:"web_ports,omitempty"`
	Raw        map[string]interface{} `json:"raw,omitempty"`
}

func (m *Machine) UnmarshalJSON(data []byte) error {
	var wire struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Template   string `json:"template"`
		Status     string `json:"status"`
		Mode       string `json:"mode"`
		BootMS     int    `json:"boot_ms"`
		Display    bool   `json:"display"`
		Persistent bool   `json:"persistent"`
		Parent     string `json:"parent"`
		TTLSeconds int    `json:"ttl_seconds"`
		ExpiresAt  string `json:"expires_at"`
		CreatedAt  string `json:"created_at"`
		WebPorts   []int  `json:"web_ports"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	var raw map[string]interface{}
	_ = json.Unmarshal(data, &raw)
	*m = Machine{
		ID: wire.ID, Name: wire.Name, Template: wire.Template, Status: wire.Status,
		Mode: wire.Mode, BootMS: wire.BootMS, Display: wire.Display,
		Persistent: wire.Persistent, Parent: wire.Parent, TTLSeconds: wire.TTLSeconds,
		ExpiresAt: parseOptionalTime(wire.ExpiresAt), CreatedAt: parseOptionalTime(wire.CreatedAt),
		WebPorts: wire.WebPorts, Raw: raw,
	}
	return nil
}

type Template struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Published   bool                   `json:"published"`
	Display     bool                   `json:"display"`
	SizeMB      int64                  `json:"size_mb,omitempty"`
	CreatedAt   *time.Time             `json:"created_at,omitempty"`
	Source      string                 `json:"source_template,omitempty"`
	Raw         map[string]interface{} `json:"raw,omitempty"`
}

func (t *Template) UnmarshalJSON(data []byte) error {
	var wire struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Published   bool   `json:"published"`
		Display     bool   `json:"display"`
		SizeMB      int64  `json:"size_mb"`
		CreatedAt   string `json:"created_at"`
		Source      string `json:"source_template"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	var raw map[string]interface{}
	_ = json.Unmarshal(data, &raw)
	id := wire.ID
	if id == "" {
		id = wire.Name
	}
	*t = Template{ID: id, Name: wire.Name, Description: wire.Description, Published: wire.Published,
		Display: wire.Display, SizeMB: wire.SizeMB, CreatedAt: parseOptionalTime(wire.CreatedAt), Source: wire.Source, Raw: raw}
	return nil
}

type Volume struct {
	ID                 string                 `json:"id"`
	Name               string                 `json:"name,omitempty"`
	SizeBytes          int64                  `json:"size_bytes,omitempty"`
	CreatedAt          *time.Time             `json:"created_at,omitempty"`
	ExpiresAt          *time.Time             `json:"expires_at,omitempty"`
	QuotaMB            int64                  `json:"quota_mb,omitempty"`
	LastVerifiedAt     *time.Time             `json:"last_verified_at,omitempty"`
	VerificationStatus string                 `json:"verification_status,omitempty"`
	Raw                map[string]interface{} `json:"raw,omitempty"`
}

func (v *Volume) UnmarshalJSON(data []byte) error {
	var wire struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		SizeBytes int64  `json:"size_bytes"`
		CreatedAt string `json:"created_at"`
		ExpiresAt string `json:"expires_at"`
		QuotaMB   int64  `json:"quota_mb"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	var raw map[string]interface{}
	_ = json.Unmarshal(data, &raw)
	*v = Volume{ID: wire.ID, Name: wire.Name, SizeBytes: wire.SizeBytes,
		CreatedAt: parseOptionalTime(wire.CreatedAt), ExpiresAt: parseOptionalTime(wire.ExpiresAt),
		QuotaMB: wire.QuotaMB, Raw: raw}
	return nil
}

type LaunchMachineRequest struct {
	Template      string                 `json:"template,omitempty"`
	Name          string                 `json:"name,omitempty"`
	TTLSeconds    int                    `json:"ttl_seconds,omitempty"`
	AllowInternet bool                   `json:"allow_internet,omitempty"`
	Persistent    bool                   `json:"persistent,omitempty"`
	VolumeID      string                 `json:"volume_id,omitempty"`
	Volumes       []string               `json:"volumes,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

type launchMachineRequest struct {
	Template      string `json:"template,omitempty"`
	TTLSeconds    int    `json:"ttl_seconds,omitempty"`
	AllowInternet bool   `json:"net,omitempty"`
	Volume        string `json:"volume,omitempty"`
	Persistent    bool   `json:"persistent,omitempty"`
}

type ExecRequest struct {
	Command string   `json:"command,omitempty"`
	Args    []string `json:"-"`
	Timeout int      `json:"timeout_seconds,omitempty"`
}

type ExecResult struct {
	ExitCode   *int   `json:"exit_code"`
	Output     string `json:"output,omitempty"`
	TimedOut   bool   `json:"timed_out"`
	DurationMS int64  `json:"duration_ms,omitempty"`
}

type Screenshot struct {
	MimeType string `json:"mime_type,omitempty"`
	Data     []byte `json:"-"`
	Base64   string `json:"data_base64,omitempty"`
}

type SetupStatus struct {
	Configured   bool            `json:"configured"`
	Healthy      bool            `json:"healthy"`
	Message      string          `json:"message,omitempty"`
	Preflight    PreflightResult `json:"preflight,omitempty"`
	ControlPlane ComponentStatus `json:"control_plane"`
	Management   ComponentStatus `json:"management"`
}

// ComponentStatus reports the health of one managed Virtual Computers component.
type ComponentStatus struct {
	Configured bool   `json:"configured"`
	Healthy    bool   `json:"healthy"`
	Message    string `json:"message,omitempty"`
}

type SetupInstallOptions struct {
	InstallDir         string
	BoringdURL         string
	Token              string
	AnthropicKey       string
	OpenRouterKey      string
	S3AccessKeyID      string
	S3SecretKey        string
	S3Endpoint         string
	S3Bucket           string
	S3Region           string
	S3UseSSL           bool
	MaxRunningMachines int
	MaxForks           int
	AllowInternet      bool
	AllowPersistent    bool
	AllowPublish       bool
	AllowVolumes       bool
	SkipDesktop        bool
}

func parseOptionalTime(value string) *time.Time {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil
	}
	parsed = parsed.UTC()
	return &parsed
}
