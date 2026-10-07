package tools

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/sandbox"
)

const dockerEnvTestHost = "tcp://docker-proxy:2375"

// setDockerClientEnvForTest sets the Docker client variables a Docker
// deployment hands to the AuraGo container and blanks the remaining ones so
// the developer's own Docker setup cannot change the expected output.
func setDockerClientEnvForTest(t *testing.T) {
	t.Helper()
	t.Setenv("DOCKER_HOST", dockerEnvTestHost)
	t.Setenv("DOCKER_TLS_VERIFY", "1")
	t.Setenv("DOCKER_CERT_PATH", filepath.Join(t.TempDir(), "certs"))
	t.Setenv("DOCKER_CONFIG", "")
	t.Setenv("DOCKER_CONTEXT", "")
	t.Setenv("DOCKER_API_VERSION", "")
}

// configureDockerPermissionForTest sets the Docker tool gate and restores the
// package default when the test ends.
func configureDockerPermissionForTest(t *testing.T, enabled bool) {
	t.Helper()
	perms := defaultRuntimePermissionsForTests()
	perms.DockerEnabled = enabled
	ConfigureRuntimePermissions(perms)
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })
}

func dockerClientEnvEntries(env []string) []string {
	var found []string
	for _, kv := range env {
		name := kv
		if idx := strings.IndexByte(kv, '='); idx >= 0 {
			name = kv[:idx]
		}
		if dockerClientEnvNames[strings.ToUpper(name)] {
			found = append(found, kv)
		}
	}
	return found
}

func TestFilteredShellEnvDropsDockerClientVarsWithoutDockerPermission(t *testing.T) {
	setDockerClientEnvForTest(t)
	configureDockerPermissionForTest(t, false)

	if leaked := dockerClientEnvEntries(filteredShellEnv()); len(leaked) > 0 {
		t.Fatalf("docker client env leaked into the shell environment: %v", leaked)
	}

	configureDockerPermissionForTest(t, true)
	found := false
	for _, kv := range filteredShellEnv() {
		if kv == "DOCKER_HOST="+dockerEnvTestHost {
			found = true
		}
	}
	if !found {
		t.Fatal("DOCKER_HOST must stay visible to shells when the Docker tool is permitted")
	}
}

func TestFilteredShellEnvDropsDockerClientVarsWhenPermissionsAreUnconfigured(t *testing.T) {
	setDockerClientEnvForTest(t)
	ClearRuntimePermissionsForTest()
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })

	if leaked := dockerClientEnvEntries(filteredShellEnv()); len(leaked) > 0 {
		t.Fatalf("docker client env must stay hidden without a permission snapshot: %v", leaked)
	}
}

func TestFilteredShellEnvKeepsSecretFilteringInBothModes(t *testing.T) {
	setSensitiveEnvForSubprocessTest(t)
	setDockerClientEnvForTest(t)
	for _, enabled := range []bool{false, true} {
		configureDockerPermissionForTest(t, enabled)
		for _, kv := range filteredShellEnv() {
			if strings.Contains(kv, "should-not-leak") || strings.Contains(kv, strings.Repeat("a", 64)) {
				t.Fatalf("docker=%v: shell environment kept a secret: %q", enabled, kv)
			}
		}
	}
}

func TestWithoutDockerClientEnvMatchesNamesCaseInsensitively(t *testing.T) {
	input := []string{
		"PATH=/usr/bin",
		"docker_host=tcp://lower:2375",
		"Docker_Cert_Path=/certs",
		"DOCKER_TLS_VERIFY=1",
		"DOCKER_CONFIG=/cfg",
		"DOCKER_CONTEXT=remote",
		"DOCKER_API_VERSION=1.41",
		"DOCKER_HOSTNAME_HINT=kept",
		"MY_DOCKER_HOST=kept",
		"NOEQUALS",
	}
	got := withoutDockerClientEnv(input)
	want := []string{"PATH=/usr/bin", "DOCKER_HOSTNAME_HINT=kept", "MY_DOCKER_HOST=kept", "NOEQUALS"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("withoutDockerClientEnv() = %q, want %q", got, want)
	}
	if len(input) != 10 || input[1] != "docker_host=tcp://lower:2375" {
		t.Fatalf("withoutDockerClientEnv() must not modify its input, got %q", input)
	}
}

// Agent-controlled shells (shell.go: ExecuteShell, ExecuteShellBackground,
// ExecuteSudo) and host Python (python.go) must stay on the shell env; the
// integration env from ensureFilteredEnv or a direct sandbox.FilterEnv would
// hand them the Docker endpoint again.
func TestShellAndPythonEntryPointsDoNotUseIntegrationEnv(t *testing.T) {
	for _, path := range []string{"shell.go", "python.go"} {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		shellEnvCalls := 0
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.Ident:
				if node.Name == "ensureFilteredEnv" {
					t.Errorf("%s: %s must use ensureFilteredShellEnv, not ensureFilteredEnv", fset.Position(node.Pos()), path)
				}
			case *ast.SelectorExpr:
				if pkg, ok := node.X.(*ast.Ident); ok && pkg.Name == "sandbox" && node.Sel.Name == "FilterEnv" {
					t.Errorf("%s: %s must use ensureFilteredShellEnv, not sandbox.FilterEnv", fset.Position(node.Pos()), path)
				}
			case *ast.CallExpr:
				if fn, ok := node.Fun.(*ast.Ident); ok && fn.Name == "ensureFilteredShellEnv" {
					shellEnvCalls++
				}
			}
			return true
		})
		if shellEnvCalls == 0 {
			t.Errorf("%s no longer calls ensureFilteredShellEnv; update this guard together with its entry points", path)
		}
	}
}

func TestEnsureFilteredShellEnvDoesNotOverrideCallerEnv(t *testing.T) {
	setDockerClientEnvForTest(t)
	configureDockerPermissionForTest(t, false)

	ensureFilteredShellEnv(nil)

	preset := exec.Command("unused")
	preset.Env = []string{"DOCKER_HOST=tcp://caller-chosen:2375", "ONLY=this"}
	ensureFilteredShellEnv(preset)
	if strings.Join(preset.Env, "\n") != "DOCKER_HOST=tcp://caller-chosen:2375\nONLY=this" {
		t.Fatalf("ensureFilteredShellEnv overrode a caller-set Env: %q", preset.Env)
	}

	empty := exec.Command("unused")
	empty.Env = []string{}
	ensureFilteredShellEnv(empty)
	if empty.Env == nil || len(empty.Env) != 0 {
		t.Fatalf("ensureFilteredShellEnv replaced an explicitly empty Env: %q", empty.Env)
	}

	inherited := exec.Command("unused")
	ensureFilteredShellEnv(inherited)
	if inherited.Env == nil {
		t.Fatal("ensureFilteredShellEnv must set an explicit environment")
	}
	if leaked := dockerClientEnvEntries(inherited.Env); len(leaked) > 0 {
		t.Fatalf("ensureFilteredShellEnv kept docker client env without permission: %v", leaked)
	}
}

// Integrations that build images or drive Docker themselves keep the Docker
// endpoint: ensureFilteredEnv (Ansible local runs, MCP stdio, SanoTTS) and the
// FilterEnv-based image builds stay independent of the shell filter.
func TestEnsureFilteredEnvKeepsDockerClientVarsForIntegrations(t *testing.T) {
	setDockerClientEnvForTest(t)
	configureDockerPermissionForTest(t, false)

	cmd := exec.Command("unused")
	ensureFilteredEnv(cmd)
	if !containsEnvEntry(cmd.Env, "DOCKER_HOST="+dockerEnvTestHost) {
		t.Fatalf("ensureFilteredEnv dropped DOCKER_HOST: %v", dockerClientEnvEntries(cmd.Env))
	}
}

func TestSpaceAgentCommandKeepsDockerHost(t *testing.T) {
	setDockerClientEnvForTest(t)
	configureDockerPermissionForTest(t, false)
	t.Setenv("AURAGO_FAKE_PYTHON", "1")
	t.Setenv("AURAGO_FAKE_PYTHON_REPORT_DOCKER", "1")
	dir := t.TempDir()
	fake := filepath.Join(t.TempDir(), "fake-docker"+executableSuffixForTest())
	installFakeExecutable(t, fake)
	logger := &recordingSpaceAgentLogger{}

	if err := runSpaceAgentCommand(logger, dir, fake); err != nil {
		t.Fatalf("runSpaceAgentCommand() error = %v", err)
	}
	// runSpaceAgentCommand adds its own DOCKER_CONFIG, so only the inherited
	// endpoint variables are asserted.
	if output := logger.joined(); !strings.Contains(output, "DOCKER_HOST,DOCKER_TLS_VERIFY") {
		t.Fatalf("space agent command must keep the Docker endpoint, logged output = %q", output)
	}
}

func TestExecuteShellHidesDockerClientEnvWithoutDockerPermission(t *testing.T) {
	skipIfWindowsPowerShellUnavailable(t)
	t.Cleanup(sandbox.SetForTest(&sandbox.FallbackSandbox{}))
	setDockerClientEnvForTest(t)

	configureDockerPermissionForTest(t, false)
	stdout, stderr, err := ExecuteShell(shellPrintDockerEnvCommand(), t.TempDir())
	if err != nil {
		t.Fatalf("ExecuteShell() error = %v, stderr = %q", err, stderr)
	}
	if !strings.Contains(stdout, "env-check-done") {
		t.Fatalf("shell command did not run to completion: %q", stdout)
	}
	if strings.Contains(stdout, dockerEnvTestHost) || strings.Contains(stdout, "docker-tls=1") {
		t.Fatalf("shell saw the Docker client env without the Docker permission: %q", stdout)
	}

	configureDockerPermissionForTest(t, true)
	stdout, stderr, err = ExecuteShell(shellPrintDockerEnvCommand(), t.TempDir())
	if err != nil {
		t.Fatalf("ExecuteShell() with Docker permission error = %v, stderr = %q", err, stderr)
	}
	if !strings.Contains(stdout, "docker-host="+dockerEnvTestHost) {
		t.Fatalf("shell must keep DOCKER_HOST when the Docker tool is permitted: %q", stdout)
	}
}

func TestExecuteShellBackgroundHidesDockerClientEnvWithoutDockerPermission(t *testing.T) {
	skipIfWindowsPowerShellUnavailable(t)
	t.Cleanup(sandbox.SetForTest(&sandbox.FallbackSandbox{}))
	setDockerClientEnvForTest(t)
	configureDockerPermissionForTest(t, false)
	registry := NewProcessRegistry(slog.Default())

	pid, err := ExecuteShellBackground(shellPrintDockerEnvCommand(), t.TempDir(), registry)
	if err != nil {
		t.Fatalf("ExecuteShellBackground() error = %v", err)
	}
	defer registry.Terminate(pid)

	output := waitForProcessOutputContaining(t, registry, pid, "env-check-done")
	if strings.Contains(output, dockerEnvTestHost) || strings.Contains(output, "docker-tls=1") {
		t.Fatalf("background shell saw the Docker client env without the Docker permission: %q", output)
	}
}

func TestHostPythonHidesDockerClientEnvWithoutDockerPermission(t *testing.T) {
	const secretValue = "probe-secret-value-7f3a9c2e5b81"
	secrets := map[string]string{"probe": secretValue}
	creds := []CredentialFields{{Name: "PROBE", Fields: map[string]string{"token": "probe-credential-token-4c8e1b6a92"}}}
	// These runs inject AURAGO_SECRET_* / AURAGO_CRED_* after the shell filter;
	// the child must see them while DOCKER_HOST stays hidden.
	injecting := map[string]bool{
		"ExecutePythonWithOptions":           true,
		"ExecutePythonWithSecrets":           true,
		"RunToolWithSecrets":                 true,
		"ExecutePythonBackgroundWithSecrets": true,
		"RunToolBackgroundWithSecrets":       true,
	}
	runs := []struct {
		name string
		run  func(t *testing.T, workspaceDir, toolsDir string) string
	}{
		{"ExecutePython", func(t *testing.T, workspaceDir, toolsDir string) string {
			stdout, stderr, err := ExecutePython(`print("unused")`, workspaceDir, toolsDir)
			return requireRunOutput(t, stdout, stderr, err)
		}},
		{"ExecutePythonWithOptions", func(t *testing.T, workspaceDir, toolsDir string) string {
			stdout, stderr, err := ExecutePythonWithOptions(PythonExecutionOptions{
				Code:         `print("unused")`,
				WorkspaceDir: workspaceDir,
				ToolsDir:     toolsDir,
				Secrets:      secrets,
				Credentials:  creds,
			})
			return requireRunOutput(t, stdout, stderr, err)
		}},
		{"ExecutePythonWithSecrets", func(t *testing.T, workspaceDir, toolsDir string) string {
			stdout, stderr, err := ExecutePythonWithSecrets(`print("unused")`, workspaceDir, toolsDir, secrets, creds)
			return requireRunOutput(t, stdout, stderr, err)
		}},
		{"RunTool", func(t *testing.T, workspaceDir, toolsDir string) string {
			writeFakeToolForTest(t, toolsDir)
			stdout, stderr, err := RunTool("tool.py", nil, workspaceDir, toolsDir)
			return requireRunOutput(t, stdout, stderr, err)
		}},
		{"RunToolWithSecrets", func(t *testing.T, workspaceDir, toolsDir string) string {
			writeFakeToolForTest(t, toolsDir)
			stdout, stderr, err := RunToolWithSecrets("tool.py", nil, workspaceDir, toolsDir, secrets, creds)
			return requireRunOutput(t, stdout, stderr, err)
		}},
		{"InstallPackage", func(t *testing.T, workspaceDir, toolsDir string) string {
			stdout, stderr, err := InstallPackage("requests", workspaceDir)
			return requireRunOutput(t, stdout, stderr, err)
		}},
		{"ExecutePythonBackground", func(t *testing.T, workspaceDir, toolsDir string) string {
			registry := NewProcessRegistry(testBackgroundTaskLogger())
			pid, err := ExecutePythonBackground(`print("unused")`, workspaceDir, toolsDir, registry)
			if err != nil {
				t.Fatalf("start error = %v", err)
			}
			defer registry.Terminate(pid)
			return waitForProcessOutputContaining(t, registry, pid, "env-clean")
		}},
		{"ExecutePythonBackgroundWithSecrets", func(t *testing.T, workspaceDir, toolsDir string) string {
			registry := NewProcessRegistry(testBackgroundTaskLogger())
			pid, err := ExecutePythonBackgroundWithSecrets(`print("unused")`, workspaceDir, toolsDir, registry, secrets, creds)
			if err != nil {
				t.Fatalf("start error = %v", err)
			}
			defer registry.Terminate(pid)
			return waitForProcessOutputContaining(t, registry, pid, "env-clean")
		}},
		{"RunToolBackground", func(t *testing.T, workspaceDir, toolsDir string) string {
			writeFakeToolForTest(t, toolsDir)
			registry := NewProcessRegistry(testBackgroundTaskLogger())
			pid, err := RunToolBackground("tool.py", nil, workspaceDir, toolsDir, registry)
			if err != nil {
				t.Fatalf("start error = %v", err)
			}
			defer registry.Terminate(pid)
			return waitForProcessOutputContaining(t, registry, pid, "env-clean")
		}},
		{"RunToolBackgroundWithSecrets", func(t *testing.T, workspaceDir, toolsDir string) string {
			writeFakeToolForTest(t, toolsDir)
			registry := NewProcessRegistry(testBackgroundTaskLogger())
			pid, err := RunToolBackgroundWithSecrets("tool.py", nil, workspaceDir, toolsDir, registry, secrets, creds)
			if err != nil {
				t.Fatalf("start error = %v", err)
			}
			defer registry.Terminate(pid)
			return waitForProcessOutputContaining(t, registry, pid, "env-clean")
		}},
	}

	setDockerClientEnvForTest(t)
	t.Setenv("AURAGO_FAKE_PYTHON", "1")
	t.Setenv("AURAGO_FAKE_PYTHON_REPORT_DOCKER", "1")
	for _, tc := range runs {
		t.Run(tc.name, func(t *testing.T) {
			workspaceDir := t.TempDir()
			toolsDir := t.TempDir()
			installFakePython(t, workspaceDir)

			wantInjected := "injected-env:none"
			if injecting[tc.name] {
				wantInjected = "injected-env:AURAGO_CRED_PROBE_TOKEN,AURAGO_SECRET_PROBE"
			}

			configureDockerPermissionForTest(t, false)
			output := tc.run(t, workspaceDir, toolsDir)
			if !strings.Contains(output, "docker-env:none") {
				t.Fatalf("host Python saw the Docker client env without the Docker permission: %q", output)
			}
			if !strings.Contains(output, wantInjected) {
				t.Fatalf("host Python child env with the Docker filter active: want %q in %q", wantInjected, output)
			}

			configureDockerPermissionForTest(t, true)
			output = tc.run(t, workspaceDir, toolsDir)
			if !strings.Contains(output, "docker-env:DOCKER_CERT_PATH,DOCKER_HOST,DOCKER_TLS_VERIFY") {
				t.Fatalf("host Python must keep the Docker client env when the Docker tool is permitted: %q", output)
			}
			if !strings.Contains(output, wantInjected) {
				t.Fatalf("host Python child env with the Docker tool permitted: want %q in %q", wantInjected, output)
			}
		})
	}
}

// reportDockerClientEnvForFake lists the Docker client variables with a value
// in the fake subprocess's environment (see runFakePythonForEnvTests).
func reportDockerClientEnvForFake() string {
	var names []string
	for _, kv := range os.Environ() {
		name, value, _ := strings.Cut(kv, "=")
		if value != "" && dockerClientEnvNames[strings.ToUpper(name)] {
			names = append(names, strings.ToUpper(name))
		}
	}
	if len(names) == 0 {
		return "none"
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

// reportInjectedEnvForFake lists the AURAGO_SECRET_* and AURAGO_CRED_*
// variables InjectSecretsEnv and InjectCredentialEnv gave the fake subprocess.
func reportInjectedEnvForFake() string {
	var names []string
	for _, kv := range os.Environ() {
		name, value, _ := strings.Cut(kv, "=")
		upper := strings.ToUpper(name)
		if value != "" && (strings.HasPrefix(upper, "AURAGO_SECRET_") || strings.HasPrefix(upper, "AURAGO_CRED_")) {
			names = append(names, upper)
		}
	}
	if len(names) == 0 {
		return "none"
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

func shellPrintDockerEnvCommand() string {
	if runtime.GOOS == "windows" {
		return `Write-Output "docker-host=$env:DOCKER_HOST"; Write-Output "docker-tls=$env:DOCKER_TLS_VERIFY"; Write-Output env-check-done`
	}
	return `printf 'docker-host=%s\ndocker-tls=%s\nenv-check-done\n' "$DOCKER_HOST" "$DOCKER_TLS_VERIFY"`
}

func requireRunOutput(t *testing.T, stdout, stderr string, err error) string {
	t.Helper()
	if err != nil {
		t.Fatalf("run error = %v, stdout = %q, stderr = %q", err, stdout, stderr)
	}
	return stdout
}

func writeFakeToolForTest(t *testing.T, toolsDir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(toolsDir, "tool.py"), []byte(`print("unused")`), 0o600); err != nil {
		t.Fatalf("write fake tool: %v", err)
	}
}

func executableSuffixForTest() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

func containsEnvEntry(env []string, entry string) bool {
	for _, kv := range env {
		if kv == entry {
			return true
		}
	}
	return false
}

func waitForProcessOutputContaining(t *testing.T, registry *ProcessRegistry, pid int, marker string) string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	var last string
	for time.Now().Before(deadline) {
		if info, ok := registry.Get(pid); ok {
			last = info.ReadOutput()
			if strings.Contains(last, marker) {
				return last
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %q in process %d output, last=%q", marker, pid, last)
	return ""
}

type recordingSpaceAgentLogger struct {
	mu    sync.Mutex
	lines []string
}

func (l *recordingSpaceAgentLogger) record(msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, msg+" "+fmt.Sprint(args...))
}

func (l *recordingSpaceAgentLogger) Info(msg string, args ...any)  { l.record(msg, args...) }
func (l *recordingSpaceAgentLogger) Warn(msg string, args ...any)  { l.record(msg, args...) }
func (l *recordingSpaceAgentLogger) Error(msg string, args ...any) { l.record(msg, args...) }

func (l *recordingSpaceAgentLogger) joined() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Join(l.lines, "\n")
}
