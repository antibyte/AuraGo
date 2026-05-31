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

    function selectedDesktopCommandIcons(anchor) {
        const icons = selectedDesktopIcons();
        if (anchor && anchor.classList && anchor.classList.contains('selected')) return icons.length ? icons : [anchor];
        return anchor ? [anchor] : icons;
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

    function desktopDragItemsForIcon(anchor) {
        return selectedDesktopCommandIcons(anchor).map(icon => ({
            icon,
            id: icon.dataset.id || '',
            left: parseInt(icon.style.left, 10) || 0,
            top: parseInt(icon.style.top, 10) || 0,
            width: icon.offsetWidth || 90,
            height: icon.offsetHeight || 88
        })).filter(item => item.icon && item.id);
    }

    function desktopDragBounds(items) {
        return items.reduce((bounds, item) => ({
            left: Math.min(bounds.left, item.left),
            top: Math.min(bounds.top, item.top),
            right: Math.max(bounds.right, item.left + item.width),
            bottom: Math.max(bounds.bottom, item.top + item.height)
        }), { left: Infinity, top: Infinity, right: -Infinity, bottom: -Infinity });
    }

    function clampDesktopDragDelta(items, dx, dy) {
        if (!items || !items.length) return { dx: 0, dy: 0 };
        const workspace = $('vd-workspace');
        const bounds = desktopDragBounds(items);
        const maxW = (workspace && workspace.clientWidth) || window.innerWidth;
        const maxH = (workspace && workspace.clientHeight) || window.innerHeight;
        return {
            dx: Math.min(maxW - 8 - bounds.right, Math.max(8 - bounds.left, dx)),
            dy: Math.min(maxH - 8 - bounds.bottom, Math.max(8 - bounds.top, dy))
        };
    }

    function moveDesktopDragItems(items, dx, dy) {
        if (desktopIconGridEnabled()) {
            positionDesktopDragItemsOnGrid(items, item => ({ left: item.left + dx, top: item.top + dy }), false);
            return;
        }
        (items || []).forEach(item => {
            item.icon.style.left = Math.round(item.left + dx) + 'px';
            item.icon.style.top = Math.round(item.top + dy) + 'px';
        });
    }

    function saveDesktopDragItems(items) {
        if (desktopIconGridEnabled()) {
            snapDesktopDragItemsToGrid(items);
            return;
        }
        (items || []).forEach(item => saveIconPosition(item.id, parseInt(item.icon.style.left, 10) || 0, parseInt(item.icon.style.top, 10) || 0));
    }

    function snapDesktopDragItemsToGrid(items) {
        positionDesktopDragItemsOnGrid(items, item => desktopDragItemCurrentPosition(item), true);
    }

    function desktopDragItemCurrentPosition(item) {
        const left = parseInt(item.icon.style.left, 10);
        const top = parseInt(item.icon.style.top, 10);
        return {
            left: Number.isFinite(left) ? left : item.left || 0,
            top: Number.isFinite(top) ? top : item.top || 0
        };
    }

    function positionDesktopDragItemsOnGrid(items, positionForItem, persist) {
        const dragItems = (items || []).filter(item => item && item.icon && item.id);
        const draggedIds = new Set(dragItems.map(item => item.id));
        const usedCells = desktopIconGridUsedCells(draggedIds);
        dragItems.forEach(item => {
            const next = typeof positionForItem === 'function' ? positionForItem(item) : desktopDragItemCurrentPosition(item);
            const pos = desktopIconGridNearestFreePosition(next.left, next.top, usedCells);
            item.icon.style.left = pos.x + 'px';
            item.icon.style.top = pos.y + 'px';
            if (persist) saveIconPosition(item.id, pos.x, pos.y);
        });
    }

    function resetDesktopDragItems(items) {
        (items || []).forEach(item => {
            item.icon.style.left = item.left + 'px';
            item.icon.style.top = item.top + 'px';
        });
    }

    function suppressDesktopIconClicks(items) {
        (items || []).forEach(item => {
            item.icon.__vdSuppressNextClick = true;
            window.setTimeout(() => { item.icon.__vdSuppressNextClick = false; }, 0);
        });
    }

    function setDesktopDragItemsDragging(items, dragging) {
        (items || []).forEach(item => item.icon.classList.toggle('vd-dragging', dragging));
    }

    function desktopBatchPathIcons(anchor) {
        return selectedDesktopCommandIcons(anchor).filter(icon => icon.dataset && icon.dataset.path && (icon.dataset.kind === 'file' || icon.dataset.kind === 'directory'));
    }

    function desktopBatchPaths(anchor) {
        return desktopBatchPathIcons(anchor).map(icon => icon.dataset.path).filter(Boolean);
    }

    function desktopBatchFileEntries(anchor) {
        return desktopBatchPathIcons(anchor).filter(icon => icon.dataset.kind === 'file').map(icon => ({
            name: icon.querySelector('.vd-icon-label') ? icon.querySelector('.vd-icon-label').textContent : icon.dataset.path,
            path: icon.dataset.path || '',
            web_path: icon.dataset.webPath || '',
            media_kind: icon.dataset.mediaKind || '',
            mime_type: icon.dataset.mimeType || ''
        })).filter(entry => entry.path);
    }

    function desktopBatchShortcutIds(anchor) {
        return selectedDesktopCommandIcons(anchor)
            .filter(icon => icon.dataset && icon.dataset.desktopEntry !== 'true' && icon.dataset.id && !isTrashIcon(icon))
            .map(icon => icon.dataset.id);
    }

    function desktopBatchAppIds(anchor) {
        return [...new Set(selectedDesktopCommandIcons(anchor)
            .map(icon => icon.dataset && icon.dataset.appId || '')
            .filter(appId => appId && !isBuiltinApp(appId)))];
    }

    function activateDesktopItems(icons) {
        (icons || []).forEach(icon => activateDesktopItem(icon));
    }

    async function deleteDesktopPaths(paths) {
        const unique = [...new Set((paths || []).filter(Boolean))];
        if (!unique.length) return;
        if (settingBool('files.confirm_delete')) {
            const confirmed = await confirmDialog(t('desktop.confirm_delete'), t('desktop.confirm_delete_msg', { path: unique.join(', ') }));
            if (!confirmed) return;
        }
        try {
            for (const path of unique) await api('/api/desktop/file?path=' + encodeURIComponent(path), { method: 'DELETE' });
            await loadBootstrap();
            const active = state.windows.get(state.activeWindowId);
            if (active && active.appId === 'files') renderFiles(active.id, state.filesPath);
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err.message });
        }
    }

    async function removeDesktopShortcuts(ids) {
        const unique = [...new Set((ids || []).filter(Boolean))];
        if (!unique.length) return;
        try {
            for (const id of unique) {
                await api('/api/desktop/shortcuts?id=' + encodeURIComponent(id), { method: 'DELETE' });
                removeIconPosition(id);
            }
            await loadBootstrap();
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err.message });
        }
    }

    async function deleteDesktopApps(appIds) {
        const ids = [...new Set((appIds || []).filter(appId => appId && !isBuiltinApp(appId)))];
        if (!ids.length) return;
        const names = ids.map(appId => {
            const app = appById(appId);
            return app ? appName(app) : appId;
        }).join(', ');
        const confirmed = await confirmDialog(t('desktop.confirm_delete_app'), t('desktop.confirm_delete_app_msg', { name: names }));
        if (!confirmed) return;
        try {
            for (const appId of ids) {
                await api('/api/desktop/apps?id=' + encodeURIComponent(appId), { method: 'DELETE' });
                removeIconPosition('app-' + appId);
            }
            await loadBootstrap();
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err.message });
        }
    }

    async function handleTrashDropForIcons(icons) {
        for (const icon of icons || []) {
            if (icon && !isTrashIcon(icon)) await handleTrashDrop(icon);
        }
    }
