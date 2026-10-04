// Package flows implements EasyDrag flows: the flow document model, templates,
// conditions, validation, the execution engine, the run scheduler, persistence
// and date/time timers. It must not import internal/agent or internal/server;
// integrations reach it through the interfaces in services.go.
package flows

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// SchemaVersion is the flow document schema this package reads and writes.
const SchemaVersion = 1

// Document limits.
const (
	MaxNodes = 500
	// MaxEdges caps the connections in one flow. MaxDocumentBytes alone still
	// allows tens of thousands of nodes and edges, so Validate rejects a flow over
	// MaxNodes or MaxEdges before it does any per-node, per-edge or graph work.
	MaxEdges         = 2000
	MaxDocumentBytes = 2 << 20
)

// Defaults applied by Normalize.
const (
	DefaultMaxRunSeconds  = 1800
	DefaultNotifyOnError  = "desktop"
	MaxRetryCount         = 5
	MaxRetryDelaySeconds  = 3600  // upper bound for RetryPolicy.DelaySeconds
	MaxNodeTimeoutSeconds = 86400 // upper bound for NodeSettings.TimeoutSeconds
	MaxRunSecondsLimit    = 86400 // upper bound for FlowSettings.MaxRunSeconds
)

// Kind distinguishes normal flows (which own a mission) from building blocks.
type Kind string

const (
	KindFlow  Kind = "flow"
	KindBlock Kind = "block"
)

// Port names shared by node definitions.
const (
	PortIn      = "in"
	PortOut     = "out"
	PortTrue    = "true"
	PortFalse   = "false"
	PortDefault = "default"
	PortError   = "error"
)

// ErrorMode decides what happens when a node fails after all retries.
type ErrorMode string

const (
	ErrorStop     ErrorMode = "stop"
	ErrorContinue ErrorMode = "continue"
	ErrorPort     ErrorMode = "error_port"
)

// ConcurrencyPolicy decides what happens when a flow is triggered while a live run is active.
type ConcurrencyPolicy string

const (
	ConcurrencyQueue    ConcurrencyPolicy = "queue"
	ConcurrencyParallel ConcurrencyPolicy = "parallel"
	ConcurrencySkip     ConcurrencyPolicy = "skip"
)

var (
	// ErrDocumentTooLarge is returned by ParseFlow when the document exceeds MaxDocumentBytes.
	ErrDocumentTooLarge = errors.New("flow document exceeds the size limit")
	// ErrUnsupportedSchema is returned (wrapped) by ParseFlow when the schema version is not SchemaVersion.
	ErrUnsupportedSchema = errors.New("unsupported flow schema version")
)

// Flow is the EasyDrag flow document.
type Flow struct {
	Schema      int          `json:"schema"`
	ID          string       `json:"id"`
	Kind        Kind         `json:"kind"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Nodes       []Node       `json:"nodes"`
	Edges       []Edge       `json:"edges"`
	Settings    FlowSettings `json:"settings"`
	Notes       []StickyNote `json:"notes,omitempty"`
	Viewport    *Viewport    `json:"viewport,omitempty"`
}

// Node is one placed node on the canvas.
type Node struct {
	ID          string         `json:"id"`
	Key         string         `json:"key"`
	Type        string         `json:"type"`
	TypeVersion int            `json:"type_version"`
	Label       string         `json:"label"`
	Position    Point          `json:"position"`
	Params      map[string]any `json:"params"`
	Settings    NodeSettings   `json:"settings"`
	Parent      string         `json:"parent,omitempty"`
}

// Point is a canvas position.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// NodeSettings holds per-node execution settings.
type NodeSettings struct {
	Disabled       bool        `json:"disabled,omitempty"`
	Notes          string      `json:"notes,omitempty"`
	OnError        ErrorMode   `json:"on_error,omitempty"`
	Retry          RetryPolicy `json:"retry"`
	TimeoutSeconds int         `json:"timeout_seconds,omitempty"`
}

// RetryPolicy configures retries after a failed attempt.
type RetryPolicy struct {
	Count        int `json:"count,omitempty"`
	DelaySeconds int `json:"delay_seconds,omitempty"`
}

// Edge connects an output port to an input port.
type Edge struct {
	ID     string  `json:"id"`
	Source PortRef `json:"source"`
	Target PortRef `json:"target"`
}

// PortRef addresses a port of a node.
type PortRef struct {
	Node string `json:"node"`
	Port string `json:"port"`
}

// FlowSettings holds flow-wide settings.
type FlowSettings struct {
	Concurrency   ConcurrencyPolicy `json:"concurrency,omitempty"`
	MaxRunSeconds int               `json:"max_run_seconds,omitempty"`
	NotifyOnError string            `json:"notify_on_error,omitempty"`
}

// StickyNote is a canvas note (editor feature of phase 2, kept in the schema now).
type StickyNote struct {
	ID       string  `json:"id"`
	Position Point   `json:"position"`
	Width    float64 `json:"width"`
	Height   float64 `json:"height"`
	Text     string  `json:"text"`
	Color    string  `json:"color,omitempty"`
}

// Viewport is the last canvas viewport of the draft.
type Viewport struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Zoom float64 `json:"zoom"`
}

// ParseFlow decodes a flow document, checks size and schema and applies defaults.
// Numbers inside node params decode as float64, as with any JSON decoded into any.
func ParseFlow(data []byte) (*Flow, error) {
	if len(data) > MaxDocumentBytes {
		return nil, ErrDocumentTooLarge
	}
	var f Flow
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("decode flow: %w", err)
	}
	if f.Schema != SchemaVersion {
		return nil, fmt.Errorf("%w: %d", ErrUnsupportedSchema, f.Schema)
	}
	f.Normalize()
	return &f, nil
}

// Normalize fills defaults, clamps numeric settings to their bounds and coerces
// unknown enum values to their defaults. It is idempotent.
func (f *Flow) Normalize() {
	if f.Kind != KindFlow && f.Kind != KindBlock {
		f.Kind = KindFlow
	}
	if f.Nodes == nil {
		f.Nodes = []Node{}
	}
	if f.Edges == nil {
		f.Edges = []Edge{}
	}
	switch f.Settings.Concurrency {
	case ConcurrencyQueue, ConcurrencyParallel, ConcurrencySkip:
	default:
		f.Settings.Concurrency = ConcurrencyQueue
	}
	if f.Settings.MaxRunSeconds <= 0 {
		f.Settings.MaxRunSeconds = DefaultMaxRunSeconds
	}
	if f.Settings.MaxRunSeconds > MaxRunSecondsLimit {
		f.Settings.MaxRunSeconds = MaxRunSecondsLimit
	}
	if f.Settings.NotifyOnError == "" {
		f.Settings.NotifyOnError = DefaultNotifyOnError
	}
	for i := range f.Nodes {
		n := &f.Nodes[i]
		if n.TypeVersion <= 0 {
			n.TypeVersion = 1
		}
		if n.Params == nil {
			n.Params = map[string]any{}
		}
		switch n.Settings.OnError {
		case ErrorStop, ErrorContinue, ErrorPort:
		default:
			n.Settings.OnError = ErrorStop
		}
		if n.Settings.Retry.Count < 0 {
			n.Settings.Retry.Count = 0
		}
		if n.Settings.Retry.Count > MaxRetryCount {
			n.Settings.Retry.Count = MaxRetryCount
		}
		if n.Settings.Retry.DelaySeconds < 0 {
			n.Settings.Retry.DelaySeconds = 0
		}
		if n.Settings.Retry.DelaySeconds > MaxRetryDelaySeconds {
			n.Settings.Retry.DelaySeconds = MaxRetryDelaySeconds
		}
		if n.Settings.TimeoutSeconds < 0 {
			n.Settings.TimeoutSeconds = 0
		}
		if n.Settings.TimeoutSeconds > MaxNodeTimeoutSeconds {
			n.Settings.TimeoutSeconds = MaxNodeTimeoutSeconds
		}
	}
}

// Marshal encodes the flow as JSON.
func (f *Flow) Marshal() ([]byte, error) {
	return json.Marshal(f)
}

// Clone returns a deep copy of the flow. It copies through JSON, so numbers
// inside node params come back as float64.
func (f *Flow) Clone() (*Flow, error) {
	data, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	var out Flow
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// NodeByID returns the node with the given id or nil.
func (f *Flow) NodeByID(id string) *Node {
	for i := range f.Nodes {
		if f.Nodes[i].ID == id {
			return &f.Nodes[i]
		}
	}
	return nil
}

// NodeByKey returns the node with the given key or nil.
func (f *Flow) NodeByKey(key string) *Node {
	for i := range f.Nodes {
		if f.Nodes[i].Key == key {
			return &f.Nodes[i]
		}
	}
	return nil
}

var (
	keyPattern    = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)
	nodeIDPattern = regexp.MustCompile(`^n_[a-z2-7]{8}$`)
	reservedKeys  = map[string]bool{
		"trigger": true, "run": true, "flow": true, "item": true, "index": true,
		"input": true, "env": true, "vars": true, "secrets": true,
	}
)

// ValidKey reports whether key is a syntactically valid, non-reserved node key.
func ValidKey(key string) bool {
	return keyPattern.MatchString(key) && !reservedKeys[key]
}

// IsReservedKey reports whether key is a reserved template root.
func IsReservedKey(key string) bool {
	return reservedKeys[key]
}

// ValidNodeID reports whether id has the node id format n_xxxxxxxx.
func ValidNodeID(id string) bool {
	return nodeIDPattern.MatchString(id)
}

const idAlphabet = "abcdefghijklmnopqrstuvwxyz234567"

func randomSuffix(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf) // crypto/rand.Read never fails since Go 1.24
	out := make([]byte, n)
	for i, b := range buf {
		out[i] = idAlphabet[int(b)%len(idAlphabet)]
	}
	return string(out)
}

// NewNodeID returns a random node id.
func NewNodeID() string { return "n_" + randomSuffix(8) }

// NewFlowID returns a random flow id.
func NewFlowID() string { return "flow_" + randomSuffix(10) }

// NewEdgeID returns a random edge id.
func NewEdgeID() string { return "e_" + randomSuffix(8) }

// NewRunID returns a random run id.
func NewRunID() string { return "run_" + randomSuffix(12) }

var umlautReplacer = strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss", "Ä", "ae", "Ö", "oe", "Ü", "ue")

// KeyFromLabel derives a unique, valid node key from a display label. The
// result is not added to taken; the caller records it. It appends a numeric
// suffix on collisions and assumes fewer than 1000 of them, which holds with
// MaxNodes = 500 (the suffix keeps the key within the 40 character limit).
func KeyFromLabel(label string, taken map[string]bool) string {
	s := strings.ToLower(umlautReplacer.Replace(strings.TrimSpace(label)))
	var b strings.Builder
	lastUnderscore := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore && b.Len() > 0 {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	key := strings.Trim(b.String(), "_")
	if key == "" {
		key = "node"
	}
	if key[0] >= '0' && key[0] <= '9' {
		key = "n_" + key
	}
	if len(key) > 36 {
		key = strings.TrimRight(key[:36], "_")
	}
	if reservedKeys[key] {
		key += "_node"
	}
	candidate := key
	for i := 2; taken[candidate]; i++ {
		candidate = key + "_" + strconv.Itoa(i)
	}
	return candidate
}
