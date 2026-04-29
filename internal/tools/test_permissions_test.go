package tools

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	ConfigureRuntimePermissions(RuntimePermissions{
		AllowShell:           true,
		AllowPython:          true,
		AllowFilesystemWrite: true,
		AllowNetworkRequests: true,
		DockerEnabled:        true,
	})
	os.Exit(m.Run())
}
