package tools

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
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

// The Landlock sandbox gets its Docker endpoint from the ExtraEnv slice built
// in cmd/aurago/main.go. The unsandboxed shell filter must hide every name that
// slice injects, under the same gate: main.go appends only inside
// `if cfg.Docker.Enabled`, and RuntimePermissionsFromConfig copies that flag
// into the DockerEnabled field requireDockerPermission reads.
func TestDockerClientEnvNamesCoverTheLandlockExtraEnvVariable(t *testing.T) {
	mainPath := filepath.Join("..", "..", "cmd", "aurago", "main.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, mainPath, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", mainPath, err)
	}
	scan := scanLandlockExtraEnv(fset, file)
	for _, problem := range scan.problems {
		t.Error(problem)
	}
	if scan.extraEnvFields != 1 {
		t.Fatalf("expected exactly one ExtraEnv field in %s, found %d", mainPath, scan.extraEnvFields)
	}
	for _, name := range scan.injected {
		if !dockerClientEnvNames[strings.ToUpper(name)] {
			t.Errorf("main.go injects %s into the Landlock sandbox; add it to dockerClientEnvNames so unsandboxed shells hide it under the same gate", name)
		}
	}
	if strings.Join(scan.injected, ",") != "DOCKER_HOST" {
		t.Fatalf("Landlock ExtraEnv injects %v; expected only DOCKER_HOST (update dockerClientEnvNames and this test together)", scan.injected)
	}

	cfg := &config.Config{}
	for _, enabled := range []bool{false, true} {
		cfg.Docker.Enabled = enabled
		if got := RuntimePermissionsFromConfig(cfg).DockerEnabled; got != enabled {
			t.Fatalf("RuntimePermissionsFromConfig(docker.enabled=%v).DockerEnabled = %v; the shell filter gate must follow cfg.Docker.Enabled", enabled, got)
		}
	}
}

// The scanner above must flag what it guards against; checked on synthetic
// sources so cmd/aurago/main.go itself is never weakened for the check.
func TestScanLandlockExtraEnvFlagsUngatedAndUnreadableInjections(t *testing.T) {
	cases := []struct {
		name         string
		body         string
		wantInjected string
		wantProblem  string
	}{
		{"gated", `if cfg.Docker.Enabled { extraEnv = append(extraEnv, "DOCKER_HOST="+host) }`, "DOCKER_HOST", ""},
		{"other name is reported", `if cfg.Docker.Enabled { extraEnv = append(extraEnv, "DOCKER_TLS_VERIFY=1") }`, "DOCKER_TLS_VERIFY", ""},
		{"ungated", `extraEnv = append(extraEnv, "DOCKER_HOST="+host)`, "DOCKER_HOST", "inside `if cfg.Docker.Enabled`"},
		{"else branch", `if cfg.Docker.Enabled { } else { extraEnv = append(extraEnv, "DOCKER_HOST="+host) }`, "DOCKER_HOST", "inside `if cfg.Docker.Enabled`"},
		{"other gate", `if cfg.Agent.AllowShell { extraEnv = append(extraEnv, "DOCKER_HOST="+host) }`, "DOCKER_HOST", "inside `if cfg.Docker.Enabled`"},
		{"computed name", `if cfg.Docker.Enabled { extraEnv = append(extraEnv, name+"="+host) }`, "", "string literal"},
		{"replaced slice", `if cfg.Docker.Enabled { extraEnv = other }`, "", "only grow through append"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := "package p\nfunc f() {\nvar extraEnv []string\n" + tc.body + "\nInit(Config{ExtraEnv: extraEnv})\n}\n"
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "synthetic.go", src, 0)
			if err != nil {
				t.Fatalf("parse synthetic source: %v", err)
			}
			scan := scanLandlockExtraEnv(fset, file)
			if got := strings.Join(scan.injected, ","); got != tc.wantInjected {
				t.Fatalf("injected = %q, want %q", got, tc.wantInjected)
			}
			problems := strings.Join(scan.problems, "\n")
			if tc.wantProblem == "" && problems != "" {
				t.Fatalf("unexpected problems: %s", problems)
			}
			if tc.wantProblem != "" && !strings.Contains(problems, tc.wantProblem) {
				t.Fatalf("problems = %q, want one containing %q", problems, tc.wantProblem)
			}
		})
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "synthetic.go", "package p\nfunc f() { Init(Config{ExtraEnv: []string{\"DOCKER_HOST=x\"}}) }\n", 0)
	if err != nil {
		t.Fatalf("parse synthetic source: %v", err)
	}
	if scan := scanLandlockExtraEnv(fset, file); scan.extraEnvFields != 1 || !strings.Contains(strings.Join(scan.problems, "\n"), "must be the extraEnv slice") {
		t.Fatalf("an inline ExtraEnv value must be flagged, got %+v", scan)
	}
}

type landlockExtraEnvScan struct {
	injected       []string
	extraEnvFields int
	problems       []string
}

// scanLandlockExtraEnv collects the names appended to extraEnv, counts the
// ExtraEnv fields, and reports appends outside `if cfg.Docker.Enabled`, entries
// without a "NAME=" literal prefix, and ExtraEnv values other than extraEnv.
func scanLandlockExtraEnv(fset *token.FileSet, file *ast.File) landlockExtraEnvScan {
	var scan landlockExtraEnvScan
	var stack []ast.Node
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		if kv, ok := n.(*ast.KeyValueExpr); ok {
			if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "ExtraEnv" {
				scan.extraEnvFields++
				if value, ok := kv.Value.(*ast.Ident); !ok || value.Name != "extraEnv" {
					scan.problems = append(scan.problems, fmt.Sprintf("%s: ExtraEnv must be the extraEnv slice, got %s", fset.Position(kv.Pos()), types.ExprString(kv.Value)))
				}
			}
			return true
		}
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return true
		}
		if target, ok := assign.Lhs[0].(*ast.Ident); !ok || target.Name != "extraEnv" {
			return true
		}
		position := fset.Position(assign.Pos())
		call, isCall := assign.Rhs[0].(*ast.CallExpr)
		if !isCall {
			scan.problems = append(scan.problems, fmt.Sprintf("%s: extraEnv may only grow through append(extraEnv, \"NAME=\"+value)", position))
			return true
		}
		if fn, isIdent := call.Fun.(*ast.Ident); !isIdent || fn.Name != "append" || len(call.Args) < 2 {
			scan.problems = append(scan.problems, fmt.Sprintf("%s: extraEnv may only grow through append(extraEnv, \"NAME=\"+value)", position))
			return true
		}
		for _, arg := range call.Args[1:] {
			literal, ok := leftmostStringLiteral(arg)
			name, _, hasValue := strings.Cut(literal, "=")
			if !ok || !hasValue || name == "" {
				scan.problems = append(scan.problems, fmt.Sprintf("%s: extraEnv entry %s must start with a \"NAME=\" string literal", position, types.ExprString(arg)))
				continue
			}
			scan.injected = append(scan.injected, name)
		}
		if !insideIfBody(stack, assign, "cfg.Docker.Enabled") {
			scan.problems = append(scan.problems, fmt.Sprintf("%s: extraEnv must only be extended inside `if cfg.Docker.Enabled`", position))
		}
		return true
	})
	return scan
}

// leftmostStringLiteral returns the unquoted string literal that starts expr,
// following "literal" + value concatenations.
func leftmostStringLiteral(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return "", false
		}
		value, err := strconv.Unquote(e.Value)
		return value, err == nil
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return "", false
		}
		return leftmostStringLiteral(e.X)
	case *ast.ParenExpr:
		return leftmostStringLiteral(e.X)
	}
	return "", false
}

// insideIfBody reports whether node sits in the then-branch of an enclosing
// if statement whose condition renders as cond.
func insideIfBody(stack []ast.Node, node ast.Node, cond string) bool {
	for _, ancestor := range stack {
		ifStmt, ok := ancestor.(*ast.IfStmt)
		if !ok || types.ExprString(ifStmt.Cond) != cond {
			continue
		}
		if node.Pos() >= ifStmt.Body.Pos() && node.End() <= ifStmt.Body.End() {
			return true
		}
	}
	return false
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
	const secretValue = "d11-secret-value-7f3a9c2e5b81"
	secrets := map[string]string{"d11": secretValue}
	creds := []CredentialFields{{Name: "D11", Fields: map[string]string{"token": "d11-credential-token-4c8e1b6a92"}}}
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
				wantInjected = "injected-env:AURAGO_CRED_D11_TOKEN,AURAGO_SECRET_D11"
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
