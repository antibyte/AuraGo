/* Code Studio workspaces (plan C): pure helpers without DOM access - request header and link/socket query injection,
   labels, per-workspace tab sessions and the active workspace in localStorage, error classification, reconnect backoff,
   remote paths, folder listings, host search and new-host validation. Runs in the code-studio bundle
   (window.CodeStudioWorkspaceModel) and under node (workspace-model.test.cjs). */
(function (root, factory) {
    'use strict';
    const api = factory();
    if (typeof module === 'object' && module && module.exports) module.exports = api;
    if (root) root.CodeStudioWorkspaceModel = api;
})(typeof window !== 'undefined' ? window : null, () => {
    'use strict';

    const LOCAL_ID = 'local';
    const HEADER = 'X-Code-Studio-Workspace';
    const SESSIONS_KEY = 'aurago.codeStudio.workspaceSessions.v1';
    const ACTIVE_KEY = 'aurago.codeStudio.workspace.v1';
    const MAX_SESSIONS = 24;
    const MAX_SESSION_PATHS = 20;
    const BACKOFF_MS = [2000, 5000, 10000, 20000, 30000, 60000];
    const ID_PATTERN = /^[A-Za-z0-9_-]{1,64}$/;
    // Workspace management and the container status are global; everything else under /api/code-studio/ is scoped.
    const UNSCOPED_PREFIXES = ['/api/code-studio/workspaces', '/api/code-studio/remote/', '/api/code-studio/status'];
    // Plan A's codes for a broken SSH connection (502/503/504 and fetch network errors count too).
    const CONNECTION_CODES = ['ssh_unavailable', 'ssh_unreachable', 'ssh_disconnected', 'ssh_connection_lost', 'sftp_unavailable', 'unreachable', 'connection_lost'];
    const PRIVATE_KEY = /-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----/;
    const HOST_PATTERN = /^[A-Za-z0-9._:[\]-]+$/;

    function normalizeId(id) {
        const value = String(id == null ? '' : id).trim();
        return ID_PATTERN.test(value) ? value : LOCAL_ID;
    }
    function isRemote(id) { return normalizeId(id) !== LOCAL_ID; }
    function isScopedPath(path) {
        const value = String(path || '');
        return value.startsWith('/api/code-studio/') && !UNSCOPED_PREFIXES.some(prefix => value.startsWith(prefix));
    }
    // options.headers is a plain object (api() in core.js); FormData uploads come without headers.
    function withWorkspaceHeaders(options, id, path) {
        const result = Object.assign({}, options || {});
        if (!isRemote(id) || !isScopedPath(path)) return result;
        result.headers = Object.assign({}, result.headers || {}, { [HEADER]: normalizeId(id) });
        return result;
    }
    function withWorkspaceQuery(url, id) {
        const value = String(url || '');
        if (!isRemote(id)) return value;
        const hashAt = value.indexOf('#');
        const base = hashAt >= 0 ? value.slice(0, hashAt) : value;
        const hash = hashAt >= 0 ? value.slice(hashAt) : '';
        if (/[?&]ws=/.test(base)) return value;
        return base + (base.includes('?') ? '&' : '?') + 'ws=' + encodeURIComponent(normalizeId(id)) + hash;
    }
    function socketURL(loc, path, id) {
        return (loc && loc.protocol === 'https:' ? 'wss:' : 'ws:') + '//' + String(loc && loc.host || '') + withWorkspaceQuery(path, id);
    }
    function timeValue(value) {
        if (typeof value === 'number') return Number.isFinite(value) && value > 0 ? value : 0;
        const parsed = Date.parse(String(value || ''));
        return Number.isFinite(parsed) ? parsed : 0;
    }

    function localWorkspace() {
        return { id: LOCAL_ID, kind: 'local', deviceId: '', deviceName: '', host: '', port: 0, user: '', root: '/workspace', label: '', lastUsedAt: 0, connected: true, capabilities: null };
    }
    function normalizeWorkspace(raw) {
        const item = raw && raw.workspace && typeof raw.workspace === 'object' ? raw.workspace : (raw || {});
        const id = normalizeId(item.id);
        if (id === LOCAL_ID) return localWorkspace();
        return {
            id, kind: 'ssh',
            deviceId: String(item.device_id || ''),
            deviceName: String(item.device_name || item.device || ''),
            host: String(item.host || ''),
            port: Number(item.port) || 22,
            user: String(item.user || item.username || ''),
            root: normalizeRemotePath(item.root || '/'),
            label: String(item.label || '').trim(),
            lastUsedAt: timeValue(item.last_used_at),
            connected: item.connected === true,
            capabilities: item.capabilities && typeof item.capabilities === 'object' ? item.capabilities : null
        };
    }
    // current is a normalized workspace, payload a raw API answer ({workspace} or {capabilities}).
    function mergeWorkspace(current, payload) {
        const base = current && current.id ? current : localWorkspace();
        const raw = payload && payload.workspace && typeof payload.workspace === 'object' ? payload.workspace : payload;
        if (raw && raw.id != null && base.id !== LOCAL_ID && normalizeId(raw.id) === base.id) {
            const fresh = normalizeWorkspace(raw);
            return Object.assign({}, base, fresh, { capabilities: fresh.capabilities || base.capabilities, connected: true });
        }
        const caps = raw && raw.capabilities && typeof raw.capabilities === 'object' ? raw.capabilities : base.capabilities;
        return Object.assign({}, base, { capabilities: caps, connected: true });
    }
    function normalizeWorkspaceList(payload) {
        const list = Array.isArray(payload) ? payload : (payload && Array.isArray(payload.workspaces) ? payload.workspaces : []);
        const seen = new Set();
        const remote = [];
        list.forEach(raw => {
            const ws = normalizeWorkspace(raw);
            if (ws.id === LOCAL_ID || seen.has(ws.id)) return;
            seen.add(ws.id);
            remote.push(ws);
        });
        remote.sort((a, b) => (b.lastUsedAt - a.lastUsedAt) || a.id.localeCompare(b.id));
        return [localWorkspace()].concat(remote);
    }

    function homeOf(user) { return user === 'root' ? '/root' : (user ? '/home/' + user : ''); }
    function tildePath(path, home) {
        const value = String(path || '');
        if (!home || home === '/') return value;
        if (value === home) return '~';
        return value.startsWith(home + '/') ? '~' + value.slice(home.length) : value;
    }
    function displayRoot(ws) { return ws && ws.kind === 'ssh' ? tildePath(ws.root, homeOf(ws.user)) : '/workspace'; }
    function userAtHost(ws) {
        if (!ws || ws.kind === 'local') return '';
        return (ws.user ? ws.user + '@' : '') + (ws.host || '') + (ws.port && ws.port !== 22 ? ':' + ws.port : '');
    }
    function workspaceLabel(ws) {
        if (!ws || ws.kind !== 'ssh') return '';
        return ws.label || ((ws.deviceName || ws.host || ws.id) + ' · ' + displayRoot(ws));
    }

    function normalizeRemotePath(path, base) {
        let value = String(path == null ? '' : path).replace(/\\/g, '/').replace(/\0/g, '').trim();
        if (value === '~' || value.startsWith('~/')) {
            const rest = value.slice(2).split('/').filter(part => part && part !== '.').join('/');
            return rest ? '~/' + rest : '~';
        }
        if (!value.startsWith('/')) value = (base && String(base).startsWith('/') ? String(base) : '') + '/' + value;
        const parts = [];
        value.split('/').forEach(part => {
            if (!part || part === '.') return;
            if (part === '..') parts.pop();
            else parts.push(part);
        });
        return '/' + parts.join('/');
    }
    function expandHome(path, home) {
        const value = String(path || '').trim();
        if (!home || !(value === '~' || value.startsWith('~/'))) return value;
        return value === '~' ? home : home.replace(/\/+$/, '') + value.slice(1);
    }
    function remoteParent(path) {
        const value = normalizeRemotePath(path);
        if (value === '/' || value.startsWith('~')) return '/';
        const index = value.lastIndexOf('/');
        return index <= 0 ? '/' : value.slice(0, index);
    }
    function remoteBreadcrumbs(path) {
        const crumbs = [{ name: '/', path: '/' }];
        let current = '';
        normalizeRemotePath(path).split('/').filter(Boolean).forEach(part => { current += '/' + part; crumbs.push({ name: part, path: current }); });
        return crumbs;
    }
    function normalizeBrowse(payload) {
        const data = payload || {};
        const path = normalizeRemotePath(data.path || '/');
        const raw = Array.isArray(data.entries) ? data.entries : (Array.isArray(data.files) ? data.files : []);
        const entries = [];
        raw.forEach(item => {
            const name = String(item && item.name || '');
            if (!name || name === '.' || name === '..' || name.includes('/')) return;
            const dir = item.type === 'directory' || item.type === 'dir' || item.is_dir === true;
            entries.push({ name, path: item.path ? normalizeRemotePath(item.path) : normalizeRemotePath(name, path), dir, git: dir && item.is_git === true, hidden: name.startsWith('.') });
        });
        const rank = entry => (entry.dir ? (entry.hidden ? 1 : 0) : 2);
        entries.sort((a, b) => (rank(a) - rank(b)) || a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' }));
        return { path, entries };
    }

    function readJSON(storage, key) {
        try {
            const raw = storage && storage.getItem(key);
            return raw ? JSON.parse(raw) : null;
        } catch (_) {
            return null;
        }
    }
    function sessionPaths(list) {
        const seen = new Set();
        const result = [];
        (Array.isArray(list) ? list : []).forEach(item => {
            const path = typeof item === 'string' ? item : (item && typeof item.path === 'string' ? item.path : '');
            if (!/^\/workspace\/./.test(path) || path.includes('\0') || path.split('/').some(part => part === '..' || part === '.') || seen.has(path)) return;
            seen.add(path);
            if (result.length < MAX_SESSION_PATHS) result.push(path);
        });
        return result;
    }
    function sanitizeSession(raw) {
        if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return null;
        const openTabs = sessionPaths(raw.openTabs);
        const index = Number(raw.activeTabIndex);
        return {
            openTabs,
            activeTabIndex: openTabs.length ? Math.min(Math.max(Number.isInteger(index) ? index : 0, 0), openTabs.length - 1) : -1,
            recentFiles: sessionPaths(raw.recentFiles),
            split: raw.split && typeof raw.split === 'object' && !Array.isArray(raw.split) ? raw.split : null,
            updatedAt: timeValue(raw.updatedAt)
        };
    }
    function readSessions(storage) {
        const data = readJSON(storage, SESSIONS_KEY);
        const sessions = {};
        if (!data || typeof data !== 'object' || Array.isArray(data)) return sessions;
        Object.keys(data).forEach(id => {
            if (normalizeId(id) !== id) return;
            const session = sanitizeSession(data[id]);
            if (session) sessions[id] = session;
        });
        return sessions;
    }
    function writeSessions(storage, sessions) {
        const keep = {};
        if (sessions[LOCAL_ID]) keep[LOCAL_ID] = sessions[LOCAL_ID];
        Object.keys(sessions).filter(id => id !== LOCAL_ID)
            .sort((a, b) => sessions[b].updatedAt - sessions[a].updatedAt)
            .slice(0, MAX_SESSIONS - (sessions[LOCAL_ID] ? 1 : 0))
            .forEach(id => { keep[id] = sessions[id]; });
        try {
            storage.setItem(SESSIONS_KEY, JSON.stringify(keep));
            return true;
        } catch (_) {
            return false;
        }
    }
    function writeSession(storage, id, session, now) {
        const sessions = readSessions(storage);
        const clean = sanitizeSession(Object.assign({}, session || {}, { updatedAt: Number(now) || Date.now() }));
        if (!clean) return false;
        sessions[normalizeId(id)] = clean;
        return writeSessions(storage, sessions);
    }
    function forgetSession(storage, id) {
        const sessions = readSessions(storage);
        if (!sessions[id]) return false;
        delete sessions[id];
        return writeSessions(storage, sessions);
    }
    function readActive(storage) {
        const data = readJSON(storage, ACTIVE_KEY);
        return normalizeId(data && typeof data === 'object' ? data.id : data);
    }
    function writeActive(storage, id) {
        try {
            storage.setItem(ACTIVE_KEY, JSON.stringify({ id: normalizeId(id) }));
            return true;
        } catch (_) {
            return false;
        }
    }

    function classifyError(err) {
        const status = Number(err && err.status) || 0;
        const code = String((err && (err.code || (err.body && err.body.code))) || '');
        if (code === 'unknown_host_key' || code === 'host_key_mismatch' || code === 'workspace_not_found') return code;
        if (code === 'auth_failed' || code === 'ssh_auth_failed') return 'auth';
        if (code === 'not_directory' || code === 'not_a_directory') return 'not_directory';
        if (code === 'device_not_found' || code === 'remote_scope_required' || code === 'passphrase_required' || code === 'invalid_private_key') return code;
        if (CONNECTION_CODES.includes(code) || status === 502 || status === 503 || status === 504) return 'unreachable';
        if (!status && err && err.name === 'TypeError') return 'unreachable';
        if (code === 'not_found' || status === 404) return 'not_found';
        return 'other';
    }
    function backoffDelay(attempt) {
        const index = Math.max(0, Math.floor(Number(attempt) || 0));
        return BACKOFF_MS[Math.min(index, BACKOFF_MS.length - 1)];
    }

    // Error values are keys of the form messages: required, host, port, key.
    function validateHostForm(values) {
        const v = values || {};
        const errors = {};
        const name = String(v.name || '').trim().slice(0, 80);
        const host = String(v.host || '').trim();
        const username = String(v.username || '').trim();
        const portText = String(v.port == null || v.port === '' ? '22' : v.port).trim();
        const auth = v.auth === 'key' ? 'key' : 'password';
        const secret = String(v.secret || '');
        if (!name) errors.name = 'required';
        if (!host) errors.host = 'required';
        else if (!HOST_PATTERN.test(host)) errors.host = 'host';
        if (!/^\d{1,5}$/.test(portText) || Number(portText) < 1 || Number(portText) > 65535) errors.port = 'port';
        if (!username) errors.username = 'required';
        if (!secret) errors.secret = 'required';
        else if (auth === 'key' && (!PRIVATE_KEY.test(secret) || secret.length > 65536)) errors.secret = 'key';
        const valid = !Object.keys(errors).length;
        return {
            valid,
            errors,
            value: valid ? { name, host: host.replace(/^\[|\]$/g, ''), port: Number(portText), username, auth, secret: auth === 'key' ? secret.trim() + '\n' : secret, passphrase: auth === 'key' ? String(v.passphrase || '') : '' } : null
        };
    }

    function normalizeDevice(raw) {
        const d = raw && raw.device && typeof raw.device === 'object' ? raw.device : (raw || {});
        const host = String(d.host || d.ip_address || '');
        return { id: String(d.id == null ? '' : d.id), name: String(d.name || host), host, port: Number(d.port) || 22, user: String(d.username || d.user || ''), lastUsedAt: timeValue(d.last_used_at) };
    }
    function normalizeDeviceList(payload) {
        const list = Array.isArray(payload) ? payload : (payload && Array.isArray(payload.devices) ? payload.devices : []);
        return list.map(normalizeDevice).filter(device => device.id);
    }
    function deviceAddress(device) {
        return device ? (device.user ? device.user + '@' : '') + device.host + ':' + (device.port || 22) : '';
    }
    function deviceLastUsed(device, workspaces) {
        let latest = device && device.lastUsedAt || 0;
        (workspaces || []).forEach(ws => { if (device && ws.deviceId === device.id && ws.lastUsedAt > latest) latest = ws.lastUsedAt; });
        return latest;
    }
    function sortDevices(devices, workspaces) {
        return (devices || []).slice().sort((a, b) => (deviceLastUsed(b, workspaces) - deviceLastUsed(a, workspaces)) || a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }));
    }
    function filterDevices(devices, query) {
        const q = String(query || '').trim().toLowerCase();
        if (!q) return (devices || []).slice();
        return (devices || []).filter(device => [device.name, device.host, device.user, deviceAddress(device)].some(value => String(value || '').toLowerCase().includes(q)));
    }
    function recentRoots(workspaces, deviceId) {
        const roots = [];
        (workspaces || []).filter(ws => ws.kind === 'ssh' && ws.deviceId === deviceId)
            .sort((a, b) => b.lastUsedAt - a.lastUsedAt)
            .forEach(ws => { if (!roots.includes(ws.root)) roots.push(ws.root); });
        return roots.slice(0, 5);
    }
    // {value, unit} for Intl.RelativeTimeFormat; null when never used.
    function relativeTime(ts, now) {
        const time = timeValue(ts);
        if (!time) return null;
        const seconds = (time - (Number(now) || Date.now())) / 1000;
        for (const [unit, size] of [['year', 31536000], ['month', 2592000], ['week', 604800], ['day', 86400], ['hour', 3600], ['minute', 60]]) {
            if (Math.abs(seconds) >= size) return { value: Math.round(seconds / size), unit };
        }
        return { value: 0, unit: 'minute' };
    }

    return {
        LOCAL_ID, HEADER, SESSIONS_KEY, ACTIVE_KEY, MAX_SESSIONS,
        normalizeId, isRemote, isScopedPath, withWorkspaceHeaders, withWorkspaceQuery, socketURL,
        localWorkspace, normalizeWorkspace, mergeWorkspace, normalizeWorkspaceList,
        displayRoot, tildePath, userAtHost, workspaceLabel,
        normalizeRemotePath, expandHome, remoteParent, remoteBreadcrumbs, normalizeBrowse,
        readSessions, writeSession, forgetSession, readActive, writeActive,
        classifyError, backoffDelay, validateHostForm,
        normalizeDevice, normalizeDeviceList, deviceAddress, deviceLastUsed, sortDevices, filterDevices, recentRoots, relativeTime
    };
});
