package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"aurago/internal/security"
)

const sseScrubTestSecret = "rtsp://admin:S3cr3tPw@cam.local/live?channel=1&subtype=0<x>"

func receiveSSEScrubMessage(t *testing.T, ch chan string) string {
	t.Helper()
	select {
	case msg := <-ch:
		return msg
	case <-time.After(time.Second):
		t.Fatal("no SSE message received")
		return ""
	}
}

func assertSSEMessageScrubbed(t *testing.T, msg string) {
	t.Helper()
	var decoded any
	if err := json.Unmarshal([]byte(msg), &decoded); err != nil {
		t.Fatalf("scrubbed SSE message is not valid JSON: %v; msg=%s", err, msg)
	}
	if flat := fmt.Sprint(decoded); strings.Contains(flat, "S3cr3tPw") {
		t.Fatalf("secret leaked through SSE: %s", flat)
	}
}

func TestSSEScrubRedactsSecretsThatJSONEscapes(t *testing.T) {
	t.Cleanup(security.RegisterScopedSensitiveExact(sseScrubTestSecret))
	b := NewSSEBroadcaster()
	ch := b.subscribe("sess-scrub")
	defer b.unsubscribe(ch)

	b.BroadcastType(EventToolCallPreview, ToolCallPreviewPayload{Action: "camera", RawJSON: "src=" + sseScrubTestSecret})
	assertSSEMessageScrubbed(t, receiveSSEScrubMessage(t, ch))

	b.BroadcastToSession("sess-scrub", EventAgentError, AgentErrorPayload{Code: "camera_failed", Message: "failed: " + sseScrubTestSecret, Status: 500})
	assertSSEMessageScrubbed(t, receiveSSEScrubMessage(t, ch))

	NewSSEBrokerAdapterWithSession(b, "sess-scrub").SendTyped("agent_action", map[string]interface{}{"detail": sseScrubTestSecret})
	assertSSEMessageScrubbed(t, receiveSSEScrubMessage(t, ch))

	escaped, err := json.Marshal(sseScrubTestSecret)
	if err != nil {
		t.Fatal(err)
	}
	b.SendJSON(`{"event":"debug","detail":` + string(escaped) + `}`)
	assertSSEMessageScrubbed(t, receiveSSEScrubMessage(t, ch))
}

func TestSSEScrubKeepsJSONValidForNumericSecrets(t *testing.T) {
	t.Cleanup(security.RegisterScopedSensitiveExact("12345678901"))
	b := NewSSEBroadcaster()
	ch := b.subscribe()
	defer b.unsubscribe(ch)

	b.SendJSON(`{"event":"pin","value":12345678901}`)
	msg := receiveSSEScrubMessage(t, ch)
	var decoded map[string]any
	if err := json.Unmarshal([]byte(msg), &decoded); err != nil {
		t.Fatalf("numeric redaction produced invalid JSON: %v; msg=%s", err, msg)
	}
	if strings.Contains(msg, "12345678901") {
		t.Fatalf("numeric secret leaked: %s", msg)
	}
}

func TestSSEScrubKeepsMessagesWithoutSecretsByteIdentical(t *testing.T) {
	b := NewSSEBroadcaster()
	ch := b.subscribe()
	defer b.unsubscribe(ch)
	const body = `{"type":"budget_update","session_id":"s","percentage":0.5,"nested":{"b":2,"a":[1,"x"]}}`
	b.SendJSON(body)
	if got := receiveSSEScrubMessage(t, ch); got != body {
		t.Fatalf("message without secrets changed:\n got %s\nwant %s", got, body)
	}
}
