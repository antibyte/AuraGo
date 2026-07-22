package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type failingTypedResponseWriter struct {
	header http.Header
}

func (w *failingTypedResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *failingTypedResponseWriter) Write([]byte) (int, error) {
	return 0, errors.New("forced write failure")
}

func (w *failingTypedResponseWriter) WriteHeader(int) {}
func (w *failingTypedResponseWriter) Flush()          {}

func TestBroadcastTypeProducesTypedEvent(t *testing.T) {
	b := NewSSEBroadcaster()
	ch := b.subscribe()

	b.BroadcastType(EventLLMStreamDelta, LLMStreamDeltaPayload{
		Content:  "Hello",
		Index:    0,
		ToolName: "test_tool",
	})

	gotMsg := false
	for i := 0; i < 10; i++ {
		select {
		case msg := <-ch:
			var evt struct {
				Type    SSEEventType          `json:"type"`
				Payload LLMStreamDeltaPayload `json:"payload"`
			}
			if err := json.Unmarshal([]byte(msg), &evt); err != nil {
				t.Fatalf("failed to unmarshal typed event: %v", err)
			}
			if evt.Type != EventLLMStreamDelta {
				t.Fatalf("event type = %q, want %q", evt.Type, EventLLMStreamDelta)
			}
			if evt.Payload.Content != "Hello" {
				t.Fatalf("payload content = %q, want %q", evt.Payload.Content, "Hello")
			}
			if evt.Payload.Index != 0 {
				t.Fatalf("payload index = %d, want 0", evt.Payload.Index)
			}
			if evt.Payload.ToolName != "test_tool" {
				t.Fatalf("payload tool_name = %q, want %q", evt.Payload.ToolName, "test_tool")
			}
			gotMsg = true
		default:
			time.Sleep(5 * time.Millisecond)
		}
		if gotMsg {
			break
		}
	}
	if !gotMsg {
		t.Fatal("no message received from broadcaster")
	}
	b.unsubscribe(ch)
}

func TestSSEBrokerAdapterSendTypedAddsSessionToPayload(t *testing.T) {
	b := NewSSEBroadcaster()
	ch := b.subscribe("sess-typed")
	defer b.unsubscribe(ch)

	adapter := NewSSEBrokerAdapterWithSession(b, "sess-typed")
	if !adapter.SendTyped(string(EventAgentAction), map[string]interface{}{"state": "started", "tool_name": "execute_shell"}) {
		t.Fatal("SendTyped returned false")
	}

	for i := 0; i < 10; i++ {
		select {
		case msg := <-ch:
			var evt struct {
				Type    SSEEventType           `json:"type"`
				Payload map[string]interface{} `json:"payload"`
			}
			if err := json.Unmarshal([]byte(msg), &evt); err != nil {
				t.Fatalf("failed to unmarshal typed event: %v", err)
			}
			if evt.Type != EventAgentAction {
				t.Fatalf("event type = %q, want %q", evt.Type, EventAgentAction)
			}
			if evt.Payload["session_id"] != "sess-typed" {
				t.Fatalf("payload session_id = %#v, want sess-typed", evt.Payload["session_id"])
			}
			if evt.Payload["state"] != "started" || evt.Payload["tool_name"] != "execute_shell" {
				t.Fatalf("payload = %#v", evt.Payload)
			}
			return
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
	t.Fatal("no typed message received from broker adapter")
}

func TestSSEBrokerAdapterSendTypedRequiresMatchingSession(t *testing.T) {
	t.Run("no clients", func(t *testing.T) {
		b := NewSSEBroadcaster()
		adapter := NewSSEBrokerAdapterWithSession(b, "sess-target")
		if adapter.SendTyped("operational_issue_notice", map[string]interface{}{"text": "notice"}) {
			t.Fatal("SendTyped reported delivery without a client")
		}
	})

	t.Run("different and legacy clients", func(t *testing.T) {
		b := NewSSEBroadcaster()
		foreign := b.subscribe("sess-other")
		legacy := b.subscribe()
		defer b.unsubscribe(foreign)
		defer b.unsubscribe(legacy)
		adapter := NewSSEBrokerAdapterWithSession(b, "sess-target")
		if adapter.SendTyped("operational_issue_notice", map[string]interface{}{"text": "notice"}) {
			t.Fatal("SendTyped reported delivery to a non-matching client")
		}
		select {
		case msg := <-foreign:
			t.Fatalf("foreign session received targeted event: %s", msg)
		default:
		}
		select {
		case msg := <-legacy:
			t.Fatalf("legacy client received targeted event: %s", msg)
		default:
		}
	})

	t.Run("matching client", func(t *testing.T) {
		b := NewSSEBroadcaster()
		matching := b.subscribe("sess-target")
		defer b.unsubscribe(matching)
		adapter := NewSSEBrokerAdapterWithSession(b, "sess-target")
		if !adapter.SendTyped("operational_issue_notice", map[string]interface{}{"text": "notice"}) {
			t.Fatal("SendTyped did not report matching session delivery")
		}
		select {
		case <-matching:
		case <-time.After(time.Second):
			t.Fatal("matching session did not receive targeted event")
		}
	})

	t.Run("full matching client", func(t *testing.T) {
		b := NewSSEBroadcaster()
		matching := b.subscribe("sess-target")
		for i := 0; i < cap(matching); i++ {
			matching <- "full"
		}
		adapter := NewSSEBrokerAdapterWithSession(b, "sess-target")
		if adapter.SendTyped("operational_issue_notice", map[string]interface{}{"text": "notice"}) {
			t.Fatal("SendTyped reported delivery to a full client")
		}
		if got := b.ClientCount(); got != 0 {
			t.Fatalf("full client was not removed: count=%d", got)
		}
	})
}

func TestTypedDirectStreamDeliveryChecksWriterResult(t *testing.T) {
	t.Run("desktop direct stream", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		broker := &desktopStreamCombinedBroker{
			stream:    &desktopStreamBroker{w: recorder, flusher: recorder, canFlush: true},
			sessionID: "sess-direct",
		}
		delivered, transport := broker.SendTypedWithTransport("operational_issue_notice", map[string]interface{}{"text": "notice"})
		if !delivered || transport != "direct_stream" || !strings.Contains(recorder.Body.String(), "operational_issue_notice") {
			t.Fatalf("desktop delivery = %v/%q body=%q", delivered, transport, recorder.Body.String())
		}
	})

	t.Run("desktop writer failure", func(t *testing.T) {
		writer := &failingTypedResponseWriter{}
		stream := &desktopStreamBroker{w: writer, flusher: writer, canFlush: true}
		broker := &desktopStreamCombinedBroker{stream: stream, sessionID: "sess-direct"}
		if delivered, _ := broker.SendTypedWithTransport("operational_issue_notice", map[string]interface{}{"text": "notice"}); delivered {
			t.Fatal("desktop writer failure reported delivery")
		}
		if !stream.closed {
			t.Fatal("desktop stream was not closed after write failure")
		}
	})

	t.Run("desktop falls back to matching SSE when direct writer fails", func(t *testing.T) {
		broadcaster := NewSSEBroadcaster()
		client := broadcaster.subscribe("sess-direct")
		t.Cleanup(func() { broadcaster.unsubscribe(client) })
		writer := &failingTypedResponseWriter{}
		stream := &desktopStreamBroker{w: writer, flusher: writer, canFlush: true}
		broker := &desktopStreamCombinedBroker{
			stream:    stream,
			sse:       NewSSEBrokerAdapterWithSession(broadcaster, "sess-direct"),
			sessionID: "sess-direct",
		}
		delivered, transport := broker.SendTypedWithTransport("operational_issue_notice", map[string]interface{}{"text": "notice"})
		if !delivered || transport != "typed_session" {
			t.Fatalf("SSE fallback delivery = %v/%q", delivered, transport)
		}
		select {
		case <-client:
		default:
			t.Fatal("matching SSE client did not receive fallback event")
		}
	})

	t.Run("realtime writer failure", func(t *testing.T) {
		writer := &failingTypedResponseWriter{}
		broker, err := newRealtimeSpeechActionResponseBroker(writer, "sess-realtime")
		if err != nil {
			t.Fatalf("newRealtimeSpeechActionResponseBroker: %v", err)
		}
		if delivered, _ := broker.SendTypedWithTransport("operational_issue_notice", map[string]interface{}{"text": "notice"}); delivered {
			t.Fatal("realtime writer failure reported delivery")
		}
	})
}

func TestSSEBrokerAdapterSendJSONAddsSessionToTypedPayload(t *testing.T) {
	b := NewSSEBroadcaster()
	ch := b.subscribe()
	defer b.unsubscribe(ch)

	adapter := NewSSEBrokerAdapterWithSession(b, "sess-question")
	adapter.SendJSON(`{"type":"question_user","payload":{"id":"q1","question":"Continue?"}}`)

	for i := 0; i < 10; i++ {
		select {
		case msg := <-ch:
			var evt struct {
				Type      SSEEventType           `json:"type"`
				SessionID string                 `json:"session_id"`
				Payload   map[string]interface{} `json:"payload"`
			}
			if err := json.Unmarshal([]byte(msg), &evt); err != nil {
				t.Fatalf("failed to unmarshal typed event: %v", err)
			}
			if evt.Type != "question_user" {
				t.Fatalf("event type = %q, want question_user", evt.Type)
			}
			if evt.SessionID != "sess-question" {
				t.Fatalf("envelope session_id = %q, want sess-question", evt.SessionID)
			}
			if evt.Payload["session_id"] != "sess-question" {
				t.Fatalf("payload session_id = %#v, want sess-question", evt.Payload["session_id"])
			}
			return
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
	t.Fatal("no typed JSON message received from broker adapter")
}

func TestBroadcastTypeLLMStreamDone(t *testing.T) {
	b := NewSSEBroadcaster()
	ch := b.subscribe()

	b.BroadcastType(EventLLMStreamDone, LLMStreamDonePayload{
		FinishReason: "stop",
	})

	gotMsg := false
	for i := 0; i < 10; i++ {
		select {
		case msg := <-ch:
			var evt struct {
				Type    SSEEventType         `json:"type"`
				Payload LLMStreamDonePayload `json:"payload"`
			}
			if err := json.Unmarshal([]byte(msg), &evt); err != nil {
				t.Fatalf("failed to unmarshal typed event: %v", err)
			}
			if evt.Type != EventLLMStreamDone {
				t.Fatalf("event type = %q, want %q", evt.Type, EventLLMStreamDone)
			}
			if evt.Payload.FinishReason != "stop" {
				t.Fatalf("payload finish_reason = %q, want %q", evt.Payload.FinishReason, "stop")
			}
			gotMsg = true
		default:
			time.Sleep(5 * time.Millisecond)
		}
		if gotMsg {
			break
		}
	}
	if !gotMsg {
		t.Fatal("no message received from broadcaster")
	}
	b.unsubscribe(ch)
}

func TestBroadcastTypeTokenUpdate(t *testing.T) {
	b := NewSSEBroadcaster()
	ch := b.subscribe()

	b.BroadcastType(EventTokenUpdate, TokenUpdatePayload{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		SessionTotal:     300,
		GlobalTotal:      5000,
		IsEstimated:      false,
	})

	gotMsg := false
	for i := 0; i < 10; i++ {
		select {
		case msg := <-ch:
			var evt struct {
				Type    SSEEventType       `json:"type"`
				Payload TokenUpdatePayload `json:"payload"`
			}
			if err := json.Unmarshal([]byte(msg), &evt); err != nil {
				t.Fatalf("failed to unmarshal typed event: %v", err)
			}
			if evt.Type != EventTokenUpdate {
				t.Fatalf("event type = %q, want %q", evt.Type, EventTokenUpdate)
			}
			if evt.Payload.TotalTokens != 150 {
				t.Fatalf("payload total = %d, want 150", evt.Payload.TotalTokens)
			}
			if evt.Payload.IsEstimated {
				t.Fatal("payload is_estimated = true, want false")
			}
			gotMsg = true
		default:
			time.Sleep(5 * time.Millisecond)
		}
		if gotMsg {
			break
		}
	}
	if !gotMsg {
		t.Fatal("no message received from broadcaster")
	}
	b.unsubscribe(ch)
}

func TestBroadcastTypeTokenUpdate_WithFinalAndSource(t *testing.T) {
	b := NewSSEBroadcaster()
	ch := b.subscribe()

	b.BroadcastType(EventTokenUpdate, TokenUpdatePayload{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		SessionTotal:     300,
		GlobalTotal:      5000,
		IsEstimated:      false,
		IsFinal:          true,
		Source:           "provider_usage",
	})

	gotMsg := false
	for i := 0; i < 10; i++ {
		select {
		case msg := <-ch:
			var evt struct {
				Type    SSEEventType       `json:"type"`
				Payload TokenUpdatePayload `json:"payload"`
			}
			if err := json.Unmarshal([]byte(msg), &evt); err != nil {
				t.Fatalf("failed to unmarshal typed event: %v", err)
			}
			if evt.Type != EventTokenUpdate {
				t.Fatalf("event type = %q, want %q", evt.Type, EventTokenUpdate)
			}
			if !evt.Payload.IsFinal {
				t.Error("payload is_final = false, want true")
			}
			if evt.Payload.Source != "provider_usage" {
				t.Errorf("payload source = %q, want %q", evt.Payload.Source, "provider_usage")
			}
			if evt.Payload.TotalTokens != 150 {
				t.Fatalf("payload total = %d, want 150", evt.Payload.TotalTokens)
			}
			gotMsg = true
		default:
			time.Sleep(5 * time.Millisecond)
		}
		if gotMsg {
			break
		}
	}
	if !gotMsg {
		t.Fatal("no message received from broadcaster")
	}
	b.unsubscribe(ch)
}

func TestBroadcastTypeTokenUpdate_FallbackEstimateSource(t *testing.T) {
	b := NewSSEBroadcaster()
	ch := b.subscribe()

	b.BroadcastType(EventTokenUpdate, TokenUpdatePayload{
		PromptTokens:     80,
		CompletionTokens: 120,
		TotalTokens:      200,
		SessionTotal:     500,
		GlobalTotal:      5000,
		IsEstimated:      true,
		IsFinal:          true,
		Source:           "fallback_estimate",
	})

	gotMsg := false
	for i := 0; i < 10; i++ {
		select {
		case msg := <-ch:
			var evt struct {
				Type    SSEEventType       `json:"type"`
				Payload TokenUpdatePayload `json:"payload"`
			}
			if err := json.Unmarshal([]byte(msg), &evt); err != nil {
				t.Fatalf("failed to unmarshal typed event: %v", err)
			}
			if evt.Type != EventTokenUpdate {
				t.Fatalf("event type = %q, want %q", evt.Type, EventTokenUpdate)
			}
			if !evt.Payload.IsFinal {
				t.Error("payload is_final = false, want true")
			}
			if evt.Payload.Source != "fallback_estimate" {
				t.Errorf("payload source = %q, want %q", evt.Payload.Source, "fallback_estimate")
			}
			if !evt.Payload.IsEstimated {
				t.Error("payload is_estimated = false, want true for fallback_estimate")
			}
			if evt.Payload.TotalTokens != 200 {
				t.Fatalf("payload total = %d, want 200", evt.Payload.TotalTokens)
			}
			gotMsg = true
		default:
			time.Sleep(5 * time.Millisecond)
		}
		if gotMsg {
			break
		}
	}
	if !gotMsg {
		t.Fatal("no message received from broadcaster")
	}
	b.unsubscribe(ch)
}

func TestSSEEventTypeConstants(t *testing.T) {
	tests := []struct {
		evt      SSEEventType
		expected string
	}{
		{EventAuditUpdate, "audit_update"},
		{EventLLMStreamDelta, "llm_stream_delta"},
		{EventLLMStreamDone, "llm_stream_done"},
		{EventTokenUpdate, "token_update"},
		{EventToolCallPreview, "tool_call_preview"},
		{EventAgentAction, "agent_action"},
		{EventSystemMetrics, "system_metrics"},
	}
	for _, tt := range tests {
		if string(tt.evt) != tt.expected {
			t.Errorf("SSEEventType %v = %q, want %q", tt.evt, string(tt.evt), tt.expected)
		}
	}
}

func TestSSEServeHTTPSetsStreamingHeaders(t *testing.T) {
	b := NewSSEBroadcaster()
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/events", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		b.ServeHTTP(rec, req)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ServeHTTP did not stop after context cancellation")
	}

	res := rec.Result()
	if got := res.Header.Get("Content-Type"); got != "text/event-stream; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want %q", got, "text/event-stream; charset=utf-8")
	}
	if got := res.Header.Get("Cache-Control"); got != "no-cache, no-store, must-revalidate, private" {
		t.Fatalf("Cache-Control = %q, want hardened SSE cache policy", got)
	}
	if got := res.Header.Get("X-Accel-Buffering"); got != "no" {
		t.Fatalf("X-Accel-Buffering = %q, want %q", got, "no")
	}
	if body := rec.Body.String(); !strings.Contains(body, ": ping\n\n") {
		t.Fatalf("initial SSE ping missing from body: %q", body)
	}
}

func TestSSEServeHTTPReturnsWhenClientChannelCloses(t *testing.T) {
	b := NewSSEBroadcaster()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest("GET", "/events", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		b.ServeHTTP(rec, req)
		close(done)
	}()

	var client chan string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		b.mu.RLock()
		for ch := range b.clients {
			client = ch
			break
		}
		b.mu.RUnlock()
		if client != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if client == nil {
		t.Fatal("timed out waiting for SSE client subscription")
	}

	b.unsubscribe(client)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("ServeHTTP did not stop after client channel closed")
	}
}
