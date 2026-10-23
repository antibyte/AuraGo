package agent

import (
	"context"
	"log/slog"
	"slices"
	"strings"
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
	if len(broker.events) != 2 || broker.events[1] != "progress" || !slices.Contains(i18n.WorkflowWaitMessages("de"), broker.detail[1]) {
		t.Fatalf("repeat status: %v / %v", broker.events, broker.detail)
	}
	feedback.Stop()
	feedback.onTimer()
	feedback.StepStarted()
	if len(broker.events) != 2 {
		t.Fatalf("status after completion: %v", broker.events)
	}
}

func TestAgentProgressFeedbackWaitVariants(t *testing.T) {
	i18n.Load(ui.Content, slog.Default())
	keys := []string{
		"backend.workflow_wait", "backend.workflow_wait_2", "backend.workflow_wait_3",
		"backend.workflow_wait_4", "backend.workflow_wait_5", "backend.workflow_wait_6",
	}
	languages := i18n.GetSupportedLanguages()
	if len(languages) != 16 {
		t.Fatalf("supported languages: got %d, want 16", len(languages))
	}
	for _, language := range languages {
		t.Run(language, func(t *testing.T) {
			messages := i18n.WorkflowWaitMessages(language)
			if len(messages) != len(keys) {
				t.Fatalf("waiting phrases: got %d, want %d", len(messages), len(keys))
			}
			unique := make(map[string]bool)
			for index, key := range keys {
				message := messages[index]
				if !i18n.HasKey(language, key) || strings.TrimSpace(message) == "" || message != i18n.T(language, key) || unique[message] {
					t.Fatalf("missing or duplicate translation for %s: %q", key, message)
				}
				if language != "en" && message == i18n.T("en", key) {
					t.Fatalf("untranslated waiting phrase %s: %q", key, message)
				}
				unique[message] = true
			}
			broker := &progressCaptureBroker{}
			feedback := &agentProgressFeedback{ctx: context.Background(), broker: broker, language: language, sent: 1}
			for round := 0; round < 32; round++ {
				feedback.sendLocked()
				message := broker.detail[round]
				if broker.events[round] != "progress" || !unique[message] {
					t.Fatalf("unexpected waiting phrase: %q", message)
				}
				if round > 0 && message == broker.detail[round-1] {
					t.Fatalf("consecutive waiting phrases repeated: %q", message)
				}
			}
		})
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
