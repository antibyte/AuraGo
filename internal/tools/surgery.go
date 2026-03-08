package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

// GeminiResponse defines the expected structure from the gemini CLI JSON output.
type GeminiResponse struct {
	Text string `json:"text"`
}

// ExecuteSurgery calls the 'gemini' CLI tool to perform code changes based on a plan.
func ExecuteSurgery(plan string, workspaceDir string, logger *slog.Logger) (string, error) {
	logger.Info("Executing AI Surgery via Gemini CLI...", "plan_len", len(plan))

	// Use the plan directly - the CLI handles the surgical persona via --output-format json
	fullPrompt := plan

	geminiCmd := "gemini"
	cmd := exec.Command(geminiCmd, "-p", fullPrompt, "--yolo", "--output-format", "json")

	cmd.Dir = workspaceDir
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "NODE_NO_WARNINGS=1")

	// Apply platform-specific attributes (e.g., HideWindow on Windows)
	SetupCmd(cmd)

	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	// Check PATH
	if _, err := exec.LookPath(geminiCmd); err != nil {
		return "", fmt.Errorf("gemini command not found in PATH")
	}

	logger.Info("Starting Gemini process...")
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start gemini: %v", err)
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
			return "", fmt.Errorf("surgery failed: %v", err)
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

// extractJSON attempts to find the first '{' and last '}' to extract a potential JSON object.
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start == -1 || end == -1 || end <= start {
		return ""
	}
	return s[start : end+1]
}
