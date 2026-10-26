package telegram

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/integrationstatus"
	"aurago/internal/planner"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type telegramPollSequence struct {
	calls     int
	failures  int
	cancelAt  int
	cancel    context.CancelFunc
	sensitive string
}

func TestTelegramWorkerWaitHonorsCancellation(t *testing.T) {
	slots := make(chan struct{}, 1)
	slots <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan bool, 1)
	go func() { done <- acquireTelegramWorker(ctx, slots) }()
	cancel()
	select {
	case acquired := <-done:
		if acquired || len(slots) != 1 {
			t.Fatal("cancelled worker acquired or released another worker's slot")
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled worker remained blocked")
	}
	<-slots
	if acquireTelegramWorker(ctx, slots) {
		t.Fatal("cancelled owner admitted new work")
	}
}

func (p *telegramPollSequence) GetUpdates(tgbotapi.UpdateConfig) ([]tgbotapi.Update, error) {
	p.calls++
	if p.calls == p.cancelAt && p.cancel != nil {
		p.cancel()
	}
	if p.calls <= p.failures {
		return nil, errors.New(p.sensitive)
	}
	return []tgbotapi.Update{{UpdateID: 42}}, nil
}

func TestRunPollingLoopThresholdAndSanitizedStatus(t *testing.T) {
	integrationstatus.SetTelegramConfigured(true, true)
	oldBackoff := telegramPollingInitialBackoff
	telegramPollingInitialBackoff = time.Millisecond
	t.Cleanup(func() { telegramPollingInitialBackoff = oldBackoff })

	ctx, cancel := context.WithCancel(context.Background())
	poller := &telegramPollSequence{
		failures:  3,
		cancelAt:  4,
		cancel:    cancel,
		sensitive: "HTTP 500 token=secret-should-never-appear",
	}
	runPollingLoop(ctx, poller, tgbotapi.NewUpdate(0), nil, nil, nil, nil)

	status := integrationstatus.TelegramStatus()
	if status.ConsecutivePollErrors != 3 || status.State != "degraded" {
		t.Fatalf("status = %#v, want three consecutive errors and degraded", status)
	}
	if status.LastErrorCode != "telegram_poll_failed" {
		t.Fatalf("last error code = %q", status.LastErrorCode)
	}
}

func TestRunPollingLoopSuccessfulPollResolvesRuntimeFailure(t *testing.T) {
	integrationstatus.SetTelegramConfigured(true, true)
	integrationstatus.MarkTelegramPollFailure("telegram_poll_failed", time.Now())
	ctx, cancel := context.WithCancel(context.Background())
	poller := &telegramPollSequence{cancelAt: 2, cancel: cancel}
	var handled int
	runPollingLoop(ctx, poller, tgbotapi.NewUpdate(0), nil, nil, func(tgbotapi.Update) bool { handled++; return true }, nil)

	status := integrationstatus.TelegramStatus()
	if status.State != "healthy" || status.ConsecutivePollErrors != 0 || status.LastErrorCode != "" {
		t.Fatalf("recovered status = %#v", status)
	}
	if handled != 1 {
		t.Fatalf("handled updates = %d, want 1", handled)
	}
}

func TestRunPollingLoopOperationalIssueStartsAtThreeAndResolves(t *testing.T) {
	db, err := planner.InitDB(filepath.Join(t.TempDir(), "planner.db"))
	if err != nil {
		t.Fatalf("planner.InitDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	integrationstatus.SetTelegramConfigured(true, true)
	oldBackoff := telegramPollingInitialBackoff
	telegramPollingInitialBackoff = time.Millisecond
	t.Cleanup(func() { telegramPollingInitialBackoff = oldBackoff })

	failureCtx, failureCancel := context.WithCancel(context.Background())
	failing := &telegramPollSequence{failures: 3, cancelAt: 4, cancel: failureCancel, sensitive: "poll failed"}
	runPollingLoop(failureCtx, failing, tgbotapi.NewUpdate(0), db, nil, nil, nil)
	page, err := planner.ListOperationalIssues(db, planner.OperationalIssueListFilter{Status: "active", Source: "telegram"})
	if err != nil || page.Total != 1 {
		t.Fatalf("active polling issue = %#v, err:%v", page, err)
	}

	successCtx, successCancel := context.WithCancel(context.Background())
	runPollingLoop(successCtx, &telegramPollSequence{cancelAt: 2, cancel: successCancel}, tgbotapi.NewUpdate(0), db, nil, nil, nil)
	page, err = planner.ListOperationalIssues(db, planner.OperationalIssueListFilter{Status: "active", Source: "telegram"})
	if err != nil || page.Total != 0 {
		t.Fatalf("resolved polling issue = %#v, err:%v", page, err)
	}
}

func TestTelegramTestMessageIsFixedAndTimestamped(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 34, 56, 0, time.FixedZone("test", 2*60*60))
	got := telegramTestMessage(now)
	want := "AuraGo Telegram test (manual) — 2026-08-29T10:34:56Z"
	if got != want {
		t.Fatalf("test message = %q, want %q", got, want)
	}
}

type telegramFloodPoller struct {
	calls  int
	first  time.Time
	second time.Time
	cancel context.CancelFunc
}

func (p *telegramFloodPoller) GetUpdates(tgbotapi.UpdateConfig) ([]tgbotapi.Update, error) {
	p.calls++
	if p.calls == 1 {
		p.first = time.Now()
		return nil, &tgbotapi.Error{
			Code:               429,
			Message:            "Too Many Requests: retry after 1",
			ResponseParameters: tgbotapi.ResponseParameters{RetryAfter: 1},
		}
	}
	p.second = time.Now()
	if p.cancel != nil {
		p.cancel()
	}
	return nil, nil
}

func TestRunPollingLoopWaitsForTelegramFloodControl(t *testing.T) {
	integrationstatus.SetTelegramConfigured(true, true)
	oldBackoff := telegramPollingInitialBackoff
	// A local backoff would retry almost immediately; only the advertised
	// retry_after may delay the next poll.
	telegramPollingInitialBackoff = 10 * time.Millisecond
	t.Cleanup(func() { telegramPollingInitialBackoff = oldBackoff })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	poller := &telegramFloodPoller{cancel: cancel}
	runPollingLoop(ctx, poller, tgbotapi.NewUpdate(0), nil, nil, nil, nil)

	if poller.calls < 2 {
		t.Fatalf("poll attempts = %d, want a retry", poller.calls)
	}
	if waited := poller.second.Sub(poller.first); waited < 900*time.Millisecond {
		t.Fatalf("retry delay = %v, want the advertised retry_after of 1s", waited)
	}
	if status := integrationstatus.TelegramStatus(); status.LastErrorCode != "telegram_poll_rate_limited" {
		t.Fatalf("last error code = %q, want telegram_poll_rate_limited", status.LastErrorCode)
	}
}

func TestTelegramRetryAfterUsesAdvertisedDelay(t *testing.T) {
	flood := &tgbotapi.Error{Code: 429, Message: "Too Many Requests", ResponseParameters: tgbotapi.ResponseParameters{RetryAfter: 7}}
	wait, ok := telegramRetryAfter(flood)
	if !ok || wait != 7*time.Second {
		t.Fatalf("retry after = %v (%t), want 7s", wait, ok)
	}
	flood.RetryAfter = 600
	if wait, ok = telegramRetryAfter(flood); !ok || wait != 60*time.Second {
		t.Fatalf("capped retry after = %v (%t), want 60s", wait, ok)
	}
	other := &tgbotapi.Error{Code: 500, Message: "Internal Server Error"}
	if _, ok = telegramRetryAfter(other); ok {
		t.Fatal("a non flood-control error must not request a retry_after delay")
	}
}

func TestTelegramPollingBackoffJitterStaysWithinTwentyPercent(t *testing.T) {
	base := 10 * time.Second
	for i := 0; i < 200; i++ {
		got := telegramPollBackoff(base)
		if got < 8*time.Second || got > 12*time.Second {
			t.Fatalf("jittered backoff = %v, want within 20%% of %v", got, base)
		}
	}
}

func TestTelegramHTTPClientBoundsStuckRequests(t *testing.T) {
	client := newTelegramHTTPClient()
	if client.Timeout != 90*time.Second {
		t.Fatalf("telegram http timeout = %v, want 90s", client.Timeout)
	}
	if client.Timeout <= 60*time.Second {
		t.Fatalf("telegram http timeout = %v, must exceed the 60s long poll", client.Timeout)
	}
}
