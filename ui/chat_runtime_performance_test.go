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
		"let _streamingScrollTimer = 0",
		"function flushStreamingBubble()",
		"function scheduleStreamingScroll()",
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
	flushBlock := sectionBetween(t, streaming, "function flushStreamingBubble()", "function queueStreamingBubbleFlush()")
	if strings.Contains(flushBlock, "chatBox.scrollTop = chatBox.scrollHeight") {
		t.Fatal("streaming bubble flush must not force synchronous scroll layout")
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

func TestChatMessageRenderingCachesMarkdownAndSkipsUnneededPostProcessing(t *testing.T) {
	t.Parallel()

	source := readEmbeddedText(t, "js/chat/chat-messages.js")
	for _, marker := range []string{
		"let cachedMarkdownRenderer = null",
		"function getMarkdownRenderer()",
		"cachedMarkdownRenderer = window.AuraChatCore.createMarkdownRenderer",
		"if (renderedBubble && shouldDecorateEmojiGlyphs(displayContent, finalHTML))",
		"if (window.MermaidLoader && messageMayContainMermaid(displayContent))",
		"if (window.ChatChartRenderer && newMessage && messageMayContainChart(displayContent))",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("chat message rendering optimization missing marker %q", marker)
		}
	}
	appendBody := sectionBetween(t, source, "function appendMessage(role, text, timestamp)", "function appendToolOutput(text, label)")
	if strings.Contains(appendBody, "window.AuraChatCore.createMarkdownRenderer({") {
		t.Fatal("appendMessage must use the cached markdown renderer instead of creating one per message")
	}
}

func TestSharedChatSanitizerReusesStaticState(t *testing.T) {
	t.Parallel()

	source := readEmbeddedText(t, "js/shared/chat-core.js")
	for _, marker := range []string{
		"const CHAT_SANITIZER_ALLOWED_TAGS = new Set",
		"const CHAT_SANITIZER_ALLOWED_ATTRS = new Set",
		"const chatSanitizeTemplate = document.createElement('template')",
		"if (html.indexOf('<') === -1) return html;",
		"CHAT_SANITIZER_ALLOWED_TAGS.has",
		"CHAT_SANITIZER_ALLOWED_ATTRS.has",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("shared chat sanitizer optimization missing marker %q", marker)
		}
	}
	body := sectionBetween(t, source, "function sanitizeRenderedHTML(html)", "function decorateEmojiGlyphs(root)")
	for _, forbidden := range []string{
		"new Set([",
		"document.createElement('template')",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("sanitizeRenderedHTML must not allocate %q per call", forbidden)
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
		"scheduleChatScroll(streamingBubble",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop chat missing single scroll scheduler marker %q", marker)
		}
	}

	keepStatus := sectionBetween(t, source, "function keepAgentStatusAtEnd()", "fetch('/api/desktop/chat/stream'")
	if strings.Contains(keepStatus, "scrollIntoView({ block: 'end', behavior: 'smooth' })") {
		t.Fatal("keepAgentStatusAtEnd must delegate smooth scrolling to scheduleChatScroll")
	}
	if strings.Contains(keepStatus, "scheduleChatScroll(statusEl") {
		t.Fatal("keepAgentStatusAtEnd must only maintain DOM order; streaming/status scrolls should be scheduled at semantic boundaries")
	}
}

func TestChatHeaderActivationAvoidsRedundantTouchListeners(t *testing.T) {
	t.Parallel()

	source := readEmbeddedText(t, "js/chat/main/state-dom.js")
	for _, marker := range []string{
		"const supportsPointerEvents = typeof window.PointerEvent !== 'undefined';",
		"if (supportsPointerEvents) {",
		"} else {",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("chat header activation missing pointer/touch feature split marker %q", marker)
		}
	}
	pointerBlock := sectionBetween(t, source, "if (supportsPointerEvents) {", "} else {")
	if strings.Contains(pointerBlock, "touchstart") || strings.Contains(pointerBlock, "touchmove") || strings.Contains(pointerBlock, "touchcancel") || strings.Contains(pointerBlock, "touchend") {
		t.Fatal("pointer-event capable browsers must not bind duplicate touch listeners")
	}
}

func TestSharedChatStreamParserIsUsedByDesktopChat(t *testing.T) {
	t.Parallel()

	parser := readEmbeddedText(t, "js/shared/chat-stream-parser.js")
	for _, marker := range []string{
		"window.AuraChatStreamParser",
		"async function readFetchEventStream(response, handlers = {})",
		"function normalizeStreamEvent(data)",
		"handlers.onEvent(normalizeStreamEvent(parsed))",
		"handlers.onDone()",
	} {
		if !strings.Contains(parser, marker) {
			t.Fatalf("shared chat stream parser missing marker %q", marker)
		}
	}

	desktopChat := readEmbeddedText(t, "js/desktop/apps/agent-chat.js")
	for _, marker := range []string{
		"window.AuraChatStreamParser.readFetchEventStream",
		"handleStreamEvent(eventData)",
	} {
		if !strings.Contains(desktopChat, marker) {
			t.Fatalf("desktop chat must use shared stream parser marker %q", marker)
		}
	}
	if strings.Contains(desktopChat, "const lines = buffer.split('\\n')") {
		t.Fatal("desktop chat must not keep manual SSE line parsing after parser extraction")
	}
}
