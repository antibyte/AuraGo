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

// SendPreparedJSON forwards a JSON string that is already final (e.g. already
// contains a session_id). It avoids the expensive unmarshal/marshal round-trip
// performed by SendJSON. Prefer this for hot paths that construct payloads
// with the session_id baked in.
func (b *SSEBrokerAdapter) SendPreparedJSON(jsonStr string) {
	b.sse.SendJSON(jsonStr)
}

func (b *SSEBrokerAdapter) SendJSON(jsonStr string) {
	if b.sessionID == "" {
		b.sse.SendJSON(jsonStr)
		return
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(jsonStr), &raw); err == nil {
		// Only a structurally present top-level session_id makes this payload
		// final. Nested IDs and text fragments still require enrichment.
		if _, exists := raw["session_id"]; exists {
			b.sse.SendJSON(jsonStr)
			return
		}
		changed := false
		if _, exists := raw["session_id"]; !exists {
			sid, _ := json.Marshal(b.sessionID)
			raw["session_id"] = sid
			changed = true
		}
		if enrichTypedEnvelopePayloadWithSessionID(raw, b.sessionID) {
			changed = true
		}
		if changed {
			if enriched, err := json.Marshal(raw); err == nil {
				b.sse.SendJSON(string(enriched))
				return
			}
		}
	}
	b.sse.SendJSON(jsonStr)
}

func enrichTypedEnvelopePayloadWithSessionID(raw map[string]json.RawMessage, sessionID string) bool {
	if sessionID == "" {
		return false
	}
	if _, ok := raw["type"]; !ok {
		return false
	}
	payloadRaw, ok := raw["payload"]
	if !ok {
		return false
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(payloadRaw, &payload); err != nil {
		return false
	}
	if _, exists := payload["session_id"]; exists {
		return false
	}
	sid, _ := json.Marshal(sessionID)
	payload["session_id"] = sid
	enriched, err := json.Marshal(payload)
	if err != nil {
		return false
	}
	raw["payload"] = enriched
	return true
}

func (b *SSEBrokerAdapter) SendTyped(eventType string, payload interface{}) bool {
	if b == nil || b.sse == nil || eventType == "" {
		return false
	}
	enriched := enrichPayloadWithSessionID(payload, b.sessionID)
	msg, err := encodeTypedSSEEvent(eventType, enriched)
	if err != nil {
		return false
	}
	b.sse.SendJSON(msg)
	return true
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

func enrichPayloadWithSessionID(payload interface{}, sessionID string) interface{} {
	if sessionID == "" {
		return payload
	}
	switch v := payload.(type) {
	case map[string]interface{}:
		if _, ok := v["session_id"]; ok {
			return payload
		}
		enriched := make(map[string]interface{}, len(v)+1)
		for key, value := range v {
			enriched[key] = value
		}
		enriched["session_id"] = sessionID
		return enriched
	case map[string]json.RawMessage:
		if _, ok := v["session_id"]; ok {
			return payload
		}
		enriched := make(map[string]json.RawMessage, len(v)+1)
		for key, value := range v {
			enriched[key] = value
		}
		sid, _ := json.Marshal(sessionID)
		enriched["session_id"] = sid
		return enriched
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return payload
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return payload
	}
	if _, ok := obj["session_id"]; ok {
		return obj
	}
	sid, _ := json.Marshal(sessionID)
	obj["session_id"] = sid
	return obj
}

func encodeTypedSSEEvent(eventType string, payload interface{}) (string, error) {
	msg, err := json.Marshal(struct {
		Type    SSEEventType `json:"type"`
		Payload interface{}  `json:"payload"`
	}{SSEEventType(eventType), payload})
	if err != nil {
		return "", err
	}
	return string(msg), nil
}
