package tools

import (
	"context"
	"errors"
	"log/slog"
	"runtime"
	"strings"
	"testing"
	"time"

	"aurago/internal/sandbox"
)

func TestShellPolicyUsesExecutionDialect(t *testing.T) {
	command := "echo ok\\; /bin/rm -rf ./classification-only"
	if err := validateIsolatedShellCommandPolicy(command); err != nil {
		t.Fatalf("POSIX worker policy rejected a literal argument: %v", err)
	}
	err := ValidateShellCommandPolicy(command)
	if (err != nil) != (runtime.GOOS == "windows") {
		t.Fatalf("host policy on %s returned %v; only PowerShell treats the semicolon as a separator", runtime.GOOS, err)
	}
}

func TestHostShellEntryPointsHonorRunPermissions(t *testing.T) {
	t.Cleanup(sandbox.SetForTest(&sandbox.FallbackSandbox{}))
	installFakeTicketSudo(t, &fakeTicketSudo{validateErr: errors.New("unexpected sudo authentication")})
	ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })

	for _, shellAllowed := range []bool{false, true} {
		perms := defaultRuntimePermissionsForTests()
		perms.AllowShell = shellAllowed
		perms.AllowUnsafeHostExecution = false
		perms.AllowUnsandboxedShell = false
		ctx := WithRuntimePermissions(context.Background(), perms)
		want := "shell execution is disabled"
		if shellAllowed {
			want = "allow_unsandboxed_shell"
			if runtime.GOOS == "windows" {
				want = "allow_unsafe_host_execution"
			}
		}
		registry := NewProcessRegistry(slog.Default())
		if _, err := ExecuteShellBackground(ctx, "echo must-not-start", t.TempDir(), registry); err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("background shell allowed=%v: %v, want %q", shellAllowed, err, want)
		}
		if _, _, err := ExecuteSudo(ctx, "id", t.TempDir(), "test-only"); err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("sudo shell allowed=%v: %v, want %q", shellAllowed, err, want)
		}
		if len(registry.List()) != 0 {
			t.Fatal("a denied call registered a process")
		}
	}
}

func TestHostShellEntryPointsDoNotStartWithCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	registry := NewProcessRegistry(slog.Default())
	if _, err := ExecuteShellBackground(ctx, "echo must-not-start", t.TempDir(), registry); !errors.Is(err, context.Canceled) {
		t.Fatalf("background error = %v, want canceled", err)
	}
	if _, _, err := ExecuteSudo(ctx, "id", t.TempDir(), "test-only"); !errors.Is(err, context.Canceled) {
		t.Fatalf("sudo error = %v, want canceled before authentication", err)
	}
	if len(registry.List()) != 0 {
		t.Fatal("a cancelled call registered a process")
	}
}

func TestExecuteShellBackgroundOutlivesDispatchContext(t *testing.T) {
	skipIfWindowsPowerShellUnavailable(t)
	t.Cleanup(sandbox.SetForTest(&sandbox.FallbackSandbox{}))
	ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := "sleep 0.3; echo background-finished"
	if runtime.GOOS == "windows" {
		command = "Start-Sleep -Milliseconds 300; Write-Output background-finished"
	}
	registry := NewProcessRegistry(slog.Default())
	pid, err := ExecuteShellBackground(ctx, command, t.TempDir(), registry)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if info, ok := registry.Get(pid); ok && info.IsAlive() {
			_ = registry.Terminate(pid)
		}
	})
	cancel()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if info, ok := registry.Get(pid); ok && !info.IsAlive() {
			if info.GetState() != ProcessStateExited || !strings.Contains(info.ReadOutput(), "background-finished") {
				t.Fatalf("state=%s, output=%q; cancellation stopped the background job", info.GetState(), info.ReadOutput())
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("background shell did not finish")
}
