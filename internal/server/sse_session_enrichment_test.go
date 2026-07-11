package server

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSSEBrokerAdapterSendJSONChecksOnlyTopLevelSessionID(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "nested session id", body: `{"type":"agent_action","payload":{"action":"start","session_id":"nested"}}`},
		{name: "text fragment", body: `{"event":"debug","detail":"text contains \"session_id\" only"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewSSEBroadcaster()
			ch := b.subscribe()
			defer b.unsubscribe(ch)
			adapter := NewSSEBrokerAdapterWithSession(b, "top-level")
			adapter.SendJSON(tt.body)

			select {
			case msg := <-ch:
				var payload map[string]interface{}
				if err := json.Unmarshal([]byte(msg), &payload); err != nil {
					t.Fatalf("unmarshal event: %v", err)
				}
				if got := payload["session_id"]; got != "top-level" {
					t.Fatalf("top-level session_id = %#v, want top-level; event=%s", got, msg)
				}
			case <-time.After(time.Second):
				t.Fatal("no SSE event received")
			}
		})
	}
}

func TestSSEBrokerAdapterSendJSONPreservesExistingTopLevelSessionID(t *testing.T) {
	b := NewSSEBroadcaster()
	ch := b.subscribe()
	defer b.unsubscribe(ch)
	adapter := NewSSEBrokerAdapterWithSession(b, "adapter-session")
	const body = `{"event":"debug","session_id":"caller-session","detail":"unchanged"}`
	adapter.SendJSON(body)
	select {
	case msg := <-ch:
		if msg != body {
			t.Fatalf("event changed: got %s want %s", msg, body)
		}
	case <-time.After(time.Second):
		t.Fatal("no SSE event received")
	}
}
