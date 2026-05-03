(function () {
    const STATE_KEY = 'aurago.codeStudio.state.v1';
    const WORKSPACE_ROOT = '/workspace';

    const state = {
        root: null,
        windowId: '',
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
        sidebarWidth: 280
    };

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

    async function api(path, options) {
        const response = await fetch(path, Object.assign({
            credentials: 'same-origin',
            headers: { 'Content-Type': 'application/json' }
        }, options || {}));
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
        createDirectory: path => api('/api/code-studio/directory', {
            method: 'POST',
            body: JSON.stringify({ path })
        }),
        exec: command => api('/api/code-studio/exec', {
            method: 'POST',
            body: JSON.stringify({ command, cwd: currentDirectory(), timeout_seconds: 300 })
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
        state.root = container;
        state.windowId = windowId;
        loadState();
        container.innerHTML = shellMarkup();
        renderLoading(tr('codeStudio.starting', 'Starting container...'));
        try {
            await prepareContainer();
            await loadEditorModule();
            renderShell();
            await refreshFiles(context && context.path ? context.path : state.currentPath);
            await restoreTabs();
            connectTerminal();
        } catch (err) {
            renderError(err.message || String(err));
        }
    }

    async function prepareContainer() {
        const status = await apiClient.status();
        const payload = status.code_studio || {};
        if (!payload.enabled) throw new Error(tr('codeStudio.dockerUnavailable', 'Docker is not available. Code Studio requires Docker.'));
        state.containerStatus = payload.running ? 'running' : 'starting';
        if (!payload.running) {
            await apiClient.files(WORKSPACE_ROOT);
            state.containerStatus = 'running';
        }
    }

    async function loadEditorModule() {
        try {
            state.cmModule = await import('/js/vendor/codemirror-bundle.esm.js');
            state.editorType = 'codemirror';
        } catch (err) {
            console.warn('CodeMirror ESM failed, using textarea fallback', err);
            state.cmModule = null;
            state.editorType = 'textarea';
        }
    }

    function shellMarkup() {
        return `<div class="code-studio" data-code-studio>
            <div class="code-studio-toolbar" data-toolbar></div>
            <div class="code-studio-body">
                <aside class="code-studio-sidebar" data-sidebar></aside>
                <main class="code-studio-main">
                    <div class="code-studio-tabs" data-tabs></div>
                    <div class="code-studio-editor" data-editor></div>
                    <div class="code-studio-terminal" data-terminal></div>
                </main>
            </div>
            <div class="code-studio-statusbar" data-statusbar></div>
        </div>`;
    }

    function renderShell() {
        const root = studioRoot();
        root.style.setProperty('--cs-sidebar-width', Math.max(220, state.sidebarWidth) + 'px');
        root.style.setProperty('--cs-terminal-height', Math.max(120, state.terminalHeight) + 'px');
        root.dataset.terminal = state.terminalVisible ? 'visible' : 'hidden';
        root.dataset.sidebar = state.sidebarVisible ? 'visible' : 'hidden';
        renderToolbar();
        renderSidebar();
        renderTabs();
        renderEditor();
        renderTerminal();
        renderStatus();
    }

    function renderToolbar() {
        const toolbar = state.root.querySelector('[data-toolbar]');
        toolbar.innerHTML = `
            <button type="button" class="cs-button" data-action="new-file">${esc(tr('codeStudio.newFile', 'New File'))}</button>
            <button type="button" class="cs-button" data-action="new-folder">${esc(tr('codeStudio.newFolder', 'New Folder'))}</button>
            <button type="button" class="cs-button primary" data-action="save">${esc(tr('codeStudio.save', 'Save'))}</button>
            <button type="button" class="cs-button" data-action="run">${esc(tr('codeStudio.run', 'Run'))}</button>
            <button type="button" class="cs-icon-button" data-action="refresh" title="${esc(tr('codeStudio.refresh', 'Refresh'))}">↻</button>
            <button type="button" class="cs-icon-button" data-action="terminal" title="${esc(tr('codeStudio.toggleTerminal', 'Toggle Terminal'))}">▣</button>
            <span class="cs-toolbar-spacer"></span>
            <span class="cs-pill">${esc(state.editorType === 'codemirror' ? 'CodeMirror' : tr('codeStudio.editorFallback', 'Basic editor'))}</span>`;
        toolbar.querySelector('[data-action="new-file"]').addEventListener('click', createNewFile);
        toolbar.querySelector('[data-action="new-folder"]').addEventListener('click', createNewFolder);
        toolbar.querySelector('[data-action="save"]').addEventListener('click', saveCurrentFile);
        toolbar.querySelector('[data-action="run"]').addEventListener('click', runCurrentFile);
        toolbar.querySelector('[data-action="refresh"]').addEventListener('click', () => refreshFiles(state.currentPath));
        toolbar.querySelector('[data-action="terminal"]').addEventListener('click', toggleTerminal);
    }

    function renderSidebar(errorMessage) {
        const sidebar = state.root.querySelector('[data-sidebar]');
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
        sidebar.querySelectorAll('[data-file-path]').forEach(row => {
            row.addEventListener('click', () => {
                const file = state.files.find(item => item.path === row.dataset.filePath);
                if (!file) return;
                if (file.type === 'directory') refreshFiles(file.path);
                else openFile(file.path);
            });
        });
    }

    function fileRow(file) {
        const icon = file.type === 'directory' ? '▸' : fileIcon(file.name);
        return `<button type="button" class="cs-file-row" data-file-path="${esc(file.path)}" data-type="${esc(file.type)}">
            <span class="cs-file-icon">${esc(icon)}</span>
            <span class="cs-file-name">${esc(file.name)}</span>
            <span class="cs-file-meta">${file.type === 'directory' ? '' : esc(formatBytes(file.size))}</span>
        </button>`;
    }

    function renderTabs() {
        const tabs = state.root.querySelector('[data-tabs]');
        tabs.innerHTML = state.openTabs.length ? state.openTabs.map((tab, index) => `
            <button type="button" class="cs-tab ${index === state.activeTabIndex ? 'active' : ''}" data-tab="${index}">
                <span>${esc(baseName(tab.path))}${tab.modified ? ' *' : ''}</span>
                <span class="cs-tab-close" data-close="${index}" title="${esc(tr('codeStudio.closeTab', 'Close tab'))}">×</span>
            </button>`).join('') : `<div class="cs-tabs-empty">${esc(tr('codeStudio.noFiles', 'No files open'))}</div>`;
        tabs.querySelectorAll('[data-tab]').forEach(btn => btn.addEventListener('click', event => {
            if (event.target.closest('[data-close]')) return;
            activateTab(Number(btn.dataset.tab));
        }));
        tabs.querySelectorAll('[data-close]').forEach(btn => btn.addEventListener('click', event => {
            event.stopPropagation();
            closeTab(Number(btn.dataset.close));
        }));
    }

    function renderEditor() {
        const editor = state.root.querySelector('[data-editor]');
        const tab = activeTab();
        if (!tab) {
            editor.innerHTML = `<div class="cs-editor-empty">${esc(tr('codeStudio.noFiles', 'No files open'))}</div>`;
            return;
        }
        editor.innerHTML = '';
        tab.view = state.editorType === 'codemirror'
            ? createCodeMirrorEditor(editor, tab)
            : createTextareaEditor(editor, tab);
    }

    function renderTerminal() {
        const terminal = state.root.querySelector('[data-terminal]');
        terminal.innerHTML = `<div class="cs-terminal-head">
            <strong>${esc(tr('codeStudio.terminal', 'Terminal'))}</strong>
            <span data-terminal-state>${esc(tr('codeStudio.stopped', 'Stopped'))}</span>
        </div><div class="cs-terminal-screen" data-terminal-screen></div>`;
    }

    function renderStatus(message) {
        const status = state.root.querySelector('[data-statusbar]');
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
        state.root.querySelector('[data-retry]').addEventListener('click', () => render(state.root, state.windowId, {}));
    }

    async function refreshFiles(path) {
        state.currentPath = path || WORKSPACE_ROOT;
        try {
            const result = await apiClient.files(state.currentPath);
            state.files = result.files || [];
            renderSidebar();
            renderStatus();
        } catch (err) {
            renderSidebar(err.message || String(err));
        }
    }

    async function restoreTabs() {
        const savedPaths = state.openTabs.map(tab => tab.path);
        const desiredActive = state.activeTabIndex;
        state.openTabs = [];
        for (const path of savedPaths) {
            try {
                await openFile(path, false);
            } catch (err) {
                console.warn('Failed to restore Code Studio tab', path, err);
            }
        }
        if (state.openTabs.length) {
            activateTab(Math.min(Math.max(desiredActive, 0), state.openTabs.length - 1));
        } else {
            renderTabs();
            renderEditor();
        }
    }

    async function openFile(path, persist) {
        const existing = state.openTabs.findIndex(tab => tab.path === path);
        if (existing >= 0) {
            activateTab(existing);
            return;
        }
        renderStatus(tr('codeStudio.editorLoading', 'Loading editor...'));
        const result = await apiClient.file(path);
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
        if (tab.view && typeof tab.view.destroy === 'function') tab.view.destroy();
        state.openTabs.splice(index, 1);
        if (state.activeTabIndex >= state.openTabs.length) state.activeTabIndex = state.openTabs.length - 1;
        renderTabs();
        renderEditor();
        renderStatus();
        saveState();
    }

    async function saveCurrentFile() {
        const tab = activeTab();
        if (!tab) return false;
        tab.content = editorValue(tab);
        await apiClient.writeFile(tab.path, tab.content);
        tab.modified = false;
        renderTabs();
        renderStatus(tr('codeStudio.save', 'Save'));
        saveState();
        return true;
    }

    async function createNewFile() {
        const name = await promptValue(tr('codeStudio.newFile', 'New File'), 'main.go');
        if (!name) return;
        const path = joinPath(state.currentPath, name);
        await apiClient.writeFile(path, '');
        await refreshFiles(state.currentPath);
        await openFile(path);
    }

    async function createNewFolder() {
        const name = await promptValue(tr('codeStudio.newFolder', 'New Folder'), 'src');
        if (!name) return;
        await apiClient.createDirectory(joinPath(state.currentPath, name));
        await refreshFiles(state.currentPath);
    }

    async function runCurrentFile() {
        const tab = activeTab();
        if (!tab) return;
        if (tab.modified) await saveCurrentFile();
        const command = runCommandFor(tab.path);
        renderStatus(tr('codeStudio.running', 'Running...'));
        writeTerminalLine('$ ' + command);
        try {
            const result = await apiClient.exec(command);
            writeTerminalLine(result.output || '');
            writeTerminalLine('exit ' + result.exit_code);
            renderStatus(tr('codeStudio.stopped', 'Stopped'));
        } catch (err) {
            writeTerminalLine(err.message || String(err));
            renderStatus(tr('codeStudio.containerError', 'Container error: {error}', { error: err.message || String(err) }));
        }
    }

    function connectTerminal() {
        const screen = state.root.querySelector('[data-terminal-screen]');
        const label = state.root.querySelector('[data-terminal-state]');
        if (!screen || !window.Terminal) {
            if (screen) screen.textContent = tr('codeStudio.terminalUnavailable', 'Terminal unavailable');
            return;
        }
        try {
            const term = new window.Terminal({ cursorBlink: true, convertEol: true, fontFamily: 'Consolas, monospace', fontSize: 13 });
            state.terminal = term;
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
            ws.onopen = () => {
                label.textContent = tr('codeStudio.running', 'Running...');
                term.onData(data => ws.readyState === WebSocket.OPEN && ws.send(data));
            };
            ws.onmessage = event => {
                if (event.data instanceof ArrayBuffer) term.write(new Uint8Array(event.data));
                else term.write(String(event.data));
            };
            ws.onerror = () => label.textContent = tr('codeStudio.terminalUnavailable', 'Terminal unavailable');
            ws.onclose = () => label.textContent = tr('codeStudio.stopped', 'Stopped');
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
            cm.oneDark,
            cm.closeBrackets && cm.closeBrackets(),
            cm.autocompletion && cm.autocompletion(),
            cm.syntaxHighlighting && cm.defaultHighlightStyle && cm.syntaxHighlighting(cm.defaultHighlightStyle),
            languageExtension(cm, tab.language),
            cm.keymap && cm.keymap.of([
                cm.indentWithTab,
                { key: 'Ctrl-s', run: () => { saveCurrentFile(); return true; } }
            ].filter(Boolean)),
            cm.EditorView.updateListener.of(update => {
                if (!update.docChanged) return;
                tab.modified = true;
                tab.content = update.state.doc.toString();
                renderTabs();
                renderStatus();
            })
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
        const updatePreview = () => {
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
        };
        textarea.addEventListener('input', updatePreview);
        textarea.addEventListener('keydown', event => {
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
        });
        updatePreview();
        tab.modified = false;
        return { getValue: () => textarea.value, setValue: value => { textarea.value = value; updatePreview(); } };
    }

    function editorValue(tab) {
        if (!tab) return '';
        if (tab.view && tab.view.state && tab.view.state.doc) return tab.view.state.doc.toString();
        if (tab.view && typeof tab.view.getValue === 'function') return tab.view.getValue();
        return tab.content || '';
    }

    function activeTab() {
        return state.openTabs[state.activeTabIndex] || null;
    }

    function currentDirectory() {
        const tab = activeTab();
        if (tab && tab.path) return tab.path.slice(0, Math.max(WORKSPACE_ROOT.length, tab.path.lastIndexOf('/')));
        return state.currentPath || WORKSPACE_ROOT;
    }

    function studioRoot() {
        return state.root.querySelector('[data-code-studio]');
    }

    function toggleTerminal() {
        state.terminalVisible = !state.terminalVisible;
        studioRoot().dataset.terminal = state.terminalVisible ? 'visible' : 'hidden';
        if (state.fitAddon && state.terminalVisible) setTimeout(() => state.fitAddon.fit(), 50);
        saveState();
    }

    function writeTerminalLine(line) {
        if (state.terminal) {
            String(line || '').split('\n').forEach(part => state.terminal.writeln(part));
            return;
        }
        const screen = state.root.querySelector('[data-terminal-screen]');
        if (screen) screen.textContent += String(line || '') + '\n';
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
            const overlay = document.createElement('div');
            overlay.className = 'cs-modal-backdrop';
            overlay.innerHTML = `<form class="cs-modal">
                <label>${esc(title)}<input name="value" value="${esc(value || '')}" autocomplete="off" spellcheck="false"></label>
                <div class="cs-modal-actions">
                    <button type="button" class="cs-button" data-cancel>${esc(tr('desktop.cancel', 'Cancel'))}</button>
                    <button type="submit" class="cs-button primary">${esc(tr('desktop.ok', 'OK'))}</button>
                </div>
            </form>`;
            document.body.appendChild(overlay);
            const input = overlay.querySelector('input');
            const cleanup = result => {
                overlay.remove();
                resolve(result);
            };
            overlay.querySelector('form').addEventListener('submit', event => {
                event.preventDefault();
                cleanup(input.value.trim());
            });
            overlay.querySelector('[data-cancel]').addEventListener('click', () => cleanup(''));
            overlay.addEventListener('click', event => {
                if (event.target === overlay) cleanup('');
            });
            input.focus();
            input.select();
        });
    }

    window.CodeStudio = {
        render,
        state,
        api: apiClient,
        loadState,
        saveState,
        refreshFiles,
        openFile,
        saveCurrentFile
    };
})();
