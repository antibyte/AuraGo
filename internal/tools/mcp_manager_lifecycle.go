package tools

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

func mcpConfigured(srv MCPServerConfig) bool {
	return srv.Enabled && srv.Name != "" && (strings.TrimSpace(srv.Command) != "" || mcpUsesNetworkTransport(srv) || (mcpRuntimeMode(srv.Runtime) == "docker" && strings.TrimSpace(srv.DockerCommand) != ""))
}

// RevokeMCPManager invalidates the current generation without waiting for I/O.
func RevokeMCPManager() uint64 {
	mcpManagerMu.Lock()
	mcpManagerGeneration++
	generation := mcpManagerGeneration
	old := globalMCPManager
	globalMCPManager = nil
	var connections map[string]*mcpConn
	if old != nil {
		connections = old.revoke()
	}
	mcpManagerMu.Unlock()
	go closeMCPConnections(connections)
	return generation
}

// InitMCPManagerGeneration only publishes a manager for the reserved generation.
func InitMCPManagerGeneration(servers []MCPServerConfig, logger *slog.Logger, generation uint64) *MCPManager {
	ctx, cancel := context.WithCancel(context.Background())
	mgr := &MCPManager{conns: make(map[string]*mcpConn), configs: make(map[string]MCPServerConfig), logger: logger, lifetime: ctx, cancel: cancel}
	for _, srv := range servers {
		if mcpConfigured(srv) {
			mgr.configs[srv.Name] = srv
		}
	}
	mcpManagerMu.Lock()
	if generation != mcpManagerGeneration {
		mcpManagerMu.Unlock()
		mgr.Close()
		return mgr
	}
	globalMCPManager = mgr
	mcpManagerMu.Unlock()
	mgr.ensureConfiguredServersConnected()
	return mgr
}

func (m *MCPManager) callContext(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := mcpRequestContext(parent)
	m.mu.Lock()
	if m.lifetime == nil {
		m.lifetime, m.cancel = context.WithCancel(context.Background())
	}
	lifetime, closed := m.lifetime, m.closed
	m.mu.Unlock()
	stop := context.AfterFunc(lifetime, cancel)
	if closed {
		cancel()
	}
	return ctx, func() { stop(); cancel() }
}

func mcpRequestContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); ok {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, mcpCallToolTimeout)
}

func (m *MCPManager) revoke() map[string]*mcpConn {
	m.mu.Lock()
	m.closed = true
	connections := m.conns
	m.conns = make(map[string]*mcpConn)
	cancel := m.cancel
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return connections
}

func closeMCPConnections(connections map[string]*mcpConn) {
	for _, conn := range connections {
		closeManagedMCPConn(conn)
	}
}

func (m *MCPManager) connect(ctx context.Context, name string) (*mcpConn, error) {
	ctx, cancel := m.callContext(ctx)
	defer cancel()
	for {
		m.mu.Lock()
		if m.closed || ctx.Err() != nil {
			m.mu.Unlock()
			return nil, fmt.Errorf("MCP manager closed or request cancelled: %w", context.Canceled)
		}
		if conn := m.conns[name]; conn != nil {
			conn.mu.Lock()
			ready := conn.ready
			conn.mu.Unlock()
			if ready {
				m.mu.Unlock()
				return conn, nil
			}
		}
		cfg, ok := m.configs[name]
		if !ok || !mcpConfigured(cfg) {
			m.mu.Unlock()
			return nil, fmt.Errorf("MCP server %q not found or not connected", name)
		}
		if wait := m.connecting[name]; wait != nil {
			m.mu.Unlock()
			select {
			case <-wait:
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		if m.connecting == nil {
			m.connecting = make(map[string]chan struct{})
		}
		done := make(chan struct{})
		m.connecting[name] = done
		m.mu.Unlock()
		conn, err := startManagedMCPConn(ctx, cfg, m.logger)
		m.mu.Lock()
		delete(m.connecting, name)
		close(done)
		if m.closed || ctx.Err() != nil {
			m.mu.Unlock()
			if conn != nil {
				closeManagedMCPConn(conn)
			}
			return nil, fmt.Errorf("MCP connection cancelled: %w", context.Canceled)
		}
		var old *mcpConn
		if err == nil {
			old = m.conns[name]
			m.conns[name] = conn
		}
		m.mu.Unlock()
		if old != nil {
			closeManagedMCPConn(old)
		}
		return conn, err
	}
}
