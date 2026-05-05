(function () {
    const STATE_KEY = 'aurago.codeStudio.state.v1';
    const WORKSPACE_ROOT = '/workspace';

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
        root.style.setProperty('--cs-sidebar-width', Math.max(220, state.sidebarWidth) + 'px');
        root.style.setProperty('--cs-terminal-height', Math.max(120, state.terminalHeight) + 'px');
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
                    { id: 'agent-panel', labelKey: 'desktop.menu_agent_panel', icon: 'chat', checked: state.agentVisible, action: bind(toggleAgentPanel) }
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
        window.location.href = '/api/code-studio/download?path=' + encodeURIComponent(file.path);
    }

    function uploadFile() {
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

    function toggleSearch() {
        state.searchVisible = !state.searchVisible;
        renderSearchPanel();
    }

    async function runSearch(formData) {
        const target = state;
        if (!isLiveInstance(target)) return;
        const query = String(formData.get('q') || '').trim();
        if (!query) return;
        renderStatus(tr('codeStudio.search', 'Search'));
        const currentPath = target.currentPath || WORKSPACE_ROOT;
        const result = await apiClient.search({
            q: query,
            path: currentPath,
            case: formData.get('case') ? 'true' : 'false',
            whole: formData.get('whole') ? 'true' : 'false',
            regex: formData.get('regex') ? 'true' : 'false',
            include: String(formData.get('include') || ''),
            exclude: String(formData.get('exclude') || '')
        });
        if (!isLiveInstance(target)) return;
        runWithInstance(target, () => {
            state.searchResults = result.results || [];
            renderSearchPanel();
            renderStatus(tr('codeStudio.search', 'Search') + ': ' + state.searchResults.length);
        });
    }

    async function openSearchResult(path, line) {
        const target = state;
        if (!isLiveInstance(target)) return;
        await runAsyncStep(target, () => openFile(path));
        if (!isLiveInstance(target)) return;
        runWithInstance(target, () => {
            const tab = activeTab();
            if (!tab || !tab.view) return;
            if (tab.view.state && tab.view.state.doc && state.cmModule && state.cmModule.EditorView) {
                const docLine = tab.view.state.doc.line(Math.max(1, line || 1));
                tab.view.dispatch({
                    selection: { anchor: docLine.from },
                    effects: state.cmModule.EditorView.scrollIntoView(docLine.from, { y: 'center' })
                });
            } else if (tab.view.textarea) {
                const lines = tab.view.textarea.value.split('\n');
                const offset = lines.slice(0, Math.max(0, (line || 1) - 1)).join('\n').length;
                tab.view.textarea.focus();
                tab.view.textarea.setSelectionRange(offset, offset);
            }
        });
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

    function toggleAgentPanel() {
        state.agentVisible = !state.agentVisible;
        ensureShellRoot().dataset.agent = state.agentVisible ? 'visible' : 'hidden';
        renderAgentPanel();
        renderWindowMenus();
    }

    async function sendAgentMessage(message) {
        const target = state;
        if (!isLiveInstance(target)) return;
        if (state.agentBusy) return;
        let context;
        runWithInstance(target, () => {
            state.agentVisible = true;
            ensureShellRoot().dataset.agent = 'visible';
            state.agentMessages.push({ role: 'user', text: message });
            state.agentMessages.push({ role: 'agent', text: tr('desktop.thinking', 'Working...') });
            state.agentBusy = true;
            context = codeStudioAgentContext();
            renderAgentPanel();
        });
        try {
            const response = await apiClient.agentChat(message, context);
            if (!isLiveInstance(target)) return;
            const answer = response.answer || tr('desktop.done', 'Done');
            runWithInstance(target, () => {
                state.agentMessages[state.agentMessages.length - 1] = { role: 'agent', text: answer };
                const suggestion = extractFirstCodeBlock(answer);
                if (suggestion) state.pendingSuggestion = suggestion;
            });
        } catch (err) {
            if (isLiveInstance(target)) {
                runWithInstance(target, () => {
                    state.agentMessages[state.agentMessages.length - 1] = { role: 'agent', text: err.message || String(err) };
                });
            }
        } finally {
            if (isLiveInstance(target)) {
                runWithInstance(target, () => {
                    state.agentBusy = false;
                    renderAgentPanel();
                });
            }
        }
    }

    function runCodeAction(action) {
        const tab = activeTab();
        if (!tab) return;
        const selection = codeStudioSelection();
        const target = selection.text ? 'selected code' : 'current file';
        const prompts = {
            explain: `Explain the ${target} in ${tab.path}.`,
            comments: `Generate clear comments for the ${target} in ${tab.path}. Return only the modified code when you change code.`,
            tests: `Generate useful tests for ${tab.path}. Return code blocks for new or changed files.`,
            refactor: `Refactor the ${target} in ${tab.path}. Return only the modified code.`
        };
        sendAgentMessage(prompts[action] || prompts.explain);
    }

    function codeStudioAgentContext() {
        const tab = activeTab();
        const cursor = codeStudioCursor();
        const selection = codeStudioSelection();
        const content = tab ? editorValue(tab) : '';
        return {
            source: 'code-studio',
            current_file: tab ? tab.path : '',
            current_language: tab ? tab.language : '',
            current_content: selection.text ? '' : content,
            cursor_line: cursor.line,
            cursor_column: cursor.column,
            selected_text: selection.text,
            open_files: state.openTabs.map(item => item.path)
        };
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

    function extractFirstCodeBlock(text) {
        const match = String(text || '').match(/```[a-zA-Z0-9_-]*\n([\s\S]*?)```/);
        return match ? match[1].trimEnd() : '';
    }

    function applyAgentSuggestion() {
        const tab = activeTab();
        if (!tab || !state.pendingSuggestion) return;
        if (tab.view && tab.view.state && tab.view.state.doc) {
            tab.view.dispatch({ changes: { from: 0, to: tab.view.state.doc.length, insert: state.pendingSuggestion } });
        } else if (tab.view && tab.view.textarea) {
            tab.view.setValue(state.pendingSuggestion);
        }
        tab.content = state.pendingSuggestion;
        tab.modified = true;
        state.pendingSuggestion = null;
        renderTabs();
        renderStatus();
        renderAgentPanel();
    }

    function showCodeActionMenu(x, y) {
        document.querySelectorAll('.cs-context-menu').forEach(menu => {
            if (typeof menu.__codeStudioCleanup === 'function') menu.__codeStudioCleanup();
            else menu.remove();
        });
        const instance = state;
        const menu = document.createElement('div');
        menu.className = 'cs-context-menu';
        menu.style.left = x + 'px';
        menu.style.top = y + 'px';
        menu.innerHTML = `
            <button type="button" data-code-action="explain">${buttonIcon('info', 'i')}<span>${esc(tr('codeStudio.explain', 'Explain'))}</span></button>
            <button type="button" data-code-action="comments">${buttonIcon('notes', 'N')}<span>${esc(tr('codeStudio.generateComments', 'Generate Comments'))}</span></button>
            <button type="button" data-code-action="tests">${buttonIcon('check-square', 'T')}<span>${esc(tr('codeStudio.generateTests', 'Generate Tests'))}</span></button>
            <button type="button" data-code-action="refactor">${buttonIcon('tools', 'R')}<span>${esc(tr('codeStudio.refactor', 'Refactor'))}</span></button>`;
        document.body.appendChild(menu);
        let boundClose = null;
        let menuClosed = false;
        let unregister = () => {};
        const cleanupMenu = () => {
            if (menuClosed) return;
            menuClosed = true;
            unregister();
            if (boundClose) document.removeEventListener('mousedown', boundClose);
            menu.remove();
        };
        menu.__codeStudioCleanup = cleanupMenu;
        runWithInstance(instance, () => {
            unregister = registerDisposer(cleanupMenu);
        });
        menu.querySelectorAll('[data-code-action]').forEach(btn => {
            btn.addEventListener('click', bind(() => {
                runCodeAction(btn.dataset.codeAction);
                cleanupMenu();
            }));
        });
        setTimeout(bind(() => {
            if (menuClosed) return;
            const close = event => {
                if (!menu.contains(event.target)) {
                    cleanupMenu();
                }
            };
            boundClose = bind(close);
            document.addEventListener('mousedown', boundClose);
        }), 0);
    }

    function connectTerminal() {
        const screen = shellPart('[data-terminal-screen]');
        const label = shellPart('[data-terminal-state]');
        if (!screen || !window.Terminal) {
            if (screen) screen.textContent = tr('codeStudio.terminalUnavailable', 'Terminal unavailable');
            return;
        }
        try {
            const term = new window.Terminal({ cursorBlink: true, convertEol: true, fontFamily: 'Consolas, monospace', fontSize: 13 });
            state.terminal = term;
            const instance = state;
            let terminalDisposed = false;
            instance.disposers.push(() => {
                if (terminalDisposed) return;
                terminalDisposed = true;
                if (term && typeof term.dispose === 'function') term.dispose();
            });
            if (window.FitAddon && window.FitAddon.FitAddon) {
                state.fitAddon = new window.FitAddon.FitAddon();
                term.loadAddon(state.fitAddon);
            }
            term.open(screen);
            if (state.fitAddon) state.fitAddon.fit();
            term.writeln('Code Studio');
            const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
            const ws = new WebSocket(protocol + '//' + location.host + '/api/code-studio/terminal');
            ws.binaryType = 'arraybuffer';
            state.ws = ws;
            ws.onopen = bindInstance(instance, () => {
                label.textContent = tr('codeStudio.running', 'Running...');
                const termDataDispose = term.onData(bindInstance(instance, data => ws.readyState === WebSocket.OPEN && ws.send(data)));
                if (termDataDispose && typeof termDataDispose.dispose === 'function') {
                    instance.disposers.push(() => termDataDispose.dispose());
                }
            });
            ws.onmessage = bindInstance(instance, event => {
                if (event.data instanceof ArrayBuffer) term.write(new Uint8Array(event.data));
                else term.write(String(event.data));
            });
            ws.onerror = bindInstance(instance, () => label.textContent = tr('codeStudio.terminalUnavailable', 'Terminal unavailable'));
            ws.onclose = bindInstance(instance, () => label.textContent = tr('codeStudio.stopped', 'Stopped'));
        } catch (err) {
            screen.textContent = tr('codeStudio.terminalUnavailable', 'Terminal unavailable');
        }
    }

    function createCodeMirrorEditor(container, tab) {
        const cm = state.cmModule;
        if (!cm || !cm.EditorState || !cm.EditorView) return createTextareaEditor(container, tab);
        const extensions = [
            cm.lineNumbers && cm.lineNumbers(),
            cm.highlightActiveLineGutter && cm.highlightActiveLineGutter(),
            cm.history && cm.history(),
            cm.drawSelection && cm.drawSelection(),
            cm.dropCursor && cm.dropCursor(),
            cm.highlightActiveLine && cm.highlightActiveLine(),
            cm.EditorView.lineWrapping,
            cm.oneDark,
            cm.closeBrackets && cm.closeBrackets(),
            cm.autocompletion && cm.autocompletion(),
            cm.highlightSelectionMatches && cm.highlightSelectionMatches(),
            cm.syntaxHighlighting && cm.defaultHighlightStyle && cm.syntaxHighlighting(cm.defaultHighlightStyle),
            languageExtension(cm, tab.language),
            cm.keymap && cm.keymap.of([
                cm.indentWithTab,
                { key: 'Ctrl-s', run: bind(() => { saveCurrentFile(); return true; }) },
                { key: 'F5', run: bind(() => { runCurrentFile(); return true; }) },
                ...(cm.searchKeymap || [])
            ].filter(Boolean)),
            cm.EditorView.updateListener.of(bind(update => {
                if (!update.docChanged) return;
                tab.modified = true;
                tab.content = update.state.doc.toString();
                renderTabs();
                renderStatus();
            }))
        ].filter(Boolean);
        return new cm.EditorView({
            state: cm.EditorState.create({ doc: tab.content, extensions }),
            parent: container
        });
    }

    function createTextareaEditor(container, tab) {
        const wrapper = document.createElement('div');
        wrapper.className = 'cs-textarea-wrap';
        const textarea = document.createElement('textarea');
        textarea.className = 'code-studio-textarea';
        textarea.value = tab.content;
        textarea.spellcheck = false;
        const preview = document.createElement('pre');
        preview.className = 'code-studio-preview hljs';
        wrapper.appendChild(textarea);
        wrapper.appendChild(preview);
        container.appendChild(wrapper);
        const updatePreview = bind(() => {
            tab.content = textarea.value;
            tab.modified = true;
            preview.textContent = textarea.value;
            if (window.hljs && tab.language) {
                try {
                    preview.innerHTML = window.hljs.highlight(textarea.value, { language: tab.language, ignoreIllegals: true }).value;
                } catch (_) {}
            }
            renderTabs();
            renderStatus();
        });
        textarea.addEventListener('input', updatePreview);
        textarea.addEventListener('keydown', bind(event => {
            if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
                event.preventDefault();
                saveCurrentFile();
            }
            if (event.key === 'Tab') {
                event.preventDefault();
                const start = textarea.selectionStart;
                const end = textarea.selectionEnd;
                textarea.value = textarea.value.slice(0, start) + '    ' + textarea.value.slice(end);
                textarea.selectionStart = textarea.selectionEnd = start + 4;
                updatePreview();
            }
        }));
        updatePreview();
        tab.modified = false;
        return { textarea, getValue: () => textarea.value, setValue: value => { textarea.value = value; updatePreview(); } };
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

    function toggleTerminal() {
        state.terminalVisible = !state.terminalVisible;
        ensureShellRoot().dataset.terminal = state.terminalVisible ? 'visible' : 'hidden';
        if (state.fitAddon && state.terminalVisible) setTimeout(bind(() => state.fitAddon.fit()), 50);
        saveState();
        renderWindowMenus();
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
        return ({ javascript: 'JS', python: 'PY', go: 'GO', rust: 'RS', json: '{}', html: '<>', css: '#', markdown: 'MD' })[lang] || '•';
    }

    function joinPath(base, name) {
        return (base || WORKSPACE_ROOT).replace(/\/+$/, '') + '/' + String(name || '').replace(/^\/+/, '');
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

    function formatBytes(size) {
        const n = Number(size || 0);
        if (n < 1024) return n + ' B';
        if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KiB';
        return (n / 1024 / 1024).toFixed(1) + ' MiB';
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

    function wireShortcuts() {
        if (state.shortcutsWired) return;
        state.shortcutsWired = true;
        const instance = state;
        const onKeydown = bindInstance(instance, event => {
            if (!state.root || !studioRoot()) return;
            const activeElement = document.activeElement;
            if (activeElement && !state.root.contains(activeElement)) return;
            const key = event.key.toLowerCase();
            if ((event.ctrlKey || event.metaKey) && key === 's') {
                event.preventDefault();
                saveCurrentFile();
            } else if ((event.ctrlKey || event.metaKey) && event.shiftKey && key === 'f') {
                event.preventDefault();
                if (!state.searchVisible) state.searchVisible = true;
                renderSearchPanel();
            } else if ((event.ctrlKey || event.metaKey) && event.shiftKey && key === 'a') {
                event.preventDefault();
                if (!state.agentVisible) toggleAgentPanel();
            } else if ((event.ctrlKey || event.metaKey) && key === 'n') {
                event.preventDefault();
                createNewFile();
            } else if (event.key === 'F5') {
                event.preventDefault();
                runCurrentFile();
            }
        });
        document.addEventListener('keydown', onKeydown);
        state.disposers.push(() => { document.removeEventListener('keydown', onKeydown); });
    }

    function runOnWindow(windowId, fn) {
        const instance = windowId ? instances.get(windowId) : currentInstance();
        if (!instance) return undefined;
        latestWindowId = instance.windowId;
        return runWithInstance(instance, fn);
    }

    function exposedLoadState(windowId) {
        return runOnWindow(windowId, loadState);
    }

    function exposedSaveState(windowId) {
        return runOnWindow(windowId, saveState);
    }

    function exposedRefreshFiles(path, windowId) {
        return runOnWindow(windowId, () => refreshFiles(path || state.currentPath));
    }

    function exposedOpenFile(path, persist, windowId) {
        return runOnWindow(windowId, () => openFile(path, persist));
    }

    function exposedSaveCurrentFile(windowId) {
        return runOnWindow(windowId, saveCurrentFile);
    }

    window.CodeStudioApp = {
        render,
        dispose,
        get state() { return currentInstance(); },
        instances,
        api: apiClient,
        loadState: exposedLoadState,
        saveState: exposedSaveState,
        refreshFiles: exposedRefreshFiles,
        openFile: exposedOpenFile,
        saveCurrentFile: exposedSaveCurrentFile
    };
    window.CodeStudioApp.dispose = dispose;
    window.CodeStudio = window.CodeStudioApp;
})();
