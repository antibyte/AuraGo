    function storageCreate(directory) {
        const instance = fm, destination = storageLocation();
        if (isReadonly()) return;
        const label = t(directory ? 'desktop.fm.new_folder_prompt' : 'desktop.fm.new_file_prompt');
        const defaultName = t(directory ? 'desktop.fm.new_folder' : 'desktop.new_file_default');
        promptDialog(label, defaultName).then(name => {
            if (!name || !isLiveInstance(instance)) return;
            storageOperation(instance, async signal => storageConflictRequest(instance, { ...destination, path: joinPath(destination.path, name) }, (loc, version) => directory
                ? storageJSON('/api/desktop/storage/directory', { method: 'POST', signal, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ location: loc }) })
                : storageJSON(storageURL('upload', loc), { method: 'POST', signal, headers: version ? { 'If-Match': version } : {}, body: '' }), signal));
        });
    }

    function storageRename(path, newName) {
        const instance = fm, source = storageLocation(path);
        if (isReadonly() || !newName.trim() || baseName(path) === newName.trim()) return;
        return storageOperation(instance, (signal, progress) => storageTransfer(instance, source, { ...source, path: joinPath(parentPath(path), newName.trim()) }, 'move', signal, progress));
    }

    async function storageDelete() {
        const instance = fm, selected = getSelectedFiles(), source = storageLocation();
        if (isReadonly() || !selected.length) return;
        const confirmed = await confirmDialog(t('desktop.fm.storage_delete', { count: selected.length, storage: storageName(instance) }), '');
        if (!confirmed || !isLiveInstance(instance)) return;
        return storageOperation(instance, async signal => {
            for (const file of selected) await storageJSON('/api/desktop/storage/entry', { method: 'DELETE', signal, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ location: { ...source, path: file.path }, version: file.version || '' }) });
        });
    }

    function storageUpload(files, instance, destination) {
        return storageOperation(instance, async (signal, progress) => {
            for (const file of Array.from(files)) {
                const limit = Number(instance.callbacks.maxFileSize || 0);
                if (limit && file.size > limit) throw new Error(t('desktop.fm.upload_too_large', { name: file.name }));
                progress(file.name);
                await storageConflictRequest(instance, { ...destination, path: joinPath(destination.path, file.name) }, (loc, version) => storageJSON(storageURL('upload', loc), { method: 'POST', signal, headers: version ? { 'If-Match': version } : {}, body: file }), signal);
            }
        });
    }

    function storageDuplicate() {
        const instance = fm, entries = getSelectedFiles().map(file => storageLocation(file.path));
        if (isReadonly()) return;
        return storageOperation(instance, async (signal, progress) => {
            for (const entry of entries) await storageTransfer(instance, entry, { ...entry, path: storageCopyPath(entry.path, 1) }, 'copy', signal, progress);
        });
    }

    function storageDownload(file) {
        const link = document.createElement('a');
        link.href = storageURL('content', storageLocation(file.path)) + '&download=1';
        link.download = file.name;
        document.body.appendChild(link); link.click(); link.remove();
    }

    function storageOpen(file) {
        fm.selectedPaths = new Set([file.path]);
        fm.previewOpen = true;
        renderAll();
        showNotification({ type: 'info', message: t('desktop.fm.storage_preview') });
    }

    function storagePaste(destBase) {
        const clipboard = window.AuraDesktopFileClipboard || sharedFileClipboard();
        if (!clipboard || isReadonly()) return;
        const entries = clipboard.entries || (clipboard.paths || []).map(localLocation);
        return storageTransferSelection(fm, entries, storageLocation(destBase == null ? fm.currentPath : destBase), clipboard.mode === 'cut' ? 'move' : 'copy', clipboard);
    }
