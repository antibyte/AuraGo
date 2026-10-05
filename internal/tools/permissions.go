package tools

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"sync/atomic"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/sandbox"
)

// RuntimePermissions are the direct execution gates enforced inside high-risk tools.
type RuntimePermissions struct {
	ProtectedNotesRoots []string
	// ProtectedDataDir and ProtectedSystemFiles feed secureResolve's denylist
	// for AuraGo configuration, credential and database state.
	ProtectedDataDir           string
	ProtectedSystemFiles       []string
	AllowShell                 bool
	AllowPython                bool
	AllowUnsafeHostExecution   bool
	AllowFilesystemWrite       bool
	AllowNetworkRequests       bool
	DockerEnabled              bool
	DockerReadOnly             bool
	AllowDockerHostAccess      bool
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

// runtimePermissionResolverState binds every direct tool gate to the server's
// published config snapshot. Agent runs never write process-wide gates; they
// narrow a single dispatch through WithRuntimePermissions instead.
type runtimePermissionResolverState struct {
	resolve func() RuntimePermissions
}

var runtimePermissionResolver atomic.Pointer[runtimePermissionResolverState]

// SetRuntimePermissionResolver makes direct tool gates read the authoritative
// server snapshot. Passing nil restores the startup/test fallback written by
// ConfigureRuntimePermissions.
func SetRuntimePermissionResolver(resolve func() RuntimePermissions) {
	if resolve == nil {
		runtimePermissionResolver.Store(nil)
		return
	}
	runtimePermissionResolver.Store(&runtimePermissionResolverState{resolve: resolve})
}

type runtimePermissionsContextKey struct{}

// WithRuntimePermissions attaches one agent run's effective gates to ctx.
// Context permissions can only narrow the server snapshot, never widen it.
func WithRuntimePermissions(ctx context.Context, perms RuntimePermissions) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	scoped := perms
	scoped.ProtectedNotesRoots = append([]string(nil), perms.ProtectedNotesRoots...)
	return context.WithValue(ctx, runtimePermissionsContextKey{}, scoped)
}

// EffectiveRuntimePermissions returns the server snapshot intersected with any
// run-scoped permissions carried by ctx. Without a configured server snapshot
// the run-scoped permissions are authoritative (standalone and test callers).
func EffectiveRuntimePermissions(ctx context.Context) (RuntimePermissions, bool) {
	server, configured := currentRuntimePermissions()
	if ctx == nil {
		return server, configured
	}
	run, scoped := ctx.Value(runtimePermissionsContextKey{}).(RuntimePermissions)
	if !scoped {
		return server, configured
	}
	if !configured {
		return run, true
	}
	return intersectRuntimePermissions(server, run), true
}

// intersectRuntimePermissions keeps the stricter value of every gate. It lists
// each field explicitly; TestIntersectRuntimePermissionsCoversEveryField fails
// when a new RuntimePermissions field is not handled here.
func intersectRuntimePermissions(a, b RuntimePermissions) RuntimePermissions {
	return RuntimePermissions{
		ProtectedNotesRoots:        unionRuntimePaths(a.ProtectedNotesRoots, b.ProtectedNotesRoots),
		ProtectedDataDir:           firstNonEmptyRuntimePath(a.ProtectedDataDir, b.ProtectedDataDir),
		ProtectedSystemFiles:       unionRuntimePaths(a.ProtectedSystemFiles, b.ProtectedSystemFiles),
		AllowShell:                 a.AllowShell && b.AllowShell,
		AllowPython:                a.AllowPython && b.AllowPython,
		AllowUnsafeHostExecution:   a.AllowUnsafeHostExecution && b.AllowUnsafeHostExecution,
		AllowFilesystemWrite:       a.AllowFilesystemWrite && b.AllowFilesystemWrite,
		AllowNetworkRequests:       a.AllowNetworkRequests && b.AllowNetworkRequests,
		DockerEnabled:              a.DockerEnabled && b.DockerEnabled,
		DockerReadOnly:             a.DockerReadOnly || b.DockerReadOnly,
		AllowDockerHostAccess:      a.AllowDockerHostAccess && b.AllowDockerHostAccess,
		SchedulerEnabled:           a.SchedulerEnabled && b.SchedulerEnabled,
		SchedulerReadOnly:          a.SchedulerReadOnly || b.SchedulerReadOnly,
		MissionsEnabled:            a.MissionsEnabled && b.MissionsEnabled,
		MissionsReadOnly:           a.MissionsReadOnly || b.MissionsReadOnly,
		MQTTEnabled:                a.MQTTEnabled && b.MQTTEnabled,
		MQTTReadOnly:               a.MQTTReadOnly || b.MQTTReadOnly,
		PackageManagerEnabled:      a.PackageManagerEnabled && b.PackageManagerEnabled,
		PackageManagerReadOnly:     a.PackageManagerReadOnly || b.PackageManagerReadOnly,
		PackageManagerAllowInstall: a.PackageManagerAllowInstall && b.PackageManagerAllowInstall,
		PackageManagerAllowRemove:  a.PackageManagerAllowRemove && b.PackageManagerAllowRemove,
		PackageManagerAllowUpgrade: a.PackageManagerAllowUpgrade && b.PackageManagerAllowUpgrade,
	}
}

func firstNonEmptyRuntimePath(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func unionRuntimePaths(a, b []string) []string {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, path := range append(append([]string(nil), a...), b...) {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
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
		ProtectedDataDir:           cfg.Directories.DataDir,
		ProtectedSystemFiles:       protectedSystemFilesFromConfig(cfg),
		AllowShell:                 cfg.Agent.AllowShell,
		AllowPython:                cfg.Agent.AllowPython,
		AllowUnsafeHostExecution:   cfg.Agent.AllowUnsafeHostExecution,
		AllowFilesystemWrite:       cfg.Agent.AllowFilesystemWrite,
		AllowNetworkRequests:       cfg.Agent.AllowNetworkRequests,
		DockerEnabled:              cfg.Docker.Enabled,
		DockerReadOnly:             cfg.Docker.ReadOnly,
		AllowDockerHostAccess:      cfg.Docker.AllowHostAccess,
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

// CurrentRuntimePermissionsForTest exposes the process gate seen by
// context-free tool callers to tests in other packages.
func CurrentRuntimePermissionsForTest() (RuntimePermissions, bool) {
	return currentRuntimePermissions()
}

func currentRuntimePermissions() (RuntimePermissions, bool) {
	if resolver := runtimePermissionResolver.Load(); resolver != nil && resolver.resolve != nil {
		return resolver.resolve(), true
	}
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
	return requireShellPermissionContext(context.Background())
}

func requireShellPermissionContext(ctx context.Context) error {
	perms, configured := EffectiveRuntimePermissions(ctx)
	if !configured {
		return requireRuntimePermission("shell execution", false)
	}
	if err := requireRuntimePermission("shell execution", perms.AllowShell); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		if !perms.AllowUnsafeHostExecution {
			return fmt.Errorf("Windows host shell requires agent.allow_unsafe_host_execution")
		}
		slog.Warn("Unsafe host execution authorized", "kind", "windows_shell")
	}
	return nil
}

func requirePythonPermission() error {
	return requirePythonPermissionContext(context.Background())
}

func requirePythonPermissionContext(ctx context.Context) error {
	perms, configured := EffectiveRuntimePermissions(ctx)
	if !configured {
		return requireRuntimePermission("python execution", false)
	}
	if err := requireRuntimePermission("python execution", perms.AllowPython); err != nil {
		return err
	}
	if !perms.AllowUnsafeHostExecution {
		return fmt.Errorf("host Python requires agent.allow_unsafe_host_execution")
	}
	slog.Warn("Unsafe host execution authorized", "kind", "host_python")
	return nil
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
