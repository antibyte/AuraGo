package networkshares

import (
	"context"
	"time"
)

const (
	ProtocolSMB = "smb"
	ProtocolNFS = "nfs"
)

// Options contains persisted policy plus runtime-only host constraints.
type Options struct {
	Enabled              bool
	ReadOnly             bool
	AllowCreate          bool
	AllowUpdate          bool
	AllowDelete          bool
	AllowedRoots         []string
	SMBEnabled           bool
	SMBAllowGuest        bool
	SMBAllowedPrincipals []string
	NFSEnabled           bool
	NFSAllowedClients    []string
	IsDocker             bool
	SudoEnabled          bool
	SudoUnrestricted     bool
	NoNewPrivileges      bool
	ProtectSystemStrict  bool
	SudoPassword         string
}

// ProtocolStatus describes one host sharing backend without mutating it.
type ProtocolStatus struct {
	Supported     bool      `json:"supported"`
	Installed     bool      `json:"installed"`
	Configured    bool      `json:"configured"`
	ServiceActive bool      `json:"service_active"`
	Readable      bool      `json:"readable"`
	Writable      bool      `json:"writable"`
	Backend       string    `json:"backend,omitempty"`
	Version       string    `json:"version,omitempty"`
	Reason        string    `json:"reason,omitempty"`
	LastProbedAt  time.Time `json:"last_probed_at"`
}

// RootStatus reports whether an allowed root can currently be used.
type RootStatus struct {
	Path      string `json:"path"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

// Status is the passive runtime capability snapshot.
type Status struct {
	Supported    bool           `json:"supported"`
	Usable       bool           `json:"usable"`
	Reason       string         `json:"reason,omitempty"`
	SMB          ProtocolStatus `json:"smb"`
	NFS          ProtocolStatus `json:"nfs"`
	AllowedRoots []RootStatus   `json:"allowed_roots"`
	LastProbedAt time.Time      `json:"last_probed_at"`
}

// ACLEntry grants or denies SMB share-level access to an existing OS principal.
type ACLEntry struct {
	Principal string `json:"principal"`
	Level     string `json:"level"`
}

// ShareAccess contains protocol-specific access settings.
type ShareAccess struct {
	Guest   bool       `json:"guest,omitempty"`
	ACL     []ACLEntry `json:"acl,omitempty"`
	Clients []string   `json:"clients,omitempty"`
}

// ShareSpec is the desired immutable identity and mutable access policy.
type ShareSpec struct {
	ID       string      `json:"id,omitempty"`
	Protocol string      `json:"protocol"`
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	Comment  string      `json:"comment,omitempty"`
	ReadOnly bool        `json:"read_only"`
	Access   ShareAccess `json:"access"`
}

// Share is the public reconciled read model.
type Share struct {
	ShareSpec
	Managed    bool      `json:"managed"`
	Mutable    bool      `json:"mutable"`
	Active     bool      `json:"active"`
	Drift      string    `json:"drift,omitempty"`
	ObservedAt time.Time `json:"observed_at"`
}

// SharePatch deliberately excludes name, path, and protocol.
type SharePatch struct {
	Comment  *string      `json:"comment,omitempty"`
	ReadOnly *bool        `json:"read_only,omitempty"`
	Access   *ShareAccess `json:"access,omitempty"`
}

type observedShare struct {
	ShareSpec
	MarkerID        string
	Active          bool
	CommentObserved bool
}

type platformAdapter interface {
	Probe(ctx context.Context, options Options) (Status, error)
	Validate(ctx context.Context, options Options, share ShareSpec) error
	List(ctx context.Context, options Options) ([]observedShare, error)
	Create(ctx context.Context, options Options, share ShareSpec) error
	Update(ctx context.Context, options Options, previous, desired ShareSpec) error
	Delete(ctx context.Context, options Options, share ShareSpec) error
}
