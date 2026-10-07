package invasion

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

// eggIDPrefix returns the first eight characters of a nest ID. Egg service
// names, SSH directories, Docker container names and Docker volume names all
// use it, so it must stay identical for existing nests: nest IDs are UUIDs and
// their prefix is the first eight hex digits. Shorter IDs and characters
// outside [A-Za-z0-9-] are rejected instead of panicking or reaching a shell
// path or a Docker name.
func eggIDPrefix(nestID string) (string, error) {
	id := strings.TrimSpace(nestID)
	if len(id) < 8 {
		return "", fmt.Errorf("invalid nest ID %q: expected at least 8 safe characters", nestID)
	}
	prefix := id[:8]
	for _, r := range prefix {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return "", fmt.Errorf("invalid nest ID %q: unsafe character %q in egg name prefix", nestID, r)
	}
	return prefix, nil
}

// ErrEggConfigNotDelivered marks a Deploy failure that happened before any
// request carrying the new egg configuration, and with it the hatch's new
// shared key, left the master. No egg on the nest can hold the new key, and
// the egg that ran before the hatch still has its own configuration. Test it
// with errors.Is; a marked error keeps its text and chain.
var ErrEggConfigNotDelivered = errors.New("egg configuration not delivered")

// configNotDeliveredError adds ErrEggConfigNotDelivered to an error's chain
// without changing its text.
type configNotDeliveredError struct{ err error }

func (e *configNotDeliveredError) Error() string        { return e.err.Error() }
func (e *configNotDeliveredError) Unwrap() error        { return e.err }
func (e *configNotDeliveredError) Is(target error) bool { return target == ErrEggConfigNotDelivered }

// configNotDelivered marks err with ErrEggConfigNotDelivered; nil stays nil.
func configNotDelivered(err error) error {
	if err == nil {
		return nil
	}
	return &configNotDeliveredError{err: err}
}
