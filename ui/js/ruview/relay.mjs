/* Parent-side relay for the opaque RuView frame. The frame has no network access. */
const channel = 'aurago-ruview';
const tourKey = 'aurago.ruview.onboarding.done';
export const tourFrameName = 'aurago-tour-done';
const resourceID = /^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$/;
const bodyLimit = 65536;
const responseLimit = 32 * 1024 * 1024;

export const streams = Object.freeze(['/ws/sensing', '/ws/introspection', '/ws/field', '/ws/train/progress', '/api/v1/stream/pose']);
export const reads = Object.freeze(['/health', '/health/health', '/health/live', '/health/ready', '/health/metrics', '/health/version', '/api/v1/info', '/api/v1/status', '/api/v1/metrics', '/api/v1/sensing/latest', '/api/v1/radar/latest', '/api/v1/nodes', '/api/v1/mesh', '/api/v1/mesh/metrics', '/api/v1/vital-signs', '/api/v1/edge-vitals', '/api/v1/edge/registry', '/api/v1/wasm-events', '/api/v1/model/info', '/api/v1/model/layers', '/api/v1/model/segments', '/api/v1/model/sona/profiles', '/api/v1/pose/current', '/api/v1/pose/physics/metrics', '/api/v1/pose/activities', '/api/v1/pose/calibration/status', '/api/v1/stream/status', '/api/v1/introspection/snapshot', '/api/v1/models', '/api/v1/models/active', '/api/v1/models/lora/profiles', '/api/v1/recording/list', '/api/v1/calibration/status', '/api/v1/adaptive/status', '/api/v1/config/dedup-factor', '/api/v1/train/status', '/api/field']);
export const writes = Object.freeze(['/api/v1/recording/start', '/api/v1/recording/stop', '/api/v1/pose/calibrate', '/api/v1/calibration/start', '/api/v1/calibration/stop', '/api/v1/calibration/cancel', '/api/v1/calibration/reset', '/api/v1/calibration/bootstrap/promote', '/api/v1/calibration/bootstrap/refine', '/api/v1/adaptive/train', '/api/v1/adaptive/unload', '/api/v1/config/dedup-factor', '/api/v1/config/ground-truth', '/api/v1/train/start', '/api/v1/train/stop', '/api/v1/train/pretrain', '/api/v1/train/lora']);

export function cleanPath(p) {
    if (p === '') return '.';
    const rooted = p.startsWith('/');
    const parts = [];
    for (const segment of p.split('/')) {
        if (segment === '' || segment === '.') continue;
        if (segment === '..') {
            if (parts.length && parts[parts.length - 1] !== '..') parts.pop();
            else if (!rooted) parts.push('..');
            continue;
        }
        parts.push(segment);
    }
    const joined = parts.join('/');
    if (rooted) return joined ? '/' + joined : '/';
    return joined || '.';
}

/** Returns 'stream', 'read', 'write', or false. Query strings and fragments never pass. */
export function endpoint(method, p) {
    if (typeof method !== 'string' || typeof p !== 'string') return false;
    if (p.includes('?') || p.includes('#') || /[%\\\u0000]/.test(p) || cleanPath(p) !== p) return false;
    const verb = method.toUpperCase();
    if (verb === 'GET' && streams.includes(p)) return 'stream';
    if (verb === 'GET' && reads.includes(p)) return 'read';
    if (verb === 'GET' && p.startsWith('/api/v1/nodes/')) {
        const parts = p.slice('/api/v1/nodes/'.length).split('/');
        if (parts.length === 2 && resourceID.test(parts[0]) && parts[1] === 'sync') return 'read';
    }
    if (verb === 'POST' && writes.includes(p)) return 'write';
    if (verb === 'DELETE') {
        for (const prefix of ['/api/v1/recording/', '/api/v1/models/']) {
            const id = p.startsWith(prefix) ? p.slice(prefix.length) : '';
            if (id && !id.includes('/') && resourceID.test(id)) return 'write';
        }
    }
    return false;
}

function reply(frame, id, payload, transfer) {
    const target = frame.contentWindow;
    if (!target || typeof id !== 'string') return;
    // Chrome rejects the target origin "null" even for an opaque frame.
    // The frame accepts a reply only when event.source is its parent.
    try { target.postMessage({channel, id, ...payload}, '*', transfer || []); } catch { /* a removed frame has no opaque window left */ }
}

function wasmURL(base, asset) {
    if (typeof asset !== 'string' || asset.includes('?') || asset.includes('#') || /[%\\\u0000]/.test(asset)) return '';
    const prefix = base + 'ui/';
    if (!asset.startsWith(prefix) || !asset.endsWith('.wasm')) return '';
    const rest = asset.slice(prefix.length);
    if (!rest || rest.split('/').some(part => part === '' || part === '.' || part === '..')) return '';
    return asset;
}

function applyTour(data) {
    try {
        if (data.op === 'set' && data.value === 'true') localStorage.setItem(tourKey, 'true');
        else if (data.op === 'remove' || data.op === 'clear') localStorage.removeItem(tourKey);
    } catch { /* storage can be blocked; the frame still keeps its own memory */ }
}

export function attachRuViewRelay(frame, base) {
    if (!frame || typeof base !== 'string' || !base.endsWith('/')) throw new Error('RuView relay requires a frame and base');
    const sockets = new Map();
    const onMessage = event => {
        if (event.source !== frame.contentWindow || event.origin !== 'null') return;
        const data = event.data;
        if (!data || data.channel !== channel) return;
        if (data.kind === 'storage') { applyTour(data); return; }
        if (typeof data.id !== 'string' || data.id.length < 1 || data.id.length > 80) return;
        if (data.kind === 'http') { void relayHTTP(data); return; }
        if (data.kind === 'ws-open') { openSocket(data); return; }
        if (data.kind === 'ws-send') { sendSocket(data); return; }
        if (data.kind === 'ws-close') closeSocket(data);
    };
    async function relayHTTP(data) {
        const method = String(data.method || 'GET').toUpperCase();
        const app = data.app === 'status' || data.app === 'capabilities' ? data.app : '';
        const asset = wasmURL(base, data.asset);
        const path = typeof data.path === 'string' ? data.path : '';
        let url = '';
        if (method === 'GET' && app && !path && !data.asset) url = base + app;
        else if (method === 'GET' && asset && !path && !data.app) url = asset;
        else if (!app && !data.asset && endpoint(method, path)) url = base + 'proxy' + path;
        if (!url || (data.body != null && (typeof data.body !== 'string' || data.body.length > bodyLimit))) {
            reply(frame, data.id, {kind: 'http', status: 404, blocked: true});
            return;
        }
        const headers = {};
        if (data.contentType === 'application/json') headers['Content-Type'] = 'application/json';
        if (typeof data.accept === 'string' && /^[\w/+.;=\s*,-]{1,200}$/.test(data.accept)) headers.Accept = data.accept;
        try {
            const response = await fetch(url, {method, headers, body: method === 'GET' || method === 'HEAD' ? undefined : data.body, credentials: 'same-origin', cache: 'no-store'});
            const body = await response.arrayBuffer();
            if (body.byteLength > responseLimit) { reply(frame, data.id, {kind: 'http', status: 502, blocked: true}); return; }
            reply(frame, data.id, {kind: 'http', status: response.status, contentType: response.headers.get('content-type') || '', body}, [body]);
        } catch {
            reply(frame, data.id, {kind: 'http', status: 502, blocked: true});
        }
    }
    function openSocket(data) {
        if (endpoint('GET', data.path) !== 'stream' || sockets.has(data.id)) {
            reply(frame, data.id, {kind: 'ws', event: 'error'});
            reply(frame, data.id, {kind: 'ws', event: 'close'});
            return;
        }
        const url = new URL(base + 'proxy' + data.path, location.href);
        url.protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
        url.search = '';
        url.hash = '';
        const socket = new WebSocket(url.href);
        sockets.set(data.id, socket);
        socket.addEventListener('open', () => reply(frame, data.id, {kind: 'ws', event: 'open'}));
        socket.addEventListener('message', event => {
            if (typeof event.data === 'string') reply(frame, data.id, {kind: 'ws', event: 'message', data: event.data});
        });
        socket.addEventListener('error', () => reply(frame, data.id, {kind: 'ws', event: 'error'}));
        socket.addEventListener('close', () => { sockets.delete(data.id); reply(frame, data.id, {kind: 'ws', event: 'close'}); });
    }
    function sendSocket(data) {
        const socket = sockets.get(data.id);
        if (!socket || socket.readyState !== WebSocket.OPEN || typeof data.data !== 'string') return;
        socket.send(data.data);
    }
    function closeSocket(data) {
        const socket = sockets.get(data.id);
        sockets.delete(data.id);
        socket?.close();
    }
    window.addEventListener('message', onMessage);
    return function detach() {
        window.removeEventListener('message', onMessage);
        for (const socket of sockets.values()) socket.close();
        sockets.clear();
    };
}
