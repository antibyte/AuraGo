package tools

import (
	"context"
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
	"sync"
	"time"
)

// HomepageConfig holds the configuration for the homepage dev environment.
type HomepageConfig struct {
	DockerHost            string
	WorkspacePath         string // host path mounted as /workspace in the container
	AgentWorkspaceDir     string // agent workdir (filesystem tool writes here); used as fallback when WorkspacePath differs
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

// activePythonServerCmd tracks the last Python HTTP server started as fallback.
var (
	activePythonServerCmd *exec.Cmd
	activePythonServerMu  sync.Mutex
)

// dockerAvailabilityCache caches DockerPing results per host to prevent flip-flopping
// between Docker and local-fallback mode within a single operation sequence.
var (
	dockerAvailabilityMu      sync.Mutex
	dockerAvailabilityResults = make(map[string]dockerAvailabilityEntry)
)

type dockerAvailabilityEntry struct {
	available bool
	expiry    time.Time
}

const dockerAvailabilityCacheTTL = 10 * time.Second

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

// â”€â”€â”€ Helpers â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

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
	// Reject shell metacharacters that could be used for command injection.
	const shellMetachars = ";|&`$(){}[]<>!\\'\""
	for _, ch := range shellMetachars {
		if strings.ContainsRune(trimmed, ch) {
			return fmt.Errorf("invalid character %q in %s %q", ch, field, path)
		}
	}
	normalized := filepath.ToSlash(trimmed)
	if filepath.IsAbs(trimmed) || strings.HasPrefix(normalized, "/") || strings.HasPrefix(normalized, homepageWorkspaceMount+"/") {
		return fmt.Errorf("%s must be relative to the homepage workspace, e.g. 'my-site/src/app/page.tsx' not %q", field, path)
	}
	return nil
}

// resolveHomepagePath resolves a workspace-relative path and validates that the
// result stays within the workspace root. Does not allow the workspace root itself.
// Returns (fullPath, nil) on success, or ("", error) on path traversal.
func resolveHomepagePath(workspacePath, relPath string) (string, error) {
	fullPath := filepath.Join(workspacePath, filepath.FromSlash(relPath))
	cleanWS := filepath.Clean(workspacePath)
	cleanFull := filepath.Clean(fullPath)
	if cleanFull == cleanWS || !strings.HasPrefix(cleanFull, cleanWS+string(os.PathSeparator)) {
		return "", fmt.Errorf("path traversal not allowed in path %q", relPath)
	}
	return fullPath, nil
}

// truncateStr returns s truncated to maxLen characters with "â€¦" suffix.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "â€¦"
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

// â”€â”€â”€ Container Lifecycle â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

// checkDockerAvailable checks if Docker is available and running.
// Results are cached per dockerHost for dockerAvailabilityCacheTTL to prevent
// successive homepage tool calls from flip-flopping between Docker and local-fallback
// mode when Docker has transient slowness.
func checkDockerAvailable(dockerHost string) bool {
	dockerAvailabilityMu.Lock()
	defer dockerAvailabilityMu.Unlock()
	now := time.Now()
	if entry, ok := dockerAvailabilityResults[dockerHost]; ok && now.Before(entry.expiry) {
		return entry.available
	}
	available := DockerPing(dockerHost) == nil
	dockerAvailabilityResults[dockerHost] = dockerAvailabilityEntry{
		available: available,
		expiry:    now.Add(dockerAvailabilityCacheTTL),
	}
	return available
}

// invalidateDockerAvailabilityCache removes the cached availability result for dockerHost,
// forcing the next call to checkDockerAvailable to re-probe the daemon.
func invalidateDockerAvailabilityCache(dockerHost string) {
	dockerAvailabilityMu.Lock()
	delete(dockerAvailabilityResults, dockerHost)
	dockerAvailabilityMu.Unlock()
}

// startPythonServer starts a Python HTTP server as fallback when Docker is not available.
// Returns the URL and process info or error.
// startPythonServer starts a Python HTTP server as fallback when Docker is not available.
// Returns the URL and process info or error. Kills any previously started server first.
func startPythonServer(port int, directory string) (string, int, error) {
	if port <= 0 {
		port = 8080
	}

	// Kill any previously tracked Python server to avoid orphaned processes.
	activePythonServerMu.Lock()
	if activePythonServerCmd != nil && activePythonServerCmd.Process != nil {
		_ = activePythonServerCmd.Process.Kill()
		_ = activePythonServerCmd.Wait()
		activePythonServerCmd = nil
	}
	activePythonServerMu.Unlock()

	cmd := exec.Command("python3", "-m", "http.server",
		strconv.Itoa(port), "--directory", directory, "--bind", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		return "", 0, fmt.Errorf("failed to start Python server: %w", err)
	}

	activePythonServerMu.Lock()
	activePythonServerCmd = cmd
	activePythonServerMu.Unlock()

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

	// Invalidate cached Docker availability so fresh state is probed after init.
	invalidateDockerAvailabilityCache(cfg.DockerHost)

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
		// Container exists â€” check if running
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

	// Create new container â€” run as the current UID/GID so bind-mounted
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

// â”€â”€â”€ Dev Commands (Token-Saving Compound Operations) â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

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
			// Pure HTML â€” create locally without any build tool
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
			// Framework needs npm/npx â€” try local npx if available
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
// Plain HTML projects (no package.json) are detected and skipped â€” they need no build step.
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

	// Detect plain HTML projects: no package.json â†’ no build needed.
	if cfg.WorkspacePath != "" {
		pkgPath := filepath.Join(cfg.WorkspacePath, projectDir, "package.json")
		if _, err := os.Stat(pkgPath); err != nil {
			logger.Info("[Homepage] No package.json found â€” plain HTML project, skipping build")
			out, _ := json.Marshal(map[string]interface{}{
				"status": "ok",
				"output": "Plain HTML project â€” no build required",
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
	// Run TypeScript check first (if tsconfig exists), then ESLint.
	// Prefer the project-local ESLint binary (faster, no npx lookup overhead).
	cmd := fmt.Sprintf(`cd /workspace/%s && { if [ -f tsconfig.json ]; then echo "=== TypeScript Check ==="; npx tsc --noEmit 2>&1 | head -50; echo; fi; echo "=== ESLint ==="; if [ -f ./node_modules/.bin/eslint ]; then ./node_modules/.bin/eslint . --format compact 2>&1; else npx eslint . --format compact 2>&1; fi | head -100; }`, projectDir)
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
