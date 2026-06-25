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
            } else if ((event.ctrlKey || event.metaKey) && key === 'o') {
                event.preventDefault();
                openFileFromDialog();
            } else if ((event.ctrlKey || event.metaKey) && key === 'k' && !event.shiftKey) {
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
            } else if (event.key === '?' && !event.ctrlKey && !event.metaKey) {
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
                { label: tr('codeStudio.gitPanel', 'Toggle Git'), keys: '' },
                { label: tr('codeStudio.commandPalette', 'Command Palette'), keys: 'Ctrl+Shift+P' },
                { label: tr('codeStudio.zenMode', 'Zen Mode'), keys: 'Ctrl+K' }
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
                <button type="button" class="cs-icon-button" data-close-overlay>${esc('\u00d7')}</button>
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

    function exposedOpenFileFromDialog(windowId) {
        return runOnWindow(windowId, openFileFromDialog);
    }

    function exposedUploadFile(windowId) {
        return runOnWindow(windowId, uploadFile);
    }

    function exposedDownloadFile(file, windowId) {
        return runOnWindow(windowId, () => downloadFile(file));
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
        openFileFromDialog: exposedOpenFileFromDialog,
        saveCurrentFile: exposedSaveCurrentFile,
        uploadFile: exposedUploadFile,
        downloadFile: exposedDownloadFile
    };
    window.CodeStudioApp.dispose = dispose;
    window.CodeStudio = window.CodeStudioApp;
})();
