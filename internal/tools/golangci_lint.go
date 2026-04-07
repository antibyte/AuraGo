package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// GolangciLintResult is the structured result returned by ExecuteGolangciLint.
type GolangciLintResult struct {
	Status     string   `json:"status"`            // "ok", "issues", "error"
	Message    string   `json:"message,omitempty"` // human-readable summary
	Issues     []string `json:"issues,omitempty"`  // individual lint findings
	IssueCount int      `json:"issue_count"`
}

func encodeGolangciResult(r GolangciLintResult) string {
	b, _ := json.Marshal(r)
	return string(b)
}

// ExecuteGolangciLint runs golangci-lint on the given path (package or directory).
// If golangci-lint is not installed, it attempts to install it via go install.
// lintPath:   package path or directory (e.g. "./...", "./internal/agent")
// configPath: optional path to a .golangci.yml config file (empty = auto-detect)
// workspaceDir: workspace root (for resolving relative paths)
func ExecuteGolangciLint(lintPath, configPath, workspaceDir string) string {
	if lintPath == "" {
		lintPath = "./..."
	}

	absWorkDir := getAbsWorkspace(workspaceDir)

	// Locate or install golangci-lint
	binaryPath, err := findOrInstallGolangciLint(absWorkDir)
	if err != nil {
		return encodeGolangciResult(GolangciLintResult{
			Status:  "error",
			Message: fmt.Sprintf("golangci-lint not available and could not be installed: %v", err),
		})
	}

	// Build args
	args := []string{"run", "--out-format=line-number"}
	if configPath != "" {
		resolved, resolveErr := filepath.Abs(filepath.Join(absWorkDir, configPath))
		if resolveErr == nil {
			args = append(args, "--config="+resolved)
		}
	}
	args = append(args, lintPath)

	cmd := exec.Command(binaryPath, args...) //nolint:gosec // binaryPath is resolved from trusted location
	cmd.Dir = absWorkDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	SetupCmd(cmd)

	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		return encodeGolangciResult(GolangciLintResult{
			Status:  "error",
			Message: fmt.Sprintf("Failed to start golangci-lint: %v", err),
		})
	}
	go func() { done <- cmd.Wait() }()

	timer := time.NewTimer(5 * time.Minute)
	defer timer.Stop()

	select {
	case <-timer.C:
		KillProcessTree(cmd.Process.Pid)
		return encodeGolangciResult(GolangciLintResult{
			Status:  "error",
			Message: "golangci-lint timed out after 5 minutes",
		})
	case runErr := <-done:
		out := stdout.String()
		errOut := strings.TrimSpace(stderr.String())

		// exit code 0 = no issues; exit code 1 = issues found (not a fatal error)
		// any other error (e.g. misconfiguration) falls through to error handling
		if runErr != nil {
			exitCode := 0
			if exitErr, ok := runErr.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
			if exitCode != 1 {
				msg := fmt.Sprintf("golangci-lint failed (exit %d)", exitCode)
				if errOut != "" {
					msg += ": " + errOut
				}
				return encodeGolangciResult(GolangciLintResult{
					Status:  "error",
					Message: msg,
				})
			}
		}

		issues := parseGolangciOutput(out)
		if len(issues) == 0 && strings.TrimSpace(out) == "" {
			msg := "No issues found"
			if lintPath != "" && lintPath != "./..." {
				msg += " in " + lintPath
			}
			return encodeGolangciResult(GolangciLintResult{
				Status:     "ok",
				Message:    msg,
				IssueCount: 0,
			})
		}

		return encodeGolangciResult(GolangciLintResult{
			Status:     "issues",
			Message:    fmt.Sprintf("Found %d issue(s)", len(issues)),
			Issues:     issues,
			IssueCount: len(issues),
		})
	}
}

// parseGolangciOutput extracts individual issue lines from golangci-lint output.
// With --out-format=line-number, each issue is on its own line.
func parseGolangciOutput(output string) []string {
	var issues []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		issues = append(issues, line)
	}
	return issues
}

// findOrInstallGolangciLint returns the path to the golangci-lint executable.
// If not found in PATH or GOPATH/bin, it installs it via `go install`.
func findOrInstallGolangciLint(workDir string) (string, error) {
	// 1. Check PATH
	if p, err := exec.LookPath("golangci-lint"); err == nil {
		return p, nil
	}

	// 2. Check GOPATH/bin (common when installed via go install but not in PATH)
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		candidate := filepath.Join(gopath, "bin", golangciLintBinary())
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// 3. Check $HOME/go/bin (default GOPATH when GOPATH is unset)
	if home, err := os.UserHomeDir(); err == nil {
		candidate := filepath.Join(home, "go", "bin", golangciLintBinary())
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// 4. Not found — install via go install
	installCmd := exec.Command("go", "install", "github.com/golangci/golangci-lint/cmd/golangci-lint@latest")
	installCmd.Dir = workDir
	var installErr bytes.Buffer
	installCmd.Stderr = &installErr

	if err := installCmd.Run(); err != nil {
		return "", fmt.Errorf("go install failed: %v — %s", err, strings.TrimSpace(installErr.String()))
	}

	// Retry lookup after installation
	if p, err := exec.LookPath("golangci-lint"); err == nil {
		return p, nil
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidate := filepath.Join(home, "go", "bin", golangciLintBinary())
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("golangci-lint installed but still not found in PATH or ~/go/bin")
}

func golangciLintBinary() string {
	if runtime.GOOS == "windows" {
		return "golangci-lint.exe"
	}
	return "golangci-lint"
}
