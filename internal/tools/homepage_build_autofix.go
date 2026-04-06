package tools

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

// ─── Build Auto-Fix ──────────────────────────────────────────────────────
//
// Pattern-based build error detection with auto-fix and one retry.
// No LLM calls — purely regex-driven.

// buildFixPattern describes a recognizable build error and its fix command.
type buildFixPattern struct {
	name    string
	pattern *regexp.Regexp
	// fix returns the shell command(s) to attempt a fix, given the regex match groups.
	fix func(matches []string, projectDir string) string
}

var buildFixPatterns = []buildFixPattern{
	{
		name:    "missing-npm-module",
		pattern: regexp.MustCompile(`(?i)Cannot find module '([^']+)'`),
		fix: func(m []string, dir string) string {
			mod := m[1]
			// Strip subpath (e.g. 'foo/bar' → 'foo')
			if idx := strings.Index(mod, "/"); idx > 0 && !strings.HasPrefix(mod, "@") {
				mod = mod[:idx]
			}
			// For scoped packages, keep scope + name
			if strings.HasPrefix(mod, "@") {
				parts := strings.SplitN(mod, "/", 3)
				if len(parts) >= 2 {
					mod = parts[0] + "/" + parts[1]
				}
			}
			return fmt.Sprintf("cd /workspace/%s && npm install %s 2>&1", dir, mod)
		},
	},
	{
		name:    "module-not-found-webpack",
		pattern: regexp.MustCompile(`(?i)Module not found:.*Can't resolve '([^']+)'`),
		fix: func(m []string, dir string) string {
			mod := m[1]
			if strings.HasPrefix(mod, ".") || strings.HasPrefix(mod, "/") {
				return "" // relative path — not an installable package
			}
			if idx := strings.Index(mod, "/"); idx > 0 && !strings.HasPrefix(mod, "@") {
				mod = mod[:idx]
			}
			if strings.HasPrefix(mod, "@") {
				parts := strings.SplitN(mod, "/", 3)
				if len(parts) >= 2 {
					mod = parts[0] + "/" + parts[1]
				}
			}
			return fmt.Sprintf("cd /workspace/%s && npm install %s 2>&1", dir, mod)
		},
	},
	{
		name:    "missing-peer-dep",
		pattern: regexp.MustCompile(`(?i)WARN.*requires a peer of ([^ ]+)`),
		fix: func(m []string, dir string) string {
			dep := m[1]
			// Strip version spec
			if idx := strings.Index(dep, "@"); idx > 0 {
				dep = dep[:idx]
			}
			return fmt.Sprintf("cd /workspace/%s && npm install %s 2>&1", dir, dep)
		},
	},
	{
		name:    "eslint-fixable",
		pattern: regexp.MustCompile(`(?i)(\d+) errors? .* potentially fixable with.*--fix`),
		fix: func(_ []string, dir string) string {
			return fmt.Sprintf("cd /workspace/%s && npx eslint . --fix 2>&1 || true", dir)
		},
	},
	{
		name:    "npm-install-needed",
		pattern: regexp.MustCompile(`(?i)npm ERR!.*missing:.*required by`),
		fix: func(_ []string, dir string) string {
			return fmt.Sprintf("cd /workspace/%s && npm install 2>&1", dir)
		},
	},
	{
		name:    "node-modules-missing",
		pattern: regexp.MustCompile(`(?i)Cannot find module.*node_modules`),
		fix: func(_ []string, dir string) string {
			return fmt.Sprintf("cd /workspace/%s && npm install 2>&1", dir)
		},
	},
	// TypeScript-specific: TS2307 module not found → install the package
	{
		name:    "ts-missing-module",
		pattern: regexp.MustCompile(`(?i)error TS2307: Cannot find module '([^']+)'`),
		fix: func(m []string, dir string) string {
			mod := m[1]
			if strings.HasPrefix(mod, ".") || strings.HasPrefix(mod, "/") {
				return "" // relative path — not an installable package
			}
			if idx := strings.Index(mod, "/"); idx > 0 && !strings.HasPrefix(mod, "@") {
				mod = mod[:idx]
			}
			if strings.HasPrefix(mod, "@") {
				parts := strings.SplitN(mod, "/", 3)
				if len(parts) >= 2 {
					mod = parts[0] + "/" + parts[1]
				}
			}
			return fmt.Sprintf("cd /workspace/%s && npm install %s 2>&1", dir, mod)
		},
	},
	// TypeScript: missing @types declaration → install @types/package
	{
		name:    "ts-missing-types",
		pattern: regexp.MustCompile(`(?i)Could not find a declaration file for module '([^']+)'`),
		fix: func(m []string, dir string) string {
			mod := m[1]
			if strings.HasPrefix(mod, ".") || strings.HasPrefix(mod, "/") {
				return "" // relative path
			}
			if idx := strings.Index(mod, "/"); idx > 0 && !strings.HasPrefix(mod, "@") {
				mod = mod[:idx]
			}
			return fmt.Sprintf("cd /workspace/%s && npm install --save-dev @types/%s 2>&1 || true", dir, mod)
		},
	},
}

// HomepageBuildWithAutoFix runs the build, and if it fails, tries pattern-based
// auto-fixes with exactly one retry. Returns the final build result.
func HomepageBuildWithAutoFix(cfg HomepageConfig, projectDir string, logger *slog.Logger) string {
	// First build attempt
	result := HomepageBuild(cfg, projectDir, logger)

	// Check if the build succeeded
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(result), &resp); err == nil {
		if status, _ := resp["status"].(string); status == "ok" {
			return result
		}
	}

	// Extract the output for error pattern matching
	output := extractOutput(result)
	if output == "" {
		output = result
	}

	// Try to find a matching fix pattern
	var fixCmd string
	var fixName string
	for _, p := range buildFixPatterns {
		matches := p.pattern.FindStringSubmatch(output)
		if matches != nil {
			candidate := p.fix(matches, projectDir)
			if candidate != "" {
				fixCmd = candidate
				fixName = p.name
				break
			}
		}
	}

	if fixCmd == "" {
		// No fixable pattern recognized — return original error
		return result
	}

	logger.Info("[Homepage] Auto-fix: detected fixable error, applying fix",
		"pattern", fixName, "project_dir", projectDir)

	// Apply the fix inside the Docker container
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	fixResult := DockerExec(dockerCfg, homepageContainerName, fixCmd, "")
	fixOutput := extractOutput(fixResult)

	logger.Info("[Homepage] Auto-fix: fix applied, retrying build",
		"pattern", fixName, "fix_output_len", len(fixOutput))

	// Retry the build exactly once
	retryResult := HomepageBuild(cfg, projectDir, logger)

	// Annotate the retry result with auto-fix info
	var retryResp map[string]interface{}
	if err := json.Unmarshal([]byte(retryResult), &retryResp); err == nil {
		retryResp["auto_fix_applied"] = true
		retryResp["auto_fix_pattern"] = fixName
		retryResp["auto_fix_output"] = truncateStr(fixOutput, 500)
		annotated, _ := json.Marshal(retryResp)
		return string(annotated)
	}

	return retryResult
}
