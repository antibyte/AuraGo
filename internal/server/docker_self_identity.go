package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	pathpkg "path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aurago/internal/tools"
)

const (
	dockerComposeProjectLabel    = "com.docker.compose.project"
	dockerSelfIdentityTimeout    = 5 * time.Second
	dockerSelfIdentityRetryAfter = 30 * time.Second
)

// dockerSelfIdentityCache keeps what Docker proved about the container AuraGo
// runs in. Labels and mounts cannot change while the container lives, so a
// successful answer is kept for the process; a failed one is retried after
// dockerSelfIdentityRetryAfter.
type dockerSelfIdentityCache struct {
	mu       sync.Mutex
	host     string
	resolved bool
	value    tools.DockerSelfIdentity
	retryAt  time.Time
}

// bindDockerSelfIdentity lets the agent Docker policy learn AuraGo's own
// Compose project and the mounts of its data directory from the container
// K12's self detection names (readContainerSelfSignals). Native runtimes
// answer with the zero identity and never call Docker.
func (s *Server) bindDockerSelfIdentity() {
	cache := &dockerSelfIdentityCache{}
	tools.SetDockerSelfIdentityResolver(func(ctx context.Context, cfg tools.DockerConfig) tools.DockerSelfIdentity {
		return cache.get(ctx, s, cfg)
	})
}

func (c *dockerSelfIdentityCache) get(ctx context.Context, s *Server, cfg tools.DockerConfig) tools.DockerSelfIdentity {
	if !containerRuntimeIsDocker(s) {
		return tools.DockerSelfIdentity{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.host == cfg.Host && (c.resolved || time.Now().Before(c.retryAt)) {
		return c.value
	}
	c.host, c.resolved, c.value = cfg.Host, false, tools.DockerSelfIdentity{}
	identity, err := resolveDockerSelfIdentity(ctx, cfg, readContainerSelfSignals(true), containerDataDir(s))
	if err != nil {
		c.retryAt = time.Now().Add(dockerSelfIdentityRetryAfter)
		if s.Logger != nil {
			s.Logger.Debug("[Docker] AuraGo's own container could not be identified", "error", err)
		}
		return c.value
	}
	c.resolved, c.value = true, identity
	return identity
}

// containerDataDir is AuraGo's data directory as the container sees it, in
// slash notation, or "" when none is configured.
func containerDataDir(s *Server) string {
	s.CfgMu.RLock()
	dir := ""
	if s.Cfg != nil {
		dir = strings.TrimSpace(s.Cfg.Directories.DataDir)
	}
	s.CfgMu.RUnlock()
	if dir == "" {
		return ""
	}
	slashed := filepath.ToSlash(dir)
	if !pathpkg.IsAbs(slashed) {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return ""
		}
		slashed = filepath.ToSlash(abs)
	}
	return pathpkg.Clean(slashed)
}

// resolveDockerSelfIdentity inspects the container the self signals name (the
// /proc container ID, else the default hostname, which Docker accepts as an ID
// prefix) and lists all containers to apply K12's containerSelfInList. With a
// network-namespace sidecar (network_mode container:/service:, e.g. Gluetun or
// Tailscale) the signals name the provider, so the identity is not proven: it
// keeps the project only when the provider and every container joining it
// carry the same project label, and no mounts (callers fall back to the
// shipped volume names). A failed list counts as a failed inspect.
func resolveDockerSelfIdentity(ctx context.Context, cfg tools.DockerConfig, signals containerSelfSignals, dataDir string) (tools.DockerSelfIdentity, error) {
	id := signals.ownID
	if id == "" {
		id = signals.hostname
	}
	if id == "" {
		return tools.DockerSelfIdentity{}, errors.New("neither /proc nor the hostname names the container")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, dockerSelfIdentityTimeout)
	defer cancel()
	data, code, err := tools.DockerRequestContext(ctx, cfg, http.MethodGet, "/containers/"+url.PathEscape(id)+"/json", "")
	if err != nil {
		return tools.DockerSelfIdentity{}, fmt.Errorf("inspect container %s: %w", id, err)
	}
	if code != http.StatusOK {
		return tools.DockerSelfIdentity{}, fmt.Errorf("inspect container %s: HTTP %d", id, code)
	}
	var info struct {
		ID     string `json:"Id"`
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
		Mounts []struct {
			Type        string `json:"Type"`
			Name        string `json:"Name"`
			Source      string `json:"Source"`
			Destination string `json:"Destination"`
		} `json:"Mounts"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return tools.DockerSelfIdentity{}, fmt.Errorf("decode container %s: %w", id, err)
	}
	project := strings.TrimSpace(info.Config.Labels[dockerComposeProjectLabel])
	entries, failure := tools.DockerListContainerEntries(cfg, true)
	if failure != "" {
		return tools.DockerSelfIdentity{}, fmt.Errorf("list containers: %s", failure)
	}
	fullID := strings.ToLower(strings.TrimSpace(info.ID))
	self, shared := containerSelfInList(entries, signals)
	switch {
	case fullID != "" && self[fullID]:
	case fullID != "" && shared[fullID]:
		for _, entry := range entries {
			if shared[strings.ToLower(entry.FullID)] && strings.TrimSpace(entry.Labels[dockerComposeProjectLabel]) != project {
				return tools.DockerSelfIdentity{}, nil
			}
		}
		return tools.DockerSelfIdentity{ComposeProject: project}, nil
	default:
		return tools.DockerSelfIdentity{}, fmt.Errorf("container %s is not in the container list", id)
	}
	identity := tools.DockerSelfIdentity{ComposeProject: project, Proven: true}
	if dataDir == "" {
		return identity, nil
	}
	prefix := strings.TrimSuffix(dataDir, "/") + "/"
	for _, mount := range info.Mounts {
		destination := pathpkg.Clean(strings.TrimSpace(mount.Destination))
		if destination != dataDir && !strings.HasPrefix(destination, prefix) {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(mount.Type)) {
		case "volume":
			if name := strings.TrimSpace(mount.Name); name != "" {
				identity.StateVolumes = append(identity.StateVolumes, name)
			}
			if source := strings.TrimSpace(mount.Source); source != "" {
				identity.StateHostPaths = append(identity.StateHostPaths, source)
			}
		case "bind":
			if source := strings.TrimSpace(mount.Source); source != "" {
				identity.StateBindSources = append(identity.StateBindSources, source)
			}
		}
	}
	return identity, nil
}
