package server

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/cydcreator"
	"aurago/internal/desktop"
	"aurago/internal/security"
	"aurago/internal/tools"
)

// cydCreatorLimits maps the live part of cyd_creator to the service limits.
// It normalizes a copy first: a snapshot that did not pass config.Load may
// carry zero or out-of-range values, and an all-zero ServiceLimits would
// become the defaults with OTA switched on.
func cydCreatorLimits(c config.CYDCreatorConfig) cydcreator.ServiceLimits {
	config.NormalizeCYDCreatorConfig(&c)
	return cydcreator.ServiceLimits{
		BuildTimeout:         time.Duration(c.BuildTimeoutSeconds) * time.Second,
		ColdBuildTimeout:     time.Duration(c.ColdBuildTimeoutSeconds) * time.Second,
		KeepBuildsPerProject: c.KeepBuildsPerProject,
		MaxRepairAttempts:    c.MaxRepairAttempts,
		JobTimeout:           time.Duration(c.JobTimeoutMinutes) * time.Minute,
		Model:                c.Model,
		OTA:                  cydcreator.OTALimits{Enabled: c.OTA.Enabled, HeartbeatSeconds: c.OTA.HeartbeatSeconds, PublicURL: c.OTA.PublicURL},
	}
}

func cydCreatorBuilderSettings(c config.CYDCreatorConfig) cydBuilderSettings {
	return cydBuilderSettings{Image: c.BuilderImage, MemoryMB: c.MemoryMB, CPUCores: c.CPUCores,
		IdleStop: time.Duration(c.IdleStopMinutes) * time.Minute}
}

// cydCreatorRuntimeConfigChanged reports a change of a builder container
// setting, which initCYDCreator reads only at startup. Switching the feature
// off, timeouts, retention, model and OTA apply live (API gate,
// Service.UpdateLimits); switching it on without a service needs a restart
// (handled next to the Game Maker check in the config handler).
func cydCreatorRuntimeConfigChanged(oldCfg, newCfg config.CYDCreatorConfig) bool {
	return oldCfg.BuilderImage != newCfg.BuilderImage ||
		oldCfg.MemoryMB != newCfg.MemoryMB ||
		oldCfg.CPUCores != newCfg.CPUCores ||
		oldCfg.IdleStopMinutes != newCfg.IdleStopMinutes
}

// cydCreatorStorageProblem reports why the directory of sqlite.cyd_creator_path
// must not be used, "" when it may. The builder's pull record
// (cyd_builder_image.json, cydBuilderTrustFile) lives next to the database;
// a record the agent could write would make a locally built image pass as
// AuraGo's own pull. The agent's file tools reach the agent_workspace root
// (mirrors tools.detectFilesystemProjectRoot) except the protected data
// directory, its desktop tools the whole virtual desktop workspace.
func cydCreatorStorageProblem(cfg *config.Config, desktopWorkspace string) string {
	if cfg == nil || strings.TrimSpace(cfg.SQLite.CYDCreatorPath) == "" {
		return "" // NewService refuses an empty database path
	}
	trust, err := filepath.Abs(cydBuilderTrustFile(cfg.SQLite.CYDCreatorPath))
	if err != nil {
		return "sqlite.cyd_creator_path cannot be resolved"
	}
	if root := strings.TrimSpace(desktopWorkspace); root != "" && cydPathWithin(root, trust) {
		return "sqlite.cyd_creator_path lies inside the virtual desktop workspace"
	}
	if workdir := strings.TrimSpace(cfg.Directories.WorkspaceDir); workdir != "" &&
		cydPathWithin(cydAgentFileRoot(workdir), trust) && !tools.IsProtectedSystemPath(trust, "", cfg) {
		return "sqlite.cyd_creator_path lies inside the agent workspace"
	}
	return ""
}

// cydAgentFileRoot is the root the agent's file tools may write below:
// the nearest agent_workspace ancestor of directories.workspace_dir, else
// the workspace itself (tools.filesystemRoots).
func cydAgentFileRoot(workdir string) string {
	current := cydCanonicalPath(workdir)
	for dir := current; ; {
		if strings.EqualFold(filepath.Base(dir), "agent_workspace") {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return current
		}
		dir = parent
	}
}

// cydCanonicalPath makes path absolute and resolves links of its longest
// existing prefix, so a link cannot hide that a path lies below a root.
func cydCanonicalPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = filepath.Clean(path)
	}
	var missing []string
	for current := abs; ; {
		if resolved, err := filepath.EvalSymlinks(current); err == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return resolved
		}
		parent := filepath.Dir(current)
		if parent == current {
			return abs
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

// cydPathWithin reports whether candidate is root or lies below it
// (filepath.Rel compares case-insensitively on Windows).
func cydPathWithin(root, candidate string) bool {
	rel, err := filepath.Rel(cydCanonicalPath(root), cydCanonicalPath(candidate))
	if err != nil || filepath.IsAbs(rel) {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// cydCreatorStartFailed is the generic cydCreatorProblem of a service that
// could not be created (database, directories); the cause is in the log.
const cydCreatorStartFailed = "CYD Creator could not start; the server log has the details"

// cydCreatorRestartReason is the restart reason a config save adds for CYD
// Creator, "" for none. While the service runs, the builder settings and
// docker.host are read only at startup. Without a service, switching the
// feature on needs a restart, unless the new configuration would be refused
// again (cydCreatorStorageProblem): a restart cannot help then, and the
// capabilities report cydCreatorProblem instead.
func (s *Server) cydCreatorRestartReason(oldCfg, newCfg *config.Config) string {
	if s == nil || oldCfg == nil || newCfg == nil {
		return ""
	}
	if s.CYDCreator != nil {
		if cydCreatorRuntimeConfigChanged(oldCfg.CYDCreator, newCfg.CYDCreator) || oldCfg.Docker.Host != newCfg.Docker.Host {
			return "CYD Creator builder"
		}
		return ""
	}
	if newCfg.CYDCreator.Enabled && cydCreatorStorageProblem(newCfg, desktop.ConfigFromAuraConfig(newCfg).WorkspaceDir) == "" {
		return "CYD Creator service"
	}
	return ""
}

// initCYDCreator creates the CYD Creator service when cyd_creator.enabled is
// on. Projects live in the virtual desktop workspace (<workspace>/CYD/),
// artefacts in <data>/cyd_creator/. A failure leaves the feature off and
// records a generic reason (no paths) in cydCreatorProblem.
func (s *Server) initCYDCreator() {
	if s == nil || s.Cfg == nil || !s.Cfg.CYDCreator.Enabled {
		return
	}
	cfg := s.Cfg
	desktopCfg := desktop.ConfigFromAuraConfig(cfg)
	if problem := cydCreatorStorageProblem(cfg, desktopCfg.WorkspaceDir); problem != "" {
		s.cydCreatorProblem = problem
		if s.Logger != nil {
			s.Logger.Error("CYD Creator stays off: the agent could write next to its database; move sqlite.cyd_creator_path out of the agent and desktop workspaces and restart",
				"reason", problem, "path", cfg.SQLite.CYDCreatorPath)
		}
		return
	}
	settings := cydCreatorBuilderSettings(cfg.CYDCreator)
	settings.TrustFile = cydBuilderTrustFile(cfg.SQLite.CYDCreatorPath)
	builder := newCYDDockerBuilder(tools.DockerConfig{Host: cfg.Docker.Host}, settings, s.Logger)
	service, err := cydcreator.NewService(cydcreator.ServiceConfig{
		WorkspaceDir: desktopCfg.WorkspaceDir,
		DataDir:      desktopCfg.DataDir,
		DBPath:       cfg.SQLite.CYDCreatorPath,
		Builder:      builder,
		Limits:       cydCreatorLimits(cfg.CYDCreator),
		Logger:       s.Logger,
		Scrub:        security.Scrub,
	})
	if err != nil {
		s.cydCreatorProblem = cydCreatorStartFailed
		if s.Logger != nil {
			s.Logger.Error("Failed to initialize CYD Creator", "error", err)
		}
		builder.Close()
		return
	}
	s.sweepCYDCreatorAgentSessions(service)
	service.SetRunner(&cydCreatorAgentRunner{server: s, service: service})
	s.CYDCreator, s.cydBuilder, s.cydCreatorProblem = service, builder, ""
	cydcreator.SetDefaultService(service)
	if s.Logger != nil {
		s.Logger.Info("CYD Creator initialized", "builder_image", cfg.CYDCreator.BuilderImage)
	}
}

// cydCreatorAgentSessionPrefix names a job's transient agent session in
// short-term memory (cyd-creator-<job>); the runner purges it after every
// phase.
const cydCreatorAgentSessionPrefix = "cyd-creator-"

// sweepCYDCreatorAgentSessions purges agent sessions that a crash left behind
// before the runner's deferred purge ran. initCYDCreator calls it before the
// runner is installed, so no phase can be using one; a session whose job is
// still queued or running is kept all the same.
func (s *Server) sweepCYDCreatorAgentSessions(service *cydcreator.Service) {
	if s == nil || s.ShortTermMem == nil || service == nil {
		return
	}
	ids, err := s.ShortTermMem.TransientSessionIDs(cydCreatorAgentSessionPrefix)
	if err != nil {
		if s.Logger != nil {
			s.Logger.Warn("CYD Creator could not list leftover agent sessions", "error", err)
		}
		return
	}
	for _, id := range ids {
		job, err := service.GetJob(context.Background(), strings.TrimPrefix(id, cydCreatorAgentSessionPrefix))
		if err == nil && (job.Status == cydcreator.JobQueued || job.Status == cydcreator.JobRunning) {
			continue
		}
		if err := s.ShortTermMem.PurgeChatSession(id); err != nil && s.Logger != nil {
			s.Logger.Warn("CYD Creator could not purge a leftover agent session", "session_id", id, "error", err)
		}
	}
}

// shutdownCYDCreator stops the build worker (a running build ends as
// interrupted), closes the database, then stops and removes the builder
// container (cydDockerBuilder.Close; the cache volume stays).
func (s *Server) shutdownCYDCreator() {
	if s == nil {
		return
	}
	if s.CYDCreator != nil {
		if err := s.CYDCreator.Close(); err != nil && s.Logger != nil {
			s.Logger.Warn("CYD Creator could not be closed", "error", err)
		}
	}
	if cydcreator.DefaultService() == s.CYDCreator {
		cydcreator.SetDefaultService(nil)
	}
	if s.cydBuilder != nil {
		s.cydBuilder.Close()
	}
}

// cydCreatorAvailable gates the API: the service exists and the switch is on.
func (s *Server) cydCreatorAvailable() bool {
	if s == nil || s.CYDCreator == nil {
		return false
	}
	cfg := s.ConfigSnapshot()
	return cfg != nil && cfg.CYDCreator.Enabled
}

// cydCreatorCapability backs the desktop capability "cyd_creator": the API is
// available and Docker is configured and writable (the builder creates,
// starts and execs into its container; docker.readonly refuses that).
func (s *Server) cydCreatorCapability() bool {
	cfg := s.ConfigSnapshot()
	return s.cydCreatorAvailable() && cfg.Docker.Enabled && !cfg.Docker.ReadOnly
}
