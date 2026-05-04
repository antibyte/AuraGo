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
        loadPreferences();
        bindKeyboard();
        navigate(initialPath || '');
    }

    function renderAll() {
        if (!fm.host) return;
        fm.host.innerHTML = buildMarkup();
        attachEvents();
        updateToolbarState();
        updateStatusBar();
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

    function buildMarkup() {
        return `<div class="file-manager" data-fm-window="${esc(fm.windowId)}">
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
                <div class="fm-dropdown-wrap">
                    <button type="button" class="fm-toolbtn" data-action="sort-menu" title="${esc(t('desktop.fm.sort_by', 'Sort by'))}">
                        ${iconMarkup('sort', '\u2195', '', 16)}
                    </button>
                </div>
                <button type="button" class="fm-toolbtn" data-action="search-toggle" title="${esc(t('desktop.search', 'Search'))} (Ctrl+F)">
                    ${iconMarkup('search', '\u2315', '', 16)}
                </button>
                <button type="button" class="fm-toolbtn" data-action="refresh" title="${esc(t('desktop.fm.refresh', 'Refresh'))}">
                    ${iconMarkup('refresh', '\u21BB', '', 16)}
                </button>
                <div class="fm-separator"></div>
                <button type="button" class="fm-btn" data-action="upload">
                    ${iconMarkup('upload', '\u2191', 'fm-btn-icon', 14)}
                    ${esc(t('desktop.fm.upload', 'Upload'))}
                </button>
                <button type="button" class="fm-btn" data-action="new-file">
                    ${iconMarkup('file-plus', '+', 'fm-btn-icon', 14)}
                    ${esc(t('desktop.fm.new_file', 'New File'))}
                </button>
                <button type="button" class="fm-btn" data-action="new-folder">
                    ${iconMarkup('folder-plus', '+', 'fm-btn-icon', 14)}
                    ${esc(t('desktop.fm.new_folder', 'New Folder'))}
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
            item.addEventListener('click', () => navigate(item.dataset.sidebarPath));
            item.addEventListener('keydown', e => { if (e.key === 'Enter') navigate(item.dataset.sidebarPath); });
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
            item.addEventListener('click', handleItemClick);
            item.addEventListener('dblclick', handleItemDblClick);
            item.addEventListener('contextmenu', handleItemContextMenu);
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
        if (action === 'back') goBack();
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
        if (e.ctrlKey || e.metaKey) {
            toggleSelection(path);
        } else if (e.shiftKey && fm.lastClickedPath) {
            rangeSelection(fm.lastClickedPath, path);
        } else {
            clearSelection();
            addSelection(path);
        }
        fm.lastClickedPath = path;
        renderAll();
    }

    function handleItemDblClick(e) {
        const path = e.currentTarget.dataset.path;
        const type = e.currentTarget.dataset.type;
        const file = fm.files.find(f => f.path === path);
        if (!file) return;
        if (type === 'directory') {
            navigate(file.path);
        } else {
            openFileEntry(file);
        }
    }

    function handleItemKeyDown(e) {
        const path = e.currentTarget.dataset.path;
        const type = e.currentTarget.dataset.type;
        if (e.key === 'Enter') {
            e.preventDefault();
            const file = fm.files.find(f => f.path === path);
            if (!file) return;
            if (type === 'directory') navigate(file.path);
            else openFileEntry(file);
        } else if (e.key === 'F2') {
            e.preventDefault();
            startRename(path);
        } else if (e.key === 'Delete') {
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
        showContextMenu(e.clientX, e.clientY, items);
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
        showContextMenu(e.clientX, e.clientY, items);
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
        if (backBtn) backBtn.disabled = !canGoBack();
        if (fwdBtn) fwdBtn.disabled = !canGoForward();
    }

    function updateStatusBar() {
        // Status bar is re-rendered with the full markup
    }

    // Clipboard operations
    function cutSelection() {
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
        showNotification({ type: 'info', message: t('desktop.fm.upload_progress', 'Uploading...') });
        for (const file of Array.from(files)) {
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
            e.preventDefault();
            deleteSelected();
            return;
        }
        if (e.key === 'F2' && !isInput) {
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
            e.preventDefault();
            cutSelection();
            return;
        }
        if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'v' && !isInput) {
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
    }

    /**
     * Navigate an existing file manager instance to a new path.
     * Called by main.js when the user double-clicks a folder shortcut on the desktop.
     */
    function navigateTo(windowId, path) {
        if (fm.windowId === windowId && fm.host) {
            navigate(path);
        }
    }

    // Expose the module
    window.FileManager = { render, navigateTo };
})();
