package tools

import (
	"aurago/internal/security"
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// GoogleWorkspaceConfig handles parameters for Google Workspace operations
type GoogleWorkspaceConfig struct {
	Action     string `json:"action"`
	MaxResults int    `json:"max_results,omitempty"`
	DocumentID string `json:"document_id,omitempty"`
	Title      string `json:"title,omitempty"`
	Text       string `json:"text,omitempty"`
	Append     bool   `json:"append,omitempty"`
}

// ExecuteGoogleWorkspace runs the Google Workspace python backend with vault-secured secrets
func ExecuteGoogleWorkspace(vault *security.Vault, workspaceDir, toolsDir string, config GoogleWorkspaceConfig) (string, error) {
	// 1. Fetch secrets from vault
	clientSecret, err := vault.ReadSecret("google_workspace_client_secret")
	if err != nil {
		return "", fmt.Errorf("failed to read Google client secret from vault: %w", err)
	}

	token, _ := vault.ReadSecret("google_workspace_token") // Optional, might be empty if not re-auth'd

	// 2. Prepare payload for Python
	payload := map[string]interface{}{
		"action": config.Action,
		"vault_secrets": map[string]string{
			"google_workspace_client_secret": clientSecret,
			"google_workspace_token":         token,
		},
	}
	if config.MaxResults > 0 {
		payload["max_results"] = config.MaxResults
	}
	if config.DocumentID != "" {
		payload["document_id"] = config.DocumentID
	}
	if config.Title != "" {
		payload["title"] = config.Title
	}
	if config.Text != "" {
		payload["text"] = config.Text
	}
	payload["append"] = config.Append

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Google Workspace payload: %w", err)
	}

	// 3. Execute via Python wrapper
	scriptPath := filepath.Join(workspaceDir, "..", "skills", "google_workspace.py")
	absScriptPath, err := filepath.Abs(scriptPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve script path: %w", err)
	}

	pythonBin := GetPythonBin(workspaceDir)
	stdout, stderr, err := runPythonWithStdin(pythonBin, absScriptPath, workspaceDir, string(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("python execution failed: %v\nStderr: %s", err, stderr)
	}

	// 4. Handle potential vault updates (e.g., new refresh token)
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err == nil {
		if update, ok := result["vault_update"].(map[string]interface{}); ok {
			for k, v := range update {
				if vs, ok := v.(string); ok {
					_ = vault.WriteSecret(k, vs)
				}
			}
		}
	}

	return stdout, nil
}

// Helper to run python with stdin
func runPythonWithStdin(pythonBin, scriptPath, workspaceDir, stdinData string) (string, string, error) {
	cmd := exec.Command(pythonBin, scriptPath)
	cmd.Dir = workspaceDir
	cmd.Stdin = strings.NewReader(stdinData)

	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	err := cmd.Run()
	return out.String(), errOut.String(), err
}
