package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

type sandboxConn interface {
	initialize(logger *slog.Logger) error
	discoverTools(logger *slog.Logger) error
	callTool(toolName string, arguments map[string]interface{}) (string, error)
	close()
	markReady()
	toolCount() int
}

type sandboxConnFactory func() (sandboxConn, error)

// ── LLM-Sandbox Integration ────────────────────────────────────────────────
//
// Launches the llm-sandbox MCP server (`python3 -m llm_sandbox.mcp_server.server`)
// as a child process and communicates via the JSON-RPC 2.0 stdio transport
// (reusing the mcpConn type from mcp.go — same package).
//
// Environment matrix:
//   - Docker available  → sandbox runs code in isolated containers (default)
//   - Docker absent     → sandbox disabled; agent falls back to execute_python
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
	connFactory  sandboxConnFactory
}

var (
	globalSandboxMgr *SandboxManager
	sandboxMgrMu     sync.RWMutex
)

// InitSandboxManager creates and starts the sandbox MCP server.
// Call once at startup. On failure the manager is still stored but status.Ready == false,
// so the agent can fall back to execute_python.
func InitSandboxManager(cfg SandboxConfig, workspaceDir string, logger *slog.Logger) *SandboxManager {
	mgr := &SandboxManager{
		logger:       logger,
		status:       SandboxStatus{Backend: cfg.Backend},
		cfg:          cfg,
		workspaceDir: workspaceDir,
	}

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
		// Docker: try ping first, fall back to CLI check
		host := cfg.DockerHost
		if err := DockerPing(host); err != nil {
			// Also try docker CLI as fallback
			if _, lookErr := exec.LookPath("docker"); lookErr != nil {
				mgr.status.Error = fmt.Sprintf("Docker unreachable: %v", err)
				logger.Warn("[Sandbox] Docker not available — sandbox disabled", "error", err)
				registerSandboxSingleton(mgr)
				return mgr
			}
		}
		mgr.status.DockerAvailable = true
	}

	// 2. Check Python availability
	pythonBin := GetPythonBin(workspaceDir)
	if _, err := exec.LookPath(pythonBin); err != nil {
		// Try system python3/python as fallback
		pythonBin = findSystemPython()
		if pythonBin == "" {
			mgr.status.Error = "Python not found"
			logger.Warn("[Sandbox] Python not found — sandbox disabled")
			registerSandboxSingleton(mgr)
			return mgr
		}
	}
	mgr.status.PythonAvailable = true
	mgr.pythonBin = pythonBin

	// 3. Auto-install llm-sandbox if configured
	if cfg.AutoInstall {
		if !isSandboxMCPInstalled(pythonBin) {
			logger.Info("[Sandbox] Installing llm-sandbox MCP package...", "backend", cfg.Backend)
			if err := installSandboxPackage(workspaceDir, cfg.Backend, logger); err != nil {
				mgr.status.Error = fmt.Sprintf("Failed to install llm-sandbox: %v", err)
				logger.Error("[Sandbox] Package installation failed", "error", err)
				registerSandboxSingleton(mgr)
				return mgr
			}
			// Recheck after install — use the venv python which now has the package
			pythonBin = GetPythonBin(workspaceDir)
			mgr.pythonBin = pythonBin
		}
	}

	// Verify package + MCP deps are present
	if isSandboxMCPInstalled(pythonBin) {
		mgr.status.PackageInstalled = true
	} else {
		mgr.status.Error = "llm-sandbox MCP extras not installed (set sandbox.auto_install: true or run: pip install 'llm-sandbox[mcp-docker]')"
		logger.Warn("[Sandbox] llm-sandbox MCP dependencies not found", "python", pythonBin)
		registerSandboxSingleton(mgr)
		return mgr
	}

	// 4. Build environment variables for the MCP server
	env := map[string]string{
		"BACKEND": cfg.Backend,
	}
	if cfg.DockerHost != "" {
		env["DOCKER_HOST"] = cfg.DockerHost
	}
	if cfg.Image != "" {
		env["IMAGE"] = cfg.Image
	}
	if !cfg.NetworkEnabled {
		env["ENABLE_NETWORKING"] = "false"
	} else {
		env["ENABLE_NETWORKING"] = "true"
	}
	mgr.env = env
	args := []string{"-m", "llm_sandbox.mcp_server.server"}
	mgr.connFactory = func() (sandboxConn, error) {
		return newMCPConn("llm-sandbox", mgr.pythonBin, args, mgr.env, logger)
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
	if m.connFactory == nil {
		return nil, fmt.Errorf("sandbox connection factory not configured")
	}

	conn, err := m.connFactory()
	if err != nil {
		return nil, fmt.Errorf("failed to start sandbox MCP server: %w", err)
	}
	if err := conn.initialize(m.logger); err != nil {
		conn.close()
		return nil, fmt.Errorf("MCP initialization failed: %w", err)
	}
	if err := conn.discoverTools(m.logger); err != nil {
		conn.close()
		return nil, fmt.Errorf("tool discovery failed: %w", err)
	}

	conn.markReady()
	return conn, nil
}

func (m *SandboxManager) borrowConn() (sandboxConn, func(), error) {
	m.mu.RLock()
	ready := m.status.Ready
	errMsg := m.status.Error
	keepAlive := m.cfg.KeepAlive
	conn := m.conn
	m.mu.RUnlock()

	if !ready {
		if errMsg == "" {
			errMsg = "sandbox not ready"
		}
		return nil, nil, fmt.Errorf("sandbox not ready: %s", errMsg)
	}

	if !keepAlive {
		conn, err := m.openConn()
		if err != nil {
			return nil, nil, err
		}
		return conn, func() { conn.close() }, nil
	}

	m.sharedConnMu.Lock()
	cleanup := func() {
		m.sharedConnMu.Unlock()
	}

	if conn != nil {
		return conn, cleanup, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.conn == nil {
		conn, err := m.openConn()
		if err != nil {
			cleanup()
			return nil, nil, err
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
	conn, cleanup, err := m.borrowConn()
	if err != nil {
		return "", err
	}
	defer cleanup()

	args := map[string]interface{}{
		"code":     code,
		"language": language,
	}
	if len(libraries) > 0 {
		args["libraries"] = libraries
	}

	// Use a timeout context; fall back to the central sandbox timeout default.
	if timeoutSecs <= 0 {
		timeoutSecs = int(GetSandboxTimeout().Seconds())
	}

	type result struct {
		output string
		err    error
	}
	ch := make(chan result, 1)
	go func() {
		out, err := conn.callTool("execute_code", args)
		ch <- result{out, err}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	select {
	case res := <-ch:
		return res.output, res.err
	case <-ctx.Done():
		// Timeout: close the connection to terminate the blocked goroutine
		// and prevent reuse of a potentially corrupted connection.
		// markUnhealthy is called asynchronously because close() may block
		// waiting for the subprocess to exit.
		go m.markUnhealthy()
		return "", fmt.Errorf("TIMEOUT: sandbox execution exceeded %ds limit", timeoutSecs)
	}
}

// GetSupportedLanguages calls get_supported_languages on the sandbox.
func (m *SandboxManager) GetSupportedLanguages() ([]string, error) {
	conn, cleanup, err := m.borrowConn()
	if err != nil {
		return nil, err
	}
	defer cleanup()

	out, err := conn.callTool("get_supported_languages", map[string]interface{}{})
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
	out, err := conn.callTool("get_supported_languages", map[string]interface{}{})
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
	mgr := GetSandboxManager()
	if mgr == nil {
		return "", fmt.Errorf("sandbox manager not initialized")
	}
	return mgr.ExecuteCode(code, language, libraries, timeoutSecs)
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

// SandboxConfig mirrors the config.Sandbox struct for the tools package
// (avoids import cycle with config package).
type SandboxConfig struct {
	Enabled        bool
	Backend        string
	DockerHost     string
	Image          string
	AutoInstall    bool
	PoolSize       int
	TimeoutSeconds int
	NetworkEnabled bool
	KeepAlive      bool
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
	cmd := exec.CommandContext(ctx, pythonBin, "-c",
		"import llm_sandbox; import llm_sandbox.mcp_server.server; print('ok')")
	out, err := cmd.CombinedOutput()
	if err == nil && strings.TrimSpace(string(out)) == "ok" {
		return true
	}
	// Fallback: check pip for the package + mcp extra marker
	cmd2 := exec.CommandContext(ctx, pythonBin, "-m", "pip", "show", "mcp")
	return cmd2.Run() == nil
}

// installSandboxPackage installs llm-sandbox with the correct MCP+backend extras.
// The mcp-docker / mcp-podman extras include the 'mcp' Python package that is
// required for the MCP stdio server to start.
func installSandboxPackage(workspaceDir string, backend string, logger *slog.Logger) error {
	// Ensure venv exists first
	if err := EnsureVenv(workspaceDir, logger); err != nil {
		return fmt.Errorf("venv setup failed: %w", err)
	}

	// Choose the right extras: mcp-docker or mcp-podman
	extras := "mcp-docker"
	if backend == "podman" {
		extras = "mcp-podman"
	}
	pkg := fmt.Sprintf("llm-sandbox[%s]", extras)
	logger.Info("[Sandbox] Installing package", "pkg", pkg)
	stdout, stderr, err := InstallPackage(pkg, workspaceDir)
	if err != nil {
		return fmt.Errorf("pip install failed: %w\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	logger.Info("[Sandbox] llm-sandbox installed successfully", "pkg", pkg)
	return nil
}
