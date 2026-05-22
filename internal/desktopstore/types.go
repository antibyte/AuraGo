// Package desktopstore manages allowlisted Docker web applications for the
// AuraGo virtual desktop.
package desktopstore

import (
	"context"
	"time"

	"aurago/internal/desktop"
)

const (
	RuntimeContainerWebApp = "container-web-app"

	BindModeLocal = "local"
	BindModeLAN   = "lan"

	AppStatusInstalling = "installing"
	AppStatusRunning    = "running"
	AppStatusStopped    = "stopped"
	AppStatusUpdating   = "updating"
	AppStatusError      = "error"

	OperationInstall   = "install"
	OperationUpdate    = "update"
	OperationStart     = "start"
	OperationStop      = "stop"
	OperationRestart   = "restart"
	OperationUninstall = "uninstall"

	OperationPending   = "pending"
	OperationRunning   = "running"
	OperationSucceeded = "succeeded"
	OperationFailed    = "failed"

	TailscaleStatusDisabled = "disabled"
	TailscaleStatusPending  = "pending"
	TailscaleStatusActive   = "active"
)

// CatalogEntry is one installable store application from the fixed allowlist.
type CatalogEntry struct {
	ID               string              `json:"id"`
	Name             string              `json:"name"`
	Description      string              `json:"description"`
	Image            string              `json:"image"`
	Icon             string              `json:"icon"`
	LogoSlug         string              `json:"logo_slug"`
	LogoURL          string              `json:"logo_url"`
	PrimaryPort      PortSpec            `json:"primary_port"`
	ExtraPorts       []PortSpec          `json:"extra_ports,omitempty"`
	Volumes          []VolumeTemplate    `json:"volumes,omitempty"`
	HostBinds        []HostBindTemplate  `json:"host_binds,omitempty"`
	Env              []string            `json:"env,omitempty"`
	ExtraHosts       []string            `json:"extra_hosts,omitempty"`
	GeneratedSecrets []GeneratedSecret   `json:"generated_secrets,omitempty"`
	Companions       []CompanionTemplate `json:"companions,omitempty"`
	SeedFiles        []SeedFile          `json:"-"`
	Metadata         map[string]string   `json:"metadata,omitempty"`
}

// PortSpec describes the container-side web UI port.
type PortSpec struct {
	ID            string `json:"id,omitempty"`
	Name          string `json:"name,omitempty"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
}

// VolumeTemplate describes a named Docker volume mounted into the app.
type VolumeTemplate struct {
	NameSuffix    string `json:"name_suffix"`
	ContainerPath string `json:"container_path"`
}

// VolumeBinding is a resolved Docker volume mount for an installed app.
type VolumeBinding struct {
	Name          string `json:"name"`
	ContainerPath string `json:"container_path"`
}

// HostBindTemplate describes a fixed allowlisted host path mounted into a Store app.
type HostBindTemplate struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"read_only,omitempty"`
}

// HostBinding is a resolved host path bind for a running Store app.
type HostBinding struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"read_only,omitempty"`
}

// GeneratedSecret describes one generated or user-supplied Store secret.
type GeneratedSecret struct {
	Key    string `json:"key"`
	Env    string `json:"env,omitempty"`
	Label  string `json:"label,omitempty"`
	Expose bool   `json:"expose,omitempty"`
}

// SecretRef records a Store secret's vault key without exposing its value.
type SecretRef struct {
	Key      string `json:"key"`
	VaultKey string `json:"-"`
	Env      string `json:"-"`
	Label    string `json:"label,omitempty"`
	Expose   bool   `json:"expose,omitempty"`
}

// CompanionTemplate describes an allowlisted sidecar container for a Store app.
type CompanionTemplate struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Image       string             `json:"image"`
	Env         []string           `json:"env,omitempty"`
	Volumes     []VolumeTemplate   `json:"volumes,omitempty"`
	HostBinds   []HostBindTemplate `json:"host_binds,omitempty"`
	NetworkMode string             `json:"network_mode,omitempty"`
}

// CompanionApp is the persisted runtime state for a companion container.
type CompanionApp struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	ContainerName string          `json:"container_name"`
	ContainerID   string          `json:"container_id,omitempty"`
	Image         string          `json:"image"`
	Status        string          `json:"status"`
	Error         string          `json:"error,omitempty"`
	NetworkMode   string          `json:"network_mode,omitempty"`
	Volumes       []VolumeBinding `json:"volumes,omitempty"`
	HostBinds     []HostBinding   `json:"host_binds,omitempty"`
	Env           []string        `json:"-"`
}

// SeedFile is copied into a newly-created container before first start.
type SeedFile struct {
	Path    string
	Content string
}

// PortBinding is a resolved Docker host binding for an installed app.
type PortBinding struct {
	ID            string `json:"id,omitempty"`
	Name          string `json:"name,omitempty"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
	HostIP        string `json:"host_ip"`
	HostPort      int    `json:"host_port"`
}

// ContainerSpec is the Docker create contract used by the store.
type ContainerSpec struct {
	Name         string            `json:"name"`
	Image        string            `json:"image"`
	Env          []string          `json:"env,omitempty"`
	PortBindings []PortBinding     `json:"port_bindings,omitempty"`
	Volumes      []VolumeBinding   `json:"volumes,omitempty"`
	HostBinds    []HostBinding     `json:"host_binds,omitempty"`
	ExtraHosts   []string          `json:"extra_hosts,omitempty"`
	NetworkMode  string            `json:"network_mode,omitempty"`
	Restart      string            `json:"restart,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
}

// ContainerState is the health/status subset returned by Docker inspect.
type ContainerState struct {
	Name    string `json:"name"`
	Running bool   `json:"running"`
	Status  string `json:"status"`
	Health  string `json:"health,omitempty"`
}

// InstalledApp is the persisted runtime state for an installed catalog app.
type InstalledApp struct {
	AppID              string          `json:"app_id"`
	DesktopAppID       string          `json:"desktop_app_id"`
	LaunchpadLinkID    string          `json:"launchpad_link_id,omitempty"`
	ContainerName      string          `json:"container_name"`
	ContainerID        string          `json:"container_id,omitempty"`
	Image              string          `json:"image"`
	Status             string          `json:"status"`
	Error              string          `json:"error,omitempty"`
	BindMode           string          `json:"bind_mode"`
	HostIP             string          `json:"host_ip"`
	HostPort           int             `json:"host_port"`
	ContainerPort      int             `json:"container_port"`
	Protocol           string          `json:"protocol"`
	TailscaleEnabled   bool            `json:"tailscale_enabled"`
	TailscaleStatus    string          `json:"tailscale_status"`
	TailscalePort      int             `json:"tailscale_port,omitempty"`
	LogoPath           string          `json:"logo_path,omitempty"`
	Ports              []PortBinding   `json:"ports,omitempty"`
	Volumes            []VolumeBinding `json:"volumes,omitempty"`
	HostBinds          []HostBinding   `json:"host_binds,omitempty"`
	SecretRefs         []SecretRef     `json:"secret_refs,omitempty"`
	Companions         []CompanionApp  `json:"companions,omitempty"`
	Env                []string        `json:"-"`
	ExtraHosts         []string        `json:"-"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
	LastOperationID    string          `json:"last_operation_id,omitempty"`
	LastOperationType  string          `json:"last_operation_type,omitempty"`
	LastOperationState string          `json:"last_operation_state,omitempty"`
}

// Operation is one background lifecycle operation.
type Operation struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"`
	AppID       string     `json:"app_id"`
	Status      string     `json:"status"`
	Message     string     `json:"message,omitempty"`
	Error       string     `json:"error,omitempty"`
	RequestJSON string     `json:"-"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// InstallRequest is the public request for installing a catalog app.
type InstallRequest struct {
	AppID            string `json:"app_id"`
	BindMode         string `json:"bind_mode"`
	TailscaleEnabled bool   `json:"tailscale_enabled"`
}

// OperationRequest carries action-specific options.
type OperationRequest struct {
	DeleteData bool `json:"delete_data,omitempty"`
}

// ExposedCredential is an admin-only response for user-visible generated credentials.
type ExposedCredential struct {
	Key   string `json:"key"`
	Label string `json:"label,omitempty"`
	Value string `json:"value"`
}

// LaunchpadLink is the small link shape needed by the store adapter.
type LaunchpadLink struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	URL         string   `json:"url"`
	Description string   `json:"description,omitempty"`
	IconPath    string   `json:"icon_path,omitempty"`
	Category    string   `json:"category,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	SortOrder   int      `json:"sort_order,omitempty"`
}

// DockerAdapter isolates Docker side effects for tests and server wiring.
type DockerAdapter interface {
	PullImage(ctx context.Context, image string) error
	CreateContainer(ctx context.Context, spec ContainerSpec) (string, error)
	CopyToContainer(ctx context.Context, containerName, destDir string, files map[string]string) error
	StartContainer(ctx context.Context, name string) error
	StopContainer(ctx context.Context, name string) error
	RestartContainer(ctx context.Context, name string) error
	RemoveContainer(ctx context.Context, name string, force bool) error
	RemoveVolume(ctx context.Context, name string, force bool) error
	InspectContainer(ctx context.Context, name string) (ContainerState, error)
}

// DesktopAdapter isolates virtual desktop mutations.
type DesktopAdapter interface {
	InstallApp(ctx context.Context, manifest desktop.AppManifest, files map[string]string, source string) error
	SetAppVisibility(ctx context.Context, id string, dockVisible, startVisible *bool, source string) error
	AddDesktopAppShortcut(ctx context.Context, appID, source string) error
	DeleteApp(ctx context.Context, appID, source string) error
}

// LaunchpadAdapter isolates Launchpad mutations.
type LaunchpadAdapter interface {
	UpsertStoreLink(ctx context.Context, link LaunchpadLink) (string, error)
	DeleteStoreLink(ctx context.Context, id string) error
}

// SecretStore is the encrypted Store secret dependency used for generated and
// user-supplied container credentials.
type SecretStore interface {
	ReadSecret(key string) (string, error)
	WriteSecret(key, value string) error
	DeleteSecret(key string) error
}

// PortAllocator chooses a host port. The preferred port is the app's default
// container port; implementations may use it or pick a free alternative.
type PortAllocator func(ctx context.Context, preferred int) (int, error)

// PortProbe checks whether a host TCP port accepts connections.
type PortProbe func(ctx context.Context, hostIP string, hostPort int) bool
