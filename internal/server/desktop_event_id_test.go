package server

import (
	"encoding/json"
	"testing"
	"time"

	"aurago/internal/desktop"
)

func receiveDesktopHubEvent(t *testing.T, ch <-chan desktop.Event) desktop.Event {
	t.Helper()
	select {
	case event := <-ch:
		return event
	case <-time.After(time.Second):
		t.Fatal("no desktop hub event received")
		return desktop.Event{}
	}
}

func receiveDesktopSSEEvent(t *testing.T, ch chan string) desktop.Event {
	t.Helper()
	select {
	case msg := <-ch:
		var decoded struct {
			Type    string        `json:"type"`
			Payload desktop.Event `json:"payload"`
		}
		if err := json.Unmarshal([]byte(msg), &decoded); err != nil {
			t.Fatalf("SSE message is not JSON: %v; msg=%s", err, msg)
		}
		if decoded.Type != string(EventVirtualDesktop) {
			t.Fatalf("SSE type = %q, want %q", decoded.Type, EventVirtualDesktop)
		}
		return decoded.Payload
	case <-time.After(time.Second):
		t.Fatal("no SSE message received")
		return desktop.Event{}
	}
}

// broadcastDesktopEvent sends each event over the desktop WebSocket hub and
// SSE. The shell listens to both, so both copies must carry the same id for
// it to handle the event once (one toast, one bell count per notification).
func TestBroadcastDesktopEventSharesOneIDAcrossTransports(t *testing.T) {
	s := &Server{SSE: NewSSEBroadcaster()}
	hub := desktop.NewHub(2)
	hubEvents, unsubscribe, err := hub.Subscribe()
	if err != nil {
		t.Fatalf("subscribe hub: %v", err)
	}
	defer unsubscribe()
	sseEvents := s.SSE.subscribe()
	defer s.SSE.unsubscribe(sseEvents)

	notify := func() desktop.Event {
		return desktop.Event{Type: "notification", Payload: map[string]any{"title": "Person", "message": "Front door"}, CreatedAt: time.Now().UTC()}
	}
	broadcastDesktopEvent(s, hub, notify())
	broadcastDesktopEvent(s, hub, notify())

	firstWS, firstSSE := receiveDesktopHubEvent(t, hubEvents), receiveDesktopSSEEvent(t, sseEvents)
	secondWS, secondSSE := receiveDesktopHubEvent(t, hubEvents), receiveDesktopSSEEvent(t, sseEvents)
	if firstWS.ID == "" || secondWS.ID == "" {
		t.Fatalf("desktop events need an id, got %q and %q", firstWS.ID, secondWS.ID)
	}
	if firstSSE.ID != firstWS.ID || secondSSE.ID != secondWS.ID {
		t.Fatalf("SSE ids %q/%q differ from WebSocket ids %q/%q", firstSSE.ID, secondSSE.ID, firstWS.ID, secondWS.ID)
	}
	if firstWS.ID == secondWS.ID {
		t.Fatalf("two notifications share id %q; the shell would drop the second one", firstWS.ID)
	}
}

func TestBroadcastDesktopEventKeepsCallerID(t *testing.T) {
	s := &Server{SSE: NewSSEBroadcaster()}
	hub := desktop.NewHub(1)
	hubEvents, unsubscribe, err := hub.Subscribe()
	if err != nil {
		t.Fatalf("subscribe hub: %v", err)
	}
	defer unsubscribe()
	sseEvents := s.SSE.subscribe()
	defer s.SSE.unsubscribe(sseEvents)

	broadcastDesktopEvent(s, hub, desktop.Event{ID: "evt-fixed", Type: "desktop_changed"})
	if got := receiveDesktopHubEvent(t, hubEvents).ID; got != "evt-fixed" {
		t.Fatalf("hub id = %q, want evt-fixed", got)
	}
	if got := receiveDesktopSSEEvent(t, sseEvents).ID; got != "evt-fixed" {
		t.Fatalf("SSE id = %q, want evt-fixed", got)
	}
}
