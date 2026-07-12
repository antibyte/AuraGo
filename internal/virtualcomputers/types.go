package virtualcomputers

import "time"

const (
	ProviderBoringComputers = "boring_computers"
	MinTTLSeconds           = 15
	MaxTTLSeconds           = 900
	DefaultTTLSeconds       = 600
)

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

type Machine struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name,omitempty"`
	Template   string                 `json:"template,omitempty"`
	Status     string                 `json:"status,omitempty"`
	TTLSeconds int                    `json:"ttl_seconds,omitempty"`
	ExpiresAt  time.Time              `json:"expires_at,omitempty"`
	CreatedAt  time.Time              `json:"created_at,omitempty"`
	WebPorts   []int                  `json:"web_ports,omitempty"`
	Raw        map[string]interface{} `json:"raw,omitempty"`
}

type Template struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Raw         map[string]interface{} `json:"raw,omitempty"`
}

type Volume struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name,omitempty"`
	SizeBytes int64                  `json:"size_bytes,omitempty"`
	Raw       map[string]interface{} `json:"raw,omitempty"`
}

type LaunchMachineRequest struct {
	Template      string                 `json:"template,omitempty"`
	Name          string                 `json:"name,omitempty"`
	TTLSeconds    int                    `json:"ttl_seconds,omitempty"`
	AllowInternet bool                   `json:"allow_internet,omitempty"`
	Persistent    bool                   `json:"persistent,omitempty"`
	Volumes       []string               `json:"volumes,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

type launchMachineRequest struct {
	Template      string                 `json:"template,omitempty"`
	Name          string                 `json:"name,omitempty"`
	TTLSeconds    int                    `json:"ttl_seconds,omitempty"`
	AllowInternet bool                   `json:"allow_internet,omitempty"`
	Persistent    bool                   `json:"persistent,omitempty"`
	Volumes       []string               `json:"volumes,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

type ExecRequest struct {
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	Timeout int      `json:"timeout_seconds,omitempty"`
}

type ExecResult struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	Output   string `json:"output,omitempty"`
}

type Screenshot struct {
	MimeType string `json:"mime_type,omitempty"`
	Data     []byte `json:"-"`
	Base64   string `json:"data_base64,omitempty"`
}

type SetupStatus struct {
	Configured bool   `json:"configured"`
	Healthy    bool   `json:"healthy"`
	Message    string `json:"message,omitempty"`
}

type SetupInstallOptions struct {
	InstallDir         string
	Token              string
	AnthropicKey       string
	OpenRouterKey      string
	S3AccessKeyID      string
	S3SecretKey        string
	MaxRunningMachines int
	MaxForks           int
	AllowInternet      bool
	AllowPersistent    bool
	AllowPublish       bool
	AllowVolumes       bool
	SkipDesktop        bool
}
