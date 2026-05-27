    // File operations
    async function createNewFile() {
        if (isReadonly()) return;
        let res = null;
        if (typeof createNewFileWithTemplate === 'function') {
            res = await createNewFileWithTemplate();
        } else {
            const name = await promptDialog(t('desktop.fm.new_file_prompt', 'File name'), 'new-file.txt');
            if (name) res = { name, content: '' };
        }
        if (!res) return;
        const path = joinPath(fm.currentPath, res.name);
        try {
            await api('/api/desktop/file', {
                method: 'PUT',
                body: JSON.stringify({ path, content: res.content })
            });
            refresh();
            showNotification({ type: 'success', message: res.name });
        } catch (err) {
            showNotification({ type: 'error', message: (err.message || String(err)) });
        }
    }

    async function createNewFolder() {
        if (isReadonly()) return;
        const name = await promptDialog(t('desktop.fm.new_folder_prompt', 'Folder name'), 'New Folder');
        if (!name) return;
        const path = joinPath(fm.currentPath, name);
        try {
            await api('/api/desktop/directory', {
                method: 'POST',
                body: JSON.stringify({ path })
            });
            refresh();
            showNotification({ type: 'success', message: name });
        } catch (err) {
            showNotification({ type: 'error', message: (err.message || String(err)) });
        }
    }

    function startRename(path) {
        if (isReadonly()) return;
        const file = fm.files.find(f => f.path === path);
        if (!file) return;
        fm.renamePath = path;
        renderAll();
        const input = fm.host.querySelector('[data-rename-input]');
        if (input) {
            input.focus();
            input.select();
        }
    }

    function finishRename(input) {
        if (!input || !fm.renamePath) return;
        const nextName = String(input.value || '').trim();
        const path = fm.renamePath;
        fm.renamePath = '';
        if (!nextName) {
            renderAll();
            return;
        }
        renamePath(path, nextName);
    }

    function cancelRename() {
        fm.renamePath = '';
        renderAll();
    }

    async function renamePath(path, newName) {
        if (isReadonly()) return;
        const file = fm.files.find(f => f.path === path);
        if (!file || newName === file.name || !newName.trim()) {
            renderAll();
            return;
        }
        const parent = parentPath(path);
        const nextPath = joinPath(parent, newName.trim());
        try {
            await api('/api/desktop/file', {
                method: 'PATCH',
                body: JSON.stringify({ old_path: path, new_path: nextPath })
            });
            pushToUndo({
                type: 'rename',
                items: [{ oldPath: path, newPath: nextPath }]
            });
            refresh();
        } catch (err) {
            showNotification({ type: 'error', message: (err.message || String(err)) });
            renderAll();
        }
    }

    async function deleteSelected() {
        if (isReadonly()) return;
        const selected = getSelectedFiles();
        if (!selected.length) return;
        let confirmed;
        if (selected.length === 1) {
            confirmed = await confirmDialog(t('desktop.fm.confirm_delete_single', 'Delete "{{name}}"?', { name: selected[0].name }), '');
        } else {
            confirmed = await confirmDialog(t('desktop.fm.confirm_delete', 'Delete {{count}} item(s)?', { count: selected.length }), '');
        }
        if (!confirmed) return;
        for (const file of selected) {
            try {
                await api('/api/desktop/file?path=' + encodeURIComponent(file.path), { method: 'DELETE' });
            } catch (err) {
                showNotification({ type: 'error', message: (err.message || String(err)) });
            }
        }
        clearSelection();
        refresh();
    }

    async function downloadFile(file) {
        if (!file || file.type !== 'file') return;
        if (fm.callbacks && typeof fm.callbacks.exportDesktopFile === 'function') {
            await fm.callbacks.exportDesktopFile({ path: file.path || '', name: file.name || '', url: file.web_path || '' });
            return;
        }
        if (file.web_path) {
            const a = document.createElement('a');
            a.href = file.web_path;
            a.download = file.name;
            document.body.appendChild(a);
            a.click();
            a.remove();
            return;
        }
        const a = document.createElement('a');
        a.href = '/api/desktop/download?path=' + encodeURIComponent(file.path || '');
        a.download = file.name;
        document.body.appendChild(a);
        a.click();
        a.remove();
    }

    async function uploadFiles() {
        if (isReadonly()) return;
        if (fm.callbacks && typeof fm.callbacks.importFilesFromHost === 'function') {
            const result = await fm.callbacks.importFilesFromHost({ path: fm.currentPath, multiple: true });
            if (result && !result.canceled) refresh();
            return;
        }
        const input = document.createElement('input');
        input.type = 'file';
        input.multiple = true;
        input.addEventListener('change', async () => {
            if (!input.files || !input.files.length) return;
            await uploadFileList(input.files);
        }, { once: true });
        input.click();
    }

    function uploadWithXHR(file, path, onProgress) {
        return new Promise((resolve, reject) => {
            const xhr = new XMLHttpRequest();
            const formData = new FormData();
            formData.append('file', file);
            formData.append('path', path);
            xhr.upload.addEventListener('progress', (e) => {
                if (e.lengthComputable) {
                    onProgress(Math.round((e.loaded / e.total) * 100));
                }
            });
            xhr.addEventListener('load', () => {
                if (xhr.status >= 200 && xhr.status < 300) {
                    resolve(xhr.response);
                } else {
                    let errMsg = t('desktop.fm.upload_error', 'Upload failed');
                    try {
                        const resp = JSON.parse(xhr.responseText);
                        errMsg = resp.error || resp.message || errMsg;
                    } catch (_) {
                        errMsg = xhr.statusText || errMsg;
                    }
                    reject(new Error(errMsg));
                }
            });
            xhr.addEventListener('error', () => reject(new Error(t('desktop.fm.upload_error', 'Upload failed'))));
            xhr.addEventListener('abort', () => reject(new Error(t('desktop.fm.upload_aborted', 'Upload aborted'))));
            xhr.open('POST', '/api/desktop/upload');
            xhr.send(formData);
        });
    }

    async function uploadFileList(files) {
        if (isReadonly()) return;
        const totalFiles = files.length;
        let completedFiles = 0;

        const overlay = document.createElement('div');
        overlay.className = 'fm-upload-overlay';
        overlay.innerHTML = `
            <div class="fm-upload-panel">
                <div class="fm-upload-title">${esc(t('desktop.fm.uploading', 'Uploading'))}</div>
                <div class="fm-upload-bar-bg"><div class="fm-upload-bar-fill" style="width:0%"></div></div>
                <div class="fm-upload-percent">0%</div>
                <div class="fm-upload-file"></div>
            </div>
        `;
        document.body.appendChild(overlay);

        const barFill = overlay.querySelector('.fm-upload-bar-fill');
        const percentEl = overlay.querySelector('.fm-upload-percent');
        const fileEl = overlay.querySelector('.fm-upload-file');

        const limit = maxFileSize();
        for (const file of Array.from(files)) {
            completedFiles++;
            if (limit > 0 && file.size > limit) {
                showNotification({ type: 'error', message: t('desktop.fm.upload_too_large', { name: file.name }) });
                continue;
            }
            fileEl.textContent = `${esc(file.name)} (${completedFiles}/${totalFiles})`;
            try {
                await uploadWithXHR(file, fm.currentPath, (pct) => {
                    barFill.style.width = pct + '%';
                    percentEl.textContent = pct + '%';
                });
            } catch (err) {
                showNotification({ type: 'error', message: file.name + ': ' + (err.message || String(err)) });
            }
        }
        overlay.remove();
        refresh();
    }

    // Properties dialog
    async function showProperties(file) {
        if (!file) return;
        const isDir = file.type === 'directory';
        let itemCount = '';
        if (isDir) {
            try {
                const result = await api('/api/desktop/files?path=' + encodeURIComponent(file.path));
                const count = Array.isArray(result.files) ? result.files.length : 0;
                itemCount = `<div class="fm-prop-row"><span class="fm-prop-label">${esc(t('desktop.fm.prop_items', 'Items'))}</span><span class="fm-prop-value">${esc(count)}</span></div>`;
            } catch (_) {}
        }
        const mimeRow = file.mime_type ? `<div class="fm-prop-row"><span class="fm-prop-label">${esc(t('desktop.fm.prop_mime', 'MIME Type'))}</span><span class="fm-prop-value">${esc(file.mime_type)}</span></div>` : '';
        const modeRow = file.mode ? `<div class="fm-prop-row"><span class="fm-prop-label">${esc(t('desktop.fm.prop_permissions', 'Permissions'))}</span><span class="fm-prop-value">${esc(file.mode)}</span></div>` : '';
        const createdRow = file.created ? `<div class="fm-prop-row"><span class="fm-prop-label">${esc(t('desktop.fm.prop_created', 'Created'))}</span><span class="fm-prop-value">${esc(formatDate(file.created))}</span></div>` : '';

        const overlay = document.createElement('div');
        overlay.className = 'fm-modal-overlay';
        const typeLabel = isDir ? t('desktop.fm.prop_folder', 'Folder') : t('desktop.fm.prop_file', 'File');
        overlay.innerHTML = `<div class="fm-modal fm-properties">
            <div class="fm-modal-title">${esc(t('desktop.fm.properties_title', 'Properties'))}</div>
            <div class="fm-prop-body">
                <div class="fm-prop-row"><span class="fm-prop-label">${esc(t('desktop.fm.prop_name', 'Name'))}</span><span class="fm-prop-value">${esc(file.name)}</span></div>
                <div class="fm-prop-row"><span class="fm-prop-label">${esc(t('desktop.fm.prop_type', 'Type'))}</span><span class="fm-prop-value">${esc(typeLabel)}</span></div>
                <div class="fm-prop-row"><span class="fm-prop-label">${esc(t('desktop.fm.prop_size', 'Size'))}</span><span class="fm-prop-value">${esc(isDir ? '\u2014' : fmtBytes(file.size))}</span></div>
                <div class="fm-prop-row"><span class="fm-prop-label">${esc(t('desktop.fm.prop_location', 'Location'))}</span><span class="fm-prop-value">${esc(parentPath(file.path) || '/')}</span></div>
                ${mimeRow}
                ${modeRow}
                <div class="fm-prop-row"><span class="fm-prop-label">${esc(t('desktop.fm.prop_modified', 'Modified'))}</span><span class="fm-prop-value">${esc(formatDate(file.modified))}</span></div>
                ${createdRow}
                ${itemCount}
            </div>
            <div class="fm-modal-actions">
                <button type="button" class="fm-btn primary" data-close>${esc(t('desktop.ok', 'OK'))}</button>
            </div>
        </div>`;
        document.body.appendChild(overlay);
        overlay.querySelector('[data-close]').addEventListener('click', () => overlay.remove());
        overlay.addEventListener('click', e => { if (e.target === overlay) overlay.remove(); });
    }

    // Drag and drop
    let dragSrcPath = null;

    function fileManagerDragPayload(path) {
        const paths = fm.selectedPaths.has(path) ? Array.from(fm.selectedPaths) : [path];
        return { source: 'file-manager', paths };
    }

    function fileManagerDragPayloadFromEvent(event) {
        const dataTransfer = event && event.dataTransfer;
        if (!dataTransfer || !Array.from(dataTransfer.types || []).includes(DESKTOP_FILE_DRAG_TYPE)) return null;
        try {
            const payload = JSON.parse(dataTransfer.getData(DESKTOP_FILE_DRAG_TYPE) || '{}');
            const paths = Array.isArray(payload.paths) ? payload.paths.filter(Boolean) : [];
            return paths.length ? { paths } : null;
        } catch (_) {
            const path = dataTransfer.getData('text/plain');
            return path ? { paths: [path] } : null;
        }
    }

    async function moveDroppedDesktopFilesToFolder(paths, destPath) {
        if (isReadonly()) return;
        const cleanPaths = Array.from(new Set((paths || []).filter(Boolean)));
        if (!cleanPaths.length) return;
        
        let progress = null;
        if (cleanPaths.length > 1) {
            progress = showProgressOverlay(t('desktop.fm.moving', 'Moving...'), cleanPaths.length);
        }
        
        let count = 0;
        const undoItems = [];
        
        for (const src of cleanPaths) {
            if (!src || src === destPath) continue;
            const name = baseName(src);
            const newPath = joinPath(destPath, name);
            if (newPath === src) continue;
            
            count++;
            if (progress) {
                progress.update(count, name);
            }
            
            try {
                await api('/api/desktop/file', {
                    method: 'PATCH',
                    body: JSON.stringify({ old_path: src, new_path: newPath })
                });
                undoItems.push({ oldPath: src, newPath: newPath });
            } catch (err) {
                showNotification({ type: 'error', message: (err.message || String(err)) });
            }
        }
        
        if (progress) {
            progress.close();
        }
        
        if (undoItems.length > 0) {
            pushToUndo({
                type: 'move',
                items: undoItems
            });
        }
        
        if (fm.callbacks && typeof fm.callbacks.refreshDesktop === 'function') await fm.callbacks.refreshDesktop();
        clearSelection();
        dragSrcPath = null;
        refresh();
    }

    function handleDragStart(e) {
        const path = e.currentTarget.dataset.path;
        dragSrcPath = path;
        if (!fm.selectedPaths.has(path)) {
            clearSelection();
            addSelection(path);
        }
        e.dataTransfer.effectAllowed = 'move';
        e.dataTransfer.setData('text/plain', path);
        e.dataTransfer.setData(DESKTOP_FILE_DRAG_TYPE, JSON.stringify(fileManagerDragPayload(path)));
    }

    function handleDragOver(e) {
        e.preventDefault();
        if (fileManagerDragPayloadFromEvent(e)) {
            e.dataTransfer.dropEffect = 'move';
            return;
        }
        if (e.dataTransfer.types.includes('Files')) {
            showDropOverlay();
        }
    }

    function handleDragLeave(e) {
        if (e.dataTransfer.types.includes('Files')) {
            hideDropOverlay();
        }
    }

    function showDropOverlay() {
        if (!fm.host) return;
        const overlay = fm.host.querySelector('[data-fm-drop-overlay]');
        if (overlay) overlay.classList.add('visible');
    }

    function hideDropOverlay() {
        if (!fm.host) return;
        const overlay = fm.host.querySelector('[data-fm-drop-overlay]');
        if (overlay) overlay.classList.remove('visible');
    }

    function handleDrop(e) {
        e.preventDefault();
        e.stopPropagation();
        hideDropOverlay();
        const payload = fileManagerDragPayloadFromEvent(e);
        if (payload) moveDroppedDesktopFilesToFolder(payload.paths, fm.currentPath);
    }

    function handleExternalDrop(e) {
        e.preventDefault();
        e.stopPropagation();
        hideDropOverlay();
        if (isReadonly()) return;
        const files = e.dataTransfer.files;
        if (files && files.length) {
            uploadFileList(files);
        }
    }

    function handleDragEnter(e) {
        const target = e.currentTarget;
        const type = target.dataset.type;
        const payload = fileManagerDragPayloadFromEvent(e);
        if (type === 'directory' && target.dataset.path !== dragSrcPath && (!payload || !payload.paths.includes(target.dataset.path))) {
            target.classList.add('drag-over');
        }
    }

    function handleDragLeaveItem(e) {
        e.currentTarget.classList.remove('drag-over');
    }

    function handleDragOverItem(e) {
        e.preventDefault();
        const target = e.currentTarget;
        const type = target.dataset.type;
        const payload = fileManagerDragPayloadFromEvent(e);
        if (payload && type !== 'directory') {
            e.dataTransfer.dropEffect = 'move';
            return;
        }
        if (type === 'directory' && target.dataset.path !== dragSrcPath && (!payload || !payload.paths.includes(target.dataset.path))) {
            e.dataTransfer.dropEffect = 'move';
        } else {
            e.dataTransfer.dropEffect = 'none';
        }
    }

    async function handleItemDrop(e) {
        e.preventDefault();
        e.stopPropagation();
        if (isReadonly()) return;
        const target = e.currentTarget;
        target.classList.remove('drag-over');
        const destPath = target.dataset.path;
        const destType = target.dataset.type;
        const payload = fileManagerDragPayloadFromEvent(e);
        if (destType !== 'directory') {
            if (payload) await moveDroppedDesktopFilesToFolder(payload.paths, fm.currentPath);
            return;
        }
        if (payload) {
            await moveDroppedDesktopFilesToFolder(payload.paths, destPath);
            dragSrcPath = null;
            return;
        }
        if (!dragSrcPath || dragSrcPath === destPath) return;
        const srcFile = fm.files.find(f => f.path === dragSrcPath);
        if (!srcFile) return;
        const pathsToMove = fm.selectedPaths.has(dragSrcPath) ? Array.from(fm.selectedPaths) : [dragSrcPath];
        await moveDroppedDesktopFilesToFolder(pathsToMove, destPath);
        dragSrcPath = null;
    }

    function handleBreadcrumbDragOver(e) {
        e.preventDefault();
        const target = e.currentTarget;
        const payload = fileManagerDragPayloadFromEvent(e);
        if (target.dataset.breadcrumbPath !== fm.currentPath && (!payload || !payload.paths.includes(target.dataset.breadcrumbPath))) {
            e.dataTransfer.dropEffect = 'move';
        } else {
            e.dataTransfer.dropEffect = 'none';
        }
    }

    function handleBreadcrumbDragEnter(e) {
        e.preventDefault();
        const target = e.currentTarget;
        const payload = fileManagerDragPayloadFromEvent(e);
        if (target.dataset.breadcrumbPath !== fm.currentPath && (!payload || !payload.paths.includes(target.dataset.breadcrumbPath))) {
            target.classList.add('drag-over');
        }
    }

    function handleBreadcrumbDragLeave(e) {
        e.currentTarget.classList.remove('drag-over');
    }

    async function handleBreadcrumbDrop(e) {
        e.preventDefault();
        e.stopPropagation();
        if (isReadonly()) return;
        const target = e.currentTarget;
        target.classList.remove('drag-over');
        const destPath = target.dataset.breadcrumbPath;
        if (destPath === undefined) return;

        const payload = fileManagerDragPayloadFromEvent(e);
        if (payload) {
            await moveDroppedDesktopFilesToFolder(payload.paths, destPath);
            dragSrcPath = null;
            return;
        }
        if (!dragSrcPath || dragSrcPath === destPath) return;

        const pathsToMove = fm.selectedPaths.has(dragSrcPath) ? Array.from(fm.selectedPaths) : [dragSrcPath];
        await moveDroppedDesktopFilesToFolder(pathsToMove, destPath);
        dragSrcPath = null;
    }

    function buildOpenWithSubmenu(file) {
        if (!file || file.type !== 'file') return [];
        const apps = [];
        
        const isText = isViewerFile(file.name) || String(file.name).endsWith('.txt') || String(file.name).endsWith('.log');
        const isImage = isImageFile(file.name);
        const isMedia = isMediaFile(file.name);
        const isDoc = isViewerFile(file.name);
        
        if (isText) {
            apps.push({ label: t('desktop.app_editor', 'Editor'), appId: 'editor' });
            apps.push({ label: t('desktop.app_code_studio', 'Code Studio'), appId: 'code-studio' });
            apps.push({ label: t('desktop.app_viewer', 'Viewer'), appId: 'viewer' });
        } else if (isImage) {
            apps.push({ label: t('desktop.app_gallery', 'Gallery'), appId: 'gallery' });
            apps.push({ label: t('desktop.app_viewer', 'Viewer'), appId: 'viewer' });
            apps.push({ label: t('desktop.app_code_studio', 'Code Studio'), appId: 'code-studio' });
        } else if (isMedia) {
            const ext = String(file.name || '').split('.').pop().toLowerCase();
            if (['mp3', 'wav', 'flac', 'ogg', 'm4a', 'opus'].includes(ext)) {
                apps.push({ label: t('desktop.app_music_player', 'Music Player'), appId: 'music-player' });
            }
            apps.push({ label: t('desktop.app_gallery', 'Gallery'), appId: 'gallery' });
            apps.push({ label: t('desktop.app_viewer', 'Viewer'), appId: 'viewer' });
        } else if (isDoc) {
            const ext = String(file.name || '').split('.').pop().toLowerCase();
            if (['docx', 'html', 'htm'].includes(ext)) {
                apps.push({ label: t('desktop.app_writer', 'Writer'), appId: 'writer' });
            }
            if (['xlsx', 'xlsm', 'csv'].includes(ext)) {
                apps.push({ label: t('desktop.app_sheets', 'Sheets'), appId: 'sheets' });
            }
            apps.push({ label: t('desktop.app_viewer', 'Viewer'), appId: 'viewer' });
        } else if (String(file.name || '').toLowerCase().endsWith('.zip')) {
            apps.push({ label: t('desktop.app_zipper', 'Zipper'), appId: 'zipper' });
        } else {
            apps.push({ label: t('desktop.app_viewer', 'Viewer'), appId: 'viewer' });
            apps.push({ label: t('desktop.app_code_studio', 'Code Studio'), appId: 'code-studio' });
        }
        
        return apps.map(app => ({
            label: app.label,
            action: 'open-with-' + app.appId,
            handler: () => {
                if (fm.callbacks && typeof fm.callbacks.openApp === 'function') {
                    fm.callbacks.openApp(app.appId, { path: file.path });
                }
            }
        }));
    }

    async function duplicateSelected() {
        if (isReadonly()) return;
        const selected = getSelectedFiles();
        if (!selected.length) return;
        for (const file of selected) {
            const parent = parentPath(file.path);
            const extIdx = file.name.lastIndexOf('.');
            let base, ext;
            if (extIdx > 0 && file.type === 'file') {
                base = file.name.slice(0, extIdx);
                ext = file.name.slice(extIdx);
            } else {
                base = file.name;
                ext = '';
            }
            const copySuffix = ' - ' + t('desktop.fm.copy', 'Copy');
            let newName = base + copySuffix + ext;
            let destPath = joinPath(parent, newName);
            let index = 2;
            while (fm.files.some(f => f.path === destPath)) {
                newName = base + copySuffix + ` (${index})` + ext;
                destPath = joinPath(parent, newName);
                index++;
            }
            try {
                await api('/api/desktop/copy', {
                    method: 'POST',
                    body: JSON.stringify({ source_path: file.path, dest_path: destPath })
                });
            } catch (err) {
                showNotification({ type: 'error', message: (err.message || String(err)) });
            }
        }
        refresh();
    }

    async function copyPathToClipboard(path) {
        let text = '';
        if (path) {
            text = path;
        } else {
            const selected = getSelectedFiles();
            if (selected.length > 0) {
                text = selected.map(f => f.path).join('\n');
            } else {
                text = fm.currentPath;
            }
        }
        if (!text) return;
        try {
            await navigator.clipboard.writeText(text);
            showNotification({ type: 'success', message: t('desktop.fm.path_copied', 'Path copied to clipboard') });
        } catch (err) {
            showNotification({ type: 'error', message: (err.message || String(err)) });
        }
    }

    function openTerminalHere(path) {
        if (fm.callbacks && typeof fm.callbacks.openApp === 'function') {
            fm.callbacks.openApp('terminal', { path });
        }
    }

    async function createSymlink(file) {
        if (isReadonly()) return;
        const defaultName = file.name + '_symlink';
        const linkName = await promptDialog(t('desktop.fm.create_symlink_prompt', 'Symlink name'), defaultName);
        if (!linkName) return;
        
        const linkPath = joinPath(fm.currentPath, linkName);
        
        try {
            await api('/api/desktop/symlink', {
                method: 'POST',
                body: JSON.stringify({
                    target_path: file.path,
                    link_path: linkPath
                })
            });
            showNotification({ type: 'success', message: t('desktop.fm.symlink_created', 'Symlink created successfully') });
            refresh();
        } catch (err) {
            showNotification({ type: 'error', message: err.message || String(err) });
        }
    }

    async function calculateFolderSize(path) {
        const span = fm.host ? fm.host.querySelector(`[data-preview-folder-size="${path.replace(/"/g, '\\"')}"]`) : null;
        if (!span) return;
        
        span.innerHTML = `<span style="color:var(--vd-muted);font-style:italic">${esc(t('desktop.fm.calculating', 'Calculating...'))}</span>`;
        try {
            const res = await api('/api/desktop/folder-size?path=' + encodeURIComponent(path));
            if (res && res.status === 'ok') {
                span.textContent = fmtBytes(res.size || 0);
                
                const file = fm.files.find(f => f.path === path);
                if (file) {
                    file.size = res.size;
                }
            } else {
                throw new Error();
            }
        } catch (err) {
            span.innerHTML = `<span style="color:red">${esc(t('desktop.fm.error', 'Error'))}</span>`;
        }
    }

    // Keyboard shortcuts
    function bindKeyboard() {
        if (fm.keyboardBound) return;
        fm.keyboardBound = true;
        document.addEventListener('keydown', handleGlobalKeyDown);
    }

    function activateKeyboardWindow() {
        fm.activeKeyboardWindow = fm.windowId;
    }

    function handleGlobalKeyDown(e) {
        if (!fm.host) return;
        const root = fm.host.querySelector('.file-manager');
        if (!root) return;
        if (fm.activeKeyboardWindow !== fm.windowId || !root.contains(document.activeElement)) return;

        // Tab and hidden file shortcuts (work even in input fields)
        if (e.ctrlKey || e.metaKey) {
            const keyLower = e.key.toLowerCase();
            if (keyLower === 't') {
                e.preventDefault();
                if (typeof createNewTab === 'function') createNewTab();
                return;
            }
            if (keyLower === 'w') {
                e.preventDefault();
                if (typeof closeTab === 'function') closeTab(fm.activeTabIndex);
                return;
            }
            if (keyLower === 'h') {
                e.preventDefault();
                fm.showHidden = !fm.showHidden;
                renderAll();
                return;
            }
            if (e.key === 'Tab') {
                e.preventDefault();
                if (typeof initTabs === 'function') initTabs(fm);
                if (fm.tabs && fm.tabs.length > 1) {
                    const nextIdx = e.shiftKey ? 
                        (fm.activeTabIndex - 1 + fm.tabs.length) % fm.tabs.length : 
                        (fm.activeTabIndex + 1) % fm.tabs.length;
                    if (typeof switchTab === 'function') switchTab(nextIdx);
                }
                return;
            }
        }

        if (e.altKey && e.key.toLowerCase() === 's') {
            e.preventDefault();
            if (typeof toggleSplitView === 'function') toggleSplitView();
            return;
        }

        const isInput = document.activeElement && (document.activeElement.tagName === 'INPUT' || document.activeElement.tagName === 'TEXTAREA');
        if (isInput && e.key !== 'Escape') return;

        if (!isInput) {
            if ((e.key === 'F10' && e.shiftKey) || e.key === 'ContextMenu') {
                const focused = document.activeElement;
                if (focused && focused.dataset && focused.dataset.path) {
                    e.preventDefault();
                    const rect = focused.getBoundingClientRect();
                    const path = focused.dataset.path;
                    const file = fm.files.find(f => f.path === path);
                    if (file) {
                        handleItemContextMenu({
                            preventDefault: () => {},
                            stopPropagation: () => {},
                            clientX: rect.left + rect.width / 2,
                            clientY: rect.top + rect.height / 2,
                            currentTarget: focused
                        });
                    }
                    return;
                } else {
                    const fmMain = root.querySelector('[data-fm-main]');
                    if (document.activeElement === fmMain || root.contains(document.activeElement)) {
                        e.preventDefault();
                        const rect = (fmMain || root).getBoundingClientRect();
                        handleEmptyContextMenu({
                            preventDefault: () => {},
                            clientX: rect.left + rect.width / 2,
                            clientY: rect.top + rect.height / 2
                        });
                        return;
                    }
                }
            }
            if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'd') {
                if (isReadonly()) return;
                e.preventDefault();
                duplicateSelected();
                return;
            }
            if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key.toLowerCase() === 'c') {
                e.preventDefault();
                copyPathToClipboard();
                return;
            }
        }

        if (e.key === 'Delete' && !isInput) {
            if (isReadonly()) return;
            e.preventDefault();
            deleteSelected();
            return;
        }
        if (e.key === 'F2' && !isInput) {
            if (isReadonly()) return;
            e.preventDefault();
            const selected = getSelectedFiles();
            if (selected.length === 1) startRename(selected[0].path);
            return;
        }
        if (e.key === 'Backspace' && !isInput) {
            e.preventDefault();
            goUp();
            return;
        }
        if (e.key === 'Escape') {
            if (fm.renamePath) {
                fm.renamePath = null;
                renderAll();
                return;
            }
            if (fm.searchQuery) {
                fm.searchQuery = '';
                applyFilter();
                renderAll();
                return;
            }
            if (fm.selectedPaths.size) {
                clearSelection();
                renderAll();
                return;
            }
            const searchBar = root.querySelector('[data-fm-search]');
            if (searchBar && !searchBar.hidden) {
                searchBar.hidden = true;
                fm.searchQuery = '';
                applyFilter();
                renderAll();
            }
            return;
        }
        if (e.key === 'Enter' && !isInput) {
            e.preventDefault();
            const selected = getSelectedFiles();
            if (selected.length === 1) {
                if (selected[0].type === 'directory') navigate(selected[0].path);
                else openFileEntry(selected[0]);
            }
            return;
        }
        if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'a' && !isInput) {
            e.preventDefault();
            selectAll();
            return;
        }
        if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'c' && !isInput) {
            e.preventDefault();
            copySelection();
            return;
        }
        if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'x' && !isInput) {
            if (isReadonly()) return;
            e.preventDefault();
            cutSelection();
            return;
        }
        if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'v' && !isInput) {
            if (isReadonly()) return;
            e.preventDefault();
            pasteClipboard();
            return;
        }
        if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'f' && !isInput) {
            e.preventDefault();
            toggleSearch();
            return;
        }
        if (e.key === '/' && !isInput) {
            e.preventDefault();
            toggleSearch();
            return;
        }
        if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'z' && !isInput) {
            e.preventDefault();
            undo();
            return;
        }
        if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'y' && !isInput) {
            e.preventDefault();
            redo();
            return;
        }
        if (e.key === ' ' && !isInput) {
            e.preventDefault();
            toggleQuickLook();
            return;
        }
        if (['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(e.key) && !isInput) {
            e.preventDefault();
            const files = getDisplayFiles();
            if (!files.length) return;
            
            let currentIndex = files.findIndex(f => f.path === (fm.lastClickedPath || ''));
            let newIndex = currentIndex;
            
            if (e.key === 'Home') {
                newIndex = 0;
            } else if (e.key === 'End') {
                newIndex = files.length - 1;
            } else if (e.key === 'ArrowUp') {
                if (fm.viewMode === 'grid') {
                    const cols = getGridColsCount();
                    newIndex = currentIndex === -1 ? 0 : Math.max(0, currentIndex - cols);
                } else {
                    newIndex = currentIndex === -1 ? 0 : Math.max(0, currentIndex - 1);
                }
            } else if (e.key === 'ArrowDown') {
                if (fm.viewMode === 'grid') {
                    const cols = getGridColsCount();
                    newIndex = currentIndex === -1 ? 0 : Math.min(files.length - 1, currentIndex + cols);
                } else {
                    newIndex = currentIndex === -1 ? 0 : Math.min(files.length - 1, currentIndex + 1);
                }
            } else if (e.key === 'ArrowLeft') {
                if (fm.viewMode === 'grid') {
                    newIndex = currentIndex === -1 ? 0 : Math.max(0, currentIndex - 1);
                }
            } else if (e.key === 'ArrowRight') {
                if (fm.viewMode === 'grid') {
                    newIndex = currentIndex === -1 ? 0 : Math.min(files.length - 1, currentIndex + 1);
                }
            }
            
            if (newIndex !== currentIndex && newIndex >= 0 && newIndex < files.length) {
                const targetPath = files[newIndex].path;
                if (e.shiftKey) {
                    if (!fm.selectionAnchorPath) {
                        fm.selectionAnchorPath = fm.lastClickedPath || files[0].path;
                    }
                    setExactRangeSelection(fm.selectionAnchorPath, targetPath);
                } else {
                    fm.selectionAnchorPath = null;
                    fm.selectedPaths.clear();
                    fm.selectedPaths.add(targetPath);
                }
                fm.lastClickedPath = targetPath;
                updateSelectionDOM();
                focusFileItem(targetPath);
                
                // Trigger preview panel update if it's open
                const previewPanel = fm.host ? fm.host.querySelector('.fm-preview-panel') : null;
                if (previewPanel && typeof renderPreviewPanelHtml === 'function') {
                    const container = previewPanel.querySelector('.fm-preview-body');
                    if (container) {
                        const nextPanel = document.createElement('div');
                        nextPanel.innerHTML = renderPreviewPanelHtml();
                        const nextBody = nextPanel.querySelector('.fm-preview-body');
                        if (nextBody) container.replaceWith(nextBody);
                    }
                }
            }
            return;
        }
    }
