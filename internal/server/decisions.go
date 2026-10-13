package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"aurago/internal/config"
	"aurago/internal/decisions"
	"aurago/internal/flows"
	"aurago/internal/tools"
)

type decisionsService interface {
	Status() decisions.Status
	Evaluate(context.Context, decisions.Request) (decisions.Response, error)
	PrepareReconfigure(config.DecisionsConfig) (func(context.Context) error, error)
	AutoStart(context.Context) error
	Install(context.Context) error
	Start(context.Context) error
	Stop(context.Context) error
	Shutdown(context.Context) error
}

func (s *Server) initDecisions(installDir string) {
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return
	}
	if installDir == "" {
		if executable, err := os.Executable(); err == nil {
			installDir = filepath.Dir(executable)
		}
	}
	local := decisions.NewManager(decisionsLocalPolicy(cfg.Decisions), cfg.Directories.DataDir, decisions.WithRuntimeDirectory(installDir),
		decisions.WithLogger(s.Logger), decisions.WithDockerSelfContainer(s.decisionsDockerSelfContainer), decisions.WithRuntimeAccess(func() decisions.RuntimeAccess {
			current := s.ConfigSnapshot()
			return decisions.RuntimeAccess{IsDocker: current.Runtime.IsDocker, DockerEnabled: current.Docker.Enabled,
				DockerReadOnly: current.Docker.ReadOnly, DockerHost: current.Docker.Host}
		}))
	s.Decisions = newRoutedDecisions(s, local, cfg)
}

func (s *Server) decisionsDockerSelfContainer(ctx context.Context, host string) (string, error) {
	if !s.ConfigSnapshot().Runtime.IsDocker {
		return "", errors.New("Decisions Docker self identity requires a container runtime")
	}
	return resolveDecisionsDockerSelf(ctx, tools.DockerConfig{Host: host}, readContainerSelfSignals(true))
}

func resolveDecisionsDockerSelf(ctx context.Context, cfg tools.DockerConfig, signals containerSelfSignals) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, dockerSelfIdentityTimeout)
	defer cancel()
	data, code, err := tools.DockerRequestContext(ctx, cfg, http.MethodGet, "/containers/json?all=true", "")
	if err != nil {
		return "", err
	}
	if code != http.StatusOK {
		return "", errors.New("Decisions could not list Docker containers")
	}
	var containers []struct {
		ID         string   `json:"Id"`
		Names      []string `json:"Names"`
		HostConfig struct {
			NetworkMode string `json:"NetworkMode"`
		} `json:"HostConfig"`
	}
	if err := json.Unmarshal(data, &containers); err != nil {
		return "", err
	}
	entries := make([]tools.DockerContainerListEntry, 0, len(containers))
	for _, container := range containers {
		entries = append(entries, tools.DockerContainerListEntry{FullID: container.ID, Names: container.Names, NetworkMode: container.HostConfig.NetworkMode})
	}
	self, shared := containerSelfInList(entries, signals)
	if len(self) == 1 && len(shared) == 0 {
		for id := range self {
			return id, nil
		}
	}
	if len(self) == 0 {
		if id := proveSelfInSharedGroup(ctx, cfg, true, shared); id != "" {
			return id, nil
		}
	}
	return "", errors.New("Decisions could not prove AuraGo container identity")
}

func (s *Server) startDecisions(ctx context.Context) {
	if s.Decisions == nil {
		return
	}
	go func() {
		if err := s.Decisions.AutoStart(ctx); err != nil && s.Logger != nil {
			s.Logger.Warn("Decisions auto-start unavailable", "code", decisions.ErrorCode(err))
		}
	}()
}

func (s *Server) reconfigureDecisions(previous, next *config.Config) {
	if s.Decisions == nil {
		return
	}
	if next == nil {
		return
	}
	changed := previous == nil || previous.Decisions != next.Decisions
	if router, ok := s.Decisions.(*routedDecisions); ok {
		catalogCleanup := router.prepareCatalogConfiguration(next)
		if catalogCleanup != nil {
			go func() {
				ctx, cancel := context.WithTimeout(localLLMLifecycleContext(s), 10*time.Second)
				defer cancel()
				if err := catalogCleanup(ctx); err != nil && s.Logger != nil {
					s.Logger.Warn("Decisions model catalog cleanup failed", "code", decisions.ErrorCode(err))
				}
			}()
		}
		changed = previous == nil || router.configurationChanged(next)
	}
	if previous != nil && decisionsUseLocalBackend(previous.Decisions) && decisionsUseLocalBackend(next.Decisions) &&
		(previous.Docker.Enabled != next.Docker.Enabled || previous.Docker.ReadOnly != next.Docker.ReadOnly || previous.Docker.Host != next.Docker.Host) {
		changed = true
	}
	if !changed {
		return
	}
	// Revoke immediately; process/container cleanup must not hold the config lock.
	var cleanup func(context.Context) error
	var err error
	if router, ok := s.Decisions.(*routedDecisions); ok {
		cleanup, err = router.prepareConfiguration(next)
	} else {
		cleanup, err = s.Decisions.PrepareReconfigure(next.Decisions)
	}
	if err != nil {
		if s.Logger != nil {
			s.Logger.Warn("Decisions configuration rejected", "code", decisions.ErrorCode(err))
		}
		return
	}
	go func() {
		// Snapshot publication normally runs under CfgMu in its caller.
		s.CfgMu.RLock()
		s.CfgMu.RUnlock()
		ctx, cancel := context.WithTimeout(localLLMLifecycleContext(s), 10*time.Second)
		defer cancel()
		if err := cleanup(ctx); err != nil {
			if s.Logger != nil {
				s.Logger.Warn("Decisions configuration cleanup failed", "code", decisions.ErrorCode(err))
			}
			return
		}
		if current := s.ConfigSnapshot(); current != nil && current.Decisions.Enabled {
			s.startDecisions(localLLMLifecycleContext(s))
		}
	}()
}

func decisionsUseLocalBackend(c config.DecisionsConfig) bool {
	return c.Normalize().Backend != "provider"
}

func (s *Server) invalidateDecisionsProvider(providerID string) {
	router, ok := s.Decisions.(*routedDecisions)
	if !ok {
		return
	}
	cleanup := router.invalidateProvider(providerID)
	if cleanup == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(localLLMLifecycleContext(s), 10*time.Second)
		defer cancel()
		if err := cleanup(ctx); err != nil && s.Logger != nil {
			s.Logger.Warn("Decisions provider revocation cleanup failed", "code", decisions.ErrorCode(err))
		}
	}()
}

func (e *flowCatalogEnv) DecisionAvailability() flows.Availability {
	result := flows.Availability{State: flows.NeedsSetupState, Code: "DECISIONS_NOT_CONFIGURED", ConfigSection: "decisions"}
	if e.decisionsServer == nil || e.decisionsServer.Decisions == nil {
		return result
	}
	status := e.decisionsServer.Decisions.Status()
	switch {
	case !status.Enabled:
		result.Code = "DECISIONS_DISABLED"
	case status.Backend == "provider":
		if status.Configured {
			return flows.Availability{State: flows.AvailableState}
		}
		result.Code = "DECISIONS_NOT_CONFIGURED"
	case status.Code == decisions.CodeCPUConsentRequired:
		result.Code = "DECISIONS_CPU_FALLBACK_DENIED"
	case status.Code == decisions.CodeRuntimeAttestationFailed:
		result.Code = "DECISIONS_DEVICE_UNVERIFIED"
	case !status.Installed:
		result.Code = "DECISIONS_NOT_INSTALLED"
	case !status.Ready:
		result.Code = "DECISIONS_RUNTIME_UNAVAILABLE"
	default:
		return flows.Availability{State: flows.AvailableState}
	}
	return result
}

func (s *Server) shutdownDecisions(ctx context.Context) {
	if s.Decisions == nil {
		return
	}
	stopCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := s.Decisions.Shutdown(stopCtx); err != nil && s.Logger != nil {
		s.Logger.Warn("Decisions shutdown incomplete", "code", decisions.ErrorCode(err))
	}
}
