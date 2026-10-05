    // File operations
    async function createNewFile() {
        const instance = fm;
        const directory = instance.currentPath;
        if (isReadonly()) return;
        let res = null;
        if (typeof createNewFileWithTemplate === 'function') {
            res = await createNewFileWithTemplate();
        } else {
            const name = await promptDialog(t('desktop.fm.new_file_prompt'), t('desktop.new_file_default'));
            if (!isLiveInstance(instance)) return;
            if (name) res = { name, content: '' };
        }
        if (!res || !isLiveInstance(instance)) return;
        const path = joinPath(directory, res.name);
        try {
            await api('/api/desktop/file', {
                method: 'PUT',
                body: JSON.stringify({ path, content: res.content })
            });
            if (!isLiveInstance(instance)) return;
            withInstance(instance, () => { refresh(); showNotification({ type: 'success', message: res.name }); });
        } catch (err) {
            if (isLiveInstance(instance)) withInstance(instance, () => showNotification({ type: 'error', message: (err.message || String(err)) }));
        }
    }

    async function createNewFolder() {
        const instance = fm;
        const directory = instance.currentPath;
        if (isReadonly()) return;
        const name = await promptDialog(t('desktop.fm.new_folder_prompt'), t('desktop.fm.new_folder'));
        if (!name || !isLiveInstance(instance)) return;
        const path = joinPath(directory, name);
        try {
            await api('/api/desktop/directory', {
                method: 'POST',
                body: JSON.stringify({ path })
            });
            if (!isLiveInstance(instance)) return;
            withInstance(instance, () => { refresh(); showNotification({ type: 'success', message: name }); });
        } catch (err) {
            if (isLiveInstance(instance)) withInstance(instance, () => showNotification({ type: 'error', message: (err.message || String(err)) }));
        }
    }

    function startRename(path) {
        if (isReadonly()) return;
        const file = fm.files.find(f => f.path === path);
        if (!file) return;
        fm.renamePath = path;
        fm.renameDraft = file.name;
        renderAll();
        const input = fm.host.querySelector('[data-rename-input]');
        if (input) {
            input.focus();
            input.select();
        }
    }

    function finishRename(input) {
        if (!input || !input.isConnected || !fm.renamePath) return;
        const nextName = String(input.value || '').trim();
        const path = fm.renamePath;
        fm.renamePath = '';
        fm.renameDraft = null;
        if (!nextName) {
            renderAll();
            return;
        }
        renamePath(path, nextName);
    }

    function cancelRename() {
        fm.renamePath = '';
        fm.renameDraft = null;
        renderAll();
    }

    async function renamePath(path, newName) {
        const instance = fm;
        if (isReadonly()) return;
        const file = instance.files.find(f => f.path === path);
        if (!file || newName === file.name || !newName.trim()) {
            renderAll();
            return;
        }
        const parent = parentPath(path);
        const nextPath = joinPath(parent, newName.trim());
        try {
            const renamed = await api('/api/desktop/file', {
                method: 'PATCH',
                body: JSON.stringify({ old_path: path, new_path: nextPath })
            });
            if (!isLiveInstance(instance)) return;
            withInstance(instance, () => {
                pushToUndo({
                    type: 'rename',
                    items: [{ oldPath: path, newPath: renamed.path || nextPath }]
                });
                refresh();
            });
        } catch (err) {
            if (!isLiveInstance(instance)) return;
            withInstance(instance, () => {
                showNotification({ type: 'error', message: (err.message || String(err)) });
                renderAll();
            });
        }
    }

    async function restoreSelected() {
        const instance = fm;
        if (isReadonly()) return;
        const selected = getSelectedFiles().filter(file => isTrashItemPath(file && file.path));
        if (!selected.length) return;
        const restoreFromTrash = instance.callbacks && instance.callbacks.restoreFromTrash;
        if (typeof restoreFromTrash === 'function') {
            await restoreFromTrash(selected.map(file => file.path));
            if (isLiveInstance(instance)) withInstance(instance, () => { clearSelection(); refresh(); });
        }
    }

    async function deleteSelected() {
        const instance = fm;
        if (isReadonly()) return;
        const selected = getSelectedFiles();
        if (!selected.length) return;
        let confirmed;
        if (selected.length === 1) {
            confirmed = await confirmDialog(t('desktop.fm.confirm_delete_single', { name: selected[0].name }), '');
        } else {
            confirmed = await confirmDialog(t('desktop.fm.confirm_delete', { count: selected.length }), '');
        }
        if (!confirmed || !isLiveInstance(instance)) return;
        for (const file of selected) {
            try {
                await api('/api/desktop/file?path=' + encodeURIComponent(file.path), { method: 'DELETE' });
            } catch (err) {
                if (isLiveInstance(instance)) withInstance(instance, () => showNotification({ type: 'error', message: (err.message || String(err)) }));
            }
        }
        if (isLiveInstance(instance)) withInstance(instance, () => { clearSelection(); refresh(); });
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
        const instance = fm;
        const directory = instance.currentPath;
        if (isReadonly()) return;
        const importFilesFromHost = instance.callbacks && instance.callbacks.importFilesFromHost;
        if (typeof importFilesFromHost === 'function') {
            const result = await importFilesFromHost({ path: directory, multiple: true });
            if (result && !result.canceled && isLiveInstance(instance)) withInstance(instance, () => refresh());
            return;
        }
        const input = document.createElement('input');
        input.type = 'file';
        input.multiple = true;
        input.addEventListener('change', async () => {
            if (!input.files || !input.files.length || !isLiveInstance(instance)) return;
            await uploadFileList(input.files, instance, directory);
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
                    let errMsg = t('desktop.fm.upload_error');
                    try {
                        const resp = JSON.parse(xhr.responseText);
                        errMsg = resp.error || resp.message || errMsg;
                    } catch (_) {
                        errMsg = xhr.statusText || errMsg;
                    }
                    reject(new Error(errMsg));
                }
            });
            xhr.addEventListener('error', () => reject(new Error(t('desktop.fm.upload_error'))));
            xhr.addEventListener('abort', () => reject(new Error(t('desktop.fm.upload_aborted'))));
            xhr.open('POST', '/api/desktop/upload');
            xhr.send(formData);
        });
    }

    async function uploadFileList(files, instance = fm, directory = instance.currentPath) {
        const targetPath = directory;
        if (isReadonly()) return;
        const totalFiles = files.length;
        let completedFiles = 0;

        const overlay = document.createElement('div');
        overlay.className = 'fm-upload-overlay';
        overlay.innerHTML = `
            <div class="fm-upload-panel">
                <div class="fm-upload-title">${esc(t('desktop.fm.uploading'))}</div>
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
            if (!isLiveInstance(instance)) break;
            completedFiles++;
            if (limit > 0 && file.size > limit) {
                withInstance(instance, () => showNotification({ type: 'error', message: t('desktop.fm.upload_too_large', { name: file.name }) }));
                continue;
            }
            fileEl.textContent = `${esc(file.name)} (${completedFiles}/${totalFiles})`;
            try {
                await uploadWithXHR(file, targetPath, (pct) => {
                    if (!isLiveInstance(instance)) return;
                    barFill.style.width = pct + '%';
                    percentEl.textContent = pct + '%';
                });
            } catch (err) {
                if (isLiveInstance(instance)) withInstance(instance, () => showNotification({ type: 'error', message: file.name + ': ' + (err.message || String(err)) }));
            }
        }
        overlay.remove();
        if (isLiveInstance(instance)) withInstance(instance, () => refresh());
    }

    // Properties dialog
    async function showProperties(file) {
        const instance = fm;
        if (!file) return;
        const isDir = file.type === 'directory';
        let itemCount = '';
        if (isDir) {
            try {
                const result = await api('/api/desktop/files?path=' + encodeURIComponent(file.path));
                if (!isLiveInstance(instance)) return;
                const count = Array.isArray(result.files) ? result.files.length : 0;
                itemCount = `<div class="fm-prop-row"><span class="fm-prop-label">${esc(t('desktop.fm.prop_items'))}</span><span class="fm-prop-value">${esc(count)}</span></div>`;
            } catch (_) {}
        }
        const mimeRow = file.mime_type ? `<div class="fm-prop-row"><span class="fm-prop-label">${esc(t('desktop.fm.prop_mime'))}</span><span class="fm-prop-value">${esc(file.mime_type)}</span></div>` : '';
        const modeRow = file.mode ? `<div class="fm-prop-row"><span class="fm-prop-label">${esc(t('desktop.fm.prop_permissions'))}</span><span class="fm-prop-value">${esc(file.mode)}</span></div>` : '';
        const createdRow = file.created ? `<div class="fm-prop-row"><span class="fm-prop-label">${esc(t('desktop.fm.prop_created'))}</span><span class="fm-prop-value">${esc(formatDate(file.created))}</span></div>` : '';

        const overlay = document.createElement('div');
        overlay.className = 'fm-modal-overlay';
        const typeLabel = isDir ? t('desktop.fm.prop_folder') : t('desktop.fm.prop_file');
        overlay.innerHTML = `<div class="fm-modal fm-properties">
            <div class="fm-modal-title">${esc(t('desktop.fm.properties_title'))}</div>
            <div class="fm-prop-body">
                <div class="fm-prop-row"><span class="fm-prop-label">${esc(t('desktop.fm.prop_name'))}</span><span class="fm-prop-value">${esc(file.name)}</span></div>
                <div class="fm-prop-row"><span class="fm-prop-label">${esc(t('desktop.fm.prop_type'))}</span><span class="fm-prop-value">${esc(typeLabel)}</span></div>
                <div class="fm-prop-row"><span class="fm-prop-label">${esc(t('desktop.fm.prop_size'))}</span><span class="fm-prop-value">${esc(isDir ? '\u2014' : fmtBytes(file.size))}</span></div>
                <div class="fm-prop-row"><span class="fm-prop-label">${esc(t('desktop.fm.prop_location'))}</span><span class="fm-prop-value">${esc(parentPath(file.path) || '/')}</span></div>
                ${mimeRow}
                ${modeRow}
                <div class="fm-prop-row"><span class="fm-prop-label">${esc(t('desktop.fm.prop_modified'))}</span><span class="fm-prop-value">${esc(formatDate(file.modified))}</span></div>
                ${createdRow}
                ${itemCount}
            </div>
            <div class="fm-modal-actions">
                <button type="button" class="fm-btn primary" data-close>${esc(t('desktop.ok'))}</button>
            </div>
        </div>`;
        document.body.appendChild(overlay);
        overlay.querySelector('[data-close]').addEventListener('click', () => overlay.remove());
        overlay.addEventListener('click', e => { if (e.target === overlay) overlay.remove(); });
    }

    // Drag and drop

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
        const instance = fm;
        const targetPath = destPath;
        const sourcePaths = Array.from(new Set((paths || []).filter(Boolean)));
        if (isReadonly()) return;
        if (String(targetPath).toLowerCase() === 'trash') {
            const moveToTrash = instance.callbacks && instance.callbacks.moveToTrash;
            if (moveToTrash) await moveToTrash(sourcePaths);
            if (isLiveInstance(instance)) withInstance(instance, () => { clearSelection(); refresh(); });
            return;
        }
        if (isReadonly()) return;
        const cleanPaths = sourcePaths;
        if (!cleanPaths.length) return;
        
        let progress = null;
        if (cleanPaths.length > 1) {
            progress = showProgressOverlay(t('desktop.fm.moving'), cleanPaths.length);
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
                const moved = await api('/api/desktop/file', {
                    method: 'PATCH',
                    body: JSON.stringify({ old_path: src, new_path: newPath })
                });
                if (!isLiveInstance(instance)) { if (progress) progress.close(); return; }
                undoItems.push({ oldPath: src, newPath: moved.path || newPath });
            } catch (err) {
                if (!isLiveInstance(instance)) { if (progress) progress.close(); return; }
                withInstance(instance, () => showNotification({ type: 'error', message: (err.message || String(err)) }));
            }
        }
        
        if (progress) {
            progress.close();
        }
        
        if (!isLiveInstance(instance)) return;
        withInstance(instance, () => {
            if (undoItems.length > 0) pushToUndo({ type: 'move', items: undoItems });
            clearSelection();
            fm.dragSrcPath = null;
        });
        const refreshDesktop = instance.callbacks && instance.callbacks.refreshDesktop;
        if (typeof refreshDesktop === 'function') await refreshDesktop();
        if (isLiveInstance(instance)) withInstance(instance, () => refresh());
    }

    function handleDragStart(e) {
        const path = e.currentTarget.dataset.path;
        fm.dragSrcPath = path;
        if (!fm.selectedPaths.has(path)) {
            clearSelection();
            addSelection(path);
        }
        e.dataTransfer.effectAllowed = 'copyMove';
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
        const instance = fm;
        e.preventDefault();
        e.stopPropagation();
        hideDropOverlay();
        const payload = fileManagerDragPayloadFromEvent(e);
        if (payload) moveDroppedDesktopFilesToFolder(payload.paths, instance.currentPath);
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
        if (type === 'directory' && target.dataset.path !== fm.dragSrcPath && (!payload || !payload.paths.includes(target.dataset.path))) {
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
        if (type === 'directory' && target.dataset.path !== fm.dragSrcPath && (!payload || !payload.paths.includes(target.dataset.path))) {
            e.dataTransfer.dropEffect = 'move';
        } else {
            e.dataTransfer.dropEffect = 'none';
        }
    }

    async function handleItemDrop(e) {
        const instance = fm;
        e.preventDefault();
        e.stopPropagation();
        if (isReadonly()) return;
        const target = e.currentTarget;
        target.classList.remove('drag-over');
        const destPath = target.dataset.path;
        const destType = target.dataset.type;
        const payload = fileManagerDragPayloadFromEvent(e);
        if (destType !== 'directory') {
            if (payload) await moveDroppedDesktopFilesToFolder(payload.paths, instance.currentPath);
            return;
        }
        if (payload) {
            await moveDroppedDesktopFilesToFolder(payload.paths, destPath);
            instance.dragSrcPath = null;
            return;
        }
        if (!instance.dragSrcPath || instance.dragSrcPath === destPath) return;
        const srcFile = instance.files.find(f => f.path === instance.dragSrcPath);
        if (!srcFile) return;
        const pathsToMove = instance.selectedPaths.has(instance.dragSrcPath) ? Array.from(instance.selectedPaths) : [instance.dragSrcPath];
        await moveDroppedDesktopFilesToFolder(pathsToMove, destPath);
        instance.dragSrcPath = null;
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
        const instance = fm;
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
            instance.dragSrcPath = null;
            return;
        }
        if (!instance.dragSrcPath || instance.dragSrcPath === destPath) return;

        const pathsToMove = instance.selectedPaths.has(instance.dragSrcPath) ? Array.from(instance.selectedPaths) : [instance.dragSrcPath];
        await moveDroppedDesktopFilesToFolder(pathsToMove, destPath);
        instance.dragSrcPath = null;
    }

    function buildOpenWithSubmenu(file) {
        if (!file || file.type !== 'file') return [];
        const apps = [];
        
        const isText = isViewerFile(file.name) || String(file.name).endsWith('.txt') || String(file.name).endsWith('.log');
        const isImage = isImageFile(file.name);
        const isMedia = isMediaFile(file.name);
        const isDoc = isViewerFile(file.name);
        
        if (isText) {
            apps.push({ label: t('desktop.app_editor'), appId: 'editor' });
            apps.push({ label: t('desktop.app_code_studio'), appId: 'code-studio' });
            apps.push({ label: t('desktop.app_viewer'), appId: 'viewer' });
        } else if (isImage) {
            apps.push({ label: t('desktop.app_gallery'), appId: 'gallery' });
            apps.push({ label: t('desktop.app_viewer'), appId: 'viewer' });
            apps.push({ label: t('desktop.app_code_studio'), appId: 'code-studio' });
        } else if (isMedia) {
            const ext = String(file.name || '').split('.').pop().toLowerCase();
            if (['mp3', 'wav', 'flac', 'ogg', 'm4a', 'opus'].includes(ext)) {
                apps.push({ label: t('desktop.app_music_player'), appId: 'music-player' });
            }
            apps.push({ label: t('desktop.app_gallery'), appId: 'gallery' });
            apps.push({ label: t('desktop.app_viewer'), appId: 'viewer' });
        } else if (isDoc) {
            const ext = String(file.name || '').split('.').pop().toLowerCase();
            if (['docx', 'html', 'htm'].includes(ext)) {
                apps.push({ label: t('desktop.app_writer'), appId: 'writer' });
            }
            if (['xlsx', 'xlsm', 'csv'].includes(ext)) {
                apps.push({ label: t('desktop.app_sheets'), appId: 'sheets' });
            }
            apps.push({ label: t('desktop.app_viewer'), appId: 'viewer' });
        } else if (String(file.name || '').toLowerCase().endsWith('.zip')) {
            apps.push({ label: t('desktop.app_zipper'), appId: 'zipper' });
        } else {
            apps.push({ label: t('desktop.app_viewer'), appId: 'viewer' });
            apps.push({ label: t('desktop.app_code_studio'), appId: 'code-studio' });
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
        const instance = fm;
        if (isReadonly()) return;
        const selected = getSelectedFiles().map(file => Object.assign({}, file));
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
            const copySuffix = ' - ' + t('desktop.fm.copy');
            let newName = base + copySuffix + ext;
            let destPath = joinPath(parent, newName);
            let index = 2;
            while (instance.files.some(f => f.path === destPath)) {
                newName = base + copySuffix + ` (${index})` + ext;
                destPath = joinPath(parent, newName);
                index++;
            }
            try {
                await api('/api/desktop/copy', {
                    method: 'POST',
                    body: JSON.stringify({ source_path: file.path, dest_path: destPath })
                });
                if (!isLiveInstance(instance)) return;
            } catch (err) {
                if (isLiveInstance(instance)) withInstance(instance, () => showNotification({ type: 'error', message: (err.message || String(err)) }));
            }
        }
        if (isLiveInstance(instance)) withInstance(instance, () => refresh());
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
            showNotification({ type: 'success', message: t('desktop.fm.path_copied') });
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
        const instance = fm;
        const directory = instance.currentPath;
        if (isReadonly()) return;
        const defaultName = file.name + '_symlink';
        const linkName = await promptDialog(t('desktop.fm.create_symlink_prompt'), defaultName);
        if (!linkName || !isLiveInstance(instance)) return;
        
        const linkPath = joinPath(directory, linkName);
        
        try {
            await api('/api/desktop/symlink', {
                method: 'POST',
                body: JSON.stringify({
                    target_path: file.path,
                    link_path: linkPath
                })
            });
            if (isLiveInstance(instance)) withInstance(instance, () => { showNotification({ type: 'success', message: t('desktop.fm.symlink_created') }); refresh(); });
        } catch (err) {
            if (isLiveInstance(instance)) withInstance(instance, () => showNotification({ type: 'error', message: err.message || String(err) }));
        }
    }

    async function calculateFolderSize(path) {
        const instance = fm;
        const span = instance.host ? instance.host.querySelector(`[data-preview-folder-size="${path.replace(/"/g, '\\"')}"]`) : null;
        if (!span) return;
        
        span.innerHTML = `<span style="color:var(--vd-muted);font-style:italic">${esc(t('desktop.fm.calculating'))}</span>`;
        try {
            const res = await api('/api/desktop/folder-size?path=' + encodeURIComponent(path));
            if (!isLiveInstance(instance) || !span.isConnected) return;
            if (res && res.status === 'ok') {
                withInstance(instance, () => { span.textContent = fmtBytes(res.size || 0); });
                
                const file = instance.files.find(f => f.path === path);
                if (file) {
                    file.size = res.size;
                }
            } else {
                throw new Error();
            }
        } catch (err) {
            if (isLiveInstance(instance) && span.isConnected) span.innerHTML = `<span style="color:red">${esc(t('desktop.fm.error'))}</span>`;
        }
    }

    // Keyboard shortcuts
    function bindKeyboard() {
        if (fm.keyboardBound) return;
        fm.keyboardBound = true;
        document.addEventListener('keydown', handleGlobalKeyDown);
    }

    function activateKeyboardWindow(event) {
        const root = event && event.currentTarget;
        if (!root) return;
        for (const instance of instances.values()) {
            if (instance.host && instance.host.contains(root) && !instance.disposed) {
                setActiveInstance(instance);
                instance.activeKeyboardWindow = instance.windowId;
                return;
            }
        }
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
