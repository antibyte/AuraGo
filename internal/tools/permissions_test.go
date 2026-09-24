package tools

import (
	"runtime"
	"strings"
	"testing"
)

func TestUnsafeHostExecutionRequiresExplicitGrant(t *testing.T) {
	perms := defaultRuntimePermissionsForTests()
	perms.AllowUnsafeHostExecution = false
	ConfigureRuntimePermissions(perms)
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })
	if _, _, err := ExecutePython("print('blocked')", tempSystemTaskDir(t), tempSystemTaskDir(t)); err == nil || !strings.Contains(err.Error(), "allow_unsafe_host_execution") {
		t.Fatalf("foreground Python error = %v", err)
	}
	if _, err := ExecutePythonBackground("print('blocked')", tempSystemTaskDir(t), tempSystemTaskDir(t), NewProcessRegistry(nil)); err == nil || !strings.Contains(err.Error(), "allow_unsafe_host_execution") {
		t.Fatalf("background Python error = %v", err)
	}
	if runtime.GOOS == "windows" {
		if _, _, err := ExecuteShell("echo blocked", tempSystemTaskDir(t)); err == nil || !strings.Contains(err.Error(), "allow_unsafe_host_execution") {
			t.Fatalf("Windows shell error = %v", err)
		}
	}
}

func TestHighRiskToolsDenyWithoutRuntimePolicy(t *testing.T) {
	ClearRuntimePermissionsForTest()
	t.Cleanup(func() {
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})

	if _, _, err := ExecuteShell("echo no", tempSystemTaskDir(t)); err == nil || !strings.Contains(err.Error(), "shell execution is disabled") {
		t.Fatalf("ExecuteShell error = %v, want permission denial", err)
	}
	if _, _, err := ExecutePython("print('no')", tempSystemTaskDir(t), tempSystemTaskDir(t)); err == nil || !strings.Contains(err.Error(), "python execution is disabled") {
		t.Fatalf("ExecutePython error = %v, want permission denial", err)
	}
	if got := ExecuteAPIRequest("GET", "https://example.com", "", nil); !strings.Contains(got, "network requests is disabled") {
		t.Fatalf("ExecuteAPIRequest = %s, want permission denial", got)
	}
	if got := ExecuteFilesystem("write_file", "x.txt", "", "no", nil, tempSystemTaskDir(t), 0, 0); !strings.Contains(got, "filesystem write is disabled") {
		t.Fatalf("ExecuteFilesystem = %s, want permission denial", got)
	}
	if got := DockerListContainers(DockerConfig{}, false); !strings.Contains(got, "docker is disabled") {
		t.Fatalf("DockerListContainers = %s, want permission denial", got)
	}
	sm := NewServiceManager()
	if _, err := sm.ManageService("status", "aurago"); err == nil || !strings.Contains(err.Error(), "shell execution is disabled") {
		t.Fatalf("ManageService error = %v, want shell permission denial", err)
	}
	mgr := NewCronManager(tempSystemTaskDir(t))
	t.Cleanup(func() { _ = mgr.Close() })
	if got, err := mgr.ManageSchedule("add", "job-1", "0 * * * *", "prompt", "en"); err != nil || !strings.Contains(got, "scheduler is disabled") {
		t.Fatalf("ManageSchedule = %s, err=%v, want scheduler permission denial", got, err)
	}
}
