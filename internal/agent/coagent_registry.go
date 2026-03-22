package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// CoAgentState describes the lifecycle status of a co-agent.
type CoAgentState string

const (
	CoAgentRunning   CoAgentState = "running"
	CoAgentCompleted CoAgentState = "completed"
	CoAgentFailed    CoAgentState = "failed"
	CoAgentCancelled CoAgentState = "cancelled"
)

// CoAgentInfo holds metadata for a running or finished co-agent.
type CoAgentInfo struct {
	ID          string
	Task        string
	Specialist  string // Specialist role ("researcher","coder", etc.) or empty for generic
	State       CoAgentState
	StartedAt   time.Time
	CompletedAt time.Time
	Result      string
	Error       string
	TokensUsed  int
	ToolCalls   int
	Cancel      context.CancelFunc
	mu          sync.Mutex
}

// Runtime returns the elapsed wall-clock time of this co-agent.
func (c *CoAgentInfo) Runtime() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.State == CoAgentRunning {
		return time.Since(c.StartedAt)
	}
	return c.CompletedAt.Sub(c.StartedAt)
}

// CoAgentRegistry is a thread-safe registry for all co-agent goroutines.
type CoAgentRegistry struct {
	mu       sync.RWMutex
	agents   map[string]*CoAgentInfo
	counter  int
	maxSlots int
	logger   *slog.Logger
}

// NewCoAgentRegistry creates a new registry with the given slot limit.
func NewCoAgentRegistry(maxSlots int, logger *slog.Logger) *CoAgentRegistry {
	return &CoAgentRegistry{
		agents:   make(map[string]*CoAgentInfo),
		maxSlots: maxSlots,
		logger:   logger,
	}
}

// SetMaxSlots updates the maximum number of concurrent co-agents.
// Safe to call at any time; takes effect immediately for new registrations.
func (r *CoAgentRegistry) SetMaxSlots(n int) {
	if n <= 0 {
		return
	}
	r.mu.Lock()
	r.maxSlots = n
	r.mu.Unlock()
}

// AvailableSlots returns the number of free co-agent slots.
func (r *CoAgentRegistry) AvailableSlots() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	running := 0
	for _, a := range r.agents {
		if a.State == CoAgentRunning {
			running++
		}
	}
	return r.maxSlots - running
}

// Register creates a new co-agent entry and returns its ID.
// Returns an error if all slots are occupied.
func (r *CoAgentRegistry) Register(task string, cancel context.CancelFunc) (string, error) {
	return r.RegisterWithPrefix("coagent", task, cancel)
}

// RegisterWithPrefix creates a new co-agent entry with a custom ID prefix.
// The prefix determines the session ID used for tool blacklist filtering.
func (r *CoAgentRegistry) RegisterWithPrefix(prefix, task string, cancel context.CancelFunc) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	running := 0
	for _, a := range r.agents {
		if a.State == CoAgentRunning {
			running++
		}
	}
	if running >= r.maxSlots {
		return "", fmt.Errorf("all %d co-agent slots are occupied", r.maxSlots)
	}

	r.counter++
	id := fmt.Sprintf("%s-%d", prefix, r.counter)

	// Derive specialist role from prefix
	specialist := ""
	if prefix != "coagent" {
		// prefix is "specialist-<role>", extract the role
		if after, found := strings.CutPrefix(prefix, "specialist-"); found {
			specialist = after
		}
	}

	r.agents[id] = &CoAgentInfo{
		ID:         id,
		Task:       task,
		Specialist: specialist,
		State:      CoAgentRunning,
		StartedAt:  time.Now(),
		Cancel:     cancel,
	}
	r.logger.Info("Co-Agent registered", "id", id, "specialist", specialist, "task", truncateStr(task, 80))
	return id, nil
}

// Complete marks a co-agent as successfully finished.
func (r *CoAgentRegistry) Complete(id, result string, tokensUsed, toolCalls int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a, ok := r.agents[id]; ok {
		a.mu.Lock()
		a.State = CoAgentCompleted
		a.CompletedAt = time.Now()
		a.Result = result
		a.TokensUsed = tokensUsed
		a.ToolCalls = toolCalls
		a.mu.Unlock()
	}
}

// Fail marks a co-agent as failed with an error message.
func (r *CoAgentRegistry) Fail(id, errMsg string, tokensUsed, toolCalls int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a, ok := r.agents[id]; ok {
		a.mu.Lock()
		a.State = CoAgentFailed
		a.CompletedAt = time.Now()
		a.Error = errMsg
		a.TokensUsed = tokensUsed
		a.ToolCalls = toolCalls
		a.mu.Unlock()
	}
}

// Stop cancels a running co-agent.
func (r *CoAgentRegistry) Stop(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.agents[id]
	if !ok {
		return fmt.Errorf("co-agent '%s' not found", id)
	}
	if a.State != CoAgentRunning {
		return fmt.Errorf("co-agent '%s' is not running (state: %s)", id, a.State)
	}
	a.Cancel()
	a.mu.Lock()
	a.State = CoAgentCancelled
	a.CompletedAt = time.Now()
	a.mu.Unlock()
	r.logger.Info("Co-Agent stopped", "id", id)
	return nil
}

// StopAll cancels all running co-agents.
func (r *CoAgentRegistry) StopAll() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, a := range r.agents {
		if a.State == CoAgentRunning {
			a.Cancel()
			a.mu.Lock()
			a.State = CoAgentCancelled
			a.CompletedAt = time.Now()
			a.mu.Unlock()
			count++
		}
	}
	r.logger.Info("All co-agents stopped", "count", count)
	return count
}

// List returns a summary of all co-agents (for Tool Output).
func (r *CoAgentRegistry) List() []map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]map[string]interface{}, 0, len(r.agents))
	for _, a := range r.agents {
		entry := map[string]interface{}{
			"id":          a.ID,
			"task":        truncateStr(a.Task, 120),
			"specialist":  a.Specialist,
			"state":       string(a.State),
			"started_at":  a.StartedAt.Format(time.RFC3339),
			"runtime":     fmt.Sprintf("%.1fs", a.Runtime().Seconds()),
			"tokens_used": a.TokensUsed,
			"tool_calls":  a.ToolCalls,
		}
		if a.State == CoAgentCompleted {
			entry["result_preview"] = truncateStr(a.Result, 200)
		}
		if a.State == CoAgentFailed {
			entry["error"] = a.Error
		}
		result = append(result, entry)
	}
	return result
}

// GetResult returns the full result of a completed co-agent.
func (r *CoAgentRegistry) GetResult(id string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.agents[id]
	if !ok {
		return "", fmt.Errorf("co-agent '%s' not found", id)
	}
	switch a.State {
	case CoAgentRunning:
		return "", fmt.Errorf("co-agent '%s' is still running (%.0fs elapsed)", id, a.Runtime().Seconds())
	case CoAgentCompleted:
		return a.Result, nil
	case CoAgentFailed:
		return "", fmt.Errorf("co-agent '%s' failed: %s", id, a.Error)
	case CoAgentCancelled:
		return "", fmt.Errorf("co-agent '%s' was cancelled", id)
	}
	return "", fmt.Errorf("unknown state")
}

// Cleanup removes finished entries older than maxAge.
func (r *CoAgentRegistry) Cleanup(maxAge time.Duration) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for id, a := range r.agents {
		if a.State != CoAgentRunning && time.Since(a.CompletedAt) > maxAge {
			delete(r.agents, id)
			count++
		}
	}
	return count
}

// StartCleanupLoop runs a background goroutine that periodically removes stale entries.
func (r *CoAgentRegistry) StartCleanupLoop() {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if n := r.Cleanup(30 * time.Minute); n > 0 {
				r.logger.Debug("Co-Agent registry cleanup", "removed", n)
			}
		}
	}()
}

// truncateStr truncates a string to maxLen, adding "…" if truncated.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}
