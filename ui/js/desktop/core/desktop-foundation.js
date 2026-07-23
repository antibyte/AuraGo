// AUDIT(Task27 / 2026-06-02): All 24 `.style.*` assignments in this file were reviewed.
// No tokenizable hard-coded colors/sizes/radii were found. Every match is one of:
//   - Reading current inline state (parseInt on style.left/top/width) for math
//   - Writing a dynamic computed value (icon grid, widget bounds, drag delta)
//   - Writing a CSS custom property via setProperty('--vd-*')
//   - Numeric fallback (`18`) for default icon offset when position data is missing
// No migration needed for Task 28. See reports/virtual_desktop_js_audit_2026-06-01.md
// for the line-by-line breakdown (lines 389-1360).
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
        selectedIconIds: new Set(),
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
    const ICON_GRID_KEY = 'aurago.desktop.iconGrid.v1';
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
    const WIDGET_AUTO_SIZE_PADDING = 6, WIDGET_FRAME_SCROLLBAR_BUFFER = 6, WIDGET_FRAME_CHROME_BUFFER = 32, WIDGET_WIDTH_GROW_THRESHOLD = 2, WIDGET_AUTO_WIDTH_MAX = 420;

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
        Trash: 'trash-empty',
        Shared: 'share'
    };
    const appIconKeys = {
        analytics: 'analytics',
        backup: 'backup',
        backups: 'backup',
        book: 'book',
        books: 'book',
        camera: 'camera',
        chess: 'chess',
        cloud: 'cloud',
        files: 'folder',
        editor: 'edit',
        forms: 'forms',
        writer: 'writer',
        sheets: 'spreadsheet',
        cheater: 'cheater',
        help: 'help',
        settings: 'settings',
        calendar: 'calendar',
        calculator: 'calculator',
        gallery: 'gallery',
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
        noisemaker: 'audio',
        radio: 'radio',
        openscad: 'openscad',
        teevee: 'teevee',
        todo: 'notes',
        'agent-chat': 'agent-chat',
        'live-speech': 'audio',
        'sip-phone': 'phone',
        terminal: 'terminal', 'quick-connect': 'server',
        browser: 'browser', viewer: 'eye',
        launchpad: 'launchpad',
        'software-store': 'software-store',
        'store-n8n': 'n8n',
        'store-node-red': 'node-red',
        'store-open-webui': 'open-webui',
        'store-olivetin': 'olivetin',
        'store-romm': 'romm',
        'store-dozzle': 'dozzle',
        'store-termix': 'termix',
        'store-commandcode': 'commandcode',
        looper: 'looper',
        'system-info': 'monitor',
        'virtual-computers': 'terminal',
        zipper: 'zipper',
        pixel: 'pixel',
        'galaxa-deluxe': 'galaxa-deluxe',
        nasscad: 'nasscad',
        'store-quakejs-rootless': 'quakejs',
        people: 'users',
        'mission-control': 'workflow'
    };
    appIconKeys['code-studio'] = 'code-studio';
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
        scad: 'openscad',
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
        heic: 'image',
        heif: 'image',
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
        openscad: 'openscad',
        teevee: 'teevee',
        workflow: 'workflow',
        workflows: 'workflow',
        'software-store': 'software-store',
        software_store: 'software-store',
        store: 'software-store',
        'app-store': 'software-store',
        zipper: 'zipper',
        zip: 'zipper',
        pixel: 'pixel',
        paint: 'pixel',
        trash: 'trash-empty',
        'trash-empty': 'trash-empty',
        'trash-full': 'trash-full'
    };
    const desktopSettingDefaults = {
        'appearance.wallpaper': 'groupshoot',
        'appearance.theme': 'standard',
        'appearance.accent': 'teal',
        'appearance.density': 'comfortable',
        'appearance.icon_theme': 'papirus',
        'appearance.fruity_mode': 'light',
        'desktop.icon_size': 'medium',
        'desktop.show_widgets': 'true',
        'windows.animations': 'true',
        'windows.default_size': 'balanced',
        'files.confirm_delete': 'true',
        'files.default_folder': 'Documents',
        'agent.show_chat_button': 'true',
        // Mobile experience (Phase 0)
        'desktop.mobile_experience': 'auto',   // 'auto' | 'enabled' | 'disabled'
        // Desktop pet (OpenPets)
        'pet.enabled': 'true',
        'pet.active_id': 'openpets-default',
        'pet.scale': '1.0',
        'pet.position_x': '24',
        'pet.position_y': '24',
        'pet.always_on_top': 'false'
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
            openscad: 'OS',
            teevee: 'TV',
            todo: 'Td',
            'agent-chat': 'A',
            gallery: 'G',
            'quick-connect': 'QC',
            'code-studio': 'CS',
            launchpad: 'LP',
            'software-store': 'SS',
            looper: 'Lp',
            cheater: 'Ch',
            chess: 'Ch',
            pixel: 'Px',
            'galaxa-deluxe': 'Gx',
            nasscad: 'NC',
            people: 'Pp',
            'mission-control': 'MC',
            noisemaker: 'Nm'
        };
        return map[id] || ((app && app.name && app.name[0]) || 'D').toUpperCase();
    }

    async function loadIconManifest() {
        const [spriteManifest, defaultThemeManifest, whitesurThemeManifest] = await Promise.all([
            api('/img/desktop-icons-sprite.json').catch(() => null),
            api('/img/papirus/manifest.json?v=5').catch(() => null),
            api('/img/whitesur/manifest.json?v=4').catch(() => null)
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

    function iconAlias(name) { return ({ 'arrow-left': 'chevron-left', 'arrow-right': 'chevron-right' })[name] || ''; }

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
        const dashed = normalized.replaceAll('_', '-');
        const candidates = [normalized, iconAlias(normalized), aliases[normalized], dashed, iconAlias(dashed), aliases[dashed]].filter(Boolean);
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

    function versionedIconAssetPath(path) {
        const clean = String(path || '').replace(/[")\\\r\n]/g, '');
        if (!clean || clean.startsWith('data:')) return clean;
        var v = window.BUILD_VERSION || 'dev';
        return clean + (clean.includes('?') ? '&' : '?') + 'v=' + encodeURIComponent(v);
    }
    function iconUrlStyle(path) { return 'url(' + versionedIconAssetPath(path) + ')'; }
    function shouldUseTileIconFallback(className) { return /\b(vd-sprite-icon|vd-sprite-start|vd-sprite-start-item|vd-sprite-file|vd-dock-icon|vd-task-icon|vd-window-header-icon|fm-thumb-fallback-icon|fm-sidebar-icon|fm-empty-icon|vd-launchpad-empty-papirus-icon|vd-launchpad-fallback-icon)\b/.test(String(className || '')); }
    function symbolFallbackMarkup(key, fallback, className, size) { const pixels = Number(size || 16) || 16; const label = String(fallback || key || '').slice(0, 3).toUpperCase(); return `<span class="${esc(className)} vd-symbol-fallback" aria-hidden="true" style="width:${pixels}px;height:${pixels}px">${esc(label)}</span>`; }

    function appLogoIconKey(app) {
        const path = app && app.metadata && String(app.metadata.logo_path || '').trim();
        return path && (/^https?:\/\//i.test(path) || path.startsWith('/')) ? 'logo:' + path : '';
    }

    function spriteMarkup(key, fallback, className, size) {
        const manifest = state.iconManifest;
        const spriteKey = normalizeIconName(key).replace(/^sprite:/, '');
        const icon = iconExists(spriteKey) ? state.iconMap.get(spriteKey) : null;
        if (!manifest || !icon) {
            if (!shouldUseTileIconFallback(className)) return symbolFallbackMarkup(key, fallback, className, size);
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
        const logoPath = String(key || '').startsWith('logo:') ? String(key).slice(5).replace(/[\r\n"<>]/g, '').trim() : '';
        if (logoPath) {
            const pixels = Number(size || 42) || 42;
            const fallbackMarkup = iconMarkup('apps', fallback, className, size);
            return `<span class="${esc(className)} vd-app-logo-icon" aria-hidden="true" style="width:${pixels}px;height:${pixels}px"><img src="${esc(logoPath)}" alt="" loading="lazy" draggable="false" data-vd-logo-img="true" ondragstart="return false" onerror="this.hidden=true;this.nextElementSibling.hidden=false">${fallbackMarkup.replace('aria-hidden="true"', 'aria-hidden="true" hidden')}</span>`;
        }
        const source = resolveIconSource(key);
        if (source.type === 'theme') {
            const pixels = Number(size || 42) || 42;
            return `<span class="${esc(className)} vd-theme-icon vd-papirus-icon" data-vd-icon-key="${esc(key)}" aria-hidden="true" style="--vd-theme-icon-url:${esc(iconUrlStyle(source.path))};width:${pixels}px;height:${pixels}px"></span>`;
        }
        return spriteMarkup(source.key || key, fallback, className, size);
    }
    function refreshThemeIconElements(root) { (root || document).querySelectorAll('.vd-theme-icon[data-vd-icon-key], .vd-papirus-icon[data-vd-icon-key]').forEach(icon => { const path = themeIconPath(icon.dataset.vdIconKey || ''); if (path) icon.style.setProperty('--vd-theme-icon-url', iconUrlStyle(path)); }); }
    function themeBackedAppIconKey(app) {
        if (!app) return '';
        for (const key of [appIconKeys[app.id], app.icon]) {
            const candidate = String(key || '').trim();
            if (candidate && themeIconPath(candidate)) return candidate;
        }
        return '';
    }
    function iconForApp(app) {
        if (!app) return 'apps';
        return themeBackedAppIconKey(app) || appLogoIconKey(app) || appIconKeys[app.id] || app.icon || 'apps';
    }
    function shortcutIconForApp(shortcut, app) {
        const themed = themeBackedAppIconKey(app);
        if (themed) return themed;
        const appLogo = appLogoIconKey(app);
        if (appLogo) return appLogo;
        if (!shortcut) shortcut = {};
        if (!app) app = {};
        return appIconKeys[app.id] || shortcut.icon || app.icon || '';
    }
    function iconForDirectory(name) { return directoryIconKeys[name] || 'folder'; }

    function iconForFile(file) {
        if (file.type === 'directory') return iconForDirectory(file.name);
        if (file.media_kind === 'image') return 'image';
        if (file.media_kind === 'audio') return 'audio';
        if (file.media_kind === 'video') return 'video';
        const ext = String(file.name || '').split('.').pop().toLowerCase();
        if (!ext || ext === String(file.name || '').toLowerCase()) return 'file';
        const extIcon = extensionIconKeys[ext];
        if (extIcon) return extIcon;
        if (file.media_kind === 'document') return 'documents';
        return 'file';
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

    function userFacingApps() {
        return allApps().filter(app => app.internal !== true);
    }

    function startMenuApps() {
        return userFacingApps().filter(app => app.start_visible !== false);
    }

    function dockApps() {
        return userFacingApps().filter(app => app.dock_visible !== false);
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
            openscad: 'OpenSCADApp',
            looper: 'LooperApp',
            camera: 'CameraApp',
            'network-cameras': 'NetworkCamerasApp',
            zipper: 'ZipperApp',
            pixel: 'PixelApp',
            'galaxa-deluxe': 'GalaxaDeluxe',
            nasscad: 'NasscadApp',
            people: 'PeopleApp',
            'homepage-studio': 'HomepageStudioApp',
            cheater: 'CheaterApp',
            'agent-chat': 'AgentChatApp',
            'live-speech': 'LiveSpeechApp',
            'sip-phone': 'SipPhoneApp',
            'virtual-computers': 'VirtualComputersApp',
            'viewer-3d': 'Viewer3DApp',
            'mission-control': 'MissionControlApp',
            'game-maker-studio': 'GameMakerStudioApp',
            settings: 'SettingsApp',
            calculator: 'CalculatorApp',
            'system-world': 'SysWorldApp',
            noisemaker: 'NoisemakerApp'
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

    function animationsEnabled() { return !(document.body && document.body.dataset.animations === 'false') && !(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches); }
    function animateThen(element, className, fallbackMs, done) { if (!element || !className || !animationsEnabled()) { if (typeof done === 'function') done(); return; } let finished = false, timer = 0; const finish = () => { if (finished) return; finished = true; element.removeEventListener('animationend', onEnd); element.removeEventListener('transitionend', onEnd); element.classList.remove(className); if (timer) window.clearTimeout(timer); if (typeof done === 'function') done(); }; const onEnd = event => { if (event.target === element) finish(); }; element.classList.remove(className); void element.offsetWidth; element.classList.add(className); element.addEventListener('animationend', onEnd); element.addEventListener('transitionend', onEnd); timer = window.setTimeout(finish, Math.max(20, Number(fallbackMs) || 160)); }
    function closeWindowMenu() { document.querySelectorAll('.vd-window-menu.open').forEach(menu => { const popover = menu.querySelector(':scope > .vd-window-menu-popover'); if (!animationsEnabled() || !popover) { menu.classList.remove('open', 'closing'); return; } menu.classList.add('closing'); animateThen(popover, 'vd-window-menu-popover-closing', isFruityTheme() ? 150 : 100, () => { menu.classList.remove('open', 'closing'); }); }); state.openWindowMenu = null; }

    function isFruityTheme() { return settingValue('appearance.theme') === 'fruity'; }

    function applyDesktopSettings() {
        const body = document.body;
        body.dataset.wallpaper = settingValue('appearance.wallpaper');
        body.dataset.theme = settingValue('appearance.theme');
        body.dataset.fruityMode = settingValue('appearance.fruity_mode');
        body.dataset.accent = settingValue('appearance.accent');
        body.dataset.density = settingValue('appearance.density');
        body.dataset.iconTheme = settingValue('appearance.icon_theme');
        body.dataset.animations = settingValue('windows.animations');
        body.dataset.widgets = settingValue('desktop.show_widgets');
        body.dataset.iconSize = settingValue('desktop.icon_size');
        const sizes = { small: 34, medium: 42, large: 52 };
        body.style.setProperty('--vd-icon-glyph-size', (sizes[settingValue('desktop.icon_size')] || 42) + 'px');
        refreshThemeIconElements(document);
        const agentButton = $('vd-agent-button');
        if (agentButton) agentButton.hidden = !settingBool('agent.show_chat_button');
        renderTaskbar();
    }

    function iconGlyphPixels() {
        const sizes = { small: 34, medium: 42, large: 52 };
        return sizes[settingValue('desktop.icon_size')] || 42;
    }

    function isCompactViewport() {
        if (!window.matchMedia) return false;
        // Width catches narrow layouts. Height alone is common on laptops/browser panes,
        // so only use it for coarse-pointer landscape mobile/tablet layouts.
        const widthMatch = window.matchMedia('(max-width: 820px)').matches;
        const heightMatch = window.matchMedia('(max-height: 720px)').matches;
        const coarsePointerMatch = window.matchMedia('(hover: none) and (pointer: coarse)').matches;
        return widthMatch || (heightMatch && coarsePointerMatch);
    }

    function isTouchLikePointer(event) {
        if (event && (event.pointerType === 'touch' || event.pointerType === 'pen')) return true;
        return !!(window.matchMedia && window.matchMedia('(hover: none) and (pointer: coarse)').matches);
    }

    function shouldOpenOnTap(event) {
        return isTouchLikePointer(event) || isCompactViewport();
    }

    /**
     * Returns whether the enhanced mobile desktop experience should be active.
     * Respects the 'desktop.mobile_experience' setting.
     */
    function useMobileDesktopMode() {
        const setting = settingValue('desktop.mobile_experience');
        if (setting === 'enabled') return true;
        if (setting === 'disabled') return false;
        // 'auto' mode → decide based on viewport + input type
        return isCompactViewport() && isTouchLikePointer();
    }

    // Expose for other modules
    window.useMobileDesktopMode = useMobileDesktopMode;

    /**
     * Returns true if we should force a maximized + single-window
     * experience for this app on mobile devices.
     */
    function shouldForceMobileMaximizedWindow(appId) {
        if (!useMobileDesktopMode()) return false;

        // These apps are intentionally wide and should keep horizontal scrolling
        const wideAppsOnMobile = [
            'files', 'writer', 'sheets', 'code-studio',
            'viewer', 'viewer-3d', 'nasscad', 'mission-control', 'launchpad'
        ];

        return !wideAppsOnMobile.includes(appId);
    }

    window.shouldForceMobileMaximizedWindow = shouldForceMobileMaximizedWindow;

    /**
     * =====================================================
     * MOBILE DESKTOP EXPERIENCE (2026)
     * =====================================================
     * Decision (confirmed):
     * - On mobile (useMobileDesktopMode() === true):
     *     → Normal apps open as a single maximized window.
     *     → Complex/wide apps (Code Studio, Sheets, etc.) keep
     *       horizontal scrolling via vd-mobile-wide-window.
     *
     * This gives a clean phone experience while preserving
     * necessary functionality for content-heavy apps.
     */

    function updateViewportMetrics() {
        const visual = window.visualViewport;
        const height = visual && visual.height ? visual.height : window.innerHeight;
        document.documentElement.style.setProperty('--vd-visual-height', Math.max(1, Math.round(height)) + 'px');
        scheduleFruityDockOcclusionCheck();
    }

    function bindViewportMetrics() {
        updateViewportMetrics();
        window.addEventListener('resize', updateViewportMetrics);
        // Orientation change can fire before visualViewport updates on some devices (esp. older iOS/Android)
        window.addEventListener('orientationchange', () => {
            // Small delay ensures visualViewport has settled after rotation
            window.setTimeout(updateViewportMetrics, 60);
            window.setTimeout(updateViewportMetrics, 220);
        });
        const workspace = $('vd-workspace');
        if (workspace) {
            workspace.addEventListener('scroll', () => {
                // Use requestAnimationFrame to throttle desktop icon rendering on scroll
                if (!state._iconScrollFrame) {
                    state._iconScrollFrame = requestAnimationFrame(() => {
                        renderIcons();
                        state._iconScrollFrame = 0;
                    });
                }
            }, { passive: true });
        }
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

    function desktopIconGridEnabled() {
        return readJSONStorage(ICON_GRID_KEY, false) === true;
    }

    function setDesktopIconGridEnabled(enabled) {
        writeJSONStorage(ICON_GRID_KEY, !!enabled);
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

    function desktopIconGridMetrics() {
        const workspace = $('vd-workspace');
        const width = (workspace && workspace.clientWidth) || window.innerWidth || 920;
        const height = (workspace && workspace.clientHeight) || window.innerHeight || 520;
        const originX = 18;
        const originY = 18;
        const cellW = 92;
        const cellH = 104;
        const maxX = Math.max(originX, width - 92 - 8);
        const maxY = Math.max(originY, height - 88 - 8);
        return {
            originX,
            originY,
            cellW,
            cellH,
            maxX,
            maxY,
            columns: Math.max(1, Math.floor((maxX - originX) / cellW) + 1),
            rows: Math.max(1, Math.floor((maxY - originY) / cellH) + 1)
        };
    }

    function desktopIconGridPosition(index) {
        const metrics = desktopIconGridMetrics();
        const col = Math.floor(index / metrics.rows);
        const row = index % metrics.rows;
        return clampDesktopIconPosition(metrics.originX + col * metrics.cellW, metrics.originY + row * metrics.cellH);
    }

    function desktopIconGridCellKey(col, row) {
        return col + ':' + row;
    }

    function desktopIconGridCellFromPosition(left, top) {
        const metrics = desktopIconGridMetrics();
        return {
            col: Math.max(0, Math.min(metrics.columns - 1, Math.round((left - metrics.originX) / metrics.cellW))),
            row: Math.max(0, Math.min(metrics.rows - 1, Math.round((top - metrics.originY) / metrics.cellH)))
        };
    }

    function desktopIconGridPositionForCell(cell) {
        const metrics = desktopIconGridMetrics();
        return clampDesktopIconPosition(metrics.originX + cell.col * metrics.cellW, metrics.originY + cell.row * metrics.cellH);
    }

    function desktopIconGridUsedCells(excludedIds) {
        const excluded = excludedIds || new Set();
        const used = new Set();
        document.querySelectorAll('.vd-icon[data-id]').forEach(icon => {
            const id = icon.dataset.id || '';
            if (excluded.has(id)) return;
            const left = parseInt(icon.style.left, 10) || 0;
            const top = parseInt(icon.style.top, 10) || 0;
            const cell = desktopIconGridCellFromPosition(left, top);
            used.add(desktopIconGridCellKey(cell.col, cell.row));
        });
        return used;
    }

    function desktopIconGridNearestFreePosition(left, top, usedCells) {
        const metrics = desktopIconGridMetrics();
        const used = usedCells || new Set();
        let best = null;
        let bestDistance = Infinity;
        for (let col = 0; col < metrics.columns; col++) {
            for (let row = 0; row < metrics.rows; row++) {
                const key = desktopIconGridCellKey(col, row);
                if (used.has(key)) continue;
                const pos = desktopIconGridPositionForCell({ col, row });
                const distance = Math.hypot(pos.x - left, pos.y - top);
                if (distance < bestDistance) {
                    best = { key, pos };
                    bestDistance = distance;
                }
            }
        }
        if (!best) return clampDesktopIconPosition(left, top);
        used.add(best.key);
        return best.pos;
    }

    function arrangeDesktopIconsToGrid() {
        document.querySelectorAll('.vd-icon[data-id]').forEach((icon, index) => {
            const pos = desktopIconGridPosition(index);
            icon.style.left = pos.x + 'px';
            icon.style.top = pos.y + 'px';
            saveIconPosition(icon.dataset.id, pos.x, pos.y);
        });
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
        const requestOptions = Object.assign({ credentials: 'same-origin', cache: 'no-store' }, options || {});
        const resp = await fetch(url, requestOptions);
        const contentType = resp.headers.get('content-type') || '';
        const shouldParseJSON = contentType.includes('application/json') || String(url).includes('.json');
        const body = shouldParseJSON ? await resp.json() : {};
        if (!resp.ok) {
            const err = new Error(body.error || body.message || ('HTTP ' + resp.status));
            err.body = body;
            throw err;
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
        if (win.appId === 'openscad') callAppDispose(window.OpenSCADApp, win.id);
        if (win.appId === 'teevee') callAppDispose(window.TeeVeeApp, win.id);
        if (win.appId === 'system-info') callAppDispose(window.SystemInfoApp, win.id);
        const disposeName = appGlobalName(win.appId);
        const fallbackName = appGlobalFallbackName(win.appId);
        const disposed = callAppDispose(disposeName ? window[disposeName] : null, win.id);
        if (!disposed && fallbackName) callAppDispose(window[fallbackName], win.id);
    }

    function registerWidgetCleanup(cleanup) {
        if (typeof cleanup !== 'function') return;
        if (!state.widgetCleanups) state.widgetCleanups = [];
        const entry = { cleanup, active: true };
        state.widgetCleanups.push(entry);
        if (state._widgetCleanupCard) {
            const card = state._widgetCleanupCard;
            if (!card._widgetCleanupEntries) card._widgetCleanupEntries = [];
            card._widgetCleanupEntries.push(entry);
        }
    }

    function runWidgetCleanupEntry(entry) {
        if (!entry) return;
        const cleanup = typeof entry === 'function' ? entry : entry.cleanup;
        if (typeof cleanup !== 'function') return;
        if (entry.active === false) return;
        entry.active = false;
        try { cleanup(); } catch (err) { console.warn('Desktop widget cleanup failed', err); }
    }

    function cleanupWidgetCard(card) {
        if (!card) return;
        blankWidgetFrames(card);
        (card._widgetCleanupEntries || []).forEach(runWidgetCleanupEntry);
        card._widgetCleanupEntries = [];
        card._widgetRuntimeReady = false;
        card._widgetLastResizePayload = null;
        if (card._widgetResizeFrame) {
            window.cancelAnimationFrame(card._widgetResizeFrame);
            card._widgetResizeFrame = 0;
        }
        if (card._widgetResizeObserver) {
            card._widgetResizeObserver.disconnect();
            card._widgetResizeObserver = null;
        }
        card._widgetCleanupRegistered = false;
    }

    function clearWidgetRuntime() {
        const cleanups = state.widgetCleanups || [];
        state.widgetCleanups = [];
        cleanups.forEach(runWidgetCleanupEntry);
    }

    function blankWidgetFrames(host) {
        if (!host || typeof host.querySelectorAll !== 'function') return;
        host.querySelectorAll('iframe').forEach(frame => {
            try { frame.src = 'about:blank'; } catch (_) {}
        });
    }

    function withWidgetCleanupScope(card, callback) {
        const previous = state._widgetCleanupCard;
        state._widgetCleanupCard = card || null;
        try { return callback(); } finally { state._widgetCleanupCard = previous; }
    }

    function widgetShouldAutoSize(widget) {
        if (!widget) return true;
        const configured = widget.auto_size !== undefined ? widget.auto_size : (widget.autoSize !== undefined ? widget.autoSize : widget.autosize);
        return !(configured === false || configured === 0 || String(configured).toLowerCase() === 'false');
    }

    function scheduleWidgetAutoSize(card, widget) { if (!card || !widgetShouldAutoSize(widget)) return; card.dataset.widgetAutoSize = 'true'; applyWidgetAutoSize(card, card._widgetLastResizePayload || {}); }
    function applyWidgetAutoSize(card, payload) { if (!card || card.dataset.widgetAutoSize !== 'true') return; const data = payload && typeof payload === 'object' ? payload : {}; const frameWrap = card.querySelector('.vd-widget-frame-wrap'); const reportedFrameHeight = Number(data.height || data.h || 0); if (frameWrap && reportedFrameHeight > 0) { const frameHeight = clampWidgetFrameHeight(card, reportedFrameHeight + WIDGET_FRAME_SCROLLBAR_BUFFER); setWidgetPixelVar(card, '--vd-widget-frame-height', frameHeight); setWidgetPixelVar(frameWrap, '--vd-widget-frame-height', frameHeight); } const measuredContentHeight = widgetMeasuredContentHeight(card, data); const renderedScrollHeight = reportedFrameHeight > 0 ? 0 : Math.ceil(card.scrollHeight || 0); const desiredHeight = Math.max(WIDGET_MIN_HEIGHT, Math.ceil(Number(data.cardHeight || data.card_height || 0)), measuredContentHeight, renderedScrollHeight); setWidgetPixelVar(card, '--vd-widget-auto-height', clampWidgetHeight(card, desiredHeight, WIDGET_MIN_HEIGHT)); }
    function resizeWidgetToContent(widgetId, payload) { const id = String(widgetId || ''); if (!id) return; const card = document.querySelector(`.vd-widget[data-widget-id="${cssSel(id)}"]`); if (!card || card.dataset.widgetAutoSize !== 'true') return; const data = payload && typeof payload === 'object' ? payload : {}; card._widgetLastResizePayload = data; const reportedWidth = Number(data.width || data.w || 0); const reportedViewportWidth = Number(data.viewportWidth || data.viewport_width || 0); if (reportedWidth > 16) { const shouldGrowWidth = !reportedViewportWidth || reportedWidth > reportedViewportWidth + WIDGET_WIDTH_GROW_THRESHOLD; const desiredWidth = shouldGrowWidth ? reportedWidth + WIDGET_FRAME_CHROME_BUFFER : widgetPreferredWidth(card); const nextWidth = Math.max(220, Math.min(Math.ceil(desiredWidth), widgetMaxWidth(card))); setWidgetWidthIfChanged(card, nextWidth); } applyWidgetAutoSize(card, data); }
    function widgetMeasuredContentHeight(card, data) { if (!card) return 0; let bottom = 0; const frameWrap = card.querySelector('.vd-widget-frame-wrap'); if (frameWrap) bottom = Math.max(bottom, widgetElementBottom(card, frameWrap)); ['.vd-widget-builtin', '.vd-widget-body', '.vd-quickchat-response'].forEach(selector => { const target = card.querySelector(selector); if (target) bottom = Math.max(bottom, widgetElementBottom(card, target)); }); const requestedCardHeight = Number(data.cardHeight || data.card_height || 0); return Math.ceil(Math.max(bottom, requestedCardHeight, 0) + WIDGET_AUTO_SIZE_PADDING); }
    function widgetElementBottom(card, element) { if (!card || !element) return 0; const cardRect = typeof card.getBoundingClientRect === 'function' ? card.getBoundingClientRect() : null; const elementRect = typeof element.getBoundingClientRect === 'function' ? element.getBoundingClientRect() : null; const cardStyle = window.getComputedStyle ? window.getComputedStyle(card) : null; const paddingBottom = parseFloat(cardStyle && cardStyle.paddingBottom) || 0; const rectBottom = cardRect && elementRect ? elementRect.bottom - cardRect.top + paddingBottom : 0; const layoutBottom = (element.offsetTop || 0) + Math.max(element.scrollHeight || 0, element.offsetHeight || 0); return Math.ceil(Math.max(rectBottom, layoutBottom)); }
    function clampWidgetFrameHeight(card, height) { const available = Math.max(WIDGET_MIN_FRAME_HEIGHT, widgetAvailableHeight(card) - 32); return Math.max(WIDGET_MIN_FRAME_HEIGHT, Math.min(Math.ceil(height), available)); }
    function clampWidgetHeight(card, height, minimum) { return Math.max(minimum, Math.min(Math.ceil(height), widgetAvailableHeight(card))); }
    function widgetAvailableHeight(card) { const workspace = $('vd-workspace'); const workspaceHeight = (workspace && workspace.clientHeight) || window.innerHeight || 600; const top = parseInt(card.style.top, 10) || card.offsetTop || 0; return Math.max(WIDGET_MIN_HEIGHT, workspaceHeight - top - WIDGET_MAX_BOTTOM_GAP); }
    function widgetMaxWidth(card) { const workspace = $('vd-workspace'); const workspaceWidth = (workspace && workspace.clientWidth) || window.innerWidth || 960; const left = parseInt(card.style.left, 10) || card.offsetLeft || 0; return Math.max(220, workspaceWidth - left - 18); }
    function widgetPreferredWidth(card) { const configured = Number(card && card.dataset.widgetDefaultWidth || 0); const preferred = configured > 16 ? configured : 320; return Math.max(220, Math.min(preferred, WIDGET_AUTO_WIDTH_MAX)); }
    function setWidgetPixelVar(element, name, value) { if (!element) return; const next = Math.ceil(value) + 'px'; if (element.style.getPropertyValue(name) !== next) element.style.setProperty(name, next); }
    function setWidgetWidthIfChanged(card, width) { if (!card) return; const next = Math.ceil(width); const current = Math.round(parseFloat(card.style.width) || card.offsetWidth || 0); if (Math.abs(current - next) > 1) card.style.width = next + 'px'; }

    function renderAppError(id, appId, err) {
        console.error('Desktop app render failed', { appId, windowId: id, error: err });
        const host = contentEl(id);
        if (!host) return;
        host.innerHTML = `<div class="vd-app-error">
            <div class="vd-app-error-title">${esc(t('desktop.app_error_title'))}</div>
            <div class="vd-app-error-message">${esc((err && err.message) || String(err || 'Error'))}</div>
        </div>`;
    }

    function trapFocus(element) {
        const focusable = element.querySelectorAll('button, input, select, textarea, [tabindex]:not(-1)');
        if (!focusable.length) return;
        const first = focusable[0];
        const last = focusable[focusable.length - 1];
        element.addEventListener('keydown', event => {
            if (event.key !== 'Tab') return;
            if (event.shiftKey && document.activeElement === first) {
                event.preventDefault();
                last.focus();
            } else if (!event.shiftKey && document.activeElement === last) {
                event.preventDefault();
                first.focus();
            }
        });
        first.focus();
    }

    // fetchBootstrapState loads bootstrap (+ desktop files) without rendering.
    // Used for parallel boot with icon manifests and for refresh paths.
    async function fetchBootstrapState() {
        state.bootstrap = await api('/api/desktop/bootstrap');
        state.desktopFiles = await resolveDesktopFilesFromBootstrap(state.bootstrap);
        return state.bootstrap;
    }

    async function loadBootstrap() {
        if (bootstrapReloadPromise) return bootstrapReloadPromise;
        bootstrapReloadPromise = (async () => {
            await fetchBootstrapState();
            renderDesktop();
            refreshPetRuntime();
            return state.bootstrap;
        })();
        try {
            return await bootstrapReloadPromise;
        } finally { bootstrapReloadPromise = null; }
    }

    async function resolveDesktopFilesFromBootstrap(boot) {
        if (!boot || !boot.enabled) return [];
        // Prefer embedded listing from bootstrap (single round-trip).
        if (Array.isArray(boot.desktop_files)) {
            return boot.desktop_files.filter(file => file && file.path);
        }
        return loadDesktopFiles();
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
        if (!state._startMenuDragWired) { state._startMenuDragWired = true; wireStartMenuDrag(); }
    }

    function openInitialDesktopApp() {
        if (state._initialAppOpened) return;
        state._initialAppOpened = true;
        const params = new URLSearchParams(window.location.search || '');
        const appId = (params.get('app') || '').trim();
        if (appId && isBuiltinApp(appId)) openApp(appId);
    }

    function refreshPetRuntime() {
        if (window.PetRuntime && typeof window.PetRuntime.load === 'function') {
            window.PetRuntime.load();
        }
    }

    function renderStartButtonIcon() {
        const startButton = $('vd-start-button');
        if (!startButton) return;
        const startGlyph = startButton.querySelector('.vd-start-glyph, .vd-sprite-start, .vd-papirus-icon');
        if (startGlyph) startGlyph.outerHTML = iconMarkup('home', 'A', 'vd-sprite-start', 32);
    }

    function runStartMenuMotion(menu, className, fallbackMs, done) { if (typeof animateThen === 'function') animateThen(menu, className, fallbackMs, done); else if (typeof done === 'function') done(); }

    function toggleStartMenu() { const menu = $('vd-start-menu'); if (menu.hidden || menu.classList.contains('vd-start-menu-closing')) openStartMenu(); else closeStartMenu(); }

    function ensureStartMenuBackdrop() {
        let backdrop = document.querySelector('.vd-start-menu-backdrop');
        if (!backdrop) {
            backdrop = document.createElement('div');
            backdrop.className = 'vd-start-menu-backdrop';
            backdrop.addEventListener('click', closeStartMenu);
            document.body.appendChild(backdrop);
        }
        return backdrop;
    }

    function openStartMenu() {
        const menu = $('vd-start-menu'); if (!menu) return;
        menu.dataset.motionState = 'open'; menu.classList.remove('vd-start-menu-closing'); menu.hidden = false; menu.style.transform = '';
        runStartMenuMotion(menu, 'vd-start-menu-opening', isFruityTheme() ? 190 : 130);
        menu.classList.add('vd-start-menu-just-opened');
        window.clearTimeout(menu._justOpenedTimer);
        menu._justOpenedTimer = window.setTimeout(() => menu.classList.remove('vd-start-menu-just-opened'), 900);
        const startButton = $('vd-start-button'); if (startButton) { startButton.dataset.active = 'true'; startButton.setAttribute('aria-expanded', 'true'); }
        if (!isCompactViewport()) $('vd-start-search').focus();
        if (isCompactViewport()) { const bd = ensureStartMenuBackdrop(); requestAnimationFrame(() => bd.classList.add('active')); }
    }

    function closeStartMenu() {
        const menu = $('vd-start-menu'); if (!menu || menu.hidden) return; menu.dataset.motionState = 'closing';
        menu.style.transition = ''; menu.style.transform = '';
        const startButton = $('vd-start-button'); if (startButton) { startButton.dataset.active = 'false'; startButton.setAttribute('aria-expanded', 'false'); }
        window.clearTimeout(menu._justOpenedTimer);
        menu.classList.remove('vd-start-menu-just-opened');
        const bd = document.querySelector('.vd-start-menu-backdrop'); if (bd) bd.classList.remove('active');
        runStartMenuMotion(menu, 'vd-start-menu-closing', isFruityTheme() ? 170 : 120, () => { if (menu.dataset.motionState === 'closing') menu.hidden = true; });
    }

    function wireStartMenuDrag() {
        const menu = $('vd-start-menu'); const handle = menu && menu.querySelector('.vd-start-sheet-handle'); if (!handle) return;
        let drag = null;
        handle.addEventListener('pointerdown', event => {
            if (event.button !== 0) return;
            drag = { pointerId: event.pointerId, y: event.clientY, startY: event.clientY, time: performance.now() };
            handle.setPointerCapture(event.pointerId);
            menu.style.transition = 'none';
        });
        handle.addEventListener('pointermove', event => {
            if (!drag || drag.pointerId !== event.pointerId) return;
            const dy = event.clientY - drag.startY;
            if (dy > 0) menu.style.transform = 'translateY(' + dy + 'px)';
        });
        handle.addEventListener('pointerup', event => {
            if (!drag || drag.pointerId !== event.pointerId) return;
            const dy = event.clientY - drag.startY;
            const elapsed = Math.max(1, performance.now() - drag.time);
            const velocity = dy / elapsed;
            drag = null;
            handle.releasePointerCapture(event.pointerId);
            menu.style.transition = '';
            if (dy > 120 || (dy > 40 && velocity > 0.6)) {
                closeStartMenu();
            } else {
                menu.style.transform = 'translateY(0)';
                window.setTimeout(() => { menu.style.transition = ''; menu.style.transform = ''; }, 240);
            }
        });
        handle.addEventListener('pointercancel', event => {
            if (drag && drag.pointerId === event.pointerId) { drag = null; handle.releasePointerCapture(event.pointerId); menu.style.transition = ''; menu.style.transform = ''; }
        });
    }

    function renderIcons() {
        const items = desktopShortcutItems();
        const positions = iconPositions();
        const visibleItems = filterVisibleIcons(items, positions);
        reconcileDesktopIcons(visibleItems, positions);
        syncDesktopIconSelection();
    }

    function getWorkspaceVisibleArea() {
        const workspace = $('vd-workspace');
        if (!workspace) return { left: 0, top: 0, right: window.innerWidth, bottom: window.innerHeight };
        return {
            left: workspace.scrollLeft,
            top: workspace.scrollTop,
            right: workspace.scrollLeft + workspace.clientWidth,
            bottom: workspace.scrollTop + workspace.clientHeight
        };
    }

    function filterVisibleIcons(items, positions) {
        const area = getWorkspaceVisibleArea();
        const buffer = 120; // Extra buffer around viewport
        return items.filter((item, index) => {
            const pos = positions[item.id] || defaultIconPosition(index);
            const iconW = 92;
            const iconH = 88;
            return (
                pos.x + iconW >= area.left - buffer &&
                pos.x <= area.right + buffer &&
                pos.y + iconH >= area.top - buffer &&
                pos.y <= area.bottom + buffer
            );
        });
    }

    function updateDesktopIconButton(btn, item, pos) {
        const iconKey = item.icon || (item.type === 'file' ? iconForFile(item.file) : item.type === 'directory' ? iconForDirectory(item.name) : iconForApp(item.app));
        const fallback = item.type === 'app' ? iconGlyph(item.app) : item.name;
        btn.className = 'vd-icon';
        btn.classList.toggle('selected', state.selectedIconIds.has(item.id));
        btn.type = 'button';
        btn.setAttribute('role', 'button');
        btn.setAttribute('aria-label', item.name);
        btn.setAttribute('aria-selected', state.selectedIconIds.has(item.id) ? 'true' : 'false');
        btn.dataset.kind = item.type;
        btn.dataset.id = item.id;
        btn.dataset.appId = item.app ? item.app.id : '';
        btn.dataset.path = item.path || '';
        btn.dataset.webPath = item.file ? item.file.web_path || '' : '';
        btn.dataset.mediaKind = item.file ? item.file.media_kind || '' : '';
        btn.dataset.mimeType = item.file ? item.file.mime_type || '' : '';
        btn.dataset.desktopEntry = item.desktopEntry ? 'true' : 'false';
        btn.style.left = (Number(pos.x) || 18) + 'px';
        btn.style.top = (Number(pos.y) || 18) + 'px';
        const renderedHTML = `${iconMarkup(iconKey, fallback, 'vd-sprite-icon', iconGlyphPixels())}<span class="vd-icon-label">${esc(item.name)}</span>`;
        if (btn.dataset.renderedHtml !== renderedHTML) {
            btn.innerHTML = renderedHTML;
            btn.dataset.renderedHtml = renderedHTML;
        }
    }

    function bindDesktopIconButton(btn) {
        if (btn.getAttribute('data-vd-icon-bound') === 'true') return;
        btn.setAttribute('data-vd-icon-bound', 'true');
        btn.addEventListener('dblclick', () => activateDesktopItem(btn));
        btn.addEventListener('click', event => {
            if (btn.__vdSuppressNextClick) {
                btn.__vdSuppressNextClick = false;
                event.preventDefault();
                return;
            }
            if (shouldOpenOnTap(event)) {
                event.preventDefault();
                activateDesktopItem(btn);
                return;
            }
            selectDesktopIcon(btn, { extend: event.ctrlKey || event.metaKey, toggle: event.ctrlKey || event.metaKey });
        });
        btn.addEventListener('contextmenu', event => showIconContextMenu(event, btn));
        wireLongPress(btn, event => showIconContextMenu(event, btn));
        wireDraggableIcon(btn);
        if (typeof wireDesktopFileIconDrag === 'function') wireDesktopFileIconDrag(btn);
    }

    function reconcileDesktopIcons(items, positions) {
        const icons = $('vd-icons');
        const seenIconIds = new Set();
        items.forEach((item, index) => {
            seenIconIds.add(item.id);
            const pos = positions[item.id] || defaultIconPosition(index);
            let btn = icons.querySelector(`.vd-icon[data-id="${cssSel(item.id)}"]`);
            if (!btn) {
                btn = document.createElement('button');
                btn.style.setProperty('--icon-index', String(index));
                icons.insertBefore(btn, icons.children[index] || null);
                updateDesktopIconButton(btn, item, pos);
                bindDesktopIconButton(btn);
            } else {
                if (btn !== icons.children[index]) icons.insertBefore(btn, icons.children[index] || null);
                updateDesktopIconButton(btn, item, pos);
            }
        });
        icons.querySelectorAll('.vd-icon[data-id]').forEach(btn => {
            if (!seenIconIds.has(btn.dataset.id)) btn.remove();
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
                    icon: shortcutIconForApp(shortcut, app),
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

    function normalizeDesktopPath(path) {
        return String(path || '').replace(/\\/g, '/').split('/').filter(Boolean).join('/');
    }

    function isTrashPath(path) {
        return normalizeDesktopPath(path).toLowerCase() === 'trash';
    }

    function isInsideTrashPath(path) {
        return normalizeDesktopPath(path).toLowerCase().startsWith('trash/');
    }

    function isTrashIcon(btn) {
        if (!btn || !btn.dataset) return false;
        return btn.dataset.id === 'dir-Trash' || (btn.dataset.kind === 'directory' && isTrashPath(btn.dataset.path));
    }

    function clearTrashDropTarget() {
        document.querySelectorAll('.vd-trash-drop-target').forEach(icon => icon.classList.remove('vd-trash-drop-target'));
    }

    function desktopTrashDropTargetAt(clientX, clientY, draggedIcon) {
        const icons = [...document.querySelectorAll('.vd-icon')];
        return icons.find(icon => {
            if (icon === draggedIcon || !isTrashIcon(icon)) return false;
            const rect = icon.getBoundingClientRect();
            return clientX >= rect.left && clientX <= rect.right && clientY >= rect.top && clientY <= rect.bottom;
        }) || null;
    }

    function wireDraggableIcon(btn) {
        let drag = null;
        function finishDrag(event) {
            if (!drag) return;
            if (event && event.pointerId != null && event.pointerId !== drag.pointerId) return;
            const dropTarget = drag.dropTarget;
            if (drag.holdTimer) window.clearTimeout(drag.holdTimer);
            if (event && btn.hasPointerCapture && btn.hasPointerCapture(drag.pointerId)) {
                btn.releasePointerCapture(drag.pointerId);
            }
            setDesktopDragItemsDragging(drag.items, false);
            clearTrashDropTarget();
            document.body.classList.remove('vd-touch-drag-active');
            if (drag.moved) {
                suppressDesktopIconClicks(drag.items);
                if (dropTarget && !isTrashIcon(btn)) {
                    resetDesktopDragItems(drag.items);
                    handleTrashDropForIcons(drag.items.map(item => item.icon));
                } else {
                    saveDesktopDragItems(drag.items);
                }
                if (event) event.preventDefault();
            }
            drag = null;
        }
        btn.addEventListener('dragstart', event => {
            if (btn.dataset.desktopEntry !== 'true') event.preventDefault();
        });
        btn.addEventListener('pointerdown', event => {
            if (event.button !== 0) return;
            const touchDrag = isTouchLikePointer(event);
            closeContextMenu();
            if (!btn.classList.contains('selected')) selectDesktopIcon(btn);
            drag = {
                id: btn.dataset.id,
                pointerId: event.pointerId,
                items: desktopDragItemsForIcon(btn),
                x: event.clientX,
                y: event.clientY,
                left: parseInt(btn.style.left, 10) || 0,
                top: parseInt(btn.style.top, 10) || 0,
                moved: false,
                ready: !touchDrag,
                touchDrag,
                holdTimer: 0,
                dropTarget: null
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
            setDesktopDragItemsDragging(drag.items, true);
            if (drag.touchDrag) document.body.classList.add('vd-touch-drag-active');
            const delta = clampDesktopDragDelta(drag.items, dx, dy);
            moveDesktopDragItems(drag.items, delta.dx, delta.dy);
            drag.dropTarget = desktopTrashDropTargetAt(event.clientX, event.clientY, btn);
            clearTrashDropTarget();
            if (drag.dropTarget) drag.dropTarget.classList.add('vd-trash-drop-target');
        });
        btn.addEventListener('pointerup', finishDrag);
        btn.addEventListener('pointercancel', finishDrag);
    }

    function widgetContentSignature(widget) {
        const isBuiltinType = widget.type === 'builtin' || widget.runtime === 'builtin';
        return JSON.stringify({
            id: widget.id || '',
            type: widget.type || '',
            runtime: widget.runtime || '',
            entry: widget.entry || '',
            app_id: widget.app_id || '',
            title: widget.title || '',
            builtin: isBuiltinType,
            autoSize: widgetShouldAutoSize(widget)
        });
    }

    function widgetBodyHTML(widget) {
        const isBuiltinType = widget.type === 'builtin' || widget.runtime === 'builtin';
        if (isBuiltinType) return `<div class="vd-widget-builtin" data-builtin-type="${esc(widget.id)}"></div>`;
        if (widget.entry) return `<div class="vd-widget-frame-wrap"></div>`;
        return `<div class="vd-widget-body">${esc(widget.type || widget.app_id || t('desktop.widget_custom'))}</div>`;
    }

    function updateWidgetCard(card, widget, index) {
        const bounds = widgetBounds(widget, index);
        const autoSize = widgetShouldAutoSize(widget);
        const signature = widgetContentSignature(widget);
        const changed = card.dataset.widgetSignature !== signature;
        if (changed) {
            cleanupWidgetCard(card);
            card.innerHTML = widgetBodyHTML(widget);
            card.dataset.widgetSignature = signature;
            card._widgetRuntimeReady = false;
        }
        card._widgetData = widget;
        card.className = 'vd-widget' + (autoSize ? ' vd-widget-auto' : '') + (widget.id === 'builtin-quickchat' ? ' vd-widget-quickchat' : '');
        card.dataset.widgetId = widget.id || '';
        card.dataset.appId = widget.app_id || '';
        card.dataset.widgetDefaultWidth = String(bounds.w);
        if (autoSize) card.dataset.widgetAutoSize = 'true';
        else delete card.dataset.widgetAutoSize;
        card.title = widget.title || widget.id || '';
        card.style.left = bounds.x + 'px';
        card.style.top = bounds.y + 'px';
        card.style.width = bounds.w + 'px';
        return changed;
    }

    function bindWidgetCard(card, widget) {
        card._widgetData = widget;
        if (card.dataset.widgetBound === 'true') return;
        card.dataset.widgetBound = 'true';
        card.addEventListener('contextmenu', event => showWidgetContextMenu(event, card._widgetData || widget));
        wireLongPress(card, event => showWidgetContextMenu(event, card._widgetData || widget));
        wireDraggableWidget(card, widget);
    }

    function renderWidgetRuntime(card, widget, changed) {
        const isBuiltinType = widget.type === 'builtin' || widget.runtime === 'builtin';
        if (isBuiltinType) {
            if (changed || !card._widgetRuntimeReady || widget.id === 'builtin-analog-clock') {
                withWidgetCleanupScope(card, () => renderBuiltinWidget(card, widget));
                card._widgetRuntimeReady = true;
            }
        } else if (widget.entry) {
            const frameWrap = card.querySelector('.vd-widget-frame-wrap');
            if (frameWrap && (changed || !card._widgetRuntimeReady)) {
                withWidgetCleanupScope(card, () => renderWidgetFrame(frameWrap, widget));
                card._widgetRuntimeReady = true;
            }
        }
        withWidgetCleanupScope(card, () => scheduleWidgetAutoSize(card, widget));
    }

    function renderWidgets() {
        const host = $('vd-widgets');
        const boot = state.bootstrap || {};
        const widgets = boot.widgets || [];
        const seenWidgetIds = new Set();
        widgets.forEach((widget, index) => {
            const widgetId = widget.id || ('widget-' + index);
            seenWidgetIds.add(widgetId);
            let card = host.querySelector(`:scope > .vd-widget[data-widget-id="${cssSel(widgetId)}"]`);
            if (!card) {
                card = document.createElement('article');
                card.dataset.widgetId = widgetId;
                host.insertBefore(card, host.children[index] || null);
            } else if (card !== host.children[index]) {
                host.insertBefore(card, host.children[index] || null);
            }
            const changed = updateWidgetCard(card, Object.assign({}, widget, { id: widgetId }), index);
            bindWidgetCard(card, card._widgetData);
            renderWidgetRuntime(card, card._widgetData, changed);
        });
        host.querySelectorAll(':scope > .vd-widget[data-widget-id]').forEach(card => {
            if (!seenWidgetIds.has(card.dataset.widgetId)) {
                cleanupWidgetCard(card);
                card.remove();
            }
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
