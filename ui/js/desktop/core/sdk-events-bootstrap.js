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
            case 'dialog:open-file':
                requirePermission(client, ['files:read', 'filesystem:read']);
                return openDesktopFileDialog(payload || {});
            case 'dialog:save-file':
                requirePermission(client, ['files:write', 'filesystem:write']);
                return saveDesktopFileDialog(payload || {});
            case 'dialog:import-files':
                requirePermission(client, ['files:write', 'filesystem:write']);
                return importHostFiles(payload || {});
            case 'dialog:export-file':
                requirePermission(client, ['files:read', 'filesystem:read']);
                return exportWorkspaceFile(payload || {});
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

    let wsReconnectAttempts = 0;
    let wsReconnectDelay = 2000;
    let wsReconnectTimer = null;
    let wsGeneration = 0;
    const MAX_WS_RETRIES = 10;
    const WS_MAX_DELAY = 30000;

    function cleanupDesktopWS() {
        if (typeof state.wsCleanup === 'function') {
            try { state.wsCleanup(); } catch (_) {}
            state.wsCleanup = null;
        }
        if (state.ws) {
            try { state.ws.close(); } catch (_) {}
            state.ws = null;
        }
    }

    function cleanupDesktopShellRuntime() {
        if (state._clockTimer) {
            clearInterval(state._clockTimer);
            state._clockTimer = null;
        }
        cleanupDesktopWS();
        closeSIPPhoneShellRuntime();
        if (wsReconnectTimer) {
            clearTimeout(wsReconnectTimer);
            wsReconnectTimer = null;
        }
    }

    function connectWS() {
        if (wsReconnectTimer) {
            clearTimeout(wsReconnectTimer);
            wsReconnectTimer = null;
        }
        cleanupDesktopWS();
        const generation = ++wsGeneration;
        const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
        const ws = new WebSocket(proto + '//' + location.host + '/api/desktop/ws');
        state.ws = ws;

        function staleSocket() {
            if (generation !== wsGeneration || ws !== state.ws) return true;
            return false;
        }

        function onOpen() {
            if (generation !== wsGeneration || ws !== state.ws) return;
            wsReconnectAttempts = 0;
            wsReconnectDelay = 2000;
            setWSState(true);
        }

        function onClose() {
            if (staleSocket()) return;
            if (wsReconnectAttempts >= MAX_WS_RETRIES) {
                setWSState(false, true);
                return;
            }
            setWSState(false);
            wsReconnectTimer = setTimeout(() => {
                if (staleSocket()) return;
                wsReconnectAttempts++;
                wsReconnectDelay = Math.min(wsReconnectDelay * 2, WS_MAX_DELAY);
                connectWS();
            }, wsReconnectDelay);
        }

        function onMessage(event) {
            if (staleSocket()) return;
            let msg;
            try { msg = JSON.parse(event.data); } catch (_) { return; }
            try {
                handleDesktopEvent(msg.type === 'welcome' ? { type: 'welcome', payload: msg.payload } : msg);
            } catch (_) {}
        }

        ws.addEventListener('open', onOpen);
        ws.addEventListener('close', onClose);
        ws.addEventListener('message', onMessage);
        state.wsCleanup = () => {
            ws.removeEventListener('open', onOpen);
            ws.removeEventListener('close', onClose);
            ws.removeEventListener('message', onMessage);
        };
    }

    function setWSState(online, failed) {
        const dot = $('vd-ws-state');
        if (!dot) return;
        if (online) {
            dot.dataset.state = 'online';
            dot.title = '';
        } else if (failed) {
            dot.dataset.state = 'offline';
            dot.title = t('desktop.ws_connection_lost');
        } else {
            dot.dataset.state = 'reconnecting';
            dot.title = t('desktop.ws_reconnecting');
        }
    }

    async function handleDesktopEvent(event) {
        if (!event || !event.type) return;
        if (event.type === 'welcome') {
            state.bootstrap = event.payload || state.bootstrap;
            renderDesktop();
            refreshPetRuntime();
            return;
        }
        if (event.type === 'desktop_changed') {
            await loadBootstrap();
            return;
        }
        if (event.type === 'open_widget' && event.payload && event.payload.path) {
            openStandaloneWidget(event.payload.path, event.payload.widget_id, event.payload);
            return;
        }
        if (event.type === 'open_app' && event.payload && event.payload.app_id) {
            if (event.payload.path && isStandaloneWidgetPath(event.payload.path) && !appById(event.payload.app_id)) {
                openStandaloneWidget(event.payload.path, event.payload.widget_id || event.payload.app_id, event.payload);
                return;
            }
            openApp(event.payload.app_id, event.payload.path ? { path: event.payload.path } : undefined);
            return;
        }
        if (event.type === 'notification') {
            showDesktopNotification(event.payload || {});
            return;
        }
        if (event.type === 'pet_changed' || event.type === 'pet_reaction_changed' || event.type === 'pet_say' || event.type === 'pet_setting_changed') {
            if (window.PetRuntime && typeof window.PetRuntime.handleEvent === 'function') {
                window.PetRuntime.handleEvent(event);
            }
            return;
        }
    }

    function showDesktopNotification(payload) {
        pushNotificationRecord(payload || {});
        const container = document.getElementById('vd-toast-container');
        if (!container) return;
        const toast = document.createElement('div');
        toast.className = 'vd-toast';
        const title = esc(payload.title || t('desktop.notification'));
        const message = esc(payload.message || '');
        toast.innerHTML = `<div><div class="vd-toast-title">${title}</div>${message ? `<div class="vd-toast-message">${message}</div>` : ''}</div><button class="vd-toast-close" type="button" aria-label="${esc(t('desktop.close'))}">✕</button>`;
        container.appendChild(toast);
        toast.querySelector('.vd-toast-close').addEventListener('click', () => removeToast(toast));
        const duration = Number(payload.duration) || 5500;
        const timer = setTimeout(() => removeToast(toast), duration);
        toast._toastTimer = timer;
    }

    function removeToast(toast) {
        if (!toast || toast._toastRemoved) return;
        toast._toastRemoved = true;
        if (toast._toastTimer) clearTimeout(toast._toastTimer);
        if (animationsEnabled()) {
            toast.classList.add('vd-toast-closing');
            setTimeout(() => toast.remove(), 150);
        } else {
            toast.remove();
        }
    }

    function updateClock() {
        const value = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
        document.querySelectorAll('.vd-clock').forEach(clock => { clock.textContent = value; });
    }

    function wireChrome() {
        $('vd-start-button').addEventListener('click', toggleStartMenu);
        $('vd-agent-button').addEventListener('click', () => openApp('agent-chat'));
        const liveSpeechButton = $('vd-live-speech-button');
        if (liveSpeechButton) liveSpeechButton.addEventListener('click', () => openApp('live-speech'));
        const widgetDrawerBtn = document.getElementById('vd-widget-drawer-btn');
        if (widgetDrawerBtn) widgetDrawerBtn.addEventListener('click', toggleWidgetDrawer);
        const showDesktopBtn = document.getElementById('vd-show-desktop-btn');
        if (showDesktopBtn) showDesktopBtn.addEventListener('click', minimizeAllWindows);

        // Hide shortcuts and widgets buttons on mobile (they make little sense on phones)
        updateTaskbarSystemButtonsForMobile();
        // Re-evaluate on rotation / major viewport changes
        window.addEventListener('resize', () => {
            if (window.useMobileDesktopMode) updateTaskbarSystemButtonsForMobile();
        }, { passive: true });
        let startSearchTimer = null;
        $('vd-start-search').addEventListener('input', (event) => {
            state.startQuery = event.target.value;
            clearTimeout(startSearchTimer);
            startSearchTimer = setTimeout(renderStartApps, 150);
        });
        $('vd-start-menu').addEventListener('keydown', (event) => {
            if (event.key === 'Escape') {
                event.preventDefault();
                closeStartMenu();
                const startButton = $('vd-start-button');
                if (startButton) startButton.focus();
                return;
            }
            if (event.key.length === 1 && !event.ctrlKey && !event.metaKey && !event.altKey) {
                const search = $('vd-start-search');
                if (search && document.activeElement !== search) search.focus();
                return;
            }
            const items = [...$('vd-start-menu').querySelectorAll('.vd-start-item')];
            if (!items.length) return;
            const idx = items.indexOf(document.activeElement);
            const firstTop = items[0].offsetTop;
            let columns = 1;
            while (columns < items.length && items[columns].offsetTop === firstTop) columns++;
            let next = -1;
            if (event.key === 'ArrowDown') next = idx < 0 ? 0 : Math.min(items.length - 1, idx + columns);
            else if (event.key === 'ArrowUp') next = idx < 0 ? items.length - 1 : Math.max(0, idx - columns);
            else if (event.key === 'ArrowRight' && columns > 1) next = idx < 0 ? 0 : Math.min(items.length - 1, idx + 1);
            else if (event.key === 'ArrowLeft' && columns > 1) next = idx < 0 ? 0 : Math.max(0, idx - 1);
            else if (event.key === 'Home') next = 0;
            else if (event.key === 'End') next = items.length - 1;
            else return;
            event.preventDefault();
            items[next].focus();
        });
        renderStartButtonIcon();
        document.addEventListener('click', (event) => {
            if (!event.target.closest('.vd-context-menu')) closeContextMenu();
            if (!event.target.closest('.vd-window-menubar')) closeWindowMenu();
            const menu = $('vd-start-menu');
            // Protect both classic start button and Fruity Dock orb from the outside-click closer
            if (!menu.hidden && !menu.contains(event.target) && !event.target.closest('#vd-start-button, [data-fruity-dock-orb]')) {
                closeStartMenu();
            }
        });
        const taskbarEl = document.querySelector('.vd-taskbar');
        if (taskbarEl) {
            taskbarEl.addEventListener('contextmenu', (event) => {
                if (event.target.closest('button, input, a, .vd-start-menu')) return;
                event.preventDefault();
                showContextMenu(event.clientX, event.clientY, [
                    { label: t('desktop.context_settings'), icon: 'settings', fallback: 'S', action: () => openApp('settings', { category: 'appearance' }) },
                    { separator: true },
                    { label: t('desktop.context_system_info'), icon: 'analytics', fallback: 'i', action: () => openApp('system-info') },
                    { label: t('desktop.context_log_viewer'), icon: 'monitor', fallback: 'l', action: () => openApp('log-viewer') }
                ]);
            });
        }
        $('vd-workspace').addEventListener('contextmenu', showDesktopContextMenu);
        $('vd-workspace').addEventListener('pointerdown', startDesktopSelectionDrag);
        $('vd-workspace').addEventListener('pointermove', updateDesktopSelectionDrag);
        $('vd-workspace').addEventListener('pointerup', finishDesktopSelectionDrag);
        $('vd-workspace').addEventListener('pointercancel', finishDesktopSelectionDrag);
        $('vd-workspace').addEventListener('click', event => {
            if (desktopSelectionSuppressClick) {
                desktopSelectionSuppressClick = false;
                event.preventDefault();
                return;
            }
            if (event.target === $('vd-workspace') || event.target === $('vd-icons')) selectDesktopIcon(null);
        });
        wireDesktopFileDrops();
        document.addEventListener('keydown', handleDesktopKeydown);
        document.addEventListener('keyup', handleDesktopKeyup);
        wireStartMenuSwipe();
        if (window.AuraSSE && typeof window.AuraSSE.on === 'function') {
            window.AuraSSE.on('virtual_desktop_event', handleDesktopEvent);
        }
        window.addEventListener('message', handleSDKMessage);
    }

    function toggleWidgetDrawer() {
        const drawer = document.getElementById('vd-widget-drawer');
        const backdrop = document.getElementById('vd-widget-drawer-backdrop');
        if (!drawer || !backdrop) return;
        const drawerBtn = document.getElementById('vd-widget-drawer-btn');

        const closeDrawer = () => {
            drawer.classList.remove('open');
            backdrop.classList.remove('active');
            if (drawerBtn) drawerBtn.setAttribute('aria-expanded', 'false');
            window.setTimeout(() => { if (!drawer.classList.contains('open')) backdrop.hidden = true; }, 220);
        };

        if (drawer.classList.contains('open')) {
            closeDrawer();
            return;
        }

        drawer.classList.add('open');
        backdrop.hidden = false;
        if (drawerBtn) drawerBtn.setAttribute('aria-expanded', 'true');
        window.requestAnimationFrame(() => backdrop.classList.add('active'));

        if (!drawer.dataset.controlsWired) {
            drawer.dataset.controlsWired = 'true';
            backdrop.addEventListener('click', closeDrawer);
            const closeBtn = document.getElementById('vd-widget-drawer-close');
            if (closeBtn) closeBtn.addEventListener('click', closeDrawer);
            document.addEventListener('keydown', (event) => {
                if (event.key === 'Escape' && drawer.classList.contains('open')) closeDrawer();
            });
        }

        renderWidgetDrawerContent(drawer);
    }

    function wireStartMenuSwipe() {
        const menu = $('vd-start-menu');
        if (!menu) return;
        let swipe = null;
        menu.addEventListener('pointerdown', event => {
            if (!isCompactViewport() || !isTouchLikePointer(event) || event.button !== 0) return;
            if (event.target.closest('input, button, a')) return;
            swipe = { pointerId: event.pointerId, y: event.clientY };
            menu.setPointerCapture(event.pointerId);
        });
        menu.addEventListener('pointerup', event => {
            if (!swipe || swipe.pointerId !== event.pointerId) return;
            const dy = event.clientY - swipe.y;
            if (menu.hasPointerCapture && menu.hasPointerCapture(event.pointerId)) {
                menu.releasePointerCapture(event.pointerId);
            }
            swipe = null;
            if (dy > 60) closeStartMenu();
        });
        menu.addEventListener('pointercancel', event => {
            if (swipe && swipe.pointerId === event.pointerId) {
                swipe = null;
                if (menu.hasPointerCapture && menu.hasPointerCapture(event.pointerId)) {
                    menu.releasePointerCapture(event.pointerId);
                }
            }
        });
    }

    function showWindowSwitcher() {
        beginWindowSwitcherHold(false);
    }

    function handleDesktopKeydown(event) {
        if (handleWindowMenuShortcut(event)) return;
        if (handleSpaceShortcut(event)) return;
        if (handleWindowSwitcherKeydown(event)) return;
        if (isEditableTarget(event.target)) return;
        if (relayGeneratedFrameKeyboardEvent(event)) return;
        if (event.ctrlKey && event.key.toLowerCase() === 'k') {
            event.preventDefault();
            openSpotlight();
            return;
        }
        if (event.key === 'F1') {
            event.preventDefault();
            showShortcutsHelp();
            return;
        }
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
        if ((event.ctrlKey || event.metaKey) && event.altKey && event.key === 'w') {
            event.preventDefault();
            beginWindowSwitcherHold(false);
            return;
        }
        if (event.key === 'F11') {
            event.preventDefault();
            if (state.activeWindowId) toggleMaximizeWindow(state.activeWindowId);
            return;
        }
        switch (event.key) {
        case 'Escape': {
            const shortcuts = document.getElementById('vd-shortcuts-help');
            if (shortcuts) { closeShortcutsHelp(); return; }
            if (document.getElementById('vd-spotlight-backdrop')) { closeSpotlight(); return; }
            closeContextMenu();
            closeWindowMenu();
            closeStartMenu();
            return;
        }
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

    function handleDesktopKeyup(event) {
        if (handleWindowSwitcherKeyup(event)) return;
        relayGeneratedFrameKeyboardEvent(event);
    }

    function relayGeneratedFrameKeyboardEvent(event) {
        if (!event || event.defaultPrevented || isEditableTarget(event.target)) return false;
        if (event.ctrlKey || event.metaKey || event.altKey) return false;
        const frame = state.activeWindowId
            ? document.querySelector(`.vd-generated-frame[data-window-id="${cssSel(state.activeWindowId)}"]`)
            : null;
        if (!frame || !frame.contentWindow) return false;
        frame.contentWindow.postMessage({
            type: 'aurago.desktop.key-event',
            eventType: event.type === 'keyup' ? 'keyup' : 'keydown',
            key: event.key,
            code: event.code,
            location: event.location || 0,
            repeat: !!event.repeat,
            ctrlKey: !!event.ctrlKey,
            shiftKey: !!event.shiftKey,
            altKey: !!event.altKey,
            metaKey: !!event.metaKey
        }, '*');
        if (event.cancelable && (event.code === 'Space' || event.key === ' ' || event.key === 'Spacebar' || String(event.key || '').indexOf('Arrow') === 0)) {
            event.preventDefault();
        }
        return true;
    }

    function selectedDesktopIcon() {
        if (state.selectedIconId) {
            const active = document.querySelector(`.vd-icon[data-id="${cssSel(state.selectedIconId)}"]`);
            if (active && active.classList.contains('selected')) return active;
        }
        const icons = selectedDesktopIcons();
        return icons.length ? icons[0] : null;
    }

    function selectedFileDirectoryIcon() {
        const icon = selectedDesktopIcon();
        if (!icon || !icon.dataset.path) return null;
        return icon.dataset.kind === 'file' || icon.dataset.kind === 'directory' ? icon : null;
    }

    async function init() {
        if (state._initialized) return;
        state._initialized = true;
        const perfOn = typeof location !== 'undefined' && /(?:\?|&)vd_perf=1(?:&|$)/.test(location.search || '');
        const mark = (name) => {
            if (!perfOn || !window.performance || typeof performance.mark !== 'function') return;
            try { performance.mark('vd:' + name); } catch (_) { /* ignore */ }
        };
        mark('init-start');
        ['vd-icons', 'vd-widgets', 'vd-window-layer', 'vd-taskbar-apps', 'vd-start-apps', 'vd-start-menu', 'vd-start-search', 'vd-ws-state', 'vd-clock', 'vd-workspace', 'vd-disabled'].forEach(id => { els[id] = $(id); });
        ensureDesktopRadialMenuAnchor();
        bindViewportMetrics();
        wireChrome();
        wireShellChromeControls();
        document.addEventListener('focusin', ensureFocusedControlVisible);
        updateClock();
        state._clockTimer = setInterval(updateClock, 15000);
        window.addEventListener('beforeunload', cleanupDesktopShellRuntime);
        // Load icon manifests and bootstrap state in parallel, then render once.
        mark('parallel-fetch-start');
        await Promise.all([
            loadIconManifest().catch(() => null),
            (async () => {
                if (bootstrapReloadPromise) return bootstrapReloadPromise;
                bootstrapReloadPromise = fetchBootstrapState()
                    .finally(() => { bootstrapReloadPromise = null; });
                return bootstrapReloadPromise;
            })()
        ]);
        mark('parallel-fetch-done');
        renderDesktop();
        initSpacesRuntime();
        initSIPPhoneShellRuntime();
        if (window.SipPhoneGadget && typeof window.SipPhoneGadget.init === 'function') window.SipPhoneGadget.init();
        refreshPetRuntime();
        mark('first-render');
        openInitialDesktopApp();
        const bootApp = new URLSearchParams(window.location.search || '').get('app');
        if (!bootApp) restoreDesktopSession();
        if (state.bootstrap && state.bootstrap.enabled) connectWS();
        if (window.PetRuntime && typeof window.PetRuntime.init === 'function') {
            window.PetRuntime.init();
        }
        if (perfOn && window.performance && typeof performance.measure === 'function') {
            try {
                performance.measure('vd:boot', 'vd:init-start', 'vd:first-render');
                const m = performance.getEntriesByName('vd:boot').pop();
                console.info('[VD perf] boot ms=', m && Math.round(m.duration));
            } catch (_) { /* ignore */ }
        }
    }

    ensureDesktopRadialMenuAnchor();
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init, { once: true });
    } else {
        init();
    }
})();
