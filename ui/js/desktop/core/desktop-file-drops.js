    const DESKTOP_FILE_DRAG_TYPE = 'application/x-aurago-desktop-files';

    function desktopDropJoinPath(base, name) {
        const left = String(base || '').replace(/\\/g, '/').replace(/\/+$/, '');
        const right = String(name || '').replace(/\\/g, '/').replace(/^\/+/, '');
        return left ? (right ? left + '/' + right : left) : right;
    }

    function desktopDropBaseName(path) {
        const parts = normalizeDesktopPath(path).split('/').filter(Boolean);
        return parts.pop() || '';
    }

    function desktopDropNameCandidate(name, index) {
        if (index <= 1) return name;
        const dot = name.lastIndexOf('.');
        if (dot > 0) return name.slice(0, dot) + ' (' + index + ')' + name.slice(dot);
        return name + ' (' + index + ')';
    }

    function dataTransferHasType(dataTransfer, type) {
        return Array.from((dataTransfer && dataTransfer.types) || []).includes(type);
    }

    function hasDesktopFileDragPayload(event) {
        return dataTransferHasType(event && event.dataTransfer, DESKTOP_FILE_DRAG_TYPE);
    }

    function desktopFileClipboard() {
        const clipboard = window.AuraDesktopFileClipboard;
        if (!clipboard || !Array.isArray(clipboard.paths) || clipboard.paths.length === 0) return null;
        const mode = clipboard.mode === 'copy' ? 'copy' : 'cut';
        const paths = clipboard.paths.map(normalizeDesktopPath).filter(Boolean);
        return paths.length ? { mode, paths } : null;
    }

    function setDesktopFileClipboard(mode, paths) {
        const cleanPaths = [...new Set((paths || []).map(normalizeDesktopPath).filter(Boolean))];
        if (!cleanPaths.length) {
            window.AuraDesktopFileClipboard = null;
            return;
        }
        window.AuraDesktopFileClipboard = { mode: mode === 'copy' ? 'copy' : 'cut', paths: cleanPaths };
        window.dispatchEvent(new CustomEvent('aurago:desktop-file-clipboard'));
    }

    function hasDesktopFileClipboard() {
        return !!desktopFileClipboard();
    }

    function desktopFileDragPayload(event) {
        const dataTransfer = event && event.dataTransfer;
        if (!dataTransfer || !hasDesktopFileDragPayload(event)) return null;
        try {
            const payload = JSON.parse(dataTransfer.getData(DESKTOP_FILE_DRAG_TYPE) || '{}');
            const paths = Array.isArray(payload.paths) ? payload.paths.map(normalizeDesktopPath).filter(Boolean) : [];
            return paths.length ? { paths } : null;
        } catch (_) {
            const path = normalizeDesktopPath(dataTransfer.getData('text/plain'));
            return path ? { paths: [path] } : null;
        }
    }

    async function uniqueDesktopDestination(src, existingNames) {
        const name = desktopDropBaseName(src) || 'item';
        for (let index = 1; index < 1000; index += 1) {
            const candidate = desktopDropNameCandidate(name, index);
            if (!existingNames.has(candidate.toLowerCase())) {
                existingNames.add(candidate.toLowerCase());
                return desktopDropJoinPath('Desktop', candidate);
            }
        }
        const fallback = name + ' ' + Date.now();
        existingNames.add(fallback.toLowerCase());
        return desktopDropJoinPath('Desktop', fallback);
    }

    async function uniqueDestinationInFolder(src, destBase, existingNames) {
        const name = desktopDropBaseName(src) || 'item';
        for (let index = 1; index < 1000; index += 1) {
            const candidate = desktopDropNameCandidate(name, index);
            if (!existingNames.has(candidate.toLowerCase())) {
                existingNames.add(candidate.toLowerCase());
                return desktopDropJoinPath(destBase, candidate);
            }
        }
        const fallback = name + ' ' + Date.now();
        existingNames.add(fallback.toLowerCase());
        return desktopDropJoinPath(destBase, fallback);
    }

    async function refreshAfterDesktopFileDrop() {
        await loadBootstrap();
        const active = state.windows.get(state.activeWindowId);
        if (active && active.appId === 'files') renderFiles(active.id, state.filesPath);
    }

    async function moveDraggedFilesToDesktop(paths, clientX, clientY) {
        const cleanPaths = [...new Set((paths || []).map(normalizeDesktopPath).filter(Boolean))];
        if (!cleanPaths.length) return;
        const body = await api('/api/desktop/files?path=' + encodeURIComponent('Desktop'));
        const entries = Array.isArray(body.files) ? body.files : [];
        const existingNames = new Set(entries.map(entry => String(entry.name || desktopDropBaseName(entry.path)).toLowerCase()));
        const basePos = clampDesktopIconPosition(clientX - 40, clientY - 44);
        let offset = 0;
        for (const src of cleanPaths) {
            if (src === 'Desktop') continue;
            let newPath = src;
            if (!src.toLowerCase().startsWith('desktop/')) {
                newPath = await uniqueDesktopDestination(src, existingNames);
                await api('/api/desktop/file', {
                    method: 'PATCH',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ old_path: src, new_path: newPath })
                });
            }
            const iconPos = clampDesktopIconPosition(basePos.x + offset, basePos.y + offset);
            saveIconPosition('desktop-entry-' + newPath, iconPos.x, iconPos.y);
            offset += 18;
        }
        await refreshAfterDesktopFileDrop();
    }

    async function pasteDesktopFileClipboard(destBase, options) {
        const clipboard = desktopFileClipboard();
        if (!clipboard) return;
        const targetBase = normalizeDesktopPath(destBase == null ? 'Desktop' : destBase);
        const body = await api('/api/desktop/files?path=' + encodeURIComponent(targetBase));
        const entries = Array.isArray(body.files) ? body.files : [];
        const existingNames = new Set(entries.map(entry => String(entry.name || desktopDropBaseName(entry.path)).toLowerCase()));
        const clientX = options && Number.isFinite(options.clientX) ? options.clientX : 48;
        const clientY = options && Number.isFinite(options.clientY) ? options.clientY : 48;
        const basePos = clampDesktopIconPosition(clientX - 40, clientY - 44);
        let offset = 0;
        for (const src of clipboard.paths) {
            const naturalPath = desktopDropJoinPath(targetBase, desktopDropBaseName(src) || 'item');
            if (clipboard.mode === 'cut' && naturalPath === src) continue;
            const newPath = await uniqueDestinationInFolder(src, targetBase, existingNames);
            if (newPath === src) continue;
            if (clipboard.mode === 'copy') {
                await api('/api/desktop/copy', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ source_path: src, dest_path: newPath })
                });
            } else {
                await api('/api/desktop/file', {
                    method: 'PATCH',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ old_path: src, new_path: newPath })
                });
            }
            if (targetBase.toLowerCase() === 'desktop') {
                const iconPos = clampDesktopIconPosition(basePos.x + offset, basePos.y + offset);
                saveIconPosition('desktop-entry-' + newPath, iconPos.x, iconPos.y);
                offset += 18;
            }
        }
        if (clipboard.mode === 'cut') window.AuraDesktopFileClipboard = null;
        await refreshAfterDesktopFileDrop();
    }

    function wireDesktopFileIconDrag(btn) {
        if (!btn || btn.dataset.desktopEntry !== 'true') return;
        btn.draggable = true;
        btn.addEventListener('dragstart', event => {
            const path = normalizeDesktopPath(btn.dataset.path || '');
            if (!path) return;
            event.dataTransfer.effectAllowed = 'move';
            event.dataTransfer.setData('text/plain', path);
            event.dataTransfer.setData(DESKTOP_FILE_DRAG_TYPE, JSON.stringify({ source: 'desktop', paths: [path] }));
            btn.classList.add('vd-dragging');
        });
        btn.addEventListener('dragend', () => btn.classList.remove('vd-dragging'));
    }

    function handleDesktopFileDragOver(event) {
        if (!hasDesktopFileDragPayload(event)) return;
        event.preventDefault();
        event.dataTransfer.dropEffect = 'move';
        const workspace = $('vd-workspace');
        if (workspace) workspace.classList.add('vd-desktop-file-drop-target');
    }

    function handleDesktopFileDragLeave(event) {
        if (event.currentTarget !== event.target) return;
        const workspace = $('vd-workspace');
        if (workspace) workspace.classList.remove('vd-desktop-file-drop-target');
    }

    async function handleDesktopFileDrop(event) {
        const payload = desktopFileDragPayload(event);
        if (!payload) return;
        event.preventDefault();
        event.stopPropagation();
        const workspace = $('vd-workspace');
        if (workspace) workspace.classList.remove('vd-desktop-file-drop-target');
        try {
            await moveDraggedFilesToDesktop(payload.paths, event.clientX, event.clientY);
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err.message });
        }
    }

    function wireDesktopFileDrops() {
        const workspace = $('vd-workspace');
        if (!workspace || workspace.dataset.fileDropBound === 'true') return;
        workspace.dataset.fileDropBound = 'true';
        workspace.addEventListener('dragover', handleDesktopFileDragOver);
        workspace.addEventListener('dragleave', handleDesktopFileDragLeave);
        workspace.addEventListener('drop', handleDesktopFileDrop);
    }

    window.AuraDesktopFileOps = {
        dragType: DESKTOP_FILE_DRAG_TYPE,
        setClipboard: setDesktopFileClipboard,
        getClipboard: desktopFileClipboard,
        hasClipboard: hasDesktopFileClipboard,
        paste: pasteDesktopFileClipboard
    };
