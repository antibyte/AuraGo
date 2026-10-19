package memory

import (
	"context"
	"errors"
	"github.com/sashabaranov/go-openai"
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestBeginSessionTurnRegistersQueuedRunBeforeWaitingAndStopCancelsIt(t *testing.T) {
	store, err := NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	const id = "shared-ingress"
	first, release, err := store.BeginSessionTurn(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	done := make(chan error, 1)
	go func() {
		_, unlock, err := store.BeginSessionTurn(context.Background(), id)
		if unlock != nil {
			unlock()
		}
		done <- err
	}()
	deadline := time.Now().Add(time.Second)
	for {
		sessionTurnGates.Lock()
		gate := sessionTurnGates.entries[sessionTurnKey{store, id}]
		queued := gate != nil && gate.refs == 2
		sessionTurnGates.Unlock()
		if queued {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("second turn never queued")
		}
		time.Sleep(time.Millisecond)
	}
	store.CancelSessionRuns(id)
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("queued error=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Stop did not cancel a queued ingress turn")
	}
	if !errors.Is(first.Err(), context.Canceled) {
		t.Fatal("active ingress was not canceled")
	}
	release()
	release()
	_, nextRelease, err := store.BeginSessionTurn(context.Background(), id)
	if err != nil {
		t.Fatalf("Stop fenced future turns: %v", err)
	}
	nextRelease()
}
func TestBeginSessionTurnKeepsHistoryAssemblyAfterPriorPublication(t *testing.T) {
	store, err := NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	const id = "cross-channel-history"
	_, release, err := store.BeginSessionTurn(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err = store.InsertMessage(id, "user", "first user", false, false); err != nil {
		t.Fatal(err)
	}
	entered := make(chan []openai.ChatCompletionMessage, 1)
	done := make(chan error, 1)
	go func() {
		_, unlock, err := store.BeginSessionTurn(context.Background(), id)
		if err != nil {
			done <- err
			return
		}
		defer unlock()
		messages, err := store.GetRecentMessages(id, 10)
		entered <- messages
		done <- err
	}()
	select {
	case <-entered:
		t.Fatal("second channel read unfinished history")
	case <-time.After(20 * time.Millisecond):
	}
	if _, err = store.InsertMessage(id, "assistant", "first answer delivered", false, false); err != nil {
		t.Fatal(err)
	}
	release()
	select {
	case messages := <-entered:
		if len(messages) != 2 || messages[1].Content != "first answer delivered" {
			t.Fatalf("stale history: %#v", messages)
		}
	case <-time.After(time.Second):
		t.Fatal("second channel stayed blocked")
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
}
