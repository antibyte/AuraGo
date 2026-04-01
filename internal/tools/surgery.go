package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"aurago/internal/sandbox"
)

const maxSurgeryPlanBytes = 128 * 1024

// GeminiResponse defines the expected structure from the gemini CLI JSON output.
type GeminiResponse struct {
	Text string `json:"text"`
}

// ExecuteSurgery calls the 'gemini' CLI tool to perform code changes based on a plan.
func ExecuteSurgery(plan string, workspaceDir string, logger *slog.Logger) (string, error) {
	logger.Info("Executing AI Surgery via Gemini CLI...", "plan_len", len(plan))
	if strings.TrimSpace(plan) == "" {
		return "", fmt.Errorf("surgery plan is required")
	}
	if len(plan) > maxSurgeryPlanBytes {
		return "", fmt.Errorf("surgery plan too large: %d bytes (max %d)", len(plan), maxSurgeryPlanBytes)
	}
	resolvedWorkspace, err := secureResolve(workspaceDir, ".")
	if err != nil {
		return "", fmt.Errorf("invalid surgery workspace: %w", err)
	}
	info, err := os.Stat(resolvedWorkspace)
	if err != nil {
		return "", fmt.Errorf("stat surgery workspace: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("surgery workspace is not a directory")
	}

	// Use the plan directly - the CLI handles the surgical persona via --output-format json
	fullPrompt := plan

	geminiCmd := "gemini"
	geminiArgs := []string{"-p", fullPrompt, "--output-format", "json"}
	var cmd *exec.Cmd
	if runtime.GOOS != "windows" {
		if sb := sandbox.Get(); sb.Available() {
			cmd = sb.PrepareExecCommand(geminiCmd, geminiArgs, resolvedWorkspace)
			logger.Info("Using sandboxed Gemini execution", "backend", sb.Name())
		} else {
			cmd = exec.Command(geminiCmd, geminiArgs...)
		}
	} else {
		cmd = exec.Command(geminiCmd, geminiArgs...)
	}

	cmd.Dir = resolvedWorkspace
	cmd.Env = filterSurgeryEnv(os.Environ())
	cmd.Env = append(cmd.Env, "NODE_NO_WARNINGS=1")

	// Apply platform-specific attributes (e.g., HideWindow on Windows)
	SetupCmd(cmd)

	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	// Check PATH
	if _, err := exec.LookPath(geminiCmd); err != nil {
		return "", fmt.Errorf("gemini command not found in PATH: %w", err)
	}

	logger.Info("Starting Gemini process...")
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start gemini: %w", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-time.After(10 * time.Minute):
		cmd.Process.Kill()
		return "", fmt.Errorf("surgery timed out after 10m")
	case err := <-done:
		rawOutput := out.String()

		if err != nil {
			return "", fmt.Errorf("surgery failed: %w", err)
		}

		cleanOutput := extractJSON(rawOutput)
		if cleanOutput == "" {
			return rawOutput, nil
		}

		var resp GeminiResponse
		if err := json.Unmarshal([]byte(cleanOutput), &resp); err != nil {
			return rawOutput, nil
		}

		result := resp.Text
		if result == "" {
			result = "Surgery successful, but no file changes were necessary or made by the AI."
		}
		logger.Info("Surgery successful", "len", len(result))
		return result, nil
	}
}

func filterSurgeryEnv(env []string) []string {
	filtered := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if isAllowedSurgeryEnvKey(key) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func isAllowedSurgeryEnvKey(key string) bool {
	if key == "" {
		return false
	}
	allowedExact := map[string]bool{
		"PATH": true, "HOME": true, "USERPROFILE": true, "TMP": true, "TEMP": true,
		"SYSTEMROOT": true, "COMSPEC": true, "PATHEXT": true, "WINDIR": true,
		"APPDATA": true, "LOCALAPPDATA": true, "PROGRAMDATA": true, "PWD": true,
		"TERM": true, "LANG": true, "SSL_CERT_FILE": true, "SSL_CERT_DIR": true,
		"HTTP_PROXY": true, "HTTPS_PROXY": true, "NO_PROXY": true,
		"GEMINI_API_KEY": true, "GOOGLE_API_KEY": true, "GOOGLE_APPLICATION_CREDENTIALS": true,
	}
	if allowedExact[key] {
		return true
	}
	for _, prefix := range []string{"LC_", "XDG_", "GEMINI_", "GOOGLE_CLOUD_", "GOOGLE_GENAI_", "CLOUDSDK_"} {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

// extractJSON attempts to find the first '{' and last '}' to extract a potential JSON object.
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start == -1 || end == -1 || end <= start {
		return ""
	}
	return s[start : end+1]
}
