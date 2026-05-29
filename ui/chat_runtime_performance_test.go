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

func TestSmartScrollerDebouncesMutationObserver(t *testing.T) {
	t.Parallel()

	source := readEmbeddedText(t, "js/chat/modules/smart-scroller.js")
	for _, marker := range []string{
		"mutationScrollDelay: 50",
		"scheduleObservedScrollCheck()",
		"this._mutationScrollTimer",
		"clearTimeout(this._mutationScrollTimer)",
		"this._mutationObserver = new MutationObserver(() => this.scheduleObservedScrollCheck())",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("smart scroller missing observer debounce marker %q", marker)
		}
	}
	if strings.Contains(source, "new MutationObserver(() => this.onScroll())") {
		t.Fatal("smart scroller observer must not call onScroll synchronously")
	}
}

func TestChatMediaLinkReplacementUsesReusableTemplatesAndFastPaths(t *testing.T) {
	t.Parallel()

	source := readEmbeddedText(t, "js/chat/chat-messages.js")
	for _, marker := range []string{
		"const chatVideoLinkTemplate =",
		"const chatYouTubeLinkTemplate =",
		"function hasVideoLinkCandidate(html)",
		"function hasYouTubeLinkCandidate(html)",
		"if (!hasVideoLinkCandidate(html)) return html;",
		"if (!hasYouTubeLinkCandidate(html)) return html;",
		"chatVideoLinkTemplate.innerHTML = html;",
		"chatYouTubeLinkTemplate.innerHTML = html;",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("chat media link replacement missing fast-path marker %q", marker)
		}
	}
}

func TestDesktopChatUsesSingleScrollScheduler(t *testing.T) {
	t.Parallel()

	source := readEmbeddedText(t, "js/desktop/apps/agent-chat.js")
	for _, marker := range []string{
		"let chatScrollFrame = 0",
		"let pendingScrollTarget = null",
		"function scheduleChatScroll(target, smooth = true)",
		"window.requestAnimationFrame ||",
		"pendingScrollTarget.scrollIntoView",
		"scheduleChatScroll(statusEl",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop chat missing single scroll scheduler marker %q", marker)
		}
	}

	keepStatus := sectionBetween(t, source, "function keepAgentStatusAtEnd()", "fetch('/api/desktop/chat/stream'")
	if strings.Contains(keepStatus, "scrollIntoView({ block: 'end', behavior: 'smooth' })") {
		t.Fatal("keepAgentStatusAtEnd must delegate smooth scrolling to scheduleChatScroll")
	}
}
