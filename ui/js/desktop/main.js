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
        desktopFiles: [],
        iconManifest: null,
        papirusIconManifest: null,
        iconMap: new Map(),
        selectedIconId: '',
        contextMenu: null,
        webampMusic: null
    };

    const SDK_REQUEST_TYPE = 'aurago.desktop.request';
    const SDK_RESPONSE_TYPE = 'aurago.desktop.response';
    const SDK_RUNTIME = 'aura-desktop-sdk@1';
    const WEBAMP_MODULE_PATH = '/js/vendor/webamp/webamp.bundle.min.mjs';
    const WEBAMP_AUDIO_PATTERN = /\.(mp3|wav|flac|ogg|m4a|opus)$/i;
    const WEBAMP_TRACK_SCAN_LIMIT = 1000;
    const GALLERY_PAGE_SIZE = 80;
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
        Music: 'audio',
        Photos: 'image',
        Videos: 'video',
        'AuraGo Documents': 'documents',
        Trash: 'trash',
        Shared: 'share'
    };
    const appIconKeys = {
        files: 'folder',
        editor: 'edit',
        settings: 'settings',
        calendar: 'calendar',
        calculator: 'calculator',
        gallery: 'image',
        'music-player': 'audio',
        todo: 'notes',
        'agent-chat': 'agent_chat',
        terminal: 'terminal',
        browser: 'browser',
        launchpad: 'apps'
    };
    appIconKeys['code-studio'] = 'code';
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
        ogg: 'audio',
        m4a: 'audio',
        opus: 'audio',
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
            calculator: 'Ca',
            'music-player': 'MP',
            todo: 'Td',
            'agent-chat': 'A',
            gallery: 'G',
            'quick-connect': 'QC',
            'code-studio': 'CS',
            launchpad: 'LP'
        };
        return map[id] || ((app && app.name && app.name[0]) || 'D').toUpperCase();
    }

    async function loadIconManifest() {
        const [spriteManifest, papirusIconManifest] = await Promise.all([
            api('/img/desktop-icons-sprite.json').catch(() => null),
            api('/img/papirus/manifest.json?v=2').catch(() => null)
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
        if (file.type === 'directory') return iconForDirectory(file.name);
        if (file.media_kind === 'image') return 'image';
        if (file.media_kind === 'audio') return 'audio';
        if (file.media_kind === 'video') return 'video';
        if (file.media_kind === 'document') return 'documents';
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

    function appById(appId) {
        return allApps().find(app => app.id === appId);
    }

    function isBuiltinApp(appId) {
        return ((state.bootstrap && state.bootstrap.builtin_apps) || []).some(app => app.id === appId);
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

    function removeIconPosition(id) {
        const positions = iconPositions();
        delete positions[id];
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
        state.desktopFiles = await loadDesktopFiles();
        renderDesktop();
    }

    async function loadDesktopFiles() {
        if (!state.bootstrap || !state.bootstrap.enabled) return [];
        try {
            const body = await api('/api/desktop/files?path=Desktop');
            return (body.files || []).filter(file => file && file.path);
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err.message });
            return [];
        }
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
        const items = desktopShortcutItems();
        const positions = iconPositions();
        icons.innerHTML = items.map(item => {
            const iconKey = item.type === 'file' ? iconForFile(item.file) : item.type === 'directory' ? iconForDirectory(item.name) : iconForApp(item.app);
            const fallback = item.type === 'app' ? iconGlyph(item.app) : item.name;
            const pos = positions[item.id] || defaultIconPosition(items.indexOf(item));
            return `<button class="vd-icon ${item.id === state.selectedIconId ? 'selected' : ''}" type="button" data-kind="${esc(item.type)}" data-id="${esc(item.id)}" data-app-id="${esc(item.app ? item.app.id : '')}" data-path="${esc(item.path || '')}" data-web-path="${esc(item.file ? item.file.web_path || '' : '')}" data-media-kind="${esc(item.file ? item.file.media_kind || '' : '')}" data-mime-type="${esc(item.file ? item.file.mime_type || '' : '')}" data-desktop-entry="${item.desktopEntry ? 'true' : 'false'}" style="left:${Number(pos.x) || 18}px;top:${Number(pos.y) || 18}px">
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

    function desktopShortcutItems() {
        const shortcuts = (state.bootstrap && state.bootstrap.shortcuts) || [];
        const shortcutItems = shortcuts.map(shortcut => {
            if (shortcut.target_type === 'app') {
                const app = appById(shortcut.target_id);
                if (!app) return null;
                return {
                    id: shortcut.id,
                    name: shortcut.name || appName(app),
                    type: 'app',
                    app,
                    path: shortcut.path || '',
                    shortcut
                };
            }
            if (shortcut.target_type === 'directory') {
                return {
                    id: shortcut.id,
                    name: shortcut.name || shortcut.path,
                    type: 'directory',
                    path: shortcut.path || shortcut.target_id || '',
                    shortcut
                };
            }
            return null;
        }).filter(Boolean);
        const desktopEntries = (state.desktopFiles || []).map(file => ({
            id: 'desktop-entry-' + file.path,
            name: file.name || file.path,
            type: file.type === 'directory' ? 'directory' : 'file',
            path: file.path,
            file,
            desktopEntry: true
        }));
        return [...shortcutItems, ...desktopEntries];
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
        if (btn.dataset.kind === 'file') {
            openDesktopFileEntry(btn);
            return;
        }
        if (btn.dataset.kind === 'directory') {
            openApp('files', { path: btn.dataset.path || '' });
            return;
        }
        const appId = btn.dataset.appId || btn.dataset.id;
        if (appId === 'files') {
            openApp('files', { path: btn.dataset.path || '' });
            return;
        }
        openApp(appId);
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
            btn.addEventListener('contextmenu', event => showStartAppContextMenu(event, btn.dataset.appId));
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

    function appWindowSize(appId) {
        const presets = {
            files: { width: 920, height: 600 },
            calculator: { width: 380, height: 520 },
            todo: { width: 900, height: 600 },
            'music-player': { width: 430, height: 260 },
            calendar: { width: 950, height: 650 },
            'quick-connect': { width: 920, height: 640 },
            'code-studio': { width: 1280, height: 850 },
            launchpad: { width: 1100, height: 700 }
        };
        return presets[appId] || defaultWindowSize();
    }

    function openApp(appId, context) {
        const existing = [...state.windows.values()].find(win => win.appId === appId && appId !== 'editor');
        if (existing) {
            focusWindow(existing.id);
            if (appId === 'files' && context && context.path != null) {
                if (window.FileManager && typeof window.FileManager.navigateTo === 'function') {
                    window.FileManager.navigateTo(existing.id, context.path);
                } else {
                    renderFiles(existing.id, context.path);
                }
            }
            return;
        }
        const title = windowTitle(appId);
        const id = 'w-' + appId + '-' + Date.now();
        const win = document.createElement('section');
        win.className = 'vd-window';
        win.dataset.windowId = id;
        win.style.left = Math.max(16, 170 + state.windows.size * 28) + 'px';
        win.style.top = Math.max(12, 72 + state.windows.size * 24) + 'px';
        const size = appWindowSize(appId);
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
            win.style.left = '0';
            win.style.top = '0';
            win.style.width = Math.max(WINDOW_MIN_W, bounds.width) + 'px';
            win.style.height = Math.max(WINDOW_MIN_H, bounds.height) + 'px';
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
        if (win.appId === 'music-player') disposeWebampMusic(id);
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
                <span class="vd-context-icon">${iconMarkup(item.icon || 'tools', item.fallback || item.icon || '', 'vd-context-papirus-icon', 16)}</span>
                <span>${esc(item.label)}</span>
            </button>`).join('');
        document.body.appendChild(menu);
        const rect = menu.getBoundingClientRect();
        menu.style.left = Math.max(8, Math.min(x, window.innerWidth - rect.width - 8)) + 'px';
        menu.style.top = Math.max(8, Math.min(y, window.innerHeight - rect.height - 8)) + 'px';
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
            { label: t('desktop.context_new_file'), icon: 'file-plus', fallback: '+', action: () => createFileInPath('Desktop') },
            { label: t('desktop.context_new_folder'), icon: 'folder-plus', fallback: '+', action: () => createFolderInPath('Desktop') },
            { separator: true },
            { label: t('desktop.context_refresh'), icon: 'refresh', fallback: 'R', action: () => loadBootstrap() },
            { label: t('desktop.context_sort_icons'), icon: 'sort', fallback: 'S', action: autoArrangeIcons }
        ]);
    }

    function showIconContextMenu(event, btn) {
        event.preventDefault();
        selectDesktopIcon(btn);
        const path = btn.dataset.path || '';
        const appId = btn.dataset.appId || '';
        const kind = btn.dataset.kind || '';
        const isDesktopEntry = btn.dataset.desktopEntry === 'true';
        const items = [
            { label: t('desktop.context_open'), icon: 'folder-open', fallback: 'O', action: () => activateDesktopItem(btn) }
        ];
        if (isDesktopEntry || kind === 'file') {
            items.push(
                { label: t('desktop.context_rename'), icon: 'edit', fallback: 'E', action: () => renamePath(path) },
                { label: t('desktop.context_delete'), icon: 'trash', fallback: 'X', action: () => deletePath(path) }
            );
            if (btn.dataset.webPath) {
                items.push({ label: t('desktop.media_download'), icon: 'download', fallback: 'D', action: () => downloadMediaPath(btn.dataset.webPath, btn.querySelector('.vd-icon-label').textContent) });
            }
        } else {
            items.push({ label: t('desktop.context_remove_from_desktop'), icon: 'x', fallback: 'X', action: () => removeDesktopShortcut(btn.dataset.id) });
        }
        if (appId) {
            const appIsBuiltin = isBuiltinApp(appId);
            items.push({ label: t('desktop.context_delete_app'), icon: 'trash', fallback: 'X', disabled: appIsBuiltin, action: () => deleteDesktopApp(appId) });
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
            { label: t('desktop.context_remove_widget'), icon: 'x', fallback: 'X', action: () => deleteWidget(widget.id) }
        ]);
    }

    function showWindowContextMenu(event, id) {
        event.preventDefault();
        const item = state.windows.get(id);
        if (!item) return;
        showContextMenu(event.clientX, event.clientY, [
            { label: t('desktop.context_restore'), icon: 'monitor', fallback: 'W', action: () => focusWindow(id) },
            { label: t('desktop.context_minimize'), icon: 'chevron-down', fallback: '_', action: () => { item.element.style.display = 'none'; renderTaskbar(); } },
            { label: item.maximized ? t('desktop.restore') : t('desktop.context_maximize'), icon: 'grid', fallback: 'M', action: () => toggleMaximizeWindow(id) },
            { separator: true },
            { label: t('desktop.context_close'), icon: 'x', fallback: 'X', action: () => closeWindow(id) }
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
        const path = joinPath(basePath, name);
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

    function showProperties(title, body) {
        showDesktopNotification({ title: title || t('desktop.context_properties'), message: body || '' });
    }

    function contentEl(id) {
        const win = state.windows.get(id);
        return win && win.element.querySelector('[data-window-content]');
    }

    function renderAppContent(id, appId, context) {
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
        if (appId === 'settings') return renderSettings(id);
        if (appId === 'calendar') return renderCalendar(id);
        if (appId === 'calculator') return renderCalculator(id);
        if (appId === 'todo') return renderTodo(id);
        if (appId === 'gallery') return renderGallery(id);
        if (appId === 'music-player') return renderMusicPlayer(id);
        if (appId === 'agent-chat') return renderChat(id);
        if (appId === 'quick-connect') return renderQuickConnect(id);
        if (appId === 'code-studio' && window.CodeStudio && typeof window.CodeStudio.render === 'function') {
            return window.CodeStudio.render(contentEl(id), id, Object.assign({}, context || {}, { iconMarkup }));
        }
        if (appId === 'launchpad') return renderLaunchpad(id);
        return renderGeneratedApp(id, appId);
    }

    async function renderFiles(id, path) {
        const host = contentEl(id);
        if (!host) return;
        state.filesPath = path || '';
        if (window.FileManager && typeof window.FileManager.render === 'function') {
            window.FileManager.render(host, id, state.filesPath, {
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
                openFile: (entry) => {
                    if (entry.web_path || entry.media_kind) return openMediaPreview(entry);
                    openEditorFile(entry.path);
                },
                openMedia: (entry) => openMediaPreview(entry),
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
            });
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
        host.querySelector('[data-action="new-file"]').addEventListener('click', () => openApp('editor', { path: joinPath(state.filesPath, 'untitled.txt'), content: '' }));
        host.querySelector('[data-action="new-folder"]').addEventListener('click', () => createFolderInPath(state.filesPath));
        try {
            const body = await api('/api/desktop/files?path=' + encodeURIComponent(state.filesPath));
            const files = body.files || [];
            host.querySelector('.vd-file-list').innerHTML = files.length ? files.map(file => `<div class="vd-file-row" data-type="${esc(file.type)}" data-path="${esc(file.path)}" data-web-path="${esc(file.web_path || '')}" data-media-kind="${esc(file.media_kind || '')}" data-mime-type="${esc(file.mime_type || '')}">
                ${iconMarkup(iconForFile(file), file.type === 'directory' ? 'D' : file.name, 'vd-sprite-file', 26)}
                <span class="vd-file-name">${esc(file.name)}</span>
                <span class="vd-file-meta">${esc(file.type === 'directory' ? t('desktop.folder') : fmtBytes(file.size))}</span>
            </div>`).join('') : `<div class="vd-empty">${esc(t('desktop.empty_folder'))}</div>`;
            host.querySelectorAll('.vd-file-row').forEach(row => {
                row.addEventListener('dblclick', () => {
                    if (row.dataset.type === 'directory') renderFiles(id, row.dataset.path);
                    else openDesktopFileEntry(row);
                });
                row.addEventListener('click', () => {
                    host.querySelectorAll('.vd-file-row').forEach(item => item.classList.toggle('selected', item === row));
                });
                row.addEventListener('contextmenu', event => {
                    event.preventDefault();
                    const actions = [
                        { label: t('desktop.context_open'), icon: 'folder-open', fallback: 'O', action: () => row.dataset.type === 'directory' ? renderFiles(id, row.dataset.path) : openDesktopFileEntry(row) },
                        { label: t('desktop.context_rename'), icon: 'edit', fallback: 'E', action: () => renamePath(row.dataset.path) },
                        { label: t('desktop.context_delete'), icon: 'trash', fallback: 'X', action: () => deletePath(row.dataset.path) },
                    ];
                    if (row.dataset.webPath) {
                        actions.push({ label: t('desktop.media_download'), icon: 'download', fallback: 'D', action: () => downloadMediaPath(row.dataset.webPath, row.querySelector('.vd-file-name').textContent) });
                    }
                    actions.push(
                        { separator: true },
                        { label: t('desktop.context_properties'), icon: 'info', fallback: 'i', action: () => showProperties(row.querySelector('.vd-file-name').textContent, row.dataset.path) }
                    );
                    showContextMenu(event.clientX, event.clientY, actions);
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

    function openDesktopFileEntry(row) {
        const entry = {
            name: row.querySelector('.vd-file-name, .vd-icon-label') ? row.querySelector('.vd-file-name, .vd-icon-label').textContent : row.dataset.path,
            path: row.dataset.path,
            web_path: row.dataset.webPath,
            media_kind: row.dataset.mediaKind,
            mime_type: row.dataset.mimeType
        };
        if (entry.web_path || entry.media_kind) return openMediaPreview(entry);
        openEditorFile(entry.path);
    }

    function downloadMediaPath(webPath, filename) {
        if (!webPath) return;
        const link = document.createElement('a');
        link.href = webPath;
        link.download = filename || '';
        document.body.appendChild(link);
        link.click();
        link.remove();
    }

    function openMediaPreview(file) {
        if (!file || !file.web_path) {
            if (file && file.path) openEditorFile(file.path);
            return;
        }
        const kind = file.media_kind || '';
        if (kind === 'document' && !String(file.mime_type || '').startsWith('text/')) {
            window.open(file.web_path, '_blank', 'noopener');
            return;
        }
        const overlay = document.createElement('div');
        overlay.className = 'vd-modal-backdrop vd-media-preview-backdrop';
        const body = kind === 'video'
            ? `<video controls autoplay src="${esc(file.web_path)}"></video>`
            : kind === 'audio'
                ? `<audio controls autoplay src="${esc(file.web_path)}"></audio>`
                : kind === 'image'
                    ? `<img src="${esc(file.web_path)}" alt="${esc(file.name || '')}">`
                    : `<iframe src="${esc(file.web_path)}" title="${esc(file.name || '')}"></iframe>`;
        overlay.innerHTML = `<div class="vd-media-preview" role="dialog" aria-modal="true">
            <div class="vd-media-preview-bar">
                <strong>${esc(file.name || file.path || t('desktop.media_open'))}</strong>
                <div>
                    <a class="vd-button" href="${esc(file.web_path)}" download="${esc(file.name || '')}">${esc(t('desktop.media_download'))}</a>
                    <button class="vd-button vd-button-primary" type="button" data-close>${esc(t('desktop.close'))}</button>
                </div>
            </div>
            <div class="vd-media-preview-body">${body}</div>
        </div>`;
        document.body.appendChild(overlay);
        const close = () => overlay.remove();
        overlay.querySelector('[data-close]').addEventListener('click', close);
        overlay.addEventListener('click', event => { if (event.target === overlay) close(); });
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
                id: 'appearance', icon: 'settings', fallback: 'A', title: 'desktop.settings_category_appearance', desc: 'desktop.settings_category_appearance_desc', items: [
                    settingSelect('appearance.wallpaper', 'desktop.settings_wallpaper', 'desktop.settings_wallpaper_desc', [
                        ['aurora', 'desktop.settings_wallpaper_aurora'], ['midnight', 'desktop.settings_wallpaper_midnight'], ['slate', 'desktop.settings_wallpaper_slate'], ['ember', 'desktop.settings_wallpaper_ember'], ['forest', 'desktop.settings_wallpaper_forest'],
                        ['alpine_dawn', 'desktop.settings_wallpaper_alpine_dawn'], ['city_rain', 'desktop.settings_wallpaper_city_rain'], ['ocean_cliff', 'desktop.settings_wallpaper_ocean_cliff'],
                        ['aurora_glass', 'desktop.settings_wallpaper_aurora_glass'], ['nebula_flow', 'desktop.settings_wallpaper_nebula_flow'], ['paper_waves', 'desktop.settings_wallpaper_paper_waves']
                    ]),
                    settingSelect('appearance.accent', 'desktop.settings_accent', 'desktop.settings_accent_desc', [
                        ['teal', 'desktop.settings_accent_teal'], ['orange', 'desktop.settings_accent_orange'], ['blue', 'desktop.settings_accent_blue'], ['violet', 'desktop.settings_accent_violet'], ['green', 'desktop.settings_accent_green']
                    ]),
                    settingSelect('appearance.density', 'desktop.settings_density', 'desktop.settings_density_desc', [
                        ['comfortable', 'desktop.settings_density_comfortable'], ['compact', 'desktop.settings_density_compact']
                    ]),
                    settingSelect('appearance.icon_theme', 'desktop.settings_icon_theme', 'desktop.settings_icon_theme_desc', [
                        ['papirus', 'desktop.settings_icon_theme_papirus'], ['aurago', 'desktop.settings_icon_theme_aurago']
                    ]),
                    settingIconCatalog('desktop.settings_icon_catalog', 'desktop.settings_icon_catalog_desc')
                ]
            },
            {
                id: 'desktop', icon: 'desktop', fallback: 'D', title: 'desktop.settings_category_desktop', desc: 'desktop.settings_category_desktop_desc', items: [
                    settingSelect('desktop.icon_size', 'desktop.settings_icon_size', 'desktop.settings_icon_size_desc', [
                        ['small', 'desktop.settings_icon_size_small'], ['medium', 'desktop.settings_icon_size_medium'], ['large', 'desktop.settings_icon_size_large']
                    ]),
                    settingToggle('desktop.show_widgets', 'desktop.settings_show_widgets', 'desktop.settings_show_widgets_desc')
                ]
            },
            {
                id: 'windows', icon: 'monitor', fallback: 'W', title: 'desktop.settings_category_windows', desc: 'desktop.settings_category_windows_desc', items: [
                    settingToggle('windows.animations', 'desktop.settings_window_animations', 'desktop.settings_window_animations_desc'),
                    settingSelect('windows.default_size', 'desktop.settings_default_window_size', 'desktop.settings_default_window_size_desc', [
                        ['compact', 'desktop.settings_window_size_compact'], ['balanced', 'desktop.settings_window_size_balanced'], ['large', 'desktop.settings_window_size_large']
                    ])
                ]
            },
            {
                id: 'files', icon: 'folder', fallback: 'F', title: 'desktop.settings_category_files', desc: 'desktop.settings_category_files_desc', items: [
                    settingToggle('files.confirm_delete', 'desktop.settings_confirm_delete', 'desktop.settings_confirm_delete_desc'),
                    settingSelect('files.default_folder', 'desktop.settings_default_folder', 'desktop.settings_default_folder_desc', [
                        ['Desktop', 'desktop.settings_folder_desktop'], ['Documents', 'desktop.settings_folder_documents'], ['Downloads', 'desktop.settings_folder_downloads'], ['Pictures', 'desktop.settings_folder_pictures'], ['Shared', 'desktop.settings_folder_shared']
                    ])
                ]
            },
            {
                id: 'agent', icon: 'apps', fallback: 'A', title: 'desktop.settings_category_agent', desc: 'desktop.settings_category_agent_desc', items: [
                    settingToggle('agent.show_chat_button', 'desktop.settings_show_agent_button', 'desktop.settings_show_agent_button_desc'),
                    settingInfo('desktop.setting_agent_control', boot.allow_agent_control ? t('desktop.on') : t('desktop.off'))
                ]
            },
            {
                id: 'system', icon: 'info', fallback: 'i', title: 'desktop.settings_category_system', desc: 'desktop.settings_category_system_desc', items: [
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

    function settingIconCatalog(label, desc) {
        const catalog = (state.bootstrap && state.bootstrap.icon_catalog) || {};
        const preferred = Array.isArray(catalog.preferred) ? catalog.preferred.slice() : [];
        const aliases = catalog.aliases && typeof catalog.aliases === 'object'
            ? Object.keys(catalog.aliases).sort().map(alias => [alias, catalog.aliases[alias]])
            : [];
        return { type: 'icon_catalog', label, desc, preferred, aliases };
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
                    ${iconMarkup(section.icon, section.fallback || section.icon, 'vd-settings-nav-icon', 18)}<span>${esc(t(section.title))}</span>
                </button>`).join('')}
            </aside>
            <section class="vd-settings-pane">
                <div class="vd-settings-pane-head">
                    <div class="vd-settings-pane-icon">${iconMarkup(active.icon, active.fallback || active.icon, 'vd-settings-pane-papirus-icon', 28)}</div>
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
        if (item.type === 'icon_catalog') return renderIconCatalogSetting(item);
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

    function renderIconCatalogSetting(item) {
        const preferred = item.preferred.length
            ? item.preferred.map(name => `<span class="vd-icon-catalog-tag">${esc(name)}</span>`).join('')
            : `<span class="vd-icon-catalog-empty">${esc(t('desktop.settings_icon_catalog_empty'))}</span>`;
        const aliases = item.aliases.length
            ? `<div class="vd-icon-catalog-aliases">${item.aliases.map(pair => `<span><b>${esc(pair[0])}</b> -&gt; ${esc(pair[1])}</span>`).join('')}</div>`
            : '';
        return `<article class="vd-setting-row vd-icon-catalog-row">
            <div>
                <div class="vd-setting-label">${esc(t(item.label))}</div>
                <div class="vd-setting-help">${esc(t(item.desc))}</div>
            </div>
            <div class="vd-icon-catalog" aria-label="${esc(t(item.label))}">
                <div class="vd-icon-catalog-tags">${preferred}</div>
                ${aliases ? `<div class="vd-icon-catalog-alias-label">${esc(t('desktop.settings_icon_catalog_aliases'))}</div>${aliases}` : ''}
            </div>
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

    function plannerJSON(url, method, body) {
        return api(url, {
            method,
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body || {})
        });
    }

    function isoDate(date) {
        const d = new Date(date);
        const month = String(d.getMonth() + 1).padStart(2, '0');
        const day = String(d.getDate()).padStart(2, '0');
        return `${d.getFullYear()}-${month}-${day}`;
    }

    function dateTimeLocalValue(value) {
        const d = value ? new Date(value) : new Date();
        const month = String(d.getMonth() + 1).padStart(2, '0');
        const day = String(d.getDate()).padStart(2, '0');
        const hour = String(d.getHours()).padStart(2, '0');
        const minute = String(d.getMinutes()).padStart(2, '0');
        return `${d.getFullYear()}-${month}-${day}T${hour}:${minute}`;
    }

    function fromLocalDateTime(value) {
        return value ? new Date(value).toISOString() : new Date().toISOString();
    }

    function renderCalculator(id) {
        const host = contentEl(id);
        if (!host) return;
        host.innerHTML = `<div class="vd-calc" tabindex="0">
            <div class="vd-calc-tabs">
                <button type="button" class="active" data-mode="standard">${esc(t('desktop.calc_standard'))}</button>
                <button type="button" data-mode="scientific">${esc(t('desktop.calc_scientific'))}</button>
            </div>
            <div class="vd-calc-display"><div data-expression>0</div><strong data-result>0</strong></div>
            <div class="vd-calc-keys">
                ${['C','CE','⌫','%','sin','cos','tan','√','7','8','9','÷','log','ln','π','x²','4','5','6','×','(',')','e','xʸ','1','2','3','-','n!','±','.','+','0','00','='].map(key => `<button type="button" class="${/[+\-×÷=%]|xʸ/.test(key) ? 'op' : /sin|cos|tan|log|ln|π|e|√|n!|x²|[()]/.test(key) ? 'fn scientific' : ''}" data-key="${esc(key)}">${esc(key)}</button>`).join('')}
            </div>
            <aside class="vd-calc-history"><div>${esc(t('desktop.calc_history'))}</div><ol></ol></aside>
        </div>`;
        const root = host.querySelector('.vd-calc');
        const expressionEl = host.querySelector('[data-expression]');
        const resultEl = host.querySelector('[data-result]');
        const historyEl = host.querySelector('.vd-calc-history ol');
        let expression = '';
        const history = [];
        const update = (result) => {
            expressionEl.textContent = expression || '0';
            resultEl.textContent = result == null ? '0' : String(result);
        };
        const factorial = n => n < 0 || !Number.isInteger(n) ? NaN : Array.from({ length: n }, (_, i) => i + 1).reduce((a, b) => a * b, 1);
        const evaluate = () => {
            if (!expression) return;
            let js = expression.replaceAll('×', '*').replaceAll('÷', '/').replaceAll('π', 'Math.PI').replaceAll('√', 'Math.sqrt').replaceAll('ln', 'Math.log').replaceAll('log', 'Math.log10').replaceAll('sin', 'Math.sin').replaceAll('cos', 'Math.cos').replaceAll('tan', 'Math.tan').replaceAll('e', 'Math.E').replaceAll('^', '**');
            js = js.replace(/(\d+(?:\.\d+)?)!/g, 'factorial($1)');
            js = js.replace(/(\d+(?:\.\d+)?)²/g, '($1**2)');
            if (!/^[0-9+\-*/().,% MathPIEsincotaglrfqu!_]*$/.test(js)) throw new Error('Invalid expression');
            const value = Function('factorial', `return (${js})`)(factorial);
            const result = Number.isFinite(value) ? Number(value.toFixed(10)) : value;
            history.unshift(`${expression} = ${result}`);
            history.splice(8);
            historyEl.innerHTML = history.map(item => `<li>${esc(item)}</li>`).join('');
            expression = String(result);
            update(result);
        };
        const press = key => {
            try {
                if (key === 'C') expression = '';
                else if (key === 'CE') expression = '';
                else if (key === '⌫') expression = expression.slice(0, -1);
                else if (key === '=') return evaluate();
                else if (key === '±') expression = expression ? `(-1*(${expression}))` : '-';
                else if (key === 'x²') expression += '²';
                else if (key === 'xʸ') expression += '^';
                else if (key === 'n!') expression += '!';
                else if (['sin', 'cos', 'tan', 'log', 'ln', '√'].includes(key)) expression += `${key}(`;
                else expression += key;
                update();
            } catch (err) {
                resultEl.textContent = err.message;
            }
        };
        host.querySelectorAll('[data-key]').forEach(btn => btn.addEventListener('click', () => press(btn.dataset.key)));
        host.querySelectorAll('[data-mode]').forEach(btn => btn.addEventListener('click', () => {
            host.querySelectorAll('[data-mode]').forEach(item => item.classList.toggle('active', item === btn));
            root.classList.toggle('scientific-on', btn.dataset.mode === 'scientific');
        }));
        root.addEventListener('keydown', event => {
            const map = { Enter: '=', Backspace: '⌫', Escape: 'C', '*': '×', '/': '÷' };
            const key = map[event.key] || event.key;
            if (/^[0-9.+\-()%]$/.test(key) || ['=', '⌫', 'C', '×', '÷'].includes(key)) {
                event.preventDefault();
                press(key);
            }
        });
        root.focus();
    }

    async function renderTodo(id) {
        const host = contentEl(id);
        if (!host) return;
        host.dataset.todoFilter = host.dataset.todoFilter || 'all';
        host.innerHTML = `<div class="vd-todo"><aside class="vd-todo-sidebar">
            ${['all', 'open', 'in_progress', 'done'].map(status => `<button type="button" data-filter="${status}" class="${host.dataset.todoFilter === status ? 'active' : ''}">${esc(t('desktop.todo_' + status))}</button>`).join('')}
        </aside><main class="vd-todo-main"><form class="vd-todo-add"><input placeholder="${esc(t('desktop.todo_title_placeholder'))}"><select><option value="low">${esc(t('desktop.todo_priority_low'))}</option><option value="medium" selected>${esc(t('desktop.todo_priority_medium'))}</option><option value="high">${esc(t('desktop.todo_priority_high'))}</option></select><button class="vd-button vd-button-primary">${esc(t('desktop.todo_add'))}</button></form><div class="vd-todo-list">${esc(t('desktop.loading'))}</div></main><section class="vd-todo-detail"><div class="vd-empty">${esc(t('desktop.todo_select_task'))}</div></section></div>`;
        const load = async (selectedID) => {
            const todos = await api('/api/todos?status=all');
            const filtered = todos.filter(todo => host.dataset.todoFilter === 'all' || todo.status === host.dataset.todoFilter)
                .sort((a, b) => (({ high: 0, medium: 1, low: 2 }[a.priority] ?? 3) - (({ high: 0, medium: 1, low: 2 }[b.priority] ?? 3)) || String(a.due_date || '9999').localeCompare(String(b.due_date || '9999'))));
            const list = host.querySelector('.vd-todo-list');
            list.innerHTML = filtered.length ? filtered.map(todo => renderTodoCard(todo, selectedID)).join('') : `<div class="vd-empty">${esc(t('desktop.empty_folder'))}</div>`;
            list.querySelectorAll('[data-todo-id]').forEach(card => card.addEventListener('click', () => renderTodoDetail(host, todos.find(todo => todo.id === card.dataset.todoId), load)));
            const selected = todos.find(todo => todo.id === selectedID) || filtered[0];
            if (selected) renderTodoDetail(host, selected, load);
        };
        host.querySelectorAll('[data-filter]').forEach(btn => btn.addEventListener('click', () => {
            host.dataset.todoFilter = btn.dataset.filter;
            renderTodo(id);
        }));
        host.querySelector('.vd-todo-add').addEventListener('submit', async event => {
            event.preventDefault();
            const input = event.currentTarget.querySelector('input');
            const title = input.value.trim();
            if (!title) return;
            const result = await plannerJSON('/api/todos', 'POST', { title, priority: event.currentTarget.querySelector('select').value, status: 'open' });
            input.value = '';
            await load(result.id);
        });
        try { await load(); } catch (err) { host.querySelector('.vd-todo-list').innerHTML = `<div class="vd-empty">${esc(err.message)}</div>`; }
    }

    function renderTodoCard(todo, selectedID) {
        const due = todo.due_date ? new Date(todo.due_date) : null;
        const overdue = due && due < new Date() && todo.status !== 'done';
        return `<article class="vd-todo-card ${todo.id === selectedID ? 'active' : ''} ${overdue ? 'overdue' : ''}" data-todo-id="${esc(todo.id)}">
            <div><strong>${esc(todo.title)}</strong><span class="vd-todo-priority ${esc(todo.priority)}">${esc(t('desktop.todo_priority_' + todo.priority))}</span></div>
            <small>${esc(t('desktop.todo_' + todo.status))}${due ? ' · ' + esc(due.toLocaleDateString()) : ''}${overdue ? ' · ' + esc(t('desktop.todo_overdue')) : ''}</small>
            <div class="vd-todo-progress"><span style="width:${Number(todo.progress_percent) || 0}%"></span></div>
        </article>`;
    }

    function renderTodoDetail(host, todo, reload) {
        const pane = host.querySelector('.vd-todo-detail');
        const items = todo.items || [];
        pane.innerHTML = `<form class="vd-todo-form"><input name="title" value="${esc(todo.title)}"><textarea name="description" placeholder="${esc(t('desktop.todo_description'))}">${esc(todo.description || '')}</textarea><div class="vd-todo-form-row"><label>${esc(t('desktop.todo_priority'))}<select name="priority">${['low','medium','high'].map(p => `<option value="${p}" ${todo.priority === p ? 'selected' : ''}>${esc(t('desktop.todo_priority_' + p))}</option>`).join('')}</select></label><label>${esc(t('desktop.todo_due_date'))}<input type="date" name="due_date" value="${esc(todo.due_date || '')}"></label></div><label class="vd-check"><input type="checkbox" name="remind_daily" ${todo.remind_daily ? 'checked' : ''}>${esc(t('desktop.todo_remind_daily'))}</label><div class="vd-todo-actions"><button class="vd-button vd-button-primary" data-action="save">${esc(t('desktop.save'))}</button><button type="button" class="vd-button" data-action="complete">${esc(t('desktop.todo_complete'))}</button><button type="button" class="vd-button" data-action="delete">${esc(t('desktop.delete'))}</button></div></form><h3>${esc(t('desktop.todo_items'))}</h3><form class="vd-todo-item-add"><input placeholder="${esc(t('desktop.todo_add_item'))}"><button class="vd-button">${esc(t('desktop.todo_add_item'))}</button></form><div class="vd-todo-items">${items.map(item => `<label class="vd-todo-item"><input type="checkbox" data-item-toggle="${esc(item.id)}" ${item.is_done ? 'checked' : ''}><span>${esc(item.title)}</span><button type="button" class="vd-todo-item-delete" data-item-delete="${esc(item.id)}" title="${esc(t('desktop.delete'))}">${iconMarkup('x', 'X', 'vd-todo-action-icon', 13)}</button></label>`).join('')}</div>`;
        pane.querySelector('.vd-todo-form').addEventListener('submit', async event => {
            event.preventDefault();
            const form = event.currentTarget;
            await plannerJSON('/api/todos/' + encodeURIComponent(todo.id), 'PUT', { title: form.title.value.trim(), description: form.description.value, priority: form.priority.value, due_date: form.due_date.value, remind_daily: form.remind_daily.checked });
            await reload(todo.id);
        });
        pane.querySelector('[data-action="complete"]').addEventListener('click', async () => { await plannerJSON('/api/todos/' + encodeURIComponent(todo.id) + '/complete', 'POST', { complete_items_too: true }); await reload(todo.id); });
        pane.querySelector('[data-action="delete"]').addEventListener('click', async () => { if (await confirmDialog(t('desktop.todo_delete_confirm'), todo.title)) { await api('/api/todos/' + encodeURIComponent(todo.id), { method: 'DELETE' }); await reload(); } });
        pane.querySelector('.vd-todo-item-add').addEventListener('submit', async event => { event.preventDefault(); const input = event.currentTarget.querySelector('input'); if (!input.value.trim()) return; await plannerJSON('/api/todos/' + encodeURIComponent(todo.id) + '/items', 'POST', { title: input.value.trim() }); await reload(todo.id); });
        pane.querySelectorAll('[data-item-toggle]').forEach(input => input.addEventListener('change', async () => { await plannerJSON('/api/todos/' + encodeURIComponent(todo.id) + '/items/' + encodeURIComponent(input.dataset.itemToggle), 'PUT', { is_done: input.checked }); await reload(todo.id); }));
        pane.querySelectorAll('[data-item-delete]').forEach(btn => btn.addEventListener('click', async () => { await api('/api/todos/' + encodeURIComponent(todo.id) + '/items/' + encodeURIComponent(btn.dataset.itemDelete), { method: 'DELETE' }); await reload(todo.id); }));
    }

    async function renderCalendar(id) {
        const host = contentEl(id);
        if (!host) return;
        host.dataset.calView = host.dataset.calView || 'month';
        host.dataset.calDate = host.dataset.calDate || isoDate(new Date());
        const activeDate = new Date(host.dataset.calDate + 'T12:00:00');
        host.innerHTML = `<div class="vd-calendar"><div class="vd-calendar-toolbar"><button type="button" data-cal-nav="prev">‹</button><button type="button" data-cal-today>${esc(t('desktop.cal_today'))}</button><button type="button" data-cal-nav="next">›</button><strong>${esc(activeDate.toLocaleDateString(undefined, { month: 'long', year: 'numeric' }))}</strong><span></span>${['month','week','day'].map(view => `<button type="button" data-cal-view="${view}" class="${host.dataset.calView === view ? 'active' : ''}">${esc(t('desktop.cal_' + view))}</button>`).join('')}<button type="button" class="vd-button vd-button-primary" data-cal-new>${esc(t('desktop.cal_new_appointment'))}</button></div><div class="vd-calendar-body">${esc(t('desktop.loading'))}</div></div>`;
        const render = async () => {
            const appointments = await api('/api/appointments?status=all');
            const body = host.querySelector('.vd-calendar-body');
            body.innerHTML = host.dataset.calView === 'month' ? calendarMonthHTML(activeDate, appointments) : calendarAgendaHTML(activeDate, appointments, host.dataset.calView);
            body.querySelectorAll('[data-cal-date]').forEach(cell => cell.addEventListener('click', () => openAppointmentModal(host, null, cell.dataset.calDate, render)));
            body.querySelectorAll('[data-appt-id]').forEach(btn => btn.addEventListener('click', event => { event.stopPropagation(); openAppointmentModal(host, appointments.find(a => a.id === btn.dataset.apptId), '', render); }));
        };
        host.querySelectorAll('[data-cal-view]').forEach(btn => btn.addEventListener('click', () => { host.dataset.calView = btn.dataset.calView; renderCalendar(id); }));
        host.querySelector('[data-cal-today]').addEventListener('click', () => { host.dataset.calDate = isoDate(new Date()); renderCalendar(id); });
        host.querySelectorAll('[data-cal-nav]').forEach(btn => btn.addEventListener('click', () => {
            const delta = btn.dataset.calNav === 'next' ? 1 : -1;
            if (host.dataset.calView === 'month') activeDate.setMonth(activeDate.getMonth() + delta);
            else activeDate.setDate(activeDate.getDate() + delta * (host.dataset.calView === 'week' ? 7 : 1));
            host.dataset.calDate = isoDate(activeDate);
            renderCalendar(id);
        }));
        host.querySelector('[data-cal-new]').addEventListener('click', () => openAppointmentModal(host, null, isoDate(activeDate), render));
        try { await render(); } catch (err) { host.querySelector('.vd-calendar-body').innerHTML = `<div class="vd-empty">${esc(err.message)}</div>`; }
    }

    function calendarMonthHTML(activeDate, appointments) {
        const first = new Date(activeDate.getFullYear(), activeDate.getMonth(), 1);
        const start = new Date(first);
        start.setDate(first.getDate() - ((first.getDay() + 6) % 7));
        const today = isoDate(new Date());
        const cells = Array.from({ length: 42 }, (_, i) => { const d = new Date(start); d.setDate(start.getDate() + i); return d; });
        return `<div class="vd-calendar-month">${cells.map(d => { const key = isoDate(d); const dayItems = appointments.filter(a => String(a.date_time || '').startsWith(key)); return `<button type="button" class="vd-calendar-cell ${d.getMonth() !== activeDate.getMonth() ? 'muted' : ''} ${key === today ? 'today' : ''}" data-cal-date="${key}"><span>${d.getDate()}</span>${dayItems.slice(0, 3).map(a => `<i class="${esc(a.status || 'upcoming')}" data-appt-id="${esc(a.id)}">${esc(a.title)}</i>`).join('')}</button>`; }).join('')}</div><aside class="vd-calendar-upcoming"><h3>${esc(t('desktop.cal_new_appointment'))}</h3>${appointments.slice(0, 8).map(a => `<button type="button" data-appt-id="${esc(a.id)}">${esc(new Date(a.date_time).toLocaleString())} · ${esc(a.title)}</button>`).join('')}</aside>`;
    }

    function calendarAgendaHTML(activeDate, appointments, view) {
        const days = view === 'week' ? Array.from({ length: 7 }, (_, i) => { const d = new Date(activeDate); d.setDate(activeDate.getDate() - ((activeDate.getDay() + 6) % 7) + i); return d; }) : [activeDate];
        return `<div class="vd-calendar-agenda ${view}">${days.map(day => { const key = isoDate(day); const dayItems = appointments.filter(a => String(a.date_time || '').startsWith(key)); return `<section><h3>${esc(day.toLocaleDateString(undefined, { weekday: 'short', day: 'numeric' }))}</h3>${Array.from({ length: 24 }, (_, hour) => `<div class="vd-calendar-hour" data-cal-date="${key}T${String(hour).padStart(2, '0')}:00"><span>${String(hour).padStart(2, '0')}:00</span>${dayItems.filter(a => new Date(a.date_time).getHours() === hour).map(a => `<button type="button" class="${esc(a.status || 'upcoming')}" data-appt-id="${esc(a.id)}">${esc(a.title)}</button>`).join('')}</div>`).join('')}</section>`; }).join('')}</div>`;
    }

    function openAppointmentModal(host, appointment, dateHint, reload) {
        const overlay = document.createElement('div');
        overlay.className = 'vd-modal-backdrop';
        const initial = appointment || { title: '', description: '', status: 'upcoming', date_time: dateHint ? fromLocalDateTime(dateHint.includes('T') ? dateHint : dateHint + 'T09:00') : new Date().toISOString(), wake_agent: false };
        overlay.innerHTML = `<form class="vd-modal vd-calendar-modal"><div class="vd-modal-title">${esc(t(appointment ? 'desktop.cal_edit_appointment' : 'desktop.cal_new_appointment'))}</div><input name="title" class="vd-modal-input" placeholder="${esc(t('desktop.cal_title'))}" value="${esc(initial.title)}"><input name="date_time" class="vd-modal-input" type="datetime-local" value="${esc(dateTimeLocalValue(initial.date_time))}"><textarea name="description" class="vd-modal-input" placeholder="${esc(t('desktop.cal_description'))}">${esc(initial.description || '')}</textarea><select name="status" class="vd-modal-input">${['upcoming','overdue','completed','cancelled'].map(status => `<option value="${status}" ${initial.status === status ? 'selected' : ''}>${esc(t('desktop.cal_status_' + status))}</option>`).join('')}</select><label class="vd-check"><input name="wake_agent" type="checkbox" ${initial.wake_agent ? 'checked' : ''}>${esc(t('desktop.cal_notification'))}</label><div class="vd-modal-actions">${appointment ? `<button type="button" class="vd-button" data-delete>${esc(t('desktop.delete'))}</button>` : ''}<button type="button" class="vd-button" data-cancel>${esc(t('desktop.cancel'))}</button><button class="vd-button vd-button-primary">${esc(t('desktop.save'))}</button></div></form>`;
        document.body.appendChild(overlay);
        const close = () => overlay.remove();
        overlay.querySelector('[data-cancel]').addEventListener('click', close);
        overlay.addEventListener('click', event => { if (event.target === overlay) close(); });
        const del = overlay.querySelector('[data-delete]');
        if (del) del.addEventListener('click', async () => { if (await confirmDialog(t('desktop.cal_delete_confirm'), appointment.title)) { await api('/api/appointments/' + encodeURIComponent(appointment.id), { method: 'DELETE' }); close(); await reload(); } });
        overlay.querySelector('form').addEventListener('submit', async event => {
            event.preventDefault();
            const form = event.currentTarget;
            const payload = { title: form.title.value.trim(), date_time: fromLocalDateTime(form.date_time.value), description: form.description.value, status: form.status.value, wake_agent: form.wake_agent.checked };
            if (!payload.title) return;
            if (appointment) await plannerJSON('/api/appointments/' + encodeURIComponent(appointment.id), 'PUT', payload);
            else await plannerJSON('/api/appointments', 'POST', payload);
            close();
            await reload();
        });
    }

    async function renderGallery(id) {
        const host = contentEl(id);
        if (!host) return;
        host.dataset.galleryTab = host.dataset.galleryTab || 'Photos';
        host.dataset.galleryOffset = '0';
        host.innerHTML = `<div class="vd-gallery">
            <div class="vd-toolbar vd-gallery-toolbar">
                <div class="vd-segmented">
                    <button class="vd-tool-button" type="button" data-gallery-tab="Photos">${iconMarkup('image', 'P', 'vd-tool-icon', 15)}<span>${esc(t('desktop.gallery_photos'))}</span></button>
                    <button class="vd-tool-button" type="button" data-gallery-tab="Videos">${iconMarkup('video', 'V', 'vd-tool-icon', 15)}<span>${esc(t('desktop.gallery_videos'))}</span></button>
                </div>
                <button class="vd-tool-button" type="button" data-gallery-refresh>${iconMarkup('refresh', 'R', 'vd-tool-icon', 15)}<span>${esc(t('desktop.gallery_refresh'))}</span></button>
                <span class="vd-path">${esc(t('desktop.gallery_title'))}</span>
            </div>
            <div class="vd-gallery-grid" data-gallery-grid>${esc(t('desktop.loading'))}</div>
            <div class="vd-gallery-footer">
                <button class="vd-button" type="button" data-gallery-more hidden>${esc(t('desktop.gallery_load_more'))}</button>
            </div>
        </div>`;

        const grid = host.querySelector('[data-gallery-grid]');
        const moreButton = host.querySelector('[data-gallery-more]');
        let visibleItems = [];

        const renderItems = (items, kind) => {
            grid.innerHTML = items.length ? items.map(file => {
                const preview = kind === 'video'
                    ? `<video src="${esc(file.web_path)}" preload="metadata" muted></video>`
                    : `<img src="${esc(file.web_path)}" alt="${esc(file.name)}" loading="lazy" decoding="async">`;
                return `<article class="vd-gallery-card" data-gallery-item data-path="${esc(file.path)}" data-web-path="${esc(file.web_path)}" data-media-kind="${esc(file.media_kind || kind)}" data-mime-type="${esc(file.mime_type || '')}" data-name="${esc(file.name)}">
                    <button type="button" class="vd-gallery-preview" data-gallery-open>${preview}</button>
                    <div class="vd-gallery-card-meta">
                        <span>${esc(file.name)}</span>
                        <div class="vd-gallery-actions">
                            <button type="button" class="vd-icon-button" data-gallery-open title="${esc(t('desktop.gallery_open'))}">${iconMarkup('folder-open', 'O', 'vd-gallery-action-icon', 14)}</button>
                            <a class="vd-icon-button" data-gallery-download href="${esc(file.web_path)}" download="${esc(file.name)}" title="${esc(t('desktop.gallery_download'))}">${iconMarkup('download', 'D', 'vd-gallery-action-icon', 14)}</a>
                            <button type="button" class="vd-icon-button" data-gallery-rename title="${esc(t('desktop.gallery_rename'))}">${iconMarkup('edit', 'E', 'vd-gallery-action-icon', 14)}</button>
                            <button type="button" class="vd-icon-button danger" data-gallery-delete title="${esc(t('desktop.gallery_delete'))}">${iconMarkup('trash', 'X', 'vd-gallery-action-icon', 14)}</button>
                        </div>
                    </div>
                </article>`;
            }).join('') : `<div class="vd-empty">${esc(t('desktop.gallery_empty'))}</div>`;
            grid.querySelectorAll('[data-gallery-item]').forEach(card => {
                const file = {
                    name: card.dataset.name,
                    path: card.dataset.path,
                    web_path: card.dataset.webPath,
                    media_kind: card.dataset.mediaKind,
                    mime_type: card.dataset.mimeType
                };
                card.querySelectorAll('[data-gallery-open]').forEach(btn => btn.addEventListener('click', () => openMediaPreview(file)));
                const rename = card.querySelector('[data-gallery-rename]');
                if (rename) rename.addEventListener('click', async () => { await renamePath(file.path); await loadGallery(false); });
                const del = card.querySelector('[data-gallery-delete]');
                if (del) del.addEventListener('click', async () => { await deletePath(file.path); await loadGallery(false); });
            });
        };

        const loadGallery = async (append) => {
            const tab = host.dataset.galleryTab || 'Photos';
            host.querySelectorAll('[data-gallery-tab]').forEach(btn => btn.classList.toggle('active', btn.dataset.galleryTab === tab));
            const offset = append ? Number(host.dataset.galleryOffset || 0) : 0;
            if (!append) grid.innerHTML = esc(t('desktop.loading'));
            moreButton.hidden = true;
            try {
                const kind = tab === 'Videos' ? 'video' : 'image';
                const params = new URLSearchParams({ path: tab, recursive: 'true', limit: String(GALLERY_PAGE_SIZE), offset: String(offset) });
                const body = await api('/api/desktop/files?' + params.toString());
                const items = (body.files || []).filter(file => file.type === 'file' && file.web_path && (!file.media_kind || file.media_kind === kind));
                visibleItems = append ? visibleItems.concat(items) : items;
                host.dataset.galleryOffset = String(offset + GALLERY_PAGE_SIZE);
                renderItems(visibleItems, kind);
                moreButton.hidden = !body.has_more;
            } catch (err) {
                grid.innerHTML = `<div class="vd-empty">${esc(err.message)}</div>`;
            }
        };

        host.querySelectorAll('[data-gallery-tab]').forEach(btn => {
            btn.addEventListener('click', () => {
                host.dataset.galleryTab = btn.dataset.galleryTab;
                host.dataset.galleryOffset = '0';
                visibleItems = [];
                loadGallery(false);
            });
        });
        host.querySelector('[data-gallery-refresh]').addEventListener('click', () => {
            host.dataset.galleryOffset = '0';
            visibleItems = [];
            loadGallery(false);
        });
        moreButton.addEventListener('click', () => loadGallery(true));
        await loadGallery(false);
    }

    function webampHostNode() {
        return $('vd-window-layer') || document.body;
    }

    function disposeWebampMusic(windowId, options) {
        const current = state.webampMusic;
        if (!current) return;
        if (windowId && current.windowId && current.windowId !== windowId) return;
        if (current.unsubscribeClose) {
            try { current.unsubscribeClose(); } catch (_) {}
        }
        if (!options || !options.fromWebampClose) {
            if (current.instance && typeof current.instance.dispose === 'function') {
                try { current.instance.dispose(); } catch (_) {}
            }
        }
        state.webampMusic = null;
    }

    async function loadWebampConstructor() {
        const mod = await import(WEBAMP_MODULE_PATH);
        return mod.default || mod.Webamp || mod;
    }

    function webampTrackTitle(name) {
        return String(name || '').replace(/\.[^.]+$/, '') || String(name || '');
    }

    async function renderMusicPlayer(id) {
        const host = contentEl(id);
        if (!host) return;
        const win = state.windows.get(id);
        if (win && win.element) {
            win.element.style.minWidth = '380px';
            win.element.style.minHeight = '220px';
        }

        let currentFolder = 'Music';
        let currentTracks = [];

        host.innerHTML = `<div class="vd-webamp-launcher">
            <div class="vd-webamp-launcher-header">
                ${iconMarkup('audio', 'MP', 'vd-sprite-start-item', 34)}
                <div class="vd-webamp-launcher-copy">
                    <strong>${esc(t('desktop.app_music_player'))}</strong>
                    <span data-status>${esc(t('desktop.loading'))}</span>
                </div>
            </div>
            <div class="vd-webamp-status">
                <span data-track-count>0 ${esc(t('desktop.winamp_tracks'))}</span>
                <span data-folder>Music</span>
            </div>
            <div class="vd-webamp-launcher-actions">
                <button class="vd-button vd-button-primary" type="button" data-action="refresh-music">${esc(t('desktop.context_refresh'))}</button>
                <button class="vd-button" type="button" data-action="load-folder">${esc(t('desktop.winamp_load_folder'))}</button>
                <button class="vd-button" type="button" data-action="reopen-webamp">${esc(t('desktop.context_open'))}</button>
            </div>
        </div>`;

        const statusEl = host.querySelector('[data-status]');
        const countEl = host.querySelector('[data-track-count]');
        const folderEl = host.querySelector('[data-folder]');

        const setStatus = message => {
            if (statusEl) statusEl.textContent = message;
        };

        const renderLauncherState = () => {
            if (countEl) countEl.textContent = currentTracks.length + ' ' + t('desktop.winamp_tracks');
            if (folderEl) folderEl.textContent = currentFolder;
        };

        const notifyError = err => {
            const message = err && err.message ? err.message : String(err);
            setStatus(message);
            showDesktopNotification({ title: t('desktop.notification'), message });
        };

        const scanMusicFolder = async folder => {
            const params = new URLSearchParams({ path: folder, recursive: 'true', limit: String(WEBAMP_TRACK_SCAN_LIMIT) });
            const body = await api('/api/desktop/files?' + params.toString());
            const files = body.files || [];
            const tracks = [];
            for (const file of files) {
                if (file.type === 'file' && WEBAMP_AUDIO_PATTERN.test(file.name)) {
                    tracks.push({
                        url: file.web_path || await desktopEmbedURL(file.path),
                        metaData: { title: webampTrackTitle(file.name) }
                    });
                }
            }
            return tracks;
        };

        const ensureWebamp = async tracks => {
            if (!tracks.length) {
                disposeWebampMusic(id);
                setStatus(t('desktop.winamp_no_tracks'));
                return;
            }

            const Webamp = await loadWebampConstructor();
            if (typeof Webamp.browserIsSupported === 'function' && !Webamp.browserIsSupported()) {
                throw new Error('Webamp is not supported in this browser.');
            }

            const current = state.webampMusic;
            if (current && current.instance && current.windowId === id) {
                if (typeof current.instance.reopen === 'function') current.instance.reopen();
                if (typeof current.instance.setTracksToPlay === 'function') {
                    current.instance.setTracksToPlay(tracks);
                    setStatus(t('desktop.done'));
                    return;
                }
                disposeWebampMusic(id);
            } else if (current && current.instance) {
                disposeWebampMusic(current.windowId);
            }

            const webamp = new Webamp({ initialTracks: tracks });
            state.webampMusic = { instance: webamp, windowId: id, unsubscribeClose: null };
            if (typeof webamp.onClose === 'function') {
                state.webampMusic.unsubscribeClose = webamp.onClose(() => {
                    disposeWebampMusic(id, { fromWebampClose: true });
                    closeWindow(id);
                });
            }
            await webamp.renderWhenReady(webampHostNode());
            setStatus(t('desktop.done'));
        };

        const loadMusicLibrary = async folder => {
            currentFolder = folder || 'Music';
            setStatus(t('desktop.loading'));
            currentTracks = await scanMusicFolder(currentFolder);
            renderLauncherState();
            await ensureWebamp(currentTracks);
        };

        host.querySelector('[data-action="refresh-music"]').addEventListener('click', () => {
            loadMusicLibrary('Music').catch(notifyError);
        });
        host.querySelector('[data-action="load-folder"]').addEventListener('click', async () => {
            const folder = await promptDialog(t('desktop.winamp_load_folder'), currentFolder || 'Music');
            if (folder == null) return;
            loadMusicLibrary(folder).catch(notifyError);
        });
        host.querySelector('[data-action="reopen-webamp"]').addEventListener('click', () => {
            const current = state.webampMusic;
            if (current && current.instance && typeof current.instance.reopen === 'function') {
                current.instance.reopen();
                return;
            }
            loadMusicLibrary(currentFolder || 'Music').catch(notifyError);
        });

        renderLauncherState();
        loadMusicLibrary('Music').catch(notifyError);
    }
    function renderQuickConnect(id) {
        const host = contentEl(id);
        if (!host) return;
        host.innerHTML = `<div class="vd-quick-connect">
            <div class="vd-qc-sidebar">
                <div class="vd-qc-sidebar-header">
                    <span class="vd-qc-title">${esc(t('desktop.qc_title'))}</span>
                    <div class="vd-qc-header-actions">
                        <button class="vd-tool-button" type="button" data-action="add" title="${esc(t('desktop.qc_add_server'))}">${iconMarkup('server', 'S', 'vd-tool-icon', 15)}</button>
                        <button class="vd-tool-button" type="button" data-action="refresh">${iconMarkup('refresh', 'R', 'vd-tool-icon', 15)}<span>${esc(t('desktop.qc_refresh'))}</span></button>
                    </div>
                </div>
                <div class="vd-qc-search">
                    <input type="search" autocomplete="off" spellcheck="false" data-i18n-placeholder="desktop.qc_search_placeholder">
                </div>
                <div class="vd-qc-device-list" data-device-list>${esc(t('desktop.loading'))}</div>
            </div>
            <div class="vd-qc-terminal-area" data-terminal-area>
                <div class="vd-qc-placeholder">
                    <span class="vd-qc-placeholder-icon">${iconMarkup('terminal', 'T', 'vd-qc-placeholder-papirus-icon', 42)}</span>
                    <span class="vd-qc-placeholder-text">${esc(t('desktop.qc_select_device'))}</span>
                </div>
            </div>
        </div>`;

        const searchInput = host.querySelector('.vd-qc-search input');
        searchInput.placeholder = t('desktop.qc_search_placeholder');
        const deviceList = host.querySelector('[data-device-list]');
        const terminalArea = host.querySelector('[data-terminal-area]');
        let activeWS = null;
        let activeTerm = null;
        let activeFitAddon = null;
        let cachedDevices = null;
        let cachedCredentials = null;

        loadAll();

        host.querySelector('[data-action="refresh"]').addEventListener('click', loadAll);
        host.querySelector('[data-action="add"]').addEventListener('click', () => showServerModal());
        searchInput.addEventListener('input', () => filterDevices());

        async function loadAll() {
            deviceList.innerHTML = `<div class="vd-empty">${esc(t('desktop.loading'))}</div>`;
            try {
                const [devBody, credBody] = await Promise.all([
                    api('/api/devices'),
                    api('/api/credentials')
                ]);
                cachedDevices = withAuraGoHostDevice((devBody.devices || devBody || []).filter(d => d.type === 'server' || d.type === 'generic' || d.type === 'linux' || d.type === 'vm' || !d.type));
                cachedCredentials = credBody || [];
                if (!cachedDevices.length) {
                    deviceList.innerHTML = `<div class="vd-empty">${esc(t('desktop.qc_no_devices'))}</div>`;
                    return;
                }
                renderDeviceList(cachedDevices);
            } catch (err) {
                deviceList.innerHTML = `<div class="vd-empty">${esc(err.message)}</div>`;
            }
        }

        function auraGoHostName() {
            return (location.hostname || location.host || 'localhost').replace(/^\[|\]$/g, '');
        }

        function withAuraGoHostDevice(devices) {
            const hostName = auraGoHostName();
            const normalizedHost = hostName.toLowerCase();
            const hasAuraHost = devices.some(device => {
                const address = String(device.ip_address || '').toLowerCase();
                const name = String(device.name || '').toLowerCase();
                return address === normalizedHost || name === 'aurago host';
            });
            if (hasAuraHost) return devices;
            return [{
                id: '__aurago-host__',
                name: 'AuraGo Host',
                type: 'server',
                ip_address: hostName,
                port: 22,
                description: 'Current AuraGo web host',
                is_template: true
            }, ...devices];
        }

        function renderDeviceList(devices) {
            const query = searchInput.value.trim().toLowerCase();
            const filtered = query ? devices.filter(d =>
                (d.name || '').toLowerCase().includes(query) ||
                (d.ip_address || '').toLowerCase().includes(query) ||
                (d.description || '').toLowerCase().includes(query)
            ) : devices;
            if (!filtered.length) {
                deviceList.innerHTML = `<div class="vd-empty">${esc(t('desktop.qc_no_devices'))}</div>`;
                return;
            }
            deviceList.innerHTML = filtered.map(d => {
                const cred = d.credential_id && cachedCredentials ? cachedCredentials.find(c => c.id === d.credential_id) : null;
                const endpoint = `${d.ip_address || ''}${d.port && d.port !== 22 ? ':' + d.port : ''}`;
                return `<button class="vd-qc-device${d.is_template ? ' template' : ''}" type="button" data-device-id="${esc(d.id)}">
                    <span class="vd-qc-device-icon">${iconMarkup(d.is_template ? 'home' : 'server', 'S', 'vd-qc-device-papirus-icon', 22)}</span>
                    <div class="vd-qc-device-main">
                        <div class="vd-qc-device-name">${esc(d.name)}</div>
                        <div class="vd-qc-device-meta">${esc(endpoint || '')}</div>
                        ${d.description ? `<div class="vd-qc-device-desc">${esc(d.description)}</div>` : ''}
                    </div>
                    <div class="vd-qc-device-badges">
                        ${d.is_template ? '<span class="vd-qc-badge vd-qc-badge-info">Setup</span>' : (d.credential_id ? '<span class="vd-qc-badge vd-qc-badge-ok">SSH</span>' : '<span class="vd-qc-badge vd-qc-badge-warn">?</span>')}
                    </div>
                </button>`;
            }).join('');
            deviceList.querySelectorAll('.vd-qc-device').forEach(btn => {
                btn.addEventListener('click', () => {
                    const dev = cachedDevices.find(d => d.id === btn.dataset.deviceId);
                    if (dev && dev.is_template) showServerModal(dev);
                    else connectToDevice(btn.dataset.deviceId);
                });
                btn.addEventListener('contextmenu', (e) => {
                    e.preventDefault();
                    const dev = cachedDevices.find(d => d.id === btn.dataset.deviceId);
                    if (dev) showDeviceContextMenu(e.clientX, e.clientY, dev);
                });
            });
        }

        function filterDevices() {
            if (cachedDevices) renderDeviceList(cachedDevices);
        }

        function showDeviceContextMenu(x, y, device) {
            closeContextMenu();
            const items = [
                { label: device.is_template ? t('desktop.qc_add_server') : t('desktop.qc_connect'), icon: device.is_template ? 'server' : 'terminal', fallback: 'T', action: () => device.is_template ? showServerModal(device) : connectToDevice(device.id) },
                { label: t('desktop.qc_edit'), icon: 'edit', fallback: 'E', action: () => showServerModal(device) }
            ];
            if (!device.is_template) {
                items.push({ separator: true }, { label: t('desktop.qc_delete'), icon: 'trash', fallback: 'X', action: () => confirmDeleteDevice(device) });
            }
            showContextMenu(x, y, items);
        }

        async function confirmDeleteDevice(device) {
            const ok = await showConfirmModal(t('desktop.qc_delete_confirm'), t('desktop.qc_delete_confirm_msg').replace('{{name}}', device.name));
            if (!ok) return;
            try {
                await api('/api/devices/' + device.id, { method: 'DELETE' });
                await loadAll();
            } catch (err) {
                showNotify(t('desktop.qc_delete_error') + ': ' + err.message);
            }
        }

        function showNotify(msg) {
            const existing = host.querySelector('.vd-qc-notify');
            if (existing) existing.remove();
            const el = document.createElement('div');
            el.className = 'vd-qc-notify';
            el.textContent = msg;
            host.querySelector('.vd-quick-connect').appendChild(el);
            setTimeout(() => el.remove(), 4000);
        }

        function showConfirmModal(title, message) {
            return new Promise(resolve => {
                const overlay = document.createElement('div');
                overlay.className = 'vd-qc-modal-overlay';
                overlay.innerHTML = `<div class="vd-qc-confirm">
                    <div class="vd-qc-confirm-title">${esc(title)}</div>
                    <div class="vd-qc-confirm-msg">${esc(message)}</div>
                    <div class="vd-qc-confirm-actions">
                        <button class="vd-qc-btn vd-qc-btn-secondary" type="button" data-action="cancel">${iconMarkup('x', 'X', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.cancel'))}</span></button>
                        <button class="vd-qc-btn vd-qc-btn-danger" type="button" data-action="ok">${iconMarkup('trash', 'X', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.delete'))}</span></button>
                    </div>
                </div>`;
                host.querySelector('.vd-quick-connect').appendChild(overlay);
                overlay.querySelector('[data-action="cancel"]').addEventListener('click', () => { overlay.remove(); resolve(false); });
                overlay.querySelector('[data-action="ok"]').addEventListener('click', () => { overlay.remove(); resolve(true); });
            });
        }

        function showServerModal(existingDevice) {
            const isTemplate = !!(existingDevice && existingDevice.is_template);
            const isEdit = !!existingDevice && !isTemplate;
            const existingCred = isEdit && existingDevice.credential_id && cachedCredentials
                ? cachedCredentials.find(c => c.id === existingDevice.credential_id) : null;

            const overlay = document.createElement('div');
            overlay.className = 'vd-qc-modal-overlay';
            overlay.innerHTML = `<div class="vd-qc-modal">
                <div class="vd-qc-modal-header">
                    <span class="vd-qc-modal-title">${esc(isEdit ? t('desktop.qc_edit_server') : t('desktop.qc_add_server'))}</span>
                    <button class="vd-qc-modal-close" type="button" data-action="close" title="${esc(t('desktop.close'))}">${iconMarkup('x', 'X', 'vd-qc-close-icon', 14)}</button>
                </div>
                <div class="vd-qc-modal-body">
                    <div class="vd-qc-form-section">
                        <div class="vd-qc-form-title">${esc(t('desktop.qc_section_server'))}</div>
                        <label class="vd-qc-label">${esc(t('desktop.qc_name'))}
                            <input class="vd-qc-input" type="text" name="name" value="${esc(existingDevice ? existingDevice.name : '')}" required>
                        </label>
                        <div class="vd-qc-form-row">
                            <label class="vd-qc-label vd-qc-flex-3">${esc(t('desktop.qc_host'))}
                                <input class="vd-qc-input" type="text" name="host" value="${esc(existingDevice ? (existingDevice.ip_address || '') : '')}" placeholder="192.168.1.1" required>
                            </label>
                            <label class="vd-qc-label vd-qc-flex-1">${esc(t('desktop.qc_port'))}
                                <input class="vd-qc-input" type="number" name="port" value="${existingDevice ? (existingDevice.port || 22) : 22}" min="1" max="65535">
                            </label>
                        </div>
                        <label class="vd-qc-label">${esc(t('desktop.qc_description'))}
                            <input class="vd-qc-input" type="text" name="description" value="${esc(existingDevice ? (existingDevice.description || '') : '')}">
                        </label>
                    </div>
                    <div class="vd-qc-form-section">
                        <div class="vd-qc-form-title">${esc(t('desktop.qc_section_credential'))}</div>
                        <label class="vd-qc-label">${esc(t('desktop.qc_username'))}
                            <input class="vd-qc-input" type="text" name="username" value="${esc(existingCred ? existingCred.username : '')}" required>
                        </label>
                        <label class="vd-qc-label">${esc(t('desktop.qc_password'))}
                            <div class="vd-qc-input-group">
                                <input class="vd-qc-input" type="password" name="password" placeholder="${isEdit && existingCred && existingCred.has_password ? t('desktop.qc_password_stored') : ''}">
                                <button class="vd-qc-input-toggle" type="button" data-action="toggle-pw" title="${esc(t('desktop.qc_password'))}">${iconMarkup('key', 'K', 'vd-qc-input-icon', 14)}</button>
                                ${isEdit && existingCred && existingCred.has_password ? `<button class="vd-qc-btn vd-qc-btn-sm vd-qc-icon-only" type="button" data-action="download-pw" title="${esc(t('desktop.qc_password'))}">${iconMarkup('download', 'D', 'vd-qc-btn-icon', 14)}</button>` : ''}
                            </div>
                        </label>
                        <label class="vd-qc-label">${esc(t('desktop.qc_certificate'))}
                            <div class="vd-qc-cert-area">
                                <textarea class="vd-qc-textarea" name="certificate_text" rows="3" placeholder="${t('desktop.qc_cert_paste_placeholder')}"></textarea>
                                <div class="vd-qc-cert-actions">
                                    <label class="vd-qc-btn vd-qc-btn-secondary vd-qc-btn-sm">
                                        ${iconMarkup('upload', 'U', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.qc_upload_cert'))}</span>
                                        <input type="file" accept=".pem,.key,.pub,.crt,.cer,.txt" name="certificate_file" hidden>
                                    </label>
                                    ${isEdit && existingCred && existingCred.has_certificate ? `<button class="vd-qc-btn vd-qc-btn-sm" type="button" data-action="download-cert">${iconMarkup('download', 'D', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.qc_download_cert'))}</span></button>` : ''}
                                </div>
                                ${isEdit && existingCred && existingCred.has_certificate ? '<span class="vd-qc-hint">' + esc(t('desktop.qc_cert_stored')) + '</span>' : ''}
                            </div>
                        </label>
                    </div>
                </div>
                <div class="vd-qc-modal-footer">
                    <button class="vd-qc-btn vd-qc-btn-secondary" type="button" data-action="cancel">${iconMarkup('x', 'X', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.cancel'))}</span></button>
                    <button class="vd-qc-btn vd-qc-btn-primary" type="button" data-action="save">${iconMarkup('save', 'S', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.qc_save'))}</span></button>
                </div>
            </div>`;

            host.querySelector('.vd-quick-connect').appendChild(overlay);

            // Certificate file upload
            const certFileInput = overlay.querySelector('input[name="certificate_file"]');
            const certTextarea = overlay.querySelector('textarea[name="certificate_text"]');
            certFileInput.addEventListener('change', async (e) => {
                const file = e.target.files[0];
                if (file) {
                    certTextarea.value = await file.text();
                }
            });

            // Password toggle
            overlay.querySelector('[data-action="toggle-pw"]').addEventListener('click', () => {
                const pwInput = overlay.querySelector('input[name="password"]');
                pwInput.type = pwInput.type === 'password' ? 'text' : 'password';
            });

            // Download password
            const dlPwBtn = overlay.querySelector('[data-action="download-pw"]');
            if (dlPwBtn && existingCred) {
                dlPwBtn.addEventListener('click', async () => {
                    try {
                        const body = await api('/api/credentials/export/' + existingCred.id + '?type=password');
                        downloadText(body.content, (existingCred.name || 'password') + '.txt');
                    } catch (err) { showNotify(err.message); }
                });
            }

            // Download certificate
            const dlCertBtn = overlay.querySelector('[data-action="download-cert"]');
            if (dlCertBtn && existingCred) {
                dlCertBtn.addEventListener('click', async () => {
                    try {
                        const body = await api('/api/credentials/export/' + existingCred.id + '?type=certificate');
                        downloadText(body.content, (existingCred.name || 'key') + '_key.pem');
                    } catch (err) { showNotify(err.message); }
                });
            }

            // Close / Cancel
            overlay.querySelector('[data-action="close"]').addEventListener('click', () => overlay.remove());
            overlay.querySelector('[data-action="cancel"]').addEventListener('click', () => overlay.remove());

            // Save
            overlay.querySelector('[data-action="save"]').addEventListener('click', async () => {
                const name = overlay.querySelector('input[name="name"]').value.trim();
                const hostVal = overlay.querySelector('input[name="host"]').value.trim();
                const port = parseInt(overlay.querySelector('input[name="port"]').value) || 22;
                const description = overlay.querySelector('input[name="description"]').value.trim();
                const username = overlay.querySelector('input[name="username"]').value.trim();
                const password = overlay.querySelector('input[name="password"]').value;
                const certificateText = certTextarea.value.trim();

                if (!name || !hostVal || !username) {
                    showNotify(t('desktop.qc_validation_error'));
                    return;
                }

                try {
                    if (isEdit) {
                        // Update credential if exists
                        if (existingCred) {
                            const credBody = { name: name, type: 'ssh', host: hostVal, username: username, description: description, certificate_mode: 'text' };
                            if (password) credBody.password = password;
                            if (certificateText) credBody.certificate_text = certificateText;
                            await api('/api/credentials/' + existingCred.id, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(credBody) });
                        } else {
                            // Create credential and link
                            const credBody = { name: name, type: 'ssh', host: hostVal, username: username, description: description, certificate_mode: 'text' };
                            if (password) credBody.password = password;
                            if (certificateText) credBody.certificate_text = certificateText;
                            const created = await api('/api/credentials', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(credBody) });
                            existingDevice.credential_id = created.id;
                        }
                        // Update device
                        await api('/api/devices/' + existingDevice.id, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name, type: existingDevice.type || 'server', ip_address: hostVal, port, description, credential_id: existingDevice.credential_id }) });
                    } else {
                        // Create credential first
                        const credBody = { name: name, type: 'ssh', host: hostVal, username: username, description: description, certificate_mode: 'text' };
                        if (password) credBody.password = password;
                        if (certificateText) credBody.certificate_text = certificateText;
                        const created = await api('/api/credentials', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(credBody) });
                        // Create device linked to credential
                        await api('/api/devices', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name, type: 'server', ip_address: hostVal, port, description, credential_id: created.id }) });
                    }
                    overlay.remove();
                    await loadAll();
                } catch (err) {
                    showNotify(t('desktop.qc_save_error') + ': ' + err.message);
                }
            });
        }

        function downloadText(content, filename) {
            const blob = new Blob([content], { type: 'text/plain' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = filename;
            a.click();
            URL.revokeObjectURL(url);
        }

        async function connectToDevice(deviceId) {
            deviceList.querySelectorAll('.vd-qc-device').forEach(btn => btn.classList.toggle('active', btn.dataset.deviceId === deviceId));
            if (activeWS) { try { activeWS.close(); } catch(_) {} activeWS = null; }
            if (activeTerm) { activeTerm.dispose(); activeTerm = null; }
            terminalArea.innerHTML = `<div class="vd-qc-placeholder"><span class="vd-qc-placeholder-text">${esc(t('desktop.qc_connecting'))}</span></div>`;

            const term = new Terminal({
                theme: {
                    background: '#0d1117', foreground: '#c9d1d9', cursor: '#58a6ff',
                    selectionBackground: 'rgba(88, 166, 255, 0.3)',
                    black: '#0d1117', red: '#ff7b72', green: '#3fb950', yellow: '#d29922',
                    blue: '#58a6ff', magenta: '#bc8cff', cyan: '#39c5cf', white: '#c9d1d9',
                    brightBlack: '#484f58', brightRed: '#ffa198', brightGreen: '#56d364',
                    brightYellow: '#e3b341', brightBlue: '#79c0ff', brightMagenta: '#d2a8ff',
                    brightCyan: '#56d4dd', brightWhite: '#f0f6fc'
                },
                fontFamily: "'Cascadia Code', 'JetBrains Mono', 'Fira Code', 'Consolas', monospace",
                fontSize: 14, cursorBlink: true, cursorStyle: 'bar', scrollback: 5000, convertEol: true
            });
            const fitAddon = new FitAddon.FitAddon();
            term.loadAddon(fitAddon);
            const termContainer = document.createElement('div');
            termContainer.className = 'vd-qc-term-container';
            terminalArea.replaceChildren(termContainer);
            term.open(termContainer);
            activeTerm = term;
            activeFitAddon = fitAddon;
            setTimeout(() => { try { fitAddon.fit(); } catch(_) {} }, 50);
            const resizeObserver = new ResizeObserver(() => {
                if (activeTerm === term) { try { fitAddon.fit(); } catch(_) {} }
            });
            resizeObserver.observe(termContainer);

            const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = proto + '//' + location.host + '/api/desktop/ssh?device_id=' + encodeURIComponent(deviceId) + '&cols=' + term.cols + '&rows=' + term.rows;
            const ws = new WebSocket(wsUrl);
            ws.binaryType = 'arraybuffer';
            activeWS = ws;

            term.onData(data => {
                if (ws.readyState === WebSocket.OPEN) ws.send(new TextEncoder().encode(data));
            });
            term.onResize(({ cols, rows }) => {
                if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: 'resize', cols, rows }));
            });
            ws.onmessage = (event) => {
                if (typeof event.data === 'string') {
                    try {
                        const msg = JSON.parse(event.data);
                        if (msg.type === 'error') term.write('\r\n\x1b[31m' + msg.message + '\x1b[0m\r\n');
                        else if (msg.type === 'disconnected') term.write('\r\n\x1b[33m' + msg.message + '\x1b[0m\r\n');
                    } catch(_) {}
                } else {
                    const bytes = event.data instanceof ArrayBuffer ? new Uint8Array(event.data) : new TextEncoder().encode(event.data);
                    term.write(bytes);
                }
            };
            ws.onclose = () => {
                if (activeWS === ws) { term.write('\r\n\x1b[33m' + t('desktop.qc_disconnected') + '\x1b[0m\r\n'); activeWS = null; }
            };
            ws.onerror = () => { term.write('\r\n\x1b[31m' + t('desktop.qc_connection_error') + '\x1b[0m\r\n'); };
        }
    }

    function renderLaunchpad(id) {
        const host = contentEl(id);
        if (!host) return;
        let links = [];
        let categories = [];
        let searchQuery = '';
        let selectedCategory = '';
        let iconSearchDebounce = null;
        let selectedIconURL = null;

        host.innerHTML = `
            <div class="vd-launchpad">
                <div class="vd-launchpad-toolbar">
                    <input type="search" class="vd-launchpad-search" data-i18n-placeholder="desktop.launchpad_search" placeholder="Search links...">
                    <select class="vd-launchpad-category"><option value="">All categories</option></select>
                    <button class="vd-tool-button" type="button" data-action="add">${iconMarkup('file-plus', '+', 'vd-tool-icon', 15)}<span>${esc(t('desktop.launchpad_add'))}</span></button>
                </div>
                <div class="vd-launchpad-grid"></div>
                <div class="vd-launchpad-empty" style="display:none">
                    <div class="vd-launchpad-empty-icon">${iconMarkup('apps', 'A', 'vd-launchpad-empty-papirus-icon', 42)}</div>
                    <div>${esc(t('desktop.launchpad_empty'))}</div>
                </div>
            </div>`;

        const grid = host.querySelector('.vd-launchpad-grid');
        const empty = host.querySelector('.vd-launchpad-empty');
        const searchInput = host.querySelector('.vd-launchpad-search');
        const categorySelect = host.querySelector('.vd-launchpad-category');

        async function load() {
            try {
                const url = selectedCategory ? '/api/launchpad/links?category=' + encodeURIComponent(selectedCategory) : '/api/launchpad/links';
                links = await api(url);
                categories = await api('/api/launchpad/categories');
                updateCategorySelect();
                render();
            } catch (e) { showDesktopNotification({ message: t('desktop.launchpad_load_error') }); }
        }

        function updateCategorySelect() {
            const val = categorySelect.value;
            categorySelect.innerHTML = '<option value="">' + esc(t('desktop.launchpad_all_categories')) + '</option>';
            categories.forEach(c => { categorySelect.innerHTML += '<option value="' + esc(c) + '">' + esc(c) + '</option>'; });
            categorySelect.value = val;
        }

        function render() {
            let filtered = links;
            const q = searchQuery.toLowerCase().trim();
            if (q) filtered = filtered.filter(l => (l.title || '').toLowerCase().includes(q) || (l.description || '').toLowerCase().includes(q));
            if (filtered.length === 0) { grid.innerHTML = ''; empty.style.display = ''; return; }
            empty.style.display = 'none';
            grid.innerHTML = filtered.map(link => {
                const icon = link.icon_path ? '<img class="vd-launchpad-tile-icon" src="/files/' + esc(link.icon_path) + '" alt="" loading="lazy" onerror="this.style.display=\'none\';this.nextElementSibling.style.display=\'flex\'">' : '';
                const fallback = '<div class="vd-launchpad-tile-fallback" style="display:' + (link.icon_path ? 'none' : 'flex') + '">' + iconMarkup('globe', 'G', 'vd-launchpad-fallback-icon', 34) + '</div>';
                return '<div class="vd-launchpad-tile" data-id="' + esc(link.id) + '">' + icon + fallback +
                    '<div class="vd-launchpad-tile-title">' + esc(link.title) + '</div>' +
                    (link.description ? '<div class="vd-launchpad-tile-desc">' + esc(link.description) + '</div>' : '') +
                    '<div class="vd-launchpad-tile-actions">' +
                    '<button type="button" class="vd-launchpad-tile-btn" data-action="edit" title="' + esc(t('desktop.launchpad_edit')) + '">' + iconMarkup('edit', 'E', 'vd-launchpad-action-icon', 15) + '</button>' +
                    '<button type="button" class="vd-launchpad-tile-btn" data-action="delete" title="' + esc(t('desktop.launchpad_delete')) + '">' + iconMarkup('trash', 'X', 'vd-launchpad-action-icon', 15) + '</button>' +
                    '</div></div>';
            }).join('');
            grid.querySelectorAll('.vd-launchpad-tile').forEach(tile => {
                tile.addEventListener('click', (e) => { if (!e.target.closest('.vd-launchpad-tile-actions')) openTileLink(tile.dataset.id); });
            });
            grid.querySelectorAll('[data-action="edit"]').forEach(btn => {
                btn.addEventListener('click', (e) => { e.stopPropagation(); openEditModal(btn.closest('.vd-launchpad-tile').dataset.id); });
            });
            grid.querySelectorAll('[data-action="delete"]').forEach(btn => {
                btn.addEventListener('click', (e) => { e.stopPropagation(); deleteLink(btn.closest('.vd-launchpad-tile').dataset.id); });
            });
        }

        function openTileLink(linkId) {
            const link = links.find(l => l.id === linkId);
            if (link && link.url) window.open(link.url, '_blank', 'noopener,noreferrer');
        }

        async function deleteLink(linkId) {
            const ok = await confirmDialog(t('desktop.launchpad_delete_confirm'), '');
            if (!ok) return;
            try { await api('/api/launchpad/links/' + linkId, { method: 'DELETE' }); await load(); }
            catch (e) { showDesktopNotification({ message: t('desktop.launchpad_delete_error') }); }
        }

        async function openEditModal(linkId) {
            const link = linkId ? links.find(l => l.id === linkId) : null;
            selectedIconURL = null;
            const backdrop = document.createElement('div');
            backdrop.className = 'vd-modal-backdrop';
            backdrop.innerHTML = `
                <form class="vd-modal" role="dialog" aria-modal="true" style="width:min(480px, calc(100vw - 32px)); max-height: 80vh; overflow-y: auto;">
                    <div class="vd-modal-title">${esc(linkId ? t('desktop.launchpad_edit_title') : t('desktop.launchpad_add_title'))}</div>
                    <div style="display:flex; flex-direction:column; gap:10px; margin: 10px 0;">
                        <input type="hidden" class="lp-id" value="${esc(linkId || '')}">
                        <input type="text" class="vd-modal-input lp-title" placeholder="${esc(t('desktop.launchpad_label_title'))}" value="${esc(link ? link.title : '')}" required>
                        <input type="url" class="vd-modal-input lp-url" placeholder="${esc(t('desktop.launchpad_label_url'))}" value="${esc(link ? link.url : '')}" required>
                        <input type="text" class="vd-modal-input lp-category" placeholder="${esc(t('desktop.launchpad_label_category'))}" list="lp-cats" value="${esc(link ? link.category : '')}">
                        <datalist id="lp-cats">${(categories || []).map(c => '<option value="' + esc(c) + '">').join('')}</datalist>
                        <input type="text" class="vd-modal-input lp-description" placeholder="${esc(t('desktop.launchpad_label_description'))}" value="${esc(link ? link.description : '')}">
                        <div style="font-size: 12px; color: var(--vd-muted); margin-top: 4px;">${esc(t('desktop.launchpad_label_icon'))}</div>
                        <div class="lp-icon-tabs"><button type="button" class="lp-icon-tab active" data-tab="search">${esc(t('desktop.launchpad_tab_search'))}</button><button type="button" class="lp-icon-tab" data-tab="url">${esc(t('desktop.launchpad_tab_url'))}</button></div>
                        <div class="lp-icon-panel active" data-panel="search"><div class="lp-icon-search-row"><input type="text" class="lp-icon-search" placeholder="plex, nginx..."><button type="button" class="vd-tool-button lp-icon-search-btn">${iconMarkup('search', 'S', 'vd-tool-icon', 15)}</button></div><div class="lp-icon-results"></div><div class="lp-icon-selected-preview" style="display:none;"></div></div>
                        <div class="lp-icon-panel" data-panel="url"><input type="url" class="lp-icon-url" placeholder="https://..."><div class="lp-icon-preview"></div></div>
                        <input type="hidden" class="lp-icon-path" value="${esc(link && link.icon_path ? link.icon_path : '')}">
                    </div>
                    <div class="vd-modal-actions">
                        <button type="button" class="vd-button" data-action="cancel">${esc(t('desktop.cancel'))}</button>
                        <button type="button" class="vd-button vd-button-primary" data-action="save">${esc(t('desktop.save'))}</button>
                    </div>
                </form>`;
            document.body.appendChild(backdrop);
            const modal = backdrop.querySelector('.vd-modal');
            const preview = modal.querySelector('.lp-icon-preview');
            const selectedPreview = modal.querySelector('.lp-icon-selected-preview');
            if (link && link.icon_path) {
                const imgTag = '<img src="/files/' + esc(link.icon_path) + '" style="max-width:64px;max-height:64px; border-radius:8px;">';
                preview.innerHTML = imgTag;
                if (selectedPreview) { selectedPreview.style.display = 'flex'; selectedPreview.innerHTML = imgTag; }
            }

            modal.querySelectorAll('.lp-icon-tab').forEach(tab => {
                tab.addEventListener('click', () => {
                    modal.querySelectorAll('.lp-icon-tab').forEach(t => t.classList.remove('active'));
                    modal.querySelectorAll('.lp-icon-panel').forEach(p => p.classList.remove('active'));
                    tab.classList.add('active');
                    modal.querySelector('[data-panel="' + tab.dataset.tab + '"]').classList.add('active');
                });
            });
            modal.querySelector('.lp-icon-search').addEventListener('input', (e) => {
                clearTimeout(iconSearchDebounce);
                iconSearchDebounce = setTimeout(() => searchIcons(modal, e.target.value), 400);
            });
            modal.querySelector('.lp-icon-search-btn').addEventListener('click', () => searchIcons(modal, modal.querySelector('.lp-icon-search').value));
            modal.querySelector('.lp-icon-url').addEventListener('input', (e) => {
                const u = e.target.value.trim();
                preview.innerHTML = u ? '<img src="' + esc(u) + '" style="max-width:64px;max-height:64px; border-radius:8px;" onerror="this.style.display=\'none\'">' : '';
            });
            modal.querySelector('[data-action="cancel"]').addEventListener('click', () => backdrop.remove());
            modal.querySelector('[data-action="save"]').addEventListener('click', () => saveLink(modal, linkId));
            backdrop.addEventListener('click', (e) => { if (e.target === backdrop) backdrop.remove(); });
        }

        async function searchIcons(modal, query) {
            const resultsEl = modal.querySelector('.lp-icon-results');
            if (!query.trim()) { resultsEl.innerHTML = ''; return; }
            resultsEl.innerHTML = '<div class="vd-loading">' + esc(t('desktop.loading')) + '</div>';
            try {
                const results = await api('/api/launchpad/icons/search?q=' + encodeURIComponent(query));
                const items = (results || []).filter(r => r.url_png || r.url_webp || r.url_svg);
                if (!items.length) {
                    resultsEl.innerHTML = '<div class="lp-icon-msg" style="color:var(--vd-muted);">' + esc(t('desktop.launchpad_icon_no_results')) + '</div>';
                    return;
                }
                resultsEl.innerHTML = items.map(r => {
                    const img = r.url_png || r.url_webp || r.url_svg;
                    return '<div class="lp-icon-result" data-url="' + esc(img) + '"><img src="' + esc(img) + '" alt="" loading="lazy"><span>' + esc(r.name) + '</span></div>';
                }).join('');
                resultsEl.querySelectorAll('.lp-icon-result').forEach(el => {
                    el.addEventListener('click', () => {
                        resultsEl.querySelectorAll('.lp-icon-result').forEach(x => x.classList.remove('selected'));
                        el.classList.add('selected');
                        selectedIconURL = el.dataset.url;
                        const previewEl = modal.querySelector('.lp-icon-selected-preview');
                        if (previewEl) {
                            previewEl.style.display = 'flex';
                            previewEl.innerHTML = '<img src="' + esc(el.dataset.url) + '" style="width:48px;height:48px;border-radius:8px;object-fit:contain;" onerror="this.style.display=\'none\'">';
                        }
                    });
                });
            } catch (e) {
                resultsEl.innerHTML = '<div class="lp-icon-msg" style="color:var(--vd-coral);">' + esc(t('desktop.launchpad_icon_search_error')) + '</div>';
            }
        }

        async function saveLink(modal, linkId) {
            const title = modal.querySelector('.lp-title').value.trim();
            const url = modal.querySelector('.lp-url').value.trim();
            const category = modal.querySelector('.lp-category').value.trim();
            const description = modal.querySelector('.lp-description').value.trim();
            let iconPath = modal.querySelector('.lp-icon-path').value;

            const activeTab = modal.querySelector('.lp-icon-tab.active');
            const iconUrl = activeTab && activeTab.dataset.tab === 'search' ? selectedIconURL : modal.querySelector('.lp-icon-url').value.trim();
            if (iconUrl) {
                try {
                    const dl = await api('/api/launchpad/icons/download', { method: 'POST', body: JSON.stringify({ image_url: iconUrl, link_id: linkId || 'new' }) });
                    if (dl && dl.local_path) iconPath = dl.local_path;
                } catch (e) { /* ignore download errors */ }
            }

            const payload = { title, url, category, description, icon_path: iconPath };
            try {
                if (linkId) {
                    await api('/api/launchpad/links/' + linkId, { method: 'PUT', body: JSON.stringify(payload) });
                } else {
                    await api('/api/launchpad/links', { method: 'POST', body: JSON.stringify(payload) });
                }
                modal.closest('.vd-modal-backdrop').remove();
                await load();
            } catch (e) { showDesktopNotification({ message: t('desktop.launchpad_save_error') }); }
        }

        host.querySelector('[data-action="add"]').addEventListener('click', () => openEditModal());
        searchInput.addEventListener('input', (e) => { searchQuery = e.target.value; render(); });
        categorySelect.addEventListener('change', (e) => { selectedCategory = e.target.value; load(); });

        load();
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
