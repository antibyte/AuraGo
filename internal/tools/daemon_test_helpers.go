package tools

import (
	"io"
	"log/slog"
	"sync"
)

func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type mockBroadcaster struct {
	mu     sync.Mutex
	events []mockEvent
}

type mockEvent struct {
	Type    string
	Payload any
}

func (b *mockBroadcaster) BroadcastDaemonEvent(eventType string, payload any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, mockEvent{Type: eventType, Payload: payload})
}

func (b *mockBroadcaster) eventCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.events)
}
