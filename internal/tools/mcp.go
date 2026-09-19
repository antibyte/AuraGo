package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"aurago/internal/sandbox"
)

// ── MCP (Model Context Protocol) Client ─────────────────────────────────────
//
// Implements a minimal MCP client using the JSON-RPC 2.0 stdio transport.
// Connects to external MCP servers, discovers their tools, and calls them.

// MCPServerConfig describes one MCP server from the config.
type MCPServerConfig struct {
	Name               string            `yaml:"name"                     json:"name"`
	Transport          string            `yaml:"transport,omitempty"      json:"transport"`
	URL                string            `yaml:"url,omitempty"            json:"url"`
	Headers            map[string]string `yaml:"headers,omitempty"        json:"headers"`
	Command            string            `yaml:"command"                  json:"command"`
	Args               []string          `yaml:"args"                     json:"args"`
	Env                map[string]string `yaml:"env"                      json:"env"`
	Enabled            bool              `yaml:"enabled"                  json:"enabled"`
	Runtime            string            `yaml:"runtime,omitempty"        json:"runtime"`
	DockerImage        string            `yaml:"docker_image,omitempty"   json:"docker_image"`
	DockerCommand      string            `yaml:"docker_command,omitempty" json:"docker_command"`
	AllowLocalFallback bool              `yaml:"allow_local_fallback,omitempty" json:"allow_local_fallback"`
	HostWorkdir        string            `yaml:"host_workdir,omitempty"   json:"host_workdir"`
	ContainerWorkdir   string            `yaml:"container_workdir,omitempty" json:"container_workdir"`
	AllowedTools       []string          `yaml:"allowed_tools,omitempty"  json:"allowed_tools,omitempty"`
	AllowDestructive   bool              `yaml:"allow_destructive,omitempty" json:"allow_destructive,omitempty"`
	Secrets            map[string]string `yaml:"-"                        json:"-"`
}

// MCPToolInfo describes a tool exposed by an MCP server.
type MCPToolInfo struct {
	Server      string                 `json:"server"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema,omitempty"`
}

type MCPConnectionTestResult struct {
	Status    string `json:"status"`
	Server    string `json:"server"`
	ToolCount int    `json:"tool_count"`
}

// ── JSON-RPC 2.0 types ─────────────────────────────────────────────────────

type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// jsonRPCNotification is a JSON-RPC notification (no ID field per spec).
type jsonRPCNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	Method  string          `json:"method,omitempty"`
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ── Single MCP server connection ────────────────────────────────────────────

type mcpTransport interface {
	Send(ctx context.Context, method string, params interface{}) (*jsonRPCResponse, error)
	Notify(ctx context.Context, method string, params interface{}) error
	Close()
}

type mcpProtocolVersionSetter interface {
	SetProtocolVersion(version string)
}

const (
	mcpProtocolVersionCurrent = "2025-11-25"
	mcpProtocolVersionLegacy  = "2024-11-05"
)

// safeBuffer is a thread-safe wrapper around bytes.Buffer for capturing
// process stderr without data races.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *safeBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Len()
}

type mcpConn struct {
	discoveryMu     sync.Mutex
	toolsGeneration uint64
	toolsRefreshed  time.Time
	name            string
	transport       mcpTransport
	mu              sync.Mutex
	tools           []MCPToolInfo
	ready           bool
	closeOnce       sync.Once
	stderrBuf       *safeBuffer // captures local MCP server stderr for diagnostics
	runtime         string
	hostDir         string
	contDir         string
}

var (
	startManagedMCPConn = startMCPServerConnection
	invokeMCPConnTool   = func(ctx context.Context, conn *mcpConn, toolName string, arguments map[string]interface{}) (string, error) {
		return conn.callTool(ctx, toolName, arguments)
	}
	closeManagedMCPConn = func(conn *mcpConn) {
		if conn != nil {
			conn.close()
		}
	}
)

func (c *mcpConn) markReady() {
	c.mu.Lock()
	c.ready = true
	c.mu.Unlock()
}

func (c *mcpConn) toolCount() int {
	return len(c.tools)
}

func expandMCPPathValue(value string) string {
	if value == "" || value[0] != '~' {
		return value
	}
	if value != "~" && !strings.HasPrefix(value, "~/") && !strings.HasPrefix(value, "~\\") {
		return value
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return value
	}
	if value == "~" {
		return home
	}
	trimmed := strings.TrimPrefix(strings.TrimPrefix(value[1:], "/"), "\\")
	if trimmed == "" {
		return home
	}
	return filepath.Join(home, filepath.FromSlash(strings.ReplaceAll(trimmed, "\\", "/")))
}

func normalizeMCPArgs(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	normalized := make([]string, len(args))
	for i, arg := range args {
		normalized[i] = expandMCPPathValue(arg)
	}
	return normalized
}

func normalizeMCPEnv(env map[string]string) map[string]string {
	if len(env) == 0 {
		return nil
	}
	normalized := make(map[string]string, len(env))
	for k, v := range env {
		normalized[k] = expandMCPPathValue(v)
	}
	return normalized
}

func mcpStderrSnippet(stderr *safeBuffer) string {
	if stderr == nil {
		return ""
	}
	text := strings.TrimSpace(stderr.String())
	if text == "" {
		return ""
	}
	const maxLen = 500
	if len(text) > maxLen {
		return text[:maxLen] + "..."
	}
	return text
}

func mcpCommandCandidates(command string) []string {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}
	if runtime.GOOS != "windows" || filepath.Ext(command) != "" {
		return []string{command}
	}

	candidates := []string{command}
	pathext := os.Getenv("PATHEXT")
	if pathext == "" {
		pathext = ".COM;.EXE;.BAT;.CMD"
	}
	for _, ext := range strings.Split(pathext, ";") {
		ext = strings.TrimSpace(ext)
		if ext == "" {
			continue
		}
		candidates = append(candidates, command+strings.ToLower(ext))
		candidates = append(candidates, command+strings.ToUpper(ext))
	}
	return candidates
}

func resolveMCPCommandPath(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}

	if filepath.IsAbs(command) || strings.ContainsRune(command, os.PathSeparator) || (os.PathSeparator != '/' && strings.ContainsRune(command, '/')) {
		return command
	}

	if p, err := exec.LookPath(command); err == nil {
		return p
	}

	var searchDirs []string
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		searchDirs = append(searchDirs,
			filepath.Join(home, ".local", "bin"),
			filepath.Join(home, "bin"),
			filepath.Join(home, "go", "bin"),
		)
	}
	searchDirs = append(searchDirs, "/usr/local/bin", "/usr/bin")

	for _, dir := range searchDirs {
		for _, candidate := range mcpCommandCandidates(command) {
			fullPath := filepath.Join(dir, candidate)
			if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
				return fullPath
			}
		}
	}

	return command
}

func newMCPConn(name, command string, args []string, env map[string]string, logger *slog.Logger, runtimeName, hostWorkdir, containerWorkdir string) (*mcpConn, error) {
	command = resolveMCPCommandPath(expandMCPPathValue(strings.TrimSpace(command)))
	args = normalizeMCPArgs(args)
	env = normalizeMCPEnv(env)

	cmd := exec.Command(command, args...)

	// Build environment from a scrubbed base; MCP servers must not inherit host secrets.
	cmdEnv := sandbox.FilterEnv(os.Environ())
	if len(env) > 0 {
		for k, v := range env {
			cmdEnv = append(cmdEnv, k+"="+v)
		}
	}
	cmd.Env = cmdEnv

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		stdinPipe.Close()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	// Capture stderr for diagnostics (MCP server startup errors, etc.)
	stderrBuf := &safeBuffer{}
	cmd.Stderr = stderrBuf

	if err := cmd.Start(); err != nil {
		stdinPipe.Close()
		if snippet := mcpStderrSnippet(stderrBuf); snippet != "" {
			return nil, fmt.Errorf("start command %q: %w (stderr: %s)", command, err, snippet)
		}
		return nil, fmt.Errorf("start command %q: %w", command, err)
	}

	conn := &mcpConn{
		name: name,
		transport: newStdioMCPTransport(
			cmd,
			stdinPipe,
			stdoutPipe,
			stderrBuf,
		),
		stderrBuf: stderrBuf,
		runtime:   runtimeName,
		hostDir:   hostWorkdir,
		contDir:   containerWorkdir,
	}

	logger.Info("[MCP] Server process started", "name", name, "command", command, "pid", cmd.Process.Pid)
	return conn, nil
}

func newLocalMCPConn(srv MCPServerConfig, logger *slog.Logger) (*mcpConn, error) {
	args, env, err := resolveMCPLaunchArgsAndEnv(srv, false)
	if err != nil {
		return nil, err
	}
	return newMCPConn(srv.Name, srv.Command, args, env, logger, "local", srv.HostWorkdir, srv.ContainerWorkdir)
}

func newDockerMCPConn(srv MCPServerConfig, logger *slog.Logger) (*mcpConn, error) {
	if strings.TrimSpace(srv.DockerImage) == "" {
		return nil, fmt.Errorf("docker_image is required for MCP server %q when runtime=docker", srv.Name)
	}
	if err := ensureMCPHostWorkdir(srv.HostWorkdir); err != nil {
		return nil, err
	}
	args, env, err := resolveMCPLaunchArgsAndEnv(srv, true)
	if err != nil {
		return nil, err
	}
	containerWorkdir := strings.TrimSpace(srv.ContainerWorkdir)
	if containerWorkdir == "" {
		containerWorkdir = defaultMCPContainerWorkdir
	}
	containerCommand := strings.TrimSpace(srv.DockerCommand)
	if containerCommand == "" {
		containerCommand = strings.TrimSpace(srv.Command)
	}
	if containerCommand == "" {
		return nil, fmt.Errorf("docker_command or command is required for MCP server %q", srv.Name)
	}

	dockerArgs := []string{
		"run", "--rm", "-i",
		"-v", fmt.Sprintf("%s:%s", srv.HostWorkdir, containerWorkdir),
		"-w", containerWorkdir,
	}
	for key, value := range env {
		dockerArgs = append(dockerArgs, "-e", key+"="+value)
	}
	dockerArgs = append(dockerArgs, strings.TrimSpace(srv.DockerImage), containerCommand)
	dockerArgs = append(dockerArgs, args...)

	return newMCPConn(srv.Name, "docker", dockerArgs, nil, logger, "docker", srv.HostWorkdir, containerWorkdir)
}

func mcpTransportMode(srv MCPServerConfig) string {
	switch strings.ToLower(strings.TrimSpace(srv.Transport)) {
	case "streamable_http", "http":
		return "streamable_http"
	case "sse":
		return "sse"
	case "websocket", "ws":
		return "websocket"
	case "stdio", "":
		return "stdio"
	default:
		return strings.ToLower(strings.TrimSpace(srv.Transport))
	}
}

func mcpUsesNetworkTransport(srv MCPServerConfig) bool {
	switch mcpTransportMode(srv) {
	case "streamable_http", "sse", "websocket":
		return true
	default:
		return false
	}
}

func startMCPServerConnection(ctx context.Context, srv MCPServerConfig, logger *slog.Logger) (*mcpConn, error) {
	var (
		conn *mcpConn
		err  error
	)
	switch mcpTransportMode(srv) {
	case "stdio":
		if mcpRuntimeMode(srv.Runtime) == "docker" {
			conn, err = newDockerMCPConn(srv, logger)
			if err != nil && srv.AllowLocalFallback {
				logger.Warn("[MCP] Docker runtime failed, falling back to local execution", "server", srv.Name, "error", err)
				conn, err = newLocalMCPConn(srv, logger)
			}
		} else {
			if strings.TrimSpace(srv.Command) == "" {
				return nil, fmt.Errorf("command is required for MCP server %q when transport=stdio", srv.Name)
			}
			conn, err = newLocalMCPConn(srv, logger)
		}
	case "streamable_http", "sse", "websocket":
		conn, err = newNetworkMCPConn(ctx, srv, logger)
	default:
		err = fmt.Errorf("unsupported MCP transport %q for server %q", srv.Transport, srv.Name)
	}
	if err != nil {
		return nil, err
	}
	if err := conn.initialize(ctx, logger); err != nil {
		closeManagedMCPConn(conn)
		return nil, fmt.Errorf("initialize failed: %w", err)
	}
	if err := conn.discoverTools(ctx, logger); err != nil {
		closeManagedMCPConn(conn)
		return nil, fmt.Errorf("tool discovery failed: %w", err)
	}
	conn.markReady()
	return conn, nil
}

func TestMCPServerConnection(srv MCPServerConfig, logger *slog.Logger) (MCPConnectionTestResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), mcpCallToolTimeout)
	defer cancel()
	conn, err := startMCPServerConnection(ctx, srv, logger)
	if err != nil {
		return MCPConnectionTestResult{
			Status: "error",
			Server: srv.Name,
		}, err
	}
	defer closeManagedMCPConn(conn)
	return MCPConnectionTestResult{
		Status:    "ok",
		Server:    srv.Name,
		ToolCount: conn.toolCount(),
	}, nil
}

// send writes a JSON-RPC request and reads the response.
func (c *mcpConn) send(ctx context.Context, method string, params interface{}) (*jsonRPCResponse, error) {
	if c.transport == nil {
		return nil, fmt.Errorf("MCP transport is not initialized")
	}
	return c.transport.Send(ctx, method, params)
}

// initialize performs the MCP initialize handshake + notifications/initialized.
func (c *mcpConn) initialize(ctx context.Context, logger *slog.Logger) error {
	initParams := map[string]interface{}{
		"protocolVersion": mcpProtocolVersionCurrent,
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "aurago",
			"version": "1.0.0",
		},
	}

	resp, err := c.send(ctx, "initialize", initParams)
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("initialize error: %s (code %d)", resp.Error.Message, resp.Error.Code)
	}

	// Parse server info for logging
	var result struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse initialize result: %w", err)
	}
	if result.ProtocolVersion != mcpProtocolVersionCurrent && result.ProtocolVersion != mcpProtocolVersionLegacy {
		return fmt.Errorf("MCP server selected unsupported protocol version %q", result.ProtocolVersion)
	}
	if setter, ok := c.transport.(mcpProtocolVersionSetter); ok {
		setter.SetProtocolVersion(result.ProtocolVersion)
	}
	if result.ServerInfo.Name != "" {
		logger.Info("[MCP] Server identified", "name", c.name, "server", result.ServerInfo.Name, "version", result.ServerInfo.Version)
	}

	if err := c.transport.Notify(ctx, "notifications/initialized", nil); err != nil {
		logger.Warn("[MCP] Failed to send notifications/initialized", "name", c.name, "error", err)
	}

	return nil
}

// discoverTools calls tools/list and caches the results.
func (c *mcpConn) discoverTools(ctx context.Context, logger *slog.Logger) error {
	c.discoveryMu.Lock()
	defer c.discoveryMu.Unlock()
	return c.discoverToolsLocked(ctx, logger)
}

func (c *mcpConn) discoverToolsLocked(ctx context.Context, logger *slog.Logger) error {
	generation := mcpToolsGeneration(c.transport)
	cursor := ""
	seen := map[string]bool{}
	names := map[string]bool{}
	var newTools []MCPToolInfo
	for page := 0; ; page++ {
		if page >= 128 {
			return fmt.Errorf("tools/list exceeded page limit")
		}
		params := map[string]interface{}{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		resp, err := c.send(ctx, "tools/list", params)
		if err != nil {
			return fmt.Errorf("tools/list: %w", err)
		}
		if resp == nil {
			return fmt.Errorf("tools/list returned no response")
		}
		if resp.Error != nil {
			return fmt.Errorf("tools/list error: %s (code %d)", resp.Error.Message, resp.Error.Code)
		}
		if len(resp.Result) > 16*1024*1024 {
			return fmt.Errorf("tools/list exceeded page size limit")
		}
		var result struct {
			Tools []struct {
				Name        string                 `json:"name"`
				Description string                 `json:"description"`
				InputSchema map[string]interface{} `json:"inputSchema"`
			} `json:"tools"`
			NextCursor string `json:"nextCursor"`
		}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return fmt.Errorf("parse tools/list result: %w", err)
		}
		if result.Tools == nil {
			return fmt.Errorf("tools/list result is missing tools")
		}
		for _, t := range result.Tools {
			if t.Name == "" || names[t.Name] {
				return fmt.Errorf("tools/list contains an empty or duplicate name")
			}
			names[t.Name] = true
			newTools = append(newTools, MCPToolInfo{Server: c.name, Name: t.Name, Description: t.Description, InputSchema: t.InputSchema})
		}
		if len(newTools) > 10000 {
			return fmt.Errorf("tools/list exceeded tool limit")
		}
		cursor = result.NextCursor
		if cursor == "" {
			break
		}
		if seen[cursor] {
			return fmt.Errorf("tools/list repeated a pagination cursor")
		}
		seen[cursor] = true
	}
	c.mu.Lock()
	c.tools, c.toolsGeneration, c.toolsRefreshed = newTools, generation, time.Now()
	c.mu.Unlock()
	if logger != nil {
		logger.Info("[MCP] Tools discovered", "server", c.name, "count", len(newTools))
	}
	return nil
}

// callTool invokes tools/call on this server connection.
func (c *mcpConn) callTool(ctx context.Context, toolName string, arguments map[string]interface{}) (string, error) {
	params := map[string]interface{}{
		"name":      toolName,
		"arguments": arguments,
	}

	resp, err := c.send(ctx, "tools/call", params)
	if err != nil {
		return "", fmt.Errorf("tools/call: %w", err)
	}
	if resp == nil {
		return "", fmt.Errorf("tools/call returned no response")
	}
	if resp.Error != nil {
		return "", fmt.Errorf("MCP server error: %s (code %d)", resp.Error.Message, resp.Error.Code)
	}

	// Parse the content array
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError           bool            `json:"isError"`
		StructuredContent json.RawMessage `json:"structuredContent"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("invalid tools/call result: %w", err)
	}
	if result.Content == nil && len(result.StructuredContent) == 0 {
		return "", fmt.Errorf("tools/call result is missing content")
	}

	if result.IsError {
		var texts []string
		for _, item := range result.Content {
			if item.Text != "" {
				texts = append(texts, item.Text)
			}
		}
		message := strings.Join(texts, "; ")
		if message == "" {
			message = "MCP tool reported an execution error"
		}
		return "", &MCPToolExecutionError{Message: message, Result: append(json.RawMessage(nil), resp.Result...)}
	}

	// Preserve all structured, resource, image and audio content when mixed.
	mixed := len(result.StructuredContent) > 0
	for _, item := range result.Content {
		if item.Type != "text" {
			mixed = true
		}
	}
	if mixed {
		return normalizeMCPResultText(string(resp.Result), c.hostDir, c.contDir), nil
	}

	var texts []string
	for _, item := range result.Content {
		if item.Type == "text" && item.Text != "" {
			texts = append(texts, item.Text)
		}
	}
	if len(texts) == 0 {
		return normalizeMCPResultText(string(resp.Result), c.hostDir, c.contDir), nil
	}
	return normalizeMCPResultText(strings.Join(texts, "\n"), c.hostDir, c.contDir), nil
}

func (c *mcpConn) close() {
	c.closeOnce.Do(func() {
		if c.transport != nil {
			c.transport.Close()
		}
	})
}

// ── MCPManager — global manager ─────────────────────────────────────────────

// MCPManager manages connections to multiple MCP servers.
type MCPManager struct {
	mu      sync.RWMutex
	conns   map[string]*mcpConn
	configs map[string]MCPServerConfig
	logger  *slog.Logger
}

var (
	globalMCPManager *MCPManager
	mcpManagerMu     sync.RWMutex
)

// InitMCPManager creates and starts an MCPManager from the config.
// Should be called once at startup (and on config hot-reload if MCP settings change).
func InitMCPManager(servers []MCPServerConfig, logger *slog.Logger) *MCPManager {
	mgr := &MCPManager{
		conns:   make(map[string]*mcpConn),
		configs: make(map[string]MCPServerConfig),
		logger:  logger,
	}

	for _, srv := range servers {
		if !srv.Enabled || srv.Name == "" {
			continue
		}
		hasCmd := strings.TrimSpace(srv.Command) != ""
		hasDockerCmd := strings.TrimSpace(srv.DockerCommand) != ""
		if !hasCmd && !mcpUsesNetworkTransport(srv) && (mcpRuntimeMode(srv.Runtime) != "docker" || !hasDockerCmd) {
			continue
		}
		mgr.configs[srv.Name] = srv

		ctx, cancel := context.WithTimeout(context.Background(), mcpCallToolTimeout)
		conn, err := startManagedMCPConn(ctx, srv, logger)
		cancel()
		if err != nil {
			logger.Error("[MCP] Failed to start server", "name", srv.Name, "error", err)
			continue
		}
		mgr.conns[srv.Name] = conn
	}

	// Register as global singleton
	mcpManagerMu.Lock()
	globalMCPManager = mgr
	mcpManagerMu.Unlock()

	if len(servers) > 0 && len(mgr.conns) == 0 {
		configured := make([]string, 0, len(servers))
		for _, srv := range servers {
			if srv.Enabled && srv.Name != "" {
				configured = append(configured, srv.Name)
			}
		}
		logger.Warn("[MCP] No configured servers connected", "configured_servers", configured)
	}
	logger.Info("[MCP] Manager initialized", "servers_connected", len(mgr.conns))
	return mgr
}

func (m *MCPManager) configuredServerNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.configs))
	for name, cfg := range m.configs {
		if cfg.Enabled && cfg.Name != "" && (cfg.Command != "" || mcpUsesNetworkTransport(cfg)) {
			names = append(names, name)
		}
	}
	return names
}

func (m *MCPManager) invalidateConnection(serverName string, reason error) {
	m.mu.Lock()
	conn := m.conns[serverName]
	delete(m.conns, serverName)
	m.mu.Unlock()
	if conn != nil {
		go closeManagedMCPConn(conn)
	}
	if reason != nil {
		m.logger.Warn("[MCP] Invalidated server connection", "server", serverName, "reason", reason)
	}
}

func (m *MCPManager) ensureServerConnected(ctx context.Context, serverName string) (*mcpConn, error) {
	m.mu.RLock()
	if conn, ok := m.conns[serverName]; ok {
		conn.mu.Lock()
		ready := conn.ready
		conn.mu.Unlock()
		if ready {
			m.mu.RUnlock()
			return conn, nil
		}
	}
	cfg, ok := m.configs[serverName]
	m.mu.RUnlock()
	if !ok || !cfg.Enabled || cfg.Name == "" || (cfg.Command == "" && !mcpUsesNetworkTransport(cfg)) {
		return nil, fmt.Errorf("MCP server %q not found or not connected", serverName)
	}

	m.logger.Warn("[MCP] Reconnecting configured server", "server", serverName)
	conn, err := startManagedMCPConn(ctx, cfg, m.logger)
	if err != nil {
		return nil, fmt.Errorf("reconnect %q failed: %w", serverName, err)
	}

	m.mu.Lock()
	if existing, ok := m.conns[serverName]; ok {
		existing.mu.Lock()
		ready := existing.ready
		existing.mu.Unlock()
		if ready {
			m.mu.Unlock()
			closeManagedMCPConn(conn)
			return existing, nil
		}
		go closeManagedMCPConn(existing)
	}
	m.conns[serverName] = conn
	m.mu.Unlock()
	return conn, nil
}

func (m *MCPManager) ensureConfiguredServersConnected() {
	for _, serverName := range m.configuredServerNames() {
		ctx, cancel := context.WithTimeout(context.Background(), mcpCallToolTimeout)
		_, err := m.ensureServerConnected(ctx, serverName)
		cancel()
		if err != nil {
			m.logger.Warn("[MCP] Configured server not connected", "server", serverName, "error", err)
		}
	}
}

func isRetryableMCPTransportError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || errors.Is(err, errMCPTransportClosed) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}
	lower := strings.ToLower(err.Error())
	needles := []string{
		"broken pipe", "eof", "read from stdout", "write to stdin",
		"connection reset", "connection closed", "file already closed",
		"timed out", "timeout", "transport is closing",
	}
	for _, needle := range needles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

// GetMCPManager returns the global MCPManager singleton.
func GetMCPManager() *MCPManager {
	mcpManagerMu.RLock()
	defer mcpManagerMu.RUnlock()
	return globalMCPManager
}

// ShutdownMCPManager gracefully stops the global MCP manager.
func ShutdownMCPManager() {
	mcpManagerMu.Lock()
	mgr := globalMCPManager
	globalMCPManager = nil
	mcpManagerMu.Unlock()
	if mgr != nil {
		mgr.Close()
	}
}

// ListTools returns all discovered tools, optionally filtered by server name.
func (m *MCPManager) ListTools(serverName string) []MCPToolInfo {
	ctx, cancel := context.WithTimeout(context.Background(), mcpCallToolTimeout)
	defer cancel()
	result, err := m.ListToolsWithStatus(ctx, serverName)
	if err != nil && m.logger != nil {
		m.logger.Warn("[MCP] Could not list tools", "error", err)
	}
	return result
}

// ListToolsWithStatus distinguishes an unavailable catalog from an empty one.
func (m *MCPManager) ListToolsWithStatus(ctx context.Context, serverName string) ([]MCPToolInfo, error) {
	m.mu.RLock()
	var names []string
	for name := range m.configs {
		if serverName == "" || serverName == name {
			names = append(names, name)
		}
	}
	m.mu.RUnlock()
	if serverName != "" && len(names) == 0 {
		return nil, fmt.Errorf("MCP server %q is not configured", serverName)
	}
	sort.Strings(names)
	var problems []error
	for _, name := range names {
		conn, err := m.ensureServerConnected(ctx, name)
		if err == nil {
			err = conn.refreshToolsIfNeeded(ctx, m.logger)
		}
		if err != nil {
			problems = append(problems, fmt.Errorf("server %s catalog unavailable: %w", name, err))
		}
	}
	if len(problems) > 0 {
		return m.CachedTools(serverName), errors.Join(problems...)
	}
	return m.CachedTools(serverName), nil
}

// CachedTools is a nonblocking snapshot for prompt construction. It never starts a connection.
func (m *MCPManager) CachedTools(serverName string) []MCPToolInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]MCPToolInfo, 0)
	for name, conn := range m.conns {
		if serverName != "" && name != serverName {
			continue
		}
		cfg, ok := m.configs[name]
		if !ok {
			continue
		}
		conn.mu.Lock()
		if cfg.Enabled && conn.ready && conn.toolsGeneration == mcpToolsGeneration(conn.transport) && (conn.transport == nil || time.Since(conn.toolsRefreshed) < time.Minute) {
			for _, t := range conn.tools {
				if mcpToolVisible(cfg, t.Name) {
					result = append(result, t)
				}
			}
		}
		conn.mu.Unlock()
	}
	return result
}

// MCPToolExecutionError distinguishes a completed MCP error result from transport failure.
type MCPToolExecutionError struct {
	Message string
	Result  json.RawMessage
}

func (e *MCPToolExecutionError) Error() string { return "tool returned error: " + e.Message }

// ListServers reports configured servers without starting connections as a side effect.
func (m *MCPManager) ListServers() []map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]map[string]interface{}, 0, len(m.configs))
	for name, cfg := range m.configs {
		ready, count := false, 0
		if conn := m.conns[name]; conn != nil {
			conn.mu.Lock()
			ready, count = conn.ready, len(conn.tools)
			conn.mu.Unlock()
		}
		result = append(result, map[string]interface{}{"name": name, "configured": true, "enabled": cfg.Enabled, "ready": ready, "tool_count": count})
	}
	sort.Slice(result, func(i, j int) bool { return result[i]["name"].(string) < result[j]["name"].(string) })
	return result
}

const mcpCallToolTimeout = 60 * time.Second

// CallTool never replays a sent request: a transport failure cannot establish
// whether a remote mutation completed. Reconnection is for a subsequent call.
func (m *MCPManager) CallTool(ctx context.Context, serverName, toolName string, arguments map[string]interface{}) (string, error) {
	if err := m.requireToolAllowed(serverName, toolName); err != nil {
		return "", err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	callCtx, cancel := context.WithTimeout(ctx, mcpCallToolTimeout)
	defer cancel()
	if err := callCtx.Err(); err != nil {
		return "", err
	}
	conn, err := m.ensureServerConnected(callCtx, serverName)
	if err != nil {
		return "", err
	}
	if err := conn.refreshToolsIfNeeded(callCtx, m.logger); err != nil {
		return "", err
	}
	// Recheck configuration after connection/discovery work.
	if err := m.requireToolAllowed(serverName, toolName); err != nil {
		return "", err
	}
	result, err := invokeMCPConnTool(callCtx, conn, toolName, arguments)
	if err != nil && (callCtx.Err() != nil || isRetryableMCPTransportError(err)) {
		m.invalidateConnection(serverName, err)
		return "", fmt.Errorf("MCP call outcome uncertain; request was not replayed (server=%s, tool=%s): %w", serverName, toolName, err)
	}
	return result, err
}

func (m *MCPManager) requireToolAllowed(serverName, toolName string) error {
	if m == nil {
		return fmt.Errorf("MCP manager not initialized")
	}
	m.mu.Lock()
	cfg, ok := m.configs[serverName]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("MCP server %q not configured", serverName)
	}
	if allowed, restricted := mcpAllowedToolSet(cfg); restricted {
		if _, ok := allowed[toolName]; !ok {
			return fmt.Errorf("MCP tool %q is not allowed for server %q", toolName, serverName)
		}
	}
	if !cfg.AllowDestructive && isMCPToolNameDestructive(toolName) {
		return fmt.Errorf("MCP tool %q is blocked for server %q because allow_destructive is false", toolName, serverName)
	}
	if !cfg.Enabled {
		return fmt.Errorf("MCP server %q is disabled", serverName)
	}
	return nil
}

func mcpAllowedToolSet(cfg MCPServerConfig) (map[string]struct{}, bool) {
	allowed := make(map[string]struct{}, len(cfg.AllowedTools))
	for _, name := range cfg.AllowedTools {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			allowed[trimmed] = struct{}{}
		}
	}
	return allowed, len(allowed) > 0
}

func mcpToolVisible(cfg MCPServerConfig, toolName string) bool {
	if allowed, restricted := mcpAllowedToolSet(cfg); restricted {
		if _, ok := allowed[toolName]; !ok {
			return false
		}
	}
	return cfg.AllowDestructive || !isMCPToolNameDestructive(toolName)
}

func isMCPToolNameDestructive(toolName string) bool {
	normalized := strings.ToLower(strings.TrimSpace(toolName))
	if normalized == "" {
		return false
	}
	parts := strings.FieldsFunc(normalized, func(r rune) bool {
		return r == '_' || r == '-' || r == '.' || r == '/' || r == ' '
	})
	for _, part := range parts {
		switch part {
		case "delete", "destroy", "remove", "drop", "wipe", "purge", "truncate", "format":
			return true
		}
	}
	return false
}

// Close shuts down all MCP server connections.
func (m *MCPManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, conn := range m.conns {
		m.logger.Info("[MCP] Stopping server", "name", name)
		closeManagedMCPConn(conn)
	}
	m.conns = make(map[string]*mcpConn)
}

// MCPListTools is a package-level shorthand for agent dispatch.
func MCPListTools(serverName string, logger *slog.Logger) ([]MCPToolInfo, error) {
	return MCPListToolsContext(context.Background(), serverName, logger)
}

func MCPListToolsContext(ctx context.Context, serverName string, logger *slog.Logger) ([]MCPToolInfo, error) {
	mgr := GetMCPManager()
	if mgr == nil {
		return nil, fmt.Errorf("MCP manager not initialized")
	}
	ctx, cancel := context.WithTimeout(ctx, mcpCallToolTimeout)
	defer cancel()
	return mgr.ListToolsWithStatus(ctx, serverName)
}

// MCPCallTool is a package-level shorthand for agent dispatch.
func MCPCallTool(ctx context.Context, serverName, toolName string, arguments map[string]interface{}, logger *slog.Logger) (string, error) {
	mgr := GetMCPManager()
	if mgr == nil {
		return "", fmt.Errorf("MCP manager not initialized")
	}
	logger.Info("[MCP] Tool call", "server", serverName, "tool", toolName)
	result, err := mgr.CallTool(ctx, serverName, toolName, arguments)
	if err != nil {
		logger.Warn("[MCP] Tool call failed", "server", serverName, "tool", toolName, "error", err)
	}
	return result, err
}

// MCPListServers is a package-level shorthand for agent dispatch.
func MCPListServers(logger *slog.Logger) ([]map[string]interface{}, error) {
	mgr := GetMCPManager()
	if mgr == nil {
		return nil, fmt.Errorf("MCP manager not initialized")
	}
	return mgr.ListServers(), nil
}
