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
        papirusIconManifest: null,
        iconMap: new Map(),
        selectedIconId: '',
        contextMenu: null
    };

    const SDK_REQUEST_TYPE = 'aurago.desktop.request';
    const SDK_RESPONSE_TYPE = 'aurago.desktop.response';
    const SDK_RUNTIME = 'aura-desktop-sdk@1';
    const ICON_POSITIONS_KEY = 'aurago.desktop.iconPositions.v1';
    const WINDOW_MIN_W = 360;
    const WINDOW_MIN_H = 280;
    const DRAG_THRESHOLD = 5;

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
    const desktopSettingDefaults = {
        'appearance.wallpaper': 'aurora',
        'appearance.accent': 'teal',
        'appearance.density': 'comfortable',
        'appearance.icon_theme': 'papirus',
        'desktop.icon_size': 'medium',
        'desktop.show_widgets': 'true',
        'windows.animations': 'true',
        'windows.default_size': 'balanced',
        'files.confirm_delete': 'true',
        'files.default_folder': 'Documents',
        'agent.show_chat_button': 'true'
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

    function cssSel(value) {
        if (window.CSS && typeof window.CSS.escape === 'function') return CSS.escape(String(value));
        return String(value).replace(/[^a-zA-Z0-9_-]/g, '\\$&');
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
        const [spriteManifest, papirusIconManifest] = await Promise.all([
            api('/img/desktop-icons-sprite.json').catch(() => null),
            api('/img/papirus/manifest.json').catch(() => null)
        ]);
        state.iconManifest = spriteManifest;
        state.iconMap = new Map(((spriteManifest && spriteManifest.icons) || []).map(icon => [icon.name, icon]));
        state.papirusIconManifest = papirusIconManifest;
    }

    function iconExists(key) {
        return key && state.iconMap.has(key);
    }

    function normalizeIconName(name) {
        return String(name || '').trim().toLowerCase().replace(/[^a-z0-9:_-]+/g, '_');
    }

    function papirusIconPath(key) {
        const manifest = state.papirusIconManifest;
        if (!manifest || !manifest.icons) return '';
        let normalized = normalizeIconName(key);
        if (!normalized || normalized.startsWith('sprite:')) return '';
        if (normalized.startsWith('papirus:')) normalized = normalized.slice('papirus:'.length);
        const aliases = manifest.aliases || {};
        const candidates = [
            normalized,
            aliases[normalized],
            normalized.replaceAll('_', '-'),
            aliases[normalized.replaceAll('_', '-')]
        ].filter(Boolean);
        for (const candidate of candidates) {
            if (manifest.icons[candidate]) return '/' + String(manifest.icons[candidate]).replace(/^\/+/, '');
        }
        return '';
    }

    function resolveIconSource(key) {
        const normalized = normalizeIconName(key);
        if (!normalized) return { type: 'fallback' };
        if (settingValue('appearance.icon_theme') !== 'aurago' && !normalized.startsWith('sprite:')) {
            const papirusPath = papirusIconPath(normalized);
            if (papirusPath) return { type: 'papirus', path: papirusPath };
        }
        const spriteKey = normalized.startsWith('sprite:') ? normalized.slice('sprite:'.length) : normalized;
        return iconExists(spriteKey) ? { type: 'sprite', key: spriteKey } : { type: 'fallback' };
    }

    function spriteMarkup(key, fallback, className, size) {
        const manifest = state.iconManifest;
        const spriteKey = normalizeIconName(key).replace(/^sprite:/, '');
        const icon = iconExists(spriteKey) ? state.iconMap.get(spriteKey) : null;
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

    function iconMarkup(key, fallback, className, size) {
        const source = resolveIconSource(key);
        if (source.type === 'papirus') {
            const pixels = Number(size || 42) || 42;
            return `<span class="${esc(className)} vd-papirus-icon" aria-hidden="true" style="--vd-papirus-url:url(${esc(source.path)});width:${pixels}px;height:${pixels}px"></span>`;
        }
        return spriteMarkup(source.key || key, fallback, className, size);
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

    function desktopSettings() {
        return Object.assign({}, desktopSettingDefaults, (state.bootstrap && state.bootstrap.settings) || {});
    }

    function settingValue(key) {
        const settings = desktopSettings();
        return settings[key] != null ? settings[key] : desktopSettingDefaults[key];
    }

    function settingBool(key) {
        return settingValue(key) !== 'false';
    }

    function applyDesktopSettings() {
        const body = document.body;
        body.dataset.wallpaper = settingValue('appearance.wallpaper');
        body.dataset.accent = settingValue('appearance.accent');
        body.dataset.density = settingValue('appearance.density');
        body.dataset.iconTheme = settingValue('appearance.icon_theme');
        body.dataset.animations = settingValue('windows.animations');
        body.dataset.widgets = settingValue('desktop.show_widgets');
        body.dataset.iconSize = settingValue('desktop.icon_size');
        const sizes = { small: 34, medium: 42, large: 52 };
        body.style.setProperty('--vd-icon-glyph-size', (sizes[settingValue('desktop.icon_size')] || 42) + 'px');
        const agentButton = $('vd-agent-button');
        if (agentButton) agentButton.hidden = !settingBool('agent.show_chat_button');
    }

    function iconGlyphPixels() {
        const sizes = { small: 34, medium: 42, large: 52 };
        return sizes[settingValue('desktop.icon_size')] || 42;
    }

    function defaultWindowSize() {
        const workspace = $('vd-window-layer') || $('vd-workspace');
        const w = (workspace && workspace.clientWidth) || window.innerWidth;
        const h = (workspace && workspace.clientHeight) || window.innerHeight;
        const preset = settingValue('windows.default_size');
        const ratios = preset === 'compact' ? [0.55, 0.62] : preset === 'large' ? [0.82, 0.86] : [0.68, 0.76];
        return {
            width: Math.round(Math.min(980, Math.max(WINDOW_MIN_W, w * ratios[0]))),
            height: Math.round(Math.min(680, Math.max(WINDOW_MIN_H, h * ratios[1])))
        };
    }

    function readJSONStorage(key, fallback) {
        try {
            const raw = localStorage.getItem(key);
            return raw ? JSON.parse(raw) : fallback;
        } catch (_) {
            return fallback;
        }
    }

    function writeJSONStorage(key, value) {
        try { localStorage.setItem(key, JSON.stringify(value)); } catch (_) { }
    }

    function iconPositions() {
        return readJSONStorage(ICON_POSITIONS_KEY, {});
    }

    function saveIconPosition(id, x, y) {
        const positions = iconPositions();
        positions[id] = { x: Math.round(x), y: Math.round(y) };
        writeJSONStorage(ICON_POSITIONS_KEY, positions);
    }

    function defaultIconPosition(index) {
        const cellW = 92;
        const cellH = 104;
        const workspace = $('vd-workspace');
        const availableH = Math.max(320, (workspace && workspace.clientHeight) || 520);
        const rows = Math.max(1, Math.floor((availableH - 36) / cellH));
        const col = Math.floor(index / rows);
        const row = index % rows;
        return { x: 18 + col * cellW, y: 18 + row * cellH };
    }

    function clampToWorkspace(x, y, w, h) {
        const workspace = $('vd-workspace');
        const maxX = Math.max(8, ((workspace && workspace.clientWidth) || window.innerWidth) - (w || 90) - 8);
        const maxY = Math.max(8, ((workspace && workspace.clientHeight) || window.innerHeight) - (h || 90) - 8);
        return {
            x: Math.min(maxX, Math.max(8, x)),
            y: Math.min(maxY, Math.max(8, y))
        };
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
        applyDesktopSettings();
        renderStartButtonIcon();
        renderIcons();
        renderWidgets();
        renderStartApps();
        renderTaskbar();
    }

    function renderStartButtonIcon() {
        const startButton = $('vd-start-button');
        if (!startButton) return;
        const startGlyph = startButton.querySelector('.vd-start-glyph, .vd-sprite-start, .vd-papirus-icon');
        if (startGlyph) startGlyph.outerHTML = iconMarkup('apps', 'A', 'vd-sprite-start', 28);
    }

    function renderIcons() {
        const icons = $('vd-icons');
        const directories = (state.bootstrap && state.bootstrap.workspace && state.bootstrap.workspace.directories) || [];
        const directoryItems = directories.slice(0, 4).map(name => ({ id: 'dir-' + name, name, type: 'directory', path: name }));
        const appItems = allApps().map(app => ({ id: app.id, name: appName(app), type: 'app', app }));
        const items = [...appItems, ...directoryItems];
        const positions = iconPositions();
        icons.innerHTML = items.map(item => {
            const iconKey = item.type === 'directory' ? iconForDirectory(item.name) : iconForApp(item.app);
            const fallback = item.type === 'directory' ? item.name : iconGlyph(item.app);
            const pos = positions[item.id] || defaultIconPosition(items.indexOf(item));
            return `<button class="vd-icon ${item.id === state.selectedIconId ? 'selected' : ''}" type="button" data-kind="${esc(item.type)}" data-id="${esc(item.id)}" data-path="${esc(item.path || '')}" style="left:${Number(pos.x) || 18}px;top:${Number(pos.y) || 18}px">
                ${iconMarkup(iconKey, fallback, 'vd-sprite-icon', iconGlyphPixels())}
                <span class="vd-icon-label">${esc(item.name)}</span>
            </button>`;
        }).join('');
        icons.querySelectorAll('.vd-icon').forEach(btn => {
            btn.addEventListener('dblclick', () => activateDesktopItem(btn));
            btn.addEventListener('click', () => selectDesktopIcon(btn));
            btn.addEventListener('contextmenu', event => showIconContextMenu(event, btn));
            wireDraggableIcon(btn);
        });
    }

    function selectDesktopIcon(btn) {
        state.selectedIconId = btn ? btn.dataset.id : '';
        document.querySelectorAll('.vd-icon').forEach(icon => icon.classList.toggle('selected', icon === btn));
    }

    function wireDraggableIcon(btn) {
        let drag = null;
        btn.addEventListener('pointerdown', event => {
            if (event.button !== 0) return;
            closeContextMenu();
            selectDesktopIcon(btn);
            drag = {
                id: btn.dataset.id,
                pointerId: event.pointerId,
                x: event.clientX,
                y: event.clientY,
                left: parseInt(btn.style.left, 10) || 0,
                top: parseInt(btn.style.top, 10) || 0,
                moved: false
            };
            btn.setPointerCapture(event.pointerId);
        });
        btn.addEventListener('pointermove', event => {
            if (!drag) return;
            const dx = event.clientX - drag.x;
            const dy = event.clientY - drag.y;
            if (!drag.moved && Math.hypot(dx, dy) < DRAG_THRESHOLD) return;
            drag.moved = true;
            btn.classList.add('vd-dragging');
            const pos = clampToWorkspace(drag.left + dx, drag.top + dy, btn.offsetWidth, btn.offsetHeight);
            btn.style.left = pos.x + 'px';
            btn.style.top = pos.y + 'px';
        });
        btn.addEventListener('pointerup', event => {
            if (!drag) return;
            btn.releasePointerCapture(event.pointerId);
            btn.classList.remove('vd-dragging');
            if (drag.moved) {
                saveIconPosition(drag.id, parseInt(btn.style.left, 10) || 0, parseInt(btn.style.top, 10) || 0);
                event.preventDefault();
            }
            drag = null;
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
        const systemBounds = defaultWidgetBounds(0);
        cards.push(`<article class="vd-widget vd-widget-system" style="left:${systemBounds.x}px;top:${systemBounds.y}px;width:${systemBounds.w}px;height:${systemBounds.h}px">
            <div class="vd-widget-head">
                ${iconMarkup('desktop', 'S', 'vd-sprite-file', 26)}
                <div>
                    <div class="vd-widget-title">${esc(t('desktop.widget_system'))}</div>
                    <div class="vd-widget-body">${esc(summary)}</div>
                </div>
            </div>
        </article>`);
        widgets.slice(0, 4).forEach((widget, index) => {
            const app = allApps().find(item => item.id === widget.app_id);
            const iconKey = widget.icon || (app ? iconForApp(app) : 'widgets');
            const bounds = widgetBounds(widget, index + 1);
            const widgetBody = widget.entry
                ? `<div class="vd-widget-frame-wrap"></div>`
                : `<div class="vd-widget-body">${esc(widget.type || widget.app_id || t('desktop.widget_custom'))}</div>`;
            cards.push(`<article class="vd-widget" data-widget-id="${esc(widget.id)}" data-app-id="${esc(widget.app_id || '')}" style="left:${bounds.x}px;top:${bounds.y}px;width:${bounds.w}px;height:${bounds.h}px">
                <div class="vd-widget-head">
                    ${iconMarkup(iconKey, widget.title || widget.id, 'vd-sprite-file', 26)}
                    <div class="vd-widget-text">
                        <div class="vd-widget-title">${esc(widget.title || widget.id)}</div>
                        ${widget.entry ? `<div class="vd-widget-kind">${esc(widget.type || widget.app_id || t('desktop.widget_custom'))}</div>` : ''}
                    </div>
                </div>
                ${widgetBody}
            </article>`);
        });
        host.innerHTML = cards.join('');
        widgets.slice(0, 4).forEach(widget => {
            if (!widget.entry) return;
            const card = host.querySelector(`[data-widget-id="${widget.id}"] .vd-widget-frame-wrap`);
            if (!card) return;
                renderWidgetFrame(card, widget);
        });
        widgets.slice(0, 4).forEach(widget => {
            const card = host.querySelector(`[data-widget-id="${cssSel(widget.id)}"]`);
            if (!card) return;
            card.addEventListener('contextmenu', event => showWidgetContextMenu(event, widget));
            wireDraggableWidget(card, widget);
        });
    }

    function defaultWidgetBounds(index) {
        const workspace = $('vd-workspace');
        const width = 320;
        const height = 132;
        const x = Math.max(18, ((workspace && workspace.clientWidth) || window.innerWidth) - width - 18);
        return { x, y: 18 + index * (height + 12), w: width, h: height };
    }

    function widgetBounds(widget, index) {
        const fallback = defaultWidgetBounds(index);
        const w = Number(widget.w || widget.W || 0);
        const h = Number(widget.h || widget.H || 0);
        return {
            x: Number(widget.x || widget.X || fallback.x) || fallback.x,
            y: Number(widget.y || widget.Y || fallback.y) || fallback.y,
            w: w > 16 ? w : Math.max(240, w * 160 || fallback.w),
            h: h > 16 ? h : Math.max(104, h * 86 || fallback.h)
        };
    }

    function wireDraggableWidget(card, widget) {
        const handle = card.querySelector('.vd-widget-head') || card;
        let drag = null;
        handle.addEventListener('pointerdown', event => {
            if (event.button !== 0) return;
            drag = {
                x: event.clientX,
                y: event.clientY,
                left: parseInt(card.style.left, 10) || 0,
                top: parseInt(card.style.top, 10) || 0,
                moved: false
            };
            handle.setPointerCapture(event.pointerId);
        });
        handle.addEventListener('pointermove', event => {
            if (!drag) return;
            const dx = event.clientX - drag.x;
            const dy = event.clientY - drag.y;
            if (!drag.moved && Math.hypot(dx, dy) < DRAG_THRESHOLD) return;
            drag.moved = true;
            card.classList.add('vd-dragging');
            const pos = clampToWorkspace(drag.left + dx, drag.top + dy, card.offsetWidth, card.offsetHeight);
            card.style.left = pos.x + 'px';
            card.style.top = pos.y + 'px';
        });
        handle.addEventListener('pointerup', async event => {
            if (!drag) return;
            handle.releasePointerCapture(event.pointerId);
            card.classList.remove('vd-dragging');
            if (drag.moved) {
                await persistWidgetBounds(widget, card);
                event.preventDefault();
            }
            drag = null;
        });
    }

    async function persistWidgetBounds(widget, card) {
        const updated = Object.assign({}, widget, {
            x: parseInt(card.style.left, 10) || 0,
            y: parseInt(card.style.top, 10) || 0,
            w: Math.round(card.offsetWidth),
            h: Math.round(card.offsetHeight)
        });
        try {
            await api('/api/desktop/widgets', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(updated)
            });
            await loadBootstrap();
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err.message });
        }
    }

    async function renderWidgetFrame(card, widget) {
        card.innerHTML = `<div class="vd-widget-body">${esc(t('desktop.loading'))}</div>`;
        const path = widgetFramePath(widget);
        try {
            const src = await desktopEmbedURL(path, { widget_id: widget.id });
            await ensureDesktopEmbedHasContent(src);
            card.replaceChildren(makeSandboxedFrame(src, widget.app_id, widget.id, '', 'vd-widget-frame', widget.title || widget.id));
        } catch (err) {
            card.innerHTML = `<div class="vd-widget-body">${esc(err.message)}</div>`;
        }
    }

    function widgetFramePath(widget) {
        return widget.app_id
            ? 'Apps/' + widget.app_id + '/' + widget.entry
            : 'Widgets/' + widget.entry;
    }

    function renderStartApps() {
        const query = state.startQuery.trim().toLowerCase();
        const apps = allApps().filter(app => !query || appName(app).toLowerCase().includes(query));
        $('vd-start-apps').innerHTML = apps.map(app => `<button class="vd-start-item" type="button" data-app-id="${esc(app.id)}">
            ${iconMarkup(iconForApp(app), iconGlyph(app), 'vd-sprite-start-item', 30)}
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
        host.querySelectorAll('[data-window-id]').forEach(btn => {
            btn.addEventListener('click', () => focusWindow(btn.dataset.windowId));
            btn.addEventListener('contextmenu', event => showWindowContextMenu(event, btn.dataset.windowId));
        });
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
        const size = defaultWindowSize();
        win.style.width = size.width + 'px';
        win.style.height = size.height + 'px';
        win.style.zIndex = String(++state.z);
        win.innerHTML = `<header class="vd-window-titlebar">
            <div>
                <div class="vd-window-title">${esc(title)}</div>
                <div class="vd-window-subtitle">${esc(t('desktop.window_ready'))}</div>
            </div>
            <div class="vd-window-actions">
                <button class="vd-window-button" type="button" data-action="minimize" title="${esc(t('desktop.minimize'))}">_</button>
                <button class="vd-window-button" type="button" data-action="maximize" title="${esc(t('desktop.maximize'))}">□</button>
                <button class="vd-window-button" type="button" data-action="close" title="${esc(t('desktop.close'))}">x</button>
            </div>
        </header>
        <div class="vd-window-content" data-window-content></div>
        ${resizeHandleMarkup()}`;
        $('vd-window-layer').appendChild(win);
        state.windows.set(id, { id, appId, title, element: win, maximized: false, restoreBounds: null });
        wireWindow(win, id);
        focusWindow(id);
        renderAppContent(id, appId, context || {});
        renderTaskbar();
    }

    function resizeHandleMarkup() {
        return ['n', 's', 'e', 'w', 'ne', 'nw', 'se', 'sw']
            .map(edge => `<span class="vd-resize-handle vd-resize-${edge}" data-resize="${edge}"></span>`)
            .join('');
    }

    function wireWindow(win, id) {
        win.addEventListener('pointerdown', () => focusWindow(id));
        win.addEventListener('contextmenu', event => {
            if (event.target.closest('.vd-window-titlebar')) showWindowContextMenu(event, id);
        });
        win.querySelector('[data-action="close"]').addEventListener('click', () => closeWindow(id));
        win.querySelector('[data-action="minimize"]').addEventListener('click', () => {
            win.style.display = 'none';
            if (state.activeWindowId === id) state.activeWindowId = '';
            renderTaskbar();
        });
        win.querySelector('[data-action="maximize"]').addEventListener('click', () => toggleMaximizeWindow(id));
        const bar = win.querySelector('.vd-window-titlebar');
        let drag = null;
        bar.addEventListener('pointerdown', (event) => {
            if (event.target.closest('button')) return;
            if (state.windows.get(id) && state.windows.get(id).maximized) return;
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
        bar.addEventListener('dblclick', event => {
            if (event.target.closest('button')) return;
            toggleMaximizeWindow(id);
        });
        wireWindowResize(win, id);
    }

    function windowBounds(win) {
        return {
            left: parseInt(win.style.left, 10) || 0,
            top: parseInt(win.style.top, 10) || 0,
            width: Math.round(win.offsetWidth),
            height: Math.round(win.offsetHeight)
        };
    }

    function workspaceBoundsForWindow() {
        const layer = $('vd-window-layer');
        return { width: layer.clientWidth, height: layer.clientHeight };
    }

    function toggleMaximizeWindow(id) {
        const item = state.windows.get(id);
        if (!item) return;
        const win = item.element;
        if (item.maximized) {
            const b = item.restoreBounds || { left: 80, top: 48, width: 820, height: 560 };
            win.classList.remove('maximized');
            win.style.left = b.left + 'px';
            win.style.top = b.top + 'px';
            win.style.width = b.width + 'px';
            win.style.height = b.height + 'px';
            item.maximized = false;
        } else {
            item.restoreBounds = windowBounds(win);
            const bounds = workspaceBoundsForWindow();
            win.classList.add('maximized');
            win.style.left = '8px';
            win.style.top = '8px';
            win.style.width = Math.max(WINDOW_MIN_W, bounds.width - 16) + 'px';
            win.style.height = Math.max(WINDOW_MIN_H, bounds.height - 16) + 'px';
            item.maximized = true;
        }
        focusWindow(id);
    }

    function wireWindowResize(win, id) {
        win.querySelectorAll('[data-resize]').forEach(handle => {
            let resize = null;
            handle.addEventListener('pointerdown', event => {
                const item = state.windows.get(id);
                if (item && item.maximized) return;
                event.preventDefault();
                event.stopPropagation();
                focusWindow(id);
                resize = {
                    edge: handle.dataset.resize,
                    x: event.clientX,
                    y: event.clientY,
                    bounds: windowBounds(win)
                };
                handle.setPointerCapture(event.pointerId);
            });
            handle.addEventListener('pointermove', event => {
                if (!resize) return;
                const dx = event.clientX - resize.x;
                const dy = event.clientY - resize.y;
                applyResize(win, resize.edge, resize.bounds, dx, dy);
            });
            handle.addEventListener('pointerup', event => {
                if (!resize) return;
                handle.releasePointerCapture(event.pointerId);
                resize = null;
            });
        });
    }

    function applyResize(win, edge, start, dx, dy) {
        const workspace = workspaceBoundsForWindow();
        let left = start.left;
        let top = start.top;
        let width = start.width;
        let height = start.height;
        if (edge.includes('e')) width = Math.max(WINDOW_MIN_W, start.width + dx);
        if (edge.includes('s')) height = Math.max(WINDOW_MIN_H, start.height + dy);
        if (edge.includes('w')) {
            width = Math.max(WINDOW_MIN_W, start.width - dx);
            left = start.left + (start.width - width);
        }
        if (edge.includes('n')) {
            height = Math.max(WINDOW_MIN_H, start.height - dy);
            top = start.top + (start.height - height);
        }
        left = Math.max(8, Math.min(left, workspace.width - 80));
        top = Math.max(8, Math.min(top, workspace.height - 80));
        width = Math.min(width, workspace.width - left - 8);
        height = Math.min(height, workspace.height - top - 8);
        win.style.left = left + 'px';
        win.style.top = top + 'px';
        win.style.width = width + 'px';
        win.style.height = height + 'px';
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

    function closeContextMenu() {
        if (state.contextMenu) {
            state.contextMenu.remove();
            state.contextMenu = null;
        }
    }

    function showContextMenu(x, y, items) {
        closeContextMenu();
        const menu = document.createElement('div');
        menu.className = 'vd-context-menu';
        menu.setAttribute('role', 'menu');
        menu.innerHTML = items.map((item, index) => item.separator
            ? '<div class="vd-context-separator" role="separator"></div>'
            : `<button type="button" class="vd-context-item" role="menuitem" data-index="${index}" ${item.disabled ? 'disabled' : ''}>
                <span class="vd-context-icon">${esc(item.icon || '')}</span>
                <span>${esc(item.label)}</span>
            </button>`).join('');
        $('vd-workspace').appendChild(menu);
        const rect = menu.getBoundingClientRect();
        const workspace = $('vd-workspace').getBoundingClientRect();
        menu.style.left = Math.max(8, Math.min(x - workspace.left, workspace.width - rect.width - 8)) + 'px';
        menu.style.top = Math.max(8, Math.min(y - workspace.top, workspace.height - rect.height - 8)) + 'px';
        menu.querySelectorAll('[data-index]').forEach(btn => {
            btn.addEventListener('click', () => {
                const item = items[Number(btn.dataset.index)];
                closeContextMenu();
                if (item && item.action) item.action();
            });
        });
        state.contextMenu = menu;
    }

    function showDesktopContextMenu(event) {
        if (event.target.closest('.vd-icon, .vd-widget, .vd-window, .vd-start-menu')) return;
        event.preventDefault();
        selectDesktopIcon(null);
        showContextMenu(event.clientX, event.clientY, [
            { label: t('desktop.context_new_file'), icon: '+', action: () => createFileInPath('Desktop') },
            { label: t('desktop.context_new_folder'), icon: '▣', action: () => createFolderInPath('Desktop') },
            { separator: true },
            { label: t('desktop.context_refresh'), icon: '↻', action: () => loadBootstrap() },
            { label: t('desktop.context_sort_icons'), icon: '⇄', action: autoArrangeIcons }
        ]);
    }

    function showIconContextMenu(event, btn) {
        event.preventDefault();
        selectDesktopIcon(btn);
        const isDirectory = btn.dataset.kind === 'directory';
        const path = btn.dataset.path || '';
        const items = [
            { label: t('desktop.context_open'), icon: '↗', action: () => activateDesktopItem(btn) },
            { label: t('desktop.context_rename'), icon: '✎', disabled: !isDirectory, action: () => renamePath(path) },
            { label: t('desktop.context_delete'), icon: '×', disabled: !isDirectory, action: () => deletePath(path) },
            { separator: true },
            { label: t('desktop.context_properties'), icon: 'i', action: () => showProperties(btn.querySelector('.vd-icon-label').textContent, path || btn.dataset.id) }
        ];
        showContextMenu(event.clientX, event.clientY, items);
    }

    function showWidgetContextMenu(event, widget) {
        event.preventDefault();
        showContextMenu(event.clientX, event.clientY, [
            { label: t('desktop.context_open'), icon: '↗', action: () => widget.app_id && openApp(widget.app_id) },
            { label: t('desktop.context_remove_widget'), icon: '×', action: () => deleteWidget(widget.id) }
        ]);
    }

    function showWindowContextMenu(event, id) {
        event.preventDefault();
        const item = state.windows.get(id);
        if (!item) return;
        showContextMenu(event.clientX, event.clientY, [
            { label: t('desktop.context_restore'), icon: '▣', action: () => focusWindow(id) },
            { label: t('desktop.context_minimize'), icon: '_', action: () => { item.element.style.display = 'none'; renderTaskbar(); } },
            { label: item.maximized ? t('desktop.restore') : t('desktop.context_maximize'), icon: '□', action: () => toggleMaximizeWindow(id) },
            { separator: true },
            { label: t('desktop.context_close'), icon: '×', action: () => closeWindow(id) }
        ]);
    }

    function autoArrangeIcons() {
        const icons = [...document.querySelectorAll('.vd-icon')];
        icons.forEach((icon, index) => {
            const pos = defaultIconPosition(index);
            icon.style.left = pos.x + 'px';
            icon.style.top = pos.y + 'px';
            saveIconPosition(icon.dataset.id, pos.x, pos.y);
        });
    }

    function pathDir(path) {
        const parts = String(path || '').split('/').filter(Boolean);
        parts.pop();
        return parts.join('/');
    }

    async function promptDialog(title, value) {
        return modalDialog({ title, input: true, value: value || '' });
    }

    async function confirmDialog(title, message) {
        return modalDialog({ title, message, confirmOnly: true });
    }

    function modalDialog(options) {
        closeContextMenu();
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
        if (input) {
            input.focus();
            input.select();
        }
        return new Promise(resolve => {
            const finish = value => {
                overlay.remove();
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
        openApp('editor', { path: joinPath(basePath, name), content: '' });
    }

    async function createFolderInPath(basePath) {
        const name = await promptDialog(t('desktop.new_folder'), 'New Folder');
        if (!name) return;
        try {
            await api('/api/desktop/directory', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ path: joinPath(basePath, name) })
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
                body: JSON.stringify({ old_path: path, new_path: joinPath(pathDir(path), name) })
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

    function showProperties(title, body) {
        showDesktopNotification({ title: title || t('desktop.context_properties'), message: body || '' });
    }

    function contentEl(id) {
        const win = state.windows.get(id);
        return win && win.element.querySelector('[data-window-content]');
    }

    function renderAppContent(id, appId, context) {
        if (appId === 'files') return renderFiles(id, context.path || settingValue('files.default_folder') || '');
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
                <button class="vd-tool-button" type="button" data-action="new-folder">${esc(t('desktop.new_folder'))}</button>
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
        host.querySelector('[data-action="new-folder"]').addEventListener('click', () => createFolderInPath(state.filesPath));
        try {
            const body = await api('/api/desktop/files?path=' + encodeURIComponent(state.filesPath));
            const files = body.files || [];
            host.querySelector('.vd-file-list').innerHTML = files.length ? files.map(file => `<div class="vd-file-row" data-type="${esc(file.type)}" data-path="${esc(file.path)}">
                ${iconMarkup(iconForFile(file), file.type === 'directory' ? 'D' : file.name, 'vd-sprite-file', 26)}
                <span class="vd-file-name">${esc(file.name)}</span>
                <span class="vd-file-meta">${esc(file.type === 'directory' ? t('desktop.folder') : fmtBytes(file.size))}</span>
            </div>`).join('') : `<div class="vd-empty">${esc(t('desktop.empty_folder'))}</div>`;
            host.querySelectorAll('.vd-file-row').forEach(row => {
                row.addEventListener('dblclick', () => {
                    if (row.dataset.type === 'directory') renderFiles(id, row.dataset.path);
                    else openEditorFile(row.dataset.path);
                });
                row.addEventListener('click', () => {
                    host.querySelectorAll('.vd-file-row').forEach(item => item.classList.toggle('selected', item === row));
                });
                row.addEventListener('contextmenu', event => {
                    event.preventDefault();
                    showContextMenu(event.clientX, event.clientY, [
                        { label: t('desktop.context_open'), icon: '↗', action: () => row.dataset.type === 'directory' ? renderFiles(id, row.dataset.path) : openEditorFile(row.dataset.path) },
                        { label: t('desktop.context_rename'), icon: '✎', action: () => renamePath(row.dataset.path) },
                        { label: t('desktop.context_delete'), icon: '×', action: () => deletePath(row.dataset.path) },
                        { separator: true },
                        { label: t('desktop.context_properties'), icon: 'i', action: () => showProperties(row.querySelector('.vd-file-name').textContent, row.dataset.path) }
                    ]);
                });
            });
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

    function settingsSections() {
        const boot = state.bootstrap || {};
        const workspace = boot.workspace || {};
        return [
            {
                id: 'appearance', icon: '◐', title: 'desktop.settings_category_appearance', desc: 'desktop.settings_category_appearance_desc', items: [
                    settingSelect('appearance.wallpaper', 'desktop.settings_wallpaper', 'desktop.settings_wallpaper_desc', [
                        ['aurora', 'desktop.settings_wallpaper_aurora'], ['midnight', 'desktop.settings_wallpaper_midnight'], ['slate', 'desktop.settings_wallpaper_slate'], ['ember', 'desktop.settings_wallpaper_ember'], ['forest', 'desktop.settings_wallpaper_forest']
                    ]),
                    settingSelect('appearance.accent', 'desktop.settings_accent', 'desktop.settings_accent_desc', [
                        ['teal', 'desktop.settings_accent_teal'], ['orange', 'desktop.settings_accent_orange'], ['blue', 'desktop.settings_accent_blue'], ['violet', 'desktop.settings_accent_violet'], ['green', 'desktop.settings_accent_green']
                    ]),
                    settingSelect('appearance.density', 'desktop.settings_density', 'desktop.settings_density_desc', [
                        ['comfortable', 'desktop.settings_density_comfortable'], ['compact', 'desktop.settings_density_compact']
                    ]),
                    settingSelect('appearance.icon_theme', 'desktop.settings_icon_theme', 'desktop.settings_icon_theme_desc', [
                        ['papirus', 'desktop.settings_icon_theme_papirus'], ['aurago', 'desktop.settings_icon_theme_aurago']
                    ])
                ]
            },
            {
                id: 'desktop', icon: '▦', title: 'desktop.settings_category_desktop', desc: 'desktop.settings_category_desktop_desc', items: [
                    settingSelect('desktop.icon_size', 'desktop.settings_icon_size', 'desktop.settings_icon_size_desc', [
                        ['small', 'desktop.settings_icon_size_small'], ['medium', 'desktop.settings_icon_size_medium'], ['large', 'desktop.settings_icon_size_large']
                    ]),
                    settingToggle('desktop.show_widgets', 'desktop.settings_show_widgets', 'desktop.settings_show_widgets_desc')
                ]
            },
            {
                id: 'windows', icon: '▣', title: 'desktop.settings_category_windows', desc: 'desktop.settings_category_windows_desc', items: [
                    settingToggle('windows.animations', 'desktop.settings_window_animations', 'desktop.settings_window_animations_desc'),
                    settingSelect('windows.default_size', 'desktop.settings_default_window_size', 'desktop.settings_default_window_size_desc', [
                        ['compact', 'desktop.settings_window_size_compact'], ['balanced', 'desktop.settings_window_size_balanced'], ['large', 'desktop.settings_window_size_large']
                    ])
                ]
            },
            {
                id: 'files', icon: '▤', title: 'desktop.settings_category_files', desc: 'desktop.settings_category_files_desc', items: [
                    settingToggle('files.confirm_delete', 'desktop.settings_confirm_delete', 'desktop.settings_confirm_delete_desc'),
                    settingSelect('files.default_folder', 'desktop.settings_default_folder', 'desktop.settings_default_folder_desc', [
                        ['Desktop', 'desktop.settings_folder_desktop'], ['Documents', 'desktop.settings_folder_documents'], ['Downloads', 'desktop.settings_folder_downloads'], ['Pictures', 'desktop.settings_folder_pictures'], ['Shared', 'desktop.settings_folder_shared']
                    ])
                ]
            },
            {
                id: 'agent', icon: '✦', title: 'desktop.settings_category_agent', desc: 'desktop.settings_category_agent_desc', items: [
                    settingToggle('agent.show_chat_button', 'desktop.settings_show_agent_button', 'desktop.settings_show_agent_button_desc'),
                    settingInfo('desktop.setting_agent_control', boot.allow_agent_control ? t('desktop.on') : t('desktop.off'))
                ]
            },
            {
                id: 'system', icon: 'ⓘ', title: 'desktop.settings_category_system', desc: 'desktop.settings_category_system_desc', items: [
                    settingInfo('desktop.setting_workspace', workspace.root || ''),
                    settingInfo('desktop.setting_readonly', boot.readonly ? t('desktop.on') : t('desktop.off')),
                    settingInfo('desktop.setting_apps', String((boot.installed_apps || []).length)),
                    settingInfo('desktop.setting_widgets', String((boot.widgets || []).length))
                ]
            }
        ];
    }

    function settingSelect(key, label, desc, options) {
        return { type: 'select', key, label, desc, options };
    }

    function settingToggle(key, label, desc) {
        return { type: 'toggle', key, label, desc };
    }

    function settingInfo(label, value) {
        return { type: 'info', label, value };
    }

    function renderSettings(id) {
        const host = contentEl(id);
        if (!host) return;
        host.dataset.activeSettings = host.dataset.activeSettings || 'appearance';
        renderSettingsShell(host);
    }

    function renderSettingsShell(host) {
        const sections = settingsSections();
        const active = sections.find(section => section.id === host.dataset.activeSettings) || sections[0];
        host.innerHTML = `<div class="vd-settings-app">
            <aside class="vd-settings-sidebar" aria-label="${esc(t('desktop.app_settings'))}">
                <div class="vd-settings-sidebar-title">${esc(t('desktop.app_settings'))}</div>
                ${sections.map(section => `<button type="button" class="vd-settings-nav ${section.id === active.id ? 'active' : ''}" data-section="${esc(section.id)}">
                    <span>${esc(section.icon)}</span><span>${esc(t(section.title))}</span>
                </button>`).join('')}
            </aside>
            <section class="vd-settings-pane">
                <div class="vd-settings-pane-head">
                    <div class="vd-settings-pane-icon">${esc(active.icon)}</div>
                    <div>
                        <div class="vd-settings-pane-title">${esc(t(active.title))}</div>
                        <div class="vd-settings-pane-desc">${esc(t(active.desc))}</div>
                    </div>
                </div>
                <div class="vd-settings-list">${active.items.map(renderSettingItem).join('')}</div>
            </section>
        </div>`;
        host.querySelectorAll('[data-section]').forEach(btn => btn.addEventListener('click', () => {
            host.dataset.activeSettings = btn.dataset.section;
            renderSettingsShell(host);
        }));
        host.querySelectorAll('[data-setting-key]').forEach(control => {
            control.addEventListener('change', async event => {
                const key = event.currentTarget.dataset.settingKey;
                const value = event.currentTarget.type === 'checkbox' ? String(event.currentTarget.checked) : event.currentTarget.value;
                await saveDesktopSetting(key, value, host);
            });
        });
    }

    function renderSettingItem(item) {
        if (item.type === 'info') {
            return `<article class="vd-setting-row readonly">
                <div><div class="vd-setting-label">${esc(t(item.label))}</div></div>
                <div class="vd-setting-value">${esc(item.value)}</div>
            </article>`;
        }
        const control = item.type === 'toggle'
            ? `<label class="vd-switch"><input type="checkbox" data-setting-key="${esc(item.key)}" ${settingBool(item.key) ? 'checked' : ''}><span></span></label>`
            : `<select class="vd-setting-select" data-setting-key="${esc(item.key)}">${item.options.map(option => `<option value="${esc(option[0])}" ${settingValue(item.key) === option[0] ? 'selected' : ''}>${esc(t(option[1]))}</option>`).join('')}</select>`;
        return `<article class="vd-setting-row">
            <div>
                <div class="vd-setting-label">${esc(t(item.label))}</div>
                <div class="vd-setting-help">${esc(t(item.desc))}</div>
            </div>
            ${control}
        </article>`;
    }

    async function saveDesktopSetting(key, value, host) {
        try {
            const body = await api('/api/desktop/settings', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ key, value })
            });
            if (!state.bootstrap) state.bootstrap = {};
            state.bootstrap.settings = body.settings || Object.assign(desktopSettings(), { [key]: value });
            applyDesktopSettings();
            renderStartButtonIcon();
            renderIcons();
            renderWidgets();
            renderStartApps();
            if (host && host.isConnected) renderSettingsShell(host);
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err.message });
            if (host && host.isConnected) renderSettingsShell(host);
        }
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
        const path = 'Apps/' + app.id + '/' + app.entry;
        host.innerHTML = `<div class="vd-empty">${esc(t('desktop.loading'))}</div>`;
        desktopEmbedURL(path)
            .then(async src => {
                await ensureDesktopEmbedHasContent(src);
                if (!contentEl(id)) return;
                host.replaceChildren(makeSandboxedFrame(src, app.id, '', id, 'vd-generated-frame', appName(app)));
            })
            .catch(err => {
                if (!contentEl(id)) return;
                host.innerHTML = `<div class="vd-empty">${esc(err.message)}</div>`;
            });
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

    async function desktopEmbedURL(path, params) {
        const body = await api('/api/desktop/embed-token?path=' + encodeURIComponent(path));
        const query = new URLSearchParams(params || {});
        if (body.token) query.set('desktop_token', body.token);
        const suffix = query.toString();
        return desktopFileURL(path) + (suffix ? '?' + suffix : '');
    }

    async function ensureDesktopEmbedHasContent(src) {
        const response = await fetch(src, { credentials: 'same-origin', cache: 'no-store' });
        if (!response.ok) throw new Error(response.statusText || ('HTTP ' + response.status));
        const html = await response.text();
        if (!html.trim()) {
            throw new Error(t('desktop.embed_empty'));
        }
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
                    icon_manifest: state.iconManifest,
                    papirus_icon_manifest: state.papirusIconManifest
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
        renderStartButtonIcon();
        document.addEventListener('click', (event) => {
            if (!event.target.closest('.vd-context-menu')) closeContextMenu();
            const menu = $('vd-start-menu');
            if (!menu.hidden && !menu.contains(event.target) && !event.target.closest('#vd-start-button')) {
                menu.hidden = true;
            }
        });
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
        if (event.target && ['INPUT', 'TEXTAREA', 'SELECT'].includes(event.target.tagName)) return;
        if (event.key === 'Escape') {
            closeContextMenu();
            $('vd-start-menu').hidden = true;
            return;
        }
        if (event.ctrlKey && event.code === 'Space') {
            event.preventDefault();
            $('vd-start-button').click();
            return;
        }
        if (event.key === 'Enter' && state.selectedIconId) {
            const icon = document.querySelector(`.vd-icon[data-id="${cssSel(state.selectedIconId)}"]`);
            if (icon) activateDesktopItem(icon);
            return;
        }
        if (event.key === 'Delete' && state.selectedIconId) {
            const icon = document.querySelector(`.vd-icon[data-id="${cssSel(state.selectedIconId)}"]`);
            if (icon && icon.dataset.kind === 'directory') deletePath(icon.dataset.path);
            return;
        }
        if (event.key === 'F2' && state.selectedIconId) {
            const icon = document.querySelector(`.vd-icon[data-id="${cssSel(state.selectedIconId)}"]`);
            if (icon && icon.dataset.kind === 'directory') renamePath(icon.dataset.path);
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
