package flows

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"
)

// DefaultNodeTimeout applies when neither the node nor its definition sets a timeout.
const DefaultNodeTimeout = 120 * time.Second

// Effect is an outward effect of a node, shown before publishing.
type Effect string

const (
	EffectSendsMessage    Effect = "sends_message"
	EffectWritesFiles     Effect = "writes_files"
	EffectControlsDevices Effect = "controls_devices"
	EffectRunsCode        Effect = "runs_code"
	EffectDeletes         Effect = "deletes"
	EffectSystemChange    Effect = "system_change"
)

// Availability states of a node type.
const (
	AvailableState  = "available"
	NeedsSetupState = "needs_setup"
	BlockedState    = "blocked"
)

// Availability tells the UI and the validator whether a node type can run now.
type Availability struct {
	State         string `json:"state"`
	Reason        string `json:"reason,omitempty"`
	ConfigSection string `json:"config_section,omitempty"`
}

// Parameter kinds understood by the editor's form generator.
const (
	ParamText           = "text"
	ParamTextarea       = "textarea"
	ParamNumber         = "number"
	ParamBool           = "bool"
	ParamSelect         = "select"
	ParamSegmented      = "segmented"
	ParamMultiSelect    = "multiselect"
	ParamFile           = "file"
	ParamJSON           = "json"
	ParamKeyValue       = "keyvalue"
	ParamSecretRef      = "secret_ref"
	ParamConditionGroup = "condition_group"
	ParamCron           = "cron"
	ParamDateTime       = "datetime"
	ParamFields         = "fields"
	ParamCases          = "cases"
	ParamTags           = "tags"
)

// Option is one choice of a select-like parameter.
type Option struct {
	Value    string `json:"value"`
	LabelKey string `json:"label_key,omitempty"`
	Label    string `json:"label,omitempty"`
}

// Visibility shows a parameter only when another parameter has one of the given values.
type Visibility struct {
	Param  string   `json:"param"`
	Equals []string `json:"equals"`
}

// ParamSpec describes one node parameter.
type ParamSpec struct {
	Name          string      `json:"name"`
	Kind          string      `json:"kind"`
	LabelKey      string      `json:"label_key,omitempty"`
	HelpKey       string      `json:"help_key,omitempty"`
	Label         string      `json:"label,omitempty"`
	Help          string      `json:"help,omitempty"`
	Required      bool        `json:"required,omitempty"`
	Templatable   bool        `json:"templatable,omitempty"`
	Default       any         `json:"default,omitempty"`
	Options       []Option    `json:"options,omitempty"`
	OptionsSource string      `json:"options_source,omitempty"`
	VisibleIf     *Visibility `json:"visible_if,omitempty"`
	SensitiveSink bool        `json:"sensitive_sink,omitempty"`
	Fields        []ParamSpec `json:"fields,omitempty"`
}

// FieldSpec describes one declared output field.
type FieldSpec struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	DescriptionKey string `json:"description_key,omitempty"`
	Primary        bool   `json:"primary,omitempty"`
}

// ExecuteFunc runs a node.
type ExecuteFunc func(ctx context.Context, in ExecInput) (ExecResult, error)

// NodeInput is the output of an upstream node delivered on an incoming edge.
type NodeInput struct {
	NodeID string
	Key    string
	Port   string
	Output map[string]any
}

// RunInfo identifies the run a node executes in.
type RunInfo struct {
	ID        string
	FlowID    string
	Mode      RunMode
	Revision  int
	StartedAt time.Time
}

// ExecInput is everything a node receives.
//
// Read-only contract: Params and Inputs (and any list or object inside them)
// can share memory with the run's data, because a single-expression template
// resolves to the referenced value itself, not a copy. A node's Execute must
// not modify them in place; clone before changing anything.
type ExecInput struct {
	Flow     *Flow
	Node     *Node
	Params   map[string]any
	Inputs   []NodeInput
	Env      *Env
	Run      RunInfo
	Services *Services
	Logger   *slog.Logger
}

// ExecResult is a node's output. Ports nil means the definition's default port.
type ExecResult struct {
	Output    map[string]any
	Ports     []string
	ItemCount int
	Stop      *StopSignal
}

// StopSignal ends the whole run (logic.stop).
type StopSignal struct {
	Status  RunStatus
	Message string
}

// NodeError is a node failure with a stable code.
type NodeError struct {
	Code    string
	Message string
}

func (e *NodeError) Error() string { return e.Code + ": " + e.Message }

// NewNodeError builds a NodeError with a formatted message.
func NewNodeError(code, format string, args ...any) *NodeError {
	return &NodeError{Code: code, Message: fmt.Sprintf(format, args...)}
}

func asNodeError(err error) *NodeError {
	var ne *NodeError
	if errors.As(err, &ne) {
		return ne
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &NodeError{Code: "FLOW_NODE_TIMEOUT", Message: "the node took too long"}
	}
	return &NodeError{Code: "FLOW_NODE_FAILED", Message: err.Error()}
}

// NodeDef defines a node type. UI metadata (keys, icon, color) is served by plan 1b's catalog API.
type NodeDef struct {
	Type             string
	Version          int
	Category         string
	Icon             string
	Color            string
	LabelKey         string
	DescriptionKey   string
	SummaryKey       string
	Label            string
	Description      string
	Trigger          bool
	Inputs           []string
	Outputs          []string
	OutputsFunc      func(n *Node) []string
	Params           []ParamSpec
	OutputFields     []FieldSpec
	OutputFieldsFunc func(n *Node) []FieldSpec
	PrimaryInput     string
	Tool             string
	Effects          []Effect
	EffectsFunc      func(n *Node) []Effect
	UntrustedOutput  bool
	DefaultTimeout   time.Duration
	AvailabilityFunc func() Availability
	Validate         func(n *Node, vc ValidateContext) []Issue
	Execute          ExecuteFunc
}

// InputPorts returns the input ports (none for triggers).
func (d *NodeDef) InputPorts() []string {
	if d.Trigger {
		return nil
	}
	if d.Inputs != nil {
		return d.Inputs
	}
	return []string{PortIn}
}

// OutputPorts returns the output ports of node n, including "error" when the node uses an error port.
// The returned slice is always a fresh copy, so callers may append to it.
func (d *NodeDef) OutputPorts(n *Node) []string {
	var ports []string
	switch {
	case d.OutputsFunc != nil && n != nil:
		ports = append([]string(nil), d.OutputsFunc(n)...)
	case d.Outputs != nil:
		ports = append([]string(nil), d.Outputs...)
	default:
		ports = []string{PortOut}
	}
	if n != nil && n.Settings.OnError == ErrorPort {
		ports = append(ports, PortError)
	}
	return ports
}

// DefaultPort returns the first non-error output port or "".
func (d *NodeDef) DefaultPort(n *Node) string {
	for _, p := range d.OutputPorts(n) {
		if p != PortError {
			return p
		}
	}
	return ""
}

// Availability returns the current availability (available when no func is set).
func (d *NodeDef) Availability() Availability {
	if d.AvailabilityFunc == nil {
		return Availability{State: AvailableState}
	}
	return d.AvailabilityFunc()
}

// FieldsOf returns the declared output fields of node n (dynamic when OutputFieldsFunc is set).
func (d *NodeDef) FieldsOf(n *Node) []FieldSpec {
	if d.OutputFieldsFunc != nil && n != nil {
		return d.OutputFieldsFunc(n)
	}
	return d.OutputFields
}

// EffectsOf returns the effects of node n.
func (d *NodeDef) EffectsOf(n *Node) []Effect {
	if d.EffectsFunc != nil {
		return d.EffectsFunc(n)
	}
	return d.Effects
}

// Timeout returns the per-attempt timeout for node n.
func (d *NodeDef) Timeout(n *Node) time.Duration {
	if n != nil && n.Settings.TimeoutSeconds > 0 {
		return time.Duration(n.Settings.TimeoutSeconds) * time.Second
	}
	if d.DefaultTimeout > 0 {
		return d.DefaultTimeout
	}
	return DefaultNodeTimeout
}

// Registry holds the node definitions. It is safe for concurrent use; the
// registered definitions themselves must be treated as read-only.
type Registry struct {
	mu   sync.RWMutex
	defs map[string]*NodeDef
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{defs: map[string]*NodeDef{}}
}

// Register adds a definition; duplicate or empty types fail.
func (r *Registry) Register(def *NodeDef) error {
	if def == nil || def.Type == "" {
		return errors.New("node definition needs a type")
	}
	if def.Version <= 0 {
		def.Version = 1
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.defs[def.Type]; exists {
		return fmt.Errorf("node type %q is already registered", def.Type)
	}
	r.defs[def.Type] = def
	return nil
}

// MustRegister is Register that panics; use it only during start-up wiring.
func (r *Registry) MustRegister(def *NodeDef) {
	if err := r.Register(def); err != nil {
		panic(err)
	}
}

// Replace adds or overwrites a definition.
func (r *Registry) Replace(def *NodeDef) {
	if def.Version <= 0 {
		def.Version = 1
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defs[def.Type] = def
}

// RemoveWhere deletes all definitions matching pred and returns how many were removed.
// pred runs while the registry is locked and must not call back into the registry.
func (r *Registry) RemoveWhere(pred func(*NodeDef) bool) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	removed := 0
	for typ, def := range r.defs {
		if pred(def) {
			delete(r.defs, typ)
			removed++
		}
	}
	return removed
}

// Lookup returns the definition of typ.
func (r *Registry) Lookup(typ string) (*NodeDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.defs[typ]
	return def, ok
}

// All returns all definitions sorted by type.
func (r *Registry) All() []*NodeDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*NodeDef, 0, len(r.defs))
	for _, def := range r.defs {
		out = append(out, def)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}
