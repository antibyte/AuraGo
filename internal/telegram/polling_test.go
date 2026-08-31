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
		cancelAt:  3,
		cancel:    cancel,
		sensitive: "HTTP 500 token=secret-should-never-appear",
	}
	runPollingLoop(ctx, poller, tgbotapi.NewUpdate(0), nil, nil, nil)

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
	poller := &telegramPollSequence{cancelAt: 1, cancel: cancel}
	var handled int
	runPollingLoop(ctx, poller, tgbotapi.NewUpdate(0), nil, nil, func(tgbotapi.Update) { handled++ })

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
	failing := &telegramPollSequence{failures: 3, cancelAt: 3, cancel: failureCancel, sensitive: "poll failed"}
	runPollingLoop(failureCtx, failing, tgbotapi.NewUpdate(0), db, nil, nil)
	page, err := planner.ListOperationalIssues(db, planner.OperationalIssueListFilter{Status: "active", Source: "telegram"})
	if err != nil || page.Total != 1 {
		t.Fatalf("active polling issue = %#v, err:%v", page, err)
	}

	successCtx, successCancel := context.WithCancel(context.Background())
	runPollingLoop(successCtx, &telegramPollSequence{cancelAt: 1, cancel: successCancel}, tgbotapi.NewUpdate(0), db, nil, nil)
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
