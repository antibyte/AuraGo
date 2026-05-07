            wireDraggableWidget(card, widget);
        });
    }

    function renderBuiltinWidget(card, widget) {
        const container = card.querySelector('.vd-widget-builtin');
        if (!container) return;
        if (widget.id === 'builtin-analog-clock') {
            renderAnalogClockWidget(container);
        } else if (widget.id === 'builtin-quickchat') {
            renderQuickChatWidget(container);
        } else {
            container.innerHTML = `<div class="vd-widget-body">${esc(widget.title)}</div>`;
        }
    }

    function renderAnalogClockWidget(container) {
        const size = Math.min(container.parentElement.offsetWidth || 200, container.parentElement.offsetHeight || 200);
        const svgSize = Math.max(80, size - 20);
        container.innerHTML = `<svg class="vd-analog-clock-svg" viewBox="0 0 200 200" width="${svgSize}" height="${svgSize}">
            <circle cx="100" cy="100" r="95" fill="none" stroke="var(--vd-border)" stroke-width="2"/>
            <g class="vd-clock-ticks"></g>
            <line class="vd-clock-hour" x1="100" y1="100" x2="100" y2="50" stroke="var(--vd-text)" stroke-width="4" stroke-linecap="round"/>
            <line class="vd-clock-minute" x1="100" y1="100" x2="100" y2="30" stroke="var(--vd-text)" stroke-width="2.5" stroke-linecap="round"/>
            <line class="vd-clock-second" x1="100" y1="100" x2="100" y2="25" stroke="var(--vd-accent)" stroke-width="1.2" stroke-linecap="round"/>
            <circle cx="100" cy="100" r="4" fill="var(--vd-accent)"/>
        </svg>`;
        const ticksG = container.querySelector('.vd-clock-ticks');
        for (let i = 0; i < 12; i++) {
            const angle = (i * 30) * Math.PI / 180;
            const isMain = i % 3 === 0;
            const r1 = isMain ? 78 : 84;
            const r2 = 90;
            const line = document.createElementNS('http://www.w3.org/2000/svg', 'line');
            line.setAttribute('x1', 100 + r1 * Math.sin(angle));
            line.setAttribute('y1', 100 - r1 * Math.cos(angle));
            line.setAttribute('x2', 100 + r2 * Math.sin(angle));
            line.setAttribute('y2', 100 - r2 * Math.cos(angle));
            line.setAttribute('stroke', isMain ? 'var(--vd-text)' : 'var(--vd-muted)');
            line.setAttribute('stroke-width', isMain ? '2.5' : '1.2');
            line.setAttribute('stroke-linecap', 'round');
            ticksG.appendChild(line);
        }
        function updateClockHands() {
            const now = new Date();
            const h = now.getHours() % 12;
            const m = now.getMinutes();
            const s = now.getSeconds();
            const hourAngle = (h + m / 60) * 30;
            const minuteAngle = (m + s / 60) * 6;
            const secondAngle = s * 6;
            const hourHand = container.querySelector('.vd-clock-hour');
            const minuteHand = container.querySelector('.vd-clock-minute');
            const secondHand = container.querySelector('.vd-clock-second');
            if (hourHand) hourHand.setAttribute('transform', `rotate(${hourAngle}, 100, 100)`);
            if (minuteHand) minuteHand.setAttribute('transform', `rotate(${minuteAngle}, 100, 100)`);
            if (secondHand) secondHand.setAttribute('transform', `rotate(${secondAngle}, 100, 100)`);
        }
        updateClockHands();
        const timer = setInterval(updateClockHands, 1000);
        container._clockTimer = timer;
        registerWidgetCleanup(() => {
            clearInterval(timer);
            if (container._clockTimer === timer) container._clockTimer = 0;
        });
    }

    function renderQuickChatWidget(container) {
        container.innerHTML = `<div class="vd-quickchat vd-quickchat-collapsed">
            <div class="vd-quickchat-response"></div>
            <form class="vd-quickchat-form">
                <input class="vd-quickchat-input" autocomplete="off" placeholder="${esc(t('desktop.chat_placeholder'))}">
                <button class="vd-quickchat-send" type="submit">${iconMarkup('chat', 'S', 'vd-quickchat-send-icon', 14)}</button>
            </form>
        </div>`;
        const input = container.querySelector('.vd-quickchat-input');
        const responseEl = container.querySelector('.vd-quickchat-response');
        const wrapper = container.querySelector('.vd-quickchat');
        container.querySelector('form').addEventListener('submit', async (event) => {
            event.preventDefault();
            if (state.chatBusy) return;
            const message = input.value.trim();
            if (!message) return;
            input.value = '';
            state.chatBusy = true;
            responseEl.textContent = t('desktop.thinking');
            responseEl.classList.add('vd-quickchat-active');
            wrapper.classList.remove('vd-quickchat-collapsed');
            try {
                await sendQuickChatStream(responseEl, message);
            } catch (err) {
                responseEl.textContent = err.message || 'Error';
            } finally {
                state.chatBusy = false;
            }
        });
    }

    async function sendQuickChatStream(responseEl, message) {
        let streamingContent = '';
        let finalized = false;
        return new Promise((resolve, reject) => {
            const ctrl = new AbortController();
            const timeout = setTimeout(() => { ctrl.abort(); doReject(new Error('Request timed out')); }, 10 * 60 * 1000);
            function doFinalize() {
                if (finalized) return;
                finalized = true;
                clearTimeout(timeout);
                resolve();
            }
            function doReject(err) {
                if (finalized) return;
                finalized = true;
                clearTimeout(timeout);
                reject(err);
            }
            fetch('/api/desktop/chat/stream', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ message }),
                signal: ctrl.signal
            }).then(response => {
                if (!response.ok) return response.text().then(text => { throw new Error(text || ('HTTP ' + response.status)); });
                const reader = response.body.getReader();
                const decoder = new TextDecoder();
                let buffer = '';
                function processChunk() {
                    reader.read().then(({ done, value }) => {
                        if (done) { doFinalize(); return; }
                        buffer += decoder.decode(value, { stream: true });
                        const lines = buffer.split('\n');
                        buffer = lines.pop();
                        for (const line of lines) {
                            if (line.startsWith('data: ')) {
                                const data = line.slice(6).trim();
                                if (data === '[DONE]') { doFinalize(); reader.cancel().catch(() => {}); return; }
                                try {
                                    const parsed = JSON.parse(data);
                                    const event = parsed.event || parsed.type;
                                    if (event === 'llm_stream_delta') {
                                        const content = parsed.content || '';
                                        if (content) {
                                            streamingContent += content;
                                            responseEl.textContent = streamingContent;
                                            responseEl.classList.add('vd-quickchat-active');
                                        }
                                    } else if (event === 'final_response') {
                                        const text = parsed.detail || parsed.message || '';
                                        if (!streamingContent.trim() && text.trim()) {
                                            streamingContent = text;
                                            responseEl.textContent = text;
                                            responseEl.classList.add('vd-quickchat-active');
                                        }
                                    }
                                } catch (_) {}
                            }
                        }
                        processChunk();
                    }).catch(doReject);
                }
                processChunk();
            }).catch(doReject);
        });
    }

    function defaultWidgetBounds(index) {
        const workspace = $('vd-workspace');
        const width = 320;
        const height = 56;
        const x = Math.max(18, ((workspace && workspace.clientWidth) || window.innerWidth) - width - 18);
        return { x, y: 18 + index * (height + 12), w: width, h: height };
    }

    function widgetBounds(widget, index) {
        const fallback = defaultWidgetBounds(index);
        const w = Number(widget.w || widget.W || 0);
        const h = Number(widget.h || widget.H || 0);
        return {
            x: Number(widget.x || widget.X || fallback.x) || fallback.x,
            y: Number(widget.y || widget.Y || fallback.y) || fallback.y,
            w: w > 16 ? w : fallback.w,
            h: h > 16 ? h : fallback.h
        };
    }

    function wireDraggableWidget(card, widget) {
        const handle = card;
        let drag = null;
        function finishDrag(event) {
            if (!drag) return;
            if (event && event.pointerId != null && event.pointerId !== drag.pointerId) return;
            if (drag.holdTimer) window.clearTimeout(drag.holdTimer);
            if (event && handle.hasPointerCapture && handle.hasPointerCapture(drag.pointerId)) {
                handle.releasePointerCapture(drag.pointerId);
            }
            card.classList.remove('vd-dragging');
            document.body.classList.remove('vd-touch-drag-active');
            if (drag.moved) {
                persistWidgetBounds(widget, card);
                if (event) event.preventDefault();
            }
            drag = null;
        }
        handle.addEventListener('pointerdown', event => {
            if (event.button !== 0) return;
            const touchDrag = isTouchLikePointer(event);
            drag = {
                pointerId: event.pointerId,
                x: event.clientX,
                y: event.clientY,
                left: parseInt(card.style.left, 10) || 0,
                top: parseInt(card.style.top, 10) || 0,
                moved: false,
                ready: !touchDrag,
                touchDrag,
                holdTimer: 0
            };
            if (touchDrag) {
                drag.holdTimer = window.setTimeout(() => {
                    if (drag) drag.ready = true;
                }, TOUCH_DRAG_HOLD_MS);
            }
            handle.setPointerCapture(event.pointerId);
        });
        handle.addEventListener('pointermove', event => {
            if (!drag) return;
            const dx = event.clientX - drag.x;
            const dy = event.clientY - drag.y;
            if (card.__vdLongPressTriggered) return;
            if (drag.touchDrag && !drag.ready) {
                if (Math.hypot(dx, dy) > LONG_PRESS_MOVE_TOLERANCE) finishDrag(event);
                return;
            }
            if (!drag.moved && Math.hypot(dx, dy) < DRAG_THRESHOLD) return;
            drag.moved = true;
            card.classList.add('vd-dragging');
            if (drag.touchDrag) document.body.classList.add('vd-touch-drag-active');
            const pos = clampToWorkspace(drag.left + dx, drag.top + dy, card.offsetWidth, card.offsetHeight);
            card.style.left = pos.x + 'px';
            card.style.top = pos.y + 'px';
        });
        handle.addEventListener('pointerup', finishDrag);
        handle.addEventListener('pointercancel', finishDrag);
    }

    async function persistWidgetBounds(widget, card) {
        const updated = Object.assign({}, widget, {
            x: parseInt(card.style.left, 10) || 0,
            y: parseInt(card.style.top, 10) || 0,
            w: Math.round(card.offsetWidth),
            h: Math.round(card.offsetHeight)
        });
        try {
            await api('/api/desktop/widgets', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(updated)
            });
            await loadBootstrap();
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err.message });
        }
    }

    async function renderWidgetFrame(card, widget) {
        card.innerHTML = `<div class="vd-widget-body">${esc(t('desktop.loading'))}</div>`;
        const path = widgetFramePath(widget);
        try {
            const src = await desktopEmbedURL(path, { widget_id: widget.id });
            await ensureDesktopEmbedHasContent(src);
            card.replaceChildren(makeSandboxedFrame(src, widget.app_id, widget.id, '', 'vd-widget-frame', widget.title || widget.id));
            scheduleWidgetAutoSize(card.closest('.vd-widget'), widget);
        } catch (err) {
            card.innerHTML = `<div class="vd-widget-body">${esc(err.message)}</div>`;
            scheduleWidgetAutoSize(card.closest('.vd-widget'), widget);
        }
    }

    function wireFruityDockScroll(host) {
        const scroller = host && host.querySelector('[data-fruity-dock-scroll-region]'), track = host && host.querySelector('[data-fruity-dock-track]');
        if (!host || !scroller || !track) return;
        const queueUpdate = () => {
            const schedule = window.requestAnimationFrame || ((callback) => window.setTimeout(callback, 16));
            if (host._fruityDockScrollFrame) return;
            host._fruityDockScrollFrame = schedule(() => {
                host._fruityDockScrollFrame = 0;
                updateFruityDockScrollControls(host);
            });
        };
        host.querySelectorAll('[data-fruity-dock-scroll-button]').forEach(button => {
            button.addEventListener('click', event => {
                event.stopPropagation();
                const direction = button.dataset.fruityDockScrollButton === 'left' ? -1 : 1;
                scroller.scrollBy({ left: direction * Math.max(180, Math.floor(scroller.clientWidth * 0.72)), behavior: 'smooth' });
            });
        });
        scroller.addEventListener('scroll', queueUpdate, { passive: true });
        if (host._fruityDockResizeObserver) host._fruityDockResizeObserver.disconnect();
        if (window.ResizeObserver) {
            host._fruityDockResizeObserver = new ResizeObserver(queueUpdate);
            [scroller, track].forEach(item => host._fruityDockResizeObserver.observe(item));
        }
        queueUpdate();
    }

    function updateFruityDockScrollControls(host) {
        const scroller = host && host.querySelector('[data-fruity-dock-scroll-region]');
        if (!host || !scroller) return;
        const maxScroll = Math.max(0, scroller.scrollWidth - scroller.clientWidth);
        const overflowing = maxScroll > 2;
        const atStart = !overflowing || scroller.scrollLeft <= 2;
        const atEnd = !overflowing || scroller.scrollLeft >= maxScroll - 2;
        host.classList.toggle('vd-dock-overflowing', overflowing);
        host.classList.toggle('vd-dock-at-start', atStart);
        host.classList.toggle('vd-dock-at-end', atEnd);
        host.querySelectorAll('[data-fruity-dock-scroll-button]').forEach(button => {
            button.disabled = !overflowing || (button.dataset.fruityDockScrollButton === 'left' ? atStart : atEnd);
        });
    }

    function widgetFramePath(widget) {
        return widget.app_id
            ? 'Apps/' + widget.app_id + '/' + widget.entry
            : 'Widgets/' + widget.entry;
    }

    function renderStartApps() {
        const query = state.startQuery.trim().toLowerCase();
        const apps = startMenuApps().filter(app => !query || appName(app).toLowerCase().includes(query));
        $('vd-start-apps').innerHTML = apps.map(app => `<button class="vd-start-item" type="button" data-app-id="${esc(app.id)}">
            ${iconMarkup(iconForApp(app), iconGlyph(app), 'vd-sprite-start-item', 30)}
            <span>${esc(appName(app))}</span>
        </button>`).join('');
        $('vd-start-apps').querySelectorAll('[data-app-id]').forEach(btn => {
            btn.addEventListener('click', () => {
                $('vd-start-menu').hidden = true;
                openApp(btn.dataset.appId);
            });
            btn.addEventListener('contextmenu', event => showStartAppContextMenu(event, btn.dataset.appId));
        });
    }

    function renderTaskbar() {
        const host = $('vd-taskbar-apps');
        if (!host) return;
        host.classList.toggle('vd-dock', isFruityTheme());
        if (isFruityTheme()) {
            renderFruityDock();
            scheduleFruityDockOcclusionCheck();
            return;
        }
        document.body.classList.remove('fruity-dock-collapsed');
        state.fruityDockFootprint = null;
        host.classList.remove('vd-dock-overflowing', 'vd-dock-at-start', 'vd-dock-at-end');
        if (host._fruityDockResizeObserver) {
            host._fruityDockResizeObserver.disconnect();
            host._fruityDockResizeObserver = null;
        }
        renderStandardTaskbar();
    }

    function renderStandardTaskbar() {
        const host = $('vd-taskbar-apps');
        host.innerHTML = [...state.windows.values()].map(win => `<button type="button" class="vd-task-button ${win.id === state.activeWindowId ? 'active' : ''}" data-window-id="${esc(win.id)}">${esc(win.title)}</button>`).join('');
        host.querySelectorAll('[data-window-id]').forEach(btn => {
            btn.addEventListener('click', () => focusWindow(btn.dataset.windowId));
            btn.addEventListener('contextmenu', event => showWindowContextMenu(event, btn.dataset.windowId));
            wireLongPress(btn, event => showWindowContextMenu(event, btn.dataset.windowId));
        });
    }

    function renderFruityDock() {
        const host = $('vd-taskbar-apps');
        const runningWindows = [...state.windows.values()];
        const dockItems = dockApps().map(app => {
            const running = runningWindows.some(win => win.appId === app.id);
            const active = runningWindows.some(win => win.appId === app.id && win.id === state.activeWindowId);
            const stateClasses = [running ? 'running' : '', active ? 'active' : ''].filter(Boolean).join(' ');
            return `<button type="button" class="vd-dock-button ${esc(stateClasses)}" data-app-id="${esc(app.id)}" title="${esc(appName(app))}">
                ${iconMarkup(iconForApp(app), iconGlyph(app), 'vd-dock-icon', 34)}
                <span class="vd-dock-label">${esc(appName(app))}</span>
            </button>`;
        }).join('');
        host.innerHTML = `<button type="button" class="vd-dock-orb" data-fruity-dock-orb title="${esc(t('desktop.start_menu'))}">
            ${iconMarkup('home', 'A', 'vd-dock-orb-icon', 34)}
        </button>
        <button type="button" class="vd-dock-scroll-button vd-dock-scroll-button-left" data-fruity-dock-scroll-button="left" aria-label="${esc(t('desktop.dock_scroll_left'))}">
            ${iconMarkup('arrow-left', '<', 'vd-dock-scroll-icon', 18)}
        </button>
        <div class="vd-dock-scroll" data-fruity-dock-scroll-region>
            <div class="vd-dock-track" data-fruity-dock-track>${dockItems}</div>
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
        host.querySelectorAll('[data-app-id]').forEach(btn => {
            btn.addEventListener('click', () => {
                const existing = [...state.windows.values()].find(win => win.appId === btn.dataset.appId);
                if (existing) focusWindow(existing.id);
                else openApp(btn.dataset.appId);
            });
            btn.addEventListener('contextmenu', event => showStartAppContextMenu(event, btn.dataset.appId));
            wireLongPress(btn, event => showStartAppContextMenu(event, btn.dataset.appId));
        });
        wireFruityDockScroll(host);
    }

    function scheduleFruityDockOcclusionCheck() {
        if (state.fruityDockOcclusionFrame) return;
        const schedule = window.requestAnimationFrame || ((callback) => window.setTimeout(callback, 16));
        state.fruityDockOcclusionFrame = schedule(() => {
            state.fruityDockOcclusionFrame = 0;
            updateFruityDockOcclusion();
        });
    }

    function updateFruityDockOcclusion() {
        const body = document.body;
        const host = $('vd-taskbar-apps');
        if (!body || !host || !isFruityTheme()) {
            if (body) body.classList.remove('fruity-dock-collapsed');
            state.fruityDockFootprint = null;
            return;
        }
        const dockRect = fruityDockFootprint(host);
        const occluded = [...state.windows.values()].some(win => windowOverlapsFruityDock(win, dockRect));
        body.classList.toggle('fruity-dock-collapsed', occluded);
    }

    function fruityDockFootprint(host) {
        const rect = host.getBoundingClientRect();
        const collapsed = document.body.classList.contains('fruity-dock-collapsed');
        if (!collapsed && rect.width > 120 && rect.height > 40) {
            state.fruityDockFootprint = {
                left: rect.left,
                top: rect.top,
                right: rect.right,
                bottom: rect.bottom
            };
        }
        if (state.fruityDockFootprint) return state.fruityDockFootprint;
        const width = Math.min(920, Math.max(160, window.innerWidth - 170));
        const left = Math.max(0, (window.innerWidth - width) / 2);
        const bottom = window.innerHeight - 8;
        const height = 110;
        return { left, top: Math.max(0, bottom - height), right: left + width, bottom };
    }

    function windowOverlapsFruityDock(win, dockRect) {
        if (!win || !win.element || win.element.style.display === 'none' || win.element.hidden) return false;
        const rect = win.element.getBoundingClientRect();
        const margin = 6;
        return rect.right > dockRect.left + margin &&
            rect.left < dockRect.right - margin &&
            rect.bottom > dockRect.top + margin &&
            rect.top < dockRect.bottom - margin;
    }

    function windowTitle(appId) {
        if (appId === 'system-info') return t('desktop.system_info_title');
        const app = allApps().find(item => item.id === appId);
        return app ? appName(app) : appId;
    }

    function appWindowSize(appId) {
        const presets = {
            files: { width: 920, height: 600 },
            writer: { width: 960, height: 700 },
            sheets: { width: 1040, height: 690 },
            calculator: { width: 380, height: 640 },
            todo: { width: 900, height: 600 },
            'music-player': { width: 430, height: 260 },
            radio: { width: 960, height: 680 },
            calendar: { width: 950, height: 650 },
            'quick-connect': { width: 920, height: 640 },
            'code-studio': { width: 1280, height: 850 },
            launchpad: { width: 1100, height: 700 },
            'system-info': { width: 800, height: 600 },
            'agent-chat': { width: 800, height: 620 },
            'looper': { width: 900, height: 750 },
            camera: { width: 720, height: 600 }
        };
        if (presets[appId]) return presets[appId];
        return defaultWindowSize();
    }

    function appWindowMinSize(appId) {
        const mins = {
            'system-info': { width: 560, height: 460 },
            calculator: { width: 280, height: 420 }
        };
        return mins[appId] || { width: WINDOW_MIN_W, height: WINDOW_MIN_H };
    }

    function clampWindowSize(size) {
        const workspace = $('vd-workspace') || document.body;
        const workspaceRect = workspace.getBoundingClientRect();
        const margin = 16;
        return {
            width: Math.min(size.width, Math.max(1, workspaceRect.width - margin * 2)),
            height: Math.min(size.height, Math.max(1, workspaceRect.height - margin * 2))
        };
    }

    function nextWindowPosition(size) {
        const workspace = $('vd-workspace') || document.body;
        const workspaceRect = workspace.getBoundingClientRect();
        const margin = 16;
        const topStart = 72;
        const stepX = 28;
        const stepY = 24;
        const maxLeft = Math.max(margin, workspaceRect.width - size.width - margin);
        const maxTop = Math.max(margin, workspaceRect.height - size.height - margin);
        const slotsX = Math.max(1, Math.floor((maxLeft - margin) / stepX) + 1);
        const slotsY = Math.max(1, Math.floor((maxTop - topStart) / stepY) + 1);
        const index = state.windows.size;
        const left = margin + (index % slotsX) * stepX;
        const top = topStart + (Math.floor(index / slotsX) % slotsY) * stepY;
        return {
            left: Math.min(maxLeft, Math.max(margin, left)),
            top: Math.min(maxTop, Math.max(margin, top))
        };
    }

    function normalizeDesktopPath(path) {
        return String(path || '').replace(/\\/g, '/').replace(/\/+/g, '/').replace(/^\.\//, '').trim();
    }

    function updateWindowContext(windowId, patch) {
        const win = state.windows.get(windowId);
        if (!win) return;
        win.context = Object.assign({}, win.context || {}, patch || {});
        if (win.context.path != null) win.context.path = normalizeDesktopPath(win.context.path);
    }

    function findExistingAppWindow(appId, context) {
        return [...state.windows.values()].find(win => {
            if (win.appId !== appId) return false;
            if ((appId === 'writer' || appId === 'sheets') && context && context.path != null) {
                const requestedPath = normalizeDesktopPath(context.path);
                return win.context && normalizeDesktopPath(win.context.path) === requestedPath;
            }
            return appId !== 'editor' && appId !== 'writer' && appId !== 'sheets';
        });
    }

    function openApp(appId, context) {
        const existing = findExistingAppWindow(appId, context || {});
        if (existing) {
            focusWindow(existing.id);
            if (appId === 'files' && context && context.path != null) {
                if (window.FileManager && typeof window.FileManager.navigateTo === 'function') {
                    window.FileManager.navigateTo(existing.id, context.path);
                } else {
                    renderFiles(existing.id, context.path);
                }
            }
            return;
        }
        const title = windowTitle(appId);
        const id = 'w-' + appId + '-' + Date.now();
        const win = document.createElement('section');
        win.className = 'vd-window';
        win.dataset.windowId = id;
        const requestedSize = appWindowSize(appId);
        const size = clampWindowSize(requestedSize);
        const position = nextWindowPosition(size);
        win.style.left = position.left + 'px';
        win.style.top = position.top + 'px';
        win.style.width = size.width + 'px';
        win.style.height = size.height + 'px';
        const isResizable = appId !== 'calculator';
        win.style.minWidth = Math.min(WINDOW_MIN_W, size.width) + 'px';
        win.style.minHeight = Math.min(WINDOW_MIN_H, size.height) + 'px';
        const minSize = appWindowMinSize(appId);
        win.style.minWidth = Math.min(minSize.width, size.width) + 'px';
        win.style.minHeight = Math.min(minSize.height, size.height) + 'px';
        if (!isResizable) {
            win.style.maxWidth = size.width + 'px';
            win.style.maxHeight = size.height + 'px';
            win.style.resize = 'none';
        }
        win.style.zIndex = String(++state.z);
        win.innerHTML = `<header class="vd-window-titlebar">
            <div>
                <div class="vd-window-title">${esc(title)}</div>
                <div class="vd-window-subtitle">${esc(t('desktop.window_ready'))}</div>
            </div>
            <div class="vd-window-actions">
                <button class="vd-window-button" type="button" data-action="minimize" title="${esc(t('desktop.minimize'))}">_</button>
                ${isResizable ? `<button class="vd-window-button" type="button" data-action="maximize" title="${esc(t('desktop.maximize'))}">â–¡</button>` : ''}
                <button class="vd-window-button" type="button" data-action="close" title="${esc(t('desktop.close'))}">x</button>
            </div>
        </header>
        <div class="vd-window-content" data-window-content></div>
        ${isResizable ? resizeHandleMarkup() : ''}`;
        $('vd-window-layer').appendChild(win);
        const windowContext = Object.assign({}, context || {});
        if (windowContext.path != null) windowContext.path = normalizeDesktopPath(windowContext.path);
        state.windows.set(id, { id, appId, title, element: win, maximized: false, restoreBounds: null, context: windowContext });
        wireWindow(win, id);
        focusWindow(id);
        renderAppContent(id, appId, windowContext);
        renderTaskbar();
    }

    function resizeHandleMarkup() {
        return ['n', 's', 'e', 'w', 'ne', 'nw', 'se', 'sw']
            .map(edge => `<span class="vd-resize-handle vd-resize-${edge}" data-resize="${edge}"></span>`)
            .join('');
    }

    function minimizeWindow(id) {
        const item = state.windows.get(id);
        if (!item) return;
        item.element.style.display = 'none';
        if (state.activeWindowId === id) state.activeWindowId = '';
        renderTaskbar();
    }

    function scheduleWindowPointerFrame(target, callback) {
        if (!target) return;
        target.pendingFrame = callback;
        if (target.raf) return;
        const schedule = window.requestAnimationFrame || ((fn) => window.setTimeout(fn, 16));
        target.raf = schedule(() => {
            const pending = target.pendingFrame;
            target.raf = 0;
            target.pendingFrame = null;
            if (pending) pending();
        });
    }

    function cancelWindowPointerFrame(target, flush) {
        if (!target || !target.raf) return;
        const pending = target.pendingFrame;
        const cancel = window.cancelAnimationFrame || window.clearTimeout;
        cancel(target.raf);
        target.raf = 0;
        target.pendingFrame = null;
        if (flush && pending) pending();
    }

    function wireWindow(win, id) {
        win.addEventListener('pointerdown', () => focusWindow(id));
        wireWindowContextMenu(win, id);
        win.querySelector('[data-action="close"]').addEventListener('click', () => closeWindow(id));
        win.querySelector('[data-action="minimize"]').addEventListener('click', () => minimizeWindow(id));
        const maximizeBtn = win.querySelector('[data-action="maximize"]');
        if (maximizeBtn) maximizeBtn.addEventListener('click', () => toggleMaximizeWindow(id));
        const bar = win.querySelector('.vd-window-titlebar');
        let drag = null;
        bar.addEventListener('pointerdown', (event) => {
            if (event.target.closest('button, .vd-window-menubar')) return;
            if (isCompactViewport()) return;
            if (state.windows.get(id) && state.windows.get(id).maximized) return;
            drag = {
                x: event.clientX,
                y: event.clientY,
                left: parseInt(win.style.left, 10) || 0,
                top: parseInt(win.style.top, 10) || 0,
                raf: 0,
                pendingFrame: null
            };
            bar.setPointerCapture(event.pointerId);
        });
        bar.addEventListener('pointermove', (event) => {
            if (!drag) return;
            drag.clientX = event.clientX;
            drag.clientY = event.clientY;
            const activeDrag = drag;
            scheduleWindowPointerFrame(drag, () => {
                if (drag !== activeDrag) return;
                const maxLeft = window.innerWidth - 80;
                const maxTop = window.innerHeight - 120;
                win.style.left = Math.min(maxLeft, Math.max(8, activeDrag.left + activeDrag.clientX - activeDrag.x)) + 'px';
                win.style.top = Math.min(maxTop, Math.max(8, activeDrag.top + activeDrag.clientY - activeDrag.y)) + 'px';
                scheduleFruityDockOcclusionCheck();
            });
        });
        bar.addEventListener('pointerup', () => {
            cancelWindowPointerFrame(drag, true);
            drag = null;
        });
        bar.addEventListener('pointercancel', () => {
            cancelWindowPointerFrame(drag);
            drag = null;
        });
        if (win.dataset.windowId && state.windows.get(win.dataset.windowId) && state.windows.get(win.dataset.windowId).appId !== 'calculator') {
            bar.addEventListener('dblclick', event => {
                if (event.target.closest('button, .vd-window-menubar')) return;
                toggleMaximizeWindow(id);
            });
        }
        wireLongPress(bar, event => showWindowContextMenu(event, id));
        wireWindowTouchGestures(win, id);
        const winAppId = state.windows.get(id)?.appId;
        if (winAppId !== 'calculator') wireWindowResize(win, id);
    }

    function wireWindowTouchGestures(win, id) {
        const bar = win.querySelector('.vd-window-titlebar');
        if (!bar) return;
        let gesture = null;
        bar.addEventListener('pointerdown', event => {
            if (!isCompactViewport() || !isTouchLikePointer(event) || event.button !== 0 || event.target.closest('button')) return;
            gesture = {
                pointerId: event.pointerId,
                x: event.clientX,
                y: event.clientY,
                time: performance.now()
            };
            bar.setPointerCapture(event.pointerId);
        });
        bar.addEventListener('pointermove', event => {
            if (!gesture || gesture.pointerId !== event.pointerId) return;
            const dy = event.clientY - gesture.y;
            if (dy > 12) event.preventDefault();
        });
        bar.addEventListener('pointerup', event => {
            if (!gesture || gesture.pointerId !== event.pointerId) return;
            const dx = event.clientX - gesture.x;
            const dy = event.clientY - gesture.y;
            const elapsed = Math.max(1, performance.now() - gesture.time);
            gesture = null;
            if (bar.hasPointerCapture && bar.hasPointerCapture(event.pointerId)) {
                bar.releasePointerCapture(event.pointerId);
            }
            if (dy > 80 && dy > Math.abs(dx) * 1.2 && dy / elapsed > 0.25) {
                event.preventDefault();
                minimizeWindow(id);
            }
        });
        bar.addEventListener('pointercancel', event => {
            if (gesture && gesture.pointerId === event.pointerId) gesture = null;
        });
    }

    function windowBounds(win) {
        return {
            left: parseInt(win.style.left, 10) || 0,
            top: parseInt(win.style.top, 10) || 0,
            width: Math.round(win.offsetWidth),
            height: Math.round(win.offsetHeight)
        };
    }

    function workspaceBoundsForWindow() {
        const layer = $('vd-window-layer');
        const taskbar = document.querySelector('.vd-taskbar');
        const width = (layer && layer.clientWidth) || window.innerWidth;
        let height = (layer && layer.clientHeight) || window.innerHeight;
        if (isCompactViewport() && window.visualViewport) {
            height = Math.min(height, Math.max(1, window.visualViewport.height - ((taskbar && taskbar.offsetHeight) || 0)));
        }
        return { width, height };
    }

    function toggleMaximizeWindow(id) {
        const item = state.windows.get(id);
        if (!item) return;
        const win = item.element;
        if (item.maximized) {
            const b = item.restoreBounds || { left: 80, top: 48, width: 820, height: 560 };
            win.classList.remove('maximized');
            win.style.left = b.left + 'px';
            win.style.top = b.top + 'px';
            win.style.width = b.width + 'px';
            win.style.height = b.height + 'px';
            item.maximized = false;
        } else {
            item.restoreBounds = windowBounds(win);
            const bounds = workspaceBoundsForWindow();
            win.classList.add('maximized');
            win.style.left = '0';
            win.style.top = '0';
            win.style.width = Math.max(WINDOW_MIN_W, bounds.width) + 'px';
            win.style.height = Math.max(WINDOW_MIN_H, bounds.height) + 'px';
            item.maximized = true;
        }
        focusWindow(id);
        renderWindowMenus(id);
        scheduleFruityDockOcclusionCheck();
    }

    function wireWindowResize(win, id) {
        win.querySelectorAll('[data-resize]').forEach(handle => {
            let resize = null;
            handle.addEventListener('pointerdown', event => {
                const item = state.windows.get(id);
                if (isCompactViewport()) return;
                if (item && item.maximized) return;
                event.preventDefault();
                event.stopPropagation();
                focusWindow(id);
                resize = {
                    edge: handle.dataset.resize,
                    x: event.clientX,
                    y: event.clientY,
                    bounds: windowBounds(win),
                    raf: 0,
                    pendingFrame: null
                };
                handle.setPointerCapture(event.pointerId);
            });
            handle.addEventListener('pointermove', event => {
                if (!resize) return;
                resize.clientX = event.clientX;
                resize.clientY = event.clientY;
                const activeResize = resize;
                scheduleWindowPointerFrame(resize, () => {
                    if (resize !== activeResize) return;
                    const dx = activeResize.clientX - activeResize.x;
                    const dy = activeResize.clientY - activeResize.y;
                    applyResize(win, activeResize.edge, activeResize.bounds, dx, dy);
                });
            });
            handle.addEventListener('pointerup', event => {
                if (!resize) return;
                handle.releasePointerCapture(event.pointerId);
                cancelWindowPointerFrame(resize, true);
                resize = null;
            });
            handle.addEventListener('pointercancel', () => {
                cancelWindowPointerFrame(resize);
                resize = null;
            });
        });
    }

    function applyResize(win, edge, start, dx, dy) {
        const workspace = workspaceBoundsForWindow();
        let left = start.left;
        let top = start.top;
        let width = start.width;
        let height = start.height;
        if (edge.includes('e')) width = Math.max(WINDOW_MIN_W, start.width + dx);
        if (edge.includes('s')) height = Math.max(WINDOW_MIN_H, start.height + dy);
        if (edge.includes('w')) {
            width = Math.max(WINDOW_MIN_W, start.width - dx);
            left = start.left + (start.width - width);
        }
        if (edge.includes('n')) {
            height = Math.max(WINDOW_MIN_H, start.height - dy);
            top = start.top + (start.height - height);
        }
        left = Math.max(8, Math.min(left, workspace.width - 80));
        top = Math.max(8, Math.min(top, workspace.height - 80));
        width = Math.min(width, workspace.width - left - 8);
        height = Math.min(height, workspace.height - top - 8);
        win.style.left = left + 'px';
        win.style.top = top + 'px';
        win.style.width = width + 'px';
        win.style.height = height + 'px';
        scheduleFruityDockOcclusionCheck();
    }

    function nearestWindowScroller(control) {
        const content = control.closest('.vd-window-content');
        let node = control.parentElement;
        while (node && content && node !== content) {
            const style = window.getComputedStyle(node);
            if (/(auto|scroll)/.test(style.overflowY) && node.scrollHeight > node.clientHeight + 1) return node;
            node = node.parentElement;
        }
        return content;
    }

    function ensureFocusedControlVisible(event) {
        const control = event && event.target;
        if (!control || !control.closest || !control.matches) return;
        if (!control.closest('.vd-window')) return;
        if (!control.matches('input, textarea, select, [contenteditable="true"]')) return;
        if (!window.visualViewport) return;
        window.setTimeout(() => {
            const rect = control.getBoundingClientRect();
            const viewportBottom = window.visualViewport.offsetTop + window.visualViewport.height;
            const targetBottom = viewportBottom - 80;
            if (rect.bottom <= targetBottom) return;
            const scroller = nearestWindowScroller(control);
            if (scroller) scroller.scrollTop += Math.ceil(rect.bottom - targetBottom);
        }, 60);
    }

    function focusWindow(id) {
        const win = state.windows.get(id);
        if (!win) return;
        if (state.z > 100000) normalizeWindowZIndexes();
        win.element.style.display = '';
        win.element.style.zIndex = String(++state.z);
        state.activeWindowId = id;
        state.windows.forEach(item => item.element.classList.toggle('active', item.id === id));
        renderTaskbar();
        scheduleFruityDockOcclusionCheck();
    }

    function closeWindow(id) {
        const win = state.windows.get(id);
        if (!win) return;
        clearWindowMenus(id);
        disposeAppWindow(win);
        win.element.remove();
        state.windows.delete(id);
        if (state.activeWindowId === id) state.activeWindowId = '';
        renderTaskbar();
        scheduleFruityDockOcclusionCheck();
    }

    function closeContextMenu() {
        if (state.contextMenu) {
            state.contextMenu.remove();
            state.contextMenu = null;
        }
        if (state.contextMenuKeydown) {
            document.removeEventListener('keydown', state.contextMenuKeydown);
            state.contextMenuKeydown = null;
        }
    }

    function closeContextMenuOnEscape(event) {
        if (event.key === 'Escape') closeContextMenu();
    }

    function registerWindowCleanup(windowId, cleanup) {
        if (!windowId || typeof cleanup !== 'function') return;
        const items = state.windowCleanups.get(windowId) || [];
        items.push(cleanup);
        state.windowCleanups.set(windowId, items);
    }

    function normalizeWindowZIndexes() {
        const wins = [...state.windows.values()].sort((a, b) => Number(a.element.style.zIndex || 0) - Number(b.element.style.zIndex || 0));
