package server

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSSEBrokerAdapterSendJSONFastPathPreservesSessionID(t *testing.T) {
	t.Parallel()

	b := NewSSEBroadcaster()
	ch := b.subscribe()
	defer b.unsubscribe(ch)

	adapter := NewSSEBrokerAdapterWithSession(b, "sess-fast")
	// Payload already contains session_id at both envelope and payload level.
	adapter.SendJSON(`{"type":"agent_action","session_id":"sess-fast","payload":{"action":"start","session_id":"sess-fast"}}`)

	for i := 0; i < 10; i++ {
		select {
		case msg := <-ch:
			var evt struct {
				Type      SSEEventType           `json:"type"`
				SessionID string                 `json:"session_id"`
				Payload   map[string]interface{} `json:"payload"`
			}
			if err := json.Unmarshal([]byte(msg), &evt); err != nil {
				t.Fatalf("failed to unmarshal event: %v", err)
			}
			if evt.SessionID != "sess-fast" {
				t.Fatalf("envelope session_id = %q, want sess-fast", evt.SessionID)
			}
			if evt.Payload["session_id"] != "sess-fast" {
				t.Fatalf("payload session_id = %#v, want sess-fast", evt.Payload["session_id"])
			}
			return
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
	t.Fatal("no message received from broker adapter")
}

func TestSSEBrokerAdapterSendPreparedJSONSkipsEnrichment(t *testing.T) {
	t.Parallel()

	b := NewSSEBroadcaster()
	ch := b.subscribe()
	defer b.unsubscribe(ch)

	adapter := NewSSEBrokerAdapterWithSession(b, "sess-prepared")
	// SendPreparedJSON must not touch the string at all.
	adapter.SendPreparedJSON(`{"type":"budget_update","session_id":"sess-prepared","percentage":0.5}`)

	for i := 0; i < 10; i++ {
		select {
		case msg := <-ch:
			if msg != `{"type":"budget_update","session_id":"sess-prepared","percentage":0.5}` {
				t.Fatalf("unexpected message: %s", msg)
			}
			return
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
	t.Fatal("no message received from broker adapter")
}
