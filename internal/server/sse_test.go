package server

import (
	"encoding/json"
	"testing"
	"time"
)

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
		{EventLLMStreamDelta, "llm_stream_delta"},
		{EventLLMStreamDone, "llm_stream_done"},
		{EventTokenUpdate, "token_update"},
		{EventToolCallPreview, "tool_call_preview"},
		{EventSystemMetrics, "system_metrics"},
	}
	for _, tt := range tests {
		if string(tt.evt) != tt.expected {
			t.Errorf("SSEEventType %v = %q, want %q", tt.evt, string(tt.evt), tt.expected)
		}
	}
}
