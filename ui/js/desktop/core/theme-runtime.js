    // Desktop theme registry and capability queries. The shell asks "what does this theme provide"
    // (layout, scheme, global menu, motion, icon family) and never "which theme is this".
    // Parsed by ui/desktop_theme_registry_test.go: one descriptor per line, single quotes, keys in this order.
    // Ids and order must equal desktop.DesktopThemeIDs() in internal/desktop/themes.go.
    const DESKTOP_THEMES = Object.freeze([
        Object.freeze({ id: 'standard', labelKey: 'desktop.settings_theme_standard', descKey: 'desktop.settings_theme_standard_desc', scheme: 'dark', schemeSetting: '', shell: 'taskbar', startMenu: 'classic', globalMenu: false, iconTheme: 'papirus', iconFamily: 'standard', accent: 'user', glassPolish: true, fx: false, staticFx: false, wallpaper: 'groupshoot', thumb: '/img/themes/standard.svg', cursorTheme: '', soundTheme: 'crystal', motion: Object.freeze({ open: 240, close: 130, minimize: 170, restore: 180, menuOpen: 100, menuClose: 100, startOpen: 130, startClose: 120 }) }),
        Object.freeze({ id: 'fruity', labelKey: 'desktop.settings_theme_fruity', descKey: 'desktop.settings_theme_fruity_desc', scheme: 'light', schemeSetting: 'appearance.fruity_mode', shell: 'dock', startMenu: 'classic', globalMenu: true, iconTheme: 'whitesur', iconFamily: 'fruity', accent: 'user', glassPolish: true, fx: false, staticFx: false, wallpaper: 'groupshoot', thumb: '/img/themes/fruity.svg', cursorTheme: '', soundTheme: 'crystal', motion: Object.freeze({ open: 240, close: 180, minimize: 200, restore: 230, menuOpen: 150, menuClose: 150, startOpen: 190, startClose: 170 }) }),
        Object.freeze({ id: 'amiga', labelKey: 'desktop.settings_theme_amiga', descKey: 'desktop.settings_theme_amiga_desc', scheme: 'light', schemeSetting: '', shell: 'taskbar', startMenu: 'classic', globalMenu: true, iconTheme: 'papirus', iconFamily: 'standard', accent: 'fixed', glassPolish: false, fx: false, staticFx: false, wallpaper: 'slate', thumb: '/img/themes/amiga.svg', cursorTheme: 'amiga', soundTheme: 'amiga', motion: Object.freeze({ open: 20, close: 20, minimize: 20, restore: 20, menuOpen: 20, menuClose: 20, startOpen: 20, startClose: 20 }) }),
        Object.freeze({ id: 'cyberpunk', labelKey: 'desktop.settings_theme_cyberpunk', descKey: 'desktop.settings_theme_cyberpunk_desc', scheme: 'dark', schemeSetting: '', shell: 'taskbar', startMenu: 'classic', globalMenu: false, iconTheme: 'papirus', iconFamily: 'standard', accent: 'fixed', glassPolish: false, fx: false, staticFx: false, wallpaper: 'city_rain', thumb: '/img/themes/cyberpunk.svg', cursorTheme: 'cyberpunk', soundTheme: 'cyberpunk', motion: Object.freeze({ open: 200, close: 150, minimize: 190, restore: 190, menuOpen: 120, menuClose: 100, startOpen: 160, startClose: 120 }) }),
        Object.freeze({ id: 'black_gloss', labelKey: 'desktop.settings_theme_black_gloss', descKey: 'desktop.settings_theme_black_gloss_desc', scheme: 'dark', schemeSetting: '', shell: 'taskbar', startMenu: 'classic', globalMenu: false, iconTheme: 'whitesur', iconFamily: 'fruity', accent: 'user', glassPolish: true, fx: false, staticFx: false, wallpaper: 'midnight', thumb: '/img/themes/black_gloss.svg', cursorTheme: 'black_gloss', soundTheme: 'black_gloss', motion: Object.freeze({ open: 260, close: 180, minimize: 230, restore: 240, menuOpen: 150, menuClose: 130, startOpen: 200, startClose: 160 }) }),
        Object.freeze({ id: 'crazy', labelKey: 'desktop.settings_theme_crazy', descKey: 'desktop.settings_theme_crazy_desc', scheme: 'dark', schemeSetting: '', shell: 'taskbar', startMenu: 'classic', globalMenu: false, iconTheme: 'papirus', iconFamily: 'standard', accent: 'fixed', glassPolish: true, fx: true, staticFx: false, wallpaper: 'fractal_trip', thumb: '/img/themes/crazy.svg', cursorTheme: 'crazy', soundTheme: 'crazy', motion: Object.freeze({ open: 520, close: 380, minimize: 440, restore: 440, menuOpen: 240, menuClose: 180, startOpen: 340, startClose: 220 }) }),
        Object.freeze({ id: 'lollipop', labelKey: 'desktop.settings_theme_lollipop', descKey: 'desktop.settings_theme_lollipop_desc', scheme: 'light', schemeSetting: '', shell: 'dock', startMenu: 'classic', globalMenu: true, iconTheme: 'papirus', iconFamily: 'standard', accent: 'fixed', glassPolish: true, fx: false, staticFx: false, wallpaper: 'paper_waves', thumb: '/img/themes/lollipop.svg', cursorTheme: 'lollipop', soundTheme: 'lollipop', motion: Object.freeze({ open: 300, close: 200, minimize: 260, restore: 280, menuOpen: 180, menuClose: 140, startOpen: 240, startClose: 180 }) }),
        Object.freeze({ id: 'organic', labelKey: 'desktop.settings_theme_organic', descKey: 'desktop.settings_theme_organic_desc', scheme: 'dark', schemeSetting: '', shell: 'taskbar', startMenu: 'classic', globalMenu: false, iconTheme: 'papirus', iconFamily: 'standard', accent: 'fixed', glassPolish: true, fx: false, staticFx: false, wallpaper: 'forest', thumb: '/img/themes/organic.svg', cursorTheme: 'organic', soundTheme: 'organic', motion: Object.freeze({ open: 280, close: 190, minimize: 240, restore: 250, menuOpen: 160, menuClose: 130, startOpen: 220, startClose: 170 }) }),
        Object.freeze({ id: 'electronic_dreams', labelKey: 'desktop.settings_theme_electronic_dreams', descKey: 'desktop.settings_theme_electronic_dreams_desc', scheme: 'dark', schemeSetting: '', shell: 'taskbar', startMenu: 'classic', globalMenu: false, iconTheme: 'papirus', iconFamily: 'standard', accent: 'fixed', glassPolish: false, fx: true, staticFx: true, wallpaper: 'electronic_dreams', thumb: '/img/themes/electronic_dreams.svg', cursorTheme: 'electronic_dreams', soundTheme: 'electronic_dreams', motion: Object.freeze({ open: 240, close: 160, minimize: 180, restore: 220, menuOpen: 120, menuClose: 100, startOpen: 150, startClose: 120 }) }),
        Object.freeze({ id: 'windows_xp', labelKey: 'desktop.settings_theme_windows_xp', descKey: 'desktop.settings_theme_windows_xp_desc', scheme: 'light', schemeSetting: '', shell: 'taskbar', startMenu: 'xp', globalMenu: false, iconTheme: 'windows_xp', iconFamily: 'standard', accent: 'fixed', glassPolish: false, fx: false, staticFx: false, wallpaper: 'windows_xp', thumb: '/img/themes/windows_xp.svg', cursorTheme: 'windows_xp', soundTheme: 'windows_xp', motion: Object.freeze({ open: 120, close: 100, minimize: 180, restore: 220, menuOpen: 120, menuClose: 100, startOpen: 150, startClose: 120 }) }),
        Object.freeze({ id: 'green_slime', labelKey: 'desktop.settings_theme_green_slime', descKey: 'desktop.settings_theme_green_slime_desc', scheme: 'dark', schemeSetting: '', shell: 'taskbar', startMenu: 'classic', globalMenu: false, iconTheme: 'papirus', iconFamily: 'standard', accent: 'fixed', glassPolish: false, fx: true, staticFx: true, wallpaper: 'green_slime', thumb: '/img/themes/green_slime.svg', cursorTheme: 'green_slime', soundTheme: 'green_slime', motion: Object.freeze({ open: 360, close: 220, minimize: 180, restore: 220, menuOpen: 120, menuClose: 100, startOpen: 150, startClose: 120 }) }),
        Object.freeze({ id: 'voxel_world', labelKey: 'desktop.settings_theme_voxel_world', descKey: 'desktop.settings_theme_voxel_world_desc', scheme: 'light', schemeSetting: '', shell: 'taskbar', startMenu: 'classic', globalMenu: false, iconTheme: 'papirus', iconFamily: 'standard', accent: 'fixed', glassPolish: false, fx: true, staticFx: true, wallpaper: 'voxel_world', thumb: '/img/themes/voxel_world.svg', cursorTheme: 'voxel_world', soundTheme: 'voxel_world', motion: Object.freeze({ open: 260, close: 100, minimize: 180, restore: 220, menuOpen: 120, menuClose: 100, startOpen: 150, startClose: 120 }) }),
        Object.freeze({ id: 'orbit', labelKey: 'desktop.settings_theme_orbit', descKey: 'desktop.settings_theme_orbit_desc', scheme: 'dark', schemeSetting: '', shell: 'taskbar', startMenu: 'classic', globalMenu: false, iconTheme: 'papirus', iconFamily: 'standard', accent: 'fixed', glassPolish: false, fx: true, staticFx: true, wallpaper: 'orbit', thumb: '/img/themes/orbit.svg', cursorTheme: '', soundTheme: 'crystal', motion: Object.freeze({ open: 620, close: 520, minimize: 460, restore: 480, menuOpen: 260, menuClose: 180, startOpen: 380, startClose: 240 }) }),
        Object.freeze({ id: 'minimal', labelKey: 'desktop.settings_theme_minimal', descKey: 'desktop.settings_theme_minimal_desc', scheme: 'light', schemeSetting: 'appearance.minimal_mode', shell: 'taskbar', startMenu: 'classic', globalMenu: false, iconTheme: 'papirus', iconFamily: 'standard', accent: 'fixed', glassPolish: false, fx: false, staticFx: false, wallpaper: 'minimal', thumb: '/img/themes/minimal.svg', cursorTheme: '', soundTheme: 'crystal', motion: Object.freeze({ open: 200, close: 140, minimize: 180, restore: 200, menuOpen: 120, menuClose: 100, startOpen: 160, startClose: 120 }) })
    ]);

    // Load token of the lazy effects bundle: a newer sync invalidates an older pending load.
    let themeFxLoadToken = 0;
    let themeFxMotionListenerBound = false;

    function desktopThemes() { return DESKTOP_THEMES; }

    function desktopThemeById(id) {
        return DESKTOP_THEMES.find(theme => theme.id === id) || null;
    }

    // An unknown stored id falls back to the first registry entry (Standard).
    function currentDesktopTheme() {
        return desktopThemeById(settingValue('appearance.theme')) || DESKTOP_THEMES[0];
    }

    // Identity alias kept for the test markers; shell code asks the capability queries below instead.
    function isFruityTheme() { return currentDesktopTheme().id === 'fruity'; }

    function shellLayout() { return currentDesktopTheme().shell; }

    function themeUsesGlobalMenu() { return currentDesktopTheme().globalMenu === true; }

    function themeScheme(theme) {
        if (theme.schemeSetting) return settingValue(theme.schemeSetting) === 'dark' ? 'dark' : 'light';
        return theme.scheme;
    }

    // JS fallback timer for animateThen; an unknown kind gets a neutral 160 ms.
    function themeMotionMs(kind) {
        const ms = currentDesktopTheme().motion[kind];
        return typeof ms === 'number' ? ms : 160;
    }

    function themeIconFamily() { return currentDesktopTheme().iconFamily; }

    function applyDesktopThemeAttributes(body) {
        const theme = currentDesktopTheme();
        if (body.dataset.theme !== theme.id) {
            body.dataset.theme = theme.id;
            desktopWindowEffectSamples.clear();
        }
        body.dataset.shell = theme.shell;
        body.dataset.startMenu = theme.startMenu;
        body.dataset.scheme = themeScheme(theme);
        body.dataset.polish = theme.glassPolish ? 'on' : 'off';
        applyDesktopCursors(body, theme);
    }

    function bindThemeFxMotionListener() {
        if (themeFxMotionListenerBound || typeof window.matchMedia !== 'function') return;
        themeFxMotionListenerBound = true;
        const query = window.matchMedia('(prefers-reduced-motion: reduce)');
        if (query && typeof query.addEventListener === 'function') query.addEventListener('change', () => syncDesktopThemeFx());
        else if (query && typeof query.addListener === 'function') query.addListener(() => syncDesktopThemeFx());
    }

    // Material themes retain a still rendering when motion is disabled. Legacy FX remain opt-in.
    let themeFxOptionsKey = '';
    function syncDesktopThemeFx() {
        bindThemeFxMotionListener();
        const theme = currentDesktopTheme();
        const motion = animationsEnabled();
        const wants = !!theme.fx && (motion || theme.staticFx) && !useMobileDesktopMode();
        const key = theme.id + ':' + motion + ':' + document.body.dataset.wallpaper;
        if (!wants) {
            ++themeFxLoadToken;
            themeFxOptionsKey = '';
            if (window.AuraDesktopThemeFx) window.AuraDesktopThemeFx.stop();
            return;
        }
        if (window.AuraDesktopThemeFx?.activeId() === theme.id && themeFxOptionsKey === key) return;
        const loadBundle = window.AuraDesktopModules?.loadBundle;
        if (typeof loadBundle !== 'function') return;
        const token = ++themeFxLoadToken;
        loadBundle('desktop-theme-fx').then(() => {
            if (token !== themeFxLoadToken) return;
            const latest = currentDesktopTheme();
            const fx = window.AuraDesktopThemeFx;
            const enabled = animationsEnabled();
            if (!fx || !latest.fx || (!enabled && !latest.staticFx) || useMobileDesktopMode()) return;
            fx.stop();
            themeFxOptionsKey = latest.id + ':' + enabled + ':' + document.body.dataset.wallpaper;
            fx.start(latest.id, { motion: enabled, wallpaper: document.body.dataset.wallpaper });
        }).catch(err => console.warn('Theme effects failed to load', err));
    }

    // The wallpaper follows a theme change only while the previous theme "owns" it: every space
    // shows the previous theme's suggestion or the factory default. A wallpaper the user picked
    // is never touched. Resolves true when it switched; errors propagate to the caller.
    async function followThemeWallpaper(previousId, nextId) {
        const factory = 'groupshoot';
        const prev = desktopThemeById(previousId);
        const next = desktopThemeById(nextId);
        const previousWallpaper = prev ? prev.wallpaper : factory;
        if (!next || next.wallpaper === previousWallpaper) return false;
        let bySpace = {};
        try {
            const parsed = JSON.parse(settingValue('appearance.wallpaper_by_space') || '{}');
            if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) bySpace = parsed;
        } catch (err) {
            bySpace = {};
        }
        const fallback = settingValue('appearance.wallpaper');
        const owned = SPACE_IDS.every(id => {
            const effective = bySpace[id] || fallback;
            return effective === previousWallpaper || effective === factory;
        });
        if (!owned) return false;
        const written = { 'appearance.wallpaper': next.wallpaper, 'appearance.wallpaper_by_space': '{}' };
        const last = await api('/api/desktop/settings', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ settings: written })
        });
        if (!state.bootstrap) state.bootstrap = {};
        const remote = last && last.settings && typeof last.settings === 'object' ? last.settings : {};
        state.bootstrap.settings = Object.assign({}, desktopSettings(), remote, written);
        return true;
    }
