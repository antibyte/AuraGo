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
        iconThemeManifests: {},
        iconMap: new Map(),
        selectedIconId: '',
        contextMenu: null,
        contextMenuKeydown: null,
        windowMenus: new Map(),
        windowCleanups: new Map(),
        widgetCleanups: [],
        appsCache: [],
        appsCacheBootstrap: null,
        openWindowMenu: null,
        webampMusic: null,
        fruityDockOcclusionFrame: 0,
        fruityDockFootprint: null
    };
    let bootstrapReloadPromise = null;

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
    const TOUCH_DRAG_HOLD_MS = 360;
    const LONG_PRESS_MS = 600;
    const LONG_PRESS_FEEDBACK_MS = 300;
    const LONG_PRESS_MOVE_TOLERANCE = 10;
    const WIDGET_MIN_HEIGHT = 56;
    const WIDGET_MIN_FRAME_HEIGHT = 80;
    const WIDGET_MAX_BOTTOM_GAP = 18;

    const els = {};
    const directoryIconKeys = {
        Desktop: 'desktop',
        Documents: 'documents',
        Downloads: 'downloads',
        Apps: 'apps',
        Widgets: 'widgets',
        Data: 'database',
        Reports: 'analytics',
        Backups: 'backup',
        Backup: 'backup',
        Books: 'book',
        Library: 'book',
        Camera: 'camera',
        Cloud: 'cloud',
        Forms: 'forms',
        Help: 'help',
        Mail: 'mail',
        Maps: 'map',
        Network: 'network',
        Phone: 'phone',
        Printers: 'printer',
        Printer: 'printer',
        Pictures: 'image',
        Music: 'audio',
        Photos: 'image',
        Videos: 'video',
        Tools: 'tools',
        Weather: 'weather',
        Workflows: 'workflow',
        'AuraGo Documents': 'documents',
        Trash: 'trash',
        Shared: 'share'
    };
    const appIconKeys = {
        analytics: 'analytics',
        backup: 'backup',
        backups: 'backup',
        book: 'book',
        books: 'book',
        camera: 'camera',
        cloud: 'cloud',
        files: 'folder',
        editor: 'edit',
        forms: 'forms',
        writer: 'writer',
        sheets: 'spreadsheet',
        help: 'help',
        settings: 'settings',
        calendar: 'calendar',
        calculator: 'calculator',
        gallery: 'image',
        mail: 'mail',
        map: 'map',
        maps: 'map',
        network: 'network',
        phone: 'phone',
        printer: 'printer',
        print: 'printer',
        run: 'run',
        tools: 'tools',
        weather: 'weather',
        workflow: 'workflow',
        workflows: 'workflow',
        music: 'audio-player',
        'music-player': 'audio-player',
        player: 'audio-player',
        radio: 'radio',
        todo: 'notes',
        'agent-chat': 'chat',
        terminal: 'terminal',
        browser: 'browser',
        launchpad: 'launchpad',
        looper: 'refresh',
        'system-info': 'analytics'
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
        doc: 'documents',
        docx: 'documents',
        xls: 'spreadsheet',
        xlsx: 'spreadsheet',
        xlsm: 'spreadsheet',
        pptx: 'presentation',
        bak: 'backup',
        backup: 'backup',
        epub: 'book',
        mobi: 'book',
        azw3: 'book',
        heic: 'camera',
        heif: 'camera',
        eml: 'mail',
        msg: 'mail',
        vcf: 'forms',
        gpx: 'map',
        kml: 'map',
        kmz: 'map',
        geojson: 'map',
        workflow: 'workflow',
        exe: 'executable',
        bin: 'binary'
    };
    const launchpadCategoryIconKeys = {
        analytics: 'analytics',
        stats: 'analytics',
        backup: 'backup',
        backups: 'backup',
        book: 'book',
        books: 'book',
        camera: 'camera',
        cloud: 'cloud',
        forms: 'forms',
        help: 'help',
        support: 'help',
        mail: 'mail',
        email: 'mail',
        map: 'map',
        maps: 'map',
        network: 'network',
        phone: 'phone',
        printer: 'printer',
        print: 'printer',
        run: 'run',
        tools: 'tools',
        weather: 'weather',
        music: 'audio-player',
        player: 'audio-player',
        radio: 'radio',
        workflow: 'workflow',
        workflows: 'workflow'
    };
    const desktopSettingDefaults = {
        'appearance.wallpaper': 'aurora',
        'appearance.theme': 'standard',
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
            writer: 'W',
            sheets: 'Sh',
            settings: 'S',
            calendar: 'C',
            calculator: 'Ca',
            'music-player': 'MP',
            radio: 'Ra',
            todo: 'Td',
            'agent-chat': 'A',
            gallery: 'G',
            'quick-connect': 'QC',
            'code-studio': 'CS',
            launchpad: 'LP',
            looper: 'Lp'
        };
        return map[id] || ((app && app.name && app.name[0]) || 'D').toUpperCase();
    }

    async function loadIconManifest() {
        const [spriteManifest, defaultThemeManifest, whitesurThemeManifest] = await Promise.all([
            api('/img/desktop-icons-sprite.json').catch(() => null),
            api('/img/papirus/manifest.json?v=2').catch(() => null),
            api('/img/whitesur/manifest.json?v=1').catch(() => null)
        ]);
        state.iconManifest = spriteManifest;
        state.iconMap = new Map(((spriteManifest && spriteManifest.icons) || []).map(icon => [icon.name, icon]));
        state.iconThemeManifests = {
            papirus: defaultThemeManifest,
            whitesur: whitesurThemeManifest
        };
    }

    function iconExists(key) {
        return key && state.iconMap.has(key);
    }

    function normalizeIconName(name) {
        return String(name || '').trim().toLowerCase().replace(/[^a-z0-9:_-]+/g, '_');
    }

    function themeIconPath(key) {
        let normalized = normalizeIconName(key);
        if (!normalized || normalized.startsWith('sprite:')) return '';
        let theme = settingValue('appearance.icon_theme') || 'papirus';
        Object.keys(state.iconThemeManifests || {}).forEach(themeKey => {
            const prefix = themeKey + ':';
            if (normalized.startsWith(prefix)) {
                theme = themeKey;
                normalized = normalized.slice(prefix.length);
            }
        });
        const manifest = (state.iconThemeManifests || {})[theme] || (state.iconThemeManifests || {}).papirus;
        if (!manifest || !manifest.icons) return '';
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
        if (!normalized.startsWith('sprite:')) {
            const path = themeIconPath(normalized);
            if (path) return { type: 'theme', path };
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
        if (source.type === 'theme') {
            const pixels = Number(size || 42) || 42;
            return `<span class="${esc(className)} vd-theme-icon vd-papirus-icon" aria-hidden="true" style="--vd-theme-icon-url:url(${esc(source.path)});width:${pixels}px;height:${pixels}px"></span>`;
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

    function launchpadCategoryIconKey(category) {
        const normalized = normalizeIconName(category).replaceAll('_', '-');
        return launchpadCategoryIconKeys[normalized] || launchpadCategoryIconKeys[normalized.replaceAll('-', '_')] || 'globe';
    }

    function appName(app) {
        const key = 'desktop.app_' + String(app.id || '').replaceAll('-', '_');
        const translated = t(key);
        return translated === key ? (app.name || app.id) : translated;
    }

    function allApps() {
        const boot = state.bootstrap || {};
        if (state.appsCacheBootstrap === boot) return state.appsCache;
        state.appsCacheBootstrap = boot;
        state.appsCache = [...(boot.builtin_apps || []), ...(boot.installed_apps || [])];
        return state.appsCache;
    }

    function startMenuApps() {
        return allApps().filter(app => app.start_visible !== false);
    }

    function dockApps() {
        return allApps().filter(app => app.dock_visible !== false);
    }

    function appById(appId) {
        return allApps().find(app => app.id === appId);
    }

    function appGlobalName(appId) {
        return {
            files: 'FileManager',
            writer: 'WriterApp',
            sheets: 'SheetsApp',
            'code-studio': 'CodeStudioApp',
            looper: 'LooperApp',
            camera: 'CameraApp'
        }[appId] || '';
    }

    function appGlobalFallbackName(appId) {
        return {
            'code-studio': 'CodeStudio'
        }[appId] || '';
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
        const value = settingValue(key);
        if (value === false || value === 0) return false;
        if (value === true || value === 1) return true;
        if (value == null || value === '') return true;
        return String(value).toLowerCase() !== 'false' && String(value) !== '0';
    }

    function isFruityTheme() {
        return settingValue('appearance.theme') === 'fruity';
    }

    function applyDesktopSettings() {
        const body = document.body;
        body.dataset.wallpaper = settingValue('appearance.wallpaper');
        body.dataset.theme = settingValue('appearance.theme');
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
        renderTaskbar();
    }

    function iconGlyphPixels() {
        const sizes = { small: 34, medium: 42, large: 52 };
        return sizes[settingValue('desktop.icon_size')] || 42;
    }

    function isCompactViewport() {
        return !!(window.matchMedia && window.matchMedia('(max-width: 820px)').matches);
    }

    function isTouchLikePointer(event) {
        if (event && (event.pointerType === 'touch' || event.pointerType === 'pen')) return true;
        return !!(window.matchMedia && window.matchMedia('(hover: none) and (pointer: coarse)').matches);
    }

    function shouldOpenOnTap(event) {
        return isTouchLikePointer(event) || isCompactViewport();
    }

    function updateViewportMetrics() {
        const visual = window.visualViewport;
        const height = visual && visual.height ? visual.height : window.innerHeight;
        document.documentElement.style.setProperty('--vd-visual-height', Math.max(1, Math.round(height)) + 'px');
        scheduleFruityDockOcclusionCheck();
    }

    function bindViewportMetrics() {
        updateViewportMetrics();
        window.addEventListener('resize', updateViewportMetrics);
        if (window.visualViewport) {
            window.visualViewport.addEventListener('resize', updateViewportMetrics);
            window.visualViewport.addEventListener('scroll', updateViewportMetrics);
        }
    }

    function wireLongPress(element, callback, options) {
        options = options || {};
        const threshold = Number(options.threshold || LONG_PRESS_MS);
        const feedbackDelay = Number(options.feedbackDelay || LONG_PRESS_FEEDBACK_MS);
        const moveTolerance = Number(options.moveTolerance || LONG_PRESS_MOVE_TOLERANCE);
        let timer = 0;
        let feedbackTimer = 0;
        let startX = 0;
        let startY = 0;
        let pointerId = null;
        let triggered = false;
        let suppressClick = false;

        function clearTimers() {
            if (timer) window.clearTimeout(timer);
            if (feedbackTimer) window.clearTimeout(feedbackTimer);
            timer = 0;
            feedbackTimer = 0;
        }

        function clearPress() {
            clearTimers();
            element.classList.remove('vd-long-press-active');
            pointerId = null;
            triggered = false;
            window.setTimeout(() => { element.__vdLongPressTriggered = false; }, 0);
        }

        element.addEventListener('pointerdown', event => {
            if (event.button !== 0 || !isTouchLikePointer(event)) return;
            clearTimers();
            startX = event.clientX;
            startY = event.clientY;
            pointerId = event.pointerId;
            triggered = false;
            element.__vdLongPressTriggered = false;
            feedbackTimer = window.setTimeout(() => {
                element.classList.add('vd-long-press-active');
            }, feedbackDelay);
            timer = window.setTimeout(() => {
                triggered = true;
                suppressClick = true;
                element.__vdLongPressTriggered = true;
                element.classList.add('vd-long-press-active');
                event.preventDefault();
                event.stopPropagation();
                callback(event);
            }, threshold);
        });

        element.addEventListener('pointermove', event => {
            if (!timer || pointerId !== event.pointerId) return;
            if (Math.abs(event.clientX - startX) > moveTolerance || Math.abs(event.clientY - startY) > moveTolerance) {
                clearPress();
            }
        });

        element.addEventListener('pointerup', event => {
            if (pointerId !== event.pointerId) return;
            if (triggered) {
                event.preventDefault();
                event.stopPropagation();
            }
            clearPress();
        });
        element.addEventListener('pointercancel', clearPress);
        element.addEventListener('click', event => {
            if (!suppressClick) return;
            suppressClick = false;
            event.preventDefault();
            event.stopPropagation();
        }, true);
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

    function clampDesktopIconPosition(left, top) {
        return clampToWorkspace(left, top, 92, 88);
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

    function callAppDispose(app, windowId) {
        if (!app || typeof app.dispose !== 'function') return false;
        try {
            app.dispose(windowId);
            return true;
        } catch (err) {
            console.warn('Desktop app dispose failed', err);
            return false;
        }
    }

    function disposeAppWindow(win) {
        if (!win) return;
        const cleanup = state.windowCleanups.get(win.id);
        if (cleanup) {
            state.windowCleanups.delete(win.id);
            cleanup.forEach(fn => {
                try { fn(); } catch (err) { console.warn('Desktop window cleanup failed', err); }
            });
        }
        if (win.appId === 'music-player') disposeWebampMusic(win.id);
        if (win.appId === 'radio') callAppDispose(window.RadioApp, win.id);
        if (win.appId === 'system-info') callAppDispose(window.SystemInfoApp, win.id);
        const disposeName = appGlobalName(win.appId);
        const fallbackName = appGlobalFallbackName(win.appId);
        const disposed = callAppDispose(disposeName ? window[disposeName] : null, win.id);
        if (!disposed && fallbackName) callAppDispose(window[fallbackName], win.id);
    }

    function registerWidgetCleanup(cleanup) {
        if (typeof cleanup !== 'function') return;
        if (!state.widgetCleanups) state.widgetCleanups = [];
        state.widgetCleanups.push(cleanup);
    }

    function clearWidgetRuntime() {
        const cleanups = state.widgetCleanups || [];
        state.widgetCleanups = [];
        cleanups.forEach(cleanup => {
            try { cleanup(); } catch (err) { console.warn('Desktop widget cleanup failed', err); }
        });
    }

    function renderAppError(id, appId, err) {
        console.error('Desktop app render failed', { appId, windowId: id, error: err });
        const host = contentEl(id);
        if (!host) return;
        host.innerHTML = `<div class="vd-app-error">
            <div class="vd-app-error-title">${esc(t('desktop.app_error_title', 'App failed to load'))}</div>
            <div class="vd-app-error-message">${esc((err && err.message) || String(err || 'Error'))}</div>
        </div>`;
    }

    async function loadBootstrap() {
        if (bootstrapReloadPromise) return bootstrapReloadPromise;
        bootstrapReloadPromise = (async () => {
            state.bootstrap = await api('/api/desktop/bootstrap');
            state.desktopFiles = await loadDesktopFiles();
            renderDesktop();
            return state.bootstrap;
        })();
        try {
            return await bootstrapReloadPromise;
        } finally { bootstrapReloadPromise = null; }
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
        if (startGlyph) startGlyph.outerHTML = iconMarkup('home', 'A', 'vd-sprite-start', 32);
    }

    function toggleStartMenu() {
        const menu = $('vd-start-menu');
        menu.hidden = !menu.hidden;
        if (!menu.hidden && !isCompactViewport()) $('vd-start-search').focus();
    }

    function renderIcons() {
        const icons = $('vd-icons');
        const items = desktopShortcutItems();
        const positions = iconPositions();
        icons.innerHTML = items.map(item => {
            const iconKey = item.icon || (item.type === 'file' ? iconForFile(item.file) : item.type === 'directory' ? iconForDirectory(item.name) : iconForApp(item.app));
            const fallback = item.type === 'app' ? iconGlyph(item.app) : item.name;
            const pos = positions[item.id] || defaultIconPosition(items.indexOf(item));
            return `<button class="vd-icon ${item.id === state.selectedIconId ? 'selected' : ''}" type="button" data-kind="${esc(item.type)}" data-id="${esc(item.id)}" data-app-id="${esc(item.app ? item.app.id : '')}" data-path="${esc(item.path || '')}" data-web-path="${esc(item.file ? item.file.web_path || '' : '')}" data-media-kind="${esc(item.file ? item.file.media_kind || '' : '')}" data-mime-type="${esc(item.file ? item.file.mime_type || '' : '')}" data-desktop-entry="${item.desktopEntry ? 'true' : 'false'}" style="left:${Number(pos.x) || 18}px;top:${Number(pos.y) || 18}px">
                ${iconMarkup(iconKey, fallback, 'vd-sprite-icon', iconGlyphPixels())}
                <span class="vd-icon-label">${esc(item.name)}</span>
            </button>`;
        }).join('');
        icons.querySelectorAll('.vd-icon').forEach(btn => {
            btn.addEventListener('dblclick', () => activateDesktopItem(btn));
            btn.addEventListener('click', event => {
                if (shouldOpenOnTap(event)) {
                    event.preventDefault();
                    activateDesktopItem(btn);
                    return;
                }
                selectDesktopIcon(btn);
            });
            btn.addEventListener('contextmenu', event => showIconContextMenu(event, btn));
            wireLongPress(btn, event => showIconContextMenu(event, btn));
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
                    icon: shortcut.icon || '',
                    shortcut
                };
            }
            if (shortcut.target_type === 'directory') {
                return {
                    id: shortcut.id,
                    name: shortcut.name || shortcut.path,
                    type: 'directory',
                    path: shortcut.path || shortcut.target_id || '',
                    icon: shortcut.icon || '',
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
        function finishDrag(event) {
            if (!drag) return;
            if (event && event.pointerId != null && event.pointerId !== drag.pointerId) return;
            if (drag.holdTimer) window.clearTimeout(drag.holdTimer);
            if (event && btn.hasPointerCapture && btn.hasPointerCapture(drag.pointerId)) {
                btn.releasePointerCapture(drag.pointerId);
            }
            btn.classList.remove('vd-dragging');
            document.body.classList.remove('vd-touch-drag-active');
            if (drag.moved) {
                saveIconPosition(drag.id, parseInt(btn.style.left, 10) || 0, parseInt(btn.style.top, 10) || 0);
                if (event) event.preventDefault();
            }
            drag = null;
        }
        btn.addEventListener('pointerdown', event => {
            if (event.button !== 0) return;
            const touchDrag = isTouchLikePointer(event);
            closeContextMenu();
            selectDesktopIcon(btn);
            drag = {
                id: btn.dataset.id,
                pointerId: event.pointerId,
                x: event.clientX,
                y: event.clientY,
                left: parseInt(btn.style.left, 10) || 0,
                top: parseInt(btn.style.top, 10) || 0,
                moved: false,
                ready: !touchDrag,
                touchDrag,
                holdTimer: 0
            };
            if (touchDrag) {
                drag.holdTimer = window.setTimeout(() => {
                    if (drag) drag.ready = true;
                }, TOUCH_DRAG_HOLD_MS);
            }
            btn.setPointerCapture(event.pointerId);
        });
        btn.addEventListener('pointermove', event => {
            if (!drag) return;
            const dx = event.clientX - drag.x;
            const dy = event.clientY - drag.y;
            if (btn.__vdLongPressTriggered) return;
            if (drag.touchDrag && !drag.ready) {
                if (Math.hypot(dx, dy) > LONG_PRESS_MOVE_TOLERANCE) finishDrag(event);
                return;
            }
            if (!drag.moved && Math.hypot(dx, dy) < DRAG_THRESHOLD) return;
            drag.moved = true;
            btn.classList.add('vd-dragging');
            if (drag.touchDrag) document.body.classList.add('vd-touch-drag-active');
            const pos = clampToWorkspace(drag.left + dx, drag.top + dy, btn.offsetWidth, btn.offsetHeight);
            btn.style.left = pos.x + 'px';
            btn.style.top = pos.y + 'px';
        });
        btn.addEventListener('pointerup', finishDrag);
        btn.addEventListener('pointercancel', finishDrag);
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

    function ensureDesktopRadialMenuAnchor() {
        const taskbarSystem = document.querySelector('.vd-taskbar-system');
        if (!taskbarSystem) return null;
        let anchor = document.getElementById('radialMenuAnchor');
        const agentButton = $('vd-agent-button');
        if (!anchor) {
            anchor = document.createElement('div');
            anchor.id = 'radialMenuAnchor';
            anchor.className = 'vd-radial-anchor';
            if (agentButton && agentButton.parentElement === taskbarSystem) {
                taskbarSystem.insertBefore(anchor, agentButton);
            } else {
                taskbarSystem.appendChild(anchor);
            }
        } else {
            anchor.classList.add('vd-radial-anchor');
            if (anchor.parentElement !== taskbarSystem) {
                if (agentButton && agentButton.parentElement === taskbarSystem) {
                    taskbarSystem.insertBefore(anchor, agentButton);
                } else {
                    taskbarSystem.appendChild(anchor);
                }
            }
        }
        if (typeof injectRadialMenu === 'function') injectRadialMenu();
        if (typeof initRadialMenu === 'function') initRadialMenu();
        return anchor;
    }

    function renderWidgets() {
        const host = $('vd-widgets');
        clearWidgetRuntime();
        const boot = state.bootstrap || {};
        const widgets = boot.widgets || [];
        const cards = [];
        widgets.forEach((widget, index) => {
            const isBuiltinType = widget.type === 'builtin' || widget.runtime === 'builtin';
            const autoSize = widgetShouldAutoSize(widget);
            const bounds = widgetBounds(widget, index);
            let sizeClass = autoSize ? ' vd-widget-auto' : '';
            if (widget.id === 'builtin-quickchat') sizeClass += ' vd-widget-quickchat';
            const autoSizeAttr = autoSize ? ' data-widget-auto-size="true"' : '';
            const widgetBody = isBuiltinType
                ? `<div class="vd-widget-builtin" data-builtin-type="${esc(widget.id)}"></div>`
                : widget.entry
                    ? `<div class="vd-widget-frame-wrap"></div>`
                    : `<div class="vd-widget-body">${esc(widget.type || widget.app_id || t('desktop.widget_custom'))}</div>`;
            cards.push(`<article class="vd-widget${sizeClass}" data-widget-id="${esc(widget.id)}" data-app-id="${esc(widget.app_id || '')}"${autoSizeAttr} title="${esc(widget.title || widget.id)}" style="left:${bounds.x}px;top:${bounds.y}px;width:${bounds.w}px;">
                ${widgetBody}
            </article>`);
        });
        host.innerHTML = cards.join('');
        widgets.forEach(widget => {
            const card = host.querySelector(`[data-widget-id="${cssSel(widget.id)}"]`);
            if (!card) return;
            const isBuiltinType = widget.type === 'builtin' || widget.runtime === 'builtin';
            if (isBuiltinType) {
                renderBuiltinWidget(card, widget);
            } else if (widget.entry) {
                const frameWrap = card.querySelector('.vd-widget-frame-wrap');
                if (frameWrap) renderWidgetFrame(frameWrap, widget);
            }
            card.addEventListener('contextmenu', event => showWidgetContextMenu(event, widget));
            wireLongPress(card, event => showWidgetContextMenu(event, widget));
            scheduleWidgetAutoSize(card, widget);
