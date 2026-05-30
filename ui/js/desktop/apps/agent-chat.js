    const state = { chatBusy: false };
    let desktopRuntime = {};
    let sidebarOpen = true;
    let lastRole = null;

    const WELCOME_PROMPTS = [
        'desktop.chat_prompt_what_can_you_do',
        'desktop.chat_prompt_help_code',
        'desktop.chat_prompt_analyze_files',
        'desktop.chat_prompt_explain'
    ];

    const WELCOME_PROMPT_FALLBACKS = [
        'What can you do?',
        'Help me with code',
        'Analyze my files',
        'Explain something'
    ];

    function useDesktopChatRuntime(context) {
        if (context && context.__desktopRuntime) desktopRuntime = context.__desktopRuntime;
        return desktopRuntime || {};
    }

    function agentChatContentEl(id) {
        const runtime = useDesktopChatRuntime();
        if (runtime && typeof runtime.contentEl === 'function') return runtime.contentEl(id);
        const windowId = String(id || '');
        const windows = document.querySelectorAll('.vd-window[data-window-id]');
        for (const win of windows) {
            if (win && win.dataset && win.dataset.windowId === windowId) {
                return win.querySelector('[data-window-content]');
            }
        }
        return null;
    }

    function esc(value) {
        const runtime = useDesktopChatRuntime();
        if (runtime && typeof runtime.esc === 'function') return runtime.esc(value);
        return String(value ?? '').replace(/[&<>"']/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch]));
    }

    function desktopText(key, fallback) {
        const runtime = useDesktopChatRuntime();
        if (runtime && typeof runtime.desktopText === 'function') return runtime.desktopText(key, fallback);
        const translated = typeof t === 'function' ? t(key) : key;
        return translated && translated !== key ? translated : fallback;
    }

    function iconMarkup(key, fallback, className, size) {
        const runtime = useDesktopChatRuntime();
        if (runtime && typeof runtime.iconMarkup === 'function') return runtime.iconMarkup(key, fallback, className, size);
        return `<span class="${esc(className || '')}" aria-hidden="true">${esc(fallback || '')}</span>`;
    }

    async function api(url, options) {
        const runtime = useDesktopChatRuntime();
        if (runtime && typeof runtime.api === 'function') return runtime.api(url, options);
        const requestOptions = Object.assign({ credentials: 'same-origin', cache: 'no-store' }, options || {});
        const resp = await fetch(url, requestOptions);
        const contentType = resp.headers.get('content-type') || '';
        const body = contentType.includes('application/json') ? await resp.json() : {};
        if (!resp.ok) throw new Error(body.error || body.message || ('HTTP ' + resp.status));
        return body;
    }

    async function loadBootstrap() {
        const runtime = useDesktopChatRuntime();
        if (runtime && typeof runtime.loadBootstrap === 'function') return runtime.loadBootstrap();
        return null;
    }

    function showDesktopNotification(payload) {
        const runtime = useDesktopChatRuntime();
        if (runtime && typeof runtime.showDesktopNotification === 'function') {
            return runtime.showDesktopNotification(payload);
        }
        return null;
    }

    function chatFileContextFromEntry(entry) {
        const path = String((entry && entry.path) || '').replace(/\\/g, '/').replace(/\/+/g, '/').trim();
        if (!path) return null;
        return {
            path,
            name: (entry && (entry.name || entry.filename)) || path.split('/').pop() || path,
            web_path: (entry && entry.web_path) || '',
            media_kind: (entry && entry.media_kind) || '',
            mime_type: (entry && entry.mime_type) || ''
        };
    }

    function renderChat(id, context) {
        useDesktopChatRuntime(context || {});
        const host = agentChatContentEl(id);
        if (!host) throw new Error('Desktop chat window content is not available');

        const sidebarCollapsed = !sidebarOpen;
        host.innerHTML = `<div class="vd-chat" data-sidebar-collapsed="${sidebarCollapsed}" ${sidebarOpen ? 'data-sidebar-open="true"' : ''}>
            <div class="vd-chat-sidebar">
                <div class="vd-chat-sidebar-header">
                    <span class="vd-chat-sidebar-title">${esc(desktopText('desktop.chat_sessions', 'Conversations'))}</span>
                    <button class="vd-chat-new-chat-btn" type="button" data-chat-new title="${esc(desktopText('desktop.chat_new', 'New Chat'))}" aria-label="${esc(desktopText('desktop.chat_new', 'New Chat'))}">
                        ${iconMarkup('plus', '+', 'vd-chat-sidebar-icon', 14)}<span>${esc(desktopText('desktop.chat_new', 'New Chat'))}</span>
                    </button>
                </div>
                <div class="vd-chat-sidebar-search">
                    <input type="text" placeholder="${esc(desktopText('desktop.chat_search', 'Search conversations...'))}" data-chat-sidebar-search autocomplete="off">
                </div>
                <div class="vd-chat-sidebar-list" data-chat-sidebar-list></div>
            </div>
            <div class="vd-chat-sidebar-backdrop" data-chat-sidebar-backdrop></div>
            <div class="vd-chat-toolbar">
                <div class="vd-chat-toolbar-left">
                    <button class="vd-chat-sidebar-toggle" type="button" data-chat-sidebar-toggle title="${esc(desktopText('desktop.chat_toggle_sidebar', 'Toggle sidebar'))}" aria-label="${esc(desktopText('desktop.chat_toggle_sidebar', 'Toggle sidebar'))}">
                        ${iconMarkup('menu', '☰', 'vd-chat-toolbar-icon', 16)}
                    </button>
                </div>
                <div class="vd-chat-toolbar-right">
                    <button class="vd-chat-clear-history" type="button" data-chat-clear-history title="${esc(desktopText('desktop.chat_clear_history', 'Clear history'))}" aria-label="${esc(desktopText('desktop.chat_clear_history', 'Clear history'))}">
                        ${iconMarkup('trash', 'X', 'vd-chat-toolbar-icon', 14)}<span>${esc(desktopText('desktop.chat_clear_history', 'Clear history'))}</span>
                    </button>
                </div>
            </div>
            <div class="vd-chat-main">
                <div class="vd-chat-log"></div>
                <div class="vd-chat-scroll-fab" data-chat-scroll-fab aria-label="${esc(desktopText('desktop.chat_scroll_bottom', 'Scroll to bottom'))}">
                    ${iconMarkup('chevron-down', '↓', 'vd-chat-scroll-icon', 18)}
                </div>
                <div class="vd-chat-drop-overlay" data-chat-drop-overlay>
                    <div class="vd-chat-drop-overlay-content">
                        <div class="vd-chat-drop-overlay-icon">📎</div>
                        <div class="vd-chat-drop-overlay-text">${esc(desktopText('desktop.chat_drop_files', 'Drop files here'))}</div>
                    </div>
                </div>
                <div class="vd-chat-context" data-chat-context hidden></div>
                <form class="vd-chat-form">
                    <div class="vd-chat-input-wrap">
                        <textarea class="vd-chat-input" rows="1" autocomplete="off" placeholder="${esc(desktopText('desktop.chat_placeholder', 'Type a message...'))}"></textarea>
                        <span class="vd-chat-input-counter" data-chat-input-counter></span>
                    </div>
                    <div class="vd-chat-form-buttons">
                        <button class="vd-chat-voice" type="button" data-i18n-title="desktop.chat_voice_input" data-i18n-aria-label="desktop.chat_voice_input">${iconMarkup('microphone', 'M', 'vd-chat-voice-icon', 15)}</button>
                        <button class="vd-chat-send" type="submit" data-chat-send-button>${iconMarkup('chat', 'S', 'vd-chat-send-icon', 15)}<span data-chat-send-label>${esc(desktopText('desktop.send', 'Send'))}</span></button>
                    </div>
                </form>
            </div>
        </div>`;

        const input = host.querySelector('.vd-chat-input');
        const voiceBtn = host.querySelector('.vd-chat-voice');

        initTextarea(host, input);
        initDesktopChatVoice(host, input, voiceBtn);
        initSidebar(host);
        initDragAndDrop(host);
        initScrollFab(host);
        setDesktopChatBusy(host, false);

        loadDesktopChatHistory(host).finally(() => applyChatLaunchContext(id, context || {}));

        const clearHistory = host.querySelector('[data-chat-clear-history]');
        if (clearHistory) clearHistory.addEventListener('click', () => clearDesktopChatHistory(host));

        host.querySelector('form').addEventListener('submit', async (event) => {
            event.preventDefault();
            if (state.chatBusy) {
                if (event.submitter && event.submitter.classList && event.submitter.classList.contains('vd-chat-send')) requestDesktopChatAbort(host);
                return;
            }
            await submitDesktopChatMessage(host, input.value.trim());
        });
    }

    function initTextarea(host, input) {
        if (!input) return;
        const counter = host.querySelector('[data-chat-input-counter]');

        function autoResize() {
            input.style.height = 'auto';
            const maxHeight = 150;
            input.style.height = Math.min(input.scrollHeight, maxHeight) + 'px';
            input.style.overflowY = input.scrollHeight > maxHeight ? 'auto' : 'hidden';
        }

        function updateCounter() {
            if (!counter) return;
            const len = input.value.length;
            if (len > 50) {
                counter.textContent = len;
                counter.classList.add('visible');
            } else {
                counter.classList.remove('visible');
            }
        }

        input.addEventListener('input', () => { autoResize(); updateCounter(); });
        input.addEventListener('keydown', (event) => {
            if (event.key === 'Enter' && !event.shiftKey) {
                event.preventDefault();
                const form = host.querySelector('form');
                if (form) form.dispatchEvent(new Event('submit', { cancelable: true }));
            }
        });

        autoResize();
    }

    function initSidebar(host) {
        const chat = host.querySelector('.vd-chat');
        const toggleBtn = host.querySelector('[data-chat-sidebar-toggle]');
        const backdrop = host.querySelector('[data-chat-sidebar-backdrop]');
        const newChatBtn = host.querySelector('[data-chat-new]');
        const searchInput = host.querySelector('[data-chat-sidebar-search]');

        function updateSidebarState() {
            if (!chat) return;
            const isWide = host.offsetWidth > 900;
            chat.dataset.sidebarCompact = isWide ? 'false' : 'true';
            const sidebarCollapsed = !sidebarOpen;
            chat.dataset.sidebarCollapsed = sidebarCollapsed ? 'true' : 'false';
            if (!isWide && sidebarOpen) chat.dataset.sidebarOpen = 'true';
            else chat.removeAttribute('data-sidebar-open');
        }

        if (toggleBtn) toggleBtn.addEventListener('click', () => {
            sidebarOpen = !sidebarOpen;
            updateSidebarState();
        });

        if (backdrop) backdrop.addEventListener('click', () => {
            sidebarOpen = false;
            updateSidebarState();
        });

        if (newChatBtn) newChatBtn.addEventListener('click', () => {
            clearDesktopChatHistory(host, true);
        });

        if (searchInput) searchInput.addEventListener('input', () => {
            filterSidebarItems(host, searchInput.value.trim().toLowerCase());
        });

        renderSidebarList(host);
        updateSidebarState();

        const ro = new ResizeObserver(() => updateSidebarState());
        ro.observe(host);
        host._sidebarResizeObserver = ro;
    }

    function renderSidebarList(host) {
        const list = host.querySelector('[data-chat-sidebar-list]');
        if (!list) return;
        const sessionId = 'virtual-desktop';
        const title = desktopText('desktop.chat_current_session', 'Current Session');
        const time = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
        list.innerHTML = `<div class="vd-chat-sidebar-item active" data-session-id="${esc(sessionId)}">
            <div class="vd-chat-sidebar-item-content">
                <div class="vd-chat-sidebar-item-title">${esc(title)}</div>
                <div class="vd-chat-sidebar-item-time">${esc(time)}</div>
            </div>
        </div>`;
    }

    function filterSidebarItems(host, query) {
        const items = host.querySelectorAll('.vd-chat-sidebar-item');
        items.forEach(item => {
            const title = item.querySelector('.vd-chat-sidebar-item-title');
            const text = title ? title.textContent.toLowerCase() : '';
            item.style.display = (!query || text.includes(query)) ? '' : 'none';
        });
    }

    function initDragAndDrop(host) {
        const chatMain = host.querySelector('.vd-chat-main');
        const overlay = host.querySelector('[data-chat-drop-overlay]');
        if (!chatMain || !overlay) return;

        if (typeof host._desktopChatDropCleanup === 'function') {
            host._desktopChatDropCleanup();
        }

        let dragCounter = 0;

        function hasFileDrag(event) {
            const types = Array.from((event && event.dataTransfer && event.dataTransfer.types) || []);
            return types.includes('Files');
        }

        function clearDropOverlay() {
            dragCounter = 0;
            overlay.classList.remove('active');
        }

        chatMain.addEventListener('dragenter', (event) => {
            if (!hasFileDrag(event)) return;
            event.preventDefault();
            dragCounter++;
            overlay.classList.add('active');
        });

        chatMain.addEventListener('dragleave', (event) => {
            if (!hasFileDrag(event)) return;
            event.preventDefault();
            dragCounter--;
            if (dragCounter <= 0 || !chatMain.contains(event.relatedTarget)) {
                clearDropOverlay();
            }
        });

        chatMain.addEventListener('dragover', (event) => {
            if (!hasFileDrag(event)) return;
            event.preventDefault();
            if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy';
        });

        chatMain.addEventListener('drop', (event) => {
            if (!hasFileDrag(event)) {
                clearDropOverlay();
                return;
            }
            event.preventDefault();
            clearDropOverlay();

            const files = event.dataTransfer && event.dataTransfer.files;
            if (!files || !files.length) return;

            const fileEntries = [];
            for (let i = 0; i < files.length; i++) {
                const file = files[i];
                if (file && file.name) {
                    fileEntries.push({ path: file.name, name: file.name, mime_type: file.type || '' });
                }
            }

            if (fileEntries.length) {
                const existing = chatAttachedFiles(host);
                const merged = existing.slice();
                fileEntries.forEach(entry => {
                    if (!merged.some(f => f.path === entry.path)) merged.push(entry);
                });
                host.dataset.chatFiles = JSON.stringify(merged);
                renderChatContextBar(host);
            }
        });

        window.addEventListener('drop', clearDropOverlay, true);
        window.addEventListener('dragend', clearDropOverlay, true);
        host._desktopChatDropCleanup = () => {
            window.removeEventListener('drop', clearDropOverlay, true);
            window.removeEventListener('dragend', clearDropOverlay, true);
        };
    }

    function initScrollFab(host) {
        const chatLog = host.querySelector('.vd-chat-log');
        const fab = host.querySelector('[data-chat-scroll-fab]');
        if (!chatLog || !fab) return;

        chatLog.addEventListener('scroll', () => {
            const distanceFromBottom = chatLog.scrollHeight - chatLog.scrollTop - chatLog.clientHeight;
            fab.classList.toggle('visible', distanceFromBottom > 200);
        });

        fab.addEventListener('click', () => {
            chatLog.scrollTo({ top: chatLog.scrollHeight, behavior: 'smooth' });
        });
    }

    function updateSidebarSessionTitle(host, text) {
        const title = host.querySelector('.vd-chat-sidebar-item-title');
        if (title && text) {
            title.textContent = text.slice(0, 50) + (text.length > 50 ? '...' : '');
        }
    }

    async function submitDesktopChatMessage(host, message) {
        const input = host && host.querySelector('.vd-chat-input');
        message = String(message || '').trim();
        if (!host || !message || state.chatBusy) return;
        if (input) { input.value = ''; input.style.height = 'auto'; }
        host._desktopChatHistoryToken = null;
        state.chatBusy = true;
        setDesktopChatBusy(host, true);

        if (!lastRole) {
            const chatLog = host.querySelector('.vd-chat-log');
            const welcome = chatLog && chatLog.querySelector('.vd-chat-welcome');
            if (welcome) welcome.remove();
        }

        updateSidebarSessionTitle(host, message);

        const chatLog = host.querySelector('.vd-chat-log');
        const renderer = window.DesktopChatRenderer;
        if (renderer) renderer.appendRichBubble(chatLog, 'user', message, lastRole);
        else appendChat(host, 'user', message);
        lastRole = 'user';

        try {
            await sendDesktopChatStream(host, message, chatContextPayload(host));
            try { await loadBootstrap(); } catch (_) { }
        } catch (err) {
            if (!isDesktopChatAbortError(err)) appendDesktopChatError(host, err);
        } finally {
            delete host.dataset.chatWindowContext;
            renderChatContextBar(host);
            state.chatBusy = false;
            host._desktopChatAbort = null;
            setDesktopChatBusy(host, false);
        }
    }

    async function loadDesktopChatHistory(host) {
        const chatLog = host && host.querySelector('.vd-chat-log');
        if (!chatLog) return;
        const token = Symbol('desktop-chat-history');
        host._desktopChatHistoryToken = token;
        chatLog.innerHTML = `<div class="vd-chat-history-status">${esc(desktopText('desktop.loading', 'Loading...'))}</div>`;
        try {
            const messages = await api('/history?session_id=virtual-desktop');
            if (host._desktopChatHistoryToken !== token) return;
            chatLog.innerHTML = '';
            lastRole = null;
            const visible = (Array.isArray(messages) ? messages : [])
                .map(normalizeDesktopChatHistoryMessage)
                .filter(Boolean)
                .slice(-60);
            if (!visible.length) {
                appendDesktopChatWelcome(host);
                return;
            }
            visible.forEach(message => appendDesktopChatHistoryBubble(host, message));
            chatLog.scrollTop = chatLog.scrollHeight;
        } catch (err) {
            if (host._desktopChatHistoryToken !== token) return;
            chatLog.innerHTML = '';
            lastRole = null;
            appendDesktopChatWelcome(host);
            if (typeof showDesktopNotification === 'function') {
                showDesktopNotification({ message: desktopText('desktop.chat_history_load_error', 'Could not load chat history.') });
            }
        }
    }

    function normalizeDesktopChatHistoryMessage(message) {
        if (!message || !message.role) return null;
        const rawRole = String(message.role || '').toLowerCase();
        const role = rawRole === 'assistant' || rawRole === 'agent' ? 'agent' : (rawRole === 'user' ? 'user' : '');
        if (!role) return null;
        const text = desktopChatHistoryDisplayText(role, message.content || '');
        if (!text) return null;
        return { role, text, timestamp: message.timestamp || message.Timestamp || '' };
    }

    function desktopChatHistoryDisplayText(role, content) {
        let text = decodeDesktopChatHistoryEntities(content).replace(/<done\s*\/?>/gi, '').trim();
        if (role === 'user') {
            text = text.replace(/^\s*;\s*(?=<external_data\b)/i, '').trim();
            const typed = text.match(/<external_data\b[^>]*type=["']desktop_user_request["'][^>]*>([\s\S]*?)<\/external_data>/i);
            if (typed) text = typed[1];
            else {
                const marker = text.match(/User request:\s*([\s\S]*)$/i);
                if (marker) text = marker[1];
            }
            text = text.replace(/<\/?external_data[^>]*>/gi, '').trim();
        }
        return text.replace(/\n{3,}/g, '\n\n').trim();
    }

    function decodeDesktopChatHistoryEntities(content) {
        let text = String(content || '');
        for (let i = 0; i < 2; i += 1) {
            const next = text
                .replace(/&lt;/gi, '<')
                .replace(/&gt;/gi, '>')
                .replace(/&quot;|&#34;/gi, '"')
                .replace(/&#39;|&apos;/gi, "'")
                .replace(/&amp;/gi, '&');
            if (next === text) break;
            text = next;
        }
        return text;
    }

    function appendDesktopChatWelcome(host) {
        const chatLog = host && host.querySelector('.vd-chat-log');
        if (!chatLog) return;

        const renderer = window.DesktopChatRenderer;
        if (!renderer) {
            appendChat(host, 'agent', t('desktop.chat_welcome'));
            return;
        }

        // Legacy path: renderer.appendRichBubble(chatLog, 'agent', t('desktop.chat_welcome'));

        const avatarHtml = (window.AuraChatCore && typeof window.AuraChatCore.personaAvatarMarkup === 'function')
            ? window.AuraChatCore.personaAvatarMarkup('agent')
            : iconMarkup('chat', '🤖', '', 32);

        const promptChips = WELCOME_PROMPTS.map((key, i) => {
            const label = desktopText(key, WELCOME_PROMPT_FALLBACKS[i]);
            return `<button class="vd-chat-welcome-prompt" type="button" data-prompt-index="${i}">${esc(label)}</button>`;
        }).join('');

        const welcome = document.createElement('div');
        welcome.className = 'vd-chat-welcome';
        welcome.innerHTML = `
            <div class="vd-chat-welcome-avatar">${avatarHtml}</div>
            <div class="vd-chat-welcome-heading">${esc(desktopText('desktop.chat_welcome_heading', 'How can I help you?'))}</div>
            <div class="vd-chat-welcome-sub">${esc(desktopText('desktop.chat_welcome_sub', 'Ask me anything or try a suggestion below'))}</div>
            <div class="vd-chat-welcome-prompts">${promptChips}</div>
        `;
        chatLog.appendChild(welcome);

        welcome.querySelectorAll('.vd-chat-welcome-prompt').forEach(btn => {
            btn.addEventListener('click', () => {
                const idx = parseInt(btn.getAttribute('data-prompt-index'), 10);
                const key = WELCOME_PROMPTS[idx];
                const text = desktopText(key, WELCOME_PROMPT_FALLBACKS[idx]);
                submitDesktopChatMessage(host, text);
            });
        });

        lastRole = null;
    }

    function appendDesktopChatHistoryBubble(host, message) {
        const chatLog = host && host.querySelector('.vd-chat-log');
        if (!chatLog || !message || !message.text) return;
        const renderer = window.DesktopChatRenderer;
        if (!renderer) {
            appendChat(host, message.role, message.text);
            lastRole = message.role;
            return;
        }

        const isGroup = lastRole === message.role;
        const bubble = renderer.createBubble(message.role, '');
        if (isGroup) bubble.dataset.roleGroup = 'continuation';

        if (message.role === 'user') {
            bubble.textContent = message.text;
        } else {
            bubble.innerHTML = renderer.renderMarkdown(message.text);
            renderer.processImages(bubble);
            renderer.enhanceCodeBlocks(bubble);
            if (window.MermaidLoader) window.MermaidLoader.processBlocks(bubble);
        }

        if (renderer.appendAvatar) {
            renderer.appendAvatar(chatLog, message.role, bubble, isGroup);
        } else {
            chatLog.appendChild(bubble);
        }

        renderer.appendTimestamp(chatLog, message.role, message.timestamp);
        lastRole = message.role;
    }

    async function clearDesktopChatHistory(host, silent) {
        if (!host || state.chatBusy) return;
        if (!silent) {
            const ok = await confirmDesktopChatClear(host);
            if (!ok) return;
        }
        try {
            await api('/clear?session_id=virtual-desktop', { method: 'DELETE' });
            host._desktopChatHistoryToken = null;
            const chatLog = host.querySelector('.vd-chat-log');
            if (chatLog) chatLog.innerHTML = '';
            lastRole = null;
            appendDesktopChatWelcome(host);
            if (!silent && typeof showDesktopNotification === 'function') {
                showDesktopNotification({ message: desktopText('desktop.chat_history_cleared', 'Chat history cleared.') });
            }
        } catch (err) {
            appendDesktopChatError(host, err);
        }
    }

    function confirmDesktopChatClear(host) {
        return new Promise(resolve => {
            const container = host && host.querySelector('.vd-chat');
            if (!container) { resolve(false); return; }
            const overlay = document.createElement('div');
            overlay.className = 'vd-qc-modal-overlay';
            overlay.innerHTML = `<div class="vd-qc-confirm">
                <div class="vd-qc-confirm-title">${esc(desktopText('desktop.chat_clear_history', 'Clear history'))}</div>
                <div class="vd-qc-confirm-msg">${esc(desktopText('desktop.chat_clear_confirm', 'Delete the visible desktop chat history?'))}</div>
                <div class="vd-qc-confirm-actions">
                    <button class="vd-qc-btn vd-qc-btn-secondary" type="button" data-action="cancel">${iconMarkup('x', 'X', 'vd-qc-btn-icon', 14)}<span>${esc(desktopText('desktop.cancel', 'Cancel'))}</span></button>
                    <button class="vd-qc-btn vd-qc-btn-danger" type="button" data-action="ok">${iconMarkup('trash', 'X', 'vd-qc-btn-icon', 14)}<span>${esc(desktopText('desktop.delete', 'Delete'))}</span></button>
                </div>
            </div>`;
            container.appendChild(overlay);
            overlay.querySelector('[data-action="cancel"]').addEventListener('click', () => { overlay.remove(); resolve(false); });
            overlay.querySelector('[data-action="ok"]').addEventListener('click', () => { overlay.remove(); resolve(true); });
            overlay.addEventListener('click', event => {
                if (event.target === overlay) { overlay.remove(); resolve(false); }
            });
        });
    }

    function normalizeDesktopQuestionPayload(data) {
        if (!data) return null;
        let payload = data.payload || data.question || data.detail || data.message || data;
        if (typeof payload === 'string') {
            try {
                const parsed = JSON.parse(payload);
                payload = parsed.payload || parsed.question || parsed;
            } catch (_) {
                payload = { question: payload, options: [] };
            }
        }
        if (!payload || typeof payload !== 'object') return null;
        return payload;
    }

    function showDesktopQuestionModal(host, payload) {
        const container = host && host.querySelector('.vd-chat');
        if (!container || !payload || !payload.question) return;
        const existing = container.querySelector('[data-desktop-question-modal]');
        if (existing) existing.remove();

        const timeoutSeconds = Math.max(1, Number(payload.timeout_seconds || 120));
        const startedAt = Date.now();
        const overlay = document.createElement('div');
        overlay.className = 'vd-qc-modal-overlay vd-chat-question-overlay';
        overlay.setAttribute('data-desktop-question-modal', 'true');
        overlay.setAttribute('role', 'dialog');
        overlay.setAttribute('aria-modal', 'true');

        const options = Array.isArray(payload.options) ? payload.options : [];
        const optionButtons = options.map((opt, index) => {
            const value = esc(opt.value || opt.label || String(index + 1));
            const label = esc(opt.label || opt.value || String(index + 1));
            const desc = opt.description ? `<span class="vd-chat-question-option-desc">${esc(opt.description)}</span>` : '';
            return `<button class="vd-qc-btn vd-qc-btn-secondary vd-chat-question-option" type="button" data-value="${value}">
                <span>${label}</span>${desc}
            </button>`;
        }).join('');
        const freeText = payload.allow_free_text ? `<form class="vd-chat-question-free-text" data-question-free-text>
                <input type="text" autocomplete="off" placeholder="${esc(desktopText('desktop.chat_question_free_text_placeholder', 'Type a custom answer...'))}">
                <button class="vd-qc-btn vd-qc-btn-primary" type="submit">${iconMarkup('chat', 'S', 'vd-qc-btn-icon', 14)}<span>${esc(desktopText('desktop.send', 'Send'))}</span></button>
            </form>` : '';

        overlay.innerHTML = `<div class="vd-qc-confirm vd-chat-question-panel">
            <div class="vd-qc-confirm-title">${esc(desktopText('desktop.chat_question_waiting', 'The agent is waiting for your answer...'))}</div>
            <div class="vd-qc-confirm-msg">${esc(payload.question)}</div>
            ${options.length ? `<div class="vd-chat-question-select">${esc(desktopText('desktop.chat_question_select', 'Select an option'))}</div><div class="vd-chat-question-options">${optionButtons}</div>` : ''}
            ${freeText}
            <div class="vd-chat-question-timer" aria-hidden="true"><span></span></div>
        </div>`;
        container.appendChild(overlay);

        const close = () => { if (overlay.parentNode) overlay.remove(); };
        const submit = async (selectedValue, freeTextValue) => {
            try {
                await fetch('/api/agent/question-response', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        session_id: 'virtual-desktop',
                        selected_value: selectedValue || '',
                        free_text: freeTextValue || ''
                    })
                });
            } catch (err) {
                appendDesktopChatError(host, err);
            }
            close();
        };

        overlay.querySelectorAll('[data-value]').forEach(btn => {
            btn.addEventListener('click', () => submit(btn.getAttribute('data-value') || '', ''));
        });
        const form = overlay.querySelector('[data-question-free-text]');
        if (form) {
            const input = form.querySelector('input');
            form.addEventListener('submit', event => {
                event.preventDefault();
                const value = input ? input.value.trim() : '';
                if (value) submit('', value);
            });
            setTimeout(() => { if (input) input.focus(); }, 50);
        }
        const timerFill = overlay.querySelector('.vd-chat-question-timer span');
        const timer = window.setInterval(() => {
            if (!overlay.parentNode) {
                window.clearInterval(timer);
                return;
            }
            const elapsed = (Date.now() - startedAt) / 1000;
            const remaining = Math.max(0, 1 - elapsed / timeoutSeconds);
            if (timerFill) timerFill.style.transform = 'scaleX(' + remaining + ')';
            if (remaining <= 0) {
                window.clearInterval(timer);
                const title = overlay.querySelector('.vd-qc-confirm-title');
                if (title) title.textContent = desktopText('desktop.chat_question_timeout', 'The question timed out.');
                setTimeout(close, 900);
            }
        }, 250);
    }

    function setDesktopChatBusy(host, busy) {
        if (!host) return;
        const input = host.querySelector('.vd-chat-input');
        const voiceBtn = host.querySelector('.vd-chat-voice');
        const sendBtn = host.querySelector('.vd-chat-send');
        const label = host.querySelector('[data-chat-send-label]');
        const clearBtn = host.querySelector('[data-chat-clear-history]');
        const stop = desktopText('desktop.chat_stop', 'Stop');
        const send = desktopText('desktop.send', 'Send');
        if (input) input.disabled = !!busy;
        if (voiceBtn) {
            const disabled = !!busy || voiceBtn.dataset.voiceAvailable === 'false';
            voiceBtn.disabled = disabled; voiceBtn.classList.toggle('is-disabled', disabled);
        }
        if (sendBtn) { sendBtn.classList.toggle('is-stop', !!busy); sendBtn.title = busy ? stop : send; }
        if (label) label.textContent = busy ? stop : send;
        if (clearBtn) clearBtn.disabled = !!busy;
    }

    function requestDesktopChatAbort(host) { if (host && typeof host._desktopChatAbort === 'function') host._desktopChatAbort(); }

    function isDesktopChatAbortError(err) {
        const name = err && err.name ? String(err.name) : '', message = err && err.message ? String(err.message) : '';
        return name === 'AbortError' || /aborted|abort/i.test(message);
    }

    function appendDesktopChatError(host, err) {
        const message = err && err.message ? err.message : String(err || 'Request failed');
        const chatLog = host && host.querySelector('.vd-chat-log');
        const renderer = window.DesktopChatRenderer;
        if (renderer && chatLog) renderer.appendRichBubble(chatLog, 'agent', message, lastRole);
        else if (host) appendChat(host, 'agent', message);
        lastRole = 'agent';
    }

    function initDesktopChatVoice(host, input, voiceBtn) {
        if (!input || !voiceBtn) return;
        const isSecure = window.location.protocol === 'https:' || window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1';
        const useBrowserSTT = !!(window.SpeechToText && window.SpeechToText.isSupported);
        const useRecorderFallback = !!(window.VoiceRecorder && navigator.mediaDevices && window.MediaRecorder);
        const unavailable = !isSecure || (!useBrowserSTT && !useRecorderFallback);
        const unavailableText = desktopText('desktop.chat_voice_unavailable', 'Speech input requires HTTPS and browser microphone support.');

        voiceBtn.title = desktopText('desktop.chat_voice_input', 'Voice input');
        voiceBtn.setAttribute('aria-label', desktopText('desktop.chat_voice_input', 'Voice input'));
        voiceBtn.dataset.voiceAvailable = unavailable ? 'false' : 'true';
        if (unavailable) {
            voiceBtn.disabled = true;
            voiceBtn.classList.add('is-disabled');
            voiceBtn.title = unavailableText;
            return;
        }

        const populateInput = (text) => {
            const value = String(text || '').trim();
            if (!value) return;
            input.value = value;
            input.dispatchEvent(new Event('input', { bubbles: true }));
            input.focus();
        };
        const showVoiceError = (message) => {
            voiceBtn.classList.remove('is-active');
            if (typeof showDesktopNotification === 'function') showDesktopNotification({ message: message || unavailableText });
        };

        if (useBrowserSTT) {
            const sttOptions = { onInterimResult: () => {}, onFinalResult: () => {}, onEnd: (text) => { voiceBtn.classList.remove('is-active'); populateInput(text); }, onError: showVoiceError };
            if (!window.SpeechToText._overlay) window.SpeechToText.init(sttOptions);
            else Object.assign(window.SpeechToText, sttOptions);
        } else if (useRecorderFallback) {
            const recorderOptions = { onTranscription: (text) => { voiceBtn.classList.remove('is-active'); populateInput(text); }, onError: showVoiceError };
            if (!window.VoiceRecorder.overlay) window.VoiceRecorder.init(recorderOptions);
            else Object.assign(window.VoiceRecorder, recorderOptions);
        }

        voiceBtn.addEventListener('click', () => {
            if (useBrowserSTT) {
                if (window.SpeechToText.isActive) {
                    window.SpeechToText.stop(); voiceBtn.classList.remove('is-active');
                } else {
                    window.SpeechToText.start(); voiceBtn.classList.add('is-active');
                }
            } else if (useRecorderFallback) {
                if (window.VoiceRecorder.isRecording) {
                    window.VoiceRecorder.send(); voiceBtn.classList.remove('is-active');
                } else {
                    window.VoiceRecorder.start(); voiceBtn.classList.add('is-active');
                }
            }
        });
    }

    function normalizeChatLaunchFiles(context) {
        const files = [];
        const raw = context && (context.chat_files || context.files || context.file);
        const list = Array.isArray(raw) ? raw : (raw ? [raw] : []);
        list.forEach(item => {
            const entry = typeof item === 'string' ? { path: item } : item;
            const file = chatFileContextFromEntry(entry || {});
            if (file && !files.some(existing => existing.path === file.path)) files.push(file);
        });
        return files;
    }

    function limitChatContextString(value, maxLength) {
        const text = String(value || '').replace(/[\r\n]+/g, ' ').trim();
        if (!maxLength || text.length <= maxLength) return text;
        return text.slice(0, maxLength).trim();
    }

    function normalizeChatLaunchWindowContext(context) {
        const raw = context && (context.window_context || context.windowContext);
        if (!raw || typeof raw !== 'object') return null;
        const normalized = {
            source: 'desktop-window',
            app_id: limitChatContextString(raw.app_id, 128),
            store_app_id: limitChatContextString(raw.store_app_id, 96),
            window_id: limitChatContextString(raw.window_id, 160),
            label: limitChatContextString(raw.label || raw.app_name || raw.app_id, 160),
            purpose: limitChatContextString(raw.purpose, 500),
            guide: limitChatContextString(raw.guide, 2000),
            resources: []
        };
        const resources = Array.isArray(raw.resources) ? raw.resources.slice(0, 8) : [];
        resources.forEach(resource => {
            if (!resource || typeof resource !== 'object') return;
            const item = {
                kind: limitChatContextString(resource.kind, 80),
                label: limitChatContextString(resource.label, 160),
                path: limitChatContextString(resource.path, 512),
                container_path: limitChatContextString(resource.container_path, 512)
            };
            if (item.kind || item.label || item.path || item.container_path) normalized.resources.push(item);
        });
        if (!normalized.label && !normalized.app_id && !normalized.purpose) return null;
        return normalized;
    }

    function chatAttachedFiles(host) {
        try {
            const files = JSON.parse((host && host.dataset.chatFiles) || '[]');
            return Array.isArray(files) ? files.filter(file => file && file.path) : [];
        } catch (_) {
            return [];
        }
    }

    function chatAttachedWindowContext(host) {
        try {
            const context = JSON.parse((host && host.dataset.chatWindowContext) || 'null');
            if (!context || typeof context !== 'object') return null;
            return context.label || context.app_id || context.purpose ? context : null;
        } catch (_) {
            return null;
        }
    }

    function renderChatContextBar(host) {
        const bar = host && host.querySelector('[data-chat-context]');
        if (!bar) return;
        const files = chatAttachedFiles(host);
        const windowContext = chatAttachedWindowContext(host);
        if (!files.length && !windowContext) {
            bar.hidden = true;
            bar.innerHTML = '';
            return;
        }

        const chips = [];
        if (windowContext) {
            const contextLabel = windowContext.label || windowContext.app_id || windowContext.purpose;
            chips.push(`<span class="vd-chat-context-chip context-window" title="${esc(desktopText('desktop.chat_request_context', 'Request context'))}: ${esc(contextLabel)}">${esc(contextLabel)}</span>`);
        }
        files.forEach(file => {
            const name = file.name || file.path;
            chips.push(`<span class="vd-chat-context-chip">${iconMarkup('file', '📄', 'vd-chat-context-icon', 12)}${esc(name)}
                <button class="vd-chat-context-chip-remove" type="button" data-remove-file="${esc(file.path)}" title="${esc(desktopText('desktop.remove', 'Remove'))}">${iconMarkup('x', 'X', 'vd-chat-context-icon', 10)}</button>
            </span>`);
        });

        bar.hidden = false;
        bar.innerHTML = `<div class="vd-chat-context-chips">${chips.join('')}</div>
            <button class="vd-chat-context-clear-all" type="button" data-chat-context-clear title="${esc(desktopText('desktop.clear', 'Clear'))}" aria-label="${esc(desktopText('desktop.clear', 'Clear all context'))}">${iconMarkup('x', 'X', 'vd-chat-context-icon', 14)}</button>`;

        bar.querySelectorAll('[data-remove-file]').forEach(btn => {
            btn.addEventListener('click', () => {
                const path = btn.getAttribute('data-remove-file');
                const current = chatAttachedFiles(host).filter(f => f.path !== path);
                host.dataset.chatFiles = JSON.stringify(current);
                renderChatContextBar(host);
            });
        });

        const clear = bar.querySelector('[data-chat-context-clear]');
        if (clear) clear.addEventListener('click', () => {
            host.dataset.chatFiles = '[]';
            delete host.dataset.chatSourceApp;
            delete host.dataset.chatWindowContext;
            renderChatContextBar(host);
        });
    }

    function applyChatLaunchContext(id, context) {
        const host = agentChatContentEl(id);
        if (!host) return;
        const existing = chatAttachedFiles(host);
        const incoming = normalizeChatLaunchFiles(context || {});
        const merged = existing.slice();
        incoming.forEach(file => {
            if (!merged.some(existingFile => existingFile.path === file.path)) merged.push(file);
        });
        host.dataset.chatFiles = JSON.stringify(merged);
        const windowContext = normalizeChatLaunchWindowContext(context || {});
        if (windowContext) host.dataset.chatWindowContext = JSON.stringify(windowContext);
        const sourceApp = String((context && (context.chat_source_app || context.source_app || context.origin_app)) || '').trim();
        if (sourceApp) host.dataset.chatSourceApp = sourceApp;
        else if (incoming.length) delete host.dataset.chatSourceApp;
        renderChatContextBar(host);
        const input = host.querySelector('.vd-chat-input');
        if (input && context && context.chat_prefill && !input.value.trim()) {
            input.value = context.chat_prefill;
            input.dispatchEvent(new Event('input', { bubbles: true }));
        }
        if (context.chat_autosend && state.chatBusy) {
            if (input) input.focus();
            return;
        }
        if (context.chat_autosend && input.value.trim() && !state.chatBusy) {
            window.setTimeout(() => {
                submitDesktopChatMessage(host, input.value.trim()).catch(err => appendDesktopChatError(host, err));
            }, 0);
        }
        if (input) input.focus();
    }

    function chatContextPayload(host) {
        const files = chatAttachedFiles(host);
        const windowContext = chatAttachedWindowContext(host);
        if (!files.length && !windowContext) return {};
        const payload = {};
        const sourceApp = String((host && host.dataset.chatSourceApp) || '').trim();
        if (files.length) {
            payload.source = 'desktop-file';
            payload.origin_app = sourceApp;
            payload.current_file = files[0].path;
            payload.open_files = files.map(file => file.path);
        }
        if (windowContext) {
            if (!payload.source) payload.source = 'desktop-window';
            payload.window_context = windowContext;
        }
        return payload;
    }

    async function sendDesktopChatStream(host, message, context) {
        const chatLog = host.querySelector('.vd-chat-log');
        const renderer = window.DesktopChatRenderer;
        if (renderer) renderer.resetDedupSets();
        const statusEl = renderer ? renderer.createThinkingStatus() : null;
        if (statusEl) chatLog.appendChild(statusEl);
        let streamingBubble = null;
        let streamingContent = '';
        let streamTextFrame = 0;
        let finalized = false;
        let chatScrollFrame = 0;
        let pendingScrollTarget = null;
        let pendingScrollSmooth = true;

        return new Promise((resolve, reject) => {
            const ctrl = new AbortController();
            const abortChatStream = () => ctrl.abort();
            host._desktopChatAbort = abortChatStream;

            function clearAbortHandle() {
                if (host._desktopChatAbort === abortChatStream) host._desktopChatAbort = null;
            }

            function cancelChatScroll() {
                if (chatScrollFrame) {
                    const cancelScroll = window.cancelAnimationFrame || window.clearTimeout;
                    cancelScroll(chatScrollFrame);
                    chatScrollFrame = 0;
                }
                pendingScrollTarget = null;
            }

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

            function doFinalize() {
                if (finalized) return;
                finalized = true;
                clearAbortHandle();
                flushStreamingBubble();
                if (statusEl && statusEl.parentNode) statusEl.remove();
                if (streamingBubble) {
                    streamingBubble.classList.remove('vd-streaming');
                    if (renderer && streamingContent.trim()) {
                        const html = renderer.renderMarkdown(streamingContent);
                        streamingBubble.innerHTML = html;
                        renderer.processImages(streamingBubble);
                        renderer.enhanceCodeBlocks(streamingBubble);
                        if (window.MermaidLoader) {
                            window.MermaidLoader.processBlocks(streamingBubble);
                        }
                    }
                    scheduleChatScroll(streamingBubble, false);
                } else {
                    cancelChatScroll();
                }
                resolve();
            }

            function doReject(err) {
                if (finalized) return;
                finalized = true;
                clearAbortHandle();
                if (streamTextFrame) {
                    const cancel = window.cancelAnimationFrame || window.clearTimeout;
                    cancel(streamTextFrame);
                    streamTextFrame = 0;
                }
                if (statusEl && statusEl.parentNode) statusEl.remove();
                cancelChatScroll();
                reject(err);
            }

            function flushStreamingBubble() {
                streamTextFrame = 0;
                if (!streamingBubble || !streamingBubble.classList.contains('vd-streaming')) return;
                streamingBubble.textContent = streamingContent;
                keepAgentStatusAtEnd();
                scheduleChatScroll(streamingBubble, false);
            }

            function queueStreamingBubbleFlush() {
                if (streamTextFrame) return;
                const schedule = window.requestAnimationFrame || ((callback) => window.setTimeout(callback, 16));
                streamTextFrame = schedule(flushStreamingBubble);
            }

            function keepAgentStatusAtEnd() {
                if (!statusEl || statusEl.parentNode !== chatLog) return;
                if (chatLog.lastElementChild !== statusEl) {
                    chatLog.appendChild(statusEl);
                }
            }

            fetch('/api/desktop/chat/stream', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ message, context }),
                signal: ctrl.signal
            }).then(response => {
                if (!response.ok) {
                    return response.text().then(text => {
                        throw new Error(text || ('HTTP ' + response.status));
                    });
                }
                function handleStreamEvent(data) {
                    if (!data) return;
                    const event = data.event || data.type;
                    if (data.event === 'llm_stream_delta' || data.type === 'llm_stream_delta') {
                        const content = data.content || '';
                        if (!content) return;
                        if (!streamingBubble) {
                            const isGroup = lastRole === 'agent';
                            streamingBubble = document.createElement('div');
                            streamingBubble.className = 'vd-chat-bubble agent vd-streaming';
                            if (isGroup) streamingBubble.dataset.roleGroup = 'continuation';

                            if (renderer && renderer.appendAvatar) {
                                renderer.appendAvatar(chatLog, 'agent', streamingBubble, isGroup);
                            } else {
                                chatLog.appendChild(streamingBubble);
                            }
                            if (renderer) renderer.appendTimestamp(chatLog, 'agent');
                            lastRole = 'agent';
                            keepAgentStatusAtEnd();
                        }
                        streamingContent += content;
                        if (streamingBubble.classList.contains('vd-streaming')) {
                            queueStreamingBubbleFlush();
                        }
                    } else if (event === 'thinking_block') {
                        const state2 = data.state || '';
                        if (statusEl && state2 === 'start' && renderer) {
                            renderer.updateStatus(statusEl, desktopText('desktop.chat_thinking', 'Reasoning...'));
                            keepAgentStatusAtEnd();
                        }
                    } else if (event === 'thinking' || event === 'tool_start' || event === 'tool_end' ||
                        event === 'co_agent_spawn' || event === 'workflow_plan' || event === 'coding' ||
                        event === 'error_recovery' || event === 'agent_action') {
                        if (statusEl && renderer) {
                            const status = renderer.formatAgentActionStatus(data);
                            if (status) {
                                renderer.updateStatus(statusEl, status);
                                keepAgentStatusAtEnd();
                            }
                        }
                    } else if (event === 'tool_call') {
                        if (renderer) {
                            const text = renderer.extractToolCallNarration(data.detail || data.message || '');
                            if (text) {
                                renderer.appendRichBubble(chatLog, 'agent', text, lastRole);
                                lastRole = 'agent';
                                keepAgentStatusAtEnd();
                            }
                        }
                    } else if (event === 'image') {
                        try {
                            const imgData = typeof data.detail === 'string' ? JSON.parse(data.detail) : data.detail;
                            if (renderer) {
                                renderer.appendImageMessage(chatLog, imgData);
                                lastRole = 'agent';
                                keepAgentStatusAtEnd();
                            }
                        } catch (_) {}
                    } else if (event === 'video') {
                        try {
                            const videoData = typeof data.detail === 'string' ? JSON.parse(data.detail) : data.detail;
                            if (renderer) {
                                renderer.appendVideoMessage(chatLog, videoData);
                                lastRole = 'agent';
                                keepAgentStatusAtEnd();
                            }
                        } catch (_) {}
                    } else if (event === 'live_stream') {
                        try {
                            const streamData = typeof data.detail === 'string' ? JSON.parse(data.detail) : data.detail;
                            if (renderer) {
                                renderer.appendLiveStreamMessage(chatLog, streamData);
                                lastRole = 'agent';
                                keepAgentStatusAtEnd();
                            }
                        } catch (_) {}
                    } else if (event === 'audio') {
                        try {
                            const audioData = typeof data.detail === 'string' ? JSON.parse(data.detail) : data.detail;
                            if (renderer) renderer.appendAudioMessage(chatLog, audioData);
                        } catch (_) {}
                    } else if (event === 'document') {
                        try {
                            const docData = typeof data.detail === 'string' ? JSON.parse(data.detail) : data.detail;
                            if (renderer) {
                                renderer.appendDocumentMessage(chatLog, docData);
                                lastRole = 'agent';
                                keepAgentStatusAtEnd();
                            }
                        } catch (_) {}
                    } else if (event === 'question_user') {
                        showDesktopQuestionModal(host, normalizeDesktopQuestionPayload(data));
                    } else if (event === 'final_response') {
                        if (data.detail || data.message) {
                            const text = data.detail || data.message || '';
                            if (!streamingBubble && text.trim()) {
                                if (renderer) {
                                    renderer.appendRichBubble(chatLog, 'agent', text, lastRole);
                                    lastRole = 'agent';
                                    keepAgentStatusAtEnd();
                                } else {
                                    appendChat(host, 'agent', text);
                                    lastRole = 'agent';
                                }
                            } else if (streamingBubble && !streamingContent.trim() && text.trim()) {
                                streamingContent = text;
                                flushStreamingBubble();
                            }
                        }
                    } else if (event === 'done') {
                        doFinalize();
                        return;
                    } else if (event === 'token_update') {
                        return;
                    }
                }

                return window.AuraChatStreamParser.readFetchEventStream(response, {
                    onEvent: eventData => handleStreamEvent(eventData),
                    onDone: () => doFinalize(),
                    onError: err => doReject(err)
                });
            }).catch(err => {
                doReject(err);
            });
        });
    }

    function appendChat(host, role, text) {
        const chatLog = host.querySelector('.vd-chat-log');
        const renderer = window.DesktopChatRenderer;
        if (renderer) {
            renderer.appendRichBubble(chatLog, role, text, lastRole);
        } else {
            const bubble = document.createElement('div');
            bubble.className = 'vd-chat-bubble ' + role;
            bubble.textContent = text;
            chatLog.appendChild(bubble);
        }
        appendChatTimestamp(host, role);
        lastRole = role;
        const last = chatLog.lastElementChild;
        if (last) last.scrollIntoView({ block: 'end' });
    }

    function appendChatTimestamp(host, role) {
        const chatLog = host && host.querySelector('.vd-chat-log');
        const renderer = window.DesktopChatRenderer;
        if (renderer && chatLog) return renderer.appendTimestamp(chatLog, role);
        return null;
    }

    window.AgentChatApp = window.AgentChatApp || {};
    window.AgentChatApp.render = renderChat;
    window.renderChat = renderChat;
