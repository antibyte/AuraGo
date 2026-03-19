package tools

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const ForegroundTimeout = 30 * time.Second

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

// InstallPackage installs a Python package using the virtual environment's pip.
// Has a generous 3-minute timeout for downloads and compilation.
func InstallPackage(pkgName, workspaceDir string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	pipCmd := GetPipBin(workspaceDir)
	cmd := exec.CommandContext(ctx, pipCmd, "install", pkgName)
	cmd.Dir = getAbsWorkspace(workspaceDir)

	slog.Debug("[InstallPackage]", "cmd", pipCmd, "args", cmd.Args)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return stdout.String(), stderr.String(), fmt.Errorf("TIMEOUT: pip install '%s' exceeded 3-minute limit", pkgName)
	}
	return stdout.String(), stderr.String(), err
}

// RunTool executes a saved tool from the tools directory with arguments (foreground, 30s timeout).
// Path traversal is blocked — name must resolve within toolsDir.
func RunTool(name string, args []string, workspaceDir, toolsDir string) (string, string, error) {
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") || name == "" {
		return "", "", fmt.Errorf("invalid tool name: must be a simple filename without path separators")
	}
	toolPath := filepath.Join(toolsDir, name)
	if _, err := os.Stat(toolPath); os.IsNotExist(err) {
		return "", "", fmt.Errorf("tool '%s' not found in %s", name, toolsDir)
	}

	absToolPath, err := filepath.Abs(toolPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve tool path: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), ForegroundTimeout)
	defer cancel()

	pythonCmd := GetPythonBin(workspaceDir)
	cmdArgs := append([]string{absToolPath}, args...)
	cmd := exec.CommandContext(ctx, pythonCmd, cmdArgs...)
	cmd.Dir = getAbsWorkspace(workspaceDir)

	slog.Debug("[RunTool]", "cmd", pythonCmd, "args", cmd.Args)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return stdout.String(), stderr.String(), fmt.Errorf("TIMEOUT: tool '%s' exceeded %s limit and was killed", name, ForegroundTimeout)
	}
	return stdout.String(), stderr.String(), err
}

// RunToolBackground starts a saved tool in the background and registers it in the process registry.
func RunToolBackground(name string, args []string, workspaceDir, toolsDir string, registry *ProcessRegistry) (int, error) {
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") || name == "" {
		return 0, fmt.Errorf("invalid tool name: must be a simple filename without path separators")
	}
	toolPath := filepath.Join(toolsDir, name)
	if _, err := os.Stat(toolPath); os.IsNotExist(err) {
		return 0, fmt.Errorf("tool '%s' not found in %s", name, toolsDir)
	}

	absToolPath, err := filepath.Abs(toolPath)
	if err != nil {
		return 0, fmt.Errorf("failed to resolve tool path: %w", err)
	}

	pythonCmd := GetPythonBin(workspaceDir)
	cmdArgs := append([]string{absToolPath}, args...)
	cmd := exec.Command(pythonCmd, cmdArgs...)
	cmd.Dir = getAbsWorkspace(workspaceDir)

	slog.Debug("[RunToolBackground]", "cmd", pythonCmd, "args", cmd.Args)

	info := &ProcessInfo{
		Output:    &bytes.Buffer{},
		StartedAt: time.Now(),
		Alive:     true,
	}
	cmd.Stdout = info
	cmd.Stderr = info

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start background tool: %w", err)
	}

	info.PID = cmd.Process.Pid
	info.Process = cmd.Process
	registry.Register(info)

	go func() {
		err := cmd.Wait()
		info.mu.Lock()
		info.Alive = false
		if err != nil {
			info.Output.WriteString(fmt.Sprintf("\n[process exited with error: %v]", err))
		}
		info.mu.Unlock()
		registry.Remove(info.PID)
	}()

	return info.PID, nil
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

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return "", "", err
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	timer := time.NewTimer(ForegroundTimeout)
	defer timer.Stop()

	select {
	case err := <-done:
		return stdout.String(), stderr.String(), err
	case <-timer.C:
		KillProcessTree(cmd.Process.Pid)
		<-done
		return stdout.String(), stderr.String(), fmt.Errorf("TIMEOUT: script exceeded %s limit and was killed", ForegroundTimeout)
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

	info := &ProcessInfo{
		Output:    &bytes.Buffer{},
		StartedAt: time.Now(),
		Alive:     true,
	}

	// Wire combined stdout+stderr to the registry buffer
	cmd.Stdout = info
	cmd.Stderr = info

	if err := cmd.Start(); err != nil {
		os.Remove(scriptPath)
		return 0, fmt.Errorf("failed to start background process: %w", err)
	}

	info.PID = cmd.Process.Pid
	info.Process = cmd.Process
	registry.Register(info)

	// Monitor the process in a goroutine
	go func() {
		err := cmd.Wait()
		info.mu.Lock()
		info.Alive = false
		if err != nil {
			info.Output.WriteString(fmt.Sprintf("\n[process exited with error: %v]", err))
		}
		info.mu.Unlock()
		registry.Remove(info.PID)
		os.Remove(scriptPath) // Clean up script after process exits
	}()

	return info.PID, nil
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
