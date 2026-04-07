package tools

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"aurago/internal/remote"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func decorateHomepageBuildFailure(raw string, projectDir string) string {
	trimmed := strings.TrimSpace(extractOutput(raw))
	if trimmed == "" {
		return raw
	}

	lower := strings.ToLower(trimmed)
	switch {
	case strings.Contains(lower, `missing script: "build"`):
		return errJSON("Build failed because package.json in project_dir %q has no 'build' script. If this is a static HTML project, remove the unused package.json or deploy the project directory directly. Otherwise add a build script and retry. Original build output: %s", projectDir, truncateStr(trimmed, 500))
	case strings.Contains(lower, "enoent") && strings.Contains(lower, "package.json"):
		return errJSON("Build failed because package.json could not be found for project_dir %q. Verify that project_dir is relative to the homepage workspace and that the project files were created with homepage write_file, not filesystem. Original build output: %s", projectDir, truncateStr(trimmed, 500))
	default:
		return raw
	}
}

// HomepageDetectWorkspacePath inspects the running homepage dev container and
// returns the host path that is bind-mounted as the workspace (/workspace inside
// the container).  This lets the Config UI auto-fill the workspace_path field
// without the user having to look up the path manually.
func HomepageDetectWorkspacePath(cfg HomepageConfig, logger *slog.Logger) string {
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	if !checkDockerAvailable(cfg.DockerHost) {
		return errJSON("Docker not available — cannot auto-detect workspace_path. Enter the absolute host workspace manually. %s", homepageWorkspacePathGuidance())
	}

	data, code, err := dockerRequest(dockerCfg, "GET", "/containers/"+homepageContainerName+"/json", "")
	if err != nil {
		return errJSON("Failed to inspect the homepage dev container: %v. If auto-detect keeps failing, enter workspace_path manually. %s", err, homepageWorkspacePathGuidance())
	}
	if code == 404 {
		return errJSON("Homepage dev container (%s) is not running. Start or initialize homepage first, or enter workspace_path manually. %s", homepageContainerName, homepageWorkspacePathGuidance())
	}
	if code != 200 {
		return errJSON("Unexpected Docker response (code %d) while inspecting the homepage container. %s", code, homepageWorkspacePathGuidance())
	}

	var inspect struct {
		Mounts []struct {
			Type        string `json:"Type"`
			Source      string `json:"Source"`
			Destination string `json:"Destination"`
		} `json:"Mounts"`
	}
	if err := json.Unmarshal(data, &inspect); err != nil {
		return errJSON("Failed to parse container inspect data: %v", err)
	}

	// The homepage container mounts the workspace at homepageWorkspaceMount ("/workspace").
	for _, m := range inspect.Mounts {
		if m.Type == "bind" && (m.Destination == homepageWorkspaceMount ||
			strings.HasSuffix(m.Destination, "/workspace")) {
			logger.Info("[Homepage] Auto-detected workspace path", "host_path", m.Source)
			return okJSON(
				"Workspace path auto-detected from the homepage container mount. In homepage tool calls, keep project_dir and path relative to the workspace.",
				"path", m.Source,
				"mount_path", homepageWorkspaceMount,
				"project_dir_example", "my-site",
			)
		}
	}

	return errJSON("No workspace bind-mount found in container '%s'. The container may not have been created by AuraGo, or the workspace mount is missing. %s", homepageContainerName, homepageWorkspacePathGuidance())
}

// HomepageOptimizeImages optimizes SVG files in the project using SVGO.
// Note: only SVG files are processed; PNG/JPEG optimization is not supported.
func HomepageOptimizeImages(cfg HomepageConfig, projectDir string, logger *slog.Logger) string {
	if projectDir == "" {
		projectDir = "."
	}
	if projectDir != "." {
		if err := sanitizeProjectDir(projectDir); err != nil {
			return errJSON("%v", err)
		}
	}
	logger.Info("[Homepage] OptimizeImages", "dir", projectDir)
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	cmd := fmt.Sprintf(`cd /workspace/%s && echo '{"svgs":' && svgo -f . -r --multipass -q 2>/dev/null && echo ',"summary":"SVG optimization complete"}'`, projectDir)
	return DockerExec(dockerCfg, homepageContainerName, cmd, "")
}

// HomepageDev starts the dev server in the container.
func HomepageDev(cfg HomepageConfig, projectDir string, port int, logger *slog.Logger) string {
	if projectDir == "" {
		projectDir = "."
	}
	if projectDir != "." {
		if err := sanitizeProjectDir(projectDir); err != nil {
			return errJSON("%v", err)
		}
	}
	if port == 0 {
		port = 3000
	}
	logger.Info("[Homepage] Dev server start", "dir", projectDir, "port", port)
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	// Start dev server in background (detached)
	cmd := fmt.Sprintf("cd /workspace/%s && nohup npm run dev -- --port %d > /tmp/dev-server.log 2>&1 &", projectDir, port)
	return DockerExec(dockerCfg, homepageContainerName, cmd, "")
}

// ─── Deployment via SFTP/SCP ──────────────────────────────────────────────

// HomepageDeploy uploads the build output to a remote server via SFTP.
func HomepageDeploy(cfg HomepageConfig, deployCfg HomepageDeployConfig, projectDir, buildDir string, logger *slog.Logger) string {
	if projectDir != "" && projectDir != "." {
		if err := sanitizeProjectDir(projectDir); err != nil {
			return errJSON("%v", err)
		}
	}
	if buildDir != "" && buildDir != "." {
		if err := sanitizeProjectDir(buildDir); err != nil {
			return errJSON("%v", err)
		}
	}
	if deployCfg.Host == "" || deployCfg.User == "" || deployCfg.Path == "" {
		return errJSON("Deploy requires host, user, and path to be configured")
	}
	if deployCfg.Port == 0 {
		deployCfg.Port = 22
	}

	// Determine authentication secret
	secret := []byte(deployCfg.Password)
	if deployCfg.Key != "" {
		secret = []byte(deployCfg.Key)
	}
	if len(secret) == 0 {
		return errJSON("Deploy requires either a password or SSH key in the vault (homepage_deploy_password or homepage_deploy_key)")
	}

	// First, build the project
	if buildDir == "" {
		logger.Info("[Homepage] Building before deploy")
		buildResult := HomepageBuild(cfg, projectDir, logger)
		var br map[string]interface{}
		if json.Unmarshal([]byte(buildResult), &br) == nil {
			if s, _ := br["status"].(string); s == "error" {
				return errJSON("Build failed before deploy for project_dir %q. Review the build output and project structure, then try a different approach instead of repeating the same deploy call. Details: %s", projectDir, decorateHomepageBuildFailure(buildResult, projectDir))
			}
		}
	}

	// Detect build output directory
	if buildDir == "" {
		buildDir = detectBuildDir(cfg, projectDir)
	}

	// Get file list from build directory inside the container
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	hostBuildPath := filepath.Join(cfg.WorkspacePath, projectDir, buildDir)

	// Walk the local build directory and upload via SFTP
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	logger.Info("[Homepage] Deploying", "host", deployCfg.Host, "remote_path", deployCfg.Path, "local_build", hostBuildPath)

	// Upload directory recursively using SFTP
	result := sftpUploadDir(ctx, deployCfg, hostBuildPath, deployCfg.Path, secret, logger)
	if result != "" {
		return result
	}

	// Also run any post-deploy command inside the container (optional clean step)
	_ = DockerExec(dockerCfg, homepageContainerName, "echo 'Deploy complete'", "")

	return okJSON("Deployment complete", "host", deployCfg.Host, "path", deployCfg.Path, "build_dir", buildDir)
}

// HomepageTestConnection tests the SFTP/SCP connection.
func HomepageTestConnection(deployCfg HomepageDeployConfig, logger *slog.Logger) string {
	if deployCfg.Host == "" || deployCfg.User == "" {
		return errJSON("host and user are required for connection test")
	}
	if deployCfg.Port == 0 {
		deployCfg.Port = 22
	}

	secret := []byte(deployCfg.Password)
	if deployCfg.Key != "" {
		secret = []byte(deployCfg.Key)
	}
	if len(secret) == 0 {
		return errJSON("No password or SSH key configured. Store credentials in the vault first.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	logger.Info("[Homepage] Testing connection", "host", deployCfg.Host, "port", deployCfg.Port, "user", deployCfg.User)

	// Test SSH connection by running a simple command
	output, err := remote.ExecuteRemoteCommand(ctx, deployCfg.Host, deployCfg.Port, deployCfg.User, secret, "echo ok && ls -la -- "+shellSingleQuote(deployCfg.Path)+" 2>&1 | head -5")
	if err != nil {
		return errJSON("Connection failed: %v", err)
	}

	return okJSON("Connection successful", "output", strings.TrimSpace(output))
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

// ─── Caddy Web Server ─────────────────────────────────────────────────────

// HomepageWebServerStart starts the Caddy container serving the build output.
func HomepageWebServerStart(cfg HomepageConfig, projectDir, buildDir string, logger *slog.Logger) string {
	if projectDir != "" && projectDir != "." {
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
		buildDir = detectBuildDir(cfg, projectDir)
	}

	hostBuildPath := filepath.Join(cfg.WorkspacePath, projectDir, buildDir)
	if _, err := os.Stat(hostBuildPath); err != nil {
		return errJSON("Local publish source does not exist: %s. homepage write_file/read_file operate in the dev workspace, while published local sites are served by container %q from document root /srv. Use homepage build/publish_local/webserver_start instead of copying files to /var/www/html.", hostBuildPath, homepageWebContainer)
	}
	port := cfg.WebServerPort
	if port == 0 {
		port = 8080
	}

	// Check if Docker is available
	dockerCfg := DockerConfig{Host: cfg.DockerHost}

	// Copy any referenced generated images into the build directory so
	// Caddy/Python can serve them as static assets (the main AuraGo server's
	// /files/generated_images/ route is NOT reachable from the separate
	// homepage web server).
	copyAssetsToBuildDir(hostBuildPath, cfg.DataDir, logger)

	if !checkDockerAvailable(cfg.DockerHost) {
		// Check if local Python server is allowed (Danger Zone)
		if !cfg.AllowLocalServer {
			return errJSON("Docker not available and local Python server is disabled for security. " +
				"Please ensure Docker is running (systemctl start docker) or enable homepage.allow_local_server in config.yaml.")
		}

		logger.Info("[Homepage] Docker not available, using Python fallback for web server")

		// Check if already running
		if c, dialErr := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), time.Second); dialErr == nil {
			c.Close()
			return okJSON("Web server already running (Python fallback)",
				"url", fmt.Sprintf("http://localhost:%d", port),
				"served_url", fmt.Sprintf("http://localhost:%d", port),
				"port", fmt.Sprintf("%d", port),
				"mode", "python",
				"deploy_target", "python_fallback",
				"source_path", hostBuildPath)
		}

		// Use Python HTTP server as fallback
		url, pid, err := startPythonServer(port, hostBuildPath)
		if err != nil {
			return errJSON("Failed to start web server (Docker not available, Python fallback failed): %v", err)
		}

		logger.Info("[Homepage] Web server started (Python fallback)", "url", url, "pid", pid)
		return okJSON("Web server started (Python fallback)",
			"url", url,
			"served_url", url,
			"port", fmt.Sprintf("%d", port),
			"pid", strconv.Itoa(pid),
			"mode", "python",
			"deploy_target", "python_fallback",
			"source_path", hostBuildPath,
			"note", "Limited functionality. Full features require Docker.")
	}

	// Docker is available - use Caddy
	// Pull image first so we never fail with "image not found" on first run.
	// Use a 5-minute context because first-time pulls can be slow.
	pullCtx, pullCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer pullCancel()
	if err := PullImageWait(pullCtx, dockerCfg, homepageWebImage, logger); err != nil {
		return errJSON("Failed to pull Caddy image: %v", err)
	}

	// Check if already running
	_, code, _ := dockerRequest(dockerCfg, "GET", "/containers/"+homepageWebContainer+"/json", "")
	if code == 200 {
		// Stop and remove existing
		DockerContainerAction(dockerCfg, homepageWebContainer, "stop", false)
		DockerContainerAction(dockerCfg, homepageWebContainer, "remove", true)
	}

	// Generate Caddyfile
	caddyfile := homepageCaddyfile(cfg.WebServerDomain, port)

	// Write Caddyfile to workspace
	caddyfilePath := filepath.Join(cfg.WorkspacePath, ".aurago-Caddyfile")
	if err := os.WriteFile(caddyfilePath, []byte(caddyfile), 0644); err != nil {
		return errJSON("Failed to write Caddyfile: %v", err)
	}

	// Determine bind address: internal-only (127.0.0.1) or all interfaces (0.0.0.0).
	hostIP := "0.0.0.0"
	if cfg.WebServerInternalOnly {
		hostIP = "127.0.0.1"
	}

	// Create Caddy container
	payload := map[string]interface{}{
		"Image": homepageWebImage,
		"ExposedPorts": map[string]interface{}{
			fmt.Sprintf("%d/tcp", port): map[string]interface{}{},
		},
		"HostConfig": map[string]interface{}{
			"Binds": []string{
				hostBuildPath + ":/srv",
				caddyfilePath + ":/etc/caddy/Caddyfile",
			},
			"PortBindings": map[string]interface{}{
				"80/tcp": []map[string]interface{}{
					{"HostIp": hostIP, "HostPort": fmt.Sprintf("%d", port)},
				},
			},
			"RestartPolicy": map[string]string{"Name": "unless-stopped"},
		},
	}
	if cfg.WebServerDomain != "" {
		// If domain is set, also bind 443
		payload["HostConfig"].(map[string]interface{})["PortBindings"].(map[string]interface{})["443/tcp"] = []map[string]interface{}{
			{"HostIp": hostIP, "HostPort": "443"},
		}
	}

	body, _ := json.Marshal(payload)
	createData, createCode, createErr := dockerRequest(dockerCfg, "POST", "/containers/create?name="+homepageWebContainer, string(body))
	if createErr != nil {
		return errJSON("Failed to create web server container: %v", createErr)
	}
	if createCode != 201 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, createCode, string(createData))
	}

	_, startCode, startErr := dockerRequest(dockerCfg, "POST", "/containers/"+homepageWebContainer+"/start", "")
	if startErr != nil || (startCode != 204 && startCode != 304) {
		return errJSON("Failed to start web server: code=%d err=%v", startCode, startErr)
	}

	logger.Info("[Homepage] Web server started", "port", port, "domain", cfg.WebServerDomain)
	url := fmt.Sprintf("http://localhost:%d", port)
	if cfg.WebServerDomain != "" {
		url = "https://" + cfg.WebServerDomain
	}
	lanIP := getLocalLANIP()
	args := []string{
		"url", url,
		"served_url", url,
		"port", fmt.Sprintf("%d", port),
		"mode", "docker",
		"deploy_target", "caddy_web_root",
		"container_name", homepageWebContainer,
		"document_root", "/srv",
		"config_path", "/etc/caddy/Caddyfile",
		"source_path", hostBuildPath,
	}
	if lanIP != "" && !cfg.WebServerInternalOnly {
		args = append(args, "lan_url", fmt.Sprintf("http://%s:%d", lanIP, port))
	}
	return okJSON("Web server started", args...)
}

// HomepageWebServerStop stops the Caddy container.
func HomepageWebServerStop(cfg HomepageConfig, logger *slog.Logger) string {
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	logger.Info("[Homepage] Stopping web server")
	DockerContainerAction(dockerCfg, homepageWebContainer, "stop", false)
	return DockerContainerAction(dockerCfg, homepageWebContainer, "remove", true)
}

// HomepageWebServerStatus returns the status of the Caddy web server.
func HomepageWebServerStatus(cfg HomepageConfig, logger *slog.Logger) string {
	if !checkDockerAvailable(cfg.DockerHost) {
		// Check Python server via TCP probe
		port := cfg.WebServerPort
		if port <= 0 {
			port = 8080
		}
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 2*time.Second)
		if err == nil {
			conn.Close()
			out, _ := json.Marshal(map[string]interface{}{
				"running": true,
				"mode":    "python",
				"url":     fmt.Sprintf("http://localhost:%d", port),
			})
			return string(out)
		}
		return `{"running":false,"mode":"python","exists":false}`
	}
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	return containerStatus(dockerCfg, homepageWebContainer)
}

// getLocalLANIP returns the first non-loopback IPv4 address of the host,
// or empty string if none found.
func getLocalLANIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
			return ipNet.IP.String()
		}
	}
	return ""
}

// extractTrycloudflareURL scans plain text for a trycloudflare.com URL and returns
// the first match, or empty string if none found.
func extractTrycloudflareURL(text string) string {
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "trycloudflare.com") {
			// Extract just the URL portion — the line may contain surrounding text.
			for _, word := range strings.Fields(line) {
				if strings.HasPrefix(word, "https://") && strings.Contains(word, "trycloudflare.com") {
					return strings.TrimRight(word, ".,;\"')")
				}
			}
			// Fallback: return the whole trimmed line.
			return line
		}
	}
	return ""
}

// HomepageTunnel starts a Cloudflare quick tunnel inside the Docker container
// that exposes a local port to the internet with a temporary *.trycloudflare.com URL.
// The tunnel runs in the background; use HomepageExec to check its status.
func HomepageTunnel(cfg HomepageConfig, port int, logger *slog.Logger) string {
	if port <= 0 {
		port = 3000
	}
	logger.Info("[Homepage] Starting Cloudflare tunnel", "port", port)
	dockerCfg := DockerConfig{Host: cfg.DockerHost}

	// Check if cloudflared is available
	checkResult := DockerExec(dockerCfg, homepageContainerName, "which cloudflared 2>/dev/null && echo FOUND || echo MISSING", "")
	if !strings.Contains(checkResult, "FOUND") {
		// Provide LAN IP as fallback
		lanIP := getLocalLANIP()
		msg := "cloudflared not available in container. Rebuild the homepage container to get tunnel support."
		if lanIP != "" {
			msg += fmt.Sprintf(" LAN URL: http://%s:%d", lanIP, port)
		}
		return errJSON("%s", msg)
	}

	// Start tunnel in background. Give cloudflared 12 seconds to register with
	// Cloudflare and write the URL to the log (3s was too short; real-world
	// registration typically takes 8–15 seconds).
	cmd := fmt.Sprintf("nohup cloudflared tunnel --url http://localhost:%d > /tmp/tunnel.log 2>&1 & sleep 12 && grep -oP 'https://[a-z0-9-]+\\.trycloudflare\\.com' /tmp/tunnel.log | head -1", port)
	result := DockerExec(dockerCfg, homepageContainerName, cmd, "")

	// Try to extract the tunnel URL from the initial output.
	if tunnelURL := extractTrycloudflareURL(extractOutput(result)); tunnelURL != "" {
		return okJSON("Cloudflare tunnel started", "tunnel_url", tunnelURL, "local_port", fmt.Sprintf("%d", port))
	}

	// cloudflared may still be registering. Poll the log file up to 3 more times
	// with 5-second gaps before giving up.
	for i := 0; i < 3; i++ {
		time.Sleep(5 * time.Second)
		pollResult := DockerExec(dockerCfg, homepageContainerName, "grep -oP 'https://[a-z0-9-]+\\.trycloudflare\\.com' /tmp/tunnel.log | head -1", "")
		if tunnelURL := extractTrycloudflareURL(extractOutput(pollResult)); tunnelURL != "" {
			logger.Info("[Homepage] Cloudflare tunnel URL found after polling", "attempt", i+1, "url", tunnelURL)
			return okJSON("Cloudflare tunnel started", "tunnel_url", tunnelURL, "local_port", fmt.Sprintf("%d", port))
		}
	}

	// Still no URL — return diagnostics so the agent can investigate.
	lanIP := getLocalLANIP()
	logContents := extractOutput(DockerExec(dockerCfg, homepageContainerName, "cat /tmp/tunnel.log 2>/dev/null | tail -20", ""))
	return okJSON("Tunnel process started but URL not yet available. cloudflared may need more time or cannot reach Cloudflare servers.",
		"local_port", fmt.Sprintf("%d", port),
		"lan_ip", lanIP,
		"tunnel_log_tail", logContents,
		"check_cmd", "cat /tmp/tunnel.log | grep -E 'trycloudflare|error|ERR'")
}

// HomepageDeployNetlify builds the project (if a build script exists) and deploys it to
// Netlify by creating an in-memory ZIP of the build/project directory and calling the
// Netlify Deploy API. The caller supplies a fully-resolved NetlifyConfig.
func HomepageDeployNetlify(cfg HomepageConfig, nfCfg NetlifyConfig, projectDir, buildDir, siteID, title string, draft bool, logger *slog.Logger) string {
	if nfCfg.Token == "" {
		return errJSON("Netlify token is required")
	}
	if cfg.WorkspacePath == "" {
		return errJSON("Homepage workspace path is not configured")
	}
	if projectDir != "" && projectDir != "." {
		if err := sanitizeProjectDir(projectDir); err != nil {
			return errJSON("%v", err)
		}
	}
	if buildDir != "" && buildDir != "." {
		if err := sanitizeProjectDir(buildDir); err != nil {
			return errJSON("%v", err)
		}
	}
	if projectDir == "" {
		projectDir = "."
	}

	// Try to build first; ignore failure for plain-HTML projects that have no build script.
	if buildDir == "" {
		// For Next.js projects, ensure output: 'export' is configured before building.
		// Without it, `next build` produces only a .next/ server bundle that Netlify cannot serve.
		if cfg.WorkspacePath != "" {
			projectRoot := filepath.Join(cfg.WorkspacePath, projectDir)
			if isNextJsProject(projectRoot) {
				if patched := ensureNextJsStaticExport(projectRoot, logger); patched {
					logger.Info("[Homepage] Netlify: Next.js config patched for static export — building now")
				}
			}
		}
		logger.Info("[Homepage] Attempting build before Netlify deploy", "dir", projectDir)
		buildResult := HomepageBuild(cfg, projectDir, logger)
		var br map[string]interface{}
		if err := json.Unmarshal([]byte(buildResult), &br); err == nil {
			if s, _ := br["status"].(string); s != "error" {
				// Build succeeded — detect the output directory.
				buildDir = detectBuildDir(cfg, projectDir)
			} else {
				return decorateHomepageBuildFailure(buildResult, projectDir)
			}
		}
	}

	// If detectBuildDir returned ".next", the build was a Next.js server build (not static).
	// Patch the config and rebuild to produce a proper static output directory.
	if buildDir == ".next" && cfg.WorkspacePath != "" {
		projectRoot := filepath.Join(cfg.WorkspacePath, projectDir)
		logger.Info("[Homepage] Netlify: .next server build detected — patching Next.js config for static export and rebuilding")
		ensureNextJsStaticExport(projectRoot, logger)
		rebuildResult := HomepageBuild(cfg, projectDir, logger)
		var rb map[string]interface{}
		if err := json.Unmarshal([]byte(rebuildResult), &rb); err == nil {
			if s, _ := rb["status"].(string); s != "error" {
				buildDir = detectBuildDir(cfg, projectDir)
			}
		}
		// If still .next after rebuild, fall through to project root (deploy will likely fail,
		// but at least we tried and the error will be visible in the deploy result).
		if buildDir == ".next" {
			buildDir = ""
		}
	}

	// Resolve the host-side path to zip.
	var deployPath string
	if buildDir == "" || buildDir == "." {
		projectRoot := filepath.Join(cfg.WorkspacePath, projectDir)
		// If no explicit buildDir was given, check whether a standard build output
		// subdirectory (out, dist, build) already contains an index.html.
		// This prevents the ZIP from including the whole project tree with the build
		// output nested under a subdirectory (e.g. out/index.html instead of index.html),
		// which would make Netlify return "Page not found" even with SPA redirects.
		detected := ""
		for _, sub := range []string{"out", "dist", "build"} {
			candidate := filepath.Join(projectRoot, sub, "index.html")
			if _, err := os.Stat(candidate); err == nil {
				detected = sub
				break
			}
		}
		if detected != "" {
			logger.Info("[Homepage] Auto-detected build output subdirectory", "subdir", detected)
			deployPath = filepath.Join(projectRoot, detected)
		} else {
			deployPath = projectRoot
		}
	} else {
		deployPath = filepath.Join(cfg.WorkspacePath, projectDir, buildDir)
	}

	// Verify the deploy path exists.
	if _, err := os.Stat(deployPath); err != nil {
		return errJSON("Deploy path does not exist: %s. project_dir must be relative to the homepage workspace, and homepage project files must be created with homepage write_file/read_file instead of the filesystem tool. For static sites, ensure the project root or a dist/build/out directory contains index.html. Local published sites are served by container %q from /srv, not /var/www/html.", deployPath, homepageWebContainer)
	}

	logger.Info("[Homepage] Packaging for Netlify deploy", "path", deployPath)

	// Create an in-memory ZIP of the deploy directory.
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)

	// Track which special Netlify config files are already present in the project.
	var hasHeaders, hasNetlifyToml, hasRedirects bool

	// Collect /files/<subdir>/<name> references found in HTML/CSS/JS files so we
	// can bundle the actual files into the ZIP (they're served by AuraGo locally
	// but must be included as static assets for Netlify to serve them).
	type assetRef struct {
		subdir string
		name   string
	}
	referencedAssets := make(map[assetRef]struct{})
	assetRegexes := []struct {
		re     *regexp.Regexp
		subdir string
	}{
		{generatedImageRefRegex, "generated_images"},
		{audioFileRefRegex, "audio"},
		{documentFileRefRegex, "documents"},
	}

	walkErr := filepath.Walk(deployPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(deployPath, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "_headers" {
			hasHeaders = true
		}
		if rel == "netlify.toml" {
			hasNetlifyToml = true
		}
		if rel == "_redirects" {
			hasRedirects = true
		}
		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lowerRel := strings.ToLower(rel)
		if strings.HasSuffix(lowerRel, ".html") || strings.HasSuffix(lowerRel, ".htm") ||
			strings.HasSuffix(lowerRel, ".css") || strings.HasSuffix(lowerRel, ".js") {
			for _, ar := range assetRegexes {
				for _, m := range ar.re.FindAllSubmatch(data, -1) {
					if len(m) > 1 {
						referencedAssets[assetRef{subdir: ar.subdir, name: string(m[1])}] = struct{}{}
					}
				}
			}
		}
		_, err = w.Write(data)
		return err
	})
	if walkErr != nil {
		return errJSON("Failed to create ZIP: %v", walkErr)
	}

	// Bundle any referenced assets into the ZIP so Netlify can serve
	// them at the same /files/<subdir>/<name> path.
	if cfg.DataDir != "" && len(referencedAssets) > 0 {
		bundled := 0
		for ref := range referencedAssets {
			srcPath := filepath.Join(cfg.DataDir, ref.subdir, filepath.Base(ref.name))
			srcData, readErr := os.ReadFile(srcPath)
			if readErr != nil {
				logger.Warn("[Homepage] Could not bundle asset", "subdir", ref.subdir, "file", ref.name, "error", readErr)
				continue
			}
			zipPath := "files/" + ref.subdir + "/" + filepath.Base(ref.name)
			if iw, werr := zw.Create(zipPath); werr == nil {
				_, _ = iw.Write(srcData)
				bundled++
			}
		}
		if bundled > 0 {
			logger.Info("[Homepage] Bundled referenced assets into deployment", "count", bundled)
		}
	}

	// Inject a _headers file if the project doesn't already have one.
	// When deploying via the Netlify ZIP API, MIME types are sometimes not
	// inferred from file extensions — explicit Content-Type headers fix this.
	if !hasHeaders {
		headersContent := "/*.html\n  Content-Type: text/html; charset=UTF-8\n/*.css\n  Content-Type: text/css; charset=UTF-8\n/*.js\n  Content-Type: application/javascript; charset=UTF-8\n"
		if w, err := zw.Create("_headers"); err == nil {
			_, _ = w.Write([]byte(headersContent))
		}
	}

	// Inject a minimal netlify.toml if the project doesn't already have one.
	if !hasNetlifyToml {
		tomlContent := "[[headers]]\n  for = \"/*.html\"\n  [headers.values]\n    Content-Type = \"text/html; charset=UTF-8\"\n\n[[headers]]\n  for = \"/*.css\"\n  [headers.values]\n    Content-Type = \"text/css; charset=UTF-8\"\n\n[[headers]]\n  for = \"/*.js\"\n  [headers.values]\n    Content-Type = \"application/javascript; charset=UTF-8\"\n\n[[redirects]]\n  from = \"/*\"\n  to = \"/index.html\"\n  status = 200\n"
		if w, err := zw.Create("netlify.toml"); err == nil {
			_, _ = w.Write([]byte(tomlContent))
		}
	}

	// Inject a _redirects file for SPA routing if the project doesn't have one.
	// This ensures single-page apps (React, Next.js static, Vue, etc.) serve
	// index.html for all routes instead of returning Netlify's default 404.
	if !hasRedirects {
		if w, err := zw.Create("_redirects"); err == nil {
			_, _ = w.Write([]byte("/*    /index.html   200\n"))
		}
	}

	if err := zw.Close(); err != nil {
		return errJSON("Failed to finalise ZIP: %v", err)
	}

	zipBytes := zipBuf.Bytes()
	if len(zipBytes) == 0 {
		return errJSON("ZIP is empty — check that %q contains files", deployPath)
	}

	// If siteID is a human-readable name (not a UUID), resolve it to a UUID first.
	// This avoids the Netlify API 404 that occurs when a name is passed where a UUID is expected.
	resolvedID := siteID
	if !looksLikeUUID(siteID) && siteID != "" {
		logger.Info("[Homepage] Resolving site name to UUID", "name", siteID)
		if uuid := netlifyResolveNameToID(nfCfg, siteID); uuid != "" {
			logger.Info("[Homepage] Site resolved", "name", siteID, "uuid", uuid)
			resolvedID = uuid
		}
		// If uuid == "", site doesn't exist yet — auto-create below after deploy attempt.
	}

	logger.Info("[Homepage] Deploying to Netlify", "site_id", resolvedID, "bytes", len(zipBytes), "draft", draft)
	deployResult := NetlifyDeployZip(nfCfg, resolvedID, title, draft, zipBytes)

	// If Netlify returned 404, the site doesn't exist yet — auto-create and retry.
	// Only do this when siteID was a name (not a UUID), to avoid recreating a deleted site by UUID.
	var dr map[string]interface{}
	if json.Unmarshal([]byte(deployResult), &dr) == nil {
		if code, _ := dr["http_code"].(float64); code == 404 && !looksLikeUUID(siteID) {
			logger.Info("[Homepage] Site not found, auto-creating", "name", siteID)
			createResult := NetlifyCreateSite(nfCfg, siteID, "")
			var cr map[string]interface{}
			if json.Unmarshal([]byte(createResult), &cr) == nil && cr["status"] == "ok" {
				newID, _ := cr["id"].(string)
				newDomain, _ := cr["default_domain"].(string)
				if newID != "" {
					logger.Info("[Homepage] Site created, retrying deploy", "site_id", newID, "domain", newDomain)
					deployResult = NetlifyDeployZip(nfCfg, newID, title, draft, zipBytes)
					// Annotate success with the auto-created site info
					var rr map[string]interface{}
					if json.Unmarshal([]byte(deployResult), &rr) == nil {
						rr["auto_created_site"] = true
						rr["new_site_id"] = newID
						rr["new_site_domain"] = newDomain
						if b, merr := json.Marshal(rr); merr == nil {
							deployResult = string(b)
						}
					}
				}
			} else {
				// Return create error to the agent
				return errJSON("Site %q not found and auto-creation failed: %s", siteID, createResult)
			}
		}
	}
	return deployResult
}

// HomepagePublishToLocal rebuilds and refreshes the local web server.
// For plain HTML projects (no build script), the build step is skipped and
// the project directory is served directly.
func HomepagePublishToLocal(cfg HomepageConfig, projectDir string, logger *slog.Logger) string {
	// For Next.js projects ensure the config has output:'export' before building.
	// Without it, `next build` produces a .next/ server bundle that Caddy cannot serve.
	// This mirrors the same logic used by HomepageDeployNetlify.
	if cfg.WorkspacePath != "" {
		projectRoot := filepath.Join(cfg.WorkspacePath, projectDir)
		if isNextJsProject(projectRoot) {
			if patched := ensureNextJsStaticExport(projectRoot, logger); patched {
				logger.Info("[Homepage] publish_local: Next.js config patched for static export")
			}
			// Warn if a stale root index.html exists alongside a Next.js project —
			// Caddy would serve it instead of the proper build output if buildDir ends up as ".".
			rootIndex := filepath.Join(projectRoot, "index.html")
			if _, err := os.Stat(rootIndex); err == nil {
				logger.Warn("[Homepage] publish_local: stale index.html found in project root alongside Next.js project — it will be shadowed by the static export", "path", rootIndex)
			}
		}
	}

	// Try build; ignore failure for plain-HTML projects that have no build script.
	buildResult := HomepageBuild(cfg, projectDir, logger)
	var br map[string]interface{}
	buildSucceeded := false
	if json.Unmarshal([]byte(buildResult), &br) == nil {
		if s, _ := br["status"].(string); s == "ok" {
			buildSucceeded = true
		} else {
			logger.Info("[Homepage] Build failed or not applicable, serving project root directly", "result", buildResult)
		}
	}

	var buildDir string
	if buildSucceeded {
		// Build produced an output directory — auto-detect it.
		buildDir = detectBuildDir(cfg, projectDir)

		// If we still ended up with .next/ (Next.js server build, output:'export' patch
		// may not have taken effect yet), patch and rebuild once more.
		if buildDir == ".next" && cfg.WorkspacePath != "" {
			projectRoot := filepath.Join(cfg.WorkspacePath, projectDir)
			logger.Info("[Homepage] publish_local: .next server build detected — patching Next.js config and rebuilding")
			ensureNextJsStaticExport(projectRoot, logger)
			rebuildResult := HomepageBuild(cfg, projectDir, logger)
			var rb map[string]interface{}
			if json.Unmarshal([]byte(rebuildResult), &rb) == nil {
				if s, _ := rb["status"].(string); s == "ok" {
					buildDir = detectBuildDir(cfg, projectDir)
				}
			}
			// If still .next after second build, fall back to project root so the
			// agent gets a visible error from Caddy rather than a silent wrong serve.
			if buildDir == ".next" {
				logger.Warn("[Homepage] publish_local: still .next after rebuild — falling back to project root")
				buildDir = "."
			}
		}
	} else {
		// No build script (plain-HTML project). Always serve from the project
		// root so that files written via write_file are immediately visible.
		// Skipping detectBuildDir avoids accidentally serving a stale sub-directory
		// (e.g. an old dist/ created by a previous manual copy).
		buildDir = "."
	}

	// Start/restart web server with detected (or project root) directory
	return HomepageWebServerStart(cfg, projectDir, buildDir, logger)
}

// ─── Internal Helpers ─────────────────────────────────────────────────────

var generatedImageRefRegex = regexp.MustCompile(`/files/generated_images/([^"' ><\\]+)`)
var audioFileRefRegex = regexp.MustCompile(`/files/audio/([^"' ><\\]+)`)
var documentFileRefRegex = regexp.MustCompile(`/files/documents/([^"' ><\\]+)`)

type assetRef struct {
	subdir string
	name   string
}

// copyAssetsToBuildDir scans HTML/CSS/JS files in the build directory for
// /files/<subdir>/<name> references (generated_images, audio, documents) and
// copies the actual files from the AuraGo data directory into the build
// directory so the Caddy/Python web server can serve them as static assets.
func copyAssetsToBuildDir(buildPath, dataDir string, logger *slog.Logger) {
	if dataDir == "" {
		return
	}

	refs := make(map[assetRef]struct{})
	regexes := []struct {
		re     *regexp.Regexp
		subdir string
	}{
		{generatedImageRefRegex, "generated_images"},
		{audioFileRefRegex, "audio"},
		{documentFileRefRegex, "documents"},
	}

	filepath.Walk(buildPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		lowerName := strings.ToLower(info.Name())
		if !strings.HasSuffix(lowerName, ".html") && !strings.HasSuffix(lowerName, ".htm") &&
			!strings.HasSuffix(lowerName, ".css") && !strings.HasSuffix(lowerName, ".js") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		for _, r := range regexes {
			for _, m := range r.re.FindAllSubmatch(data, -1) {
				if len(m) > 1 {
					refs[assetRef{subdir: r.subdir, name: string(m[1])}] = struct{}{}
				}
			}
		}
		return nil
	})

	if len(refs) == 0 {
		return
	}

	copied := 0
	for ref := range refs {
		srcPath := filepath.Join(dataDir, ref.subdir, filepath.Base(ref.name))
		srcData, readErr := os.ReadFile(srcPath)
		if readErr != nil {
			logger.Warn("[Homepage] Could not copy asset for local serving", "subdir", ref.subdir, "file", ref.name, "error", readErr)
			continue
		}
		targetDir := filepath.Join(buildPath, "files", ref.subdir)
		if mkdirErr := os.MkdirAll(targetDir, 0755); mkdirErr != nil {
			logger.Warn("[Homepage] Could not create target directory for assets", "dir", targetDir, "error", mkdirErr)
			return
		}
		dstPath := filepath.Join(targetDir, filepath.Base(ref.name))
		if writeErr := os.WriteFile(dstPath, srcData, 0644); writeErr != nil {
			logger.Warn("[Homepage] Could not write asset to build dir", "file", ref.name, "error", writeErr)
			continue
		}
		copied++
	}
	if copied > 0 {
		logger.Info("[Homepage] Copied referenced assets to build directory for local serving", "count", copied)
	}
}

func homepageBuildImage(dockerCfg DockerConfig) string {
	// Build the image via the Docker Engine HTTP API.
	// POST /build with a tar archive containing the Dockerfile — no docker CLI needed.
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	contents := []byte(homepageDockerfile)
	if err := tw.WriteHeader(&tar.Header{
		Name: "Dockerfile",
		Mode: 0644,
		Size: int64(len(contents)),
	}); err != nil {
		return errJSON("Failed to write tar header: %v", err)
	}
	if _, err := tw.Write(contents); err != nil {
		return errJSON("Failed to write Dockerfile to tar: %v", err)
	}
	_ = tw.Close()

	client := getDockerClient(dockerCfg)
	reqURL := "http://localhost/v1.45/build?t=" + homepageImageName
	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost, reqURL, &buf)
	if err != nil {
		return errJSON("Failed to create build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-tar")

	// Build can take several minutes (pulling base image + npm install)
	buildClient := &http.Client{Transport: client.Transport, Timeout: 15 * time.Minute}
	resp, err := buildClient.Do(req)
	if err != nil {
		return errJSON("Image build request failed: %v", err)
	}
	defer resp.Body.Close()

	// Docker streams JSON progress lines; drain and check final line for errors.
	var lastLine string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			lastLine = line
		}
	}

	var streamErr struct {
		Error string `json:"error"`
	}
	if json.Unmarshal([]byte(lastLine), &streamErr) == nil && streamErr.Error != "" {
		return errJSON("Image build error: %s", streamErr.Error)
	}
	if resp.StatusCode != http.StatusOK {
		return errJSON("Image build failed with status %d", resp.StatusCode)
	}

	res, _ := json.Marshal(map[string]interface{}{"status": "ok", "output": "Image built successfully"})
	return string(res)
}

func containerStatus(dockerCfg DockerConfig, name string) string {
	data, code, err := dockerRequest(dockerCfg, "GET", "/containers/"+name+"/json", "")
	if err != nil || code != 200 {
		return `{"running":false,"exists":false}`
	}
	var info map[string]interface{}
	if err := json.Unmarshal(data, &info); err != nil {
		return `{"running":false,"exists":false}`
	}
	state, _ := info["State"].(map[string]interface{})
	running, _ := state["Running"].(bool)
	status, _ := state["Status"].(string)
	result := map[string]interface{}{
		"exists":  true,
		"running": running,
		"status":  status,
	}
	out, _ := json.Marshal(result)
	return string(out)
}

// looksLikeUUID returns true if s matches the standard 8-4-4-4-12 UUID format.
// Used to distinguish Netlify site UUIDs from human-readable site names.
func looksLikeUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}

func detectBuildDir(cfg HomepageConfig, projectDir string) string {
	if !checkDockerAvailable(cfg.DockerHost) {
		if cfg.WorkspacePath != "" {
			for _, dir := range []string{"out", "dist", "build", ".next", "public"} {
				p := filepath.Join(cfg.WorkspacePath, projectDir, dir)
				if s, err := os.Stat(p); err == nil && s.IsDir() {
					return dir
				}
			}
		}
		// No standard build dir found — serve project root directly.
		// This handles plain HTML projects that have no build step.
		return "."
	}
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	// Try common build output directories
	for _, dir := range []string{"out", "dist", "build", ".next", "public"} {
		result := DockerExec(dockerCfg, homepageContainerName, fmt.Sprintf("test -d /workspace/%s/%s && echo yes", projectDir, dir), "")
		if strings.Contains(result, "yes") {
			return dir
		}
	}
	// No standard build dir found — serve project root directly.
	return "."
}

func homepageCaddyfile(domain string, port int) string {
	if domain != "" {
		return fmt.Sprintf(`%s {
    root * /srv
    file_server
    encode gzip
    try_files {path} /index.html
}
`, domain)
	}
	// The container always binds host port → container port 80.
	// Caddy must therefore listen on :80 regardless of the host-side port.
	return `:80 {
    root * /srv
    file_server
    encode gzip
    try_files {path} /index.html
}
`
}

func okJSON(message string, kvPairs ...string) string {
	result := map[string]interface{}{
		"status":  "ok",
		"message": message,
	}
	for i := 0; i+1 < len(kvPairs); i += 2 {
		result[kvPairs[i]] = kvPairs[i+1]
	}
	out, _ := json.Marshal(result)
	return string(out)
}

// sftpUploadDir recursively uploads a local directory to a remote path via SFTP.
func sftpUploadDir(ctx context.Context, deployCfg HomepageDeployConfig, localDir, remoteDir string, secret []byte, logger *slog.Logger) string {
	sshCfg, err := remote.GetSSHConfig(deployCfg.User, secret)
	if err != nil {
		return errJSON("SSH config failed: %v", err)
	}

	addr := fmt.Sprintf("%s:%d", deployCfg.Host, deployCfg.Port)
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return errJSON("Failed to connect: %v", err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, sshCfg)
	if err != nil {
		conn.Close()
		return errJSON("SSH handshake failed: %v", err)
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return errJSON("SFTP client failed: %v", err)
	}
	defer sftpClient.Close()

	uploaded := 0
	err = filepath.Walk(localDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		relPath, _ := filepath.Rel(localDir, path)
		remotePath := filepath.ToSlash(filepath.Join(remoteDir, relPath))

		if info.IsDir() {
			sftpClient.MkdirAll(remotePath)
			return nil
		}

		localFile, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", path, err)
		}

		remoteFile, err := sftpClient.Create(remotePath)
		if err != nil {
			localFile.Close()
			return fmt.Errorf("failed to create remote %s: %w", remotePath, err)
		}

		_, copyErr := remoteFile.ReadFrom(localFile)
		remoteFile.Close()
		localFile.Close()
		if copyErr != nil {
			return fmt.Errorf("failed to upload %s: %w", relPath, copyErr)
		}
		uploaded++
		return nil
	})

	if err != nil {
		return errJSON("Upload failed after %d files: %v", uploaded, err)
	}

	logger.Info("[Homepage] Deploy complete", "files", uploaded, "host", deployCfg.Host)
	return "" // success — caller handles the ok response
}

// isNextJsProject returns true if the directory contains a Next.js config file.
func isNextJsProject(projectRoot string) bool {
	for _, name := range []string{"next.config.js", "next.config.ts", "next.config.mjs", "next.config.cjs"} {
		if _, err := os.Stat(filepath.Join(projectRoot, name)); err == nil {
			return true
		}
	}
	return false
}

// ensureNextJsStaticExport ensures the Next.js project config has output: 'export' so that
// `next build` produces a static site in the `out/` directory (required for Netlify and other
// static hosts). Returns true if the config was created or modified (a rebuild is needed).
func ensureNextJsStaticExport(projectRoot string, logger *slog.Logger) bool {
	// Check each known config filename
	configNames := []string{"next.config.js", "next.config.ts", "next.config.mjs", "next.config.cjs"}
	var configPath string
	for _, name := range configNames {
		p := filepath.Join(projectRoot, name)
		if _, err := os.Stat(p); err == nil {
			configPath = p
			break
		}
	}

	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			logger.Warn("[Homepage] Could not read Next.js config", "path", configPath, "error", err)
		} else {
			content := string(data)
			// Already has an output setting — don't touch it.
			if strings.Contains(content, "output") {
				return false
			}
			// Try to inject output: 'export' into the existing config object.
			if patched := nextJsInjectOutputExport(content); patched != "" {
				if werr := os.WriteFile(configPath, []byte(patched), 0644); werr == nil {
					logger.Info("[Homepage] Injected output: 'export' into Next.js config", "file", configPath)
					return true
				}
			}
		}
		// Fallback: overwrite with a minimal config that preserves the filename.
	}

	// No config found, or injection failed — write a minimal next.config.js.
	if configPath == "" {
		configPath = filepath.Join(projectRoot, "next.config.js")
	}
	minimal := "/** @type {import('next').NextConfig} */\nconst nextConfig = {\n  output: 'export',\n};\n\nmodule.exports = nextConfig;\n"
	if strings.HasSuffix(configPath, ".ts") || strings.HasSuffix(configPath, ".mjs") {
		minimal = "import type { NextConfig } from 'next';\n\nconst nextConfig: NextConfig = {\n  output: 'export',\n};\n\nexport default nextConfig;\n"
	}
	if werr := os.WriteFile(configPath, []byte(minimal), 0644); werr == nil {
		logger.Info("[Homepage] Wrote minimal Next.js config with output: 'export'", "file", configPath)
		return true
	}
	logger.Warn("[Homepage] Could not write Next.js config", "file", configPath)
	return false
}

// nextJsInjectOutputExport tries to inject `output: 'export',` into a Next.js config string
// by finding the opening brace of the config object. Returns the patched string, or "" on failure.
func nextJsInjectOutputExport(content string) string {
	// Patterns that precede the config object opening brace:
	patterns := []string{
		"nextConfig = {",
		"nextConfig: NextConfig = {",
		"module.exports = {",
		"export default {",
		"const config = {",
	}
	for _, p := range patterns {
		if idx := strings.Index(content, p); idx != -1 {
			insertAt := idx + len(p)
			return content[:insertAt] + "\n  output: 'export'," + content[insertAt:]
		}
	}
	return ""
}
