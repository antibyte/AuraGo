package rocketchat

import (
	"aurago/internal/config"
	"aurago/internal/memory"
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestRocketChatIngressWaitsBeforePersistingAndCancellationReleasesQueue(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	msg := message{Msg: "queued ingress"}
	msg.User.ID = "alice"
	msg.User.Username = "alice"
	id, err := store.ResolveUserSession(context.Background(), rocketChatSessionID("room", "alice"))
	if err != nil {
		t.Fatal(err)
	}
	unlock, err := store.LockSessionTurn(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	cfg := &config.Config{}
	cfg.RocketChat.Enabled = true
	cfg.RocketChat.AllowedUsers = []string{"id:alice"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan any, 1)
	go func() {
		defer func() { done <- recover() }()
		processMessage(ctx, cfg, logger, nil, store, nil, nil, nil, nil, nil, nil, nil, "room", msg, nil, nil, nil, nil, nil)
	}()
	select {
	case panicValue := <-done:
		t.Fatalf("queued Rocket.Chat turn entered before unlock (panic=%v)", panicValue)
	case <-time.After(20 * time.Millisecond):
	}
	messages, err := store.GetRecentMessages(id, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 0 {
		t.Fatal("Rocket.Chat wrote history before acquiring shared turn gate")
	}
	cancel()
	select {
	case panicValue := <-done:
		if panicValue != nil {
			t.Fatal("Rocket.Chat entered processing after queue cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("Rocket.Chat queue ignored cancellation")
	}
}
