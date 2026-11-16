    function minimizeWindow(id) {
        const item = state.windows.get(id);
        if (!item) return;
        if (item.isGadget) return; // gadgets cannot be minimized
        if (item.minimizing) return;
        closeWindowLayoutPopupFor(id);
        emitDesktopWindowEffect('minimize', id);
        desktopSound('window.minimize');
        item.minimizing = true;
        const minimizeRevision = item.minimizeRevision = (item.minimizeRevision || 0) + 1;
        if (state.activeWindowId === id) state.activeWindowId = '';
        const delta = windowMinimizeTargetDelta(item.element, id);
        if (delta) {
            item.element.style.setProperty('--vd-minimize-dx', delta.dx + 'px');
            item.element.style.setProperty('--vd-minimize-dy', delta.dy + 'px');
        }
        renderTaskbar();
        animateThen(item.element, 'vd-window-minimizing', themeMotionMs('minimize'), () => {
            if (item.minimizeRevision !== minimizeRevision || !item.minimizing || item.closing) return;
            item.element.style.display = 'none';
            refreshDesktopWindowEffects();
            item.minimizing = false;
            scheduleFruityDockOcclusionCheck();
        });
    }

    /* Compute the translation from the window center to its taskbar button
       (standard) or dock icon (fruity) so minimize/restore can travel there. */
    function windowMinimizeTargetDelta(win, id) {
        const item = state.windows.get(id);
        const host = $('vd-taskbar-apps');
        if (!item || !host) return null;
        let target = null;
        if (shellLayout() === 'dock') {
            target = host.querySelector('.vd-dock-button[data-app-id="' + cssSel(item.appId) + '"]');
        } else {
            target = host.querySelector('.vd-task-button[data-app-id="' + cssSel(item.appId) + '"]');
        }
        if (!target) return null;
        const winRect = win.getBoundingClientRect();
        const btnRect = target.getBoundingClientRect();
        if (!winRect.width || !btnRect.width) return null;
        return {
            dx: Math.round((btnRect.left + btnRect.width / 2) - (winRect.left + winRect.width / 2)),
            dy: Math.round((btnRect.top + btnRect.height / 2) - (winRect.top + winRect.height / 2))
        };
    }

    /* Run a programmatic bounds change (maximize/restore/snap) with a smooth
       transition. Never used during pointer drag/resize. */
    function animateWindowBounds(win, apply) {
        const reduced = typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
        if (reduced || (document.body && document.body.dataset.animations === 'false')) { apply(); return; }
        win.classList.add('vd-window-bounds-animated');
        apply();
        window.setTimeout(() => win.classList.remove('vd-window-bounds-animated'), 240);
    }

    function clearShowDesktopPeek() {
        state.showDesktopPeekIds = null;
        const btn = document.getElementById('vd-show-desktop-btn');
        if (btn) {
            btn.classList.remove('active');
            btn.setAttribute('aria-pressed', 'false');
            btn.title = t('desktop.show_desktop');
        }
    }

    function toggleShowDesktop() {
        if (isCompactViewport()) {
            minimizeAllWindows();
            return;
        }
        const btn = document.getElementById('vd-show-desktop-btn');
        if (state.showDesktopPeekIds && state.showDesktopPeekIds.length) {
            const ids = state.showDesktopPeekIds.filter(id => state.windows.has(id));
            state.showDesktopPeekIds = null;
            if (btn) {
                btn.classList.remove('active');
                btn.setAttribute('aria-pressed', 'false');
                btn.title = t('desktop.show_desktop');
            }
            ids.forEach(id => {
                const win = state.windows.get(id);
                if (!win || !win.element) return;
                win.minimizing = false;
                win.element.style.display = '';
            });
            if (ids.length) focusWindow(ids[ids.length - 1], { fromShowDesktopRestore: true });
            renderTaskbar();
            scheduleSessionPersist();
            return;
        }
        const ids = taskbarWindows()
            .filter(win => win.element && win.element.style.display !== 'none')
            .map(win => win.id);
        if (!ids.length) return;
        state.showDesktopPeekIds = ids.slice();
        ids.forEach(id => minimizeWindow(id));
        if (btn) {
            btn.classList.add('active');
            btn.setAttribute('aria-pressed', 'true');
            btn.title = t('desktop.show_desktop_restore');
        }
    }

    function minimizeAllWindows() {
        state.windows.forEach((item, id) => minimizeWindow(id));
        clearShowDesktopPeek();
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

    let activeDesktopWindowLayoutPopup = null;

    function wireWindowLayoutControl(button, id) {
        let holdTimer = 0;
        let suppressNextClick = false;
        button.setAttribute('aria-haspopup', 'menu');
        button.setAttribute('aria-expanded', 'false');
        button.setAttribute('aria-controls', 'vd-window-layout-menu');
        button.addEventListener('click', event => {
            if (suppressNextClick) {
                event.preventDefault();
                event.stopPropagation();
                suppressNextClick = false;
                return;
            }
            toggleMaximizeWindow(id);
        });
        button.addEventListener('contextmenu', event => {
            event.preventDefault();
            event.stopPropagation();
            openWindowLayoutMenu(id);
        });
        button.addEventListener('keydown', event => {
            if (event.key !== 'ContextMenu' && !(event.key === 'F10' && event.shiftKey)) return;
            event.preventDefault();
            openWindowLayoutMenu(id);
        });
        button.addEventListener('pointerdown', event => {
            if (event.button !== 0) return;
            if (holdTimer) window.clearTimeout(holdTimer);
            holdTimer = window.setTimeout(() => {
                holdTimer = 0;
                suppressNextClick = true;
                openWindowLayoutMenu(id);
            }, 550);
        });
        const cancelHold = () => {
            if (holdTimer) window.clearTimeout(holdTimer);
            holdTimer = 0;
        };
        button.addEventListener('pointerup', () => {
            cancelHold();
            window.setTimeout(() => { suppressNextClick = false; }, 0);
        });
        button.addEventListener('pointercancel', () => { cancelHold(); suppressNextClick = false; });
        button.addEventListener('pointerleave', () => { cancelHold(); suppressNextClick = false; });
    }

    function windowLayoutZones() {
        return DESKTOP_WINDOW_LAYOUT_ZONES.slice();
    }

    function windowLayoutOptions(id) {
        const item = state.windows.get(id);
        if (!item || !item.element || item.isGadget) return [];
        const limits = windowLayoutLimits(item.element);
        return windowLayoutZones().map(zone => {
            const rect = desktopWindowLayoutRect(zone, limits.width, limits.height);
            return { zone, rect, fits: desktopWindowLayoutFits(rect, limits) };
        });
    }

    function windowLayoutCanRestore(id) {
        const item = state.windows.get(id);
        return !!(item && !item.isGadget && item.restoreBounds);
    }

    function closeWindowLayoutPopup(restoreFocus) {
        const popup = activeDesktopWindowLayoutPopup;
        if (!popup) return;
        activeDesktopWindowLayoutPopup = null;
        document.removeEventListener('pointerdown', popup.onPointerDown, true);
        document.removeEventListener('keydown', popup.onKeyDown, true);
        popup.element.remove();
        if (popup.opener) popup.opener.setAttribute('aria-expanded', 'false');
        if (restoreFocus && popup.opener && popup.opener.isConnected) popup.opener.focus({ preventScroll: true });
    }

    function closeWindowLayoutPopupFor(id) {
        const popup = activeDesktopWindowLayoutPopup;
        if (popup && (popup.windowId === id || popup.candidateIds.includes(id))) closeWindowLayoutPopup(false);
    }

    function positionWindowLayoutPopup(popup, anchor) {
        const rect = anchor && anchor.getBoundingClientRect ? anchor.getBoundingClientRect() : null;
        const margin = 8;
        const width = popup.offsetWidth || 280;
        const height = popup.offsetHeight || 360;
        let left = rect ? rect.left : margin;
        let top = rect ? rect.bottom + 6 : window.innerHeight - height - margin;
        left = Math.max(margin, Math.min(left, window.innerWidth - width - margin));
        if (top + height > window.innerHeight - margin) top = rect ? rect.top - height - 6 : margin;
        top = Math.max(margin, Math.min(top, window.innerHeight - height - margin));
        popup.style.left = Math.round(left) + 'px';
        popup.style.top = Math.round(top) + 'px';
    }

    function wireWindowLayoutPopup(popup, opener, windowId, candidateIds) {
        const onPointerDown = event => {
            if (popup.contains(event.target) || event.target === opener) return;
            closeWindowLayoutPopup(false);
        };
        const onKeyDown = event => {
            if (event.key === 'Escape') {
                event.preventDefault();
                event.stopPropagation();
                closeWindowLayoutPopup(true);
                return;
            }
            if (event.key === 'Tab') {
                closeWindowLayoutPopup(true);
                return;
            }
            const items = [...popup.querySelectorAll('button:not(:disabled)')];
            if (!items.length) return;
            const index = items.indexOf(document.activeElement);
            let next = null;
            if (event.key === 'ArrowDown') next = items[index < 0 ? 0 : (index + 1) % items.length];
            else if (event.key === 'ArrowUp') next = items[index < 0 ? items.length - 1 : (index - 1 + items.length) % items.length];
            else if (event.key === 'Home') next = items[0];
            else if (event.key === 'End') next = items[items.length - 1];
            if (!next) return;
            event.preventDefault();
            next.focus({ preventScroll: true });
        };
        activeDesktopWindowLayoutPopup = { element: popup, opener, windowId, candidateIds: candidateIds || [], onPointerDown, onKeyDown };
        document.addEventListener('pointerdown', onPointerDown, true);
        document.addEventListener('keydown', onKeyDown, true);
    }

    function openWindowLayoutMenu(id) {
        const item = state.windows.get(id);
        const opener = item && item.element && item.element.querySelector('[data-action="maximize"]');
        if (!item || item.isGadget || !opener) return false;
        closeWindowLayoutPopup(false);
        if (state.contextMenu) closeContextMenu(true);
        const menu = document.createElement('div');
        menu.id = 'vd-window-layout-menu';
        menu.className = 'vd-context-menu vd-window-layout-menu';
        menu.setAttribute('role', 'menu');
        menu.setAttribute('aria-label', t('desktop.window_layouts'));
        const layoutLabels = {
            maximize: 'desktop.maximize',
            'left-half': 'desktop.snap_left',
            'right-half': 'desktop.snap_right',
            'top-half': 'desktop.snap_top_half',
            'bottom-half': 'desktop.snap_bottom_half',
            'top-left': 'desktop.snap_top_left',
            'top-right': 'desktop.snap_top_right',
            'bottom-left': 'desktop.snap_bottom_left',
            'bottom-right': 'desktop.snap_bottom_right'
        };
        const addButton = (labelKey, icon, action, disabled, checked, title) => {
            const button = document.createElement('button');
            button.type = 'button';
            button.className = 'vd-context-item vd-window-layout-item';
            button.setAttribute('role', action === 'restore' ? 'menuitem' : 'menuitemradio');
            if (action !== 'restore') button.setAttribute('aria-checked', checked ? 'true' : 'false');
            const glyph = document.createElement('span');
            glyph.className = 'vd-context-icon';
            glyph.setAttribute('aria-hidden', 'true');
            glyph.textContent = icon;
            const label = document.createElement('span');
            label.className = 'vd-context-label';
            label.textContent = t(labelKey);
            button.append(glyph, label);
            if (disabled) {
                button.disabled = true;
                button.title = t('desktop.snap_layout_too_small');
                button.setAttribute('aria-description', t('desktop.snap_layout_too_small'));
            } else {
                button.addEventListener('click', () => {
                    closeWindowLayoutPopup(true);
                    if (action === 'restore') restoreWindowFromSnap(id);
                    else applyWindowSnap(item.element, action);
                });
            }
            if (title) button.title = title;
            menu.appendChild(button);
        };
        if (windowLayoutCanRestore(id)) addButton('desktop.restore', '↶', 'restore', false, false);
        windowLayoutOptions(id).forEach(option => {
            addButton(layoutLabels[option.zone], option.zone === 'maximize' ? '□' : '▦', option.zone,
                !option.fits, item.snapped === option.zone || (option.zone === 'maximize' && item.maximized));
        });
        document.body.appendChild(menu);
        positionWindowLayoutPopup(menu, opener);
        opener.setAttribute('aria-expanded', 'true');
        wireWindowLayoutPopup(menu, opener, id);
        const first = menu.querySelector('button:not(:disabled)');
        if (first) first.focus({ preventScroll: true });
        return true;
    }

    function desktopWindowSnapPartnerZone(zone) {
        if (zone === 'left-half') return 'right-half';
        if (zone === 'right-half') return 'left-half';
        return '';
    }

    function windowSnapPartnerCandidates(windowId, partnerZone) {
        const source = state.windows.get(windowId);
        if (!source || !source.element || source.closing || source.checkingClose || source.minimizing
            || source.element.style.display === 'none' || source.element.classList.contains('vd-space-hidden')
            || desktopWindowSnapPartnerZone(source.snapped) !== partnerZone) return [];
        const candidates = [...state.windows.values()].filter(candidate => {
            if (candidate === source || candidate.isGadget || candidate.closing || candidate.checkingClose || !candidate.element) return false;
            if (windowSpaceId(candidate) !== windowSpaceId(source)) return false;
            const bounds = windowLayoutLimits(candidate.element);
            return desktopWindowLayoutFits(desktopWindowLayoutRect(partnerZone, bounds.width, bounds.height), bounds);
        });
        const occupiedSide = partnerZone === 'left-half' ? 'left' : 'right';
        if (candidates.some(candidate => candidate.element.style.display !== 'none' && !candidate.minimizing
            && desktopWindowLayoutOccupiesHalf(candidate, occupiedSide))) return [];
        return candidates;
    }

    function chooseWindowSnapPartner(windowId, candidateId, partnerZone) {
        const candidate = windowSnapPartnerCandidates(windowId, partnerZone).find(win => win.id === candidateId);
        return !!candidate && applyWindowSnap(candidate.element, partnerZone, { showPartnerChooser: false });
    }

    function showWindowSnapPartnerChooser(windowId, partnerZone) {
        if (!windowSnapPartnerCandidates(windowId, partnerZone).length) return false;
        return openDesktopWindowOverview({
            mode: 'snap-partner', sourceId: windowId, partnerZone,
            opener: state.windows.get(windowId)?.element.querySelector('[data-action="maximize"]')
        });
    }

function wireWindow(win, id) {
        win.addEventListener('pointerdown', () => focusWindow(id));
        wireWindowContextMenu(win, id);
        win.querySelector('[data-action="close"]').addEventListener('click', () => closeWindow(id));
        win.querySelector('[data-action="minimize"]').addEventListener('click', () => minimizeWindow(id));
        const aiBtn = win.querySelector('[data-action="ai-context"]'); if (aiBtn) aiBtn.addEventListener('click', () => openAgentChatForWindow(id));
        const maximizeBtn = win.querySelector('[data-action="maximize"]');
        if (maximizeBtn) wireWindowLayoutControl(maximizeBtn, id);
        const bar = win.querySelector('.vd-window-titlebar');
        let drag = null;
        bar.addEventListener('pointerdown', (event) => {
            if (event.target.closest('button, .vd-window-menu-popover')) return;
            if (window.useMobileDesktopMode && window.useMobileDesktopMode()) return;
            if (state.windows.get(id) && state.windows.get(id).maximized) return;
            drag = {
                x: event.clientX,
                y: event.clientY,
                left: parseInt(win.style.left, 10) || 0,
                top: parseInt(win.style.top, 10) || 0,
                moved: false,
                raf: 0,
                pendingFrame: null,
                snapZone: null
            };
            bar.setPointerCapture(event.pointerId);
            win.classList.add('dragging');
            emitDesktopWindowEffect('drag-start', id);
        });
        bar.addEventListener('pointermove', (event) => {
            if (!drag) return;
            drag.clientX = event.clientX;
            drag.clientY = event.clientY;
            const activeDrag = drag;
            scheduleWindowPointerFrame(drag, () => {
                if (drag !== activeDrag) return;
                const maxLeft = window.innerWidth - 80;
                const taskbar = document.querySelector('.vd-taskbar');
                const taskbarReserve = (shellLayout() === 'taskbar' && taskbar) ? taskbar.offsetHeight : 0;
                const maxTop = Math.max(0, window.innerHeight - taskbarReserve - win.offsetHeight);
                const newLeft = Math.min(maxLeft, Math.max(0, activeDrag.left + activeDrag.clientX - activeDrag.x));
                const newTop = Math.min(maxTop, Math.max(0, activeDrag.top + activeDrag.clientY - activeDrag.y));
                if (!activeDrag.moved && (Math.abs(activeDrag.clientX - activeDrag.x) > 2 || Math.abs(activeDrag.clientY - activeDrag.y) > 2)) activeDrag.moved = true;
                win.style.left = newLeft + 'px';
                win.style.top = newTop + 'px';
                emitDesktopWindowEffect('drag', id);
                updateSnapZone(activeDrag, newLeft, newTop, win.offsetWidth, win.offsetHeight, win);
                scheduleFruityDockOcclusionCheck();
            });
        });
        bar.addEventListener('pointerup', () => {
            cancelWindowPointerFrame(drag, true);
            if (drag) emitDesktopWindowEffect('drag-end', id);
            if (drag && drag.snapZone) applyWindowSnap(win, drag.snapZone);
            else if (drag && drag.moved) {
                const item = state.windows.get(id);
                if (item) { item.snapped = ''; item.restoreBounds = null; }
            }
            hideSnapOverlay();
            drag = null;
            win.classList.remove('dragging');
            scheduleSessionPersist();
        });
        bar.addEventListener('pointercancel', () => {
            cancelWindowPointerFrame(drag);
            if (drag) emitDesktopWindowEffect('drag-end', id);
            hideSnapOverlay();
            drag = null;
            win.classList.remove('dragging');
        });
        if (win.dataset.windowId && state.windows.get(win.dataset.windowId) && state.windows.get(win.dataset.windowId).appId !== 'calculator') {
            bar.addEventListener('dblclick', event => {
                if (event.target.closest('button, .vd-window-menu-popover')) return;
                toggleMaximizeWindow(id);
            });
        }
        wireLongPress(bar, event => showWindowContextMenu(event, id));
        wireWindowTouchGestures(win, id);
        const winAppId = state.windows.get(id)?.appId;
        if (winAppId !== 'calculator') wireWindowResize(win, id);
    }

    const SNAP_EDGE = 4;
    const SNAP_TOP_SIDE_EDGE = 96;

    function updateSnapZone(drag, left, top, width, height, win) {
        const limits = windowLayoutLimits(win);
        const ww = limits.width;
        const wh = limits.height;
        const nearLeftEdge = left <= SNAP_TOP_SIDE_EDGE;
        const nearRightEdge = left + width >= ww - SNAP_TOP_SIDE_EDGE;
        const nearBottomEdge = top + height >= wh - SNAP_EDGE;
        let zone = null;
        if (top <= SNAP_EDGE && nearLeftEdge) zone = 'top-left';
        else if (top <= SNAP_EDGE && nearRightEdge) zone = 'top-right';
        else if (top <= SNAP_EDGE) zone = 'maximize';
        else if (nearBottomEdge && nearLeftEdge) zone = 'bottom-left';
        else if (nearBottomEdge && nearRightEdge) zone = 'bottom-right';
        else if (left <= SNAP_EDGE) zone = 'left-half';
        else if (left + width >= ww - SNAP_EDGE) zone = 'right-half';
        if (zone && !desktopWindowLayoutFits(desktopWindowLayoutRect(zone, ww, wh), limits)) zone = null;
        drag.snapZone = zone;
        showSnapOverlay(zone, ww, wh);
    }

    function showSnapOverlay(zone, ww, wh) {
        if (!zone) { hideSnapOverlay(); return; }
        let overlay = document.getElementById('vd-snap-overlay');
        if (!overlay) {
            overlay = document.createElement('div');
            overlay.id = 'vd-snap-overlay';
            overlay.className = 'vd-snap-overlay';
            $('vd-window-layer').appendChild(overlay);
        }
        overlay.hidden = false;
        const p = desktopWindowLayoutRect(zone, ww, wh);
        if (!p) { hideSnapOverlay(); return; }
        overlay.style.left = p.left + 'px';
        overlay.style.top = p.top + 'px';
        overlay.style.width = p.width + 'px';
        overlay.style.height = p.height + 'px';
        overlay.dataset.snapZone = zone;
    }

    function hideSnapOverlay() {
        const overlay = document.getElementById('vd-snap-overlay');
        if (overlay) overlay.hidden = true;
    }

    function applyWindowSnap(win, zone, options) {
        const windowId = win.dataset.windowId;
        const item = state.windows.get(windowId);
        if (!item || item.isGadget) return false;
        closeWindowLayoutPopupFor(windowId);
        const limits = windowLayoutLimits(win);
        const ww = limits.width, wh = limits.height;
        const p = desktopWindowLayoutRect(zone, ww, wh);
        if (!desktopWindowLayoutFits(p, limits)) return false;
        if (zone === 'maximize') {
            item.snapped = '';
            if (!item.maximized) toggleMaximizeWindow(windowId);
            return true;
        }
        if (!p) return false;
        animateWindowBounds(win, () => {
            desktopWindowRememberRestoreBounds(item, windowBounds(win));
            if (item.maximized) {
                item.maximized = false;
                win.classList.remove('maximized');
            }
            item.snapped = zone;
            win.style.left = p.left + 'px';
            win.style.top = p.top + 'px';
            win.style.width = p.width + 'px';
            win.style.height = p.height + 'px';
        });
        emitDesktopWindowEffect('snap', windowId);
        desktopSound('window.snap');
        scheduleFruityDockOcclusionCheck();
        scheduleSessionPersist();
        const partnerZone = desktopWindowSnapPartnerZone(zone);
        if (partnerZone && (!options || options.showPartnerChooser !== false)) showWindowSnapPartnerChooser(windowId, partnerZone);
        return true;
    }

    function restoreWindowFromSnap(id) {
        const item = state.windows.get(id);
        if (!item || !item.element || item.isGadget) return;
        closeWindowLayoutPopupFor(id);
        if (item.maximized) {
            toggleMaximizeWindow(id);
            return;
        }
        const bounds = item.restoreBounds;
        if (bounds) {
            animateWindowBounds(item.element, () => {
                item.element.classList.remove('maximized');
                item.element.style.left = bounds.left + 'px';
                item.element.style.top = bounds.top + 'px';
                item.element.style.width = bounds.width + 'px';
                item.element.style.height = bounds.height + 'px';
                item.maximized = false;
                item.snapped = '';
                item.restoreBounds = null;
            });
            emitDesktopWindowEffect('restore', id);
            scheduleFruityDockOcclusionCheck();
            scheduleSessionPersist();
            return;
        }
        minimizeWindow(id);
    }

    function handleWindowSnapShortcut(event) {
        if (isCompactViewport() || isEditableTarget(event.target)) return false;
        if (!event.metaKey || event.ctrlKey || event.altKey) return false;
        if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight' && event.key !== 'ArrowUp' && event.key !== 'ArrowDown') return false;
        const item = state.windows.get(state.activeWindowId);
        if (!item || !item.element || item.isGadget) return false;
        event.preventDefault();
        const zone = desktopWindowKeyboardLayout(item.snapped, event.key);
        if (zone === 'restore') restoreWindowFromSnap(item.id);
        else if (zone) applyWindowSnap(item.element, zone);
        return true;
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
                void closeWindow(id);
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
            width: Math.round(win.offsetWidth || parseFloat(win.style.width) || 0),
            height: Math.round(win.offsetHeight || parseFloat(win.style.height) || 0)
        };
    }

    function workspaceBoundsForWindow() {
        const layer = $('vd-window-layer');
        const taskbar = document.querySelector('.vd-taskbar');
        const width = (layer && layer.clientWidth) || window.innerWidth;
        const layerTop = layer ? layer.getBoundingClientRect().top : 0;
        let height = Math.min(
            (layer && layer.clientHeight) || window.innerHeight,
            Math.max(1, window.innerHeight - layerTop)
        );
        if (shellLayout() === 'taskbar' && taskbar) {
            height = Math.max(1, height - taskbar.offsetHeight);
        } else if (isCompactViewport() && window.visualViewport) {
            height = Math.min(height, Math.max(1, window.visualViewport.height - ((taskbar && taskbar.offsetHeight) || 0)));
        }
        return { width, height };
    }

    function toggleMaximizeWindow(id) {
        const item = state.windows.get(id);
        if (!item) return;
        closeWindowLayoutPopupFor(id);
        const win = item.element;
        desktopSound(item.maximized ? 'window.restore' : 'window.maximize');
        animateWindowBounds(win, () => {
            if (item.maximized) {
                const b = item.restoreBounds || { left: 80, top: 48, width: 820, height: 560 };
                win.classList.remove('maximized');
                win.style.left = b.left + 'px';
                win.style.top = b.top + 'px';
                win.style.width = b.width + 'px';
                win.style.height = b.height + 'px';
                item.maximized = false;
                item.snapped = '';
                item.restoreBounds = null;
            } else {
                desktopWindowRememberRestoreBounds(item, windowBounds(win));
                item.snapped = '';
                const bounds = windowLayoutLimits(win);
                const rect = desktopWindowLayoutRect('maximize', bounds.width, bounds.height);
                win.classList.add('maximized');
                win.style.left = rect.left + 'px';
                win.style.top = rect.top + 'px';
                win.style.width = rect.width + 'px';
                win.style.height = rect.height + 'px';
                item.maximized = true;
            }
        });
        emitDesktopWindowEffect(item.maximized ? 'maximize' : 'restore', id);
        focusWindow(id);
        renderWindowMenus(id);
        scheduleFruityDockOcclusionCheck();
        scheduleSessionPersist();
    }

    function wireWindowResize(win, id) {
        win.querySelectorAll('[data-resize]').forEach(handle => {
            let resize = null;
            handle.addEventListener('pointerdown', event => {
                const item = state.windows.get(id);
                if (window.useMobileDesktopMode && window.useMobileDesktopMode()) return;
                if (item && item.maximized) return;
                event.preventDefault();
                event.stopPropagation();
                focusWindow(id);
                resize = {
                    edge: handle.dataset.resize,
                    x: event.clientX,
                    y: event.clientY,
                    bounds: windowBounds(win),
                    style: { left: win.style.left, top: win.style.top, width: win.style.width, height: win.style.height },
                    snapped: item && item.snapped || '',
                    restoreBounds: item && item.restoreBounds ? { ...item.restoreBounds } : null,
                    raf: 0,
                    pendingFrame: null
                };
                handle.setPointerCapture(event.pointerId);
                win.classList.add('resizing');
                emitDesktopWindowEffect('resize-start', id);
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
                    emitDesktopWindowEffect('resize', id);
                });
            });
            handle.addEventListener('pointerup', event => {
                if (!resize) return;
                const activeResize = resize;
                handle.releasePointerCapture(event.pointerId);
                cancelWindowPointerFrame(resize, true);
                const item = state.windows.get(id), current = windowBounds(win), start = activeResize.bounds;
                if (item && (current.left !== start.left || current.top !== start.top || current.width !== start.width || current.height !== start.height)) { item.snapped = ''; item.restoreBounds = null; }
                emitDesktopWindowEffect('resize-end', id);
                resize = null;
                win.classList.remove('resizing');
                scheduleSessionPersist();
            });
            handle.addEventListener('pointercancel', event => {
                if (handle.hasPointerCapture && handle.hasPointerCapture(event.pointerId)) {
                    handle.releasePointerCapture(event.pointerId);
                }
                if (resize) {
                    const item = state.windows.get(id), start = resize;
                    Object.assign(win.style, start.style);
                    if (item) { item.snapped = start.snapped; item.restoreBounds = start.restoreBounds; }
                    scheduleFruityDockOcclusionCheck();
                }
                cancelWindowPointerFrame(resize);
                if (resize) emitDesktopWindowEffect('resize-end', id);
                resize = null;
                win.classList.remove('resizing');
            });
        });
    }

    function windowLayoutLimits(win) {
        const workspace = workspaceBoundsForWindow();
        win.style.setProperty('--vd-workspace-height', workspace.height + 'px');
        const minimum = appWindowMinSize(win.dataset.appId);
        const minWidth = Math.min(workspace.width, minimum.width || WINDOW_MIN_W);
        const minHeight = Math.min(workspace.height, minimum.height || WINDOW_MIN_H);
        const cssLimits = window.getComputedStyle ? window.getComputedStyle(win) : win.style;
        win.style.minWidth = minWidth + 'px';
        win.style.minHeight = minHeight + 'px';
        return { ...workspace, minWidth, minHeight,
            maxWidth: desktopWindowLayoutMaxLimit(cssLimits && cssLimits.maxWidth, workspace.width),
            maxHeight: desktopWindowLayoutMaxLimit(cssLimits && cssLimits.maxHeight, workspace.height) };
    }

    function applyResize(win, edge, start, dx, dy) {
        const limits = windowLayoutLimits(win);
        const axis = (origin, size, delta, backwards, forwards, minimum, available) => {
            const clamp = (value, low, high) => Math.max(low, Math.min(value, high));
            if (backwards) {
                const end = clamp(origin + size, minimum, available);
                const begin = clamp(origin + delta, 0, end - minimum);
                return [begin, end - begin];
            }
            const begin = clamp(origin, 0, available - minimum);
            return [begin, clamp(size + (forwards ? delta : 0), minimum, available - begin)];
        };
        const [left, width] = axis(start.left, start.width, dx, edge.includes('w'), edge.includes('e'), limits.minWidth, limits.width);
        const [top, height] = axis(start.top, start.height, dy, edge.includes('n'), edge.includes('s'), limits.minHeight, limits.height);
        let nextLeft = left;
        let nextTop = top;
        let nextWidth = width;
        let nextHeight = height;
        const aspect = appWindowAspect(win.dataset.appId);
        if (aspect) {
            // A window with a fixed shape (the terminal's tube monitor) follows the pointer on the axis the
            // handle drags and takes the other one from the ratio, so the picture stays 4:3 while the window
            // grows. The edge opposite the handle stays where it was.
            const horizontal = edge.includes('e') || edge.includes('w');
            const vertical = edge.includes('n') || edge.includes('s');
            const driveWidth = horizontal && (!vertical || Math.abs(dx) >= Math.abs(dy));
            const bounds = aspectUnitBounds(aspect, { width: limits.minWidth, height: limits.minHeight }, { width: limits.width, height: limits.height });
            const unit = driveWidth ? Math.round(width / aspect.width) : Math.round(height / aspect.height);
            const shaped = aspectWindowSize(aspect, Math.min(Math.max(unit, bounds.min), bounds.max));
            nextWidth = shaped.width;
            nextHeight = shaped.height;
            nextLeft = edge.includes('w') ? (start.left + start.width) - nextWidth : left;
            nextTop = edge.includes('n') ? (start.top + start.height) - nextHeight : top;
            nextLeft = Math.max(0, Math.min(nextLeft, limits.width - nextWidth));
            nextTop = Math.max(0, Math.min(nextTop, limits.height - nextHeight));
        }
        win.style.left = nextLeft + 'px';
        win.style.top = nextTop + 'px';
        win.style.width = nextWidth + 'px';
        win.style.height = nextHeight + 'px';
        scheduleFruityDockOcclusionCheck();
    }

    function clampWindowsAfterViewportResize() {
        const mobile = useMobileDesktopMode();
        let changed = false;
        state.windows.forEach(item => {
            const win = item && item.element;
            if (!win || !win.isConnected || item.isGadget) return;
            if (!mobile && win.classList.contains('vd-mobile-forced-maximized')) {
                win.classList.remove('vd-mobile-forced-maximized');
                if (!item.maximized) win.classList.remove('maximized');
            }
            // Maximized bounds are derived from the current workspace, while
            // restoreBounds retains the user's original desktop geometry.
            if (item.maximized || (mobile && win.classList.contains('vd-mobile-forced-maximized'))) {
                const bounds = windowLayoutLimits(win);
                if (item.maximized) {
                    win.style.width = bounds.width + 'px';
                    win.style.height = bounds.height + 'px';
                }
                return;
            }
            if (item.snapped) {
                const bounds = windowLayoutLimits(win);
                const rect = desktopWindowLayoutRect(item.snapped, bounds.width, bounds.height);
                if (desktopWindowLayoutFits(rect, bounds)) {
                    win.style.left = rect.left + 'px';
                    win.style.top = rect.top + 'px';
                    win.style.width = rect.width + 'px';
                    win.style.height = rect.height + 'px';
                    return;
                }
                item.snapped = '';
                if (!mobile) {
                    const before = `${win.style.left}|${win.style.top}|${win.style.width}|${win.style.height}`;
                    applyResize(win, 'se', item.restoreBounds || windowBounds(win), 0, 0);
                    changed ||= before !== `${win.style.left}|${win.style.top}|${win.style.width}|${win.style.height}`;
                }
                return;
            }
            if (mobile) return;
            const bounds = windowBounds(win);
            if (!bounds.width) bounds.width = parseFloat(win.style.width) || WINDOW_MIN_W;
            if (!bounds.height) bounds.height = parseFloat(win.style.height) || WINDOW_MIN_H;
            const before = `${win.style.left}|${win.style.top}|${win.style.width}|${win.style.height}`;
            applyResize(win, 'se', bounds, 0, 0);
            changed ||= before !== `${win.style.left}|${win.style.top}|${win.style.width}|${win.style.height}`;
        });
        if (changed) scheduleSessionPersist();
    }

    if (!state._windowViewportClampWired) {
        state._windowViewportClampWired = true;
        window.addEventListener('resize', () => window.requestAnimationFrame(clampWindowsAfterViewportResize));
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

    function focusWindow(id, options) {
        const win = state.windows.get(id);
        if (!win) return;
        if (!options || !options.fromShowDesktopRestore) clearShowDesktopPeek();

        const isMobileMode = window.useMobileDesktopMode && window.useMobileDesktopMode();

        // On mobile: Enforce single active window by minimizing the previous one
        if (isMobileMode && state.activeWindowId && state.activeWindowId !== id) {
            const previousWin = state.windows.get(state.activeWindowId);
            if (previousWin && !previousWin.minimized) {
                minimizeWindow(state.activeWindowId);
            }
        }

        if (state.z > 100000) normalizeWindowZIndexes();
        const wasHidden = win.element.style.display === 'none' || win.element.hidden;
        win.minimizeRevision = (win.minimizeRevision || 0) + 1;
        win.minimizing = false;
        win.element.classList.remove('vd-window-minimizing');
        win.element.style.display = '';
        assignWindowZ(win);
        state.activeWindowId = id;
        state.windows.forEach(item => item.element.classList.toggle('active', item.id === id));
        emitDesktopWindowEffect(wasHidden ? 'restore' : 'focus', id);
        if (wasHidden) {
            animateThen(win.element, 'vd-window-restoring', themeMotionMs('restore'));
            desktopSound('window.restore');
        }
        renderTaskbar();
        scheduleFruityDockOcclusionCheck();
        scheduleSessionPersist();
        // Demote the SIP phone gadget so normal windows stay above it unless
        // the gadget is configured as always-on-top.
        if (window.SipPhoneGadget && typeof window.SipPhoneGadget.blur === 'function') {
            window.SipPhoneGadget.blur();
        }
    }

    async function closeWindow(id) {
        const win = state.windows.get(id);
        if (!win) return 'skipped';
        if (win.isGadget) return 'skipped'; // gadgets are closed via their own context menu
        if (win.closing || win.checkingClose) return 'skipped';
        if (typeof win.beforeClose === 'function') {
            win.checkingClose = true;
            try { if (!await win.beforeClose()) { desktopSound('window.deny'); return 'cancelled'; } }
            catch (error) { console.warn('Window close was cancelled', error); desktopSound('window.deny'); return 'cancelled'; }
            finally { win.checkingClose = false; }
            if (state.windows.get(id) !== win || win.closing || win.checkingClose) return 'skipped';
        }
        closeWindowLayoutPopupFor(id);
        win.closing = true;
        emitDesktopWindowEffect('close', id);
        desktopSound('window.close');
        clearWindowMenus(id);
        if (state.activeWindowId === id) state.activeWindowId = '';
        renderTaskbar();
        animateThen(win.element, 'vd-window-closing', themeMotionMs('close'), () => {
            disposeAppWindow(win);
            win.element.remove();
            state.windows.delete(id);
            refreshDesktopWindowEffects();
            renderTaskbar();
            scheduleFruityDockOcclusionCheck();
            scheduleSessionPersist();
        });
        return 'accepted';
    }

    async function closeDesktopWindowBatch(ids) {
        return desktopCloseWindowBatch(ids, id => {
            const item = state.windows.get(id);
            return !!(item && !item.isGadget);
        }, closeWindow);
    }

    function closeContextMenu(immediate) {
        if (state.contextMenu) {
            const menu = state.contextMenu;
            state.contextMenu = null;
            if (immediate || menu.classList.contains('vd-context-menu-closing')) {
                menu.remove();
            } else {
                animateThen(menu, 'vd-context-menu-closing', themeMotionMs('menuClose'), () => menu.remove());
            }
        }
        if (state.contextMenuKeydown) {
            document.removeEventListener('keydown', state.contextMenuKeydown);
            state.contextMenuKeydown = null;
        }
    }
    function closeContextMenuOnEscape(event) {
        if (event.key === 'Escape') { closeContextMenu(); return; }
        const menu = state.contextMenu;
        if (!menu) return;
        const navKeys = ['ArrowDown', 'ArrowUp', 'ArrowLeft', 'ArrowRight', 'Home', 'End'];
        if (!navKeys.includes(event.key)) return;
        const items = [...menu.querySelectorAll('.vd-context-item:not(:disabled)')].filter(el => el.offsetParent !== null);
        if (!items.length) return;
        const idx = items.indexOf(document.activeElement);
        let next = null;
        if (event.key === 'ArrowDown') next = items[idx < 0 ? 0 : (idx + 1) % items.length];
        else if (event.key === 'ArrowUp') next = items[idx < 0 ? items.length - 1 : (idx - 1 + items.length) % items.length];
        else if (event.key === 'Home') next = items[0];
        else if (event.key === 'End') next = items[items.length - 1];
        else if (event.key === 'ArrowRight' && idx >= 0) {
            const sub = document.activeElement.closest('.vd-context-submenu');
            const pop = sub ? sub.querySelector(':scope > .vd-context-submenu-popover') : null;
            if (pop) next = pop.querySelector('.vd-context-item:not(:disabled)');
        } else if (event.key === 'ArrowLeft' && idx >= 0) {
            const pop = document.activeElement.closest('.vd-context-submenu-popover');
            if (pop) next = pop.closest('.vd-context-submenu').querySelector(':scope > .vd-context-item');
        }
        if (!next) return;
        event.preventDefault();
        next.focus({ preventScroll: true });
    }
    function registerWindowCleanup(windowId, cleanup) {
        if (!windowId || typeof cleanup !== 'function') return;
        const items = state.windowCleanups.get(windowId) || [];
        items.push(cleanup);
        state.windowCleanups.set(windowId, items);
    }
    const WINDOW_ALWAYS_ON_TOP_Z = 200000;

    function windowAlwaysOnTop(win) {
        return !!(win && win.alwaysOnTop && !win.isGadget);
    }

    function applyWindowAlwaysOnTopAttr(win) {
        if (!win || !win.element) return;
        win.element.dataset.alwaysOnTop = windowAlwaysOnTop(win) ? 'true' : 'false';
    }

    function assignWindowZ(win) {
        if (!win || !win.element) return;
        if (state.z > 100000) normalizeWindowZIndexes();
        const base = windowAlwaysOnTop(win) ? WINDOW_ALWAYS_ON_TOP_Z : 0;
        win.element.style.zIndex = String(base + (++state.z));
        applyWindowAlwaysOnTopAttr(win);
    }

    function toggleWindowAlwaysOnTop(id) {
        const win = state.windows.get(id);
        if (!win || win.isGadget) return;
        win.alwaysOnTop = !win.alwaysOnTop;
        applyWindowAlwaysOnTopAttr(win);
        assignWindowZ(win);
        if (typeof scheduleSessionPersist === 'function') scheduleSessionPersist();
    }

    function normalizeWindowZIndexes() {
        const wins = [...state.windows.values()].sort((a, b) => Number(a.element.style.zIndex || 0) - Number(b.element.style.zIndex || 0));
        const normal = wins.filter(win => !windowAlwaysOnTop(win));
        const onTop = wins.filter(win => windowAlwaysOnTop(win));
        let z = 0;
        normal.forEach(win => {
            z += 10;
            win.element.style.zIndex = String(z);
            applyWindowAlwaysOnTopAttr(win);
        });
        onTop.forEach(win => {
            z += 10;
            win.element.style.zIndex = String(WINDOW_ALWAYS_ON_TOP_Z + z);
            applyWindowAlwaysOnTopAttr(win);
        });
        state.z = z;
    }
