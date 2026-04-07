package server

import "aurago/internal/security"

type SSEBrokerAdapter struct {
	sse *SSEBroadcaster
}

func NewSSEBrokerAdapter(sse *SSEBroadcaster) *SSEBrokerAdapter {
	return &SSEBrokerAdapter{sse: sse}
}

func (b *SSEBrokerAdapter) Send(event, message string) {
	b.sse.Send(event, message)
}

func (b *SSEBrokerAdapter) SendJSON(jsonStr string) {
	b.sse.SendJSON(jsonStr)
}

func (b *SSEBrokerAdapter) SendLLMStreamDelta(content, toolName, toolID string, index int, finishReason string) {
	b.sse.BroadcastType(EventLLMStreamDelta, LLMStreamDeltaPayload{
		Content:      content,
		ToolName:     toolName,
		ToolID:       toolID,
		Index:        index,
		FinishReason: finishReason,
	})
}

func (b *SSEBrokerAdapter) SendLLMStreamDone(finishReason string) {
	b.sse.BroadcastType(EventLLMStreamDone, LLMStreamDonePayload{
		FinishReason: finishReason,
	})
}

func (b *SSEBrokerAdapter) SendTokenUpdate(prompt, completion, total, sessionTotal, globalTotal int, isEstimated, isFinal bool, source string) {
	b.sse.BroadcastType(EventTokenUpdate, TokenUpdatePayload{
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
		Provider: provider,
		Content:  content,
		State:    state,
	})
}

func (b *SSEBrokerAdapter) Scrub(s string) string {
	return security.Scrub(s)
}
