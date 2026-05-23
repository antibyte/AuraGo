    let desktopSelectionDrag = null;
    let desktopSelectionSuppressClick = false;

    function ensureDesktopIconSelectionState() {
        if (!state.selectedIconIds || typeof state.selectedIconIds.has !== 'function') {
            state.selectedIconIds = new Set(state.selectedIconId ? [state.selectedIconId] : []);
        }
    }

    function firstSelectedIconId() {
        ensureDesktopIconSelectionState();
        for (const id of state.selectedIconIds) return id;
        return '';
    }

    function selectedDesktopIcons() {
        ensureDesktopIconSelectionState();
        return [...document.querySelectorAll('.vd-icon.selected')];
    }

    function syncDesktopIconSelection() {
        ensureDesktopIconSelectionState();
        const present = new Set();
        document.querySelectorAll('.vd-icon').forEach(icon => {
            const id = icon.dataset.id || '';
            if (id) present.add(id);
            const selected = !!id && state.selectedIconIds.has(id);
            icon.classList.toggle('selected', selected);
            icon.setAttribute('aria-selected', selected ? 'true' : 'false');
        });
        [...state.selectedIconIds].forEach(id => {
            if (!present.has(id)) state.selectedIconIds.delete(id);
        });
        if (!state.selectedIconIds.has(state.selectedIconId)) state.selectedIconId = firstSelectedIconId();
    }

    function selectDesktopIcon(btn, options) {
        ensureDesktopIconSelectionState();
        options = options || {};
        const id = btn && btn.dataset ? btn.dataset.id || '' : '';
        if (!id) {
            state.selectedIconIds.clear();
            state.selectedIconId = '';
            syncDesktopIconSelection();
            return;
        }
        if (!options.extend) state.selectedIconIds.clear();
        if (options.toggle && state.selectedIconIds.has(id)) {
            state.selectedIconIds.delete(id);
            state.selectedIconId = firstSelectedIconId();
        } else {
            state.selectedIconIds.add(id);
            state.selectedIconId = id;
        }
        syncDesktopIconSelection();
    }

    function desktopSelectionRectFromPoints(startX, startY, currentX, currentY, workspaceRect) {
        const left = Math.min(startX, currentX);
        const top = Math.min(startY, currentY);
        const right = Math.max(startX, currentX);
        const bottom = Math.max(startY, currentY);
        return {
            left,
            top,
            right,
            bottom,
            width: right - left,
            height: bottom - top,
            localLeft: left - workspaceRect.left,
            localTop: top - workspaceRect.top
        };
    }

    function desktopIconIntersectsSelection(icon, rect) {
        const iconRect = icon.getBoundingClientRect();
        return iconRect.right >= rect.left && iconRect.left <= rect.right && iconRect.bottom >= rect.top && iconRect.top <= rect.bottom;
    }

    function ensureDesktopSelectionMarquee() {
        let marquee = document.querySelector('[data-selection-marquee]');
        if (marquee) return marquee;
        marquee = document.createElement('div');
        marquee.className = 'vd-desktop-selection-marquee';
        marquee.setAttribute('data-selection-marquee', 'true');
        $('vd-workspace').appendChild(marquee);
        return marquee;
    }

    function updateDesktopSelectionMarquee(rect) {
        const marquee = ensureDesktopSelectionMarquee();
        marquee.style.left = Math.round(rect.localLeft) + 'px';
        marquee.style.top = Math.round(rect.localTop) + 'px';
        marquee.style.width = Math.round(rect.width) + 'px';
        marquee.style.height = Math.round(rect.height) + 'px';
    }

    function removeDesktopSelectionMarquee() {
        const marquee = document.querySelector('[data-selection-marquee]');
        if (marquee) marquee.remove();
        document.body.classList.remove('vd-desktop-selecting');
    }

    function startDesktopSelectionDrag(event) {
        if (!event || event.defaultPrevented || event.button !== 0 || isTouchLikePointer(event)) return;
        if (event.target.closest('.vd-icon, .vd-widget, .vd-window, .vd-start-menu, .vd-taskbar')) return;
        const workspace = $('vd-workspace');
        if (!workspace) return;
        closeContextMenu();
        ensureDesktopIconSelectionState();
        desktopSelectionDrag = {
            pointerId: event.pointerId,
            startX: event.clientX,
            startY: event.clientY,
            currentX: event.clientX,
            currentY: event.clientY,
            moved: false,
            extend: event.ctrlKey || event.metaKey,
            baseSelection: new Set(state.selectedIconIds || [])
        };
        if (workspace.setPointerCapture) workspace.setPointerCapture(event.pointerId);
    }

    function updateDesktopSelectionDrag(event) {
        if (!desktopSelectionDrag || event.pointerId !== desktopSelectionDrag.pointerId) return;
        desktopSelectionDrag.currentX = event.clientX;
        desktopSelectionDrag.currentY = event.clientY;
        const dx = event.clientX - desktopSelectionDrag.startX;
        const dy = event.clientY - desktopSelectionDrag.startY;
        if (!desktopSelectionDrag.moved && Math.hypot(dx, dy) < DRAG_THRESHOLD) return;
        desktopSelectionDrag.moved = true;
        document.body.classList.add('vd-desktop-selecting');
        const workspace = $('vd-workspace');
        const rect = desktopSelectionRectFromPoints(desktopSelectionDrag.startX, desktopSelectionDrag.startY, event.clientX, event.clientY, workspace.getBoundingClientRect());
        updateDesktopSelectionMarquee(rect);
        const next = new Set(desktopSelectionDrag.extend ? desktopSelectionDrag.baseSelection : []);
        document.querySelectorAll('.vd-icon').forEach(icon => {
            if (desktopIconIntersectsSelection(icon, rect) && icon.dataset.id) next.add(icon.dataset.id);
        });
        state.selectedIconIds = next;
        state.selectedIconId = firstSelectedIconId();
        syncDesktopIconSelection();
        event.preventDefault();
    }

    function finishDesktopSelectionDrag(event) {
        if (!desktopSelectionDrag || (event && event.pointerId !== desktopSelectionDrag.pointerId)) return;
        const workspace = $('vd-workspace');
        if (event && workspace && workspace.hasPointerCapture && workspace.hasPointerCapture(desktopSelectionDrag.pointerId)) {
            workspace.releasePointerCapture(desktopSelectionDrag.pointerId);
        }
        if (desktopSelectionDrag.moved) {
            desktopSelectionSuppressClick = true;
            window.setTimeout(() => { desktopSelectionSuppressClick = false; }, 0);
            if (event) event.preventDefault();
        }
        removeDesktopSelectionMarquee();
        desktopSelectionDrag = null;
    }
