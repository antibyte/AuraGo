(function () {
    'use strict';

    const instances = new Map();

    function render(container, windowId, context) {
        dispose(windowId);

        const { esc, api, t, iconMarkup, notify } = context;

        const state = {
            chatBusy: false,
            abortCtrl: null,
            lastRole: null,
            target: 'local',
            previewUrl: '',
            statusLoaded: false,
            homepageEnabled: false,
            disposed: false
        };
        instances.set(windowId, state);

        container.innerHTML = `
            <div class="vd-hp-studio">
                <div class="vd-hp-chat">
                    <div class="vd-hp-chat-header">
                        <span class="vd-hp-chat-header-label">${esc(t('homepage_studio.target_label', 'Target'))}</span>
                        <select class="vd-hp-target-select" id="hp-target-${windowId}">
                            <option value="local">${esc(t('homepage_studio.target_local', 'Local Server'))}</option>
                            <option value="vercel">${esc(t('homepage_studio.target_vercel', 'Vercel'))}</option>
                            <option value="netlify">${esc(t('homepage_studio.target_netlify', 'Netlify'))}</option>
                            <option value="remote">${esc(t('homepage_studio.target_remote', 'Remote Server'))}</option>
                        </select>
                        <span class="vd-hp-status-dot loading" id="hp-status-${windowId}" title="${esc(t('homepage_studio.checking_status', 'Checking status...'))}"></span>
                    </div>
                    <div class="vd-hp-chat-log" id="hp-log-${windowId}">
                        <div class="vd-hp-welcome">
                            <div class="vd-hp-welcome-icon">🌐</div>
                            <div class="vd-hp-welcome-heading">${esc(t('homepage_studio.welcome_heading', 'Homepage Studio'))}</div>
                            <div class="vd-hp-welcome-sub">${esc(t('homepage_studio.welcome', 'Welcome to Homepage Studio! Describe the website you want to build, and I\'ll create it for you.'))}</div>
                        </div>
                    </div>
                    <form class="vd-hp-chat-form" id="hp-form-${windowId}">
                        <textarea class="vd-hp-chat-input" id="hp-input-${windowId}" rows="1" placeholder="${esc(t('homepage_studio.chat_placeholder', 'Describe your website changes...'))}" autocomplete="off" enterkeyhint="send"></textarea>
                        <button type="submit" class="vd-hp-send-btn" id="hp-send-${windowId}">
                            ${iconMarkup('chat', 'S', 'vd-hp-send-icon', 15)}
                            <span id="hp-send-label-${windowId}">${esc(t('desktop.send', 'Send'))}</span>
                        </button>
                    </form>
                </div>
                <div class="vd-hp-preview">
                    <div class="vd-hp-preview-header">
                        <span class="vd-hp-preview-url" id="hp-url-${windowId}" title="${esc(t('homepage_studio.no_url', 'No preview URL available for this target'))}">—</span>
                        <div class="vd-hp-preview-actions">
                            <button type="button" class="vd-hp-preview-btn" id="hp-refresh-${windowId}" title="${esc(t('homepage_studio.refresh_preview', 'Refresh preview'))}">
                                ${iconMarkup('refresh', '↻', 'vd-hp-btn-icon', 14)}
                                <span>${esc(t('homepage_studio.refresh', 'Refresh'))}</span>
                            </button>
                            <button type="button" class="vd-hp-preview-btn" id="hp-external-${windowId}" title="${esc(t('homepage_studio.open_external', 'Open in new tab'))}">
                                ${iconMarkup('external', '↗', 'vd-hp-btn-icon', 14)}
                            </button>
                        </div>
                    </div>
                    <div class="vd-hp-preview-body" id="hp-preview-body-${windowId}">
                        <div class="vd-hp-preview-placeholder" id="hp-placeholder-${windowId}">
                            <div class="vd-hp-preview-placeholder-icon">🌐</div>
                            <div class="vd-hp-preview-placeholder-text">${esc(t('homepage_studio.preview_unavailable', 'Preview unavailable — start the homepage container first'))}</div>
                        </div>
                        <div class="vd-hp-preview-loading" id="hp-loading-${windowId}">
                            <div class="vd-hp-preview-spinner"></div>
                        </div>
                    </div>
                </div>
            </div>
        `;

        const $ = id => container.querySelector('#' + id);
        const chatLog = $(`hp-log-${windowId}`);
        const chatInput = $(`hp-input-${windowId}`);
        const chatForm = $(`hp-form-${windowId}`);
        const sendBtn = $(`hp-send-${windowId}`);
        const sendLabel = $(`hp-send-label-${windowId}`);
        const targetSelect = $(`hp-target-${windowId}`);
        const statusDot = $(`hp-status-${windowId}`);
        const previewUrl = $(`hp-url-${windowId}`);
        const previewBody = $(`hp-preview-body-${windowId}`);
        const previewPlaceholder = $(`hp-placeholder-${windowId}`);
        const previewLoading = $(`hp-loading-${windowId}`);
        const refreshBtn = $(`hp-refresh-${windowId}`);
        const externalBtn = $(`hp-external-${windowId}`);

        autoResizeTextarea(chatInput);

        chatInput.addEventListener('input', () => autoResizeTextarea(chatInput));
        chatInput.addEventListener('keydown', e => {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                chatForm.dispatchEvent(new Event('submit', { cancelable: true }));
            }
        });

        chatForm.addEventListener('submit', e => {
            e.preventDefault();
            if (state.chatBusy) {
                if (state.abortCtrl) state.abortCtrl.abort();
                return;
            }
            const msg = chatInput.value.trim();
            if (msg) sendMessage(msg);
        });

        targetSelect.addEventListener('change', () => {
            state.target = targetSelect.value;
            loadStatus();
        });

        refreshBtn.addEventListener('click', () => refreshPreview());
        externalBtn.addEventListener('click', () => {
            if (state.previewUrl) window.open(state.previewUrl, '_blank');
        });

        loadStatus();

        function autoResizeTextarea(el) {
            if (!el) return;
            el.style.height = 'auto';
            const max = 120;
            el.style.height = Math.min(el.scrollHeight, max) + 'px';
            el.style.overflowY = el.scrollHeight > max ? 'auto' : 'hidden';
        }

        async function loadStatus() {
            statusDot.className = 'vd-hp-status-dot loading';
            statusDot.title = t('homepage_studio.checking_status', 'Checking status...');
            try {
                const data = await api('/api/homepage/status');
                state.statusLoaded = true;
                state.homepageEnabled = data && data.status !== 'disabled';

                if (!state.homepageEnabled) {
                    statusDot.className = 'vd-hp-status-dot offline';
                    statusDot.title = t('homepage_studio.status_disabled', 'Homepage is disabled');
                    state.previewUrl = '';
                    updatePreviewUrl();
                    return;
                }

                const webRunning = data && data.web_container && data.web_container.running;
                statusDot.className = webRunning ? 'vd-hp-status-dot online' : 'vd-hp-status-dot offline';
                statusDot.title = webRunning
                    ? t('homepage_studio.status_online', 'Web server running')
                    : t('homepage_studio.status_offline', 'Web server not running');

                if (webRunning) {
                    const port = 8080;
                    state.previewUrl = 'http://localhost:' + port;
                    if (data.tunnel_url) {
                        state.previewUrl = data.tunnel_url;
                    }
                } else {
                    state.previewUrl = '';
                }
                updatePreviewUrl();
            } catch (_) {
                statusDot.className = 'vd-hp-status-dot offline';
                statusDot.title = t('homepage_studio.status_error', 'Could not check status');
                state.previewUrl = '';
                updatePreviewUrl();
            }
        }

        function updatePreviewUrl() {
            if (state.previewUrl) {
                previewUrl.textContent = state.previewUrl;
                previewUrl.title = state.previewUrl;
                showPreview(state.previewUrl);
            } else {
                previewUrl.textContent = '—';
                previewUrl.title = t('homepage_studio.no_url', 'No preview URL available for this target');
                hidePreview();
            }
        }

        function showPreview(url) {
            let iframe = previewBody.querySelector('.vd-hp-preview-iframe');
            if (!iframe) {
                previewPlaceholder.style.display = 'none';
                iframe = document.createElement('iframe');
                iframe.className = 'vd-hp-preview-iframe';
                iframe.sandbox = 'allow-scripts allow-same-origin allow-forms allow-popups';
                previewBody.insertBefore(iframe, previewLoading);
            }
            if (iframe.src !== url) {
                previewLoading.classList.add('active');
                iframe.onload = () => previewLoading.classList.remove('active');
                iframe.onerror = () => previewLoading.classList.remove('active');
                iframe.src = url;
            }
        }

        function hidePreview() {
            const iframe = previewBody.querySelector('.vd-hp-preview-iframe');
            if (iframe) iframe.remove();
            previewPlaceholder.style.display = '';
        }

        function refreshPreview() {
            if (!state.previewUrl) {
                loadStatus();
                return;
            }
            previewLoading.classList.add('active');
            const iframe = previewBody.querySelector('.vd-hp-preview-iframe');
            if (iframe) {
                iframe.onload = () => previewLoading.classList.remove('active');
                iframe.src = state.previewUrl;
            } else {
                showPreview(state.previewUrl);
            }
        }

        async function sendMessage(message) {
            chatInput.value = '';
            autoResizeTextarea(chatInput);
            state.chatBusy = true;
            setBusy(true);

            const welcome = chatLog.querySelector('.vd-hp-welcome');
            if (welcome) welcome.remove();

            appendBubble('user', message);

            const renderer = window.DesktopChatRenderer;
            const statusEl = renderer ? renderer.createThinkingStatus() : null;
            if (statusEl) chatLog.appendChild(statusEl);

            let streamingBubble = null;
            let streamingContent = '';
            let streamTextFrame = 0;
            let finalized = false;

            function flushStreamingBubble() {
                streamTextFrame = 0;
                if (!streamingBubble || !streamingBubble.classList.contains('vd-streaming')) return;
                streamingBubble.textContent = streamingContent;
                scrollToEnd();
            }

            function queueFlush() {
                if (streamTextFrame) return;
                const schedule = window.requestAnimationFrame || (cb => window.setTimeout(cb, 16));
                streamTextFrame = schedule(flushStreamingBubble);
            }

            function scrollToEnd() {
                chatLog.scrollTop = chatLog.scrollHeight;
            }

            function doFinalize() {
                if (finalized) return;
                finalized = true;
                if (streamTextFrame) {
                    (window.cancelAnimationFrame || window.clearTimeout)(streamTextFrame);
                    streamTextFrame = 0;
                }
                if (statusEl && statusEl.parentNode) statusEl.remove();
                if (streamingBubble) {
                    streamingBubble.classList.remove('vd-streaming');
                    if (renderer && streamingContent.trim()) {
                        streamingBubble.innerHTML = renderer.renderMarkdown(streamingContent);
                        renderer.processImages(streamingBubble);
                        renderer.enhanceCodeBlocks(streamingBubble);
                        if (window.MermaidLoader) window.MermaidLoader.processBlocks(streamingBubble);
                    }
                }
                state.chatBusy = false;
                state.abortCtrl = null;
                setBusy(false);
                scrollToEnd();

                if (streamingContent.trim()) {
                    setTimeout(() => refreshPreview(), 500);
                }
            }

            function doReject(err) {
                if (finalized) return;
                finalized = true;
                if (streamTextFrame) {
                    (window.cancelAnimationFrame || window.clearTimeout)(streamTextFrame);
                    streamTextFrame = 0;
                }
                if (statusEl && statusEl.parentNode) statusEl.remove();
                const msg = (err && err.message) || String(err || 'Request failed');
                appendBubble('agent', msg);
                state.chatBusy = false;
                state.abortCtrl = null;
                setBusy(false);
            }

            const ctrl = new AbortController();
            state.abortCtrl = ctrl;

            const chatContext = {
                source: 'homepage-studio',
                target: state.target,
                homepage_mode: true
            };

            try {
                const response = await fetch('/api/desktop/chat/stream', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ message, context: chatContext }),
                    signal: ctrl.signal
                });

                if (!response.ok) {
                    const text = await response.text();
                    throw new Error(text || ('HTTP ' + response.status));
                }

                const parser = window.AuraChatStreamParser;
                if (!parser) throw new Error('Chat stream parser not loaded');

                await parser.readFetchEventStream(response, {
                    onEvent: data => {
                        if (!data) return;
                        const event = data.event || data.type;

                        if (event === 'llm_stream_delta') {
                            const content = data.content || '';
                            if (!content) return;
                            if (!streamingBubble) {
                                streamingBubble = document.createElement('div');
                                streamingBubble.className = 'vd-chat-bubble agent vd-streaming';
                                chatLog.appendChild(streamingBubble);
                                if (renderer) renderer.appendTimestamp(chatLog, 'agent');
                                state.lastRole = 'agent';
                            }
                            streamingContent += content;
                            if (streamingBubble.classList.contains('vd-streaming')) queueFlush();
                        } else if (event === 'thinking_block') {
                            if (statusEl && (data.state === 'start') && renderer) {
                                renderer.updateStatus(statusEl, t('desktop.chat_thinking', 'Reasoning...'));
                            }
                        } else if (event === 'thinking' || event === 'tool_start' || event === 'tool_end' ||
                            event === 'co_agent_spawn' || event === 'workflow_plan' || event === 'coding' ||
                            event === 'error_recovery' || event === 'agent_action') {
                            if (statusEl && renderer) {
                                const status = renderer.formatAgentActionStatus(data);
                                if (status) renderer.updateStatus(statusEl, status);
                            }
                        } else if (event === 'tool_call') {
                            if (renderer) {
                                const text = renderer.extractToolCallNarration(data.detail || data.message || '');
                                if (text) {
                                    appendBubble('agent', text);
                                }
                            }
                        } else if (event === 'final_response') {
                            if (data.detail || data.message) {
                                const text = data.detail || data.message || '';
                                if (!streamingBubble && text.trim()) {
                                    appendBubble('agent', text);
                                } else if (streamingBubble && !streamingContent.trim() && text.trim()) {
                                    streamingContent = text;
                                    flushStreamingBubble();
                                }
                            }
                        } else if (event === 'done') {
                            doFinalize();
                        }
                    },
                    onDone: () => doFinalize(),
                    onError: err => doReject(err)
                });
            } catch (err) {
                if (err.name === 'AbortError') {
                    doFinalize();
                } else {
                    doReject(err);
                }
            }
        }

        function appendBubble(role, text) {
            const renderer = window.DesktopChatRenderer;
            if (renderer) {
                renderer.appendRichBubble(chatLog, role, text, state.lastRole);
            } else {
                const bubble = document.createElement('div');
                bubble.className = 'vd-chat-bubble ' + role;
                bubble.textContent = text;
                chatLog.appendChild(bubble);
            }
            state.lastRole = role;
            chatLog.scrollTop = chatLog.scrollHeight;
        }

        function setBusy(busy) {
            chatInput.disabled = !!busy;
            sendBtn.classList.toggle('is-stop', !!busy);
            const sendText = t('desktop.send', 'Send');
            const stopText = t('desktop.chat_stop', 'Stop');
            sendLabel.textContent = busy ? stopText : sendText;
            sendBtn.title = busy ? stopText : sendText;
        }
    }

    function dispose(windowId) {
        const state = instances.get(windowId);
        if (!state) return;
        state.disposed = true;
        if (state.abortCtrl) { state.abortCtrl.abort(); state.abortCtrl = null; }
        instances.delete(windowId);
    }

    window.HomepageStudioApp = { render, dispose };
})();
