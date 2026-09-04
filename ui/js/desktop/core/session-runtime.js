    const SESSION_SKIP_APP_IDS = new Set(['sip-phone', 'live-speech', 'quick-connect', 'galaxa-deluxe', 'music-player']);
    const SESSION_CONTEXT_KEYS = ['path', 'category'];
    let sessionPersistTimer = 0;

    function defaultDockPinIds() {
        return ['files', 'writer', 'code-studio', 'settings', 'calendar'];
    }

    function parseDockPinsRaw() {
        const raw = settingValue('appearance.dock_pins');
        if (!raw) return defaultDockPinIds();
        try {
            const parsed = JSON.parse(raw);
            return Array.isArray(parsed) ? parsed.filter(id => typeof id === 'string' && id) : defaultDockPinIds();
        } catch (_) {
            return defaultDockPinIds();
        }
    }

    function dockPinIds() {
        const pins = parseDockPinsRaw();
        const valid = pins.filter(id => userFacingApps().some(app => app.id === id));
        return valid.length ? valid : defaultDockPinIds();
    }

    function isAppDockPinned(appId) {
        return dockPinIds().includes(appId);
    }

    async function setDockPins(appIds) {
        const json = JSON.stringify(appIds);
        await saveSetting('appearance.dock_pins', json);
        if (state.bootstrap) {
            state.bootstrap.settings = Object.assign({}, state.bootstrap.settings || {}, { 'appearance.dock_pins': json });
        }
        renderTaskbar();
    }

    async function toggleDockPin(appId) {
        const pins = dockPinIds().slice();
        const idx = pins.indexOf(appId);
        if (idx >= 0) pins.splice(idx, 1);
        else pins.push(appId);
        await setDockPins(pins);
    }

    function sessionRestoreEnabled() {
        return settingBool('windows.restore_session');
    }

    function sanitizeSessionContext(context) {
        if (!context || typeof context !== 'object') return {};
        const out = {};
        SESSION_CONTEXT_KEYS.forEach(key => {
            if (context[key] != null && context[key] !== '') out[key] = context[key];
        });
        return out;
    }

    function captureSessionSnapshot() {
        const windows = [];
        state.windows.forEach(item => {
            if (!item || !item.appId || item.isGadget || item.closing) return;
            if (String(item.appId).startsWith('widget:')) return;
            if (SESSION_SKIP_APP_IDS.has(item.appId)) return;
            const el = item.element;
            if (!el) return;
            windows.push({
                appId: item.appId,
                left: parseInt(el.style.left, 10) || 0,
                top: parseInt(el.style.top, 10) || 0,
                width: parseInt(el.style.width, 10) || 800,
                height: parseInt(el.style.height, 10) || 600,
                maximized: !!item.maximized,
                minimized: el.style.display === 'none',
                z: parseInt(el.style.zIndex, 10) || 0,
                spaceId: windowSpaceId(item),
                alwaysOnTop: !!item.alwaysOnTop,
                context: sanitizeSessionContext(item.context)
            });
        });
        return {
            version: 2,
            activeSpaceId: normalizeSpaceId(state.activeSpaceId),
            windows
        };
    }

    function scheduleSessionPersist() {
        if (!sessionRestoreEnabled() || state.sessionRestoring) return;
        if (sessionPersistTimer) window.clearTimeout(sessionPersistTimer);
        sessionPersistTimer = window.setTimeout(persistSessionSnapshot, 800);
    }

    async function persistSessionSnapshot() {
        sessionPersistTimer = 0;
        if (!sessionRestoreEnabled() || state.sessionRestoring) return;
        try {
            const snapshot = captureSessionSnapshot();
            const json = JSON.stringify(snapshot);
            await saveSetting('session.windows', json);
            if (state.bootstrap) {
                state.bootstrap.settings = Object.assign({}, state.bootstrap.settings || {}, { 'session.windows': json });
            }
        } catch (_) { /* ignore persist errors */ }
    }

    function parseSessionSnapshot() {
        const raw = settingValue('session.windows');
        if (!raw) return null;
        try {
            const parsed = JSON.parse(raw);
            if (!parsed || !Array.isArray(parsed.windows)) return null;
            return parsed;
        } catch (_) {
            return null;
        }
    }

    async function restoreDesktopSession() {
        if (!sessionRestoreEnabled() || state._sessionRestored) return;
        state._sessionRestored = true;
        const snapshot = parseSessionSnapshot();
        if (!snapshot || !snapshot.windows.length) return;
        state.sessionRestoring = true;
        restoreActiveSpaceFromSnapshot(snapshot);
        renderSpacePager();
        const sorted = snapshot.windows.slice().sort((a, b) => (a.z || 0) - (b.z || 0));
        for (let i = 0; i < sorted.length; i++) {
            const entry = sorted[i];
            if (!entry || !entry.appId || SESSION_SKIP_APP_IDS.has(entry.appId)) continue;
            if (!appById(entry.appId)) continue;
            const ctx = Object.assign({}, sanitizeSessionContext(entry.context), {
                sessionRestore: {
                    left: entry.left,
                    top: entry.top,
                    width: entry.width,
                    height: entry.height,
                    maximized: !!entry.maximized,
                    minimized: !!entry.minimized,
                    z: entry.z || 0,
                    spaceId: entry.spaceId,
                    alwaysOnTop: !!entry.alwaysOnTop,
                    active: i === sorted.length - 1
                }
            });
            openApp(entry.appId, ctx);
            await new Promise(resolve => window.setTimeout(resolve, 60));
        }
        state.sessionRestoring = false;
        applySpaceVisibility();
        const visibleOnSpace = taskbarWindows().filter(win => win.element && win.element.style.display !== 'none');
        if (visibleOnSpace.length) {
            const top = visibleOnSpace.reduce((best, win) => {
                const z = parseInt(win.element.style.zIndex, 10) || 0;
                const bestZ = parseInt(best.element.style.zIndex, 10) || 0;
                return z >= bestZ ? win : best;
            });
            focusWindow(top.id);
        } else {
            state.activeWindowId = '';
        }
        scheduleSessionPersist();
    }

    function parseDefaultAppsMap() {
        const raw = settingValue('files.default_apps');
        if (!raw) return {};
        try {
            const parsed = JSON.parse(raw);
            return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {};
        } catch (_) {
            return {};
        }
    }

    async function setDefaultAppForExtension(ext, appId) {
        const normalized = String(ext || '').toLowerCase().replace(/^\./, '');
        if (!normalized || !appId) return;
        const map = parseDefaultAppsMap();
        map[normalized] = appId;
        const json = JSON.stringify(map);
        await saveSetting('files.default_apps', json);
        if (state.bootstrap) {
            state.bootstrap.settings = Object.assign({}, state.bootstrap.settings || {}, { 'files.default_apps': json });
        }
    }

    function defaultAppForExtension(ext) {
        const normalized = String(ext || '').toLowerCase().replace(/^\./, '');
        const map = parseDefaultAppsMap();
        return map[normalized] || '';
    }

    const RECENT_FILES_KEY = 'aurago.desktop.recentFiles.v1';
    const RECENT_FILES_MAX = 12;

    function readRecentFiles() {
        return readJSONStorage(RECENT_FILES_KEY, []).filter(entry => entry && entry.path).slice(0, RECENT_FILES_MAX);
    }

    function recordRecentFile(path, appId) {
        const normalized = normalizeDesktopPath(path);
        if (!normalized) return;
        const name = pathBaseName(normalized);
        const recent = readRecentFiles().filter(entry => entry.path !== normalized);
        recent.unshift({ path: normalized, name, appId: appId || '', openedAt: Date.now() });
        writeJSONStorage(RECENT_FILES_KEY, recent.slice(0, RECENT_FILES_MAX));
    }
