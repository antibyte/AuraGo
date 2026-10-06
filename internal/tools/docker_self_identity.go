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
	// ComposeProject is the container's com.docker.compose.project label. When
	// AuraGo shares another container's network namespace it is set only when
	// every container of that namespace carries the same project.
	ComposeProject string
	// Proven reports that Docker named exactly AuraGo's own container: the
	// self signals name a container nobody else joins. Only then are the
	// mounts below known; otherwise IsAuraGoStateVolume falls back to the
	// shipped volume names.
	Proven bool
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

// auraGoDataVolumeKey is the data volume of the shipped docker-compose.yml;
// Compose names it <project>_aurago_data.
const auraGoDataVolumeKey = "aurago_data"

// IsAuraGoStateVolume reports a named volume that holds AuraGo's data
// directory. Native installs (inContainer false) keep it in a host directory,
// so no volume is. In a container it is one of self.StateVolumes when Docker
// proved AuraGo's own container (self.Proven), otherwise the shipped name
// (aurago_data, <project>_aurago_data). The workdir volume is the agent
// workspace itself and never matches.
func IsAuraGoStateVolume(name string, inContainer bool, self DockerSelfIdentity) bool {
	name = strings.TrimSpace(name)
	if !inContainer || name == "" {
		return false
	}
	if self.Proven {
		for _, volume := range self.StateVolumes {
			if strings.EqualFold(strings.TrimSpace(volume), name) {
				return true
			}
		}
		return false
	}
	lower := strings.ToLower(name)
	return lower == auraGoDataVolumeKey || strings.HasSuffix(lower, "_"+auraGoDataVolumeKey)
}

// DockerBindTouchesAuraGoState reports a create/run volume string that mounts
// AuraGo's data volume, or a host directory bound at AuraGo's data directory.
func DockerBindTouchesAuraGoState(bind string, inContainer bool, self DockerSelfIdentity) bool {
	if !inContainer {
		return false
	}
	spec, ok := parseDockerBindMount(bind)
	if !ok {
		return false
	}
	if !spec.isHostPath {
		return IsAuraGoStateVolume(spec.hostPath, inContainer, self)
	}
	for _, root := range self.StateBindSources {
		if dockerPathEqualOrWithin(spec.hostPath, root) {
			return true
		}
	}
	return false
}

// DockerComposeModelVolumeNames lists the named volumes a resolved model uses
// or declares: each service mount of type volume (its source and the name of
// the top-level volume it refers to) and each top-level volume (key and name).
func DockerComposeModelVolumeNames(model DockerComposeModel) []string {
	var names []string
	for _, service := range SortedDockerComposeKeys(model.Services) {
		for _, mount := range model.Services[service].Volumes {
			source := strings.TrimSpace(mount.Source)
			if !strings.EqualFold(strings.TrimSpace(mount.Type), "volume") || source == "" {
				continue
			}
			names = append(names, source)
			if volume, ok := model.Volumes[source]; ok && strings.TrimSpace(volume.Name) != "" {
				names = append(names, volume.Name)
			}
		}
	}
	for _, key := range SortedDockerComposeKeys(model.Volumes) {
		names = append(names, key)
		if name := strings.TrimSpace(model.Volumes[key].Name); name != "" {
			names = append(names, name)
		}
	}
	return names
}
