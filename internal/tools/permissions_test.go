package tools

import (
	"strings"
	"testing"
)

func TestHighRiskToolsDenyWithoutRuntimePolicy(t *testing.T) {
	ClearRuntimePermissionsForTest()
	t.Cleanup(func() {
		ConfigureRuntimePermissions(RuntimePermissions{
			AllowShell:           true,
			AllowPython:          true,
			AllowFilesystemWrite: true,
			AllowNetworkRequests: true,
			DockerEnabled:        true,
		})
	})

	if _, _, err := ExecuteShell("echo no", t.TempDir()); err == nil || !strings.Contains(err.Error(), "shell execution is disabled") {
		t.Fatalf("ExecuteShell error = %v, want permission denial", err)
	}
	if _, _, err := ExecutePython("print('no')", t.TempDir(), t.TempDir()); err == nil || !strings.Contains(err.Error(), "python execution is disabled") {
		t.Fatalf("ExecutePython error = %v, want permission denial", err)
	}
	if got := ExecuteAPIRequest("GET", "https://example.com", "", nil); !strings.Contains(got, "network requests is disabled") {
		t.Fatalf("ExecuteAPIRequest = %s, want permission denial", got)
	}
	if got := ExecuteFilesystem("write_file", "x.txt", "", "no", nil, t.TempDir(), 0, 0); !strings.Contains(got, "filesystem write is disabled") {
		t.Fatalf("ExecuteFilesystem = %s, want permission denial", got)
	}
	if got := DockerListContainers(DockerConfig{}, false); !strings.Contains(got, "docker is disabled") {
		t.Fatalf("DockerListContainers = %s, want permission denial", got)
	}
}
