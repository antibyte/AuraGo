package agent

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	"aurago/internal/i18n"
	"aurago/ui"
)

type progressCaptureBroker struct {
	NoopBroker
	mu     sync.Mutex
	events []string
	detail []string
}

func (b *progressCaptureBroker) Send(event, message string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, event)
	b.detail = append(b.detail, message)
}

func TestAgentProgressFeedbackStepsAndCompletion(t *testing.T) {
	i18n.Load(ui.Content, slog.Default())
	broker := &progressCaptureBroker{}
	feedback := newAgentProgressFeedback(context.Background(), broker, "de", true)
	defer feedback.Stop()
	for step := 0; step < 3; step++ {
		feedback.StepStarted()
	}
	if len(broker.events) != 0 {
		t.Fatalf("status before fourth step: %v", broker.events)
	}
	feedback.StepStarted()
	if len(broker.events) != 1 || broker.events[0] != "progress" || broker.detail[0] != "Ich schaue mir das an. Einen Moment." {
		t.Fatalf("first status: %v / %v", broker.events, broker.detail)
	}
	feedback.onTimer()
	if len(broker.events) != 2 || broker.detail[1] != "Ich bin noch dran. Es dauert etwas länger." {
		t.Fatalf("repeat status: %v / %v", broker.events, broker.detail)
	}
	feedback.Stop()
	feedback.onTimer()
	feedback.StepStarted()
	if len(broker.events) != 2 {
		t.Fatalf("status after completion: %v", broker.events)
	}
}

func TestAgentProgressFeedbackCancelledOrDisabled(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		ctx, cancel := context.WithCancel(context.Background())
		broker := &progressCaptureBroker{}
		feedback := newAgentProgressFeedback(ctx, broker, "de", enabled)
		cancel()
		feedback.onTimer()
		for step := 0; step < 4; step++ {
			feedback.StepStarted()
		}
		feedback.Stop()
		if len(broker.events) != 0 {
			t.Fatalf("status after cancellation (enabled=%v): %v", enabled, broker.events)
		}
	}
}
