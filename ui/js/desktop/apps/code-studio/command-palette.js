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
        const cmds = [
            { id: 'new-file', label: tr('codeStudio.newFile', 'New File'), shortcut: 'Ctrl+N', icon: 'file-plus', action: () => getApp()?.api && state && typeof state === 'object' && getApp() },
            { id: 'new-folder', label: tr('codeStudio.newFolder', 'New Folder'), icon: 'folder-plus' },
            { id: 'save', label: tr('codeStudio.save', 'Save'), shortcut: 'Ctrl+S', icon: 'save' },
            { id: 'save-all', label: tr('codeStudio.saveAll', 'Save All'), icon: 'save' },
            { id: 'run', label: tr('codeStudio.run', 'Run'), shortcut: 'F5', icon: 'run' },
            { id: 'upload', label: tr('codeStudio.upload', 'Upload'), icon: 'upload' },
            { id: 'refresh', label: tr('codeStudio.refresh', 'Refresh'), icon: 'refresh' },
            { id: 'toggle-sidebar', label: tr('codeStudio.sidebar', 'Toggle Sidebar'), shortcut: 'Ctrl+B', icon: 'sidebar' },
            { id: 'toggle-terminal', label: tr('codeStudio.toggleTerminal', 'Toggle Terminal'), icon: 'terminal' },
            { id: 'toggle-agent', label: tr('codeStudio.agentChat', 'Toggle Agent Chat'), shortcut: 'Ctrl+Shift+A', icon: 'chat' },
            { id: 'toggle-search', label: tr('codeStudio.search', 'Search in Files'), shortcut: 'Ctrl+Shift+F', icon: 'search' },
            { id: 'toggle-zen', label: tr('codeStudio.zenMode', 'Toggle Zen Mode'), shortcut: 'Ctrl+K Z', icon: 'maximize' },
            { id: 'zoom-in', label: tr('codeStudio.zoomIn', 'Zoom In'), shortcut: 'Ctrl+=', icon: 'zoom-in' },
            { id: 'zoom-out', label: tr('codeStudio.zoomOut', 'Zoom Out'), shortcut: 'Ctrl+-', icon: 'zoom-out' },
            { id: 'zoom-reset', label: tr('codeStudio.zoomReset', 'Reset Zoom'), shortcut: 'Ctrl+0', icon: 'zoom-reset' }
        ];
        return cmds;
    }

    function getOpenTabs() {
        const state = getState();
        if (!state || !state.openTabs) return [];
        return state.openTabs.map((tab, index) => ({
            id: 'tab:' + index,
            label: tab.path.split('/').filter(Boolean).pop() || tab.path,
            path: tab.path,
            icon: 'file',
            type: 'file'
        }));
    }

    function getRecentFiles() {
        const state = getState();
        if (!state || !state.recentFiles) return [];
        return state.recentFiles.slice(0, 8).map(path => ({
            id: 'recent:' + path,
            label: path.split('/').filter(Boolean).pop() || path,
            path: path,
            icon: 'clock',
            type: 'recent'
        }));
    }

    function getAllItems() {
        const commands = getCommands().map(cmd => ({ ...cmd, type: 'command' }));
        const tabs = getOpenTabs();
        const recent = getRecentFiles();
        return [...commands, ...tabs, ...recent];
    }

    function executeItem(item) {
        const app = getApp();
        const state = getState();
        if (!app || !state) return;
        const id = item.id || '';
        if (id === 'new-file' && typeof app.api !== 'undefined') {
            // Use the global functions exposed in the IIFE
        }
        // Dispatch via command id mapping
        const actions = {
            'new-file': () => callAppFunction('createNewFile'),
            'new-folder': () => callAppFunction('createNewFolder'),
            'save': () => callAppFunction('saveCurrentFile'),
            'save-all': () => callAppFunction('saveCurrentFile'),
            'run': () => callAppFunction('runCurrentFile'),
            'upload': () => callAppFunction('uploadFile'),
            'refresh': () => callAppFunction('refreshFiles'),
            'toggle-sidebar': () => callAppFunction('toggleSidebar'),
            'toggle-terminal': () => callAppFunction('toggleTerminal'),
            'toggle-agent': () => callAppFunction('toggleAgentPanel'),
            'toggle-search': () => callAppFunction('toggleSearch'),
            'toggle-zen': () => callAppFunction('toggleZenMode'),
            'zoom-in': () => callAppFunction('adjustEditorZoom', 1),
            'zoom-out': () => callAppFunction('adjustEditorZoom', -1),
            'zoom-reset': () => callAppFunction('resetEditorZoom')
        };
        if (item.type === 'file' || item.type === 'recent') {
            callAppFunction('openFile', item.path);
        } else if (actions[id]) {
            actions[id]();
        }
    }

    function callAppFunction(name, ...args) {
        // Functions are inside the IIFE, so we need to access them through the DOM event system
        // We'll dispatch keyboard shortcuts or click events as fallback
        const state = getState();
        if (!state) return;
        const root = state.root;
        if (!root) return;
        const studio = root.querySelector('[data-code-studio]');
        if (!studio) return;

        // Map function names to toolbar/activity bar button clicks
        const buttonMap = {
            'createNewFile': '[data-action="new-file"]',
            'createNewFolder': '[data-action="new-folder"]',
            'saveCurrentFile': '[data-action="save"]',
            'runCurrentFile': '[data-action="run"]',
            'uploadFile': '[data-action="upload"]',
            'refreshFiles': '[data-action="refresh"]',
            'toggleSidebar': '[data-activity="explorer"]',
            'toggleTerminal': '[data-activity="terminal"]',
            'toggleAgentPanel': '[data-activity="agent"]',
            'toggleSearch': '[data-activity="search"]'
        };

        if (buttonMap[name]) {
            const btn = studio.querySelector(buttonMap[name]);
            if (btn) { btn.click(); return; }
        }

        // For toggle-zen, dispatch custom event
        if (name === 'toggleZenMode') {
            document.dispatchEvent(new CustomEvent('code-studio:toggle-zen'));
            return;
        }

        // For open file, dispatch custom event
        if (name === 'openFile' && args[0]) {
            document.dispatchEvent(new CustomEvent('code-studio:open-file', { detail: { path: args[0] } }));
            return;
        }

        // For zoom, dispatch custom event
        if (name === 'adjustEditorZoom' || name === 'resetEditorZoom') {
            document.dispatchEvent(new CustomEvent('code-studio:zoom', { detail: { fn: name, args } }));
            return;
        }
    }

    function renderPalette() {
        if (backdrop) return;
        backdrop = document.createElement('div');
        backdrop.className = 'cs-command-palette-backdrop';
        backdrop.innerHTML = `<div class="cs-command-palette">
            <div class="cs-command-palette-input">
                <span class="cs-cp-icon">${getApp()?.api ? '' : '?'}</span>
                <input type="text" placeholder="${esc(tr('codeStudio.cpPlaceholder', 'Search files, commands, tabs...'))}" autocomplete="off" spellcheck="false">
            </div>
            <div class="cs-command-palette-results" data-cp-results></div>
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
                    closePalette();
                    executeItem(filteredItems[selectedIndex]);
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
            commands.forEach((item, i) => {
                const globalIndex = filteredItems.indexOf(item);
                html += renderItem(item, globalIndex, query);
            });
        }
        if (files.length) {
            html += `<div class="cs-cp-section-label">${esc(tr('codeStudio.files', 'Files'))}</div>`;
            files.forEach((item) => {
                const globalIndex = filteredItems.indexOf(item);
                html += renderItem(item, globalIndex, query);
            });
        }
        container.innerHTML = html;

        container.querySelectorAll('.cs-cp-item').forEach(el => {
            el.addEventListener('click', () => {
                const idx = Number(el.dataset.index);
                if (filteredItems[idx]) {
                    closePalette();
                    executeItem(filteredItems[idx]);
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
        const iconHtml = item.icon ? `<span class="cs-cp-item-icon">${esc(item.icon === 'file' ? '{ }' : item.icon === 'clock' ? '⏱' : '>')}</span>` : '';
        const shortcutHtml = item.shortcut ? `<span class="cs-cp-item-shortcut">${esc(item.shortcut)}</span>` : '';
        const pathHtml = item.path ? `<span class="cs-cp-item-path">${esc(item.path)}</span>` : '';
        return `<button type="button" class="cs-cp-item${isSelected ? ' selected' : ''}" data-index="${index}">
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

    // Listen for custom events from the command palette
    document.addEventListener('code-studio:toggle-zen', () => {
        const app = getApp();
        if (app) {
            const state = getState();
            if (state) {
                // Toggle zen through the root element
                const root = state.root;
                if (root) {
                    const studio = root.querySelector('[data-code-studio]');
                    if (studio) {
                        const isZen = studio.dataset.zen === 'true';
                        studio.dataset.zen = isZen ? 'false' : 'true';
                    }
                }
            }
        }
    });

    document.addEventListener('code-studio:open-file', (event) => {
        const app = getApp();
        if (app && event.detail && event.detail.path) {
            // Try to use the exposed API
            if (typeof app.openFile === 'function') {
                app.openFile(event.detail.path);
            }
        }
    });

    document.addEventListener('code-studio:zoom', (event) => {
        const app = getApp();
        if (app && event.detail) {
            if (event.detail.fn === 'adjustEditorZoom' && typeof app.loadState === 'function') {
                // Dispatch keyboard events as fallback
                const key = event.detail.args[0] > 0 ? '=' : '-';
                document.dispatchEvent(new KeyboardEvent('keydown', { key, ctrlKey: true, bubbles: true }));
            } else if (event.detail.fn === 'resetEditorZoom') {
                document.dispatchEvent(new KeyboardEvent('keydown', { key: '0', ctrlKey: true, bubbles: true }));
            }
        }
    });

    window.CodeStudioCommandPalette = {
        open: renderPalette,
        close: closePalette,
        toggle: togglePalette
    };
})();
