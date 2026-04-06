package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/beevik/etree"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"gopkg.in/yaml.v3"
)

// HomepageConfig holds the configuration for the homepage dev environment.
type HomepageConfig struct {
	DockerHost            string
	WorkspacePath         string // host path mounted as /workspace in the container
	DataDir               string // host path to AuraGo's data directory (for bundling generated images into deploys)
	WebServerPort         int
	WebServerDomain       string
	WebServerInternalOnly bool // bind Caddy port only on 127.0.0.1 (internal-only)
	AllowLocalServer      bool // Danger Zone: allow Python HTTP server fallback when Docker unavailable
}

const (
	homepageContainerName  = "aurago-homepage"
	homepageImageName      = "aurago-homepage:latest"
	homepageWebContainer   = "aurago-homepage-web"
	homepageWebImage       = "caddy:2.11.2-alpine"
	homepageWorkspaceMount = "/workspace"
)

var homepageDockerExecFunc = DockerExec
var homepageWebCaptureFunc = WebCapture

// homepageDockerfile is the embedded Dockerfile for the dev container.
const homepageDockerfile = `FROM mcr.microsoft.com/playwright:v1.58.2-noble
WORKDIR /workspace
ARG CLOUDFLARED_VERSION=2026.3.0
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        git \
        jq \
        libvips-dev \
        wget; \
    arch="$(dpkg --print-architecture)"; \
    case "$arch" in \
        amd64) cloudflared_arch="amd64" ;; \
        arm64) cloudflared_arch="arm64" ;; \
        *) echo "unsupported architecture: $arch" >&2; exit 1 ;; \
    esac; \
    curl -fsSL "https://github.com/cloudflare/cloudflared/releases/download/${CLOUDFLARED_VERSION}/cloudflared-linux-${cloudflared_arch}.deb" -o /tmp/cloudflared.deb; \
    dpkg -i /tmp/cloudflared.deb; \
    rm -f /tmp/cloudflared.deb; \
    rm -rf /var/lib/apt/lists/*
RUN npm install -g \
    vercel netlify-cli \
    lighthouse \
    svgo \
    typescript ts-node \
    && npm cache clean --force
ENV CI=true
ENV NODE_ENV=development
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

// isValidHomepageURL validates that a URL is a well-formed HTTP(S) URL
// and does not contain shell metacharacters that could lead to command injection.
func isValidHomepageURL(u string) bool {
	if u == "" {
		return false
	}
	// Must start with http:// or https://
	lower := strings.ToLower(u)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return false
	}
	// Reject shell metacharacters
	for _, c := range u {
		switch c {
		case ';', '|', '&', '`', '$', '(', ')', '{', '}', '<', '>', '\\', '!', '\n', '\r', '"', '\'':
			return false
		}
	}
	// SSRF protection: reject private/loopback IPs
	parsed, err := url.Parse(u)
	if err != nil {
		return false
	}
	hostname := parsed.Hostname()
	if hostname == "" {
		return false
	}
	if isPrivateHost(hostname) {
		return false
	}
	return true
}

// isPrivateHost checks if a hostname resolves to a private or loopback IP address.
func isPrivateHost(hostname string) bool {
	// Check if it's a direct IP
	if ip := net.ParseIP(hostname); ip != nil {
		return isPrivateIP(ip)
	}
	// Resolve hostname and check all IPs
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return true // fail closed: unresolvable hosts are rejected
	}
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return true
		}
	}
	return false
}

// isPrivateIP returns true for loopback, private, and link-local addresses.
func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

// sanitizeProjectDir validates a project directory name for use in shell commands.
// It rejects path traversal, shell metacharacters, and absolute paths.
func sanitizeProjectDir(projectDir string) error {
	if strings.Contains(projectDir, "..") {
		return fmt.Errorf("path traversal detected in project_dir %q. Use a simple relative homepage workspace path such as 'my-site'", projectDir)
	}
	if strings.HasPrefix(projectDir, "/") || strings.HasPrefix(projectDir, "\\") {
		return fmt.Errorf("absolute paths not allowed for project_dir %q. project_dir must be relative to the homepage workspace, e.g. 'ki-news' instead of '/workspace/ki-news'", projectDir)
	}
	for _, c := range projectDir {
		switch c {
		case ';', '|', '&', '`', '$', '(', ')', '{', '}', '<', '>', '\\', '!', '"', '\'', '\n', '\r', ' ':
			return fmt.Errorf("invalid character %q in project directory %q. Use a simple relative directory name like 'my-site' or 'sites/landing-page'", c, projectDir)
		}
	}
	return nil
}

func homepageWorkspacePathGuidance() string {
	return "Configure homepage.workspace_path as the absolute host directory mounted as /workspace. In homepage tool calls, use relative project_dir/path values like 'my-site' or 'my-site/src/app/page.tsx', never '/workspace/my-site' or host filesystem paths."
}

func homepageWorkspacePathNotConfiguredJSON() string {
	return errJSON("workspace_path not configured. %s", homepageWorkspacePathGuidance())
}

func validateHomepageRelativePathArg(path, field string) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return fmt.Errorf("%s is required", field)
	}
	if strings.Contains(trimmed, "..") {
		return fmt.Errorf("path traversal not allowed in %s %q", field, path)
	}
	normalized := filepath.ToSlash(trimmed)
	if filepath.IsAbs(trimmed) || strings.HasPrefix(normalized, "/") || strings.HasPrefix(normalized, homepageWorkspaceMount+"/") {
		return fmt.Errorf("%s must be relative to the homepage workspace, e.g. 'my-site/src/app/page.tsx' not %q", field, path)
	}
	return nil
}

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
	// Always bind to loopback — the URL returned is http://localhost:... and
	// the workspace directory must not be served to the public network.
	cmd := exec.Command("python3", "-m", "http.server",
		strconv.Itoa(port), "--directory", directory, "--bind", "127.0.0.1")
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
		if err := json.Unmarshal(inspectData, &info); err == nil {
			state, _ := info["State"].(map[string]interface{})
			running, _ := state["Running"].(bool)
			if running {
				return okJSON("Dev container already running", "container", homepageContainerName)
			}
		}
		// Start existing stopped container
		_, startCode, startErr := dockerRequest(dockerCfg, "POST", "/containers/"+homepageContainerName+"/start", "")
		if startErr != nil || (startCode != 204 && startCode != 304) {
			return errJSON("Failed to start existing container: code=%d err=%v", startCode, startErr)
		}
		return okJSON("Dev container started", "container", homepageContainerName)
	}

	// Create new container — run as the current UID/GID so bind-mounted
	// workspace files are owned by the aurago user, not root.
	workspaceMount := cfg.WorkspacePath + ":" + homepageWorkspaceMount
	currentUser := fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
	payload := map[string]interface{}{
		"Image": homepageImageName,
		"Tty":   false,
		"User":  currentUser,
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
// Pass env as nil unless additional environment variables need to be injected.
func HomepageExec(cfg HomepageConfig, command string, env []string, logger *slog.Logger) string {
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	if command == "" {
		return errJSON("command is required")
	}
	logger.Info("[Homepage] Exec", "cmd", command)
	return dockerExecInternal(dockerCfg, homepageContainerName, command, "", env)
}

// HomepageInitProject scaffolds a new web project inside the container.
// If template is non-empty, starter content is applied after scaffolding.
func HomepageInitProject(cfg HomepageConfig, framework, name, template string, logger *slog.Logger) string {
	if name == "" {
		name = "my-site"
	}
	// Validate name to prevent shell injection
	if strings.ContainsAny(name, ";|&`$(){}\"' <>") {
		return errJSON("Invalid project name: %s", name)
	}
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return errJSON("Path traversal not allowed in project name")
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
			return errJSON("Docker not available and workspace_path not configured. %s", homepageWorkspacePathGuidance())
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
			if template != "" {
				applyHomepageTemplate(cfg, name, template, logger)
			}
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
				if template != "" {
					applyHomepageTemplate(cfg, name, template, logger)
				}
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

	// Apply template if requested
	if template != "" {
		applyHomepageTemplate(cfg, name, template, logger)
	}

	return scaffoldResult
}

// HomepageBuild runs the build command in the project directory.
// Plain HTML projects (no package.json) are detected and skipped — they need no build step.
func HomepageBuild(cfg HomepageConfig, projectDir string, logger *slog.Logger) string {
	if projectDir == "" {
		projectDir = "."
	}
	if projectDir != "." {
		if err := sanitizeProjectDir(projectDir); err != nil {
			return errJSON("%v", err)
		}
	}
	logger.Info("[Homepage] Build", "dir", projectDir)

	// Detect plain HTML projects: no package.json → no build needed.
	if cfg.WorkspacePath != "" {
		pkgPath := filepath.Join(cfg.WorkspacePath, projectDir, "package.json")
		if _, err := os.Stat(pkgPath); err != nil {
			logger.Info("[Homepage] No package.json found — plain HTML project, skipping build")
			out, _ := json.Marshal(map[string]interface{}{
				"status": "ok",
				"output": "Plain HTML project — no build required",
				"note":   "This project has no package.json. deploy_netlify and publish_local will serve or package the project directory directly.",
			})
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
	if projectDir != "." {
		if err := sanitizeProjectDir(projectDir); err != nil {
			return errJSON("%v", err)
		}
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
	if !isValidHomepageURL(url) {
		return errJSON("invalid URL: must be a valid http:// or https:// URL without shell metacharacters")
	}
	logger.Info("[Homepage] Lighthouse", "url", url)
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	// Run lighthouse with JSON output, extract key scores
	cmd := fmt.Sprintf(`lighthouse "%s" --output json --chrome-flags="--headless --no-sandbox --disable-gpu" 2>/dev/null | jq '{performance: .categories.performance.score, accessibility: .categories.accessibility.score, bestPractices: .categories["best-practices"].score, seo: .categories.seo.score, fcp: .audits["first-contentful-paint"].displayValue, lcp: .audits["largest-contentful-paint"].displayValue, cls: .audits["cumulative-layout-shift"].displayValue, tbt: .audits["total-blocking-time"].displayValue, errorsInConsole: .audits["errors-in-console"].score, consoleErrors: (.audits["errors-in-console"].details.items // [] | length)}'`, url)
	return DockerExec(dockerCfg, homepageContainerName, cmd, "")
}

// HomepageScreenshot takes a screenshot using Playwright.
func HomepageScreenshot(ctx context.Context, cfg HomepageConfig, url, viewport string, logger *slog.Logger) string {
	if url == "" {
		return errJSON("url is required for screenshot")
	}
	if !isValidHomepageURL(url) {
		return errJSON("invalid URL: must be a valid http:// or https:// URL without shell metacharacters")
	}
	if viewport == "" {
		viewport = "1280x720"
	}
	parts := strings.Split(viewport, "x")
	if len(parts) != 2 {
		return errJSON("invalid viewport %q: must be WIDTHxHEIGHT with numeric values 1-9999", viewport)
	}
	w, errW := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, errH := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errW != nil || errH != nil || w < 1 || w > 9999 || h < 1 || h > 9999 {
		return errJSON("invalid viewport %q: must be WIDTHxHEIGHT with numeric values 1-9999", viewport)
	}
	width, height := strconv.Itoa(w), strconv.Itoa(h)

	if logger != nil {
		logger.Info("[Homepage] Screenshot", "url", url, "viewport", viewport)
	}
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

	result := homepageDockerExecFunc(dockerCfg, homepageContainerName, script, "")
	if strings.Contains(result, "Cannot find module 'playwright'") || strings.Contains(result, "MODULE_NOT_FOUND") {
		if logger != nil {
			logger.Warn("[Homepage] Playwright missing in homepage container, falling back to web_capture")
		}
		return homepageWebCaptureFunc(ctx, "screenshot", url, "", true, "agent_workspace/workdir")
	}
	return result
}

// HomepageLint runs ESLint in the project directory.
func HomepageLint(cfg HomepageConfig, projectDir string, logger *slog.Logger) string {
	if projectDir == "" {
		projectDir = "."
	}
	if projectDir != "." {
		if err := sanitizeProjectDir(projectDir); err != nil {
			return errJSON("%v", err)
		}
	}
	logger.Info("[Homepage] Lint", "dir", projectDir)
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	// Run TypeScript check first (if tsconfig exists), then ESLint
	cmd := fmt.Sprintf(`cd /workspace/%s && { if [ -f tsconfig.json ]; then echo "=== TypeScript Check ==="; npx tsc --noEmit 2>&1 | head -50; echo; fi; echo "=== ESLint ==="; npx eslint . --format compact 2>&1 | head -100; }`, projectDir)
	return DockerExec(dockerCfg, homepageContainerName, cmd, "")
}

// HomepageCheckJS navigates to a URL and captures any JavaScript console errors.
func HomepageCheckJS(ctx context.Context, cfg HomepageConfig, url string, logger *slog.Logger) string {
	if url == "" {
		return errJSON("url is required for check_js")
	}
	if !isValidHomepageURL(url) {
		return errJSON("invalid URL: must be a valid http:// or https:// URL without shell metacharacters")
	}
	if logger != nil {
		logger.Info("[Homepage] CheckJS", "url", url)
	}
	dockerCfg := DockerConfig{Host: cfg.DockerHost}

	script := fmt.Sprintf(`node -e "
const {chromium} = require('playwright');
(async()=>{
  const errors = [];
  const b = await chromium.launch({args:['--no-sandbox']});
  const p = await b.newPage();
  p.on('pageerror', e => errors.push({type:'error',message:e.message}));
  p.on('console', m => { if(m.type()==='error') errors.push({type:'console-error',message:m.text()}); });
  await p.goto('%s',{waitUntil:'networkidle',timeout:30000});
  await new Promise(r=>setTimeout(r,2000));
  await b.close();
  console.log(JSON.stringify({errorCount:errors.length,errors:errors.slice(0,20)}));
})();"`, url)

	result := homepageDockerExecFunc(dockerCfg, homepageContainerName, script, "")
	if strings.Contains(result, "Cannot find module 'playwright'") || strings.Contains(result, "MODULE_NOT_FOUND") {
		return errJSON("Playwright not available in homepage container. Install with: homepage exec 'npm i -g playwright && npx playwright install chromium'")
	}
	return result
}

// HomepageListFiles lists files in a directory inside the container.
func HomepageListFiles(cfg HomepageConfig, path string, logger *slog.Logger) string {
	if path == "" {
		path = "."
	}
	if path != "." {
		if err := validateHomepageRelativePathArg(path, "path"); err != nil {
			return errJSON("%v", err)
		}
	}
	logger.Info("[Homepage] ListFiles", "path", path)

	if !checkDockerAvailable(cfg.DockerHost) {
		if cfg.WorkspacePath == "" {
			return homepageWorkspacePathNotConfiguredJSON()
		}
		base := filepath.Join(cfg.WorkspacePath, filepath.FromSlash(path))
		cleanWS := filepath.Clean(cfg.WorkspacePath)
		cleanBase := filepath.Clean(base)
		if cleanBase != cleanWS && !strings.HasPrefix(cleanBase, cleanWS+string(os.PathSeparator)) {
			return errJSON("path traversal not allowed in path %q", path)
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
	if err := validateHomepageRelativePathArg(path, "path"); err != nil {
		return errJSON("%v", err)
	}
	logger.Info("[Homepage] ReadFile", "path", path)

	if !checkDockerAvailable(cfg.DockerHost) {
		if cfg.WorkspacePath == "" {
			return homepageWorkspacePathNotConfiguredJSON()
		}
		fullPath := filepath.Join(cfg.WorkspacePath, filepath.FromSlash(path))
		cleanWS := filepath.Clean(cfg.WorkspacePath)
		cleanFull := filepath.Clean(fullPath)
		if cleanFull == cleanWS || !strings.HasPrefix(cleanFull, cleanWS+string(os.PathSeparator)) {
			return errJSON("path traversal not allowed in path %q", path)
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
// maxHomepageWriteFileSize is the maximum content size for HomepageWriteFile (2 MB).
const maxHomepageWriteFileSize = 2 * 1024 * 1024

func HomepageWriteFile(cfg HomepageConfig, path, content string, logger *slog.Logger) string {
	if err := validateHomepageRelativePathArg(path, "path"); err != nil {
		return errJSON("%v", err)
	}
	if len(content) > maxHomepageWriteFileSize {
		return errJSON("content too large: %d bytes exceeds maximum of %d bytes", len(content), maxHomepageWriteFileSize)
	}
	logger.Info("[Homepage] WriteFile", "path", path, "size", len(content))

	if !checkDockerAvailable(cfg.DockerHost) {
		if cfg.WorkspacePath == "" {
			return homepageWorkspacePathNotConfiguredJSON()
		}
		fullPath := filepath.Join(cfg.WorkspacePath, filepath.FromSlash(path))
		cleanWS := filepath.Clean(cfg.WorkspacePath)
		cleanFull := filepath.Clean(fullPath)
		if cleanFull == cleanWS || !strings.HasPrefix(cleanFull, cleanWS+string(os.PathSeparator)) {
			return errJSON("path traversal not allowed in path %q", path)
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

// HomepageEditFile performs precise file editing inside the container (or locally).
// It reads the file, applies the edit in Go, then writes back.
func HomepageEditFile(cfg HomepageConfig, path, operation, old, new_, marker, content string, startLine, endLine int, logger *slog.Logger) string {
	if err := validateHomepageRelativePathArg(path, "path"); err != nil {
		return errJSON("%v", err)
	}
	logger.Info("[Homepage] EditFile", "path", path, "op", operation)

	if !checkDockerAvailable(cfg.DockerHost) {
		// Local fallback: use file_editor directly on workspace path
		if cfg.WorkspacePath == "" {
			return homepageWorkspacePathNotConfiguredJSON()
		}
		fullPath := filepath.Join(cfg.WorkspacePath, filepath.FromSlash(path))
		cleanWS := filepath.Clean(cfg.WorkspacePath)
		cleanFull := filepath.Clean(fullPath)
		if cleanFull == cleanWS || !strings.HasPrefix(cleanFull, cleanWS+string(os.PathSeparator)) {
			return errJSON("path traversal not allowed in path %q", path)
		}
		return ExecuteFileEditor(operation, path, old, new_, marker, content, startLine, endLine, 0, cfg.WorkspacePath)
	}

	// Docker: read file from container, apply edit in Go, write back
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	readResult := DockerExec(dockerCfg, homepageContainerName, fmt.Sprintf("cat /workspace/%s", path), "")

	// DockerExec returns JSON; try to detect errors
	var readResp map[string]interface{}
	if err := json.Unmarshal([]byte(readResult), &readResp); err == nil {
		if status, ok := readResp["status"].(string); ok && status == "error" {
			return readResult
		}
		// If there's an "output" field, use that
		if output, ok := readResp["output"].(string); ok {
			readResult = output
		}
	}

	// Apply the edit operation on the content
	edited, editErr := applyHomepageEdit(readResult, operation, old, new_, marker, content, startLine, endLine)
	if editErr != "" {
		return editErr
	}

	// Write back via base64
	encoded := base64.StdEncoding.EncodeToString([]byte(edited))
	dir := filepath.Dir(path)
	writeCmd := fmt.Sprintf("mkdir -p /workspace/%s && echo '%s' | base64 -d > /workspace/%s", dir, encoded, path)
	return DockerExec(dockerCfg, homepageContainerName, writeCmd, "")
}

// applyHomepageEdit applies an editing operation to file content in memory.
// Returns the edited content and an error JSON string (empty if success).
func applyHomepageEdit(text, operation, old, new_, marker, content string, startLine, endLine int) (string, string) {
	switch operation {
	case "str_replace":
		if old == "" {
			return "", errJSON("'old' text is required for str_replace")
		}
		count := strings.Count(text, old)
		if count == 0 {
			return "", errJSON("'old' text not found in file")
		}
		if count > 1 {
			return "", errJSON("'old' text found %d times — must be unique for str_replace", count)
		}
		return strings.Replace(text, old, new_, 1), ""

	case "str_replace_all":
		if old == "" {
			return "", errJSON("'old' text is required")
		}
		if !strings.Contains(text, old) {
			return "", errJSON("'old' text not found in file")
		}
		return strings.ReplaceAll(text, old, new_), ""

	case "insert_after", "insert_before":
		if marker == "" {
			return "", errJSON("'marker' is required")
		}
		if content == "" {
			return "", errJSON("'content' is required")
		}
		lines := strings.Split(text, "\n")
		idx := -1
		for i, line := range lines {
			if strings.Contains(line, marker) {
				if idx >= 0 {
					return "", errJSON("marker found on multiple lines")
				}
				idx = i
			}
		}
		if idx < 0 {
			return "", errJSON("marker not found")
		}
		insertLines := strings.Split(content, "\n")
		insertIdx := idx
		if operation == "insert_after" {
			insertIdx = idx + 1
		}
		newLines := make([]string, 0, len(lines)+len(insertLines))
		newLines = append(newLines, lines[:insertIdx]...)
		newLines = append(newLines, insertLines...)
		newLines = append(newLines, lines[insertIdx:]...)
		return strings.Join(newLines, "\n"), ""

	case "append":
		if content == "" {
			return "", errJSON("'content' is required")
		}
		if len(text) > 0 && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		return text + content, ""

	case "prepend":
		if content == "" {
			return "", errJSON("'content' is required")
		}
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		return content + text, ""

	case "delete_lines":
		if startLine < 1 {
			return "", errJSON("start_line must be >= 1")
		}
		if endLine < startLine {
			return "", errJSON("end_line must be >= start_line")
		}
		lines := strings.Split(text, "\n")
		if startLine > len(lines) {
			return "", errJSON("start_line exceeds file length")
		}
		if endLine > len(lines) {
			endLine = len(lines)
		}
		newLines := make([]string, 0, len(lines)-(endLine-startLine+1))
		newLines = append(newLines, lines[:startLine-1]...)
		newLines = append(newLines, lines[endLine:]...)
		return strings.Join(newLines, "\n"), ""

	default:
		return "", errJSON("unknown edit operation: %s", operation)
	}
}

// HomepageJsonEdit edits a JSON file inside the homepage container (or local workspace).
func HomepageJsonEdit(cfg HomepageConfig, path, operation, jsonPath string, setValue interface{}, content string, logger *slog.Logger) string {
	if err := validateHomepageRelativePathArg(path, "path"); err != nil {
		return errJSON("%v", err)
	}
	logger.Info("[Homepage] JsonEdit", "path", path, "op", operation)

	if !checkDockerAvailable(cfg.DockerHost) {
		if cfg.WorkspacePath == "" {
			return homepageWorkspacePathNotConfiguredJSON()
		}
		fullPath := filepath.Join(cfg.WorkspacePath, filepath.FromSlash(path))
		cleanWS := filepath.Clean(cfg.WorkspacePath)
		cleanFull := filepath.Clean(fullPath)
		if cleanFull == cleanWS || !strings.HasPrefix(cleanFull, cleanWS+string(os.PathSeparator)) {
			return errJSON("path traversal not allowed in path %q", path)
		}
		return ExecuteJsonEditor(operation, fullPath, jsonPath, setValue, content, cfg.WorkspacePath)
	}

	// Docker: read from container
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	readResult := DockerExec(dockerCfg, homepageContainerName, fmt.Sprintf("cat /workspace/%s", path), "")
	fileContent := extractDockerOutput(readResult)
	if fileContent == "" && operation != "set" {
		return errJSON("could not read file from container")
	}

	// Apply JSON operation on content
	result, edited, err := applyHomepageJsonEdit(fileContent, operation, jsonPath, setValue)
	if err != "" {
		return err
	}

	// For read-only operations, return the result
	switch operation {
	case "get", "keys", "validate":
		return result
	}

	// Write edited content back
	encoded := base64.StdEncoding.EncodeToString([]byte(edited))
	dir := filepath.Dir(path)
	writeCmd := fmt.Sprintf("mkdir -p /workspace/%s && echo '%s' | base64 -d > /workspace/%s", dir, encoded, path)
	DockerExec(dockerCfg, homepageContainerName, writeCmd, "")
	return result
}

// applyHomepageJsonEdit performs JSON operations on in-memory content.
// Returns (resultJSON, editedContent, errorJSON). errorJSON is empty on success.
func applyHomepageJsonEdit(content, operation, jsonPath string, setValue interface{}) (string, string, string) {
	encode := func(r JsonEditorResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	switch operation {
	case "get":
		if !gjson.Valid(content) {
			return "", "", encode(JsonEditorResult{Status: "error", Message: "not valid JSON"})
		}
		if jsonPath == "" {
			return encode(JsonEditorResult{Status: "ok", Data: json.RawMessage(content)}), "", ""
		}
		r := gjson.Get(content, jsonPath)
		if !r.Exists() {
			return "", "", encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("path '%s' not found", jsonPath)})
		}
		return encode(JsonEditorResult{Status: "ok", Data: json.RawMessage(r.Raw)}), "", ""

	case "set":
		if jsonPath == "" {
			return "", "", encode(JsonEditorResult{Status: "error", Message: "'json_path' is required for set"})
		}
		data := content
		if data == "" {
			data = "{}"
		}
		if !gjson.Valid(data) {
			return "", "", encode(JsonEditorResult{Status: "error", Message: "not valid JSON"})
		}
		updated, err := sjson.Set(data, jsonPath, setValue)
		if err != nil {
			return "", "", encode(JsonEditorResult{Status: "error", Message: err.Error()})
		}
		var raw json.RawMessage = []byte(updated)
		formatted, _ := json.MarshalIndent(raw, "", "  ")
		return encode(JsonEditorResult{Status: "ok", Message: fmt.Sprintf("set '%s'", jsonPath)}), string(formatted) + "\n", ""

	case "delete":
		if jsonPath == "" {
			return "", "", encode(JsonEditorResult{Status: "error", Message: "'json_path' is required for delete"})
		}
		if !gjson.Valid(content) {
			return "", "", encode(JsonEditorResult{Status: "error", Message: "not valid JSON"})
		}
		updated, err := sjson.Delete(content, jsonPath)
		if err != nil {
			return "", "", encode(JsonEditorResult{Status: "error", Message: err.Error()})
		}
		var raw json.RawMessage = []byte(updated)
		formatted, _ := json.MarshalIndent(raw, "", "  ")
		return encode(JsonEditorResult{Status: "ok", Message: fmt.Sprintf("deleted '%s'", jsonPath)}), string(formatted) + "\n", ""

	case "keys":
		if !gjson.Valid(content) {
			return "", "", encode(JsonEditorResult{Status: "error", Message: "not valid JSON"})
		}
		target := content
		if jsonPath != "" {
			r := gjson.Get(content, jsonPath)
			if !r.Exists() {
				return "", "", encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("path '%s' not found", jsonPath)})
			}
			target = r.Raw
		}
		var keys []string
		gjson.Parse(target).ForEach(func(key, _ gjson.Result) bool {
			keys = append(keys, key.String())
			return true
		})
		return encode(JsonEditorResult{Status: "ok", Data: keys}), "", ""

	case "validate":
		if gjson.Valid(content) {
			return encode(JsonEditorResult{Status: "ok", Message: "valid JSON", Data: true}), "", ""
		}
		return encode(JsonEditorResult{Status: "ok", Message: "invalid JSON", Data: false}), "", ""

	case "format":
		if !gjson.Valid(content) {
			return "", "", encode(JsonEditorResult{Status: "error", Message: "not valid JSON"})
		}
		var raw json.RawMessage = []byte(content)
		formatted, _ := json.MarshalIndent(raw, "", "  ")
		return encode(JsonEditorResult{Status: "ok", Message: "formatted"}), string(formatted) + "\n", ""

	default:
		return "", "", encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("unknown operation: %s", operation)})
	}
}

// HomepageYamlEdit edits a YAML file inside the homepage container (or local workspace).
func HomepageYamlEdit(cfg HomepageConfig, path, operation, dotPath string, setValue interface{}, logger *slog.Logger) string {
	if err := validateHomepageRelativePathArg(path, "path"); err != nil {
		return errJSON("%v", err)
	}
	logger.Info("[Homepage] YamlEdit", "path", path, "op", operation)

	if !checkDockerAvailable(cfg.DockerHost) {
		if cfg.WorkspacePath == "" {
			return homepageWorkspacePathNotConfiguredJSON()
		}
		fullPath := filepath.Join(cfg.WorkspacePath, filepath.FromSlash(path))
		cleanWS := filepath.Clean(cfg.WorkspacePath)
		cleanFull := filepath.Clean(fullPath)
		if cleanFull == cleanWS || !strings.HasPrefix(cleanFull, cleanWS+string(os.PathSeparator)) {
			return errJSON("path traversal not allowed in path %q", path)
		}
		return ExecuteYamlEditor(operation, fullPath, dotPath, setValue, cfg.WorkspacePath)
	}

	// Docker: read from container
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	readResult := DockerExec(dockerCfg, homepageContainerName, fmt.Sprintf("cat /workspace/%s", path), "")
	fileContent := extractDockerOutput(readResult)
	if fileContent == "" && operation != "set" {
		return errJSON("could not read file from container")
	}

	// Apply YAML operation on content
	result, edited, err := applyHomepageYamlEdit(fileContent, operation, dotPath, setValue)
	if err != "" {
		return err
	}

	// For read-only operations, return the result
	switch operation {
	case "get", "keys", "validate":
		return result
	}

	// Write edited content back
	encoded := base64.StdEncoding.EncodeToString([]byte(edited))
	dir := filepath.Dir(path)
	writeCmd := fmt.Sprintf("mkdir -p /workspace/%s && echo '%s' | base64 -d > /workspace/%s", dir, encoded, path)
	DockerExec(dockerCfg, homepageContainerName, writeCmd, "")
	return result
}

// applyHomepageYamlEdit performs YAML operations on in-memory content.
// Returns (resultJSON, editedContent, errorJSON). errorJSON is empty on success.
func applyHomepageYamlEdit(content, operation, dotPath string, setValue interface{}) (string, string, string) {
	encode := func(r JsonEditorResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	switch operation {
	case "get":
		var doc interface{}
		if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
			return "", "", encode(JsonEditorResult{Status: "error", Message: "invalid YAML: " + err.Error()})
		}
		if dotPath == "" {
			return encode(JsonEditorResult{Status: "ok", Data: doc}), "", ""
		}
		val, found := yamlNavigate(doc, strings.Split(dotPath, "."))
		if !found {
			return "", "", encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("path '%s' not found", dotPath)})
		}
		return encode(JsonEditorResult{Status: "ok", Data: val}), "", ""

	case "set":
		if dotPath == "" {
			return "", "", encode(JsonEditorResult{Status: "error", Message: "'json_path' is required for set"})
		}
		var node yaml.Node
		data := []byte(content)
		if len(data) > 0 {
			if err := yaml.Unmarshal(data, &node); err != nil {
				return "", "", encode(JsonEditorResult{Status: "error", Message: "invalid YAML: " + err.Error()})
			}
		}
		if node.Kind == 0 {
			node = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
		}
		parts := strings.Split(dotPath, ".")
		if err := yamlNodeSet(&node, parts, setValue); err != nil {
			return "", "", encode(JsonEditorResult{Status: "error", Message: err.Error()})
		}
		out, err := yaml.Marshal(&node)
		if err != nil {
			return "", "", encode(JsonEditorResult{Status: "error", Message: err.Error()})
		}
		return encode(JsonEditorResult{Status: "ok", Message: fmt.Sprintf("set '%s'", dotPath)}), string(out), ""

	case "delete":
		if dotPath == "" {
			return "", "", encode(JsonEditorResult{Status: "error", Message: "'json_path' is required for delete"})
		}
		var node yaml.Node
		if err := yaml.Unmarshal([]byte(content), &node); err != nil {
			return "", "", encode(JsonEditorResult{Status: "error", Message: "invalid YAML: " + err.Error()})
		}
		parts := strings.Split(dotPath, ".")
		if !yamlNodeDelete(&node, parts) {
			return "", "", encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("path '%s' not found", dotPath)})
		}
		out, err := yaml.Marshal(&node)
		if err != nil {
			return "", "", encode(JsonEditorResult{Status: "error", Message: err.Error()})
		}
		return encode(JsonEditorResult{Status: "ok", Message: fmt.Sprintf("deleted '%s'", dotPath)}), string(out), ""

	case "keys":
		var doc interface{}
		if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
			return "", "", encode(JsonEditorResult{Status: "error", Message: "invalid YAML: " + err.Error()})
		}
		target := doc
		if dotPath != "" {
			val, found := yamlNavigate(doc, strings.Split(dotPath, "."))
			if !found {
				return "", "", encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("path '%s' not found", dotPath)})
			}
			target = val
		}
		m, ok := target.(map[string]interface{})
		if !ok {
			return "", "", encode(JsonEditorResult{Status: "error", Message: "value at path is not a mapping"})
		}
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		return encode(JsonEditorResult{Status: "ok", Data: keys}), "", ""

	case "validate":
		var doc interface{}
		if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
			return encode(JsonEditorResult{Status: "ok", Message: "invalid YAML", Data: false}), "", ""
		}
		return encode(JsonEditorResult{Status: "ok", Message: "valid YAML", Data: true}), "", ""

	default:
		return "", "", encode(JsonEditorResult{Status: "error", Message: fmt.Sprintf("unknown operation: %s", operation)})
	}
}

// extractDockerOutput extracts the output text from a DockerExec JSON response.
func extractDockerOutput(result string) string {
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(result), &resp); err == nil {
		if status, ok := resp["status"].(string); ok && status == "error" {
			return ""
		}
		if output, ok := resp["output"].(string); ok {
			return output
		}
	}
	return result
}

// HomepageXmlEdit performs XML editing operations on homepage project files.
func HomepageXmlEdit(cfg HomepageConfig, path, operation, xpath string, setValue interface{}, logger *slog.Logger) string {
	if err := validateHomepageRelativePathArg(path, "path"); err != nil {
		return errJSON("%v", err)
	}
	logger.Info("[Homepage] XmlEdit", "path", path, "op", operation)

	if !checkDockerAvailable(cfg.DockerHost) {
		if cfg.WorkspacePath == "" {
			return homepageWorkspacePathNotConfiguredJSON()
		}
		fullPath := filepath.Join(cfg.WorkspacePath, filepath.FromSlash(path))
		cleanWS := filepath.Clean(cfg.WorkspacePath)
		cleanFull := filepath.Clean(fullPath)
		if cleanFull == cleanWS || !strings.HasPrefix(cleanFull, cleanWS+string(os.PathSeparator)) {
			return errJSON("path traversal not allowed in path %q", path)
		}
		return ExecuteXmlEditor(operation, fullPath, xpath, setValue, cfg.WorkspacePath)
	}

	// Docker: read from container
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	readResult := DockerExec(dockerCfg, homepageContainerName, fmt.Sprintf("cat /workspace/%s", path), "")
	fileContent := extractDockerOutput(readResult)
	if fileContent == "" && operation != "add_element" {
		return errJSON("could not read file from container")
	}

	// Apply XML operation on content
	result, edited, errMsg := applyHomepageXmlEdit(fileContent, operation, xpath, setValue)
	if errMsg != "" {
		return errMsg
	}

	// For read-only operations, return the result
	switch operation {
	case "get", "validate":
		return result
	}

	// Write edited content back
	encoded := base64.StdEncoding.EncodeToString([]byte(edited))
	dir := filepath.Dir(path)
	writeCmd := fmt.Sprintf("mkdir -p /workspace/%s && echo '%s' | base64 -d > /workspace/%s", dir, encoded, path)
	DockerExec(dockerCfg, homepageContainerName, writeCmd, "")
	return result
}

// applyHomepageXmlEdit performs XML operations on in-memory content.
func applyHomepageXmlEdit(content, operation, xpath string, setValue interface{}) (string, string, string) {
	encode := func(r XmlEditorResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	doc := etree.NewDocument()
	if content != "" {
		if err := doc.ReadFromString(content); err != nil {
			return "", "", encode(XmlEditorResult{Status: "error", Message: "invalid XML: " + err.Error()})
		}
	}

	switch operation {
	case "get":
		if xpath == "" {
			return "", "", encode(XmlEditorResult{Status: "error", Message: "'xpath' is required for get"})
		}
		elements := doc.FindElements(xpath)
		if len(elements) == 0 {
			return "", "", encode(XmlEditorResult{Status: "error", Message: fmt.Sprintf("No elements found for path '%s'", xpath)})
		}
		var results []map[string]interface{}
		for _, el := range elements {
			entry := map[string]interface{}{"tag": el.Tag, "text": strings.TrimSpace(el.Text())}
			if len(el.Attr) > 0 {
				attrs := make(map[string]string)
				for _, a := range el.Attr {
					key := a.Key
					if a.Space != "" {
						key = a.Space + ":" + key
					}
					attrs[key] = a.Value
				}
				entry["attributes"] = attrs
			}
			results = append(results, entry)
		}
		if len(results) == 1 {
			return encode(XmlEditorResult{Status: "success", Data: results[0]}), "", ""
		}
		return encode(XmlEditorResult{Status: "success", Data: results}), "", ""

	case "set_text":
		if xpath == "" {
			return "", "", encode(XmlEditorResult{Status: "error", Message: "'xpath' is required for set_text"})
		}
		text := fmt.Sprintf("%v", setValue)
		elements := doc.FindElements(xpath)
		if len(elements) == 0 {
			return "", "", encode(XmlEditorResult{Status: "error", Message: fmt.Sprintf("No elements found for path '%s'", xpath)})
		}
		for _, el := range elements {
			el.SetText(text)
		}
		doc.Indent(2)
		output, err := doc.WriteToString()
		if err != nil {
			return "", "", encode(XmlEditorResult{Status: "error", Message: err.Error()})
		}
		return encode(XmlEditorResult{Status: "success", Message: fmt.Sprintf("Set text on %d element(s)", len(elements))}), output, ""

	case "set_attribute":
		if xpath == "" {
			return "", "", encode(XmlEditorResult{Status: "error", Message: "'xpath' is required for set_attribute"})
		}
		attrs, ok := setValue.(map[string]interface{})
		if !ok {
			return "", "", encode(XmlEditorResult{Status: "error", Message: "set_value must be {name, value}"})
		}
		attrName, _ := attrs["name"].(string)
		if attrName == "" {
			return "", "", encode(XmlEditorResult{Status: "error", Message: "set_value.name is required"})
		}
		attrValue := fmt.Sprintf("%v", attrs["value"])
		elements := doc.FindElements(xpath)
		if len(elements) == 0 {
			return "", "", encode(XmlEditorResult{Status: "error", Message: fmt.Sprintf("No elements found for path '%s'", xpath)})
		}
		for _, el := range elements {
			el.CreateAttr(attrName, attrValue)
		}
		doc.Indent(2)
		output, err := doc.WriteToString()
		if err != nil {
			return "", "", encode(XmlEditorResult{Status: "error", Message: err.Error()})
		}
		return encode(XmlEditorResult{Status: "success", Message: fmt.Sprintf("Set attribute '%s' on %d element(s)", attrName, len(elements))}), output, ""

	case "add_element":
		if xpath == "" {
			return "", "", encode(XmlEditorResult{Status: "error", Message: "'xpath' is required for add_element"})
		}
		spec, ok := setValue.(map[string]interface{})
		if !ok {
			return "", "", encode(XmlEditorResult{Status: "error", Message: "set_value must be {tag, text?, attributes?}"})
		}
		tag, _ := spec["tag"].(string)
		if tag == "" {
			return "", "", encode(XmlEditorResult{Status: "error", Message: "set_value.tag is required"})
		}
		parents := doc.FindElements(xpath)
		if len(parents) == 0 {
			return "", "", encode(XmlEditorResult{Status: "error", Message: fmt.Sprintf("No parent elements found for path '%s'", xpath)})
		}
		for _, parent := range parents {
			child := parent.CreateElement(tag)
			if text, ok := spec["text"].(string); ok {
				child.SetText(text)
			}
			if childAttrs, ok := spec["attributes"].(map[string]interface{}); ok {
				for k, v := range childAttrs {
					child.CreateAttr(k, fmt.Sprintf("%v", v))
				}
			}
		}
		doc.Indent(2)
		output, err := doc.WriteToString()
		if err != nil {
			return "", "", encode(XmlEditorResult{Status: "error", Message: err.Error()})
		}
		return encode(XmlEditorResult{Status: "success", Message: fmt.Sprintf("Added <%s> to %d parent(s)", tag, len(parents))}), output, ""

	case "delete":
		if xpath == "" {
			return "", "", encode(XmlEditorResult{Status: "error", Message: "'xpath' is required for delete"})
		}
		elements := doc.FindElements(xpath)
		if len(elements) == 0 {
			return "", "", encode(XmlEditorResult{Status: "error", Message: fmt.Sprintf("No elements found for path '%s'", xpath)})
		}
		count := 0
		for _, el := range elements {
			if p := el.Parent(); p != nil {
				p.RemoveChild(el)
				count++
			}
		}
		doc.Indent(2)
		output, err := doc.WriteToString()
		if err != nil {
			return "", "", encode(XmlEditorResult{Status: "error", Message: err.Error()})
		}
		return encode(XmlEditorResult{Status: "success", Message: fmt.Sprintf("Deleted %d element(s)", count)}), output, ""

	case "validate":
		root := doc.Root()
		if root == nil {
			return encode(XmlEditorResult{Status: "error", Message: "XML document has no root element"}), "", ""
		}
		return encode(XmlEditorResult{Status: "success", Message: "Valid XML", Data: map[string]interface{}{"root_tag": root.Tag}}), "", ""

	case "format":
		doc.Indent(2)
		output, err := doc.WriteToString()
		if err != nil {
			return "", "", encode(XmlEditorResult{Status: "error", Message: err.Error()})
		}
		return encode(XmlEditorResult{Status: "success", Message: "File formatted"}), output, ""

	default:
		return "", "", encode(XmlEditorResult{Status: "error", Message: fmt.Sprintf("unknown operation: %s", operation)})
	}
}

// HomepageOptimizeImages runs SVG optimization on the project.
