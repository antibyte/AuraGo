    function minimizeWindow(id) {
        const item = state.windows.get(id);
        if (!item) return;
        if (item.minimizing) return;
        item.minimizing = true;
        if (state.activeWindowId === id) state.activeWindowId = '';
        renderTaskbar();
        animateThen(item.element, 'vd-window-minimizing', isFruityTheme() ? 180 : 130, () => {
            item.element.style.display = 'none';
            item.minimizing = false;
            scheduleFruityDockOcclusionCheck();
        });
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
        const aiBtn = win.querySelector('[data-action="ai-context"]'); if (aiBtn) aiBtn.addEventListener('click', () => openAgentChatForWindow(id));
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
            if (gesture && gesture.pointerId === event.pointerId) {
                gesture = null;
                if (bar.hasPointerCapture && bar.hasPointerCapture(event.pointerId)) {
                    bar.releasePointerCapture(event.pointerId);
                }
            }
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
        animateThen(win, 'vd-window-state-changing', isFruityTheme() ? 180 : 120);
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
            handle.addEventListener('pointercancel', event => {
                if (handle.hasPointerCapture && handle.hasPointerCapture(event.pointerId)) {
                    handle.releasePointerCapture(event.pointerId);
                }
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
        const wasHidden = win.element.style.display === 'none' || win.element.hidden;
        win.minimizing = false;
        win.element.style.display = '';
        win.element.style.zIndex = String(++state.z);
        state.activeWindowId = id;
        state.windows.forEach(item => item.element.classList.toggle('active', item.id === id));
        if (wasHidden) animateThen(win.element, 'vd-window-restoring', isFruityTheme() ? 210 : 140);
        renderTaskbar();
        scheduleFruityDockOcclusionCheck();
    }

    function closeWindow(id) {
        const win = state.windows.get(id);
        if (!win) return;
        if (win.closing) return;
        win.closing = true;
        clearWindowMenus(id);
        if (state.activeWindowId === id) state.activeWindowId = '';
        renderTaskbar();
        animateThen(win.element, 'vd-window-closing', isFruityTheme() ? 180 : 130, () => {
            disposeAppWindow(win);
            win.element.remove();
            state.windows.delete(id);
            renderTaskbar();
            scheduleFruityDockOcclusionCheck();
        });
    }

    function closeContextMenu(immediate) {
        if (state.contextMenu) {
            const menu = state.contextMenu;
            state.contextMenu = null;
            if (immediate || menu.classList.contains('vd-context-menu-closing')) {
                menu.remove();
            } else {
                animateThen(menu, 'vd-context-menu-closing', isFruityTheme() ? 150 : 100, () => menu.remove());
            }
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
        wins.forEach((win, i) => {
            const z = (i + 1) * 10;
            win.element.style.zIndex = String(z);
        });
        state.z = wins.length * 10;
    }
