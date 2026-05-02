(function () {
    const state = {
        bootstrap: null,
        filesPath: '',
        windows: new Map(),
        activeWindowId: '',
        z: 40,
        ws: null,
        chatBusy: false,
        startQuery: '',
        iconManifest: null,
        iconMap: new Map()
    };

    const SDK_REQUEST_TYPE = 'aurago.desktop.request';
    const SDK_RESPONSE_TYPE = 'aurago.desktop.response';
    const SDK_RUNTIME = 'aura-desktop-sdk@1';

    const els = {};
    const directoryIconKeys = {
        Desktop: 'desktop',
        Documents: 'documents',
        Downloads: 'downloads',
        Apps: 'apps',
        Widgets: 'widgets',
        Data: 'database',
        Pictures: 'image',
        Trash: 'trash',
        Shared: 'share'
    };
    const appIconKeys = {
        files: 'folder',
        editor: 'edit',
        settings: 'settings',
        calendar: 'calendar',
        'agent-chat': 'agent_chat',
        terminal: 'terminal',
        browser: 'browser'
    };
    const extensionIconKeys = {
        txt: 'text',
        log: 'text',
        md: 'markdown',
        js: 'javascript',
        mjs: 'javascript',
        html: 'html',
        htm: 'html',
        css: 'css',
        json: 'json',
        yaml: 'yaml',
        yml: 'yaml',
        xml: 'xml',
        py: 'python',
        go: 'go',
        pdf: 'pdf',
        png: 'image',
        jpg: 'image',
        jpeg: 'image',
        gif: 'image',
        webp: 'image',
        svg: 'image',
        mp3: 'audio',
        wav: 'audio',
        flac: 'audio',
        mp4: 'video',
        webm: 'video',
        mov: 'video',
        zip: 'archive',
        tar: 'archive',
        gz: 'archive',
        db: 'database',
        sqlite: 'database',
        csv: 'spreadsheet',
        xlsx: 'spreadsheet',
        pptx: 'presentation',
        exe: 'executable',
        bin: 'binary'
    };

    function $(id) {
        return document.getElementById(id);
    }

    function esc(value) {
        return String(value == null ? '' : value)
            .replaceAll('&', '&amp;')
            .replaceAll('<', '&lt;')
            .replaceAll('>', '&gt;')
            .replaceAll('"', '&quot;')
            .replaceAll("'", '&#39;');
    }

    function iconGlyph(app) {
        const id = (app && app.id) || '';
        const map = {
            files: 'F',
            editor: 'E',
            settings: 'S',
            calendar: 'C',
            'agent-chat': 'A'
        };
        return map[id] || ((app && app.name && app.name[0]) || 'D').toUpperCase();
    }

    async function loadIconManifest() {
        try {
            const manifest = await api('/img/desktop-icons-sprite.json');
            state.iconManifest = manifest;
            state.iconMap = new Map((manifest.icons || []).map(icon => [icon.name, icon]));
        } catch (_) {
            state.iconManifest = null;
            state.iconMap = new Map();
        }
    }

    function iconExists(key) {
        return key && state.iconMap.has(key);
    }

    function spriteMarkup(key, fallback, className, size) {
        const manifest = state.iconManifest;
        const icon = iconExists(key) ? state.iconMap.get(key) : null;
        if (!manifest || !icon) {
            return `<span class="${esc(className)} vd-icon-letter">${esc(String(fallback || 'D').slice(0, 2).toUpperCase())}</span>`;
        }
        const scale = (size || 42) / (manifest.icon_size || 64);
        const sheetW = Math.round((manifest.width || 768) * scale * 1000) / 1000;
        const sheetH = Math.round((manifest.height || 768) * scale * 1000) / 1000;
        const x = Math.round(-(icon.x || 0) * scale * 1000) / 1000;
        const y = Math.round(-(icon.y || 0) * scale * 1000) / 1000;
        return `<span class="${esc(className)}" aria-hidden="true" style="--vd-sprite-x:${x}px;--vd-sprite-y:${y}px;--vd-sprite-sheet:${sheetW}px ${sheetH}px"></span>`;
    }

    function iconForApp(app) {
        if (!app) return 'apps';
        return appIconKeys[app.id] || app.icon || 'apps';
    }

    function iconForDirectory(name) {
        return directoryIconKeys[name] || 'folder';
    }

    function iconForFile(file) {
        if (file.type === 'directory') return 'folder';
        const ext = String(file.name || '').split('.').pop().toLowerCase();
        if (!ext || ext === String(file.name || '').toLowerCase()) return 'file';
        return extensionIconKeys[ext] || 'file';
    }

    function appName(app) {
        const key = 'desktop.app_' + String(app.id || '').replaceAll('-', '_');
        const translated = t(key);
        return translated === key ? (app.name || app.id) : translated;
    }

    function allApps() {
        const boot = state.bootstrap || {};
        return [...(boot.builtin_apps || []), ...(boot.installed_apps || [])];
    }

    function fmtBytes(size) {
        const n = Number(size || 0);
        if (n < 1024) return t('desktop.bytes', { count: n });
        if (n < 1024 * 1024) return t('desktop.kib', { count: (n / 1024).toFixed(1) });
        return t('desktop.mib', { count: (n / 1024 / 1024).toFixed(1) });
    }

    async function api(url, options) {
        const resp = await fetch(url, options);
        const contentType = resp.headers.get('content-type') || '';
        const shouldParseJSON = contentType.includes('application/json') || String(url).includes('.json');
        const body = shouldParseJSON ? await resp.json() : {};
        if (!resp.ok) {
            throw new Error(body.error || body.message || ('HTTP ' + resp.status));
        }
        return body;
    }

    async function loadBootstrap() {
        state.bootstrap = await api('/api/desktop/bootstrap');
        renderDesktop();
    }

    function renderDesktop() {
        const enabled = state.bootstrap && state.bootstrap.enabled;
        $('vd-disabled').hidden = !!enabled;
        renderIcons();
        renderWidgets();
        renderStartApps();
        renderTaskbar();
    }

    function renderIcons() {
        const icons = $('vd-icons');
        const directories = (state.bootstrap && state.bootstrap.workspace && state.bootstrap.workspace.directories) || [];
        const directoryItems = directories.slice(0, 4).map(name => ({ id: 'dir-' + name, name, type: 'directory', path: name }));
        const appItems = allApps().map(app => ({ id: app.id, name: appName(app), type: 'app', app }));
        const items = [...appItems, ...directoryItems];
        icons.innerHTML = items.map(item => {
            const iconKey = item.type === 'directory' ? iconForDirectory(item.name) : iconForApp(item.app);
            const fallback = item.type === 'directory' ? item.name : iconGlyph(item.app);
            return `<button class="vd-icon" type="button" data-kind="${esc(item.type)}" data-id="${esc(item.id)}" data-path="${esc(item.path || '')}">
                ${spriteMarkup(iconKey, fallback, 'vd-sprite-icon', 42)}
                <span class="vd-icon-label">${esc(item.name)}</span>
            </button>`;
        }).join('');
        icons.querySelectorAll('.vd-icon').forEach(btn => {
            btn.addEventListener('dblclick', () => activateDesktopItem(btn));
            btn.addEventListener('click', () => activateDesktopItem(btn));
        });
    }

    function activateDesktopItem(btn) {
        if (btn.dataset.kind === 'directory') {
            openApp('files', { path: btn.dataset.path || '' });
            return;
        }
        openApp(btn.dataset.id);
    }

    function renderWidgets() {
        const host = $('vd-widgets');
        const boot = state.bootstrap || {};
        const widgets = boot.widgets || [];
        const directories = (boot.workspace && boot.workspace.directories) || [];
        const summary = directories.length + ' ' + t('desktop.folder') + ' / ' + (boot.installed_apps || []).length + ' ' + t('desktop.setting_apps');
        const cards = [];
        cards.push(`<article class="vd-widget">
            <div class="vd-widget-head">
                ${spriteMarkup('desktop', 'S', 'vd-sprite-file', 26)}
                <div>
                    <div class="vd-widget-title">${esc(t('desktop.widget_system'))}</div>
                    <div class="vd-widget-body">${esc(summary)}</div>
                </div>
            </div>
        </article>`);
        widgets.slice(0, 4).forEach(widget => {
            const app = allApps().find(item => item.id === widget.app_id);
            const iconKey = widget.icon || (app ? iconForApp(app) : 'widgets');
            const widgetBody = widget.entry && widget.app_id
                ? `<div class="vd-widget-frame-wrap"></div>`
                : `<div class="vd-widget-body">${esc(widget.type || widget.app_id || t('desktop.widget_custom'))}</div>`;
            cards.push(`<article class="vd-widget" data-widget-id="${esc(widget.id)}" data-app-id="${esc(widget.app_id || '')}">
                <div class="vd-widget-head">
                    ${spriteMarkup(iconKey, widget.title || widget.id, 'vd-sprite-file', 26)}
                    <div class="vd-widget-text">
                        <div class="vd-widget-title">${esc(widget.title || widget.id)}</div>
                        ${widget.entry && widget.app_id ? `<div class="vd-widget-kind">${esc(widget.type || widget.app_id || t('desktop.widget_custom'))}</div>` : ''}
                    </div>
                </div>
                ${widgetBody}
            </article>`);
        });
        host.innerHTML = cards.join('');
        widgets.slice(0, 4).forEach(widget => {
            if (!widget.entry || !widget.app_id) return;
            const card = host.querySelector(`[data-widget-id="${widget.id}"] .vd-widget-frame-wrap`);
            if (!card) return;
            const src = desktopFileURL('Apps/' + widget.app_id + '/' + widget.entry) + '?widget_id=' + encodeURIComponent(widget.id);
            card.replaceChildren(makeSandboxedFrame(src, widget.app_id, widget.id, '', 'vd-widget-frame', widget.title || widget.id));
        });
    }

    function renderStartApps() {
        const query = state.startQuery.trim().toLowerCase();
        const apps = allApps().filter(app => !query || appName(app).toLowerCase().includes(query));
        $('vd-start-apps').innerHTML = apps.map(app => `<button class="vd-start-item" type="button" data-app-id="${esc(app.id)}">
            ${spriteMarkup(iconForApp(app), iconGlyph(app), 'vd-sprite-start-item', 30)}
            <span>${esc(appName(app))}</span>
        </button>`).join('');
        $('vd-start-apps').querySelectorAll('[data-app-id]').forEach(btn => {
            btn.addEventListener('click', () => {
                $('vd-start-menu').hidden = true;
                openApp(btn.dataset.appId);
            });
        });
    }

    function renderTaskbar() {
        const host = $('vd-taskbar-apps');
        host.innerHTML = [...state.windows.values()].map(win => `<button type="button" class="vd-task-button ${win.id === state.activeWindowId ? 'active' : ''}" data-window-id="${esc(win.id)}">${esc(win.title)}</button>`).join('');
        host.querySelectorAll('[data-window-id]').forEach(btn => btn.addEventListener('click', () => focusWindow(btn.dataset.windowId)));
    }

    function windowTitle(appId) {
        const app = allApps().find(item => item.id === appId);
        return app ? appName(app) : appId;
    }

    function openApp(appId, context) {
        const existing = [...state.windows.values()].find(win => win.appId === appId && appId !== 'editor');
        if (existing) {
            focusWindow(existing.id);
            if (appId === 'files' && context && context.path != null) renderFiles(existing.id, context.path);
            return;
        }
        const title = windowTitle(appId);
        const id = 'w-' + appId + '-' + Date.now();
        const win = document.createElement('section');
        win.className = 'vd-window';
        win.dataset.windowId = id;
        win.style.left = Math.max(16, 170 + state.windows.size * 28) + 'px';
        win.style.top = Math.max(12, 72 + state.windows.size * 24) + 'px';
        win.style.zIndex = String(++state.z);
        win.innerHTML = `<header class="vd-window-titlebar">
            <div>
                <div class="vd-window-title">${esc(title)}</div>
                <div class="vd-window-subtitle">${esc(t('desktop.window_ready'))}</div>
            </div>
            <div class="vd-window-actions">
                <button class="vd-window-button" type="button" data-action="minimize" title="${esc(t('desktop.minimize'))}">_</button>
                <button class="vd-window-button" type="button" data-action="close" title="${esc(t('desktop.close'))}">x</button>
            </div>
        </header>
        <div class="vd-window-content" data-window-content></div>`;
        $('vd-window-layer').appendChild(win);
        state.windows.set(id, { id, appId, title, element: win });
        wireWindow(win, id);
        focusWindow(id);
        renderAppContent(id, appId, context || {});
        renderTaskbar();
    }

    function wireWindow(win, id) {
        win.addEventListener('pointerdown', () => focusWindow(id));
        win.querySelector('[data-action="close"]').addEventListener('click', () => closeWindow(id));
        win.querySelector('[data-action="minimize"]').addEventListener('click', () => {
            win.style.display = 'none';
            if (state.activeWindowId === id) state.activeWindowId = '';
            renderTaskbar();
        });
        const bar = win.querySelector('.vd-window-titlebar');
        let drag = null;
        bar.addEventListener('pointerdown', (event) => {
            if (event.target.closest('button')) return;
            drag = {
                x: event.clientX,
                y: event.clientY,
                left: parseInt(win.style.left, 10) || 0,
                top: parseInt(win.style.top, 10) || 0
            };
            bar.setPointerCapture(event.pointerId);
        });
        bar.addEventListener('pointermove', (event) => {
            if (!drag) return;
            const maxLeft = window.innerWidth - 80;
            const maxTop = window.innerHeight - 120;
            win.style.left = Math.min(maxLeft, Math.max(8, drag.left + event.clientX - drag.x)) + 'px';
            win.style.top = Math.min(maxTop, Math.max(8, drag.top + event.clientY - drag.y)) + 'px';
        });
        bar.addEventListener('pointerup', () => { drag = null; });
    }

    function focusWindow(id) {
        const win = state.windows.get(id);
        if (!win) return;
        win.element.style.display = '';
        win.element.style.zIndex = String(++state.z);
        state.activeWindowId = id;
        state.windows.forEach(item => item.element.classList.toggle('active', item.id === id));
        renderTaskbar();
    }

    function closeWindow(id) {
        const win = state.windows.get(id);
        if (!win) return;
        win.element.remove();
        state.windows.delete(id);
        if (state.activeWindowId === id) state.activeWindowId = '';
        renderTaskbar();
    }

    function contentEl(id) {
        const win = state.windows.get(id);
        return win && win.element.querySelector('[data-window-content]');
    }

    function renderAppContent(id, appId, context) {
        if (appId === 'files') return renderFiles(id, context.path || '');
        if (appId === 'editor') return renderEditor(id, context.path || 'Documents/untitled.txt', context.content || '');
        if (appId === 'settings') return renderSettings(id);
        if (appId === 'calendar') return renderCalendar(id);
        if (appId === 'agent-chat') return renderChat(id);
        return renderGeneratedApp(id, appId);
    }

    async function renderFiles(id, path) {
        const host = contentEl(id);
        if (!host) return;
        state.filesPath = path || '';
        host.innerHTML = `<div class="vd-panel">
            <div class="vd-toolbar">
                <button class="vd-tool-button" type="button" data-action="up">${esc(t('desktop.up'))}</button>
                <button class="vd-tool-button" type="button" data-action="new-file">${esc(t('desktop.new_file'))}</button>
                <span class="vd-path">${esc(state.filesPath || t('desktop.workspace_root'))}</span>
            </div>
            <div class="vd-file-list">${esc(t('desktop.loading'))}</div>
        </div>`;
        host.querySelector('[data-action="up"]').addEventListener('click', () => {
            const parts = state.filesPath.split('/').filter(Boolean);
            parts.pop();
            renderFiles(id, parts.join('/'));
        });
        host.querySelector('[data-action="new-file"]').addEventListener('click', () => openApp('editor', { path: joinPath(state.filesPath, 'untitled.txt'), content: '' }));
        try {
            const body = await api('/api/desktop/files?path=' + encodeURIComponent(state.filesPath));
            const files = body.files || [];
            host.querySelector('.vd-file-list').innerHTML = files.length ? files.map(file => `<div class="vd-file-row" data-type="${esc(file.type)}" data-path="${esc(file.path)}">
                ${spriteMarkup(iconForFile(file), file.type === 'directory' ? 'D' : file.name, 'vd-sprite-file', 26)}
                <span class="vd-file-name">${esc(file.name)}</span>
                <span class="vd-file-meta">${esc(file.type === 'directory' ? t('desktop.folder') : fmtBytes(file.size))}</span>
            </div>`).join('') : `<div class="vd-empty">${esc(t('desktop.empty_folder'))}</div>`;
            host.querySelectorAll('.vd-file-row').forEach(row => row.addEventListener('click', () => {
                if (row.dataset.type === 'directory') renderFiles(id, row.dataset.path);
                else openEditorFile(row.dataset.path);
            }));
        } catch (err) {
            host.querySelector('.vd-file-list').innerHTML = `<div class="vd-empty">${esc(err.message)}</div>`;
        }
    }

    function joinPath(base, name) {
        return [base, name].filter(Boolean).join('/');
    }

    function openEditorFile(path) {
        openApp('editor', { path });
    }

    async function renderEditor(id, path, initialContent) {
        const host = contentEl(id);
        if (!host) return;
        host.innerHTML = `<div class="vd-editor">
            <div class="vd-toolbar">
                <button class="vd-tool-button" type="button" data-action="save">${esc(t('desktop.save'))}</button>
                <span class="vd-path">${esc(path)}</span>
                <span class="vd-chat-meta" data-status></span>
            </div>
            <textarea spellcheck="false"></textarea>
        </div>`;
        const textarea = host.querySelector('textarea');
        const status = host.querySelector('[data-status]');
        textarea.value = initialContent;
        if (!initialContent) {
            try {
                const body = await api('/api/desktop/file?path=' + encodeURIComponent(path));
                textarea.value = body.content || '';
            } catch (_) {
                textarea.value = '';
            }
        }
        host.querySelector('[data-action="save"]').addEventListener('click', async () => {
            status.textContent = t('desktop.saving');
            try {
                await api('/api/desktop/file', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ path, content: textarea.value })
                });
                status.textContent = t('desktop.saved');
                await loadBootstrap();
            } catch (err) {
                status.textContent = err.message;
            }
        });
    }

    function renderSettings(id) {
        const host = contentEl(id);
        const boot = state.bootstrap || {};
        const workspace = boot.workspace || {};
        const cards = [
            ['desktop.setting_workspace', workspace.root || ''],
            ['desktop.setting_agent_control', boot.allow_agent_control ? t('desktop.on') : t('desktop.off')],
            ['desktop.setting_generated_apps', boot.allow_generated_apps ? t('desktop.on') : t('desktop.off')],
            ['desktop.setting_readonly', boot.readonly ? t('desktop.on') : t('desktop.off')],
            ['desktop.setting_apps', String((boot.installed_apps || []).length)],
            ['desktop.setting_widgets', String((boot.widgets || []).length)]
        ];
        host.innerHTML = `<div class="vd-settings-grid">${cards.map(card => `<div class="vd-setting-card">
            <div class="vd-setting-label">${esc(t(card[0]))}</div>
            <div class="vd-setting-value">${esc(card[1])}</div>
        </div>`).join('')}</div>`;
    }

    function renderCalendar(id) {
        const host = contentEl(id);
        const now = new Date();
        const first = new Date(now.getFullYear(), now.getMonth(), 1);
        const startOffset = (first.getDay() + 6) % 7;
        const days = new Date(now.getFullYear(), now.getMonth() + 1, 0).getDate();
        const cells = [];
        for (let i = 0; i < startOffset; i++) cells.push('');
        for (let day = 1; day <= days; day++) cells.push(String(day));
        host.innerHTML = `<div class="vd-calendar">
            <div class="vd-toolbar"><span class="vd-window-title">${esc(now.toLocaleDateString(undefined, { month: 'long', year: 'numeric' }))}</span></div>
            <div class="vd-calendar-grid">${cells.map(day => `<div class="vd-calendar-cell ${Number(day) === now.getDate() ? 'today' : ''}">${esc(day)}</div>`).join('')}</div>
        </div>`;
    }

    function renderChat(id) {
        const host = contentEl(id);
        host.innerHTML = `<div class="vd-chat">
            <div class="vd-chat-log">
                <div class="vd-chat-bubble agent">${esc(t('desktop.chat_welcome'))}</div>
            </div>
            <form class="vd-chat-form">
                <input class="vd-chat-input" autocomplete="off" data-i18n-placeholder="desktop.chat_placeholder">
                <button class="vd-chat-send" type="submit">${esc(t('desktop.send'))}</button>
            </form>
        </div>`;
        const input = host.querySelector('.vd-chat-input');
        input.placeholder = t('desktop.chat_placeholder');
        host.querySelector('form').addEventListener('submit', async (event) => {
            event.preventDefault();
            if (state.chatBusy) return;
            const message = input.value.trim();
            if (!message) return;
            input.value = '';
            appendChat(host, 'user', message);
            appendChat(host, 'agent', t('desktop.thinking'));
            state.chatBusy = true;
            try {
                const body = await api('/api/desktop/chat', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ message })
                });
                const bubbles = host.querySelectorAll('.vd-chat-bubble.agent');
                bubbles[bubbles.length - 1].textContent = body.answer || t('desktop.done');
                await loadBootstrap();
            } catch (err) {
                const bubbles = host.querySelectorAll('.vd-chat-bubble.agent');
                bubbles[bubbles.length - 1].textContent = err.message;
            } finally {
                state.chatBusy = false;
            }
        });
    }

    function appendChat(host, role, text) {
        const bubble = document.createElement('div');
        bubble.className = 'vd-chat-bubble ' + role;
        bubble.textContent = text;
        host.querySelector('.vd-chat-log').appendChild(bubble);
        bubble.scrollIntoView({ block: 'end' });
    }

    function renderGeneratedApp(id, appId) {
        const host = contentEl(id);
        const app = allApps().find(item => item.id === appId);
        if (!app) {
            host.innerHTML = `<div class="vd-empty">${esc(t('desktop.app_missing'))}</div>`;
            return;
        }
        const src = desktopFileURL('Apps/' + app.id + '/' + app.entry);
        host.replaceChildren(makeSandboxedFrame(src, app.id, '', id, 'vd-generated-frame', appName(app)));
    }

    function makeSandboxedFrame(src, appId, widgetId, windowId, className, title) {
        const iframe = document.createElement('iframe');
        iframe.className = className;
        iframe.title = title || appId || 'Aura Desktop app';
        iframe.src = src;
        iframe.dataset.appId = appId || '';
        iframe.dataset.widgetId = widgetId || '';
        iframe.dataset.windowId = windowId || '';
        iframe.setAttribute('sandbox', 'allow-scripts allow-forms allow-modals allow-popups');
        return iframe;
    }

    function desktopFileURL(path) {
        return '/files/desktop/' + path.split('/').map(encodeURIComponent).join('/');
    }

    function findSDKClient(source) {
        const frames = document.querySelectorAll('.vd-generated-frame, .vd-widget-frame');
        for (const frame of frames) {
            if (frame.contentWindow !== source) continue;
            const app = allApps().find(item => item.id === frame.dataset.appId);
            const widgets = (state.bootstrap && state.bootstrap.widgets) || [];
            const widget = widgets.find(item => item.id === frame.dataset.widgetId);
            return {
                app,
                widget,
                appId: frame.dataset.appId || '',
                widgetId: frame.dataset.widgetId || '',
                windowId: frame.dataset.windowId || ''
            };
        }
        return null;
    }

    function sendSDKResponse(source, id, ok, value) {
        if (!source || !id) return;
        source.postMessage(ok ? {
            type: SDK_RESPONSE_TYPE,
            id,
            ok: true,
            payload: value
        } : {
            type: SDK_RESPONSE_TYPE,
            id,
            ok: false,
            error: value && value.message ? value.message : String(value || 'Desktop bridge request failed')
        }, '*');
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
        if (!client || !client.app) return;
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
                    icon_manifest: state.iconManifest
                };
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
            settings: boot.settings || {}
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
            openApp(event.payload.app_id);
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
        $('vd-start-button').addEventListener('click', () => {
            const menu = $('vd-start-menu');
            menu.hidden = !menu.hidden;
            if (!menu.hidden) $('vd-start-search').focus();
        });
        $('vd-agent-button').addEventListener('click', () => openApp('agent-chat'));
        $('vd-start-search').addEventListener('input', (event) => {
            state.startQuery = event.target.value;
            renderStartApps();
        });
        const startGlyph = $('vd-start-button').querySelector('.vd-start-glyph');
        if (startGlyph) startGlyph.outerHTML = spriteMarkup('apps', 'A', 'vd-sprite-start', 28);
        document.addEventListener('click', (event) => {
            const menu = $('vd-start-menu');
            if (!menu.hidden && !menu.contains(event.target) && !event.target.closest('#vd-start-button')) {
                menu.hidden = true;
            }
        });
        if (window.AuraSSE && typeof window.AuraSSE.on === 'function') {
            window.AuraSSE.on('virtual_desktop_event', handleDesktopEvent);
        }
        window.addEventListener('message', handleSDKMessage);
    }

    async function init() {
        ['vd-icons', 'vd-widgets', 'vd-window-layer', 'vd-taskbar-apps', 'vd-start-apps', 'vd-start-menu', 'vd-start-search', 'vd-ws-state', 'vd-clock', 'vd-workspace', 'vd-disabled'].forEach(id => { els[id] = $(id); });
        await loadIconManifest();
        wireChrome();
        updateClock();
        setInterval(updateClock, 15000);
        await loadBootstrap();
        if (state.bootstrap && state.bootstrap.enabled) connectWS();
    }

    document.addEventListener('DOMContentLoaded', init);
})();
