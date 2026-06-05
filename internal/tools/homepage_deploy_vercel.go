package tools

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

func normalizeVercelDeployTarget(target string) string {
	target = strings.ToLower(strings.TrimSpace(target))
	switch target {
	case "", "preview", "staging":
		return "preview"
	case "prod", "production":
		return "production"
	default:
		return target
	}
}

func detectHomepageFramework(projectRoot string) string {
	switch {
	case isNextJsProject(projectRoot):
		return "nextjs"
	case fileExists(filepath.Join(projectRoot, "astro.config.mjs")) || fileExists(filepath.Join(projectRoot, "astro.config.ts")) || fileExists(filepath.Join(projectRoot, "astro.config.js")):
		return "astro"
	case fileExists(filepath.Join(projectRoot, "nuxt.config.ts")) || fileExists(filepath.Join(projectRoot, "nuxt.config.js")):
		return "nuxtjs"
	case fileExists(filepath.Join(projectRoot, "vite.config.ts")) || fileExists(filepath.Join(projectRoot, "vite.config.js")) || fileExists(filepath.Join(projectRoot, "vite.config.mjs")):
		return "vite"
	case fileExists(filepath.Join(projectRoot, "package.json")):
		return "other"
	default:
		return "other"
	}
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func extractVercelDeploymentURL(output string) string {
	for _, field := range strings.Fields(output) {
		field = strings.TrimSpace(strings.TrimRight(field, ".,;\"')"))
		if strings.HasPrefix(field, "https://") && strings.Contains(field, ".vercel.app") {
			return field
		}
	}
	return ""
}

func parseToolJSON(raw string) map[string]interface{} {
	var out map[string]interface{}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

func vercelHTTPCode(result map[string]interface{}) int {
	switch value := result["http_code"].(type) {
	case float64:
		return int(value)
	case int:
		return value
	default:
		return 0
	}
}

func vercelScopeFlag(cfg VercelConfig) string {
	if id := strings.TrimSpace(cfg.TeamID); id != "" {
		return " --scope " + shellSingleQuote(id)
	}
	if slug := strings.TrimSpace(cfg.TeamSlug); slug != "" {
		return " --scope " + shellSingleQuote(slug)
	}
	return ""
}

func buildVercelDeployCommand(deploySubdir, projectRef, target string, cfg VercelConfig) string {
	scopeFlag := vercelScopeFlag(cfg)
	linkPrefix := ""
	if projectRef = strings.TrimSpace(projectRef); projectRef != "" {
		linkPrefix = fmt.Sprintf("vercel link --yes --token $VERCEL_TOKEN --project %s%s && ",
			shellSingleQuote(projectRef), scopeFlag)
	}
	return fmt.Sprintf("cd /workspace/%s && %svercel deploy --yes --archive=tgz --token $VERCEL_TOKEN --target=%s%s 2>&1",
		deploySubdir, linkPrefix, shellSingleQuote(target), scopeFlag)
}

func HomepageDeployVercel(cfg HomepageConfig, vcfg VercelConfig, projectDir, buildDir, projectID, target, alias, domain string, allowProjectManagement, allowDomainManagement bool, logger *slog.Logger) string {
	if strings.TrimSpace(vcfg.Token) == "" {
		return errJSON("Vercel token is required")
	}
	if cfg.WorkspacePath == "" {
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

	projectRoot := filepath.Join(cfg.WorkspacePath, projectDir)
	if _, err := os.Stat(projectRoot); err != nil {
		return errJSON("Project directory does not exist: %s", projectRoot)
	}

	target = normalizeVercelDeployTarget(target)
	framework := detectHomepageFramework(projectRoot)
	projectInfo, projectInfoErr := homepageResolveProject(cfg, projectDir)
	if projectInfoErr != nil {
		return errJSON("%v", projectInfoErr)
	}

	if projectInfo.HasPackageJSON {
		logger.Info("[Homepage] Attempting build before Vercel deploy", "dir", projectDir, "target", target)
		buildResult := HomepageBuildWithAutoFix(cfg, projectDir, logger)
		var br map[string]interface{}
		if err := json.Unmarshal([]byte(buildResult), &br); err == nil {
			if s, _ := br["status"].(string); s == "error" {
				return decorateHomepageBuildFailure(buildResult, projectDir)
			}
		}
	}

	candidate := homepageDeployCandidate{
		BuildDir:        ".",
		Path:            projectRoot,
		ContainerSubdir: projectDir,
		Kind:            "framework-source",
	}
	if strings.TrimSpace(buildDir) != "" && buildDir != "." && !projectInfo.HasPackageJSON {
		var candidateErr error
		candidate, candidateErr = homepageDetectDeployCandidate(cfg, projectDir, buildDir, framework)
		if candidateErr != nil {
			return errJSON("No valid Vercel static deploy output for %q: %v", projectDir, candidateErr)
		}
	} else if !projectInfo.HasPackageJSON {
		var candidateErr error
		candidate, candidateErr = homepageDetectDeployCandidate(cfg, projectDir, ".", framework)
		if candidateErr != nil {
			return errJSON("No valid Vercel static deploy output for %q: %v", projectDir, candidateErr)
		}
	}
	deploySubdir, staticOutputDeploy := homepageVercelDeploySubdir(projectDir, framework, buildDir, candidate, projectInfo.HasPackageJSON)
	if !projectInfo.HasPackageJSON {
		staticOutputDeploy = true
	}
	deployPath := filepath.Join(cfg.WorkspacePath, filepath.FromSlash(deploySubdir))
	if _, err := os.Stat(deployPath); err != nil {
		return errJSON("Deploy path does not exist: %s", deployPath)
	}

	projectRef := strings.TrimSpace(projectID)
	if projectRef == "" {
		projectRef = strings.TrimSpace(vcfg.DefaultProjectID)
	}
	if projectRef == "" && projectDir != "." {
		projectRef = filepath.Base(projectDir)
	}
	if projectRef == "" {
		projectRef = "aurago-homepage"
	}

	projectResult := parseToolJSON(VercelGetProject(vcfg, projectRef))
	projectExists := projectResult["status"] == "ok"
	if !projectExists {
		httpCode := vercelHTTPCode(projectResult)
		if httpCode != 404 && httpCode != 0 {
			msg, _ := projectResult["message"].(string)
			if msg == "" {
				msg = "Failed to look up Vercel project"
			}
			return errJSON("%s", msg)
		}
		if !allowProjectManagement {
			return errJSON("Vercel project %q was not found. Set vercel.default_project_id, pass project_id, or enable vercel.allow_project_management so AuraGo may create the project automatically.", projectRef)
		}
		createRaw := VercelCreateProject(vcfg, projectRef, framework, "", "")
		createResult := parseToolJSON(createRaw)
		if createResult["status"] != "ok" {
			msg, _ := createResult["message"].(string)
			if msg == "" {
				msg = createRaw
			}
			return errJSON("Failed to create Vercel project %q: %s", projectRef, truncateStr(msg, 500))
		}
		projectExists = true
		projectResult = createResult
	}

	cliProjectRef := projectRef
	if projectObj, ok := projectResult["project"].(map[string]interface{}); ok {
		if name := strVal(projectObj, "name"); name != "" {
			cliProjectRef = name
		}
		if id := strVal(projectObj, "id"); id != "" {
			projectRef = id
		} else if name := strVal(projectObj, "name"); name != "" {
			projectRef = name
		}
	}

	logger.Info("[Homepage] Deploying to Vercel", "project", projectRef, "path", deploySubdir, "target", target, "static_output", staticOutputDeploy)

	scopeFlag := vercelScopeFlag(vcfg)
	deployCmd := buildVercelDeployCommand(deploySubdir, cliProjectRef, target, vcfg)
	deployRaw := HomepageExec(cfg, deployCmd, []string{"VERCEL_TOKEN=" + vcfg.Token}, logger)
	deployOutput := extractOutput(deployRaw)

	var deployResp map[string]interface{}
	if err := json.Unmarshal([]byte(deployRaw), &deployResp); err == nil {
		if code, _ := deployResp["exit_code"].(float64); code != 0 {
			return errJSON("Vercel deployment failed for project %q. Output: %s", projectRef, truncateStr(strings.TrimSpace(deployOutput), 1200))
		}
	}

	deploymentURL := extractVercelDeploymentURL(deployOutput)
	if deploymentURL == "" {
		return errJSON("Vercel deploy finished but no deployment URL could be detected. Output: %s", truncateStr(strings.TrimSpace(deployOutput), 1200))
	}
	verify := homepageVerifyDeploymentURL(deploymentURL)
	if verify.Status != "ok" {
		return errJSON("Vercel deploy completed but live verification failed: %s", verify.Message)
	}

	aliasResults := []map[string]interface{}{}
	aliasTargets := []string{}
	for _, candidate := range []string{alias, domain} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		alreadyQueued := false
		for _, existing := range aliasTargets {
			if existing == candidate {
				alreadyQueued = true
				break
			}
		}
		if !alreadyQueued {
			aliasTargets = append(aliasTargets, candidate)
		}
	}
	if len(aliasTargets) > 0 && !allowDomainManagement {
		return errJSON("Alias or domain assignment for Vercel requires vercel.allow_domain_management=true.")
	}

	for _, candidate := range aliasTargets {
		if strings.EqualFold(candidate, strings.TrimSpace(domain)) {
			addResult := parseToolJSON(VercelAddDomain(vcfg, projectRef, candidate))
			addStatus, _ := addResult["status"].(string)
			if addStatus != "ok" {
				httpCode := vercelHTTPCode(addResult)
				msg, _ := addResult["message"].(string)
				if httpCode != 409 && !strings.Contains(strings.ToLower(msg), "already") {
					return errJSON("Vercel domain %q could not be added to project %q: %s", candidate, projectRef, truncateStr(msg, 500))
				}
			}
			verifyResult := parseToolJSON(VercelVerifyDomain(vcfg, projectRef, candidate))
			aliasResults = append(aliasResults, map[string]interface{}{
				"type":   "domain",
				"value":  candidate,
				"verify": verifyResult,
			})
		}

		aliasCmd := fmt.Sprintf("cd /workspace/%s && vercel alias set %s %s --token $VERCEL_TOKEN%s 2>&1",
			deploySubdir, shellSingleQuote(deploymentURL), shellSingleQuote(candidate), scopeFlag)
		aliasRaw := HomepageExec(cfg, aliasCmd, []string{"VERCEL_TOKEN=" + vcfg.Token}, logger)
		aliasOutput := extractOutput(aliasRaw)
		var aliasResp map[string]interface{}
		if err := json.Unmarshal([]byte(aliasRaw), &aliasResp); err == nil {
			if code, _ := aliasResp["exit_code"].(float64); code != 0 {
				return errJSON("Vercel deployment succeeded, but alias assignment for %q failed. Output: %s", candidate, truncateStr(strings.TrimSpace(aliasOutput), 1200))
			}
		}
		aliasResults = append(aliasResults, map[string]interface{}{
			"type":   "alias",
			"value":  candidate,
			"output": strings.TrimSpace(aliasOutput),
		})
	}

	deploymentDetails := parseToolJSON(VercelListDeployments(vcfg, projectRef))
	deploymentID := ""
	if deployments, ok := deploymentDetails["deployments"].([]interface{}); ok {
		for _, item := range deployments {
			deployment, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			urlStr := strVal(deployment, "url")
			if urlStr != "" && !strings.HasPrefix(urlStr, "https://") {
				urlStr = "https://" + urlStr
			}
			if urlStr == deploymentURL {
				deploymentID = strVal(deployment, "id")
				break
			}
		}
	}

	result := map[string]interface{}{
		"status":      "ok",
		"message":     "Vercel deployment complete",
		"project_id":  projectRef,
		"project_dir": projectDir,
		"deploy_path": filepath.ToSlash(deploySubdir),
		"deploy_strategy": func() string {
			if staticOutputDeploy {
				return "static-output"
			}
			return "framework-source"
		}(),
		"target":         target,
		"deployment_url": deploymentURL,
		"verified":       true,
		"aliases":        aliasResults,
		"output":         strings.TrimSpace(deployOutput),
	}
	if deploymentID != "" {
		result["deployment_id"] = deploymentID
	}
	out, _ := json.Marshal(result)
	return string(out)
}

// HomepagePublishToLocal rebuilds and refreshes the local web server.
// For plain HTML projects (no build script), the build step is skipped and
// the project directory is served directly.
