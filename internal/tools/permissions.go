package tools

import (
	"fmt"
	"sync/atomic"
)

// RuntimePermissions are the direct execution gates enforced inside high-risk tools.
type RuntimePermissions struct {
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
	perms, configured := currentRuntimePermissions()
	if !configured {
		return requireRuntimePermission("mqtt", false)
	}
	return requireRuntimePermission("mqtt", perms.MQTTEnabled)
}

func requireMQTTPublishPermission() error {
	if err := requireMQTTPermission(); err != nil {
		return err
	}
	perms, _ := currentRuntimePermissions()
	if perms.MQTTReadOnly {
		return fmt.Errorf("mqtt publish is disabled by runtime permissions")
	}
	return nil
}

func requireMQTTMutationPermission() error {
	if err := requireMQTTPermission(); err != nil {
		return err
	}
	perms, _ := currentRuntimePermissions()
	if perms.MQTTReadOnly {
		return fmt.Errorf("mqtt mutation is disabled by runtime permissions")
	}
	return nil
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
