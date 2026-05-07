        const selected = fm.selectedPaths.has(file.path) ? ' selected' : '';
        const cut = (fm.clipboard && fm.clipboard.mode === 'cut' && fm.clipboard.paths.includes(file.path)) ? ' cut-item' : '';
        const typeLabel = isDir ? t('desktop.fm.prop_folder', 'Folder') : (String(file.name || '').split('.').pop().toUpperCase() || t('desktop.fm.prop_file', 'File'));
        const nameContent = fm.renamePath === file.path
            ? `<input class="fm-rename-input" data-rename-input value="${esc(file.name)}" aria-label="${esc(t('desktop.fm.rename', 'Rename'))}">`
            : esc(file.name);
        return `<div class="fm-list-row${selected}${cut}" data-path="${esc(file.path)}" data-type="${esc(file.type)}" role="button" tabindex="0">
            <div class="fm-list-cell fm-col-name">
                <span class="fm-list-icon">${thumbnailMarkup(file, iconKey, isDir ? '\u25A0' : '\u25A1', 'list')}</span>
                <span class="fm-list-name">${nameContent}</span>
            </div>
            <div class="fm-list-cell fm-col-size">${isDir ? '\u2014' : esc(fmtBytes(file.size))}</div>
            <div class="fm-list-cell fm-col-date">${esc(formatDate(file.modified))}</div>
            <div class="fm-list-cell fm-col-type">${esc(typeLabel)}</div>
        </div>`;
    }

    function renderStatusBarHtml() {
        const files = getDisplayFiles();
        const selectedCount = fm.selectedPaths.size;
        const totalItems = files.length;
        const selectedSize = selectedCount > 0 ? ' (' + fmtBytes(getSelectedSize()) + ')' : '';
        return `<div class="fm-statusbar">
            <div class="fm-status-left">
                <span>${esc(t('desktop.fm.items', '{{count}} items', { count: totalItems }))}</span>
                ${selectedCount > 0 ? `<span class="fm-status-sep">|</span><span>${esc(t('desktop.fm.selected', '{{count}} selected', { count: selectedCount }))}${esc(selectedSize)}</span>` : ''}
            </div>
            <div class="fm-status-right">
                <span>${esc(fm.viewMode === 'grid' ? t('desktop.fm.view_grid', 'Grid View') : t('desktop.fm.view_list', 'List View'))}</span>
                <span class="fm-status-sep">|</span>
                <span>${esc(t('desktop.fm.sort_name', 'Name'))}: ${esc(fm.sortAsc ? t('desktop.fm.sort_asc', 'Ascending') : t('desktop.fm.sort_desc', 'Descending'))}</span>
            </div>
        </div>`;
    }

    function renderDropOverlayHtml() {
        if (isReadonly()) return '';
        return `<div class="fm-drop-overlay" data-fm-drop-overlay>
            <div class="fm-drop-message">
                ${iconMarkup('upload', '\u2191', 'fm-drop-icon', 48)}
                <div>${esc(t('desktop.fm.drop_here', 'Drop files here to upload'))}</div>
            </div>
        </div>`;
    }

    function attachEvents() {
        if (!fm.host) return;
        const root = fm.host.querySelector('.file-manager');
        if (!root) return;

        root.addEventListener('focusin', activateKeyboardWindow);
        root.addEventListener('pointerdown', activateKeyboardWindow);

        // Toolbar buttons
        root.querySelectorAll('[data-action]').forEach(btn => {
            btn.addEventListener('click', handleActionClick);
        });

        // Breadcrumb segments
        root.querySelectorAll('[data-breadcrumb-path]').forEach(seg => {
            seg.addEventListener('click', () => navigate(seg.dataset.breadcrumbPath));
            seg.addEventListener('keydown', e => { if (e.key === 'Enter') navigate(seg.dataset.breadcrumbPath); });
        });

        // Sidebar items
        root.querySelectorAll('[data-sidebar-path]').forEach(item => {
            item.addEventListener('click', () => {
                fm.sidebarOpen = false;
                navigate(item.dataset.sidebarPath);
            });
            item.addEventListener('keydown', e => {
                if (e.key === 'Enter') {
                    fm.sidebarOpen = false;
                    navigate(item.dataset.sidebarPath);
                }
            });
        });

        // Search input
        const searchInput = root.querySelector('.fm-search-input');
        if (searchInput) {
            searchInput.addEventListener('input', e => {
                fm.searchQuery = e.target.value;
                applyFilter();
                renderFileContent();
            });
            searchInput.addEventListener('keydown', e => {
                if (e.key === 'Escape') { fm.searchQuery = ''; applyFilter(); renderAll(); }
            });
        }

        attachFileItemEvents(root);
        const renameInput = fm.host.querySelector('[data-rename-input]');
        if (renameInput) {
            renameInput.addEventListener('click', event => event.stopPropagation());
            renameInput.addEventListener('keydown', event => {
                event.stopPropagation();
                if (event.key === 'Enter') {
                    event.preventDefault();
                    finishRename(renameInput);
                }
                if (event.key === 'Escape') {
                    event.preventDefault();
                    cancelRename();
                }
            });
            renameInput.addEventListener('blur', () => finishRename(renameInput));
        }
        attachMainAreaEvents(root, true);
        attachThumbnailEvents(root);
    }

    function attachThumbnailEvents(root) {
        root.querySelectorAll('img[data-fm-thumb], .fm-thumb img').forEach(img => {
            img.addEventListener('error', () => {
                const thumb = img.closest('.fm-thumb');
                if (thumb) thumb.classList.add('failed');
            }, { once: true });
        });
    }

    function attachFileItemEvents(root) {
        root.querySelectorAll('[data-path]').forEach(item => {
            if (item.dataset.fmBound === 'true') return;
            item.dataset.fmBound = 'true';
            item.addEventListener('click', handleItemClick);
            item.addEventListener('dblclick', handleItemDblClick);
            item.addEventListener('contextmenu', handleItemContextMenu);
            wireLongPress(item, handleItemContextMenu);
            item.addEventListener('keydown', handleItemKeyDown);
            item.draggable = true;
            item.addEventListener('dragstart', handleDragStart);
            item.addEventListener('dragenter', handleDragEnter);
            item.addEventListener('dragleave', handleDragLeaveItem);
            item.addEventListener('dragover', handleDragOverItem);
            item.addEventListener('drop', handleItemDrop);
        });
    }

    function attachMainAreaEvents(root, includePersistentDropOverlay) {
        root.querySelectorAll('[data-sort]').forEach(header => {
            header.addEventListener('click', () => {
                const sortKey = header.dataset.sort;
                if (fm.sortBy === sortKey) fm.sortAsc = !fm.sortAsc;
                else { fm.sortBy = sortKey; fm.sortAsc = true; }
                savePreferences();
                renderFileContent();
            });
        });

        // Click empty space to deselect
        const main = root.querySelector('[data-fm-main]');
        if (main) {
            main.addEventListener('click', e => {
                if (e.target === main || e.target.classList.contains('fm-empty') || e.target.classList.contains('fm-grid') || e.target.classList.contains('fm-list-body')) {
                    clearSelection();
                    renderFileContent();
                }
            });
            main.addEventListener('contextmenu', handleEmptyContextMenu);
        }

        // Drag and drop on content area
        if (main) {
            main.addEventListener('dragover', handleDragOver);
            main.addEventListener('dragleave', handleDragLeave);
            main.addEventListener('drop', handleDrop);
        }

        if (includePersistentDropOverlay) {
            const dropOverlay = root.querySelector('[data-fm-drop-overlay]');
            if (dropOverlay) {
                dropOverlay.addEventListener('dragover', e => { e.preventDefault(); e.stopPropagation(); });
                dropOverlay.addEventListener('dragleave', e => { hideDropOverlay(); });
                dropOverlay.addEventListener('drop', handleExternalDrop);
            }
        }
    }

    function handleActionClick(e) {
        const action = e.currentTarget.dataset.action;
        if (action === 'sidebar-toggle') handleSidebarToggle();
        else if (action === 'back') goBack();
        else if (action === 'forward') goForward();
        else if (action === 'up') goUp();
        else if (action === 'view-grid') { fm.viewMode = 'grid'; savePreferences(); renderAll(); }
        else if (action === 'view-list') { fm.viewMode = 'list'; savePreferences(); renderAll(); }
        else if (action === 'search-toggle') { toggleSearch(); }
        else if (action === 'search-clear') { fm.searchQuery = ''; applyFilter(); renderAll(); }
        else if (action === 'refresh') refresh();
        else if (action === 'upload') uploadFiles();
        else if (action === 'new-file') createNewFile();
        else if (action === 'new-folder') createNewFolder();
        else if (action === 'sort-menu') showSortMenu(e);
    }

    function handleSidebarToggle() {
        fm.sidebarOpen = !fm.sidebarOpen;
        renderAll();
    }

    function readonlyGuardItems(items) {
        if (!isReadonly()) return items;
        const blocked = new Set(['cut', 'paste', 'rename', 'delete', 'new-file', 'new-folder']);
        return items.map(item => item.separator || !blocked.has(item.action) ? item : Object.assign({}, item, { disabled: true, handler: () => {} }));
    }

    function toggleSearch() {
        const searchBar = fm.host.querySelector('[data-fm-search]');
        if (!searchBar) return;
        if (searchBar.hidden) {
            searchBar.hidden = false;
            const input = searchBar.querySelector('.fm-search-input');
            if (input) input.focus();
        } else {
            searchBar.hidden = true;
            fm.searchQuery = '';
            applyFilter();
            renderAll();
        }
    }

    function showSortMenu(e) {
        const items = [
            { label: t('desktop.fm.sort_name', 'Name'), action: 'sort-name', handler: () => { fm.sortBy = 'name'; savePreferences(); renderFileContent(); } },
            { label: t('desktop.fm.sort_size', 'Size'), action: 'sort-size', handler: () => { fm.sortBy = 'size'; savePreferences(); renderFileContent(); } },
            { label: t('desktop.fm.sort_date', 'Date Modified'), action: 'sort-date', handler: () => { fm.sortBy = 'date'; savePreferences(); renderFileContent(); } },
            { label: t('desktop.fm.sort_type', 'Type'), action: 'sort-type', handler: () => { fm.sortBy = 'type'; savePreferences(); renderFileContent(); } },
            { separator: true },
            { label: t('desktop.fm.sort_asc', 'Ascending'), action: 'sort-asc', handler: () => { fm.sortAsc = true; savePreferences(); renderFileContent(); } },
            { label: t('desktop.fm.sort_desc', 'Descending'), action: 'sort-desc', handler: () => { fm.sortAsc = false; savePreferences(); renderFileContent(); } },
        ];
        const rect = e.currentTarget.getBoundingClientRect();
        showContextMenu(rect.left, rect.bottom + 4, items);
    }

    function handleItemClick(e) {
        const path = e.currentTarget.dataset.path;
        const type = e.currentTarget.dataset.type;
        if (isTouchLikePointer(e)) {
            e.preventDefault();
            openFileItem(path, type);
            return;
        }
        if (e.ctrlKey || e.metaKey) {
            toggleSelection(path);
        } else if (e.shiftKey && fm.lastClickedPath) {
            rangeSelection(fm.lastClickedPath, path);
        } else {
            clearSelection();
            addSelection(path);
        }
        fm.lastClickedPath = path;
        activateKeyboardWindow();
        updateSelectionDOM();
        focusFileItem(path);
    }

    function openFileItem(path, type) {
        const file = fm.files.find(f => f.path === path);
        if (!file) return;
        if (type === 'directory') {
            navigate(file.path);
        } else {
            openFileEntry(file);
        }
    }

    function handleItemDblClick(e) {
        const path = e.currentTarget.dataset.path;
        const type = e.currentTarget.dataset.type;
        openFileItem(path, type);
    }

    function handleItemKeyDown(e) {
        const path = e.currentTarget.dataset.path;
        const type = e.currentTarget.dataset.type;
        if (e.key === 'Enter') {
            e.preventDefault();
            openFileItem(path, type);
        } else if (e.key === 'F2') {
            if (isReadonly()) return;
            e.preventDefault();
            startRename(path);
        } else if (e.key === 'Delete') {
            if (isReadonly()) return;
            e.preventDefault();
            deleteSelected();
        }
    }

    function handleItemContextMenu(e) {
        e.preventDefault();
        e.stopPropagation();
        const path = e.currentTarget.dataset.path;
        const type = e.currentTarget.dataset.type;
        const file = fm.files.find(f => f.path === path);
        if (!file) return;
        if (!fm.selectedPaths.has(path)) {
            clearSelection();
            addSelection(path);
            renderFileContent();
        }
        const hasClipboard = fm.clipboard && fm.clipboard.paths.length > 0;
        const items = [
            { label: t('desktop.fm.open', 'Open'), action: 'open', icon: 'folder-open', shortcut: 'Enter', handler: () => { if (type === 'directory') navigate(path); else openFileEntry(file); } },
            { separator: true },
            { label: t('desktop.fm.cut', 'Cut'), action: 'cut', icon: 'scissors', shortcut: 'Ctrl+X', handler: () => cutSelection() },
            { label: t('desktop.fm.copy', 'Copy'), action: 'copy', icon: 'copy', shortcut: 'Ctrl+C', handler: () => copySelection() },
            { label: t('desktop.fm.paste', 'Paste'), action: 'paste', icon: 'clipboard', shortcut: 'Ctrl+V', disabled: !hasClipboard, handler: () => pasteClipboard() },
            { separator: true },
            { label: t('desktop.fm.rename', 'Rename'), action: 'rename', icon: 'edit', shortcut: 'F2', handler: () => startRename(path) },
            { label: t('desktop.fm.delete', 'Delete'), action: 'delete', icon: 'trash', shortcut: 'Del', handler: () => deleteSelected() },
        ];
        if (type === 'file' && file.web_path) {
            items.push({ label: t('desktop.fm.download', 'Download'), action: 'download', icon: 'download', handler: () => downloadFile(file) });
        } else if (type === 'file') {
            items.push({ label: t('desktop.fm.download', 'Download'), action: 'download', icon: 'download', handler: () => downloadFile(file) });
        }
        items.push({ separator: true });
        items.push({ label: t('desktop.fm.properties', 'Properties'), action: 'properties', icon: 'info', handler: () => showProperties(file) });
        showContextMenu(e.clientX, e.clientY, readonlyGuardItems(items));
    }

    function handleEmptyContextMenu(e) {
        e.preventDefault();
        const hasClipboard = fm.clipboard && fm.clipboard.paths.length > 0;
        const items = [
            { label: t('desktop.fm.new_file', 'New File'), action: 'new-file', icon: 'file-plus', handler: () => createNewFile() },
            { label: t('desktop.fm.new_folder', 'New Folder'), action: 'new-folder', icon: 'folder-plus', handler: () => createNewFolder() },
            { separator: true },
            { label: t('desktop.fm.paste', 'Paste'), action: 'paste', icon: 'clipboard', shortcut: 'Ctrl+V', disabled: !hasClipboard, handler: () => pasteClipboard() },
            { separator: true },
            { label: t('desktop.fm.sort_by', 'Sort by') + ' >', action: 'sort-submenu', icon: 'sort', handler: () => showSortMenu(e) },
            { label: t('desktop.fm.refresh', 'Refresh'), action: 'refresh', icon: 'refresh', shortcut: 'F5', handler: () => refresh() },
            { separator: true },
            { label: t('desktop.fm.select_all', 'Select All'), action: 'select-all', icon: 'check-square', shortcut: 'Ctrl+A', handler: () => selectAll() },
        ];
        showContextMenu(e.clientX, e.clientY, readonlyGuardItems(items));
    }

    function openFileEntry(file) {
        if (!file || file.type === 'directory') return;
        if (isImageFile(file.name) || isMediaFile(file.name)) {
            if (fm.callbacks && typeof fm.callbacks.openMedia === 'function') {
                fm.callbacks.openMedia(file);
                return;
            }
        }
        if (fm.callbacks && typeof fm.callbacks.openFile === 'function') {
            fm.callbacks.openFile(file);
        }
    }

    function clearSelection() {
        fm.selectedPaths.clear();
        fm.lastClickedPath = null;
    }

    function addSelection(path) {
        fm.selectedPaths.add(path);
    }

    function toggleSelection(path) {
        if (fm.selectedPaths.has(path)) fm.selectedPaths.delete(path);
        else fm.selectedPaths.add(path);
    }

    function rangeSelection(fromPath, toPath) {
        const files = getDisplayFiles();
        const fromIndex = files.findIndex(f => f.path === fromPath);
        const toIndex = files.findIndex(f => f.path === toPath);
        if (fromIndex < 0 || toIndex < 0) return;
        const start = Math.min(fromIndex, toIndex);
        const end = Math.max(fromIndex, toIndex);
        for (let i = start; i <= end; i++) {
            fm.selectedPaths.add(files[i].path);
        }
    }

    function selectAll() {
        getDisplayFiles().forEach(f => fm.selectedPaths.add(f.path));
        renderAll();
    }

    function updateToolbarState() {
        if (!fm.host) return;
        const backBtn = fm.host.querySelector('[data-action="back"]');
        const fwdBtn = fm.host.querySelector('[data-action="forward"]');
        const sidebarBtn = fm.host.querySelector('[data-action="sidebar-toggle"]');
        if (backBtn) backBtn.disabled = !canGoBack();
        if (fwdBtn) fwdBtn.disabled = !canGoForward();
        if (sidebarBtn) {
            sidebarBtn.classList.toggle('active', fm.sidebarOpen);
            sidebarBtn.setAttribute('aria-expanded', fm.sidebarOpen ? 'true' : 'false');
        }
    }

    function updateStatusBar() {
        // Status bar is re-rendered with the full markup
    }

    // Clipboard operations
    function cutSelection() {
        if (isReadonly()) return;
        if (!fm.selectedPaths.size) return;
        fm.clipboard = { mode: 'cut', paths: Array.from(fm.selectedPaths) };
        renderAll();
    }

    function copySelection() {
        if (!fm.selectedPaths.size) return;
        fm.clipboard = { mode: 'copy', paths: Array.from(fm.selectedPaths) };
        renderAll();
    }

    async function pasteClipboard() {
        if (isReadonly()) return;
        if (!fm.clipboard || !fm.clipboard.paths.length) return;
        const destBase = fm.currentPath;
        for (const srcPath of fm.clipboard.paths) {
            const name = baseName(srcPath);
            let destPath = joinPath(destBase, name);
            const exists = fm.files.some(f => f.name === name);
            if (exists && fm.clipboard.mode === 'copy') {
                const newName = name + ' (' + t('desktop.fm.copy_of', 'copy') + ')';
                destPath = joinPath(destBase, newName);
            } else if (exists && fm.clipboard.mode === 'cut') {
                const overwrite = await confirmDialog(t('desktop.fm.paste_exists', 'An item named "{{name}}" already exists. Overwrite?', { name: name }));
                if (!overwrite) continue;
            }
            try {
                if (fm.clipboard.mode === 'copy') {
                    await api('/api/desktop/copy', {
                        method: 'POST',
                        body: JSON.stringify({ source_path: srcPath, dest_path: destPath })
                    });
                } else {
                    await api('/api/desktop/file', {
                        method: 'PATCH',
                        body: JSON.stringify({ old_path: srcPath, new_path: destPath })
                    });
                }
            } catch (err) {
                showNotification({ type: 'error', message: (err.message || String(err)) });
            }
        }
        if (fm.clipboard.mode === 'cut') fm.clipboard = null;
        refresh();
    }

    // File operations
    async function createNewFile() {
        if (isReadonly()) return;
        const name = await promptDialog(t('desktop.fm.new_file_prompt', 'File name'), 'new-file.txt');
        if (!name) return;
        const path = joinPath(fm.currentPath, name);
        try {
            await api('/api/desktop/file', {
                method: 'PUT',
                body: JSON.stringify({ path, content: '' })
            });
            refresh();
            showNotification({ type: 'success', message: name });
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

    function uploadFiles() {
        if (isReadonly()) return;
        const input = document.createElement('input');
        input.type = 'file';
        input.multiple = true;
        input.addEventListener('change', async () => {
            if (!input.files || !input.files.length) return;
            await uploadFileList(input.files);
        }, { once: true });
        input.click();
    }

    async function uploadFileList(files) {
        if (isReadonly()) return;
        showNotification({ type: 'info', message: t('desktop.fm.upload_progress', 'Uploading...') });
        const limit = maxFileSize();
        for (const file of Array.from(files)) {
            if (limit > 0 && file.size > limit) {
                showNotification({ type: 'error', message: t('desktop.fm.upload_too_large', '{{name}} exceeds the maximum upload size.', { name: file.name }) });
                continue;
            }
            const formData = new FormData();
            formData.append('file', file);
            formData.append('path', fm.currentPath);
            try {
                await api('/api/desktop/upload', { method: 'POST', body: formData });
            } catch (err) {
                showNotification({ type: 'error', message: file.name + ': ' + (err.message || String(err)) });
            }
        }
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
                <div class="fm-prop-row"><span class="fm-prop-label">${esc(t('desktop.fm.prop_modified', 'Modified'))}</span><span class="fm-prop-value">${esc(formatDate(file.modified))}</span></div>
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

    function handleDragStart(e) {
        const path = e.currentTarget.dataset.path;
        dragSrcPath = path;
        if (!fm.selectedPaths.has(path)) {
            clearSelection();
            addSelection(path);
        }
        e.dataTransfer.effectAllowed = 'move';
        e.dataTransfer.setData('text/plain', path);
    }

    function handleDragOver(e) {
        e.preventDefault();
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
        if (type === 'directory' && target.dataset.path !== dragSrcPath) {
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
        if (type === 'directory' && target.dataset.path !== dragSrcPath) {
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
        if (destType !== 'directory') return;
        if (!dragSrcPath || dragSrcPath === destPath) return;
        const srcFile = fm.files.find(f => f.path === dragSrcPath);
        if (!srcFile) return;
        // If the dragged item is selected, move all selected items
        const pathsToMove = fm.selectedPaths.has(dragSrcPath) ? Array.from(fm.selectedPaths) : [dragSrcPath];
        for (const src of pathsToMove) {
            const name = baseName(src);
            const newPath = joinPath(destPath, name);
            try {
                await api('/api/desktop/file', {
                    method: 'PATCH',
                    body: JSON.stringify({ old_path: src, new_path: newPath })
                });
            } catch (err) {
                showNotification({ type: 'error', message: (err.message || String(err)) });
            }
        }
        clearSelection();
        dragSrcPath = null;
        refresh();
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

        const isInput = document.activeElement && (document.activeElement.tagName === 'INPUT' || document.activeElement.tagName === 'TEXTAREA');
        if (isInput && e.key !== 'Escape') return;

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
