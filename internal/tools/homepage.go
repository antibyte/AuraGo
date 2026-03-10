package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/remote"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// HomepageConfig holds the configuration for the homepage dev environment.
type HomepageConfig struct {
	DockerHost      string
	WorkspacePath   string // host path mounted as /workspace in the container
	WebServerPort   int
	WebServerDomain string
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

// ─── Container Lifecycle ──────────────────────────────────────────────────

// HomepageInit builds the image (if needed) and creates the dev container.
func HomepageInit(cfg HomepageConfig, logger *slog.Logger) string {
	dockerCfg := DockerConfig{Host: cfg.DockerHost}

	// Ensure workspace dir exists on the host
	if cfg.WorkspacePath != "" {
		if err := os.MkdirAll(cfg.WorkspacePath, 0755); err != nil {
			return errJSON("Failed to create workspace directory: %v", err)
		}
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

	// Dev container status
	devStatus := containerStatus(dockerCfg, homepageContainerName)
	result["dev_container"] = devStatus

	// Web container status
	webStatus := containerStatus(dockerCfg, homepageWebContainer)
	result["web_container"] = webStatus

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
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	return DockerExec(dockerCfg, homepageContainerName, "cd /workspace && "+cmd, "")
}

// HomepageBuild runs the build command in the project directory.
func HomepageBuild(cfg HomepageConfig, projectDir string, logger *slog.Logger) string {
	if projectDir == "" {
		projectDir = "."
	}
	logger.Info("[Homepage] Build", "dir", projectDir)
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
	logger.Info("[Homepage] Install deps", "packages", packages)
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
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
	// Prevent path traversal outside /workspace
	if strings.Contains(path, "..") {
		return errJSON("Path traversal not allowed")
	}
	logger.Info("[Homepage] ListFiles", "path", path)
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
	// Use base64 to safely pass content through shell
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	logger.Info("[Homepage] WriteFile", "path", path, "size", len(content))
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	cmd := fmt.Sprintf("mkdir -p /workspace/%s && echo '%s' | base64 -d > /workspace/%s", dir, encoded, path)
	return DockerExec(dockerCfg, homepageContainerName, cmd, "")
}

// HomepageOptimizeImages runs SVG optimization on the project.
func HomepageOptimizeImages(cfg HomepageConfig, projectDir string, logger *slog.Logger) string {
	if projectDir == "" {
		projectDir = "."
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
				return errJSON("Build failed before deploy: %s", buildResult)
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
	output, err := remote.ExecuteRemoteCommand(ctx, deployCfg.Host, deployCfg.Port, deployCfg.User, secret, "echo ok && ls -la "+deployCfg.Path+" 2>&1 | head -5")
	if err != nil {
		return errJSON("Connection failed: %v", err)
	}

	return okJSON("Connection successful", "output", strings.TrimSpace(output))
}

// ─── Caddy Web Server ─────────────────────────────────────────────────────

// HomepageWebServerStart starts the Caddy container serving the build output.
func HomepageWebServerStart(cfg HomepageConfig, projectDir, buildDir string, logger *slog.Logger) string {
	dockerCfg := DockerConfig{Host: cfg.DockerHost}

	if buildDir == "" {
		buildDir = detectBuildDir(cfg, projectDir)
	}

	hostBuildPath := filepath.Join(cfg.WorkspacePath, projectDir, buildDir)
	port := cfg.WebServerPort
	if port == 0 {
		port = 8080
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

	// Create Caddy container
	portKey := fmt.Sprintf("%d/tcp", port)
	exposedPorts := map[string]interface{}{
		portKey: map[string]interface{}{},
	}
	portBindings := map[string]interface{}{
		portKey: []map[string]interface{}{
			{"HostPort": fmt.Sprintf("%d", port)},
		},
	}
	if cfg.WebServerDomain != "" {
		// With a domain Caddy handles HTTP→HTTPS redirects and TLS; expose 80 + 443.
		exposedPorts["80/tcp"] = map[string]interface{}{}
		exposedPorts["443/tcp"] = map[string]interface{}{}
		portBindings["80/tcp"] = []map[string]interface{}{{"HostPort": "80"}}
		portBindings["443/tcp"] = []map[string]interface{}{{"HostPort": "443"}}
	}
	payload := map[string]interface{}{
		"Image":        homepageWebImage,
		"ExposedPorts": exposedPorts,
		"HostConfig": map[string]interface{}{
			"Binds": []string{
				hostBuildPath + ":/srv",
				caddyfilePath + ":/etc/caddy/Caddyfile",
			},
			"PortBindings": portBindings,
			"RestartPolicy": map[string]string{"Name": "unless-stopped"},
		},
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
	return okJSON("Web server started", "url", url, "port", fmt.Sprintf("%d", port))
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
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	return containerStatus(dockerCfg, homepageWebContainer)
}

// HomepagePublishToLocal rebuilds and refreshes the local web server.
func HomepagePublishToLocal(cfg HomepageConfig, projectDir string, logger *slog.Logger) string {
	// Build first
	buildResult := HomepageBuild(cfg, projectDir, logger)
	var br map[string]interface{}
	if json.Unmarshal([]byte(buildResult), &br) == nil {
		if s, _ := br["status"].(string); s == "error" {
			return errJSON("Build failed: %s", buildResult)
		}
	}

	buildDir := detectBuildDir(cfg, projectDir)

	// Start/restart web server with new build
	return HomepageWebServerStart(cfg, projectDir, buildDir, logger)
}

// ─── Internal Helpers ─────────────────────────────────────────────────────

func homepageBuildImage(dockerCfg DockerConfig) string {
	// Write Dockerfile to a temp directory, create tar, and POST /build
	tmpDir, err := os.MkdirTemp("", "aurago-homepage-build-")
	if err != nil {
		return errJSON("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(homepageDockerfile), 0644); err != nil {
		return errJSON("Failed to write Dockerfile: %v", err)
	}

	// Use docker CLI to build (more reliable for context handling)
	args := []string{"build", "-t", homepageImageName, "-f", dockerfilePath, tmpDir}
	return runDockerCLIHelper(dockerCfg, args...)
}

func containerStatus(dockerCfg DockerConfig, name string) string {
	data, code, err := dockerRequest(dockerCfg, "GET", "/containers/"+name+"/json", "")
	if err != nil || code != 200 {
		return `{"running":false,"exists":false}`
	}
	var info map[string]interface{}
	json.Unmarshal(data, &info)
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

func detectBuildDir(cfg HomepageConfig, projectDir string) string {
	dockerCfg := DockerConfig{Host: cfg.DockerHost}
	// Try common build output directories
	for _, dir := range []string{"out", "dist", "build", ".next", "public"} {
		result := DockerExec(dockerCfg, homepageContainerName, fmt.Sprintf("test -d /workspace/%s/%s && echo yes", projectDir, dir), "")
		if strings.Contains(result, "yes") {
			return dir
		}
	}
	return "out" // default fallback
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
	return fmt.Sprintf(`:%d {
    root * /srv
    file_server
    encode gzip
    try_files {path} /index.html
}
`, port)
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
		defer localFile.Close()

		remoteFile, err := sftpClient.Create(remotePath)
		if err != nil {
			return fmt.Errorf("failed to create remote %s: %w", remotePath, err)
		}
		defer remoteFile.Close()

		if _, err := remoteFile.ReadFrom(localFile); err != nil {
			return fmt.Errorf("failed to upload %s: %w", relPath, err)
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
