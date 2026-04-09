package tools

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

// Timeout categories managed by the central timeout configuration.
// All execution timeouts flow through these categories to ensure consistency.
const (
	// DefaultForegroundTimeout is the default timeout for foreground executions (shell, python scripts).
	DefaultForegroundTimeout = 30 * time.Second
	// DefaultSkillTimeout is the default timeout for skill invocations.
	DefaultSkillTimeout = 120 * time.Second
	// DefaultBackgroundTimeout is the default timeout for background processes.
	DefaultBackgroundTimeout = 1 * time.Hour
	// DefaultSandboxTimeout is the default timeout for sandbox code execution.
	DefaultSandboxTimeout = 30 * time.Second
	// DefaultNetworkTimeout is the default timeout for network operations (MCP, HTTP, etc.).
	DefaultNetworkTimeout = 60 * time.Second
)

// TimeoutConfig holds the configured timeouts for all execution categories.
// Use GetTimeoutConfig() to get the current configuration.
type TimeoutConfig struct {
	Foreground time.Duration // Python scripts, shell commands
	Skills     time.Duration // Skill invocations
	Background time.Duration // Background processes
	Sandbox    time.Duration // Sandboxed code execution
	Network    time.Duration // Network operations (MCP, HTTP calls)
}

// foregroundTimeout stores the default execution timeout for Python scripts and shell commands (nanoseconds).
var foregroundTimeout atomic.Int64

// skillTimeout stores the default execution timeout for skill invocations (nanoseconds).
var skillTimeout atomic.Int64

// backgroundTimeout stores the default execution timeout for background Python/shell/tool processes (nanoseconds).
var backgroundTimeout atomic.Int64

// sandboxTimeout stores the default execution timeout for sandbox code execution (nanoseconds).
var sandboxTimeout atomic.Int64

// networkTimeout stores the default execution timeout for network operations (nanoseconds).
var networkTimeout atomic.Int64

func init() {
	foregroundTimeout.Store(int64(DefaultForegroundTimeout))
	skillTimeout.Store(int64(DefaultSkillTimeout))
	backgroundTimeout.Store(int64(DefaultBackgroundTimeout))
	sandboxTimeout.Store(int64(DefaultSandboxTimeout))
	networkTimeout.Store(int64(DefaultNetworkTimeout))
}

// GetForegroundTimeout returns the current foreground execution timeout.
func GetForegroundTimeout() time.Duration {
	return time.Duration(foregroundTimeout.Load())
}

// SetForegroundTimeout sets the foreground execution timeout (for testing).
func SetForegroundTimeout(d time.Duration) {
	foregroundTimeout.Store(int64(d))
}

// GetSkillTimeout returns the current skill execution timeout.
func GetSkillTimeout() time.Duration {
	return time.Duration(skillTimeout.Load())
}

// SetSkillTimeout sets the skill execution timeout (for testing).
func SetSkillTimeout(d time.Duration) {
	skillTimeout.Store(int64(d))
}

// GetBackgroundTimeout returns the current background execution timeout.
func GetBackgroundTimeout() time.Duration {
	return time.Duration(backgroundTimeout.Load())
}

// SetBackgroundTimeout sets the background execution timeout (for testing).
func SetBackgroundTimeout(d time.Duration) {
	backgroundTimeout.Store(int64(d))
}

// GetSandboxTimeout returns the current sandbox execution timeout.
func GetSandboxTimeout() time.Duration {
	return time.Duration(sandboxTimeout.Load())
}

// SetSandboxTimeout sets the sandbox execution timeout (for testing).
func SetSandboxTimeout(d time.Duration) {
	sandboxTimeout.Store(int64(d))
}

// GetNetworkTimeout returns the current network operation timeout.
func GetNetworkTimeout() time.Duration {
	return time.Duration(networkTimeout.Load())
}

// SetNetworkTimeout sets the network operation timeout (for testing).
func SetNetworkTimeout(d time.Duration) {
	networkTimeout.Store(int64(d))
}

// GetTimeoutConfig returns the current timeout configuration as a struct.
// This provides a centralized view of all timeouts and their values.
func GetTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		Foreground: GetForegroundTimeout(),
		Skills:     GetSkillTimeout(),
		Background: GetBackgroundTimeout(),
		Sandbox:    GetSandboxTimeout(),
		Network:    GetNetworkTimeout(),
	}
}

// ConfigureTimeouts sets package-level timeouts from configuration.
// Values <= 0 are ignored (defaults are kept).
// This function now supports all timeout categories including Sandbox and Network.
func ConfigureTimeouts(pythonSeconds, skillSeconds, backgroundSeconds int) {
	if pythonSeconds > 0 {
		foregroundTimeout.Store(int64(time.Duration(pythonSeconds) * time.Second))
	}
	if skillSeconds > 0 {
		skillTimeout.Store(int64(time.Duration(skillSeconds) * time.Second))
	}
	if backgroundSeconds > 0 {
		backgroundTimeout.Store(int64(time.Duration(backgroundSeconds) * time.Second))
	}
}

// ConfigureAllTimeouts sets all timeout categories from a TimeoutConfig struct.
// Only values > 0 are applied; defaults are preserved for zero values.
func ConfigureAllTimeouts(cfg TimeoutConfig) {
	if cfg.Foreground > 0 {
		foregroundTimeout.Store(int64(cfg.Foreground))
	}
	if cfg.Skills > 0 {
		skillTimeout.Store(int64(cfg.Skills))
	}
	if cfg.Background > 0 {
		backgroundTimeout.Store(int64(cfg.Background))
	}
	if cfg.Sandbox > 0 {
		sandboxTimeout.Store(int64(cfg.Sandbox))
	}
	if cfg.Network > 0 {
		networkTimeout.Store(int64(cfg.Network))
	}
}

// getAbsWorkspace ensures that working directories are absolute. Passing a relative path
// to cmd.Dir can cause OS executors to evaluate the CWD incorrectly or default to the binary's dir.
func getAbsWorkspace(workspaceDir string) string {
	if abs, err := filepath.Abs(workspaceDir); err == nil {
		return abs
	}
	return workspaceDir
}

// GetPythonBin returns the absolute path to the Python executable inside the isolated virtual environment.
func GetPythonBin(workspaceDir string) string {
	var binPath string
	if runtime.GOOS == "windows" {
		binPath = filepath.Join(workspaceDir, "venv", "Scripts", "python.exe")
	} else {
		binPath = filepath.Join(workspaceDir, "venv", "bin", "python")
	}
	if abs, err := filepath.Abs(binPath); err == nil {
		return abs
	}
	return binPath
}

// GetPipBin returns the absolute path to the pip executable inside the isolated virtual environment.
func GetPipBin(workspaceDir string) string {
	var binPath string
	if runtime.GOOS == "windows" {
		binPath = filepath.Join(workspaceDir, "venv", "Scripts", "pip.exe")
	} else {
		binPath = filepath.Join(workspaceDir, "venv", "bin", "pip")
	}
	if abs, err := filepath.Abs(binPath); err == nil {
		return abs
	}
	return binPath
}

// EnsureVenv checks if the virtual environment exists and has a working pip binary, creating or recreating it if necessary.
func EnsureVenv(workspaceDir string, logger *slog.Logger) error {
	venvDir := filepath.Join(workspaceDir, "venv")

	// Determine the pip binary path to validate
	var pipBin string
	if runtime.GOOS == "windows" {
		pipBin = filepath.Join(venvDir, "Scripts", "pip.exe")
	} else {
		pipBin = filepath.Join(venvDir, "bin", "pip")
	}

	// If venv exists AND pip binary is present, we're good
	if _, err := os.Stat(pipBin); err == nil {
		return nil
	}

	// Either venv dir is missing or pip binary is absent (incomplete/corrupt venv)
	if _, err := os.Stat(venvDir); err == nil {
		logger.Warn("Python venv exists but pip binary is missing — recreating venv", "dir", venvDir, "pip", pipBin)
		if err := os.RemoveAll(venvDir); err != nil {
			return fmt.Errorf("failed to remove broken venv: %w", err)
		}
	} else {
		logger.Info("Creating Python virtual environment", "dir", venvDir)
	}

	return createVenv(workspaceDir, logger)
}

// createVenv creates a new virtual environment in workspaceDir using python3 or python.
func createVenv(workspaceDir string, logger *slog.Logger) error {
	candidates := []string{"python3", "python"}
	if runtime.GOOS == "windows" {
		candidates = []string{"python", "python3"}
	}

	var lastErr error
	for _, pyCmd := range candidates {
		cmd := exec.Command(pyCmd, "-m", "venv", "venv")
		cmd.Dir = workspaceDir
		if out, err := cmd.CombinedOutput(); err == nil {
			logger.Info("Python virtual environment created", "python", pyCmd)
			return nil
		} else {
			logger.Debug("venv creation attempt failed", "python", pyCmd, "error", err, "output", string(out))
			lastErr = fmt.Errorf("%s: %w (output: %s)", pyCmd, err, string(out))
		}
	}
	return fmt.Errorf("failed to create venv: %w", lastErr)
}

// validPackageName matches pip-safe package name specifiers.
// Allows: name, name[extra], name>=1.0, name==1.0.0, etc.
// Blocks: paths, flags (--index-url), shell metacharacters.
var validPackageName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._\-]*(\[[\w,\s]+\])?([\s]*(==|!=|<=|>=|<|>|~=)[^\s;]+)?$`)

// InstallPackage installs a Python package using the virtual environment's pip.
// Has a generous 3-minute timeout for downloads and compilation.
func InstallPackage(pkgName, workspaceDir string) (string, string, error) {
	// Validate package name to prevent pip flag injection or path traversal.
	pkgName = strings.TrimSpace(pkgName)
	if !validPackageName.MatchString(pkgName) {
		return "", "", fmt.Errorf("invalid package name %q: must match pip package name format", pkgName)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	pipCmd := GetPipBin(workspaceDir)
	cmd := exec.CommandContext(ctx, pipCmd, "install", pkgName)
	cmd.Dir = getAbsWorkspace(workspaceDir)

	slog.Debug("[InstallPackage]", "cmd", pipCmd, "args", cmd.Args)

	stdout := NewBoundedBuffer(1024 * 1024)
	stderr := NewBoundedBuffer(1024 * 1024)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return stdout.String(), stderr.String(), fmt.Errorf("TIMEOUT: pip install '%s' exceeded 3-minute limit", pkgName)
	}
	return stdout.String(), stderr.String(), err
}

// resolveToolPath validates the tool name and returns its absolute path within toolsDir.
// Returns an error if name contains path separators, traversal sequences, or if the tool does not exist.
func resolveToolPath(name, toolsDir string) (string, error) {
	if name == "" || strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid tool name: must be a simple filename without path separators")
	}
	toolPath := filepath.Join(toolsDir, name)
	if _, err := os.Stat(toolPath); os.IsNotExist(err) {
		return "", fmt.Errorf("tool '%s' not found in %s", name, toolsDir)
	}
	abs, err := filepath.Abs(toolPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve tool path: %w", err)
	}
	return abs, nil
}

// RunTool executes a saved tool from the tools directory with arguments (foreground, 30s timeout).
// Path traversal is blocked — name must resolve within toolsDir.
func RunTool(name string, args []string, workspaceDir, toolsDir string) (string, string, error) {
	absToolPath, err := resolveToolPath(name, toolsDir)
	if err != nil {
		return "", "", err
	}

	pythonCmd := GetPythonBin(workspaceDir)
	cmdArgs := append([]string{absToolPath}, args...)
	cmd := exec.Command(pythonCmd, cmdArgs...)
	cmd.Dir = getAbsWorkspace(workspaceDir)
	SetupCmd(cmd)

	slog.Debug("[RunTool]", "cmd", pythonCmd, "args", cmd.Args)

	stdout := NewBoundedBuffer(1024 * 1024)
	stderr := NewBoundedBuffer(1024 * 1024)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return "", "", err
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	timer := time.NewTimer(GetForegroundTimeout())
	defer timer.Stop()

	select {
	case err := <-done:
		return stdout.String(), stderr.String(), err
	case <-timer.C:
		KillProcessTree(cmd.Process.Pid)
		select {
		case <-done:
		case <-time.After(10 * time.Second):
		}
		return stdout.String(), stderr.String(), fmt.Errorf("TIMEOUT: tool '%s' exceeded %s limit and was killed", name, GetForegroundTimeout())
	}
}

// RunToolWithSecrets is like RunTool but injects vault secrets and credential secrets
// as environment variables and scrubs secrets from the output.
func RunToolWithSecrets(name string, args []string, workspaceDir, toolsDir string, secrets map[string]string, creds []CredentialFields) (string, string, error) {
	absToolPath, err := resolveToolPath(name, toolsDir)
	if err != nil {
		return "", "", err
	}

	pythonCmd := GetPythonBin(workspaceDir)
	cmdArgs := append([]string{absToolPath}, args...)
	cmd := exec.Command(pythonCmd, cmdArgs...)
	cmd.Dir = getAbsWorkspace(workspaceDir)
	SetupCmd(cmd)
	InjectSecretsEnv(cmd, secrets)
	InjectCredentialEnv(cmd, creds)

	stdout := NewBoundedBuffer(1024 * 1024)
	stderr := NewBoundedBuffer(1024 * 1024)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return "", "", err
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	timer := time.NewTimer(GetForegroundTimeout())
	defer timer.Stop()

	select {
	case err := <-done:
		so, se := ScrubSecretOutput(stdout.String(), stderr.String())
		return so, se, err
	case <-timer.C:
		KillProcessTree(cmd.Process.Pid)
		select {
		case <-done:
		case <-time.After(10 * time.Second):
		}
		so, se := ScrubSecretOutput(stdout.String(), stderr.String())
		return so, se, fmt.Errorf("TIMEOUT: tool '%s' exceeded %s limit and was killed", name, GetForegroundTimeout())
	}
}

// RunToolBackground starts a saved tool in the background and registers it in the process registry.
func RunToolBackground(name string, args []string, workspaceDir, toolsDir string, registry *ProcessRegistry) (int, error) {
	absToolPath, err := resolveToolPath(name, toolsDir)
	if err != nil {
		return 0, err
	}

	pythonCmd := GetPythonBin(workspaceDir)
	cmdArgs := append([]string{absToolPath}, args...)
	cmd := exec.Command(pythonCmd, cmdArgs...)
	cmd.Dir = getAbsWorkspace(workspaceDir)

	slog.Debug("[RunToolBackground]", "cmd", pythonCmd, "args", cmd.Args)

	pid, err := registerManagedBackgroundProcess(cmd, registry, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to start background tool: %w", err)
	}
	return pid, nil
}

// RunToolBackgroundWithSecrets is like RunToolBackground but injects vault secrets
// and credential secrets as environment variables. Output scrubbing happens at read_process_logs time.
func RunToolBackgroundWithSecrets(name string, args []string, workspaceDir, toolsDir string, registry *ProcessRegistry, secrets map[string]string, creds []CredentialFields) (int, error) {
	absToolPath, err := resolveToolPath(name, toolsDir)
	if err != nil {
		return 0, err
	}

	pythonCmd := GetPythonBin(workspaceDir)
	cmdArgs := append([]string{absToolPath}, args...)
	cmd := exec.Command(pythonCmd, cmdArgs...)
	cmd.Dir = getAbsWorkspace(workspaceDir)
	InjectSecretsEnv(cmd, secrets)
	InjectCredentialEnv(cmd, creds)

	pid, err := registerManagedBackgroundProcess(cmd, registry, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to start background tool: %w", err)
	}
	return pid, nil
}

// ExecutePython saves the provided Python code to a temporary file,
// executes it within the sandbox workspace with a 30-second timeout,
// and returns stdout and stderr.
// Uses KillProcessTree on timeout so any subprocesses spawned by the script
// (e.g., via subprocess.Popen) are also terminated and the pipes are closed.
func ExecutePython(code, workspaceDir, toolsDir string) (string, string, error) {
	scriptPath, cleanup, err := writeScript(code, toolsDir)
	if err != nil {
		return "", "", err
	}
	defer cleanup()

	pythonCmd := GetPythonBin(workspaceDir)
	cmd := exec.Command(pythonCmd, scriptPath)
	cmd.Dir = getAbsWorkspace(workspaceDir)
	SetupCmd(cmd)

	stdout := NewBoundedBuffer(1024 * 1024)
	stderr := NewBoundedBuffer(1024 * 1024)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return "", "", err
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	timer := time.NewTimer(GetForegroundTimeout())
	defer timer.Stop()

	select {
	case err := <-done:
		return stdout.String(), stderr.String(), err
	case <-timer.C:
		KillProcessTree(cmd.Process.Pid)
		select {
		case <-done:
		case <-time.After(10 * time.Second):
		}
		return stdout.String(), stderr.String(), fmt.Errorf("TIMEOUT: script exceeded %s limit and was killed", GetForegroundTimeout())
	}
}

// ExecutePythonWithSecrets is like ExecutePython but injects vault secrets and credential secrets
// as environment variables and scrubs secrets from the output.
func ExecutePythonWithSecrets(code, workspaceDir, toolsDir string, secrets map[string]string, creds []CredentialFields) (string, string, error) {
	scriptPath, cleanup, err := writeScript(code, toolsDir)
	if err != nil {
		return "", "", err
	}
	defer cleanup()

	pythonCmd := GetPythonBin(workspaceDir)
	cmd := exec.Command(pythonCmd, scriptPath)
	cmd.Dir = getAbsWorkspace(workspaceDir)
	SetupCmd(cmd)
	InjectSecretsEnv(cmd, secrets)
	InjectCredentialEnv(cmd, creds)

	stdout := NewBoundedBuffer(1024 * 1024)
	stderr := NewBoundedBuffer(1024 * 1024)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return "", "", err
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	timer := time.NewTimer(GetForegroundTimeout())
	defer timer.Stop()

	select {
	case err := <-done:
		so, se := ScrubSecretOutput(stdout.String(), stderr.String())
		return so, se, err
	case <-timer.C:
		KillProcessTree(cmd.Process.Pid)
		select {
		case <-done:
		case <-time.After(10 * time.Second):
		}
		so, se := ScrubSecretOutput(stdout.String(), stderr.String())
		return so, se, fmt.Errorf("TIMEOUT: script exceeded %s limit and was killed", GetForegroundTimeout())
	}
}

// ExecutePythonBackground starts a Python script in the background,
// registers it in the process registry, and returns the PID immediately.
func ExecutePythonBackground(code, workspaceDir, toolsDir string, registry *ProcessRegistry) (int, error) {
	scriptPath, _, err := writeScript(code, toolsDir)
	if err != nil {
		return 0, err
	}
	// Note: we do NOT defer cleanup for background scripts — they need the file while running.

	pythonCmd := GetPythonBin(workspaceDir)
	cmd := exec.Command(pythonCmd, scriptPath)
	cmd.Dir = getAbsWorkspace(workspaceDir)

	pid, err := registerManagedBackgroundProcess(cmd, registry, func() {
		os.Remove(scriptPath)
	})
	if err != nil {
		os.Remove(scriptPath)
		return 0, fmt.Errorf("failed to start background process: %w", err)
	}
	return pid, nil
}

// ExecutePythonBackgroundWithSecrets is like ExecutePythonBackground but injects vault secrets
// and credential secrets as environment variables. Output scrubbing happens via ReadOutput + security.Scrub at read time.
func ExecutePythonBackgroundWithSecrets(code, workspaceDir, toolsDir string, registry *ProcessRegistry, secrets map[string]string, creds []CredentialFields) (int, error) {
	scriptPath, _, err := writeScript(code, toolsDir)
	if err != nil {
		return 0, err
	}

	pythonCmd := GetPythonBin(workspaceDir)
	cmd := exec.Command(pythonCmd, scriptPath)
	cmd.Dir = getAbsWorkspace(workspaceDir)
	InjectSecretsEnv(cmd, secrets)
	InjectCredentialEnv(cmd, creds)

	pid, err := registerManagedBackgroundProcess(cmd, registry, func() {
		os.Remove(scriptPath)
	})
	if err != nil {
		os.Remove(scriptPath)
		return 0, fmt.Errorf("failed to start background process: %w", err)
	}
	return pid, nil
}

// maxScriptBytes is the maximum allowed Python script size. Scripts larger than
// this are rejected before touching the filesystem to prevent DoS via disk fill.
const maxScriptBytes = 512 * 1024 // 512 KB

// writeScript creates a temporary Python file and returns its absolute path and a cleanup function.
func writeScript(code, toolsDir string) (string, func(), error) {
	if len(code) > maxScriptBytes {
		return "", nil, fmt.Errorf("script too large: %d bytes (max %d KB)", len(code), maxScriptBytes/1024)
	}

	tmpFile, err := os.CreateTemp(toolsDir, "aurago_agent_*.py")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	if _, err := tmpFile.WriteString(code); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", nil, fmt.Errorf("failed to write code to temp file: %w", err)
	}
	tmpFile.Close()

	absPath, err := filepath.Abs(tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", nil, fmt.Errorf("failed to resolve script path: %w", err)
	}

	cleanup := func() {
		os.Remove(absPath)
	}

	return absPath, cleanup, nil
}
