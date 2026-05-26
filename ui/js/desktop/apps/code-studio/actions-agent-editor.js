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
        renderActivityBar();
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
        renderActivityBar();
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
        state.terminalSessions = [{ name: 'Shell 1', term: null, ws: null }];
        state.activeTerminalSession = 0;
        connectTerminalSession(0, screen, label);
    }

    function connectTerminalSession(index, screen, label) {
        if (!screen) screen = shellPart('[data-terminal-screen]');
        if (!label) label = shellPart('[data-terminal-state]');
        if (!screen || !window.Terminal) return;
        try {
            const term = new window.Terminal({ cursorBlink: true, convertEol: true, fontFamily: "'Cascadia Code', 'JetBrains Mono', 'SF Mono', 'Fira Code', Consolas, monospace", fontSize: 13 });
            const instance = state;
            let terminalDisposed = false;
            instance.disposers.push(() => {
                if (terminalDisposed) return;
                terminalDisposed = true;
                if (term && typeof term.dispose === 'function') term.dispose();
            });
            if (window.FitAddon && window.FitAddon.FitAddon) {
                const fitAddon = new window.FitAddon.FitAddon();
                term.loadAddon(fitAddon);
                if (index === 0) state.fitAddon = fitAddon;
                if (state.terminalSessions[index]) state.terminalSessions[index].fitAddon = fitAddon;
            }
            term.open(screen);
            if (state.terminalSessions[index]) state.terminalSessions[index].term = term;
            if (index === 0) state.terminal = term;
            const fitTarget = state.terminalSessions[index]?.fitAddon || state.fitAddon;
            if (fitTarget) fitTarget.fit();
            term.writeln('Code Studio - Shell ' + (index + 1));
            const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
            const ws = new WebSocket(protocol + '//' + location.host + '/api/code-studio/terminal');
            ws.binaryType = 'arraybuffer';
            if (state.terminalSessions[index]) state.terminalSessions[index].ws = ws;
            if (index === 0) state.ws = ws;
            ws.onopen = bindInstance(instance, () => {
                if (label && index === (state.activeTerminalSession || 0)) label.textContent = tr('codeStudio.running', 'Running...');
                const termDataDispose = term.onData(bindInstance(instance, data => ws.readyState === WebSocket.OPEN && ws.send(data)));
                if (termDataDispose && typeof termDataDispose.dispose === 'function') {
                    instance.disposers.push(() => termDataDispose.dispose());
                }
            });
            ws.onmessage = bindInstance(instance, event => {
                if (event.data instanceof ArrayBuffer) term.write(new Uint8Array(event.data));
                else term.write(String(event.data));
            });
            ws.onerror = bindInstance(instance, () => {
                if (label && index === (state.activeTerminalSession || 0)) label.textContent = tr('codeStudio.terminalUnavailable', 'Terminal unavailable');
            });
            ws.onclose = bindInstance(instance, () => {
                if (label && index === (state.activeTerminalSession || 0)) label.textContent = tr('codeStudio.stopped', 'Stopped');
            });
        } catch (err) {
            screen.textContent = tr('codeStudio.terminalUnavailable', 'Terminal unavailable');
        }
    }

    function switchTerminalSession(index) {
        if (!state.terminalSessions || index < 0 || index >= state.terminalSessions.length) return;
        state.activeTerminalSession = index;
        const screen = shellPart('[data-terminal-screen]');
        if (screen) screen.innerHTML = '';
        const session = state.terminalSessions[index];
        if (session && session.term) {
            const screen = shellPart('[data-terminal-screen]');
            if (screen) session.term.open(screen);
            if (session.fitAddon) session.fitAddon.fit();
            else if (state.fitAddon) state.fitAddon.fit();
        }
        state.terminal = session?.term || null;
        state.ws = session?.ws || null;
        renderTerminal();
    }

    function addTerminalSession() {
        if (!state.terminalSessions) state.terminalSessions = [];
        const index = state.terminalSessions.length;
        state.terminalSessions.push({ name: 'Shell ' + (index + 1), term: null, ws: null });
        state.activeTerminalSession = index;
        renderTerminal();
        const screen = shellPart('[data-terminal-screen]');
        const label = shellPart('[data-terminal-state]');
        if (screen) screen.innerHTML = '';
        connectTerminalSession(index, screen, label);
    }

    function closeTerminalSession(index) {
        if (!state.terminalSessions || index < 0 || index >= state.terminalSessions.length) return;
        const session = state.terminalSessions[index];
        if (session) {
            if (session.ws && session.ws.readyState !== WebSocket.CLOSED) session.ws.close();
            if (session.term && typeof session.term.dispose === 'function') session.term.dispose();
        }
        state.terminalSessions.splice(index, 1);
        if (!state.terminalSessions.length) {
            state.terminalSessions.push({ name: 'Shell 1', term: null, ws: null });
            state.activeTerminalSession = 0;
            renderTerminal();
            const screen = shellPart('[data-terminal-screen]');
            const label = shellPart('[data-terminal-state]');
            if (screen) screen.innerHTML = '';
            connectTerminalSession(0, screen, label);
        } else {
            state.activeTerminalSession = Math.min(index, state.terminalSessions.length - 1);
            switchTerminalSession(state.activeTerminalSession);
        }
    }

    function createCodeMirrorEditor(container, tab) {
        const cm = state.cmModule;
        if (!cm || !cm.EditorState || !cm.EditorView) return createTextareaEditor(container, tab);
        const extensions = [
            cm.lineNumbers && cm.lineNumbers(),
            cm.highlightActiveLineGutter && cm.highlightActiveLineGutter(),
            cm.highlightSpecialChars && cm.highlightSpecialChars(),
            cm.history && cm.history(),
            cm.drawSelection && cm.drawSelection(),
            cm.dropCursor && cm.dropCursor(),
            cm.highlightActiveLine && cm.highlightActiveLine(),
            cm.EditorState.allowMultipleSelections && cm.EditorState.allowMultipleSelections.of(true),
            cm.indentUnit && cm.indentUnit.of('    '),
            cm.EditorView.lineWrapping,
            cm.oneDark,
            cm.closeBrackets && cm.closeBrackets(),
            cm.autocompletion && cm.autocompletion(),
            cm.rectangularSelection && cm.rectangularSelection(),
            cm.crosshairCursor && cm.crosshairCursor(),
            cm.highlightSelectionMatches && cm.highlightSelectionMatches(),
            cm.syntaxHighlighting && cm.defaultHighlightStyle && cm.syntaxHighlighting(cm.defaultHighlightStyle),
            languageExtension(cm, tab.language),
            cm.keymap && cm.keymap.of([
                cm.indentWithTab,
                ...(cm.closeBracketsKeymap || []),
                ...(cm.defaultKeymap || []),
                ...(cm.searchKeymap || []),
                ...(cm.historyKeymap || []),
                ...(cm.completionKeymap || []),
                ...(cm.lintKeymap || []),
                { key: 'Ctrl-s', run: bind(() => { saveCurrentFile(); return true; }) },
                { key: 'F5', run: bind(() => { runCurrentFile(); return true; }) }
            ].filter(Boolean)),
            cm.EditorView.theme({
                '&': {
                    fontSize: 'var(--cs-editor-font-size, 13px)',
                    fontFamily: 'var(--cs-mono-font)'
                },
                '.cm-scroller': {
                    fontFamily: 'var(--cs-mono-font)'
                },
                '.cm-gutters': {
                    background: 'var(--cs-panel)',
                    borderRight: '1px solid var(--cs-border-subtle)'
                },
                '.cm-activeLineGutter': {
                    background: 'var(--cs-accent-soft)'
                },
                '.cm-activeLine': {
                    background: 'rgba(62, 198, 181, 0.04)'
                },
                '.cm-matchingBracket': {
                    background: 'var(--cs-accent-soft)',
                    outline: '1px solid var(--cs-accent-glow)'
                },
                '.cm-selectionBackground': {
                    background: 'rgba(62, 198, 181, 0.18) !important'
                },
                '&.cm-focused .cm-selectionBackground': {
                    background: 'rgba(62, 198, 181, 0.22) !important'
                },
                '.cm-cursor': {
                    borderLeftColor: 'var(--cs-accent)',
                    borderLeftWidth: '2px'
                },
                '.cm-indentGuide': {
                    borderLeft: '1px solid var(--cs-border-subtle)'
                }
            }),
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

    function toggleSidebar() {
        state.sidebarVisible = !state.sidebarVisible;
        ensureShellRoot().dataset.sidebar = state.sidebarVisible ? 'visible' : 'hidden';
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

    function toggleTerminal() {
        state.terminalVisible = !state.terminalVisible;
        ensureShellRoot().dataset.terminal = state.terminalVisible ? 'visible' : 'hidden';
        if (state.fitAddon && state.terminalVisible) setTimeout(bind(() => state.fitAddon.fit()), 50);
        saveState();
        renderActivityBar();
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
            if ((event.ctrlKey || event.metaKey) && event.shiftKey && key === 'p') {
                event.preventDefault();
                if (typeof window.CodeStudioCommandPalette === 'object' && window.CodeStudioCommandPalette.toggle) {
                    window.CodeStudioCommandPalette.toggle();
                }
            } else if ((event.ctrlKey || event.metaKey) && key === 'p') {
                event.preventDefault();
                if (typeof window.CodeStudioCommandPalette === 'object' && window.CodeStudioCommandPalette.toggle) {
                    window.CodeStudioCommandPalette.toggle();
                }
            } else if ((event.ctrlKey || event.metaKey) && key === 's') {
                event.preventDefault();
                saveCurrentFile();
            } else if ((event.ctrlKey || event.metaKey) && event.shiftKey && key === 'f') {
                event.preventDefault();
                if (!state.searchVisible) state.searchVisible = true;
                renderSearchPanel();
                renderActivityBar();
            } else if ((event.ctrlKey || event.metaKey) && event.shiftKey && key === 'a') {
                event.preventDefault();
                if (!state.agentVisible) toggleAgentPanel();
            } else if ((event.ctrlKey || event.metaKey) && key === 'b') {
                event.preventDefault();
                toggleSidebar();
            } else if ((event.ctrlKey || event.metaKey) && key === 'n') {
                event.preventDefault();
                createNewFile();
            } else if ((event.ctrlKey || event.metaKey) && key === 'k' && event.shiftKey && key === 'z') {
                event.preventDefault();
                toggleZenMode();
            } else if (event.key === 'F5') {
                event.preventDefault();
                runCurrentFile();
            } else if (event.key === 'Escape') {
                if (state.zenMode) {
                    event.preventDefault();
                    toggleZenMode();
                }
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

    function showShortcutOverlay() {
        const existing = document.querySelector('.cs-shortcut-overlay');
        if (existing) { existing.remove(); return; }
        const overlay = document.createElement('div');
        overlay.className = 'cs-shortcut-overlay';
        const sections = [
            { title: tr('codeStudio.shortcutsFile', 'File'), items: [
                { label: tr('codeStudio.newFile', 'New File'), keys: 'Ctrl+N' },
                { label: tr('codeStudio.save', 'Save'), keys: 'Ctrl+S' },
                { label: tr('codeStudio.upload', 'Upload'), keys: '' }
            ]},
            { title: tr('codeStudio.shortcutsEditor', 'Editor'), items: [
                { label: tr('codeStudio.search', 'Search in Files'), keys: 'Ctrl+Shift+F' },
                { label: tr('codeStudio.run', 'Run'), keys: 'F5' },
                { label: tr('codeStudio.zoomIn', 'Zoom In'), keys: 'Ctrl+=' },
                { label: tr('codeStudio.zoomOut', 'Zoom Out'), keys: 'Ctrl+-' },
                { label: tr('codeStudio.zoomReset', 'Reset Zoom'), keys: 'Ctrl+0' }
            ]},
            { title: tr('codeStudio.shortcutsView', 'View'), items: [
                { label: tr('codeStudio.sidebar', 'Toggle Sidebar'), keys: 'Ctrl+B' },
                { label: tr('codeStudio.agentChat', 'Toggle Agent'), keys: 'Ctrl+Shift+A' },
                { label: tr('codeStudio.commandPalette', 'Command Palette'), keys: 'Ctrl+Shift+P' },
                { label: tr('codeStudio.zenMode', 'Zen Mode'), keys: 'Ctrl+K Z' }
            ]}
        ];
        const bodyHtml = sections.map(section => `
            <div class="cs-shortcut-section">
                <h4>${esc(section.title)}</h4>
                ${section.items.map(item => `
                    <div class="cs-shortcut-row">
                        <span>${esc(item.label)}</span>
                        ${item.keys ? `<kbd>${esc(item.keys)}</kbd>` : ''}
                    </div>`).join('')}
            </div>`).join('');
        overlay.innerHTML = `<div class="cs-shortcut-modal">
            <div class="cs-shortcut-modal-head">
                <h3>${esc(tr('codeStudio.keyboardShortcuts', 'Keyboard Shortcuts'))}</h3>
                <button type="button" class="cs-icon-button" data-close-overlay>${esc('×')}</button>
            </div>
            <div class="cs-shortcut-modal-body">${bodyHtml}</div>
        </div>`;
        document.body.appendChild(overlay);
        overlay.querySelector('[data-close-overlay]').addEventListener('click', () => overlay.remove());
        overlay.addEventListener('mousedown', event => { if (event.target === overlay) overlay.remove(); });
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
