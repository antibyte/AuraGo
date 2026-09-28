    function isEditableTarget(element) {
        if (!element || element === document.body) return false;
        const tag = String(element.tagName || '').toLowerCase();
        return tag === 'input' || tag === 'textarea' || tag === 'select' || element.isContentEditable === true;
    }

    function openCommandPalette() {
        if (window.CodeStudioCommandPalette && typeof window.CodeStudioCommandPalette.toggle === 'function') {
            window.CodeStudioCommandPalette.toggle();
        }
    }

    function wireShortcuts() {
        if (state.shortcutsWired) return;
        state.shortcutsWired = true;
        const instance = state;
        const onKeydown = bindInstance(instance, event => {
            if (!state.root || !studioRoot()) return;
            const activeElement = document.activeElement;
            if (activeElement && !state.root.contains(activeElement)) return;
            const key = String(event.key || '').toLowerCase();
            const mod = event.ctrlKey || event.metaKey;
            const editing = isEditableTarget(event.target) || isEditableTarget(activeElement);
            if (mod && event.shiftKey && key === 'p') {
                event.preventDefault();
                openCommandPalette();
            } else if (mod && !event.shiftKey && key === 'p') {
                event.preventDefault();
                openCommandPalette();
            } else if (mod && !event.shiftKey && key === 's') {
                event.preventDefault();
                saveCurrentFile();
            } else if (mod && event.shiftKey && key === 'f') {
                event.preventDefault();
                if (!state.searchVisible) toggleSearch();
                else {
                    const input = shellPart('[data-search] input[name="q"]');
                    if (input) { input.focus(); input.select(); }
                }
            } else if (mod && event.shiftKey && key === 'a') {
                event.preventDefault();
                if (!state.agentVisible) toggleAgentPanel();
            } else if (mod && key === 'b') {
                event.preventDefault();
                toggleSidebar();
            } else if (mod && key === 'n') {
                event.preventDefault();
                createNewFile();
            } else if (mod && key === 'o') {
                event.preventDefault();
                openFileFromDialog();
            } else if (mod && key === 'k' && !event.shiftKey && !event.altKey) {
                event.preventDefault();
                toggleZenMode();
            } else if (mod && (key === '=' || key === '+')) {
                event.preventDefault();
                adjustEditorZoom(1);
            } else if (mod && (key === '-' || key === '_')) {
                event.preventDefault();
                adjustEditorZoom(-1);
            } else if (mod && key === '0') {
                event.preventDefault();
                resetEditorZoom();
            } else if (event.key === 'F5') {
                event.preventDefault();
                runCurrentFile();
            } else if (event.key === 'Escape') {
                if (state.zenMode) {
                    event.preventDefault();
                    toggleZenMode();
                } else if (state.searchVisible && activeElement && activeElement.closest('[data-search]')) {
                    event.preventDefault();
                    toggleSearch();
                }
            } else if (event.key === '?' && !mod && !event.altKey && !editing) {
                event.preventDefault();
                showShortcutOverlay();
            }
        });
        document.addEventListener('keydown', onKeydown);
        state.disposers.push(() => { document.removeEventListener('keydown', onKeydown); });
    }

    function showShortcutOverlay() {
        const existing = document.querySelector('.cs-shortcut-overlay');
        if (existing) { existing.remove(); return; }
        const overlay = document.createElement('div');
        overlay.className = 'cs-shortcut-overlay';
        const sections = [
            { title: tr('codeStudio.shortcutsFile', 'File'), items: [
                { label: tr('codeStudio.newFile', 'New File'), keys: 'Ctrl+N' },
                { label: tr('desktop.file_dialog_open', 'Open'), keys: 'Ctrl+O' },
                { label: tr('codeStudio.save', 'Save'), keys: 'Ctrl+S' },
                { label: tr('codeStudio.run', 'Run'), keys: 'F5' }
            ]},
            { title: tr('codeStudio.shortcutsEditor', 'Editor'), items: [
                { label: tr('codeStudio.quickOpen', 'Quick open'), keys: 'Ctrl+P' },
                { label: tr('codeStudio.commandPalette', 'Command Palette'), keys: 'Ctrl+Shift+P' },
                { label: tr('codeStudio.searchFiles', 'Search in Files'), keys: 'Ctrl+Shift+F' },
                { label: tr('codeStudio.zoomIn', 'Zoom In'), keys: 'Ctrl+=' },
                { label: tr('codeStudio.zoomOut', 'Zoom Out'), keys: 'Ctrl+-' },
                { label: tr('codeStudio.zoomReset', 'Reset Zoom'), keys: 'Ctrl+0' }
            ]},
            { title: tr('codeStudio.shortcutsView', 'View'), items: [
                { label: tr('codeStudio.sidebar', 'Sidebar'), keys: 'Ctrl+B' },
                { label: tr('codeStudio.agentChat', 'Agent Chat'), keys: 'Ctrl+Shift+A' },
                { label: tr('codeStudio.zenMode', 'Zen Mode'), keys: 'Ctrl+K' },
                { label: tr('codeStudio.shortcutsHelp', 'Show keyboard shortcuts'), keys: '?' }
            ]},
            { title: tr('codeStudio.shortcutsExplorer', 'Explorer'), items: [
                { label: tr('codeStudio.treeNavigate', 'Navigate with arrow keys'), keys: '↑ ↓ ← →' },
                { label: tr('codeStudio.rename', 'Rename'), keys: 'F2' },
                { label: tr('desktop.delete', 'Delete'), keys: 'Del' }
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
        overlay.innerHTML = `<div class="cs-shortcut-modal" role="dialog" aria-modal="true">
            <div class="cs-shortcut-modal-head">
                <h3>${esc(tr('codeStudio.keyboardShortcuts', 'Keyboard Shortcuts'))}</h3>
                <button type="button" class="cs-icon-button" data-close-overlay title="${esc(tr('desktop.close', 'Close'))}">${esc('×')}</button>
            </div>
            <div class="cs-shortcut-modal-body">${bodyHtml}</div>
        </div>`;
        document.body.appendChild(overlay);
        const close = () => {
            document.removeEventListener('keydown', onKey);
            overlay.remove();
        };
        const onKey = event => {
            if (event.key === 'Escape') {
                event.preventDefault();
                close();
            }
        };
        document.addEventListener('keydown', onKey);
        overlay.querySelector('[data-close-overlay]').addEventListener('click', close);
        overlay.addEventListener('mousedown', event => { if (event.target === overlay) close(); });
        overlay.querySelector('[data-close-overlay]').focus();
    }

    function studioCommandTable() {
        return {
            toggleZenMode,
            adjustEditorZoom,
            resetEditorZoom,
            saveCurrentFile,
            saveAllFiles,
            closeTab,
            closeOtherTabs,
            closeAllTabs,
            toggleSidebar,
            toggleTerminal,
            toggleAgentPanel,
            toggleGitPanel,
            toggleSearch,
            createNewFile,
            createNewFolder,
            runCurrentFile,
            refreshFiles: () => refreshFiles(state.currentPath),
            uploadFile,
            openFileFromDialog,
            openFile,
            splitEditor,
            showShortcutOverlay,
            collapseAllDirectories
        };
    }

    function exposedCommand(name, args, windowId) {
        const table = studioCommandTable();
        const fn = table[name];
        if (typeof fn !== 'function') return undefined;
        return runOnWindow(windowId, () => fn(...(Array.isArray(args) ? args : [])));
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

    function exposedOpenFileFromDialog(windowId) {
        return runOnWindow(windowId, openFileFromDialog);
    }

    function exposedUploadFile(windowId) {
        return runOnWindow(windowId, uploadFile);
    }

    function exposedDownloadFile(file, windowId) {
        return runOnWindow(windowId, () => downloadFile(file));
    }

    function exposedKnownFiles(windowId) {
        return runOnWindow(windowId, () => {
            const seen = new Set();
            const files = [];
            const push = entry => {
                if (!entry || entry.type !== 'file' || seen.has(entry.path)) return;
                seen.add(entry.path);
                files.push({ path: entry.path, name: entry.name });
            };
            (state.files || []).forEach(push);
            Object.values(state.treeCache || {}).forEach(list => (list || []).forEach(push));
            return files;
        }) || [];
    }

    window.CodeStudioApp = {
        render,
        dispose,
        get state() { return currentInstance(); },
        instances,
        api: apiClient,
        command: exposedCommand,
        knownFiles: exposedKnownFiles,
        loadState: exposedLoadState,
        saveState: exposedSaveState,
        refreshFiles: exposedRefreshFiles,
        openFile: exposedOpenFile,
        openFileFromDialog: exposedOpenFileFromDialog,
        saveCurrentFile: exposedSaveCurrentFile,
        uploadFile: exposedUploadFile,
        downloadFile: exposedDownloadFile
    };
    window.CodeStudioApp.dispose = dispose;
    window.CodeStudio = window.CodeStudioApp;
})();
