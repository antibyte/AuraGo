package tools

import (
	"os"
	"testing"
)

func defaultRuntimePermissionsForTests() RuntimePermissions {
	return RuntimePermissions{
		AllowShell:           true,
		AllowPython:          true,
		AllowFilesystemWrite: true,
		AllowNetworkRequests: true,
		DockerEnabled:        true,
		DockerReadOnly:       false,
		SchedulerEnabled:     true,
		MissionsEnabled:      true,
		MQTTEnabled:          true,
	}
}

func TestMain(m *testing.M) {
	ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	os.Exit(m.Run())
}
