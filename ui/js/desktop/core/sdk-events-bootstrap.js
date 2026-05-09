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

    let wsReconnectAttempts = 0;
    let wsReconnectDelay = 2000;
    let wsReconnectTimer = null;
    const MAX_WS_RETRIES = 10;
    const WS_MAX_DELAY = 30000;

    function connectWS() {
        if (wsReconnectTimer) {
            clearTimeout(wsReconnectTimer);
            wsReconnectTimer = null;
        }
        if (state.ws) {
            try { state.ws.close(); } catch (_) {}
        }
        const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
        const ws = new WebSocket(proto + '//' + location.host + '/api/desktop/ws');
        state.ws = ws;
        ws.addEventListener('open', () => {
            wsReconnectAttempts = 0;
            wsReconnectDelay = 2000;
            setWSState(true);
        });
        ws.addEventListener('close', () => {
            if (wsReconnectAttempts >= MAX_WS_RETRIES) {
                setWSState(false, true);
                return;
            }
            setWSState(false);
            wsReconnectTimer = setTimeout(() => {
                wsReconnectAttempts++;
                wsReconnectDelay = Math.min(wsReconnectDelay * 2, WS_MAX_DELAY);
                connectWS();
            }, wsReconnectDelay);
        });
        ws.addEventListener('message', async (event) => {
            let msg;
            try { msg = JSON.parse(event.data); } catch (_) { return; }
            handleDesktopEvent(msg.type === 'welcome' ? { type: 'welcome', payload: msg.payload } : msg);
        });
    }

    function setWSState(online, failed) {
        const dot = $('vd-ws-state');
        if (online) {
            dot.dataset.state = 'online';
            dot.title = '';
        } else if (failed) {
            dot.dataset.state = 'offline';
            dot.title = t('desktop.ws_connection_lost', 'Connection lost. Please refresh the page.');
        } else {
            dot.dataset.state = 'reconnecting';
            dot.title = t('desktop.ws_reconnecting', 'Reconnecting...');
        }
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
        let startSearchTimer = null;
        $('vd-start-search').addEventListener('input', (event) => {
            state.startQuery = event.target.value;
            clearTimeout(startSearchTimer);
            startSearchTimer = setTimeout(renderStartApps, 150);
        });
        renderStartButtonIcon();
        document.addEventListener('click', (event) => {
            if (!event.target.closest('.vd-context-menu')) closeContextMenu();
            if (!event.target.closest('.vd-window-menubar')) closeWindowMenu();
            const menu = $('vd-start-menu');
            if (!menu.hidden && !menu.contains(event.target) && !event.target.closest('#vd-start-button')) {
                closeStartMenu();
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
        if ((event.ctrlKey || event.metaKey) && event.key === '/') {
            event.preventDefault();
            toggleShortcutsHelp();
            return;
        }
        if (event.key === 'F1') {
            event.preventDefault();
            toggleShortcutsHelp();
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
        if (event.altKey && event.key === 'Tab') {
            event.preventDefault();
            const wins = [...state.windows.values()];
            if (!wins.length) return;
            const index = wins.findIndex(win => win.id === state.activeWindowId);
            focusWindow(wins[(index + 1 + wins.length) % wins.length].id);
        }
        switch (event.key) {
        case 'Escape': {
            const shortcuts = document.getElementById('vd-shortcuts-help');
            if (shortcuts) { shortcuts.remove(); return; }
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

    function toggleShortcutsHelp() {
        const existing = document.getElementById('vd-shortcuts-help');
        if (existing) { existing.remove(); return; }
        showShortcutsHelp();
    }

    function showShortcutsHelp() {
        const shortcuts = [
            { keys: 'Ctrl+Space', action: t('desktop.shortcut_start_menu', 'Open Start menu') },
            { keys: 'Alt+F4', action: t('desktop.shortcut_close_window', 'Close active window') },
            { keys: 'Alt+Tab', action: t('desktop.shortcut_switch_windows', 'Switch windows') },
            { keys: 'F2', action: t('desktop.shortcut_rename', 'Rename selected item') },
            { keys: 'Delete', action: t('desktop.shortcut_delete', 'Delete selected item') },
            { keys: 'Ctrl+/  /  F1', action: t('desktop.shortcut_help', 'Show keyboard shortcuts') }
        ];
        const overlay = document.createElement('div');
        overlay.id = 'vd-shortcuts-help';
        overlay.className = 'vd-shortcuts-overlay';
        overlay.innerHTML = `
            <div class="vd-shortcuts-modal">
                <div class="vd-shortcuts-header">
                    <span class="vd-shortcuts-title">${esc(t('desktop.keyboard_shortcuts_title', 'Keyboard Shortcuts'))}</span>
                    <button class="vd-shortcuts-close" aria-label="${esc(t('desktop.keyboard_shortcuts_close', 'Close'))}">×</button>
                </div>
                <div class="vd-shortcuts-body">
                    ${shortcuts.map(s => `
                        <div class="vd-shortcuts-row">
                            <kbd class="vd-shortcuts-keys">${esc(s.keys)}</kbd>
                            <span class="vd-shortcuts-action">${esc(s.action)}</span>
                        </div>
                    `).join('')}
                </div>
            </div>
        `;
        overlay.querySelector('.vd-shortcuts-close').addEventListener('click', () => overlay.remove());
        overlay.addEventListener('click', e => { if (e.target === overlay) overlay.remove(); });
        document.body.appendChild(overlay);
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

    async function renderCalendar(id) {
        const host = contentEl(id);
        if (!host) return;
        host.dataset.calView = host.dataset.calView || 'month';
        host.dataset.calDate = host.dataset.calDate || isoDate(new Date());
        const activeDate = new Date(host.dataset.calDate + 'T12:00:00');
        const view = host.dataset.calView;
        host.innerHTML = `<div class="vd-calendar-shell">
            <header class="vd-calendar-command">
                <div class="vd-calendar-titlebar">
                    <div class="vd-calendar-nav">
                        <button class="vd-calendar-icon-button" type="button" data-cal-nav="prev" title="${esc(t('desktop.cal_previous'))}">${iconMarkup('chevron-left', 'L', 'vd-calendar-action-icon', 15)}</button>
                        <button class="vd-calendar-icon-button" type="button" data-cal-nav="next" title="${esc(t('desktop.cal_next'))}">${iconMarkup('chevron-right', 'R', 'vd-calendar-action-icon', 15)}</button>
                    </div>
                    <div class="vd-calendar-heading">
                        <strong>${esc(calendarRangeLabel(activeDate, view))}</strong>
                        <span>${esc(t('desktop.cal_drag_hint'))}</span>
                    </div>
                </div>
                <div class="vd-calendar-actions">
                    <button class="vd-button vd-button-primary" type="button" data-cal-create>${iconMarkup('calendar', 'C', 'vd-calendar-action-icon', 15)}<span>${esc(t('desktop.cal_new'))}</span></button>
                    <button class="vd-button" type="button" data-cal-today>${iconMarkup('calendar', 'C', 'vd-calendar-action-icon', 15)}<span>${esc(t('desktop.cal_today'))}</span></button>
                    <div class="vd-calendar-view-switch" role="group" aria-label="${esc(t('desktop.menu_view'))}">
                        ${['month','week','day'].map(item => `<button type="button" data-cal-view="${item}" class="${view === item ? 'active' : ''}">${esc(t('desktop.cal_' + item))}</button>`).join('')}
                    </div>
                </div>
            </header>
            <div class="vd-calendar-stage">
                <div class="vd-calendar-body" data-cal-body>${esc(t('desktop.loading'))}</div>
                <aside class="vd-calendar-sidebar" data-cal-sidebar>${esc(t('desktop.loading'))}</aside>
            </div>
        </div>`;
        const showCalendarContextMenu = (event, appointments, render) => {
            const apptEl = event.target.closest('[data-appt-id]');
            const cellEl = event.target.closest('[data-cal-date]');
            if (!apptEl && !cellEl) return false;
            const appt = apptEl ? appointments.find(item => item.id === apptEl.dataset.apptId) : null;
            const date = cellEl ? cellEl.dataset.calDate : isoDate(activeDate);
            const items = [
                appt
                    ? { labelKey: 'desktop.launchpad_edit', icon: 'edit', action: () => openAppointmentModal(host, appt, '', render) }
                    : { labelKey: 'desktop.cal_new_appointment', icon: 'calendar', action: () => openAppointmentModal(host, null, date, render) }
            ];
            if (appt && (appt.status === 'upcoming' || appt.status === 'overdue')) {
                items.push(
                    { labelKey: 'desktop.cal_mark_complete', icon: 'check-square', action: async () => { await updateAppointmentStatus(appt, 'completed', render); } },
                    { labelKey: 'desktop.cal_cancel_appointment', icon: 'x', action: async () => { await updateAppointmentStatus(appt, 'cancelled', render); } }
                );
            }
            items.push({ separator: true }, { labelKey: 'desktop.context_refresh', icon: 'refresh', action: render });
            showContextMenu(event.clientX, event.clientY, items);
            return true;
        };
        wireContextMenuBoundary(host);
        const render = async () => {
            const appointments = normalizeCalendarAppointments(await api('/api/appointments?status=all'));
            const body = host.querySelector('.vd-calendar-body');
            const sidebar = host.querySelector('[data-cal-sidebar]');
            body.innerHTML = host.dataset.calView === 'month' ? calendarMonthHTML(activeDate, appointments) : calendarAgendaHTML(activeDate, appointments, host.dataset.calView);
            sidebar.innerHTML = calendarSidebarHTML(activeDate, appointments);
            wireCalendarBody(host, appointments, render);
            body.oncontextmenu = event => {
                if (showCalendarContextMenu(event, appointments, render)) event.preventDefault();
            };
            sidebar.oncontextmenu = event => {
                if (showCalendarContextMenu(event, appointments, render)) event.preventDefault();
            };
        };
        host.querySelectorAll('[data-cal-view]').forEach(btn => btn.addEventListener('click', () => { host.dataset.calView = btn.dataset.calView; renderCalendar(id); }));
        host.querySelector('[data-cal-today]').addEventListener('click', () => { host.dataset.calDate = isoDate(new Date()); renderCalendar(id); });
        host.querySelector('[data-cal-create]').addEventListener('click', () => openAppointmentModal(host, null, isoDate(activeDate), render));
        host.querySelectorAll('[data-cal-nav]').forEach(btn => btn.addEventListener('click', () => {
            const delta = btn.dataset.calNav === 'next' ? 1 : -1;
            if (host.dataset.calView === 'month') activeDate.setMonth(activeDate.getMonth() + delta);
            else activeDate.setDate(activeDate.getDate() + delta * (host.dataset.calView === 'week' ? 7 : 1));
            host.dataset.calDate = isoDate(activeDate);
            renderCalendar(id);
        }));
        setCalendarMenus(id, host, activeDate, render);
        try { await render(); } catch (err) { host.querySelector('.vd-calendar-body').innerHTML = `<div class="vd-empty">${esc(err.message)}</div>`; }
    }

    function setCalendarMenus(id, host, activeDate, render) {
        setWindowMenus(id, [
            {
                id: 'file',
                labelKey: 'desktop.menu_file',
                items: [
                    { id: 'new-appointment', labelKey: 'desktop.cal_new_appointment', icon: 'calendar', shortcut: 'Ctrl+N', action: () => openAppointmentModal(host, null, isoDate(activeDate), render) }
                ]
            },
            {
                id: 'view',
                labelKey: 'desktop.menu_view',
                items: [
                    { id: 'today', labelKey: 'desktop.cal_today', icon: 'calendar', action: () => { host.dataset.calDate = isoDate(new Date()); renderCalendar(id); } },
                    { id: 'refresh', labelKey: 'desktop.context_refresh', icon: 'refresh', shortcut: 'F5', action: render }
                ]
            }
        ]);
    }

    function normalizeCalendarAppointments(appointments) {
        return (appointments || []).map(item => Object.assign({
            title: '',
            description: '',
            status: 'upcoming',
            participants: [],
            contact_ids: []
        }, item || {})).sort((left, right) => calendarDate(left).getTime() - calendarDate(right).getTime());
    }

    function calendarDate(appointment) {
        const d = new Date((appointment && appointment.date_time) || Date.now());
        return Number.isNaN(d.getTime()) ? new Date() : d;
    }

    function calendarRangeLabel(activeDate, view) {
        if (view === 'day') return activeDate.toLocaleDateString(undefined, { weekday: 'long', month: 'long', day: 'numeric', year: 'numeric' });
        if (view === 'week') {
            const days = calendarWeekDays(activeDate);
            const first = days[0].toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
            const last = days[6].toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
            return `${first} - ${last}`;
        }
        return activeDate.toLocaleDateString(undefined, { month: 'long', year: 'numeric' });
    }

    function calendarWeekDays(activeDate) {
        return Array.from({ length: 7 }, (_, i) => {
            const d = new Date(activeDate);
            d.setDate(activeDate.getDate() - ((activeDate.getDay() + 6) % 7) + i);
            return d;
        });
    }

    function calendarDayItems(appointments, date) {
        const key = isoDate(date);
        return appointments.filter(a => String(a.date_time || '').startsWith(key));
    }

    function calendarStatusLabel(status) {
        const key = 'desktop.cal_status_' + (status || 'upcoming');
        const label = t(key);
        return label === key ? String(status || '') : label;
    }

    function calendarTimeLabel(value) {
        const d = new Date(value);
        if (Number.isNaN(d.getTime())) return '';
        return d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
    }

    function calendarDateTimeLabel(value) {
        const d = new Date(value);
        if (Number.isNaN(d.getTime())) return '';
        return d.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
    }

    function calendarEventButtonHTML(appointment, mode) {
        const status = esc(appointment.status || 'upcoming');
        const participants = (appointment.participants || []).length;
        const time = calendarTimeLabel(appointment.date_time);
        return `<button type="button" draggable="true" class="vd-calendar-event ${status} ${mode || ''}" data-appt-id="${esc(appointment.id)}" title="${esc(appointment.title)}">
            <span class="vd-calendar-event-time">${esc(time)}</span>
            <span class="vd-calendar-event-title">${esc(appointment.title)}</span>
            <span class="vd-calendar-event-badges">${appointment.wake_agent ? iconMarkup('agent', 'A', 'vd-calendar-mini-icon', 12) : ''}${participants ? `<em>${esc(String(participants))}</em>` : ''}</span>
        </button>`;
    }

    function calendarMonthHTML(activeDate, appointments) {
        const first = new Date(activeDate.getFullYear(), activeDate.getMonth(), 1);
        const start = new Date(first);
        start.setDate(first.getDate() - ((first.getDay() + 6) % 7));
        const today = isoDate(new Date());
        const cells = Array.from({ length: 42 }, (_, i) => { const d = new Date(start); d.setDate(start.getDate() + i); return d; });
        const weekdays = calendarWeekDays(new Date()).map(d => `<span>${esc(d.toLocaleDateString(undefined, { weekday: 'short' }))}</span>`).join('');
        return `<div class="vd-calendar-month-wrap">
            <div class="vd-calendar-weekdays">${weekdays}</div>
            <div class="vd-calendar-month">${cells.map(d => {
                const key = isoDate(d);
                const dayItems = calendarDayItems(appointments, d);
                const hidden = Math.max(0, dayItems.length - 3);
                return `<div role="button" tabindex="0" class="vd-calendar-cell ${d.getMonth() !== activeDate.getMonth() ? 'muted' : ''} ${key === today ? 'today' : ''}" data-cal-date="${key}" data-cal-drop-date="${key}">
                    <div class="vd-calendar-cell-head"><span>${d.getDate()}</span>${dayItems.length ? `<em>${dayItems.length}</em>` : ''}</div>
                    <div class="vd-calendar-cell-events">${dayItems.slice(0, 3).map(a => calendarEventButtonHTML(a, 'compact')).join('')}${hidden ? `<button type="button" class="vd-calendar-more" data-cal-date="${key}">${esc(t('desktop.cal_more_events')).replace('{{count}}', String(hidden))}</button>` : ''}</div>
                </div>`;
            }).join('')}</div>
        </div>`;
    }

    function calendarAgendaHTML(activeDate, appointments, view) {
        const days = view === 'week' ? calendarWeekDays(activeDate) : [activeDate];
        const hours = Array.from({ length: 17 }, (_, i) => i + 6);
        return `<div class="vd-calendar-time-grid ${esc(view)}" style="--vd-calendar-days:${days.length}">
            <div class="vd-calendar-time-corner">${esc(t('desktop.cal_schedule'))}</div>
            ${days.map(day => `<div class="vd-calendar-day-head ${isoDate(day) === isoDate(new Date()) ? 'today' : ''}"><strong>${esc(day.toLocaleDateString(undefined, { weekday: 'short' }))}</strong><span>${esc(day.toLocaleDateString(undefined, { month: 'short', day: 'numeric' }))}</span></div>`).join('')}
            ${hours.map(hour => `<div class="vd-calendar-time-label">${String(hour).padStart(2, '0')}:00</div>${days.map(day => {
                const key = `${isoDate(day)}T${String(hour).padStart(2, '0')}:00`;
                const dayItems = calendarDayItems(appointments, day).filter(a => calendarDate(a).getHours() === hour);
                return `<div class="vd-calendar-hour" data-cal-date="${key}" data-cal-drop-date="${key}">
                    ${dayItems.map(a => calendarEventButtonHTML(a, 'wide')).join('') || `<span class="vd-calendar-empty-slot">${esc(t('desktop.cal_no_events'))}</span>`}
                </div>`;
            }).join('')}`).join('')}
        </div>`;
    }

    function calendarSidebarHTML(activeDate, appointments) {
        const now = new Date();
        const todayItems = calendarDayItems(appointments, new Date());
        const upcoming = appointments.filter(a => calendarDate(a) >= now && a.status !== 'cancelled' && a.status !== 'completed').slice(0, 8);
        const overdue = appointments.filter(a => (a.status === 'overdue') || (a.status === 'upcoming' && calendarDate(a) < now));
        return `<section class="vd-calendar-side-section">
            <div class="vd-calendar-side-title"><span>${esc(t('desktop.cal_today_panel'))}</span><strong>${todayItems.length}</strong></div>
            <div class="vd-calendar-side-list">${todayItems.length ? todayItems.map(a => calendarSidebarItemHTML(a)).join('') : `<p>${esc(t('desktop.cal_no_events'))}</p>`}</div>
        </section>
        <section class="vd-calendar-side-section">
            <div class="vd-calendar-side-title"><span>${esc(t('desktop.cal_upcoming'))}</span><strong>${upcoming.length}</strong></div>
            <div class="vd-calendar-side-list">${upcoming.length ? upcoming.map(a => calendarSidebarItemHTML(a)).join('') : `<p>${esc(t('desktop.cal_no_events'))}</p>`}</div>
        </section>
        <section class="vd-calendar-side-section compact">
            <div class="vd-calendar-side-title overdue"><span>${esc(t('desktop.cal_overdue'))}</span><strong>${overdue.length}</strong></div>
        </section>`;
    }

    function calendarSidebarItemHTML(appointment) {
        const participants = (appointment.participants || []).map(p => p.name).filter(Boolean).slice(0, 2).join(', ');
        return `<button type="button" class="vd-calendar-side-item ${esc(appointment.status || 'upcoming')}" data-appt-id="${esc(appointment.id)}">
            <strong>${esc(calendarDateTimeLabel(appointment.date_time))}</strong>
            <span>${esc(appointment.title)}</span>
            ${participants ? `<small>${esc(participants)}</small>` : ''}
            ${appointment.wake_agent ? `<em>${esc(t('desktop.cal_agent_instruction'))}</em>` : ''}
        </button>`;
    }

    function wireCalendarBody(host, appointments, reload) {
        const root = host.querySelector('.vd-calendar-stage');
        if (!root) return;
        root.querySelectorAll('[data-cal-date]').forEach(cell => {
            cell.addEventListener('click', event => {
                if (event.target.closest('[data-appt-id]')) return;
                openAppointmentModal(host, null, cell.dataset.calDate, reload);
            });
            cell.addEventListener('keydown', event => {
                if (event.key === 'Enter' || event.key === ' ') {
                    event.preventDefault();
                    openAppointmentModal(host, null, cell.dataset.calDate, reload);
                }
            });
        });
        root.querySelectorAll('[data-appt-id]').forEach(btn => {
            const appointment = appointments.find(a => a.id === btn.dataset.apptId);
            btn.addEventListener('click', event => {
                event.stopPropagation();
                if (appointment) openAppointmentModal(host, appointment, '', reload);
            });
            btn.addEventListener('dragstart', event => {
                event.dataTransfer.setData('text/plain', btn.dataset.apptId || '');
                event.dataTransfer.effectAllowed = 'move';
                btn.classList.add('dragging');
            });
            btn.addEventListener('dragend', () => btn.classList.remove('dragging'));
        });
        root.querySelectorAll('[data-cal-drop-date]').forEach(zone => {
            zone.addEventListener('dragover', event => {
                event.preventDefault();
                zone.classList.add('drop-target');
            });
            zone.addEventListener('dragleave', () => zone.classList.remove('drop-target'));
            zone.addEventListener('drop', async event => {
                event.preventDefault();
                zone.classList.remove('drop-target');
                const id = event.dataTransfer.getData('text/plain');
                const appointment = appointments.find(a => a.id === id);
                if (appointment) await updateAppointmentDateTime(appointment, zone.dataset.calDropDate, reload);
            });
        });
    }

    async function updateAppointmentDateTime(appointment, dateHint, reload) {
        const previous = calendarDate(appointment);
        let next;
        if (String(dateHint || '').includes('T')) {
            next = new Date(dateHint);
            next.setMinutes(previous.getMinutes(), 0, 0);
        } else {
            next = new Date(`${dateHint}T${String(previous.getHours()).padStart(2, '0')}:${String(previous.getMinutes()).padStart(2, '0')}:00`);
        }
        if (Number.isNaN(next.getTime())) return;
        await plannerJSON('/api/appointments/' + encodeURIComponent(appointment.id), 'PUT', { date_time: next.toISOString() });
        await reload();
    }

    async function updateAppointmentStatus(appointment, status, reload) {
        await plannerJSON('/api/appointments/' + encodeURIComponent(appointment.id), 'PUT', { status });
        await reload();
    }

    function calendarOptionalDateTime(value) {
        return value ? new Date(value).toISOString() : '';
    }

    function shiftCalendarDate(value, repeat, amount) {
        const date = new Date(value);
        if (repeat === 'daily') date.setDate(date.getDate() + amount);
        if (repeat === 'weekly') date.setDate(date.getDate() + amount * 7);
        if (repeat === 'monthly') date.setMonth(date.getMonth() + amount);
        return date.toISOString();
    }

    async function createRecurringAppointments(payload, repeat, count) {
        const total = Math.max(1, Math.min(Number(count) || 1, 30));
        for (let i = 0; i < total; i++) {
            const item = Object.assign({}, payload, {
                date_time: shiftCalendarDate(payload.date_time, repeat, i)
            });
            if (payload.notification_at) item.notification_at = shiftCalendarDate(payload.notification_at, repeat, i);
            await plannerJSON('/api/appointments', 'POST', item);
        }
    }

    function openAppointmentModal(host, appointment, dateHint, reload) {
        const overlay = document.createElement('div');
        overlay.className = 'vd-modal-backdrop';
        const initial = appointment || { title: '', description: '', status: 'upcoming', date_time: dateHint ? fromLocalDateTime(dateHint.includes('T') ? dateHint : dateHint + 'T09:00') : new Date().toISOString(), wake_agent: false };
        const participants = (initial.participants || []).map(p => p.name).filter(Boolean).join(', ');
        overlay.innerHTML = `<form class="vd-modal vd-calendar-modal"><div class="vd-modal-title">${esc(t(appointment ? 'desktop.cal_edit_appointment' : 'desktop.cal_new_appointment'))}</div>
            <div class="vd-calendar-modal-grid">
                <label><span>${esc(t('desktop.cal_title'))}</span><input name="title" class="vd-modal-input" value="${esc(initial.title)}"></label>
                <label><span>${esc(t('desktop.cal_date_time'))}</span><input name="date_time" class="vd-modal-input" type="datetime-local" value="${esc(dateTimeLocalValue(initial.date_time))}"></label>
                <label><span>${esc(t('desktop.cal_reminder'))}</span><input name="notification_at" class="vd-modal-input" type="datetime-local" value="${esc(initial.notification_at ? dateTimeLocalValue(initial.notification_at) : '')}"></label>
                <label><span>${esc(t('desktop.cal_status'))}</span><select name="status" class="vd-modal-input">${['upcoming','overdue','completed','cancelled'].map(status => `<option value="${status}" ${initial.status === status ? 'selected' : ''}>${esc(calendarStatusLabel(status))}</option>`).join('')}</select></label>
            </div>
            <label class="vd-calendar-modal-block"><span>${esc(t('desktop.cal_description'))}</span><textarea name="description" class="vd-modal-input">${esc(initial.description || '')}</textarea></label>
            <label class="vd-check vd-calendar-wake"><input name="wake_agent" type="checkbox" ${initial.wake_agent ? 'checked' : ''}>${esc(t('desktop.cal_notification'))}</label>
            <label class="vd-calendar-modal-block"><span>${esc(t('desktop.cal_agent_instruction'))}</span><textarea name="agent_instruction" class="vd-modal-input">${esc(initial.agent_instruction || '')}</textarea></label>
            ${participants ? `<div class="vd-calendar-participants"><strong>${esc(t('desktop.cal_participants'))}</strong><span>${esc(participants)}</span></div>` : ''}
            ${appointment ? '' : `<div class="vd-calendar-recurring">
                <label><span>${esc(t('desktop.cal_recurring'))}</span><select name="repeat" class="vd-modal-input">
                    <option value="none">${esc(t('desktop.cal_repeat_none'))}</option>
                    <option value="daily">${esc(t('desktop.cal_repeat_daily'))}</option>
                    <option value="weekly">${esc(t('desktop.cal_repeat_weekly'))}</option>
                    <option value="monthly">${esc(t('desktop.cal_repeat_monthly'))}</option>
                </select></label>
                <label><span>${esc(t('desktop.cal_repeat_count'))}</span><input name="repeat_count" class="vd-modal-input" type="number" min="1" max="30" value="1"></label>
            </div>`}
            ${appointment && (initial.status === 'upcoming' || initial.status === 'overdue') ? `<div class="vd-calendar-status-actions">
                <button type="button" class="vd-button" data-cal-status-action="completed">${iconMarkup('check-square', 'C', 'vd-modal-action-icon', 15)}<span>${esc(t('desktop.cal_mark_complete'))}</span></button>
                <button type="button" class="vd-button" data-cal-status-action="cancelled">${iconMarkup('x', 'X', 'vd-modal-action-icon', 15)}<span>${esc(t('desktop.cal_cancel_appointment'))}</span></button>
            </div>` : ''}
            <div class="vd-modal-actions">${appointment ? `<button type="button" class="vd-button" data-delete>${iconMarkup('trash', 'X', 'vd-modal-action-icon', 15)}<span>${esc(t('desktop.delete'))}</span></button>` : ''}<button type="button" class="vd-button" data-cancel>${iconMarkup('x', 'X', 'vd-modal-action-icon', 15)}<span>${esc(t('desktop.cancel'))}</span></button><button class="vd-button vd-button-primary">${iconMarkup('save', 'S', 'vd-modal-action-icon', 15)}<span>${esc(t('desktop.save'))}</span></button></div></form>`;
        document.body.appendChild(overlay);
        const close = () => overlay.remove();
        overlay.querySelector('[data-cancel]').addEventListener('click', close);
        overlay.addEventListener('click', event => { if (event.target === overlay) close(); });
        const del = overlay.querySelector('[data-delete]');
        if (del) del.addEventListener('click', async () => { if (await confirmDialog(t('desktop.cal_delete_confirm'), appointment.title)) { await api('/api/appointments/' + encodeURIComponent(appointment.id), { method: 'DELETE' }); close(); await reload(); } });
        overlay.querySelectorAll('[data-cal-status-action]').forEach(btn => btn.addEventListener('click', async () => {
            await updateAppointmentStatus(appointment, btn.dataset.calStatusAction, reload);
            close();
        }));
        overlay.querySelector('form').addEventListener('submit', async event => {
            event.preventDefault();
            const form = event.currentTarget;
            const payload = { title: form.title.value.trim(), date_time: fromLocalDateTime(form.date_time.value), notification_at: calendarOptionalDateTime(form.notification_at.value), description: form.description.value, status: form.status.value, wake_agent: form.wake_agent.checked, agent_instruction: form.agent_instruction.value.trim() };
            if (!payload.title) return;
            if (appointment) await plannerJSON('/api/appointments/' + encodeURIComponent(appointment.id), 'PUT', payload);
            else if (form.repeat.value !== 'none' && Number(form.repeat_count.value) > 1) await createRecurringAppointments(payload, form.repeat.value, form.repeat_count.value);
            else await plannerJSON('/api/appointments', 'POST', payload);
            close();
            await reload();
        });
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
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init, { once: true });
    } else {
        init();
    }
})();
