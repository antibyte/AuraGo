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
            pendingSuggestion: null,
            shortcutsWired: false,
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
        let result;
        try {
            result = fn(instance);
        } finally {
            state = previous;
        }
        return result;
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
            await runAsyncStep(instance, () => refreshFiles(context && context.path ? context.path : state.currentPath));
            if (!isLiveInstance(instance)) return;
            await runAsyncStep(instance, restoreTabs);
            if (!isLiveInstance(instance)) return;
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
                <aside class="code-studio-sidebar" data-sidebar></aside>
                <main class="code-studio-main">
                    <div class="code-studio-tabs" data-tabs></div>
                    <div class="code-studio-editor" data-editor></div>
                    <div class="code-studio-terminal" data-terminal></div>
                </main>
                <aside class="code-studio-chat" data-agent-panel></aside>
            </div>
            <div class="code-studio-statusbar" data-statusbar></div>
        </div>`;
    }

    function renderShell() {
        const root = ensureShellRoot();
        if (state.context && typeof state.context.wireContextMenuBoundary === 'function') state.context.wireContextMenuBoundary(root);
        root.style.setProperty('--cs-sidebar-width', Math.max(220, state.sidebarWidth) + 'px');
        root.style.setProperty('--cs-terminal-height', Math.max(120, state.terminalHeight) + 'px');
        root.style.setProperty('--cs-editor-font-size', clampEditorFontSize(state.editorFontSize) + 'px');
        root.dataset.terminal = state.terminalVisible ? 'visible' : 'hidden';
        root.dataset.sidebar = state.sidebarVisible ? 'visible' : 'hidden';
        root.dataset.agent = state.agentVisible ? 'visible' : 'hidden';
        renderToolbar();
        renderSearchPanel();
        renderSidebar();
        renderTabs();
        renderEditor();
        renderTerminal();
        renderAgentPanel();
        renderStatus();
        renderWindowMenus();
        wireShortcuts();
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
        toolbar.innerHTML = `
            <button type="button" class="cs-button" data-action="new-file">${buttonIcon('file-plus', '+')}<span>${esc(tr('codeStudio.newFile', 'New File'))}</span></button>
            <button type="button" class="cs-button" data-action="new-folder">${buttonIcon('folder-plus', '+')}<span>${esc(tr('codeStudio.newFolder', 'New Folder'))}</span></button>
            <button type="button" class="cs-button primary" data-action="save">${buttonIcon('save', 'S')}<span>${esc(tr('codeStudio.save', 'Save'))}</span></button>
            <button type="button" class="cs-button" data-action="run">${buttonIcon('run', 'R')}<span>${esc(tr('codeStudio.run', 'Run'))}</span></button>
            <button type="button" class="cs-button" data-action="search">${buttonIcon('search', 'S')}<span>${esc(tr('codeStudio.search', 'Search'))}</span></button>
            <button type="button" class="cs-button" data-action="agent">${buttonIcon('chat', 'A')}<span>${esc(tr('codeStudio.agentChat', 'Agent Chat'))}</span></button>
            <button type="button" class="cs-button" data-action="upload">${buttonIcon('upload', 'U')}<span>${esc(tr('codeStudio.upload', 'Upload'))}</span></button>
            <button type="button" class="cs-icon-button" data-action="refresh" title="${esc(tr('codeStudio.refresh', 'Refresh'))}">${iconMarkup('refresh', 'R', 'cs-icon-button-icon', 16)}</button>
            <button type="button" class="cs-icon-button" data-action="terminal" title="${esc(tr('codeStudio.toggleTerminal', 'Toggle Terminal'))}">${iconMarkup('terminal', 'T', 'cs-icon-button-icon', 16)}</button>
            <span class="cs-toolbar-spacer"></span>
            <span class="cs-pill">${esc(state.editorType === 'codemirror' ? 'CodeMirror' : tr('codeStudio.editorFallback', 'Basic editor'))}</span>`;
        toolbar.querySelector('[data-action="new-file"]').addEventListener('click', bind(createNewFile));
        toolbar.querySelector('[data-action="new-folder"]').addEventListener('click', bind(createNewFolder));
        toolbar.querySelector('[data-action="save"]').addEventListener('click', bind(saveCurrentFile));
        toolbar.querySelector('[data-action="run"]').addEventListener('click', bind(runCurrentFile));
        toolbar.querySelector('[data-action="search"]').addEventListener('click', bind(toggleSearch));
        toolbar.querySelector('[data-action="agent"]').addEventListener('click', bind(toggleAgentPanel));
        toolbar.querySelector('[data-action="upload"]').addEventListener('click', bind(uploadFile));
        toolbar.querySelector('[data-action="refresh"]').addEventListener('click', bind(() => refreshFiles(state.currentPath)));
        toolbar.querySelector('[data-action="terminal"]').addEventListener('click', bind(toggleTerminal));
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
        const messages = state.agentMessages.length ? state.agentMessages.map(message => `
            <div class="cs-agent-message ${esc(message.role)}">${esc(message.text)}</div>`).join('') : `
            <div class="cs-agent-message agent">${esc(tr('desktop.chat_welcome', 'Ask me to create apps, widgets, or files for this desktop.'))}</div>`;
        const suggestion = state.pendingSuggestion ? `<div class="code-studio-diff">
            <div class="cs-diff-head">
                <strong>${esc(tr('codeStudio.applyChanges', 'Apply Changes'))}</strong>
                <button type="button" class="cs-button primary" data-agent-apply>${buttonIcon('check-square', 'Y')}<span>${esc(tr('codeStudio.applyChanges', 'Apply Changes'))}</span></button>
                <button type="button" class="cs-button" data-agent-discard>${buttonIcon('x', 'X')}<span>${esc(tr('codeStudio.discardChanges', 'Discard Changes'))}</span></button>
            </div>
            <pre>${esc(state.pendingSuggestion)}</pre>
        </div>` : '';
        panel.innerHTML = `<div class="cs-agent-head">
            <strong>${esc(tr('codeStudio.agentChat', 'Agent Chat'))}</strong>
            <button type="button" class="cs-icon-button" data-agent-close title="${esc(tr('codeStudio.closeTab', 'Close tab'))}">${iconMarkup('x', 'X', 'cs-icon-button-icon', 16)}</button>
        </div>
        <div class="cs-agent-actions">
            <button type="button" class="cs-button" data-code-action="explain">${buttonIcon('info', 'i')}<span>${esc(tr('codeStudio.explain', 'Explain'))}</span></button>
            <button type="button" class="cs-button" data-code-action="comments">${buttonIcon('notes', 'N')}<span>${esc(tr('codeStudio.generateComments', 'Generate Comments'))}</span></button>
            <button type="button" class="cs-button" data-code-action="tests">${buttonIcon('check-square', 'T')}<span>${esc(tr('codeStudio.generateTests', 'Generate Tests'))}</span></button>
            <button type="button" class="cs-button" data-code-action="refactor">${buttonIcon('tools', 'R')}<span>${esc(tr('codeStudio.refactor', 'Refactor'))}</span></button>
        </div>
        <div class="cs-agent-log">${messages}</div>
        ${suggestion}
        <form class="cs-agent-form" data-agent-form>
            <input name="message" autocomplete="off" spellcheck="false" placeholder="${esc(tr('desktop.chat_placeholder', 'Ask the agent...'))}">
            <button type="submit" class="cs-button primary">${buttonIcon('chat', 'S')}<span>${esc(tr('desktop.send', 'Send'))}</span></button>
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
        const apply = panel.querySelector('[data-agent-apply]');
        if (apply) apply.addEventListener('click', bind(applyAgentSuggestion));
        const discard = panel.querySelector('[data-agent-discard]');
        if (discard) discard.addEventListener('click', bind(() => {
            state.pendingSuggestion = null;
            renderAgentPanel();
        }));
    }

    function renderSidebar(errorMessage) {
        const sidebar = shellPart('[data-sidebar]');
        if (!sidebar) return;
        if (errorMessage) {
            sidebar.innerHTML = `<div class="cs-sidebar-head"><strong>${esc(tr('codeStudio.title', 'Code Studio'))}</strong></div>
                <div class="code-studio-error compact">${esc(errorMessage)}</div>`;
            return;
        }
        const rows = state.files.length ? state.files.map(file => fileRow(file)).join('') : `<div class="cs-empty">${esc(tr('codeStudio.noFiles', 'No files open'))}</div>`;
        sidebar.innerHTML = `<div class="cs-sidebar-head">
            <strong>${esc(tr('codeStudio.title', 'Code Studio'))}</strong>
            <span>${esc(state.currentPath)}</span>
        </div><div class="cs-file-tree">${rows}</div>`;
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
        sidebar.querySelectorAll('[data-file-path]').forEach(row => {
            row.addEventListener('click', bind(event => {
                const action = event.target.closest('[data-file-action]');
                if (action) return;
                const file = state.files.find(item => item.path === row.dataset.filePath);
                if (!file) return;
                if (file.type === 'directory') refreshFiles(file.path);
                else openFile(file.path);
            }));
            row.addEventListener('keydown', bind(event => {
                const file = state.files.find(item => item.path === row.dataset.filePath);
                if (!file) return;
                if (event.key === 'Enter') {
                    event.preventDefault();
                    if (file.type === 'directory') refreshFiles(file.path);
                    else openFile(file.path);
                }
                if (event.key === 'F2') {
                    event.preventDefault();
                    renamePath(file);
                }
                if (event.key === 'Delete') {
                    event.preventDefault();
                    deletePath(file);
                }
            }));
        });
        sidebar.querySelectorAll('[data-file-action]').forEach(btn => {
            btn.addEventListener('click', bind(event => {
                event.stopPropagation();
                const file = state.files.find(item => item.path === btn.closest('[data-file-path]').dataset.filePath);
                if (!file) return;
                const action = btn.dataset.fileAction;
                if (action === 'rename') renamePath(file);
                if (action === 'delete') deletePath(file);
                if (action === 'download') downloadFile(file);
            }));
            btn.addEventListener('keydown', bind(event => {
                if (event.key !== 'Enter' && event.key !== ' ') return;
                event.preventDefault();
                btn.click();
            }));
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
            <button type="button" class="cs-tab ${index === state.activeTabIndex ? 'active' : ''}" data-tab="${index}">
                <span>${esc(baseName(tab.path))}${tab.modified ? ' *' : ''}</span>
                <span class="cs-tab-close" data-close="${index}" title="${esc(tr('codeStudio.closeTab', 'Close tab'))}">${iconMarkup('x', 'X', 'cs-tab-close-icon', 12)}</span>
            </button>`).join('') : `<div class="cs-tabs-empty">${esc(tr('codeStudio.noFiles', 'No files open'))}</div>`;
        tabs.querySelectorAll('[data-tab]').forEach(btn => btn.addEventListener('click', bind(event => {
            if (event.target.closest('[data-close]')) return;
            activateTab(Number(btn.dataset.tab));
        })));
        tabs.querySelectorAll('[data-close]').forEach(btn => btn.addEventListener('click', bind(event => {
            event.stopPropagation();
            closeTab(Number(btn.dataset.close));
        })));
        renderWindowMenus();
    }

    function renderEditor() {
        const editor = shellPart('[data-editor]');
        if (!editor) return;
        const tab = activeTab();
        if (!tab) {
            state.openTabs.forEach(destroyTabView);
            editor.innerHTML = `<div class="cs-editor-empty">${esc(tr('codeStudio.noFiles', 'No files open'))}</div>`;
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

    function renderTerminal() {
        const terminal = shellPart('[data-terminal]');
        if (!terminal) return;
        terminal.innerHTML = `<div class="cs-terminal-head">
            <strong>${esc(tr('codeStudio.terminal', 'Terminal'))}</strong>
            <span data-terminal-state>${esc(tr('codeStudio.stopped', 'Stopped'))}</span>
        </div><div class="cs-terminal-screen" data-terminal-screen></div>`;
    }

    function applyEditorZoom() {
        const root = shellPart('.code-studio');
        if (!root) return;
        root.style.setProperty('--cs-editor-font-size', clampEditorFontSize(state.editorFontSize) + 'px');
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

    function renderStatus(message) {
        const status = shellPart('[data-statusbar]');
        if (!status) return;
        const tab = activeTab();
        status.innerHTML = `<span>${esc(message || state.containerStatus)}</span>
            <span>${esc(tab ? tab.path : tr('codeStudio.noFiles', 'No files open'))}</span>
            <span>${esc(state.editorType || '')}</span>`;
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

    async function refreshFiles(path) {
        const target = state;
        if (!target) return;
        const nextPath = path || WORKSPACE_ROOT;
        state.currentPath = nextPath;
        try {
            const result = await apiClient.files(nextPath);
            if (!isLiveInstance(target)) return;
            runWithInstance(target, () => {
                state.files = result.files || [];
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
        renderEditor();
        renderStatus();
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
