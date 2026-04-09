package tools

import (
	"fmt"
	"log/slog"
	"strings"
)

// ─── Git Version Control for Homepage Projects ───────────────────────────
//
// All git operations run inside the homepage Docker container where git is
// pre-installed. The project directory is relative to /workspace.

// HomepageGitInit initializes a git repository in the project directory.
func HomepageGitInit(cfg HomepageConfig, projectDir string, logger *slog.Logger) string {
	if projectDir == "" {
		projectDir = "."
	}
	if projectDir != "." {
		if err := sanitizeProjectDir(projectDir); err != nil {
			return errJSON("%v", err)
		}
	}
	logger.Info("[Homepage] Git init", "dir", projectDir)
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	// Use git -C to specify the working tree without shell cd, and properly quote projectDir.
	// shellSingleQuote is defined in homepage_deploy.go and accessible within this package.
	cmd := fmt.Sprintf("git -C /workspace/%s init -b main && git -C /workspace/%s config user.email 'aurago@local' && git -C /workspace/%s config user.name 'Aurago Agent' && git -C /workspace/%s add -A && git -C /workspace/%s commit -m 'Initial commit' --allow-empty 2>&1",
		shellSingleQuote(projectDir), shellSingleQuote(projectDir), shellSingleQuote(projectDir), shellSingleQuote(projectDir), shellSingleQuote(projectDir))
	return DockerExec(dockerCfg, homepageContainerName, cmd, "")
}

// HomepageGitCommit stages all changes and commits with the given message.
func HomepageGitCommit(cfg HomepageConfig, projectDir, message string, logger *slog.Logger) string {
	if projectDir == "" {
		projectDir = "."
	}
	if projectDir != "." {
		if err := sanitizeProjectDir(projectDir); err != nil {
			return errJSON("%v", err)
		}
	}
	if message == "" {
		message = "Update"
	}
	// Sanitize commit message for shell safety: replace single quotes
	safeMsg := strings.ReplaceAll(message, "'", "'\\''")
	// Additional shell metacharacter check
	for _, c := range safeMsg {
		switch c {
		case ';', '|', '&', '`', '$', '(', ')', '{', '}', '<', '>', '\\':
			return errJSON("commit message contains invalid characters")
		}
	}
	logger.Info("[Homepage] Git commit", "dir", projectDir, "message", message)
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	// Use git -C for safe path handling and properly quote projectDir.
	cmd := fmt.Sprintf("git -C /workspace/%s add -A && git -C /workspace/%s diff --cached --stat && git -C /workspace/%s commit -m %s 2>&1",
		shellSingleQuote(projectDir), shellSingleQuote(projectDir), shellSingleQuote(projectDir), shellSingleQuote(safeMsg))
	return DockerExec(dockerCfg, homepageContainerName, cmd, "")
}

// HomepageGitStatus returns the git status (porcelain format) for the project.
func HomepageGitStatus(cfg HomepageConfig, projectDir string, logger *slog.Logger) string {
	if projectDir == "" {
		projectDir = "."
	}
	if projectDir != "." {
		if err := sanitizeProjectDir(projectDir); err != nil {
			return errJSON("%v", err)
		}
	}
	logger.Info("[Homepage] Git status", "dir", projectDir)
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	// Use git -C for safe path handling and properly quote projectDir.
	cmd := fmt.Sprintf("git -C /workspace/%s status --short 2>&1", shellSingleQuote(projectDir))
	return DockerExec(dockerCfg, homepageContainerName, cmd, "")
}

// HomepageGitDiff returns the current diff for the project.
func HomepageGitDiff(cfg HomepageConfig, projectDir string, logger *slog.Logger) string {
	if projectDir == "" {
		projectDir = "."
	}
	if projectDir != "." {
		if err := sanitizeProjectDir(projectDir); err != nil {
			return errJSON("%v", err)
		}
	}
	logger.Info("[Homepage] Git diff", "dir", projectDir)
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	// Use git -C for safe path handling and properly quote projectDir.
	cmd := fmt.Sprintf("git -C /workspace/%s diff --stat && echo '---' && git -C /workspace/%s diff 2>&1 | head -500", shellSingleQuote(projectDir), shellSingleQuote(projectDir))
	return DockerExec(dockerCfg, homepageContainerName, cmd, "")
}

// HomepageGitLog returns the last N commits in oneline format.
func HomepageGitLog(cfg HomepageConfig, projectDir string, count int, logger *slog.Logger) string {
	if projectDir == "" {
		projectDir = "."
	}
	if projectDir != "." {
		if err := sanitizeProjectDir(projectDir); err != nil {
			return errJSON("%v", err)
		}
	}
	if count <= 0 {
		count = 10
	}
	if count > 100 {
		count = 100
	}
	logger.Info("[Homepage] Git log", "dir", projectDir, "count", count)
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	// Use git -C for safe path handling and properly quote projectDir.
	cmd := fmt.Sprintf("git -C /workspace/%s log --oneline --graph -n %d 2>&1", shellSingleQuote(projectDir), count)
	return DockerExec(dockerCfg, homepageContainerName, cmd, "")
}

// HomepageGitRollback reverts the last N commits by creating new revert commits.
// This is a safe approach — it never rewrites history.
func HomepageGitRollback(cfg HomepageConfig, projectDir string, steps int, logger *slog.Logger) string {
	if projectDir == "" {
		projectDir = "."
	}
	if projectDir != "." {
		if err := sanitizeProjectDir(projectDir); err != nil {
			return errJSON("%v", err)
		}
	}
	if steps <= 0 {
		steps = 1
	}
	if steps > 10 {
		return errJSON("Maximum rollback is 10 commits at once")
	}
	logger.Info("[Homepage] Git rollback", "dir", projectDir, "steps", steps)
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	// Revert commits one by one (newest first) to avoid merge conflicts.
	// Use git -C for safe path handling and properly quote projectDir.
	cmd := fmt.Sprintf("for i in $(seq 0 %d); do git -C /workspace/%s revert --no-edit HEAD~$i 2>&1 || break; done && git -C /workspace/%s log --oneline -n %d 2>&1",
		steps-1, shellSingleQuote(projectDir), shellSingleQuote(projectDir), steps+2)
	return DockerExec(dockerCfg, homepageContainerName, cmd, "")
}
