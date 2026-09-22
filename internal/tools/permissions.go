package tools

import (
	"fmt"
	"path/filepath"
	"sync/atomic"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/sandbox"
)

// RuntimePermissions are the direct execution gates enforced inside high-risk tools.
type RuntimePermissions struct {
	ProtectedNotesRoots        []string
	AllowShell                 bool
	AllowPython                bool
	AllowFilesystemWrite       bool
	AllowNetworkRequests       bool
	DockerEnabled              bool
	DockerReadOnly             bool
	SchedulerEnabled           bool
	SchedulerReadOnly          bool
	MissionsEnabled            bool
	MissionsReadOnly           bool
	MQTTEnabled                bool
	MQTTReadOnly               bool
	PackageManagerEnabled      bool
	PackageManagerReadOnly     bool
	PackageManagerAllowInstall bool
	PackageManagerAllowRemove  bool
	PackageManagerAllowUpgrade bool
}

var runtimePermissions atomic.Pointer[RuntimePermissions]

// mqttPermissionResolverState keeps server-owned MQTT gates independent from
// the process-wide snapshot used by agent dispatch. Agent turns may update the
// latter for their scoped config; the live bridge must still consult the
// authoritative server snapshot.
type mqttPermissionResolverState struct {
	resolve func() (enabled, readOnly bool)
}

var mqttPermissionResolver atomic.Pointer[mqttPermissionResolverState]

// SetMQTTPermissionResolver binds the live MQTT bridge to the server's
// immutable config snapshot. Passing nil restores the test/standalone fallback
// to RuntimePermissions.
func SetMQTTPermissionResolver(resolve func() (enabled, readOnly bool)) {
	if resolve == nil {
		mqttPermissionResolver.Store(nil)
		return
	}
	mqttPermissionResolver.Store(&mqttPermissionResolverState{resolve: resolve})
}

// RuntimePermissionsFromConfig builds a complete runtime gate snapshot from a
// config. Keep this as the single conversion used at startup, reload, and
// agent dispatch so omitted fields cannot silently disable an integration.
func RuntimePermissionsFromConfig(cfg *config.Config) RuntimePermissions {
	if cfg == nil {
		return RuntimePermissions{}
	}
	packageManagerEnabled := cfg.Agent.AllowPackageManager && cfg.PackageManager.Enabled && (!cfg.Runtime.IsDocker || cfg.Agent.SudoEnabled)
	var protectedNotes []string
	if cfg.VirtualDesktop.WorkspaceDir != "" {
		protectedNotes = []string{
			filepath.Join(cfg.VirtualDesktop.WorkspaceDir, filepath.FromSlash(desktop.NotesDirectory)),
			filepath.Join(cfg.VirtualDesktop.WorkspaceDir, filepath.FromSlash(desktop.NotesTrashDirectory)),
		}
	}
	return RuntimePermissions{
		ProtectedNotesRoots:        protectedNotes,
		AllowShell:                 cfg.Agent.AllowShell,
		AllowPython:                cfg.Agent.AllowPython,
		AllowFilesystemWrite:       cfg.Agent.AllowFilesystemWrite,
		AllowNetworkRequests:       cfg.Agent.AllowNetworkRequests,
		DockerEnabled:              cfg.Docker.Enabled,
		DockerReadOnly:             cfg.Docker.ReadOnly,
		SchedulerEnabled:           cfg.Tools.Scheduler.Enabled,
		SchedulerReadOnly:          cfg.Tools.Scheduler.ReadOnly,
		MissionsEnabled:            cfg.Tools.Missions.Enabled,
		MissionsReadOnly:           cfg.Tools.Missions.ReadOnly,
		MQTTEnabled:                cfg.MQTT.Enabled,
		MQTTReadOnly:               cfg.MQTT.ReadOnly,
		PackageManagerEnabled:      packageManagerEnabled,
		PackageManagerReadOnly:     cfg.PackageManager.ReadOnly,
		PackageManagerAllowInstall: cfg.PackageManager.AllowInstall,
		PackageManagerAllowRemove:  cfg.PackageManager.AllowRemove,
		PackageManagerAllowUpgrade: cfg.PackageManager.AllowUpgrade,
	}
}

func ConfigureRuntimePermissions(perms RuntimePermissions) {
	copy := perms
	runtimePermissions.Store(&copy)
}

func ClearRuntimePermissionsForTest() {
	runtimePermissions.Store(nil)
}

func currentRuntimePermissions() (RuntimePermissions, bool) {
	if perms := runtimePermissions.Load(); perms != nil {
		return *perms, true
	}
	return RuntimePermissions{}, false
}

func requireUnprotectedNotesPath(path string, parents bool) error {
	perms, _ := currentRuntimePermissions()
	if sandbox.ProtectedFilePath(path, perms.ProtectedNotesRoots, parents) {
		return fmt.Errorf("Desktop Notes are protected; use desktop_notes to list, read, search or create. Existing notes cannot be changed or deleted")
	}
	return nil
}

func requireRuntimePermission(name string, allowed bool) error {
	if !allowed {
		return fmt.Errorf("%s is disabled by runtime permissions", name)
	}
	return nil
}

func requireShellPermission() error {
	perms, configured := currentRuntimePermissions()
	if !configured {
		return requireRuntimePermission("shell execution", false)
	}
	return requireRuntimePermission("shell execution", perms.AllowShell)
}

func requirePythonPermission() error {
	perms, configured := currentRuntimePermissions()
	if !configured {
		return requireRuntimePermission("python execution", false)
	}
	return requireRuntimePermission("python execution", perms.AllowPython)
}

func requireNetworkPermission() error {
	perms, configured := currentRuntimePermissions()
	if !configured {
		return requireRuntimePermission("network requests", false)
	}
	return requireRuntimePermission("network requests", perms.AllowNetworkRequests)
}

func requireDockerPermission() error {
	perms, configured := currentRuntimePermissions()
	if !configured {
		return requireRuntimePermission("docker", false)
	}
	return requireRuntimePermission("docker", perms.DockerEnabled)
}

func requireDockerMutationPermission() error {
	if err := requireDockerPermission(); err != nil {
		return err
	}
	perms, _ := currentRuntimePermissions()
	if perms.DockerReadOnly {
		return fmt.Errorf("docker mutation is disabled by runtime permissions")
	}
	return nil
}

func requireSchedulerPermission(operation string) error {
	if operation == "list" {
		return nil
	}
	perms, configured := currentRuntimePermissions()
	if !configured {
		return requireRuntimePermission("scheduler", false)
	}
	if err := requireRuntimePermission("scheduler", perms.SchedulerEnabled); err != nil {
		return err
	}
	if perms.SchedulerReadOnly && operation != "list" {
		return fmt.Errorf("scheduler mutation is disabled by runtime permissions")
	}
	return nil
}

func requireMissionMutationPermission() error {
	perms, configured := currentRuntimePermissions()
	if !configured {
		return requireRuntimePermission("missions", false)
	}
	if err := requireRuntimePermission("missions", perms.MissionsEnabled); err != nil {
		return err
	}
	if perms.MissionsReadOnly {
		return fmt.Errorf("mission mutation is disabled by runtime permissions")
	}
	return nil
}

func requireMQTTPermission() error {
	enabled, _, configured := currentMQTTPermissions()
	if !configured {
		return requireRuntimePermission("mqtt", false)
	}
	return requireRuntimePermission("mqtt", enabled)
}

func requireMQTTPublishPermission() error {
	enabled, readOnly, configured := currentMQTTPermissions()
	if !configured {
		return requireRuntimePermission("mqtt", false)
	}
	if err := requireRuntimePermission("mqtt", enabled); err != nil {
		return err
	}
	if readOnly {
		return fmt.Errorf("mqtt publish is disabled by runtime permissions")
	}
	return nil
}

func requireMQTTMutationPermission() error {
	enabled, readOnly, configured := currentMQTTPermissions()
	if !configured {
		return requireRuntimePermission("mqtt", false)
	}
	if err := requireRuntimePermission("mqtt", enabled); err != nil {
		return err
	}
	if readOnly {
		return fmt.Errorf("mqtt mutation is disabled by runtime permissions")
	}
	return nil
}

func currentMQTTPermissions() (enabled, readOnly, configured bool) {
	if resolver := mqttPermissionResolver.Load(); resolver != nil && resolver.resolve != nil {
		enabled, readOnly = resolver.resolve()
		return enabled, readOnly, true
	}
	perms, configured := currentRuntimePermissions()
	return perms.MQTTEnabled, perms.MQTTReadOnly, configured
}

func requirePackageManagerPermission() error {
	perms, configured := currentRuntimePermissions()
	if !configured {
		return requireRuntimePermission("package manager", false)
	}
	return requireRuntimePermission("package manager", perms.PackageManagerEnabled)
}

func requirePackageManagerMutationPermission(operation string) error {
	if err := requirePackageManagerPermission(); err != nil {
		return err
	}
	perms, _ := currentRuntimePermissions()
	if perms.PackageManagerReadOnly {
		return fmt.Errorf("package manager mutation is disabled by runtime permissions")
	}
	switch operation {
	case "install":
		if !perms.PackageManagerAllowInstall {
			return fmt.Errorf("package install is disabled by runtime permissions")
		}
	case "remove":
		if !perms.PackageManagerAllowRemove {
			return fmt.Errorf("package removal is disabled by runtime permissions")
		}
	case "update", "upgrade":
		if !perms.PackageManagerAllowUpgrade {
			return fmt.Errorf("package update/upgrade is disabled by runtime permissions")
		}
	}
	return nil
}

func requireFilesystemWritePermission() error {
	perms, configured := currentRuntimePermissions()
	if !configured {
		return requireRuntimePermission("filesystem write", false)
	}
	return requireRuntimePermission("filesystem write", perms.AllowFilesystemWrite)
}
