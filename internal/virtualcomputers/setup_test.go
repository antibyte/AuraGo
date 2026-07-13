package virtualcomputers

import (
	"context"
	"os"
	"runtime"
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

func TestSetupPreflightSSHFallbackRequestsFullHostMetadata(t *testing.T) {
	executor := &fakeSSHExecutor{output: "HOST_OS=linux\nARCH=x86_64\nHAS_KVM=1\nOS_ID=ubuntu\nRUNNING_IN_DOCKER=0\nHAS_SYSTEMD=1\nHAS_SUDO_OR_ROOT=1\n"}
	manager := SetupManager{Executor: executor}
	result, err := manager.Preflight(context.Background())
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if !result.Supported {
		t.Fatalf("expected supported result, got issues=%v", result.Issues)
	}
	if len(executor.runs) != 1 {
		t.Fatalf("expected one fallback command, got %d", len(executor.runs))
	}
	command := executor.runs[0]
	for _, want := range []string{"HOST_OS", "RUNNING_IN_DOCKER", "HAS_SYSTEMD", "HAS_SUDO_OR_ROOT"} {
		if !strings.Contains(command, want) {
			t.Fatalf("fallback preflight command missing %q: %s", want, command)
		}
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
	if !status.ControlPlane.Healthy || !status.Management.Healthy {
		t.Fatalf("component status = %+v", status)
	}
	if len(executor.scripts) != 1 {
		t.Fatalf("expected one install script, got %d", len(executor.scripts))
	}
	script := executor.scripts[0]
	for _, want := range []string{"BORING_ADDR_VALUE='127.0.0.1:18080'", "BORING_ADDR=${BORING_ADDR_VALUE}", "BORING_MAX_VALUE=3", "BORING_MAX_FORKS_VALUE=2", "SKIP_DESKTOP_VALUE=1", PinnedUpstreamRevision, "boring-web.service"} {
		if !strings.Contains(script, want) {
			t.Fatalf("install script missing %q", want)
		}
	}
	if len(executor.runs) != 3 {
		t.Fatalf("expected preflight and two component healthcheck commands, got %d", len(executor.runs))
	}
	if !strings.Contains(executor.runs[1], "healthz") {
		t.Fatalf("healthcheck command not run: %v", executor.runs)
	}
	if !strings.Contains(executor.runs[2], "/boring-computers/") {
		t.Fatalf("management healthcheck command not run: %v", executor.runs)
	}
}

func TestSetupInstallUsesConfiguredBoringdURL(t *testing.T) {
	executor := &fakeSSHExecutor{output: "ARCH=x86_64\nHAS_KVM=1\nOS_ID=ubuntu\n"}
	manager := SetupManager{
		Executor: executor,
		InstallOptions: SetupInstallOptions{
			BoringdURL: "http://127.0.0.1:18080",
			Token:      "boring-secret",
		},
	}
	status, err := manager.Install(context.Background())
	if err != nil {
		t.Fatalf("Install: %v status=%+v", err, status)
	}
	if len(executor.scripts) != 1 {
		t.Fatalf("expected one install script, got %d", len(executor.scripts))
	}
	script := executor.scripts[0]
	for _, want := range []string{"BORING_ADDR_VALUE='127.0.0.1:18080'", "BORING_ADDR=${BORING_ADDR_VALUE}", "http://127.0.0.1:18080/healthz"} {
		if !strings.Contains(script, want) {
			t.Fatalf("install script missing %q", want)
		}
	}
	if len(executor.runs) != 3 {
		t.Fatalf("expected preflight and two component healthcheck commands, got %d", len(executor.runs))
	}
	if !strings.Contains(executor.runs[1], "http://127.0.0.1:18080/healthz") {
		t.Fatalf("healthcheck command uses wrong URL: %v", executor.runs)
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

func TestParsePreflightOutputBlocksLocalUnsupportedChecks(t *testing.T) {
	result := ParsePreflightOutput("HOST_OS=windows\nARCH=amd64\nHAS_KVM=0\nOS_ID=ubuntu\nRUNNING_IN_DOCKER=1\nHAS_SYSTEMD=0\nHAS_SUDO_OR_ROOT=0\n")
	if result.Supported {
		t.Fatal("expected local unsupported result")
	}
	joined := strings.Join(result.Issues, "; ")
	for _, want := range []string{
		"local boring-computers setup requires Linux",
		"KVM is not available",
		"Docker",
		"systemd",
		"root or passwordless sudo",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("issues %v missing %q", result.Issues, want)
		}
	}
}

func TestLocalCommandExecutorPreflightReportsSupportedLinuxHost(t *testing.T) {
	executor := LocalCommandExecutor{
		RuntimeGOOS:   "linux",
		RuntimeArch:   "amd64",
		OSReleaseData: "ID=ubuntu\nVERSION_ID=\"24.04\"\n",
		PathExists: func(path string) bool {
			return path == "/dev/kvm" || path == "/run/systemd/system"
		},
		EffectiveUID:   func() int { return 0 },
		DockerDetected: func() bool { return false },
	}
	out, err := executor.Preflight(context.Background())
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	result := ParsePreflightOutput(out)
	if !result.Supported {
		t.Fatalf("expected supported local host, got issues=%v checks=%v", result.Issues, result.Checks)
	}
	for key, want := range map[string]string{
		"HOST_OS":           "linux",
		"ARCH":              "amd64",
		"HAS_KVM":           "1",
		"OS_ID":             "ubuntu",
		"OS_VERSION":        "24.04",
		"RUNNING_IN_DOCKER": "0",
		"HAS_SYSTEMD":       "1",
		"HAS_SUDO_OR_ROOT":  "1",
	} {
		if got := result.Checks[key]; got != want {
			t.Fatalf("check %s = %q, want %q; all checks=%v", key, got, want, result.Checks)
		}
	}
}

func TestLocalCommandExecutorRunScriptUsesSudoAndRemovesTempScript(t *testing.T) {
	tempDir := t.TempDir()
	var scriptPath string
	var sawScript bool
	var sawExecutable bool
	executor := LocalCommandExecutor{
		RuntimeGOOS: "linux",
		TempDir:     tempDir,
		EffectiveUID: func() int {
			return 1000
		},
		CommandRunner: func(ctx context.Context, name string, args ...string) (string, error) {
			if name != "sudo" {
				t.Fatalf("command name = %q, want sudo", name)
			}
			if len(args) != 3 || args[0] != "-n" || args[1] != "bash" {
				t.Fatalf("args = %v, want [-n bash script]", args)
			}
			scriptPath = args[2]
			info, err := os.Stat(scriptPath)
			if err != nil {
				t.Fatalf("script should exist during execution: %v", err)
			}
			sawScript = true
			sawExecutable = runtime.GOOS == "windows" || info.Mode().Perm() == 0o700
			return "ok", nil
		},
	}
	out, err := executor.RunScript(context.Background(), "echo local setup")
	if err != nil {
		t.Fatalf("RunScript: %v", err)
	}
	if strings.TrimSpace(out) != "ok" {
		t.Fatalf("output = %q", out)
	}
	if !sawScript || !sawExecutable {
		t.Fatalf("script existed=%v executable0700=%v", sawScript, sawExecutable)
	}
	if _, err := os.Stat(scriptPath); !os.IsNotExist(err) {
		t.Fatalf("temporary script should be removed, stat err=%v", err)
	}
}
