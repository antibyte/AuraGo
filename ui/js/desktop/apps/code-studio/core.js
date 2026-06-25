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
            gitVisible: false,
            gitBranch: '',
            gitChanges: [],
            gitLog: [],
            splitMode: null,
            splitRatio: 0.5,
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
        }),
        gitStatus: () => api('/api/code-studio/git/status'),
        gitDiff: (file, staged) => api('/api/code-studio/git/diff?file=' + encodeURIComponent(file || '') + '&staged=' + (staged ? 'true' : 'false')),
        gitCommit: (message, addAll) => api('/api/code-studio/git/commit', {
            method: 'POST',
            body: JSON.stringify({ message, add_all: addAll })
        }),
        gitBranch: (name, action) => api('/api/code-studio/git/branch?' + new URLSearchParams({ name: name || '', action: action || 'list' })),
        gitLog: (path, count) => api('/api/code-studio/git/log?' + new URLSearchParams({ path: path || '', count: String(count || 20) })),
        agentStream: (message, context) => api('/api/code-studio/agent/stream?' + new URLSearchParams({ message, context: JSON.stringify(context || {}) }))
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
        closeTerminalSessionSockets(instance);
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

    function closeTerminalSessionSockets(instance) {
        if (!instance || !Array.isArray(instance.terminalSessions)) return;
        instance.terminalSessions.forEach(session => {
            if (session && session.ws && (typeof WebSocket === 'undefined' || session.ws.readyState !== WebSocket.CLOSED)) {
                session.ws.close();
            }
        });
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
                    <button type="button" class="cs-activity-btn" data-activity="git" title="${esc(tr('codeStudio.gitPanel', 'Git'))}">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="18" cy="18" r="3"/><circle cx="6" cy="6" r="3"/><path d="M13 6h3a2 2 0 012 2v7"/><line x1="6" y1="9" x2="6" y2="21"/></svg>
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
                <aside class="code-studio-git" data-git-panel hidden></aside>
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
        root.dataset.git = state.gitVisible ? 'visible' : 'hidden';
        root.dataset.zen = state.zenMode ? 'true' : 'false';
        renderToolbar();
        renderSearchPanel();
        renderSidebar();
        renderTabs();
        renderBreadcrumbs();
        renderEditor();
        renderTerminal();
        renderAgentPanel();
        renderGitPanel();
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
                    { id: 'git-panel', label: tr('codeStudio.gitPanel', 'Git'), icon: 'git', checked: state.gitVisible, action: bind(toggleGitPanel) },
                    { type: 'separator' },
                    { id: 'split-right', label: tr('codeStudio.splitRight', 'Split Right'), icon: 'split', action: bind(() => splitEditor('right')) },
                    { id: 'split-down', label: tr('codeStudio.splitDown', 'Split Down'), icon: 'split', action: bind(() => splitEditor('down')) },
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
            items.push(`<span class="cs-breadcrumb-sep">\u203a</span>`);
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

    function renderStatus(message) {
        const statusEl = shellPart('[data-statusbar]');
        if (!statusEl) return;
        const tab = activeTab();
        const lang = tab ? tab.language || '' : '';
        const lineInfo = tab ? cursorPositionText(tab) : '';
        const leftItems = [];
        if (message) leftItems.push(`<span>${esc(message)}</span>`);
        else leftItems.push(`<span>${esc(state.containerStatus)}</span>`);
        if (tab) leftItems.push(`<span>${tab.modified ? '<span style="color:var(--cs-accent)">\u25cf</span> ' : ''}${esc(baseName(tab.path))}</span>`);
        const rightItems = [];
        if (lang) rightItems.push(`<span data-clickable title="${esc(tr('codeStudio.language', 'Language'))}">${esc(lang)}</span>`);
        if (lineInfo) rightItems.push(`<span>${esc(lineInfo)}</span>`);
        rightItems.push(`<span>${esc(state.editorType === 'codemirror' ? 'CodeMirror' : 'Basic')}</span>`);
        statusEl.innerHTML = leftItems.join('') + '<span style="flex:1"></span>' + rightItems.join('');
    }

    function renderLoading(message) {
        state.root.innerHTML = `<div class="code-studio"><div class="code-studio-loading">
            <div class="cs-loader"></div><div>${esc(message)}</div>
        </div></div>`;
    }

    function renderError(message) {
        state.containerStatus = 'error';
        state.root.innerHTML = `<div class="code-studio"><div class="code-studio-error">
            <strong>${esc(tr('codeStudio.containerError', 'Container error: {error}', { error: message }))}</strong>
            <button type="button" class="cs-button primary" data-retry>${esc(tr('codeStudio.refresh', 'Refresh'))}</button>
        </div></div>`;
        state.root.querySelector('[data-retry]').addEventListener('click', bind(() => render(state.root, state.windowId, state.context || {})));
    }

    function applyEditorZoom() {
        const root = studioRoot();
        if (!root) return;
        root.style.setProperty('--cs-editor-font-size', clampEditorFontSize(state.editorFontSize) + 'px');
        const target = state;
        const refresh = () => {
            if (isLiveInstance(target)) runWithInstance(target, refreshActiveEditorZoomLayout);
        };
        if (typeof window.requestAnimationFrame === 'function') {
            window.requestAnimationFrame(refresh);
        } else {
            refresh();
        }
    }

    function refreshActiveEditorZoomLayout() {
        const tab = activeTab();
        if (!tab || !tab.view) return;
        if (typeof tab.view.requestMeasure === 'function') {
            tab.view.requestMeasure();
        }
        if (tab.view.textarea && typeof tab.view.textarea.getBoundingClientRect === 'function') {
            tab.view.textarea.getBoundingClientRect();
        }
    }

    function adjustEditorZoom(delta) {
        const nextSize = clampEditorFontSize(state.editorFontSize + delta);
        if (nextSize === state.editorFontSize) return;
        state.editorFontSize = nextSize;
        applyEditorZoom();
        saveState();
        renderWindowMenus();
        renderStatus();
    }

    function resetEditorZoom() {
        if (state.editorFontSize === DEFAULT_EDITOR_FONT_SIZE) return;
        state.editorFontSize = DEFAULT_EDITOR_FONT_SIZE;
        applyEditorZoom();
        saveState();
        renderWindowMenus();
        renderStatus();
    }

    async function refreshFiles(path) {
        const target = state;
        if (!target) return;
        const nextPath = normalizeCodeStudioPath(path || WORKSPACE_ROOT);
        state.currentPath = nextPath;
        try {
            const result = await apiClient.files(nextPath);
            if (!isLiveInstance(target)) return;
            runWithInstance(target, () => {
                state.files = result.files || [];
                state.treeCache[nextPath] = state.files.slice().sort((a, b) => {
                    if (a.type === b.type) return a.name.localeCompare(b.name);
                    return a.type === 'directory' ? -1 : 1;
                });
                renderSidebar();
                renderStatus();
            });
        } catch (err) {
            if (isLiveInstance(target)) {
                runWithInstance(target, () => renderSidebar(err.message || String(err)));
            }
        }
    }

    async function restoreTabs() {
        const target = state;
        if (!target) return;
        const savedPaths = target.openTabs.map(tab => tab.path);
        const desiredActive = target.activeTabIndex;
        state.openTabs = [];
        for (const path of savedPaths) {
            try {
                await runAsyncStep(target, () => openFile(path, false));
            } catch (err) {
                console.warn('Failed to restore Code Studio tab', path, err);
            }
            if (!isLiveInstance(target)) return;
        }
        if (!isLiveInstance(target)) return;
        runWithInstance(target, () => {
            if (state.openTabs.length) {
                activateTab(Math.min(Math.max(desiredActive, 0), state.openTabs.length - 1));
            } else {
                renderTabs();
                renderEditor();
            }
        });
    }

    async function openFile(path, persist) {
        const target = state;
        if (!target) return;
        path = normalizeCodeStudioPath(path);
        if (path === WORKSPACE_ROOT) {
            await refreshFiles(WORKSPACE_ROOT);
            return;
        }
        const existing = state.openTabs.findIndex(tab => tab.path === path);
        if (existing >= 0) {
            activateTab(existing);
            return;
        }
        renderStatus(tr('codeStudio.editorLoading', 'Loading editor...'));
        const result = await apiClient.file(path);
        if (!isLiveInstance(target)) return;
        runWithInstance(target, () => {
            const tab = {
                path,
                content: result.content || '',
                modified: false,
                language: languageForPath(path),
                view: null
            };
            state.openTabs.push(tab);
            state.recentFiles = [path, ...state.recentFiles.filter(item => item !== path)].slice(0, 20);
            activateTab(state.openTabs.length - 1, persist !== false);
            if (persist !== false) saveState();
        });
    }

    function activateTab(index, persist) {
        state.activeTabIndex = index;
        renderTabs();
        renderBreadcrumbs();
        renderEditor();
        renderStatus();
        if (persist !== false) saveState();
    }

    function closeTab(index) {
        const tab = state.openTabs[index];
        if (!tab) return;
        destroyTabView(tab);
        state.openTabs.splice(index, 1);
        if (state.activeTabIndex >= state.openTabs.length) state.activeTabIndex = state.openTabs.length - 1;
        renderTabs();
        renderBreadcrumbs();
        renderEditor();
        renderStatus();
        saveState();
    }

    function reorderTab(fromIndex, toIndex, insertBefore) {
        if (fromIndex < 0 || fromIndex >= state.openTabs.length) return;
        const tab = state.openTabs.splice(fromIndex, 1)[0];
        let newIndex = toIndex;
        if (fromIndex < toIndex) newIndex--;
        if (!insertBefore) newIndex++;
        newIndex = Math.max(0, Math.min(state.openTabs.length, newIndex));
        state.openTabs.splice(newIndex, 0, tab);
        if (state.activeTabIndex === fromIndex) {
            state.activeTabIndex = newIndex;
        } else if (fromIndex < state.activeTabIndex && newIndex >= state.activeTabIndex) {
            state.activeTabIndex--;
        } else if (fromIndex > state.activeTabIndex && newIndex <= state.activeTabIndex) {
            state.activeTabIndex++;
        }
        renderTabs();
        saveState();
    }

    async function saveCurrentFile() {
        const target = state;
        if (!isLiveInstance(target)) return false;
        const tab = activeTab();
        if (!tab) return false;
        tab.content = editorValue(tab);
        const content = tab.content;
        const path = tab.path;
        await apiClient.writeFile(path, content);
        if (!isLiveInstance(target)) return false;
        return runWithInstance(target, () => {
            tab.modified = false;
            renderTabs();
            renderStatus(tr('codeStudio.save', 'Save'));
            saveState();
            return true;
        });
    }

    async function createNewFile() {
        const target = state;
        if (!isLiveInstance(target)) return;
        const name = await promptValue(tr('codeStudio.newFile', 'New File'), 'main.go');
        if (!name) return;
        if (!isLiveInstance(target)) return;
        const currentPath = target.currentPath;
        const path = joinPath(currentPath, name);
        try {
            await apiClient.writeFile(path, '');
            if (!isLiveInstance(target)) return;
            await runAsyncStep(target, () => refreshFiles(currentPath));
            if (!isLiveInstance(target)) return;
            await runAsyncStep(target, () => openFile(path));
        } catch (err) {
            if (isLiveInstance(target)) runWithInstance(target, () => showOperationError(err));
        }
    }

    async function createNewFolder() {
        const target = state;
        if (!isLiveInstance(target)) return;
        const name = await promptValue(tr('codeStudio.newFolder', 'New Folder'), 'src');
        if (!name) return;
        if (!isLiveInstance(target)) return;
        const currentPath = target.currentPath;
        try {
            await apiClient.createDirectory(joinPath(currentPath, name));
            if (!isLiveInstance(target)) return;
            await runAsyncStep(target, () => refreshFiles(currentPath));
            if (!isLiveInstance(target)) return;
            runWithInstance(target, () => renderStatus(tr('codeStudio.newFolder', 'New Folder') + ': ' + name));
        } catch (err) {
            if (isLiveInstance(target)) runWithInstance(target, () => showOperationError(err));
        }
    }

    async function renamePath(file) {
        const target = state;
        if (!isLiveInstance(target)) return;
        const name = await promptValue(tr('codeStudio.rename', 'Rename'), file.name);
        if (!name || name === file.name) return;
        if (!isLiveInstance(target)) return;
        const newPath = joinPath(parentPath(file.path), name);
        await apiClient.renamePath(file.path, newPath);
        if (!isLiveInstance(target)) return;
        const currentPath = target.currentPath;
        runWithInstance(target, () => {
            state.openTabs.forEach(tab => {
                if (tab.path === file.path) tab.path = newPath;
                else if (file.type === 'directory' && tab.path.startsWith(file.path + '/')) {
                    tab.path = newPath + tab.path.slice(file.path.length);
                }
            });
        });
        await runAsyncStep(target, () => refreshFiles(currentPath));
        if (!isLiveInstance(target)) return;
        runWithInstance(target, () => {
            renderTabs();
            renderStatus();
            saveState();
        });
    }

    async function deletePath(file) {
        const target = state;
        if (!isLiveInstance(target)) return;
        const confirmed = await confirmValue(tr('codeStudio.deleteConfirm', 'Are you sure you want to delete {{name}}?', { name: file.name }));
        if (!confirmed) return;
        if (!isLiveInstance(target)) return;
        await apiClient.deletePath(file.path);
        if (!isLiveInstance(target)) return;
        const currentPath = target.currentPath;
        runWithInstance(target, () => {
            const removedTabs = state.openTabs.filter(tab => tab.path === file.path || tab.path.startsWith(file.path + '/'));
            removedTabs.forEach(destroyTabView);
            state.openTabs = state.openTabs.filter(tab => !removedTabs.includes(tab));
            if (state.activeTabIndex >= state.openTabs.length) state.activeTabIndex = state.openTabs.length - 1;
        });
        await runAsyncStep(target, () => refreshFiles(currentPath));
        if (!isLiveInstance(target)) return;
        runWithInstance(target, () => {
            renderTabs();
            renderEditor();
            renderStatus();
            saveState();
        });
    }

    function downloadFile(file) {
        if (!file || file.type !== 'file') return;
        const target = state;
        const ctx = target && target.context || {};
        if (typeof ctx.exportDesktopFile === 'function') {
            ctx.exportDesktopFile({
                path: file.path,
                name: file.name || baseName(file.path),
                url: '/api/code-studio/download?path=' + encodeURIComponent(file.path)
            }).catch(err => {
                if (isLiveInstance(target)) runWithInstance(target, () => showOperationError(err));
            });
            return;
        }
        window.location.href = '/api/code-studio/download?path=' + encodeURIComponent(file.path);
    }

    async function uploadFile() {
        const target = state;
        if (!isLiveInstance(target)) return;
        const ctx = target.context || {};
        if (typeof ctx.importFilesFromHost === 'function') {
            const currentPath = target.currentPath;
            const result = await ctx.importFilesFromHost({
                path: currentPath,
                multiple: false,
                uploadURL: '/api/code-studio/upload'
            });
            if (result && !result.canceled && isLiveInstance(target)) await runAsyncStep(target, () => refreshFiles(currentPath));
            return;
        }
        const input = document.createElement('input');
        input.type = 'file';
        input.addEventListener('change', bind(async () => {
            const target = state;
            if (!isLiveInstance(target)) return;
            if (!input.files || !input.files[0]) return;
            const currentPath = target.currentPath;
            const file = input.files[0];
            try {
                await apiClient.uploadFile(currentPath, file);
                if (!isLiveInstance(target)) return;
                await runAsyncStep(target, () => refreshFiles(currentPath));
            } catch (err) {
                if (isLiveInstance(target)) runWithInstance(target, () => showOperationError(err));
            }
        }), { once: true });
        input.click();
    }

    async function openFileFromDialog() {
        const target = state;
        if (!isLiveInstance(target)) return;
        const ctx = target.context || {};
        if (typeof ctx.openFileDialog !== 'function') return;
        const result = await ctx.openFileDialog({
            title: tr('desktop.file_dialog_open', 'Open'),
            initialPath: codeStudioDesktopPath(target.currentPath),
            filters: [{
                label: tr('codeStudio.files', 'Files'),
                extensions: ['.go', '.js', '.mjs', '.ts', '.tsx', '.jsx', '.py', '.rs', '.json', '.html', '.css', '.md', '.txt', '.yaml', '.yml']
            }]
        });
        if (!result || result.canceled || !result.path || !isLiveInstance(target)) return;
        await runAsyncStep(target, () => openFile(desktopToCodeStudioPath(result.path)));
    }

    async function runCurrentFile() {
        const target = state;
        if (!isLiveInstance(target)) return;
        const tab = activeTab();
        if (!tab) return;
        if (tab.modified) {
            await runAsyncStep(target, saveCurrentFile);
            if (!isLiveInstance(target)) return;
        }
        const command = runWithInstance(target, () => runCommandFor(tab.path));
        const cwd = runWithInstance(target, () => tab.path.slice(0, Math.max(WORKSPACE_ROOT.length, tab.path.lastIndexOf('/'))));
        runWithInstance(target, () => {
            renderStatus(tr('codeStudio.running', 'Running...'));
            writeTerminalLine('$ ' + command);
        });
        try {
            const result = await api('/api/code-studio/exec', {
                method: 'POST',
                body: JSON.stringify({ command, cwd, timeout_seconds: 300 })
            });
            if (!isLiveInstance(target)) return;
            runWithInstance(target, () => {
                writeTerminalLine(result.output || '');
                writeTerminalLine('exit ' + result.exit_code);
                renderStatus(tr('codeStudio.stopped', 'Stopped'));
            });
        } catch (err) {
            if (isLiveInstance(target)) {
                runWithInstance(target, () => {
                    writeTerminalLine(err.message || String(err));
                    renderStatus(tr('codeStudio.containerError', 'Container error: {error}', { error: err.message || String(err) }));
                });
            }
        }
    }

    function toggleSidebar() {
        state.sidebarVisible = !state.sidebarVisible;
        ensureShellRoot().dataset.sidebar = state.sidebarVisible ? 'visible' : 'hidden';
        saveState();
        renderActivityBar();
        renderWindowMenus();
    }

    function toggleTerminal() {
        state.terminalVisible = !state.terminalVisible;
        ensureShellRoot().dataset.terminal = state.terminalVisible ? 'visible' : 'hidden';
        if (state.fitAddon && state.terminalVisible) setTimeout(bind(() => state.fitAddon.fit()), 50);
        saveState();
        renderActivityBar();
        renderWindowMenus();
    }

    function toggleZenMode() {
        state.zenMode = !state.zenMode;
        const root = ensureShellRoot();
        root.dataset.zen = state.zenMode ? 'true' : 'false';
        if (state.zenMode) {
            root.querySelector('[data-zen-exit]')?.addEventListener('click', bind(() => toggleZenMode()));
        }
    }

    function wireSidebarResize() {
        const handle = shellPart('[data-sidebar-resize]');
        if (!handle) return;
        let startX = 0;
        let startWidth = 0;
        const onPointerDown = bind(event => {
            event.preventDefault();
            event.stopPropagation();
            const root = studioRoot();
            if (!root) return;
            startWidth = parseInt(root.style.getPropertyValue('--cs-sidebar-width')) || state.sidebarWidth || 280;
            startX = event.clientX;
            handle.classList.add('dragging');
            handle.setPointerCapture(event.pointerId);
            handle.addEventListener('pointermove', onPointerMove);
            handle.addEventListener('pointerup', onPointerUp);
            handle.addEventListener('pointercancel', onPointerUp);
        });
        const onPointerMove = bind(event => {
            const delta = event.clientX - startX;
            const newWidth = Math.max(180, Math.min(500, startWidth + delta));
            const root = studioRoot();
            if (root) root.style.setProperty('--cs-sidebar-width', newWidth + 'px');
            state.sidebarWidth = newWidth;
        });
        const onPointerUp = bind(event => {
            handle.classList.remove('dragging');
            handle.releasePointerCapture(event.pointerId);
            handle.removeEventListener('pointermove', onPointerMove);
            handle.removeEventListener('pointerup', onPointerUp);
            handle.removeEventListener('pointercancel', onPointerUp);
            saveState();
        });
        handle.addEventListener('pointerdown', onPointerDown);
    }

    function writeTerminalLine(line) {
        if (state.terminal) {
            String(line || '').split('\n').forEach(part => state.terminal.writeln(part));
            return;
        }
        const screen = state.root.querySelector('[data-terminal-screen]');
        if (screen) screen.textContent += String(line || '') + '\n';
    }

    function showOperationError(err) {
        const message = err && err.message ? err.message : String(err || '');
        renderStatus(tr('codeStudio.containerError', 'Container error: {error}', { error: message }));
        writeTerminalLine(message);
    }

    function languageForPath(path) {
        const ext = String(path || '').split('.').pop().toLowerCase();
        return ({ js: 'javascript', mjs: 'javascript', ts: 'javascript', jsx: 'javascript', tsx: 'javascript', py: 'python', go: 'go', rs: 'rust', json: 'json', html: 'html', htm: 'html', css: 'css', md: 'markdown' })[ext] || '';
    }

    function languageExtension(cm, language) {
        const map = { javascript: cm.javascript, python: cm.python, go: cm.go, rust: cm.rust, json: cm.json, html: cm.html, css: cm.css, markdown: cm.markdown };
        const factory = map[language];
        return typeof factory === 'function' ? factory() : [];
    }

    function runCommandFor(path) {
        const quoted = "'" + String(path).replaceAll("'", "'\"'\"'") + "'";
        const lang = languageForPath(path);
        if (lang === 'go') return 'go run ' + quoted;
        if (lang === 'python') return 'python3 ' + quoted;
        if (lang === 'javascript') return 'node ' + quoted;
        if (lang === 'rust') return 'rustc ' + quoted + ' -o /tmp/cs-run && /tmp/cs-run';
        return 'cat ' + quoted;
    }

    function fileIcon(name) {
        const lang = languageForPath(name);
        return ({ javascript: 'JS', python: 'PY', go: 'GO', rust: 'RS', json: '{}', html: '<>', css: '#', markdown: 'MD' })[lang] || '\u2022';
    }

    function editorValue(tab) {
        if (!tab) return '';
        if (tab.view && tab.view.state && tab.view.state.doc) return tab.view.state.doc.toString();
        if (tab.view && typeof tab.view.getValue === 'function') return tab.view.getValue();
        return tab.content || '';
    }

    function activeTab() {
        if (!state) return null;
        return state.openTabs[state.activeTabIndex] || null;
    }

    function currentDirectory() {
        const tab = activeTab();
        if (tab && tab.path) return tab.path.slice(0, Math.max(WORKSPACE_ROOT.length, tab.path.lastIndexOf('/')));
        return state && state.currentPath || WORKSPACE_ROOT;
    }

    function studioRoot() {
        if (!state || !state.root) return null;
        return state.root.querySelector('[data-code-studio]');
    }

    function ensureShellRoot() {
        let root = studioRoot();
        if (!root) {
            state.root.innerHTML = shellMarkup();
            root = studioRoot();
        }
        return root;
    }

    function shellPart(selector) {
        const root = ensureShellRoot();
        return root ? root.querySelector(selector) : null;
    }

    function joinPath(base, name) {
        if (!base || base === WORKSPACE_ROOT) return WORKSPACE_ROOT + '/' + name;
        return base + '/' + name;
    }

    function parentPath(path) {
        const clean = String(path || WORKSPACE_ROOT).replace(/\/+$/, '');
        const index = clean.lastIndexOf('/');
        if (index <= 0) return WORKSPACE_ROOT;
        return clean.slice(0, index) || WORKSPACE_ROOT;
    }

    function baseName(path) {
        return String(path || '').split('/').filter(Boolean).pop() || WORKSPACE_ROOT;
    }

    function cursorPositionText(tab) {
        if (!tab || !tab.view) return '';
        let line = 1, col = 1;
        if (tab.view.state && tab.view.state.doc) {
            const head = tab.view.state.selection.main.head;
            const ln = tab.view.state.doc.lineAt(head);
            line = ln.number;
            col = head - ln.from + 1;
        } else if (tab.view.textarea) {
            const value = tab.view.textarea.value.slice(0, tab.view.textarea.selectionStart || 0);
            const lines = value.split('\n');
            line = lines.length;
            col = lines[lines.length - 1].length + 1;
        }
        return 'Ln ' + line + ', Col ' + col;
    }

    function formatBytes(size) {
        const n = Number(size || 0);
        if (n < 1024) return n + ' B';
        if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KiB';
        return (n / 1024 / 1024).toFixed(1) + ' MiB';
    }

    function codeStudioCursor() {
        const tab = activeTab();
        if (!tab || !tab.view) return { line: 0, column: 0 };
        if (tab.view.state && tab.view.state.doc) {
            const head = tab.view.state.selection.main.head;
            const line = tab.view.state.doc.lineAt(head);
            return { line: line.number, column: head - line.from + 1 };
        }
        if (tab.view.textarea) {
            const value = tab.view.textarea.value.slice(0, tab.view.textarea.selectionStart || 0);
            const lines = value.split('\n');
            return { line: lines.length, column: lines[lines.length - 1].length + 1 };
        }
        return { line: 0, column: 0 };
    }

    function codeStudioSelection() {
        const tab = activeTab();
        if (!tab || !tab.view) return { text: '' };
        if (tab.view.state && tab.view.state.doc) {
            const range = tab.view.state.selection.main;
            if (range.empty) return { text: '' };
            return { text: tab.view.state.doc.sliceString(range.from, range.to) };
        }
        if (tab.view.textarea) {
            const start = tab.view.textarea.selectionStart || 0;
            const end = tab.view.textarea.selectionEnd || 0;
            return { text: start === end ? '' : tab.view.textarea.value.slice(start, end) };
        }
        return { text: '' };
    }

    function promptValue(title, value) {
        return new Promise(resolve => {
            const instance = state;
            const overlay = document.createElement('div');
            overlay.className = 'cs-modal-backdrop';
            overlay.innerHTML = `<form class="cs-modal">
                <label>${esc(title)}<input name="value" value="${esc(value || '')}" autocomplete="off" spellcheck="false"></label>
                <div class="cs-modal-actions">
                    <button type="button" class="cs-button" data-cancel>${buttonIcon('x', 'X')}<span>${esc(tr('desktop.cancel', 'Cancel'))}</span></button>
                    <button type="submit" class="cs-button primary">${buttonIcon('check-square', 'Y')}<span>${esc(tr('desktop.ok', 'OK'))}</span></button>
                </div>
            </form>`;
            document.body.appendChild(overlay);
            const input = overlay.querySelector('input');
            let settled = false;
            let unregister = () => {};
            const cleanup = result => {
                if (settled) return;
                settled = true;
                unregister();
                overlay.remove();
                resolve(result);
            };
            runWithInstance(instance, () => {
                unregister = registerDisposer(() => cleanup(''));
            });
            overlay.querySelector('form').addEventListener('submit', bind(event => {
                event.preventDefault();
                cleanup(input.value.trim());
            }));
            overlay.querySelector('[data-cancel]').addEventListener('click', bind(() => cleanup('')));
            overlay.addEventListener('click', bind(event => {
                if (event.target === overlay) cleanup('');
            }));
            input.focus();
            input.select();
        });
    }

    function confirmValue(message) {
        return new Promise(resolve => {
            const instance = state;
            const overlay = document.createElement('div');
            overlay.className = 'cs-modal-backdrop';
            overlay.innerHTML = `<div class="cs-modal">
                <p>${esc(message)}</p>
                <div class="cs-modal-actions">
                    <button type="button" class="cs-button" data-cancel>${buttonIcon('x', 'X')}<span>${esc(tr('desktop.cancel', 'Cancel'))}</span></button>
                    <button type="button" class="cs-button danger" data-confirm>${buttonIcon('trash', 'X')}<span>${esc(tr('desktop.delete', 'Delete'))}</span></button>
                </div>
            </div>`;
            document.body.appendChild(overlay);
            let settled = false;
            let unregister = () => {};
            const cleanup = result => {
                if (settled) return;
                settled = true;
                unregister();
                overlay.remove();
                resolve(result);
            };
            runWithInstance(instance, () => {
                unregister = registerDisposer(() => cleanup(false));
            });
            overlay.querySelector('[data-confirm]').addEventListener('click', bind(() => cleanup(true)));
            overlay.querySelector('[data-cancel]').addEventListener('click', bind(() => cleanup(false)));
            overlay.addEventListener('click', bind(event => {
                if (event.target === overlay) cleanup(false);
            }));
            overlay.querySelector('[data-confirm]').focus();
        });
    }

    function runOnWindow(windowId, fn) {
        const instance = windowId ? instances.get(windowId) : currentInstance();
        if (!instance) return undefined;
        latestWindowId = instance.windowId;
        return runWithInstance(instance, fn);
    }
