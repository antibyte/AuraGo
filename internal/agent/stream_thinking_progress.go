package agent

import (
	"context"
	"sync/atomic"

	"aurago/internal/llm"
)

// Anthropic keeps signed thinking out of ordinary deltas until its complete
// envelope is available. Count the live callback without changing that history.
type streamThinkingProgress struct {
	ready chan struct{}
	chars atomic.Int64
}

func withStreamThinkingProgress(ctx context.Context, broker FeedbackBroker) (context.Context, *streamThinkingProgress) {
	p := &streamThinkingProgress{ready: make(chan struct{}, 1)}
	return llm.WithThinkingCallback(ctx, func(content, state string) {
		if state == "delta" && content != "" {
			p.chars.Add(int64(len(content)))
			llm.ObserveStreamOutput(ctx, len(content))
			select {
			case p.ready <- struct{}{}:
			default:
			}
		}
		if broker != nil {
			broker.SendThinkingBlock("anthropic", content, state)
		}
	}), p
}

func (p *streamThinkingProgress) reasoningChars(reasoning string) int {
	text := llm.ReasoningText(reasoning)
	if text != reasoning {
		// The complete opaque envelope repeats the live callback's text. Its
		// encoded bytes and already measured text must not extend the budget.
		return max(0, len(text)-int(p.chars.Swap(0)))
	}
	return len(text)
}
