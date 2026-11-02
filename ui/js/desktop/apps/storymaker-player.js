// Storymaker app player bridge: the sandboxed iframe and the parent side of the typed protocol of contract section 8.
// The iframe is sandbox="allow-scripts" only (opaque origin, no allow-same-origin, no allow permissions) and loads
// /api/desktop/storymaker/player. The parent accepts a message only when event.source is the iframe's own window and the
// message fits a fixed schema with size caps (everything else is dropped and counted); it posts with targetOrigin '*'
// because the iframe origin is opaque, sends `load` after every `ready` (also after an iframe reload: each reload resets all
// state), never sends tokens, paths or API data, answers `need_assets` only with what the asset resolver serves, and
// reports unknown keys as missing. Classic script, publishes S.createPlayer and S.player.checkIncoming.
(function (root) {
    'use strict';

    const S = root.StorymakerParts = root.StorymakerParts || {};

    const LIMITS = { scenes: 400, issues: 2000, keys: 2000, keyLength: 240, text: 2000, id: 64, frame: 100000000, thumbWaitMs: 12000 };
    const TYPES = ['ready', 'need_assets', 'compiled', 'time', 'thumb', 'error'];

    function isObject(v) { return v !== null && typeof v === 'object' && !Array.isArray(v); }
    function isInt(v, lo, hi) { return typeof v === 'number' && isFinite(v) && Math.floor(v) === v && v >= lo && v <= hi; }
    function text(v, max) { return typeof v === 'string' ? v.slice(0, max) : ''; }
    function okReq(v) { return (typeof v === 'string' || typeof v === 'number') && String(v).length <= 64; }

    function cleanIssue(raw) {
        if (!isObject(raw)) return null;
        return {
            severity: ['error', 'warning', 'minor'].indexOf(raw.severity) >= 0 ? raw.severity : 'warning',
            code: text(raw.code, 40),
            path: text(raw.path, 400),
            message: text(raw.message, LIMITS.text),
            scene_id: text(raw.scene_id, LIMITS.id),
            fix: text(raw.fix, LIMITS.text)
        };
    }

    // checkIncoming(msg) -> a cleaned copy of a valid message, or null. The copy holds only the fields the schema names.
    function checkIncoming(msg) {
        if (!isObject(msg) || msg.sm !== 1 || TYPES.indexOf(msg.type) < 0) return null;
        switch (msg.type) {
            case 'ready':
                return { type: 'ready', caps: { workers: !!(isObject(msg.caps) && msg.caps.workers), offscreen: !!(isObject(msg.caps) && msg.caps.offscreen) } };
            case 'need_assets': {
                if (!okReq(msg.req) || !Array.isArray(msg.keys) || msg.keys.length > LIMITS.keys) return null;
                const keys = [];
                for (let i = 0; i < msg.keys.length; i++) {
                    if (typeof msg.keys[i] !== 'string' || msg.keys[i].length > LIMITS.keyLength) return null;
                    keys.push(msg.keys[i]);
                }
                return { type: 'need_assets', req: msg.req, keys };
            }
            case 'compiled': {
                const r = msg.report;
                if (!isObject(r) || !isInt(r.frame_count, 0, LIMITS.frame) || !isInt(r.fps, 1, 240) || !Array.isArray(r.scenes) || r.scenes.length > LIMITS.scenes) return null;
                const scenes = [];
                for (let i = 0; i < r.scenes.length; i++) {
                    const s = r.scenes[i];
                    if (!isObject(s) || typeof s.id !== 'string' || s.id.length > LIMITS.id || !isInt(s.start_frame, 0, LIMITS.frame) || !isInt(s.end_frame, 0, LIMITS.frame)) return null;
                    scenes.push({ id: s.id, start_frame: s.start_frame, end_frame: s.end_frame, kind: text(s.kind, 16) });
                }
                const issues = [], unverified = [];
                (Array.isArray(r.issues) ? r.issues.slice(0, LIMITS.issues) : []).forEach(raw => { const issue = cleanIssue(raw); if (issue) issues.push(issue); });
                (Array.isArray(r.unverified) ? r.unverified.slice(0, LIMITS.issues) : []).forEach(raw => { const issue = cleanIssue(raw); if (issue) unverified.push(issue); });
                return {
                    type: 'compiled',
                    report: {
                        frame_count: r.frame_count, fps: r.fps, scenes, issues, unverified,
                        width: isInt(r.width, 1, 16384) ? r.width : 0,
                        height: isInt(r.height, 1, 16384) ? r.height : 0,
                        duration_ms: isInt(r.duration_ms, 0, 1e12) ? r.duration_ms : Math.round(r.frame_count * 1000 / r.fps),
                        catalog_version: text(r.catalog_version, 64)
                    }
                };
            }
            case 'time':
                if (!isInt(msg.frame, 0, LIMITS.frame) || typeof msg.playing !== 'boolean') return null;
                return { type: 'time', frame: msg.frame, playing: msg.playing };
            case 'thumb': {
                if (!okReq(msg.req) || typeof msg.scene_id !== 'string' || msg.scene_id.length > LIMITS.id) return null;
                const isBitmap = typeof root.ImageBitmap === 'function' && msg.bitmap instanceof root.ImageBitmap;
                if (!isBitmap) return null;
                return { type: 'thumb', req: msg.req, scene_id: msg.scene_id, bitmap: msg.bitmap };
            }
            case 'error': {
                if (typeof msg.code !== 'string' || msg.code.length > 40) return null;
                const out = { type: 'error', code: msg.code, message: text(msg.message, LIMITS.text) };
                if (typeof msg.scene_id === 'string' && msg.scene_id.length <= LIMITS.id) out.scene_id = msg.scene_id;
                if (isInt(msg.line, 0, 1e7)) out.line = msg.line;
                if (isInt(msg.column, 0, 1e7)) out.column = msg.column;
                return out;
            }
            default:
                return null;
        }
    }

    // createPlayer({container, src, resolve, title}) -> player. resolve(keys) -> Promise<{assets, missing}>.
    // Events: ready(caps), loading, compiled(report), time({frame, playing}), error(error), dropped(count), unavailable.
    function createPlayer(options) {
        const o = options || {};
        const handlers = {};
        const emit = (name, payload) => (handlers[name] || []).slice().forEach(fn => { try { fn(payload); } catch (err) { console.error('storymaker player handler failed', name, err); } });
        const iframe = S.el('iframe', {
            class: 'stm-player',
            title: o.title || 'Storymaker preview',
            sandbox: 'allow-scripts',
            referrerpolicy: 'no-referrer',
            tabindex: '-1'
        });
        iframe.setAttribute('sandbox', 'allow-scripts');
        let ready = false, loaded = false, pending = null, disposed = false, dropped = 0, thumbSeq = 0, resume = 0, readyCount = 0, loadTimer = 0;
        const thumbs = new Map();

        function post(message, transfer) {
            const target = iframe.contentWindow;
            if (!target || disposed) return false;
            target.postMessage(Object.assign({ sm: 1 }, message), '*', transfer || []);
            return true;
        }

        function failThumbs(reason) {
            thumbs.forEach(entry => { clearTimeout(entry.timer); entry.reject(reason); });
            thumbs.clear();
        }

        function sendLoad() {
            if (!ready || !pending || disposed) return;
            const payload = pending;
            pending = null;
            loaded = post(Object.assign({ type: 'load' }, payload));
            if (loaded) emit('loading', payload);
        }

        function onMessage(event) {
            if (disposed || !iframe.contentWindow || event.source !== iframe.contentWindow) return;
            const msg = checkIncoming(event.data);
            if (!msg) { dropped++; emit('dropped', dropped); return; }
            switch (msg.type) {
                case 'ready':
                    // Every document announces itself once, so a ready after a reload starts from nothing: the host of the
                    // new document knows no story, and the thumbnails asked of the old one will never come.
                    ready = true;
                    loaded = false;
                    readyCount++;
                    failThumbs(new Error('reload'));
                    emit('ready', msg.caps);
                    sendLoad();
                    break;
                case 'need_assets': {
                    const answer = (assets, missing) => post({ type: 'assets', req: msg.req, assets, missing });
                    Promise.resolve(o.resolve ? o.resolve(msg.keys) : { assets: {}, missing: msg.keys }).then(
                        result => answer(result.assets || {}, result.missing || []),
                        () => answer({}, msg.keys));
                    break;
                }
                case 'compiled':
                    emit('compiled', msg.report);
                    break;
                case 'time':
                    if (resume > 0 && !msg.playing) {
                        const frame = resume;
                        resume = 0;
                        if (msg.frame !== frame) post({ type: 'seek', frame });
                    }
                    emit('time', { frame: msg.frame, playing: msg.playing });
                    break;
                case 'thumb': {
                    const entry = thumbs.get(String(msg.req));
                    if (entry) {
                        thumbs.delete(String(msg.req));
                        clearTimeout(entry.timer);
                        entry.resolve(msg.bitmap);
                    } else if (typeof msg.bitmap.close === 'function') {
                        msg.bitmap.close();
                    }
                    break;
                }
                case 'error':
                    emit('error', msg);
                    break;
                default:
                    break;
            }
        }

        // The message of the document and its load event arrive in either order, so the load event only arms a watchdog: a
        // document that does not say `ready` is reported as unavailable (CSP, a blocked script or a missing bundle).
        function onFrameLoad() {
            clearTimeout(loadTimer);
            const before = readyCount;
            loadTimer = setTimeout(() => { if (!disposed && readyCount === before && !ready) emit('unavailable'); }, 6000);
        }

        root.addEventListener('message', onMessage);
        iframe.addEventListener('load', onFrameLoad);
        iframe.src = o.src;
        if (o.container) o.container.appendChild(iframe);

        return {
            iframe,
            on(name, fn) {
                (handlers[name] = handlers[name] || []).push(fn);
                return () => { handlers[name] = (handlers[name] || []).filter(candidate => candidate !== fn); };
            },
            // load(payload, resumeFrame): the next `load` message (story, catalog, assets, fonts, code_bundles, format); the
            // frame is shown again after the compile (the host starts at frame 0).
            load(payload, resumeFrame) {
                pending = payload;
                resume = Number(resumeFrame) > 0 ? Math.floor(resumeFrame) : 0;
                failThumbs(new Error('superseded'));
                sendLoad();
            },
            play() { return loaded && post({ type: 'play' }); },
            pause() { return loaded && post({ type: 'pause' }); },
            seek(frame) { return loaded && post({ type: 'seek', frame: Math.max(0, Math.floor(frame)) }); },
            reset() { loaded = false; pending = null; failThumbs(new Error('reset')); return post({ type: 'reset' }); },
            // thumb(sceneId, width, frame?) -> Promise<ImageBitmap>
            thumb(sceneId, width, frame) {
                if (!loaded) return Promise.reject(new Error('not loaded'));
                const req = 't' + (++thumbSeq);
                return new Promise((resolve, reject) => {
                    const timer = setTimeout(() => { thumbs.delete(req); reject(new Error('timeout')); }, LIMITS.thumbWaitMs);
                    thumbs.set(req, { resolve, reject, timer });
                    const message = { type: 'thumb', req, scene_id: sceneId, width };
                    if (Number.isInteger(frame) && frame >= 0) message.frame = frame;
                    if (!post(message)) { clearTimeout(timer); thumbs.delete(req); reject(new Error('unavailable')); }
                });
            },
            isReady: () => ready,
            isLoaded: () => loaded,
            droppedCount: () => dropped,
            dispose() {
                if (disposed) return;
                failThumbs(new Error('disposed'));
                post({ type: 'reset' });
                disposed = true;
                clearTimeout(loadTimer);
                root.removeEventListener('message', onMessage);
                iframe.removeEventListener('load', onFrameLoad);
                iframe.remove();
            }
        };
    }

    S.createPlayer = createPlayer;
    S.player = { checkIncoming, LIMITS };
})(typeof globalThis !== 'undefined' ? globalThis : window);
