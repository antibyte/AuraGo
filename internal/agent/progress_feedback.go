package agent

import (
	"context"
	"sync"
	"time"

	"aurago/internal/i18n"
)

const (
	progressFirstDelay  = 5 * time.Second
	progressRepeatDelay = 30 * time.Second
	progressStepLimit   = 4
)

// agentProgressFeedback sends transient status only while an interactive turn
// is still running. It never adds an assistant message or changes the tool result.
type agentProgressFeedback struct {
	mu       sync.Mutex
	ctx      context.Context
	broker   FeedbackBroker
	language string
	timer    *time.Timer
	steps    int
	sent     int
	stopped  bool
}

func newAgentProgressFeedback(ctx context.Context, broker FeedbackBroker, language string, enabled bool) *agentProgressFeedback {
	feedback := &agentProgressFeedback{ctx: ctx, broker: broker, language: language, stopped: !enabled || broker == nil}
	if !feedback.stopped {
		feedback.timer = time.AfterFunc(progressFirstDelay, feedback.onTimer)
	}
	return feedback
}

func (f *agentProgressFeedback) onTimer() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stopped || f.ctx.Err() != nil {
		return
	}
	f.sendLocked()
	f.timer = time.AfterFunc(progressRepeatDelay, f.onTimer)
}

func (f *agentProgressFeedback) sendLocked() {
	key := "backend.workflow_feedback"
	if f.sent > 0 {
		key = "backend.workflow_wait"
	}
	f.broker.Send("progress", i18n.T(f.language, key))
	f.sent++
}

func (f *agentProgressFeedback) StepStarted() {
	if f == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stopped || f.ctx.Err() != nil {
		return
	}
	f.steps++
	if f.steps != progressStepLimit || f.sent != 0 {
		return
	}
	f.sendLocked()
	if f.timer != nil {
		f.timer.Stop()
	}
	f.timer = time.AfterFunc(progressRepeatDelay, f.onTimer)
}

func (f *agentProgressFeedback) Stop() {
	if f == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped = true
	if f.timer != nil {
		f.timer.Stop()
	}
}
