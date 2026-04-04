package agent

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type StallGuardConfig struct {
	Enabled          bool
	IdleTimeoutSecs  int
	MaxContinuations int
	MinToolCalls     int
}

type StallGuardContinuationFunc func(sessionID string, continuationPrompt string) error

type stallGuardSession struct {
	toolCallCount     int
	continuationCount int
	timer             *time.Timer
	lastActivity      time.Time
}

type StallGuard struct {
	mu         sync.Mutex
	config     StallGuardConfig
	sessions   map[string]*stallGuardSession
	continueFn StallGuardContinuationFunc
	logger     *slog.Logger
	stopCh     chan struct{}
}

var globalStallGuard *StallGuard

func InitStallGuard(cfg StallGuardConfig, continueFn StallGuardContinuationFunc, logger *slog.Logger) {
	if !cfg.Enabled {
		return
	}
	if cfg.IdleTimeoutSecs <= 0 {
		cfg.IdleTimeoutSecs = 45
	}
	if cfg.MaxContinuations <= 0 {
		cfg.MaxContinuations = 3
	}
	if cfg.MinToolCalls <= 0 {
		cfg.MinToolCalls = 1
	}
	sg := &StallGuard{
		config:     cfg,
		sessions:   make(map[string]*stallGuardSession),
		continueFn: continueFn,
		logger:     logger,
		stopCh:     make(chan struct{}),
	}
	globalStallGuard = sg
	logger.Info("[StallGuard] Initialized", "idle_timeout_secs", cfg.IdleTimeoutSecs, "max_continuations", cfg.MaxContinuations, "min_tool_calls", cfg.MinToolCalls)
}

func StopStallGuard() {
	if globalStallGuard == nil {
		return
	}
	globalStallGuard.stop()
	globalStallGuard = nil
}

func (sg *StallGuard) stop() {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	close(sg.stopCh)
	for _, sess := range sg.sessions {
		if sess.timer != nil {
			sess.timer.Stop()
		}
	}
	sg.sessions = make(map[string]*stallGuardSession)
}

func StallGuardRecordTurnComplete(sessionID string, toolCallCount int) {
	if globalStallGuard == nil || !globalStallGuard.config.Enabled {
		return
	}
	globalStallGuard.recordTurnComplete(sessionID, toolCallCount)
}

func StallGuardRecordUserMessage(sessionID string) {
	if globalStallGuard == nil || !globalStallGuard.config.Enabled {
		return
	}
	globalStallGuard.recordUserMessage(sessionID)
}

func StallGuardReset(sessionID string) {
	if globalStallGuard == nil || !globalStallGuard.config.Enabled {
		return
	}
	globalStallGuard.reset(sessionID)
}

func (sg *StallGuard) recordTurnComplete(sessionID string, toolCallCount int) {
	if toolCallCount < sg.config.MinToolCalls {
		return
	}

	sg.mu.Lock()
	defer sg.mu.Unlock()

	if sess, exists := sg.sessions[sessionID]; exists {
		if sess.timer != nil {
			sess.timer.Stop()
		}
		sess.toolCallCount = toolCallCount
		sess.lastActivity = time.Now()
	} else {
		sg.sessions[sessionID] = &stallGuardSession{
			toolCallCount: toolCallCount,
			lastActivity:  time.Now(),
		}
	}

	sess := sg.sessions[sessionID]
	timeout := time.Duration(sg.config.IdleTimeoutSecs) * time.Second
	sess.timer = time.AfterFunc(timeout, func() {
		sg.triggerContinuation(sessionID)
	})

	sg.logger.Debug("[StallGuard] Timer started", "session", sessionID, "tool_calls", toolCallCount, "timeout", timeout)
}

func (sg *StallGuard) recordUserMessage(sessionID string) {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	if sess, exists := sg.sessions[sessionID]; exists {
		if sess.timer != nil {
			sess.timer.Stop()
		}
		sess.continuationCount = 0
		sess.lastActivity = time.Now()
		sg.logger.Debug("[StallGuard] Timer cancelled by user message", "session", sessionID)
	}
}

func (sg *StallGuard) reset(sessionID string) {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	if sess, exists := sg.sessions[sessionID]; exists {
		if sess.timer != nil {
			sess.timer.Stop()
		}
		delete(sg.sessions, sessionID)
	}
}

func (sg *StallGuard) triggerContinuation(sessionID string) {
	sg.mu.Lock()
	sess, exists := sg.sessions[sessionID]
	if !exists {
		sg.mu.Unlock()
		return
	}

	if sess.continuationCount >= sg.config.MaxContinuations {
		sg.logger.Info("[StallGuard] Max continuations reached, stopping", "session", sessionID, "count", sess.continuationCount)
		delete(sg.sessions, sessionID)
		sg.mu.Unlock()
		return
	}

	sess.continuationCount++
	count := sess.continuationCount
	toolCalls := sess.toolCallCount
	sess.lastActivity = time.Now()
	sg.mu.Unlock()

	sg.logger.Info("[StallGuard] Triggering continuation check", "session", sessionID, "continuation", count, "previous_tool_calls", toolCalls)

	prompt := fmt.Sprintf(
		"[SYSTEM: STALL GUARD - CONTINUATION CHECK %d/%d]\n"+
			"Your previous turn completed after %d tool calls. "+
			"The stall guard detected that your work may be incomplete.\n\n"+
			"Review the conversation context above:\n"+
			"1. Were ALL steps of the requested task fully completed?\n"+
			"2. Did any tool call fail or return an error that was not resolved?\n"+
			"3. Were there announced next steps that were never executed?\n\n"+
			"If any work remains unfinished: Continue with the next step immediately.\n"+
			"If all tasks are complete: Briefly confirm that the work is done.\n"+
			"Do NOT ask the user for confirmation — act autonomously.",
		count, sg.config.MaxContinuations, toolCalls,
	)

	if sg.continueFn != nil {
		if err := sg.continueFn(sessionID, prompt); err != nil {
			sg.logger.Error("[StallGuard] Continuation failed", "session", sessionID, "error", err)
		}
	}

	sg.mu.Lock()
	defer sg.mu.Unlock()
	if sess, still := sg.sessions[sessionID]; still && sess.continuationCount < sg.config.MaxContinuations {
		timeout := time.Duration(sg.config.IdleTimeoutSecs) * time.Second
		sess.timer = time.AfterFunc(timeout, func() {
			sg.triggerContinuation(sessionID)
		})
	} else if still {
		sg.logger.Info("[StallGuard] All continuation attempts exhausted", "session", sessionID)
		delete(sg.sessions, sessionID)
	}
}
