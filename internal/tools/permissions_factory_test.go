package tools

import (
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestRuntimePermissionsFromConfigMapsAllRuntimeGates(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.AllowShell = true
	cfg.Agent.AllowPython = true
	cfg.Agent.AllowFilesystemWrite = true
	cfg.Agent.AllowNetworkRequests = true
	cfg.Agent.AllowPackageManager = true
	cfg.Docker.Enabled = true
	cfg.Docker.ReadOnly = true
	cfg.Tools.Scheduler.Enabled = true
	cfg.Tools.Scheduler.ReadOnly = true
	cfg.Tools.Missions.Enabled = true
	cfg.Tools.Missions.ReadOnly = true
	cfg.MQTT.Enabled = true
	cfg.MQTT.ReadOnly = true
	cfg.PackageManager.Enabled = true
	cfg.PackageManager.ReadOnly = true
	cfg.PackageManager.AllowInstall = true
	cfg.PackageManager.AllowRemove = true
	cfg.PackageManager.AllowUpgrade = true
	cfg.VirtualDesktop.WorkspaceDir = filepath.Join("C:", "workspace")

	perms := RuntimePermissionsFromConfig(cfg)
	if !perms.AllowShell || !perms.AllowPython || !perms.AllowFilesystemWrite || !perms.AllowNetworkRequests {
		t.Fatalf("agent permissions not mapped: %+v", perms)
	}
	if !perms.DockerEnabled || !perms.DockerReadOnly || !perms.SchedulerEnabled || !perms.SchedulerReadOnly || !perms.MissionsEnabled || !perms.MissionsReadOnly {
		t.Fatalf("integration permissions not mapped: %+v", perms)
	}
	if !perms.MQTTEnabled || !perms.MQTTReadOnly {
		t.Fatalf("mqtt permissions not mapped: %+v", perms)
	}
	if !perms.PackageManagerEnabled || !perms.PackageManagerReadOnly || !perms.PackageManagerAllowInstall || !perms.PackageManagerAllowRemove || !perms.PackageManagerAllowUpgrade {
		t.Fatalf("package manager permissions not mapped: %+v", perms)
	}
	if len(perms.ProtectedNotesRoots) != 2 || !strings.Contains(perms.ProtectedNotesRoots[0], "Documents") || !strings.Contains(perms.ProtectedNotesRoots[1], "Trash") {
		t.Fatalf("protected notes roots = %v", perms.ProtectedNotesRoots)
	}
}

func TestMQTTPermissionResolverOverridesScopedRuntimeSnapshot(t *testing.T) {
	ConfigureRuntimePermissions(RuntimePermissions{MQTTEnabled: true})
	SetMQTTPermissionResolver(func() (bool, bool) { return false, true })
	t.Cleanup(func() {
		SetMQTTPermissionResolver(nil)
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})

	if err := MQTTSubscribe("home/test", 0, nil); err == nil || !strings.Contains(err.Error(), "mqtt is disabled") {
		t.Fatalf("MQTTSubscribe error = %v, want live resolver disabled gate", err)
	}
}

func TestMQTTPublishPermissionReadsLiveResolverOnce(t *testing.T) {
	ConfigureRuntimePermissions(RuntimePermissions{MQTTEnabled: true})
	calls := 0
	SetMQTTPermissionResolver(func() (bool, bool) {
		calls++
		if calls == 1 {
			return true, true
		}
		return false, false
	})
	t.Cleanup(func() {
		SetMQTTPermissionResolver(nil)
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})

	err := MQTTPublish("home/test", "payload", 0, false, nil)
	if err == nil || !strings.Contains(err.Error(), "mqtt publish is disabled") {
		t.Fatalf("MQTTPublish error = %v, want read-only denial from one live snapshot", err)
	}
	if calls != 1 {
		t.Fatalf("live MQTT resolver calls = %d, want 1", calls)
	}
}
