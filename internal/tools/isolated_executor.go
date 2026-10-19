package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

func isolatedExecutionRequired(ctx context.Context, shell bool) bool {
	perms, _ := EffectiveRuntimePermissions(ctx)
	return !perms.AllowUnsafeHostExecution && !(shell && runtime.GOOS != "windows" && perms.AllowUnsandboxedShell)
}

// RequiresIsolatedExecution reports the effective execution mode for dispatch.
func RequiresIsolatedExecution(ctx context.Context, shell bool) bool {
	return isolatedExecutionRequired(ctx, shell)
}

// ScheduleIsolatedExecution retains the existing durable background manager's
// ownership, cancellation and result lifecycle instead of inventing a host PID.
func ScheduleIsolatedExecution(ctx context.Context, code, language, kind, description string) (*BackgroundTask, error) {
	if err := requireIsolatedPermission(ctx, kind == "shell"); err != nil {
		return nil, err
	}
	if !SandboxReady() {
		return nil, fmt.Errorf("sandbox manager is not ready: %w", ErrSandboxUnavailable)
	}
	manager := DefaultBackgroundTaskManager()
	if manager == nil {
		return nil, fmt.Errorf("background task manager is unavailable")
	}
	return manager.ScheduleIsolatedCodeContext(ctx, code, language, BackgroundTaskScheduleOptions{Source: "isolated_execution", Description: description, Timeout: GetBackgroundTimeout(), ExecutionKind: kind, NotifyOnCompletion: true})
}

func ScheduleIsolatedShell(ctx context.Context, command string) (*BackgroundTask, error) {
	if err := ValidateShellCommandPolicy(command); err != nil {
		return nil, err
	}
	return ScheduleIsolatedExecution(ctx, isolatedShellCode(command), "python", "shell", "Isolated background shell")
}

func ScheduleIsolatedTool(ctx context.Context, name string, args []string, workspaceDir, toolsDir string) (*BackgroundTask, error) {
	path, err := resolveToolPath(name, toolsDir)
	if err != nil {
		return nil, err
	}
	source, err := readIsolatedScript(path, workspaceDir)
	if err != nil {
		return nil, err
	}
	return ScheduleIsolatedExecution(ctx, isolatedScriptCode(source, name, args, ""), "python", "python", "Isolated background tool "+filepath.Base(name))
}

func requireIsolatedPermission(ctx context.Context, shell bool) error {
	perms, configured := EffectiveRuntimePermissions(ctx)
	if shell {
		return requireRuntimePermission("shell execution", configured && perms.AllowShell)
	}
	return requireRuntimePermission("python execution", configured && perms.AllowPython)
}

// runIsolatedWorker transports source only. Workers receive no host mounts,
// environment credentials, Docker endpoint or ambient Tool Bridge token.
func runIsolatedWorker(ctx context.Context, code, language string, timeout time.Duration) (string, string, error) {
	out, err := SandboxExecuteCodeContext(ctx, code, language, nil, int(timeout.Seconds()), nil)
	return decodeIsolatedWorkerOutput(out, err)
}

// ExecuteIsolatedBackgroundCode is the supervisor callback for the durable
// background manager. The manager owns cancellation and the captured run gates.
func ExecuteIsolatedBackgroundCode(ctx context.Context, code, language string, timeout time.Duration) (string, error) {
	stdout, stderr, err := runIsolatedWorker(ctx, code, language, timeout)
	return security.Scrub(stdout + stderr), err
}

func decodeIsolatedWorkerOutput(out string, err error) (string, string, error) {
	if err != nil {
		return "", "", err
	}
	var result struct {
		Stdout   string `json:"stdout"`
		Stderr   string `json:"stderr"`
		ExitCode *int   `json:"exit_code"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return "", "", fmt.Errorf("invalid sandbox execution response: %w", err)
	}
	if result.ExitCode == nil {
		return "", "", fmt.Errorf("sandbox execution response omitted exit_code")
	}
	if *result.ExitCode != 0 {
		return result.Stdout, result.Stderr, fmt.Errorf("sandbox process exited with code %d", *result.ExitCode)
	}
	return result.Stdout, result.Stderr, nil
}

func readIsolatedScript(path, workspaceDir string) (string, error) {
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = workspaceDir
	file, err := OpenToolInputFile(path, cfg)
	if err != nil {
		return "", err
	}
	defer file.Close()
	source, err := io.ReadAll(io.LimitReader(file, 1024*1024+1))
	if err != nil {
		return "", err
	}
	if len(source) > 1024*1024 {
		return "", fmt.Errorf("script exceeds 1 MiB")
	}
	return string(source), nil
}

func isolatedScriptCode(source, name string, args []string, input string) string {
	encodedArgs, _ := json.Marshal(append([]string{filepath.Base(name)}, args...))
	return "import sys, io, json\nsys.argv = json.loads(" + strconv.Quote(string(encodedArgs)) + ")\nsys.stdin = io.StringIO(" + strconv.Quote(input) + ")\nexec(compile(" + strconv.Quote(source) + ", " + strconv.Quote(filepath.Base(name)) + ", 'exec'), {'__name__':'__main__'})\n"
}

func isolatedShellCode(command string) string {
	return "import subprocess, sys\np = subprocess.run(['/bin/sh', '-c', " + strconv.Quote(command) + "])\nsys.exit(p.returncode)\n"
}

func isolatedScriptLanguage(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".py":
		return "python"
	case ".js":
		return "javascript"
	case ".sh":
		return "shell"
	}
	return ""
}

func isolatedSkillCode(source, path, input string) (string, string, error) {
	switch isolatedScriptLanguage(path) {
	case "python":
		return isolatedScriptCode(source, path, nil, input), "python", nil
	case "shell":
		return isolatedShellCode("printf '%s' " + shellSingleQuote(input) + " | /bin/sh -c " + shellSingleQuote(source)), "python", nil
	case "javascript":
		encodedSource, _ := json.Marshal(source)
		encodedInput, _ := json.Marshal(input)
		return "const p = require('child_process').spawnSync(process.execPath, ['-e', " + string(encodedSource) + "], {input:" + string(encodedInput) + ", stdio:['pipe','inherit','inherit']}); if(p.error) throw p.error; process.exit(p.status === null ? 1 : p.status);", "javascript", nil
	}
	return "", "", fmt.Errorf("native skill requires an explicit unsafe host execution grant")
}
