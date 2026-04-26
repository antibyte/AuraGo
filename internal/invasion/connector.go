package invasion

import (
	"context"
)

// EggDeployPayload contains all data needed to deploy an egg to a nest.
type EggDeployPayload struct {
	BinaryPath   string // path to the aurago binary for the target architecture
	ConfigYAML   []byte // generated config.yaml content
	ResourcesPkg string // path to the egg-specific resources.dat
	SharedKey    string // hex-encoded AES-256 key for master↔egg communication
	EggPort      int    // HTTP port the egg server listens on
	Permanent    bool   // install as systemd service (true) or run once (false)
	IncludeVault bool   // include encrypted vault file
	VaultData    []byte // AES-256-GCM encrypted vault (empty if IncludeVault=false)
	MasterKey    string // hex-encoded master key for the egg's own vault
}

// NestConnector abstracts the deployment mechanism for different nest types.
// Implementations exist for SSH, Docker (remote and local), and potentially
// Kubernetes/Proxmox in the future.
type NestConnector interface {
	// Validate tests connectivity to the nest. Returns nil if reachable.
	Validate(ctx context.Context, nest NestRecord, secret []byte) error

	// Deploy transfers the egg binary, config, and resources to the nest,
	// then starts the egg process. Existing deployments are backed up first
	// so they can be restored via Rollback.
	Deploy(ctx context.Context, nest NestRecord, secret []byte, payload EggDeployPayload) error

	// Stop halts the running egg on the nest.
	Stop(ctx context.Context, nest NestRecord, secret []byte) error

	// Status checks whether the egg is currently running on the nest.
	// Returns a status string: "running", "stopped", "unknown".
	Status(ctx context.Context, nest NestRecord, secret []byte) (string, error)

	// HealthCheck verifies that the deployed egg is running and responsive.
	HealthCheck(ctx context.Context, nest NestRecord, secret []byte) error

	// Rollback reverts to the previous deployment backup created during Deploy.
	Rollback(ctx context.Context, nest NestRecord, secret []byte) error

	// Reconfigure applies a safe config patch to a running egg.
	// It writes the new config YAML and restarts the egg process/container.
	// The configYAML parameter contains the fully patched config.
	Reconfigure(ctx context.Context, nest NestRecord, secret []byte, configYAML []byte) error
}
