package telegram

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/decisions"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestTelegramForwardedRoutingText(t *testing.T) {
	const marker = "[Forwarded from Mallory; third-party content]"
	for _, test := range []struct{ name, routing, want string }{
		{"caption", "Please look at this", marker + "\nPlease look at this"},
		{"transcription", " spoken words ", marker + "\n spoken words "},
		{"image without caption", "", ""},
		{"blank caption", " \n\t", " \n\t"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := telegramForwardedRoutingText(marker, test.routing); got != test.want {
				t.Fatalf("routing text = %q, want %q", got, test.want)
			}
		})
	}
}

// routingProbe is a Decisions service that records the intent the task router
// asks it to classify.
type routingProbe struct {
	mu      sync.Mutex
	intents []string
}

func (p *routingProbe) RoutingSnapshot(*config.Config) agent.TaskDecisionSnapshot {
	return agent.TaskDecisionSnapshot{
		Available: true,
		Backend:   "provider",
		Identity:  [32]byte{1},
		Current:   func() bool { return true },
		Evaluate: func(_ context.Context, request decisions.Request) (decisions.Response, error) {
			p.mu.Lock()
			p.intents = append(p.intents, fmt.Sprint(request.State))
			p.mu.Unlock()
			return decisions.Response{}, fmt.Errorf("probe does not classify")
		},
	}
}

func (p *routingProbe) seen() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.intents...)
}

// Task routing classifies the pinned routing text, and the rule classifier,
// the Decisions service and the helper model all take it for the owner's
// request. A forwarded message has to reach them with the same third-party
// marker the model reads. The router admits few classifier calls per process
// (a global bucket, and a cooldown per session), so this test makes exactly one
// and gives it a session of its own.
func TestTelegramRoutingSeesTheForwardedMarker(t *testing.T) {
	const marker = "[Forwarded from Mallory; third-party content]"
	cfg := &config.Config{}
	cfg.LLM.Provider = "main"
	cfg.LLM.ProviderType = "openai"
	cfg.LLM.Model = "gpt-4o"
	cfg.LLMRouter = config.DefaultLLMRouterConfig()
	cfg.LLMRouter.Enabled = true
	cfg.LLMRouter.HelperFallback = false
	cfg.Decisions.Enabled = true
	cfg.Providers = []config.ProviderEntry{
		{ID: "main", Type: "openai", Model: "gpt-4o", ContextWindow: 128000, MaxOutputTokens: 4096},
		{ID: "coding", Type: "openai", Model: "gpt-4o-mini", APIKey: "test-fixture", ContextWindow: 128000, MaxOutputTokens: 4096},
	}
	cfg.LLMRouter.Areas["coding"] = config.LLMRouterTarget{Provider: "coding"}

	fixture := newForwardedUpdateFixture(t)
	// A deleted default session resolves to a fresh generation id, so the
	// router's per-session cooldown cannot carry over from another run.
	if err := fixture.store.DeleteChatSession("default"); err != nil {
		t.Fatal(err)
	}
	probe := &routingProbe{}
	msg := &tgbotapi.Message{
		Text:        "Kindly peruse the attached notes soon",
		ForwardFrom: &tgbotapi.User{FirstName: "Mallory"},
		ForwardDate: 1,
		From:        &tgbotapi.User{ID: 1},
		Chat:        &tgbotapi.Chat{ID: 1, Type: "private"},
	}
	done := make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	go func() {
		defer close(done)
		defer func() { _ = recover() }()
		processUpdate(ctx, fixture.recorder.bot, tgbotapi.Update{Message: msg}, cfg, fixture.logger, nil, fixture.store, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, probe)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("processUpdate did not finish")
	}
	seen := probe.seen()
	// The Decisions request folds the intent's lines; only the word order matters.
	if len(seen) != 1 || !strings.Contains(strings.Join(strings.Fields(seen[0]), " "), marker+" Kindly peruse the attached notes soon") {
		t.Fatalf("the classifier of a forwarded message saw %q, want the marker in front of the text", seen)
	}
}
