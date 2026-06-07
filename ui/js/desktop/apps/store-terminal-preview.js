(function () {
    'use strict';

    function cleanupExisting(host) {
        if (host && typeof host.__storeTerminalPreviewCleanup === 'function') {
            host.__storeTerminalPreviewCleanup();
        }
    }

    async function ensureAssets(t) {
        await Promise.all([loadStyle('/css/xterm.css')]);
        await loadScript('/js/vendor/xterm.min.js');
        await loadScript('/js/vendor/xterm-addon-fit.min.js');
        if (!window.Terminal) {
            throw new Error(t('common.error', 'Error'));
        }
    }

    function loadStyle(href) {
        if (window.AuraLazyAssets && typeof window.AuraLazyAssets.loadStyle === 'function') {
            return window.AuraLazyAssets.loadStyle(href);
        }
        const existing = document.querySelector('link[data-store-terminal-href="' + href + '"],link[href="' + href + '"]');
        if (existing) return Promise.resolve(existing);
        return new Promise((resolve, reject) => {
            const link = document.createElement('link');
            link.rel = 'stylesheet';
            link.dataset.storeTerminalHref = href;
            link.href = href;
            link.onload = () => resolve(link);
            link.onerror = () => reject(new Error('Failed to load stylesheet: ' + href));
            document.head.appendChild(link);
        });
    }

    function loadScript(src) {
        if (window.AuraLazyAssets && typeof window.AuraLazyAssets.loadScript === 'function') {
            return window.AuraLazyAssets.loadScript(src);
        }
        const existing = document.querySelector('script[data-store-terminal-src="' + src + '"],script[src="' + src + '"]');
        if (existing && existing.dataset.storeTerminalLoaded === '1') return Promise.resolve(existing);
        return new Promise((resolve, reject) => {
            const script = existing || document.createElement('script');
            script.dataset.storeTerminalSrc = src;
            script.onload = () => {
                script.dataset.storeTerminalLoaded = '1';
                resolve(script);
            };
            script.onerror = () => reject(new Error('Failed to load script: ' + src));
            if (!existing) {
                script.src = src;
                document.head.appendChild(script);
            }
        });
    }

    function previewStatusURL(previewURL) {
        try {
            return new URL('/__commandcode_preview_status', previewURL).toString();
        } catch (_) {
            return String(previewURL || '').replace(/\/?$/, '/__commandcode_preview_status');
        }
    }

    async function render(id, app, storeAppId, deps) {
        const contentEl = deps.contentEl;
        const esc = deps.esc;
        const t = deps.t;
        const api = deps.api;
        const iconMarkup = deps.iconMarkup;
        const appName = deps.appName;
        const registerWindowCleanup = deps.registerWindowCleanup;
        const makeSandboxedFrame = deps.makeSandboxedFrame;
        const storeFrameURL = deps.storeFrameURL;
        const cacheBustURL = deps.cacheBustURL;
        const showDesktopNotification = deps.showDesktopNotification;

        const host = contentEl(id);
        if (!host) return;
        cleanupExisting(host);
        host.innerHTML = `<div class="vd-store-frame-loading">${esc(t('desktop.loading'))}</div>`;
        try {
            await ensureAssets(t);
            const metadata = (app && app.metadata) || {};
            const previewPortID = metadata.preview_port_id || 'web';
            const workspacePath = metadata.workspace_path || 'Shared/CommandCode';
            const body = await api('/api/desktop/store/apps/' + encodeURIComponent(storeAppId) + '/open-url?port_id=' + encodeURIComponent(previewPortID));
            if (!contentEl(id)) return;

            const newSessionLabel = t('desktop.store_terminal_new_session', 'New session');
            const restartLabel = t('desktop.store_terminal_restart_session', 'Restart session');
            const copyLabel = t('desktop.fm.copy', 'Copy');
            const pasteLabel = t('desktop.fm.paste', 'Paste');
            const showPreviewLabel = t('desktop.store_terminal_show_preview', 'Show preview');
            const hidePreviewLabel = t('desktop.store_terminal_hide_preview', 'Hide preview');
            const headerTitle = t('desktop.store_terminal_header_title', 'CommandCode');
            const headerSubtitle = t('desktop.store_terminal_header_subtitle', 'Console-first workspace');
            const statusStarting = t('desktop.store_terminal_status_starting', 'Starting');
            const statusConnected = t('desktop.store_terminal_status_connected', 'Connected');
            const statusPreviewWaiting = t('desktop.store_terminal_status_preview_waiting', 'Preview waiting');
            const statusPreviewReady = t('desktop.store_terminal_status_preview_ready', 'Preview ready');

            host.innerHTML = `<div class="vd-store-terminal-shell">
                <header class="vd-store-terminal-header">
                    <div class="vd-store-terminal-brand">
                        ${iconMarkup('terminal', 'C', 'vd-store-terminal-brand-icon', 18)}
                        <div>
                            <div class="vd-store-terminal-brand-title">${esc(headerTitle)}</div>
                            <div class="vd-store-terminal-brand-subtitle">${esc(headerSubtitle)}</div>
                        </div>
                    </div>
                    <div class="vd-store-terminal-status-row">
                        <span class="vd-store-terminal-chip" data-store-terminal-connection data-state="starting">${esc(statusStarting)}</span>
                        <span class="vd-store-terminal-chip" data-store-terminal-preview-status data-state="waiting">${esc(statusPreviewWaiting)}</span>
                        <span class="vd-store-terminal-workspace" title="${esc(workspacePath)}">${esc(workspacePath)}</span>
                    </div>
                </header>
                <div class="vd-store-terminal-preview">
                    <div class="vd-store-terminal-pane">
                        <div class="vd-store-terminal-toolbar">
                            <div class="vd-store-terminal-tabs" data-store-terminal-tabs></div>
                            <div class="vd-store-terminal-actions">
                                <button type="button" class="vd-store-terminal-action" data-store-terminal-new title="${esc(newSessionLabel)}" aria-label="${esc(newSessionLabel)}">${iconMarkup('plus', '+', 'vd-store-terminal-action-icon', 15)}</button>
                                <button type="button" class="vd-store-terminal-action" data-store-terminal-restart title="${esc(restartLabel)}" aria-label="${esc(restartLabel)}">${iconMarkup('refresh', 'R', 'vd-store-terminal-action-icon', 15)}</button>
                                <button type="button" class="vd-store-terminal-action" data-store-terminal-copy title="${esc(copyLabel)}" aria-label="${esc(copyLabel)}">${iconMarkup('copy', 'C', 'vd-store-terminal-action-icon', 15)}</button>
                                <button type="button" class="vd-store-terminal-action" data-store-terminal-paste title="${esc(pasteLabel)}" aria-label="${esc(pasteLabel)}">${iconMarkup('clipboard', 'P', 'vd-store-terminal-action-icon', 15)}</button>
                                <button type="button" class="vd-store-terminal-action" data-store-preview-toggle title="${esc(hidePreviewLabel)}" aria-label="${esc(hidePreviewLabel)}" aria-pressed="true">${iconMarkup('eye-off', 'P', 'vd-store-terminal-action-icon', 15)}</button>
                            </div>
                        </div>
                        <div class="vd-store-terminal-surface" data-store-terminal></div>
                    </div>
                    <div class="vd-store-terminal-resizer" data-store-terminal-resizer></div>
                    <div class="vd-store-preview-pane"></div>
                </div>
            </div>`;

            const connectionChip = host.querySelector('[data-store-terminal-connection]');
            const previewStatusChip = host.querySelector('[data-store-terminal-preview-status]');
            const terminalStack = host.querySelector('[data-store-terminal]');
            const terminalTabs = host.querySelector('[data-store-terminal-tabs]');
            const newButton = host.querySelector('[data-store-terminal-new]');
            const restartButton = host.querySelector('[data-store-terminal-restart]');
            const copyButton = host.querySelector('[data-store-terminal-copy]');
            const pasteButton = host.querySelector('[data-store-terminal-paste]');
            const previewToggleButton = host.querySelector('[data-store-preview-toggle]');
            const resizer = host.querySelector('[data-store-terminal-resizer]');
            const terminalPreview = host.querySelector('.vd-store-terminal-preview');
            const previewHost = host.querySelector('.vd-store-preview-pane');
            const terminalSessions = new Map();
            let activeTerminalSessionID = '';
            let terminalSessionSequence = 0;
            let previewVisible = true;
            let previewPollTimer = null;
            let previewReady = false;
            let terminalPasteHandler = null;
            let resizeMoveHandler = null;
            let resizeUpHandler = null;
            let disposed = false;

            function setConnectionState(state) {
                if (!connectionChip) return;
                connectionChip.dataset.state = state;
                connectionChip.textContent = state === 'connected' ? statusConnected : statusStarting;
            }

            function setPreviewState(state, detail) {
                if (!previewStatusChip) return;
                previewStatusChip.dataset.state = state;
                previewStatusChip.textContent = state === 'ready'
                    ? (detail || statusPreviewReady)
                    : statusPreviewWaiting;
            }

            function cleanupApp() {
                if (disposed) return;
                disposed = true;
                if (previewPollTimer) {
                    clearTimeout(previewPollTimer);
                    previewPollTimer = null;
                }
                if (terminalPasteHandler) {
                    host.removeEventListener('paste', terminalPasteHandler, true);
                    terminalPasteHandler = null;
                }
                if (resizeMoveHandler) {
                    window.removeEventListener('pointermove', resizeMoveHandler);
                    resizeMoveHandler = null;
                }
                if (resizeUpHandler) {
                    window.removeEventListener('pointerup', resizeUpHandler);
                    window.removeEventListener('pointercancel', resizeUpHandler);
                    resizeUpHandler = null;
                }
                terminalSessions.forEach(session => session.cleanup());
                terminalSessions.clear();
                if (host.__storeTerminalPreviewCleanup === cleanupApp) {
                    host.__storeTerminalPreviewCleanup = null;
                }
            }
            host.__storeTerminalPreviewCleanup = cleanupApp;
            registerWindowCleanup(id, cleanupApp);

            const scheme = window.location.protocol === 'https:' ? 'wss' : 'ws';
            const terminalInputEncoder = new TextEncoder();
            const activeTerminalSession = () => terminalSessions.get(activeTerminalSessionID) || null;

            function refocusActiveTerminalAfterPreviewLoad() {
                if (disposed) return;
                const session = activeTerminalSession();
                if (!session || !session.terminal) return;
                window.requestAnimationFrame(() => {
                    if (disposed || activeTerminalSession() !== session || !session.terminal) return;
                    session.terminal.focus();
                });
            }

            function copyCommandToClipboard(command, event) {
                if (event) event.preventDefault();
                if (!navigator.clipboard || typeof navigator.clipboard.writeText !== 'function') return;
                navigator.clipboard.writeText(command).catch(() => {});
            }

            function renderPreviewPlaceholder() {
                const placeholder = document.createElement('div');
                placeholder.className = 'vd-store-preview-placeholder';
                placeholder.innerHTML = `<div class="vd-store-preview-placeholder-body">
                    <div class="vd-store-preview-placeholder-title">${esc(t('desktop.store_terminal_preview_placeholder_title', 'Preview is idle'))}</div>
                    <p>${esc(t('desktop.store_terminal_preview_placeholder_copy', 'Start your development server in the terminal. Open the preview when your page is ready.'))}</p>
                    <div class="vd-store-terminal-onboarding">
                        <div class="vd-store-terminal-onboarding-step"><span>1</span><p>${esc(t('desktop.store_terminal_onboarding_cmd', 'CommandCode starts automatically in the terminal.'))}</p></div>
                        <div class="vd-store-terminal-onboarding-step"><span>2</span><p>${esc(t('desktop.store_terminal_onboarding_api_key', 'Paste your API key from browser auth using the paste button.'))}</p></div>
                        <div class="vd-store-terminal-onboarding-step"><span>3</span><p>${esc(t('desktop.store_terminal_onboarding_dev_server', 'Start your dev server and point the preview to it.'))}</p></div>
                    </div>
                    <div class="vd-store-preview-placeholder-code">
                        <button type="button" class="vd-store-preview-command" data-copy-command="npm run dev -- --host 0.0.0.0"><code>npm run dev -- --host 0.0.0.0</code><span>${esc(t('desktop.store_terminal_copy_command', 'Copy'))}</span></button>
                        <button type="button" class="vd-store-preview-command" data-copy-command="preview-port 5173"><code>preview-port 5173</code><span>${esc(t('desktop.store_terminal_copy_command', 'Copy'))}</span></button>
                    </div>
                    <button type="button" class="vd-store-preview-open" data-store-preview-open>${iconMarkup('monitor', 'P', 'vd-store-terminal-action-icon', 15)}<span>${esc(t('desktop.store_terminal_open_preview', 'Open preview'))}</span></button>
                </div>`;
                placeholder.querySelectorAll('[data-copy-command]').forEach(button => {
                    button.addEventListener('click', event => copyCommandToClipboard(button.dataset.copyCommand, event));
                });
                const openButton = placeholder.querySelector('[data-store-preview-open]');
                if (openButton) openButton.addEventListener('click', openPreviewFrame);
                return placeholder;
            }

            function updatePreviewToggleButton() {
                if (!previewToggleButton) return;
                const label = previewVisible ? hidePreviewLabel : showPreviewLabel;
                previewToggleButton.setAttribute('title', label);
                previewToggleButton.setAttribute('aria-label', label);
                previewToggleButton.setAttribute('aria-pressed', previewVisible ? 'true' : 'false');
                previewToggleButton.innerHTML = iconMarkup(previewVisible ? 'eye-off' : 'eye', 'P', 'vd-store-terminal-action-icon', 15);
            }

            function setPreviewVisible(visible) {
                previewVisible = visible !== false;
                if (terminalPreview) terminalPreview.classList.toggle('is-preview-hidden', !previewVisible);
                if (!previewVisible) {
                    previewHost.replaceChildren(renderPreviewPlaceholder());
                } else if (!previewHost.firstElementChild) {
                    previewHost.replaceChildren(renderPreviewPlaceholder());
                }
                updatePreviewToggleButton();
                terminalSessions.forEach(session => scheduleTerminalSessionFit(session));
                refocusActiveTerminalAfterPreviewLoad();
            }

            function openPreviewFrame() {
                if (disposed) return;
                if (!previewVisible) setPreviewVisible(true);
                const frameURL = cacheBustURL(storeFrameURL(body.url, storeAppId), 'aurago_store_embed');
                const frame = makeSandboxedFrame(frameURL, app.id, '', id, 'vd-generated-frame vd-store-app-frame', appName(app), { allowSameOrigin: true, allowDownloads: true, allowStorageAccess: true, allowTopNavigationByUserActivation: true, allowPointerLock: true, allowFullscreen: true, allowGamepad: true, disableAutoFocus: true });
                frame.addEventListener('load', refocusActiveTerminalAfterPreviewLoad);
                previewHost.replaceChildren(frame);
                refocusActiveTerminalAfterPreviewLoad();
            }

            async function pollPreviewStatus() {
                if (disposed) return;
                try {
                    const response = await fetch(previewStatusURL(body.url), { credentials: 'same-origin', cache: 'no-store' });
                    if (!response.ok) throw new Error('preview status unavailable');
                    const status = await response.json();
                    if (status && status.ready) {
                        const target = typeof status.target === 'string' ? status.target : (status.target && status.target.href) || '';
                        const portMatch = target.match(/:(\d+)/);
                        const detail = portMatch ? statusPreviewReady + ' :' + portMatch[1] : statusPreviewReady;
                        if (!previewReady) {
                            previewReady = true;
                            setPreviewState('ready', detail);
                            showDesktopNotification({
                                title: appName(app),
                                message: t('desktop.store_terminal_preview_ready_toast', 'Development server detected. Opening preview.')
                            });
                            openPreviewFrame();
                        } else {
                            setPreviewState('ready', detail);
                        }
                    } else {
                        previewReady = false;
                        setPreviewState('waiting');
                    }
                } catch (_) {
                    previewReady = false;
                    setPreviewState('waiting');
                }
                previewPollTimer = window.setTimeout(pollPreviewStatus, 2000);
            }

            const writeTerminalInput = (session, text) => {
                if (!session || !text || !session.socket || session.socket.readyState !== WebSocket.OPEN) return false;
                session.socket.send(terminalInputEncoder.encode(text));
                return true;
            };

            terminalPasteHandler = event => {
                if (disposed) return;
                const text = event.clipboardData && event.clipboardData.getData('text/plain');
                if (!text) return;
                const session = activeTerminalSession();
                if (writeTerminalInput(session, text)) {
                    event.preventDefault();
                    if (session.terminal) session.terminal.focus();
                }
            };
            host.addEventListener('paste', terminalPasteHandler, true);

            if (copyButton) {
                copyButton.addEventListener('click', async () => {
                    const session = activeTerminalSession();
                    if (!session || !session.terminal || !navigator.clipboard || typeof navigator.clipboard.writeText !== 'function') {
                        if (session && session.terminal) session.terminal.focus();
                        return;
                    }
                    const selection = session.terminal.getSelection ? session.terminal.getSelection() : '';
                    if (selection) await navigator.clipboard.writeText(selection).catch(() => {});
                    session.terminal.focus();
                });
            }
            if (pasteButton) {
                pasteButton.addEventListener('click', async () => {
                    const session = activeTerminalSession();
                    if (!navigator.clipboard || typeof navigator.clipboard.readText !== 'function') {
                        if (session && session.terminal) session.terminal.focus();
                        return;
                    }
                    const text = await navigator.clipboard.readText().catch(() => '');
                    if (writeTerminalInput(session, text) && session.terminal) session.terminal.focus();
                });
            }
            terminalStack.addEventListener('keydown', event => {
                if (!(event.ctrlKey || event.metaKey) || String(event.key || '').toLowerCase() !== 'v') return;
                const session = activeTerminalSession();
                if (!session || !session.socket || session.socket.readyState !== WebSocket.OPEN) return;
                if (navigator.clipboard && typeof navigator.clipboard.readText === 'function') {
                    event.preventDefault();
                    navigator.clipboard.readText()
                        .then(text => {
                            if (writeTerminalInput(session, text) && session.terminal) session.terminal.focus();
                        })
                        .catch(() => {
                            if (session.terminal) session.terminal.focus();
                        });
                }
            }, true);

            function fitTerminalSession(session) {
                if (disposed || !session || session.disposed || !session.terminal) return;
                if (session.fitAddon) {
                    try { session.fitAddon.fit(); } catch (_) { return; }
                }
                if (session.socket && session.socket.readyState === WebSocket.OPEN) {
                    session.socket.send(JSON.stringify({ type: 'resize', cols: session.terminal.cols, rows: session.terminal.rows }));
                }
            }
            function scheduleTerminalSessionFit(session) {
                if (!session || session.fitScheduled) return;
                session.fitScheduled = true;
                window.requestAnimationFrame(() => {
                    session.fitScheduled = false;
                    fitTerminalSession(session);
                });
            }
            function activateTerminalSession(sessionID) {
                const session = terminalSessions.get(sessionID);
                if (!session) return;
                activeTerminalSessionID = sessionID;
                terminalSessions.forEach(current => {
                    const active = current.id === sessionID;
                    current.tab.classList.toggle('is-active', active);
                    current.tab.setAttribute('aria-selected', active ? 'true' : 'false');
                    current.surface.hidden = !active;
                });
                scheduleTerminalSessionFit(session);
                session.terminal.focus();
            }
            function closeTerminalSession(sessionID) {
                const session = terminalSessions.get(sessionID);
                if (!session) return;
                const wasActive = activeTerminalSessionID === sessionID;
                const remainingIDs = Array.from(terminalSessions.keys()).filter(currentID => currentID !== sessionID);
                session.cleanup();
                terminalSessions.delete(sessionID);
                if (wasActive) {
                    if (remainingIDs.length > 0) {
                        activateTerminalSession(remainingIDs[Math.max(0, remainingIDs.length - 1)]);
                    } else if (!disposed) {
                        createTerminalSession();
                    }
                }
            }
            function createTerminalSession() {
                terminalSessionSequence += 1;
                const sessionID = 'terminal-session-' + terminalSessionSequence;
                const sessionLabel = t('desktop.store_terminal_session_label', 'Session') + ' ' + terminalSessionSequence;
                const surface = document.createElement('div');
                surface.className = 'vd-store-terminal-session';
                surface.hidden = true;
                surface.dataset.storeTerminalSession = sessionID;
                terminalStack.appendChild(surface);
                const tab = document.createElement('button');
                tab.type = 'button';
                tab.className = 'vd-store-terminal-tab';
                tab.dataset.storeTerminalTab = sessionID;
                tab.setAttribute('role', 'tab');
                tab.innerHTML = `<span class="vd-store-terminal-tab-label">${esc(sessionLabel)}</span><span class="vd-store-terminal-tab-close" data-store-terminal-close="${esc(sessionID)}" aria-hidden="true">${iconMarkup('x', 'X', 'vd-store-terminal-tab-close-icon', 12)}</span>`;
                terminalTabs.appendChild(tab);
                const terminal = new window.Terminal({
                    cursorBlink: true,
                    convertEol: true,
                    fontFamily: "'Fira Code', 'Cascadia Code', Consolas, monospace",
                    fontSize: 13,
                    scrollback: 3000,
                    theme: { background: '#05070a', foreground: '#d7e1ec', cursor: '#8bd3ff' }
                });
                let fitAddon = null;
                if (window.FitAddon && window.FitAddon.FitAddon) {
                    fitAddon = new window.FitAddon.FitAddon();
                    terminal.loadAddon(fitAddon);
                }
                const session = {
                    id: sessionID,
                    tab,
                    surface,
                    terminal,
                    fitAddon,
                    resizeObserver: null,
                    socket: null,
                    fitScheduled: false,
                    disposed: false,
                    cleanup() {
                        if (session.disposed) return;
                        session.disposed = true;
                        if (session.resizeObserver) {
                            session.resizeObserver.disconnect();
                            session.resizeObserver = null;
                        }
                        if (session.socket) {
                            session.socket.onopen = null;
                            session.socket.onmessage = null;
                            session.socket.onerror = null;
                            session.socket.onclose = null;
                            if (session.socket.readyState === WebSocket.OPEN || session.socket.readyState === WebSocket.CONNECTING) {
                                session.socket.close();
                            }
                            session.socket = null;
                        }
                        session.terminal.dispose();
                        session.tab.remove();
                        session.surface.remove();
                    }
                };
                terminalSessions.set(sessionID, session);
                tab.addEventListener('click', event => {
                    const closeTarget = event.target && event.target.closest && event.target.closest('[data-store-terminal-close]');
                    if (closeTarget) {
                        event.stopPropagation();
                        closeTerminalSession(sessionID);
                        return;
                    }
                    activateTerminalSession(sessionID);
                });
                session.terminal.open(session.surface);
                session.socket = new WebSocket(scheme + '://' + window.location.host + '/api/desktop/store/apps/' + encodeURIComponent(storeAppId) + '/terminal');
                session.socket.binaryType = 'arraybuffer';
                session.terminal.onData(data => writeTerminalInput(session, data));
                if (window.ResizeObserver) {
                    session.resizeObserver = new ResizeObserver(() => scheduleTerminalSessionFit(session));
                    session.resizeObserver.observe(session.surface);
                }
                session.socket.onopen = () => {
                    setConnectionState('connected');
                    scheduleTerminalSessionFit(session);
                };
                session.socket.onmessage = event => {
                    if (disposed || session.disposed || !session.terminal) return;
                    if (typeof event.data === 'string') {
                        session.terminal.write(event.data);
                        return;
                    }
                    session.terminal.write(new TextDecoder().decode(event.data));
                };
                session.socket.onerror = () => {
                    setConnectionState('starting');
                    if (session.terminal) session.terminal.write('\r\n[' + esc(t('common.error', 'Error')) + ']\r\n');
                };
                session.socket.onclose = () => setConnectionState('starting');
                activateTerminalSession(sessionID);
                return session;
            }
            function restartActiveTerminalSession() {
                const sessionID = activeTerminalSessionID;
                if (!sessionID) {
                    createTerminalSession();
                    return;
                }
                closeTerminalSession(sessionID);
            }
            function setTerminalPaneWidthPct(widthPct) {
                const clamped = Math.max(24, Math.min(72, widthPct));
                if (terminalPreview) terminalPreview.style.setProperty('--store-terminal-width', clamped.toFixed(1) + '%');
                terminalSessions.forEach(session => scheduleTerminalSessionFit(session));
            }
            function startTerminalPreviewResize(event) {
                if (!resizer || !terminalPreview || event.button !== 0) return;
                event.preventDefault();
                if (typeof resizer.setPointerCapture === 'function') {
                    try { resizer.setPointerCapture(event.pointerId); } catch (_) {}
                }
                if (resizeMoveHandler) window.removeEventListener('pointermove', resizeMoveHandler);
                if (resizeUpHandler) {
                    window.removeEventListener('pointerup', resizeUpHandler);
                    window.removeEventListener('pointercancel', resizeUpHandler);
                }
                resizeMoveHandler = moveEvent => {
                    const bounds = terminalPreview.getBoundingClientRect();
                    if (!bounds.width) return;
                    setTerminalPaneWidthPct(((moveEvent.clientX - bounds.left) / bounds.width) * 100);
                };
                resizeUpHandler = () => {
                    if (typeof resizer.releasePointerCapture === 'function') {
                        try { resizer.releasePointerCapture(event.pointerId); } catch (_) {}
                    }
                    if (resizeMoveHandler) {
                        window.removeEventListener('pointermove', resizeMoveHandler);
                        resizeMoveHandler = null;
                    }
                    if (resizeUpHandler) {
                        window.removeEventListener('pointerup', resizeUpHandler);
                        window.removeEventListener('pointercancel', resizeUpHandler);
                        resizeUpHandler = null;
                    }
                };
                window.addEventListener('pointermove', resizeMoveHandler);
                window.addEventListener('pointerup', resizeUpHandler);
                window.addEventListener('pointercancel', resizeUpHandler);
            }

            if (newButton) newButton.addEventListener('click', () => createTerminalSession());
            if (restartButton) restartButton.addEventListener('click', restartActiveTerminalSession);
            if (previewToggleButton) previewToggleButton.addEventListener('click', () => setPreviewVisible(!previewVisible));
            if (resizer) resizer.addEventListener('pointerdown', startTerminalPreviewResize);
            createTerminalSession();
            previewHost.replaceChildren(renderPreviewPlaceholder());
            refocusActiveTerminalAfterPreviewLoad();
            pollPreviewStatus();
        } catch (err) {
            cleanupExisting(host);
            if (!contentEl(id)) return;
            host.innerHTML = `<div class="vd-store-frame-error">
                <div class="vd-store-frame-error-title">${esc(appName(app))}</div>
                <div class="vd-store-frame-error-msg">${esc(err.message)}</div>
                <button type="button" class="vd-store-btn vd-store-primary" data-action="start">${iconMarkup('run', 'S', 'vd-store-btn-icon', 15)}<span>${esc(t('desktop.store.start', 'Start'))}</span></button>
            </div>`;
            const start = host.querySelector('[data-action="start"]');
            if (start) {
                start.addEventListener('click', async () => {
                    try {
                        await api('/api/desktop/store/apps/' + encodeURIComponent(storeAppId) + '/start', { method: 'POST' });
                        setTimeout(() => render(id, app, storeAppId, deps), 1200);
                    } catch (startErr) {
                        showDesktopNotification({ title: appName(app), message: startErr.message });
                    }
                });
            }
        }
    }

    window.StoreTerminalPreviewApp = window.StoreTerminalPreviewApp || {};
    window.StoreTerminalPreviewApp.render = render;
    window.StoreTerminalPreviewApp.cleanupExisting = cleanupExisting;
})();