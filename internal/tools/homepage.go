package tools

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// HomepageConfig holds the configuration for the homepage dev environment.
type HomepageConfig struct {
	DockerHost            string
	WorkspacePath         string // host path mounted as /workspace in the container
	WebServerPort         int
	WebServerDomain       string
	WebServerInternalOnly bool // bind Caddy port only on 127.0.0.1 (internal-only)
	AllowLocalServer      bool // Danger Zone: allow Python HTTP server fallback when Docker unavailable
}

const (
	homepageContainerName  = "aurago-homepage"
	homepageImageName      = "aurago-homepage:latest"
	homepageWebContainer   = "aurago-homepage-web"
	homepageWebImage       = "caddy:alpine"
	homepageWorkspaceMount = "/workspace"
)

// homepageDockerfile is the embedded Dockerfile for the dev container.
const homepageDockerfile = `FROM mcr.microsoft.com/playwright:v1.42.0-jammy
WORKDIR /workspace
RUN apt-get update && apt-get install -y \
    git curl wget jq libvips-dev \
    && rm -rf /var/lib/apt/lists/*
RUN npm install -g \
    vercel netlify-cli \
    lighthouse \
    svgo \
    typescript ts-node
ENV CI=true
ENV PLAYWRIGHT_BROWSERS_PATH=/ms-playwright
EXPOSE 3000
CMD ["tail", "-f", "/dev/null"]
`

// HomepageDeployConfig holds SFTP/SCP deployment credentials.
type HomepageDeployConfig struct {
	Host     string
	Port     int
	User     string
	Password string // or SSH key
	Key      string // SSH private key (PEM)
	Path     string
	Method   string // "sftp" or "scp"
}

// ─── Helpers ──────────────────────────────────────────────────────────────

// truncateStr returns s truncated to maxLen characters with "…" suffix.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}

// extractOutput parses a DockerExec JSON result and returns the "output" field.
func extractOutput(jsonResult string) string {
	var m map[string]interface{}
	if json.Unmarshal([]byte(jsonResult), &m) == nil {
		if o, ok := m["output"].(string); ok {
			return o
		}
	}
	return jsonResult
}

// ─── Container Lifecycle ──────────────────────────────────────────────────

// checkDockerAvailable checks if Docker is available and running.
func checkDockerAvailable(dockerHost string) bool {
	return DockerPing(dockerHost) == nil
}

// startPythonServer starts a Python HTTP server as fallback when Docker is not available.
// Returns the URL and process info or error.
func startPythonServer(port int, directory string) (string, int, error) {
	if port <= 0 {
		port = 8080
	}
	cmd := exec.Command("python3", "-m", "http.server",
		strconv.Itoa(port), "--directory", directory)
	err := cmd.Start()
	if err != nil {
		return "", 0, fmt.Errorf("failed to start Python server: %w", err)
	}

	// Give the server a moment to start
	time.Sleep(500 * time.Millisecond)

	url := fmt.Sprintf("http://localhost:%d", port)
	return url, cmd.Process.Pid, nil
}

// HomepageInit builds the image (if needed) and creates the dev container.
// If Docker is not available, falls back to Python HTTP server (only if AllowLocalServer is true).
func HomepageInit(cfg HomepageConfig, logger *slog.Logger) string {
	// Ensure workspace dir exists on the host
	if cfg.WorkspacePath != "" {
		if err := os.MkdirAll(cfg.WorkspacePath, 0755); err != nil {
			return errJSON("Failed to create workspace directory: %v", err)
		}
	}

	// Check if Docker is available
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	if !checkDockerAvailable(cfg.DockerHost) {
		// Check if local Python server is allowed (Danger Zone)
		if !cfg.AllowLocalServer {
			return errJSON("Docker not available at %s and local Python server is disabled for security. "+
				"Please ensure Docker is running (systemctl start docker) or enable homepage.allow_local_server in config.yaml.",
				cfg.DockerHost)
		}

		logger.Info("[Homepage] Docker not available, using Python fallback")

		// Check if already running on the configured port
		checkPort := cfg.WebServerPort
		if checkPort <= 0 {
			checkPort = 8080
		}
		if c, dialErr := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", checkPort), time.Second); dialErr == nil {
			c.Close()
			return okJSON("Web server already running (Python fallback)",
				"url", fmt.Sprintf("http://localhost:%d", checkPort),
				"mode", "python")
		}

		// Try to start Python server as fallback
		url, pid, err := startPythonServer(cfg.WebServerPort, cfg.WorkspacePath)
		if err != nil {
			return errJSON("Docker not available (%v) and Python fallback failed: %v. "+
				"Please ensure either Docker is running (systemctl start docker) or Python is installed.",
				DockerPing(cfg.DockerHost), err)
		}

		return okJSON("Web server started (Python fallback)",
			"url", url,
			"pid", strconv.Itoa(pid),
			"mode", "python",
			"note", "Limited functionality without Docker. Full dev environment requires Docker.")
	}

	// Check if image exists
	imageExists := false
	data, code, err := dockerRequest(dockerCfg, "GET", "/images/json?filters=%7B%22reference%22%3A%5B%22"+homepageImageName+"%22%5D%7D", "")
	if err == nil && code == 200 {
		var images []interface{}
		if json.Unmarshal(data, &images) == nil && len(images) > 0 {
			imageExists = true
		}
	}

	if !imageExists {
		logger.Info("[Homepage] Building dev container image", "image", homepageImageName)
		result := homepageBuildImage(dockerCfg)
		var res map[string]interface{}
		if json.Unmarshal([]byte(result), &res) == nil {
			if s, _ := res["status"].(string); s == "error" {
				return result
			}
		}
	}

	// Check if container already exists
	inspectData, inspectCode, _ := dockerRequest(dockerCfg, "GET", "/containers/"+homepageContainerName+"/json", "")
	if inspectCode == 200 {
		// Container exists — check if running
		var info map[string]interface{}
		json.Unmarshal(inspectData, &info)
		state, _ := info["State"].(map[string]interface{})
		running, _ := state["Running"].(bool)
		if running {
			return okJSON("Dev container already running", "container", homepageContainerName)
		}
		// Start existing stopped container
		_, startCode, startErr := dockerRequest(dockerCfg, "POST", "/containers/"+homepageContainerName+"/start", "")
		if startErr != nil || (startCode != 204 && startCode != 304) {
			return errJSON("Failed to start existing container: code=%d err=%v", startCode, startErr)
		}
		return okJSON("Dev container started", "container", homepageContainerName)
	}

	// Create new container
	workspaceMount := cfg.WorkspacePath + ":" + homepageWorkspaceMount
	payload := map[string]interface{}{
		"Image": homepageImageName,
		"Tty":   false,
		"HostConfig": map[string]interface{}{
			"Binds":         []string{workspaceMount},
			"RestartPolicy": map[string]string{"Name": "unless-stopped"},
		},
	}
	body, _ := json.Marshal(payload)
	createData, createCode, createErr := dockerRequest(dockerCfg, "POST", "/containers/create?name="+homepageContainerName, string(body))
	if createErr != nil {
		return errJSON("Failed to create container: %v", createErr)
	}
	if createCode != 201 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, createCode, string(createData))
	}

	// Start the container
	_, startCode, startErr := dockerRequest(dockerCfg, "POST", "/containers/"+homepageContainerName+"/start", "")
	if startErr != nil || (startCode != 204 && startCode != 304) {
		return errJSON("Failed to start container: code=%d err=%v", startCode, startErr)
	}

	logger.Info("[Homepage] Dev container initialized and running", "container", homepageContainerName)
	return okJSON("Dev container initialized and running", "container", homepageContainerName)
}

// HomepageStart starts the dev container.
func HomepageStart(cfg HomepageConfig, logger *slog.Logger) string {
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	logger.Info("[Homepage] Starting dev container")
	return DockerContainerAction(dockerCfg, homepageContainerName, "start", false)
}

// HomepageStop stops the dev container.
func HomepageStop(cfg HomepageConfig, logger *slog.Logger) string {
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	logger.Info("[Homepage] Stopping dev container")
	return DockerContainerAction(dockerCfg, homepageContainerName, "stop", false)
}

// HomepageStatus returns the status of dev and web containers.
func HomepageStatus(cfg HomepageConfig, logger *slog.Logger) string {
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	result := map[string]interface{}{"status": "ok"}

	// Check Docker availability
	dockerAvailable := checkDockerAvailable(cfg.DockerHost)
	result["docker_available"] = dockerAvailable

	if !dockerAvailable {
		result["mode"] = "python_fallback"
		result["message"] = "Docker not available. Using Python HTTP server (limited functionality)."

		// Check if Python server is running by testing the port
		port := cfg.WebServerPort
		if port <= 0 {
			port = 8080
		}
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 2*time.Second)
		if err == nil {
			conn.Close()
			result["python_server"] = map[string]interface{}{
				"running": true,
				"url":     fmt.Sprintf("http://localhost:%d", port),
			}
		} else {
			result["python_server"] = map[string]interface{}{
				"running": false,
				"error":   "Server not responding",
			}
		}

		out, _ := json.Marshal(result)
		return string(out)
	}

	// Dev container status
	devStatus := containerStatus(dockerCfg, homepageContainerName)
	result["dev_container"] = json.RawMessage(devStatus)

	// Web container status
	webStatus := containerStatus(dockerCfg, homepageWebContainer)
	result["web_container"] = json.RawMessage(webStatus)

	out, _ := json.Marshal(result)
	return string(out)
}

// HomepageRebuild removes and recreates the dev container and image.
func HomepageRebuild(cfg HomepageConfig, logger *slog.Logger) string {
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	logger.Info("[Homepage] Rebuilding dev container")

	// Stop and remove container
	DockerContainerAction(dockerCfg, homepageContainerName, "stop", false)
	DockerContainerAction(dockerCfg, homepageContainerName, "remove", true)

	// Remove old image
	DockerRemoveImage(dockerCfg, homepageImageName, true)

	// Rebuild
	return HomepageInit(cfg, logger)
}

// HomepageDestroy stops and removes both containers and the image.
func HomepageDestroy(cfg HomepageConfig, logger *slog.Logger) string {
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	logger.Info("[Homepage] Destroying homepage environment")

	DockerContainerAction(dockerCfg, homepageContainerName, "stop", false)
	DockerContainerAction(dockerCfg, homepageContainerName, "remove", true)
	DockerContainerAction(dockerCfg, homepageWebContainer, "stop", false)
	DockerContainerAction(dockerCfg, homepageWebContainer, "remove", true)
	DockerRemoveImage(dockerCfg, homepageImageName, true)

	return okJSON("Homepage environment destroyed")
}

// ─── Dev Commands (Token-Saving Compound Operations) ──────────────────────

// HomepageExec runs a command inside the dev container.
func HomepageExec(cfg HomepageConfig, command string, logger *slog.Logger) string {
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	if command == "" {
		return errJSON("command is required")
	}
	logger.Info("[Homepage] Exec", "cmd", command)
	return DockerExec(dockerCfg, homepageContainerName, command, "")
}

// HomepageInitProject scaffolds a new web project inside the container.
func HomepageInitProject(cfg HomepageConfig, framework, name string, logger *slog.Logger) string {
	if name == "" {
		name = "my-site"
	}
	// Validate name to prevent shell injection
	if strings.ContainsAny(name, ";|&`$(){}\"' <>") {
		return errJSON("Invalid project name: %s", name)
	}
	var cmd string
	switch strings.ToLower(framework) {
	case "next", "nextjs", "next.js":
		cmd = fmt.Sprintf("npx --yes create-next-app@latest %s --ts --tailwind --app --src-dir --no-import-alias --eslint", name)
	case "vite", "react":
		cmd = fmt.Sprintf("npx --yes create-vite@latest %s -- --template react-ts", name)
	case "astro":
		cmd = fmt.Sprintf("npx --yes create-astro@latest %s -- --template basics --install --no-git --typescript strict", name)
	case "svelte", "sveltekit":
		cmd = fmt.Sprintf("npx --yes create-svelte@latest %s", name)
	case "vue", "nuxt":
		cmd = fmt.Sprintf("npx --yes nuxi@latest init %s", name)
	case "html", "static", "vanilla":
		cmd = fmt.Sprintf("mkdir -p %s && echo '<!DOCTYPE html><html><head><title>My Site</title></head><body><h1>Hello World</h1></body></html>' > %s/index.html", name, name)
	default:
		return errJSON("Unknown framework: %s. Use: next, vite, astro, svelte, vue, html", framework)
	}

	logger.Info("[Homepage] Init project", "framework", framework, "name", name)

	// Docker-unavailable path
	if !checkDockerAvailable(cfg.DockerHost) {
		if cfg.WorkspacePath == "" {
			return errJSON("Docker not available and workspace_path not configured")
		}
		projectPath := filepath.Join(cfg.WorkspacePath, name)

		switch strings.ToLower(framework) {
		case "html", "static", "vanilla":
			// Pure HTML — create locally without any build tool
			if err := os.MkdirAll(projectPath, 0755); err != nil {
				return errJSON("Failed to create project directory: %v", err)
			}
			html := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>` + name + `</title>
  <style>
    body { font-family: sans-serif; max-width: 800px; margin: 2rem auto; padding: 0 1rem; }
  </style>
</head>
<body>
  <h1>` + name + `</h1>
  <p>Edit index.html to get started.</p>
</body>
</html>`
			indexPath := filepath.Join(projectPath, "index.html")
			if err := os.WriteFile(indexPath, []byte(html), 0644); err != nil {
				return errJSON("Failed to write index.html: %v", err)
			}
			out, _ := json.Marshal(map[string]interface{}{
				"status":  "ok",
				"mode":    "local",
				"message": fmt.Sprintf("Project '%s' created locally (html). Use write_file to add more files.", name),
				"path":    name,
				"files":   []string{name + "/index.html"},
			})
			return string(out)

		default:
			// Framework needs npm/npx — try local npx if available
			if _, err := exec.LookPath("npx"); err == nil {
				if err := os.MkdirAll(cfg.WorkspacePath, 0755); err != nil {
					return errJSON("Failed to access workspace: %v", err)
				}
				// Split pre-built cmd into args and set Dir instead of using
				// bash -c to avoid shell injection via cfg.WorkspacePath.
				parts := strings.Fields(cmd)
				exeCmd := exec.Command(parts[0], parts[1:]...)
				exeCmd.Dir = cfg.WorkspacePath
				out, runErr := exeCmd.CombinedOutput()
				if runErr != nil {
					return errJSON("Project init failed (local npx): %s", strings.TrimSpace(string(out)))
				}
				res, _ := json.Marshal(map[string]interface{}{
					"status":  "ok",
					"mode":    "local",
					"message": fmt.Sprintf("Project '%s' scaffolded locally via npx (Docker not available).", name),
					"path":    name,
					"output":  strings.TrimSpace(string(out)),
				})
				return string(res)
			}
			// Neither Docker nor npx available
			return errJSON("Docker not available and npx not found locally. "+
				"Framework '%s' requires npm/npx to scaffold. "+
				"Options: (1) Start Docker, (2) Install Node.js+npm, (3) Use framework='html' for a plain HTML project (works without Docker or npm).", framework)
		}
	}

	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	scaffoldResult := DockerExec(dockerCfg, homepageContainerName, "cd /workspace && "+cmd, "")

	// Verify the project directory was actually created.
	// Scaffolding tools like create-vite may print warnings but fail silently
	// (e.g. Node version mismatch / EBADENGINE) without creating the directory.
	verifyResult := DockerExec(dockerCfg, homepageContainerName, fmt.Sprintf("test -d /workspace/%s && echo EXISTS", name), "")
	var vr map[string]interface{}
	if json.Unmarshal([]byte(verifyResult), &vr) == nil {
		out, _ := vr["output"].(string)
		if !strings.Contains(out, "EXISTS") {
			return errJSON("Project scaffolding did not create directory '/workspace/%s'. "+
				"This often means the container's Node.js version is too old for the requested framework. "+
				"Try framework='html' for a plain project, or rebuild the homepage container to update Node.js. "+
				"Scaffold output: %s", name, truncateStr(extractOutput(scaffoldResult), 500))
		}
	}

	return scaffoldResult
}

// HomepageBuild runs the build command in the project directory.
// Plain HTML projects (no package.json) are detected and skipped — they need no build step.
func HomepageBuild(cfg HomepageConfig, projectDir string, logger *slog.Logger) string {
	if projectDir == "" {
		projectDir = "."
	}
	logger.Info("[Homepage] Build", "dir", projectDir)

	// Detect plain HTML projects: no package.json → no build needed.
	if cfg.WorkspacePath != "" {
		pkgPath := filepath.Join(cfg.WorkspacePath, projectDir, "package.json")
		if _, err := os.Stat(pkgPath); err != nil {
			logger.Info("[Homepage] No package.json found — plain HTML project, skipping build")
			out, _ := json.Marshal(map[string]interface{}{"status": "ok", "output": "Plain HTML project — no build required"})
			return string(out)
		}
	}

	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	return DockerExec(dockerCfg, homepageContainerName, fmt.Sprintf("cd /workspace/%s && npm run build 2>&1", projectDir), "")
}

// HomepageInstallDeps installs npm packages inside the container.
func HomepageInstallDeps(cfg HomepageConfig, projectDir string, packages []string, logger *slog.Logger) string {
	if projectDir == "" {
		projectDir = "."
	}
	cmd := "npm install"
	if len(packages) > 0 {
		// Validate package names to prevent injection
		for _, p := range packages {
			if strings.ContainsAny(p, ";|&`$(){}") {
				return errJSON("Invalid package name: %s", p)
			}
		}
		cmd += " " + strings.Join(packages, " ")
	}
	logger.Info("[Homepage] Install deps", "project_dir", projectDir, "packages", packages)
	dockerCfg := DockerConfig{Host: cfg.DockerHost}

	// Pre-check: verify the project directory exists to give a clear error
	if projectDir != "." {
		checkResult := DockerExec(dockerCfg, homepageContainerName, fmt.Sprintf("test -d /workspace/%s && echo EXISTS", projectDir), "")
		if !strings.Contains(checkResult, "EXISTS") {
			return errJSON("Project directory '/workspace/%s' does not exist in the homepage container. "+
				"Run init_project first, or use homepage write_file to create files in the correct location.", projectDir)
		}
	}

	return DockerExec(dockerCfg, homepageContainerName, fmt.Sprintf("cd /workspace/%s && %s 2>&1", projectDir, cmd), "")
}

// HomepageLighthouse runs a Lighthouse audit and returns a compact summary.
func HomepageLighthouse(cfg HomepageConfig, url string, logger *slog.Logger) string {
	if url == "" {
		return errJSON("url is required for lighthouse audit")
	}
	logger.Info("[Homepage] Lighthouse", "url", url)
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	// Run lighthouse with JSON output, extract key scores
	cmd := fmt.Sprintf(`lighthouse "%s" --output json --chrome-flags="--headless --no-sandbox --disable-gpu" 2>/dev/null | jq '{performance: .categories.performance.score, accessibility: .categories.accessibility.score, bestPractices: .categories["best-practices"].score, seo: .categories.seo.score, fcp: .audits["first-contentful-paint"].displayValue, lcp: .audits["largest-contentful-paint"].displayValue, cls: .audits["cumulative-layout-shift"].displayValue, tbt: .audits["total-blocking-time"].displayValue}'`, url)
	return DockerExec(dockerCfg, homepageContainerName, cmd, "")
}

// HomepageScreenshot takes a screenshot using Playwright.
func HomepageScreenshot(cfg HomepageConfig, url, viewport string, logger *slog.Logger) string {
	if url == "" {
		return errJSON("url is required for screenshot")
	}
	if viewport == "" {
		viewport = "1280x720"
	}
	parts := strings.Split(viewport, "x")
	width, height := "1280", "720"
	if len(parts) == 2 {
		width, height = parts[0], parts[1]
	}

	logger.Info("[Homepage] Screenshot", "url", url, "viewport", viewport)
	dockerCfg := DockerConfig{Host: cfg.DockerHost}

	// Use a Node.js one-liner with Playwright
	script := fmt.Sprintf(`node -e "
const {chromium} = require('playwright');
(async()=>{
  const b = await chromium.launch({args:['--no-sandbox']});
  const p = await b.newPage();
  await p.setViewportSize({width:%s,height:%s});
  await p.goto('%s',{waitUntil:'networkidle'});
  await p.screenshot({path:'/workspace/_screenshot.png',fullPage:true});
  await b.close();
  console.log('screenshot saved to /workspace/_screenshot.png');
})();"`, width, height, url)

	return DockerExec(dockerCfg, homepageContainerName, script, "")
}

// HomepageLint runs ESLint in the project directory.
func HomepageLint(cfg HomepageConfig, projectDir string, logger *slog.Logger) string {
	if projectDir == "" {
		projectDir = "."
	}
	logger.Info("[Homepage] Lint", "dir", projectDir)
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	return DockerExec(dockerCfg, homepageContainerName, fmt.Sprintf("cd /workspace/%s && npx eslint . --format compact 2>&1 | head -100", projectDir), "")
}

// HomepageListFiles lists files in a directory inside the container.
func HomepageListFiles(cfg HomepageConfig, path string, logger *slog.Logger) string {
	if path == "" {
		path = "."
	}
	// Prevent path traversal outside workspace
	if strings.Contains(path, "..") {
		return errJSON("Path traversal not allowed")
	}
	logger.Info("[Homepage] ListFiles", "path", path)

	if !checkDockerAvailable(cfg.DockerHost) {
		if cfg.WorkspacePath == "" {
			return errJSON("workspace_path not configured")
		}
		base := filepath.Join(cfg.WorkspacePath, filepath.FromSlash(path))
		cleanWS := filepath.Clean(cfg.WorkspacePath)
		cleanBase := filepath.Clean(base)
		if cleanBase != cleanWS && !strings.HasPrefix(cleanBase, cleanWS+string(os.PathSeparator)) {
			return errJSON("Path traversal not allowed")
		}
		var files []string
		_ = filepath.Walk(base, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			// Return paths relative to WorkspacePath so they are usable with read_file / write_file
			rel, _ := filepath.Rel(cfg.WorkspacePath, p)
			slashRel := filepath.ToSlash(rel)
			if strings.Contains(slashRel, "/node_modules") || strings.Contains(slashRel, "/.next") || strings.Contains(slashRel, "/.git") {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			// Limit depth to 3 segments below WorkspacePath
			parts := strings.Split(slashRel, "/")
			if len(parts) > 4 {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if len(files) < 200 {
				files = append(files, slashRel)
			}
			return nil
		})
		out, _ := json.Marshal(map[string]interface{}{"status": "ok", "mode": "local", "workspace": cfg.WorkspacePath, "files": files})
		return string(out)
	}

	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	return DockerExec(dockerCfg, homepageContainerName, fmt.Sprintf("find /workspace/%s -maxdepth 2 -not -path '*/node_modules/*' -not -path '*/.next/*' -not -path '*/.git/*' | head -200", path), "")
}

// HomepageReadFile reads a file from the container.
func HomepageReadFile(cfg HomepageConfig, path string, logger *slog.Logger) string {
	if path == "" {
		return errJSON("path is required")
	}
	if strings.Contains(path, "..") {
		return errJSON("Path traversal not allowed")
	}
	logger.Info("[Homepage] ReadFile", "path", path)

	if !checkDockerAvailable(cfg.DockerHost) {
		if cfg.WorkspacePath == "" {
			return errJSON("workspace_path not configured")
		}
		fullPath := filepath.Join(cfg.WorkspacePath, filepath.FromSlash(path))
		cleanWS := filepath.Clean(cfg.WorkspacePath)
		cleanFull := filepath.Clean(fullPath)
		if cleanFull == cleanWS || !strings.HasPrefix(cleanFull, cleanWS+string(os.PathSeparator)) {
			return errJSON("Path traversal not allowed")
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return errJSON("Failed to read file: %v", err)
		}
		out, _ := json.Marshal(map[string]interface{}{"status": "ok", "content": string(data)})
		return string(out)
	}

	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	return DockerExec(dockerCfg, homepageContainerName, fmt.Sprintf("cat /workspace/%s", path), "")
}

// HomepageWriteFile writes content to a file inside the container.
func HomepageWriteFile(cfg HomepageConfig, path, content string, logger *slog.Logger) string {
	if path == "" {
		return errJSON("path is required")
	}
	if strings.Contains(path, "..") {
		return errJSON("Path traversal not allowed")
	}
	logger.Info("[Homepage] WriteFile", "path", path, "size", len(content))

	if !checkDockerAvailable(cfg.DockerHost) {
		if cfg.WorkspacePath == "" {
			return errJSON("workspace_path not configured")
		}
		fullPath := filepath.Join(cfg.WorkspacePath, filepath.FromSlash(path))
		cleanWS := filepath.Clean(cfg.WorkspacePath)
		cleanFull := filepath.Clean(fullPath)
		if cleanFull == cleanWS || !strings.HasPrefix(cleanFull, cleanWS+string(os.PathSeparator)) {
			return errJSON("Path traversal not allowed")
		}
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return errJSON("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return errJSON("Failed to write file: %v", err)
		}
		out, _ := json.Marshal(map[string]interface{}{"status": "ok", "path": path, "size": len(content)})
		return string(out)
	}

	// Use base64 to safely pass content through shell
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	cmd := fmt.Sprintf("mkdir -p /workspace/%s && echo '%s' | base64 -d > /workspace/%s", dir, encoded, path)
	return DockerExec(dockerCfg, homepageContainerName, cmd, "")
}

// HomepageOptimizeImages runs SVG optimization on the project.
