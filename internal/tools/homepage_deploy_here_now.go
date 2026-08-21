package tools

import (
	"context"
	"encoding/json"
	"log/slog"
	"path/filepath"
	"strings"
)

// HomepageDeployHereNow builds a Homepage project when needed and publishes its
// static output through the authenticated native here.now client.
func HomepageDeployHereNow(ctx context.Context, cfg HomepageConfig, hnCfg HereNowConfig, projectDir, buildDir string, opts HereNowPublishOptions, logger *slog.Logger) string {
	if hnCfg.ReadOnly || !hnCfg.AllowPublish {
		return errJSON("here.now publishing is disabled by configuration")
	}
	if strings.TrimSpace(hnCfg.APIKey) == "" {
		return errJSON("here.now API key is required")
	}
	if strings.TrimSpace(cfg.WorkspacePath) == "" {
		return errJSON("Homepage workspace path is not configured")
	}
	if projectDir == "" {
		projectDir = "."
	}
	if projectDir != "." {
		if err := sanitizeProjectDir(projectDir); err != nil {
			return errJSON("%v", err)
		}
	}
	if buildDir != "" && buildDir != "." {
		if err := sanitizeProjectDir(buildDir); err != nil {
			return errJSON("%v", err)
		}
	}

	if buildDir == "" {
		projectRoot := filepath.Join(cfg.WorkspacePath, filepath.FromSlash(projectDir))
		if isNextJsProject(projectRoot) {
			_ = ensureNextJsStaticExport(projectRoot, logger)
		}
		buildResult := HomepageBuildWithAutoFix(cfg, projectDir, logger)
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(buildResult), &parsed); err == nil {
			if status, _ := parsed["status"].(string); status == "error" {
				if fallbackDir, candidate, ok := homepageNetlifyStaticFallbackCandidate(cfg, projectDir, logger); ok {
					projectDir = fallbackDir
					buildDir = candidate.BuildDir
				} else {
					return decorateHomepageBuildFailure(buildResult, projectDir)
				}
			} else {
				buildDir = detectBuildDir(cfg, projectDir)
			}
		}
	}
	if buildDir == ".next" {
		projectRoot := filepath.Join(cfg.WorkspacePath, filepath.FromSlash(projectDir))
		_ = ensureNextJsStaticExport(projectRoot, logger)
		if rebuilt := HomepageBuildWithAutoFix(cfg, projectDir, logger); strings.Contains(rebuilt, `"status":"error"`) {
			return decorateHomepageBuildFailure(rebuilt, projectDir)
		}
		buildDir = detectBuildDir(cfg, projectDir)
	}

	candidate, err := homepageDetectDeployCandidate(cfg, projectDir, buildDir, "")
	if err != nil {
		return errJSON("No valid here.now deploy output for %q: %v. here.now publishing requires a static directory with index.html.", projectDir, err)
	}
	client, err := NewHereNowClient(hnCfg.APIKey, hnCfg.DefaultAccount)
	if err != nil {
		return errJSON("%v", err)
	}
	result, err := client.PublishWorkspaceDirectory(ctx, cfg.WorkspacePath, candidate.ContainerSubdir, opts)
	if err != nil {
		return encodeHereNowOutput(nil, err)
	}
	result.ProjectDir = projectDir
	result.BuildDir = candidate.BuildDir
	encoded, err := json.Marshal(result)
	if err != nil {
		return errJSON("encode here.now deployment result: %v", err)
	}
	return string(encoded)
}
