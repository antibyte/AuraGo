package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type homepageProjectInfo struct {
	ProjectDir     string
	Root           string
	Framework      string
	PackageManager string
	HasPackageJSON bool
}

type homepagePrepareResult struct {
	Status         string `json:"status"`
	Message        string `json:"message,omitempty"`
	PackageManager string `json:"package_manager,omitempty"`
	InstallCommand string `json:"install_command,omitempty"`
	InstallRan     bool   `json:"install_ran"`
	Output         string `json:"output,omitempty"`
}

type homepageDeployCandidate struct {
	BuildDir        string   `json:"build_dir"`
	Path            string   `json:"path"`
	ContainerSubdir string   `json:"container_subdir"`
	Kind            string   `json:"kind"`
	HasIndex        bool     `json:"has_index"`
	Warnings        []string `json:"warnings,omitempty"`
}

type homepageVerifyResult struct {
	Status        string   `json:"status"`
	Message       string   `json:"message,omitempty"`
	URL           string   `json:"url,omitempty"`
	HTTPStatus    int      `json:"http_status,omitempty"`
	CheckedAssets []string `json:"checked_assets,omitempty"`
}

var homepageVerifyHTTPClient = &http.Client{Timeout: 12 * time.Second}

func homepageResolveProject(cfg HomepageConfig, projectDir string) (homepageProjectInfo, error) {
	projectDir = strings.TrimSpace(projectDir)
	if projectDir == "" {
		projectDir = "."
	}
	if projectDir != "." {
		if err := sanitizeProjectDir(projectDir); err != nil {
			return homepageProjectInfo{}, err
		}
	}
	if strings.TrimSpace(cfg.WorkspacePath) == "" {
		return homepageProjectInfo{}, fmt.Errorf("workspace_path is required")
	}
	root := filepath.Join(cfg.WorkspacePath, filepath.FromSlash(projectDir))
	info, err := os.Stat(root)
	if err != nil {
		return homepageProjectInfo{}, fmt.Errorf("project_dir %q not found: %w", projectDir, err)
	}
	if !info.IsDir() {
		return homepageProjectInfo{}, fmt.Errorf("project_dir %q is not a directory", projectDir)
	}
	hasPackageJSON := fileExists(filepath.Join(root, "package.json"))
	framework := detectHomepageFramework(root)
	pm := ""
	if hasPackageJSON {
		pm = homepageDetectPackageManager(root)
	}
	return homepageProjectInfo{
		ProjectDir:     filepath.ToSlash(filepath.Clean(projectDir)),
		Root:           root,
		Framework:      framework,
		PackageManager: pm,
		HasPackageJSON: hasPackageJSON,
	}, nil
}

func homepageDetectPackageManager(projectRoot string) string {
	switch {
	case fileExists(filepath.Join(projectRoot, "pnpm-lock.yaml")):
		return "pnpm"
	case fileExists(filepath.Join(projectRoot, "yarn.lock")):
		return "yarn"
	case fileExists(filepath.Join(projectRoot, "bun.lockb")), fileExists(filepath.Join(projectRoot, "bun.lock")):
		return "bun"
	default:
		return "npm"
	}
}

func homepageDependencyArtifactsPresent(projectRoot, packageManager string) bool {
	if !fileExists(filepath.Join(projectRoot, "package.json")) {
		return true
	}
	if packageManager == "yarn" && fileExists(filepath.Join(projectRoot, ".pnp.cjs")) {
		return true
	}
	return dirExists(filepath.Join(projectRoot, "node_modules"))
}

func homepageInstallCommand(packageManager, projectRoot string) string {
	switch packageManager {
	case "pnpm":
		return "corepack enable >/dev/null 2>&1 || true; pnpm install --frozen-lockfile 2>&1 || pnpm install 2>&1"
	case "yarn":
		return "corepack enable >/dev/null 2>&1 || true; yarn install --immutable 2>&1 || yarn install 2>&1"
	case "bun":
		return "if command -v bun >/dev/null 2>&1; then bun install 2>&1; else npm install 2>&1; fi"
	default:
		if fileExists(filepath.Join(projectRoot, "package-lock.json")) {
			return "npm ci 2>&1"
		}
		return "npm install 2>&1"
	}
}

func homepageBuildCommand(packageManager string) string {
	switch packageManager {
	case "pnpm":
		return "pnpm run build"
	case "yarn":
		return "yarn build"
	case "bun":
		return "bun run build"
	default:
		return "npm run build"
	}
}

func homepagePrepareDependencies(cfg HomepageConfig, project homepageProjectInfo, logger *slog.Logger) homepagePrepareResult {
	if !project.HasPackageJSON {
		return homepagePrepareResult{Status: "ok", Message: "No package.json found; dependency install skipped"}
	}
	pm := project.PackageManager
	if pm == "" {
		pm = homepageDetectPackageManager(project.Root)
	}
	if homepageDependencyArtifactsPresent(project.Root, pm) {
		return homepagePrepareResult{
			Status:         "ok",
			Message:        "Dependencies already present",
			PackageManager: pm,
			InstallRan:     false,
		}
	}
	if err := homepageEnsureNodeRuntime(cfg, logger); err != nil {
		return homepagePrepareResult{
			Status:         "error",
			Message:        err.Error(),
			PackageManager: pm,
			InstallRan:     false,
		}
	}
	cmd := homepageInstallCommand(pm, project.Root)
	if logger != nil {
		logger.Info("[Homepage] Installing missing dependencies", "project_dir", project.ProjectDir, "package_manager", pm)
	}
	raw := homepageDockerExecFunc(DockerConfig{Host: cfg.DockerHost}, homepageContainerName, fmt.Sprintf("cd /workspace/%s && %s", project.ProjectDir, cmd), "")
	var parsed struct {
		ExitCode int    `json:"exit_code"`
		Output   string `json:"output"`
		Status   string `json:"status"`
		Message  string `json:"message"`
	}
	_ = json.Unmarshal([]byte(raw), &parsed)
	if parsed.ExitCode != 0 || parsed.Status == "error" {
		msg := strings.TrimSpace(parsed.Message)
		if msg == "" {
			msg = strings.TrimSpace(parsed.Output)
		}
		if msg == "" {
			msg = "dependency install failed"
		}
		return homepagePrepareResult{
			Status:         "error",
			Message:        msg,
			PackageManager: pm,
			InstallCommand: cmd,
			InstallRan:     true,
			Output:         parsed.Output,
		}
	}
	return homepagePrepareResult{
		Status:         "ok",
		Message:        "Dependencies installed",
		PackageManager: pm,
		InstallCommand: cmd,
		InstallRan:     true,
		Output:         parsed.Output,
	}
}

func homepageDetectDeployCandidate(cfg HomepageConfig, projectDir, buildDir, framework string) (homepageDeployCandidate, error) {
	projectDir = strings.TrimSpace(projectDir)
	if projectDir == "" {
		projectDir = "."
	}
	if projectDir != "." {
		if err := sanitizeProjectDir(projectDir); err != nil {
			return homepageDeployCandidate{}, err
		}
	}
	projectRoot := filepath.Join(cfg.WorkspacePath, filepath.FromSlash(projectDir))
	if framework == "" {
		framework = detectHomepageFramework(projectRoot)
	}
	tryDirs := []string{}
	if strings.TrimSpace(buildDir) != "" {
		tryDirs = append(tryDirs, strings.TrimSpace(buildDir))
	} else {
		tryDirs = append(tryDirs, "out", "dist", "build", ".next", "public", ".")
	}
	for _, dir := range tryDirs {
		if dir != "." && strings.Contains(filepath.ToSlash(filepath.Clean(dir)), "..") {
			continue
		}
		full := filepath.Join(projectRoot, filepath.FromSlash(dir))
		if !dirExists(full) {
			continue
		}
		indexPath := filepath.Join(full, "index.html")
		if !fileExists(indexPath) {
			continue
		}
		stat, _ := os.Stat(indexPath)
		warnings := []string{}
		if stat != nil && time.Since(stat.ModTime()) > 24*time.Hour {
			warnings = append(warnings, "index.html is older than 24 hours; verify that the build output is fresh")
		}
		containerSubdir := projectDir
		if dir != "." {
			containerSubdir = pathJoinSlash(projectDir, dir)
		}
		return homepageDeployCandidate{
			BuildDir:        filepath.ToSlash(filepath.Clean(dir)),
			Path:            full,
			ContainerSubdir: containerSubdir,
			Kind:            homepageCandidateKind(framework, dir),
			HasIndex:        true,
			Warnings:        warnings,
		}, nil
	}
	if strings.TrimSpace(buildDir) != "" {
		return homepageDeployCandidate{}, fmt.Errorf("build_dir %q is not a deployable static output: missing index.html", buildDir)
	}
	return homepageDeployCandidate{}, fmt.Errorf("no deployable static output found for %q; expected index.html in dist, build, out, public, or project root", projectDir)
}

func homepageCandidateKind(framework, buildDir string) string {
	framework = strings.ToLower(strings.TrimSpace(framework))
	if framework == "next" || framework == "nextjs" || buildDir == ".next" {
		return "next"
	}
	switch framework {
	case "vite", "react", "vue", "svelte", "astro", "nuxt":
		return "spa"
	default:
		return "static"
	}
}

func homepageVercelDeploySubdir(projectDir, framework, explicitBuildDir string, candidate homepageDeployCandidate, hasPackageJSON ...bool) (string, bool) {
	projectDir = strings.TrimSpace(projectDir)
	if projectDir == "" {
		projectDir = "."
	}
	if len(hasPackageJSON) > 0 && hasPackageJSON[0] {
		return filepath.ToSlash(filepath.Clean(projectDir)), false
	}
	if strings.TrimSpace(explicitBuildDir) != "" && candidate.ContainerSubdir != "" {
		return candidate.ContainerSubdir, true
	}
	return filepath.ToSlash(filepath.Clean(projectDir)), false
}

func homepageVerifyDeploymentURL(rawURL string) homepageVerifyResult {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return homepageVerifyResult{Status: "error", Message: "deployment URL is empty"}
	}
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Host == "" {
		return homepageVerifyResult{Status: "error", Message: "deployment URL is invalid", URL: rawURL}
	}
	resp, err := homepageVerifyHTTPClient.Get(parsedURL.String())
	if err != nil {
		return homepageVerifyResult{Status: "error", Message: fmt.Sprintf("root request failed: %v", err), URL: parsedURL.String()}
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	body := string(bodyBytes)
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return homepageVerifyResult{Status: "error", Message: fmt.Sprintf("root returned HTTP %d", resp.StatusCode), URL: parsedURL.String(), HTTPStatus: resp.StatusCode}
	}
	lower := strings.ToLower(body)
	errorNeedles := []string{"deployment_not_found", "404: not_found", "function_invocation_failed", "application error", "site not found", "page not found"}
	for _, needle := range errorNeedles {
		if strings.Contains(lower, needle) {
			return homepageVerifyResult{Status: "error", Message: fmt.Sprintf("provider error page detected: %s", needle), URL: parsedURL.String(), HTTPStatus: resp.StatusCode}
		}
	}
	assets := homepageExtractCriticalAssets(body, parsedURL)
	checked := []string{}
	for _, asset := range assets {
		req, _ := http.NewRequest(http.MethodGet, asset, nil)
		assetResp, err := homepageVerifyHTTPClient.Do(req)
		if err != nil {
			return homepageVerifyResult{Status: "error", Message: fmt.Sprintf("asset request failed for %s: %v", asset, err), URL: parsedURL.String(), HTTPStatus: resp.StatusCode, CheckedAssets: checked}
		}
		_ = assetResp.Body.Close()
		checked = append(checked, asset)
		if assetResp.StatusCode >= 400 {
			return homepageVerifyResult{Status: "error", Message: fmt.Sprintf("asset %s returned HTTP %d", asset, assetResp.StatusCode), URL: parsedURL.String(), HTTPStatus: resp.StatusCode, CheckedAssets: checked}
		}
	}
	return homepageVerifyResult{Status: "ok", Message: "Deployment URL verified", URL: parsedURL.String(), HTTPStatus: resp.StatusCode, CheckedAssets: checked}
}

func homepageExtractCriticalAssets(html string, base *url.URL) []string {
	re := regexp.MustCompile(`(?i)(?:src|href)=["']([^"']+\.(?:js|css)(?:\?[^"']*)?)["']`)
	matches := re.FindAllStringSubmatch(html, -1)
	seen := map[string]bool{}
	assets := []string{}
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		ref, err := url.Parse(strings.TrimSpace(match[1]))
		if err != nil {
			continue
		}
		resolved := base.ResolveReference(ref).String()
		if seen[resolved] {
			continue
		}
		seen[resolved] = true
		assets = append(assets, resolved)
		if len(assets) >= 8 {
			break
		}
	}
	return assets
}

func pathJoinSlash(parts ...string) string {
	clean := []string{}
	for _, part := range parts {
		part = strings.Trim(strings.TrimSpace(filepath.ToSlash(part)), "/")
		if part != "" && part != "." {
			clean = append(clean, part)
		}
	}
	if len(clean) == 0 {
		return "."
	}
	return strings.Join(clean, "/")
}

func dirExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func homepageProviderNameFromProjectDir(projectDir string) string {
	projectDir = strings.Trim(strings.ToLower(filepath.Base(filepath.ToSlash(projectDir))), ". ")
	if projectDir == "" || projectDir == "/" {
		projectDir = "aurago-homepage"
	}
	re := regexp.MustCompile(`[^a-z0-9-]+`)
	name := re.ReplaceAllString(projectDir, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		return "aurago-homepage"
	}
	return name
}
