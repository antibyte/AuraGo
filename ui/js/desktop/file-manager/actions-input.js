    function renderListRow(file) {
        const isDir = file.type === 'directory';
        const iconKey = isDir ? iconForDirectory(file.name) : iconForFile(file);
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
        const clipboard = sharedFileClipboard();
        const clipboardIndicator = clipboard && clipboard.paths && clipboard.paths.length
            ? `<span class="fm-status-sep">|</span><span class="fm-status-clipboard" title="${esc(clipboard.paths.join('\n'))}">📋 ${esc(clipboard.mode === 'cut' ? t('desktop.fm.clipboard_cut', '{{count}} cut', { count: clipboard.paths.length }) : t('desktop.fm.clipboard_copied', '{{count}} copied', { count: clipboard.paths.length }))}</span>`
            : '';
        return `<div class="fm-statusbar">
            <div class="fm-status-left">
                <span>${esc(t('desktop.fm.items', '{{count}} items', { count: totalItems }))}</span>
                ${selectedCount > 0 ? `<span class="fm-status-sep">|</span><span>${esc(t('desktop.fm.selected', '{{count}} selected', { count: selectedCount }))}${esc(selectedSize)}</span>` : ''}
                ${clipboardIndicator}
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

        // Tab click and drag-drop events
        root.querySelectorAll('.fm-tab').forEach(tab => {
            tab.addEventListener('click', e => {
                if (e.target.closest('[data-action="close-tab"]')) return;
                const idx = parseInt(tab.dataset.tabIndex);
                if (typeof switchTab === 'function') switchTab(idx);
            });
            tab.addEventListener('dragstart', e => {
                if (typeof handleTabDragStart === 'function') handleTabDragStart(e);
            });
            tab.addEventListener('dragover', e => {
                if (typeof handleTabDragOver === 'function') handleTabDragOver(e);
            });
            tab.addEventListener('drop', e => {
                if (typeof handleTabDrop === 'function') handleTabDrop(e);
            });
        });
        const tabNewBtn = root.querySelector('.fm-tab-new');
        if (tabNewBtn) {
            tabNewBtn.addEventListener('click', () => {
                if (typeof createNewTab === 'function') createNewTab();
            });
        }
        root.querySelectorAll('[data-action="close-tab"]').forEach(btn => {
            btn.addEventListener('click', e => {
                e.stopPropagation();
                if (typeof closeTab === 'function') closeTab(parseInt(btn.dataset.tabIndex));
            });
        });

        // Breadcrumb segments
        root.querySelectorAll('[data-breadcrumb-path]').forEach(seg => {
            seg.addEventListener('click', () => navigate(seg.dataset.breadcrumbPath));
            seg.addEventListener('keydown', e => { if (e.key === 'Enter') navigate(seg.dataset.breadcrumbPath); });
            seg.addEventListener('dragover', handleBreadcrumbDragOver);
            seg.addEventListener('dragenter', handleBreadcrumbDragEnter);
            seg.addEventListener('dragleave', handleBreadcrumbDragLeave);
            seg.addEventListener('drop', handleBreadcrumbDrop);
        });

        // Sidebar items
        root.querySelectorAll('[data-sidebar-path]').forEach(item => {
            item.addEventListener('click', e => {
                if (e.target.closest('[data-action="remove-favorite"]')) return;
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

        // Favorites drag & drop
        root.querySelectorAll('.fm-favorite-item').forEach(item => {
            item.addEventListener('dragstart', e => {
                if (typeof handleFavDragStart === 'function') handleFavDragStart(e);
            });
            item.addEventListener('dragover', e => {
                if (typeof handleFavDragOver === 'function') handleFavDragOver(e);
            });
            item.addEventListener('drop', e => {
                if (typeof handleFavDrop === 'function') handleFavDrop(e);
            });
        });

        const favSection = root.querySelector('.fm-favorites-section');
        if (favSection) {
            favSection.addEventListener('dragover', e => {
                if (typeof handleFavoritesSectionDragOver === 'function') handleFavoritesSectionDragOver(e);
            });
            favSection.addEventListener('drop', e => {
                if (typeof handleFavoritesSectionDrop === 'function') handleFavoritesSectionDrop(e);
            });
        }

        root.querySelectorAll('[data-action="remove-favorite"]').forEach(btn => {
            btn.addEventListener('click', e => {
                e.stopPropagation();
                if (typeof toggleFavorite === 'function') toggleFavorite(btn.dataset.path);
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
        
        // Split pane pointerdown activation
        root.querySelectorAll('.fm-pane').forEach(pane => {
            pane.addEventListener('pointerdown', e => {
                const paneName = pane.dataset.pane;
                if (paneName && typeof switchActivePane === 'function') {
                    switchActivePane(paneName);
                }
            });
        });

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
            wireLongPress(item, handleItemLongPress);
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
        root.querySelectorAll('.fm-selection-toolbar [data-action]').forEach(btn => {
            if (btn.dataset.fmSelBound === 'true') return;
            btn.dataset.fmSelBound = 'true';
            btn.addEventListener('click', e => {
                e.stopPropagation();
                const action = btn.dataset.action;
                const selected = getSelectedFiles();
                const singleFile = selected.length === 1 ? selected[0] : null;
                switch (action) {
                    case 'selection-close': exitSelectionMode(); break;
                    case 'selection-open':
                        if (singleFile) openFileItem(singleFile.path, singleFile.type);
                        break;
                    case 'selection-copy': copySelection(); break;
                    case 'selection-cut': cutSelection(); break;
                    case 'selection-delete': deleteSelected(); break;
                    case 'selection-download':
                        if (singleFile && singleFile.type === 'file') downloadFile(singleFile);
                        break;
                    case 'selection-properties':
                        if (singleFile) showProperties(singleFile);
                        break;
                }
            });
        });
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
                    exitSelectionMode();
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
        else if (action === 'toggle-hidden') { fm.showHidden = !fm.showHidden; renderAll(); }
        else if (action === 'toggle-split') { if (typeof toggleSplitView === 'function') toggleSplitView(); }
        else if (action === 'calc-folder-size') {
            const path = e.currentTarget.dataset.path;
            calculateFolderSize(path);
        }
    }

    function handleSidebarToggle() {
        fm.sidebarOpen = !fm.sidebarOpen;
        renderAll();
        const backdrop = fm.host.querySelector('.fm-sidebar-backdrop');
        if (backdrop) {
            backdrop.hidden = !fm.sidebarOpen || !isCompactViewportFM();
            if (!backdrop.hidden) {
                backdrop.addEventListener('click', () => {
                    fm.sidebarOpen = false;
                    renderAll();
                }, { once: true });
            }
        }
    }

    function isCompactViewportFM() {
        return !!(window.matchMedia && window.matchMedia('(max-width: 820px)').matches);
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
            if (fm.selectionMode) {
                toggleSelection(path);
                fm.lastClickedPath = path;
                activateKeyboardWindow();
                updateSelectionDOM();
                focusFileItem(path);
                return;
            }
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

    function handleItemLongPress(e) {
        const path = e.currentTarget.dataset.path;
        fm.selectionMode = true;
        toggleSelection(path);
        fm.lastClickedPath = path;
        activateKeyboardWindow();
        updateSelectionDOM();
        focusFileItem(path);
        if (navigator.vibrate) navigator.vibrate(15);
    }

    function exitSelectionMode() {
        fm.selectionMode = false;
        clearSelection();
        updateSelectionDOM();
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
        const hasClipboard = hasSharedFileClipboard();
        const items = [
            { label: t('desktop.fm.open', 'Open'), action: 'open', icon: 'folder-open', shortcut: 'Enter', handler: () => { if (type === 'directory') navigate(path); else openFileEntry(file); } },
        ];
        if (type === 'file') {
            const openWithItems = buildOpenWithSubmenu(file);
            if (openWithItems.length) {
                items.push({ label: t('desktop.fm.open_with', 'Open with...'), action: 'open-with', icon: 'apps', items: openWithItems });
            }
        }
        if (type === 'file' && isViewerFile(file.name || '')) {
            items.push({ label: t('desktop.fm.view', 'View'), action: 'view', icon: 'eye', handler: () => { if (fm.callbacks && typeof fm.callbacks.openApp === 'function') fm.callbacks.openApp('viewer', { path: file.path }); else openFileEntry(file); } });
        }
        if (type === 'file') {
            items.push(
                { separator: true },
                { label: t('desktop.fm.add_to_chat', 'Add to chat'), action: 'add-to-chat', icon: 'chat', handler: () => { if (fm.callbacks && typeof fm.callbacks.addFileToChat === 'function') fm.callbacks.addFileToChat(file); } },
                { label: t('desktop.fm.ask_agent', 'Ask Agent'), action: 'ask-agent', icon: 'agent', handler: () => { if (fm.callbacks && typeof fm.callbacks.askAgentAboutFile === 'function') fm.callbacks.askAgentAboutFile(file); } }
            );
        }
        items.push(
            { separator: true },
            { label: t('desktop.fm.cut', 'Cut'), action: 'cut', icon: 'scissors', shortcut: 'Ctrl+X', handler: () => cutSelection() },
            { label: t('desktop.fm.copy', 'Copy'), action: 'copy', icon: 'copy', shortcut: 'Ctrl+C', handler: () => copySelection() },
            { label: t('desktop.fm.copy_path', 'Copy Path'), action: 'copy-path', icon: 'text', shortcut: 'Ctrl+Shift+C', handler: () => copyPathToClipboard(path) },
            { label: t('desktop.fm.paste', 'Paste'), action: 'paste', icon: 'clipboard', shortcut: 'Ctrl+V', disabled: !hasClipboard, handler: () => pasteClipboard(type === 'directory' ? path : fm.currentPath) },
            { label: t('desktop.fm.duplicate', 'Duplicate'), action: 'duplicate', icon: 'copy', shortcut: 'Ctrl+D', handler: () => duplicateSelected() },
            { label: t('desktop.fm.create_symlink', 'Create Symlink'), action: 'create-symlink', icon: 'link', handler: () => createSymlink(file) },
        );
        const selected = getSelectedFiles();
        if (selected.length > 1) {
            items.push({ label: t('desktop.fm.batch_rename', 'Batch Rename'), action: 'batch-rename', icon: 'edit', handler: () => { if (typeof executeBatchRename === 'function') executeBatchRename(); } });
        }
        if (selected.length > 0) {
            items.push({ label: t('desktop.fm.compress_zip', 'Compress to ZIP'), action: 'compress-zip', icon: 'archive', handler: () => { if (typeof compressSelectionToZip === 'function') compressSelectionToZip(); } });
        }
        if (type === 'file' && String(file.name).endsWith('.zip')) {
            items.push(
                { label: t('desktop.fm.extract_zip', 'Extract ZIP Here'), action: 'extract-zip-here', icon: 'archive', handler: () => { if (typeof extractZip === 'function') extractZip(file, true); } },
                { label: t('desktop.fm.extract_zip_to', 'Extract ZIP to...'), action: 'extract-zip-to', icon: 'archive', handler: () => { if (typeof extractZip === 'function') extractZip(file, false); } }
            );
        }
        if (type === 'directory') {
            const isFav = fm.favorites && fm.favorites.includes(path);
            items.push({
                label: isFav ? t('desktop.fm.remove_favorite', 'Remove from Favorites') : t('desktop.fm.add_favorite', 'Add to Favorites'),
                action: isFav ? 'remove-favorite' : 'add-favorite',
                icon: 'star',
                handler: () => { if (typeof toggleFavorite === 'function') toggleFavorite(path); }
            });
        }
        items.push(
            { separator: true },
            { label: t('desktop.fm.rename', 'Rename'), action: 'rename', icon: 'edit', shortcut: 'F2', handler: () => startRename(path) },
            { label: t('desktop.fm.delete', 'Delete'), action: 'delete', icon: 'trash', shortcut: 'Del', handler: () => deleteSelected() },
        );
        if (type === 'directory') {
            items.push(
                { separator: true },
                { label: t('desktop.fm.open_terminal', 'Open Terminal Here'), action: 'open-terminal', icon: 'terminal', handler: () => openTerminalHere(path) }
            );
        }
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
        const hasClipboard = hasSharedFileClipboard();
        const items = [
            { label: t('desktop.fm.new_file', 'New File'), action: 'new-file', icon: 'file-plus', handler: () => createNewFile() },
            { label: t('desktop.fm.new_folder', 'New Folder'), action: 'new-folder', icon: 'folder-plus', handler: () => createNewFolder() },
            { separator: true },
            { label: t('desktop.fm.paste', 'Paste'), action: 'paste', icon: 'clipboard', shortcut: 'Ctrl+V', disabled: !hasClipboard, handler: () => pasteClipboard() },
            { separator: true },
            { label: t('desktop.fm.sort_by', 'Sort by') + ' >', action: 'sort-submenu', icon: 'sort', handler: () => showSortMenu(e) },
            { label: t('desktop.fm.refresh', 'Refresh'), action: 'refresh', icon: 'refresh', shortcut: 'F5', handler: () => refresh() },
            { label: t('desktop.fm.open_terminal', 'Open Terminal Here'), action: 'open-terminal', icon: 'terminal', handler: () => openTerminalHere(fm.currentPath) },
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

    function setExactRangeSelection(fromPath, toPath) {
        const files = getDisplayFiles();
        const fromIndex = files.findIndex(f => f.path === fromPath);
        const toIndex = files.findIndex(f => f.path === toPath);
        if (fromIndex < 0 || toIndex < 0) return;
        fm.selectedPaths.clear();
        const start = Math.min(fromIndex, toIndex);
        const end = Math.max(fromIndex, toIndex);
        for (let i = start; i <= end; i++) {
            fm.selectedPaths.add(files[i].path);
        }
    }

    function getGridColsCount() {
        if (!fm.host) return 1;
        const container = fm.host.querySelector('.fm-main');
        if (!container) return 1;
        const items = container.querySelectorAll('.fm-grid-item');
        if (items.length <= 1) return 1;
        const firstTop = items[0].getBoundingClientRect().top;
        let count = 0;
        for (const item of items) {
            if (Math.abs(item.getBoundingClientRect().top - firstTop) < 5) {
                count++;
            } else {
                break;
            }
        }
        return count || 1;
    }

    function showProgressOverlay(title, total) {
        const overlay = document.createElement('div');
        overlay.className = 'fm-upload-overlay';
        overlay.innerHTML = `
            <div class="fm-upload-panel">
                <div class="fm-upload-title">${esc(title)}</div>
                <div class="fm-upload-bar-bg"><div class="fm-upload-bar-fill" style="width:0%"></div></div>
                <div class="fm-upload-percent">0%</div>
                <div class="fm-upload-file"></div>
            </div>
        `;
        document.body.appendChild(overlay);
        const barFill = overlay.querySelector('.fm-upload-bar-fill');
        const percentEl = overlay.querySelector('.fm-upload-percent');
        const fileEl = overlay.querySelector('.fm-upload-file');
        
        return {
            update(current, fileName) {
                const pct = Math.round((current / total) * 100);
                barFill.style.width = pct + '%';
                percentEl.textContent = pct + '%';
                fileEl.textContent = `${esc(fileName)} (${current}/${total})`;
            },
            close() {
                overlay.remove();
            }
        };
    }

    const undoStack = [];
    const redoStack = [];

    function pushToUndo(action) {
        undoStack.push(action);
        if (undoStack.length > 20) {
            undoStack.shift();
        }
        redoStack.length = 0;
    }

    async function undo() {
        if (!undoStack.length) {
            showNotification({ type: 'warning', message: t('desktop.fm.nothing_to_undo', 'Nothing to undo') });
            return;
        }
        const action = undoStack.pop();
        try {
            let progress = null;
            if (action.items.length > 1) {
                progress = showProgressOverlay(t('desktop.fm.undoing', 'Undoing...'), action.items.length);
            }
            let count = 0;
            for (const item of action.items) {
                count++;
                if (progress) progress.update(count, baseName(item.oldPath));
                await api('/api/desktop/file', {
                    method: 'PATCH',
                    body: JSON.stringify({ old_path: item.newPath, new_path: item.oldPath })
                });
            }
            if (progress) progress.close();
            
            redoStack.push(action);
            showNotification({ type: 'success', message: t('desktop.fm.undone', 'Operation undone') });
            refresh();
        } catch (err) {
            showNotification({ type: 'error', message: t('desktop.fm.undo_error', 'Undo failed: {{error}}', { error: err.message || String(err) }) });
        }
    }

    async function redo() {
        if (!redoStack.length) {
            showNotification({ type: 'warning', message: t('desktop.fm.nothing_to_redo', 'Nothing to redo') });
            return;
        }
        const action = redoStack.pop();
        try {
            let progress = null;
            if (action.items.length > 1) {
                progress = showProgressOverlay(t('desktop.fm.redoing', 'Redoing...'), action.items.length);
            }
            let count = 0;
            for (const item of action.items) {
                count++;
                if (progress) progress.update(count, baseName(item.newPath));
                await api('/api/desktop/file', {
                    method: 'PATCH',
                    body: JSON.stringify({ old_path: item.oldPath, new_path: item.newPath })
                });
            }
            if (progress) progress.close();
            
            undoStack.push(action);
            showNotification({ type: 'success', message: t('desktop.fm.redone', 'Operation redone') });
            refresh();
        } catch (err) {
            showNotification({ type: 'error', message: t('desktop.fm.redo_error', 'Redo failed: {{error}}', { error: err.message || String(err) }) });
        }
    }

    let quickLookOverlay = null;

    function toggleQuickLook() {
        if (quickLookOverlay) {
            quickLookOverlay.remove();
            quickLookOverlay = null;
            return;
        }
        
        const selected = getSelectedFiles();
        if (selected.length !== 1) return;
        
        const file = selected[0];
        const isImg = isPreviewableImage(file);
        const isMedia = isMediaFile(file.name);
        
        const ext = String(file.name || '').split('.').pop().toLowerCase();
        const textExts = new Set(['txt', 'log', 'md', 'json', 'yaml', 'yml', 'sh', 'py', 'go', 'js', 'css', 'html', 'xml', 'ini', 'conf']);
        const isTxt = textExts.has(ext);
        
        const overlay = document.createElement('div');
        overlay.className = 'fm-modal-overlay fm-quick-look-overlay';
        overlay.style.zIndex = '999999';
        
        let content = '';
        if (isImg) {
            content = `<img src="${previewURL(file)}" style="max-width: 90vw; max-height: 80vh; object-fit: contain; border-radius: 8px;" />`;
        } else if (isMedia) {
            const mime = file.mime || '';
            const isAudio = mime.startsWith('audio/') || ext === 'mp3' || ext === 'wav' || ext === 'ogg';
            if (isAudio) {
                content = `<audio src="${previewURL(file)}" controls style="width: min(400px, 90vw)"></audio>`;
            } else {
                content = `<video src="${previewURL(file)}" controls style="max-width: 90vw; max-height: 80vh; border-radius: 8px;"></video>`;
            }
        } else if (isTxt) {
            content = `<div class="fm-quick-look-text" style="width: min(700px, 90vw); height: 60vh; background: var(--ds-color-surface-2); border: 1px solid var(--ds-color-border-subtle); border-radius: 8px; padding: 16px; overflow: auto; text-align: left; font-family: monospace; white-space: pre-wrap; font-size: 0.85rem; color: var(--ds-color-fg-primary);">Loading preview...</div>`;
            fetchTextPreview(file.path);
        } else {
            content = `
                <div style="display:flex; flex-direction:column; align-items:center; gap:16px;">
                    <div style="font-size: 4rem;">📄</div>
                    <div style="font-size: 1.1rem; font-weight: 600;">${esc(file.name)}</div>
                    <div style="color: var(--ds-color-fg-muted);">${esc(fmtBytes(file.size))}</div>
                </div>
            `;
        }
        
        overlay.innerHTML = `
            <div style="position: absolute; top: 16px; right: 16px; display: flex; gap: 8px; z-index: 10;">
                <button class="fm-btn" data-close-ql style="padding: 6px 12px; font-size: 0.85rem;">Close (Space)</button>
            </div>
            <div style="display: flex; justify-content: center; align-items: center; width: 100%; height: 100%;">
                <div class="fm-quick-look-panel" style="padding: 24px; background: rgba(0,0,0,0.85); border-radius: 12px; border: 1px solid rgba(255,255,255,0.1); box-shadow: 0 8px 32px rgba(0,0,0,0.5); display: flex; justify-content: center; align-items: center; flex-direction: column; min-width: 280px; max-width: 95vw;">
                    ${content}
                    <div style="margin-top: 16px; font-size: 0.8rem; color: var(--ds-color-fg-muted);">${esc(file.path)}</div>
                </div>
            </div>
        `;
        
        document.body.appendChild(overlay);
        quickLookOverlay = overlay;
        
        overlay.querySelector('[data-close-ql]').addEventListener('click', () => {
            overlay.remove();
            quickLookOverlay = null;
        });
        overlay.addEventListener('click', e => {
            if (e.target === overlay) {
                overlay.remove();
                quickLookOverlay = null;
            }
        });
        
        async function fetchTextPreview(path) {
            try {
                const res = await fetch('/api/desktop/file-content?path=' + encodeURIComponent(path));
                if (!res.ok) throw new Error();
                const text = await res.text();
                const el = overlay.querySelector('.fm-quick-look-text');
                if (el) el.textContent = text;
            } catch (err) {
                const el = overlay.querySelector('.fm-quick-look-text');
                if (el) el.textContent = 'Cannot load preview.';
            }
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
    function sharedFileOps() {
        return window.AuraDesktopFileOps || null;
    }

    function sharedFileClipboard() {
        const ops = sharedFileOps();
        if (ops && typeof ops.getClipboard === 'function') return ops.getClipboard();
        return fm.clipboard && fm.clipboard.paths && fm.clipboard.paths.length ? fm.clipboard : null;
    }

    function hasSharedFileClipboard() {
        const ops = sharedFileOps();
        if (ops && typeof ops.hasClipboard === 'function') return ops.hasClipboard();
        return !!sharedFileClipboard();
    }

    function setSharedFileClipboard(mode, paths) {
        const cleanPaths = Array.from(new Set((paths || []).filter(Boolean)));
        fm.clipboard = cleanPaths.length ? { mode, paths: cleanPaths } : null;
        const ops = sharedFileOps();
        if (ops && typeof ops.setClipboard === 'function') ops.setClipboard(mode, cleanPaths);
    }

    function cutSelection() {
        if (isReadonly()) return;
        if (!fm.selectedPaths.size) return;
        setSharedFileClipboard('cut', Array.from(fm.selectedPaths));
        renderAll();
    }

    function copySelection() {
        if (!fm.selectedPaths.size) return;
        setSharedFileClipboard('copy', Array.from(fm.selectedPaths));
        renderAll();
    }

    async function pasteClipboard(destBase) {
        if (isReadonly()) return;
        const ops = sharedFileOps();
        if (ops && typeof ops.paste === 'function') {
            await ops.paste(destBase == null ? fm.currentPath : destBase);
            fm.clipboard = null;
            refresh();
            return;
        }
        const clipboard = sharedFileClipboard();
        if (!clipboard || !clipboard.paths.length) return;
        const targetBase = destBase || fm.currentPath;

        let progress = null;
        if (clipboard.paths.length > 1) {
            const title = clipboard.mode === 'copy' 
                ? t('desktop.fm.copying', 'Copying...') 
                : t('desktop.fm.moving', 'Moving...');
            progress = showProgressOverlay(title, clipboard.paths.length);
        }

        let count = 0;
        const undoItems = [];

        for (const srcPath of clipboard.paths) {
            count++;
            const name = baseName(srcPath);
            let destPath = joinPath(targetBase, name);
            const exists = fm.files.some(f => f.name === name);
            if (exists && clipboard.mode === 'copy') {
                const newName = name + ' (' + t('desktop.fm.copy_of', 'copy') + ')';
                destPath = joinPath(targetBase, newName);
            } else if (exists && clipboard.mode === 'cut') {
                const overwrite = await confirmDialog(t('desktop.fm.paste_exists', 'An item named "{{name}}" already exists. Overwrite?', { name: name }));
                if (!overwrite) continue;
            }

            if (progress) {
                progress.update(count, name);
            }

            try {
                if (clipboard.mode === 'copy') {
                    await api('/api/desktop/copy', {
                        method: 'POST',
                        body: JSON.stringify({ source_path: srcPath, dest_path: destPath })
                    });
                } else {
                    await api('/api/desktop/file', {
                        method: 'PATCH',
                        body: JSON.stringify({ old_path: srcPath, new_path: destPath })
                    });
                    undoItems.push({ oldPath: srcPath, newPath: destPath });
                }
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

        if (clipboard.mode === 'cut') fm.clipboard = null;
        refresh();
    }
