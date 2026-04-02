package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ── MCP (Model Context Protocol) Client ─────────────────────────────────────
//
// Implements a minimal MCP client using the JSON-RPC 2.0 stdio transport.
// Connects to external MCP servers, discovers their tools, and calls them.

// MCPServerConfig describes one MCP server from the config.
type MCPServerConfig struct {
	Name    string            `yaml:"name"    json:"name"`
	Command string            `yaml:"command" json:"command"`
	Args    []string          `yaml:"args"    json:"args"`
	Env     map[string]string `yaml:"env"     json:"env"`
	Enabled bool              `yaml:"enabled" json:"enabled"`
}

// MCPToolInfo describes a tool exposed by an MCP server.
type MCPToolInfo struct {
	Server      string                 `json:"server"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema,omitempty"`
}

// ── JSON-RPC 2.0 types ─────────────────────────────────────────────────────

type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type jsonRPCResponse struct {
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

type mcpConn struct {
	name      string
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    *bufio.Reader
	mu        sync.Mutex
	nextID    int64
	tools     []MCPToolInfo
	ready     bool
	closeCh   chan struct{}
	closeOnce sync.Once // ensures close() is idempotent
	stderrBuf *bytes.Buffer // captures MCP server stderr for diagnostics
}

func (c *mcpConn) markReady() {
	c.mu.Lock()
	c.ready = true
	c.mu.Unlock()
}

func (c *mcpConn) toolCount() int {
	return len(c.tools)
}

func newMCPConn(name, command string, args []string, env map[string]string, logger *slog.Logger) (*mcpConn, error) {
	cmd := exec.Command(command, args...)

	// Build environment
	if len(env) > 0 {
		cmdEnv := cmd.Environ()
		for k, v := range env {
			cmdEnv = append(cmdEnv, k+"="+v)
		}
		cmd.Env = cmdEnv
	}

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
	stderrBuf := &bytes.Buffer{}
	cmd.Stderr = stderrBuf

	if err := cmd.Start(); err != nil {
		stdinPipe.Close()
		return nil, fmt.Errorf("start command %q: %w", command, err)
	}

	conn := &mcpConn{
		name:      name,
		cmd:       cmd,
		stdin:     stdinPipe,
		stdout:    bufio.NewReaderSize(stdoutPipe, 1024*1024), // 1MB buffer for large responses
		closeCh:   make(chan struct{}),
		stderrBuf: stderrBuf,
	}

	logger.Info("[MCP] Server process started", "name", name, "command", command, "pid", cmd.Process.Pid)
	return conn, nil
}

// send writes a JSON-RPC request and reads the response.
func (c *mcpConn) send(method string, params interface{}) (*jsonRPCResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := atomic.AddInt64(&c.nextID, 1)
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Write request + newline delimiter
	if _, err := c.stdin.Write(append(data, '\n')); err != nil {
		return nil, fmt.Errorf("write to stdin: %w", err)
	}

	// Read responses until we find the one with our ID (skip notifications)
	for {
		line, err := c.stdout.ReadBytes('\n')
		if err != nil {
			if c.stderrBuf != nil && c.stderrBuf.Len() > 0 {
				slog.Default().Error("[MCP] Server stderr", "server", c.name, "stderr", c.stderrBuf.String())
			}
			return nil, fmt.Errorf("read from stdout: %w", err)
		}
		line = []byte(strings.TrimSpace(string(line)))
		if len(line) == 0 {
			continue
		}

		var resp jsonRPCResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			continue // skip malformed lines
		}

		// Skip notifications (no ID)
		if resp.ID == nil {
			continue
		}

		if *resp.ID == id {
			return &resp, nil
		}
	}
}

// initialize performs the MCP initialize handshake + notifications/initialized.
func (c *mcpConn) initialize(logger *slog.Logger) error {
	initParams := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "aurago",
			"version": "1.0.0",
		},
	}

	resp, err := c.send("initialize", initParams)
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("initialize error: %s (code %d)", resp.Error.Message, resp.Error.Code)
	}

	// Parse server info for logging
	var result struct {
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(resp.Result, &result); err == nil && result.ServerInfo.Name != "" {
		logger.Info("[MCP] Server identified", "name", c.name, "server", result.ServerInfo.Name, "version", result.ServerInfo.Version)
	}

	// Send notifications/initialized (no response expected, but we still need to write it)
	notif := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      0, // notifications can have id=0 or none, but for safety we use 0
		Method:  "notifications/initialized",
	}
	data, err := json.Marshal(notif)
	if err != nil {
		logger.Warn("[MCP] Failed to marshal notifications/initialized", "name", c.name, "error", err)
		return nil // non-fatal: server already initialized
	}
	c.mu.Lock()
	_, writeErr := c.stdin.Write(append(data, '\n'))
	c.mu.Unlock()
	if writeErr != nil {
		logger.Warn("[MCP] Failed to send notifications/initialized", "name", c.name, "error", writeErr)
	}

	return nil
}

// discoverTools calls tools/list and caches the results.
func (c *mcpConn) discoverTools(logger *slog.Logger) error {
	resp, err := c.send("tools/list", map[string]interface{}{})
	if err != nil {
		return fmt.Errorf("tools/list: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("tools/list error: %s (code %d)", resp.Error.Message, resp.Error.Code)
	}

	var result struct {
		Tools []struct {
			Name        string                 `json:"name"`
			Description string                 `json:"description"`
			InputSchema map[string]interface{} `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse tools/list result: %w", err)
	}

	newTools := make([]MCPToolInfo, len(result.Tools))
	for i, t := range result.Tools {
		newTools[i] = MCPToolInfo{
			Server:      c.name,
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}
	c.mu.Lock()
	c.tools = newTools
	c.mu.Unlock()

	logger.Info("[MCP] Tools discovered", "server", c.name, "count", len(newTools))
	return nil
}

// callTool invokes tools/call on this server connection.
func (c *mcpConn) callTool(toolName string, arguments map[string]interface{}) (string, error) {
	params := map[string]interface{}{
		"name":      toolName,
		"arguments": arguments,
	}

	resp, err := c.send("tools/call", params)
	if err != nil {
		return "", fmt.Errorf("tools/call: %w", err)
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
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		// Fallback: return raw result
		return string(resp.Result), nil
	}

	if result.IsError {
		var texts []string
		for _, c := range result.Content {
			if c.Text != "" {
				texts = append(texts, c.Text)
			}
		}
		return "", fmt.Errorf("tool returned error: %s", strings.Join(texts, "; "))
	}

	var texts []string
	for _, c := range result.Content {
		if c.Type == "text" && c.Text != "" {
			texts = append(texts, c.Text)
		}
	}
	if len(texts) == 0 {
		return string(resp.Result), nil
	}
	return strings.Join(texts, "\n"), nil
}

func (c *mcpConn) close() {
	c.closeOnce.Do(func() {
		close(c.closeCh)
		c.stdin.Close()
		// Give the process a moment to exit gracefully
		done := make(chan error, 1)
		go func() { done <- c.cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			c.cmd.Process.Kill()
		}
	})
}

// ── MCPManager — global manager ─────────────────────────────────────────────

// MCPManager manages connections to multiple MCP servers.
type MCPManager struct {
	mu     sync.RWMutex
	conns  map[string]*mcpConn
	logger *slog.Logger
}

var (
	globalMCPManager *MCPManager
	mcpManagerMu     sync.RWMutex
)

// InitMCPManager creates and starts an MCPManager from the config.
// Should be called once at startup (and on config hot-reload if MCP settings change).
func InitMCPManager(servers []MCPServerConfig, logger *slog.Logger) *MCPManager {
	mgr := &MCPManager{
		conns:  make(map[string]*mcpConn),
		logger: logger,
	}

	for _, srv := range servers {
		if !srv.Enabled || srv.Command == "" || srv.Name == "" {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		conn, err := newMCPConn(srv.Name, srv.Command, srv.Args, srv.Env, logger)
		cancel()
		if err != nil {
			logger.Error("[MCP] Failed to start server", "name", srv.Name, "error", err)
			continue
		}

		// Initialize the connection
		if err := conn.initialize(logger); err != nil {
			logger.Error("[MCP] Initialization failed", "name", srv.Name, "error", err)
			conn.close()
			continue
		}

		// Discover tools
		if err := conn.discoverTools(logger); err != nil {
			logger.Error("[MCP] Tool discovery failed", "name", srv.Name, "error", err)
			conn.close()
			continue
		}

		conn.markReady()
		mgr.conns[srv.Name] = conn
		_ = ctx // suppress unused var
	}

	// Register as global singleton
	mcpManagerMu.Lock()
	globalMCPManager = mgr
	mcpManagerMu.Unlock()

	logger.Info("[MCP] Manager initialized", "servers_connected", len(mgr.conns))
	return mgr
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
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []MCPToolInfo
	for name, conn := range m.conns {
		if serverName != "" && name != serverName {
			continue
		}
		conn.mu.Lock()
		if conn.ready {
			result = append(result, conn.tools...)
		}
		conn.mu.Unlock()
	}
	return result
}

// ListServers returns a summary of all connected servers and their tool counts.
func (m *MCPManager) ListServers() []map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []map[string]interface{}
	for name, conn := range m.conns {
		conn.mu.Lock()
		ready := conn.ready
		count := len(conn.tools)
		conn.mu.Unlock()
		result = append(result, map[string]interface{}{
			"name":       name,
			"ready":      ready,
			"tool_count": count,
		})
	}
	return result
}

// mcpCallToolTimeout is the maximum duration for a single MCP tool call.
const mcpCallToolTimeout = 60 * time.Second

// CallTool invokes a tool on a specific MCP server with a timeout.
func (m *MCPManager) CallTool(serverName, toolName string, arguments map[string]interface{}) (string, error) {
	m.mu.RLock()
	conn, ok := m.conns[serverName]
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("MCP server %q not found or not connected", serverName)
	}
	conn.mu.Lock()
	ready := conn.ready
	conn.mu.Unlock()
	if !ready {
		return "", fmt.Errorf("MCP server %q is not ready", serverName)
	}

	type result struct {
		s   string
		err error
	}
	ch := make(chan result, 1)
	go func() {
		s, err := conn.callTool(toolName, arguments)
		ch <- result{s, err}
	}()
	select {
	case r := <-ch:
		return r.s, r.err
	case <-time.After(mcpCallToolTimeout):
		// Close the connection to release the blocked goroutine and prevent
		// future callers from using a potentially corrupted connection.
		go conn.close()
		m.mu.Lock()
		delete(m.conns, serverName)
		m.mu.Unlock()
		return "", fmt.Errorf("MCP tool call timed out after %s (server=%s, tool=%s) â connection closed", mcpCallToolTimeout, serverName, toolName)
	}
}

// Close shuts down all MCP server connections.
func (m *MCPManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, conn := range m.conns {
		m.logger.Info("[MCP] Stopping server", "name", name)
		conn.close()
	}
	m.conns = make(map[string]*mcpConn)
}

// MCPListTools is a package-level shorthand for agent dispatch.
func MCPListTools(serverName string, logger *slog.Logger) ([]MCPToolInfo, error) {
	mgr := GetMCPManager()
	if mgr == nil {
		return nil, fmt.Errorf("MCP manager not initialized")
	}
	return mgr.ListTools(serverName), nil
}

// MCPCallTool is a package-level shorthand for agent dispatch.
func MCPCallTool(serverName, toolName string, arguments map[string]interface{}, logger *slog.Logger) (string, error) {
	mgr := GetMCPManager()
	if mgr == nil {
		return "", fmt.Errorf("MCP manager not initialized")
	}
	logger.Info("[MCP] Tool call", "server", serverName, "tool", toolName)
	result, err := mgr.CallTool(serverName, toolName, arguments)
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
