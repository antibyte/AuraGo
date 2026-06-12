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
	code := m.Run()
	closeAllSystemTaskStores()
	os.Exit(code)
}

func tempSystemTaskDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Cleanup(closeAllSystemTaskStores)
	return dir
}
