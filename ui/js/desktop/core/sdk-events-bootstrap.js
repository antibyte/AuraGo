            if (!item || item.hidden) return null;
            if (item.type === 'separator' || item.separator) return { type: 'separator' };
            const actionId = item.actionId || (typeof item.action === 'string' ? item.action : '') || item.id || '';
            const submenuItems = item.items || item.children;
            const normalized = {
                id: item.id || actionId,
                label: item.label || '',
                labelKey: item.labelKey || '',
                icon: item.icon || '',
                fallback: item.fallback || '',
                shortcut: item.shortcut || '',
                disabled: !!item.disabled,
                checked: !!item.checked
            };
            if (submenuItems) {
                normalized.items = sdkMenuItems(client, submenuItems);
            } else if (actionId) {
                normalized.action = () => postSDKMenuAction(client.windowId, actionId);
            }
            return normalized;
        }).filter(Boolean);
    }

    function sdkMenus(client, menus) {
        return (Array.isArray(menus) ? menus : []).map(menu => ({
            id: menu && menu.id || '',
            label: menu && menu.label || '',
            labelKey: menu && menu.labelKey || '',
            items: sdkMenuItems(client, menu && menu.items)
        }));
    }

    function sdkContextMenuItems(client, items) {
        return (Array.isArray(items) ? items : []).map(item => {
            if (!item || item.hidden) return null;
            if (item.type === 'separator' || item.separator) return { type: 'separator' };
            const actionId = item.actionId || (typeof item.action === 'string' ? item.action : '') || item.id || '';
            const normalized = {
                id: item.id || actionId,
                label: item.label || '',
                labelKey: item.labelKey || '',
                icon: item.icon || '',
                fallback: item.fallback || '',
                shortcut: item.shortcut || '',
                disabled: !!item.disabled,
                checked: !!item.checked
            };
            const submenuItems = item.items || item.children;
            if (submenuItems) {
                normalized.items = sdkContextMenuItems(client, submenuItems);
            } else if (actionId) {
                normalized.action = () => postSDKContextMenuAction(client, actionId);
            }
            return normalized;
        }).filter(Boolean);
    }

    function declaredPermissions(client) {
        const appPermissions = (client.app && client.app.permissions) || [];
        const widgetPermissions = (client.widget && client.widget.permissions) || [];
        return new Set([...appPermissions, ...widgetPermissions].map(item => String(item).toLowerCase().trim()).filter(Boolean));
    }

    function hasPermission(client, permission) {
        if (!permission) return true;
        const permissions = declaredPermissions(client);
        const normalized = String(permission).toLowerCase();
        const prefix = normalized.includes(':') ? normalized.split(':')[0] + ':*' : '';
        return permissions.has('*') || permissions.has(normalized) || (prefix && permissions.has(prefix));
    }

    function requirePermission(client, permissions) {
        const required = Array.isArray(permissions) ? permissions : [permissions];
        if (required.some(permission => hasPermission(client, permission))) return;
        throw new Error('Permission denied: ' + required.join(' or '));
    }

    async function handleSDKMessage(event) {
        const msg = event.data;
        if (!msg || msg.type !== SDK_REQUEST_TYPE) return;
        const client = findSDKClient(event.source);
        if (!client || (!client.app && msg.action !== 'desktop:widget:resize')) return;
        try {
            const result = await runSDKAction(client, msg.action, msg.payload || {});
            sendSDKResponse(event.source, msg.id, true, result);
        } catch (err) {
            sendSDKResponse(event.source, msg.id, false, err);
        }
    }

    async function runSDKAction(client, action, payload) {
        switch (action) {
            case 'desktop:context':
                return {
                    runtime: SDK_RUNTIME,
                    app: client.app,
                    widget: client.widget || null,
                    bootstrap: sdkBootstrap(),
                    icon_manifest: state.iconManifest,
                    icon_theme_manifests: state.iconThemeManifests
                };
            case 'desktop:widget:resize':
                if (!client.widgetId) throw new Error('Widget resize is only available inside widget frames.');
                resizeWidgetToContent(client.widgetId, payload || {});
                return { status: 'ok' };
            case 'desktop:menu:set':
                if (!client.windowId) throw new Error('Menus are only available for app windows.');
                setWindowMenus(client.windowId, sdkMenus(client, payload.menus || []));
                return { status: 'ok' };
            case 'desktop:menu:clear':
                if (client.windowId) clearWindowMenus(client.windowId);
                return { status: 'ok' };
            case 'desktop:context-menu:show':
                showContextMenu(Number(payload.x) || 0, Number(payload.y) || 0, sdkContextMenuItems(client, payload.items || []));
                return { status: 'ok' };
            case 'desktop:context-menu:clear':
                closeContextMenu();
                return { status: 'ok' };
            case 'desktop:clipboard:read-text': {
                if (!navigator.clipboard || typeof navigator.clipboard.readText !== 'function') throw new Error('Clipboard read is not available.');
                return { text: await navigator.clipboard.readText() };
            }
            case 'desktop:clipboard:write-text':
                if (!navigator.clipboard || typeof navigator.clipboard.writeText !== 'function') throw new Error('Clipboard write is not available.');
                await navigator.clipboard.writeText(String(payload.text || ''));
                return { status: 'ok' };
            case 'fs:list':
                requirePermission(client, ['files:read', 'filesystem:read']);
                return api('/api/desktop/files?path=' + encodeURIComponent(payload.path || ''));
            case 'fs:read':
                requirePermission(client, ['files:read', 'filesystem:read']);
                return api('/api/desktop/file?path=' + encodeURIComponent(payload.path || ''));
            case 'fs:write':
                requirePermission(client, ['files:write', 'filesystem:write']);
                await api('/api/desktop/file', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ path: payload.path || '', content: payload.content || '' })
                });
                await loadBootstrap();
                return { status: 'ok' };
            case 'app:open':
                requirePermission(client, ['apps:open']);
                openApp(payload.app_id || payload.id || client.appId);
                return { status: 'ok' };
            case 'notification:show':
                requirePermission(client, ['notifications']);
                showDesktopNotification({ title: payload.title || client.app.name, message: payload.message || payload.content || '' });
                return { status: 'ok' };
            case 'widget:upsert': {
                requirePermission(client, ['widgets:write']);
                const widget = Object.assign({}, payload || {});
                if (!widget.app_id) widget.app_id = client.appId;
                if (!widget.icon && client.app && client.app.icon) widget.icon = client.app.icon;
                await api('/api/desktop/widgets', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(widget)
                });
                await loadBootstrap();
                return { status: 'ok' };
            }
            default:
                throw new Error('Unsupported desktop SDK action: ' + action);
        }
    }

    function sdkBootstrap() {
        const boot = state.bootstrap || {};
        const workspace = boot.workspace || {};
        const iconCatalog = Object.assign({}, boot.icon_catalog || {});
        if (boot.icon_catalog) iconCatalog.theme = settingValue('appearance.icon_theme');
        return {
            enabled: !!boot.enabled,
            readonly: !!boot.readonly,
            allow_generated_apps: !!boot.allow_generated_apps,
            allow_python_jobs: !!boot.allow_python_jobs,
            workspace: {
                directories: workspace.directories || [],
                max_file_size: workspace.max_file_size || 0
            },
            installed_apps: boot.installed_apps || [],
            widgets: boot.widgets || [],
            settings: boot.settings || {},
            icon_catalog: boot.icon_catalog ? iconCatalog : null
        };
    }

    function connectWS() {
        if (state.ws) state.ws.close();
        const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
        const ws = new WebSocket(proto + '//' + location.host + '/api/desktop/ws');
        state.ws = ws;
        ws.addEventListener('open', () => setWSState(true));
        ws.addEventListener('close', () => {
            setWSState(false);
            setTimeout(connectWS, 4000);
        });
        ws.addEventListener('message', async (event) => {
            let msg;
            try { msg = JSON.parse(event.data); } catch (_) { return; }
            handleDesktopEvent(msg.type === 'welcome' ? { type: 'welcome', payload: msg.payload } : msg);
        });
    }

    function setWSState(online) {
        $('vd-ws-state').dataset.state = online ? 'online' : 'offline';
    }

    async function handleDesktopEvent(event) {
        if (!event || !event.type) return;
        if (event.type === 'welcome') {
            state.bootstrap = event.payload || state.bootstrap;
            renderDesktop();
            return;
        }
        if (event.type === 'desktop_changed') {
            await loadBootstrap();
            return;
        }
        if (event.type === 'open_app' && event.payload && event.payload.app_id) {
            openApp(event.payload.app_id, event.payload.path ? { path: event.payload.path } : undefined);
            return;
        }
        if (event.type === 'notification') {
            showDesktopNotification(event.payload || {});
        }
    }

    function showDesktopNotification(payload) {
        const note = document.createElement('div');
        note.className = 'vd-widget';
        note.style.position = 'absolute';
        note.style.right = '18px';
        note.style.bottom = '72px';
        note.style.zIndex = '60';
        note.innerHTML = `<div class="vd-widget-title">${esc(payload.title || t('desktop.notification'))}</div>
            <div class="vd-widget-body">${esc(payload.message || '')}</div>`;
        $('vd-workspace').appendChild(note);
        setTimeout(() => note.remove(), 5500);
    }

    function updateClock() {
        $('vd-clock').textContent = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    }

    function wireChrome() {
        $('vd-start-button').addEventListener('click', toggleStartMenu);
        $('vd-agent-button').addEventListener('click', () => openApp('agent-chat'));
        $('vd-start-search').addEventListener('input', (event) => {
            state.startQuery = event.target.value;
            renderStartApps();
        });
        renderStartButtonIcon();
        document.addEventListener('click', (event) => {
            if (!event.target.closest('.vd-context-menu')) closeContextMenu();
            if (!event.target.closest('.vd-window-menubar')) closeWindowMenu();
            const menu = $('vd-start-menu');
            if (!menu.hidden && !menu.contains(event.target) && !event.target.closest('#vd-start-button')) {
                menu.hidden = true;
            }
        });
        const taskbarEl = document.querySelector('.vd-taskbar');
        if (taskbarEl) {
            taskbarEl.addEventListener('contextmenu', (event) => {
                if (event.target.closest('button, input, a, .vd-start-menu')) return;
                event.preventDefault();
                showContextMenu(event.clientX, event.clientY, [
                    { label: t('desktop.context_system_info'), icon: 'analytics', fallback: 'i', action: () => openApp('system-info') }
                ]);
            });
        }
        $('vd-workspace').addEventListener('contextmenu', showDesktopContextMenu);
        $('vd-workspace').addEventListener('click', event => {
            if (event.target === $('vd-workspace') || event.target === $('vd-icons')) selectDesktopIcon(null);
        });
        document.addEventListener('keydown', handleDesktopKeydown);
        if (window.AuraSSE && typeof window.AuraSSE.on === 'function') {
            window.AuraSSE.on('virtual_desktop_event', handleDesktopEvent);
        }
        window.addEventListener('message', handleSDKMessage);
    }

    function handleDesktopKeydown(event) {
        if (handleWindowMenuShortcut(event)) return;
        if (isEditableTarget(event.target)) return;
        if (event.ctrlKey && event.code === 'Space') {
            event.preventDefault();
            $('vd-start-button').click();
            return;
        }
        if (event.altKey && event.key === 'F4') {
            event.preventDefault();
            if (state.activeWindowId) closeWindow(state.activeWindowId);
            return;
        }
        if (event.altKey && event.key === 'Tab') {
            event.preventDefault();
            const wins = [...state.windows.values()];
            if (!wins.length) return;
            const index = wins.findIndex(win => win.id === state.activeWindowId);
            focusWindow(wins[(index + 1 + wins.length) % wins.length].id);
        }
        switch (event.key) {
        case 'Escape':
            closeContextMenu();
            closeWindowMenu();
            $('vd-start-menu').hidden = true;
            return;
        case 'Enter': {
            const icon = selectedDesktopIcon();
            if (icon) activateDesktopItem(icon);
            return;
        }
        case 'Delete': {
            const icon = selectedFileDirectoryIcon();
            if (icon) {
                event.preventDefault();
                deletePath(icon.dataset.path);
            }
            return;
        }
        case 'F2': {
            const icon = selectedFileDirectoryIcon();
            if (icon) {
                event.preventDefault();
                renamePath(icon.dataset.path);
            }
            return;
        }
        }
    }

    function selectedDesktopIcon() {
        if (!state.selectedIconId) return null;
        return document.querySelector(`.vd-icon[data-id="${cssSel(state.selectedIconId)}"]`);
    }

    function selectedFileDirectoryIcon() {
        const icon = selectedDesktopIcon();
        if (!icon || !icon.dataset.path) return null;
        return icon.dataset.kind === 'file' || icon.dataset.kind === 'directory' ? icon : null;
    }

    async function init() {
        ['vd-icons', 'vd-widgets', 'vd-window-layer', 'vd-taskbar-apps', 'vd-start-apps', 'vd-start-menu', 'vd-start-search', 'vd-ws-state', 'vd-clock', 'vd-workspace', 'vd-disabled'].forEach(id => { els[id] = $(id); });
        ensureDesktopRadialMenuAnchor();
        await loadIconManifest();
        bindViewportMetrics();
        wireChrome();
        document.addEventListener('focusin', ensureFocusedControlVisible);
        updateClock();
        setInterval(updateClock, 15000);
        await loadBootstrap();
        if (state.bootstrap && state.bootstrap.enabled) connectWS();
    }

    ensureDesktopRadialMenuAnchor();
    document.addEventListener('DOMContentLoaded', init);
})();
