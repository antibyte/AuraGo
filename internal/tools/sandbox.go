package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed sandbox_mcp_launch.py
var sandboxMCPLaunchSource string

type sandboxConn interface {
	initialize(ctx context.Context, logger *slog.Logger) error
	discoverTools(ctx context.Context, logger *slog.Logger) error
	callTool(ctx context.Context, toolName string, arguments map[string]interface{}) (string, error)
	close()
	markReady()
	toolCount() int
}

type sandboxConnFactory func() (sandboxConn, error)

// A transport can be retired concurrently by cancellation and ordinary release.
// The manager owns exactly one close, independently of transport implementations.
type ownedSandboxConn struct {
	sandboxConn
	closeOnce sync.Once
}

func (c *ownedSandboxConn) close() { c.closeOnce.Do(c.sandboxConn.close) }

// ── LLM-Sandbox Integration ────────────────────────────────────────────────
//
// Launches the llm-sandbox MCP server through an embedded Python launcher
// that applies IMAGE before the upstream server starts. The child process
// uses the JSON-RPC 2.0 stdio transport (reusing the mcpConn type from mcp.go).
//
// Environment matrix:
//   - Docker available  → sandbox runs code in isolated containers (default)
//   - Docker absent     → sandbox unavailable; execution remains fail-closed
//   - Podman available  → BACKEND=podman
//
// The SandboxManager is a separate singleton from MCPManager so that user-managed
// MCP servers and the internal sandbox do not interfere with each other.

// SandboxStatus reports the sandbox readiness state.
type SandboxStatus struct {
	Ready            bool     `json:"ready"`
	Backend          string   `json:"backend"`
	DockerAvailable  bool     `json:"docker_available"`
	PythonAvailable  bool     `json:"python_available"`
	PackageInstalled bool     `json:"package_installed"`
	Languages        []string `json:"languages,omitempty"`
	Error            string   `json:"error,omitempty"`
}

// SandboxManager manages the llm-sandbox MCP server process.
type SandboxManager struct {
	mu           sync.RWMutex
	sharedConnMu sync.Mutex
	conn         sandboxConn
	logger       *slog.Logger
	status       SandboxStatus
	cfg          SandboxConfig
	workspaceDir string
	pythonBin    string
	env          map[string]string
	launchScript string
	connFactory  sandboxConnFactory
	// transportGrace lets the MCP server tear down its container after the
	// execution deadline. The execution timeout itself is passed separately.
	transportGrace time.Duration
}

const sandboxMCPTransportGrace = 15 * time.Second

var (
	globalSandboxMgr *SandboxManager
	sandboxMgrMu     sync.RWMutex
)

// ErrSandboxUnavailable marks a missing, not-ready, or failed-to-start sandbox.
// Timeouts and code-execution errors stay outside this sentinel.
var ErrSandboxUnavailable = errors.New("sandbox unavailable")

// InitSandboxManager creates and starts the sandbox MCP server.
// Call once at startup. On failure the manager is still stored but status.Ready == false,
// so execution reports the setup failure without a host fallback.
func InitSandboxManager(cfg SandboxConfig, workspaceDir string, logger *slog.Logger) *SandboxManager {
	mgr := &SandboxManager{
		logger:         logger,
		status:         SandboxStatus{Backend: cfg.Backend},
		cfg:            cfg,
		workspaceDir:   workspaceDir,
		transportGrace: sandboxMCPTransportGrace,
	}
	runtimeDir, err := sandboxSupervisorRuntimeDir(cfg.RuntimeDir, workspaceDir)
	if err != nil {
		mgr.status.Error = err.Error()
		registerSandboxSingleton(mgr)
		return mgr
	}
	mgr.workspaceDir = runtimeDir

	// Warn about config options that are parsed but not yet implemented, so users
	// are not misled into thinking they have container pooling active.
	if cfg.PoolSize > 0 {
		logger.Warn("[Sandbox] sandbox.pool_size is set but container pooling is not yet implemented — value ignored", "pool_size", cfg.PoolSize)
	}

	// 1. Check Docker / Podman availability
	if cfg.Backend == "podman" {
		if _, err := exec.LookPath("podman"); err != nil {
			mgr.status.Error = "podman not found in PATH"
			logger.Warn("[Sandbox] Podman not found — sandbox disabled", "error", err)
			registerSandboxSingleton(mgr)
			return mgr
		}
		mgr.status.DockerAvailable = true
	} else {
		host := cfg.DockerHost
		if err := DockerPing(host); err != nil {
			mgr.status.Error = err.Error()
			logger.Warn("[Sandbox] Docker not available — sandbox disabled", "error", err)
			registerSandboxSingleton(mgr)
			return mgr
		}
		mgr.status.DockerAvailable = true
	}

	// 2. Check Python availability
	pythonBin := GetPythonBin(runtimeDir)
	if _, err := exec.LookPath(pythonBin); err != nil && !cfg.AutoInstall {
		mgr.status.Error = "Private supervisor Python is not provisioned; install the pinned sandbox package in sandbox-supervisor/venv under data_dir"
		registerSandboxSingleton(mgr)
		return mgr
	}
	mgr.pythonBin = pythonBin

	// 3. Auto-install llm-sandbox if configured
	if cfg.AutoInstall {
		if !isSandboxMCPInstalled(pythonBin) {
			logger.Info("[Sandbox] Installing llm-sandbox MCP package...", "backend", cfg.Backend)
			if err := installSandboxPackage(runtimeDir, workspaceDir, cfg.Backend, logger); err != nil {
				mgr.status.Error = fmt.Sprintf("Failed to install llm-sandbox: %v", err)
				logger.Error("[Sandbox] Package installation failed", "error", err)
				registerSandboxSingleton(mgr)
				return mgr
			}
			// Recheck after install — use the venv python which now has the package
			pythonBin = GetPythonBin(runtimeDir)
			mgr.pythonBin = pythonBin
		}
	}

	// Verify package + MCP deps are present
	if isSandboxMCPInstalled(pythonBin) {
		mgr.status.PythonAvailable = true
		mgr.status.PackageInstalled = true
	} else {
		mgr.status.Error = sandboxNotInstalledStatus(cfg.Backend)
		logger.Warn("[Sandbox] llm-sandbox MCP dependencies not found", "python", pythonBin)
		registerSandboxSingleton(mgr)
		return mgr
	}

	// 4. Build environment variables for the MCP server.
	// A rejected image must leave Ready false and must not start the process.
	env, err := sandboxProcessEnv(cfg)
	if err != nil {
		mgr.status.Error = err.Error()
		logger.Error("[Sandbox] invalid sandbox configuration", "error", err)
		registerSandboxSingleton(mgr)
		return mgr
	}
	mgr.env = env

	scriptPath, err := writeSandboxMCPLauncher()
	if err != nil {
		mgr.status.Error = err.Error()
		logger.Error("[Sandbox] MCP launcher setup failed", "error", err)
		registerSandboxSingleton(mgr)
		return mgr
	}
	mgr.launchScript = scriptPath
	args := []string{"-I", scriptPath}
	mgr.connFactory = func() (sandboxConn, error) {
		ctx := withContainerSandboxLandlock(context.Background(), mgr.workspaceDir, mgr.cfg.DockerHost)
		return newMCPConn("llm-sandbox", mgr.pythonBin, args, mgr.env, logger, "local", "", "", ctx)
	}

	// 5. Validate the MCP server and optionally keep it warm.
	conn, err := mgr.openConn()
	if err != nil {
		mgr.status.Error = err.Error()
		logger.Error("[Sandbox] MCP server start failed", "error", err)
		registerSandboxSingleton(mgr)
		return mgr
	}
	mgr.status.Ready = true
	mgr.status.Languages = mgr.fetchLanguagesFromConn(conn)
	toolCount := conn.toolCount()
	if cfg.KeepAlive {
		mgr.conn = conn
	} else {
		conn.close()
	}

	logger.Info("[Sandbox] Manager initialized",
		"backend", cfg.Backend,
		"keep_alive", cfg.KeepAlive,
		"languages", len(mgr.status.Languages),
		"tools", toolCount)

	registerSandboxSingleton(mgr)
	return mgr
}

func (m *SandboxManager) openConn() (sandboxConn, error) {
	return m.openConnContext(context.Background())
}

func (m *SandboxManager) openConnContext(parent context.Context) (sandboxConn, error) {
	if err := parent.Err(); err != nil {
		return nil, err
	}
	if m.connFactory == nil {
		return nil, fmt.Errorf("sandbox connection factory not configured: %w", ErrSandboxUnavailable)
	}

	conn, err := m.connFactory()
	if err != nil {
		return nil, fmt.Errorf("failed to start sandbox MCP server: %w: %w", err, ErrSandboxUnavailable)
	}
	conn = &ownedSandboxConn{sandboxConn: conn}
	ctx, cancel := context.WithTimeout(parent, mcpCallToolTimeout)
	defer cancel()
	if err := conn.initialize(ctx, m.logger); err != nil {
		conn.close()
		return nil, fmt.Errorf("MCP initialization failed: %w", err)
	}
	if err := conn.discoverTools(ctx, m.logger); err != nil {
		conn.close()
		return nil, fmt.Errorf("tool discovery failed: %w", err)
	}

	conn.markReady()
	return conn, nil
}

// wrapSandboxOpenError keeps ErrSandboxUnavailable on process-start failures.
// Handshake errors after the process starts stay unwrapped.
func wrapSandboxOpenError(err error) error {
	if err == nil || !errors.Is(err, ErrSandboxUnavailable) {
		return err
	}
	return fmt.Errorf("%w", err)
}

func (m *SandboxManager) borrowConn() (sandboxConn, func(), error) {
	return m.borrowConnContext(context.Background())
}

func (m *SandboxManager) borrowConnContext(ctx context.Context) (sandboxConn, func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	m.mu.RLock()
	ready := m.status.Ready
	errMsg := m.status.Error
	keepAlive := m.cfg.KeepAlive
	m.mu.RUnlock()

	if !ready {
		if errMsg == "" {
			errMsg = "sandbox not ready"
		}
		return nil, nil, fmt.Errorf("sandbox not ready: %s: %w", errMsg, ErrSandboxUnavailable)
	}

	if !keepAlive {
		conn, err := m.openConnContext(ctx)
		if err != nil {
			return nil, nil, wrapSandboxOpenError(err)
		}
		return conn, func() { conn.close() }, nil
	}

	for !m.sharedConnMu.TryLock() {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
	cleanup := func() {
		m.sharedConnMu.Unlock()
	}

	m.mu.RLock()
	conn := m.conn
	m.mu.RUnlock()
	if conn != nil {
		return conn, cleanup, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.conn == nil {
		conn, err := m.openConnContext(ctx)
		if err != nil {
			cleanup()
			return nil, nil, wrapSandboxOpenError(err)
		}
		m.conn = conn
	}

	return m.conn, cleanup, nil
}

// markUnhealthy closes the current connection and removes it from the pool.
// This is used when a connection is suspected to be in a bad state (e.g., after timeout).
// Safe to call multiple times.
func (m *SandboxManager) markUnhealthy() {
	m.sharedConnMu.Lock()
	defer m.sharedConnMu.Unlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.conn != nil {
		m.logger.Warn("[Sandbox] Marking connection unhealthy and closing")
		m.conn.close()
		m.conn = nil
	}
}

func registerSandboxSingleton(mgr *SandboxManager) {
	sandboxMgrMu.Lock()
	old := globalSandboxMgr
	globalSandboxMgr = mgr
	sandboxMgrMu.Unlock()
	// Close old manager outside the lock to avoid holding it during MCP shutdown.
	// This prevents Python subprocess orphans when InitSandboxManager is called again
	// (e.g. after a config reload or re-initialization).
	if old != nil && old != mgr {
		old.Close()
	}
}

// GetSandboxManager returns the global SandboxManager singleton.
func GetSandboxManager() *SandboxManager {
	sandboxMgrMu.RLock()
	defer sandboxMgrMu.RUnlock()
	return globalSandboxMgr
}

// SandboxReady reports whether a manager exists and its sandbox is operational.
func SandboxReady() bool {
	mgr := GetSandboxManager()
	return mgr != nil && mgr.IsReady()
}

// ShutdownSandboxManager gracefully stops the sandbox.
func ShutdownSandboxManager() {
	sandboxMgrMu.Lock()
	mgr := globalSandboxMgr
	globalSandboxMgr = nil
	sandboxMgrMu.Unlock()
	if mgr != nil {
		mgr.Close()
	}
}

// Status returns the current sandbox status.
func (m *SandboxManager) Status() SandboxStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// IsReady returns true if the sandbox is operational.
func (m *SandboxManager) IsReady() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status.Ready
}

// ExecuteCode runs code in the sandbox via the MCP execute_code tool.
func (m *SandboxManager) ExecuteCode(code, language string, libraries []string, timeoutSecs int) (string, error) {
	return m.ExecuteCodeContext(context.Background(), code, language, libraries, timeoutSecs)
}

func (m *SandboxManager) ExecuteCodeContext(parent context.Context, code, language string, libraries []string, timeoutSecs int) (string, error) {
	return m.executeCodeContext(parent, code, language, libraries, timeoutSecs, nil)
}

func (m *SandboxManager) executeCodeContext(parent context.Context, code, language string, libraries []string, timeoutSecs int, bridge *PythonExecutionOptions) (string, error) {
	if parent == nil {
		parent = context.Background()
	}
	if err := parent.Err(); err != nil {
		return "", err
	}
	if len(libraries) > 0 {
		return "", fmt.Errorf("sandbox libraries must be provisioned in the trusted image")
	}
	conn, cleanup, err := m.borrowConnContext(parent)
	if err != nil {
		return "", err
	}
	defer cleanup()
	cancelFile, err := os.CreateTemp("", "aurago-sandbox-cancel-*")
	if err != nil {
		return "", fmt.Errorf("create sandbox cancellation channel: %w", err)
	}
	cancelPath := cancelFile.Name()
	if err := cancelFile.Close(); err != nil {
		os.Remove(cancelPath)
		return "", err
	}

	args := map[string]interface{}{
		"code":        code,
		"language":    language,
		"cancel_file": cancelPath,
	}
	if len(libraries) > 0 {
		args["libraries"] = libraries
	}

	// Use a timeout context; fall back to the central sandbox timeout default.
	if timeoutSecs <= 0 {
		timeoutSecs = int(GetSandboxTimeout().Seconds())
	}
	// Keep the MCP server's own execution deadline aligned with AuraGo's limit.
	// The outer transport deadline includes a bounded grace period because the
	// MCP response is sent only after its short-lived container is removed.
	args["timeout"] = timeoutSecs

	ctx, cancel := context.WithTimeout(parent, time.Duration(timeoutSecs)*time.Second+m.transportGrace)
	defer cancel()
	stopMarker := context.AfterFunc(ctx, func() { _ = os.WriteFile(cancelPath, []byte("cancel"), 0o600) })
	defer func() {
		stopMarker()
		if ctx.Err() == nil {
			_ = os.Remove(cancelPath)
			return
		}
		_ = os.WriteFile(cancelPath, []byte("cancel"), 0o600)
		go func() {
			defer os.Remove(cancelPath)
			deadline := time.NewTimer(5 * time.Second)
			defer deadline.Stop()
			poll := time.NewTicker(25 * time.Millisecond)
			defer poll.Stop()
			for {
				select {
				case <-deadline.C:
					return
				case <-poll.C:
					data, _ := os.ReadFile(cancelPath)
					if string(data) == "terminated" {
						return
					}
				}
			}
		}()
	}()

	tool := "execute_supervised_code"
	if bridge != nil {
		tool = "execute_brokered_code"
		delete(args, "language")
		args["broker_url"] = bridge.ToolBridgeURL
		args["broker_token"] = bridge.ToolBridgeToken
		args["allowed_tools"] = bridge.ToolBridgeAllowedTools
		args["call_limit"] = normalizeToolBridgeCallLimit(bridge.ToolBridgeCallLimit)
	}
	out, err := conn.callTool(ctx, tool, args)
	if ctx.Err() != nil {
		// Timeout: close the connection to terminate the blocked goroutine
		// and prevent reuse of a potentially corrupted connection.
		// markUnhealthy is called asynchronously because close() may block
		// waiting for the subprocess to exit.
		m.mu.Lock()
		if m.conn == conn {
			m.conn = nil
		}
		m.mu.Unlock()
		go conn.close()
		if parent.Err() != nil {
			return "", fmt.Errorf("sandbox execution cancelled: %w", parent.Err())
		}
		return "", fmt.Errorf("TIMEOUT: sandbox execution exceeded %ds limit", timeoutSecs)
	}
	return out, err
}

// GetSupportedLanguages calls get_supported_languages on the sandbox.
func (m *SandboxManager) GetSupportedLanguages() ([]string, error) {
	conn, cleanup, err := m.borrowConn()
	if err != nil {
		return nil, err
	}
	defer cleanup()

	out, err := conn.callTool(context.Background(), "get_supported_languages", map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	// Try to parse as JSON array or newline-separated list
	var langs []string
	if json.Unmarshal([]byte(out), &langs) == nil {
		return langs, nil
	}
	// Fallback: split on newlines/commas
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			langs = append(langs, line)
		}
	}
	return langs, nil
}

// Close shuts down the sandbox MCP server process.
func (m *SandboxManager) Close() {
	m.sharedConnMu.Lock()
	defer m.sharedConnMu.Unlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.conn != nil {
		m.logger.Info("[Sandbox] Shutting down MCP server")
		m.conn.close()
		m.conn = nil
	}
	if m.launchScript != "" {
		if err := os.Remove(m.launchScript); err != nil && !os.IsNotExist(err) {
			m.logger.Warn("[Sandbox] could not remove MCP launcher", "error", err, "path", m.launchScript)
		}
		m.launchScript = ""
	}
	m.status.Ready = false
}

// fetchLanguages calls get_supported_languages and returns the result silently.
func (m *SandboxManager) fetchLanguages() []string {
	langs, err := m.GetSupportedLanguages()
	if err != nil {
		m.logger.Debug("[Sandbox] Could not fetch languages", "error", err)
		return []string{"python"} // safe default
	}
	if len(langs) == 0 {
		return []string{"python"}
	}
	return langs
}

func (m *SandboxManager) fetchLanguagesFromConn(conn sandboxConn) []string {
	out, err := conn.callTool(context.Background(), "get_supported_languages", map[string]interface{}{})
	if err != nil {
		m.logger.Debug("[Sandbox] Could not fetch languages", "error", err)
		return []string{"python"}
	}

	var langs []string
	if json.Unmarshal([]byte(out), &langs) == nil && len(langs) > 0 {
		return langs
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			langs = append(langs, line)
		}
	}
	if len(langs) == 0 {
		return []string{"python"}
	}
	return langs
}

// ── Package-level shortcuts ─────────────────────────────────────────────────

// SandboxExecuteCode is a package-level shorthand for agent dispatch.
func SandboxExecuteCode(code, language string, libraries []string, timeoutSecs int, logger *slog.Logger) (string, error) {
	return SandboxExecuteCodeContext(context.Background(), code, language, libraries, timeoutSecs, logger)
}

func SandboxExecuteCodeContext(ctx context.Context, code, language string, libraries []string, timeoutSecs int, logger *slog.Logger) (string, error) {
	mgr := GetSandboxManager()
	if mgr == nil {
		return "", fmt.Errorf("sandbox manager not initialized: %w", ErrSandboxUnavailable)
	}
	return mgr.ExecuteCodeContext(ctx, code, language, libraries, timeoutSecs)
}

// SandboxGetStatus is a package-level shorthand for the status API.
func SandboxGetStatus() *SandboxStatus {
	mgr := GetSandboxManager()
	if mgr == nil {
		return &SandboxStatus{Error: "sandbox manager not initialized"}
	}
	s := mgr.Status()
	return &s
}

// ── Helpers ─────────────────────────────────────────────────────────────────

const llmSandboxVersion = "0.3.45"

var sandboxImagePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,200}@sha256:[a-f0-9]{64}$`)

const defaultSandboxImage = "python:3.13-slim-bookworm@sha256:a1165e272e578941b84abc79e4ab38a0305cd12803a5c4247979ac7655f4d641"

func llmSandboxRequirement(backend string) (string, error) {
	switch backend {
	case "", "docker":
		return "llm-sandbox[mcp-docker]==" + llmSandboxVersion, nil
	case "podman":
		return "llm-sandbox[mcp-podman]==" + llmSandboxVersion, nil
	default:
		return "", fmt.Errorf("unsupported sandbox backend %q", backend)
	}
}

func sandboxMCPEnv(cfg SandboxConfig) (map[string]string, error) {
	backend := cfg.Backend
	if backend == "" {
		backend = "docker"
	}
	if backend != "docker" && backend != "podman" {
		return nil, fmt.Errorf("unsupported sandbox backend %q", backend)
	}
	// Generic execution never inherits the host/control network. Network access
	// uses the scoped Tool Bridge broker rather than a broad Docker bridge.
	mode := "none"
	env := map[string]string{
		"BACKEND":              backend,
		"SANDBOX_NETWORK_MODE": mode,
	}
	memory, cpus, pids, output := cfg.MemoryMB, cfg.CPUs, cfg.PIDsLimit, cfg.MaxOutputBytes
	if memory <= 0 {
		memory = 512
	}
	if cpus <= 0 {
		cpus = 1
	}
	if pids <= 0 {
		pids = 64
	}
	if output <= 0 {
		output = 1048576
	}
	env["SANDBOX_MEM_LIMIT"] = strconv.Itoa(memory) + "m"
	env["SANDBOX_CPUS"] = strconv.FormatFloat(cpus, 'f', -1, 64)
	env["AURAGO_SANDBOX_PIDS"] = strconv.Itoa(pids)
	env["AURAGO_SANDBOX_OUTPUT"] = strconv.Itoa(output)
	env["COMMIT_CONTAINER"] = "false"
	env["KEEP_TEMPLATE"] = "true"
	images := map[string]string{"python": defaultSandboxImage}
	for lang, image := range cfg.Images {
		if !sandboxImagePattern.MatchString(image) {
			return nil, fmt.Errorf("sandbox %s image requires a digest-pinned reference", lang)
		}
		images[lang] = image
	}
	encodedImages, _ := json.Marshal(images)
	env["AURAGO_SANDBOX_IMAGES"] = string(encodedImages)
	if cfg.DockerHost != "" {
		env["DOCKER_HOST"] = cfg.DockerHost
	}
	if image := strings.TrimSpace(cfg.Image); image != "" {
		if !sandboxImagePattern.MatchString(image) {
			return nil, fmt.Errorf("sandbox image %q is not a single image reference", cfg.Image)
		}
		env["IMAGE"] = image
	}
	return env, nil
}

// sandboxProcessEnv is the MCP child environment. An omitted IMAGE becomes an
// explicit empty value so the process overrides an inherited IMAGE. Go keeps
// the last value when a key appears twice.
func sandboxProcessEnv(cfg SandboxConfig) (map[string]string, error) {
	env, err := sandboxMCPEnv(cfg)
	if err != nil {
		return nil, err
	}
	if _, ok := env["IMAGE"]; !ok {
		env["IMAGE"] = ""
	}
	return env, nil
}

func writeSandboxMCPLauncher() (string, error) {
	f, err := os.CreateTemp("", "aurago-sandbox-mcp-*.py")
	if err != nil {
		return "", fmt.Errorf("create sandbox launcher: %w", err)
	}
	path := f.Name()
	if _, err := f.WriteString(sandboxMCPLaunchSource); err != nil {
		f.Close()
		os.Remove(path)
		return "", fmt.Errorf("write sandbox launcher: %w", err)
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		os.Remove(path)
		return "", fmt.Errorf("protect sandbox launcher: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return "", fmt.Errorf("close sandbox launcher: %w", err)
	}
	return path, nil
}

// SandboxConfig mirrors the config.Sandbox struct for the tools package
// (avoids import cycle with config package).
type SandboxConfig struct {
	Enabled        bool
	RuntimeDir     string
	Backend        string
	DockerHost     string
	Image          string
	AutoInstall    bool
	PoolSize       int
	TimeoutSeconds int
	NetworkEnabled bool
	KeepAlive      bool
	MemoryMB       int
	CPUs           float64
	PIDsLimit      int
	MaxOutputBytes int
	Images         map[string]string
}

// findSystemPython returns the path to a system python3 or python binary, or "" if not found.
func findSystemPython() string {
	candidates := []string{"python3", "python"}
	if runtime.GOOS == "windows" {
		candidates = []string{"python", "python3"}
	}
	for _, name := range candidates {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

// isSandboxMCPInstalled checks whether 'llm_sandbox' and its MCP server module
// can be imported by the given Python. Installing just llm-sandbox[docker] is
// not enough — the MCP server requires the 'mcp' package which is only
// bundled in the mcp-docker / mcp-podman extras.
func isSandboxMCPInstalled(pythonBin string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Check both the base package and the MCP server sub-module
	cmd := supervisorPythonCommand(ctx, pythonBin, "-c",
		"import importlib.metadata; import llm_sandbox.mcp_server.server; assert importlib.metadata.version('llm-sandbox') == '0.3.45'; print('ok')")
	out, err := cmd.CombinedOutput()
	return err == nil && strings.TrimSpace(string(out)) == "ok"
}
