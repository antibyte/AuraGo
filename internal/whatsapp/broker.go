package whatsapp

import (
	"context"
	"strings"
	"sync"
	"time"

	"aurago/internal/agent"

	"go.mau.fi/whatsmeow/types"
)

// statusCoalesceWindow is the shortest gap between two status messages of one turn.
const statusCoalesceWindow = 10 * time.Second

// Status prefixes, the same as TelegramBroker writes (internal/telegram/bot.go).
const (
	statusPrefix   = "⚙️ "
	progressPrefix = "⏳ "
	budgetWarning  = "⚠️ "
	budgetBlocked  = "🚫 "
)

var _ agent.FeedbackBroker = (*whatsAppBroker)(nil)

// whatsAppBroker implements agent.FeedbackBroker for one owner turn (TelegramBroker parity
// for text events; audio and YouTube capture arrive in stage 3). It sends only through the
// sender, whose owner guard also covers to: the pipeline takes it from ownerRecipients.
// It logs nothing, so no message content reaches a log.
type whatsAppBroker struct {
	ctx        context.Context
	sender     *sender
	to         types.JID
	now        func() time.Time
	mu         sync.Mutex
	lastStatus time.Time
}

// Send delivers the events a chat user needs and ignores the rest.
//
//   - question_user, budget_warning and budget_blocked are delivered at once.
//   - tool_start, error_recovery, api_retry and progress are status updates: at most
//     one per statusCoalesceWindow, the others are dropped.
//   - audio, youtube_video and every other event are ignored.
func (b *whatsAppBroker) Send(event, message string) {
	switch event {
	case "question_user":
		b.deliver(message, "question")
	case "tool_start", "error_recovery", "api_retry", "progress":
		text := statusPrefix + strings.ToUpper(event) + ": " + message
		if event == "progress" {
			text = progressPrefix + message
		}
		if b.takeStatusSlot() {
			b.deliver(text, "status update")
		}
	case "budget_warning":
		b.deliver(budgetWarning+message, "budget notice")
	case "budget_blocked":
		b.deliver(budgetBlocked+message, "budget notice")
	}
}

// takeStatusSlot reports whether a status update may go out now, and claims the slot.
func (b *whatsAppBroker) takeStatusSlot() bool {
	now := time.Now
	if b.now != nil {
		now = b.now
	}
	t := now()
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.lastStatus.IsZero() && t.Sub(b.lastStatus) < statusCoalesceWindow {
		return false
	}
	b.lastStatus = t
	return true
}

// deliver hands text to the sender, which counts and logs a failed delivery.
func (b *whatsAppBroker) deliver(text, label string) {
	_ = b.sender.deliverText(b.ctx, b.to, text, label)
}

func (b *whatsAppBroker) SendJSON(string)                                             {}
func (b *whatsAppBroker) SendLLMStreamDelta(string, string, string, int, string)      {}
func (b *whatsAppBroker) SendLLMStreamDone(string)                                    {}
func (b *whatsAppBroker) SendTokenUpdate(int, int, int, int, int, bool, bool, string) {}
func (b *whatsAppBroker) SendThinkingBlock(string, string, string)                    {}
