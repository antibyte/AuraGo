(function () {
    const STATE_KEY = 'aurago.codeStudio.state.v1';
    const WORKSPACE_ROOT = '/workspace';
    const DEFAULT_EDITOR_FONT_SIZE = 13;
    const MIN_EDITOR_FONT_SIZE = 10;
    const MAX_EDITOR_FONT_SIZE = 24;

    const instances = new Map();
    let state = null;
    let latestWindowId = '';

    function createInstance(container, windowId, context) {
        return {
            root: container,
            windowId,
            context: context || {},
            editorType: null,
            cmModule: null,
            files: [],
            openTabs: [],
            activeTabIndex: -1,
            terminal: null,
            fitAddon: null,
            ws: null,
            terminalSessions: [],
            activeTerminalSession: 0,
            currentPath: WORKSPACE_ROOT,
            recentFiles: [],
            containerStatus: 'unknown',
            terminalVisible: true,
            terminalHeight: 220,
            sidebarVisible: true,
            sidebarWidth: 280,
            editorFontSize: DEFAULT_EDITOR_FONT_SIZE,
            searchVisible: false,
            searchResults: [],
            agentVisible: false,
            agentMessages: [],
agentBusy: false,
            agentAbortController: null,
            pendingSuggestion: null,
            shortcutsWired: false,
            zenMode: false,
            expandedDirs: new Set(),
            treeCache: {},
            iconMarkup: context && typeof context.iconMarkup === 'function' ? context.iconMarkup : null,
            disposers: []
        };
    }

    function currentInstance() {
        if (state && instances.get(state.windowId) === state) return state;
        if (latestWindowId && instances.has(latestWindowId)) return instances.get(latestWindowId);
        const iterator = instances.values().next();
        return iterator.done ? null : iterator.value;
    }

    function isLiveInstance(instance) {
        return instance && instances.get(instance.windowId) === instance;
    }

    function runWithInstance(instance, fn) {
        if (!instance || typeof fn !== 'function') return undefined;
        const previous = state;
        state = instance;
        try {
            return fn();
        } finally {
            state = previous;
        }
    }

    async function runAsyncStep(instance, fn) {
        if (!isLiveInstance(instance)) return undefined;
        const previous = state;
        state = instance;
        try {
            return await fn(instance);
        } finally {
            state = previous;
        }
    }

    function withCurrentInstance(fn) {
        const instance = currentInstance();
        if (!instance) return undefined;
        return runWithInstance(instance, fn);
    }

    function bindInstance(instance, fn) {
        return function boundCodeStudioHandler(...args) {
            if (!isLiveInstance(instance)) return undefined;
            return runWithInstance(instance, () => fn.apply(this, args));
        };
    }

    function bind(fn) {
        return bindInstance(state, fn);
    }

    function addDisposer(instance, disposeFn) {
        if (instance && typeof disposeFn === 'function') instance.disposers.push(disposeFn);
    }

    function registerDisposer(disposeFn) {
        if (!state || typeof disposeFn !== 'function') return () => {};
        const owner = state;
        owner.disposers.push(disposeFn);
        return () => {
            if (state === owner) state.disposers = state.disposers.filter(item => item !== disposeFn);
            else owner.disposers = owner.disposers.filter(item => item !== disposeFn);
        };
    }

    function clampEditorFontSize(value) {
        const numeric = Number(value);
        if (!Number.isFinite(numeric)) return DEFAULT_EDITOR_FONT_SIZE;
        return Math.min(MAX_EDITOR_FONT_SIZE, Math.max(MIN_EDITOR_FONT_SIZE, Math.round(numeric)));
    }

    function destroyTabView(tab) {
        if (tab && tab.view && typeof tab.view.destroy === 'function') {
            try { tab.view.destroy(); } catch (_) {}
        }
        if (tab) tab.view = null;
    }

    function tr(key, fallback, vars) {
        const translator = typeof window.t === 'function'
            ? window.t
            : (window.AuraGo && typeof window.AuraGo.t === 'function' ? window.AuraGo.t : null);
        let text = translator ? translator(key, vars || {}) : key;
        if (text === key) text = fallback || key;
        Object.entries(vars || {}).forEach(([name, value]) => {
            text = text.replaceAll('{{' + name + '}}', String(value));
            text = text.replaceAll('{' + name + '}', String(value));
        });
        return text;
    }

    function esc(value) {
        return String(value == null ? '' : value)
            .replaceAll('&', '&amp;')
            .replaceAll('<', '&lt;')
            .replaceAll('>', '&gt;')
            .replaceAll('"', '&quot;')
            .replaceAll("'", '&#39;');
    }

    function iconMarkup(key, fallback, className, size) {
        if (state && typeof state.iconMarkup === 'function') {
            return state.iconMarkup(key, fallback, className || 'cs-papirus-icon', size || 15);
        }
        const pixels = Number(size || 15) || 15;
        return `<span class="${esc(className || 'cs-papirus-icon')}" style="font-size:${pixels}px">${esc(fallback || key || '')}</span>`;
    }

    function buttonIcon(key, fallback) {
        return iconMarkup(key, fallback, 'cs-button-icon', 15);
    }

    function fileIconName(name) {
        const lang = languageForPath(name);
        return ({
            javascript: 'javascript',
            python: 'python',
            go: 'go',
            rust: 'code',
            json: 'json',
            html: 'html',
            css: 'css',
            markdown: 'markdown'
        })[lang] || 'text';
    }

    function normalizeCodeStudioPath(rawPath) {
        let value = String(rawPath || '').replace(/\\/g, '/').replace(/\0/g, '').trim();
        if (!value) return WORKSPACE_ROOT;
        value = value.replace(/^file:\/+/i, '/').replace(/\/+/g, '/');
        if (/^[a-zA-Z]:\//.test(value) || value.startsWith('~')) return WORKSPACE_ROOT;
        const lower = value.toLowerCase();
        if (lower.startsWith('/home/') || lower.startsWith('/users/') || lower.startsWith('/var/') || lower.startsWith('/tmp/')) return WORKSPACE_ROOT;
        if (value === 'workspace' || lower.startsWith('workspace/')) {
            value = '/' + value;
        } else if (!value.startsWith('/')) {
            value = WORKSPACE_ROOT + '/' + value.replace(/^\.\//, '');
        }
        if (value !== WORKSPACE_ROOT && !value.startsWith(WORKSPACE_ROOT + '/')) return WORKSPACE_ROOT;
        const parts = [];
        for (const part of value.split('/')) {
            if (!part || part === '.') continue;
            if (part === '..') {
                if (parts.length <= 1) return WORKSPACE_ROOT;
                parts.pop();
                continue;
            }
            parts.push(part);
        }
        if (!parts.length || parts[0] !== 'workspace') return WORKSPACE_ROOT;
        return '/' + parts.join('/');
    }

    function codeStudioDesktopPath(rawPath) {
        const path = normalizeCodeStudioPath(rawPath);
        return path === WORKSPACE_ROOT ? '' : path.slice(WORKSPACE_ROOT.length + 1);
    }

    function desktopToCodeStudioPath(rawPath) {
        const value = String(rawPath || '').replace(/\\/g, '/').replace(/^\/+/, '');
        if (!value) return WORKSPACE_ROOT;
        if (value === 'workspace' || value.startsWith('workspace/')) return normalizeCodeStudioPath(value);
        return normalizeCodeStudioPath(WORKSPACE_ROOT + '/' + value);
    }

    function codeStudioParentPath(rawPath) {
        const path = normalizeCodeStudioPath(rawPath);
        if (path === WORKSPACE_ROOT) return WORKSPACE_ROOT;
        const index = path.lastIndexOf('/');
        if (index <= WORKSPACE_ROOT.length) return WORKSPACE_ROOT;
        return path.slice(0, index) || WORKSPACE_ROOT;
    }

    async function api(path, options) {
        const requestOptions = Object.assign({
            credentials: 'same-origin',
            headers: { 'Content-Type': 'application/json' }
        }, options || {});
        if (requestOptions.body instanceof FormData) delete requestOptions.headers;
        const response = await fetch(path, requestOptions);
        if (!response.ok) {
            let message = response.statusText;
            try {
                const body = await response.json();
                message = body.error || body.message || message;
            } catch (_) {
                message = await response.text() || message;
            }
            throw new Error(message);
        }
        return response.json();
    }

    const apiClient = {
        status: () => api('/api/code-studio/status'),
        files: path => api('/api/code-studio/files?path=' + encodeURIComponent(path || WORKSPACE_ROOT)),
        file: path => api('/api/code-studio/file?path=' + encodeURIComponent(path)),
        writeFile: (path, content) => api('/api/code-studio/file', {
            method: 'PUT',
            body: JSON.stringify({ path, content })
        }),
        renamePath: (oldPath, newPath) => api('/api/code-studio/file', {
            method: 'PATCH',
            body: JSON.stringify({ old_path: oldPath, new_path: newPath })
        }),
        deletePath: path => api('/api/code-studio/file?path=' + encodeURIComponent(path), { method: 'DELETE' }),
        uploadFile: (path, file) => {
            const body = new FormData();
            body.append('path', path || WORKSPACE_ROOT);
            body.append('file', file);
            return api('/api/code-studio/upload', { method: 'POST', body });
        },
        createDirectory: path => api('/api/code-studio/directory', {
            method: 'POST',
            body: JSON.stringify({ path })
        }),
        exec: command => runOnWindow(null, () => api('/api/code-studio/exec', {
            method: 'POST',
            body: JSON.stringify({ command, cwd: currentDirectory(), timeout_seconds: 300 })
        })),
        search: options => api('/api/code-studio/search?' + new URLSearchParams(options)),
        agentChat: (message, context) => api('/api/desktop/chat', {
            method: 'POST',
            body: JSON.stringify({ message, context })
        })
    };

    function loadState() {
        try {
            const raw = localStorage.getItem(STATE_KEY);
            if (!raw) return;
            const saved = JSON.parse(raw);
            state.activeTabIndex = Number(saved.activeTabIndex ?? -1);
            state.sidebarVisible = saved.sidebarVisible !== false;
            state.sidebarWidth = Number(saved.sidebarWidth || 280);
            state.editorFontSize = clampEditorFontSize(saved.editorFontSize);
            state.terminalVisible = saved.terminalVisible !== false;
            state.terminalHeight = Number(saved.terminalHeight || 220);
            state.recentFiles = Array.isArray(saved.recentFiles) ? saved.recentFiles.slice(0, 20) : [];
            state.openTabs = Array.isArray(saved.openTabs) ? saved.openTabs.map(tab => ({
                path: tab.path,
                content: '',
                modified: false,
                language: languageForPath(tab.path),
                view: null
            })).filter(tab => tab.path) : [];
        } catch (err) {
            console.warn('Failed to load Code Studio state', err);
        }
    }

    function saveState() {
        const payload = {
            openTabs: state.openTabs.map(tab => ({ path: tab.path })),
            activeTabIndex: state.activeTabIndex,
            sidebarVisible: state.sidebarVisible,
            sidebarWidth: state.sidebarWidth,
            editorFontSize: state.editorFontSize,
            terminalVisible: state.terminalVisible,
            terminalHeight: state.terminalHeight,
            recentFiles: state.recentFiles.slice(0, 20)
        };
        localStorage.setItem(STATE_KEY, JSON.stringify(payload));
    }

    async function render(container, windowId, context) {
        if (!container) return;
        dispose(windowId);
        const instance = createInstance(container, windowId, context);
        instances.set(windowId, instance);
        latestWindowId = windowId;
        runWithInstance(instance, () => {
            loadState();
            container.innerHTML = shellMarkup();
            renderLoading(tr('codeStudio.starting', 'Starting container...'));
        });
        try {
            await runAsyncStep(instance, prepareContainer);
            if (!isLiveInstance(instance)) return;
            await runAsyncStep(instance, loadEditorModule);
            if (!isLiveInstance(instance)) return;
            runWithInstance(instance, () => {
                container.innerHTML = shellMarkup();
                renderShell();
            });
            const launchPath = normalizeCodeStudioPath(context && context.path);
            const hasLaunchPath = !!(context && context.path) && launchPath !== WORKSPACE_ROOT;
            await runAsyncStep(instance, () => refreshFiles(hasLaunchPath ? codeStudioParentPath(launchPath) : (context && context.path ? launchPath : state.currentPath)));
            if (!isLiveInstance(instance)) return;
            await runAsyncStep(instance, restoreTabs);
            if (!isLiveInstance(instance)) return;
            if (hasLaunchPath) {
                await runAsyncStep(instance, async () => {
                    try {
                        await openFile(launchPath);
                    } catch (err) {
                        await refreshFiles(launchPath);
                        renderStatus((err && err.message) || String(err));
                    }
                });
                if (!isLiveInstance(instance)) return;
            }
            runWithInstance(instance, connectTerminal);
        } catch (err) {
            if (isLiveInstance(instance)) runWithInstance(instance, () => renderError(err.message || String(err)));
        }
    }

    function dispose(windowId) {
        if (!windowId) {
            const instance = currentInstance();
            windowId = instance && instance.windowId;
        }
        const instance = instances.get(windowId);
        if (!instance) return;
        if (instance.ws && (typeof WebSocket === 'undefined' || instance.ws.readyState !== WebSocket.CLOSED)) {
            instance.ws.close();
        }
        for (const disposeFn of instance.disposers || []) {
            try { disposeFn(); } catch (_) {}
        }
        instance.openTabs.forEach(destroyTabView);
        instances.delete(windowId);
        if (state === instance) state = null;
        if (latestWindowId === windowId) latestWindowId = instances.size ? Array.from(instances.keys()).pop() : '';
    }

    async function prepareContainer(instance) {
        const target = instance || state;
        const status = await apiClient.status();
        const payload = status.code_studio || {};
        if (!payload.enabled) throw new Error(tr('codeStudio.dockerUnavailable', 'Docker is not available. Code Studio requires Docker.'));
        if (!isLiveInstance(target)) return;
        runWithInstance(target, () => {
            state.containerStatus = payload.running ? 'running' : 'starting';
        });
        if (!payload.running) {
            await apiClient.files(WORKSPACE_ROOT);
            if (!isLiveInstance(target)) return;
            runWithInstance(target, () => {
                state.containerStatus = 'running';
            });
        }
    }

    async function loadEditorModule(instance) {
        const target = instance || state;
        try {
            const cmModule = await import('/js/vendor/codemirror-bundle.esm.js');
            if (!isLiveInstance(target)) return;
            runWithInstance(target, () => {
                state.cmModule = cmModule;
                state.editorType = 'codemirror';
            });
        } catch (err) {
            console.warn('CodeMirror ESM failed, using textarea fallback', err);
            if (!isLiveInstance(target)) return;
            runWithInstance(target, () => {
                state.cmModule = null;
                state.editorType = 'textarea';
            });
        }
    }

    function shellMarkup() {
        return `<div class="code-studio" data-code-studio>
            <div class="code-studio-toolbar" data-toolbar></div>
            <div class="code-studio-search" data-search hidden></div>
            <div class="code-studio-body">
                <nav class="code-studio-activity-bar" data-activity-bar>
                    <button type="button" class="cs-activity-btn active" data-activity="explorer" title="${esc(tr('codeStudio.title', 'Explorer'))}">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"/></svg>
                    </button>
                    <button type="button" class="cs-activity-btn" data-activity="search" title="${esc(tr('codeStudio.search', 'Search'))}">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/></svg>
                    </button>
                    <button type="button" class="cs-activity-btn" data-activity="agent" title="${esc(tr('codeStudio.agentChat', 'Agent'))}">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15a2 2 0 01-2 2H7l-4 4V5a2 2 0 012-2h14a2 2 0 012 2z"/></svg>
                    </button>
                    <span class="cs-activity-spacer"></span>
                    <button type="button" class="cs-activity-btn" data-activity="terminal" title="${esc(tr('codeStudio.toggleTerminal', 'Terminal'))}">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>
                    </button>
                </nav>
<aside class="code-studio-sidebar" data-sidebar></aside>
                <main class="code-studio-main">
                    <div class="code-studio-tabs" data-tabs></div>
                    <div class="code-studio-breadcrumbs" data-breadcrumbs></div>
                    <div class="code-studio-editor" data-editor></div>
                    <div class="code-studio-terminal" data-terminal></div>
                </main>
                <aside class="code-studio-chat" data-agent-panel></aside>
            </div>
            <div class="code-studio-statusbar" data-statusbar></div>
            <button type="button" class="cs-zen-exit" data-zen-exit title="Exit Zen Mode (Esc)">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" width="14" height="14"><path d="M8 3v3a2 2 0 01-2 2H3m18 0h-3a2 2 0 01-2-2V3m0 18v-3a2 2 0 012-2h3M3 16h3a2 2 0 012 2v3"/></svg>
                <span>${esc(tr('codeStudio.exitZen', 'Exit Zen Mode'))}</span>
            </button>
        </div>`;
    }

    function renderShell() {
        const root = ensureShellRoot();
        if (state.context && typeof state.context.wireContextMenuBoundary === 'function') state.context.wireContextMenuBoundary(root);
        root.style.setProperty('--cs-sidebar-width', Math.max(180, state.sidebarWidth) + 'px');
        root.style.setProperty('--cs-terminal-height', Math.max(120, state.terminalHeight) + 'px');
        root.style.setProperty('--cs-editor-font-size', clampEditorFontSize(state.editorFontSize) + 'px');
        root.dataset.terminal = state.terminalVisible ? 'visible' : 'hidden';
        root.dataset.sidebar = state.sidebarVisible ? 'visible' : 'hidden';
        root.dataset.agent = state.agentVisible ? 'visible' : 'hidden';
        root.dataset.zen = state.zenMode ? 'true' : 'false';
        renderToolbar();
        renderSearchPanel();
        renderSidebar();
        renderTabs();
        renderBreadcrumbs();
        renderEditor();
        renderTerminal();
        renderAgentPanel();
        renderStatus();
        renderWindowMenus();
        renderActivityBar();
        wireShortcuts();
        wireSidebarResize();
        const zenExit = root.querySelector('[data-zen-exit]');
        if (zenExit) zenExit.addEventListener('click', bind(() => toggleZenMode()));
    }

    function renderWindowMenus() {
        if (!state || !state.context || typeof state.context.setWindowMenus !== 'function') return;
        const hasActiveTab = !!activeTab();
        state.context.setWindowMenus(state.windowId, [
            {
                id: 'file',
                labelKey: 'desktop.menu_file',
                items: [
                    { id: 'new-file', label: tr('codeStudio.newFile', 'New File'), icon: 'file-plus', shortcut: 'Ctrl+N', action: bind(createNewFile) },
                    { id: 'new-folder', label: tr('codeStudio.newFolder', 'New Folder'), icon: 'folder-plus', action: bind(createNewFolder) },
                    { id: 'open-file-dialog', labelKey: 'desktop.file_dialog_open', icon: 'folder-open', shortcut: 'Ctrl+O', action: bind(openFileFromDialog) },
                    { id: 'save', label: tr('codeStudio.save', 'Save'), icon: 'save', shortcut: 'Ctrl+S', disabled: !hasActiveTab, action: bind(saveCurrentFile) },
                    { id: 'upload', label: tr('codeStudio.upload', 'Upload'), icon: 'upload', action: bind(uploadFile) },
                    { type: 'separator' },
                    { id: 'refresh', label: tr('codeStudio.refresh', 'Refresh'), icon: 'refresh', action: bind(() => refreshFiles(state.currentPath)) }
                ]
            },
            {
                id: 'edit',
                labelKey: 'desktop.menu_edit',
                items: [
                    { id: 'search', label: tr('codeStudio.search', 'Search'), icon: 'search', shortcut: 'Ctrl+F', action: bind(toggleSearch) }
                ]
            },
            {
                id: 'view',
                labelKey: 'desktop.menu_view',
                items: [
                    { id: 'sidebar', label: tr('codeStudio.sidebar', 'Sidebar'), icon: 'sidebar', shortcut: 'Ctrl+B', checked: state.sidebarVisible, action: bind(toggleSidebar) },
                    { id: 'terminal', labelKey: 'desktop.menu_terminal', icon: 'terminal', checked: state.terminalVisible, action: bind(toggleTerminal) },
                    { id: 'agent-panel', labelKey: 'desktop.menu_agent_panel', icon: 'chat', checked: state.agentVisible, action: bind(toggleAgentPanel) },
                    { type: 'separator' },
                    { id: 'zoom-in', label: tr('codeStudio.zoomIn', 'Zoom In'), icon: 'zoom-in', shortcut: 'Ctrl+=', disabled: state.editorFontSize >= MAX_EDITOR_FONT_SIZE, action: bind(() => adjustEditorZoom(1)) },
                    { id: 'zoom-out', label: tr('codeStudio.zoomOut', 'Zoom Out'), icon: 'zoom-out', shortcut: 'Ctrl+-', disabled: state.editorFontSize <= MIN_EDITOR_FONT_SIZE, action: bind(() => adjustEditorZoom(-1)) },
                    { id: 'zoom-reset', label: tr('codeStudio.zoomReset', 'Reset Zoom'), icon: 'zoom-reset', shortcut: 'Ctrl+0', disabled: state.editorFontSize === DEFAULT_EDITOR_FONT_SIZE, action: bind(resetEditorZoom) }
                ]
            },
            {
                id: 'run',
                labelKey: 'desktop.menu_run',
                items: [
                    { id: 'run-current', label: tr('codeStudio.run', 'Run'), icon: 'run', shortcut: 'F5', disabled: !hasActiveTab, action: bind(runCurrentFile) }
                ]
            }
        ]);
    }

    function renderToolbar() {
        const toolbar = shellPart('[data-toolbar]');
        if (!toolbar) return;
        const tab = activeTab();
        const fileName = tab ? baseName(tab.path) : '';
        toolbar.innerHTML = `
            <span class="cs-toolbar-group">
                <button type="button" class="cs-icon-button" data-action="new-file" title="${esc(tr('codeStudio.newFile', 'New File'))} (Ctrl+N)">${iconMarkup('file-plus', '+', 'cs-icon-button-icon', 16)}</button>
                <button type="button" class="cs-icon-button" data-action="new-folder" title="${esc(tr('codeStudio.newFolder', 'New Folder'))}">${iconMarkup('folder-plus', '+', 'cs-icon-button-icon', 16)}</button>
                <button type="button" class="cs-icon-button" data-action="open-file-dialog" title="${esc(tr('desktop.file_dialog_open', 'Open'))} (Ctrl+O)">${iconMarkup('folder-open', 'O', 'cs-icon-button-icon', 16)}</button>
                <button type="button" class="cs-icon-button" data-action="save" title="${esc(tr('codeStudio.save', 'Save'))} (Ctrl+S)">${iconMarkup('save', 'S', 'cs-icon-button-icon', 16)}</button>
            </span>
            <span class="cs-toolbar-separator"></span>
            <span class="cs-toolbar-group">
                <button type="button" class="cs-icon-button" data-action="upload" title="${esc(tr('codeStudio.upload', 'Upload'))}">${iconMarkup('upload', 'U', 'cs-icon-button-icon', 16)}</button>
                <button type="button" class="cs-icon-button" data-action="refresh" title="${esc(tr('codeStudio.refresh', 'Refresh'))}">${iconMarkup('refresh', 'R', 'cs-icon-button-icon', 16)}</button>
            </span>
            <span class="cs-toolbar-filename">${fileName ? '<strong>' + esc(fileName) + '</strong>' + (tab && tab.modified ? ' <span style="color:var(--cs-accent)">*</span>' : '') : esc(tr('codeStudio.title', 'Code Studio'))}</span>
            <span class="cs-toolbar-group">
                <button type="button" class="cs-icon-button" data-action="command-palette" title="${esc(tr('codeStudio.commandPalette', 'Command Palette'))} (Ctrl+Shift+P)">${iconMarkup('search', 'P', 'cs-icon-button-icon', 16)}</button>
                <button type="button" class="cs-button primary" data-action="run" title="${esc(tr('codeStudio.run', 'Run'))} (F5)">${iconMarkup('run', 'R', 'cs-button-icon', 15)}<span>${esc(tr('codeStudio.run', 'Run'))}</span></button>
            </span>`;
        toolbar.querySelector('[data-action="new-file"]').addEventListener('click', bind(createNewFile));
        toolbar.querySelector('[data-action="new-folder"]').addEventListener('click', bind(createNewFolder));
        toolbar.querySelector('[data-action="open-file-dialog"]').addEventListener('click', bind(openFileFromDialog));
        toolbar.querySelector('[data-action="save"]').addEventListener('click', bind(saveCurrentFile));
        toolbar.querySelector('[data-action="run"]').addEventListener('click', bind(runCurrentFile));
        toolbar.querySelector('[data-action="upload"]').addEventListener('click', bind(uploadFile));
        toolbar.querySelector('[data-action="refresh"]').addEventListener('click', bind(() => refreshFiles(state.currentPath)));
        const cpBtn = toolbar.querySelector('[data-action="command-palette"]');
        if (cpBtn) cpBtn.addEventListener('click', bind(() => {
            if (typeof window.CodeStudioCommandPalette === 'object' && window.CodeStudioCommandPalette.toggle) {
                window.CodeStudioCommandPalette.toggle();
            }
        }));
    }

    function renderSearchPanel() {
        const panel = shellPart('[data-search]');
        if (!panel) return;
        panel.hidden = !state.searchVisible;
        if (!state.searchVisible) return;
        const results = state.searchResults.length ? state.searchResults.map(result => `
            <button type="button" class="cs-search-result" data-search-path="${esc(result.path)}" data-search-line="${esc(result.line)}">
                <span>${esc(result.path)}:${esc(result.line)}</span>
                <code>${esc(result.preview)}</code>
            </button>`).join('') : `<div class="cs-empty">${esc(tr('codeStudio.noFiles', 'No files open'))}</div>`;
        panel.innerHTML = `<form class="cs-search-form" data-search-form>
            <input name="q" placeholder="${esc(tr('codeStudio.searchFiles', 'Search in Files'))}" autocomplete="off" spellcheck="false">
            <input name="include" placeholder="*.go" autocomplete="off" spellcheck="false">
            <input name="exclude" placeholder="vendor/" autocomplete="off" spellcheck="false">
            <label><input type="checkbox" name="case"> Aa</label>
            <label><input type="checkbox" name="whole"> Ab</label>
            <label><input type="checkbox" name="regex"> .*</label>
            <button type="submit" class="cs-button primary">${buttonIcon('search', 'S')}<span>${esc(tr('codeStudio.search', 'Search'))}</span></button>
        </form><div class="cs-search-results">${results}</div>`;
        panel.querySelector('[data-search-form]').addEventListener('submit', bind(event => {
            event.preventDefault();
            runSearch(new FormData(event.currentTarget));
        }));
        panel.querySelectorAll('[data-search-path]').forEach(btn => {
            btn.addEventListener('click', bind(() => openSearchResult(btn.dataset.searchPath, Number(btn.dataset.searchLine || 1))));
        });
        const input = panel.querySelector('input[name="q"]');
        if (input && !input.value) input.focus();
    }

    function renderAgentPanel() {
        const panel = shellPart('[data-agent-panel]');
        if (!panel) return;
        if (!state.agentVisible) {
            panel.innerHTML = '';
            return;
        }
        const messages = state.agentMessages.length ? state.agentMessages.map(message => {
            const roleClass = message.role === 'user' ? 'user' : 'agent';
            const content = message.role === 'user' ? esc(message.text) : renderMarkdown(message.text);
            return `<div class="cs-agent-message ${roleClass}"><div class="cs-md-content">${content}</div></div>`;
        }).join('') : `<div class="cs-agent-message agent"><div class="cs-md-content">${esc(tr('desktop.chat_welcome', 'Ask me to create apps, widgets, or files for this desktop.'))}</div></div>`;

        const quickActions = `<div class="cs-quick-actions">
            <button type="button" class="cs-quick-action" data-code-action="explain">${esc(tr('codeStudio.explain', 'Explain'))}</button>
            <button type="button" class="cs-quick-action" data-code-action="comments">${esc(tr('codeStudio.generateComments', 'Comments'))}</button>
            <button type="button" class="cs-quick-action" data-code-action="tests">${esc(tr('codeStudio.generateTests', 'Tests'))}</button>
            <button type="button" class="cs-quick-action" data-code-action="refactor">${esc(tr('codeStudio.refactor', 'Refactor'))}</button>
        </div>`;

        const suggestion = state.pendingSuggestion ? `<div class="code-studio-diff">
            <div class="cs-diff-head">
                <strong>${esc(tr('codeStudio.applyChanges', 'Apply Changes'))}</strong>
                <button type="button" class="cs-button primary" data-agent-apply>${buttonIcon('check-square', 'Y')}<span>${esc(tr('codeStudio.applyChanges', 'Apply Changes'))}</span></button>
                <button type="button" class="cs-button" data-agent-discard>${buttonIcon('x', 'X')}<span>${esc(tr('codeStudio.discardChanges', 'Discard Changes'))}</span></button>
            </div>
            <pre>${esc(state.pendingSuggestion)}</pre>
        </div>` : '';

        const typingIndicator = state.agentBusy ? `<div class="cs-agent-typing"><span class="cs-typing-dot"></span><span class="cs-typing-dot"></span><span class="cs-typing-dot"></span></div>` : '';

        panel.innerHTML = `<div class="cs-agent-head">
            <strong>${esc(tr('codeStudio.agentChat', 'Agent Chat'))}</strong>
            <button type="button" class="cs-icon-button" data-agent-close title="${esc(tr('codeStudio.closeTab', 'Close tab'))}">${iconMarkup('x', 'X', 'cs-icon-button-icon', 16)}</button>
        </div>
        ${quickActions}
        <div class="cs-agent-log">${messages}${typingIndicator}</div>
        ${suggestion}
        <form class="cs-agent-form" data-agent-form>
            <input name="message" autocomplete="off" spellcheck="false" placeholder="${esc(tr('desktop.chat_placeholder', 'Ask the agent...'))}">
            ${state.agentBusy
                ? `<button type="button" class="cs-agent-stop" data-agent-stop>${esc(tr('codeStudio.stop', 'Stop'))}</button>`
                : `<button type="submit" class="cs-button primary">${buttonIcon('chat', 'S')}<span>${esc(tr('desktop.send', 'Send'))}</span></button>`
            }
        </form>`;
        panel.querySelector('[data-agent-close]').addEventListener('click', bind(toggleAgentPanel));
        panel.querySelectorAll('[data-code-action]').forEach(btn => {
            btn.addEventListener('click', bind(() => runCodeAction(btn.dataset.codeAction)));
        });
        panel.querySelector('[data-agent-form]').addEventListener('submit', bind(event => {
            event.preventDefault();
            const input = event.currentTarget.elements.message;
            const message = input.value.trim();
            if (!message) return;
            input.value = '';
            sendAgentMessage(message);
        }));
        const stopBtn = panel.querySelector('[data-agent-stop]');
if (stopBtn) stopBtn.addEventListener('click', bind(() => {
            if (state.agentAbortController) {
                state.agentAbortController.abort();
                state.agentAbortController = null;
            }
            state.agentBusy = false;
            renderAgentPanel();
        }));
        const apply = panel.querySelector('[data-agent-apply]');
        if (apply) apply.addEventListener('click', bind(applyAgentSuggestion));
        const discard = panel.querySelector('[data-agent-discard]');
        if (discard) discard.addEventListener('click', bind(() => {
            state.pendingSuggestion = null;
            renderAgentPanel();
        }));
        // Wire copy buttons on code blocks
        panel.querySelectorAll('.cs-md-code-copy').forEach(btn => {
            btn.addEventListener('click', bind(() => {
                const code = btn.closest('pre')?.querySelector('code');
                if (code) {
                    navigator.clipboard.writeText(code.textContent).then(() => {
                        btn.textContent = 'Copied!';
                        setTimeout(() => { btn.textContent = 'Copy'; }, 1500);
                    }).catch(() => {});
                }
            }));
        });
        // Scroll log to bottom
        const log = panel.querySelector('.cs-agent-log');
        if (log) log.scrollTop = log.scrollHeight;
    }

    function renderMarkdown(text) {
        if (!text) return '';
        let html = esc(text);
        // Code blocks with language
        html = html.replace(/```(\w*)\n([\s\S]*?)```/g, (_, lang, code) => {
            const langAttr = lang ? ` data-lang="${lang}"` : '';
            return `<pre${langAttr}><code class="language-${lang || 'text'}">${code}</code><button type="button" class="cs-md-code-copy">Copy</button></pre>`;
        });
        // Inline code
        html = html.replace(/`([^`\n]+)`/g, '<code>$1</code>');
        // Headers
        html = html.replace(/^### (.+)$/gm, '<h3>$1</h3>');
        html = html.replace(/^## (.+)$/gm, '<h2>$1</h2>');
        html = html.replace(/^# (.+)$/gm, '<h1>$1</h1>');
        // Bold and italic
        html = html.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
        html = html.replace(/\*([^*]+)\*/g, '<em>$1</em>');
        // Blockquotes
        html = html.replace(/^&gt; (.+)$/gm, '<blockquote>$1</blockquote>');
        // Horizontal rules
        html = html.replace(/^---$/gm, '<hr>');
        // Unordered lists
        html = html.replace(/^[\-\*] (.+)$/gm, '<li>$1</li>');
        html = html.replace(/((?:<li>.*<\/li>\n?)+)/g, '<ul>$1</ul>');
        // Ordered lists
        html = html.replace(/^\d+\. (.+)$/gm, '<li>$1</li>');
        // Links
        html = html.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>');
        // Paragraphs (lines not already wrapped in block elements)
        html = html.replace(/^(?!<[a-z/])((?!<).+)$/gm, '<p>$1</p>');
        // Clean up empty paragraphs
        html = html.replace(/<p>\s*<\/p>/g, '');
        return html;
    }

    function renderSidebar(errorMessage) {
        const sidebar = shellPart('[data-sidebar]');
        if (!sidebar) return;
if (errorMessage) {
            sidebar.innerHTML = `<div class="cs-sidebar-head"><strong>${esc(tr('codeStudio.title', 'Code Studio'))}</strong></div>
                <div class="code-studio-error compact">${esc(errorMessage)}</div>
                <div class="cs-sidebar-resize" data-sidebar-resize></div>`;
            wireSidebarResize();
            return;
        }
        const rows = state.files.length ? state.files.map(file => treeItemRow(file, 0)).join('') : `<div class="cs-empty">${esc(tr('codeStudio.noFiles', 'No files open'))}</div>`;
sidebar.innerHTML = `<div class="cs-sidebar-head">
            <strong>${esc(tr('codeStudio.title', 'Code Studio'))}</strong>
            <span>${esc(state.currentPath)}</span>
        </div><div class="cs-file-tree">${rows}</div>
        <div class="cs-sidebar-resize" data-sidebar-resize></div>`;
        wireSidebarTreeEvents(sidebar);
        wireSidebarDragDrop(sidebar);
        wireSidebarResize();
    }

    function treeItemRow(file, depth) {
        const isDir = file.type === 'directory';
        const isExpanded = isDir && state.expandedDirs.has(file.path);
        const indent = depth * 14;
        const icon = isDir
            ? (isExpanded ? iconMarkup('folder-open', 'D', 'cs-file-papirus-icon', 16) : iconMarkup('folder', 'D', 'cs-file-papirus-icon', 16))
            : iconMarkup(fileIconName(file.name), fileIcon(file.name), 'cs-file-papirus-icon', 16);
        const chevron = isDir
            ? `<span class="cs-tree-chevron${isExpanded ? ' expanded' : ''}">›</span>`
            : '<span style="width:18px;display:inline-block"></span>';
        const childrenHtml = isDir && isExpanded ? treeChildrenHtml(file.path, depth + 1) : '';
        return `<div class="cs-tree-item${isDir ? ' is-dir' : ' is-file'}" data-file-path="${esc(file.path)}" data-type="${esc(file.type)}" data-depth="${depth}" style="padding-left:${6 + indent}px">
            ${chevron}
            <span class="cs-file-icon">${icon}</span>
            <span class="cs-file-name">${esc(file.name)}</span>
            <span class="cs-file-actions">
                <span role="button" tabindex="0" class="cs-file-action" data-file-action="rename" title="${esc(tr('codeStudio.rename', 'Rename'))}">${iconMarkup('edit', 'E', 'cs-file-action-icon', 14)}</span>
                ${!isDir ? `<span role="button" tabindex="0" class="cs-file-action" data-file-action="download" title="${esc(tr('codeStudio.download', 'Download'))}">${iconMarkup('download', 'D', 'cs-file-action-icon', 14)}</span>` : ''}
                <span role="button" tabindex="0" class="cs-file-action danger" data-file-action="delete" title="${esc(tr('desktop.delete', 'Delete'))}">${iconMarkup('trash', 'X', 'cs-file-action-icon', 14)}</span>
            </span>
        </div>${childrenHtml}`;
    }

    function treeChildrenHtml(dirPath, depth) {
        const children = state.treeCache[dirPath];
        if (!children || !children.length) return '<div class="cs-tree-children" data-dir-path="' + esc(dirPath) + '"></div>';
        return '<div class="cs-tree-children" data-dir-path="' + esc(dirPath) + '">' +
            children.map(file => treeItemRow(file, depth)).join('') + '</div>';
    }

    async function expandDirectory(dirPath) {
        const target = state;
        if (!isLiveInstance(target)) return;
        if (state.expandedDirs.has(dirPath)) {
            state.expandedDirs.delete(dirPath);
            renderSidebar();
            return;
        }
        state.expandedDirs.add(dirPath);
        if (!state.treeCache[dirPath]) {
            try {
                const result = await apiClient.files(dirPath);
                if (!isLiveInstance(target)) return;
                runWithInstance(target, () => {
                    state.treeCache[dirPath] = (result.files || []).sort((a, b) => {
                        if (a.type === b.type) return a.name.localeCompare(b.name);
                        return a.type === 'directory' ? -1 : 1;
                    });
                    renderSidebar();
                });
            } catch (err) {
                if (isLiveInstance(target)) {
                    runWithInstance(target, () => {
                        state.expandedDirs.delete(dirPath);
                        renderSidebar();
                    });
                }
            }
        } else {
            renderSidebar();
        }
    }

    function wireSidebarTreeEvents(sidebar) {
        sidebar.querySelectorAll('.cs-tree-item').forEach(row => {
            row.addEventListener('click', bind(event => {
                const action = event.target.closest('[data-file-action]');
                if (action) return;
                const filePath = row.dataset.filePath;
                const fileType = row.dataset.type;
                if (fileType === 'directory') {
                    expandDirectory(filePath);
                } else {
                    openFile(filePath);
                }
            }));
            row.addEventListener('keydown', bind(event => {
                const filePath = row.dataset.filePath;
                const fileType = row.dataset.type;
                if (event.key === 'Enter') {
                    event.preventDefault();
                    if (fileType === 'directory') expandDirectory(filePath);
                    else openFile(filePath);
                }
                if (event.key === 'F2') {
                    event.preventDefault();
                    const file = findFileInTree(filePath);
                    if (file) renamePath(file);
                }
                if (event.key === 'Delete') {
                    event.preventDefault();
                    const file = findFileInTree(filePath);
                    if (file) deletePath(file);
                }
            }));
        });
        sidebar.querySelectorAll('[data-file-action]').forEach(btn => {
            btn.addEventListener('click', bind(event => {
                event.stopPropagation();
                const filePath = btn.closest('[data-file-path]').dataset.filePath;
                const file = findFileInTree(filePath);
                if (!file) return;
                const action = btn.dataset.fileAction;
                if (action === 'rename') renamePath(file);
                if (action === 'delete') deletePath(file);
                if (action === 'download') downloadFile(file);
            }));
        });
    }

    function findFileInTree(path) {
        for (const files of Object.values(state.treeCache)) {
            const found = files.find(f => f.path === path);
            if (found) return found;
        }
        const found = state.files.find(f => f.path === path);
        if (found) return found;
        const name = path.split('/').filter(Boolean).pop() || '';
        const parent = path.slice(0, Math.max(0, path.lastIndexOf('/')));
        return { path, name, type: 'file', size: 0 };
    }

    function wireSidebarDragDrop(sidebar) {
        sidebar.ondragover = bind(event => {
            event.preventDefault();
            sidebar.classList.add('dragover');
        });
        sidebar.ondragleave = bind(() => sidebar.classList.remove('dragover'));
        sidebar.ondrop = bind(async event => {
            const target = state;
            if (!isLiveInstance(target)) return;
            event.preventDefault();
            sidebar.classList.remove('dragover');
            const files = Array.from(event.dataTransfer && event.dataTransfer.files ? event.dataTransfer.files : []);
            const currentPath = target.currentPath;
            try {
                for (const file of files) {
                    await apiClient.uploadFile(currentPath, file);
                    if (!isLiveInstance(target)) return;
                }
                if (files.length) await runAsyncStep(target, () => refreshFiles(currentPath));
            } catch (err) {
                if (isLiveInstance(target)) runWithInstance(target, () => showOperationError(err));
            }
        });
    }

    function fileRow(file) {
        const icon = file.type === 'directory'
            ? iconMarkup('folder', 'D', 'cs-file-papirus-icon', 18)
            : iconMarkup(fileIconName(file.name), fileIcon(file.name), 'cs-file-papirus-icon', 18);
        return `<div role="button" tabindex="0" class="cs-file-row" data-file-path="${esc(file.path)}" data-type="${esc(file.type)}">
            <span class="cs-file-icon">${icon}</span>
            <span class="cs-file-name">${esc(file.name)}</span>
            <span class="cs-file-meta">${file.type === 'directory' ? '' : esc(formatBytes(file.size))}</span>
            <span class="cs-file-actions">
                <span role="button" tabindex="0" class="cs-file-action" data-file-action="rename" title="${esc(tr('codeStudio.rename', 'Rename'))}">${iconMarkup('edit', 'E', 'cs-file-action-icon', 14)}</span>
                ${file.type === 'file' ? `<span role="button" tabindex="0" class="cs-file-action" data-file-action="download" title="${esc(tr('codeStudio.download', 'Download'))}">${iconMarkup('download', 'D', 'cs-file-action-icon', 14)}</span>` : ''}
                <span role="button" tabindex="0" class="cs-file-action danger" data-file-action="delete" title="${esc(tr('desktop.delete', 'Delete'))}">${iconMarkup('trash', 'X', 'cs-file-action-icon', 14)}</span>
            </span>
        </div>`;
    }

    function renderTabs() {
        const tabs = shellPart('[data-tabs]');
        if (!tabs) return;
        tabs.innerHTML = state.openTabs.length ? state.openTabs.map((tab, index) => `
            <button type="button" class="cs-tab ${index === state.activeTabIndex ? 'active' : ''}" data-tab="${index}" draggable="true" title="${esc(tab.path)}">
                <span>${esc(baseName(tab.path))}</span>
                ${tab.modified ? '<span class="cs-tab-modified"></span>' : ''}
                <span class="cs-tab-close" data-close="${index}" title="${esc(tr('codeStudio.closeTab', 'Close tab'))}">${iconMarkup('x', 'X', 'cs-tab-close-icon', 12)}</span>
            </button>`).join('') : `<div class="cs-tabs-empty">${esc(tr('codeStudio.noFiles', 'No files open'))}</div>`;
        tabs.querySelectorAll('[data-tab]').forEach(btn => {
            btn.addEventListener('click', bind(event => {
                if (event.target.closest('[data-close]')) return;
                activateTab(Number(btn.dataset.tab));
            }));
            btn.addEventListener('dragstart', bind(event => {
                event.dataTransfer.setData('text/plain', btn.dataset.tab);
                event.dataTransfer.effectAllowed = 'move';
                btn.classList.add('dragging');
            }));
            btn.addEventListener('dragend', bind(() => btn.classList.remove('dragging')));
            btn.addEventListener('dragover', bind(event => {
                event.preventDefault();
                event.dataTransfer.dropEffect = 'move';
                const rect = btn.getBoundingClientRect();
                const mid = rect.left + rect.width / 2;
                btn.classList.toggle('drag-over-left', event.clientX < mid);
                btn.classList.toggle('drag-over-right', event.clientX >= mid);
            }));
            btn.addEventListener('dragleave', bind(() => {
                btn.classList.remove('drag-over-left', 'drag-over-right');
            }));
            btn.addEventListener('drop', bind(event => {
                event.preventDefault();
                btn.classList.remove('drag-over-left', 'drag-over-right');
                const fromIndex = Number(event.dataTransfer.getData('text/plain'));
                const toIndex = Number(btn.dataset.tab);
                if (isNaN(fromIndex) || isNaN(toIndex) || fromIndex === toIndex) return;
                const rect = btn.getBoundingClientRect();
                const insertBefore = event.clientX < rect.left + rect.width / 2;
                reorderTab(fromIndex, toIndex, insertBefore);
            }));
        });
        tabs.querySelectorAll('[data-close]').forEach(btn => btn.addEventListener('click', bind(event => {
            event.stopPropagation();
            closeTab(Number(btn.dataset.close));
        })));
        renderWindowMenus();
    }

    function renderBreadcrumbs() {
        const bar = shellPart('[data-breadcrumbs]');
        if (!bar) return;
        const tab = activeTab();
        if (!tab) {
            bar.innerHTML = '';
            return;
        }
        const relPath = tab.path.replace(WORKSPACE_ROOT + '/', '').replace(WORKSPACE_ROOT, '');
        if (!relPath) {
            bar.innerHTML = `<span class="cs-breadcrumb-item current">${esc(tr('codeStudio.title', 'Code Studio'))}</span>`;
            return;
        }
        const parts = relPath.split('/').filter(Boolean);
        const items = [];
        items.push(`<button type="button" class="cs-breadcrumb-item" data-bc-path="${esc(WORKSPACE_ROOT)}">${esc(tr('codeStudio.title', 'Code Studio'))}</button>`);
        let buildPath = WORKSPACE_ROOT;
        parts.forEach((part, i) => {
            buildPath += '/' + part;
            const isLast = i === parts.length - 1;
            items.push(`<span class="cs-breadcrumb-sep">›</span>`);
            if (isLast) {
                items.push(`<span class="cs-breadcrumb-item current">${esc(part)}</span>`);
            } else {
                items.push(`<button type="button" class="cs-breadcrumb-item" data-bc-path="${esc(buildPath)}">${esc(part)}</button>`);
            }
        });
        bar.innerHTML = items.join('');
        bar.querySelectorAll('[data-bc-path]').forEach(btn => {
            btn.addEventListener('click', bind(() => {
                const path = btn.dataset.bcPath;
                if (path === WORKSPACE_ROOT) refreshFiles(WORKSPACE_ROOT);
                else refreshFiles(path);
            }));
        });
    }

    function renderActivityBar() {
        const bar = shellPart('[data-activity-bar]');
        if (!bar) return;
        bar.querySelectorAll('.cs-activity-btn').forEach(btn => {
            const activity = btn.dataset.activity;
            // Remove existing badge
            const existingBadge = btn.querySelector('.cs-activity-badge');
            if (existingBadge) existingBadge.remove();
            if (activity === 'explorer') {
                btn.classList.toggle('active', state.sidebarVisible);
            } else if (activity === 'search') {
                btn.classList.toggle('active', state.searchVisible);
            } else if (activity === 'agent') {
                btn.classList.toggle('active', state.agentVisible);
            } else if (activity === 'terminal') {
                btn.classList.toggle('active', state.terminalVisible);
            }
            // Add badge for open files count on explorer
            if (activity === 'explorer' && state.openTabs && state.openTabs.length > 0) {
                const badge = document.createElement('span');
                badge.className = 'cs-activity-badge';
                badge.textContent = state.openTabs.length;
                btn.appendChild(badge);
            }
            if (!btn._wired) {
                btn._wired = true;
                btn.addEventListener('click', bind(() => {
                    const act = btn.dataset.activity;
                    if (act === 'explorer') toggleSidebar();
                    else if (act === 'search') toggleSearch();
                    else if (act === 'agent') toggleAgentPanel();
                    else if (act === 'terminal') toggleTerminal();
                }));
            }
        });
    }

    function renderEditor() {
        const editor = shellPart('[data-editor]');
        if (!editor) return;
        const tab = activeTab();
        if (!tab) {
            state.openTabs.forEach(destroyTabView);
            editor.innerHTML = `<div class="cs-editor-empty">
                <div class="cs-empty-icon">{ }</div>
                <div class="cs-empty-title">${esc(tr('codeStudio.welcome', 'Welcome to Code Studio'))}</div>
                <div class="cs-empty-hint">${esc(tr('codeStudio.welcomeHint', 'Open a file from the sidebar or press Ctrl+Shift+P to open the Command Palette'))}</div>
            </div>`;
            return;
        }
        state.openTabs.forEach(openTab => {
            if (openTab !== tab) destroyTabView(openTab);
        });
        destroyTabView(tab);
        editor.innerHTML = '';
        tab.view = state.editorType === 'codemirror'
            ? createCodeMirrorEditor(editor, tab)
            : createTextareaEditor(editor, tab);
        editor.oncontextmenu = bind(event => {
            event.preventDefault();
            showCodeActionMenu(event.clientX, event.clientY);
        });
    }
