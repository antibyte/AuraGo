    function renderSidebar(errorMessage) {
        const sidebar = shellPart('[data-sidebar]');
        if (!sidebar) return;
        const focused = document.activeElement && sidebar.contains(document.activeElement)
            ? (document.activeElement.closest('[data-file-path]') || {}).dataset
            : null;
        const focusedPath = focused ? focused.filePath : '';
        if (errorMessage) {
            sidebar.innerHTML = `${sidebarHeadMarkup()}
                <div class="code-studio-error compact">${esc(errorMessage)}</div>
                <div class="cs-sidebar-resize" data-sidebar-resize></div>`;
            wireSidebarHead(sidebar);
            wireSidebarResize();
            return;
        }
        const rows = state.files.length
            ? state.files.map(file => treeItemRow(file, 0)).join('')
            : `<div class="cs-tree-empty">${esc(tr('codeStudio.emptyFolder', 'Empty folder'))}</div>`;
        sidebar.innerHTML = `${sidebarHeadMarkup()}<div class="cs-file-tree" data-file-tree role="tree" aria-label="${esc(tr('codeStudio.explorer', 'Explorer'))}">${rows}</div>
        <div class="cs-sidebar-resize" data-sidebar-resize></div>`;
        wireSidebarHead(sidebar);
        wireSidebarTreeEvents(sidebar);
        wireSidebarDragDrop(sidebar);
        wireSidebarResize();
        if (focusedPath) {
            const row = sidebar.querySelector(`.cs-tree-item[data-file-path="${cssEscape(focusedPath)}"]`);
            if (row) row.focus();
        }
    }

    function cssEscape(value) {
        if (window.CSS && typeof window.CSS.escape === 'function') return window.CSS.escape(String(value));
        return String(value).replace(/["\\]/g, '\\$&');
    }

    function sidebarHeadMarkup() {
        const segments = codeStudioDesktopPath(state.currentPath).split('/').filter(Boolean);
        let buildPath = WORKSPACE_ROOT;
        const crumbs = [`<button type="button" class="cs-sidebar-crumb${segments.length ? '' : ' current'}" data-sidebar-nav="${esc(WORKSPACE_ROOT)}" title="${esc(WORKSPACE_ROOT)}">${iconMarkup('home', 'W', 'cs-sidebar-crumb-icon', 13)}<span>${esc(tr('codeStudio.workspaceRoot', 'Workspace'))}</span></button>`];
        segments.forEach((segment, index) => {
            buildPath += '/' + segment;
            crumbs.push('<span class="cs-sidebar-crumb-sep">\u203a</span>');
            crumbs.push(index === segments.length - 1
                ? `<span class="cs-sidebar-crumb current" title="${esc(buildPath)}">${esc(segment)}</span>`
                : `<button type="button" class="cs-sidebar-crumb" data-sidebar-nav="${esc(buildPath)}" title="${esc(buildPath)}">${esc(segment)}</button>`);
        });
        return `<div class="cs-sidebar-head">
            <div class="cs-sidebar-title">
                <strong>${esc(tr('codeStudio.explorer', 'Explorer'))}</strong>
                <span class="cs-sidebar-tools">
                    <button type="button" class="cs-sidebar-tool" data-sidebar-tool="new-file" title="${esc(tr('codeStudio.newFile', 'New File'))}">${iconMarkup('file-plus', '+', 'cs-sidebar-tool-icon', 14)}</button>
                    <button type="button" class="cs-sidebar-tool" data-sidebar-tool="new-folder" title="${esc(tr('codeStudio.newFolder', 'New Folder'))}">${iconMarkup('folder-plus', '+', 'cs-sidebar-tool-icon', 14)}</button>
                    <button type="button" class="cs-sidebar-tool" data-sidebar-tool="collapse" title="${esc(tr('codeStudio.collapseAll', 'Collapse all'))}">${iconMarkup('chevron-up', '^', 'cs-sidebar-tool-icon', 14)}</button>
                    <button type="button" class="cs-sidebar-tool" data-sidebar-tool="refresh" title="${esc(tr('codeStudio.refresh', 'Refresh'))}">${iconMarkup('refresh', 'R', 'cs-sidebar-tool-icon', 14)}</button>
                </span>
            </div>
            <div class="cs-sidebar-path" data-sidebar-path>${crumbs.join('')}</div>
        </div>`;
    }

    function wireSidebarHead(sidebar) {
        sidebar.querySelectorAll('[data-sidebar-nav]').forEach(btn => {
            btn.addEventListener('click', bind(() => refreshFiles(btn.dataset.sidebarNav)));
        });
        sidebar.querySelectorAll('[data-sidebar-tool]').forEach(btn => {
            btn.addEventListener('click', bind(() => {
                const tool = btn.dataset.sidebarTool;
                if (tool === 'new-file') createNewFile();
                else if (tool === 'new-folder') createNewFolder();
                else if (tool === 'collapse') collapseAllDirectories();
                else if (tool === 'refresh') refreshFiles(state.currentPath);
            }));
        });
    }

    function collapseAllDirectories() {
        state.expandedDirs.clear();
        state.selectedDir = state.currentPath;
        renderSidebar();
    }

    function treeItemRow(file, depth) {
        const isDir = file.type === 'directory';
        const isExpanded = isDir && state.expandedDirs.has(file.path);
        const active = activeTab();
        const isActive = !isDir && !!active && active.path === file.path;
        const isSelected = isDir && state.selectedDir === file.path && file.path !== state.currentPath;
        const isOpen = !isDir && state.openTabs.some(tab => tab.path === file.path);
        const isModified = !isDir && state.openTabs.some(tab => tab.path === file.path && tab.modified);
        const indent = depth * 14;
        const icon = isDir
            ? (isExpanded ? iconMarkup('folder-open', 'D', 'cs-file-papirus-icon', 16) : iconMarkup('folder', 'D', 'cs-file-papirus-icon', 16))
            : iconMarkup(fileIconName(file.name), fileIcon(file.name), 'cs-file-papirus-icon', 16);
        const chevron = isDir
            ? `<span class="cs-tree-chevron${isExpanded ? ' expanded' : ''}">\u203a</span>`
            : '<span class="cs-tree-chevron-spacer"></span>';
        const childrenHtml = isDir && isExpanded ? treeChildrenHtml(file.path, depth + 1) : '';
        const classes = ['cs-tree-item', isDir ? 'is-dir' : 'is-file', isActive ? 'active' : '', isSelected ? 'selected' : '', isOpen ? 'is-open' : '', isModified ? 'is-modified' : ''].filter(Boolean).join(' ');
        const expandedAttr = isDir ? ` aria-expanded="${isExpanded ? 'true' : 'false'}"` : '';
        return `<div class="${classes}" role="treeitem" tabindex="0"${expandedAttr} aria-selected="${isActive || isSelected ? 'true' : 'false'}" data-file-path="${esc(file.path)}" data-type="${esc(file.type)}" data-depth="${depth}" title="${esc(file.path)}" style="padding-left:${6 + indent}px">
            ${chevron}
            <span class="cs-file-icon">${icon}</span>
            <span class="cs-file-name">${esc(file.name)}</span>
            <span class="cs-file-actions">
                <span role="button" tabindex="0" class="cs-file-action" data-file-action="rename" title="${esc(tr('codeStudio.rename', 'Rename'))}">${iconMarkup('edit', 'E', 'cs-file-action-icon', 14)}</span>
                ${!isDir ? `<span role="button" tabindex="0" class="cs-file-action" data-file-action="download" title="${esc(tr('codeStudio.download', 'Download'))}">${iconMarkup('download', 'D', 'cs-file-action-icon', 14)}</span>` : ''}
                <span role="button" tabindex="0" class="cs-file-action danger" data-file-action="delete" title="${esc(tr('desktop.delete', 'Delete'))}">${iconMarkup('trash', 'X', 'cs-file-action-icon', 14)}</span>
            </span>
        </div>${childrenHtml}`;
    }

    function treeChildrenHtml(dirPath, depth) {
        const children = state.treeCache[dirPath];
        if (!children) return '<div class="cs-tree-children" data-dir-path="' + esc(dirPath) + '"></div>';
        if (!children.length) {
            return '<div class="cs-tree-children" data-dir-path="' + esc(dirPath) + '"><div class="cs-tree-empty" style="padding-left:' + (24 + depth * 14) + 'px">' + esc(tr('codeStudio.emptyFolder', 'Empty folder')) + '</div></div>';
        }
        return '<div class="cs-tree-children" data-dir-path="' + esc(dirPath) + '">' +
            children.map(file => treeItemRow(file, depth)).join('') + '</div>';
    }

    async function expandDirectory(dirPath) {
        const target = state;
        if (!isLiveInstance(target)) return;
        if (state.expandedDirs.has(dirPath)) {
            state.expandedDirs.delete(dirPath);
            renderSidebar();
            return;
        }
        state.expandedDirs.add(dirPath);
        if (!state.treeCache[dirPath]) {
            renderSidebar();
            try {
                const result = await apiClient.files(dirPath);
                if (!isLiveInstance(target)) return;
                runWithInstance(target, () => {
                    state.treeCache[dirPath] = sortTreeEntries(result.files || []);
                    renderSidebar();
                });
            } catch (err) {
                if (isLiveInstance(target)) {
                    runWithInstance(target, () => {
                        state.expandedDirs.delete(dirPath);
                        renderSidebar();
                        showOperationError(err);
                    });
                }
            }
        } else {
            renderSidebar();
        }
    }

    function activateTreeRow(row) {
        const filePath = row.dataset.filePath;
        if (row.dataset.type === 'directory') {
            state.selectedDir = filePath;
            expandDirectory(filePath);
        } else {
            state.selectedDir = codeStudioParentPath(filePath);
            openFile(filePath);
        }
    }

    function visibleTreeRows() {
        const root = studioRoot();
        return root ? Array.from(root.querySelectorAll('.cs-tree-item')) : [];
    }

    function treeParentRow(row, rows) {
        const depth = Number(row.dataset.depth || 0);
        if (depth <= 0) return null;
        for (let i = rows.indexOf(row) - 1; i >= 0; i--) {
            if (Number(rows[i].dataset.depth || 0) < depth) return rows[i];
        }
        return null;
    }

    function treeRowKeydown(event, row) {
        const filePath = row.dataset.filePath;
        const isDir = row.dataset.type === 'directory';
        const rows = visibleTreeRows();
        const index = rows.indexOf(row);
        const focusRow = target => { if (target) target.focus(); };
        switch (event.key) {
            case 'Enter':
            case ' ':
                event.preventDefault();
                activateTreeRow(row);
                break;
            case 'ArrowDown':
                event.preventDefault();
                focusRow(rows[index + 1]);
                break;
            case 'ArrowUp':
                event.preventDefault();
                focusRow(rows[index - 1]);
                break;
            case 'ArrowRight':
                event.preventDefault();
                if (isDir && !state.expandedDirs.has(filePath)) {
                    state.selectedDir = filePath;
                    expandDirectory(filePath);
                } else {
                    focusRow(rows[index + 1]);
                }
                break;
            case 'ArrowLeft':
                event.preventDefault();
                if (isDir && state.expandedDirs.has(filePath)) expandDirectory(filePath);
                else focusRow(treeParentRow(row, rows));
                break;
            case 'Home':
                event.preventDefault();
                focusRow(rows[0]);
                break;
            case 'End':
                event.preventDefault();
                focusRow(rows[rows.length - 1]);
                break;
            case 'F2': {
                event.preventDefault();
                const file = findFileInTree(filePath);
                if (file) renamePath(file);
                break;
            }
            case 'Delete': {
                event.preventDefault();
                const file = findFileInTree(filePath);
                if (file) deletePath(file);
                break;
            }
            default:
                break;
        }
    }

    function wireSidebarTreeEvents(sidebar) {
        sidebar.querySelectorAll('.cs-tree-item').forEach(row => {
            row.addEventListener('click', bind(event => {
                if (event.target.closest('[data-file-action]')) return;
                activateTreeRow(row);
            }));
            row.addEventListener('dblclick', bind(event => {
                if (event.target.closest('[data-file-action]')) return;
                if (row.dataset.type === 'directory') refreshFiles(row.dataset.filePath);
            }));
            row.addEventListener('keydown', bind(event => treeRowKeydown(event, row)));
            row.addEventListener('contextmenu', bind(event => {
                event.preventDefault();
                event.stopPropagation();
                const file = findFileInTree(row.dataset.filePath);
                if (!file) return;
                if (file.type === 'directory') state.selectedDir = file.path;
                showTreeContextMenu(file, event.clientX, event.clientY);
            }));
        });
        sidebar.querySelectorAll('[data-file-action]').forEach(btn => {
            btn.addEventListener('click', bind(event => {
                event.stopPropagation();
                const filePath = btn.closest('[data-file-path]').dataset.filePath;
                const file = findFileInTree(filePath);
                if (!file) return;
                const action = btn.dataset.fileAction;
                if (action === 'rename') renamePath(file);
                if (action === 'delete') deletePath(file);
                if (action === 'download') downloadFile(file);
            }));
            btn.addEventListener('keydown', bind(event => {
                if (event.key !== 'Enter' && event.key !== ' ') return;
                event.preventDefault();
                event.stopPropagation();
                btn.click();
            }));
        });
        const tree = sidebar.querySelector('[data-file-tree]');
        if (tree) {
            tree.addEventListener('contextmenu', bind(event => {
                if (event.target.closest('.cs-tree-item')) return;
                event.preventDefault();
                event.stopPropagation();
                state.selectedDir = state.currentPath;
                showTreeContextMenu(null, event.clientX, event.clientY);
            }));
        }
    }

    function showTreeContextMenu(file, x, y) {
        const isDir = !file || file.type === 'directory';
        const dirPath = !file ? state.currentPath : (isDir ? file.path : codeStudioParentPath(file.path));
        const items = [];
        if (file && !isDir) items.push({ id: 'open', label: tr('desktop.file_dialog_open', 'Open'), icon: 'file', action: bind(() => openFile(file.path)) });
        if (file && isDir) items.push({ id: 'open-folder', label: tr('desktop.file_dialog_open', 'Open'), icon: 'folder-open', action: bind(() => refreshFiles(file.path)) });
        items.push({ id: 'new-file', label: tr('codeStudio.newFile', 'New File'), icon: 'file-plus', action: bind(() => {
            state.selectedDir = dirPath;
            if (file && isDir) state.expandedDirs.add(dirPath);
            createNewFile();
        }) });
        items.push({ id: 'new-folder', label: tr('codeStudio.newFolder', 'New Folder'), icon: 'folder-plus', action: bind(() => {
            state.selectedDir = dirPath;
            if (file && isDir) state.expandedDirs.add(dirPath);
            createNewFolder();
        }) });
        if (file) {
            items.push({ separator: true });
            items.push({ id: 'rename', label: tr('codeStudio.rename', 'Rename'), icon: 'edit', shortcut: 'F2', action: bind(() => renamePath(file)) });
            if (!isDir) items.push({ id: 'download', label: tr('codeStudio.download', 'Download'), icon: 'download', action: bind(() => downloadFile(file)) });
            items.push({ id: 'copy-path', label: tr('codeStudio.copyPath', 'Copy path'), icon: 'copy', action: bind(() => copyTextToClipboard(file.path)) });
            items.push({ separator: true });
            items.push({ id: 'delete', label: tr('desktop.delete', 'Delete'), icon: 'trash', shortcut: 'Del', action: bind(() => deletePath(file)) });
        } else {
            items.push({ separator: true });
            items.push({ id: 'refresh', label: tr('codeStudio.refresh', 'Refresh'), icon: 'refresh', action: bind(() => refreshFiles(state.currentPath)) });
        }
        showStudioContextMenu(x, y, items);
    }

    function highlightActiveTreeRow() {
        const root = studioRoot();
        if (!root) return;
        const active = activeTab();
        root.querySelectorAll('.cs-tree-item').forEach(row => {
            const isFile = row.dataset.type !== 'directory';
            const path = row.dataset.filePath;
            const isActive = isFile && !!active && path === active.path;
            row.classList.toggle('active', isActive);
            row.classList.toggle('is-open', isFile && state.openTabs.some(tab => tab.path === path));
            row.classList.toggle('is-modified', isFile && state.openTabs.some(tab => tab.path === path && tab.modified));
            if (isFile) row.setAttribute('aria-selected', isActive ? 'true' : 'false');
        });
    }

    function findFileInTree(path) {
        for (const files of Object.values(state.treeCache)) {
            const found = files.find(f => f.path === path);
            if (found) return found;
        }
        const found = state.files.find(f => f.path === path);
        if (found) return found;
        const name = path.split('/').filter(Boolean).pop() || '';
        return { path, name, type: 'file', size: 0 };
    }

    function wireSidebarDragDrop(sidebar) {
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
            const dropRow = event.target.closest && event.target.closest('.cs-tree-item[data-type="directory"]');
            const uploadDir = dropRow ? dropRow.dataset.filePath : targetDirectory();
            try {
                for (const file of files) {
                    await apiClient.uploadFile(uploadDir, file);
                    if (!isLiveInstance(target)) return;
                }
                if (files.length) await runAsyncStep(target, () => reloadTreeDirectory(uploadDir));
            } catch (err) {
                if (isLiveInstance(target)) runWithInstance(target, () => showOperationError(err));
            }
        });
    }

    function renderActivityBar() {
        const bar = shellPart('[data-activity-bar]');
        if (!bar) return;
        bar.querySelectorAll('.cs-activity-btn').forEach(btn => {
            const activity = btn.dataset.activity;
            const existingBadge = btn.querySelector('.cs-activity-badge');
            if (existingBadge) existingBadge.remove();
            if (activity === 'explorer') {
                btn.classList.toggle('active', state.sidebarVisible);
            } else if (activity === 'search') {
                btn.classList.toggle('active', state.searchVisible);
            } else if (activity === 'git') {
                btn.classList.toggle('active', state.gitVisible);
                if (state.gitChanges && state.gitChanges.length > 0) {
                    const badge = document.createElement('span');
                    badge.className = 'cs-activity-badge';
                    badge.textContent = state.gitChanges.length;
                    btn.appendChild(badge);
                }
            } else if (activity === 'agent') {
                btn.classList.toggle('active', state.agentVisible);
            } else if (activity === 'terminal') {
                btn.classList.toggle('active', state.terminalVisible);
            }
            btn.setAttribute('aria-pressed', btn.classList.contains('active') ? 'true' : 'false');
            if (!btn._wired) {
                btn._wired = true;
                btn.addEventListener('click', bind(() => {
                    const act = btn.dataset.activity;
                    if (act === 'explorer') toggleSidebar();
                    else if (act === 'search') toggleSearch();
                    else if (act === 'git') toggleGitPanel();
                    else if (act === 'agent') toggleAgentPanel();
                    else if (act === 'terminal') toggleTerminal();
                }));
            }
        });
    }
