package dockerstacks

// LintInput is what the editor sends while typing.
type LintInput struct {
	Compose string `json:"compose"`
	Env     string `json:"env"`
	Stack   string `json:"stack,omitempty"` // its own published ports are not conflicts
}

// Diagnostic is one finding. Lines are 1-based, 0 means the whole file. Every code except env_syntax refers to the
// compose file; env_syntax lines refer to the .env file (Params["file"] is ".env").
type Diagnostic struct {
	Line     int               `json:"line"`
	Column   int               `json:"column,omitempty"`
	Severity string            `json:"severity"` // "error" | "warning" | "hint"
	Code     string            `json:"code"`     // the UI localizes lint_<code>
	Message  string            `json:"message"`  // English fallback
	Params   map[string]string `json:"params,omitempty"`
	Service  string            `json:"service,omitempty"`
	Fix      *Fix              `json:"fix,omitempty"`
}

// Fix is a one-click remedy the UI can offer.
type Fix struct {
	Kind     string `json:"kind"` // "env_random": add VARIABLE=<random> to .env
	Variable string `json:"variable"`
}

// LintResult is the answer of Lint.
type LintResult struct {
	Diagnostics []Diagnostic `json:"diagnostics"` // sorted by line, then severity
	Preview     Preview      `json:"preview"`
}

// Preview is what the new-stack screen shows next to the editor.
type Preview struct {
	Services     []ServicePreview  `json:"services"`
	NamedVolumes []string          `json:"named_volumes"` // volumes compose will create
	Variables    []VariablePreview `json:"variables"`
}

type ServicePreview struct {
	Name    string          `json:"name"`
	Line    int             `json:"line"`
	Image   string          `json:"image,omitempty"`
	Build   bool            `json:"build,omitempty"`
	Ports   []PortPreview   `json:"ports"`
	Volumes []VolumePreview `json:"volumes"`
}

type PortPreview struct {
	HostIP    string `json:"host_ip,omitempty"`
	Host      string `json:"host"`      // "8080", "8000-8010" or "" (not published)
	Container string `json:"container"` // "80" or "8000-8010"
	Protocol  string `json:"protocol"`
	Conflict  string `json:"conflict,omitempty"` // container already publishing the host port
}

type VolumePreview struct {
	Kind     string `json:"kind"` // "named" | "bind" | "relative" | "anonymous" | "tmpfs"
	Source   string `json:"source,omitempty"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"read_only,omitempty"`
}

// VariablePreview describes one variable of the compose file. Line is its first use; HasDefault is true when every
// use names a default; Required when any use is ${X:?msg} / ${X?msg}; InEnv when .env defines it.
type VariablePreview struct {
	Name       string `json:"name"`
	Line       int    `json:"line"`
	HasDefault bool   `json:"has_default"`
	Required   bool   `json:"required"`
	InEnv      bool   `json:"in_env"`
}
