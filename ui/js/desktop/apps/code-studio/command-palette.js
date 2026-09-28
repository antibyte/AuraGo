(function () {
    'use strict';

    let backdrop = null;
    let selectedIndex = 0;
    let filteredItems = [];
    let lastQuery = '';

    function esc(value) {
        return String(value == null ? '' : value)
            .replaceAll('&', '&amp;')
            .replaceAll('<', '&lt;')
            .replaceAll('>', '&gt;')
            .replaceAll('"', '&quot;')
            .replaceAll("'", '&#39;');
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

    function getApp() {
        return window.CodeStudioApp || window.CodeStudio || null;
    }

    function getState() {
        const app = getApp();
        return app && typeof app.state === 'object' ? app.state : null;
    }

    function iconMarkup(key, fallback) {
        const state = getState();
        if (state && typeof state.iconMarkup === 'function') return state.iconMarkup(key, fallback, 'cs-cp-papirus-icon', 16);
        return esc(fallback || '');
    }

    function fuzzyMatch(query, text) {
        const q = query.toLowerCase();
        const t = text.toLowerCase();
        if (t.includes(q)) return { match: true, score: t.indexOf(q) === 0 ? 2 : 1 };
        let qi = 0;
        for (let ti = 0; ti < t.length && qi < q.length; ti++) {
            if (t[ti] === q[qi]) qi++;
        }
        return { match: qi === q.length, score: qi === q.length ? 0.5 : 0 };
    }

    function highlightMatch(text, query) {
        if (!query) return esc(text);
        const q = query.toLowerCase();
        const t = text;
        const tl = t.toLowerCase();
        const idx = tl.indexOf(q);
        if (idx >= 0) {
            return esc(t.slice(0, idx)) + '<mark>' + esc(t.slice(idx, idx + q.length)) + '</mark>' + esc(t.slice(idx + q.length));
        }
        let result = '';
        let qi = 0;
        for (let i = 0; i < t.length; i++) {
            if (qi < q.length && t[i].toLowerCase() === q[qi]) {
                result += '<mark>' + esc(t[i]) + '</mark>';
                qi++;
            } else {
                result += esc(t[i]);
            }
        }
        return result;
    }

    function getCommands() {
        const state = getState();
        if (!state) return [];
        const hasTab = !!(state.openTabs && state.openTabs.length);
        const modified = !!(state.openTabs || []).some(tab => tab.modified);
        return [
            { id: 'new-file', label: tr('codeStudio.newFile', 'New File'), shortcut: 'Ctrl+N', icon: 'file-plus', run: 'createNewFile' },
            { id: 'new-folder', label: tr('codeStudio.newFolder', 'New Folder'), icon: 'folder-plus', run: 'createNewFolder' },
            { id: 'open-file', label: tr('desktop.file_dialog_open', 'Open'), shortcut: 'Ctrl+O', icon: 'folder-open', run: 'openFileFromDialog' },
            { id: 'save', label: tr('codeStudio.save', 'Save'), shortcut: 'Ctrl+S', icon: 'save', run: 'saveCurrentFile', disabled: !hasTab },
            { id: 'save-all', label: tr('codeStudio.saveAll', 'Save All'), icon: 'save', run: 'saveAllFiles', disabled: !modified },
            { id: 'close-tab', label: tr('codeStudio.closeTab', 'Close tab'), icon: 'x', run: 'closeTab', args: [state.activeTabIndex], disabled: !hasTab },
            { id: 'close-others', label: tr('codeStudio.closeOthers', 'Close others'), icon: 'x', run: 'closeOtherTabs', args: [state.activeTabIndex], disabled: !(state.openTabs && state.openTabs.length > 1) },
            { id: 'close-all', label: tr('codeStudio.closeAll', 'Close all'), icon: 'x', run: 'closeAllTabs', disabled: !hasTab },
            { id: 'run', label: tr('codeStudio.run', 'Run'), shortcut: 'F5', icon: 'run', run: 'runCurrentFile', disabled: !hasTab },
            { id: 'upload', label: tr('codeStudio.upload', 'Upload'), icon: 'upload', run: 'uploadFile' },
            { id: 'refresh', label: tr('codeStudio.refresh', 'Refresh'), icon: 'refresh', run: 'refreshFiles' },
            { id: 'collapse-all', label: tr('codeStudio.collapseAll', 'Collapse all'), icon: 'chevron-up', run: 'collapseAllDirectories' },
            { id: 'toggle-sidebar', label: tr('codeStudio.sidebar', 'Sidebar'), shortcut: 'Ctrl+B', icon: 'sidebar', run: 'toggleSidebar' },
            { id: 'toggle-terminal', label: tr('codeStudio.toggleTerminal', 'Toggle Terminal'), icon: 'terminal', run: 'toggleTerminal' },
            { id: 'toggle-agent', label: tr('codeStudio.agentChat', 'Agent Chat'), shortcut: 'Ctrl+Shift+A', icon: 'chat', run: 'toggleAgentPanel' },
            { id: 'toggle-git', label: tr('codeStudio.gitPanel', 'Source Control'), icon: 'git', run: 'toggleGitPanel' },
            { id: 'toggle-search', label: tr('codeStudio.searchFiles', 'Search in Files'), shortcut: 'Ctrl+Shift+F', icon: 'search', run: 'toggleSearch' },
            { id: 'split-right', label: tr('codeStudio.splitRight', 'Split Right'), icon: 'columns', run: 'splitEditor', args: ['right'], disabled: !hasTab },
            { id: 'split-down', label: tr('codeStudio.splitDown', 'Split Down'), icon: 'layout', run: 'splitEditor', args: ['down'], disabled: !hasTab },
            { id: 'toggle-zen', label: tr('codeStudio.zenMode', 'Toggle Zen Mode'), shortcut: 'Ctrl+K', icon: 'maximize', run: 'toggleZenMode' },
            { id: 'zoom-in', label: tr('codeStudio.zoomIn', 'Zoom In'), shortcut: 'Ctrl+=', icon: 'zoom-in', run: 'adjustEditorZoom', args: [1] },
            { id: 'zoom-out', label: tr('codeStudio.zoomOut', 'Zoom Out'), shortcut: 'Ctrl+-', icon: 'zoom-out', run: 'adjustEditorZoom', args: [-1] },
            { id: 'zoom-reset', label: tr('codeStudio.zoomReset', 'Reset Zoom'), shortcut: 'Ctrl+0', icon: 'zoom-reset', run: 'resetEditorZoom' },
            { id: 'shortcuts', label: tr('codeStudio.keyboardShortcuts', 'Keyboard Shortcuts'), shortcut: '?', icon: 'help', run: 'showShortcutOverlay' }
        ].filter(cmd => !cmd.disabled);
    }

    function fileLabel(path) {
        return String(path || '').split('/').filter(Boolean).pop() || path;
    }

    function getFileItems() {
        const state = getState();
        if (!state) return [];
        const app = getApp();
        const seen = new Set();
        const items = [];
        const push = (path, type, icon) => {
            if (!path || seen.has(path)) return;
            seen.add(path);
            items.push({ id: type + ':' + path, label: fileLabel(path), path, icon, type });
        };
        (state.openTabs || []).forEach(tab => push(tab.path, 'file', 'file'));
        (state.recentFiles || []).slice(0, 8).forEach(path => push(path, 'recent', 'clock'));
        const known = app && typeof app.knownFiles === 'function' ? app.knownFiles() : [];
        known.forEach(entry => push(entry.path, 'file', 'file'));
        return items;
    }

    function getAllItems() {
        const commands = getCommands().map(cmd => ({ ...cmd, type: 'command' }));
        return [...commands, ...getFileItems()];
    }

    function executeItem(item) {
        const app = getApp();
        if (!app || !item) return;
        if (item.type === 'file' || item.type === 'recent') {
            if (typeof app.command === 'function') app.command('openFile', [item.path]);
            else if (typeof app.openFile === 'function') app.openFile(item.path);
            return;
        }
        if (item.run && typeof app.command === 'function') {
            app.command(item.run, item.args || []);
        }
    }

    function renderPalette() {
        if (backdrop) return;
        backdrop = document.createElement('div');
        backdrop.className = 'cs-command-palette-backdrop';
        backdrop.innerHTML = `<div class="cs-command-palette" role="dialog" aria-modal="true" aria-label="${esc(tr('codeStudio.commandPalette', 'Command Palette'))}">
            <div class="cs-command-palette-input">
                <span class="cs-cp-icon">${iconMarkup('search', '>')}</span>
                <input type="text" placeholder="${esc(tr('codeStudio.cpPlaceholder', 'Search files, commands, tabs...'))}" autocomplete="off" spellcheck="false" inputmode="search" enterkeyhint="search" autocapitalize="off">
                <kbd class="cs-cp-hint">Esc</kbd>
            </div>
            <div class="cs-command-palette-results" data-cp-results role="listbox"></div>
        </div>`;
        document.body.appendChild(backdrop);

        const input = backdrop.querySelector('input');
        const resultsEl = backdrop.querySelector('[data-cp-results]');

        selectedIndex = 0;
        lastQuery = '';
        filteredItems = getAllItems();
        renderResults(resultsEl, '');

        input.addEventListener('input', () => {
            lastQuery = input.value.trim();
            filteredItems = filterItems(lastQuery);
            selectedIndex = 0;
            renderResults(resultsEl, lastQuery);
        });

        input.addEventListener('keydown', event => {
            if (event.key === 'ArrowDown') {
                event.preventDefault();
                selectedIndex = Math.min(selectedIndex + 1, filteredItems.length - 1);
                renderResults(resultsEl, lastQuery);
                scrollToSelected(resultsEl);
            } else if (event.key === 'ArrowUp') {
                event.preventDefault();
                selectedIndex = Math.max(selectedIndex - 1, 0);
                renderResults(resultsEl, lastQuery);
                scrollToSelected(resultsEl);
            } else if (event.key === 'Enter') {
                event.preventDefault();
                if (filteredItems[selectedIndex]) {
                    const item = filteredItems[selectedIndex];
                    closePalette();
                    executeItem(item);
                }
            } else if (event.key === 'Escape') {
                event.preventDefault();
                closePalette();
            }
        });

        backdrop.addEventListener('mousedown', event => {
            if (event.target === backdrop) closePalette();
        });

        requestAnimationFrame(() => input.focus());
    }

    function filterItems(query) {
        const items = getAllItems();
        if (!query) return items;
        return items
            .map(item => {
                const labelMatch = fuzzyMatch(query, item.label);
                const pathMatch = item.path ? fuzzyMatch(query, item.path) : { match: false, score: 0 };
                const best = labelMatch.score >= pathMatch.score ? labelMatch : pathMatch;
                return { ...item, _match: best, _labelMatch: labelMatch };
            })
            .filter(item => item._match.match)
            .sort((a, b) => b._match.score - a._match.score);
    }

    function renderResults(container, query) {
        if (!filteredItems.length) {
            container.innerHTML = `<div class="cs-cp-empty">${esc(tr('codeStudio.noResults', 'No results found'))}</div>`;
            return;
        }

        const commands = filteredItems.filter(i => i.type === 'command');
        const files = filteredItems.filter(i => i.type === 'file' || i.type === 'recent');

        let html = '';
        if (commands.length) {
            html += `<div class="cs-cp-section-label">${esc(tr('codeStudio.commands', 'Commands'))}</div>`;
            commands.forEach(item => {
                html += renderItem(item, filteredItems.indexOf(item), query);
            });
        }
        if (files.length) {
            html += `<div class="cs-cp-section-label">${esc(tr('codeStudio.files', 'Files'))}</div>`;
            files.forEach(item => {
                html += renderItem(item, filteredItems.indexOf(item), query);
            });
        }
        container.innerHTML = html;

        container.querySelectorAll('.cs-cp-item').forEach(el => {
            el.addEventListener('click', () => {
                const idx = Number(el.dataset.index);
                if (filteredItems[idx]) {
                    const item = filteredItems[idx];
                    closePalette();
                    executeItem(item);
                }
            });
            el.addEventListener('mouseenter', () => {
                selectedIndex = Number(el.dataset.index);
                container.querySelectorAll('.cs-cp-item').forEach(item => item.classList.toggle('selected', Number(item.dataset.index) === selectedIndex));
            });
        });
    }

    function renderItem(item, index, query) {
        const isSelected = index === selectedIndex;
        const glyph = item.icon === 'file' ? '{ }' : item.icon === 'clock' ? '⏱' : '>';
        const iconHtml = `<span class="cs-cp-item-icon">${iconMarkup(item.icon || 'tools', glyph)}</span>`;
        const shortcutHtml = item.shortcut ? `<span class="cs-cp-item-shortcut">${esc(item.shortcut)}</span>` : '';
        const pathHtml = item.path ? `<span class="cs-cp-item-path">${esc(item.path)}</span>` : '';
        return `<button type="button" class="cs-cp-item${isSelected ? ' selected' : ''}" data-index="${index}" role="option" aria-selected="${isSelected ? 'true' : 'false'}">
            ${iconHtml}
            <span class="cs-cp-item-label">${highlightMatch(item.label, query)}</span>
            ${pathHtml}
            ${shortcutHtml}
        </button>`;
    }

    function scrollToSelected(container) {
        const selected = container.querySelector('.cs-cp-item.selected');
        if (selected) selected.scrollIntoView({ block: 'nearest' });
    }

    function closePalette() {
        if (backdrop) {
            backdrop.remove();
            backdrop = null;
        }
    }

    function togglePalette() {
        if (backdrop) closePalette();
        else renderPalette();
    }

    window.CodeStudioCommandPalette = {
        open: renderPalette,
        close: closePalette,
        toggle: togglePalette
    };
})();
