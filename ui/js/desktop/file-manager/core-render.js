(function () {
    'use strict';

    const LS_VIEW_KEY = 'aurago.fm.view';
    const LS_SORT_KEY = 'aurago.fm.sort';
    const PREVIEW_IMAGE_EXTS = new Set(['avif', 'bmp', 'gif', 'jpeg', 'jpg', 'png', 'webp']);
    const PREVIEW_IMAGE_MIMES = new Set(['image/avif', 'image/bmp', 'image/gif', 'image/jpeg', 'image/png', 'image/webp']);
    const FILE_RENDER_BATCH_SIZE = 250;
    const FILE_INCREMENTAL_THRESHOLD = 600;
    const DESKTOP_FILE_DRAG_TYPE = 'application/x-aurago-desktop-files';

    const instances = new Map();
    let fm = createInstance();

    function createInstance() {
        return {
            windowId: '',
            host: null,
            callbacks: null,
            currentPath: '',
            files: [],
            filteredFiles: null,
            selectedPaths: new Set(),
            clipboard: null,
            viewMode: 'list',
            sortBy: 'name',
            sortAsc: true,
            history: [],
            historyIndex: -1,
            searchQuery: '',
            directories: [],
            loading: false,
            lastClickedPath: null,
            renamePath: null,
            dragOverPath: null,
            keyboardBound: false,
            activeKeyboardWindow: '',
            sidebarOpen: false,
            incrementalRenderToken: 0,
            tabs: null,
            activeTabIndex: 0,
            showHidden: false,
            previewOpen: false,
            previewWidth: 250,
            favorites: [],
            splitViewEnabled: false,
            activePane: 'left',
            leftPane: null,
            rightPane: null,
        };
    }

    function setActiveInstance(instance) {
        if (instance) fm = instance;
        return fm;
    }

    function instanceForWindow(windowId) {
        return instances.get(windowId) || null;
    }



    function fmtBytes(size) {
        if (fm.callbacks && typeof fm.callbacks.fmtBytes === 'function') {
            return fm.callbacks.fmtBytes(size);
        }
        const n = Number(size || 0);
        if (n < 1024) return n + ' B';
        if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KiB';
        if (n < 1024 * 1024 * 1024) return (n / 1024 / 1024).toFixed(1) + ' MiB';
        return (n / 1024 / 1024 / 1024).toFixed(1) + ' GiB';
    }

    function formatDate(dateStr) {
        if (!dateStr) return '\u2014';
        try {
            const d = new Date(dateStr);
            return d.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' }) + ' ' +
                   d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
        } catch { return '\u2014'; }
    }

    function showContextMenu(x, y, items) {
        if (fm.callbacks && typeof fm.callbacks.showContextMenu === 'function') {
            // Convert file-manager format {action: string, handler: fn} to main.js format {action: fn}
            const convertItem = item => {
                if (item.separator) return item;
                const converted = {
                    label: item.label,
                    icon: item.icon || 'tools',
                    fallback: contextIconGlyph(item.icon)
                };
                if (item.disabled) converted.disabled = true;
                converted.action = typeof item.handler === 'function' ? item.handler : (typeof item.action === 'function' ? item.action : () => {});
                if (item.items) {
                    converted.items = item.items.map(convertItem);
                } else if (item.children) {
                    converted.items = item.children.map(convertItem);
                }
                return converted;
            };
            const converted = items.map(convertItem);
            fm.callbacks.showContextMenu(x, y, converted);
            return;
        }
        closeContextMenu();
        const menu = document.createElement('div');
        menu.className = 'fm-context-menu';
        menu.style.left = x + 'px';
        menu.style.top = y + 'px';
        menu.innerHTML = items.map(item => {
            if (item.separator) return '<div class="fm-context-separator"></div>';
            return `<button type="button" class="fm-context-item${item.disabled ? ' disabled' : ''}" data-action="${esc(item.action || '')}">
                ${item.icon ? iconMarkup(item.icon, item.icon, 'fm-context-icon', 14) : ''}
                <span>${esc(item.label)}</span>
                ${item.shortcut ? `<kbd class="fm-context-shortcut">${esc(item.shortcut)}</kbd>` : ''}
            </button>`;
        }).join('');
        document.body.appendChild(menu);
        const menuRect = menu.getBoundingClientRect();
        let nextLeft = x;
        let nextTop = y;
        if (menuRect.right > window.innerWidth) nextLeft = Math.max(8, window.innerWidth - menuRect.width - 8);
        if (menuRect.bottom > window.innerHeight) nextTop = Math.max(8, window.innerHeight - menuRect.height - 8);
        if (menuRect.left < 8) nextLeft = Math.max(8, nextLeft);
        if (menuRect.top < 8) nextTop = Math.max(8, nextTop);
        menu.style.left = nextLeft + 'px';
        menu.style.top = nextTop + 'px';
        menu.querySelectorAll('.fm-context-item:not(.disabled)').forEach(btn => {
            btn.addEventListener('click', () => {
                const action = btn.dataset.action;
                const item = items.find(i => i.action === action);
                closeContextMenu();
                if (item && typeof item.handler === 'function') item.handler();
            });
        });
        setTimeout(() => {
            const closeHandler = event => {
                if (!menu.contains(event.target)) {
                    closeContextMenu();
                    document.removeEventListener('mousedown', closeHandler);
                    document.removeEventListener('keydown', escapeHandler);
                }
            };
            const escapeHandler = event => {
                if (event.key === 'Escape') {
                    closeContextMenu();
                    document.removeEventListener('mousedown', closeHandler);
                    document.removeEventListener('keydown', escapeHandler);
                }
            };
            document.addEventListener('mousedown', closeHandler);
            document.addEventListener('keydown', escapeHandler);
        }, 0);
    }

    function closeContextMenu() {
        if (fm.callbacks && typeof fm.callbacks.closeContextMenu === 'function') {
            fm.callbacks.closeContextMenu();
            return;
        }
        document.querySelectorAll('.fm-context-menu').forEach(menu => menu.remove());
    }

    function promptDialog(title, value) {
        if (fm.callbacks && typeof fm.callbacks.promptDialog === 'function') {
            return fm.callbacks.promptDialog(title, value);
        }
        return new Promise(resolve => {
            const overlay = document.createElement('div');
            overlay.className = 'fm-modal-overlay';
            overlay.innerHTML = `<form class="fm-modal">
                <div class="fm-modal-title">${esc(title)}</div>
                <input type="text" name="value" value="${esc(value || '')}" autocomplete="off" spellcheck="false">
                <div class="fm-modal-actions">
                    <button type="button" class="fm-btn" data-cancel>${esc(t('desktop.cancel', 'Cancel'))}</button>
                    <button type="submit" class="fm-btn primary">${esc(t('desktop.ok', 'OK'))}</button>
                </div>
            </form>`;
            document.body.appendChild(overlay);
            const input = overlay.querySelector('input');
            const cleanup = result => { overlay.remove(); resolve(result); };
            overlay.querySelector('form').addEventListener('submit', e => { e.preventDefault(); cleanup(input.value.trim()); });
            overlay.querySelector('[data-cancel]').addEventListener('click', () => cleanup(''));
            overlay.addEventListener('click', e => { if (e.target === overlay) cleanup(''); });
            input.focus();
            input.select();
        });
    }

    function confirmDialog(title, message) {
        if (fm.callbacks && typeof fm.callbacks.confirmDialog === 'function') {
            return fm.callbacks.confirmDialog(title, message);
        }
        return new Promise(resolve => {
            const overlay = document.createElement('div');
            overlay.className = 'fm-modal-overlay';
            overlay.innerHTML = `<div class="fm-modal">
                <div class="fm-modal-title">${esc(title)}</div>
                <p class="fm-modal-message">${esc(message)}</p>
                <div class="fm-modal-actions">
                    <button type="button" class="fm-btn" data-cancel>${esc(t('desktop.cancel', 'Cancel'))}</button>
                    <button type="button" class="fm-btn danger" data-confirm>${esc(t('desktop.delete', 'Delete'))}</button>
                </div>
            </div>`;
            document.body.appendChild(overlay);
            const cleanup = result => { overlay.remove(); resolve(result); };
            overlay.querySelector('[data-confirm]').addEventListener('click', () => cleanup(true));
            overlay.querySelector('[data-cancel]').addEventListener('click', () => cleanup(false));
            overlay.addEventListener('click', e => { if (e.target === overlay) cleanup(false); });
            overlay.querySelector('[data-confirm]').focus();
        });
    }

    function showNotification(payload) {
        if (fm.callbacks && typeof fm.callbacks.showNotification === 'function') {
            fm.callbacks.showNotification(payload);
            return;
        }
        console.log('[FileManager]', payload);
    }

    function joinPath(base, name) {
        const b = String(base || '').replace(/\/+$/, '');
        const n = String(name || '').replace(/^[\/]+/, '');
        if (!b) return n;
        if (!n) return b;
        return b + '/' + n;
    }

    function baseName(path) {
        return String(path || '').split(/[\\/]/).filter(Boolean).pop() || '';
    }

    function parentPath(path) {
        const clean = String(path || '').replace(/[\/]+$/, '');
        const idx = clean.lastIndexOf('/');
        if (idx <= 0) return '';
        return clean.slice(0, idx);
    }

    function isImageFile(name) {
        const ext = String(name || '').split('.').pop().toLowerCase();
        return ['png', 'jpg', 'jpeg', 'gif', 'svg', 'webp', 'bmp', 'ico'].includes(ext);
    }

    function isMediaFile(name) {
        const ext = String(name || '').split('.').pop().toLowerCase();
        return ['mp4', 'mkv', 'avi', 'mov', 'webm', 'mp3', 'wav', 'flac', 'ogg', 'm4a'].includes(ext);
    }

    function isViewerFile(name) {
        const ext = String(name || '').split('.').pop().toLowerCase();
        return ['md', 'pdf', 'docx', 'xlsx', 'xlsm', 'csv'].includes(ext);
    }

    function loadPreferences() {
        try {
            const view = localStorage.getItem(LS_VIEW_KEY);
            if (view === 'grid' || view === 'list') fm.viewMode = view;
            const sort = localStorage.getItem(LS_SORT_KEY);
            if (sort) {
                const parts = sort.split(':');
                if (parts[0]) fm.sortBy = parts[0];
                if (parts[1]) fm.sortAsc = parts[1] === 'asc';
            }
            fm.previewOpen = localStorage.getItem('aurago.fm.previewOpen') === 'true';
            fm.previewWidth = parseInt(localStorage.getItem('aurago.fm.previewWidth')) || 250;
            try {
                const favs = localStorage.getItem('aurago.fm.favorites');
                fm.favorites = favs ? JSON.parse(favs) : [];
            } catch (_) {
                fm.favorites = [];
            }
        } catch (_) {}
    }

    function savePreferences() {
        try {
            localStorage.setItem(LS_VIEW_KEY, fm.viewMode);
            localStorage.setItem(LS_SORT_KEY, fm.sortBy + ':' + (fm.sortAsc ? 'asc' : 'desc'));
            localStorage.setItem('aurago.fm.previewOpen', fm.previewOpen);
        } catch (_) {}
    }

    function sortFiles(files) {
        const dirs = files.filter(f => f.type === 'directory');
        const regFiles = files.filter(f => f.type !== 'directory');
        const cmp = (a, b) => {
            let va, vb;
            if (fm.sortBy === 'size') { va = a.size || 0; vb = b.size || 0; }
            else if (fm.sortBy === 'date') { va = new Date(a.modified || 0).getTime(); vb = new Date(b.modified || 0).getTime(); }
            else if (fm.sortBy === 'type') { va = String(a.name || '').split('.').pop().toLowerCase(); vb = String(b.name || '').split('.').pop().toLowerCase(); }
            else { va = String(a.name || '').toLowerCase(); vb = String(b.name || '').toLowerCase(); }
            if (va < vb) return fm.sortAsc ? -1 : 1;
            if (va > vb) return fm.sortAsc ? 1 : -1;
            return 0;
        };
        dirs.sort(cmp);
        regFiles.sort(cmp);
        return [...dirs, ...regFiles];
    }

    function getDisplayFiles() {
        let list = fm.filteredFiles !== null ? fm.filteredFiles : fm.files;
        if (!fm.showHidden) {
            list = list.filter(f => !String(f.name || '').startsWith('.'));
        }
        list = sortFiles(list);
        return list;
    }

    function getSelectedFiles() {
        const list = getDisplayFiles();
        return list.filter(f => fm.selectedPaths.has(f.path));
    }

    function getSelectedSize() {
        return getSelectedFiles().reduce((sum, f) => sum + (f.size || 0), 0);
    }

    function addToHistory(path) {
        if (fm.historyIndex >= 0 && fm.history[fm.historyIndex] === path) return;
        fm.history = fm.history.slice(0, fm.historyIndex + 1);
        fm.history.push(path);
        fm.historyIndex = fm.history.length - 1;
        if (fm.history.length > 100) {
            fm.history.shift();
            fm.historyIndex--;
        }
    }

    function canGoBack() { return fm.historyIndex > 0; }
    function canGoForward() { return fm.historyIndex >= 0 && fm.historyIndex < fm.history.length - 1; }

    function goBack() {
        if (!canGoBack()) return;
        fm.historyIndex--;
        navigate(fm.history[fm.historyIndex], false);
    }

    function goForward() {
        if (!canGoForward()) return;
        fm.historyIndex++;
        navigate(fm.history[fm.historyIndex], false);
    }

    function goUp() {
        const parent = parentPath(fm.currentPath);
        if (parent !== fm.currentPath) navigate(parent);
    }

    async function navigate(path, addHistory = true) {
        if (!path && path !== '') path = '';
        fm.loading = true;
        fm.currentPath = path;
        fm.files = [];
        fm.filteredFiles = null;
        fm.selectedPaths.clear();
        fm.lastClickedPath = null;
        fm.renamePath = null;
        fm.searchQuery = '';
        if (addHistory) addToHistory(path);
        if (fm.callbacks && typeof fm.callbacks.onPathChange === 'function') {
            fm.callbacks.onPathChange(path);
        }
        renderAll();
        try {
            const result = await api('/api/desktop/files?path=' + encodeURIComponent(path));
            fm.files = Array.isArray(result.files) ? result.files : [];
        } catch (err) {
            showNotification({ type: 'error', message: t('desktop.fm.error_load', 'Failed to load files') + ': ' + (err.message || String(err)) });
            fm.files = [];
        }
        fm.loading = false;
        applyFilter();
        renderAll();
    }

    function applyFilter() {
        if (!fm.searchQuery.trim()) {
            fm.filteredFiles = null;
            return;
        }
        const q = fm.searchQuery.toLowerCase();
        fm.filteredFiles = fm.files.filter(f => String(f.name || '').toLowerCase().includes(q));
    }

    function refresh() {
        navigate(fm.currentPath, false);
        if (fm.callbacks && typeof fm.callbacks.refreshDesktop === 'function') {
            fm.callbacks.refreshDesktop();
        }
    }

    function render(host, windowId, initialPath, callbacks) {
        dispose(windowId);
        const instance = createInstance();
        setActiveInstance(instance);
        fm.host = host;
        fm.windowId = windowId;
        fm.callbacks = callbacks || {};
        fm.directories = Array.isArray(callbacks.directories) ? callbacks.directories : [];
        instances.set(windowId, instance);
        if (typeof fm.callbacks.wireContextMenuBoundary === 'function') fm.callbacks.wireContextMenuBoundary(host);
        loadPreferences();
        bindKeyboard();
        navigate(initialPath || '');
    }

    function renderAll() {
        if (!fm.host) return;
        if (typeof syncActiveTab === 'function') syncActiveTab();
        fm.incrementalRenderToken++;
        updateWindowMenus();
        fm.host.innerHTML = buildMarkup();
        attachEvents();
        const rootEl = fm.host.querySelector('.file-manager');
        if (rootEl) {
            if (typeof initPreviewResize === 'function' && fm.previewOpen) initPreviewResize(rootEl);
            if (typeof initColumnResize === 'function' && fm.viewMode === 'list') initColumnResize(rootEl);
            if (fm.splitViewEnabled && typeof initSplitResize === 'function') initSplitResize(rootEl);
        }
        scheduleIncrementalFileRender(rootEl);
        updateToolbarState();
        updateStatusBar();
    }

    function updateWindowMenus() {
        if (!fm.callbacks || typeof fm.callbacks.setWindowMenus !== 'function' || !fm.windowId) return;
        const selected = getSelectedFiles();
        const hasSelection = selected.length > 0;
        const hasClipboard = hasSharedFileClipboard();
        const readonly = isReadonly();
        const selectedFile = selected.length === 1 ? selected[0] : null;
        fm.callbacks.setWindowMenus(fm.windowId, [
            {
                id: 'file',
                labelKey: 'desktop.menu_file',
                items: [
                    { id: 'new-file', labelKey: 'desktop.fm.new_file', icon: 'file-plus', shortcut: 'Ctrl+N', disabled: readonly, action: () => createNewFile() },
                    { id: 'new-folder', labelKey: 'desktop.fm.new_folder', icon: 'folder-plus', disabled: readonly, action: () => createNewFolder() },
                    { id: 'upload', labelKey: 'desktop.fm.upload', icon: 'upload', disabled: readonly, action: () => uploadFiles() },
                    { type: 'separator' },
                    { id: 'duplicate', labelKey: 'desktop.fm.duplicate', shortcut: 'Ctrl+D', disabled: readonly || !hasSelection, action: () => duplicateSelected() },
                    { id: 'open-terminal', labelKey: 'desktop.fm.open_terminal', disabled: selected.length > 1, action: () => openTerminalHere(selectedFile && selectedFile.type === 'directory' ? selectedFile.path : fm.currentPath) },
                    { type: 'separator' },
                    { id: 'download', labelKey: 'desktop.fm.download', icon: 'download', disabled: !selectedFile || selectedFile.type !== 'file', action: () => selectedFile && downloadFile(selectedFile) },
                    { id: 'properties', labelKey: 'desktop.fm.properties', icon: 'info', disabled: !selectedFile, action: () => selectedFile && showProperties(selectedFile) }
                ]
            },
            {
                id: 'edit',
                labelKey: 'desktop.menu_edit',
                items: [
                    { id: 'cut', labelKey: 'desktop.fm.cut', icon: 'scissors', shortcut: 'Ctrl+X', disabled: readonly || !hasSelection, action: () => cutSelection() },
                    { id: 'copy', labelKey: 'desktop.fm.copy', icon: 'copy', shortcut: 'Ctrl+C', disabled: !hasSelection, action: () => copySelection() },
                    { id: 'copy-path', labelKey: 'desktop.fm.copy_path', shortcut: 'Ctrl+Shift+C', disabled: !hasSelection, action: () => copyPathToClipboard() },
                    { id: 'paste', labelKey: 'desktop.fm.paste', icon: 'clipboard', shortcut: 'Ctrl+V', disabled: readonly || !hasClipboard, action: () => pasteClipboard() },
                    { type: 'separator' },
                    { id: 'rename', labelKey: 'desktop.fm.rename', icon: 'edit', shortcut: 'F2', disabled: readonly || selected.length !== 1, action: () => selectedFile && startRename(selectedFile.path) },
                    { id: 'delete', labelKey: 'desktop.fm.delete', icon: 'trash', shortcut: 'Del', disabled: readonly || !hasSelection, action: () => deleteSelected() },
                    { type: 'separator' },
                    { id: 'select-all', labelKey: 'desktop.fm.select_all', icon: 'check-square', shortcut: 'Ctrl+A', action: () => selectAll() }
                ]
            },
            {
                id: 'view',
                labelKey: 'desktop.menu_view',
                items: [
                    { id: 'refresh', labelKey: 'desktop.fm.refresh', icon: 'refresh', shortcut: 'F5', action: () => refresh() },
                    { id: 'search', labelKey: 'desktop.search', icon: 'search', shortcut: 'Ctrl+F', action: () => toggleSearch() },
                    { type: 'separator' },
                    { id: 'view-grid', labelKey: 'desktop.fm.view_grid', icon: 'grid', checked: fm.viewMode === 'grid', action: () => { fm.viewMode = 'grid'; savePreferences(); renderAll(); } },
                    { id: 'view-list', labelKey: 'desktop.fm.view_list', icon: 'list', checked: fm.viewMode === 'list', action: () => { fm.viewMode = 'list'; savePreferences(); renderAll(); } },
                    { type: 'separator' },
                    { id: 'toggle-split', labelKey: 'desktop.fm.toggle_split', icon: 'columns', checked: fm.splitViewEnabled, action: () => toggleSplitView() },
                    { type: 'separator' },
                    {
                        id: 'sort',
                        labelKey: 'desktop.fm.sort_by',
                        icon: 'sort',
                        items: [
                            { id: 'sort-name', labelKey: 'desktop.fm.sort_name', checked: fm.sortBy === 'name', action: () => { fm.sortBy = 'name'; savePreferences(); renderFileContent(); } },
                            { id: 'sort-size', labelKey: 'desktop.fm.sort_size', checked: fm.sortBy === 'size', action: () => { fm.sortBy = 'size'; savePreferences(); renderFileContent(); } },
                            { id: 'sort-date', labelKey: 'desktop.fm.sort_date', checked: fm.sortBy === 'date', action: () => { fm.sortBy = 'date'; savePreferences(); renderFileContent(); } },
                            { id: 'sort-type', labelKey: 'desktop.fm.sort_type', checked: fm.sortBy === 'type', action: () => { fm.sortBy = 'type'; savePreferences(); renderFileContent(); } },
                            { type: 'separator' },
                            { id: 'sort-asc', labelKey: 'desktop.fm.sort_asc', checked: fm.sortAsc, action: () => { fm.sortAsc = true; savePreferences(); renderFileContent(); } },
                            { id: 'sort-desc', labelKey: 'desktop.fm.sort_desc', checked: !fm.sortAsc, action: () => { fm.sortAsc = false; savePreferences(); renderFileContent(); } }
                        ]
                    }
                ]
            }
        ]);
    }

    function focusFileItem(path) {
        if (!fm.host || !path) return;
        const root = fm.host.querySelector('.file-manager');
        if (!root) return;
        const item = Array.from(root.querySelectorAll('[data-path]')).find(node => node.dataset.path === path);
        if (item && typeof item.focus === 'function') {
            item.focus({ preventScroll: true });
        } else if (typeof root.focus === 'function') {
            root.focus({ preventScroll: true });
        }
    }

    function renderFileContent() {
        if (!fm.host) return;
        const root = fm.host.querySelector('.file-manager');
        const main = root && root.querySelector('[data-fm-main]');
        if (!root || !main) {
            return;
        }
        const nextMain = main.cloneNode(false);
        nextMain.innerHTML = renderContentHtml();
        main.replaceWith(nextMain);
        const status = root.querySelector('.fm-statusbar');
        if (status) status.outerHTML = renderStatusBarHtml();
        attachFileItemEvents(root);
        attachMainAreaEvents(root, false);
        scheduleIncrementalFileRender(root);
        updateToolbarState();
    }

    function updateSelectionDOM() {
        if (!fm.host) return;
        const root = fm.host.querySelector('.file-manager');
        if (!root) return;
        root.querySelectorAll('.fm-grid-item[data-path], .fm-list-row[data-path]').forEach(item => {
            const selected = fm.selectedPaths.has(item.dataset.path);
            item.classList.toggle('selected', selected);
            item.setAttribute('aria-selected', selected ? 'true' : 'false');
        });
        const status = root.querySelector('.fm-statusbar');
        if (status) status.outerHTML = renderStatusBarHtml();
        updateWindowMenus();
        updateToolbarState();
    }

    function buildMarkup() {
        const tabHtml = typeof renderTabBarHtml === 'function' ? renderTabBarHtml() : '';
        const previewHtml = fm.previewOpen && typeof renderPreviewPanelHtml === 'function' ? renderPreviewPanelHtml() : '';
        
        let bodyHtml = '';
        if (fm.splitViewEnabled) {
            let leftContentHtml = '';
            let rightContentHtml = '';
            
            const originalActivePane = fm.activePane;
            if (originalActivePane === 'right') {
                fm.activePane = 'left';
                loadPaneState(fm.leftPane);
                leftContentHtml = renderPaneHtml('left');
                
                fm.activePane = 'right';
                loadPaneState(fm.rightPane);
                rightContentHtml = renderPaneHtml('right');
            } else {
                leftContentHtml = renderPaneHtml('left');
                
                fm.activePane = 'right';
                loadPaneState(fm.rightPane);
                rightContentHtml = renderPaneHtml('right');
                
                fm.activePane = 'left';
                loadPaneState(fm.leftPane);
            }
            
            bodyHtml = `
                ${renderSidebarHtml()}
                <div class="fm-split-body" style="display: flex; flex: 1; min-height: 0; position: relative;">
                    ${leftContentHtml}
                    <div class="fm-split-resizer" style="width: 4px; background: var(--vd-border, #333); cursor: col-resize; z-index: 10;"></div>
                    ${rightContentHtml}
                </div>
                ${previewHtml}
            `;
        } else {
            bodyHtml = `
                ${renderSidebarHtml()}
                <div class="fm-content">
                    <div class="fm-main" data-fm-main>
                        ${renderContentHtml()}
                    </div>
                </div>
                ${previewHtml}
            `;
        }

        return `<div class="file-manager" data-fm-window="${esc(fm.windowId)}" ${fm.sidebarOpen ? 'data-sidebar-open="true"' : ''} ${isReadonly() ? 'data-readonly="true"' : ''} tabindex="-1">
            ${tabHtml}
            ${renderToolbarHtml()}
            ${renderSearchHtml()}
            <div class="fm-body">
                ${bodyHtml}
            </div>
            ${renderStatusBarHtml()}
            ${renderDropOverlayHtml()}
        </div>`;
    }

    function renderPaneHtml(paneName) {
        const isActive = fm.activePane === paneName ? ' active' : '';
        return `<div class="fm-pane fm-content${isActive}" data-pane="${paneName}" style="display: flex; flex: 1; flex-direction: column; min-width: 0; position: relative;">
            <div class="fm-pane-header" style="padding: 4px 8px; font-size: 0.75rem; background: rgba(0,0,0,0.2); border-bottom: 1px solid var(--vd-border, #333); display: flex; align-items: center; justify-content: space-between;">
                <span class="fm-pane-path" style="overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: bold; color: ${fm.activePane === paneName ? 'var(--vd-accent, #27c7a6)' : 'var(--vd-muted, #888)'};">${esc(fm.currentPath || '/')}</span>
            </div>
            <div class="fm-main" data-fm-main style="flex: 1; overflow-y: auto;">
                ${renderContentHtml()}
            </div>
        </div>`;
    }

    function createPaneState(path, history = [], historyIndex = -1) {
        return {
            currentPath: path || '',
            files: [],
            filteredFiles: null,
            selectedPaths: new Set(),
            history: history,
            historyIndex: historyIndex,
            searchQuery: '',
            lastClickedPath: null,
            renamePath: null,
            dragOverPath: null,
            scrollPosition: 0
        };
    }

    function toggleSplitView() {
        if (!fm.splitViewEnabled) {
            fm.splitViewEnabled = true;
            fm.activePane = 'left';
            
            fm.leftPane = createPaneState(fm.currentPath, [...fm.history], fm.historyIndex);
            fm.leftPane.files = [...fm.files];
            fm.leftPane.filteredFiles = fm.filteredFiles ? [...fm.filteredFiles] : null;
            fm.leftPane.selectedPaths = new Set(fm.selectedPaths);
            fm.leftPane.lastClickedPath = fm.lastClickedPath;
            
            fm.rightPane = createPaneState(fm.currentPath, [...fm.history], fm.historyIndex);
            fm.rightPane.files = [...fm.files];
            fm.rightPane.filteredFiles = fm.filteredFiles ? [...fm.filteredFiles] : null;
            
            loadPaneState(fm.leftPane);
        } else {
            const activeState = fm.activePane === 'right' ? fm.rightPane : fm.leftPane;
            fm.splitViewEnabled = false;
            if (activeState) {
                fm.currentPath = activeState.currentPath;
                fm.files = [...activeState.files];
                fm.filteredFiles = activeState.filteredFiles ? [...activeState.filteredFiles] : null;
                fm.selectedPaths = new Set(activeState.selectedPaths);
                fm.history = [...activeState.history];
                fm.historyIndex = activeState.historyIndex;
                fm.searchQuery = activeState.searchQuery;
                fm.lastClickedPath = activeState.lastClickedPath;
                fm.renamePath = activeState.renamePath;
            }
            fm.leftPane = null;
            fm.rightPane = null;
        }
        renderAll();
    }

    function saveActivePaneState() {
        if (!fm.splitViewEnabled) return;
        const pane = fm.activePane === 'right' ? fm.rightPane : fm.leftPane;
        if (!pane) return;
        
        pane.currentPath = fm.currentPath;
        pane.files = [...fm.files];
        pane.filteredFiles = fm.filteredFiles ? [...fm.filteredFiles] : null;
        pane.selectedPaths = new Set(fm.selectedPaths);
        pane.history = [...fm.history];
        pane.historyIndex = fm.historyIndex;
        pane.searchQuery = fm.searchQuery;
        pane.lastClickedPath = fm.lastClickedPath;
        pane.renamePath = fm.renamePath;
        
        const main = fm.host ? fm.host.querySelector(`.fm-pane[data-pane="${fm.activePane}"] [data-fm-main]`) : null;
        pane.scrollPosition = main ? main.scrollTop : 0;
    }

    function loadPaneState(pane) {
        if (!pane) return;
        fm.currentPath = pane.currentPath;
        fm.files = [...pane.files];
        fm.filteredFiles = pane.filteredFiles ? [...pane.filteredFiles] : null;
        fm.selectedPaths = new Set(pane.selectedPaths);
        fm.history = [...pane.history];
        fm.historyIndex = pane.historyIndex;
        fm.searchQuery = pane.searchQuery;
        fm.lastClickedPath = pane.lastClickedPath;
        fm.renamePath = pane.renamePath;
        
        const searchInput = fm.host ? fm.host.querySelector('.fm-search-input') : null;
        if (searchInput) searchInput.value = pane.searchQuery || '';
    }

    function switchActivePane(paneName) {
        if (!fm.splitViewEnabled || fm.activePane === paneName) return;
        
        saveActivePaneState();
        fm.activePane = paneName;
        const nextPane = paneName === 'right' ? fm.rightPane : fm.leftPane;
        loadPaneState(nextPane);
        
        if (fm.host) {
            fm.host.querySelectorAll('.fm-pane').forEach(pane => {
                if (pane.dataset.pane === paneName) {
                    pane.classList.add('active');
                    const pathSpan = pane.querySelector('.fm-pane-path');
                    if (pathSpan) pathSpan.style.color = 'var(--vd-accent, #27c7a6)';
                } else {
                    pane.classList.remove('active');
                    const pathSpan = pane.querySelector('.fm-pane-path');
                    if (pathSpan) pathSpan.style.color = 'var(--vd-muted, #888)';
                }
            });
            
            const breadcrumbWrap = fm.host.querySelector('[data-fm-breadcrumb]');
            if (breadcrumbWrap) breadcrumbWrap.innerHTML = renderBreadcrumbSegments();
            
            const statusBar = fm.host.querySelector('.fm-statusbar');
            if (statusBar) {
                const tempDiv = document.createElement('div');
                tempDiv.innerHTML = renderStatusBarHtml();
                statusBar.replaceWith(tempDiv.firstChild);
            }
            
            const sidebar = fm.host.querySelector('.fm-sidebar');
            if (sidebar) {
                const tempDiv = document.createElement('div');
                tempDiv.innerHTML = renderSidebarHtml();
                sidebar.replaceWith(tempDiv.firstChild);
            }
            
            const searchInput = fm.host.querySelector('.fm-search-input');
            if (searchInput) {
                searchInput.value = fm.searchQuery || '';
            }
            
            attachEvents();
        }
    }

    function initSplitResize(root) {
        const resizer = root.querySelector('.fm-split-resizer');
        if (!resizer) return;
        
        const leftPane = root.querySelector('.fm-pane[data-pane="left"]');
        const rightPane = root.querySelector('.fm-pane[data-pane="right"]');
        if (!leftPane || !rightPane) return;
        
        let startX = 0;
        let startLeftWidth = 0;
        let containerWidth = 0;
        
        function onPointerMove(e) {
            const dx = e.clientX - startX;
            const newLeftWidth = Math.max(100, Math.min(containerWidth - 100, startLeftWidth + dx));
            const leftPercent = (newLeftWidth / containerWidth) * 100;
            leftPane.style.flex = `none`;
            leftPane.style.width = `${leftPercent}%`;
            rightPane.style.flex = `none`;
            rightPane.style.width = `${100 - leftPercent}%`;
        }
        
        function onPointerUp() {
            document.removeEventListener('pointermove', onPointerMove);
            document.removeEventListener('pointerup', onPointerUp);
            localStorage.setItem('aurago.fm.splitRatio', leftPane.style.width);
        }
        
        resizer.addEventListener('pointerdown', e => {
            e.preventDefault();
            startX = e.clientX;
            startLeftWidth = leftPane.offsetWidth;
            containerWidth = resizer.parentElement.offsetWidth;
            document.addEventListener('pointermove', onPointerMove);
            document.addEventListener('pointerup', onPointerUp);
        });
        
        const savedRatio = localStorage.getItem('aurago.fm.splitRatio');
        if (savedRatio) {
            leftPane.style.flex = 'none';
            leftPane.style.width = savedRatio;
            rightPane.style.flex = 'none';
            const ratioVal = parseFloat(savedRatio);
            rightPane.style.width = `${100 - ratioVal}%`;
        }
    }

    function renderToolbarHtml() {
        const backDisabled = !canGoBack() ? ' disabled' : '';
        const fwdDisabled = !canGoForward() ? ' disabled' : '';
        return `<div class="fm-toolbar">
            <div class="fm-toolbar-group">
                <button type="button" class="fm-toolbtn fm-sidebar-toggle" data-action="sidebar-toggle" title="${esc(t('desktop.fm.toggle_sidebar', 'Toggle sidebar'))}" aria-label="${esc(t('desktop.fm.toggle_sidebar', 'Toggle sidebar'))}">
                    ${iconMarkup('list', '\u2630', '', 16)}
                </button>
                <button type="button" class="fm-toolbtn" data-action="back" title="${esc(t('desktop.back'))}"${backDisabled}>
                    ${iconMarkup('chevron-left', '\u2039', '', 16)}
                </button>
                <button type="button" class="fm-toolbtn" data-action="forward" title="${esc(t('desktop.forward'))}"${fwdDisabled}>
                    ${iconMarkup('chevron-right', '\u203A', '', 16)}
                </button>
                <button type="button" class="fm-toolbtn" data-action="up" title="${esc(t('desktop.up', 'Up'))}">
                    ${iconMarkup('arrow-up', '\u2191', '', 16)}
                </button>
            </div>
            <div class="fm-toolbar-group fm-breadcrumb-wrap" data-fm-breadcrumb>
                ${renderBreadcrumbSegments()}
            </div>
            <div class="fm-toolbar-group fm-toolbar-right">
                <button type="button" class="fm-toolbtn${fm.previewOpen ? ' active' : ''}" data-action="toggle-preview" title="${esc(t('desktop.fm.toggle_preview', 'Toggle Preview Panel (Ctrl+P)'))}">
                    ${iconMarkup('layout', '\u25EB', '', 16)}
                </button>
                <button type="button" class="fm-toolbtn${fm.splitViewEnabled ? ' active' : ''}" data-action="toggle-split" title="${esc(t('desktop.fm.toggle_split', 'Toggle Split View (Alt+S)'))}">
                    ${iconMarkup('columns', '\u25EB', '', 16)}
                </button>
                <button type="button" class="fm-toolbtn${fm.showHidden ? ' active' : ''}" data-action="toggle-hidden" title="${esc(t('desktop.fm.toggle_hidden', 'Show/Hide Hidden Files (Ctrl+H)'))}">
                    ${iconMarkup(fm.showHidden ? 'eye' : 'eye-off', fm.showHidden ? '\uD83D\uDC41' : '\uD83D\uDC41\u0338', '', 16)}
                </button>
                <button type="button" class="fm-toolbtn${fm.viewMode === 'grid' ? ' active' : ''}" data-action="view-grid" title="${esc(t('desktop.fm.view_grid', 'Grid View'))}">
                    ${iconMarkup('grid', '\u25A6', '', 16)}
                </button>
                <button type="button" class="fm-toolbtn${fm.viewMode === 'list' ? ' active' : ''}" data-action="view-list" title="${esc(t('desktop.fm.view_list', 'List View'))}">
                    ${iconMarkup('list', '\u2630', '', 16)}
                </button>
                <button type="button" class="fm-toolbtn" data-action="search-toggle" title="${esc(t('desktop.search'))} (Ctrl+F)">
                    ${iconMarkup('search', '\u2315', '', 16)}
                </button>
            </div>
        </div>`;
    }

    function renderSearchHtml() {
        const hidden = fm.searchQuery ? '' : ' hidden';
        return `<div class="fm-search-bar" data-fm-search${hidden}>
            ${iconMarkup('search', '\u2315', 'fm-search-icon', 14)}
            <input type="text" class="fm-search-input" placeholder="${esc(t('desktop.fm.search_placeholder', 'Search files...'))}" value="${esc(fm.searchQuery)}">
            <button type="button" class="fm-search-clear" data-action="search-clear">${iconMarkup('x', '\u00D7', '', 14)}</button>
        </div>`;
    }

    function renderSidebarHtml() {
        const items = fm.directories.map(dir => {
            const isActive = fm.currentPath === dir || fm.currentPath.startsWith(dir + '/');
            const iconKey = iconForDirectory(dir);
            return `<div class="fm-sidebar-item${isActive ? ' active' : ''}" data-sidebar-path="${esc(dir)}" role="button" tabindex="0">
                ${iconMarkup(iconKey, '\u25A0', 'fm-sidebar-icon', 18)}
                <span class="fm-sidebar-label">${esc(baseName(dir) || dir)}</span>
            </div>`;
        }).join('');

        const favs = fm.favorites || [];
        const favItems = favs.map((fav, idx) => {
            const isActive = fm.currentPath === fav || fm.currentPath.startsWith(fav + '/');
            return `<div class="fm-sidebar-item${isActive ? ' active' : ''} fm-favorite-item" data-sidebar-path="${esc(fav)}" draggable="true" data-fav-index="${idx}" role="button" tabindex="0" style="position:relative">
                ${iconMarkup('star', '\u2605', 'fm-sidebar-icon', 18)}
                <span class="fm-sidebar-label">${esc(baseName(fav) || fav)}</span>
                <button type="button" class="fm-favorite-remove" data-action="remove-favorite" data-path="${esc(fav)}" title="${esc(t('desktop.fm.remove_favorite', 'Remove from Favorites'))}" style="position:absolute;right:8px;background:none;border:none;color:var(--vd-muted);cursor:pointer;font-size:1rem;line-height:1;display:none;align-items:center;justify-content:center;height:100%;top:0">&times;</button>
            </div>`;
        }).join('');

        return `<aside class="fm-sidebar">
            <div class="fm-sidebar-section">
                <div class="fm-sidebar-head">${esc(t('desktop.fm.quick_access', 'Quick Access'))}</div>
                ${items || `<div class="fm-sidebar-empty">${esc(t('desktop.fm.workspace_root', 'Workspace'))}</div>`}
            </div>
            <div class="fm-sidebar-section fm-favorites-section">
                <div class="fm-sidebar-head">${esc(t('desktop.fm.favorites', 'Favorites'))}</div>
                ${favItems || `<div class="fm-sidebar-empty" style="font-size:0.7rem;color:var(--vd-muted);padding:8px 12px">${esc(t('desktop.fm.drag_favorite_hint', 'Drag folders here'))}</div>`}
            </div>
        </aside>`;
    }

    function renderBreadcrumbSegments() {
        const parts = fm.currentPath.split('/').filter(Boolean);
        let accumulated = '';
        const segments = parts.map((part, index) => {
            accumulated = joinPath(accumulated, part);
            const isLast = index === parts.length - 1;
            return `<span class="fm-breadcrumb-segment${isLast ? ' current' : ''}" data-breadcrumb-path="${esc(accumulated)}" role="button" tabindex="0">${esc(part)}</span>`;
        });
        return segments.join(`<span class="fm-breadcrumb-separator">/</span>`);
    }

    function renderContentHtml() {
        if (fm.loading) {
            return `<div class="fm-loading"><div class="fm-spinner"></div><div>${esc(t('desktop.fm.loading', 'Loading...'))}</div></div>`;
        }
        const files = getDisplayFiles();
        if (!files.length) {
            if (fm.filteredFiles !== null && fm.searchQuery) {
                return `<div class="fm-empty">${iconMarkup('search', '\u2315', 'fm-empty-icon', 32)}<div>${esc(t('desktop.fm.no_results', 'No files match "{{query}}"', { query: fm.searchQuery }))}</div></div>`;
            }
            return `<div class="fm-empty">${iconMarkup('folder-open', '\u25A1', 'fm-empty-icon', 32)}<div>${esc(t('desktop.fm.empty_folder', 'This folder is empty'))}</div></div>`;
        }
        const renderFiles = files.length > FILE_INCREMENTAL_THRESHOLD ? files.slice(0, FILE_RENDER_BATCH_SIZE) : files;
        const incrementalAttr = files.length > FILE_INCREMENTAL_THRESHOLD ? ` data-fm-incremental="${esc(String(files.length))}"` : '';
        if (fm.viewMode === 'grid') {
            return `<div class="fm-grid"${incrementalAttr}>${renderFiles.map(f => renderGridItem(f)).join('')}</div>`;
        }
        return `<div class="fm-list">
            <div class="fm-list-header">
                <div class="fm-list-cell fm-col-name" data-sort="name" role="button" tabindex="0" style="position:relative">
                    ${esc(t('desktop.fm.sort_name', 'Name'))}
                    ${fm.sortBy === 'name' ? iconMarkup(fm.sortAsc ? 'chevron-up' : 'chevron-down', fm.sortAsc ? '\u2191' : '\u2193', 'fm-sort-indicator', 12) : ''}
                    <div class="fm-col-resize-handle" data-col-key="name"></div>
                </div>
                <div class="fm-list-cell fm-col-size" data-sort="size" role="button" tabindex="0" style="position:relative">
                    ${esc(t('desktop.fm.sort_size', 'Size'))}
                    ${fm.sortBy === 'size' ? iconMarkup(fm.sortAsc ? 'chevron-up' : 'chevron-down', fm.sortAsc ? '\u2191' : '\u2193', 'fm-sort-indicator', 12) : ''}
                    <div class="fm-col-resize-handle" data-col-key="size"></div>
                </div>
                <div class="fm-list-cell fm-col-date" data-sort="date" role="button" tabindex="0" style="position:relative">
                    ${esc(t('desktop.fm.sort_date', 'Date Modified'))}
                    ${fm.sortBy === 'date' ? iconMarkup(fm.sortAsc ? 'chevron-up' : 'chevron-down', fm.sortAsc ? '\u2191' : '\u2193', 'fm-sort-indicator', 12) : ''}
                    <div class="fm-col-resize-handle" data-col-key="date"></div>
                </div>
                <div class="fm-list-cell fm-col-type" data-sort="type" role="button" tabindex="0" style="position:relative">
                    ${esc(t('desktop.fm.sort_type', 'Type'))}
                    ${fm.sortBy === 'type' ? iconMarkup(fm.sortAsc ? 'chevron-up' : 'chevron-down', fm.sortAsc ? '\u2191' : '\u2193', 'fm-sort-indicator', 12) : ''}
                    <div class="fm-col-resize-handle" data-col-key="type"></div>
                </div>
            </div>
            <div class="fm-list-body"${incrementalAttr}>${renderFiles.map(f => renderListRow(f)).join('')}</div>
        </div>`;
    }

    function scheduleIncrementalFileRender(root) {
        if (!root) return;
        const files = getDisplayFiles();
        if (files.length <= FILE_INCREMENTAL_THRESHOLD) return;
        const target = root.querySelector('[data-fm-incremental]');
        if (!target) return;
        const token = ++fm.incrementalRenderToken;
        let index = FILE_RENDER_BATCH_SIZE;
        const schedule = window.requestAnimationFrame || ((callback) => window.setTimeout(callback, 16));
        function pump() {
            if (token !== fm.incrementalRenderToken || !fm.host || !target.isConnected) return;
            const chunk = files.slice(index, index + FILE_RENDER_BATCH_SIZE);
            if (chunk.length) {
                const html = chunk.map(file => fm.viewMode === 'grid' ? renderGridItem(file) : renderListRow(file)).join('');
                target.insertAdjacentHTML('beforeend', html);
                attachFileItemEvents(root);
            }
            index += chunk.length;
            if (index < files.length) schedule(pump);
            else delete target.dataset.fmIncremental;
        }
        schedule(pump);
    }

    function renderGridItem(file) {
        const isDir = file.type === 'directory';
        const iconKey = isDir ? iconForDirectory(file.name) : iconForFile(file);
        const selected = fm.selectedPaths.has(file.path) ? ' selected' : '';
        const cut = (fm.clipboard && fm.clipboard.mode === 'cut' && fm.clipboard.paths.includes(file.path)) ? ' cut-item' : '';
        const preview = !isDir && isPreviewableImage(file);
        const nameContent = fm.renamePath === file.path
            ? `<input class="fm-rename-input" data-rename-input value="${esc(file.name)}" aria-label="${esc(t('desktop.fm.rename', 'Rename'))}">`
            : esc(file.name);
        return `<div class="fm-grid-item${selected}${cut}" data-path="${esc(file.path)}" data-type="${esc(file.type)}" role="button" tabindex="0" title="${esc(file.name)}">
            <div class="fm-grid-icon${preview ? ' has-preview' : ''}">${thumbnailMarkup(file, iconKey, isDir ? '\u25A0' : '\u25A1', 'grid')}</div>
            <div class="fm-grid-name">${nameContent}</div>
        </div>`;
    }

    function renderListRow(file) {
        const isDir = file.type === 'directory';
        const iconKey = isDir ? iconForDirectory(file.name) : iconForFile(file);
