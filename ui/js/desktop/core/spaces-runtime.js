    const SPACE_IDS = ['1', '2', '3'];
    const DEFAULT_SPACE_ID = '1';

    if (!state.activeSpaceId) state.activeSpaceId = DEFAULT_SPACE_ID;

    function spacesEnabled() {
        return !isCompactViewport();
    }

    function normalizeSpaceId(id) {
        const value = String(id || DEFAULT_SPACE_ID);
        return SPACE_IDS.includes(value) ? value : DEFAULT_SPACE_ID;
    }

    function windowSpaceId(win) {
        if (!win) return DEFAULT_SPACE_ID;
        if (win.spaceId) return normalizeSpaceId(win.spaceId);
        if (win.element && win.element.dataset.spaceId) return normalizeSpaceId(win.element.dataset.spaceId);
        return DEFAULT_SPACE_ID;
    }

    function assignWindowSpace(win, spaceId) {
        if (!win) return;
        const normalized = normalizeSpaceId(spaceId);
        win.spaceId = normalized;
        if (win.element) win.element.dataset.spaceId = normalized;
    }

    function isWindowOnActiveSpace(win) {
        if (!spacesEnabled()) return true;
        return windowSpaceId(win) === normalizeSpaceId(state.activeSpaceId);
    }

    function taskbarWindows() {
        return [...state.windows.values()].filter(win => win && !win.isGadget && isWindowOnActiveSpace(win));
    }

    function applySpaceVisibility() {
        state.windows.forEach(win => {
            if (!win || !win.element || win.isGadget) return;
            const onActive = isWindowOnActiveSpace(win);
            win.element.classList.toggle('vd-space-hidden', !onActive);
            if (onActive) win.element.removeAttribute('data-space-hidden');
            else win.element.setAttribute('data-space-hidden', 'true');
        });
    }

    function renderSpacePager() {
        let pager = document.getElementById('vd-space-pager');
        const system = document.querySelector('.vd-taskbar-system');
        if (!pager && system) {
            pager = document.createElement('div');
            pager.id = 'vd-space-pager';
            pager.className = 'vd-space-pager';
            pager.setAttribute('role', 'group');
            const notifyBtn = document.getElementById('vd-notification-button');
            if (notifyBtn) system.insertBefore(pager, notifyBtn);
            else system.prepend(pager);
        }
        if (!pager) return;
        pager.hidden = !spacesEnabled();
        pager.setAttribute('aria-label', t('desktop.space'));
        pager.innerHTML = SPACE_IDS.map(id => {
            const active = normalizeSpaceId(state.activeSpaceId) === id;
            const label = t('desktop.space_n').replace('{{n}}', id);
            return `<button type="button" class="vd-space-pager-btn${active ? ' active' : ''}" data-space-id="${esc(id)}" aria-pressed="${active ? 'true' : 'false'}" title="${esc(label)}"><span class="vd-space-pager-label">${esc(id)}</span></button>`;
        }).join('');
        if (pager.dataset.spacesWired === 'true') return;
        pager.dataset.spacesWired = 'true';
        pager.addEventListener('click', event => {
            const btn = event.target.closest('[data-space-id]');
            if (!btn) return;
            switchSpace(btn.dataset.spaceId);
        });
    }

    function switchSpace(id) {
        if (!spacesEnabled()) return;
        const next = normalizeSpaceId(id);
        if (next === normalizeSpaceId(state.activeSpaceId)) return;
        state.activeSpaceId = next;
        applySpaceVisibility();
        renderSpacePager();
        renderTaskbar();
        const visible = taskbarWindows().filter(win => win.element && win.element.style.display !== 'none');
        if (visible.length) {
            const top = visible.reduce((best, win) => {
                const z = parseInt(win.element.style.zIndex, 10) || 0;
                const bestZ = parseInt(best.element.style.zIndex, 10) || 0;
                return z >= bestZ ? win : best;
            });
            focusWindow(top.id);
        } else {
            state.activeWindowId = '';
            state.windows.forEach(item => {
                if (item.element) item.element.classList.remove('active');
            });
        }
        scheduleSessionPersist();
    }

    function cycleSpace(direction) {
        if (!spacesEnabled()) return;
        const ids = SPACE_IDS;
        const current = normalizeSpaceId(state.activeSpaceId);
        const idx = ids.indexOf(current);
        const next = ids[(idx + direction + ids.length) % ids.length];
        switchSpace(next);
    }

    function moveWindowToSpace(windowId, spaceId) {
        if (!spacesEnabled()) return;
        const win = state.windows.get(windowId);
        if (!win || win.isGadget) return;
        assignWindowSpace(win, spaceId);
        applySpaceVisibility();
        renderTaskbar();
        scheduleSessionPersist();
        if (normalizeSpaceId(spaceId) === normalizeSpaceId(state.activeSpaceId)) focusWindow(windowId);
    }

    function restoreActiveSpaceFromSnapshot(snapshot) {
        if (!snapshot || snapshot.version < 2 || !snapshot.activeSpaceId) return;
        state.activeSpaceId = normalizeSpaceId(snapshot.activeSpaceId);
    }

    function refreshSpacesForViewport() {
        renderSpacePager();
        if (!spacesEnabled()) state.activeSpaceId = DEFAULT_SPACE_ID;
        applySpaceVisibility();
        renderTaskbar();
    }

    function initSpacesRuntime() {
        renderSpacePager();
        applySpaceVisibility();
        if (!state._spacesViewportWired) {
            state._spacesViewportWired = true;
            window.addEventListener('resize', refreshSpacesForViewport);
            if (window.visualViewport) window.visualViewport.addEventListener('resize', refreshSpacesForViewport);
        }
    }

    function handleSpaceShortcut(event) {
        if (!spacesEnabled() || isEditableTarget(event.target)) return false;
        if (!event.ctrlKey || !event.altKey || event.metaKey) return false;
        if (event.key === 'ArrowLeft') {
            event.preventDefault();
            cycleSpace(-1);
            return true;
        }
        if (event.key === 'ArrowRight') {
            event.preventDefault();
            cycleSpace(1);
            return true;
        }
        return false;
    }
