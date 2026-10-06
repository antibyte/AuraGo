package tools

import (
	"context"
	"strings"
	"sync/atomic"
)

// DockerSelfIdentity is what Docker proves about the container AuraGo runs in.
// The zero value means "not in a container, or not proven": callers then apply
// no self-specific rule.
type DockerSelfIdentity struct {
	// ComposeProject is the container's com.docker.compose.project label.
	ComposeProject string
	// StateVolumes are the named volumes mounted at or below AuraGo's data
	// directory (vault, databases, master key).
	StateVolumes []string
	// StateBindSources are the host directories bound at or below it.
	StateBindSources []string
}

type dockerSelfIdentityResolverState struct {
	resolve func(context.Context, DockerConfig) DockerSelfIdentity
}

var dockerSelfIdentityResolver atomic.Pointer[dockerSelfIdentityResolverState]

// SetDockerSelfIdentityResolver binds the server's self detection. Passing nil
// removes it; DockerSelfIdentityFor then returns the zero identity.
func SetDockerSelfIdentityResolver(resolve func(context.Context, DockerConfig) DockerSelfIdentity) {
	if resolve == nil {
		dockerSelfIdentityResolver.Store(nil)
		return
	}
	dockerSelfIdentityResolver.Store(&dockerSelfIdentityResolverState{resolve: resolve})
}

// DockerSelfIdentityFor returns the identity of AuraGo's own container on the
// engine of cfg, or the zero identity when no resolver is bound.
func DockerSelfIdentityFor(ctx context.Context, cfg DockerConfig) DockerSelfIdentity {
	state := dockerSelfIdentityResolver.Load()
	if state == nil {
		return DockerSelfIdentity{}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return state.resolve(ctx, cfg)
}

// OwnsComposeProject reports whether a resolved Compose project name is the
// project AuraGo's own container belongs to (Compose lower-cases names).
func (id DockerSelfIdentity) OwnsComposeProject(project string) bool {
	own := strings.TrimSpace(id.ComposeProject)
	return own != "" && strings.EqualFold(own, strings.TrimSpace(project))
}
