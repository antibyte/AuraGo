package virtualcomputers

import "time"

const (
	WorkspaceProtocolVersion = "aurago.workspace.v1"
	WorkspaceVolumeFormat    = "workspace_v2"
	WorkspaceRPCPath         = "workspace"

	WorkspaceStateOpening = "opening"
	WorkspaceStateReady   = "ready"
	WorkspaceStateClosing = "closing"
	WorkspaceStateClosed  = "closed"
	WorkspaceStateFailed  = "failed"
	WorkspaceStateLost    = "lost"

	JobModeSync = "sync"
	JobModePTY  = "pty"

	JobStateQueued      = "queued"
	JobStateRunning     = "running"
	JobStateCompleted   = "completed"
	JobStateFailed      = "failed"
	JobStateCanceled    = "canceled"
	JobStateInterrupted = "interrupted"

	BrowserStateOpen   = "open"
	BrowserStateClosed = "closed"

	GrantUsageShell   = "shell"
	GrantUsageBrowser = "browser"
	GrantPending      = "pending_approval"
	GrantActive       = "active"
	GrantRevoked      = "revoked"
	GrantExpired      = "expired"
	GrantConsumed     = "consumed"

	ControlOwnerAgent = "agent"
	ControlOwnerHuman = "human"
)

type WorkspaceIdentity struct {
	SessionID string `json:"session_id"`
	MissionID string `json:"mission_id,omitempty"`
	Actor     string `json:"actor"`
	Admin     bool   `json:"-"`
}

type Workspace struct {
	ID                    string     `json:"id"`
	OwnerSessionID        string     `json:"owner_session_id"`
	MissionID             string     `json:"mission_id,omitempty"`
	Actor                 string     `json:"actor,omitempty"`
	MachineID             string     `json:"machine_id"`
	State                 string     `json:"state"`
	Template              string     `json:"template"`
	NetworkProfile        string     `json:"network_profile"`
	VolumeID              string     `json:"volume_id,omitempty"`
	Capabilities          []string   `json:"capabilities"`
	InstanceNonce         string     `json:"instance_nonce,omitempty"`
	ControlOwner          string     `json:"control_owner"`
	ControlLeaseExpiresAt *time.Time `json:"control_lease_expires_at,omitempty"`
	LastError             string     `json:"last_error,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
	LastActivityAt        time.Time  `json:"last_activity_at"`
	LeaseExpiresAt        time.Time  `json:"lease_expires_at"`
	MaxExpiresAt          time.Time  `json:"max_expires_at"`
}

type WorkspaceJob struct {
	ID              string     `json:"id"`
	WorkspaceID     string     `json:"workspace_id"`
	Mode            string     `json:"mode"`
	State           string     `json:"state"`
	CommandHash     string     `json:"command_hash,omitempty"`
	CommandSummary  string     `json:"command_summary,omitempty"`
	ExitCode        *int       `json:"exit_code,omitempty"`
	PID             int        `json:"pid,omitempty"`
	ProcessGroup    int        `json:"process_group,omitempty"`
	OutputCursor    int64      `json:"output_cursor,omitempty"`
	OutputTruncated bool       `json:"output_truncated,omitempty"`
	Error           string     `json:"error,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}

type BrowserSession struct {
	ID                    string     `json:"id"`
	WorkspaceID           string     `json:"workspace_id"`
	State                 string     `json:"state"`
	ActivePageID          string     `json:"active_page_id,omitempty"`
	URLOrigin             string     `json:"url_origin,omitempty"`
	ControlOwner          string     `json:"control_owner"`
	ControlLeaseExpiresAt *time.Time `json:"control_lease_expires_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type CredentialGrant struct {
	ID           string     `json:"id"`
	WorkspaceID  string     `json:"workspace_id"`
	CredentialID string     `json:"credential_id"`
	UsageType    string     `json:"usage_type"`
	Origin       string     `json:"origin,omitempty"`
	JobID        string     `json:"job_id,omitempty"`
	FieldNames   []string   `json:"field_names"`
	Purpose      string     `json:"purpose,omitempty"`
	Status       string     `json:"status"`
	RequestedBy  string     `json:"requested_by,omitempty"`
	ApprovedBy   string     `json:"approved_by,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	ExpiresAt    time.Time  `json:"expires_at"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
}

type WorkspaceEvent struct {
	ID          int64                  `json:"id"`
	WorkspaceID string                 `json:"workspace_id"`
	Type        string                 `json:"type"`
	Summary     string                 `json:"summary,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
}

type WorkspaceOpenRequest struct {
	Template       string `json:"template,omitempty"`
	NetworkProfile string `json:"network_profile,omitempty"`
	VolumeID       string `json:"volume_id,omitempty"`
}

type WorkspaceExecRequest struct {
	Command        string            `json:"command"`
	WorkingDir     string            `json:"working_dir,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
	Environment    map[string]string `json:"environment,omitempty"`
	MaxOutputBytes int64             `json:"max_output_bytes,omitempty"`
}

type WorkspaceStartJobRequest struct {
	Command                string `json:"command"`
	WorkingDir             string `json:"working_dir,omitempty"`
	PTY                    bool   `json:"pty,omitempty"`
	Rows                   uint16 `json:"rows,omitempty"`
	Cols                   uint16 `json:"cols,omitempty"`
	TimeoutSeconds         int    `json:"timeout_seconds,omitempty"`
	WaitForCredentialGrant bool   `json:"wait_for_credential_grant,omitempty"`
	MaxOutputBytes         int64  `json:"max_output_bytes,omitempty"`
}

type WorkspaceJobOutput struct {
	JobID      string `json:"job_id"`
	Data       string `json:"data"`
	Cursor     int64  `json:"cursor"`
	NextCursor int64  `json:"next_cursor"`
	EOF        bool   `json:"eof"`
	Truncated  bool   `json:"truncated"`
}

type WorkspaceFileEntry struct {
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	Directory  bool      `json:"directory"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
}

type WorkspaceCapabilities struct {
	ProtocolVersion string   `json:"protocol_version"`
	MachineID       string   `json:"machine_id"`
	InstanceNonce   string   `json:"instance_nonce"`
	Capabilities    []string `json:"capabilities"`
	MaxMessageBytes int64    `json:"max_message_bytes"`
}

type LegacyVolumeImportEntry struct {
	Path           string `json:"path"`
	Kind           string `json:"kind"`
	SizeBytes      int64  `json:"size_bytes"`
	FileCount      int    `json:"file_count"`
	DirectoryCount int    `json:"directory_count"`
}

type LegacyVolumeImportPreview struct {
	ID             string                    `json:"id"`
	SourceVolumeID string                    `json:"source_volume_id"`
	Paths          []string                  `json:"paths"`
	Entries        []LegacyVolumeImportEntry `json:"entries"`
	TotalBytes     int64                     `json:"total_bytes"`
	FileCount      int                       `json:"file_count"`
	ExpiresAt      time.Time                 `json:"expires_at"`
}

type BrowserActionRequest struct {
	Operation  string                 `json:"operation"`
	SessionID  string                 `json:"session_id,omitempty"`
	PageID     string                 `json:"page_id,omitempty"`
	URL        string                 `json:"url,omitempty"`
	ElementRef string                 `json:"element_ref,omitempty"`
	Selector   string                 `json:"selector,omitempty"`
	Text       string                 `json:"text,omitempty"`
	Value      string                 `json:"value,omitempty"`
	Key        string                 `json:"key,omitempty"`
	TimeoutMS  int                    `json:"timeout_ms,omitempty"`
	FullPage   bool                   `json:"full_page,omitempty"`
	X          float64                `json:"x,omitempty"`
	Y          float64                `json:"y,omitempty"`
	DeltaX     float64                `json:"delta_x,omitempty"`
	DeltaY     float64                `json:"delta_y,omitempty"`
	ToX        float64                `json:"to_x,omitempty"`
	ToY        float64                `json:"to_y,omitempty"`
	Path       string                 `json:"path,omitempty"`
	GrantID    string                 `json:"grant_id,omitempty"`
	Fields     map[string]string      `json:"fields,omitempty"`
	Options    map[string]interface{} `json:"options,omitempty"`
}

type BrowserActionResult struct {
	Session BrowserSession         `json:"session,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

type CredentialGrantRequest struct {
	CredentialID string   `json:"credential_id"`
	UsageType    string   `json:"usage_type"`
	Origin       string   `json:"origin,omitempty"`
	JobID        string   `json:"job_id,omitempty"`
	FieldNames   []string `json:"field_names,omitempty"`
	Purpose      string   `json:"purpose,omitempty"`
}
