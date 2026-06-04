    function isEditableTarget(target) { return !!(target && target.closest && target.closest('input, textarea, select, [contenteditable="true"], [contenteditable=""], .ql-editor, .xterm-helper-textarea')); }
    function isNativeContextMenuTarget(target) { return isEditableTarget(target); }
    function shouldAllowBrowserContextMenu(event) { const target = event && event.target; if (isNativeContextMenuTarget(target)) return true; const selection = window.getSelection && window.getSelection(); if (!selection || selection.isCollapsed || !String(selection).trim()) return false; if (!target || !target.closest) return false; return !!target.closest('.vd-window-content, .vd-modal, .vd-qc-modal, .vd-context-native-text'); }
    function suppressBrowserContextMenu(event) { if (!event || event.defaultPrevented || shouldAllowBrowserContextMenu(event)) return false; event.preventDefault(); event.stopPropagation(); closeContextMenu(); return true; }

    function wireContextMenuBoundary(root, options) {
        if (!root || root.dataset.contextSuppressed === 'true') return;
        root.dataset.contextSuppressed = 'true';
        root.addEventListener('contextmenu', event => {
            if (event.defaultPrevented) return;
            if (options && typeof options.onContextMenu === 'function' && options.onContextMenu(event) === true) {
                event.preventDefault();
                event.stopPropagation();
                return;
            }
            suppressBrowserContextMenu(event);
        });
    }

    function wireWindowContextMenu(win, id) {
        win.addEventListener('contextmenu', event => {
            if (event.defaultPrevented) return;
            if (event.target.closest('.vd-window-titlebar')) {
                showWindowContextMenu(event, id);
                return;
            }
            suppressBrowserContextMenu(event);
        });
    }

    function normalizeContextMenuItems(items) {
        return (Array.isArray(items) ? items : []).map(item => {
            if (!item || item.hidden) return null;
            if (item.type === 'separator' || item.separator) return { separator: true };
            return Object.assign({}, item, {
                label: item.label || (item.labelKey ? t(item.labelKey) : ''),
                disabled: typeof item.disabled === 'function' ? !!item.disabled() : !!item.disabled
            });
        }).filter(Boolean);
    }

    function contextMenuUsableBottom() {
        const taskbar = document.querySelector('.vd-taskbar');
        if (window.matchMedia && window.matchMedia('(max-width: 820px)').matches && taskbar) {
            const rect = taskbar.getBoundingClientRect();
            if (rect.top > 0) return Math.max(8, Math.floor(rect.top - 8));
        }
        return window.innerHeight - 8;
    }

    function showContextMenu(x, y, items) {
        closeContextMenu(true);
        items = normalizeContextMenuItems(items);
        if (!items.length) return;
        const actions = new Map();
        const renderItems = (menuItems, path) => (menuItems || []).map((item, index) => {
            if (item.separator) return '<div class="vd-context-separator" role="separator"></div>';
            const actionKey = path.concat(String(item.id || index)).join('/');
            const icon = `<span class="vd-context-icon">${iconMarkup(item.icon || 'tools', item.fallback || item.icon || '', 'vd-context-papirus-icon', 16)}</span>`;
            const label = `<span>${esc(item.label)}</span>`;
            const disabled = item.disabled ? 'disabled' : '';
            const submenuItems = normalizeContextMenuItems(item.items || item.children || []);
            if (submenuItems.length) {
                return `<div class="vd-context-submenu" role="none">
                    <button type="button" class="vd-context-item" role="menuitem" ${disabled}>${icon}${label}<span class="vd-context-arrow">â€º</span></button>
                    <div class="vd-context-submenu-popover" role="menu">${renderItems(submenuItems, path.concat(String(item.id || index)))}</div>
                </div>`;
            }
            actions.set(actionKey, item);
            return `<button type="button" class="vd-context-item" role="menuitem" data-context-action="${esc(actionKey)}" ${disabled}>${icon}${label}</button>`;
        }).join('');
        const menu = document.createElement('div');
        menu.className = 'vd-context-menu';
        menu.setAttribute('role', 'menu');
        menu.innerHTML = renderItems(items, []);
        document.body.appendChild(menu);
        const usableBottom = contextMenuUsableBottom();
        const maxMenuHeight = Math.max(160, usableBottom - 8);
        menu.style.maxHeight = maxMenuHeight + 'px';
        menu.style.overflowY = menu.scrollHeight > maxMenuHeight ? 'auto' : '';
        const rect = menu.getBoundingClientRect();
        menu.style.left = Math.max(8, Math.min(x, window.innerWidth - rect.width - 8)) + 'px';
        menu.style.top = Math.max(8, Math.min(y, usableBottom - rect.height)) + 'px';
        animateThen(menu, 'vd-context-menu-opening', isFruityTheme() ? 150 : 100);
        menu.querySelectorAll('[data-context-action]').forEach(btn => {
            btn.addEventListener('click', () => {
                const item = actions.get(btn.dataset.contextAction);
                closeContextMenu();
                if (item && item.action) item.action();
            });
        });
        state.contextMenu = menu;
        state.contextMenuKeydown = closeContextMenuOnEscape;
        document.addEventListener('keydown', closeContextMenuOnEscape);
    }

    function showDesktopContextMenu(event) {
        if (event.target.closest('.vd-icon, .vd-widget, .vd-window, .vd-start-menu')) return;
        event.preventDefault();
        selectDesktopIcon(null);
        const items = [
            { label: t('desktop.context_new_file'), icon: 'file-plus', fallback: '+', action: () => createFileInPath('Desktop') },
            { label: t('desktop.context_new_folder'), icon: 'folder-plus', fallback: '+', action: () => createFolderInPath('Desktop') },
            { label: t('desktop.fm.paste'), icon: 'clipboard', fallback: 'V', disabled: !hasDesktopFileClipboard(), action: () => pasteDesktopFileClipboard('Desktop', { clientX: event.clientX, clientY: event.clientY }) },
            { separator: true },
            { label: t('desktop.widget_manager'), icon: 'widgets', fallback: 'W', action: () => showWidgetManager() },
            { label: t('desktop.app_manager'), icon: 'apps', fallback: 'A', action: () => showAppManager() },
            { separator: true },
            { label: t('desktop.context_refresh'), icon: 'refresh', fallback: 'R', action: () => loadBootstrap() },
            { label: t('desktop.context_icon_grid'), icon: desktopIconGridEnabled() ? 'check-square' : 'square', fallback: 'G', action: toggleDesktopIconGrid },
            { label: t('desktop.context_sort_icons'), icon: 'sort', fallback: 'S', action: autoArrangeIcons }
        ];
        showContextMenu(event.clientX, event.clientY, items);
    }

    function showIconContextMenu(event, btn) {
        event.preventDefault();
        const commandIcons = selectedDesktopCommandIcons(btn);
        if (!btn.classList.contains('selected')) selectDesktopIcon(btn);
        const path = btn.dataset.path || '';
        const appId = btn.dataset.appId || '';
        const kind = btn.dataset.kind || '';
        const isDesktopEntry = btn.dataset.desktopEntry === 'true';
        const items = [
            { label: t('desktop.context_open'), icon: 'folder-open', fallback: 'O', action: () => activateDesktopItems(commandIcons) }
        ];
        if (isTrashIcon(btn)) {
            items.push(
                { label: t('desktop.context_empty_trash'), icon: 'trash', fallback: 'X', action: () => emptyTrash() },
                { separator: true },
                { label: t('desktop.context_properties'), icon: 'info', fallback: 'i', action: () => showProperties(btn.querySelector('.vd-icon-label').textContent, path || btn.dataset.id) }
            );
            showContextMenu(event.clientX, event.clientY, items);
            return;
        }
        if (isDesktopEntry || kind === 'file') {
            if (kind === 'file') {
                items.push({ separator: true }, { label: t('desktop.fm.add_to_chat', 'Add to chat'), icon: 'chat', fallback: 'A', action: () => desktopBatchFileEntries(btn).forEach(addFileContextToChat) }, { label: t('desktop.fm.ask_agent', 'Ask Agent'), icon: 'agent', fallback: 'Q', action: () => desktopBatchFileEntries(btn).forEach(askAgentAboutFile) });
            }
            items.push(
                { separator: true },
                { label: t('desktop.fm.cut'), icon: 'scissors', fallback: 'X', action: () => setDesktopFileClipboard('cut', desktopBatchPaths(btn)) },
                { label: t('desktop.fm.copy'), icon: 'copy', fallback: 'C', action: () => setDesktopFileClipboard('copy', desktopBatchPaths(btn)) },
                { label: t('desktop.fm.paste'), icon: 'clipboard', fallback: 'V', disabled: kind !== 'directory' || !hasDesktopFileClipboard(), action: () => pasteDesktopFileClipboard(path) },
                { separator: true },
                { label: t('desktop.context_rename'), icon: 'edit', fallback: 'E', action: () => renamePath(path) },
                { label: t('desktop.context_delete'), icon: 'trash', fallback: 'X', action: () => deleteDesktopPaths(desktopBatchPaths(btn)) }
            );
            if (btn.dataset.webPath) {
                items.push({ label: t('desktop.media_download'), icon: 'download', fallback: 'D', action: () => downloadMediaPath(btn.dataset.webPath, btn.querySelector('.vd-icon-label').textContent) });
            } else if (kind === 'file') {
                items.push({ label: t('desktop.media_download'), icon: 'download', fallback: 'D', action: () => downloadDesktopPath(path, btn.querySelector('.vd-icon-label').textContent) });
            }
        } else {
            items.push({ label: t('desktop.context_remove_from_desktop'), icon: 'x', fallback: 'X', action: () => removeDesktopShortcuts(desktopBatchShortcutIds(btn)) });
        }
        if (appId) {
            const appIsBuiltin = isBuiltinApp(appId);
            items.push({ label: t('desktop.context_delete_app'), icon: 'trash', fallback: 'X', disabled: appIsBuiltin, action: () => deleteDesktopApps(desktopBatchAppIds(btn)) });
        }
        items.push(
            { separator: true },
            { label: t('desktop.context_properties'), icon: 'info', fallback: 'i', action: () => showProperties(btn.querySelector('.vd-icon-label').textContent, path || btn.dataset.id) }
        );
        showContextMenu(event.clientX, event.clientY, items);
    }

    function showStartAppContextMenu(event, appId) {
        event.preventDefault();
        const items = [
            { label: t('desktop.context_open'), icon: 'folder-open', fallback: 'O', action: () => openApp(appId) },
            { label: t('desktop.context_add_to_desktop'), icon: 'desktop', fallback: 'D', action: () => addDesktopShortcut(appId) }
        ];
        if (!isBuiltinApp(appId)) {
            items.push({ separator: true }, { label: t('desktop.context_delete_app'), icon: 'trash', fallback: 'X', action: () => deleteDesktopApp(appId) });
        }
        showContextMenu(event.clientX, event.clientY, items);
    }

    function showWidgetContextMenu(event, widget) {
        event.preventDefault();
        showContextMenu(event.clientX, event.clientY, [
            { label: t('desktop.context_open'), icon: 'folder-open', fallback: 'O', action: () => widget.app_id && openApp(widget.app_id) },
            { label: t('desktop.widget_remove_from_desktop'), icon: 'x', fallback: 'X', action: () => setWidgetVisible(widget.id, false) },
            { separator: true },
            { label: t('desktop.widget_manager'), icon: 'widgets', fallback: 'W', action: () => showWidgetManager() }
        ]);
    }

    function showWindowContextMenu(event, id) {
        event.preventDefault();
        const item = state.windows.get(id);
        if (!item) return;
        showContextMenu(event.clientX, event.clientY, [
            { label: t('desktop.context_restore'), icon: 'monitor', fallback: 'W', action: () => focusWindow(id) },
            { label: t('desktop.context_minimize'), icon: 'chevron-down', fallback: '_', action: () => minimizeWindow(id) },
            { label: item.maximized ? t('desktop.restore') : t('desktop.context_maximize'), icon: 'grid', fallback: 'M', action: () => toggleMaximizeWindow(id) },
            { separator: true },
            { label: t('desktop.context_close'), icon: 'x', fallback: 'X', action: () => closeWindow(id) }
        ]);
    }

    function autoArrangeIcons() {
        arrangeDesktopIconsToGrid();
    }

    function toggleDesktopIconGrid() {
        const enabled = !desktopIconGridEnabled();
        setDesktopIconGridEnabled(enabled);
        if (enabled) arrangeDesktopIconsToGrid();
    }

    function pathDir(path) {
        const parts = String(path || '').split('/').filter(Boolean);
        parts.pop();
        return parts.join('/');
    }

    function workspaceJoinPath(base, name) {
        const left = String(base || '').replace(/\\/g, '/').replace(/\/+$/, '');
        const right = String(name || '').replace(/\\/g, '/').replace(/^\/+/, '');
        return left ? (right ? left + '/' + right : left) : right;
    }

    function pathBaseName(path) {
        const parts = normalizeDesktopPath(path).split('/').filter(Boolean);
        return parts.pop() || '';
    }

    function trashNameCandidate(name, index) {
        if (index <= 1) return name;
        const dot = name.lastIndexOf('.');
        if (dot > 0) return name.slice(0, dot) + ' (' + index + ')' + name.slice(dot);
        return name + ' (' + index + ')';
    }

    async function listTrashEntries() {
        const body = await api('/api/desktop/files?path=' + encodeURIComponent('Trash'));
        return Array.isArray(body.files) ? body.files : [];
    }

    async function uniqueTrashDestination(path) {
        const name = pathBaseName(path) || 'item';
        const entries = await listTrashEntries();
        const existing = new Set(entries.map(entry => String(entry.name || pathBaseName(entry.path)).toLowerCase()));
        for (let index = 1; index < 1000; index += 1) {
            const candidate = trashNameCandidate(name, index);
            if (!existing.has(candidate.toLowerCase())) return workspaceJoinPath('Trash', candidate);
        }
        return workspaceJoinPath('Trash', name + ' ' + Date.now());
    }

    async function refreshDesktopAfterFileChange() {
        await loadBootstrap();
        const active = state.windows.get(state.activeWindowId);
        if (active && active.appId === 'files') renderFiles(active.id, state.filesPath);
    }

    async function promptDialog(title, value) {
        return modalDialog({ title, input: true, value: value || '' });
    }

    async function confirmDialog(title, message) {
        return modalDialog({ title, message, confirmOnly: true });
    }

    function desktopFileDialogContext() {
        return {
            openFileDialog: options => openDesktopFileDialog(options || {}),
            saveFileDialog: options => saveDesktopFileDialog(options || {}),
            importFilesFromHost: options => importHostFiles(options || {}),
            exportDesktopFile: options => exportWorkspaceFile(options || {})
        };
    }

    function withDesktopFileDialogs(context, extras) {
        return Object.assign({}, context || {}, desktopFileDialogContext(), extras || {});
    }

    function desktopReadonly() {
        return !!((state.bootstrap || {}).readonly);
    }

    function officeAppContext(context) {
        return withDesktopFileDialogs(context, { esc, api, t, iconMarkup, notify: showDesktopNotification, readonly: desktopReadonly(), loadBootstrap, updateWindowContext: updateWindowContext, openAgentChatForFile, setWindowMenus, clearWindowMenus, wireContextMenuBoundary, promptDialog });
    }

    function viewerAppContext(context) {
        return withDesktopFileDialogs(context, { esc, api, t, iconMarkup, notify: showDesktopNotification, readonly: desktopReadonly(), loadBootstrap, setWindowMenus, clearWindowMenus, wireContextMenuBoundary, openApp });
    }

function modalDialog(options) {
        closeContextMenu();
        const previousFocus = document.activeElement;
        const overlay = document.createElement('div');
        overlay.className = 'vd-modal-backdrop';
        overlay.innerHTML = `<form class="vd-modal" role="dialog" aria-modal="true">
            <div class="vd-modal-title">${esc(options.title || '')}</div>
            ${options.message ? `<div class="vd-modal-copy">${esc(options.message)}</div>` : ''}
            ${options.input ? `<input class="vd-modal-input" value="${esc(options.value || '')}" autocomplete="off">` : ''}
            <div class="vd-modal-actions">
                <button type="button" class="vd-button" data-cancel>${esc(t('desktop.cancel'))}</button>
                <button type="submit" class="vd-button vd-button-primary">${esc(t('desktop.ok'))}</button>
            </div>
        </form>`;
        document.body.appendChild(overlay);
        const form = overlay.querySelector('form');
        const input = overlay.querySelector('input');
        const primaryBtn = overlay.querySelector('[type="submit"]');
        if (input) {
            input.focus();
            input.select();
        } else if (primaryBtn) {
            primaryBtn.focus();
        }
        function trapFocus(event) {
            if (!overlay.contains(event.target)) {
                event.preventDefault();
                (input || primaryBtn || form).focus();
            }
        }
        document.addEventListener('focusin', trapFocus);
        return new Promise(resolve => {
            const finish = value => {
                document.removeEventListener('focusin', trapFocus);
                overlay.remove();
                if (previousFocus && typeof previousFocus.focus === 'function') previousFocus.focus();
                resolve(value);
            };
            overlay.querySelector('[data-cancel]').addEventListener('click', () => finish(options.input ? null : false));
            overlay.addEventListener('click', event => { if (event.target === overlay) finish(options.input ? null : false); });
            form.addEventListener('submit', event => {
                event.preventDefault();
                finish(options.input ? input.value.trim() : true);
            });
        });
    }

    async function createFileInPath(basePath) {
        const name = await promptDialog(t('desktop.new_file'), 'untitled.txt');
        if (!name) return;
        const path = workspaceJoinPath(basePath, name);
        try {
            await api('/api/desktop/file', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ path, content: '' })
            });
            await loadBootstrap();
            const active = state.windows.get(state.activeWindowId);
            if (active && active.appId === 'files') renderFiles(active.id, state.filesPath);
            openApp('editor', { path, content: '' });
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err.message });
        }
    }

    async function createFolderInPath(basePath) {
        const name = await promptDialog(t('desktop.new_folder'), 'New Folder');
        if (!name) return;
        try {
            await api('/api/desktop/directory', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ path: workspaceJoinPath(basePath, name) })
            });
            await loadBootstrap();
            const active = state.windows.get(state.activeWindowId);
            if (active && active.appId === 'files') renderFiles(active.id, state.filesPath);
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err.message });
        }
    }

    async function renamePath(path) {
        if (!path) return;
        const current = String(path).split('/').pop();
        const name = await promptDialog(t('desktop.rename'), current);
        if (!name || name === current) return;
        try {
            await api('/api/desktop/file', {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ old_path: path, new_path: workspaceJoinPath(pathDir(path), name) })
            });
            await loadBootstrap();
            const active = state.windows.get(state.activeWindowId);
            if (active && active.appId === 'files') renderFiles(active.id, state.filesPath);
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err.message });
        }
    }

    async function deletePath(path) {
        if (!path) return;
        if (settingBool('files.confirm_delete')) {
            const confirmed = await confirmDialog(t('desktop.confirm_delete'), t('desktop.confirm_delete_msg', { path }));
            if (!confirmed) return;
        }
        try {
            await api('/api/desktop/file?path=' + encodeURIComponent(path), { method: 'DELETE' });
            await loadBootstrap();
            const active = state.windows.get(state.activeWindowId);
            if (active && active.appId === 'files') renderFiles(active.id, state.filesPath);
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err.message });
        }
    }

    async function movePathToTrash(path) {
        const cleanPath = normalizeDesktopPath(path);
        if (!cleanPath || isTrashPath(cleanPath) || isInsideTrashPath(cleanPath)) return;
        try {
            const trashDestination = await uniqueTrashDestination(cleanPath);
            await api('/api/desktop/file', {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ old_path: cleanPath, new_path: trashDestination })
            });
            removeIconPosition('desktop-entry-' + cleanPath);
            await refreshDesktopAfterFileChange();
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err.message });
        }
    }

    async function handleTrashDrop(btn) {
        if (!btn || isTrashIcon(btn)) return;
        if (btn.dataset.desktopEntry === 'true') {
            await movePathToTrash(btn.dataset.path || '');
            return;
        }
        await removeDesktopShortcut(btn.dataset.id || '');
    }

    async function emptyTrash() {
        try {
            const entries = await listTrashEntries();
            if (entries.length === 0) {
                showDesktopNotification({ title: t('desktop.notification'), message: t('desktop.trash_empty') });
                return;
            }
            if (settingBool('files.confirm_delete')) {
                const confirmed = await confirmDialog(t('desktop.confirm_empty_trash'), t('desktop.confirm_empty_trash_msg', { count: entries.length }));
                if (!confirmed) return;
            }
            for (const entry of entries) {
                if (entry && entry.path) await api('/api/desktop/file?path=' + encodeURIComponent(entry.path), { method: 'DELETE' });
            }
            await refreshDesktopAfterFileChange();
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err.message });
        }
    }

    async function addDesktopShortcut(appId) {
        if (!appId) return;
        try {
            await api('/api/desktop/shortcuts', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ app_id: appId })
            });
            await loadBootstrap();
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err.message });
        }
    }

    async function removeDesktopShortcut(id) {
        if (!id) return;
        try {
            await api('/api/desktop/shortcuts?id=' + encodeURIComponent(id), { method: 'DELETE' });
            removeIconPosition(id);
            await loadBootstrap();
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err.message });
        }
    }

    async function deleteDesktopApp(appId) {
        if (!appId || isBuiltinApp(appId)) return;
        const app = appById(appId);
        const name = app ? appName(app) : appId;
        const confirmed = await confirmDialog(t('desktop.confirm_delete_app'), t('desktop.confirm_delete_app_msg', { name }));
        if (!confirmed) return;
        try {
            await api('/api/desktop/apps?id=' + encodeURIComponent(appId), { method: 'DELETE' });
            removeIconPosition('app-' + appId);
            await loadBootstrap();
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err.message });
        }
    }

    async function deleteWidget(id) {
        if (!id) return;
        const confirmed = await confirmDialog(t('desktop.context_remove_widget'), t('desktop.confirm_delete_msg', { path: id }));
        if (!confirmed) return;
        try {
            await api('/api/desktop/widgets?id=' + encodeURIComponent(id), { method: 'DELETE' });
            await loadBootstrap();
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err.message });
        }
    }

    async function setWidgetVisible(id, visible) {
        if (!id) return;
        try {
            await api('/api/desktop/widgets?id=' + encodeURIComponent(id), {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ visible })
            });
            await loadBootstrap();
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err.message });
        }
    }

    async function setAppVisibility(id, patch) {
        if (!id || !patch) return;
        try {
            await api('/api/desktop/apps?id=' + encodeURIComponent(id), {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(patch)
            });
            await loadBootstrap();
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err.message });
        }
    }

    async function showWidgetManager() {
        closeContextMenu();
        const boot = state.bootstrap || {};
        const allWidgets = boot.all_widgets || [];
        const overlay = document.createElement('div');
        overlay.className = 'vd-modal-backdrop vd-widget-manager-backdrop';

        function renderCards() {
            const currentWidgets = (state.bootstrap && state.bootstrap.all_widgets) || [];
            return currentWidgets.map(widget => {
                const isOnDesktop = widget.visible !== false;
                const isBuiltin = widget.builtin === true;
                const statusBadge = isOnDesktop
                    ? `<span class="vd-wm-badge vd-wm-badge-on">${esc(t('desktop.widget_on_desktop'))}</span>`
                    : `<span class="vd-wm-badge vd-wm-badge-off">${esc(t('desktop.widget_available'))}</span>`;
                const builtinBadge = isBuiltin
                    ? `<span class="vd-wm-badge vd-wm-badge-builtin">${esc(t('desktop.widget_builtin'))}</span>`
                    : '';
                const actions = [];
                if (isOnDesktop) {
                    actions.push(`<button type="button" class="vd-wm-btn vd-wm-btn-remove" data-action="hide" data-id="${esc(widget.id)}">${esc(t('desktop.widget_remove_from_desktop'))}</button>`);
                } else {
                    actions.push(`<button type="button" class="vd-wm-btn vd-wm-btn-add" data-action="show" data-id="${esc(widget.id)}">${esc(t('desktop.widget_add_to_desktop'))}</button>`);
                }
                if (!isBuiltin) {
                    actions.push(`<button type="button" class="vd-wm-btn vd-wm-btn-delete" data-action="delete" data-id="${esc(widget.id)}">${esc(t('desktop.widget_delete_permanent'))}</button>`);
                }
                const iconKey = widget.icon || 'widgets';
                return `<div class="vd-wm-card${isBuiltin ? ' vd-wm-card-builtin' : ''}" data-widget-id="${esc(widget.id)}">
                    <div class="vd-wm-card-head">
                        ${iconMarkup(iconKey, widget.title || widget.id, 'vd-sprite-file', 24)}
                        <div class="vd-wm-card-info">
                            <div class="vd-wm-card-title">${esc(widget.title || widget.id)}</div>
                            <div class="vd-wm-card-badges">${statusBadge}${builtinBadge}</div>
                        </div>
                    </div>
                    <div class="vd-wm-card-actions">${actions.join('')}</div>
                </div>`;
            }).join('');
        }

        overlay.innerHTML = `<div class="vd-widget-manager" role="dialog" aria-modal="true">
            <div class="vd-wm-header">
                <div class="vd-wm-title">${esc(t('desktop.widget_manager'))}</div>
                <div class="vd-window-actions">
                    <button type="button" class="vd-window-button" data-action="close" data-close title="${esc(t('desktop.close'))}" aria-label="${esc(t('desktop.close'))}"></button>
                </div>
            </div>
            <div class="vd-wm-cards">${renderCards()}</div>
        </div>`;
        document.body.appendChild(overlay);

        async function refresh() {
            await loadBootstrap();
            overlay.querySelector('.vd-wm-cards').innerHTML = renderCards();
            wireButtons();
        }

        function wireButtons() {
            overlay.querySelectorAll('.vd-wm-btn[data-action="show"]').forEach(btn => {
                btn.addEventListener('click', () => setWidgetVisible(btn.dataset.id, true).then(refresh));
            });
            overlay.querySelectorAll('.vd-wm-btn[data-action="hide"]').forEach(btn => {
                btn.addEventListener('click', () => setWidgetVisible(btn.dataset.id, false).then(refresh));
            });
            overlay.querySelectorAll('.vd-wm-btn[data-action="delete"]').forEach(btn => {
                btn.addEventListener('click', async () => {
                    const widget = ((state.bootstrap && state.bootstrap.all_widgets) || []).find(w => w.id === btn.dataset.id);
                    const name = widget ? (widget.title || widget.id) : btn.dataset.id;
                    const confirmed = await confirmDialog(t('desktop.widget_confirm_delete'), t('desktop.widget_confirm_delete_msg', { name }));
                    if (!confirmed) return;
                    try {
                        await api('/api/desktop/widgets?id=' + encodeURIComponent(btn.dataset.id), { method: 'DELETE' });
                        await refresh();
                    } catch (err) {
                        showDesktopNotification({ title: t('desktop.notification'), message: err.message });
                    }
                });
            });
        }

        function close() { overlay.remove(); }
        overlay.querySelector('[data-close]').addEventListener('click', close);
        overlay.querySelector('.vd-window-actions').addEventListener('click', event => event.stopPropagation());
        overlay.addEventListener('click', event => { if (event.target === overlay) close(); });
        wireButtons();
    }

    async function showAppManager() {
        closeContextMenu();
        const overlay = document.createElement('div');
        overlay.className = 'vd-modal-backdrop vd-app-manager-backdrop';

        function renderCards() {
            return userFacingApps().map(app => {
                const inDock = app.dock_visible !== false;
                const inStart = app.start_visible !== false;
                const isBuiltin = app.builtin === true || isBuiltinApp(app.id);
                const canDelete = app.deletable === true && !isBuiltin;
                const dockBadge = inDock
                    ? `<span class="vd-wm-badge vd-wm-badge-on vd-wm-badge-dock">${esc(t('desktop.app_in_dock'))}</span>`
                    : `<span class="vd-wm-badge vd-wm-badge-off vd-wm-badge-dock">${esc(t('desktop.app_hidden_from_dock'))}</span>`;
                const startBadge = inStart
                    ? `<span class="vd-wm-badge vd-wm-badge-on vd-wm-badge-start">${esc(t('desktop.app_in_start'))}</span>`
                    : `<span class="vd-wm-badge vd-wm-badge-off vd-wm-badge-start">${esc(t('desktop.app_hidden_from_start'))}</span>`;
                const sourceBadge = isBuiltin
                    ? `<span class="vd-wm-badge vd-wm-badge-builtin">${esc(t('desktop.app_system'))}</span>`
                    : `<span class="vd-wm-badge vd-wm-badge-user">${esc(t('desktop.app_generated'))}</span>`;
                const actions = [];
                if (inDock) {
                    actions.push(`<button type="button" class="vd-wm-btn vd-wm-btn-remove" data-action="hide-dock" data-id="${esc(app.id)}">${esc(t('desktop.app_remove_from_dock'))}</button>`);
                } else {
                    actions.push(`<button type="button" class="vd-wm-btn vd-wm-btn-add" data-action="show-dock" data-id="${esc(app.id)}">${esc(t('desktop.app_add_to_dock'))}</button>`);
                }
                if (inStart) {
                    actions.push(`<button type="button" class="vd-wm-btn vd-wm-btn-remove" data-action="hide-start" data-id="${esc(app.id)}">${esc(t('desktop.app_remove_from_start'))}</button>`);
                } else {
                    actions.push(`<button type="button" class="vd-wm-btn vd-wm-btn-add" data-action="show-start" data-id="${esc(app.id)}">${esc(t('desktop.app_add_to_start'))}</button>`);
                }
                if (canDelete) {
                    actions.push(`<button type="button" class="vd-wm-btn vd-wm-btn-delete" data-action="delete" data-id="${esc(app.id)}">${esc(t('desktop.app_delete_permanent'))}</button>`);
                }
                return `<div class="vd-wm-card${isBuiltin ? ' vd-wm-card-builtin' : ''}" data-app-id="${esc(app.id)}">
                    <div class="vd-wm-card-head">
                        ${iconMarkup(iconForApp(app), iconGlyph(app), 'vd-sprite-file', 24)}
                        <div class="vd-wm-card-info">
                            <div class="vd-wm-card-title">${esc(appName(app))}</div>
                            <div class="vd-wm-card-badges">${dockBadge}${startBadge}${sourceBadge}</div>
                        </div>
                    </div>
                    <div class="vd-wm-card-actions">${actions.join('')}</div>
                </div>`;
            }).join('');
        }

        overlay.innerHTML = `<div class="vd-widget-manager vd-app-manager" role="dialog" aria-modal="true">
            <div class="vd-wm-header">
                <div class="vd-wm-title">${esc(t('desktop.app_manager'))}</div>
                <div class="vd-window-actions">
                    <button type="button" class="vd-window-button" data-action="close" data-close title="${esc(t('desktop.close'))}" aria-label="${esc(t('desktop.close'))}"></button>
                </div>
            </div>
            <div class="vd-wm-cards">${renderCards()}</div>
        </div>`;
        document.body.appendChild(overlay);

        async function refresh() {
            await loadBootstrap();
            overlay.querySelector('.vd-wm-cards').innerHTML = renderCards();
            wireButtons();
        }

        function wireButtons() {
            overlay.querySelectorAll('.vd-wm-btn[data-action="hide-dock"]').forEach(btn => {
                btn.addEventListener('click', () => setAppVisibility(btn.dataset.id, { dock_visible: false }).then(refresh));
            });
            overlay.querySelectorAll('.vd-wm-btn[data-action="show-dock"]').forEach(btn => {
                btn.addEventListener('click', () => setAppVisibility(btn.dataset.id, { dock_visible: true }).then(refresh));
            });
            overlay.querySelectorAll('.vd-wm-btn[data-action="hide-start"]').forEach(btn => {
                btn.addEventListener('click', () => setAppVisibility(btn.dataset.id, { start_visible: false }).then(refresh));
            });
            overlay.querySelectorAll('.vd-wm-btn[data-action="show-start"]').forEach(btn => {
                btn.addEventListener('click', () => setAppVisibility(btn.dataset.id, { start_visible: true }).then(refresh));
            });
            overlay.querySelectorAll('.vd-wm-btn[data-action="delete"]').forEach(btn => {
                btn.addEventListener('click', async () => {
                    const app = allApps().find(candidate => candidate.id === btn.dataset.id);
                    const name = app ? appName(app) : btn.dataset.id;
                    const confirmed = await confirmDialog(t('desktop.confirm_delete_app'), t('desktop.confirm_delete_app_msg', { name }));
                    if (!confirmed) return;
                    try {
                        await api('/api/desktop/apps?id=' + encodeURIComponent(btn.dataset.id), { method: 'DELETE' });
                        await refresh();
                    } catch (err) {
                        showDesktopNotification({ title: t('desktop.notification'), message: err.message });
                    }
                });
            });
        }

        function close() { overlay.remove(); }
        overlay.querySelector('[data-close]').addEventListener('click', close);
        overlay.querySelector('.vd-window-actions').addEventListener('click', event => event.stopPropagation());
        overlay.addEventListener('click', event => { if (event.target === overlay) close(); });
        wireButtons();
    }

    function showProperties(title, body) {
        showDesktopNotification({ title: title || t('desktop.context_properties'), message: body || '' });
    }

    function contentEl(id) {
        const win = state.windows.get(id);
        return win && win.element.querySelector('[data-window-content]');
    }

    function menuLabel(item) {
        const fallback = item && (item.label || item.id || '');
        if (!item || !item.labelKey) return fallback;
        const translated = t(item.labelKey);
        return translated && translated !== item.labelKey ? translated : fallback;
    }

    function desktopText(key, fallback) {
        const translated = t(key);
        return translated && translated !== key ? translated : fallback;
    }

    function normalizeWindowMenuItems(items, menuId, actions, path) {
        return (Array.isArray(items) ? items : []).map((item, index) => {
            if (!item || item.hidden) return null;
            if (item.type === 'separator' || item.separator) return { type: 'separator' };
            const id = String(item.id || item.actionId || item.action || ('item-' + index));
            const actionKey = path.concat(id).join('/');
            const submenuItems = item.items || item.children;
            const normalized = {
                type: submenuItems ? 'submenu' : 'item',
                id,
                label: item.label || '',
                labelKey: item.labelKey || '',
                icon: item.icon || '',
                fallback: item.fallback || '',
                shortcut: item.shortcut || '',
                disabled: typeof item.disabled === 'function' ? !!item.disabled() : !!item.disabled,
                checked: typeof item.checked === 'function' ? !!item.checked() : !!item.checked,
                actionKey: ''
            };
            if (submenuItems) {
                normalized.items = normalizeWindowMenuItems(submenuItems, menuId, actions, path.concat(id));
            } else if (typeof item.action === 'function') {
                normalized.actionKey = actionKey;
                actions.set(actionKey, item.action);
            }
            return normalized;
        }).filter(Boolean);
    }

    function normalizeWindowMenus(windowId, rawMenus, actions) {
        return (Array.isArray(rawMenus) ? rawMenus : [])
            .concat(automaticWindowMenu(windowId))
            .filter(menu => menu && !menu.hidden)
            .map((menu, index) => {
                const id = String(menu.id || ('menu-' + index));
                return {
                    id,
                    label: menu.label || '',
                    labelKey: menu.labelKey || '',
                    items: normalizeWindowMenuItems(menu.items || [], id, actions, [windowId, id])
                };
            })
            .filter(menu => menu.items.length);
    }

    function automaticWindowMenu(windowId) {
        const item = state.windows.get(windowId);
        const hasMaximize = !!(item && item.element && item.element.querySelector('[data-action="maximize"]'));
        return {
            id: 'window',
            labelKey: 'desktop.menu_window',
            items: [
                { id: 'minimize', labelKey: 'desktop.menu_minimize_window', icon: 'minus', action: () => minimizeWindow(windowId) },
                {
                    id: 'maximize',
                    labelKey: item && item.maximized ? 'desktop.menu_restore_window' : 'desktop.menu_maximize_window',
                    icon: 'maximize',
                    disabled: !hasMaximize,
                    action: () => hasMaximize && toggleMaximizeWindow(windowId)
                },
                { type: 'separator' },
                { id: 'close', labelKey: 'desktop.menu_close_window', icon: 'x', shortcut: 'Alt+F4', action: () => closeWindow(windowId) }
            ]
        };
    }

    function renderWindowMenuItems(items) {
        return (items || []).map(item => {
            if (item.type === 'separator') return '<div class="vd-window-menu-separator" role="separator"></div>';
            const label = esc(menuLabel(item));
            const disabled = item.disabled ? ' disabled' : '';
            const checked = item.checked ? ' checked' : '';
            const icon = item.icon
                ? `<span class="vd-window-menu-icon">${iconMarkup(item.icon, item.fallback || item.icon, 'vd-window-menu-papirus-icon', 14)}</span>`
                : '<span class="vd-window-menu-icon empty"></span>';
            if (item.type === 'submenu') {
                return `<div class="vd-window-menu-submenu${disabled}" role="none">
                    <button type="button" class="vd-window-menu-item${checked}" role="menuitem" ${disabled ? 'disabled' : ''}>
                        ${icon}<span>${label}</span><span class="vd-window-menu-arrow">â€º</span>
                    </button>
                    <div class="vd-window-menu-popover" role="menu">${renderWindowMenuItems(item.items)}</div>
                </div>`;
            }
            return `<button type="button" class="vd-window-menu-item${checked}" role="menuitem" data-menu-action="${esc(item.actionKey)}" ${disabled}>
                ${icon}<span>${label}</span>${item.shortcut ? `<kbd>${esc(item.shortcut)}</kbd>` : '<kbd></kbd>'}
            </button>`;
        }).join('');
    }

    function setWindowMenus(windowId, menus) {
        if (!state.windows.has(windowId)) return;
        state.windowMenus.set(windowId, { rawMenus: Array.isArray(menus) ? menus : [], renderedMenus: [], actions: new Map() });
        renderWindowMenus(windowId);
    }

    function clearWindowMenus(windowId) {
        const win = state.windows.get(windowId);
        state.windowMenus.delete(windowId);
        if (!win || !win.element) return;
        win.element.classList.remove('has-window-menu');
        const bar = win.element.querySelector('.vd-window-menubar');
        if (bar) bar.remove();
        if (state.openWindowMenu && state.openWindowMenu.windowId === windowId) state.openWindowMenu = null;
    }

    function renderWindowMenus(windowId) {
        const win = state.windows.get(windowId);
        const record = state.windowMenus.get(windowId);
        if (!win || !win.element || !record) return;
        const actions = new Map();
        const menus = normalizeWindowMenus(windowId, record.rawMenus, actions);
        record.renderedMenus = menus;
        record.actions = actions;
        win.element.classList.toggle('has-window-menu', menus.length > 0);
        const titlebar = win.element.querySelector('.vd-window-titlebar');
        if (!titlebar) return;
        let bar = titlebar.querySelector('.vd-window-menubar');
        if (!menus.length) {
            if (bar) bar.remove();
            return;
        }
        if (!bar) {
            titlebar.insertAdjacentHTML('beforeend', '<nav class="vd-window-menubar" role="menubar"></nav>');
            bar = titlebar.querySelector('.vd-window-menubar');
        }
        bar.innerHTML = menus.map(menu => `<div class="vd-window-menu" data-menu-id="${esc(menu.id)}">
            <button type="button" class="vd-window-menu-button" role="menuitem" data-window-menu="${esc(menu.id)}">${esc(menuLabel(menu))}</button>
            <div class="vd-window-menu-popover" role="menu">${renderWindowMenuItems(menu.items)}</div>
        </div>`).join('');
        bar.querySelectorAll('[data-window-menu]').forEach(button => {
            const open = event => toggleWindowMenu(event, windowId, button.dataset.windowMenu);
            button.addEventListener('click', open);
            button.addEventListener('mouseenter', event => {
                if (state.openWindowMenu && state.openWindowMenu.windowId === windowId) open(event);
            });
        });
        bar.querySelectorAll('[data-menu-action]').forEach(button => {
            button.addEventListener('click', event => {
                event.preventDefault();
                event.stopPropagation();
                if (button.disabled) return;
                runWindowMenuAction(windowId, button.dataset.menuAction);
            });
        });
    }

    function toggleWindowMenu(event, windowId, menuId) {
        event.preventDefault();
        event.stopPropagation();
        focusWindow(windowId);
        const win = state.windows.get(windowId);
        const menu = win && win.element.querySelector(`.vd-window-menu[data-menu-id="${cssSel(menuId)}"]`);
        if (!menu) return;
        const isOpen = menu.classList.contains('open');
        closeWindowMenu();
        if (isOpen) return;
        menu.classList.add('open');
        state.openWindowMenu = { windowId, menuId };
    }

    function runWindowMenuAction(windowId, actionKey) {
        const record = state.windowMenus.get(windowId);
        const action = record && record.actions && record.actions.get(actionKey);
        closeWindowMenu();
        if (typeof action === 'function') action();
    }

    function flattenWindowMenuItems(menus) {
        const flat = [];
        (menus || []).forEach(menu => {
            (menu.items || []).forEach(function visit(item) {
                if (!item || item.type === 'separator') return;
                if (item.type === 'submenu') {
                    (item.items || []).forEach(visit);
                    return;
                }
                flat.push(item);
            });
        });
        return flat;
    }

    function editableShortcutTarget(event, shortcut) {
        const target = event.target;
        if (!target || !target.closest) return false;
        if (!target.closest('input, textarea, select, [contenteditable="true"]')) return false;
        const normalized = String(shortcut || '').toLowerCase();
        return !['ctrl+s', 'meta+s', 'f5', 'ctrl+=', 'meta+=', 'ctrl+-', 'meta+-', 'ctrl+0', 'meta+0'].includes(normalized);
    }

    function handleWindowMenuShortcut(event) {
        const windowId = state.activeWindowId;
        if (!windowId || event.defaultPrevented) return false;
        const record = state.windowMenus.get(windowId);
        if (!record) return false;
        for (const item of flattenWindowMenuItems(record.renderedMenus || [])) {
            if (!item.shortcut || item.disabled || !item.actionKey) continue;
            if (editableShortcutTarget(event, item.shortcut)) continue;
            if (!shortcutMatches(event, item.shortcut)) continue;
            event.preventDefault();
            event.stopPropagation();
            runWindowMenuAction(windowId, item.actionKey);
            return true;
        }
        return false;
    }

    function shortcutMatches(event, shortcut) {
        const parts = String(shortcut || '').split('+').map(part => part.trim().toLowerCase()).filter(Boolean);
        if (!parts.length) return false;
        const wantCtrl = parts.includes('ctrl') || parts.includes('control');
        const wantMeta = parts.includes('meta') || parts.includes('cmd') || parts.includes('command');
        const wantAlt = parts.includes('alt') || parts.includes('option');
        const wantShift = parts.includes('shift');
        const key = parts.find(part => !['ctrl', 'control', 'meta', 'cmd', 'command', 'alt', 'option', 'shift'].includes(part));
        if (!key) return false;
        const allowsImplicitShift = !wantShift && (key === '=' || key === '+');
        if (wantCtrl && !(event.ctrlKey || event.metaKey)) return false;
        if (wantMeta && !event.metaKey) return false;
        if (!wantCtrl && !wantMeta && (event.ctrlKey || event.metaKey)) return false;
        if (wantAlt !== !!event.altKey) return false;
        if (wantShift !== !!event.shiftKey && !(allowsImplicitShift && event.shiftKey)) return false;
        const eventKey = String(event.key || '').toLowerCase();
        const eventCode = String(event.code || '').toLowerCase();
        const aliases = { del: 'delete', esc: 'escape', space: ' ', return: 'enter' };
        const wanted = aliases[key] || key;
        return shortcutKeyMatches(eventKey, eventCode, wanted);
    }

    function renderAppContent(id, appId, context) {
        if (!contentEl(id)) return;
        clearWindowMenus(id);
        clearDesktopFileWindowDropState(id);
        wireDesktopFileWindowDrop(id);
        try {
        const modules = window.AuraDesktopModules;
        if (modules && typeof modules.appAssetsReady === 'function' && !modules.appAssetsReady(appId)) {
            const host = contentEl(id);
            if (host) host.innerHTML = `<div class="vd-empty">${esc(t('desktop.loading'))}</div>`;
            modules.loadAppAssets(appId)
                .then(() => renderAppContent(id, appId, context))
                .catch(err => renderAppError(id, appId, err));
            return;
        }
        if (appId === 'files') {
            const path = Object.prototype.hasOwnProperty.call(context || {}, 'path')
                ? (context.path || '')
                : (settingValue('files.default_folder') || '');
            const item = state.windows.get(id);
            if (item) {
                const subtitle = item.element.querySelector('.vd-window-subtitle');
                if (subtitle) subtitle.textContent = path || t('desktop.workspace_root');
            }
            return renderFiles(id, path);
        }
        if (appId === 'editor') return renderEditor(id, context.path || 'Documents/untitled.txt', context.content || '');
        if (appId === 'writer' && window.WriterApp && typeof window.WriterApp.render === 'function') {
            return window.WriterApp.render(contentEl(id), id, officeAppContext(context));
        }
        if (appId === 'sheets' && window.SheetsApp && typeof window.SheetsApp.render === 'function') {
            return window.SheetsApp.render(contentEl(id), id, officeAppContext(context));
        }
        if (appId === 'settings') return renderSettings(id);
        if (appId === 'calendar') return renderCalendar(id);
        if (appId === 'calculator') return renderCalculator(id);
        if (appId === 'todo') return renderTodo(id);
        if (appId === 'gallery') return renderGallery(id);
        if (appId === 'music-player') return renderMusicPlayer(id);
        if (appId === 'radio' && window.RadioApp && typeof window.RadioApp.render === 'function') {
            return window.RadioApp.render(contentEl(id), id, Object.assign({}, context || {}, { esc, t, iconMarkup, setWindowMenus, clearWindowMenus, showContextMenu, wireContextMenuBoundary }));
        }
if (appId === 'system-info') {
            if (!window.SystemInfoApp) {
                window.AuraDesktopModules.loadAppScript('system-info').then(() => renderAppContent(id, appId, context)).catch(err => renderAppError(id, appId, err));
                return;
            }
            if (typeof window.SystemInfoApp.render === 'function') return window.SystemInfoApp.render(contentEl(id), id, Object.assign({}, context || {}, {esc, t, iconMarkup}));
        }
        if (appId === 'agent-chat') return window.AgentChatApp && typeof window.AgentChatApp.render === 'function' ? window.AgentChatApp.render(id, Object.assign({}, context || {}, { __desktopRuntime: { contentEl, esc, desktopText, iconMarkup, api, loadBootstrap, showDesktopNotification } })) : renderAppError(id, appId, new Error('Agent chat renderer is not loaded'));
        if (appId === 'viewer' && window.ViewerApp && typeof window.ViewerApp.render === 'function') {
            return window.ViewerApp.render(contentEl(id), id, viewerAppContext(context));
        }
        if (appId === 'viewer-3d') {
            if (!window.Viewer3DApp) {
                window.AuraDesktopModules.loadAppScript('viewer-3d').then(() => renderAppContent(id, appId, context)).catch(err => renderAppError(id, appId, err));
                return;
            }
            if (typeof window.Viewer3DApp.render === 'function') return window.Viewer3DApp.render(contentEl(id), id, withDesktopFileDialogs(context, { esc, api, t, iconMarkup, notify: showDesktopNotification, setWindowMenus, clearWindowMenus, wireContextMenuBoundary, openApp }));
        } if (appId === 'quick-connect') return renderQuickConnect(id);
        if (appId === 'code-studio' && window.CodeStudio && typeof window.CodeStudio.render === 'function') {
            return window.CodeStudio.render(contentEl(id), id, withDesktopFileDialogs(context, { iconMarkup, setWindowMenus, clearWindowMenus, wireContextMenuBoundary }));
        }
        if (appId === 'launchpad') return renderLaunchpad(id); if (appId === 'software-store' && window.SoftwareStoreApp && typeof window.SoftwareStoreApp.render === 'function') return window.SoftwareStoreApp.render(contentEl(id), id, Object.assign({}, context || {}, { esc, api, t, iconMarkup, notify: showDesktopNotification, loadBootstrap, openApp, setWindowMenus, clearWindowMenus, wireContextMenuBoundary }));
        if (appId === 'looper' && window.LooperApp && typeof window.LooperApp.render === 'function') {
            return window.LooperApp.render(contentEl(id), id, Object.assign({}, context || {}, { esc, api, t, iconMarkup, notify: showDesktopNotification, readonly: desktopReadonly(), loadBootstrap, updateWindowContext, setWindowMenus, clearWindowMenus, wireContextMenuBoundary }));
        }
        if (appId === 'cheater' && window.CheaterApp && typeof window.CheaterApp.render === 'function') {
            return window.CheaterApp.render(contentEl(id), id, Object.assign({}, context || {}, { esc, t, iconMarkup, notify: showDesktopNotification, readonly: desktopReadonly(), loadBootstrap, openApp, setWindowMenus, clearWindowMenus, wireContextMenuBoundary, updateWindowContext }));
        }
        if (appId === 'camera') {
            if (!window.CameraApp) {
                window.AuraDesktopModules.loadAppScript('camera').then(() => renderAppContent(id, appId, context)).catch(err => renderAppError(id, appId, err));
                return;
            }
            if (typeof window.CameraApp.render === 'function') return window.CameraApp.render(contentEl(id), id, Object.assign({}, context || {}, {esc, api, t, iconMarkup, notify: showDesktopNotification, readonly: desktopReadonly(), loadBootstrap, setWindowMenus, clearWindowMenus}));
        }
if (appId === 'zipper') {
            if (!window.ZipperApp) {
                window.AuraDesktopModules.loadAppScript('zipper').then(() => renderAppContent(id, appId, context)).catch(err => renderAppError(id, appId, err));
                return;
            }
            if (typeof window.ZipperApp.render === 'function') return window.ZipperApp.render(contentEl(id), id, Object.assign({}, context || {}, { esc, api, t, iconMarkup, notify: showDesktopNotification, setWindowMenus, clearWindowMenus, wireContextMenuBoundary, promptDialog, openFileDialog: options => openDesktopFileDialog(options || {}), openApp, loadBootstrap, fileOps: window.AuraDesktopFileOps }));
        }
if (appId === 'pixel') {
            if (!window.PixelApp) {
                window.AuraDesktopModules.loadAppScript('pixel').then(() => renderAppContent(id, appId, context)).catch(err => renderAppError(id, appId, err));
                return;
            }
            if (typeof window.PixelApp.render === 'function') return window.PixelApp.render(contentEl(id), id, withDesktopFileDialogs(context, {
                esc, api, t, iconMarkup,
                notify: showDesktopNotification,
                readonly: desktopReadonly(),
                loadBootstrap,
                setWindowMenus,
                clearWindowMenus,
                wireContextMenuBoundary,
                promptDialog,
                openApp,
                fileOps: window.AuraDesktopFileOps
            }));
        }
        if (appId === 'galaxa-deluxe') {
            if (!window.GalaxaDeluxe) {
                window.AuraDesktopModules.loadAppScript('galaxa-deluxe').then(() => renderAppContent(id, appId, context)).catch(err => renderAppError(id, appId, err));
                return;
            }
            if (typeof window.GalaxaDeluxe.render === 'function') return window.GalaxaDeluxe.render(contentEl(id), id, Object.assign({}, context || {}, { esc, api, t, iconMarkup, notify: showDesktopNotification }));
        }
        if (appId === 'people') {
            if (!window.PeopleApp) {
                window.AuraDesktopModules.loadAppScript('people').then(() => renderAppContent(id, appId, context)).catch(err => renderAppError(id, appId, err));
                return;
            }
            if (typeof window.PeopleApp.render === 'function') return window.PeopleApp.render(contentEl(id), id, Object.assign({}, context || {}, { esc, api, t, iconMarkup, notify: showDesktopNotification }));
        }
        if (appId === 'mission-control' && window.MissionControlApp && typeof window.MissionControlApp.render === 'function') {
            return window.MissionControlApp.render(contentEl(id), id, Object.assign({}, context || {}, {
                esc, api, t, iconMarkup, notify: showDesktopNotification,
                readonly: desktopReadonly(), loadBootstrap, updateWindowContext,
                setWindowMenus, clearWindowMenus, wireContextMenuBoundary,
                confirmDialog, promptDialog
            }));
        }
        return renderGeneratedApp(id, appId);
        } catch (err) { renderAppError(id, appId, err); return undefined; }
    }

    async function renderFiles(id, path) {
        const host = contentEl(id);
        if (!host) return;
        state.filesPath = path || '';
        if (window.FileManager && typeof window.FileManager.render === 'function') {
            window.FileManager.render(host, id, state.filesPath, Object.assign(desktopFileDialogContext(), {
                esc,
                api,
                t,
                fmtBytes,
                iconMarkup,
                iconForFile,
                iconForDirectory,
                showContextMenu,
                closeContextMenu,
                promptDialog,
                confirmDialog,
                showNotification: showDesktopNotification,
                readonly: !!((state.bootstrap || {}).readonly),
                maxFileSize: Number((((state.bootstrap || {}).workspace || {}).max_file_size) || 0),
                setWindowMenus,
                clearWindowMenus,
                wireContextMenuBoundary,
                openFile: (entry) => {
                    if (entry.name && /\.zip$/i.test(entry.name)) return openApp('zipper', { path: entry.path });
                    if (isWriterFile(entry)) return openApp('writer', { path: entry.path });
                    if (isSheetsFile(entry)) return openApp('sheets', { path: entry.path }); if (is3DFile(entry)) return openApp('viewer-3d', { path: entry.path });
                    if (isPixelImageFile(entry)) return openApp('pixel', { path: entry.path });
                    if (isViewerFile(entry)) return openApp('viewer', { path: entry.path });
                    if (entry.web_path || entryLooksPlayableMedia(entry)) return openMediaPreview(entry);
                    openEditorFile(entry.path);
                },
                openMedia: (entry) => openMediaPreview(entry),
                openApp: (appId, ctx) => openApp(appId, ctx),
                addFileToChat: (entry) => addFileContextToChat(entry),
                askAgentAboutFile: (entry) => askAgentAboutFile(entry),
                refreshDesktop: loadBootstrap,
                onPathChange: (newPath) => {
                    state.filesPath = newPath;
                    const item = state.windows.get(id);
                    if (item) {
                        const subtitle = item.element.querySelector('.vd-window-subtitle');
                        if (subtitle) subtitle.textContent = newPath || t('desktop.workspace_root');
                    }
                },
                directories: (state.bootstrap && state.bootstrap.workspace && state.bootstrap.workspace.directories) || []
            }));
            return;
        }
        // Fallback: old file browser if FileManager module is not loaded
        host.innerHTML = `<div class="vd-panel">
            <div class="vd-toolbar">
                <button class="vd-tool-button" type="button" data-action="up">${iconMarkup('arrow-up', 'U', 'vd-tool-icon', 15)}<span>${esc(t('desktop.up'))}</span></button>
                <button class="vd-tool-button" type="button" data-action="new-file">${iconMarkup('file-plus', '+', 'vd-tool-icon', 15)}<span>${esc(t('desktop.new_file'))}</span></button>
                <button class="vd-tool-button" type="button" data-action="new-folder">${iconMarkup('folder-plus', '+', 'vd-tool-icon', 15)}<span>${esc(t('desktop.new_folder'))}</span></button>
                <span class="vd-path">${esc(state.filesPath || t('desktop.workspace_root'))}</span>
            </div>
            <div class="vd-file-list">${esc(t('desktop.loading'))}</div>
        </div>`;
        host.querySelector('[data-action="up"]').addEventListener('click', () => {
            const parts = state.filesPath.split('/').filter(Boolean);
            parts.pop();
            renderFiles(id, parts.join('/'));
        });
        host.querySelector('[data-action="new-file"]').addEventListener('click', () => openApp('editor', { path: workspaceJoinPath(state.filesPath, 'untitled.txt'), content: '' }));
        host.querySelector('[data-action="new-folder"]').addEventListener('click', () => createFolderInPath(state.filesPath));
        setFallbackFileMenus(id, state.filesPath);
        try {
            const body = await api('/api/desktop/files?path=' + encodeURIComponent(state.filesPath));
