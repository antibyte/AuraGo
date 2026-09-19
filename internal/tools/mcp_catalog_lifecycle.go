package tools

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// Notifications only advance a generation. Readers refresh outside transport
// locks; a notification received during pagination is never accidentally cleared.
type mcpNotificationState struct{ toolsGeneration atomic.Uint64 }

func (s *mcpNotificationState) observeNotification(response *jsonRPCResponse) {
	if response.ID == nil && response.Method == "notifications/tools/list_changed" {
		s.toolsGeneration.Add(1)
	}
}

func (s *mcpNotificationState) ToolsGeneration() uint64 { return s.toolsGeneration.Load() }

func mcpToolsGeneration(transport mcpTransport) uint64 {
	if source, ok := transport.(interface{ ToolsGeneration() uint64 }); ok {
		return source.ToolsGeneration()
	}
	return 0
}

func (c *mcpConn) refreshToolsIfNeeded(ctx context.Context, logger *slog.Logger) error {
	if c.transport == nil {
		return nil
	} // Test adapters supply a prebuilt snapshot.
	c.discoveryMu.Lock()
	defer c.discoveryMu.Unlock()
	c.mu.Lock()
	stale := c.toolsGeneration != mcpToolsGeneration(c.transport) || time.Since(c.toolsRefreshed) > time.Minute
	c.mu.Unlock()
	if stale {
		return c.discoverToolsLocked(ctx, logger)
	}
	return nil
}
