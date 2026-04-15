package server

import (
	"encoding/json"

	"aurago/internal/security"
)

type SSEBrokerAdapter struct {
	sse       *SSEBroadcaster
	sessionID string
}

func NewSSEBrokerAdapter(sse *SSEBroadcaster) *SSEBrokerAdapter {
	return &SSEBrokerAdapter{sse: sse}
}

func NewSSEBrokerAdapterWithSession(sse *SSEBroadcaster, sessionID string) *SSEBrokerAdapter {
	return &SSEBrokerAdapter{sse: sse, sessionID: sessionID}
}

func (b *SSEBrokerAdapter) Send(event, message string) {
	if b.sessionID != "" {
		payload, _ := json.Marshal(struct {
			Event     string `json:"event"`
			Detail    string `json:"detail"`
			SessionID string `json:"session_id,omitempty"`
		}{event, message, b.sessionID})
		b.sse.SendJSON(string(payload))
	} else {
		b.sse.Send(event, message)
	}
}

func (b *SSEBrokerAdapter) SendJSON(jsonStr string) {
	if b.sessionID == "" {
		b.sse.SendJSON(jsonStr)
		return
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(jsonStr), &raw); err == nil {
		if _, exists := raw["session_id"]; !exists {
			sid, _ := json.Marshal(b.sessionID)
			raw["session_id"] = sid
			if enriched, err := json.Marshal(raw); err == nil {
				b.sse.SendJSON(string(enriched))
				return
			}
		}
	}
	b.sse.SendJSON(jsonStr)
}

func (b *SSEBrokerAdapter) SendLLMStreamDelta(content, toolName, toolID string, index int, finishReason string) {
	b.sse.BroadcastType(EventLLMStreamDelta, LLMStreamDeltaPayload{
		SessionID:    b.sessionID,
		Content:      content,
		ToolName:     toolName,
		ToolID:       toolID,
		Index:        index,
		FinishReason: finishReason,
	})
}

func (b *SSEBrokerAdapter) SendLLMStreamDone(finishReason string) {
	b.sse.BroadcastType(EventLLMStreamDone, LLMStreamDonePayload{
		SessionID:    b.sessionID,
		FinishReason: finishReason,
	})
}

func (b *SSEBrokerAdapter) SendTokenUpdate(prompt, completion, total, sessionTotal, globalTotal int, isEstimated, isFinal bool, source string) {
	b.sse.BroadcastType(EventTokenUpdate, TokenUpdatePayload{
		SessionID:        b.sessionID,
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      total,
		SessionTotal:     sessionTotal,
		GlobalTotal:      globalTotal,
		IsEstimated:      isEstimated,
		IsFinal:          isFinal,
		Source:           source,
	})
}

func (b *SSEBrokerAdapter) SendThinkingBlock(provider, content, state string) {
	b.sse.BroadcastType(EventThinkingBlock, ThinkingBlockPayload{
		SessionID: b.sessionID,
		Provider:  provider,
		Content:   content,
		State:     state,
	})
}

func (b *SSEBrokerAdapter) Scrub(s string) string {
	return security.Scrub(s)
}
