# Frontend Runtime Optimization Chat Desktop Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the remaining runtime-performance and memory-stability fixes from `reports/frontend_runtime_optimierung_chat_desktop_2026-05-30.md` for the chat UI and virtual desktop without redoing optimizations that are already present in the current tree.

**Architecture:** Keep the current vanilla JavaScript plus generated-bundle architecture. Make source edits in split source files under `ui/js/**`, add static Go regression tests under `ui/`, then regenerate generated bundles with `scripts/build-ui-bundles.js`. Treat hot-path fixes, desktop reconciliation, shared chat rendering, and canvas/lifecycle work as separate waves so each can be tested and committed independently.

**Tech Stack:** Go 1.26, vanilla JavaScript, embedded UI assets, generated JS/CSS bundles, existing `go test ./ui` static regression suite, `node scripts/build-ui-bundles.js`.

---

## Audit Triage

The report was checked against the current source tree on 2026-05-30.

Confirmed still valid:

- Normal chat SSE streaming still writes `bubble.textContent` or `bubble.innerHTML`, calls `decorateEmojiGlyphs`, and forces `chatBox.scrollTop = chatBox.scrollHeight` inside the `llm_stream_delta` hot path in `ui/js/chat/chat-streaming.js`.
- Normal chat `seenSSE*` sets are still cleared after HTTP `/v1/chat/completions` rendering, but not on SSE `done` in `ui/js/chat/chat-streaming.js`.
- `SmartScroller` still calls `onScroll()` directly from the `MutationObserver` callback in `ui/js/chat/modules/smart-scroller.js`.
- Video/YouTube link replacement still creates a new `template` per call and scans rendered message content without a cheap no-link fast path in `ui/js/chat/chat-messages.js`.
- Desktop chat stream text batching already exists, but `keepAgentStatusAtEnd()` still calls smooth `scrollIntoView()` independently from bubble append/render paths in `ui/js/desktop/apps/agent-chat.js`.
- Desktop shell rendering still fully rebuilds desktop icons, widgets, standard taskbar, and Fruity dock using `innerHTML` in `ui/js/desktop/core/desktop-foundation.js` and `ui/js/desktop/core/window-shell-runtime.js`.
- Widget lifecycle cleanup exists, but widget iframe nodes are still destroyed by `host.innerHTML = cards.join('')`; iframe `src` is not blanked before reset.
- Galaxa Deluxe and Pixel still contain the canvas hot spots called out by the report.
- Desktop WebSocket reconnect now closes the previous socket, but it still uses anonymous listeners without a generation guard or listener cleanup, so old close events can schedule reconnects.
- Desktop chat rendering still has local fallback copies and a custom sanitizer in `ui/js/desktop/chat-renderer.js`; `AuraChatCore` is used but is not yet the only source of truth.

Already done or obsolete in the current tree:

- Legacy duplicate `ui/js/chat/drag-drop.js` and `ui/js/chat/voice-recorder.js` are already removed; only `ui/js/chat/modules/*` versions remain.
- Chat shader and Three.js assets are already lazy-loaded through `ui/js/chat/theme-effects.js`, and `ui/frontend_optimization_test.go` covers this.
- Legacy theme CSS files are already consolidated into `ui/css/chat-themes.css` and bundled into `ui/css/chat.bundle.css`.
- Desktop `fetch` plus `eval` script assembly is already replaced by prebuilt bundles in `ui/js/desktop/core/module-loader.js`.
- Desktop app lifecycle support already exists in `disposeAppWindow()`, `callAppDispose()`, and app-specific `dispose()` exports for several apps.
- Desktop chat fetch-stream text updates are already batched with `requestAnimationFrame`; the remaining problem is duplicate scroll scheduling and shared parser duplication.

## Mandatory Preflight For Execution

GitNexus MCP still reported the AuraGo index as 15 commits behind while this plan was written. Before any worker modifies JavaScript symbols, refresh and verify the index, then run impact analysis for each symbol being edited.

- [ ] **Step 1: Refresh GitNexus if it reports stale**

```powershell
npx gitnexus analyze
```

Expected: GitNexus repo listing no longer reports `commitsBehind`.

- [ ] **Step 2: Run focused impact analysis before each symbol edit**

Use `mcp__gitnexus.impact` or the available GitNexus impact command for the exact symbol names in the task. Examples:

```text
impact target: connectSSE, direction: upstream
impact target: handleSSEMessage, direction: upstream
impact target: renderWidgets, direction: upstream
impact target: renderIcons, direction: upstream
impact target: renderTaskbar, direction: upstream
impact target: renderFruityDock, direction: upstream
impact target: sendDesktopChatStream, direction: upstream
```

Expected: Report direct callers, affected execution flows, and risk level in the implementation log before editing. If risk is HIGH or CRITICAL, pause and warn the user before changing code.

- [ ] **Step 3: Use disposable cache locations for test/build scratch data**

```powershell
if (-not (Test-Path disposable)) { New-Item -ItemType Directory disposable | Out-Null }
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
$env:GOMODCACHE = (Resolve-Path disposable).Path + '\gomodcache'
```

Expected: temporary caches stay under `disposable/`, which is not pushed.

## File Structure

- Create `ui/chat_runtime_performance_test.go`: static regression tests for normal chat SSE batching, dedup reset, SmartScroller observer debounce, video link fast paths, and desktop chat scroll batching.
- Create `ui/desktop_runtime_performance_test.go`: static regression tests for desktop WebSocket cleanup, widget iframe cleanup, taskbar/dock reconciliation, desktop icon reconciliation, canvas pooling/history caps, and app lifecycle cleanup markers.
- Modify `ui/js/chat/main/feedback-audio-plan.js`: add a shared `resetSSEDedupSets()` helper.
- Modify `ui/js/chat/main/network-submit.js`: call `resetSSEDedupSets()` instead of clearing each set manually.
- Modify `ui/js/chat/chat-streaming.js`: batch streaming bubble DOM writes and scrolls with `requestAnimationFrame`, skip emoji decoration during streaming, clear dedup sets on SSE `done`.
- Modify `ui/js/chat/modules/smart-scroller.js`: debounce observer-triggered `onScroll()` calls and cancel timers on destroy.
- Modify `ui/js/chat/chat-messages.js`: reuse templates for video/YouTube link replacement and add fast no-link guards.
- Modify `ui/js/desktop/apps/agent-chat.js`: replace competing scroll calls with a single per-frame scroll scheduler and later use the shared stream parser.
- Modify `ui/js/desktop/core/sdk-events-bootstrap.js`: add WebSocket listener cleanup and generation guards; clear shell timers on page unload.
- Modify `ui/js/desktop/core/desktop-foundation.js`: blank widget iframes before rebuild, then add differential icon/widget rendering.
- Modify `ui/js/desktop/core/window-shell-runtime.js`: add differential standard taskbar and Fruity dock reconciliation.
- Modify `ui/js/shared/chat-core.js`: expose any missing helpers needed by both chat renderers.
- Modify `ui/js/chat/chat-messages.js` and `ui/js/desktop/chat-renderer.js`: remove local fallback implementations after `AuraChatCore` coverage is complete.
- Create `ui/js/shared/chat-stream-parser.js`: shared fetch-stream SSE line parser for desktop chat first, and reusable event normalization for future normal chat use.
- Modify `scripts/build-ui-bundles.js`: include new shared parser in the chat runtime bundle and any desktop app asset list that needs it.
- Modify `ui/js/desktop/apps/galaxa-deluxe.js`: cache/reuse offscreen canvases and gradients, reduce pixel draw calls.
- Modify `ui/js/desktop/apps/pixel.js`: add canvas pooling and tighter full-image history limits.
- Regenerate `ui/js/chat/bundles/chat-runtime.bundle.js`, `ui/js/chat/main.js`, and `ui/js/desktop/bundles/main.bundle.js` after source edits.

## Wave 1: Chat Hot Path Fixes

### Task 1: Batch Normal Chat SSE DOM Writes And Clear Dedup Sets

**Files:**

- Create: `ui/chat_runtime_performance_test.go`
- Modify: `ui/js/chat/main/feedback-audio-plan.js`
- Modify: `ui/js/chat/main/network-submit.js`
- Modify: `ui/js/chat/chat-streaming.js`
- Generated: `ui/js/chat/main.js`
- Generated: `ui/js/chat/bundles/chat-runtime.bundle.js`

- [ ] **Step 1: Add the failing regression tests**

Create `ui/chat_runtime_performance_test.go` with this initial content:

```go
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
```

- [ ] **Step 2: Run the new focused test and verify it fails**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestChatStreamingBatchesDOMWritesAndResetsSSEDedup -count=1
```

Expected: FAIL because `chat-streaming.js` does not contain batching markers and still decorates/scrolls inside the delta handler.

- [ ] **Step 3: Add the dedup reset helper**

In `ui/js/chat/main/feedback-audio-plan.js`, after the `seenSSESTLs` declaration, add:

```js
function resetSSEDedupSets() {
    [
        seenSSEImages,
        seenSSEAudios,
        seenSSEVideos,
        seenSSELiveStreams,
        seenSSEYouTubeVideos,
        seenSSEDocuments,
        seenSSESTLs
    ].forEach(set => {
        if (set && typeof set.clear === 'function') set.clear();
    });
}
```

- [ ] **Step 4: Use the helper in the HTTP completion path**

In `ui/js/chat/main/network-submit.js`, replace the manual clear block:

```js
seenSSEImages.clear(); // reset after final response is rendered
seenSSEAudios.clear();
seenSSEVideos.clear();
seenSSEYouTubeVideos.clear();
seenSSEDocuments.clear();
seenSSESTLs.clear();
```

with:

```js
resetSSEDedupSets(); // reset after final response is rendered
```

- [ ] **Step 5: Batch stream rendering in `connectSSE()`**

In `ui/js/chat/chat-streaming.js`, inside `connectSSE()` near the existing `_streamingRow` variables, add these helpers:

```js
    let _streamingFlushFrame = 0;
    let _streamingNeedsFinalDecoration = false;

    function streamingBubble() {
        return _streamingRow ? _streamingRow.querySelector('.bubble') : null;
    }

    function renderStreamingBubble() {
        const bubble = streamingBubble();
        if (!bubble) return;
        if (_inThinkingBlock) return;
        if (_thinkingContent) {
            const label = typeof t === 'function' ? t('chat.thinking_label') : 'Reasoning';
            const detailsHtml = '<details class="thinking-block"><summary>' + label + '</summary><div class="thinking-content">' + escapeHtml(_thinkingContent) + '</div></details>';
            bubble.innerHTML = detailsHtml + '\n\n' + escapeHtml(_streamingContent);
        } else {
            bubble.textContent = _streamingContent;
        }
    }

    function flushStreamingBubble() {
        _streamingFlushFrame = 0;
        renderStreamingBubble();
        if (chatBox) chatBox.scrollTop = chatBox.scrollHeight;
    }

    function queueStreamingBubbleFlush() {
        if (_streamingFlushFrame) return;
        const schedule = window.requestAnimationFrame || ((callback) => window.setTimeout(callback, 16));
        _streamingFlushFrame = schedule(flushStreamingBubble);
    }

    function flushStreamingBubbleNow() {
        if (_streamingFlushFrame) {
            const cancel = window.cancelAnimationFrame || window.clearTimeout;
            cancel(_streamingFlushFrame);
            _streamingFlushFrame = 0;
        }
        flushStreamingBubble();
    }
```

Then change the `llm_stream_delta` handler so it only appends content and calls:

```js
        _streamingNeedsFinalDecoration = true;
        queueStreamingBubbleFlush();
```

Remove `window.decorateEmojiGlyphs(bubble)` and direct `chatBox.scrollTop = chatBox.scrollHeight` from the delta handler.

- [ ] **Step 6: Decorate only after final stream rendering**

In the `llm_stream_done` handler, before resetting `_streamingRow`, add:

```js
        flushStreamingBubbleNow();
        const bubble = streamingBubble();
        if (_streamingNeedsFinalDecoration && bubble && window.decorateEmojiGlyphs) {
            window.decorateEmojiGlyphs(bubble);
        }
        resetSSEDedupSets();
        _streamingNeedsFinalDecoration = false;
```

In the legacy `data.event === 'done'` branch of `handleSSEMessage(e)`, add:

```js
            resetSSEDedupSets();
```

before `return;`.

- [ ] **Step 7: Regenerate bundles**

```powershell
node scripts/build-ui-bundles.js
```

Expected output includes:

```text
Built ui/js/chat/bundles/chat-runtime.bundle.js
Built ui/js/chat/main.js
```

- [ ] **Step 8: Verify the focused test passes**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestChatStreamingBatchesDOMWritesAndResetsSSEDedup -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit**

```powershell
git add ui/chat_runtime_performance_test.go ui/js/chat/main/feedback-audio-plan.js ui/js/chat/main/network-submit.js ui/js/chat/chat-streaming.js ui/js/chat/main.js ui/js/chat/bundles/chat-runtime.bundle.js
git commit -m "perf: batch chat SSE rendering"
```

### Task 2: Debounce SmartScroller Mutation Observer

**Files:**

- Modify: `ui/chat_runtime_performance_test.go`
- Modify: `ui/js/chat/modules/smart-scroller.js`
- Generated: `ui/js/chat/bundles/chat-runtime.bundle.js`

- [ ] **Step 1: Add the failing observer debounce test**

Append this test to `ui/chat_runtime_performance_test.go`:

```go
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
```

- [ ] **Step 2: Run the focused test and verify it fails**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestSmartScrollerDebouncesMutationObserver -count=1
```

Expected: FAIL because the observer calls `onScroll()` directly.

- [ ] **Step 3: Implement debounced observer scheduling**

In `ui/js/chat/modules/smart-scroller.js`, add properties to `SmartScroller`:

```js
        mutationScrollDelay: 50,
        _mutationScrollTimer: null,
```

Add this method before `bindEvents()`:

```js
        scheduleObservedScrollCheck() {
            clearTimeout(this._mutationScrollTimer);
            this._mutationScrollTimer = setTimeout(() => {
                this._mutationScrollTimer = null;
                this.onScroll();
            }, this.mutationScrollDelay);
        },
```

Replace:

```js
this._mutationObserver = new MutationObserver(() => this.onScroll());
```

with:

```js
this._mutationObserver = new MutationObserver(() => this.scheduleObservedScrollCheck());
```

In `destroy()`, before disconnecting the observer, add:

```js
            clearTimeout(this._mutationScrollTimer);
            this._mutationScrollTimer = null;
```

- [ ] **Step 4: Regenerate bundles**

```powershell
node scripts/build-ui-bundles.js
```

Expected: chat runtime bundle rebuilt.

- [ ] **Step 5: Verify the focused test passes**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestSmartScrollerDebouncesMutationObserver -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add ui/chat_runtime_performance_test.go ui/js/chat/modules/smart-scroller.js ui/js/chat/bundles/chat-runtime.bundle.js
git commit -m "perf: debounce chat scroll observer"
```

### Task 3: Add Fast Paths For Chat Video And YouTube Link Replacement

**Files:**

- Modify: `ui/chat_runtime_performance_test.go`
- Modify: `ui/js/chat/chat-messages.js`
- Generated: `ui/js/chat/bundles/chat-runtime.bundle.js`

- [ ] **Step 1: Add the failing link-render fast-path test**

Append this test to `ui/chat_runtime_performance_test.go`:

```go
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
```

- [ ] **Step 2: Run the focused test and verify it fails**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestChatMediaLinkReplacementUsesReusableTemplatesAndFastPaths -count=1
```

Expected: FAIL because templates are created inside each function.

- [ ] **Step 3: Add reusable templates and candidate checks**

In `ui/js/chat/chat-messages.js`, before `renderVideoLinksAsPlayers(html)`, add:

```js
const chatVideoLinkTemplate = typeof document !== 'undefined' ? document.createElement('template') : null;
const chatYouTubeLinkTemplate = typeof document !== 'undefined' ? document.createElement('template') : null;
const chatVideoLinkCandidatePattern = /(?:\/files\/[^\s<>()"']+\.(?:mp4|m4v|mov|webm|ogv|ogg)|https?:\/\/[^\s<>()"']+\.(?:mp4|m4v|mov|webm|ogv|ogg))/i;
const chatYouTubeLinkCandidatePattern = /(?:youtube\.com|youtu\.be|youtube-nocookie\.com)/i;

function hasVideoLinkCandidate(html) {
    return !!html && chatVideoLinkCandidatePattern.test(String(html));
}

function hasYouTubeLinkCandidate(html) {
    return !!html && chatYouTubeLinkCandidatePattern.test(String(html));
}
```

- [ ] **Step 4: Reuse templates in both renderers**

At the start of `renderVideoLinksAsPlayers(html)`, add:

```js
    if (!hasVideoLinkCandidate(html)) return html;
    const template = chatVideoLinkTemplate || document.createElement('template');
    template.innerHTML = html;
```

and remove the existing local `const template = document.createElement('template');`.

At the start of `renderYouTubeLinksAsPlayers(html)`, add:

```js
    if (!hasYouTubeLinkCandidate(html)) return html;
    const template = chatYouTubeLinkTemplate || document.createElement('template');
    template.innerHTML = html;
```

and remove the existing local `const template = document.createElement('template');`.

Before returning from each function, leave `template.innerHTML = ''` after capturing the result:

```js
    const output = template.innerHTML;
    template.innerHTML = '';
    return output;
```

- [ ] **Step 5: Regenerate bundles**

```powershell
node scripts/build-ui-bundles.js
```

Expected: chat runtime bundle rebuilt.

- [ ] **Step 6: Verify the focused test passes**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestChatMediaLinkReplacementUsesReusableTemplatesAndFastPaths -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add ui/chat_runtime_performance_test.go ui/js/chat/chat-messages.js ui/js/chat/bundles/chat-runtime.bundle.js
git commit -m "perf: avoid unnecessary chat media scans"
```

### Task 4: Centralize Desktop Chat Scroll Scheduling

**Files:**

- Modify: `ui/chat_runtime_performance_test.go`
- Modify: `ui/js/desktop/apps/agent-chat.js`

- [ ] **Step 1: Add the failing scroll scheduler test**

Append this test to `ui/chat_runtime_performance_test.go`:

```go
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
```

- [ ] **Step 2: Run the focused test and verify it fails**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestDesktopChatUsesSingleScrollScheduler -count=1
```

Expected: FAIL because `keepAgentStatusAtEnd()` calls smooth scroll directly.

- [ ] **Step 3: Add a single scroll scheduler in `sendDesktopChatStream()`**

In `ui/js/desktop/apps/agent-chat.js`, inside `sendDesktopChatStream()` after `let finalized = false;`, add:

```js
        let chatScrollFrame = 0;
        let pendingScrollTarget = null;
        let pendingScrollSmooth = true;

        function scheduleChatScroll(target, smooth = true) {
            if (!target) return;
            pendingScrollTarget = target;
            pendingScrollSmooth = smooth;
            if (chatScrollFrame) return;
            const schedule = window.requestAnimationFrame || ((callback) => window.setTimeout(callback, 16));
            chatScrollFrame = schedule(() => {
                chatScrollFrame = 0;
                if (!pendingScrollTarget) return;
                pendingScrollTarget.scrollIntoView({
                    block: 'end',
                    behavior: pendingScrollSmooth ? 'smooth' : 'auto'
                });
                pendingScrollTarget = null;
            });
        }
```

- [ ] **Step 4: Use the scheduler in status and bubble append paths**

Replace the direct scroll in `keepAgentStatusAtEnd()`:

```js
statusEl.scrollIntoView({ block: 'end', behavior: 'smooth' });
```

with:

```js
scheduleChatScroll(statusEl, true);
```

For one-off non-streaming `appendChat()`, keep the existing immediate `bubble.scrollIntoView({ block: 'end' });`; this function is outside the streaming hot path.

In `doReject()`, cancel any pending frame:

```js
                if (chatScrollFrame) {
                    const cancelScroll = window.cancelAnimationFrame || window.clearTimeout;
                    cancelScroll(chatScrollFrame);
                    chatScrollFrame = 0;
                    pendingScrollTarget = null;
                }
```

- [ ] **Step 5: Verify the focused test passes**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestDesktopChatUsesSingleScrollScheduler -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add ui/chat_runtime_performance_test.go ui/js/desktop/apps/agent-chat.js
git commit -m "perf: coalesce desktop chat scrolling"
```

## Wave 2: Desktop Shell Runtime Stability

### Task 5: Add WebSocket Listener Cleanup And Generation Guards

**Files:**

- Create: `ui/desktop_runtime_performance_test.go`
- Modify: `ui/js/desktop/core/sdk-events-bootstrap.js`
- Generated: `ui/js/desktop/bundles/main.bundle.js`

- [ ] **Step 1: Add the failing WebSocket cleanup test**

Create `ui/desktop_runtime_performance_test.go` with:

```go
package ui

import (
	"strings"
	"testing"
)

func TestDesktopWebSocketReconnectCleansPreviousListeners(t *testing.T) {
	t.Parallel()

	source := readEmbeddedText(t, "js/desktop/core/sdk-events-bootstrap.js")
	for _, marker := range []string{
		"let wsGeneration = 0",
		"function cleanupDesktopWS()",
		"state.wsCleanup",
		"ws.removeEventListener('open', onOpen)",
		"ws.removeEventListener('close', onClose)",
		"ws.removeEventListener('message', onMessage)",
		"const generation = ++wsGeneration",
		"if (generation !== wsGeneration || ws !== state.ws) return",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop websocket cleanup missing marker %q", marker)
		}
	}
}
```

- [ ] **Step 2: Run the focused test and verify it fails**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestDesktopWebSocketReconnectCleansPreviousListeners -count=1
```

Expected: FAIL because listeners are anonymous and no cleanup function exists.

- [ ] **Step 3: Implement cleanup before reconnect**

In `ui/js/desktop/core/sdk-events-bootstrap.js`, near the WebSocket state variables, add:

```js
    let wsGeneration = 0;

    function cleanupDesktopWS() {
        if (typeof state.wsCleanup === 'function') {
            try { state.wsCleanup(); } catch (_) {}
            state.wsCleanup = null;
        }
        if (state.ws) {
            try { state.ws.close(); } catch (_) {}
            state.ws = null;
        }
    }
```

Then replace the top of `connectWS()`:

```js
        if (state.ws) {
            try { state.ws.close(); } catch (_) {}
        }
```

with:

```js
        cleanupDesktopWS();
        const generation = ++wsGeneration;
```

- [ ] **Step 4: Convert anonymous listeners into removable handlers**

Inside `connectWS()`, define handlers before registering listeners:

```js
        function staleSocket() {
            return generation !== wsGeneration || ws !== state.ws;
        }

        function onOpen() {
            if (staleSocket()) return;
            wsReconnectAttempts = 0;
            wsReconnectDelay = 2000;
            setWSState(true);
        }

        function onClose() {
            if (staleSocket()) return;
            if (wsReconnectAttempts >= MAX_WS_RETRIES) {
                setWSState(false, true);
                return;
            }
            setWSState(false);
            wsReconnectTimer = setTimeout(() => {
                if (staleSocket()) return;
                wsReconnectAttempts++;
                wsReconnectDelay = Math.min(wsReconnectDelay * 2, WS_MAX_DELAY);
                connectWS();
            }, wsReconnectDelay);
        }

        function onMessage(event) {
            if (staleSocket()) return;
            let msg;
            try { msg = JSON.parse(event.data); } catch (_) { return; }
            try {
                handleDesktopEvent(msg.type === 'welcome' ? { type: 'welcome', payload: msg.payload } : msg);
            } catch (_) {}
        }

        ws.addEventListener('open', onOpen);
        ws.addEventListener('close', onClose);
        ws.addEventListener('message', onMessage);
        state.wsCleanup = () => {
            ws.removeEventListener('open', onOpen);
            ws.removeEventListener('close', onClose);
            ws.removeEventListener('message', onMessage);
        };
```

Remove the old anonymous listener registrations.

- [ ] **Step 5: Regenerate bundles**

```powershell
node scripts/build-ui-bundles.js
```

Expected: desktop main bundle rebuilt.

- [ ] **Step 6: Verify the focused test passes**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestDesktopWebSocketReconnectCleansPreviousListeners -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add ui/desktop_runtime_performance_test.go ui/js/desktop/core/sdk-events-bootstrap.js ui/js/desktop/bundles/main.bundle.js
git commit -m "fix: clean up desktop websocket reconnects"
```

### Task 6: Stop Widget Iframes Before Rebuild

**Files:**

- Modify: `ui/desktop_runtime_performance_test.go`
- Modify: `ui/js/desktop/core/desktop-foundation.js`
- Generated: `ui/js/desktop/bundles/main.bundle.js`

- [ ] **Step 1: Add the failing iframe cleanup test**

Append to `ui/desktop_runtime_performance_test.go`:

```go
func TestDesktopWidgetsBlankIframesBeforeRebuild(t *testing.T) {
	t.Parallel()

	source := readEmbeddedText(t, "js/desktop/core/desktop-foundation.js")
	for _, marker := range []string{
		"function blankWidgetFrames(host)",
		"host.querySelectorAll('iframe')",
		"frame.src = 'about:blank'",
		"blankWidgetFrames(host);",
		"clearWidgetRuntime();",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop widget iframe cleanup missing marker %q", marker)
		}
	}
}
```

- [ ] **Step 2: Run the focused test and verify it fails**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestDesktopWidgetsBlankIframesBeforeRebuild -count=1
```

Expected: FAIL because no `blankWidgetFrames` helper exists.

- [ ] **Step 3: Add iframe cleanup helper**

In `ui/js/desktop/core/desktop-foundation.js`, near `clearWidgetRuntime()`, add:

```js
    function blankWidgetFrames(host) {
        if (!host || typeof host.querySelectorAll !== 'function') return;
        host.querySelectorAll('iframe').forEach(frame => {
            try { frame.src = 'about:blank'; } catch (_) {}
        });
    }
```

In `renderWidgets()`, after `const host = $('vd-widgets');` and before `clearWidgetRuntime();`, add:

```js
        blankWidgetFrames(host);
```

- [ ] **Step 4: Regenerate bundles**

```powershell
node scripts/build-ui-bundles.js
```

Expected: desktop main bundle rebuilt.

- [ ] **Step 5: Verify the focused test passes**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestDesktopWidgetsBlankIframesBeforeRebuild -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add ui/desktop_runtime_performance_test.go ui/js/desktop/core/desktop-foundation.js ui/js/desktop/bundles/main.bundle.js
git commit -m "fix: stop widget iframes before rebuild"
```

### Task 7: Reconcile Standard Taskbar And Fruity Dock Instead Of Full Rebuild

**Files:**

- Modify: `ui/desktop_runtime_performance_test.go`
- Modify: `ui/js/desktop/core/window-shell-runtime.js`
- Generated: `ui/js/desktop/bundles/main.bundle.js`

- [ ] **Step 1: Add the failing taskbar reconciliation test**

Append to `ui/desktop_runtime_performance_test.go`:

```go
func TestDesktopTaskbarAndDockUseReconciliation(t *testing.T) {
	t.Parallel()

	source := readEmbeddedText(t, "js/desktop/core/window-shell-runtime.js")
	for _, marker := range []string{
		"function reconcileStandardTaskbar()",
		"const seenWindowIds = new Set()",
		"data-taskbar-bound",
		"function updateTaskbarButton(btn, win, index)",
		"function reconcileFruityDock()",
		"const seenDockAppIds = new Set()",
		"data-dock-bound",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop taskbar reconciliation missing marker %q", marker)
		}
	}
	if strings.Contains(source, "function renderStandardTaskbar() {\n        const host = $('vd-taskbar-apps');\n        host.innerHTML =") {
		t.Fatal("standard taskbar must not fully rebuild via host.innerHTML")
	}
}
```

- [ ] **Step 2: Run the focused test and verify it fails**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestDesktopTaskbarAndDockUseReconciliation -count=1
```

Expected: FAIL because taskbar and dock use `host.innerHTML`.

- [ ] **Step 3: Extract standard taskbar button creation and update**

In `ui/js/desktop/core/window-shell-runtime.js`, replace `renderStandardTaskbar()` with helpers following this shape:

```js
    function taskbarButtonHTML(win, index) {
        const app = appById(win.appId);
        const icon = iconMarkup(win.icon || iconForApp(app), win.iconGlyph || iconGlyph(app), 'vd-task-icon', 16);
        return `${icon}<span class="vd-task-label">${esc(win.title)}</span>`;
    }

    function updateTaskbarButton(btn, win, index) {
        btn.classList.toggle('active', win.id === state.activeWindowId);
        btn.dataset.windowId = win.id;
        btn.style.setProperty('--dock-index', index);
        const nextHTML = taskbarButtonHTML(win, index);
        if (btn.dataset.renderedHtml !== nextHTML) {
            btn.innerHTML = nextHTML;
            btn.dataset.renderedHtml = nextHTML;
        }
    }

    function bindTaskbarButton(btn) {
        if (btn.dataset.taskbarBound === 'true') return;
        btn.dataset.taskbarBound = 'true';
        btn.addEventListener('click', () => focusWindow(btn.dataset.windowId));
        btn.addEventListener('contextmenu', event => showWindowContextMenu(event, btn.dataset.windowId));
        wireLongPress(btn, event => showWindowContextMenu(event, btn.dataset.windowId));
    }

    function reconcileStandardTaskbar() {
        const host = $('vd-taskbar-apps');
        const seenWindowIds = new Set();
        [...state.windows.values()].forEach((win, index) => {
            seenWindowIds.add(win.id);
            let btn = host.querySelector(`[data-window-id="${cssSel(win.id)}"]`);
            if (!btn) {
                btn = document.createElement('button');
                btn.type = 'button';
                btn.className = 'vd-task-button';
                host.appendChild(btn);
                bindTaskbarButton(btn);
            }
            updateTaskbarButton(btn, win, index);
        });
        host.querySelectorAll('[data-window-id]').forEach(btn => {
            if (!seenWindowIds.has(btn.dataset.windowId)) btn.remove();
        });
    }

    function renderStandardTaskbar() {
        reconcileStandardTaskbar();
    }
```

- [ ] **Step 4: Reconcile Fruity dock app buttons**

Keep the static dock shell (`orb`, scroll buttons, track) created once when absent, then update only app buttons in the track:

```js
    function ensureFruityDockShell(host) {
        if (host.querySelector('[data-fruity-dock-track]')) return host.querySelector('[data-fruity-dock-track]');
        host.innerHTML = `<button type="button" class="vd-dock-orb" data-fruity-dock-orb title="${esc(t('desktop.start_menu'))}">
            ${iconMarkup('home', 'A', 'vd-dock-orb-icon', 34)}
        </button>
        <button type="button" class="vd-dock-scroll-button vd-dock-scroll-button-left" data-fruity-dock-scroll-button="left" aria-label="${esc(t('desktop.dock_scroll_left'))}">
            ${iconMarkup('arrow-left', '<', 'vd-dock-scroll-icon', 18)}
        </button>
        <div class="vd-dock-scroll" data-fruity-dock-scroll-region>
            <div class="vd-dock-track" data-fruity-dock-track></div>
        </div>
        <button type="button" class="vd-dock-scroll-button vd-dock-scroll-button-right" data-fruity-dock-scroll-button="right" aria-label="${esc(t('desktop.dock_scroll_right'))}">
            ${iconMarkup('arrow-right', '>', 'vd-dock-scroll-icon', 18)}
        </button>`;
        const orb = host.querySelector('[data-fruity-dock-orb]');
        if (orb) {
            orb.addEventListener('click', event => {
                event.stopPropagation();
                toggleStartMenu();
            });
        }
        wireFruityDockScroll(host);
        return host.querySelector('[data-fruity-dock-track]');
    }
```

Add `reconcileFruityDock()` using `const seenDockAppIds = new Set()` and `data-dock-bound`, then call it from `renderFruityDock()`.

- [ ] **Step 5: Regenerate bundles**

```powershell
node scripts/build-ui-bundles.js
```

Expected: desktop main bundle rebuilt.

- [ ] **Step 6: Verify focused tests pass**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestDesktopTaskbarAndDockUseReconciliation -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add ui/desktop_runtime_performance_test.go ui/js/desktop/core/window-shell-runtime.js ui/js/desktop/bundles/main.bundle.js
git commit -m "perf: reconcile desktop taskbar and dock"
```

### Task 8: Reconcile Desktop Icons Instead Of Full Rebuild

**Files:**

- Modify: `ui/desktop_runtime_performance_test.go`
- Modify: `ui/js/desktop/core/desktop-foundation.js`
- Generated: `ui/js/desktop/bundles/main.bundle.js`

- [ ] **Step 1: Add the failing desktop icon reconciliation test**

Append to `ui/desktop_runtime_performance_test.go`:

```go
func TestDesktopIconsUseReconciliation(t *testing.T) {
	t.Parallel()

	source := readEmbeddedText(t, "js/desktop/core/desktop-foundation.js")
	for _, marker := range []string{
		"function reconcileDesktopIcons(items, positions)",
		"function updateDesktopIconButton(btn, item, pos)",
		"function bindDesktopIconButton(btn)",
		"data-vd-icon-bound",
		"const seenIconIds = new Set()",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop icon reconciliation missing marker %q", marker)
		}
	}
	if strings.Contains(source, "icons.innerHTML = items.map(item =>") {
		t.Fatal("desktop icons must not fully rebuild via icons.innerHTML")
	}
}
```

- [ ] **Step 2: Run the focused test and verify it fails**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestDesktopIconsUseReconciliation -count=1
```

Expected: FAIL because `renderIcons()` still uses `icons.innerHTML`.

- [ ] **Step 3: Split icon update and event binding**

In `ui/js/desktop/core/desktop-foundation.js`, replace the `icons.innerHTML = items.map(...).join('')` flow with:

```js
    function updateDesktopIconButton(btn, item, pos) {
        const iconKey = item.icon || (item.type === 'file' ? iconForFile(item.file) : item.type === 'directory' ? iconForDirectory(item.name) : iconForApp(item.app));
        const fallback = item.type === 'app' ? iconGlyph(item.app) : item.name;
        btn.className = 'vd-icon ' + (state.selectedIconIds.has(item.id) ? 'selected' : '');
        btn.type = 'button';
        btn.setAttribute('role', 'button');
        btn.setAttribute('aria-label', item.name);
        btn.setAttribute('aria-selected', state.selectedIconIds.has(item.id) ? 'true' : 'false');
        btn.dataset.kind = item.type;
        btn.dataset.id = item.id;
        btn.dataset.appId = item.app ? item.app.id : '';
        btn.dataset.path = item.path || '';
        btn.dataset.webPath = item.file ? item.file.web_path || '' : '';
        btn.dataset.mediaKind = item.file ? item.file.media_kind || '' : '';
        btn.dataset.mimeType = item.file ? item.file.mime_type || '' : '';
        btn.dataset.desktopEntry = item.desktopEntry ? 'true' : 'false';
        btn.style.left = (Number(pos.x) || 18) + 'px';
        btn.style.top = (Number(pos.y) || 18) + 'px';
        const renderedHTML = `${iconMarkup(iconKey, fallback, 'vd-sprite-icon', iconGlyphPixels())}<span class="vd-icon-label">${esc(item.name)}</span>`;
        if (btn.dataset.renderedHtml !== renderedHTML) {
            btn.innerHTML = renderedHTML;
            btn.dataset.renderedHtml = renderedHTML;
        }
    }

    function bindDesktopIconButton(btn) {
        if (btn.dataset.vdIconBound === 'true') return;
        btn.dataset.vdIconBound = 'true';
        btn.addEventListener('dblclick', () => activateDesktopItem(btn));
        btn.addEventListener('click', event => {
            if (btn.__vdSuppressNextClick) {
                btn.__vdSuppressNextClick = false;
                event.preventDefault();
                return;
            }
            if (shouldOpenOnTap(event)) {
                event.preventDefault();
                activateDesktopItem(btn);
                return;
            }
            selectDesktopIcon(btn, { extend: event.ctrlKey || event.metaKey, toggle: event.ctrlKey || event.metaKey });
        });
        btn.addEventListener('contextmenu', event => showIconContextMenu(event, btn));
        wireLongPress(btn, event => showIconContextMenu(event, btn));
        wireDraggableIcon(btn);
        if (typeof wireDesktopFileIconDrag === 'function') wireDesktopFileIconDrag(btn);
    }
```

Add:

```js
    function reconcileDesktopIcons(items, positions) {
        const icons = $('vd-icons');
        const seenIconIds = new Set();
        items.forEach((item, index) => {
            seenIconIds.add(item.id);
            const pos = positions[item.id] || defaultIconPosition(index);
            let btn = icons.querySelector(`[data-id="${cssSel(item.id)}"]`);
            if (!btn) {
                btn = document.createElement('button');
                icons.appendChild(btn);
                bindDesktopIconButton(btn);
            }
            updateDesktopIconButton(btn, item, pos);
        });
        icons.querySelectorAll('.vd-icon[data-id]').forEach(btn => {
            if (!seenIconIds.has(btn.dataset.id)) btn.remove();
        });
    }
```

Change `renderIcons()` to:

```js
        reconcileDesktopIcons(items, positions);
        syncDesktopIconSelection();
```

- [ ] **Step 4: Regenerate bundles**

```powershell
node scripts/build-ui-bundles.js
```

Expected: desktop main bundle rebuilt.

- [ ] **Step 5: Verify focused test passes**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestDesktopIconsUseReconciliation -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add ui/desktop_runtime_performance_test.go ui/js/desktop/core/desktop-foundation.js ui/js/desktop/bundles/main.bundle.js
git commit -m "perf: reconcile desktop icons"
```

## Wave 3: Shared Chat Rendering And Stream Parsing

### Task 9: Make AuraChatCore The Only Chat Sanitizer/Helper Source

**Files:**

- Modify: `ui/frontend_optimization_test.go`
- Modify: `ui/js/shared/chat-core.js`
- Modify: `ui/js/chat/chat-messages.js`
- Modify: `ui/js/desktop/chat-renderer.js`
- Generated: `ui/js/chat/bundles/chat-runtime.bundle.js`

- [ ] **Step 1: Tighten the existing core delegation test**

In `ui/frontend_optimization_test.go`, extend `TestChatRenderersDelegateToSharedChatCore` with these checks:

```go
for _, forbidden := range []string{
	"return String(str)\n                .replace(/&/g, '&amp;')",
	"const allowed = new Set([",
	"const allowedAttrs = new Set([",
	"function decorateEmojiGlyphs(root) {\n    if (window.AuraChatCore",
} {
	if strings.Contains(desktopChatJS, forbidden) {
		t.Fatalf("desktop chat renderer must not keep local fallback implementation marker %q", forbidden)
	}
}
for _, forbidden := range []string{
	"const emojiGlyphPattern =",
	"function decorateEmojiGlyphs(root) {\n    if (window.AuraChatCore",
} {
	if strings.Contains(chatJS, forbidden) {
		t.Fatalf("chat message renderer must not keep local fallback implementation marker %q", forbidden)
	}
}
```

- [ ] **Step 2: Run the focused test and verify it fails**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestChatRenderersDelegateToSharedChatCore -count=1
```

Expected: FAIL while local fallback bodies remain.

- [ ] **Step 3: Add missing APIs to `AuraChatCore` before deleting fallbacks**

In `ui/js/shared/chat-core.js`, verify these APIs exist and export them if missing:

```js
sanitizeRenderedHTML,
decorateEmojiGlyphs,
escapeHtml,
escapeAttr,
isSafeHref,
createMarkdownRenderer,
formatTimestamp,
normalizeTimestamp,
prepareDisplayContent,
prepareMarkdownContent,
applyMarkdownLinkTargets,
replaceThinkingPlaceholders,
removeSeenMarkdownImages,
videoMimeTypeForPath,
filenameFromPath
```

If any desktop-only sanitizer rule is missing, move that rule into `sanitizeRenderedHTML()` rather than keeping it in `ui/js/desktop/chat-renderer.js`.

- [ ] **Step 4: Remove local fallbacks from chat renderers**

In `ui/js/chat/chat-messages.js` and `ui/js/desktop/chat-renderer.js`, convert helper methods to direct calls. Example:

```js
function escapeHtml(str) {
    return window.AuraChatCore.escapeHtml(str);
}
```

For `DesktopChatRenderer.sanitizeHTML(html)`, replace the custom body with:

```js
        sanitizeHTML(html) {
            return window.AuraChatCore.sanitizeRenderedHTML(html);
        },
```

- [ ] **Step 5: Regenerate bundles**

```powershell
node scripts/build-ui-bundles.js
```

Expected: chat runtime bundle rebuilt.

- [ ] **Step 6: Verify focused and security tests pass**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run "TestChatRenderersDelegateToSharedChatCore|TestChatFrontend_ToolLeakSanitizerPatternsRemainPresent|TestVirtualDesktopChat_MessageTimestampsRemainWired" -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add ui/frontend_optimization_test.go ui/js/shared/chat-core.js ui/js/chat/chat-messages.js ui/js/desktop/chat-renderer.js ui/js/chat/bundles/chat-runtime.bundle.js
git commit -m "refactor: centralize chat rendering helpers"
```

### Task 10: Extract A Shared Fetch SSE Stream Parser

**Files:**

- Modify: `ui/chat_runtime_performance_test.go`
- Create: `ui/js/shared/chat-stream-parser.js`
- Modify: `ui/js/desktop/core/module-loader.js`
- Modify: `ui/js/desktop/apps/agent-chat.js`
- Modify: `scripts/build-ui-bundles.js`
- Generated: `ui/js/chat/bundles/chat-runtime.bundle.js` if the parser is bundled for chat runtime

- [ ] **Step 1: Add the failing shared parser test**

Append this test to `ui/chat_runtime_performance_test.go`:

```go
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
```

- [ ] **Step 2: Run the focused test and verify it fails**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestSharedChatStreamParserIsUsedByDesktopChat -count=1
```

Expected: FAIL because the parser file does not exist and desktop chat parses lines manually.

- [ ] **Step 3: Create the shared parser**

Create `ui/js/shared/chat-stream-parser.js`:

```js
(function () {
    'use strict';

    function normalizeStreamEvent(data) {
        const event = data && (data.event || data.type) || '';
        return Object.assign({}, data || {}, { event, type: event });
    }

    async function readFetchEventStream(response, handlers = {}) {
        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';
        try {
            while (true) {
                const { done, value } = await reader.read();
                if (done) break;
                buffer += decoder.decode(value, { stream: true });
                const lines = buffer.split('\n');
                buffer = lines.pop() || '';
                for (const line of lines) {
                    if (!line.startsWith('data: ')) continue;
                    const raw = line.slice(6).trim();
                    if (raw === '[DONE]') {
                        if (typeof handlers.onDone === 'function') handlers.onDone();
                        try { await reader.cancel(); } catch (_) {}
                        return;
                    }
                    try {
                        const parsed = JSON.parse(raw);
                        if (typeof handlers.onEvent === 'function') {
                            handlers.onEvent(normalizeStreamEvent(parsed));
                        }
                    } catch (err) {
                        if (typeof handlers.onError === 'function') handlers.onError(err);
                    }
                }
            }
            if (typeof handlers.onDone === 'function') handlers.onDone();
        } catch (err) {
            if (typeof handlers.onError === 'function') handlers.onError(err);
            else throw err;
        }
    }

    window.AuraChatStreamParser = {
        normalizeStreamEvent,
        readFetchEventStream
    };
})();
```

- [ ] **Step 4: Load the parser before desktop agent chat**

In `ui/js/desktop/core/module-loader.js`, add `/js/shared/chat-stream-parser.js` before `/js/desktop/apps/agent-chat.js` in the `agent-chat` scripts list.

If this parser should also be available to the normal chat bundle, add `ui/js/shared/chat-stream-parser.js` to `chatRuntimeParts` in `scripts/build-ui-bundles.js` immediately after `ui/js/shared/chat-core.js`.

- [ ] **Step 5: Replace manual desktop fetch-stream parsing**

In `ui/js/desktop/apps/agent-chat.js`, replace the `reader`, `decoder`, `buffer`, and `processChunk()` block with:

```js
                return window.AuraChatStreamParser.readFetchEventStream(response, {
                    onEvent: eventData => handleStreamEvent(eventData),
                    onDone: () => doFinalize(),
                    onError: err => doReject(err)
                });
```

Keep the existing `handleStreamEvent(data)` function and `doFinalize()` behavior.

- [ ] **Step 6: Regenerate bundles**

```powershell
node scripts/build-ui-bundles.js
```

Expected: relevant bundles rebuilt.

- [ ] **Step 7: Verify focused test passes**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestSharedChatStreamParserIsUsedByDesktopChat -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```powershell
git add ui/chat_runtime_performance_test.go ui/js/shared/chat-stream-parser.js ui/js/desktop/core/module-loader.js ui/js/desktop/apps/agent-chat.js scripts/build-ui-bundles.js ui/js/chat/bundles/chat-runtime.bundle.js
git commit -m "refactor: share chat stream parser"
```

## Wave 4: Canvas And Long-Session Resource Optimizations

### Task 11: Reduce Galaxa Deluxe Canvas Allocation And Draw Pressure

**Files:**

- Modify: `ui/desktop_runtime_performance_test.go`
- Modify: `ui/js/desktop/apps/galaxa-deluxe.js`

- [ ] **Step 1: Add the failing Galaxa performance test**

Append to `ui/desktop_runtime_performance_test.go`:

```go
func TestGalaxaDeluxeCachesCanvasResources(t *testing.T) {
	t.Parallel()

	source := readEmbeddedText(t, "js/desktop/apps/galaxa-deluxe.js")
	for _, marker := range []string{
		"function ensureNebulaCanvas()",
		"nebulaCv.width = W",
		"const radialGradientCache = new Map()",
		"function cachedRadialGradient",
		"function drawPixelSprite",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("galaxa deluxe canvas optimization missing marker %q", marker)
		}
	}
	if strings.Contains(source, "nebulaCv = document.createElement('canvas'); nebulaCv.width = W; nebulaCv.height = H;") {
		t.Fatal("galaxa deluxe must reuse the nebula canvas instead of allocating a new one per stage")
	}
}
```

- [ ] **Step 2: Run the focused test and verify it fails**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestGalaxaDeluxeCachesCanvasResources -count=1
```

Expected: FAIL because current code creates a new nebula canvas and gradients in hot paths.

- [ ] **Step 3: Reuse the nebula canvas**

In `ui/js/desktop/apps/galaxa-deluxe.js`, add:

```js
        function ensureNebulaCanvas() {
            if (!nebulaCv) {
                nebulaCv = document.createElement('canvas');
            }
            if (nebulaCv.width !== W) nebulaCv.width = W;
            if (nebulaCv.height !== H) nebulaCv.height = H;
            return nebulaCv;
        }
```

Replace the current stage allocation with:

```js
            nebulaCv = ensureNebulaCanvas();
            const nc = nebulaCv.getContext('2d');
            nc.clearRect(0, 0, W, H);
```

- [ ] **Step 4: Cache repeated radial gradients**

Add:

```js
        const radialGradientCache = new Map();
        function cachedRadialGradient(ctx, key, x, y, r, stops) {
            const cacheKey = `${key}:${Math.round(r)}:${stops.map(stop => stop.join('@')).join('|')}`;
            if (radialGradientCache.has(cacheKey)) return radialGradientCache.get(cacheKey);
            const gradient = ctx.createRadialGradient(x, y, 0, x, y, r);
            stops.forEach(([offset, color]) => gradient.addColorStop(offset, color));
            radialGradientCache.set(cacheKey, gradient);
            return gradient;
        }
```

Use it for stable per-frame effects where the center/radius are not materially changing. Do not cache gradients whose coordinates intentionally vary every particle.

- [ ] **Step 5: Introduce a pixel sprite helper for dense 1x1 draws**

Add:

```js
        function drawPixelSprite(ctx, pixels, x, y, scale = 1) {
            pixels.forEach(pixel => {
                ctx.fillStyle = pixel.color;
                ctx.fillRect(Math.floor(x + pixel.x * scale), Math.floor(y + pixel.y * scale), Math.max(1, scale), Math.max(1, scale));
            });
        }
```

Use it for static dense sprite shapes first. Keep gameplay behavior unchanged.

- [ ] **Step 6: Verify focused test passes**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestGalaxaDeluxeCachesCanvasResources -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add ui/desktop_runtime_performance_test.go ui/js/desktop/apps/galaxa-deluxe.js
git commit -m "perf: reuse Galaxa canvas resources"
```

### Task 12: Add Pixel Editor Canvas Pool And Tighter History Budget

**Files:**

- Modify: `ui/desktop_runtime_performance_test.go`
- Modify: `ui/js/desktop/apps/pixel.js`

- [ ] **Step 1: Add the failing Pixel performance test**

Append to `ui/desktop_runtime_performance_test.go`:

```go
func TestPixelEditorUsesCanvasPoolAndBoundedHistory(t *testing.T) {
	t.Parallel()

	source := readEmbeddedText(t, "js/desktop/apps/pixel.js")
	for _, marker := range []string{
		"const MAX_HISTORY = 5",
		"const canvasPool = []",
		"function acquireTempCanvas(width, height)",
		"function releaseTempCanvas(canvas)",
		"releaseTempCanvas(tmpCanvas)",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("pixel editor runtime optimization missing marker %q", marker)
		}
	}
	if strings.Contains(source, "if (state.history.length > 20)") {
		t.Fatal("pixel editor history must not keep 20 full ImageData snapshots")
	}
}
```

- [ ] **Step 2: Run the focused test and verify it fails**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestPixelEditorUsesCanvasPoolAndBoundedHistory -count=1
```

Expected: FAIL because history is capped at 20 and temp canvases are allocated repeatedly.

- [ ] **Step 3: Add a canvas pool**

In `ui/js/desktop/apps/pixel.js`, near module state, add:

```js
    const MAX_HISTORY = 5;
    const canvasPool = [];

    function acquireTempCanvas(width, height) {
        const canvas = canvasPool.pop() || document.createElement('canvas');
        canvas.width = width;
        canvas.height = height;
        return canvas;
    }

    function releaseTempCanvas(canvas) {
        if (!canvas) return;
        const ctx = canvas.getContext('2d');
        if (ctx) ctx.clearRect(0, 0, canvas.width, canvas.height);
        if (canvasPool.length < 4) canvasPool.push(canvas);
    }
```

- [ ] **Step 4: Replace temp canvas allocations**

Replace each filter/resize local allocation:

```js
const tmpCanvas = document.createElement('canvas');
tmpCanvas.width = canvas.width;
tmpCanvas.height = canvas.height;
```

with:

```js
const tmpCanvas = acquireTempCanvas(canvas.width, canvas.height);
```

After the operation finishes, add:

```js
releaseTempCanvas(tmpCanvas);
```

- [ ] **Step 5: Tighten history cap**

Replace:

```js
if (state.history.length > 20)
```

with:

```js
if (state.history.length > MAX_HISTORY)
```

- [ ] **Step 6: Verify focused and existing Pixel tests pass**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run "TestPixelEditorUsesCanvasPoolAndBoundedHistory|TestDesktopPixel" -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add ui/desktop_runtime_performance_test.go ui/js/desktop/apps/pixel.js
git commit -m "perf: bound Pixel editor canvas memory"
```

### Task 13: Finish Desktop Long-Session Cleanup Hooks

**Files:**

- Modify: `ui/desktop_runtime_performance_test.go`
- Modify: `ui/js/desktop/core/sdk-events-bootstrap.js`
- Modify: `ui/js/desktop/apps/quickconnect-launchpad-chat.js`
- Generated: `ui/js/desktop/bundles/main.bundle.js`

- [ ] **Step 1: Add the failing cleanup test**

Append to `ui/desktop_runtime_performance_test.go`:

```go
func TestDesktopLongSessionResourcesExposeCleanupHooks(t *testing.T) {
	t.Parallel()

	events := readEmbeddedText(t, "js/desktop/core/sdk-events-bootstrap.js")
	for _, marker := range []string{
		"function cleanupDesktopShellRuntime()",
		"clearInterval(state._clockTimer)",
		"cleanupDesktopWS();",
		"window.addEventListener('beforeunload', cleanupDesktopShellRuntime)",
	} {
		if !strings.Contains(events, marker) {
			t.Fatalf("desktop shell cleanup missing marker %q", marker)
		}
	}

	quickConnect := readEmbeddedText(t, "js/desktop/apps/quickconnect-launchpad-chat.js")
	for _, marker := range []string{
		"function disconnectActiveResizeObserver()",
		"disconnectActiveResizeObserver();",
		"activeResizeObserver = resizeObserver;",
	} {
		if !strings.Contains(quickConnect, marker) {
			t.Fatalf("quick connect resize observer cleanup missing marker %q", marker)
		}
	}
}
```

- [ ] **Step 2: Run the focused test and verify it fails**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestDesktopLongSessionResourcesExposeCleanupHooks -count=1
```

Expected: FAIL until cleanup hooks are present.

- [ ] **Step 3: Add shell cleanup**

In `ui/js/desktop/core/sdk-events-bootstrap.js`, add:

```js
    function cleanupDesktopShellRuntime() {
        if (state._clockTimer) {
            clearInterval(state._clockTimer);
            state._clockTimer = null;
        }
        cleanupDesktopWS();
        if (wsReconnectTimer) {
            clearTimeout(wsReconnectTimer);
            wsReconnectTimer = null;
        }
    }
```

In `init()`, after clock setup, add:

```js
        window.addEventListener('beforeunload', cleanupDesktopShellRuntime);
```

- [ ] **Step 4: Disconnect stale QuickConnect ResizeObserver before replacing it**

In `ui/js/desktop/apps/quickconnect-launchpad-chat.js`, near `activeResizeObserver`, add:

```js
        function disconnectActiveResizeObserver() {
            if (activeResizeObserver) {
                activeResizeObserver.disconnect();
                activeResizeObserver = null;
            }
        }
```

Call it immediately before creating a new `ResizeObserver` and in existing disconnect paths:

```js
disconnectActiveResizeObserver();
const resizeObserver = new ResizeObserver(() => {
    fit.fit();
});
activeResizeObserver = resizeObserver;
```

- [ ] **Step 5: Regenerate bundles**

```powershell
node scripts/build-ui-bundles.js
```

Expected: desktop main bundle rebuilt.

- [ ] **Step 6: Verify focused test passes**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run TestDesktopLongSessionResourcesExposeCleanupHooks -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add ui/desktop_runtime_performance_test.go ui/js/desktop/core/sdk-events-bootstrap.js ui/js/desktop/apps/quickconnect-launchpad-chat.js ui/js/desktop/bundles/main.bundle.js
git commit -m "fix: clean up desktop long-session resources"
```

## Final Verification

- [ ] **Step 1: Run focused UI regression suite**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -run "TestChat|TestDesktop.*Performance|TestDesktop.*Runtime|TestDesktopModuleLoader|TestDesktopAppsExposeDisposeLifecycle|TestFrontend" -count=1
```

Expected: PASS.

- [ ] **Step 2: Run all UI tests**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./ui -count=1
```

Expected: PASS.

- [ ] **Step 3: Run full Go test suite if time permits**

```powershell
$env:GOCACHE = (Resolve-Path disposable).Path + '\gocache'
go test ./... -count=1
```

Expected: PASS. If unrelated long-running or environment-dependent packages fail, record exact package/test names and rerun focused UI suite.

- [ ] **Step 4: Run GitNexus change detection before final commit or PR**

```text
mcp__gitnexus.detect_changes(scope: "all", repo: "AuraGo")
```

Expected: affected symbols match the planned chat/desktop runtime scope. Investigate any unexpected backend, security, or config flows before pushing.

- [ ] **Step 5: Manual browser smoke test**

Run AuraGo locally, open chat and desktop, and verify:

- A long streamed chat response remains smooth and auto-scrolls at most once per frame.
- SSE image/audio/video/document cards still render once, and markdown duplicate media is still suppressed.
- The scroll-to-bottom button still appears when the user scrolls up.
- Desktop windows can open, focus, minimize, close, and appear in the taskbar/dock without duplicate buttons.
- Desktop widgets still render, generated app iframes load, and widget reload does not leave active old iframe content.
- Agent chat in desktop streams text, tool status updates, media cards, and final markdown formatting.
- Pixel and Galaxa open, render, and close without console errors.

Expected: no console errors, no missing assets, no obvious layout overlap.

## Execution Order Recommendation

1. Wave 1 first: it has the highest user-visible impact and lowest blast radius.
2. Task 5 and Task 6 next: they reduce long-session leak risk without changing visible layout.
3. Task 7 and Task 8 after that: they touch core desktop shell behavior and need careful browser smoke testing.
4. Wave 3 after the shell is stable: centralization is valuable but higher security/regression risk.
5. Wave 4 last: canvas and lifecycle cleanup improve long sessions but need more manual app testing.
