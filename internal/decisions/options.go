package decisions

import (
	"context"
	"log/slog"
	"net/http"
)

// RuntimeAccess is a live snapshot of Docker policy from the owning server.
// IsDocker describes the AuraGo host context; the other fields are the current
// Docker grant and engine endpoint. A Stop remains permitted when Docker is
// disabled or readonly so an already managed runtime can be contained.
type RuntimeAccess struct {
	IsDocker       bool
	DockerEnabled  bool
	DockerReadOnly bool
	DockerHost     string
}

// ManagerOption configures trusted process dependencies for the managed service.
type ManagerOption func(*managerOptions)

type managerOptions struct {
	logger              *slog.Logger
	dockerHost          string
	runtimeAccess       func() RuntimeAccess
	dockerSelfContainer func(context.Context, string) (string, error)
	runtimeDirectory    string
	httpClient          *http.Client
	runtimeFactory      func() runtimeFactory
}

// WithLogger supplies the application's structured logger.
func WithLogger(logger *slog.Logger) ManagerOption {
	return func(options *managerOptions) {
		if logger != nil {
			options.logger = logger
		}
	}
}

// WithDockerHost supplies a fixed Engine host for standalone manager users.
// It does not enable Docker; callers still need a RuntimeAccess grant.
func WithDockerHost(host string) ManagerOption {
	return func(options *managerOptions) {
		options.dockerHost = host
	}
}

// WithRuntimeAccess supplies the current container state and Docker policy.
// The function is reevaluated for every mutating Docker operation.
func WithRuntimeAccess(access func() RuntimeAccess) ManagerOption {
	return func(options *managerOptions) {
		options.runtimeAccess = access
	}
}

// WithDockerSelfContainer supplies verified Docker container identity for
// private sidecar network and mount setup. No hostname fallback is attempted.
func WithDockerSelfContainer(resolve func(context.Context, string) (string, error)) ManagerOption {
	return func(options *managerOptions) {
		options.dockerSelfContainer = resolve
	}
}

// WithRuntimeDirectory selects a trusted local runtime build directory. The
// server supplies the executable directory; a standard install's trailing
// bin directory is normalized to its application root. This is never
// user-configurable.
func WithRuntimeDirectory(directory string) ManagerOption {
	return func(options *managerOptions) {
		options.runtimeDirectory = directory
	}
}

func (options managerOptions) access() RuntimeAccess {
	var access RuntimeAccess
	if options.runtimeAccess != nil {
		access = options.runtimeAccess()
	}
	if access.DockerHost == "" {
		access.DockerHost = options.dockerHost
	}
	return access
}
