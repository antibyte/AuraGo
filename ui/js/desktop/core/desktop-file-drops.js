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
