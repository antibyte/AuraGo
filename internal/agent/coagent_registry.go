package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"
)

// CoAgentState describes the lifecycle status of a co-agent.
type CoAgentState string

const (
	CoAgentQueued    CoAgentState = "queued"
	CoAgentRunning   CoAgentState = "running"
	CoAgentCompleted CoAgentState = "completed"
	CoAgentFailed    CoAgentState = "failed"
	CoAgentCancelled CoAgentState = "cancelled"
)

// CoAgentEvent holds a small, user-visible lifecycle event for observability.
type CoAgentEvent struct {
	At      time.Time `json:"at"`
	Message string    `json:"message"`
}

// CoAgentInfo holds metadata for a running or finished co-agent.
type CoAgentInfo struct {
	ID            string
	Task          string
	Specialist    string // Specialist role ("researcher","coder", etc.) or empty for generic
	State         CoAgentState
	StartedAt     time.Time
	CompletedAt   time.Time
	Result        string
	Error         string
	TokensUsed    int
	ToolCalls     int
	Cancel        context.CancelFunc
	Priority      int
	RetryCount    int
	QueuePosition int
	LastEvent     string
	LastError     string
	Events        []CoAgentEvent

	startCh   chan struct{}
	startOnce sync.Once
	mu        sync.Mutex
}

// Runtime returns the elapsed wall-clock time of this co-agent.
func (c *CoAgentInfo) Runtime() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch c.State {
	case CoAgentRunning, CoAgentQueued:
		return time.Since(c.StartedAt)
	default:
		if c.CompletedAt.IsZero() {
			return time.Since(c.StartedAt)
		}
		return c.CompletedAt.Sub(c.StartedAt)
	}
}

func (c *CoAgentInfo) recordEvent(message string) {
	if strings.TrimSpace(message) == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.LastEvent = message
	c.Events = append(c.Events, CoAgentEvent{
		At:      time.Now(),
		Message: message,
	})
	if len(c.Events) > 8 {
		c.Events = c.Events[len(c.Events)-8:]
	}
}

func (c *CoAgentInfo) signalStart() {
	c.startOnce.Do(func() {
		if c.startCh != nil {
			close(c.startCh)
		}
	})
}

func normalizeCoAgentPriority(priority int) int {
	switch {
	case priority <= 1:
		return 1
	case priority >= 3:
		return 3
	default:
		return 2
	}
}

// CoAgentRegistry is a thread-safe registry for all co-agent goroutines.
type CoAgentRegistry struct {
	mu              sync.RWMutex
	agents          map[string]*CoAgentInfo
	counter         int
	maxSlots        int
	logger          *slog.Logger
	cleanupInterval time.Duration
	cleanupMaxAge   time.Duration
}

// NewCoAgentRegistry creates a new registry with the given slot limit.
func NewCoAgentRegistry(maxSlots int, logger *slog.Logger) *CoAgentRegistry {
	if maxSlots <= 0 {
		maxSlots = 3
	}
	return &CoAgentRegistry{
		agents:          make(map[string]*CoAgentInfo),
		maxSlots:        maxSlots,
		logger:          logger,
		cleanupInterval: 10 * time.Minute,
		cleanupMaxAge:   30 * time.Minute,
	}
}

// ConfigureLifecycle updates cleanup timing settings.
func (r *CoAgentRegistry) ConfigureLifecycle(interval, maxAge time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if interval > 0 {
		r.cleanupInterval = interval
	}
	if maxAge > 0 {
		r.cleanupMaxAge = maxAge
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
	r.promoteQueuedLocked()
	r.mu.Unlock()
}

func (r *CoAgentRegistry) countRunningLocked() int {
	running := 0
	for _, a := range r.agents {
		a.mu.Lock()
		isRunning := a.State == CoAgentRunning
		a.mu.Unlock()
		if isRunning {
			running++
		}
	}
	return running
}

func (r *CoAgentRegistry) queuedAgentsLocked() []*CoAgentInfo {
	queued := make([]*CoAgentInfo, 0)
	for _, a := range r.agents {
		a.mu.Lock()
		isQueued := a.State == CoAgentQueued
		a.mu.Unlock()
		if isQueued {
			queued = append(queued, a)
		}
	}
	sort.SliceStable(queued, func(i, j int) bool {
		queued[i].mu.Lock()
		pi, si := queued[i].Priority, queued[i].StartedAt
		queued[i].mu.Unlock()
		queued[j].mu.Lock()
		pj, sj := queued[j].Priority, queued[j].StartedAt
		queued[j].mu.Unlock()
		if pi != pj {
			return pi > pj
		}
		return si.Before(sj)
	})
	return queued
}

func (r *CoAgentRegistry) refreshQueuePositionsLocked() {
	queued := r.queuedAgentsLocked()
	for idx, a := range queued {
		a.mu.Lock()
		a.QueuePosition = idx + 1
		a.mu.Unlock()
	}
	for _, a := range r.agents {
		a.mu.Lock()
		if a.State != CoAgentQueued {
			a.QueuePosition = 0
		}
		a.mu.Unlock()
	}
}

func (r *CoAgentRegistry) promoteQueuedLocked() {
	for r.countRunningLocked() < r.maxSlots {
		queued := r.queuedAgentsLocked()
		if len(queued) == 0 {
			break
		}
		next := queued[0]
		next.mu.Lock()
		if next.State != CoAgentQueued {
			next.mu.Unlock()
			continue
		}
		next.State = CoAgentRunning
		next.QueuePosition = 0
		next.mu.Unlock()
		next.recordEvent("started after queue")
		next.signalStart()
		r.logger.Info("Co-Agent promoted from queue", "id", next.ID)
	}
	r.refreshQueuePositionsLocked()
}

// AvailableSlots returns the number of free co-agent slots.
func (r *CoAgentRegistry) AvailableSlots() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.maxSlots - r.countRunningLocked()
}

// Register creates a new co-agent entry and returns its ID.
// Returns an error if all slots are occupied.
func (r *CoAgentRegistry) Register(task string, cancel context.CancelFunc) (string, error) {
	return r.RegisterWithPrefix("coagent", task, cancel)
}

// RegisterWithPrefix creates a new co-agent entry with a custom ID prefix.
// The prefix determines the session ID used for tool blacklist filtering.
func (r *CoAgentRegistry) RegisterWithPrefix(prefix, task string, cancel context.CancelFunc) (string, error) {
	id, _, err := r.RegisterWithPriority(prefix, task, cancel, 2)
	return id, err
}

// RegisterWithPriority creates a new co-agent entry and either starts it immediately
// or queues it when no slots are available.
func (r *CoAgentRegistry) RegisterWithPriority(prefix, task string, cancel context.CancelFunc, priority int) (string, CoAgentState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.counter++
	id := fmt.Sprintf("%s-%d", prefix, r.counter)

	specialist := ""
	if prefix != "coagent" {
		if after, found := strings.CutPrefix(prefix, "specialist-"); found {
			specialist = after
		}
	}

	state := CoAgentRunning
	if r.countRunningLocked() >= r.maxSlots {
		state = CoAgentQueued
	}

	info := &CoAgentInfo{
		ID:         id,
		Task:       task,
		Specialist: specialist,
		State:      state,
		StartedAt:  time.Now(),
		Cancel:     cancel,
		Priority:   normalizeCoAgentPriority(priority),
		startCh:    make(chan struct{}),
	}
	if state == CoAgentRunning {
		info.signalStart()
		info.recordEvent("started")
	} else {
		info.recordEvent("queued")
	}
	r.agents[id] = info
	r.refreshQueuePositionsLocked()
	r.logger.Info("Co-Agent registered", "id", id, "specialist", specialist, "state", state, "priority", info.Priority, "task", truncateStr(task, 80))
	return id, state, nil
}

// WaitForStart blocks until a queued co-agent gets a slot.
func (r *CoAgentRegistry) WaitForStart(id string, ctx context.Context) error {
	r.mu.RLock()
	a, ok := r.agents[id]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("co-agent '%s' not found", id)
	}

	a.mu.Lock()
	state := a.State
	startCh := a.startCh
	a.mu.Unlock()

	if state != CoAgentQueued {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-startCh:
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	switch a.State {
	case CoAgentRunning:
		return nil
	case CoAgentCancelled:
		return fmt.Errorf("co-agent '%s' was cancelled before start", id)
	case CoAgentFailed:
		return fmt.Errorf("co-agent '%s' failed before start: %s", id, a.Error)
	default:
		return nil
	}
}

// RecordEvent appends an observable lifecycle event to a co-agent.
func (r *CoAgentRegistry) RecordEvent(id, message string) {
	r.mu.RLock()
	a := r.agents[id]
	r.mu.RUnlock()
	if a == nil {
		return
	}
	a.recordEvent(message)
}

// RecordRetry increments the retry counter and stores the latest transient error.
func (r *CoAgentRegistry) RecordRetry(id, errMsg string) {
	r.mu.RLock()
	a := r.agents[id]
	r.mu.RUnlock()
	if a == nil {
		return
	}
	a.mu.Lock()
	a.RetryCount++
	a.LastError = errMsg
	a.mu.Unlock()
	a.recordEvent("retry scheduled")
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
		a.recordEvent("completed")
		r.promoteQueuedLocked()
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
		a.LastError = errMsg
		a.TokensUsed = tokensUsed
		a.ToolCalls = toolCalls
		a.mu.Unlock()
		a.recordEvent("failed")
		a.signalStart()
		r.promoteQueuedLocked()
	}
}

// Stop cancels a running or queued co-agent.
func (r *CoAgentRegistry) Stop(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.agents[id]
	if !ok {
		return fmt.Errorf("co-agent '%s' not found", id)
	}
	a.mu.Lock()
	if a.State != CoAgentRunning && a.State != CoAgentQueued {
		state := a.State
		a.mu.Unlock()
		return fmt.Errorf("co-agent '%s' is not active (state: %s)", id, state)
	}
	a.State = CoAgentCancelled
	a.CompletedAt = time.Now()
	a.mu.Unlock()
	if a.Cancel != nil {
		a.Cancel()
	}
	a.signalStart()
	a.recordEvent("cancelled")
	r.promoteQueuedLocked()
	r.logger.Info("Co-Agent stopped", "id", id)
	return nil
}

// StopAll cancels all active co-agents.
func (r *CoAgentRegistry) StopAll() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, a := range r.agents {
		a.mu.Lock()
		active := a.State == CoAgentRunning || a.State == CoAgentQueued
		if active {
			a.State = CoAgentCancelled
			a.CompletedAt = time.Now()
		}
		a.mu.Unlock()
		if active {
			if a.Cancel != nil {
				a.Cancel()
			}
			a.signalStart()
			a.recordEvent("cancelled")
			count++
		}
	}
	r.promoteQueuedLocked()
	r.logger.Info("All co-agents stopped", "count", count)
	return count
}

// List returns a summary of all co-agents (for Tool Output).
func (r *CoAgentRegistry) List() []map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]map[string]interface{}, 0, len(r.agents))
	for _, a := range r.agents {
		a.mu.Lock()
		runtime := time.Since(a.StartedAt)
		if a.State != CoAgentRunning && a.State != CoAgentQueued && !a.CompletedAt.IsZero() {
			runtime = a.CompletedAt.Sub(a.StartedAt)
		}
		entry := map[string]interface{}{
			"id":             a.ID,
			"task":           truncateStr(a.Task, 120),
			"specialist":     a.Specialist,
			"state":          string(a.State),
			"started_at":     a.StartedAt.Format(time.RFC3339),
			"runtime":        fmt.Sprintf("%.1fs", runtime.Seconds()),
			"tokens_used":    a.TokensUsed,
			"tool_calls":     a.ToolCalls,
			"priority":       a.Priority,
			"retry_count":    a.RetryCount,
			"queue_position": a.QueuePosition,
			"last_event":     a.LastEvent,
		}
		if a.LastError != "" {
			entry["last_error"] = a.LastError
		}
		if len(a.Events) > 0 {
			events := make([]map[string]string, 0, len(a.Events))
			for _, ev := range a.Events {
				events = append(events, map[string]string{
					"at":      ev.At.Format(time.RFC3339),
					"message": ev.Message,
				})
			}
			entry["recent_events"] = events
		}
		if a.State == CoAgentCompleted {
			entry["result_preview"] = truncateStr(a.Result, 200)
		}
		if a.State == CoAgentFailed {
			entry["error"] = a.Error
		}
		a.mu.Unlock()
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
	a.mu.Lock()
	defer a.mu.Unlock()
	switch a.State {
	case CoAgentQueued:
		return "", fmt.Errorf("co-agent '%s' is still queued (position %d)", id, a.QueuePosition)
	case CoAgentRunning:
		return "", fmt.Errorf("co-agent '%s' is still running (%.0fs elapsed)", id, time.Since(a.StartedAt).Seconds())
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
		a.mu.Lock()
		removable := a.State != CoAgentRunning && a.State != CoAgentQueued && !a.CompletedAt.IsZero() && time.Since(a.CompletedAt) > maxAge
		a.mu.Unlock()
		if removable {
			delete(r.agents, id)
			count++
		}
	}
	return count
}

// StartCleanupLoop runs a background goroutine that periodically removes stale entries.
func (r *CoAgentRegistry) StartCleanupLoop() {
	go func() {
		for {
			r.mu.RLock()
			interval := r.cleanupInterval
			maxAge := r.cleanupMaxAge
			r.mu.RUnlock()
			if interval <= 0 {
				interval = 10 * time.Minute
			}
			timer := time.NewTimer(interval)
			<-timer.C
			if n := r.Cleanup(maxAge); n > 0 {
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
