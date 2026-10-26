package telegram

import (
	"context"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// telegramSingleUpdatePoller hands out one update and then stays busy without
// handing out further updates, so a cursor retry can only come from the
// poller itself.
type telegramSingleUpdatePoller struct {
	update tgbotapi.Update
	served bool
}

func (p *telegramSingleUpdatePoller) GetUpdates(tgbotapi.UpdateConfig) ([]tgbotapi.Update, error) {
	if !p.served {
		p.served = true
		return []tgbotapi.Update{p.update}, nil
	}
	time.Sleep(5 * time.Millisecond)
	return nil, nil
}

// A cursor write that fails must not be taken for a success: the cursor stays
// unwritten and the poller retries it, also when no further update arrives.
func TestTelegramCursorWriteIsRetriedAfterAFailure(t *testing.T) {
	oldInterval := telegramOffsetPersistInterval
	telegramOffsetPersistInterval = 10 * time.Millisecond
	t.Cleanup(func() { telegramOffsetPersistInterval = oldInterval })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	poller := &telegramSingleUpdatePoller{update: tgbotapi.Update{UpdateID: 7}}
	attempts := make(chan int, 8)
	done := make(chan int, 1)
	go func() {
		done <- runPollingLoop(ctx, poller, tgbotapi.NewUpdate(0), nil, nil, func(tgbotapi.Update) bool { return true }, func(offset int) bool {
			select {
			case attempts <- offset:
			default:
			}
			return false // the store rejects the write
		})
	}()
	// The first write follows the update, the second one is the retry that the
	// failed write has to trigger.
	for attempt := 1; attempt <= 2; attempt++ {
		select {
		case offset := <-attempts:
			if offset != 8 {
				t.Fatalf("persisted cursor = %d, want 8", offset)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("attempt %d: the failed cursor write was not retried", attempt)
		}
	}
	cancel()
	awaitTelegram(t, done)
}
