package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// gitTimeout is the maximum duration for git operations
const gitTimeout = 30 * time.Second

// GitBackupRequest outlines parameters for git tools
type GitBackupRequest struct {
	Action     string `json:"action"`
	CommitMsg  string `json:"commit_message,omitempty"`
	CommitHash string `json:"commit_hash,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	Mode       string `json:"mode,omitempty"`
}

// ExecuteGit handles the git_backup_restore skill internally
func ExecuteGit(workspaceDir string, req GitBackupRequest) string {
	var result map[string]interface{}

	switch req.Action {
	case "create_backup":
		msg := req.CommitMsg
		if msg == "" {
			msg = "Auto-backup before AuraGo fix"
		}
		result = createBackup(workspaceDir, msg)
	case "list_backups":
		limit := req.Limit
		if limit <= 0 {
			limit = 10
		}
		result = listBackups(workspaceDir, limit)
	case "restore":
		if req.CommitHash == "" {
			result = map[string]interface{}{"status": "error", "message": "Missing commit_hash"}
		} else {
			mode := req.Mode
			if mode == "" {
				mode = "revert"
			}
			result = restoreGit(workspaceDir, req.CommitHash, mode)
		}
	case "rollback_to_previous":
		result = rollbackToPrevious(workspaceDir)
	case "show_diff":
		result = showDiff(workspaceDir, req.CommitHash)
	default:
		result = map[string]interface{}{"status": "error", "message": fmt.Sprintf("Unknown action: %s", req.Action)}
	}

	b, _ := json.Marshal(result)
	return string(b)
}

func runGitCmd(dir string, args ...string) (string, string, int) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	err := cmd.Run()
	rc := 0
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return out.String(), errOut.String() + "\n[timeout after 30s]", 1
		}
		if exitError, ok := err.(*exec.ExitError); ok {
			rc = exitError.ExitCode()
		} else {
			rc = 1
		}
	}
	return out.String(), errOut.String(), rc
}

func createBackup(dir, msg string) map[string]interface{} {
	_, stderr, rc := runGitCmd(dir, "rev-parse", "--is-inside-work-tree")
	if rc != 0 {
		return map[string]interface{}{"status": "error", "message": fmt.Sprintf("Not a git repository: %s", stderr)}
	}

	stdout, stderr, rc := runGitCmd(dir, "status", "--porcelain")
	if rc != 0 {
		return map[string]interface{}{"status": "error", "message": fmt.Sprintf("Git status failed: %s", stderr)}
	}
	if strings.TrimSpace(stdout) == "" {
		return map[string]interface{}{"status": "info", "message": "No changes to commit."}
	}

	_, stderr, rc = runGitCmd(dir, "add", "-A")
	if rc != 0 {
		return map[string]interface{}{"status": "error", "message": fmt.Sprintf("Git add failed: %s", stderr)}
	}

	stdout, stderr, rc = runGitCmd(dir, "commit", "-m", msg)
	if rc != 0 {
		if strings.Contains(strings.ToLower(stderr), "nothing to commit") {
			return map[string]interface{}{"status": "info", "message": "Nothing to commit (already staged)."}
		}
		return map[string]interface{}{"status": "error", "message": fmt.Sprintf("Git commit failed: %s", stderr)}
	}

	hash, hashErr := resolveGitHeadHash(dir)
	if hashErr != nil {
		return map[string]interface{}{
			"status":      "success",
			"message":     fmt.Sprintf("Created backup commit, but failed to resolve commit hash: %v", hashErr),
			"commit_hash": "",
		}
	}

	return map[string]interface{}{"status": "success", "message": fmt.Sprintf("Created backup commit %s", hash), "commit_hash": hash}
}

func resolveGitHeadHash(dir string) (string, error) {
	stdout, stderr, rc := runGitCmd(dir, "rev-parse", "HEAD")
	if rc != 0 {
		return "", fmt.Errorf("git rev-parse HEAD failed: %s", strings.TrimSpace(stderr))
	}
	hash := strings.TrimSpace(stdout)
	if hash == "" {
		return "", fmt.Errorf("git rev-parse HEAD returned empty output")
	}
	return hash, nil
}

func listBackups(dir string, limit int) map[string]interface{} {
	stdout, stderr, rc := runGitCmd(dir, "log", fmt.Sprintf("--max-count=%d", limit), "--oneline")
	if rc != 0 {
		return map[string]interface{}{"status": "error", "message": fmt.Sprintf("Git log failed: %s", stderr)}
	}

	var commits []map[string]string
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			commits = append(commits, map[string]string{"hash": parts[0], "message": parts[1]})
		}
	}
	return map[string]interface{}{"status": "success", "commits": commits}
}

func restoreGit(dir, hash, mode string) map[string]interface{} {
	if mode == "revert" {
		_, stderr, rc := runGitCmd(dir, "revert", "--no-edit", hash)
		if rc != 0 {
			return map[string]interface{}{"status": "error", "message": fmt.Sprintf("Git revert failed: %s", stderr)}
		}
		return map[string]interface{}{"status": "success", "message": fmt.Sprintf("Reverted commit %s", hash)}
	} else if mode == "checkout" {
		_, stderr, rc := runGitCmd(dir, "reset", "--hard", hash)
		if rc != 0 {
			return map[string]interface{}{"status": "error", "message": fmt.Sprintf("Git reset failed: %s", stderr)}
		}
		return map[string]interface{}{"status": "success", "message": fmt.Sprintf("Checked out commit %s (hard reset)", hash)}
	}
	return map[string]interface{}{"status": "error", "message": fmt.Sprintf("Invalid mode: %s. Use 'revert' or 'checkout'.", mode)}
}

func rollbackToPrevious(dir string) map[string]interface{} {
	stdout, _, rc := runGitCmd(dir, "log", "-1", "--pretty=format:%H")
	if rc != 0 || strings.TrimSpace(stdout) == "" {
		return map[string]interface{}{"status": "error", "message": "No commits found or git log failed"}
	}
	return restoreGit(dir, strings.TrimSpace(stdout), "revert")
}

func showDiff(dir, hash string) map[string]interface{} {
	var args []string
	if hash != "" {
		args = []string{"diff", hash, "HEAD"}
	} else {
		args = []string{"diff", "HEAD"}
	}

	stdout, stderr, rc := runGitCmd(dir, args...)
	if rc != 0 {
		return map[string]interface{}{"status": "error", "message": fmt.Sprintf("Git diff failed: %s", stderr)}
	}
	return map[string]interface{}{"status": "success", "diff": stdout}
}
