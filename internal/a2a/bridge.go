package a2a

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"aurago/internal/agent"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

// Bridge connects the co-agent system to remote A2A agents.
type Bridge struct {
	clientMgr  *ClientManager
	coRegistry *agent.CoAgentRegistry
	logger     *slog.Logger
}

// NewBridge creates a new A2A co-agent bridge.
func NewBridge(clientMgr *ClientManager, coRegistry *agent.CoAgentRegistry, logger *slog.Logger) *Bridge {
	return &Bridge{
		clientMgr:  clientMgr,
		coRegistry: coRegistry,
		logger:     logger.With("component", "a2a-bridge"),
	}
}

// SpawnRemote sends a task to a remote A2A agent and tracks it in the co-agent registry.
func (b *Bridge) SpawnRemote(ctx context.Context, agentID string, task string) (string, error) {
	if !b.clientMgr.IsAvailable(agentID) {
		return "", fmt.Errorf("remote A2A agent %q is not available", agentID)
	}

	// Register in co-agent registry for unified tracking
	coID, err := b.coRegistry.Register(fmt.Sprintf("[A2A:%s] %s", agentID, task), nil)
	if err != nil {
		return "", fmt.Errorf("failed to register remote A2A task: %w", err)
	}

	go func() {
		b.logger.Info("A2A remote task started", "co_id", coID, "agent", agentID, "task_len", len(task))

		result, err := b.clientMgr.SendMessage(ctx, agentID, task)
		if err != nil {
			b.logger.Error("A2A remote task failed", "co_id", coID, "error", err)
			b.coRegistry.Fail(coID, err.Error(), 0, 0)
			return
		}

		// Extract text from result
		resultText := extractResultText(result)
		b.logger.Info("A2A remote task completed", "co_id", coID, "result_len", len(resultText))
		b.coRegistry.Complete(coID, resultText, 0, 0)
	}()

	return coID, nil
}

// ListRemoteAgents returns a summary of available remote agents.
func (b *Bridge) ListRemoteAgents() []RemoteAgentStatus {
	return b.clientMgr.ListRemoteAgents()
}

// extractResultText extracts text from an A2A SendMessageResult.
func extractResultText(result a2a.SendMessageResult) string {
	switch r := result.(type) {
	case *a2a.Task:
		// Extract from task artifacts or status message
		var parts []string
		for _, art := range r.Artifacts {
			for _, p := range art.Parts {
				if t, ok := p.Content.(a2a.Text); ok {
					parts = append(parts, string(t))
				}
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n")
		}
		if r.Status.Message != nil {
			return extractTextFromMessage(r.Status.Message)
		}
		return fmt.Sprintf("Task %s completed with state: %s", r.ID, r.Status.State)
	case *a2a.Message:
		return extractTextFromMessage(r)
	default:
		return "Unknown result type"
	}
}
