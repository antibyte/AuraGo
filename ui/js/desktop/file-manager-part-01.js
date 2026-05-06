(function () {
    'use strict';

    const LS_VIEW_KEY = 'aurago.fm.view';
    const LS_SORT_KEY = 'aurago.fm.sort';
    const PREVIEW_IMAGE_EXTS = new Set(['avif', 'bmp', 'gif', 'jpeg', 'jpg', 'png', 'webp']);
    const PREVIEW_IMAGE_MIMES = new Set(['image/avif', 'image/bmp', 'image/gif', 'image/jpeg', 'image/png', 'image/webp']);

    const fm = {
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
    };

    function t(key, fallback, vars) {
        if (fallback && typeof fallback === 'object' && !Array.isArray(fallback)) {
            vars = fallback;
            fallback = '';
        }
        if (fm.callbacks && typeof fm.callbacks.t === 'function') {
            const translated = fm.callbacks.t(key, vars || {});
            if (translated && translated !== key) return translated;
        }
        let text = fallback || key;
        Object.entries(vars || {}).forEach(([name, value]) => {
            text = text.replaceAll('{{' + name + '}}', String(value));
            text = text.replaceAll('{' + name + '}', String(value));
        });
        return text;
    }

    function esc(value) {
        if (fm.callbacks && typeof fm.callbacks.esc === 'function') {
            return fm.callbacks.esc(value);
        }
        return String(value == null ? '' : value)
            .replaceAll('&', '&amp;')
            .replaceAll('<', '&lt;')
            .replaceAll('>', '&gt;')
            .replaceAll('"', '&quot;')
            .replaceAll("'", '&#39;');
    }

    function api(url, options) {
        if (fm.callbacks && typeof fm.callbacks.api === 'function') {
            return fm.callbacks.api(url, options);
        }
        const opts = Object.assign({ credentials: 'same-origin', headers: { 'Content-Type': 'application/json' } }, options || {});
        if (opts.body instanceof FormData) delete opts.headers;
        return fetch(url, opts).then(async res => {
            if (!res.ok) {
                let message = res.statusText;
                try { const body = await res.json(); message = body.error || body.message || message; } catch (_) { message = await res.text() || message; }
                throw new Error(message);
            }
            return res.json();
        });
    }

    function isReadonly() {
        return !!(fm.callbacks && fm.callbacks.readonly);
    }

    function maxFileSize() {
        const value = Number(fm.callbacks && fm.callbacks.maxFileSize);
        return Number.isFinite(value) && value > 0 ? value : 0;
    }

    function isTouchLikePointer(event) {
        if (event && (event.pointerType === 'touch' || event.pointerType === 'pen')) return true;
        if (window.matchMedia && window.matchMedia('(hover: none) and (pointer: coarse)').matches) return true;
        return !!(window.matchMedia && window.matchMedia('(max-width: 820px)').matches);
    }

    function wireLongPress(element, callback, options) {
        options = options || {};
        const threshold = Number(options.threshold || 600);
        const feedbackDelay = Number(options.feedbackDelay || 300);
        const moveTolerance = Number(options.moveTolerance || 10);
        let timer = 0;
        let feedbackTimer = 0;
        let startX = 0;
        let startY = 0;
        let pointerId = null;
        let triggered = false;
        let suppressClick = false;

        function clearTimers() {
            if (timer) window.clearTimeout(timer);
            if (feedbackTimer) window.clearTimeout(feedbackTimer);
            timer = 0;
            feedbackTimer = 0;
        }

        function clearPress() {
            clearTimers();
            element.classList.remove('vd-long-press-active');
            pointerId = null;
            triggered = false;
        }

        element.addEventListener('pointerdown', event => {
            if (event.button !== 0 || !isTouchLikePointer(event)) return;
            clearTimers();
            startX = event.clientX;
            startY = event.clientY;
            pointerId = event.pointerId;
            triggered = false;
            feedbackTimer = window.setTimeout(() => {
                element.classList.add('vd-long-press-active');
            }, feedbackDelay);
            timer = window.setTimeout(() => {
                triggered = true;
                suppressClick = true;
                element.classList.add('vd-long-press-active');
                event.preventDefault();
                event.stopPropagation();
                callback(event);
            }, threshold);
        });

        element.addEventListener('pointermove', event => {
            if (!timer || pointerId !== event.pointerId) return;
            if (Math.abs(event.clientX - startX) > moveTolerance || Math.abs(event.clientY - startY) > moveTolerance) {
                clearPress();
            }
        });

        element.addEventListener('pointerup', event => {
            if (pointerId !== event.pointerId) return;
            if (triggered) {
                event.preventDefault();
                event.stopPropagation();
            }
            clearPress();
        });
        element.addEventListener('pointercancel', clearPress);
        element.addEventListener('click', event => {
            if (!suppressClick) return;
            suppressClick = false;
            event.preventDefault();
            event.stopPropagation();
        }, true);
    }

    function iconMarkup(key, fallback, className, size) {
        if (fm.callbacks && typeof fm.callbacks.iconMarkup === 'function') {
            return fm.callbacks.iconMarkup(key, fallback, className, size);
        }
        const pixels = Number(size || 16) || 16;
        return `<span class="${esc(className || '')}" style="font-size:${pixels}px">${esc(fallback || key || '')}</span>`;
    }

    function iconForFile(file) {
        if (fm.callbacks && typeof fm.callbacks.iconForFile === 'function') {
            return fm.callbacks.iconForFile(file);
        }
        const ext = String(file.name || '').split('.').pop().toLowerCase();
        const map = {
            go: 'file-go', js: 'file-js', ts: 'file-js', mjs: 'file-js', jsx: 'file-js', tsx: 'file-js',
            py: 'file-py', rs: 'file-rs', json: 'file-json', yaml: 'file-yaml', yml: 'file-yaml',
            md: 'file-md', html: 'file-html', htm: 'file-html', css: 'file-css', scss: 'file-css', sass: 'file-css',
            png: 'file-image', jpg: 'file-image', jpeg: 'file-image', gif: 'file-image', svg: 'file-image', webp: 'file-image',
            mp4: 'file-video', mkv: 'file-video', avi: 'file-video', mov: 'file-video',
            mp3: 'file-audio', wav: 'file-audio', flac: 'file-audio', ogg: 'file-audio',
            zip: 'file-archive', tar: 'file-archive', gz: 'file-archive', rar: 'file-archive', '7z': 'file-archive',
            pdf: 'file-pdf', doc: 'file-doc', docx: 'file-doc', xls: 'file-xls', xlsx: 'file-xls', ppt: 'file-ppt', pptx: 'file-ppt',
            txt: 'file-text', log: 'file-text', csv: 'file-csv', sql: 'file-sql', dockerfile: 'file-docker',
            sh: 'file-shell', bash: 'file-shell', ps1: 'file-shell', zsh: 'file-shell',
            c: 'file-c', cpp: 'file-cpp', h: 'file-c', hpp: 'file-cpp', cs: 'file-csharp', java: 'file-java', kt: 'file-kotlin',
            php: 'file-php', rb: 'file-ruby', swift: 'file-swift',
        };
        return map[ext] || 'file';
    }

    function fileExt(name) {
        const parts = String(name || '').split('.');
        return parts.length > 1 ? parts.pop().toLowerCase() : '';
    }

    function isPreviewableImage(file) {
        if (!file || file.type !== 'file') return false;
        const mime = String(file.mime_type || '').toLowerCase();
        if (mime && PREVIEW_IMAGE_MIMES.has(mime)) return true;
        return PREVIEW_IMAGE_EXTS.has(fileExt(file.name));
    }

    function previewURL(file) {
        return '/api/desktop/preview?path=' + encodeURIComponent(file.path || '');
    }

    function thumbnailMarkup(file, iconKey, fallback, mode) {
        const icon = iconMarkup(iconKey, fallback, 'fm-thumb-fallback-icon', mode === 'grid' ? 38 : 18);
        if (!isPreviewableImage(file)) return icon;
        return `<span class="fm-thumb fm-thumb-${esc(mode || 'grid')}" aria-hidden="true">
            <img src="${esc(previewURL(file))}" loading="lazy" decoding="async" alt="">
            <span class="fm-thumb-fallback">${icon}</span>
        </span>`;
    }

    function iconForDirectory(name) {
        if (fm.callbacks && typeof fm.callbacks.iconForDirectory === 'function') {
            return fm.callbacks.iconForDirectory(name);
        }
        const lower = String(name || '').toLowerCase();
        if (lower === 'desktop') return 'folder-desktop';
        if (lower === 'documents') return 'folder-documents';
        if (lower === 'downloads') return 'folder-downloads';
        if (lower === 'pictures' || lower === 'images') return 'folder-pictures';
        if (lower === 'music' || lower === 'audio') return 'folder-music';
        if (lower === 'videos' || lower === 'movies') return 'folder-videos';
        if (lower === 'src' || lower === 'source') return 'folder-src';
        if (lower === 'dist' || lower === 'build' || lower === 'out') return 'folder-build';
        if (lower === 'node_modules') return 'folder-npm';
        if (lower === '.git') return 'folder-git';
        if (lower === 'config' || lower === '.config') return 'folder-config';
        if (lower === 'public') return 'folder-public';
        if (lower === 'assets') return 'folder-assets';
        if (lower === 'templates' || lower === 'views') return 'folder-templates';
        if (lower === 'scripts' || lower === 'bin') return 'folder-scripts';
        if (lower === 'test' || lower === 'tests') return 'folder-tests';
        if (lower === '.github') return 'folder-github';
        if (lower === 'workflows') return 'folder-workflows';
        if (lower === 'ui' || lower === 'www' || lower === 'web') return 'folder-ui';
        if (lower === 'internal') return 'folder-internal';
        if (lower === 'cmd') return 'folder-cmd';
        if (lower === 'api') return 'folder-api';
        if (lower === 'pkg') return 'folder-pkg';
        if (lower === 'data') return 'folder-data';
        if (lower === 'db' || lower === 'database' || lower === 'migrations') return 'folder-db';
        if (lower === 'deploy' || lower === 'deployment') return 'folder-deploy';
        if (lower === 'docs' || lower === 'documentation') return 'folder-docs';
        if (lower === 'reports') return 'folder-reports';
        if (lower === 'tools') return 'folder-tools';
        if (lower === 'lib' || lower === 'libs' || lower === 'vendor') return 'folder-lib';
        if (lower === 'agent_workspace' || lower === 'workspace') return 'folder-workspace';
        if (lower === 'logs') return 'folder-logs';
        if (lower === 'secrets' || lower === 'vault') return 'folder-secrets';
        if (lower === 'media') return 'folder-media';
        if (lower === 'backups') return 'folder-backups';
        if (lower === 'tmp' || lower === 'temp') return 'folder-temp';
        return 'folder';
    }

    function contextIconGlyph(icon) {
        const map = {
            'check-square': '\u2713',
            clipboard: '\u2398',
            copy: '\u2398',
            download: '\u2193',
            edit: '\u270e',
            'file-plus': '+',
            'folder-open': '\u25a1',
            'folder-plus': '+',
            info: 'i',
            refresh: '\u21bb',
            scissors: '\u2702',
            sort: '\u2195',
            trash: '\u00d7',
        };
        return map[icon] || icon || '';
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
            const converted = items.map(item => {
                if (item.separator) return item;
                const converted = { label: item.label, icon: contextIconGlyph(item.icon) };
                if (item.disabled) converted.disabled = true;
                converted.action = typeof item.handler === 'function' ? item.handler : (typeof item.action === 'function' ? item.action : () => {});
                return converted;
            });
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
                }
            };
            document.addEventListener('mousedown', closeHandler);
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
        } catch (_) {}
    }

    function savePreferences() {
        try {
            localStorage.setItem(LS_VIEW_KEY, fm.viewMode);
            localStorage.setItem(LS_SORT_KEY, fm.sortBy + ':' + (fm.sortAsc ? 'asc' : 'desc'));
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
        fm.host = host;
        fm.windowId = windowId;
        fm.callbacks = callbacks || {};
        fm.directories = Array.isArray(callbacks.directories) ? callbacks.directories : [];
        if (typeof fm.callbacks.wireContextMenuBoundary === 'function') fm.callbacks.wireContextMenuBoundary(host);
        loadPreferences();
        bindKeyboard();
        navigate(initialPath || '');
    }

    function renderAll() {
        if (!fm.host) return;
        updateWindowMenus();
        fm.host.innerHTML = buildMarkup();
        attachEvents();
        updateToolbarState();
        updateStatusBar();
    }

    function updateWindowMenus() {
        if (!fm.callbacks || typeof fm.callbacks.setWindowMenus !== 'function' || !fm.windowId) return;
        const selected = getSelectedFiles();
        const hasSelection = selected.length > 0;
        const hasClipboard = fm.clipboard && fm.clipboard.paths && fm.clipboard.paths.length > 0;
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
            renderAll();
            return;
        }
        const nextMain = main.cloneNode(false);
        nextMain.innerHTML = renderContentHtml();
        main.replaceWith(nextMain);
        const status = root.querySelector('.fm-statusbar');
        if (status) status.outerHTML = renderStatusBarHtml();
        attachFileItemEvents(root);
        attachMainAreaEvents(root, false);
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
        return `<div class="file-manager" data-fm-window="${esc(fm.windowId)}" ${fm.sidebarOpen ? 'data-sidebar-open="true"' : ''} ${isReadonly() ? 'data-readonly="true"' : ''} tabindex="-1">
            ${renderToolbarHtml()}
            ${renderSearchHtml()}
            <div class="fm-body">
                ${renderSidebarHtml()}
                <div class="fm-content">
                    <div class="fm-main" data-fm-main>
                        ${renderContentHtml()}
                    </div>
                </div>
            </div>
            ${renderStatusBarHtml()}
            ${renderDropOverlayHtml()}
        </div>`;
    }

    function renderToolbarHtml() {
        const backDisabled = !canGoBack() ? ' disabled' : '';
        const fwdDisabled = !canGoForward() ? ' disabled' : '';
        return `<div class="fm-toolbar">
            <div class="fm-toolbar-group">
                <button type="button" class="fm-toolbtn fm-sidebar-toggle" data-action="sidebar-toggle" title="${esc(t('desktop.fm.toggle_sidebar', 'Toggle sidebar'))}" aria-label="${esc(t('desktop.fm.toggle_sidebar', 'Toggle sidebar'))}">
                    ${iconMarkup('list', '\u2630', '', 16)}
                </button>
                <button type="button" class="fm-toolbtn" data-action="back" title="${esc(t('desktop.back', 'Back'))}"${backDisabled}>
                    ${iconMarkup('chevron-left', '\u2039', '', 16)}
                </button>
                <button type="button" class="fm-toolbtn" data-action="forward" title="${esc(t('desktop.forward', 'Forward'))}"${fwdDisabled}>
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
                <button type="button" class="fm-toolbtn${fm.viewMode === 'grid' ? ' active' : ''}" data-action="view-grid" title="${esc(t('desktop.fm.view_grid', 'Grid View'))}">
                    ${iconMarkup('grid', '\u25A6', '', 16)}
                </button>
                <button type="button" class="fm-toolbtn${fm.viewMode === 'list' ? ' active' : ''}" data-action="view-list" title="${esc(t('desktop.fm.view_list', 'List View'))}">
                    ${iconMarkup('list', '\u2630', '', 16)}
                </button>
                <button type="button" class="fm-toolbtn" data-action="search-toggle" title="${esc(t('desktop.search', 'Search'))} (Ctrl+F)">
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
        return `<aside class="fm-sidebar">
            <div class="fm-sidebar-section">
                <div class="fm-sidebar-head">${esc(t('desktop.fm.quick_access', 'Quick Access'))}</div>
                ${items || `<div class="fm-sidebar-empty">${esc(t('desktop.fm.workspace_root', 'Workspace'))}</div>`}
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
        if (fm.viewMode === 'grid') {
            return `<div class="fm-grid">${files.map(f => renderGridItem(f)).join('')}</div>`;
        }
        return `<div class="fm-list">
            <div class="fm-list-header">
                <div class="fm-list-cell fm-col-name" data-sort="name" role="button" tabindex="0">
                    ${esc(t('desktop.fm.sort_name', 'Name'))}
                    ${fm.sortBy === 'name' ? iconMarkup(fm.sortAsc ? 'chevron-up' : 'chevron-down', fm.sortAsc ? '\u2191' : '\u2193', 'fm-sort-indicator', 12) : ''}
                </div>
                <div class="fm-list-cell fm-col-size" data-sort="size" role="button" tabindex="0">
                    ${esc(t('desktop.fm.sort_size', 'Size'))}
                    ${fm.sortBy === 'size' ? iconMarkup(fm.sortAsc ? 'chevron-up' : 'chevron-down', fm.sortAsc ? '\u2191' : '\u2193', 'fm-sort-indicator', 12) : ''}
                </div>
                <div class="fm-list-cell fm-col-date" data-sort="date" role="button" tabindex="0">
                    ${esc(t('desktop.fm.sort_date', 'Date Modified'))}
                    ${fm.sortBy === 'date' ? iconMarkup(fm.sortAsc ? 'chevron-up' : 'chevron-down', fm.sortAsc ? '\u2191' : '\u2193', 'fm-sort-indicator', 12) : ''}
                </div>
                <div class="fm-list-cell fm-col-type" data-sort="type" role="button" tabindex="0">
                    ${esc(t('desktop.fm.sort_type', 'Type'))}
                    ${fm.sortBy === 'type' ? iconMarkup(fm.sortAsc ? 'chevron-up' : 'chevron-down', fm.sortAsc ? '\u2191' : '\u2193', 'fm-sort-indicator', 12) : ''}
                </div>
            </div>
            <div class="fm-list-body">${files.map(f => renderListRow(f)).join('')}</div>
        </div>`;
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
