package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"aurago/internal/security"
)

// SkillManifest represents the structure of a skill config file (.json).
type SkillManifest struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Executable   string            `json:"executable"`             // e.g., "scan.py" or "custom_tool.exe"
	Parameters   map[string]string `json:"parameters,omitempty"`   // map of arg name to description
	Returns      string            `json:"returns,omitempty"`      // describes expected output format
	Dependencies []string          `json:"dependencies,omitempty"` // pip packages required by this skill
	VaultKeys    []string          `json:"vault_keys,omitempty"`   // vault secret keys this skill needs at runtime
}

// ListSkills scans the skills directory for .json manifest files and returns them.
func ListSkills(skillsDir string) ([]SkillManifest, error) {
	var skills []SkillManifest

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return skills, nil // Empty but not an error if directory doesn't exist yet
		}
		return nil, fmt.Errorf("failed to read skills directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			path := filepath.Join(skillsDir, entry.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue // Skip unreadable files
			}

			var manifest SkillManifest
			if err := json.Unmarshal(data, &manifest); err == nil && manifest.Name != "" && manifest.Executable != "" {
				skills = append(skills, manifest)
			}
		}
	}

	return skills, nil
}

// ExecuteSkill dynamically executes the requested skill script, routing Python scripts to the venv.
func ExecuteSkill(skillsDir, workspaceDir, skillName string, argsJSON map[string]interface{}) (string, error) {
	// First, lookup the skill manifest to find its executable
	skills, err := ListSkills(skillsDir)
	if err != nil {
		return "", fmt.Errorf("failed to scan skills: %v", err)
	}

	var manifest *SkillManifest
	for _, s := range skills {
		if s.Name == skillName {
			manifest = &s
			break
		}
	}

	if manifest == nil {
		return "", fmt.Errorf("skill '%s' not found", skillName)
	}

	// Ensure the skill executable path is absolute.
	// This is CRITICAL because cmd.Dir is set to workspaceDir, which would break relative paths.
	absExecPath, err := filepath.Abs(filepath.Join(skillsDir, manifest.Executable))
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path for skill '%s': %v", skillName, err)
	}

	if _, err := os.Stat(absExecPath); os.IsNotExist(err) {
		return "", fmt.Errorf("skill executable '%s' not found at %s", manifest.Executable, absExecPath)
	}

	if argsJSON == nil {
		argsJSON = make(map[string]interface{})
	}
	argsBytes, err := json.Marshal(argsJSON)
	if err != nil {
		return "", fmt.Errorf("failed to serialize args JSON: %v", err)
	}
	argsString := string(argsBytes)
	slog.Debug("[ExecuteSkill] Prepared JSON input", "skill", skillName, "input", argsString)

	// Route based on extension
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if strings.HasSuffix(manifest.Executable, ".py") {
		cfgPythonBin := GetPythonBin(workspaceDir)
		cmd = exec.CommandContext(ctx, cfgPythonBin, "-u", absExecPath)
	} else if strings.HasSuffix(manifest.Executable, ".sh") && runtime.GOOS != "windows" {
		cmd = exec.CommandContext(ctx, "bash", absExecPath)
	} else if strings.HasSuffix(manifest.Executable, ".ps1") && runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "powershell", "-ExecutionPolicy", "Bypass", "-File", absExecPath)
	} else {
		// Attempt to run directly (e.g., .exe or native binary)
		cmd = exec.CommandContext(ctx, absExecPath)
	}

	cmd.Dir = workspaceDir
	SetupCmd(cmd)

	// Manual Stdin pipe management for maximum synchronization on Windows.
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdin pipe: %v", err)
	}

	var outBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start skill execution: %v", err)
	}

	// Write and CLOSE immediately to send EOF
	slog.Debug("[ExecuteSkill] Writing to Stdin...", "length", len(argsString))
	fmt.Fprint(stdin, argsString)
	if err := stdin.Close(); err != nil {
		slog.Error("[ExecuteSkill] Failed to close stdin pipe", "error", err)
	} else {
		slog.Debug("[ExecuteSkill] Stdin closed (EOF sent)")
	}

	err = cmd.Wait()
	output := outBuf.String()
	if ctx.Err() == context.DeadlineExceeded {
		return output, fmt.Errorf("TIMEOUT: skill '%s' exceeded 2-minute limit and was killed", skillName)
	}
	if err != nil {
		return output, fmt.Errorf("execution failed: %v", err)
	}

	return output, nil
}

// ExecuteSkillWithSecrets is like ExecuteSkill but injects vault secrets and credential secrets
// as environment variables and scrubs secrets from the output.
func ExecuteSkillWithSecrets(skillsDir, workspaceDir, skillName string, argsJSON map[string]interface{}, secrets map[string]string, creds []CredentialFields) (string, error) {
	skills, err := ListSkills(skillsDir)
	if err != nil {
		return "", fmt.Errorf("failed to scan skills: %v", err)
	}

	var manifest *SkillManifest
	for _, s := range skills {
		if s.Name == skillName {
			manifest = &s
			break
		}
	}
	if manifest == nil {
		return "", fmt.Errorf("skill '%s' not found", skillName)
	}

	absExecPath, err := filepath.Abs(filepath.Join(skillsDir, manifest.Executable))
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path for skill '%s': %v", skillName, err)
	}
	if _, err := os.Stat(absExecPath); os.IsNotExist(err) {
		return "", fmt.Errorf("skill executable '%s' not found at %s", manifest.Executable, absExecPath)
	}

	if argsJSON == nil {
		argsJSON = make(map[string]interface{})
	}
	argsBytes, err := json.Marshal(argsJSON)
	if err != nil {
		return "", fmt.Errorf("failed to serialize args JSON: %v", err)
	}
	argsString := string(argsBytes)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if strings.HasSuffix(manifest.Executable, ".py") {
		cfgPythonBin := GetPythonBin(workspaceDir)
		cmd = exec.CommandContext(ctx, cfgPythonBin, "-u", absExecPath)
	} else if strings.HasSuffix(manifest.Executable, ".sh") && runtime.GOOS != "windows" {
		cmd = exec.CommandContext(ctx, "bash", absExecPath)
	} else if strings.HasSuffix(manifest.Executable, ".ps1") && runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "powershell", "-ExecutionPolicy", "Bypass", "-File", absExecPath)
	} else {
		cmd = exec.CommandContext(ctx, absExecPath)
	}

	cmd.Dir = workspaceDir
	SetupCmd(cmd)
	InjectSecretsEnv(cmd, secrets)
	InjectCredentialEnv(cmd, creds)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdin pipe: %v", err)
	}

	var outBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start skill execution: %v", err)
	}

	fmt.Fprint(stdin, argsString)
	stdin.Close()

	err = cmd.Wait()
	output := security.Scrub(outBuf.String())
	if ctx.Err() == context.DeadlineExceeded {
		return output, fmt.Errorf("TIMEOUT: skill '%s' exceeded 2-minute limit and was killed", skillName)
	}
	if err != nil {
		return output, fmt.Errorf("execution failed: %v", err)
	}

	return output, nil
}

// ProvisionSkillDependencies scans all skills and installs their pip dependencies into the venv.
func ProvisionSkillDependencies(skillsDir, workspaceDir string, logger *slog.Logger) {
	skills, err := ListSkills(skillsDir)
	if err != nil {
		logger.Warn("Failed to scan skills for dependency provisioning", "error", err)
		return
	}

	// Aggregate unique dependencies
	seen := make(map[string]bool)
	var deps []string
	for _, s := range skills {
		for _, dep := range s.Dependencies {
			dep = strings.TrimSpace(dep)
			if dep != "" && !seen[dep] {
				seen[dep] = true
				deps = append(deps, dep)
			}
		}
	}

	if len(deps) == 0 {
		logger.Info("No skill dependencies to provision.")
		return
	}

	logger.Info("Provisioning skill dependencies", "packages", strings.Join(deps, ", "))

	// Ensure venv exists before installing
	if err := EnsureVenv(workspaceDir, logger); err != nil {
		logger.Error("Failed to ensure Python virtual environment", "error", err)
		return
	}

	pipBin := GetPipBin(workspaceDir)
	args := append([]string{"install"}, deps...)
	cmd := exec.Command(pipBin, args...)
	cmd.Dir = workspaceDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("Failed to provision skill dependencies", "error", err, "output", string(output))
		return
	}
	logger.Info("Skill dependencies provisioned successfully.")
}
