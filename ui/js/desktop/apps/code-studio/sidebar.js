    function renderSidebar(errorMessage) {
        const sidebar = shellPart('[data-sidebar]');
        if (!sidebar) return;
        if (errorMessage) {
            sidebar.innerHTML = `<div class="cs-sidebar-head"><strong>${esc(tr('codeStudio.title', 'Code Studio'))}</strong></div>
                <div class="code-studio-error compact">${esc(errorMessage)}</div>
                <div class="cs-sidebar-resize" data-sidebar-resize></div>`;
            wireSidebarResize();
            return;
        }
        const rows = state.files.length ? state.files.map(file => treeItemRow(file, 0)).join('') : `<div class="cs-empty">${esc(tr('codeStudio.noFiles', 'No files open'))}</div>`;
        sidebar.innerHTML = `<div class="cs-sidebar-head">
            <strong>${esc(tr('codeStudio.title', 'Code Studio'))}</strong>
            <span>${esc(state.currentPath)}</span>
        </div><div class="cs-file-tree">${rows}</div>
        <div class="cs-sidebar-resize" data-sidebar-resize></div>`;
        wireSidebarTreeEvents(sidebar);
        wireSidebarDragDrop(sidebar);
        wireSidebarResize();
    }

    function treeItemRow(file, depth) {
        const isDir = file.type === 'directory';
        const isExpanded = isDir && state.expandedDirs.has(file.path);
        const indent = depth * 14;
        const icon = isDir
            ? (isExpanded ? iconMarkup('folder-open', 'D', 'cs-file-papirus-icon', 16) : iconMarkup('folder', 'D', 'cs-file-papirus-icon', 16))
            : iconMarkup(fileIconName(file.name), fileIcon(file.name), 'cs-file-papirus-icon', 16);
        const chevron = isDir
            ? `<span class="cs-tree-chevron${isExpanded ? ' expanded' : ''}">\u203a</span>`
            : '<span style="width:18px;display:inline-block"></span>';
        const childrenHtml = isDir && isExpanded ? treeChildrenHtml(file.path, depth + 1) : '';
        return `<div class="cs-tree-item${isDir ? ' is-dir' : ' is-file'}" data-file-path="${esc(file.path)}" data-type="${esc(file.type)}" data-depth="${depth}" style="padding-left:${6 + indent}px">
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
        if (!children || !children.length) return '<div class="cs-tree-children" data-dir-path="' + esc(dirPath) + '"></div>';
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
            try {
                const result = await apiClient.files(dirPath);
                if (!isLiveInstance(target)) return;
                runWithInstance(target, () => {
                    state.treeCache[dirPath] = (result.files || []).sort((a, b) => {
                        if (a.type === b.type) return a.name.localeCompare(b.name);
                        return a.type === 'directory' ? -1 : 1;
                    });
                    renderSidebar();
                });
            } catch (err) {
                if (isLiveInstance(target)) {
                    runWithInstance(target, () => {
                        state.expandedDirs.delete(dirPath);
                        renderSidebar();
                    });
                }
            }
        } else {
            renderSidebar();
        }
    }

    function wireSidebarTreeEvents(sidebar) {
        sidebar.querySelectorAll('.cs-tree-item').forEach(row => {
            row.addEventListener('click', bind(event => {
                const action = event.target.closest('[data-file-action]');
                if (action) return;
                const filePath = row.dataset.filePath;
                const fileType = row.dataset.type;
                if (fileType === 'directory') {
                    expandDirectory(filePath);
                } else {
                    openFile(filePath);
                }
            }));
            row.addEventListener('keydown', bind(event => {
                const filePath = row.dataset.filePath;
                const fileType = row.dataset.type;
                if (event.key === 'Enter') {
                    event.preventDefault();
                    if (fileType === 'directory') expandDirectory(filePath);
                    else openFile(filePath);
                }
                if (event.key === 'F2') {
                    event.preventDefault();
                    const file = findFileInTree(filePath);
                    if (file) renamePath(file);
                }
                if (event.key === 'Delete') {
                    event.preventDefault();
                    const file = findFileInTree(filePath);
                    if (file) deletePath(file);
                }
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
            if (activity === 'explorer' && state.openTabs && state.openTabs.length > 0) {
                const badge = document.createElement('span');
                badge.className = 'cs-activity-badge';
                badge.textContent = state.openTabs.length;
                btn.appendChild(badge);
            }
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
