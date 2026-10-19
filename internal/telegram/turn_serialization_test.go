package telegram

import (
	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/tools"
	"context"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestTelegramIngressWaitsBeforePersistingAndCancellationReleasesQueue(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	id, err := store.ResolveUserSession(context.Background(), "default")
	if err != nil {
		t.Fatal(err)
	}
	unlock, err := store.LockSessionTurn(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan any, 1)
	update := tgbotapi.Update{Message: &tgbotapi.Message{From: &tgbotapi.User{ID: 1}, Chat: &tgbotapi.Chat{ID: 1}, Text: "queued ingress"}}
	go func() {
		defer func() { done <- recover() }()
		processUpdate(ctx, nil, update, &config.Config{}, logger, nil, store, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	}()
	select {
	case panicValue := <-done:
		t.Fatalf("queued Telegram turn entered before unlock (panic=%v)", panicValue)
	case <-time.After(20 * time.Millisecond):
	}
	messages, err := store.GetRecentMessages(id, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 0 {
		t.Fatal("Telegram wrote history before acquiring shared turn gate")
	}
	cancel()
	select {
	case panicValue := <-done:
		if panicValue != nil {
			t.Fatal("Telegram entered processing after queue cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("Telegram queue ignored cancellation")
	}
}

func TestTelegramQuestionReplyBypassesTheWaitingTurnsGate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	id, err := store.ResolveUserSession(context.Background(), "default")
	if err != nil {
		t.Fatal(err)
	}
	releaseTurn, err := store.LockSessionTurn(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseTurn()
	response := tools.RegisterQuestion(id, &tools.PendingQuestion{Question: "Continue?", AllowFreeText: true})
	defer tools.CancelQuestion(id)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan any, 1)
	update := tgbotapi.Update{Message: &tgbotapi.Message{From: &tgbotapi.User{ID: 1}, Chat: &tgbotapi.Chat{ID: 1}, Text: "continue"}}
	go func() {
		defer func() { done <- recover() }()
		processUpdate(ctx, nil, update, &config.Config{}, logger, nil, store, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	}()
	select {
	case answer := <-response:
		if answer.Source != tools.QuestionSourceTelegram || answer.FreeText != "continue" {
			t.Fatalf("question reply changed: %+v", answer)
		}
	case <-time.After(time.Second):
		t.Fatal("question reply waited behind the turn that needed its answer")
	}
	select {
	case panicValue := <-done:
		if panicValue != nil {
			t.Fatalf("reply processing panicked: %v", panicValue)
		}
	case <-time.After(time.Second):
		t.Fatal("question reply did not finish")
	}
	messages, err := store.GetRecentMessages(id, 10)
	if err != nil || len(messages) != 0 {
		t.Fatalf("question reply became a new history turn: %v %v", messages, err)
	}
}
