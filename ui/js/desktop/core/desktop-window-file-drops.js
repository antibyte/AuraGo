    const DESKTOP_WINDOW_TEXT_EXTS = ['txt', 'log', 'md', 'json', 'yaml', 'yml', 'sh', 'py', 'go', 'js', 'mjs', 'ts', 'tsx', 'jsx', 'rs', 'css', 'html', 'htm', 'xml', 'ini', 'conf'];
    const DESKTOP_WINDOW_DROP_CAPABILITIES = {
        files: { multiple: true, accepts: () => true, effect: 'move' },
        pixel: { multiple: false, accepts: path => desktopWindowDropExtIn(path, ['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'svg', 'ico', 'tiff', 'tif', 'avif']), effect: 'copy' },
        viewer: { multiple: false, accepts: path => desktopWindowDropExtIn(path, ['md', 'pdf', 'docx', 'xlsx', 'xlsm', 'csv']), effect: 'copy' },
        'viewer-3d': { multiple: false, accepts: path => desktopWindowDropExtIn(path, ['stl']), effect: 'copy' },
        writer: { multiple: false, accepts: path => desktopWindowDropExtIn(path, ['docx', 'html', 'htm', 'md', 'txt']), effect: 'copy' },
        sheets: { multiple: false, accepts: path => desktopWindowDropExtIn(path, ['xlsx', 'xlsm', 'csv']), effect: 'copy' },
        zipper: { multiple: false, accepts: path => desktopWindowDropExtIn(path, ['zip']), effect: 'copy' },
        'code-studio': { multiple: false, accepts: path => desktopWindowDropExtIn(path, DESKTOP_WINDOW_TEXT_EXTS), effect: 'copy' },
        editor: { multiple: false, accepts: path => desktopWindowDropExtIn(path, DESKTOP_WINDOW_TEXT_EXTS), effect: 'copy' },
        'agent-chat': { multiple: false, accepts: path => !!desktopWindowDropPathInfo(path).name, effect: 'copy' }
    };

    function desktopWindowFileOps() {
        return window.AuraDesktopFileOps || null;
    }

    function desktopWindowHasDragPayload(event) {
        const ops = desktopWindowFileOps();
        if (ops && typeof ops.hasDragPayload === 'function') return ops.hasDragPayload(event);
        const types = Array.from((event && event.dataTransfer && event.dataTransfer.types) || []);
        return types.includes('application/x-aurago-desktop-files');
    }

    function desktopWindowReadDragPayload(event) {
        const ops = desktopWindowFileOps();
        if (ops && typeof ops.readDragPayload === 'function') return ops.readDragPayload(event);
        return null;
    }

    function desktopWindowDropPathInfo(path) {
        const ops = desktopWindowFileOps();
        if (ops && typeof ops.pathInfo === 'function') return ops.pathInfo(path);
        const cleanPath = normalizeDesktopPath(path);
        const name = cleanPath.split('/').filter(Boolean).pop() || '';
        const dot = name.lastIndexOf('.');
        return { path: cleanPath, name, ext: dot > 0 ? name.slice(dot + 1).toLowerCase() : '' };
    }

    function desktopWindowDropExtIn(path, extensions) {
        const ext = desktopWindowDropPathInfo(path).ext;
        return !!ext && extensions.includes(ext);
    }

    function desktopWindowAcceptedDropTarget(windowId, payload) {
        const win = state.windows.get(windowId);
        const appId = win && win.appId;
        const capability = appId && DESKTOP_WINDOW_DROP_CAPABILITIES[appId];
        if (!capability || !payload || !Array.isArray(payload.paths)) return null;
        const paths = payload.paths.map(normalizeDesktopPath).filter(Boolean);
        if (!paths.length) return null;
        if (capability.multiple !== true && paths.length !== 1) return null;
        const acceptedPaths = paths.filter(path => capability.accepts(path));
        if (acceptedPaths.length !== paths.length) return null;
        return {
            accepted: true,
            appId,
            effect: capability.effect || 'copy',
            path: acceptedPaths[0] || '',
            paths: acceptedPaths
        };
    }

    function desktopWindowDropTarget(windowId, payload) {
        const win = state.windows.get(windowId);
        const appId = win && win.appId;
        if (!appId) return null;
        const paths = payload && Array.isArray(payload.paths) ? payload.paths.map(normalizeDesktopPath).filter(Boolean) : [];
        if (!DESKTOP_WINDOW_DROP_CAPABILITIES[appId]) return {
            accepted: false,
            appId,
            effect: 'none',
            path: '',
            paths
        };
        return desktopWindowAcceptedDropTarget(windowId, payload) || {
            accepted: false,
            appId,
            effect: 'none',
            path: '',
            paths
        };
    }

    function clearDesktopFileWindowDropState(windowId) {
        const win = state.windows.get(windowId);
        if (!win || !win.element) return;
        win.element.classList.remove('vd-window-file-drop-target', 'vd-window-file-drop-reject');
    }

    async function openDesktopFileDropInWindow(windowId, target) {
        if (!target || !target.accepted) return false;
        const win = state.windows.get(windowId);
        if (!win) return false;
        const appId = target.appId;
        if (appId === 'files' && window.FileManager && typeof window.FileManager.dropDesktopFiles === 'function') {
            return window.FileManager.dropDesktopFiles(windowId, target.paths);
        }
        const path = normalizeDesktopPath(target.path || target.paths[0] || '');
        if (!path) return false;
        const entry = desktopWindowDropPathInfo(path);
        if (appId === 'agent-chat' && typeof applyChatLaunchContext === 'function') {
            applyChatLaunchContext(windowId, { chat_files: [{ path: entry.path, name: entry.name }], chat_source_app: 'desktop-drop' });
            return true;
        }
        updateWindowContext(windowId, { path });
        if (appId === 'code-studio' && window.CodeStudio && typeof window.CodeStudio.openFile === 'function') {
            await window.CodeStudio.openFile(path, true, windowId);
            return true;
        }
        const nextContext = Object.assign({}, win.context || {}, { path });
        if (appId === 'editor') nextContext.content = '';
        renderAppContent(windowId, appId, nextContext);
        return true;
    }

    function handleDesktopFileWindowDragOver(event) {
        if (!desktopWindowHasDragPayload(event)) return;
        if (event.defaultPrevented) {
            event.stopPropagation();
            return;
        }
        const payload = desktopWindowReadDragPayload(event);
        const target = desktopWindowDropTarget(event.currentTarget.dataset.windowId, payload);
        if (!target) return;
        event.preventDefault();
        event.stopPropagation();
        event.dataTransfer.dropEffect = target.effect;
        const win = state.windows.get(event.currentTarget.dataset.windowId);
        if (win && win.element) {
            win.element.classList.remove('vd-window-file-drop-target', 'vd-window-file-drop-reject');
            win.element.classList.add(target.accepted ? 'vd-window-file-drop-target' : 'vd-window-file-drop-reject');
        }
    }

    function handleDesktopFileWindowDragLeave(event) {
        if (event.currentTarget !== event.target && event.currentTarget.contains(event.relatedTarget)) return;
        clearDesktopFileWindowDropState(event.currentTarget.dataset.windowId);
    }

    async function handleDesktopFileWindowDrop(event) {
        if (!desktopWindowHasDragPayload(event)) return;
        if (event.defaultPrevented) {
            event.stopPropagation();
            clearDesktopFileWindowDropState(event.currentTarget.dataset.windowId);
            return;
        }
        const payload = desktopWindowReadDragPayload(event);
        const target = desktopWindowDropTarget(event.currentTarget.dataset.windowId, payload);
        if (!target) return;
        event.preventDefault();
        event.stopPropagation();
        clearDesktopFileWindowDropState(event.currentTarget.dataset.windowId);
        if (!target.accepted) return;
        try {
            await openDesktopFileDropInWindow(event.currentTarget.dataset.windowId, target);
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err.message || String(err) });
        }
    }

    function wireDesktopFileWindowDrop(windowId) {
        const win = state.windows.get(windowId);
        if (!win || !win.element || win.element.dataset.fileWindowDropBound === 'true') return;
        win.element.dataset.fileWindowDropBound = 'true';
        win.element.addEventListener('dragover', handleDesktopFileWindowDragOver);
        win.element.addEventListener('dragleave', handleDesktopFileWindowDragLeave);
        win.element.addEventListener('drop', handleDesktopFileWindowDrop);
    }
