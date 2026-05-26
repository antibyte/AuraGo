    function renderTerminal() {
        const terminal = shellPart('[data-terminal]');
        if (!terminal) return;
        const sessionTabs = (state.terminalSessions || []).map((session, index) => `
            <button type="button" class="cs-terminal-tab${index === (state.activeTerminalSession || 0) ? ' active' : ''}" data-terminal-tab="${index}">
                <span>${esc(session.name || 'Shell ' + (index + 1))}</span>
                <span class="cs-terminal-tab-close" data-terminal-close="${index}">×</span>
            </button>`).join('');
        const activeIdx = state.activeTerminalSession || 0;
        terminal.innerHTML = `<div class="cs-terminal-resize" data-terminal-resize></div>
            <div class="cs-terminal-head">
                <div class="cs-terminal-tabs">
                    ${sessionTabs || `<button type="button" class="cs-terminal-tab active" data-terminal-tab="0"><span>${esc(tr('codeStudio.terminal', 'Terminal'))}</span></button>`}
                    <button type="button" class="cs-terminal-add" data-terminal-add title="${esc(tr('codeStudio.newTerminal', 'New Terminal'))}">+</button>
                </div>
                <span data-terminal-state>${esc(tr('codeStudio.stopped', 'Stopped'))}</span>
            </div><div class="cs-terminal-screen" data-terminal-screen></div>`;
        wireTerminalResize();
        terminal.querySelectorAll('[data-terminal-tab]').forEach(btn => {
            btn.addEventListener('click', bind(() => switchTerminalSession(Number(btn.dataset.terminalTab))));
        });
        terminal.querySelectorAll('[data-terminal-close]').forEach(btn => {
            btn.addEventListener('click', bind(event => {
                event.stopPropagation();
                closeTerminalSession(Number(btn.dataset.terminalClose));
            }));
        });
        const addBtn = terminal.querySelector('[data-terminal-add]');
        if (addBtn) addBtn.addEventListener('click', bind(() => addTerminalSession()));
    }

    function wireTerminalResize() {
        const handle = shellPart('[data-terminal-resize]');
        if (!handle) return;
        let startY = 0;
        let startHeight = 0;
        const onPointerDown = bind(event => {
            event.preventDefault();
            const root = studioRoot();
            if (!root) return;
            startHeight = parseInt(root.style.getPropertyValue('--cs-terminal-height')) || state.terminalHeight || 220;
            startY = event.clientY;
            handle.classList.add('dragging');
            handle.setPointerCapture(event.pointerId);
            handle.addEventListener('pointermove', onPointerMove);
            handle.addEventListener('pointerup', onPointerUp);
            handle.addEventListener('pointercancel', onPointerUp);
        });
        const onPointerMove = bind(event => {
            const delta = startY - event.clientY;
            const newHeight = Math.max(80, Math.min(600, startHeight + delta));
            const root = studioRoot();
            if (root) root.style.setProperty('--cs-terminal-height', newHeight + 'px');
            state.terminalHeight = newHeight;
        });
        const onPointerUp = bind(event => {
            handle.classList.remove('dragging');
            handle.releasePointerCapture(event.pointerId);
            handle.removeEventListener('pointermove', onPointerMove);
            handle.removeEventListener('pointerup', onPointerUp);
            handle.removeEventListener('pointercancel', onPointerUp);
            saveState();
            if (state.fitAddon) setTimeout(bind(() => state.fitAddon.fit()), 50);
        });
        handle.addEventListener('pointerdown', onPointerDown);
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

    function renderStatus(message) {
        const status = shellPart('[data-statusbar]');
        if (!status) return;
        const tab = activeTab();
        const lang = tab ? tab.language || '' : '';
        const lineInfo = tab ? cursorPositionText(tab) : '';
        const leftItems = [];
        if (message) leftItems.push(`<span>${esc(message)}</span>`);
        else leftItems.push(`<span>${esc(state.containerStatus)}</span>`);
        if (tab) leftItems.push(`<span>${tab.modified ? '<span style="color:var(--cs-accent)">●</span> ' : ''}${esc(baseName(tab.path))}</span>`);
        const rightItems = [];
        if (lang) rightItems.push(`<span data-clickable title="${esc(tr('codeStudio.language', 'Language'))}">${esc(lang)}</span>`);
        if (lineInfo) rightItems.push(`<span>${esc(lineInfo)}</span>`);
        rightItems.push(`<span>${esc(state.editorType === 'codemirror' ? 'CodeMirror' : 'Basic')}</span>`);
        status.innerHTML = leftItems.join('') + '<span style="flex:1"></span>' + rightItems.join('');
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
