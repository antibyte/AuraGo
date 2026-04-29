package tools

import (
	"fmt"
	"sync/atomic"
)

// RuntimePermissions are the direct execution gates enforced inside high-risk tools.
type RuntimePermissions struct {
	AllowShell           bool
	AllowPython          bool
	AllowFilesystemWrite bool
	AllowNetworkRequests bool
	DockerEnabled        bool
	DockerReadOnly       bool
	SchedulerEnabled     bool
	SchedulerReadOnly    bool
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

func requireFilesystemWritePermission() error {
	perms, configured := currentRuntimePermissions()
	if !configured {
		return requireRuntimePermission("filesystem write", false)
	}
	return requireRuntimePermission("filesystem write", perms.AllowFilesystemWrite)
}
