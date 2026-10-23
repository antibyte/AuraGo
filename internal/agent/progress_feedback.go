package agent

import (
	"context"
	"math/rand/v2"
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
	mu          sync.Mutex
	ctx         context.Context
	broker      FeedbackBroker
	language    string
	timer       *time.Timer
	steps       int
	sent        int
	waitVariant int
	stopped     bool
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
	message := i18n.T(f.language, "backend.workflow_feedback")
	if f.sent > 0 {
		messages := i18n.WorkflowWaitMessages(f.language)
		count := len(messages)
		if f.sent > 1 {
			count--
		}
		variant := rand.IntN(count)
		// Skip the previous phrase so consecutive updates always differ.
		if f.sent > 1 && variant >= f.waitVariant {
			variant++
		}
		f.waitVariant = variant
		message = messages[variant]
	}
	f.broker.Send("progress", message)
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
