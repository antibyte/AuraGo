package ui

import (
	"strings"
	"testing"
)

func sectionBetween(t *testing.T, source, start, end string) string {
	t.Helper()
	startAt := strings.Index(source, start)
	if startAt < 0 {
		t.Fatalf("missing start marker %q", start)
	}
	rest := source[startAt:]
	endAt := strings.Index(rest, end)
	if endAt < 0 {
		t.Fatalf("missing end marker %q after %q", end, start)
	}
	return rest[:endAt]
}

func TestChatStreamingBatchesDOMWritesAndResetsSSEDedup(t *testing.T) {
	t.Parallel()

	streaming := readEmbeddedText(t, "js/chat/chat-streaming.js")
	for _, marker := range []string{
		"let _streamingFlushFrame = 0",
		"function flushStreamingBubble()",
		"function queueStreamingBubbleFlush()",
		"window.requestAnimationFrame ||",
		"resetSSEDedupSets();",
		"llm_stream_done",
		"data.event === 'done'",
	} {
		if !strings.Contains(streaming, marker) {
			t.Fatalf("chat streaming runtime missing batching/reset marker %q", marker)
		}
	}

	deltaBlock := sectionBetween(t, streaming, "window.AuraSSE.on('llm_stream_delta'", "window.AuraSSE.on('llm_stream_done'")
	for _, forbidden := range []string{
		"window.decorateEmojiGlyphs",
		"chatBox.scrollTop = chatBox.scrollHeight",
	} {
		if strings.Contains(deltaBlock, forbidden) {
			t.Fatalf("llm_stream_delta hot path must not contain %q", forbidden)
		}
	}

	state := readEmbeddedText(t, "js/chat/main/feedback-audio-plan.js")
	if !strings.Contains(state, "function resetSSEDedupSets()") {
		t.Fatal("chat state must expose resetSSEDedupSets for SSE and HTTP completion paths")
	}
}
