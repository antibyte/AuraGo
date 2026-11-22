(function () {
    'use strict';

    const HISTORY_KEY = 'aurago.desktop.notifications.v1';
    const POLICY_KEY = 'aurago.desktop.notification-policy.v1';
    const VERSION = 1;
    const MAX_HISTORY = 40;
    const MAX_TITLE = 160;
    const MAX_MESSAGE = 280;
    const MAX_STORAGE_CHARS = 100000;
    const APP_ID = /^[a-z0-9][a-z0-9._-]{0,63}$/;
    const SESSION_SCOPE = /^[a-f0-9]{64}$/;
    const listeners = new Set();
    const policyListeners = new Set();
    let items = [];
    let currentPolicy = { dnd: false, apps: Object.create(null) };
    let ready = false;
    let initPromise = null;
    let authMode = 'pending';
    let sessionScope = '';
    let historyStorage = 'checking';
    let policyStorage = 'checking';
    let authEnded = false;
    let authRefreshing = false;
    let authRefreshPromise = null;
    let authStatusCheckedAt = 0;
    let recordsDuringRefresh = [];
    const AUTH_RECHECK_DEBOUNCE_MS = 2000;
    const AUTH_STATUS_TIMEOUT_MS = 5000;

    function cloneEntry(entry) {
        return {
            id: entry.id,
            title: entry.title,
            message: entry.message,
            appId: entry.appId,
            context: entry.context ? Object.assign({}, entry.context) : undefined,
            ts: entry.ts,
            read: entry.read
        };
    }

    function history() {
        return ready && !authRefreshing ? items.map(cloneEntry) : [];
    }

    function status() {
        return {
            ready,
            authMode,
            storage: historyStorage,
            policyStorage,
            tabOnly: historyStorage === 'memory',
            storageFallback: historyStorage === 'memory' || policyStorage === 'memory'
        };
    }

    function emit() {
        const snapshot = history();
        const currentStatus = status();
        listeners.forEach(listener => { try { listener(snapshot, currentStatus); } catch (_) {} });
    }

    function emitPolicy() {
        const snapshot = policy();
        const currentStatus = status();
        policyListeners.forEach(listener => { try { listener(snapshot, currentStatus); } catch (_) {} });
    }

    function storageArea(name) {
        try { return window[name] || null; } catch (_) { return null; }
    }

    function cleanText(value, limit) {
        if (typeof value !== 'string') return '';
        const clean = value.replace(/[\u0000-\u001f\u007f]/g, ' ').replace(/\s+/g, ' ').trim();
        return Array.from(clean).slice(0, limit).join('');
    }

    function cleanAppId(value) {
        return typeof value === 'string' && APP_ID.test(value) ? value : '';
    }

    function cleanContext(appId, context) {
        if (!context || typeof context !== 'object' || Array.isArray(context)) return undefined;
        if (appId === 'meshcore' && typeof context.conversation_id === 'string' && /^[a-f0-9]{64}$/.test(context.conversation_id)) {
            return { conversation_id: context.conversation_id };
        }
        if (appId === 'storymaker' && typeof (context.projectId || context.project_id) === 'string' && /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(context.projectId || context.project_id)) {
            return { projectId: context.projectId || context.project_id };
        }
        if (appId === 'easydrag' && typeof context.flow_id === 'string' && /^flow_[a-z0-9]{10}$/.test(context.flow_id)) {
            return /^run_[a-z0-9]{12}$/.test(context.run_id || '')
                ? { flow_id: context.flow_id, run_id: context.run_id }
                : { flow_id: context.flow_id };
        }
        if (appId === 'network-cameras' && Number.isSafeInteger(context.event_id) && context.event_id > 0) {
            return typeof context.camera_id === 'string' && /^[a-z0-9_-]{1,64}$/.test(context.camera_id)
                ? { event_id: context.event_id, camera_id: context.camera_id }
                : { event_id: context.event_id };
        }
        if (appId === 'detective' && typeof context.case_id === 'string' && /^case_[a-z0-9]{1,64}$/.test(context.case_id)) {
            return { case_id: context.case_id };
        }
        return undefined;
    }

    function makeId() {
        return 'n-' + Date.now() + '-' + Math.random().toString(36).slice(2, 9);
    }

    function normalizeEntry(raw) {
        if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return null;
        if (typeof raw.id !== 'string' || !/^n-\d{1,16}-[a-z0-9]{1,12}$/.test(raw.id)) return null;
        if (!Number.isSafeInteger(raw.ts) || raw.ts <= 0 || raw.ts > Date.now() + 86400000) return null;
        const appId = cleanAppId(raw.appId);
        const title = cleanText(raw.title, MAX_TITLE);
        if (!title) return null;
        return {
            id: raw.id,
            title,
            message: cleanText(raw.message, MAX_MESSAGE),
            appId,
            context: cleanContext(appId, raw.context),
            ts: raw.ts,
            read: raw.read === true,
            persist: raw.persist !== false
        };
    }

    function serializeEntry(entry) {
        return {
            id: entry.id,
            title: entry.title,
            message: entry.message,
            appId: entry.appId,
            context: entry.context,
            ts: entry.ts,
            read: entry.read
        };
    }

    function normalizedPolicy(raw) {
        const result = { dnd: !!(raw && raw.dnd === true), apps: Object.create(null) };
        const apps = raw && raw.apps;
        if (!apps || typeof apps !== 'object' || Array.isArray(apps)) return result;
        for (const id of Object.keys(apps).slice(0, 128)) {
            if (!APP_ID.test(id)) continue;
            const app = apps[id];
            if (!app || typeof app !== 'object' || Array.isArray(app)) continue;
            result.apps[id] = {
                toast: app.toast !== false,
                sound: app.sound !== false
            };
        }
        return result;
    }

    function policy() {
        return { dnd: currentPolicy.dnd, apps: Object.fromEntries(Object.entries(currentPolicy.apps).map(([id, value]) => [id, { toast: value.toast, sound: value.sound }])) };
    }

    function readPolicy() {
        const storage = storageArea('localStorage');
        if (!storage || typeof storage.getItem !== 'function') {
            policyStorage = 'memory';
            return;
        }
        try {
            const value = storage.getItem(POLICY_KEY);
            policyStorage = 'localStorage';
            if (value === null) return;
            if (value.length > 20000) throw new Error('notification policy is too large');
            const parsed = JSON.parse(value);
            if (!parsed || parsed.version !== VERSION) throw new Error('unsupported notification policy');
            currentPolicy = normalizedPolicy(parsed);
        } catch (_) {
            currentPolicy = { dnd: false, apps: Object.create(null) };
            try { storage.removeItem(POLICY_KEY); } catch (_) { policyStorage = 'memory'; }
        }
    }

    function writePolicy() {
        const storage = storageArea('localStorage');
        if (!storage || typeof storage.setItem !== 'function') {
            policyStorage = 'memory';
            return;
        }
        try {
            storage.setItem(POLICY_KEY, JSON.stringify({ version: VERSION, dnd: currentPolicy.dnd, apps: currentPolicy.apps }));
            policyStorage = 'localStorage';
        } catch (_) {
            policyStorage = 'memory';
        }
    }

    function setPolicy(next) {
        if (!next || typeof next !== 'object' || Array.isArray(next)) return policy();
        const merged = policy();
        if (typeof next.dnd === 'boolean') merged.dnd = next.dnd;
        if (next.apps && typeof next.apps === 'object' && !Array.isArray(next.apps)) {
            for (const id of Object.keys(next.apps).slice(0, 128)) {
                if (!APP_ID.test(id)) continue;
                const app = next.apps[id];
                if (app === null) { delete merged.apps[id]; continue; }
                if (!app || typeof app !== 'object' || Array.isArray(app)) continue;
                const previous = merged.apps[id] || { toast: true, sound: true };
                merged.apps[id] = {
                    toast: typeof app.toast === 'boolean' ? app.toast : previous.toast,
                    sound: typeof app.sound === 'boolean' ? app.sound : previous.sound
                };
            }
        }
        currentPolicy = normalizedPolicy(merged);
        writePolicy();
        emitPolicy();
        return policy();
    }

    function delivery(payload) {
        const app = currentPolicy.apps[cleanAppId(payload && payload.appId) || 'desktop'] || { toast: true, sound: true };
        return {
            toast: !currentPolicy.dnd && app.toast,
            sound: !currentPolicy.dnd && app.sound
        };
    }

    function persistHistory() {
        if (!ready || authRefreshing || historyStorage !== 'session' || (authMode !== 'disabled' && authMode !== 'authenticated')) return;
        const storage = storageArea('sessionStorage');
        if (!storage || typeof storage.setItem !== 'function') {
            historyStorage = 'memory';
            emit();
            return;
        }
        const records = items.filter(item => item.persist).slice(0, MAX_HISTORY).map(serializeEntry);
        if (!records.length) {
            removeStoredHistory();
            return;
        }
        try {
            storage.setItem(HISTORY_KEY, JSON.stringify({ version: VERSION, scope: sessionScope, records }));
        } catch (_) {
            historyStorage = 'memory';
            emit();
        }
    }

    function removeStoredHistory() {
        const storage = storageArea('sessionStorage');
        if (storage && typeof storage.removeItem === 'function') {
            try { storage.removeItem(HISTORY_KEY); } catch (_) { historyStorage = 'memory'; }
        }
    }

    function readStoredHistory(scope) {
        const storage = storageArea('sessionStorage');
        if (!storage || typeof storage.getItem !== 'function') {
            historyStorage = 'memory';
            return [];
        }
        try {
            const raw = storage.getItem(HISTORY_KEY);
            historyStorage = 'session';
            if (raw === null) return [];
            if (raw.length > MAX_STORAGE_CHARS) throw new Error('notification history is too large');
            const parsed = JSON.parse(raw);
            if (!parsed || parsed.version !== VERSION || parsed.scope !== scope || !Array.isArray(parsed.records)) {
                removeStoredHistory();
                return [];
            }
            return parsed.records.slice(0, MAX_HISTORY).map(normalizeEntry).filter(Boolean);
        } catch (_) {
            removeStoredHistory();
            return [];
        }
    }

    function uniqueRecords(records) {
        const seen = new Set();
        return records.filter(item => {
            if (!item || seen.has(item.id)) return false;
            seen.add(item.id);
            return true;
        }).slice(0, MAX_HISTORY);
    }

    function record(payload, safeContext) {
        payload = payload && typeof payload === 'object' && !Array.isArray(payload) ? payload : {};
        if (authEnded || authMode === 'expired') return null;
        let fallbackTitle = 'Notification';
        try { if (typeof t === 'function') fallbackTitle = t('desktop.notification') || fallbackTitle; } catch (_) {}
        const appId = cleanAppId(payload.appId);
        const entry = {
            id: makeId(),
            title: cleanText(payload.title, MAX_TITLE) || cleanText(fallbackTitle, MAX_TITLE),
            message: cleanText(payload.message, MAX_MESSAGE),
            appId,
            context: cleanContext(appId, safeContext || payload.context),
            ts: Date.now(),
            read: false,
            persist: payload.persist !== false
        };
        if (authRefreshing) recordsDuringRefresh = uniqueRecords([entry, ...recordsDuringRefresh]);
        else items = uniqueRecords([entry, ...items]);
        if (ready) {
            if (!authRefreshing) persistHistory();
            emit();
        }
        return cloneEntry(entry);
    }

    function markRead(id) {
        const entry = items.find(item => item.id === id);
        if (!entry || entry.read) return false;
        entry.read = true;
        persistHistory();
        emit();
        return true;
    }

    function markAllRead() {
        let changed = false;
        items.forEach(item => { if (!item.read) { item.read = true; changed = true; } });
        if (changed) {
            persistHistory();
            emit();
        }
        return changed;
    }

    function remove(id) {
        const next = items.filter(item => item.id !== id);
        if (next.length === items.length) return false;
        items = next;
        persistHistory();
        emit();
        return true;
    }

    function clear() {
        items = [];
        if (authRefreshing) recordsDuringRefresh = [];
        removeStoredHistory();
        emit();
    }

    function onAuthEnded() {
        authEnded = true;
        authMode = 'expired';
        sessionScope = '';
        authRefreshing = false;
        recordsDuringRefresh = [];
        items = [];
        removeStoredHistory();
        ready = true;
        emit();
    }

    function requestAuthStatus() {
        const controller = typeof AbortController === 'function' ? new AbortController() : null;
        let timeout = 0;
        const request = window.fetch('/api/auth/status', {
            credentials: 'same-origin',
            cache: 'no-store',
            ...(controller ? { signal: controller.signal } : {})
        }).then(async response => ({
            response,
            auth: response && response.ok ? await response.json() : null
        }));
        const deadline = new Promise((_, reject) => {
            timeout = window.setTimeout(() => {
                controller?.abort();
                reject(new Error('auth status timed out'));
            }, AUTH_STATUS_TIMEOUT_MS);
        });
        return Promise.race([request, deadline]).finally(() => {
            if (timeout) window.clearTimeout(timeout);
        });
    }

    function subscribe(listener) {
        if (typeof listener !== 'function') return function () {};
        listeners.add(listener);
        try { listener(history(), status()); } catch (_) {}
        return () => listeners.delete(listener);
    }

    function subscribePolicy(listener) {
        if (typeof listener !== 'function') return function () {};
        policyListeners.add(listener);
        try { listener(policy(), status()); } catch (_) {}
        return () => policyListeners.delete(listener);
    }

    async function init() {
        if (initPromise) return initPromise;
        initPromise = (async function () {
            const fresh = items.slice();
            authRefreshing = true;
            recordsDuringRefresh = [];
            readPolicy();
            emitPolicy();
            authStatusCheckedAt = Date.now();
            try {
                const { response, auth } = await requestAuthStatus();
                if (!response || !response.ok) throw new Error('auth status unavailable');
                if (!auth || typeof auth.enabled !== 'boolean' || typeof auth.authenticated !== 'boolean') throw new Error('invalid auth status');
                if (authEnded) {
                    authMode = 'expired';
                    sessionScope = '';
                    items = [];
                    removeStoredHistory();
                } else if (!auth.enabled) {
                    authMode = 'disabled';
                    sessionScope = 'local';
                    items = uniqueRecords([...recordsDuringRefresh, ...fresh, ...readStoredHistory(sessionScope)]);
                } else if (auth.authenticated && typeof auth.session_scope === 'string' && SESSION_SCOPE.test(auth.session_scope)) {
                    authMode = 'authenticated';
                    sessionScope = auth.session_scope;
                    items = uniqueRecords([...recordsDuringRefresh, ...fresh, ...readStoredHistory(sessionScope)]);
                } else {
                    authMode = 'expired';
                    sessionScope = '';
                    items = [];
                    removeStoredHistory();
                }
            } catch (_) {
                if (authEnded) {
                    authMode = 'expired';
                    sessionScope = '';
                    items = [];
                    removeStoredHistory();
                } else {
                    authMode = 'unknown';
                    sessionScope = '';
                    historyStorage = 'memory';
                    items = uniqueRecords([...recordsDuringRefresh, ...fresh]);
                }
            }
            authRefreshing = false;
            recordsDuringRefresh = [];
            ready = true;
            persistHistory();
            emit();
            emitPolicy();
            return history();
        })();
        return initPromise;
    }

    function refreshAuthScope(force) {
        if (!ready) return init();
        if (authEnded) return Promise.resolve([]);
        if (authRefreshPromise) return authRefreshPromise;
        const now = Date.now();
        if (!force && now - authStatusCheckedAt < AUTH_RECHECK_DEBOUNCE_MS) return Promise.resolve(history());

        const previousItems = items.slice();
        const previousMode = authMode;
        const previousScope = sessionScope;
        recordsDuringRefresh = [];
        authRefreshing = true;
        authMode = 'pending';
        items = [];
        authStatusCheckedAt = now;
        emit();

        authRefreshPromise = (async function () {
            try {
                const { response, auth } = await requestAuthStatus();
                if (!response || !response.ok) throw new Error('auth status unavailable');
                if (!auth || typeof auth.enabled !== 'boolean' || typeof auth.authenticated !== 'boolean') throw new Error('invalid auth status');
                if (authEnded) {
                    authMode = 'expired';
                    sessionScope = '';
                    items = [];
                    removeStoredHistory();
                } else if (!auth.enabled) {
                    authMode = 'disabled';
                    sessionScope = 'local';
                    const previous = previousMode === authMode && previousScope === sessionScope ? previousItems : [];
                    items = uniqueRecords([...recordsDuringRefresh, ...previous, ...readStoredHistory(sessionScope)]);
                } else if (auth.authenticated && typeof auth.session_scope === 'string' && SESSION_SCOPE.test(auth.session_scope)) {
                    authMode = 'authenticated';
                    sessionScope = auth.session_scope;
                    const previous = previousMode === authMode && previousScope === sessionScope ? previousItems : [];
                    items = uniqueRecords([...recordsDuringRefresh, ...previous, ...readStoredHistory(sessionScope)]);
                } else {
                    authMode = 'expired';
                    sessionScope = '';
                    items = [];
                    removeStoredHistory();
                }
            } catch (_) {
                if (authEnded) {
                    authMode = 'expired';
                    sessionScope = '';
                    items = [];
                    removeStoredHistory();
                } else {
                    authMode = 'unknown';
                    sessionScope = '';
                    historyStorage = 'memory';
                    items = uniqueRecords(recordsDuringRefresh);
                }
            }
            authRefreshing = false;
            recordsDuringRefresh = [];
            ready = true;
            persistHistory();
            emit();
            emitPolicy();
            return history();
        })().finally(() => { authRefreshPromise = null; });
        return authRefreshPromise;
    }

    function onStorage(event) {
        if (!event || event.key !== POLICY_KEY) return;
        if (event.newValue === null) {
            currentPolicy = { dnd: false, apps: Object.create(null) };
            policyStorage = 'localStorage';
            emitPolicy();
            return;
        }
        try {
            if (event.newValue.length > 20000) return;
            const parsed = JSON.parse(event.newValue);
            if (!parsed || parsed.version !== VERSION) return;
            currentPolicy = normalizedPolicy(parsed);
            policyStorage = 'localStorage';
            emitPolicy();
        } catch (_) {}
    }

    window.addEventListener('storage', onStorage);
    window.addEventListener('pageshow', event => {
        if (event && event.persisted) return refreshAuthScope(true);
    });
    document.addEventListener('visibilitychange', () => {
        if (document.visibilityState === 'visible') return refreshAuthScope(false);
    });
    document.addEventListener('aurago:auth-ended', onAuthEnded);
    window.AuraDesktopNotificationCenter = {
        init,
        history,
        subscribe,
        record,
        markRead,
        markAllRead,
        remove,
        clear,
        delivery,
        policy,
        setPolicy,
        subscribePolicy,
        status
    };
}());
