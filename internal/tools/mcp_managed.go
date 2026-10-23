package tools

import (
	"context"
	"fmt"
	"log/slog"
)

// SetManagedMCPServer replaces one owner-managed connection, preserving user servers.
// A nil config revokes it, including calls already using its transport.
func SetManagedMCPServer(name string, cfg *MCPServerConfig) error {
	m := GetMCPManager()
	if m == nil {
		return nil
	}
	m.mu.Lock()
	if old, exists := m.configs[name]; exists && old.ManagedRevision == "" {
		m.mu.Unlock()
		return fmt.Errorf("MCP server %q is user-managed", name)
	}
	if m.closed {
		m.mu.Unlock()
		return fmt.Errorf("MCP manager is closed")
	}
	if cfg != nil && (cfg.Name != name || cfg.ManagedRevision == "") {
		m.mu.Unlock()
		return fmt.Errorf("invalid managed MCP configuration")
	}
	old := m.conns[name]
	delete(m.conns, name)
	delete(m.configs, name)
	if cfg != nil {
		m.configs[name] = *cfg
	}
	m.mu.Unlock()
	if old != nil {
		closeManagedMCPConn(old)
	}
	return nil
}

// ProbeMCPServerTools performs only the existing initialization and discovery handshake.
func ProbeMCPServerTools(ctx context.Context, cfg MCPServerConfig, logger *slog.Logger) ([]string, error) {
	conn, err := startMCPServerConnection(ctx, cfg, logger)
	if err != nil {
		return nil, err
	}
	defer closeManagedMCPConn(conn)
	conn.mu.Lock()
	defer conn.mu.Unlock()
	names := make([]string, 0, len(conn.tools))
	for _, tool := range conn.tools {
		names = append(names, tool.Name)
	}
	return names, nil
}
