package dockerstacks

import "time"

// FileContent is one file of a stack as it is on the host.
type FileContent struct {
	Name     string `json:"name"` // "compose.yaml", ".env", ...
	Content  string `json:"content"`
	Revision string `json:"revision"` // "" when the file does not exist
	Exists   bool   `json:"exists"`
}

// Status is the state of a stack derived from its containers.
type Status string

const (
	StatusDraft   Status = "draft"   // files, no containers
	StatusRunning Status = "running" // every container running and none unhealthy/restarting
	StatusPartial Status = "partial" // some running, others down, unhealthy or restarting
	StatusStopped Status = "stopped" // nothing running, no non-zero exit
	StatusError   Status = "error"   // nothing running, at least one non-zero exit (or dead)
)

// ServiceCount counts the containers of one project.
type ServiceCount struct {
	Total   int `json:"total"`
	Running int `json:"running"`
}

// PortLink is one published port.
type PortLink struct {
	Service     string `json:"service"`
	IP          string `json:"ip,omitempty"` // host IP the port is bound to ("" or "0.0.0.0" = all)
	PublicPort  int    `json:"public_port"`
	PrivatePort int    `json:"private_port"`
	Protocol    string `json:"protocol"` // tcp | udp
}

// OperationRef points at the operation that is running on a stack.
type OperationRef struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	State string `json:"state"`
}

// StackSummary is one row of the stack list.
type StackSummary struct {
	Name         string        `json:"name"`
	HostID       string        `json:"host_id"`
	Dir          string        `json:"dir"`          // <stacks_dir>/<name>
	ComposeFile  string        `json:"compose_file"` // one of ComposeFileNames
	HasEnv       bool          `json:"has_env"`
	Status       Status        `json:"status"`
	Services     ServiceCount  `json:"services"`
	Ports        []PortLink    `json:"ports"`
	ContainerIDs []string      `json:"container_ids"`       // 12-char ids
	Created      int64         `json:"created,omitempty"`   // oldest container creation (unix s); 0 for drafts
	Operation    *OperationRef `json:"operation,omitempty"` // the running operation, if any
}

// ProjectSummary is a compose project that is not one of the host's stacks.
type ProjectSummary struct {
	Name         string       `json:"name"`
	WorkingDir   string       `json:"working_dir"`
	Status       Status       `json:"status"`
	Services     ServiceCount `json:"services"`
	Ports        []PortLink   `json:"ports"`
	ContainerIDs []string     `json:"container_ids"`
}

// IgnoredFolder is a folder of the stacks directory that holds a compose file
// but cannot be a stack.
type IgnoredFolder struct {
	Name   string `json:"name"`
	Reason string `json:"reason"` // "name_invalid"
}

// Listing is the answer of List.
type Listing struct {
	HostID    string           `json:"host_id"`
	StacksDir string           `json:"stacks_dir"`
	Stacks    []StackSummary   `json:"stacks"`   // sorted by name
	External  []ProjectSummary `json:"external"` // sorted by name
	Ignored   []IgnoredFolder  `json:"ignored"`
	ScannedAt time.Time        `json:"scanned_at"`
	Cached    bool             `json:"cached"`
}

// ServiceDetail is one service of a stack.
type ServiceDetail struct {
	Name        string     `json:"name"`
	Image       string     `json:"image"` // from the compose file, else the container
	State       string     `json:"state"` // running|exited|created|restarting|paused|dead|missing
	Health      string     `json:"health,omitempty"`
	ExitCode    *int       `json:"exit_code,omitempty"`
	ContainerID string     `json:"container_id,omitempty"`
	Ports       []PortLink `json:"ports"`
}

// StackDetail is the answer of Get.
type StackDetail struct {
	StackSummary
	ServiceList []ServiceDetail `json:"service_list"` // compose services first (file order), then stray containers
	Compose     FileContent     `json:"compose"`
	Env         FileContent     `json:"env"`
	ParseError  string          `json:"parse_error,omitempty"` // compose file not parseable: services from containers only
}

// CreateInput is the body of Create.
type CreateInput struct {
	Name    string  `json:"name"`
	Compose string  `json:"compose"`
	Env     *string `json:"env,omitempty"` // nil = no .env file
}

// FileWrite is one file of a write.
type FileWrite struct {
	Content  string `json:"content"`
	Revision string `json:"revision"` // the revision the edit is based on ("" = file must not exist yet)
}

// FilesInput is the body of WriteFiles.
type FilesInput struct {
	Compose *FileWrite `json:"compose,omitempty"`
	Env     *FileWrite `json:"env,omitempty"`
}

// FileRevisions are the revisions of the files a write stored.
type FileRevisions struct {
	Compose string `json:"compose,omitempty"`
	Env     string `json:"env,omitempty"`
}

// ProjectVolume is one volume a compose project created.
type ProjectVolume struct {
	Name   string `json:"name"`
	Driver string `json:"driver"`
	Size   int64  `json:"size"` // bytes; -1 when the engine does not report it
}

// HelperStatus is what a host has of the helpers the stacks need.
type HelperStatus struct {
	APIVersion         string `json:"api_version"`
	APISupported       bool   `json:"api_supported"`
	RunnerImage        string `json:"runner_image"` // RunnerImageTag
	RunnerImagePresent bool   `json:"runner_image_present"`
	IOContainer        string `json:"io_container"` // "ok" | "missing" | "mismatch"
	Prepared           bool   `json:"prepared"`     // APISupported && RunnerImagePresent && IOContainer == "ok"
}

// PublishedPort is a host port that a container on the host publishes.
type PublishedPort struct {
	HostIP    string `json:"host_ip"`
	Port      int    `json:"port"`
	Protocol  string `json:"protocol"`
	Container string `json:"container"` // container name without "/"
	Project   string `json:"project"`   // compose project label, "" for single containers
}

// Operation kinds.
const (
	KindPrepare = "prepare" // host level, stack ""
	KindConfig  = "config"
	KindUp      = "up"   // check + up -d (pulls missing images); followed by the health wait
	KindDown    = "down" // Options.Volumes adds --volumes (UI only)
	KindStop    = "stop"
	KindRestart = "restart"
	KindPull    = "pull"
	KindUpdate  = "update" // pull, then up -d; followed by the health wait
	KindDelete  = "delete" // down (by project name) + remove the stack folder; Options.Volumes adds --volumes
)

// StackOperationKinds are the kinds Start accepts.
var StackOperationKinds = []string{KindConfig, KindUp, KindDown, KindStop, KindRestart, KindPull, KindUpdate, KindDelete}

// Operation states.
const (
	StateQueued      = "queued"
	StateRunning     = "running"
	StateHealthWait  = "health_wait"
	StateSucceeded   = "succeeded"
	StateFailed      = "failed"
	StateInterrupted = "interrupted" // AuraGo restarted and no runner container was left to reattach to
)

// OpOptions are the options of an operation.
type OpOptions struct {
	Volumes bool `json:"volumes,omitempty"`
}

// Operation is one entry of the journal.
type Operation struct {
	ID         string     `json:"id"` // "op_" + 16 lowercase hex
	HostID     string     `json:"host_id"`
	Stack      string     `json:"stack"`
	Kind       string     `json:"kind"`
	Options    OpOptions  `json:"options"`
	State      string     `json:"state"`
	RunnerID   string     `json:"runner_id,omitempty"`
	ExitCode   *int       `json:"exit_code,omitempty"`
	Code       string     `json:"code,omitempty"`       // error code on failure (runner_image_missing, timeout, runner_failed, ...)
	Error      string     `json:"error,omitempty"`      // one line
	ErrorLine  int        `json:"error_line,omitempty"` // compose file line named by the error ("line 12:"), 0 = none
	Warning    string     `json:"warning,omitempty"`    // health wait result, e.g. "db is unhealthy"
	Output     string     `json:"output"`               // last OutputTailBytes, ANSI kept
	Ports      []PortLink `json:"ports,omitempty"`      // after up/update
	CreatedAt  time.Time  `json:"created_at"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// Done reports a finished operation: succeeded, failed or interrupted.
func (o Operation) Done() bool {
	return o.State == StateSucceeded || o.State == StateFailed || o.State == StateInterrupted
}

// OperationFilter selects journal entries.
type OperationFilter struct {
	HostID string
	Stack  string
	Active bool // queued | running | health_wait only
	Limit  int  // default 20, max 100
}

// Event is one item of an operation's live stream.
type Event struct {
	Type      string     `json:"type"` // "output" | "progress" | "state" | "done"
	Seq       int64      `json:"seq"`
	Text      string     `json:"text,omitempty"`      // output: raw chunk (may hold several lines)
	Progress  *Progress  `json:"progress,omitempty"`  // progress: one parsed compose --progress json line
	Operation *Operation `json:"operation,omitempty"` // state: without Output; done: full
}

// Progress is one parsed progress line of compose or of an image pull.
type Progress struct {
	ID      string  `json:"id"`
	Parent  string  `json:"parent,omitempty"`
	Status  string  `json:"status,omitempty"`
	Text    string  `json:"text,omitempty"`
	Current int64   `json:"current,omitempty"`
	Total   int64   `json:"total,omitempty"`
	Percent float64 `json:"percent,omitempty"`
}
