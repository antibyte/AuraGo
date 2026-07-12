package virtualcomputers

import (
	"context"
	"strings"
	"testing"
)

type fakeSSHExecutor struct {
	output  string
	err     error
	runs    []string
	scripts []string
}

func (f *fakeSSHExecutor) Run(ctx context.Context, command string) (string, error) {
	f.runs = append(f.runs, command)
	return f.output, f.err
}

func (f *fakeSSHExecutor) RunScript(ctx context.Context, script string) (string, error) {
	f.scripts = append(f.scripts, script)
	return "[setup] ok", nil
}

func TestSetupPreflightParsesUnsupportedHost(t *testing.T) {
	executor := &fakeSSHExecutor{output: "ARCH=riscv64\nHAS_KVM=0\nOS_ID=debian\n"}
	manager := SetupManager{Executor: executor}
	result, err := manager.Preflight(context.Background())
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if result.Supported {
		t.Fatal("expected unsupported result")
	}
	want := []string{"unsupported architecture", "KVM is not available", "Ubuntu is required"}
	for _, part := range want {
		if !strings.Contains(strings.Join(result.Issues, "; "), part) {
			t.Fatalf("issues %v missing %q", result.Issues, part)
		}
	}
}

func TestSetupPreflightAllowsArm64UbuntuKVM(t *testing.T) {
	executor := &fakeSSHExecutor{output: "ARCH=aarch64\nHAS_KVM=1\nOS_ID=ubuntu\n"}
	manager := SetupManager{Executor: executor}
	result, err := manager.Preflight(context.Background())
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if !result.Supported {
		t.Fatalf("expected supported result, got issues=%v", result.Issues)
	}
}

func TestSetupInstallRunsScriptAndHealthCheck(t *testing.T) {
	executor := &fakeSSHExecutor{output: "ARCH=x86_64\nHAS_KVM=1\nOS_ID=ubuntu\n"}
	manager := SetupManager{
		Executor: executor,
		InstallOptions: SetupInstallOptions{
			InstallDir:         "/opt/boring-test",
			Token:              "boring-secret",
			MaxRunningMachines: 3,
			MaxForks:           2,
			AllowInternet:      true,
			AllowPersistent:    false,
			AllowPublish:       false,
			SkipDesktop:        true,
		},
	}
	status, err := manager.Install(context.Background())
	if err != nil {
		t.Fatalf("Install: %v status=%+v", err, status)
	}
	if !status.Configured || !status.Healthy {
		t.Fatalf("status = %+v", status)
	}
	if len(executor.scripts) != 1 {
		t.Fatalf("expected one install script, got %d", len(executor.scripts))
	}
	script := executor.scripts[0]
	for _, want := range []string{"BORING_ADDR=127.0.0.1:8080", "BORING_MAX_VALUE=3", "BORING_MAX_FORKS_VALUE=2", "SKIP_DESKTOP_VALUE=1"} {
		if !strings.Contains(script, want) {
			t.Fatalf("install script missing %q", want)
		}
	}
	if len(executor.runs) != 2 {
		t.Fatalf("expected preflight and healthcheck commands, got %d", len(executor.runs))
	}
	if !strings.Contains(executor.runs[1], "healthz") {
		t.Fatalf("healthcheck command not run: %v", executor.runs)
	}
}

func TestSetupInstallLogRedactsSecrets(t *testing.T) {
	manager := SetupManager{}
	log := manager.RedactInstallLog("export BORING_TOKEN=super-secret\nANTHROPIC_API_KEY=abc\nok")
	for _, leaked := range []string{"super-secret", "abc"} {
		if strings.Contains(log, leaked) {
			t.Fatalf("redacted log leaked %q: %s", leaked, log)
		}
	}
	if !strings.Contains(log, "<redacted>") {
		t.Fatalf("redacted log should contain marker: %s", log)
	}
}
