package tools

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestPackageRequirementsAndHostGrants(t *testing.T) {
	for _, spec := range []string{"requests", "foo[bar,baz]>=1.2,<2.0", "numpy==1.2.3", "name~=1.0"} {
		if !validPackageName.MatchString(spec) {
			t.Errorf("valid requirement rejected: %s", spec)
		}
	}
	for _, spec := range []string{"--target=/tmp", "-rfile", "https://example.com/pkg", "foo @ https://example.com", "foo==1; rm", "../pkg", "foo==http://example.com"} {
		if validPackageName.MatchString(spec) {
			t.Errorf("unsafe requirement accepted: %s", spec)
		}
	}
	perms := defaultRuntimePermissionsForTests()
	ConfigureRuntimePermissions(perms)
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })
	for _, denied := range []string{"python", "shell", "host"} {
		scoped := perms
		switch denied {
		case "python":
			scoped.AllowPython = false
		case "shell":
			scoped.AllowShell = false
		case "host":
			scoped.AllowUnsafeHostExecution = false
		}
		if _, _, err := InstallPackageContext(WithRuntimePermissions(context.Background(), scoped), "requests", t.TempDir()); err == nil {
			t.Fatalf("missing %s grant permitted installation", denied)
		}
	}
}

func TestManageProcessesUsesOnlyRegisteredHandle(t *testing.T) {
	if os.Getenv("AURAGO_PROCESS_FIXTURE") == "1" {
		time.Sleep(time.Minute)
		return
	}
	perms := defaultRuntimePermissionsForTests()
	ConfigureRuntimePermissions(perms)
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })
	cmd := exec.Command(os.Args[0], "-test.run=^TestManageProcessesUsesOnlyRegisteredHandle$")
	cmd.Env = append(os.Environ(), "AURAGO_PROCESS_FIXTURE=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	registry := NewProcessRegistry(slog.New(slog.NewTextHandler(io.Discard, nil)))
	result := ManageProcessesContext(context.Background(), "kill", int32(cmd.Process.Pid), "en", registry)
	if !strings.Contains(result, "not registered") {
		t.Fatalf("unmanaged process accepted: %s", result)
	}
	registry.Register(&ProcessInfo{PID: cmd.Process.Pid, Process: cmd.Process, Alive: true, StartedAt: time.Now()})
	var response ProcResult
	json.Unmarshal([]byte(ManageProcessesContext(context.Background(), "kill", int32(cmd.Process.Pid), "en", registry)), &response)
	if response.Status != "success" {
		t.Fatalf("managed process not terminated: %#v", response)
	}
}
