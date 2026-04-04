package agent

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestStallGuard_RecordTurnCompleteStartsTimer(t *testing.T) {
	var triggered atomic.Int32
	sg := &StallGuard{
		config: StallGuardConfig{
			Enabled:          true,
			IdleTimeoutSecs:  1,
			MaxContinuations: 3,
			MinToolCalls:     1,
		},
		sessions: make(map[string]*stallGuardSession),
		continueFn: func(sessionID string, continuationPrompt string) error {
			triggered.Add(1)
			return nil
		},
		stopCh: make(chan struct{}),
	}

	sg.recordTurnComplete("test-session", 3)
	if _, exists := sg.sessions["test-session"]; !exists {
		t.Fatal("expected session to be created")
	}

	time.Sleep(1500 * time.Millisecond)
	if triggered.Load() != 1 {
		t.Fatalf("expected 1 continuation trigger, got %d", triggered.Load())
	}
}

func TestStallGuard_UserMessageCancelsTimer(t *testing.T) {
	var triggered atomic.Int32
	sg := &StallGuard{
		config: StallGuardConfig{
			Enabled:          true,
			IdleTimeoutSecs:  1,
			MaxContinuations: 3,
			MinToolCalls:     1,
		},
		sessions: make(map[string]*stallGuardSession),
		continueFn: func(sessionID string, continuationPrompt string) error {
			triggered.Add(1)
			return nil
		},
		stopCh: make(chan struct{}),
	}

	sg.recordTurnComplete("test-session", 2)
	sg.recordUserMessage("test-session")

	time.Sleep(1500 * time.Millisecond)
	if triggered.Load() != 0 {
		t.Fatalf("expected 0 triggers after user message, got %d", triggered.Load())
	}
}

func TestStallGuard_MinToolCallsFilter(t *testing.T) {
	var triggered atomic.Int32
	sg := &StallGuard{
		config: StallGuardConfig{
			Enabled:          true,
			IdleTimeoutSecs:  1,
			MaxContinuations: 3,
			MinToolCalls:     2,
		},
		sessions: make(map[string]*stallGuardSession),
		continueFn: func(sessionID string, continuationPrompt string) error {
			triggered.Add(1)
			return nil
		},
		stopCh: make(chan struct{}),
	}

	sg.recordTurnComplete("test-session", 1)

	time.Sleep(1500 * time.Millisecond)
	if triggered.Load() != 0 {
		t.Fatalf("expected 0 triggers with insufficient tool calls, got %d", triggered.Load())
	}
}

func TestStallGuard_MaxContinuations(t *testing.T) {
	var triggered atomic.Int32
	sg := &StallGuard{
		config: StallGuardConfig{
			Enabled:          true,
			IdleTimeoutSecs:  1,
			MaxContinuations: 2,
			MinToolCalls:     1,
		},
		sessions: make(map[string]*stallGuardSession),
		continueFn: func(sessionID string, continuationPrompt string) error {
			triggered.Add(1)
			return nil
		},
		stopCh: make(chan struct{}),
	}

	sg.recordTurnComplete("test-session", 3)
	time.Sleep(5500 * time.Millisecond)

	count := triggered.Load()
	if count > 2 {
		t.Fatalf("expected at most 2 continuation triggers, got %d", count)
	}
}

func TestStallGuard_Reset(t *testing.T) {
	var triggered atomic.Int32
	sg := &StallGuard{
		config: StallGuardConfig{
			Enabled:          true,
			IdleTimeoutSecs:  1,
			MaxContinuations: 3,
			MinToolCalls:     1,
		},
		sessions: make(map[string]*stallGuardSession),
		continueFn: func(sessionID string, continuationPrompt string) error {
			triggered.Add(1)
			return nil
		},
		stopCh: make(chan struct{}),
	}

	sg.recordTurnComplete("test-session", 2)
	sg.reset("test-session")

	if _, exists := sg.sessions["test-session"]; exists {
		t.Fatal("expected session to be removed after reset")
	}

	time.Sleep(1500 * time.Millisecond)
	if triggered.Load() != 0 {
		t.Fatalf("expected 0 triggers after reset, got %d", triggered.Load())
	}
}

func TestStallGuard_ContinuationPrompt(t *testing.T) {
	var capturedPrompt string
	sg := &StallGuard{
		config: StallGuardConfig{
			Enabled:          true,
			IdleTimeoutSecs:  1,
			MaxContinuations: 3,
			MinToolCalls:     1,
		},
		sessions: make(map[string]*stallGuardSession),
		continueFn: func(sessionID string, continuationPrompt string) error {
			capturedPrompt = continuationPrompt
			return nil
		},
		stopCh: make(chan struct{}),
	}

	sg.recordTurnComplete("test-session", 5)
	time.Sleep(1500 * time.Millisecond)

	if capturedPrompt == "" {
		t.Fatal("expected continuation prompt to be captured")
	}
	if !strings.Contains(capturedPrompt, "STALL GUARD") {
		t.Fatal("expected continuation prompt to contain 'STALL GUARD'")
	}
	if !strings.Contains(capturedPrompt, "5") {
		t.Fatal("expected continuation prompt to reference tool call count")
	}
}

func TestStallGuard_Stop(t *testing.T) {
	sg := &StallGuard{
		config: StallGuardConfig{
			Enabled:          true,
			IdleTimeoutSecs:  60,
			MaxContinuations: 3,
			MinToolCalls:     1,
		},
		sessions: make(map[string]*stallGuardSession),
		continueFn: func(sessionID string, continuationPrompt string) error {
			return nil
		},
		stopCh: make(chan struct{}),
	}

	sg.recordTurnComplete("test-session", 2)
	sg.stop()

	if len(sg.sessions) != 0 {
		t.Fatal("expected all sessions to be cleared after stop")
	}
}

func TestStallGuard_CoAgentWaitPausesTimer(t *testing.T) {
	var triggered atomic.Int32
	sg := &StallGuard{
		config: StallGuardConfig{
			Enabled:          true,
			IdleTimeoutSecs:  1,
			MaxContinuations: 3,
			MinToolCalls:     1,
		},
		sessions: make(map[string]*stallGuardSession),
		continueFn: func(sessionID string, continuationPrompt string) error {
			triggered.Add(1)
			return nil
		},
		stopCh: make(chan struct{}),
	}

	sg.recordTurnComplete("test-session", 2)
	sg.setWaitingForCoAgents("test-session", true)

	time.Sleep(1500 * time.Millisecond)
	if triggered.Load() != 0 {
		t.Fatalf("expected 0 triggers while waiting for co-agents, got %d", triggered.Load())
	}

	sg.setWaitingForCoAgents("test-session", false)
	time.Sleep(1500 * time.Millisecond)
	if triggered.Load() != 1 {
		t.Fatalf("expected 1 trigger after co-agent wait cleared, got %d", triggered.Load())
	}
}

func TestStallGuard_CoAgentWaitSkipsRecordTurn(t *testing.T) {
	var triggered atomic.Int32
	sg := &StallGuard{
		config: StallGuardConfig{
			Enabled:          true,
			IdleTimeoutSecs:  1,
			MaxContinuations: 3,
			MinToolCalls:     1,
		},
		sessions: make(map[string]*stallGuardSession),
		continueFn: func(sessionID string, continuationPrompt string) error {
			triggered.Add(1)
			return nil
		},
		stopCh: make(chan struct{}),
	}

	sg.setWaitingForCoAgents("test-session", true)
	sg.recordTurnComplete("test-session", 2)

	time.Sleep(1500 * time.Millisecond)
	if triggered.Load() != 0 {
		t.Fatalf("expected 0 triggers when co-agent wait is set before turn, got %d", triggered.Load())
	}
}
